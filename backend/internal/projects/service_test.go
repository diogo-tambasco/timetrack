package projects_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/projects"
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

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name)
		 VALUES ($1, $1 || '@example.com', 'User') RETURNING id`, googleID).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedClient(t *testing.T, pool *pgxpool.Pool, wsID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO clients (workspace_id, name) VALUES ($1, $2) RETURNING id`,
		wsID, name).Scan(&id); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return id
}

func assertDomainCode(t *testing.T, err error, code, contexto string) {
	t.Helper()
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != code {
		t.Errorf("%s: esperado %s, veio %v", contexto, code, err)
	}
}

func ptr[T any](v T) *T { return &v }

// CA-06.1 — nome vazio, só espaços ou >120: validation_error, nada persiste.
func TestProjetoNomeInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	ctx := context.Background()

	for _, nome := range []string{"", "   ", strings.Repeat("a", 121)} {
		_, err := svc.Create(ctx, ws, owner, projects.CreateInput{
			Name: nome, Color: "#4f9dde", Visibility: "shared",
		})
		assertDomainCode(t, err, "validation_error", "Create(nome "+nome+")")
	}
	if n := dbtest.CountRows(t, pool, "projects"); n != 0 {
		t.Fatalf("nenhum projeto deveria ter sido persistido, há %d", n)
	}

	// fronteira: 120 após trim passa, com owner_user_id da sessão (CA-06)
	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: " " + strings.Repeat("b", 120) + " ", Color: "#4f9dde", Visibility: "shared",
	})
	if err != nil {
		t.Fatalf("nome com 120 caracteres após trim deveria passar: %v", err)
	}
	if p.OwnerUserID != owner {
		t.Errorf("owner_user_id deveria ser o usuário da sessão, veio %q", p.OwnerUserID)
	}
}

// CA-07 — cor fora de #RRGGBB ou meta fora de (0,168]: validation_error.
func TestProjetoCorEMetaInvalidas(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	ctx := context.Background()

	for _, cor := range []string{"", "4f9dde", "#4f9dd", "#4f9ddez", "#4f9dde0", "blue"} {
		_, err := svc.Create(ctx, ws, owner, projects.CreateInput{
			Name: "App", Color: cor, Visibility: "shared",
		})
		assertDomainCode(t, err, "validation_error", "Create(cor "+cor+")")
	}

	for _, meta := range []float64{0, -1, 168.01, 999} {
		_, err := svc.Create(ctx, ws, owner, projects.CreateInput{
			Name: "App", Color: "#4f9dde", Visibility: "shared", WeeklyHoursGoal: ptr(meta),
		})
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != "validation_error" {
			t.Errorf("Create(meta %v): esperado validation_error, veio %v", meta, err)
		}
	}
	if n := dbtest.CountRows(t, pool, "projects"); n != 0 {
		t.Fatalf("nada deveria ter sido persistido, há %d", n)
	}

	// fronteiras válidas: maiúsculas no hex e meta = 168
	if _, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4F9Dde", Visibility: "private", WeeklyHoursGoal: ptr(168.0),
	}); err != nil {
		t.Fatalf("cor #4F9Dde e meta 168 deveriam ser aceitas: %v", err)
	}
}

// CA-08 — client_id existente mas de OUTRO workspace: invalid_reference.
func TestProjetoClienteDeOutroWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	wsA := seedWorkspace(t, pool)
	wsB := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	clienteB := seedClient(t, pool, wsB, "Cliente do B")
	ctx := context.Background()

	_, err := svc.Create(ctx, wsA, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared", ClientID: &clienteB,
	})
	assertDomainCode(t, err, "invalid_reference", "Create(cliente de outro ws)")

	// client_id inexistente cai no MESMO invalid_reference
	fantasma := "00000000-0000-0000-0000-000000000000"
	_, err = svc.Create(ctx, wsA, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared", ClientID: &fantasma,
	})
	assertDomainCode(t, err, "invalid_reference", "Create(cliente inexistente)")

	if n := dbtest.CountRows(t, pool, "projects"); n != 0 {
		t.Fatalf("nada deveria ter sido persistido, há %d", n)
	}
}

// CA-08.1 — client_id de cliente ARQUIVADO: invalid_reference; cliente
// ativo do mesmo workspace passa.
func TestProjetoClienteArquivado400(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	cliente := seedClient(t, pool, ws, "Acme")
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE clients SET archived_at = now() WHERE id = $1`, cliente); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared", ClientID: &cliente,
	})
	assertDomainCode(t, err, "invalid_reference", "Create(cliente arquivado)")

	ativo := seedClient(t, pool, ws, "Lauda")
	if _, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared", ClientID: &ativo,
	}); err != nil {
		t.Fatalf("cliente ativo do mesmo workspace deveria passar: %v", err)
	}
}
