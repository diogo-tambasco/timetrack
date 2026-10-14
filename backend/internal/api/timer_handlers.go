// timer_handlers.go — handlers REST do timer (fase 03). Rotas POR USUÁRIO:
// o user vem da sessão; toda resposta é o Result canônico do timer.Service
// (mesmo corpo do GET /timer — RNF-05).
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/timer"
	"timetrack.live/backend/internal/workspace"
)

// handleTimerStart — POST /api/v1/timer/start (CA-04/05/12/13/20/27/29).
// Rota por usuário: a membership/role do workspace_id vem do PAYLOAD e é
// validada aqui (não-membro → 404 sem expor dados, CA-29.1; viewer → 403,
// CA-29.2) — equivalente manual da cadeia RequireMember→RequireWriteRole.
func handleTimerStart(svc *timer.Service, ws *workspace.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in timer.StartInput
		if !decodeJSON(w, r, &in) {
			return
		}
		userID := reqctx.UserID(r.Context())
		role, err := ws.MemberRole(r.Context(), in.WorkspaceID, userID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if role == "" {
			respond.Error(w, http.StatusNotFound, "not_found", "workspace não encontrado")
			return
		}
		if role == "viewer" {
			respond.Error(w, http.StatusForbidden, "forbidden",
				"viewer não tem permissão de escrita")
			return
		}
		res, err := svc.Start(r.Context(), userID, in)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusCreated, res)
	}
}

// handleTimerEdit — PATCH /api/v1/timer (fase 27, allowance HITL 23/06).
// Rota POR USUÁRIO (user da sessão): edita SÓ description/project_id/tags da
// entry RODANDO. Body {description?, project_id? (tri-state), tags?} — SEM
// workspace_id (o ws é o da entry rodando, resolvido no service). Timing
// permanece intocado (CA-13.1); 404/409 canônicos via writeDomainError.
func handleTimerEdit(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// project_id é tri-state: ausente (não mexe) × null (remove) × valor.
		// json.RawMessage distingue "campo ausente" de "null".
		var body struct {
			Description *string          `json:"description"`
			Tags        *[]string        `json:"tags"`
			ProjectID   *json.RawMessage `json:"project_id"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		in := timer.EditCurrentInput{
			Description: body.Description,
			Tags:        body.Tags,
		}
		if body.ProjectID != nil {
			in.ProjectIDSet = true
			var pid *string
			if err := json.Unmarshal(*body.ProjectID, &pid); err != nil {
				respond.Error(w, http.StatusBadRequest, "validation_error", "project_id inválido")
				return
			}
			in.ProjectID = pid
		}
		res, err := svc.EditCurrent(r.Context(), reqctx.UserID(r.Context()), in)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}

// handleTimerStop — POST /api/v1/timer/stop (CA-06/06.1). Sem body.
func handleTimerStop(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.Stop(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}

// handleTimerState — GET /api/v1/timer (CA-21/22/22.1/23): o estado
// canônico, única fonte do front; mesmo shape de toda mutação.
func handleTimerState(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.State(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}

// handleTimerResume — POST /api/v1/timer/resume (CA-09/10/26). Sem body.
func handleTimerResume(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.Resume(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}

// handleTimerSkipBreak — POST /api/v1/timer/break/skip (CA-11/26). Sem body.
func handleTimerSkipBreak(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.SkipBreak(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}

// handleTimerPause — POST /api/v1/timer/pause (CA-07/08/25/25.1). Body
// OPCIONAL {"break_minutes": n}: presente vale só nesta pausa.
func handleTimerPause(svc *timer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			BreakMinutes *int `json:"break_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			respond.Error(w, http.StatusBadRequest, "validation_error", "payload inválido")
			return
		}
		res, err := svc.Pause(r.Context(), reqctx.UserID(r.Context()), body.BreakMinutes)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, res)
	}
}
