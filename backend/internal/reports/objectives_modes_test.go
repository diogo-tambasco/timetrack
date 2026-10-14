// objectives_modes_test.go — fase 29 T-04.1 / CA-06.3 (29-H7): os Relatórios
// derivam "concluído by_hours" POR MODO (tracked cumulativo per-usuário >= meta),
// não só do status cru. Um goal by_hours com a meta batida mas status='open'
// (sem mutação de task que convergisse — CA-06.2) aparece concluído nos
// Relatórios. by_tasks/by_date seguem por status (sem regressão).
package reports_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reports"
)

// seedObjectiveMode é como seedObjective mas com completion_mode/target_date.
func seedObjectiveMode(t *testing.T, pool *pgxpool.Pool, wsID, projectID, createdBy, name, status, mode string, goalHours *float64) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO objectives (workspace_id, project_id, name, status, completion_mode, goal_hours, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		wsID, projectID, name, status, mode, goalHours, createdBy).Scan(&id); err != nil {
		t.Fatalf("seed objective mode %s: %v", name, err)
	}
	return id
}

// TestObjectivesByHoursMetaBatidaConcluido: goal by_hours, meta 2h, com tracked
// cumulativo >= 2h mas status='open' → aparece como concluído (is_complete) nos
// Relatórios (CA-06.3). Mutação que deriva "concluído" só do status cru quebra.
func TestObjectivesByHoursMetaBatidaConcluido(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-bh", "bh29@tt.dev")
	ws := seedWorkspace(t, pool, "BH29")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	// by_hours, meta 2h (7200s), status OPEN (não convergiu — CA-06.2).
	obj := seedObjectiveMode(t, pool, ws, proj, user, "Bater 2h", "open", "by_hours", f64(2))
	tk := seedTask(t, pool, ws, proj, user, "t1", "todo", &obj)
	// entry cumulativa DENTRO da semana, 7300s > 2h → meta batida.
	insertClosedTask(t, pool, ws, user, &proj, &tk, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 7300)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo by_hours não veio no relatório: %+v", rep.Objectives)
	}
	if o.CompletionMode != "by_hours" {
		t.Errorf("completion_mode esperado by_hours, veio %q", o.CompletionMode)
	}
	if !o.IsComplete {
		t.Errorf("by_hours com a meta batida deveria aparecer concluído (is_complete) mesmo com status=open, veio status=%q is_complete=%v", o.Status, o.IsComplete)
	}
}

// TestObjectivesByHoursAbaixoDaMetaNaoConcluido: by_hours abaixo da meta e
// status open → NÃO concluído.
func TestObjectivesByHoursAbaixoDaMetaNaoConcluido(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-bh2", "bh229@tt.dev")
	ws := seedWorkspace(t, pool, "BH229")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	obj := seedObjectiveMode(t, pool, ws, proj, user, "Bater 2h", "open", "by_hours", f64(2))
	tk := seedTask(t, pool, ws, proj, user, "t1", "todo", &obj)
	insertClosedTask(t, pool, ws, user, &proj, &tk, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 3600) // 1h < 2h

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo by_hours não veio: %+v", rep.Objectives)
	}
	if o.IsComplete {
		t.Errorf("by_hours abaixo da meta NÃO deveria ser concluído, veio is_complete=%v", o.IsComplete)
	}
}

