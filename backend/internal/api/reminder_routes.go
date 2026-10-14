// reminder_routes.go — REST dos lembretes (fase 20, plan §4). Mesma estrutura
// da fase 19: o POST/DELETE por-task ficam num subrouter IRMÃO do workspace SEM
// RequireWriteRole (o carve-out viewer-assignee é por-task no reminders.Service);
// o GET /api/v1/reminders é POR USUÁRIO (cross-ws — espelha /api/v1/timer).
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reminders"
	"timetrack.live/backend/internal/reqctx"
)

// Reminders agrega o service dos lembretes (fase 20).
type Reminders struct{ Svc *reminders.Service }

// WithReminders liga as rotas de lembrete no router.
func WithReminders(rm Reminders) Option {
	return func(cfg *routerCfg) {
		cfg.reminders = &rm
	}
}

func (rm *Reminders) handleCreate(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		RemindAt string `json:"remind_at"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.RemindAt == "" {
		respond.Error(w, http.StatusBadRequest, "validation_error", "remind_at é obrigatório")
		return
	}
	at, err := time.Parse(time.RFC3339, body.RemindAt)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "validation_error", "remind_at deve ser RFC3339")
		return
	}
	rem, err := rm.Svc.CreateManual(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), at)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, rem)
}

func (rm *Reminders) handleDelete(w http.ResponseWriter, r *http.Request) {
	found, err := rm.Svc.Cancel(r.Context(), reqctx.UserID(r.Context()),
		chi.URLParam(r, "taskId"), chi.URLParam(r, "reminderId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !found {
		WriteError(w, http.StatusNotFound, "not_found", "lembrete não encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListMine — GET /api/v1/reminders?status=pending (os meus, cross-ws).
func (rm *Reminders) handleListMine(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // vazio → pending (no Store)
	list, err := rm.Svc.List(r.Context(), reqctx.UserID(r.Context()), status)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"reminders": list})
}
