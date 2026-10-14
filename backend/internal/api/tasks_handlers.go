// tasks_handlers.go — REST de tasks (plan §3.3, CA-15.2/16..22). A
// permissão de escrita (D-05) e a visibilidade herdada do projeto moram
// no service; aqui só tradução HTTP.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/tasks"
)

func (c *Core) handleListTasks(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var projectID, status, objectiveID *string
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID = &v
	}
	if v := r.URL.Query().Get("status"); v != "" {
		status = &v
	}
	if v := r.URL.Query().Get("objective_id"); v != "" {
		objectiveID = &v
	}
	list, err := c.Tasks.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), projectID, status, objectiveID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"tasks": list})
}

func (c *Core) handleGetTask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	t, err := c.Tasks.Get(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (c *Core) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		ProjectID   string  `json:"project_id"`
		Name        string  `json:"name"`
		ObjectiveID *string `json:"objective_id"`
		Status      *string `json:"status"`     // fase 24
		Priority    *int    `json:"priority"`   // fase 24
		NotionURL   *string `json:"notion_url"` // fase 24
		DueDate     *string `json:"due_date"`
		Recurrence  *string `json:"recurrence"`
		Notes       string  `json:"notes"`
		Assignee    *string `json:"assignee"`
		RequestedBy *string `json:"requested_by"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	t, notify, err := c.Tasks.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role, tasks.CreateInput{
		ProjectID:   body.ProjectID,
		Name:        body.Name,
		ObjectiveID: body.ObjectiveID,
		Status:      body.Status,
		Priority:    body.Priority,
		NotionURL:   body.NotionURL,
		DueDate:     body.DueDate,
		Recurrence:  body.Recurrence,
		Notes:       body.Notes,
		Assignee:    body.Assignee,
		RequestedBy: body.RequestedBy,
		TZ:          r.URL.Query().Get("tz"), // fase 20: fuso do reminder due (09:00 local)
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	c.fireOnAssign(r, notify)
	respond.JSON(w, http.StatusCreated, t)
}

// patchTaskBody usa json.RawMessage pra distinguir "ausente" de "null" em
// due_date e recurrence (null limpa).
type patchTaskBody struct {
	Name        *string         `json:"name"`
	Status      *string         `json:"status"`
	Priority    *int            `json:"priority"`   // fase 24: presente-ou-ausente
	NotionURL   json.RawMessage `json:"notion_url"` // fase 24: tri-state (null limpa)
	Position    *int            `json:"position"`   // fase 24: presente-ou-ausente
	DueDate     json.RawMessage `json:"due_date"`
	Recurrence  json.RawMessage `json:"recurrence"`
	Notes       *string         `json:"notes"`
	ObjectiveID json.RawMessage `json:"objective_id"`
	Assignee    json.RawMessage `json:"assignee"`
	RequestedBy json.RawMessage `json:"requested_by"`
}

func (c *Core) handlePatchTask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body patchTaskBody
	if !decodeJSON(w, r, &body) {
		return
	}
	in := tasks.PatchInput{Name: body.Name, Status: body.Status, Notes: body.Notes,
		TZ: r.URL.Query().Get("tz")} // fase 20: fuso do reminder due (09:00 local)
	// fase 24: priority/position são presente-ou-ausente (não há "null"); notion_url
	// é tri-state (null limpa — D-09).
	if body.Priority != nil {
		in.PrioritySet = true
		in.Priority = body.Priority
	}
	if body.Position != nil {
		in.PositionSet = true
		in.Position = body.Position
	}
	if !decodeTriState(w, body.NotionURL, "notion_url inválido", &in.NotionURLSet, &in.NotionURL) {
		return
	}
	if body.DueDate != nil {
		in.DueDateSet = true
		if string(body.DueDate) != "null" {
			var v string
			if err := json.Unmarshal(body.DueDate, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "due_date inválida")
				return
			}
			in.DueDate = &v
		}
	}
	if body.Recurrence != nil {
		in.RecurrenceSet = true
		if string(body.Recurrence) != "null" {
			var v string
			if err := json.Unmarshal(body.Recurrence, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "recurrence inválida")
				return
			}
			in.Recurrence = &v
		}
	}
	if body.ObjectiveID != nil {
		in.ObjectiveIDSet = true
		if string(body.ObjectiveID) != "null" {
			var v string
			if err := json.Unmarshal(body.ObjectiveID, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "objective_id inválido")
				return
			}
			in.ObjectiveID = &v
		}
	}
	// fase 18: tri-state de assignee/requested_by (mesma mecânica — null limpa).
	if !decodeTriState(w, body.Assignee, "assignee inválido", &in.AssigneeSet, &in.Assignee) {
		return
	}
	if !decodeTriState(w, body.RequestedBy, "requested_by inválido", &in.RequestedBySet, &in.RequestedBy) {
		return
	}

	t, notify, err := c.Tasks.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	c.fireOnAssign(r, notify)
	respond.JSON(w, http.StatusOK, t)
}

// fireOnAssign dispara o fan-out da notificação de atribuição pós-commit
// (CA-13): só quando o service sinalizou (notify != nil) e há hook injetado.
// O byUserID é o user da sessão (quem atribuiu).
func (c *Core) fireOnAssign(r *http.Request, notify *tasks.AssignNotify) {
	if notify == nil || c.OnAssign == nil {
		return
	}
	c.OnAssign(notify.TargetUserID, notify.TaskID, reqctx.UserID(r.Context()))
}

// decodeTriState aplica a mecânica tri-state de um campo string opcional do
// patchTaskBody (json.RawMessage): nil=ausente (set=false), "null"=limpa
// (set=true, ptr=nil), valor=define. Devolve false (e responde 400) quando o
// JSON é inválido. Reduz a repetição dos campos novos da fase 18.
func decodeTriState(w http.ResponseWriter, raw json.RawMessage, msg string, set *bool, dst **string) bool {
	if raw == nil {
		return true
	}
	*set = true
	if string(raw) == "null" {
		return true
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation_error", msg)
		return false
	}
	*dst = &v
	return true
}

func (c *Core) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Tasks.Delete(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "taskId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
