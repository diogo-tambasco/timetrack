// settings_calendar.go — fase 11 T-03: GET|PUT /api/v1/settings/calendar.
// Rota POR USUÁRIO (user da sessão, sem workspace na URL). Handlers finos:
// validação e upsert moram no calendar.Service (espelho do pomodoro).
package api

import (
	"net/http"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/reqctx"
)

func handleGetCalendarSettings(svc *calendar.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.GetSettings(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, st)
	}
}

func handlePutCalendarSettings(svc *calendar.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body calendar.Settings
		if !decodeJSON(w, r, &body) {
			return
		}
		saved, err := svc.SaveSettings(r.Context(), reqctx.UserID(r.Context()), body)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, saved)
	}
}
