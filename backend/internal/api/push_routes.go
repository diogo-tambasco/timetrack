// push_routes.go — fase 16 T-06: POST /api/v1/push/subscribe + GET
// /api/v1/push/key. Rotas POR USUÁRIO (user da sessão, sem workspace na URL —
// espelho do settings/pomodoro). Atrás do RequireAuth (401 canônico sem
// sessão). Handlers finos: persistência mora no push.Subscriptions.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/push"
	"timetrack.live/backend/internal/reqctx"
)

// Push agrega o que as rotas de push precisam (fase 16). Construir o sender
// com a VAPID public key (vazia → push desligado, /push/key vazio).
type Push struct {
	Subs   *push.Subscriptions
	Sender *push.Sender
}

// WithPush liga as rotas de push da fase 16 no router.
func WithPush(p Push) Option {
	return func(c *routerCfg) { c.push = &p }
}

func mountPush(r chi.Router, p *Push) {
	r.Post("/push/subscribe", handlePushSubscribe(p.Subs))
	r.Get("/push/key", handlePushKey(p.Sender))
}

// subscribeBody é o corpo do navegador (PushSubscription.toJSON()):
// {"endpoint": "...", "keys": {"p256dh": "...", "auth": "..."}}.
type subscribeBody struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func handlePushSubscribe(subs *push.Subscriptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body subscribeBody
		if !decodeJSON(w, r, &body) {
			return
		}
		// CA-13: corpo inválido (faltando endpoint/chaves) → 400 validation_error
		if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			respond.Error(w, http.StatusBadRequest, "validation_error",
				"endpoint, keys.p256dh e keys.auth são obrigatórios")
			return
		}
		err := subs.Upsert(r.Context(), reqctx.UserID(r.Context()), push.Subscription{
			Endpoint: body.Endpoint,
			P256dh:   body.Keys.P256dh,
			Auth:     body.Keys.Auth,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func handlePushKey(sender *push.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CA-14: a pública vai pro cliente; vazia se o push estiver desligado.
		// A private NUNCA é exposta (RNF-04).
		respond.JSON(w, http.StatusOK, map[string]string{"key": sender.PublicKey()})
	}
}
