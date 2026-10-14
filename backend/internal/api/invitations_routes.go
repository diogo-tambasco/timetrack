// invitations_routes.go — wiring da fase 17: gestão de convites + membros no
// subrouter /api/v1/workspaces/{workspaceId}, atrás de um grupo Owner-only
// (RequireOwner). Handlers finos: decode → service → writeDomainError/respond.
// Espelha push_routes.go/core_routes.go.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/invitations"
	"timetrack.live/backend/internal/reqctx"
)

// Invitations agrega o service de convites/membros (fase 17).
type Invitations struct {
	Service *invitations.Service
}

// WithInvitations liga as rotas de gestão da fase 17 no router.
func WithInvitations(inv Invitations) Option {
	return func(c *routerCfg) { c.invitations = &inv }
}

// mountInvitations pendura as rotas de gestão num grupo Owner-only no subrouter
// do workspace (depois do mountCore). RequireMember (404) já rodou no subrouter;
// RequireOwner (403) garante a ordem 404-antes-de-403 (CA-21.1).
func mountInvitations(wr chi.Router, inv *Invitations) {
	svc := inv.Service
	wr.Group(func(o chi.Router) {
		o.Use(RequireOwner)
		o.Post("/invitations", handleCreateInvitation(svc))
		o.Get("/invitations", handleListInvitations(svc))
		o.Delete("/invitations/{inviteId}", handleRevokeInvitation(svc))
		o.Post("/members", handleAddMember(svc))
		o.Delete("/members/{userId}", handleRemoveMember(svc))
		o.Patch("/members/{userId}", handleChangeRole(svc))
	})
}

type inviteBody struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func handleCreateInvitation(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body inviteBody
		if !decodeJSON(w, r, &body) {
			return
		}
		wsID := chi.URLParam(r, "workspaceId")
		inv, err := svc.Create(r.Context(), wsID, reqctx.UserID(r.Context()), body.Email, body.Role)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusCreated, inv)
	}
}

func handleListInvitations(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wsID := chi.URLParam(r, "workspaceId")
		list, err := svc.List(r.Context(), wsID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"invitations": list})
	}
}

func handleRevokeInvitation(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wsID := chi.URLParam(r, "workspaceId")
		inviteID := chi.URLParam(r, "inviteId")
		if err := svc.Revoke(r.Context(), wsID, inviteID); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAddMember(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body inviteBody
		if !decodeJSON(w, r, &body) {
			return
		}
		wsID := chi.URLParam(r, "workspaceId")
		m, err := svc.AddExistingMember(r.Context(), wsID, body.Email, body.Role)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusCreated, m)
	}
}

func handleRemoveMember(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wsID := chi.URLParam(r, "workspaceId")
		userID := chi.URLParam(r, "userId")
		if err := svc.RemoveMember(r.Context(), wsID, userID); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type roleBody struct {
	Role string `json:"role"`
}

func handleChangeRole(svc *invitations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body roleBody
		if !decodeJSON(w, r, &body) {
			return
		}
		wsID := chi.URLParam(r, "workspaceId")
		userID := chi.URLParam(r, "userId")
		if err := svc.ChangeRole(r.Context(), wsID, userID, body.Role); err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]string{"user_id": userID, "role": body.Role})
	}
}
