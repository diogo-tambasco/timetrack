// sender_test.go — TDD do envio (fase 16 T-05, CA-15/16/RNF-04). Teste
// INTERNO (package push) pra injetar o sendFunc fake (sem crypto VAPID real):
// o foco é a lógica best-effort + limpeza de 404/410 + no-op sem VAPID.
package push

import (
	"context"
	"sync"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// fakeSend grava os envios e devolve um status por endpoint.
type fakeSend struct {
	mu       sync.Mutex
	sent     []string       // endpoints chamados, em ordem
	statusBy map[string]int // status a devolver por endpoint
	errBy    map[string]error
}

func (f *fakeSend) fn(_ context.Context, _ []byte, sub Subscription) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sub.Endpoint)
	if e, ok := f.errBy[sub.Endpoint]; ok && e != nil {
		return 0, e
	}
	st, ok := f.statusBy[sub.Endpoint]
	if !ok {
		st = 201
	}
	return st, nil
}

func newPushUser(t *testing.T) (*Subscriptions, string) {
	t.Helper()
	pool := dbtest.NewPool(t)
	var uid string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ('g-snd', 'snd@b.c', 'U') RETURNING id`).
		Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return NewSubscriptions(pool), uid
}

// CA-15/16 — sucesso (201) não remove; 410 remove a sub; 1 sub falhando não
// impede a outra (best-effort).
func TestSendBestEffortELimpa410(t *testing.T) {
	subs, uid := newPushUser(t)
	ctx := context.Background()

	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e-ok", P256dh: "k", Auth: "a"})
	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e-gone", P256dh: "k", Auth: "a"})
	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e-ok2", P256dh: "k", Auth: "a"})

	fake := &fakeSend{statusBy: map[string]int{"e-ok": 201, "e-gone": 410, "e-ok2": 201}}
	sender := NewSender(subs, "pub", "priv", "mailto:x@y.z", nil)
	sender.send = fake.fn

	sender.SendToUser(ctx, uid, []byte(`{"title":"x"}`))

	// todas as 3 foram tentadas (best-effort: a 410 no meio não para as outras)
	if len(fake.sent) != 3 {
		t.Fatalf("as 3 subs deveriam ser tentadas, veio %d (%v)", len(fake.sent), fake.sent)
	}
	// só a 410 foi removida
	got, err := subs.ListByUser(ctx, uid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("só a sub 410 deveria ser removida, sobraram %d", len(got))
	}
	for _, s := range got {
		if s.Endpoint == "e-gone" {
			t.Errorf("a sub 410 (Gone) deveria ter sido removida (CA-16)")
		}
	}
}

// CA-16 — 404 também remove (Gone); erro de transporte NÃO remove (transitório).
func TestSend404RemoveErroTransporteMantem(t *testing.T) {
	subs, uid := newPushUser(t)
	ctx := context.Background()

	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e-404", P256dh: "k", Auth: "a"})
	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e-erro", P256dh: "k", Auth: "a"})

	fake := &fakeSend{
		statusBy: map[string]int{"e-404": 404},
		errBy:    map[string]error{"e-erro": context.DeadlineExceeded},
	}
	sender := NewSender(subs, "pub", "priv", "mailto:x@y.z", nil)
	sender.send = fake.fn

	sender.SendToUser(ctx, uid, []byte(`{}`))

	got, _ := subs.ListByUser(ctx, uid)
	// a 404 sai; a de erro de transporte FICA (transitório — pode voltar)
	if len(got) != 1 {
		t.Fatalf("só a 404 deveria sair (erro transitório mantém), sobraram %d", len(got))
	}
	if got[0].Endpoint != "e-erro" {
		t.Errorf("a sub com erro de transporte deveria ficar, veio %q", got[0].Endpoint)
	}
}

// CA-16/RNF-04 — sem VAPID public key → no-op: nenhuma sub é tentada nem
// removida (push desligado, mas o resto do app funciona).
func TestSendSemVapidNoOp(t *testing.T) {
	subs, uid := newPushUser(t)
	ctx := context.Background()
	_ = subs.Upsert(ctx, uid, Subscription{Endpoint: "e", P256dh: "k", Auth: "a"})

	fake := &fakeSend{}
	sender := NewSender(subs, "", "", "", nil) // SEM vapidPub → desligado
	sender.send = fake.fn

	if sender.Enabled() {
		t.Errorf("Enabled deveria ser false sem VAPID public key")
	}
	sender.SendToUser(ctx, uid, []byte(`{}`))

	if len(fake.sent) != 0 {
		t.Errorf("sem VAPID nenhum envio deveria ser tentado, veio %d", len(fake.sent))
	}
	if got, _ := subs.ListByUser(ctx, uid); len(got) != 1 {
		t.Errorf("nenhuma sub deveria ser removida com push desligado, sobraram %d", len(got))
	}
}
