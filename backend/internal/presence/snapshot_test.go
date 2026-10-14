// snapshot_test.go — fase 21 T-05: presence.Service.Snapshot (CA-11/14, D-05).
// Lista SÓ os membros do ws com timer rodando (stopped_at IS NULL), isolado por
// e.workspace_id + join de membership; visibilidade priv/shared aplicada no
// service. Mutação removendo o WHERE e.workspace_id mata o teste de isolamento.
package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/presence"
)

// seedRunning semeia uma entry RODANDO (stopped_at NULL) do user no ws.
func seedRunning(t *testing.T, pool *pgxpool.Pool, wsID, userID string, projectID *string, started time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, started_at, source)
		VALUES ($1, $2, $3, '', $4, 'timer')`,
		wsID, userID, projectID, started); err != nil {
		t.Fatalf("seed running entry: %v", err)
	}
}

func seedStopped(t *testing.T, pool *pgxpool.Pool, wsID, userID string, started, stopped time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, NULL, '', $3, $4, 60, 'timer')`,
		wsID, userID, started, stopped); err != nil {
		t.Fatalf("seed stopped entry: %v", err)
	}
}

func newSnapSvc(pool *pgxpool.Pool) *presence.Service {
	return presence.NewService(pool, func(string, any) {})
}

// CA-11 — snapshot lista os membros do ws com timer rodando; cada um com a regra
// de visibilidade (shared mostra nome, private omite); ocioso não aparece.
func TestSnapshotListaRodandoComVisibilidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	viewer := mkUser(t, pool, "snap-viewer")
	a := mkUser(t, pool, "snap-a")
	b := mkUser(t, pool, "snap-b")
	c := mkUser(t, pool, "snap-c") // ocioso
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, viewer, "owner")
	mkMember(t, pool, ws, a, "member")
	mkMember(t, pool, ws, b, "member")
	mkMember(t, pool, ws, c, "member")
	pShared := mkProject(t, pool, ws, a, "Site", "shared")
	pPrivate := mkProject(t, pool, ws, b, "Segredo", "private") // dono = b, viewer NÃO vê
	started := time.Now().UTC().Add(-5 * time.Minute)
	seedRunning(t, pool, ws, a, &pShared, started)
	seedRunning(t, pool, ws, b, &pPrivate, started)
	// c não tem entry rodando → não aparece

	list, err := newSnapSvc(pool).Snapshot(context.Background(), ws, viewer)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("snapshot deveria ter 2 rodando (a, b), veio %d: %+v", len(list), list)
	}
	byUser := map[string]presence.PresencePayload{}
	for _, p := range list {
		if !p.Running {
			t.Errorf("snapshot só traz running:true: %+v", p)
		}
		if p.WorkspaceID != ws {
			t.Errorf("workspace_id deveria ser %s: %+v", ws, p)
		}
		if p.Since == nil {
			t.Errorf("since deveria ser preenchido: %+v", p)
		}
		byUser[p.UserID] = p
	}
	if pa, ok := byUser[a]; !ok || pa.Project == nil || *pa.Project != "Site" {
		t.Errorf("A (shared) deveria mostrar o nome: %+v", byUser[a])
	}
	if pb, ok := byUser[b]; !ok || pb.Project != nil {
		t.Errorf("B (private de B, viewer não dono) NÃO deveria mostrar o nome: %+v", byUser[b])
	}
}

// CA-14 — isolamento de tenant: a entry rodando de um membro em OUTRO workspace
// dele NÃO aparece no snapshot de W. Mutação removendo WHERE e.workspace_id mata
// o teste.
func TestSnapshotIsoladoPorTenant(t *testing.T) {
	pool := dbtest.NewPool(t)
	viewer := mkUser(t, pool, "snapiso-viewer")
	a := mkUser(t, pool, "snapiso-a")
	w1 := mkWorkspace(t, pool, "W1")
	w2 := mkWorkspace(t, pool, "W2")
	mkMember(t, pool, w1, viewer, "owner")
	mkMember(t, pool, w1, a, "member")
	mkMember(t, pool, w2, a, "owner")
	// A está rodando uma entry em W2 (não no W1 do viewer)
	seedRunning(t, pool, w2, a, nil, time.Now().UTC())

	list, err := newSnapSvc(pool).Snapshot(context.Background(), w1, viewer)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("snapshot de W1 NÃO pode listar a entry de A rodando em W2: %+v", list)
	}
}

// D-05 — entry parada (stopped_at != NULL) nunca entra no snapshot.
func TestSnapshotIgnoraEntryParada(t *testing.T) {
	pool := dbtest.NewPool(t)
	viewer := mkUser(t, pool, "snapstop-v")
	a := mkUser(t, pool, "snapstop-a")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, viewer, "owner")
	mkMember(t, pool, ws, a, "member")
	now := time.Now().UTC()
	seedStopped(t, pool, ws, a, now.Add(-2*time.Hour), now.Add(-time.Hour))

	list, err := newSnapSvc(pool).Snapshot(context.Background(), ws, viewer)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("entry parada não pode aparecer no snapshot: %+v", list)
	}
}

// (verificação do build) D-03 — o Snapshot NÃO inclui o PRÓPRIO viewer: A não
// aparece na sua própria faixa de presence (o delta exclui A via m2.user_id<>A;
// o snapshot tem de concordar — senão A fica preso na faixa após parar o timer,
// já que o running:false do delta nunca chega a A). Mutação removendo
// `e.user_id <> viewer` mata este teste.
func TestSnapshotExcluiOProprioViewer(t *testing.T) {
	pool := dbtest.NewPool(t)
	viewer := mkUser(t, pool, "snapself-v")
	b := mkUser(t, pool, "snapself-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, viewer, "owner")
	mkMember(t, pool, ws, b, "member")
	started := time.Now().UTC().Add(-time.Minute)
	seedRunning(t, pool, ws, viewer, nil, started) // o PRÓPRIO viewer rodando
	seedRunning(t, pool, ws, b, nil, started)

	list, err := newSnapSvc(pool).Snapshot(context.Background(), ws, viewer)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	// só B aparece; o viewer NÃO se vê na própria faixa (D-03).
	if len(list) != 1 || list[0].UserID != b {
		t.Errorf("snapshot deveria listar só B (não o próprio viewer), veio %+v", list)
	}
}
