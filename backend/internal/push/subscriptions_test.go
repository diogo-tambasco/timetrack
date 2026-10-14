// subscriptions_test.go — TDD do repo (fase 16 T-04, CA-13): upsert por
// endpoint (re-inscrever não duplica), troca de user reatribui, ListByUser
// isolado por user, Delete por endpoint.
package push_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/push"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', 'U') RETURNING id`,
		googleID).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", googleID, err)
	}
	return id
}

func countSubs(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM push_subscriptions`).Scan(&n); err != nil {
		t.Fatalf("contar subs: %v", err)
	}
	return n
}

// CA-13 — upsert por endpoint: re-inscrever o mesmo navegador NÃO duplica,
// substitui as chaves.
func TestUpsertPorEndpointNaoDuplica(t *testing.T) {
	pool := dbtest.NewPool(t)
	repo := push.NewSubscriptions(pool)
	u := seedUser(t, pool, "g-upsert")
	ctx := context.Background()

	if err := repo.Upsert(ctx, u, push.Subscription{Endpoint: "https://p/x", P256dh: "k1", Auth: "a1"}); err != nil {
		t.Fatalf("1º upsert: %v", err)
	}
	// re-inscreve o MESMO endpoint com chaves novas
	if err := repo.Upsert(ctx, u, push.Subscription{Endpoint: "https://p/x", P256dh: "k2", Auth: "a2"}); err != nil {
		t.Fatalf("2º upsert: %v", err)
	}
	if n := countSubs(t, pool); n != 1 {
		t.Fatalf("upsert por endpoint deveria manter 1 linha, veio %d", n)
	}
	subs, err := repo.ListByUser(ctx, u)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 || subs[0].P256dh != "k2" || subs[0].Auth != "a2" {
		t.Errorf("as chaves deveriam ser as do 2º upsert (k2/a2), veio %+v", subs)
	}
}

// CA-13 — troca de user no mesmo endpoint reatribui a inscrição (o device
// trocou de conta).
func TestUpsertTrocaDeUserReatribui(t *testing.T) {
	pool := dbtest.NewPool(t)
	repo := push.NewSubscriptions(pool)
	uA := seedUser(t, pool, "g-troca-a")
	uB := seedUser(t, pool, "g-troca-b")
	ctx := context.Background()

	if err := repo.Upsert(ctx, uA, push.Subscription{Endpoint: "https://p/y", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if err := repo.Upsert(ctx, uB, push.Subscription{Endpoint: "https://p/y", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	if n := countSubs(t, pool); n != 1 {
		t.Fatalf("ainda deveria ser 1 linha (reatribuída), veio %d", n)
	}
	subsA, _ := repo.ListByUser(ctx, uA)
	subsB, _ := repo.ListByUser(ctx, uB)
	if len(subsA) != 0 {
		t.Errorf("a inscrição não deveria mais pertencer a A, veio %d", len(subsA))
	}
	if len(subsB) != 1 {
		t.Errorf("a inscrição deveria ter sido reatribuída a B, veio %d", len(subsB))
	}
}

// CA-13 — ListByUser só vê as do user (isolamento). A mutação removendo o
// WHERE user_id deve quebrar este teste.
func TestListByUserIsolado(t *testing.T) {
	pool := dbtest.NewPool(t)
	repo := push.NewSubscriptions(pool)
	uA := seedUser(t, pool, "g-list-a")
	uB := seedUser(t, pool, "g-list-b")
	ctx := context.Background()

	_ = repo.Upsert(ctx, uA, push.Subscription{Endpoint: "https://p/a1", P256dh: "k", Auth: "a"})
	_ = repo.Upsert(ctx, uA, push.Subscription{Endpoint: "https://p/a2", P256dh: "k", Auth: "a"})
	_ = repo.Upsert(ctx, uB, push.Subscription{Endpoint: "https://p/b1", P256dh: "k", Auth: "a"})

	subsA, err := repo.ListByUser(ctx, uA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(subsA) != 2 {
		t.Errorf("A deveria ter 2 subs (isolado de B), veio %d", len(subsA))
	}
	for _, s := range subsA {
		if s.Endpoint == "https://p/b1" {
			t.Errorf("vazou a sub de B na lista de A")
		}
	}
}

// CA-16 — Delete por endpoint remove a linha.
func TestDeletePorEndpoint(t *testing.T) {
	pool := dbtest.NewPool(t)
	repo := push.NewSubscriptions(pool)
	u := seedUser(t, pool, "g-del")
	ctx := context.Background()

	_ = repo.Upsert(ctx, u, push.Subscription{Endpoint: "https://p/del", P256dh: "k", Auth: "a"})
	if err := repo.Delete(ctx, "https://p/del"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := countSubs(t, pool); n != 0 {
		t.Errorf("a sub deveria ter sido removida, sobraram %d", n)
	}
	// delete de endpoint inexistente não erra
	if err := repo.Delete(ctx, "https://p/inexistente"); err != nil {
		t.Errorf("delete de endpoint inexistente não deveria errar, veio %v", err)
	}
}
