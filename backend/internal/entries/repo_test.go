package entries_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/entries"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $2, $2) RETURNING id`,
		googleID, email).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return id
}

func seedWorkspace(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed workspace %s: %v", name, err)
	}
	return id
}

func closedEntry(wsID, userID, desc string, start time.Time) entries.Entry {
	stop := start.Add(30 * time.Minute)
	dur := 1800
	return entries.Entry{
		WorkspaceID: wsID, UserID: userID, Description: desc,
		Tags: []string{}, StartedAt: start, StoppedAt: &stop,
		DurationSeconds: &dur, Source: "manual",
	}
}

// janela ampla: o recorte fino por dia/tz é responsabilidade do T-12
func wideWindow() (time.Time, time.Time) {
	return time.Now().Add(-24 * time.Hour), time.Now().Add(24 * time.Hour)
}

// CA-01 — entries de um workspace NUNCA aparecem na listagem de outro,
// mesmo sendo do MESMO usuário; update/delete cruzando workspace não acham
// a entry (vazamento = bug crítico).
func TestIsolamentoEntriesPorWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()

	user := seedUser(t, pool, "g-iso", "iso@tt.dev")
	wsA := seedWorkspace(t, pool, "A")
	wsB := seedWorkspace(t, pool, "B")

	base := time.Now().UTC().Truncate(time.Second)
	for i, desc := range []string{"a1", "a2"} {
		if _, err := entries.Insert(ctx, pool, closedEntry(wsA, user, desc, base.Add(time.Duration(i)*time.Hour))); err != nil {
			t.Fatalf("insert em A: %v", err)
		}
	}
	inB, err := entries.Insert(ctx, pool, closedEntry(wsB, user, "b1", base))
	if err != nil {
		t.Fatalf("insert em B: %v", err)
	}

	from, to := wideWindow()
	listA, err := entries.List(ctx, pool, wsA, user, from, to)
	if err != nil {
		t.Fatalf("listar A: %v", err)
	}
	if len(listA) != 2 {
		t.Fatalf("workspace A deveria listar 2 entries, veio %d", len(listA))
	}
	for _, e := range listA {
		if e.WorkspaceID != wsA {
			t.Errorf("entry %s de outro workspace VAZOU na listagem de A", e.ID)
		}
	}

	// leitura cruzada: a entry de B não existe sob A
	if _, err := entries.GetOwned(ctx, pool, wsA, user, inB.ID); !errors.As(err, new(*domain.Error)) {
		t.Errorf("GetOwned cruzando workspace deveria responder not_found, veio %v", err)
	}
	// delete cruzado não remove nada
	if err := entries.Delete(ctx, pool, wsA, user, inB.ID); err == nil {
		t.Error("Delete cruzando workspace deveria responder not_found")
	}
	listB, err := entries.List(ctx, pool, wsB, user, from, to)
	if err != nil {
		t.Fatalf("listar B: %v", err)
	}
	if len(listB) != 1 || listB[0].ID != inB.ID {
		t.Errorf("entry de B deveria seguir intacta após o delete cruzado (veio %d entries)", len(listB))
	}
}

// CA-01.1 — com 2 usuários no MESMO workspace, a listagem (e o
// get/delete) de um nunca alcança entries do outro.
func TestListagemSoEntriesProprias(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()

	ws := seedWorkspace(t, pool, "Compartilhado")
	ana := seedUser(t, pool, "g-ana", "ana@tt.dev")
	bia := seedUser(t, pool, "g-bia", "bia@tt.dev")

	base := time.Now().UTC().Truncate(time.Second)
	minha, err := entries.Insert(ctx, pool, closedEntry(ws, ana, "da ana", base))
	if err != nil {
		t.Fatalf("insert da ana: %v", err)
	}
	dela, err := entries.Insert(ctx, pool, closedEntry(ws, bia, "da bia", base.Add(time.Hour)))
	if err != nil {
		t.Fatalf("insert da bia: %v", err)
	}

	from, to := wideWindow()
	listAna, err := entries.List(ctx, pool, ws, ana, from, to)
	if err != nil {
		t.Fatalf("listar da ana: %v", err)
	}
	if len(listAna) != 1 || listAna[0].ID != minha.ID {
		t.Fatalf("listagem da ana deveria ter SÓ a entry dela (veio %d)", len(listAna))
	}

	// entry de outro usuário responde como inexistente (CA-29.3)
	if _, err := entries.GetOwned(ctx, pool, ws, ana, dela.ID); err == nil {
		t.Error("GetOwned de entry alheia deveria responder not_found")
	}
	if err := entries.Delete(ctx, pool, ws, ana, dela.ID); err == nil {
		t.Error("Delete de entry alheia deveria responder not_found")
	}
	listBia, err := entries.List(ctx, pool, ws, bia, from, to)
	if err != nil {
		t.Fatalf("listar da bia: %v", err)
	}
	if len(listBia) != 1 || listBia[0].ID != dela.ID {
		t.Errorf("entry da bia deveria seguir intacta (veio %d entries)", len(listBia))
	}
}
