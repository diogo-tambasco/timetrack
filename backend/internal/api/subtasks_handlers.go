// subtasks_handlers.go — fase 24 T-06: REST de sub-tasks ("goals da task").
// Handlers finos: o gate (visibilidade→404, escrita→403, arquivado→400) e a
// validação moram no subtasks.Service. Montado no Core (decisão F — T-07).
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/subtasks"
)

func (c *Core) handleListSubtasks(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Subtasks.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"subtasks": list})
}

func (c *Core) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	st, err := c.Subtasks.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), subtasks.CreateInput{Title: body.Title})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, st)
}

func (c *Core) handlePatchSubtask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Title    *string `json:"title"`
		Done     *bool   `json:"done"`
		IsGoal   *bool   `json:"is_goal"`
		Position *int    `json:"position"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	st, err := c.Subtasks.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), chi.URLParam(r, "subtaskId"), subtasks.PatchInput{
			Title:    body.Title,
			Done:     body.Done,
			IsGoal:   body.IsGoal,
			Position: body.Position,
		})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, st)
}

func (c *Core) handleDeleteSubtask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Subtasks.Delete(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), chi.URLParam(r, "subtaskId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
