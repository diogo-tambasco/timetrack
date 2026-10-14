// planned_test.go — fase 06 ângulo 4 (Planejado × Real): PlannedReport
// agrega planejado (eventos COM horário, por dia do start; all-day fora —
// CA-13.1) × realizado (entries fechadas por dia do started_at). weekWindow
// idêntica ao /reports/week (CA-13/15).
package reports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reports"
)

// fakeSource implementa reports.PlannedSource (sem Google).
type fakeSource struct {
	connected bool
	events    []reports.PlannedEvent
	eventsErr error
	called    bool
}

func (f *fakeSource) Connected(_ context.Context, _ string) (bool, error) { return f.connected, nil }
func (f *fakeSource) Events(_ context.Context, _ string, _, _ time.Time) ([]reports.PlannedEvent, error) {
	f.called = true
	return f.events, f.eventsErr
}

func dayOf(rep reports.PlannedReport, date string) reports.PlannedDay {
	for _, d := range rep.Days {
		if d.Date == date {
			return d
		}
	}
	return reports.PlannedDay{Date: "FALTANDO"}
}

// CA-13/13.1/15: planejado por dia do start (all-day excluído; fora da
// semana não conta); realizado por dia do started_at; week_end = domingo.
func TestPlannedReportConectado(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g-planned", "planned@example.com")
	ws := seedWorkspace(t, pool, "WS Planned")
	loc := brt(t)
	svc := newSvc(t, pool, clockMeioDaSemana) // semana 2026-06-08 → 06-14 em SP

	mon := time.Date(2026, 6, 8, 0, 0, 0, 0, loc)
	src := &fakeSource{connected: true, events: []reports.PlannedEvent{
		{Start: mon.Add(10 * time.Hour), End: mon.Add(12 * time.Hour), AllDay: false},                                   // seg 7200s
		{Start: time.Date(2026, 6, 9, 0, 0, 0, 0, loc), End: time.Date(2026, 6, 10, 0, 0, 0, 0, loc), AllDay: true},     // ter all-day — excluído
		{Start: time.Date(2026, 6, 20, 10, 0, 0, 0, loc), End: time.Date(2026, 6, 20, 11, 0, 0, 0, loc), AllDay: false}, // FORA da semana
	}}
	// realizado: 3600s na segunda
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 9, 0, 0, 0, loc), 3600)

	rep, err := svc.PlannedReport(context.Background(), ws, user, "", tzSP, src)
	if err != nil {
		t.Fatalf("PlannedReport: %v", err)
	}
	if rep.WeekStart != "2026-06-08" || rep.WeekEnd != "2026-06-14" {
		t.Errorf("semana errada: %s → %s (esperado 2026-06-08 → 2026-06-14)", rep.WeekStart, rep.WeekEnd)
	}
	if !rep.Connected {
		t.Error("connected devia ser true")
	}
	if len(rep.Days) != 7 {
		t.Fatalf("esperava 7 dias, veio %d", len(rep.Days))
	}
	if d := dayOf(rep, "2026-06-08"); d.PlannedSeconds != 7200 || d.RealizedSeconds != 3600 {
		t.Errorf("segunda: planned=%d realized=%d (esperado 7200/3600)", d.PlannedSeconds, d.RealizedSeconds)
	}
	if d := dayOf(rep, "2026-06-09"); d.PlannedSeconds != 0 {
		t.Errorf("terça (all-day) devia ter planned=0, veio %d", d.PlannedSeconds)
	}
	total := 0
	for _, d := range rep.Days {
		total += d.PlannedSeconds
	}
	if total != 7200 {
		t.Errorf("evento fora da semana entrou no total: %d (esperado 7200)", total)
	}
}

// CA-14: não conectado → connected:false, planned 0, realizado preenchido,
// e o Google (Events) NÃO é chamado.
func TestPlannedReportNaoConectado(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g-noconn", "noconn@example.com")
	ws := seedWorkspace(t, pool, "WS NoConn")
	loc := brt(t)
	svc := newSvc(t, pool, clockMeioDaSemana)

	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 9, 0, 0, 0, loc), 1800)
	src := &fakeSource{connected: false}

	rep, err := svc.PlannedReport(context.Background(), ws, user, "", tzSP, src)
	if err != nil {
		t.Fatalf("PlannedReport: %v", err)
	}
	if rep.Connected {
		t.Error("connected devia ser false")
	}
	if src.called {
		t.Error("não conectado NÃO devia chamar Events (Google)")
	}
	if d := dayOf(rep, "2026-06-08"); d.PlannedSeconds != 0 || d.RealizedSeconds != 1800 {
		t.Errorf("segunda: planned=%d realized=%d (esperado 0/1800)", d.PlannedSeconds, d.RealizedSeconds)
	}
}

// Verificação do build (#3): token revogado / Google fora (Events erra) NÃO
// derruba o relatório nem some com o realizado — degrada pra connected:false.
func TestPlannedReportDegradaQuandoEventsFalha(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g-degrada", "degrada@example.com")
	ws := seedWorkspace(t, pool, "WS Degrada")
	loc := brt(t)
	svc := newSvc(t, pool, clockMeioDaSemana)

	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 9, 0, 0, 0, loc), 1200)
	src := &fakeSource{connected: true, eventsErr: errors.New("token revogado")}

	rep, err := svc.PlannedReport(context.Background(), ws, user, "", tzSP, src)
	if err != nil {
		t.Fatalf("degradação NÃO devia retornar erro: %v", err)
	}
	if rep.Connected {
		t.Error("Events falho devia degradar pra connected:false")
	}
	if d := dayOf(rep, "2026-06-08"); d.PlannedSeconds != 0 || d.RealizedSeconds != 1200 {
		t.Errorf("realizado devia seguir preenchido: planned=%d realized=%d (esperado 0/1200)", d.PlannedSeconds, d.RealizedSeconds)
	}
}
