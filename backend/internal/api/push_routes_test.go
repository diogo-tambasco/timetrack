// push_routes_test.go — fase 16 T-06: POST /api/v1/push/subscribe + GET
// /api/v1/push/key (CA-13/14). Rotas POR USUÁRIO atrás do RequireAuth.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/push"
	"timetrack.live/backend/internal/workspace"
)

// newPushRouter monta o router REAL (auth 01 + push 16) com a VAPID public key
// dada (vazia → push desligado, /push/key vazio).
func newPushRouter(t *testing.T, pool *pgxpool.Pool, vapidPub string) (http.Handler, *auth.Service, *googlestub.Server, *push.Subscriptions) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	subs := push.NewSubscriptions(pool)
	sender := push.NewSender(subs, vapidPub, "priv-x", "mailto:x@y.z", nil)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithPush(api.Push{Subs: subs, Sender: sender}),
	)
	return router, svc, stub, subs
}

func countUserSubs(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM push_subscriptions WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("contar subs: %v", err)
	}
	return n
}

// CA-13 — subscribe válido 200 + persiste sob o user da sessão.
func TestPushSubscribeValidoPersiste(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newPushRouter(t, pool, "pub-key")
	cookie := loginVia(t, svc, stub, "g-sub1", "sub1@tt.dev", "Sub")
	uid := userIDOf(t, pool, "g-sub1")

	rec := doJSON(router, http.MethodPost, "/api/v1/push/subscribe", cookie,
		`{"endpoint":"https://push/abc","keys":{"p256dh":"k","auth":"a"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe válido esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if n := countUserSubs(t, pool, uid); n != 1 {
		t.Errorf("a subscription deveria ser persistida sob o user da sessão, veio %d", n)
	}
}

// CA-13 — re-subscribe do mesmo endpoint não duplica (upsert).
func TestPushSubscribeUpsertNaoDuplica(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newPushRouter(t, pool, "pub-key")
	cookie := loginVia(t, svc, stub, "g-sub2", "sub2@tt.dev", "Sub")
	uid := userIDOf(t, pool, "g-sub2")

	body := `{"endpoint":"https://push/x","keys":{"p256dh":"k1","auth":"a1"}}`
	doJSON(router, http.MethodPost, "/api/v1/push/subscribe", cookie, body)
	doJSON(router, http.MethodPost, "/api/v1/push/subscribe", cookie,
		`{"endpoint":"https://push/x","keys":{"p256dh":"k2","auth":"a2"}}`)
	if n := countUserSubs(t, pool, uid); n != 1 {
		t.Errorf("re-subscribe do mesmo endpoint não deveria duplicar, veio %d", n)
	}
}

// CA-13 — corpo inválido (sem endpoint/chaves) → 400 validation_error.
func TestPushSubscribeInvalido400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newPushRouter(t, pool, "pub-key")
	cookie := loginVia(t, svc, stub, "g-sub3", "sub3@tt.dev", "Sub")

	for _, body := range []string{
		`{"keys":{"p256dh":"k","auth":"a"}}`,               // sem endpoint
		`{"endpoint":"https://p/x","keys":{"auth":"a"}}`,   // sem p256dh
		`{"endpoint":"https://p/x","keys":{"p256dh":"k"}}`, // sem auth
		`{"endpoint":"","keys":{"p256dh":"k","auth":"a"}}`, // endpoint vazio
	} {
		rec := doJSON(router, http.MethodPost, "/api/v1/push/subscribe", cookie, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("corpo %q esperava 400, veio %d", body, rec.Code)
		}
		if c := errCode(t, rec); c != "validation_error" {
			t.Errorf("corpo %q esperava validation_error, veio %q", body, c)
		}
	}
}

// CA-13 — sem sessão → 401 canônico nas duas rotas.
func TestPushSemSessao401(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, _, _, _ := newPushRouter(t, pool, "pub-key")

	rec := doJSON(router, http.MethodPost, "/api/v1/push/subscribe", nil,
		`{"endpoint":"https://p/x","keys":{"p256dh":"k","auth":"a"}}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("subscribe sem sessão esperava 401, veio %d", rec.Code)
	}
	rec = doJSON(router, http.MethodGet, "/api/v1/push/key", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("key sem sessão esperava 401, veio %d", rec.Code)
	}
}

// CA-14 — GET /push/key devolve a VAPID public key (e só ela).
func TestPushKeyDevolvePublica(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newPushRouter(t, pool, "minha-public-key")
	cookie := loginVia(t, svc, stub, "g-key1", "key1@tt.dev", "Key")

	rec := doJSON(router, http.MethodGet, "/api/v1/push/key", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("key esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta fora do contrato: %s", rec.Body.String())
	}
	if body.Key != "minha-public-key" {
		t.Errorf("key deveria devolver a pública, veio %q", body.Key)
	}
}

// CA-14/RNF-04 — push desligado (sem VAPID): /push/key responde vazio.
func TestPushKeyVazioQuandoDesligado(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newPushRouter(t, pool, "") // sem VAPID
	cookie := loginVia(t, svc, stub, "g-key2", "key2@tt.dev", "Key")

	rec := doJSON(router, http.MethodGet, "/api/v1/push/key", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("key esperava 200 mesmo desligado, veio %d", rec.Code)
	}
	var body struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Key != "" {
		t.Errorf("com push desligado a key deveria ser vazia, veio %q", body.Key)
	}
}
