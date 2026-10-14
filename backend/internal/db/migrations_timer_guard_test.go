package db

import (
	"context"
	"strings"
	"testing"
)

// Fase 16 T-01 — TestMigracaoTimerGuard: a migração 0016 adiciona
// time_entries.auto_cut (default false — backstop dos CAs do corte) e cria
// a tabela push_subscriptions (PK endpoint, FK user_id cascade, índice por
// user_id). Entries antigas viram auto_cut=false sem reprocessamento.
func TestMigracaoTimerGuard(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var userA, userB, wsID, entryID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-tg-a', 'tga@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-tg-b', 'tgb@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	// time_entries.auto_cut: insert SEM o campo → default false (entries
	// antigas continuam não-cortadas; CA-01).
	var autoCut bool
	if err := pool.QueryRow(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'timer') RETURNING id, auto_cut`,
		wsID, userA).Scan(&entryID, &autoCut); err != nil {
		t.Fatalf("insert entry com default de auto_cut: %v", err)
	}
	if autoCut {
		t.Errorf("auto_cut default deveria ser false, veio %v", autoCut)
	}

	// auto_cut é NOT NULL: NULL explícito viola a constraint.
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source, auto_cut)
		 VALUES ($1, $2, now(), 'timer', NULL)`, wsID, userB); err == nil {
		t.Errorf("auto_cut NULL deveria violar NOT NULL")
	}

	// push_subscriptions: insert por user, leitura de volta.
	if _, err := pool.Exec(ctx,
		`INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth)
		 VALUES ('https://push.example/abc', $1, 'p256dh-x', 'auth-x')`, userA); err != nil {
		t.Fatalf("insert push_subscription: %v", err)
	}

	// PK por endpoint: o MESMO endpoint não pode inserir duas vezes (o upsert
	// do repo resolve isso com ON CONFLICT).
	if _, err := pool.Exec(ctx,
		`INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth)
		 VALUES ('https://push.example/abc', $1, 'p2', 'a2')`, userB); err == nil {
		t.Errorf("endpoint duplicado deveria violar a PK")
	}

	// p256dh e auth são NOT NULL.
	for _, c := range []struct {
		desc string
		sql  string
	}{
		{"p256dh NULL", `INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth) VALUES ('e2', $1, NULL, 'a')`},
		{"auth NULL", `INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth) VALUES ('e3', $1, 'p', NULL)`},
	} {
		if _, err := pool.Exec(ctx, c.sql, userA); err == nil {
			t.Errorf("%s deveria violar NOT NULL", c.desc)
		}
	}

	// FK cascade: apagar o user remove suas subscriptions. Usa um user
	// dedicado SEM entries (entries não têm cascade pra users) pra isolar o
	// efeito do ON DELETE CASCADE de push_subscriptions.
	var userC string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-tg-c', 'tgc@b.c', 'Cau') RETURNING id`).Scan(&userC); err != nil {
		t.Fatalf("insert user C: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth)
		 VALUES ('https://push.example/cascade', $1, 'p', 'a')`, userC); err != nil {
		t.Fatalf("insert push_subscription do user C: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userC); err != nil {
		t.Fatalf("delete user C (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM push_subscriptions WHERE endpoint = 'https://push.example/cascade'`).Scan(&n); err != nil {
		t.Fatalf("contar subscriptions após cascade: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE deveria remover a subscription do user removido, sobraram %d", n)
	}

	// índice por user_id presente (CA-01 — RNF-01 pede leitura indexada).
	var idxName string
	if err := pool.QueryRow(ctx,
		`SELECT indexname FROM pg_indexes WHERE tablename = 'push_subscriptions' AND indexname = 'idx_push_subs_user'`).Scan(&idxName); err != nil {
		t.Fatalf("índice idx_push_subs_user não encontrado: %v", err)
	}
	if !strings.Contains(idxName, "idx_push_subs_user") {
		t.Errorf("índice por user_id esperado, veio %q", idxName)
	}
}
