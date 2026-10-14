package comments_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedUser cria um user e devolve o id.
func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $2, $3) RETURNING id`,
		googleID, email, name).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", googleID, err)
	}
	return id
}

// seedWorkspace cria um workspace e devolve o id.
func seedWorkspace(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed workspace %s: %v", name, err)
	}
	return id
}

// addMember vincula um user a um workspace com a role dada.
func addMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		wsID, userID, role); err != nil {
		t.Fatalf("vincular %s como %s: %v", userID, role, err)
	}
}

// seedProject cria um projeto com a visibilidade dada e devolve o id.
func seedProject(t *testing.T, pool *pgxpool.Pool, wsID, ownerID, name, visibility string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, $3, '#112233', $4) RETURNING id`,
		wsID, ownerID, name, visibility).Scan(&id); err != nil {
		t.Fatalf("seed project %s: %v", name, err)
	}
	return id
}

// seedTask cria uma task. assignee nil → assignee IS NULL.
func seedTask(t *testing.T, pool *pgxpool.Pool, wsID, projID, createdBy string, assignee *string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, assignee)
		 VALUES ($1, $2, 'T', $3, $4) RETURNING id`,
		wsID, projID, createdBy, assignee).Scan(&id); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return id
}
