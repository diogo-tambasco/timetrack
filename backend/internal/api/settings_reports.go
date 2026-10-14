// settings_reports.go — fase 14 T-07: GET|PUT /api/v1/settings/reports. Rota
// POR USUÁRIO (user da sessão, sem workspace na URL). Handlers finos: validação
// e upsert moram no reports.Service (espelho do settings/calendar da 11).
package api

import (
	"net/http"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/reqctx"
)

func handleGetReportSettings(svc *reports.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.GetSettings(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, st)
	}
}

func handlePutReportSettings(svc *reports.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body reports.Settings
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