// TestObjectivesByTasksPorStatus: by_tasks (default) segue por status (sem
// regressão) — status done → is_complete true; open → false.
func TestObjectivesByTasksPorStatus(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-bt", "bt29@tt.dev")
	ws := seedWorkspace(t, pool, "BT29")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	objDone := seedObjectiveMode(t, pool, ws, proj, user, "Done", "done", "by_tasks", f64(1))
	objOpen := seedObjectiveMode(t, pool, ws, proj, user, "Open", "open", "by_tasks", f64(1))
	tkD := seedTask(t, pool, ws, proj, user, "td", "done", &objDone)
	tkO := seedTask(t, pool, ws, proj, user, "to", "todo", &objOpen)
	insertClosedTask(t, pool, ws, user, &proj, &tkD, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)
	insertClosedTask(t, pool, ws, user, &proj, &tkO, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if o := objByID(rep.Objectives, objDone); o == nil || !o.IsComplete {
		t.Errorf("by_tasks status=done deveria ser concluído, veio %+v", o)
	}
	if o := objByID(rep.Objectives, objOpen); o == nil || o.IsComplete {
		t.Errorf("by_tasks status=open NÃO deveria ser concluído, veio %+v", o)
	}
}

// TestObjectivesByHoursCrossUserNaoConta (T-13/#6): horas de OUTRO usuário numa
// task do objetivo by_hours NÃO contam pro is_complete do relatório de `user`
// (bucket PER-USUÁRIO — decisão E). Mutação que remove `e.user_id = $2` da
// q-tracked-cum (reports/objectives.go) passaria a contar a entry alheia.
func TestObjectivesByHoursCrossUserNaoConta(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-cu", "cu29@tt.dev")
	other := seedUser(t, pool, "g29-cuo", "cuo29@tt.dev")
	ws := seedWorkspace(t, pool, "CU29")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	obj := seedObjectiveMode(t, pool, ws, proj, user, "Bater 2h", "open", "by_hours", f64(2))
	tk := seedTask(t, pool, ws, proj, user, "t1", "todo", &obj)
	// entry de 7300s (> 2h) mas do OUTRO usuário → não conta pro `user`.
	insertClosedTask(t, pool, ws, other, &proj, &tk, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 7300)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo by_hours (com meta) deveria aparecer: %+v", rep.Objectives)
	}
	if o.IsComplete {
		t.Errorf("horas de OUTRO usuário NÃO deveriam concluir o by_hours (per-usuário), veio is_complete=%v", o.IsComplete)
	}
}

// TestObjectivesByHoursCumulativoForaDaJanela (T-13/#9): is_complete by_hours usa
// o tracked CUMULATIVO (sem janela), não o realizado do período. Uma entry que
// bate a meta FORA da janela do período conclui mesmo assim. Mutação que troca
// trackedCum por realized (reports/objectives.go:196) deixaria de concluir.
func TestObjectivesByHoursCumulativoForaDaJanela(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-cfj", "cfj29@tt.dev")
	ws := seedWorkspace(t, pool, "CFJ29")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	obj := seedObjectiveMode(t, pool, ws, proj, user, "Bater 2h", "open", "by_hours", f64(2))
	tk := seedTask(t, pool, ws, proj, user, "t1", "todo", &obj)
	// entry de 7300s no MÊS ANTERIOR (fora da semana 08–14): trackedCum conta,
	// realized do período NÃO.
	insertClosedTask(t, pool, ws, user, &proj, &tk, time.Date(2026, 5, 20, 9, 0, 0, 0, loc), 7300)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo by_hours (com meta) deveria aparecer: %+v", rep.Objectives)
	}
	if o.RealizedSeconds != 0 {
		t.Errorf("realized do período deveria ser 0 (entry fora da janela), veio %d", o.RealizedSeconds)
	}
	if !o.IsComplete {
		t.Errorf("by_hours com a meta batida por horas CUMULATIVAS (fora da janela) deveria concluir, veio is_complete=%v", o.IsComplete)
	}
}

// TestObjectivesByTasksComHorasNaoConcluiPorHoras (T-13/#10): SÓ by_hours deriva
// "concluído" por horas. Um by_tasks (ou by_date) open com a meta de horas batida
// NÃO conclui por horas — segue por status (CA-06.3). Mutação que remove o guard
// `mode == "by_hours"` em isObjectiveComplete concluiria indevidamente.
func TestObjectivesByTasksComHorasNaoConcluiPorHoras(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g29-bth", "bth29@tt.dev")
	ws := seedWorkspace(t, pool, "BTH29")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	// by_tasks (não by_hours), open, COM meta de horas e horas batendo a meta.
	obj := seedObjectiveMode(t, pool, ws, proj, user, "Tasks com meta", "open", "by_tasks", f64(2))
	tk := seedTask(t, pool, ws, proj, user, "t1", "todo", &obj)
	insertClosedTask(t, pool, ws, user, &proj, &tk, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 7300) // > 2h

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo by_tasks deveria aparecer: %+v", rep.Objectives)
	}
	if o.IsComplete {
		t.Errorf("by_tasks open com horas batidas NÃO deveria concluir (só by_hours deriva por horas), veio is_complete=%v", o.IsComplete)
	}
}
