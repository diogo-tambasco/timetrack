// assigned_test.go — fase 18 T-06: visões cross-workspace Assigned/Requested
// (CA-18/19/20/21/22). White-box (package tasks): exercita a Service real
// contra o dbtest e confere o conjunto/ordem/contexto retornados.
package tasks

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

type assignedSeed struct {
	me, other    string // dois users
	wsA, wsB     string // dois workspaces que ME participa
	wsC          string // workspace que ME NÃO participa
	clientA      string
	projAShared  string // shared no wsA, com cliente
	projBShared  string // shared no wsB
	projAPrivMe  string // private no wsA, owner = me
	projAPrivOth string // private no wsA, owner = other
	projCShared  string // shared no wsC (me não é membro)
}

func seedAssigned(t *testing.T, pool *pgxpool.Pool) assignedSeed {
	t.Helper()
	ctx := context.Background()
	var s assignedSeed
	scan := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
		return id
	}
	s.me = scan(`INSERT INTO users (google_id, email, name) VALUES ('me', 'me@b.c', 'Me') RETURNING id`)
	s.other = scan(`INSERT INTO users (google_id, email, name) VALUES ('ot', 'ot@b.c', 'Other') RETURNING id`)
	s.wsA = scan(`INSERT INTO workspaces (name) VALUES ('WS A') RETURNING id`)
	s.wsB = scan(`INSERT INTO workspaces (name) VALUES ('WS B') RETURNING id`)
	s.wsC = scan(`INSERT INTO workspaces (name) VALUES ('WS C') RETURNING id`)
	mem := func(ws, u, role string) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`, ws, u, role); err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}
	mem(s.wsA, s.me, "owner")
	mem(s.wsB, s.me, "member")
	mem(s.wsA, s.other, "member")
	mem(s.wsC, s.other, "owner") // me NÃO é membro do wsC
	s.clientA = scan(`INSERT INTO clients (workspace_id, name) VALUES ($1, 'Acme') RETURNING id`, s.wsA)
	s.projAShared = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id, client_id)
		VALUES ($1, 'PA', '#4f9dde', 'shared', $2, $3) RETURNING id`, s.wsA, s.other, s.clientA)
	s.projBShared = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'PB', '#22c55e', 'shared', $2) RETURNING id`, s.wsB, s.me)
	s.projAPrivMe = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'PrivMe', '#000000', 'private', $2) RETURNING id`, s.wsA, s.me)
	s.projAPrivOth = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'PrivOth', '#111111', 'private', $2) RETURNING id`, s.wsA, s.other)
	s.projCShared = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'PC', '#333333', 'shared', $2) RETURNING id`, s.wsC, s.other)
	return s
}

