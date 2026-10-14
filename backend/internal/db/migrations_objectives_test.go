package db

import (
	"context"
	"testing"
)

// Fase 12 T-01 — TestMigrationsObjectives: a migração 0013_objectives cria a
// tabela objectives (camada entre projeto e task), liga tasks.objective_id
// (ON DELETE SET NULL → bucket "Geral", CA-07/11) e o índice da soma por task
// do board (idx_entries_task, RNF-04). goal_hours é numeric(6,2) pra caber a
// faixa (0,9999] (numeric(5,2) estouraria em 9999 — furo da verificação da spec).
func TestMigrationsObjectives(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// tabela existe
	var ok bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('objectives') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("tabela objectives não existe (err=%v)", err)
	}

	// seed mínimo (fase 01/02) pras FKs
	var userID, wsID, projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-obj', 'o@b.c', 'Obj') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'App', '#4f9dde', 'shared', $2) RETURNING id`, wsID, userID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// defaults: status='open', description='', position=0, goal_hours NULL aceito
	var objID, status, description string
	var position int
	var goalHours *float64
	if err := pool.QueryRow(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'Lançar onboarding', $3)
		 RETURNING id, status, description, position, goal_hours`,
		wsID, projectID, userID).Scan(&objID, &status, &description, &position, &goalHours); err != nil {
		t.Fatalf("insert objective: %v", err)
	}
	if status != "open" {
		t.Errorf("status default esperado 'open', veio %q", status)
	}
	if description != "" {
		t.Errorf("description default esperado '', veio %q", description)
	}
	if position != 0 {
		t.Errorf("position default esperado 0, veio %d", position)
	}
	if goalHours != nil {
		t.Errorf("goal_hours sem meta deveria ser NULL, veio %v", *goalHours)
	}

	// enum objective_status fechado
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET status = 'concluido' WHERE id = $1`, objID); err == nil {
		t.Error("status 'concluido' deveria falhar no enum objective_status")
	}

	// CHECK goal_hours: barra <=0 e >9999; aceita o teto 9999 (cabe em numeric(6,2))
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET goal_hours = 0 WHERE id = $1`, objID); err == nil {
		t.Error("goal_hours = 0 deveria violar o CHECK (>0)")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET goal_hours = 10000 WHERE id = $1`, objID); err == nil {
		t.Error("goal_hours = 10000 deveria violar o CHECK (<=9999)")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET goal_hours = 9999 WHERE id = $1`, objID); err != nil {
		t.Errorf("goal_hours = 9999 deveria caber em numeric(6,2) e passar o CHECK: %v", err)
	}

	// tasks.objective_id + ON DELETE SET NULL (objetivo some → task vira "Geral", nunca apaga)
	var taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, objective_id)
		 VALUES ($1, $2, 'Definir telas', $3, $4) RETURNING id`,
		wsID, projectID, userID, objID).Scan(&taskID); err != nil {
		t.Fatalf("insert task com objective_id: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM objectives WHERE id = $1`, objID); err != nil {
		t.Fatalf("delete objective: %v", err)
	}
	var stillThere bool
	var orphan *string
	if err := pool.QueryRow(ctx,
		`SELECT objective_id IS NULL FROM tasks WHERE id = $1`, taskID).Scan(&stillThere); err != nil {
		t.Fatalf("task deveria sobreviver ao delete do objetivo (não apagar): %v", err)
	}
	if !stillThere {
		t.Error("ON DELETE SET NULL: objective_id da task deveria virar NULL após apagar o objetivo")
	}
	_ = orphan

	// idx_entries_task existe (soma por task do board — RNF-04). to_regclass
	// resolve pelo search_path → o índice DESTE schema de teste (sem falso
	// positivo de schemas paralelos).
	var idxExists bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass('idx_entries_task') IS NOT NULL`).Scan(&idxExists); err != nil || !idxExists {
		t.Errorf("índice idx_entries_task deveria existir (err=%v)", err)
	}
}
