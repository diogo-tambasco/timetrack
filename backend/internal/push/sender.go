package push

import (
	"context"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Payload é o corpo da notificação push (vira o JSON que o service worker
// recebe no evento `push` e mostra via showNotification).
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Kind  string `json:"kind"`
}

// sendFunc é o envio de UMA notificação. Em produção é o webpush.Send; nos
// testes é injetado pra simular respostas do push service sem crypto real.
// Devolve o status HTTP (404/410 → limpar a sub — CA-16) e erro de transporte.
type sendFunc func(ctx context.Context, msg []byte, sub Subscription) (status int, err error)

// Sender envia o Web Push do corte (CA-15/16). best-effort: uma sub que falha
// não impede as outras nem o corte (que já commitou — push é pós-commit).
// vapidPub vazio → no-op (push desligado — CA-16/RNF-04).
type Sender struct {
	subs      *Subscriptions
	vapidPub  string
	vapidPriv string
	subject   string
	log       *slog.Logger
	send      sendFunc // injetável nos testes
}

// NewSender cria o sender. vapidPub vazio → push desligado (SendToUser é no-op).
func NewSender(subs *Subscriptions, vapidPub, vapidPriv, subject string, log *slog.Logger) *Sender {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	s := &Sender{
		subs:      subs,
		vapidPub:  vapidPub,
		vapidPriv: vapidPriv,
		subject:   subject,
		log:       log,
	}
	s.send = s.webpushSend // produção: webpush-go (VAPID real)
	return s
}

// Enabled reporta se o push está ligado (há VAPID public key). O endpoint
// /push/key e o boot consultam isto.
func (s *Sender) Enabled() bool { return s.vapidPub != "" }

// PublicKey expõe a VAPID public key (vai pro cliente — CA-14). A private
// NUNCA sai daqui (RNF-04).
func (s *Sender) PublicKey() string { return s.vapidPub }

// webpushSend é o envio real via webpush-go: assina com VAPID e POSTa no
// endpoint da subscription. httpClient injetável fica no campo (testes usam o
// send fake; produção usa o default *http.Client da lib).
func (s *Sender) webpushSend(ctx context.Context, msg []byte, sub Subscription) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, msg, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber:      s.subject,
		VAPIDPublicKey:  s.vapidPub,
		VAPIDPrivateKey: s.vapidPriv,
		TTL:             60,
		HTTPClient:      http.DefaultClient,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// SendToUser envia o payload a TODAS as subs do usuário (CA-15). best-effort:
// uma sub que falha não impede as outras. 404/410 (Gone) → Delete a sub
// (CA-16); outros erros são logados e seguem. Sem VAPID → no-op (CA-16/RNF-04).
func (s *Sender) SendToUser(ctx context.Context, userID string, payload []byte) {
	if !s.Enabled() {
		return // push desligado — degrada silenciosamente (RNF-04)
	}
	subs, err := s.subs.ListByUser(ctx, userID)
	if err != nil {
		s.log.Error("push: listar subscriptions falhou", "user", userID, "err", err)
		return
	}
	for _, sub := range subs {
		status, err := s.send(ctx, payload, sub)
		if err != nil {
			// erro de transporte (transitório) — logar e seguir, NÃO remover
			s.log.Warn("push: envio falhou (mantém a sub)", "endpoint", sub.Endpoint, "err", err)
			continue
		}
		if status == http.StatusNotFound || status == http.StatusGone {
			// 404/410 → o navegador desinscreveu/expirou: remover (CA-16)
			if err := s.subs.Delete(ctx, sub.Endpoint); err != nil {
				s.log.Error("push: remover sub morta falhou", "endpoint", sub.Endpoint, "err", err)
			}
		}
	}
}
