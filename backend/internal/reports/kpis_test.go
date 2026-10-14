// kpis_test.go — T-04 da fase 14: focus_by_hour (CA-22/23) e kpis (CA-24).
// Tudo deriva da list (zero query nova). 24 buckets sempre; bucket pela hora
// LOCAL do started_at (só foco ≥1500s); KPIs com div/0 protegida.
package reports_test

import (
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reports"
)

func hourBucket(buckets []reports.FocusHourBucket, h int) int {
	for _, b := range buckets {
		if b.Hour == h {
			return b.FocusSeconds
		}
	}
	return -1
}

// CA-22/23 — entry de FOCO às 16h local soma no bucket 16; fragmento (<1500s)
// NÃO soma; sempre 24 buckets (0–23, zeros).
func TestFocusByHourBucket(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-fh", "fh14@tt.dev")
	ws := seedWorkspace(t, pool, "FH14")
	loc := brt(t)

	// foco às 16h local (2h)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 16, 0, 0, 0, loc), 7200)
	// outro foco às 16h (30min, no limiar)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 16, 30, 0, 0, loc), 1500)
	// fragmento às 9h (não soma em foco/hora)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if len(rep.FocusByHour) != 24 {
		t.Fatalf("esperava 24 buckets, veio %d", len(rep.FocusByHour))
	}
	if got := hourBucket(rep.FocusByHour, 16); got != 8700 { // 7200 + 1500
		t.Errorf("bucket 16h = %d, esperava 8700", got)
	}
	if got := hourBucket(rep.FocusByHour, 9); got != 0 {
		t.Errorf("bucket 9h = %d, esperava 0 (fragmento não soma)", got)
	}
	// todos os outros buckets zerados
	for _, b := range rep.FocusByHour {
		if b.Hour != 16 && b.FocusSeconds != 0 {
			t.Errorf("bucket %dh deveria ser 0, veio %d", b.Hour, b.FocusSeconds)
		}
	}
}

// CA-22 — bucket pela hora LOCAL do fuso: entry às 23:30 UTC do dia 09 = 20:30
// em SP (UTC-3). Some no bucket 20, não no 23.
func TestFocusByHourFuso(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-ff", "ff14@tt.dev")
	ws := seedWorkspace(t, pool, "FF14")

	// 2026-06-09 23:30 UTC = 20:30 America/Sao_Paulo (UTC-3); foco de 2h
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 9, 23, 30, 0, 0, time.UTC), 7200)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if got := hourBucket(rep.FocusByHour, 20); got != 7200 {
		t.Errorf("bucket 20h (local SP) = %d, esperava 7200", got)
	}
	if got := hourBucket(rep.FocusByHour, 23); got != 0 {
		t.Errorf("bucket 23h (UTC) = %d, esperava 0 (não usa UTC)", got)
	}
}

// CA-24 — kpis: deep_focus_pct = round(focus/tracked×100); avg_session =
// tracked/nEntries; untracked = availability.untracked. Valores exatos.
func TestKpisValores(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-kp", "kp14@tt.dev")
	ws := seedWorkspace(t, pool, "KP14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	// 2 entries: 1 foco (3600), 1 fragmento (1200) → tracked 4800, focus 3600
	insertClosed(t, pool, ws, user, nil, meio, 3600)
	insertClosed(t, pool, ws, user, nil, meio.Add(2*time.Hour), 1200)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	k := rep.Kpis
	if k.TrackedSeconds != 4800 {
		t.Errorf("tracked %d, esperava 4800", k.TrackedSeconds)
	}
	if k.DeepFocusPct != 75 { // round(3600/4800×100)
		t.Errorf("deep_focus_pct %d, esperava 75", k.DeepFocusPct)
	}
	if k.AvgSessionSeconds != 2400 { // 4800/2
		t.Errorf("avg_session %d, esperava 2400", k.AvgSessionSeconds)
	}
	if k.UntrackedSeconds != rep.Availability.UntrackedSeconds {
		t.Errorf("untracked %d ≠ availability.untracked %d", k.UntrackedSeconds, rep.Availability.UntrackedSeconds)
	}
}

// CA-24 — período vazio: deep_focus_pct e avg_session = 0 (sem NaN/div-zero),
// e os 24 buckets zerados.
func TestKpisVazioSemNaN(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-kv", "kv14@tt.dev")
	ws := seedWorkspace(t, pool, "KV14")

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	k := rep.Kpis
	if k.TrackedSeconds != 0 || k.DeepFocusPct != 0 || k.AvgSessionSeconds != 0 {
		t.Errorf("vazio: esperava 0/0/0, veio %+v", k)
	}
	if len(rep.FocusByHour) != 24 {
		t.Errorf("esperava 24 buckets mesmo vazio, veio %d", len(rep.FocusByHour))
	}
	for _, b := range rep.FocusByHour {
		if b.FocusSeconds != 0 {
			t.Errorf("bucket %dh deveria ser 0 em período vazio", b.Hour)
		}
	}
}
