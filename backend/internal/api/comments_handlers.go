// comments_handlers.go — REST da thread de comentários (fase 19, plan §4).
// Tradução fina: visibilidade, carve-out viewer-assignee e autor/owner moram
// no comments.Service. As rotas são montadas num subrouter IRMÃO do workspace
// SEM RequireWriteRole (plan §3 D-02): o gate de escrita é condicional por
// task (member ⇒ visibilidade; viewer ⇒ ser assignee), não a regra binária
// "viewer nunca escreve".
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/reqctx"
)

// Comments agrega o service da thread de comentários (fase 19).
type Comments struct{ Svc *comments.Service }

// WithComments liga as rotas de comentário no router.
func WithComments(c Comments) Option {
	return func(cfg *routerCfg) {
		cfg.comments = &c
	}
}

func (c *Comments) handleList(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Svc.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"comments": list})
}

func (c *Comments) handleCreate(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	cm, err := c.Svc.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), body.Body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, cm)
}

func (c *Comments) handleDelete(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Svc.Delete(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), chi.URLParam(r, "commentId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
