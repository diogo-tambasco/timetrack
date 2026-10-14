// reports_routes.go — wiring da fase 07/14: GET /reports/week (07, INTOCADO) e
// GET /reports (14, genérico) no subrouter do workspace, atrás da cadeia
// RequireAuth → RequireMember → RequireWriteRole (leitura passa pra viewer —
// CA-03/02). /reports/week segue chamando svc.Week (não-regressão, RNF-05).
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/reqctx"
)

// WithReports liga os relatórios (semanal da 07 + genérico da 14) no router.
func WithReports(svc *reports.Service) Option {
	return func(c *routerCfg) {
		c.reports = svc
	}
}

func mountReports(wr chi.Router, svc *reports.Service) {
	wr.Get("/reports/week", handleWeekReport(svc)) // fase 07 — INTOCADO (CA-04)
	wr.Get("/reports", handlePeriodReport(svc))    // fase 14 — genérico
}

// handleWeekReport — contrato §3.1 do plan da 07: date (YYYY-MM-DD) e tz
// (IANA) opcionais com defaults no service (CA-06); malformados → 400
// validation_error (CA-08, via domain.Validation do service).
func handleWeekReport(svc *reports.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		rep, err := svc.Week(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
			r.URL.Query().Get("date"), r.URL.Query().Get("tz"))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, rep)
	}
}

// handlePeriodReport — fase 14 (contrato §3.1): period|from&to&tz&filtros&
// compare. ResolveWindow valida period/datas/tz/to<from (CA-07); Report valida
// os filtros (CA-11). settings vêm do report_settings do usuário (CA-21).
func handlePeriodReport(svc *reports.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		m := reqctx.MembershipFrom(r.Context())
		userID := reqctx.UserID(r.Context())

		win, err := svc.ResolveWindow(q.Get("period"), q.Get("date"),
			q.Get("from"), q.Get("to"), q.Get("tz"))
		if err != nil {
			writeDomainError(w, err)
			return
		}

		settings, err := svc.GetSettings(r.Context(), userID)
		if err != nil {
			writeDomainError(w, err)
			return
		}

		filters := reports.Filters{
			ProjectID:   q.Get("project_id"),
			ClientID:    q.Get("client_id"),
			ObjectiveID: q.Get("objective_id"),
		}
		compare := q.Get("compare") == "previous"

		rep, err := svc.Report(r.Context(), m.WorkspaceID, userID, win, filters, settings, compare)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, rep)
	}
}
