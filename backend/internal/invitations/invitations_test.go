// invitations_test.go — harness compartilhado dos testes do service (fase 17):
// seeds de user/workspace/membership por SQL direto no schema de teste.
package invitations

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

// seedUser cria um user e devolve o id.
func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $2, $3) RETURNING id`,
		googleID, email, name).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
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

// seedMember vincula um user a um workspace com a role dada.
func seedMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		wsID, userID, role); err != nil {
		t.Fatalf("seed member %s/%s: %v", userID, role, err)
	}
}

// memberRole devolve a role do user no workspace, ou "" se não for membro.
func memberRole(t *testing.T, pool *pgxpool.Pool, wsID, userID string) string {
	t.Helper()
	var role string
	err := pool.QueryRow(context.Background(),
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		wsID, userID).Scan(&role)
	if err != nil {
		return ""
	}
	return role
}

// domainCode extrai o code do *domain.Error (ou "" se não for um).
func domainCode(err error) string {
	if de, ok := err.(*domain.Error); ok {
		return de.Code
	}
	return ""
}

// newService monta o Service de teste com pool migrado e clock real.
func newService(t *testing.T, opts ...Option) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.NewPool(t)
	return NewService(pool, opts...), pool
}

// countInvitations conta os convites de um workspace (asserts de efeito).
func countInvitations(t *testing.T, pool *pgxpool.Pool, wsID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM invitations WHERE workspace_id = $1`, wsID).Scan(&n); err != nil {
		t.Fatalf("contar convites: %v", err)
	}
	return n
}
