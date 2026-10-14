package db

import (
	"context"
	"strings"
	"testing"
)

// Fase 17 T-01 — TestMigracaoInvitations: a migração 0017 cria a tabela
// invitations (convites de workspace) com status default 'pending', CHECK de
// status, FK cascade no workspace, FK em invited_by, token UNIQUE, role
// reusando o enum workspace_role (0003). O índice único PARCIAL
// ux_invitations_pending barra 2 pending pro mesmo (workspace, lower(email)) e
// PERMITE um 2º pending depois de revogar o 1º (CA-02/09).
func TestMigracaoInvitations(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var userA, wsA, wsB string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-inv-a', 'inva@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS A') RETURNING id`).Scan(&wsA); err != nil {
		t.Fatalf("insert workspace A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS B') RETURNING id`).Scan(&wsB); err != nil {
		t.Fatalf("insert workspace B: %v", err)
	}

	// insert SEM status → default 'pending' (CA-01 backstop).
	var status string
	var invID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'cadu@example.com', 'member', 'tok-1', $2)
		 RETURNING id, status`, wsA, userA).Scan(&invID, &status); err != nil {
		t.Fatalf("insert invitation com default de status: %v", err)
	}
	if status != "pending" {
		t.Errorf("status default deveria ser pending, veio %q", status)
	}

	// CHECK de status: valor fora de {pending,accepted,revoked} é rejeitado.
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by, status)
		 VALUES ($1, 'x@example.com', 'member', 'tok-bad-status', $2, 'foo')`, wsA, userA); err == nil {
		t.Errorf("status inválido deveria violar o CHECK")
	}

	// token UNIQUE: o mesmo token não pode existir em 2 linhas.
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'outro@example.com', 'member', 'tok-1', $2)`, wsA, userA); err == nil {
		t.Errorf("token duplicado deveria violar o UNIQUE")
	}

	// índice único PARCIAL: 2 pending pro mesmo (workspace, lower(email)) →
	// viola (case-insensitive). 'CADU@example.com' colide com 'cadu@example.com'.
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'CADU@example.com', 'member', 'tok-2', $2)`, wsA, userA); err == nil {
		t.Errorf("2º pending pro mesmo (workspace, lower(email)) deveria violar o índice parcial")
	}

	// o MESMO email pending em OUTRO workspace é OK (escopo por workspace_id).
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'cadu@example.com', 'member', 'tok-3', $2)`, wsB, userA); err != nil {
		t.Errorf("mesmo email pending em outro workspace deveria ser permitido: %v", err)
	}

	// revogar o 1º pending LIBERA um 2º pending pro mesmo email/workspace (CA-09).
	if _, err := pool.Exec(ctx,
		`UPDATE invitations SET status = 'revoked' WHERE id = $1`, invID); err != nil {
		t.Fatalf("revogar convite: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'cadu@example.com', 'member', 'tok-4', $2)`, wsA, userA); err != nil {
		t.Errorf("novo pending após revogar o 1º deveria ser permitido (índice só barra pending↔pending): %v", err)
	}

	// FK cascade no workspace: apagar o workspace remove seus convites.
	if _, err := pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, wsB); err != nil {
		t.Fatalf("delete workspace B (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM invitations WHERE workspace_id = $1`, wsB).Scan(&n); err != nil {
		t.Fatalf("contar convites após cascade: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE deveria remover os convites do workspace removido, sobraram %d", n)
	}

	// role reusa o enum workspace_role: role fora do enum é rejeitada.
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (workspace_id, email, role, token, invited_by)
		 VALUES ($1, 'enum@example.com', 'gerente', 'tok-enum', $2)`, wsA, userA); err == nil {
		t.Errorf("role fora do enum workspace_role deveria falhar")
	}

	// índice de resolução no login presente (RNF-02 — busca por email pendente).
	var idxName string
	if err := pool.QueryRow(ctx,
		`SELECT indexname FROM pg_indexes WHERE tablename = 'invitations' AND indexname = 'idx_invitations_email_pending'`).Scan(&idxName); err != nil {
		t.Fatalf("índice idx_invitations_email_pending não encontrado: %v", err)
	}
	if !strings.Contains(idxName, "idx_invitations_email_pending") {
		t.Errorf("índice de resolução por email esperado, veio %q", idxName)
	}
}
