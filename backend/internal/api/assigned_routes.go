// assigned_routes.go — fase 18 T-07: GET /api/v1/tasks/assigned + /tasks/requested.
// Rotas POR USUÁRIO (sem workspace na URL — cruzam todos os workspaces que o user
// participa), montadas no /api/v1 atrás do RequireAuth (junto do mountPush/
// mountTimer, FORA do subrouter /workspaces/{id}). Handlers finos: a query mora
// no tasks.Service (assigned.go).
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/tasks"
)

func mountAssigned(r chi.Router, t *tasks.Service) {
	r.Get("/tasks/assigned", handleAssigned(t))
	r.Get("/tasks/requested", handleRequested(t))
}

func handleAssigned(t *tasks.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := t.Assigned(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"tasks": list})
	}
}

func handleRequested(t *tasks.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := t.Requested(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"tasks": list})
	}
}
