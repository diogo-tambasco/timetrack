// board_v2_test.go — fase 24 T-05: board estendido (CA-14/14.1). BoardTask ganha
// priority/notion_url/position + os agregados de sub-task (subtask_count/done,
// goal_count/done) numa query única (sem N+1); tasks ordenadas por position;
// BoardProject ganha client_name via LEFT JOIN clients.
package objectives

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

func findTask(tasks []BoardTask, id string) *BoardTask {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

func mkSubtask(t *testing.T, pool *pgxpool.Pool, ws, taskID string, done, isGoal bool, pos int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO task_subtasks (task_id, workspace_id, title, done, is_goal, position)
		VALUES ($1, $2, 'st', $3, $4, $5)`, taskID, ws, done, isGoal, pos); err != nil {
		t.Fatalf("mkSubtask: %v", err)
	}
}

// CA-14: BoardTask traz priority/notion_url/position e os agregados de sub-task.
func TestBoardCamposV2EAgregados(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	p := s.shared

	// task com priority/notion_url + 4 sub-tasks. As contagens são propositalmente
	// DIVERGENTES (subtask_done=2 ≠ goal_done=1) pra pinar que os dois filtros são
	// distintos: count(*) FILTER (WHERE done) conta TODAS as feitas (meta ou não),
	// count(*) FILTER (WHERE is_goal AND done) só as metas feitas. Sem a sub-task
	// done-geral abaixo, ambos dariam 1 e uma mutação conflando os filtros sobreviveria.
	var t1 string
	url := "https://www.notion.so/Doc-abc"
	if err := pool.QueryRow(ctx, `
		INSERT INTO tasks (workspace_id, project_id, name, created_by, status, priority, notion_url)
		VALUES ($1,$2,'T1',$3,'todo',2,$4) RETURNING id`, s.ws, p, s.pOwner, url).Scan(&t1); err != nil {
		t.Fatalf("insert t1: %v", err)
	}
	mkSubtask(t, pool, s.ws, t1, true, true, 0)   // done + meta (meta-feita)
	mkSubtask(t, pool, s.ws, t1, false, true, 1)  // meta não-feita
	mkSubtask(t, pool, s.ws, t1, false, false, 2) // geral não-feita
	mkSubtask(t, pool, s.ws, t1, true, false, 3)  // geral FEITA (conta em done, NÃO em goal_done)

	// task sem sub-task → contagens 0.
	t2 := mkTask(t, pool, s, nil, "todo")

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", p)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	bt1 := findTask(b.General.Tasks, t1)
	if bt1 == nil {
		t.Fatal("t1 não veio no board")
	}
	if bt1.Priority != 2 {
		t.Errorf("priority deveria ser 2, veio %d", bt1.Priority)
	}
	if bt1.NotionURL == nil || *bt1.NotionURL != url {
		t.Errorf("notion_url deveria ser %q, veio %v", url, bt1.NotionURL)
	}
	// done=2 (meta-feita + geral-feita) ≠ goalDone=1 (só a meta-feita) — divergência
	// que mata a mutação que conflaria count(WHERE done) com count(WHERE is_goal AND done).
	if bt1.SubtaskCount != 4 || bt1.SubtaskDone != 2 || bt1.GoalCount != 2 || bt1.GoalDone != 1 {
		t.Errorf("agregados de t1 errados: count=%d done=%d goalCount=%d goalDone=%d (esperado 4/2/2/1)",
			bt1.SubtaskCount, bt1.SubtaskDone, bt1.GoalCount, bt1.GoalDone)
	}
	bt2 := findTask(b.General.Tasks, t2)
	if bt2 == nil {
		t.Fatal("t2 não veio no board")
	}
	if bt2.SubtaskCount != 0 || bt2.SubtaskDone != 0 || bt2.GoalCount != 0 || bt2.GoalDone != 0 {
		t.Errorf("task sem sub-task deveria ter contagens 0, veio %d/%d/%d/%d",
			bt2.SubtaskCount, bt2.SubtaskDone, bt2.GoalCount, bt2.GoalDone)
	}
}

// CA-14: tasks de cada bucket ordenadas por position asc (mutação que tira o
// ORDER BY position morre).
func TestBoardOrdenaPorPosition(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	p := s.shared

	// 2 tasks no MESMO status; a criada PRIMEIRO tem position 1, a 2ª position 0.
	var first, second string
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (workspace_id, project_id, name, created_by, status, position) VALUES ($1,$2,'first',$3,'todo',1) RETURNING id`, s.ws, p, s.pOwner).Scan(&first); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (workspace_id, project_id, name, created_by, status, position) VALUES ($1,$2,'second',$3,'todo',0) RETURNING id`, s.ws, p, s.pOwner).Scan(&second); err != nil {
		t.Fatalf("second: %v", err)
	}
	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", p)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(b.General.Tasks) != 2 {
		t.Fatalf("esperava 2 tasks no geral, veio %d", len(b.General.Tasks))
	}
	// position 0 (second) deve vir antes de position 1 (first).
	if b.General.Tasks[0].ID != second || b.General.Tasks[1].ID != first {
		t.Errorf("tasks deveriam vir ordenadas por position [second(0), first(1)], veio [%s, %s]",
			b.General.Tasks[0].Name, b.General.Tasks[1].Name)
	}
}

// CA-14.1: BoardProject.client_name = nome do cliente; null se projeto sem cliente.
func TestBoardClientName(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// projeto sem cliente (s.shared) → null.
	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board sem cliente: %v", err)
	}
	if b.Project.ClientName != nil {
		t.Errorf("projeto sem cliente deveria ter client_name null, veio %v", b.Project.ClientName)
	}

	// projeto COM cliente → nome.
	var clientID, projID string
	if err := pool.QueryRow(ctx, `INSERT INTO clients (workspace_id, name) VALUES ($1,'Lauda.ai') RETURNING id`, s.ws).Scan(&clientID); err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO projects (workspace_id, client_id, name, color, visibility, owner_user_id) VALUES ($1,$2,'PC','#4f9dde','shared',$3) RETURNING id`, s.ws, clientID, s.pOwner).Scan(&projID); err != nil {
		t.Fatalf("proj com cliente: %v", err)
	}
	b, err = svc.Board(ctx, s.ws, s.pOwner, "member", projID)
	if err != nil {
		t.Fatalf("board com cliente: %v", err)
	}
	if b.Project.ClientName == nil || *b.Project.ClientName != "Lauda.ai" {
		t.Errorf("client_name deveria ser 'Lauda.ai', veio %v", b.Project.ClientName)
	}
}
