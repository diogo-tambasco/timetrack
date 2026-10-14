// board_test.go — fase 12 T-05: read agregado do projeto (CA-16/16.1/17, D-06).
// Pina: tracked por task = task_id ∈ projeto (sem filtrar project_id da entry);
// tracked do projeto = por project_id; buckets independentes (sem invariante
// projeto ≥ Σ); entry de outro user não soma (CA-15); can_write (CA-22).
package objectives

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

func mkTask(t *testing.T, pool *pgxpool.Pool, s gateSeed, objectiveID *string, status string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, objective_id, status)
		 VALUES ($1, $2, 'T', $3, $4, $5) RETURNING id`,
		s.ws, s.shared, s.pOwner, objectiveID, status).Scan(&id); err != nil {
		t.Fatalf("mkTask: %v", err)
	}
	return id
}

func mkEntry(t *testing.T, pool *pgxpool.Pool, ws, user string, projectID, taskID *string, seconds int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, now() - make_interval(secs => $5), now(), $5, 'manual')`,
		ws, user, projectID, taskID, seconds); err != nil {
		t.Fatalf("mkEntry: %v", err)
	}
}

func findObj(b Board, id string) *BoardObjective {
	for i := range b.Objectives {
		if b.Objectives[i].ID == id {
			return &b.Objectives[i]
		}
	}
	return nil
}

func TestBoardArvore(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	p := s.shared

	o1, _ := svc.Create(ctx, s.ws, s.pOwner, "member", p, CreateInput{Name: "O1", GoalHours: ptr(2.0)})
	t1 := mkTask(t, pool, s, &o1.ID, "done")
	t2 := mkTask(t, pool, s, &o1.ID, "todo")
	t3 := mkTask(t, pool, s, nil, "todo") // Geral

	mkEntry(t, pool, s.ws, s.pOwner, &p, &t1, 3600)
	mkEntry(t, pool, s.ws, s.pOwner, &p, &t2, 1800)
	mkEntry(t, pool, s.ws, s.pOwner, &p, &t3, 600)
	mkEntry(t, pool, s.ws, s.pOwner, &p, nil, 1200) // sem task → só projeto

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", p)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if b.Project.TrackedSeconds != 7200 {
		t.Errorf("projeto tracked = %d, esperava 7200", b.Project.TrackedSeconds)
	}
	if !b.Project.CanWrite {
		t.Error("dono deveria ter can_write=true")
	}
	o := findObj(b, o1.ID)
	if o == nil {
		t.Fatal("objetivo O1 não veio no board")
	}
	if o.TaskCount != 2 || o.DoneCount != 1 || o.DonePct != 50 {
		t.Errorf("O1 progresso = (%d,%d,%d%%), esperava (2,1,50%%)", o.TaskCount, o.DoneCount, o.DonePct)
	}
	if o.TrackedSeconds != 5400 {
		t.Errorf("O1 tracked = %d, esperava 5400 (3600+1800)", o.TrackedSeconds)
	}
	if len(o.Tasks) != 2 {
		t.Errorf("O1 deveria ter 2 tasks, veio %d", len(o.Tasks))
	}
	if b.General.TaskCount != 1 || b.General.TrackedSeconds != 600 {
		t.Errorf("Geral = (%d tasks, %ds), esperava (1, 600)", b.General.TaskCount, b.General.TrackedSeconds)
	}
}

func TestBoardEntrySemProjectIdContaNaTask(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o1, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "O1"})
	t1 := mkTask(t, pool, s, &o1.ID, "todo")
	// entry com task do projeto mas SEM project_id → conta na task/objetivo, NÃO no projeto (D-06)
	mkEntry(t, pool, s.ws, s.pOwner, nil, &t1, 1000)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	o := findObj(b, o1.ID)
	if o.TrackedSeconds != 1000 {
		t.Errorf("objetivo deveria contar a entry sem project_id (1000), veio %d", o.TrackedSeconds)
	}
	if b.Project.TrackedSeconds != 0 {
		t.Errorf("projeto NÃO deveria contar entry sem project_id, veio %d (Σ tasks pode exceder o projeto — D-06)", b.Project.TrackedSeconds)
	}
}

func TestBoardEntryOutroUserNaoSoma(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o1, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "O1"})
	t1 := mkTask(t, pool, s, &o1.ID, "todo")
	p := s.shared
	mkEntry(t, pool, s.ws, s.pOwner, &p, &t1, 1000) // do usuário da sessão
	mkEntry(t, pool, s.ws, s.mNone, &p, &t1, 5000)  // de OUTRO usuário — NÃO deve somar (CA-15)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	o := findObj(b, o1.ID)
	if o.TrackedSeconds != 1000 {
		t.Errorf("só o tempo do user da sessão deveria somar (1000), veio %d (vazamento entre usuários?)", o.TrackedSeconds)
	}
	if b.Project.TrackedSeconds != 1000 {
		t.Errorf("projeto só do user da sessão (1000), veio %d", b.Project.TrackedSeconds)
	}
}

func TestBoardCanWriteEViewArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "O"})

	// membro access=view → can_write false (CA-22)
	bView, err := svc.Board(ctx, s.ws, s.mView, "member", s.shared)
	if err != nil {
		t.Fatalf("board view: %v", err)
	}
	if bView.Project.CanWrite {
		t.Error("access=view deveria ter can_write=false")
	}
	// membro access=edit → can_write true
	bEdit, _ := svc.Board(ctx, s.ws, s.mEdit, "member", s.shared)
	if !bEdit.Project.CanWrite {
		t.Error("access=edit deveria ter can_write=true")
	}
	// projeto arquivado → can_write false mesmo pro dono
	bArch, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.archived)
	if err != nil {
		t.Fatalf("board arquivado: %v", err)
	}
	if bArch.Project.CanWrite {
		t.Error("projeto arquivado deveria ter can_write=false")
	}
}

func TestBoardObjetivoDeletadoSomeTasksViramGeral(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o1, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "O1"})
	t1 := mkTask(t, pool, s, &o1.ID, "todo")
	p := s.shared
	mkEntry(t, pool, s.ws, s.pOwner, &p, &t1, 900)

	if err := svc.Delete(ctx, s.ws, s.pOwner, "member", s.shared, o1.ID); err != nil {
		t.Fatalf("delete objetivo: %v", err)
	}
	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(b.Objectives) != 0 {
		t.Errorf("objetivo deletado não deveria aparecer, veio %d", len(b.Objectives))
	}
	if b.General.TaskCount != 1 || b.General.TrackedSeconds != 900 {
		t.Errorf("task órfã deveria cair no Geral com tracked preservado, veio (%d, %d)", b.General.TaskCount, b.General.TrackedSeconds)
	}
}
