// timer_routes.go — wiring da fase 03: rotas POR USUÁRIO penduradas direto
// no /api/v1 (atrás do RequireAuth, fora do subrouter de workspace —
// convenção do specs/INDEX.md). O timer atravessa workspaces ("máx 1 timer
// por usuário" é regra global); membership do workspace_id do payload é
// validada no handler de start (T-11.1).
package api

import (
	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/timer"
	"timetrack.live/backend/internal/workspace"
)

// WithTimer liga as rotas de timer/settings da fase 03 no router.
func WithTimer(svc *timer.Service) Option {
	return func(c *routerCfg) {
		c.timer = svc
	}
}

func mountTimer(r chi.Router, svc *timer.Service, ws *workspace.Store) {
	r.Get("/settings/pomodoro", handleGetPomodoro(svc))
	r.Put("/settings/pomodoro", handlePutPomodoro(svc))
	r.Route("/timer", func(tr chi.Router) {
		tr.Get("/", handleTimerState(svc))
		tr.Patch("/", handleTimerEdit(svc)) // fase 27 — allowance HITL 23/06
		tr.Post("/start", handleTimerStart(svc, ws))
		tr.Post("/stop", handleTimerStop(svc))
		tr.Post("/pause", handleTimerPause(svc))
		tr.Post("/resume", handleTimerResume(svc))
		tr.Post("/break/skip", handleTimerSkipBreak(svc))
	})
}
