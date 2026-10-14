// timertest_test.go — harness da fase 03: estende o newCoreRouter com o
// timer.Service de CLOCK INJETÁVEL. Reusa loginVia/doJSON/errCode do
// coretest_test.go (fase 02).
package api_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/realtime"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/timer"
	"timetrack.live/backend/internal/workspace"
)

// fakeClock controla o relógio do servidor nos testes HTTP (D-01).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{t: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTimerRouter monta o router REAL (auth 01 + core 02 + timer 03) com o
// clock injetado. Desde a fase 04 o hub realtime vem plugado (o boot do
// cmd/server faz o mesmo wiring) — os testes da 03 rodam idênticos.
func newTimerRouter(t *testing.T, pool *pgxpool.Pool, now func() time.Time) (http.Handler, *auth.Service, *googlestub.Server, *timer.Service) {
	router, svc, stub, tsvc, _ := newRealtimeRouter(t, pool, now, 0, 0)
	return router, svc, stub, tsvc
}

// newRealtimeRouter — harness da fase 04: newTimerRouter + o hub exposto e
// ping do /ws injetável (0 = defaults do D-08).
func newRealtimeRouter(t *testing.T, pool *pgxpool.Pool, now func() time.Time, pingInterval, pingTimeout time.Duration) (http.Handler, *auth.Service, *googlestub.Server, *timer.Service, *realtime.Hub) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	var opts []timer.Option
	var eopts []entries.ServiceOption
	if now != nil {
		opts = append(opts, timer.WithClock(now))
		eopts = append(eopts, entries.WithClock(now))
	}
	tsvc := timer.NewService(pool, opts...)
	hub := realtime.NewHub()
	tsvc.SetOnChange(hub.BroadcastTimer) // mesmo wiring do boot (plan §3.3)
	ropts := []api.Option{
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:  clients.NewService(pool),
			Projects: projects.NewService(pool),
			Tasks:    tasks.NewService(pool),
		}),
		api.WithTimer(tsvc),
		api.WithEntries(entries.NewService(pool, eopts...)),
		api.WithRealtime(hub),
	}
	if pingInterval > 0 {
		ropts = append(ropts, api.WithWSPing(pingInterval, pingTimeout))
	}
	return api.NewRouter(ropts...), svc, stub, tsvc, hub
}
