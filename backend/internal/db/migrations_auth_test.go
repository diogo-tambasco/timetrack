package db

import (
	"context"
	"strings"
	"testing"
)

// Fase 01 T-02 — TestMigracoesAuth: schema de auth completo via runner.
// CA-16: enum de roles restringe workspace_members.role no banco.
func TestMigracoesAuth(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// as 5 tabelas da fase existem
	for _, table := range []string{"users", "workspaces", "workspace_members", "sessions", "oauth_tokens"} {
		var ok bool
		if err := pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Errorf("tabela %s não existe (err=%v)", table, err)
		}
	}

	// seed mínimo pra exercitar constraints
	var userID, wsID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-1', 'a@b.c', 'Ana') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	// CA-16: role válida funciona…
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`,
		wsID, userID); err != nil {
		t.Errorf("role 'member' deveria ser aceita: %v", err)
	}
	// …e role fora do enum falha NO BANCO
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'admin')`,
		wsID, userID); err == nil {
		t.Error("role 'admin' deveria falhar no enum workspace_role")
	}

	// users.google_id e users.email são UNIQUE
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-1', 'x@y.z', 'Dup')`); err == nil {
		t.Error("google_id duplicado deveria falhar (UNIQUE)")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-2', 'a@b.c', 'Dup')`); err == nil {
		t.Error("email duplicado deveria falhar (UNIQUE)")
	}

	// oauth_tokens tem PK composta (user_id, provider)
	if _, err := pool.Exec(ctx,
		`INSERT INTO oauth_tokens (user_id, provider, access_token_enc, scopes)
		 VALUES ($1, 'google', '\x01'::bytea, '{openid}')`, userID); err != nil {
		t.Fatalf("insert oauth_tokens: %v", err)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO oauth_tokens (user_id, provider, access_token_enc, scopes)
		 VALUES ($1, 'google', '\x02'::bytea, '{openid}')`, userID)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Errorf("PK composta (user_id, provider) deveria barrar duplicata, veio: %v", err)
	}
}
