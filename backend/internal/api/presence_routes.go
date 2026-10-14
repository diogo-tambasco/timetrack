// presence_routes.go — wiring da fase 21: GET /workspaces/{id}/presence
// (snapshot inicial, CA-11/12/13/14). Aninhado no subrouter do workspace, atrás
// da cadeia RequireAuth → RequireMember (leitura passa pra viewer — RequireWriteRole
// só barra escrita). Não-membro/ws inexistente → 404 uniforme (RequireMember);
// sem sessão → 401 (RequireAuth). É só leitura — sem 403/validação de payload.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/presence"
	"timetrack.live/backend/internal/reqctx"
)

// WithPresence liga o snapshot de presence no router (fase 21).
func WithPresence(svc *presence.Service) Option {
	return func(c *routerCfg) {
		c.presence = svc
	}
}

func mountPresence(wr chi.Router, svc *presence.Service) {
	wr.Get("/presence", handlePresenceSnapshot(svc))
}

// handlePresenceSnapshot — lista os membros do ws com timer rodando agora
// (PresencePayload por item), sob a regra de visibilidade do projeto pro caller.
// viewerID = reqctx.UserID (o próprio). Lista vazia = {"members":[]}.
func handlePresenceSnapshot(svc *presence.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		members, err := svc.Snapshot(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if members == nil {
			members = []presence.PresencePayload{}
		}
		respond.JSON(w, http.StatusOK, map[string]any{"members": members})
	}
}
