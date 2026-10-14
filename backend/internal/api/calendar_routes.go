// calendar_routes.go — wiring da fase 06: rotas POR USUÁRIO do Google
// Calendar (leitura) no /api/v1 atrás do RequireAuth (D-05). O ângulo 4
// (/reports/planned) é montado no subrouter do workspace em auth_routes.go.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/reqctx"
)

// WithCalendar liga a leitura do Google Calendar no router.
func WithCalendar(svc *calendar.Service) Option {
	return func(c *routerCfg) { c.calendar = svc }
}

// mountCalendar pendura status/events/connect no /api/v1 (já atrás do
// RequireAuth — connect exige sessão, CA-02).
func mountCalendar(api chi.Router, cfg *routerCfg) {
	api.Get("/calendar/status", handleCalendarStatus(cfg.calendar))
	api.Get("/calendar/events", handleCalendarEvents(cfg.calendar))
	api.Get("/calendar/connect", handleCalendarConnect(cfg.auth))
	// fase 11: preferências da UI de calendário por usuário (sincroniza devices)
	api.Get("/settings/calendar", handleGetCalendarSettings(cfg.calendar))
	api.Put("/settings/calendar", handlePutCalendarSettings(cfg.calendar))
}

// handleCalendarStatus (CA-04): o usuário conectou o calendar?
func handleCalendarStatus(svc *calendar.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connected, err := svc.Status(r.Context(), reqctx.UserID(r.Context()))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]bool{"connected": connected})
	}
}

// handleCalendarEvents (CA-05/06/10/10.1/11): eventos da agenda primary na
// semana pedida. Erros tipados (*domain.Error) fluem pelo writeDomainError.
func handleCalendarEvents(svc *calendar.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from, to, err := calendarWindow(q.Get("from"), q.Get("to"), q.Get("tz"))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		events, err := svc.Events(r.Context(), reqctx.UserID(r.Context()), from, to)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"events": events})
	}
}

// handleCalendarConnect (CA-01/02): inicia o consentimento incremental com
// o escopo de leitura do calendar; retorno → /calendario.
func handleCalendarConnect(authSvc interface {
	StartConsent(http.ResponseWriter, *http.Request, string, ...string)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authSvc.StartConsent(w, r, "/calendario", calendar.ReadonlyScope)
	}
}

// calendarPlannedSource adapta o calendar.Service à interface consumidora
// reports.PlannedSource (reports não importa calendar — sem ciclo).
type calendarPlannedSource struct{ cal *calendar.Service }

func (c calendarPlannedSource) Connected(ctx context.Context, userID string) (bool, error) {
	return c.cal.Status(ctx, userID)
}

func (c calendarPlannedSource) Events(ctx context.Context, userID string, from, to time.Time) ([]reports.PlannedEvent, error) {
	evs, err := c.cal.Events(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]reports.PlannedEvent, len(evs))
	for i, e := range evs {
		out[i] = reports.PlannedEvent{Start: e.Start, End: e.End, AllDay: e.AllDay}
	}
	return out, nil
}

// handlePlannedReport (CA-13..CA-17): ângulo 4 no subrouter do workspace
// (precisa de membership pras time entries). Erros tipados via writeDomainError.
func handlePlannedReport(rsvc *reports.Service, csvc *calendar.Service) http.HandlerFunc {
	src := calendarPlannedSource{cal: csvc}
	return func(w http.ResponseWriter, r *http.Request) {
		m := reqctx.MembershipFrom(r.Context())
		rep, err := rsvc.PlannedReport(r.Context(), m.WorkspaceID, reqctx.UserID(r.Context()),
			r.URL.Query().Get("date"), r.URL.Query().Get("tz"), src)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, rep)
	}
}

// calendarWindow resolve [from 00:00 loc, (to+1d) 00:00 loc) no fuso tz
// (mesma regra do entries.ListDay). Defaults: from/to vazios → hoje; tz
// vazio → UTC. Malformados → 400 validation_error (CA-11).
func calendarWindow(fromS, toS, tz string) (time.Time, time.Time, error) {
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return time.Time{}, time.Time{}, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	today := time.Now().In(loc).Format("2006-01-02")
	if fromS == "" {
		fromS = today
	}
	if toS == "" {
		toS = today
	}
	from, err := time.ParseInLocation("2006-01-02", fromS, loc)
	if err != nil {
		return time.Time{}, time.Time{}, domain.Validation("from deve ser uma data YYYY-MM-DD")
	}
	to, err := time.ParseInLocation("2006-01-02", toS, loc)
	if err != nil {
		return time.Time{}, time.Time{}, domain.Validation("to deve ser uma data YYYY-MM-DD")
	}
	return from, to.AddDate(0, 0, 1), nil
}
