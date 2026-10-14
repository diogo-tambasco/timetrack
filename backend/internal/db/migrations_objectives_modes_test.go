package db

import (
	"context"
	"testing"
)

// Fase 29 T-01 — TestMigracaoObjectivesModes: a migração 0026 adiciona
// objectives.completion_mode (text NOT NULL DEFAULT 'by_tasks', CHECK nos 3
// valores — CA-01) e objectives.target_date (date nullable). Objetivos da fase
// 12 (sem esses campos) seguem válidos: completion_mode='by_tasks', target_date
// NULL. CHECK barra 'foo'. SEM enum novo (text+CHECK).
func TestMigracaoObjectivesModes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// completion_mode = text NOT NULL DEFAULT 'by_tasks'.
	var dataType, isNullable, colDefault string
	if err := pool.QueryRow(ctx,
		`SELECT data_type, is_nullable, COALESCE(column_default, '')
		   FROM information_schema.columns
		  WHERE table_name = 'objectives' AND table_schema = current_schema() AND column_name = 'completion_mode'`).
		Scan(&dataType, &isNullable, &colDefault); err != nil {
		t.Fatalf("objectives.completion_mode não existe: %v", err)
	}
	if dataType != "text" {
		t.Errorf("completion_mode deveria ser text, veio %q", dataType)
	}
	if isNullable != "NO" {
		t.Errorf("completion_mode deveria ser NOT NULL, veio is_nullable=%q", isNullable)
	}
	if colDefault == "" || colDefault[0:1] != "'" {
		t.Errorf("completion_mode default deveria ser 'by_tasks', veio %q", colDefault)
	}

	// target_date = date nullable.
	var tdType, tdNullable string
	if err := pool.QueryRow(ctx,
		`SELECT data_type, is_nullable FROM information_schema.columns
		  WHERE table_name = 'objectives' AND table_schema = current_schema() AND column_name = 'target_date'`).Scan(&tdType, &tdNullable); err != nil {
		t.Fatalf("objectives.target_date não existe: %v", err)
	}
	if tdType != "date" {
		t.Errorf("target_date deveria ser date, veio %q", tdType)
	}
	if tdNullable != "YES" {
		t.Errorf("target_date deveria ser nullable, veio is_nullable=%q", tdNullable)
	}

	// seed mínimo pras FKs.
	var userID, wsID, projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-mode', 'm@b.c', 'M') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Mode') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`,
		wsID, userID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// objetivo criado SEM os campos novos (jeito da fase 12) → default by_tasks,
	// target_date NULL (objetivos existentes seguem válidos — CA-01).
	var mode string
	var targetDate *string
	if err := pool.QueryRow(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'Legado fase 12', $3)
		 RETURNING completion_mode, to_char(target_date, 'YYYY-MM-DD')`,
		wsID, projectID, userID).Scan(&mode, &targetDate); err != nil {
		t.Fatalf("insert objective legado: %v", err)
	}
	if mode != "by_tasks" {
		t.Errorf("completion_mode default deveria ser 'by_tasks', veio %q", mode)
	}
	if targetDate != nil {
		t.Errorf("target_date default deveria ser NULL, veio %v", *targetDate)
	}

	// CHECK barra modo inválido.
	if _, err := pool.Exec(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by, completion_mode)
		 VALUES ($1, $2, 'Inválido', $3, 'foo')`,
		wsID, projectID, userID); err == nil {
		t.Error("completion_mode = 'foo' deveria violar o CHECK")
	}

	// aceita by_hours/by_date + target_date YYYY-MM-DD.
	var got string
	if err := pool.QueryRow(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by, completion_mode, target_date)
		 VALUES ($1, $2, 'Marco', $3, 'by_date', '2026-07-15')
		 RETURNING to_char(target_date, 'YYYY-MM-DD')`,
		wsID, projectID, userID).Scan(&got); err != nil {
		t.Fatalf("insert objective by_date com target_date: %v", err)
	}
	if got != "2026-07-15" {
		t.Errorf("target_date deveria ser 2026-07-15, veio %q", got)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by, completion_mode)
		 VALUES ($1, $2, 'Horas', $3, 'by_hours')`,
		wsID, projectID, userID); err != nil {
		t.Errorf("by_hours deveria ser aceito: %v", err)
	}
}
