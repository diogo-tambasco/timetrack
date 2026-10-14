// settings_test.go — T-05 da fase 14: report.Settings (Get/Save, CA-29) e a
// comparação previous (CA-25/26). Settings espelha calendar.settings (defaults
// sem linha, validação ANTES do SQL, upsert, isolamento). previous = 2ª passada
// com a janela deslocada (week −7d; month addMonths −1; custom −duração).
package reports_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/reports"
)

// CA-29 — GetSettings sem linha → defaults 8/true.
func TestSettingsDefaults(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-s1", "s1@tt.dev")
	svc := newSvc(t, pool, clockMeioDaSemana)

	st, err := svc.GetSettings(context.Background(), user)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.WorkHoursPerDay != 8 || !st.WorkdaysOnly {
		t.Errorf("defaults errados: %+v, esperava 8/true", st)
	}
}

// CA-29 — SaveSettings persiste e GetSettings devolve o salvo (upsert).
func TestSettingsUpsert(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-s2", "s2@tt.dev")
	svc := newSvc(t, pool, clockMeioDaSemana)
	ctx := context.Background()

	if _, err := svc.SaveSettings(ctx, user, reports.Settings{WorkHoursPerDay: 6, WorkdaysOnly: false}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	st, err := svc.GetSettings(ctx, user)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.WorkHoursPerDay != 6 || st.WorkdaysOnly {
		t.Errorf("após save: %+v, esperava 6/false", st)
	}
	// upsert: salvar de novo atualiza
	if _, err := svc.SaveSettings(ctx, user, reports.Settings{WorkHoursPerDay: 10, WorkdaysOnly: true}); err != nil {
		t.Fatalf("SaveSettings 2: %v", err)
	}
	st, _ = svc.GetSettings(ctx, user)
	if st.WorkHoursPerDay != 10 || !st.WorkdaysOnly {
		t.Errorf("após upsert: %+v, esperava 10/true", st)
	}
}

// CA-29 — validação ANTES do SQL: hpd ≤0 ou >24 → 400 e NÃO persiste.
func TestSettingsValidacaoNaoPersiste(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-s3", "s3@tt.dev")
	svc := newSvc(t, pool, clockMeioDaSemana)
	ctx := context.Background()

	for _, hpd := range []float64{0, -1, 25} {
		_, err := svc.SaveSettings(ctx, user, reports.Settings{WorkHoursPerDay: hpd, WorkdaysOnly: true})
		de, ok := err.(*domain.Error)
		if !ok || de.Code != "validation_error" {
			t.Errorf("hpd=%g: esperava validation_error, veio %T %v", hpd, err, err)
		}
	}
	// nada persistido → ainda defaults
	st, _ := svc.GetSettings(ctx, user)
	if st.WorkHoursPerDay != 8 {
		t.Errorf("inválido persistiu: %+v", st)
	}
}

// CA-29 — isolamento: settings de um user não vazam pro outro.
func TestSettingsIsolamento(t *testing.T) {
	pool := dbtest.NewPool(t)
	ua := seedUser(t, pool, "g14-s4a", "s4a@tt.dev")
	ub := seedUser(t, pool, "g14-s4b", "s4b@tt.dev")
	svc := newSvc(t, pool, clockMeioDaSemana)
	ctx := context.Background()

	if _, err := svc.SaveSettings(ctx, ua, reports.Settings{WorkHoursPerDay: 4, WorkdaysOnly: false}); err != nil {
		t.Fatalf("save ua: %v", err)
	}
	stb, _ := svc.GetSettings(ctx, ub)
	if stb.WorkHoursPerDay != 8 || !stb.WorkdaysOnly {
		t.Errorf("settings de ua vazaram pro ub: %+v", stb)
	}
}

// CA-25 — previous na semana = semana anterior, mesmos filtros, total+kpis.
func TestPreviousSemana(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-pv-s", "pvs@tt.dev")
	ws := seedWorkspace(t, pool, "PVS14")
	loc := brt(t)

	// semana atual 08–14: 600s; semana anterior 01–07: 1800s
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 3, 9, 0, 0, 0, loc), 1800)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP,
		reports.Filters{}, reports.DefaultSettings(), true)

	if rep.TotalSeconds != 600 {
		t.Errorf("período atual %d, esperava 600", rep.TotalSeconds)
	}
	if rep.Previous == nil {
		t.Fatal("previous nil com compare=true")
	}
	if rep.Previous.TotalSeconds != 1800 {
		t.Errorf("previous total %d, esperava 1800 (semana anterior)", rep.Previous.TotalSeconds)
	}
	if rep.Previous.Kpis.TrackedSeconds != 1800 {
		t.Errorf("previous kpis tracked %d, esperava 1800", rep.Previous.Kpis.TrackedSeconds)
	}
}

// CA-25 — previous no mês = mês anterior (addMonths −1, clamp do 1º dia).
func TestPreviousMes(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-pv-m", "pvm@tt.dev")
	ws := seedWorkspace(t, pool, "PVM14")
	loc := brt(t)

	// junho: 600s; maio: 1200s
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 5, 15, 9, 0, 0, 0, loc), 1200)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "month", "2026-06-10", "", "", tzSP,
		reports.Filters{}, reports.DefaultSettings(), true)

	if rep.From != "2026-06-01" || rep.To != "2026-06-30" {
		t.Errorf("período %s→%s, esperava junho", rep.From, rep.To)
	}
	if rep.TotalSeconds != 600 {
		t.Errorf("junho %d, esperava 600", rep.TotalSeconds)
	}
	if rep.Previous == nil || rep.Previous.TotalSeconds != 1200 {
		t.Errorf("previous (maio) %+v, esperava 1200", rep.Previous)
	}
}

// CA-25 — previous custom = janela de mesmo tamanho terminando no dia antes de
// from. Custom de 3 dias (10–12) → anterior é 07–09.
func TestPreviousCustom(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-pv-c", "pvc@tt.dev")
	ws := seedWorkspace(t, pool, "PVC14")
	loc := brt(t)

	// custom 10–12: 600s; anterior 07–09: 900s
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 11, 9, 0, 0, 0, loc), 600)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 9, 0, 0, 0, loc), 900)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := reportWith(t, svc, ws, user, "", "", "2026-06-10", "2026-06-12", tzSP,
		reports.Filters{}, reports.DefaultSettings(), true)

	if rep.TotalSeconds != 600 {
		t.Errorf("custom 10–12 %d, esperava 600", rep.TotalSeconds)
	}
	if rep.Previous == nil || rep.Previous.TotalSeconds != 900 {
		t.Errorf("previous (07–09) %+v, esperava 900", rep.Previous)
	}
}

// CA-26 — sem compare → previous nil (sem custo).
func TestSemCompareNil(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-nc", "nc@tt.dev")
	ws := seedWorkspace(t, pool, "NC14")
	svc := newSvc(t, pool, clockMeioDaSemana)

	rep := reportWith(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP,
		reports.Filters{}, reports.DefaultSettings(), false)
	if rep.Previous != nil {
		t.Errorf("sem compare: previous deveria ser nil, veio %+v", rep.Previous)
	}
}
