package db

import (
	"context"
	"testing"
)

// Fase 28 T-01 — TestMigracaoProjectsDeadline: a migração 0025 adiciona a
// coluna projects.deadline (date, NULL default — CA-02) e o índice parcial
// idx_tasks_due em (project_id, due_date). Projeto criado sem deadline fica
// NULL; aceita uma data (CA-01).
func TestMigracaoProjectsDeadline(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// coluna projects.deadline = date, nullable.
	var dataType, isNullable string
	if err := pool.QueryRow(ctx,
		`SELECT data_type, is_nullable FROM information_schema.columns
		  WHERE table_name = 'projects' AND table_schema = current_schema() AND column_name = 'deadline'`).Scan(&dataType, &isNullable); err != nil {
		t.Fatalf("projects.deadline não existe: %v", err)
	}
	if dataType != "date" {
		t.Errorf("projects.deadline deveria ser date, veio %q", dataType)
	}
	if isNullable != "YES" {
		t.Errorf("projects.deadline deveria ser nullable, veio is_nullable=%q", isNullable)
	}

	// índice idx_tasks_due presente.
	var oid string
	if err := pool.QueryRow(ctx, `
		SELECT i.indexrelid::regclass::text
		  FROM pg_index i
		 WHERE i.indrelid = 'tasks'::regclass
		   AND i.indexrelid = to_regclass('idx_tasks_due')`).Scan(&oid); err != nil {
		t.Fatalf("índice idx_tasks_due não encontrado em tasks: %v", err)
	}

	// fixtures: user/ws pra exercitar o default e o aceite de data.
	var userID, wsID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-dl', 'dl@b.c', 'DL') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS DL') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}

	// projeto criado sem deadline → NULL (CA-02).
	var deadline *string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING deadline`,
		wsID, userID).Scan(&deadline); err != nil {
		t.Fatalf("insert project sem deadline: %v", err)
	}
	if deadline != nil {
		t.Errorf("deadline default deveria ser NULL, veio %v", *deadline)
	}

	// projeto aceita uma data (CA-01).
	var got string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility, deadline)
		 VALUES ($1, $2, 'P2', '#112233', 'shared', '2025-09-30')
		 RETURNING to_char(deadline, 'YYYY-MM-DD')`,
		wsID, userID).Scan(&got); err != nil {
		t.Fatalf("insert project com deadline: %v", err)
	}
	if got != "2025-09-30" {
		t.Errorf("deadline deveria ser 2025-09-30, veio %q", got)
	}
}
