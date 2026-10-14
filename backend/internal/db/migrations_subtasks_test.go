package db

import (
	"context"
	"testing"
)

// Fase 24 T-02 — TestMigracaoTaskSubtasks: a migração 0022 cria task_subtasks
// ("goals da task") com workspace_id denormalizado, defaults done=false/
// is_goal=false/position=0, FK task_id ON DELETE CASCADE e os índices
// (task_id, position) + parcial (task_id) WHERE is_goal (CA-06/09/10/14).
func TestMigracaoTaskSubtasks(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// tabela existe.
	var ok bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('task_subtasks') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("tabela task_subtasks não existe (err=%v)", err)
	}

	// seed: user + ws + project + task.
	var userID, wsID, projID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-sub', 'sub@b.c', 'Sub') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Sub') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3) RETURNING id`, wsID, projID, userID).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// defaults: done=false, is_goal=false, position=0, created_at preenchido.
	var done, isGoal bool
	var position int
	var createdOK bool
	if err := pool.QueryRow(ctx,
		`INSERT INTO task_subtasks (task_id, workspace_id, title)
		 VALUES ($1, $2, 'Revisar')
		 RETURNING done, is_goal, position, created_at IS NOT NULL`,
		taskID, wsID).Scan(&done, &isGoal, &position, &createdOK); err != nil {
		t.Fatalf("insert subtask com defaults: %v", err)
	}
	if done || isGoal {
		t.Errorf("done/is_goal deveriam nascer false, veio done=%v is_goal=%v", done, isGoal)
	}
	if position != 0 {
		t.Errorf("position default deveria ser 0, veio %d", position)
	}
	if !createdOK {
		t.Errorf("created_at deveria nascer preenchido")
	}

	// índices existem.
	for _, idx := range []string{"idx_subtasks_task", "idx_subtasks_task_goal"} {
		if err := pool.QueryRow(ctx, `
			SELECT 1 FROM pg_index i
			 WHERE i.indrelid = 'task_subtasks'::regclass
			   AND i.indexrelid = to_regclass($1)`, idx).Scan(new(int)); err != nil {
			t.Fatalf("índice %s não encontrado: %v", idx, err)
		}
	}
	// idx_subtasks_task_goal é PARCIAL (WHERE is_goal).
	var hasPred bool
	if err := pool.QueryRow(ctx, `
		SELECT i.indpred IS NOT NULL FROM pg_index i
		 WHERE i.indrelid = 'task_subtasks'::regclass
		   AND i.indexrelid = to_regclass('idx_subtasks_task_goal')`).Scan(&hasPred); err != nil {
		t.Fatalf("ler idx_subtasks_task_goal: %v", err)
	}
	if !hasPred {
		t.Errorf("idx_subtasks_task_goal deveria ser PARCIAL (WHERE is_goal)")
	}

	// FK task_id ON DELETE CASCADE: apagar a task pai limpa as sub-tasks (CA-09).
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("delete task (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM task_subtasks WHERE task_id = $1`, taskID).Scan(&n); err != nil {
		t.Fatalf("contar sub-tasks após cascade: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE da task deveria limpar as sub-tasks, sobraram %d", n)
	}
}
