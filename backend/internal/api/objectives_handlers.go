// objectives_handlers.go — fase 12 T-06: REST de objetivos + board (handlers
// finos; a regra de permissão/visibilidade/agregação mora no objectives.Service).
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/reqctx"
)

func (c *Core) handleListObjectives(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Objectives.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "projectId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"objectives": list})
}

func (c *Core) handleCreateObjective(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		GoalHours      *float64 `json:"goal_hours"`
		CompletionMode string   `json:"completion_mode"` // fase 29: "" → by_tasks (default no service)
		TargetDate     *string  `json:"target_date"`     // fase 29: nil → sem marco
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	o, err := c.Objectives.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), objectives.CreateInput{
			Name:           body.Name,
			Description:    body.Description,
			GoalHours:      body.GoalHours,
			CompletionMode: body.CompletionMode,
			TargetDate:     body.TargetDate,
		})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, o)
}

// patchObjectiveBody usa json.RawMessage no goal_hours/target_date pra distinguir
// ausente (mantém) de null (remove — CA-04/03.1). completion_mode é ponteiro
// simples (ausente mantém; valor troca o modo — fase 29).
type patchObjectiveBody struct {
	Name           *string         `json:"name"`
	Description    *string         `json:"description"`
	Status         *string         `json:"status"`
	GoalHours      json.RawMessage `json:"goal_hours"`
	Position       *int            `json:"position"`
	CompletionMode *string         `json:"completion_mode"` // fase 29
	TargetDate     json.RawMessage `json:"target_date"`     // fase 29: tri-state
}

func (c *Core) handlePatchObjective(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body patchObjectiveBody
	if !decodeJSON(w, r, &body) {
		return
	}
	in := objectives.PatchInput{Name: body.Name, Description: body.Description, Status: body.Status}
	if body.GoalHours != nil {
		in.GoalSet = true
		if string(body.GoalHours) != "null" {
			var v float64
			if err := json.Unmarshal(body.GoalHours, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "goal_hours inválido")
				return
			}
			in.GoalHours = &v
		}
	}
	if body.Position != nil {
		in.PositionSet = true
		in.Position = body.Position
	}
	// fase 29: completion_mode ponteiro (ausente mantém); target_date tri-state
	// via json.RawMessage (ausente mantém, null limpa, valor define — CA-03.1).
	if body.CompletionMode != nil {
		in.CompletionMode = body.CompletionMode
	}
	if body.TargetDate != nil {
		in.TargetDateSet = true
		if string(body.TargetDate) != "null" {
			var v string
			if err := json.Unmarshal(body.TargetDate, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "target_date inválido")
				return
			}
			in.TargetDate = &v
		}
	}
	o, err := c.Objectives.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "objectiveId"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, o)
}

func (c *Core) handleDeleteObjective(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Objectives.Delete(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), chi.URLParam(r, "objectiveId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Core) handleBoard(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	b, err := c.Objectives.Board(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, b)
}
