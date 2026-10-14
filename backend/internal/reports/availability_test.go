// availability_test.go — T-03 da fase 14: rastreado × não-rastreado (CA-20/21).
// expected = work_hours_per_day×3600 × dias do período (todos, ou só seg–sex se
// workdays_only). Dias por calendário no loc (D-06). Sem div/0: expected=0 →
// tracked_pct=0; untracked clampa em 0.
package reports_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reports"
)

// reportWith roda o Report com Settings e compare explícitos.
func reportWith(t *testing.T, svc *reports.Service, ws, user, period, date, from, to, tz string,
	f reports.Filters, st reports.Settings, compare bool) reports.PeriodReport {
	t.Helper()
	win, err := svc.ResolveWindow(period, date, from, to, tz)
	if err != nil {
		t.Fatalf("ResolveWindow: %v", err)
	}
	rep, err := svc.Report(context.Background(), ws, user, win, f, st, compare)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	return rep
}

// CA-21 — semana com workdays_only (default) = 5 dias úteis × 8h = 144000s.
func TestAvailabilitySemanaUtil(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-av1", "av1@tt.dev")
	ws := seedWorkspace(t, pool, "AV1")
	loc := brt(t)
	// 2h rastreadas na semana
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 7200)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP,
		reports.Filters{}, reports.Settings{WorkHoursPerDay: 8, WorkdaysOnly: true}, false)

	a := rep.Availability
	if a.ExpectedSeconds != 144000 { // 5 × 8 × 3600
		t.Errorf("expected %d, esperava 144000 (5 dias úteis × 8h)", a.ExpectedSeconds)
	}
	if a.TrackedSeconds != 7200 {
		t.Errorf("tracked %d, esperava 7200", a.TrackedSeconds)
	}
	if a.UntrackedSeconds != 144000-7200 {
		t.Errorf("untracked %d, esperava %d", a.UntrackedSeconds, 144000-7200)
	}
	if a.TrackedPct != 5 { // round(7200/144000×100) = 5
		t.Errorf("tracked_pct %d, esperava 5", a.TrackedPct)
	}
	if a.ExpectedBasis == "" {
		t.Error("expected_basis vazio")
	}
}

// CA-21 — workdays_only=false conta os 7 dias da semana × 8h = 201600s.
func TestAvailabilitySemanaInteira(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-av2", "av2@tt.dev")
	ws := seedWorkspace(t, pool, "AV2")

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP,
		reports.Filters{}, reports.Settings{WorkHoursPerDay: 8, WorkdaysOnly: false}, false)

	if rep.Availability.ExpectedSeconds != 201600 { // 7 × 8 × 3600
		t.Errorf("expected %d, esperava 201600 (7 dias × 8h)", rep.Availability.ExpectedSeconds)
	}
}

// CA-20/21 — custom de 1 dia num SÁBADO com workdays_only → expected 0 →
// tracked_pct 0 (sem div/0). 2026-06-13 é sábado.
func TestAvailabilitySabadoSemDiaUtil(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-av3", "av3@tt.dev")
	ws := seedWorkspace(t, pool, "AV3")
	loc := brt(t)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 13, 10, 0, 0, 0, loc), 1800)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "", "", "2026-06-13", "2026-06-13", tzSP,
		reports.Filters{}, reports.Settings{WorkHoursPerDay: 8, WorkdaysOnly: true}, false)

	a := rep.Availability
	if a.ExpectedSeconds != 0 {
		t.Errorf("expected %d, esperava 0 (sábado, só dias úteis)", a.ExpectedSeconds)
	}
	if a.TrackedPct != 0 {
		t.Errorf("tracked_pct %d, esperava 0 (sem div/0 com expected=0)", a.TrackedPct)
	}
	if a.UntrackedSeconds != 0 {
		t.Errorf("untracked %d, esperava 0 (max(exp−tracked,0))", a.UntrackedSeconds)
	}
}

// CA-20 — untracked clampa em 0 quando tracked > expected; tracked_pct capa
// em 100.
func TestAvailabilityClampUntracked(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-av4", "av4@tt.dev")
	ws := seedWorkspace(t, pool, "AV4")
	loc := brt(t)
	// dia útil (seg 2026-06-08), 10h rastreadas com expected 8h → tracked>expected
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 9, 0, 0, 0, loc), 36000)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "day", "2026-06-08", "", "", tzSP,
		reports.Filters{}, reports.Settings{WorkHoursPerDay: 8, WorkdaysOnly: true}, false)

	a := rep.Availability
	if a.ExpectedSeconds != 28800 { // 1 dia útil × 8h
		t.Errorf("expected %d, esperava 28800", a.ExpectedSeconds)
	}
	if a.UntrackedSeconds != 0 {
		t.Errorf("untracked %d, esperava 0 (tracked>expected clampa)", a.UntrackedSeconds)
	}
	if a.TrackedPct != 100 {
		t.Errorf("tracked_pct %d, esperava 100 (capado)", a.TrackedPct)
	}
}
