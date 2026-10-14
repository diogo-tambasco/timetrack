package db

import (
	"context"
	"testing"
)

// Fase 02 T-01 — TestMigrationsCoreSchema: schema do domínio core
// (clients, projects, project_members, tasks) conforme plan.md §2.
// Enums restringem visibility/access/status NO BANCO.
func TestMigrationsCoreSchema(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	for _, table := range []string{"clients", "projects", "project_members", "tasks"} {
		var ok bool
		if err := pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Errorf("tabela %s não existe (err=%v)", table, err)
		}
	}

	// seed mínimo da fase 01 pra exercitar as FKs
	var userID, wsID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-1', 'a@b.c', 'Ana') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	// clients: colunas canônicas (archived_at nullable, created_at default)
	var clientID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO clients (workspace_id, name) VALUES ($1, 'Acme')
		 RETURNING id`, wsID).Scan(&clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	var archivedNull bool
	if err := pool.QueryRow(ctx,
		`SELECT archived_at IS NULL AND created_at IS NOT NULL FROM clients WHERE id = $1`,
		clientID).Scan(&archivedNull); err != nil || !archivedNull {
		t.Errorf("clients deveria nascer com archived_at NULL e created_at preenchido (err=%v)", err)
	}

	// projects: enum project_visibility válido + colunas canônicas
	var projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal, visibility, owner_user_id)
		 VALUES ($1, $2, 'App', '#4f9dde', 10.5, 'shared', $3)
		 RETURNING id`, wsID, clientID, userID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'X', '#000000', 'public', $2)`, wsID, userID); err == nil {
		t.Error("visibility 'public' deveria falhar no enum project_visibility")
	}

	// project_members: enum project_access + PK composta
	if _, err := pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, 'edit')`,
		projectID, userID); err != nil {
		t.Errorf("access 'edit' deveria ser aceito: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, 'view')`,
		projectID, userID); err == nil {
		t.Error("PK composta (project_id, user_id) deveria barrar duplicata")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, 'admin')`,
		projectID, userID); err == nil {
		t.Error("access 'admin' deveria falhar no enum project_access")
	}

	// tasks: status default 'todo', workspace_id denormalizado, notes default '',
	// created_by NOT NULL, assignee/requested_by nullable
	var taskID, status, notes string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'Onboarding', $3)
		 RETURNING id, status, notes`, wsID, projectID, userID).Scan(&taskID, &status, &notes); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if status != "todo" {
		t.Errorf("status default esperado 'todo', veio %q", status)
	}
	if notes != "" {
		t.Errorf("notes default esperado '', veio %q", notes)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'feito' WHERE id = $1`, taskID); err == nil {
		t.Error("status 'feito' deveria falhar no enum task_status")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET due_date = '2026-06-15', recurrence = 'FREQ=WEEKLY', assignee = $2, requested_by = $2
		 WHERE id = $1`, taskID, userID); err != nil {
		t.Errorf("due_date/recurrence/assignee/requested_by deveriam existir: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name) VALUES ($1, $2, 'sem autor')`,
		wsID, projectID); err == nil {
		t.Error("created_by é NOT NULL — insert sem autor deveria falhar")
	}
}
