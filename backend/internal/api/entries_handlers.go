// entries_handlers.go — REST de entradas manuais (T-12), aninhado em
// /workspaces/{workspaceId}/entries ATRÁS da cadeia da fase 02
// (RequireAuth → RequireMember → RequireWriteRole): membership e role já
// chegaram validadas no contexto quando um handler daqui roda.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/timer"
)

// WithEntries liga o CRUD de entries da fase 03 no subrouter do workspace.
func WithEntries(svc *entries.Service) Option {
	return func(c *routerCfg) {
		c.entries = svc
	}
}

func mountEntries(wr chi.Router, svc *entries.Service, tsvc *timer.Service) {
	wr.Route("/entries", func(r chi.Router) {
		r.Get("/", handleListEntries(svc))
		r.Post("/", handleCreateEntry(svc))
		r.Patch("/{entryId}", handlePatchEntry(svc))
		r.Delete("/{entryId}", handleDeleteEntry(svc))
		r.Post("/{entryId}/continue", handleContinueEntry(svc, tsvc))
	})
}

// handleContinueEntry — POST /entries/{id}/continue (CA-17): clona
// descrição/projeto/task/tags da entry PRÓPRIA e inicia count up
// source='timer' pelo timer.Service (a troca do CA-05 vem de graça).
func handleContinueEntry(svc *entries.Service, tsvc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		userID := reqctx.UserID(r.Context())
		e, err := svc.Get(r.Context(), m.WorkspaceID, userID, chi.URLParam(r, "entryId"))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		res, err := tsvc.Start(r.Context(), userID, timer.StartInput{
			WorkspaceID: e.WorkspaceID,
			Description: e.Description,
			ProjectID:   e.ProjectID,
			TaskID:      e.TaskID,
			Tags:        e.Tags,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusCreated, res)
	}
}

// handleListEntries — GET /entries?from&to&tz (CA-14.1): recorte "hoje" no
// fuso IANA de tz; resposta {"entries": [...], "total_seconds": n}.
func handleListEntries(svc *entries.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		q := r.URL.Query()
		list, total, err := svc.ListDay(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
			entries.ListInput{From: q.Get("from"), To: q.Get("to"), TZ: q.Get("tz")})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{
			"entries":       list,
			"total_seconds": total,
		})
	}
}

// handleCreateEntry — POST /entries (CA-14/24/29): entrada manual com
// duração calculada no servidor.
func handleCreateEntry(svc *entries.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		var in entries.CreateInput
		if !decodeJSON(w, r, &in) {
			return
		}
		e, err := svc.Create(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()), in)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusCreated, e)
	}
}

// patchEntryBody usa json.RawMessage pra distinguir "ausente" de "null" em
// project_id e task_id (null remove o vínculo) — padrão do PATCH da fase 02.
type patchEntryBody struct {
	Description *string         `json:"description"`
	Tags        *[]string       `json:"tags"`
	ProjectID   json.RawMessage `json:"project_id"`
	TaskID      json.RawMessage `json:"task_id"`
	StartedAt   *time.Time      `json:"started_at"`
	StoppedAt   *time.Time      `json:"stopped_at"`
}

func rawToRef(w http.ResponseWriter, raw json.RawMessage, field string) (set bool, val *string, ok bool) {
	if raw == nil {
		return false, nil, true
	}
	if string(raw) == "null" {
		return true, nil, true
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation_error", field+" inválido")
		return false, nil, false
	}
	return true, &v, true
}

// handlePatchEntry — PATCH /entries/{id} (CA-15/24.1/29.3): campos
// parciais; mudou horário → duração recalculada no servidor.
func handlePatchEntry(svc *entries.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		var body patchEntryBody
		if !decodeJSON(w, r, &body) {
			return
		}
		in := entries.PatchInput{
			Description: body.Description,
			Tags:        body.Tags,
			StartedAt:   body.StartedAt,
			StoppedAt:   body.StoppedAt,
		}
		var ok bool
		if in.ProjectIDSet, in.ProjectID, ok = rawToRef(w, body.ProjectID, "project_id"); !ok {
			return
		}
		if in.TaskIDSet, in.TaskID, ok = rawToRef(w, body.TaskID, "task_id"); !ok {
			return
		}
		e, err := svc.Patch(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
			chi.URLParam(r, "entryId"), in)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, e)
	}
}

// handleDeleteEntry — DELETE /entries/{id} (CA-16/24.1/24.2/29.3).
func handleDeleteEntry(svc *entries.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		if err := svc.Remove(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
			chi.URLParam(r, "entryId")); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
