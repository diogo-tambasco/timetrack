package clients_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

func seedWorkspace(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return id
}

// CA-02 — nome vazio, só espaços ou >120 caracteres: erro de validação e
// NADA persiste.
func TestClienteNomeInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := clients.NewService(pool)
	ws := seedWorkspace(t, pool)
	ctx := context.Background()

	for _, nome := range []string{"", "   ", strings.Repeat("a", 121)} {
		_, err := svc.Create(ctx, ws, nome)
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != "validation_error" {
			t.Errorf("Create(%q): esperado validation_error, veio %v", nome, err)
		}
	}
	if n := dbtest.CountRows(t, pool, "clients"); n != 0 {
		t.Fatalf("nenhum cliente deveria ter sido persistido, há %d", n)
	}

	// fronteira: 120 caracteres após trim é válido (CA-01)
	valido, err := svc.Create(ctx, ws, "  "+strings.Repeat("b", 120)+"  ")
	if err != nil {
		t.Fatalf("nome com 120 caracteres após trim deveria ser aceito: %v", err)
	}
	if valido.Name != strings.Repeat("b", 120) {
		t.Errorf("nome deveria ser persistido com trim, veio %q", valido.Name)
	}

	// renomear segue a MESMA regra de validação
	if _, err := svc.Rename(ctx, ws, valido.ID, "   "); err == nil {
		t.Error("Rename com nome só de espaços deveria falhar com validation_error")
	}
}