// mkTask insere uma task com assignee/requested_by/due_date explícitos.
func mkTaskFull(t *testing.T, pool *pgxpool.Pool, ws, proj, name string, assignee, requestedBy, dueDate *string, creator string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO tasks (workspace_id, project_id, name, created_by, assignee, requested_by, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date) RETURNING id`,
		ws, proj, name, creator, assignee, requestedBy, dueDate).Scan(&id); err != nil {
		t.Fatalf("mkTaskFull (%s): %v", name, err)
	}
	return id
}

func ids(list []AssignedTask) map[string]AssignedTask {
	m := map[string]AssignedTask{}
	for _, a := range list {
		m[a.ID] = a
	}
	return m
}

// CA-18/20/21 — Assigned cruza workspaces, respeita visibilidade e isolamento.
func TestAssignedCrossWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedAssigned(t, pool)
	me, ot := &s.me, &s.other

	// tasks atribuídas a MIM em wsA (shared) e wsB (shared) → ambas aparecem
	d1 := "2026-06-10"
	tA := mkTaskFull(t, pool, s.wsA, s.projAShared, "TaskA", me, ot, &d1, s.other)
	tB := mkTaskFull(t, pool, s.wsB, s.projBShared, "TaskB", me, nil, nil, s.me)
	// task atribuída a mim num projeto private MEU em wsA → aparece (sou owner)
	tPrivMe := mkTaskFull(t, pool, s.wsA, s.projAPrivMe, "PrivMine", me, nil, nil, s.me)
	// task atribuída a mim num projeto private de OUTRO → NÃO aparece (CA-20)
	mkTaskFull(t, pool, s.wsA, s.projAPrivOth, "PrivOther", me, nil, nil, s.other)
	// task atribuída a OUTRO no wsA shared → NÃO aparece (CA-21 filtro assignee)
	mkTaskFull(t, pool, s.wsA, s.projAShared, "NotMine", ot, nil, nil, s.other)
	// task atribuída a mim num ws que NÃO participo (wsC) → NÃO aparece (membership CA-21)
	mkTaskFull(t, pool, s.wsC, s.projCShared, "OrphanWs", me, nil, nil, s.other)

	got, err := svc.Assigned(context.Background(), s.me)
	if err != nil {
		t.Fatalf("Assigned: %v", err)
	}
	m := ids(got)
	if _, ok := m[tA]; !ok {
		t.Errorf("TaskA (wsA shared) deveria aparecer")
	}
	if _, ok := m[tB]; !ok {
		t.Errorf("TaskB (wsB shared) deveria aparecer")
	}
	if _, ok := m[tPrivMe]; !ok {
		t.Errorf("PrivMine (private meu) deveria aparecer (CA-20 edge)")
	}
	if len(got) != 3 {
		t.Fatalf("esperadas exatamente 3 tasks (cross-ws, visíveis, minhas), vieram %d: %+v", len(got), got)
	}

	// contexto cross-ws enriquecido na TaskA
	a := m[tA]
	if a.Project.ID != s.projAShared || a.Project.Name != "PA" || a.Project.Color != "#4f9dde" {
		t.Errorf("contexto de projeto errado: %+v", a.Project)
	}
	if a.Workspace.ID != s.wsA || a.Workspace.Name != "WS A" {
		t.Errorf("contexto de workspace errado: %+v", a.Workspace)
	}
	if a.Client == nil || *a.Client != "Acme" {
		t.Errorf("cliente deveria ser Acme: %v", a.Client)
	}
	// task sem cliente → Client nil
	if m[tB].Client != nil {
		t.Errorf("TaskB não tem cliente, deveria ser nil: %v", *m[tB].Client)
	}
}

// CA-22 — ordem estável: due_date NULLS LAST, depois created_at, id.
func TestAssignedOrdemEstavel(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedAssigned(t, pool)
	me := &s.me

	dEarly := "2026-01-01"
	dLate := "2026-12-31"
	tLate := mkTaskFull(t, pool, s.wsA, s.projAShared, "late", me, nil, &dLate, s.other)
	tEarly := mkTaskFull(t, pool, s.wsA, s.projAShared, "early", me, nil, &dEarly, s.other)
	tNoDue := mkTaskFull(t, pool, s.wsA, s.projAShared, "nodue", me, nil, nil, s.other)

	got, err := svc.Assigned(context.Background(), s.me)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("esperadas 3, vieram %d", len(got))
	}
	// early (jan) < late (dez) < nodue (NULLS LAST)
	if got[0].ID != tEarly || got[1].ID != tLate || got[2].ID != tNoDue {
		t.Errorf("ordem deveria ser early,late,nodue; veio %s,%s,%s", got[0].Name, got[1].Name, got[2].Name)
	}
}

// CA-19 — Requested filtra por requested_by = eu.
func TestRequestedFiltraRequestedBy(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedAssigned(t, pool)
	me, ot := &s.me, &s.other

	// pedida por mim (assignee = other) em wsA shared
	tReq := mkTaskFull(t, pool, s.wsA, s.projAShared, "PedidaPorMim", ot, me, nil, s.me)
	// atribuída a mim mas pedida por outro → NÃO entra em Requested
	mkTaskFull(t, pool, s.wsA, s.projAShared, "MinhaMasPedidaPorOutro", me, ot, nil, s.other)

	got, err := svc.Requested(context.Background(), s.me)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != tReq {
		t.Fatalf("Requested deveria trazer só a pedida por mim, veio %+v", got)
	}
}

// lista vazia → slice não-nil (CA-22).
func TestAssignedVazioNaoNil(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedAssigned(t, pool)
	got, err := svc.Assigned(context.Background(), s.me)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Errorf("lista vazia deveria ser [] não-nil")
	}
	if len(got) != 0 {
		t.Errorf("sem tasks atribuídas, esperado 0, veio %d", len(got))
	}
}
