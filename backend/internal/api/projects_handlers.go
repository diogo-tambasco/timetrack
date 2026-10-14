// projects_handlers.go — REST de projetos (plan §3.2, CA-06..15.1).
// Identidade (user/role/workspace) vem SEMPRE do contexto validado pela
// cadeia da T-02; a visibilidade é resolvida no service (D-04).
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/reqctx"
)

// projectWithMembers é o shape do 201 e do detalhe (members embutidos);
// a listagem usa o Project puro, sem members (plan §3.2).
type projectWithMembers struct {
	projects.Project
	Members []projects.Member `json:"members"`
}

func (c *Core) handleListProjects(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Projects.List(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		r.URL.Query().Get("archived") == "true")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"projects": list})
}

func (c *Core) handleGetProject(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	p, err := c.Projects.Get(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), chi.URLParam(r, "projectId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	members, err := c.Projects.Members(r.Context(), p.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, projectWithMembers{Project: p, Members: members})
}

// createProjectBody cobre o contrato do POST (plan §3.2).
type createProjectBody struct {
	Name            string            `json:"name"`
	Color           string            `json:"color"`
	Visibility      string            `json:"visibility"`
	ClientID        *string           `json:"client_id"`
	WeeklyHoursGoal *float64          `json:"weekly_hours_goal"`
	Members         []projects.Member `json:"members"`
	Deadline        *string           `json:"deadline"` // fase 28: nil = sem prazo (CA-01)
}

func (c *Core) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body createProjectBody
	if !decodeJSON(w, r, &body) {
		return
	}
	p, err := c.Projects.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), projects.CreateInput{
		Name:            body.Name,
		Color:           body.Color,
		Visibility:      body.Visibility,
		ClientID:        body.ClientID,
		WeeklyHoursGoal: body.WeeklyHoursGoal,
		Members:         body.Members,
		Deadline:        body.Deadline,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	members, err := c.Projects.Members(r.Context(), p.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, projectWithMembers{Project: p, Members: members})
}

// patchProjectBody usa json.RawMessage pra distinguir "ausente" de "null"
// em client_id e weekly_hours_goal (plan §3.2: null remove).
type patchProjectBody struct {
	Name            *string            `json:"name"`
	Color           *string            `json:"color"`
	Visibility      *string            `json:"visibility"`
	ClientID        json.RawMessage    `json:"client_id"`
	WeeklyHoursGoal json.RawMessage    `json:"weekly_hours_goal"`
	Members         *[]projects.Member `json:"members"`
	Deadline        json.RawMessage    `json:"deadline"` // fase 28 tri-state (null remove)
}

func (c *Core) handlePatchProject(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body patchProjectBody
	if !decodeJSON(w, r, &body) {
		return
	}
	in := projects.PatchInput{Name: body.Name, Color: body.Color, Visibility: body.Visibility}
	if body.Members != nil {
		in.MembersSet = true
		in.Members = *body.Members
	}
	if body.ClientID != nil {
		in.ClientIDSet = true
		if string(body.ClientID) != "null" {
			var v string
			if err := json.Unmarshal(body.ClientID, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "client_id inválido")
				return
			}
			in.ClientID = &v
		}
	}
	if body.WeeklyHoursGoal != nil {
		in.GoalSet = true
		if string(body.WeeklyHoursGoal) != "null" {
			var v float64
			if err := json.Unmarshal(body.WeeklyHoursGoal, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "weekly_hours_goal inválida")
				return
			}
			in.WeeklyHoursGoal = &v
		}
	}
	if body.Deadline != nil {
		in.DeadlineSet = true
		if string(body.Deadline) != "null" {
			var v string
			if err := json.Unmarshal(body.Deadline, &v); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "deadline inválido")
				return
			}
			in.Deadline = &v
		}
	}

	p, err := c.Projects.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, p)
}

func (c *Core) handleReplaceMembers(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	var body struct {
		Members []projects.Member `json:"members"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	persisted, err := c.Projects.ReplaceMembers(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		m.Role, chi.URLParam(r, "projectId"), body.Members)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"members": persisted})
}

func (c *Core) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	if err := c.Projects.Archive(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), m.Role,
		chi.URLParam(r, "projectId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleProjectsSummary — fase 28 (CA-04..08): cards vivos num único request.
func (c *Core) handleProjectsSummary(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	list, err := c.Projects.Summary(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		r.URL.Query().Get("tz"), r.URL.Query().Get("archived") == "true")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"projects": list})
}

// handleProjectCalendar — fase 28 (CA-09..15.1): itens datados do projeto por
// janela (?month ou ?from/?to). NÃO retorna eventos do GCal (CA-13).
func (c *Core) handleProjectCalendar(w http.ResponseWriter, r *http.Request) {
	m := reqctx.MembershipFrom(r.Context())
	q := r.URL.Query()
	from, to, err := c.Projects.ProjectCalendarWindow(q.Get("month"), q.Get("from"), q.Get("to"), q.Get("tz"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items, err := c.Projects.ProjectCalendar(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
		chi.URLParam(r, "projectId"), from, to)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "from": from, "to": to})
}
