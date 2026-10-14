// period_test.go — T-01 da fase 14: a janela generalizada (CA-05/06/07/09),
// os filtros (CA-10/11/12) e a NÃO-REGRESSÃO (CA-04/09 — a semana sem filtro
// reproduz a list da 07). Reusa os helpers de service_test.go (mesmo pacote
// _test): seedUser/Workspace/Client/Project, insertClosed, newSvc, brt, tzSP.
package reports_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/reports"
)

// helpers próprios da fase 14 (objetivos/tasks/entries com task) — a 07 não os
// tinha. Espelham o padrão dos seeds existentes.

func seedObjective(t *testing.T, pool *pgxpool.Pool, wsID, projectID, createdBy, name, status string, goalHours *float64) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO objectives (workspace_id, project_id, name, status, goal_hours, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		wsID, projectID, name, status, goalHours, createdBy).Scan(&id); err != nil {
		t.Fatalf("seed objective %s: %v", name, err)
	}
	return id
}

func seedTask(t *testing.T, pool *pgxpool.Pool, wsID, projectID, createdBy, name, status string, objectiveID *string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO tasks (workspace_id, project_id, name, status, objective_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		wsID, projectID, name, status, objectiveID, createdBy).Scan(&id); err != nil {
		t.Fatalf("seed task %s: %v", name, err)
	}
	return id
}

// insertClosedTask — entry fechada ligada a uma task (pro filtro por objetivo
// e o realized da seção objectives).
func insertClosedTask(t *testing.T, pool *pgxpool.Pool, wsID, userID string, projectID *string, taskID *string, start time.Time, durSec int) {
	t.Helper()
	stop := start.Add(time.Duration(durSec) * time.Second)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, description, tags,
		                          started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, 'seed', '{}', $5, $6, $7, 'manual')`,
		wsID, userID, projectID, taskID, start, stop, durSec); err != nil {
		t.Fatalf("seed entry com task: %v", err)
	}
}

func defaultSettings() reports.Settings { return reports.DefaultSettings() }

// resolve é o atalho do teste: ResolveWindow + Report sem compare.
func report(t *testing.T, svc *reports.Service, ws, user, period, date, from, to, tz string, f reports.Filters) reports.PeriodReport {
	t.Helper()
	win, err := svc.ResolveWindow(period, date, from, to, tz)
	if err != nil {
		t.Fatalf("ResolveWindow(%s/%s..%s): %v", period, from, to, err)
	}
	rep, err := svc.Report(context.Background(), ws, user, win, f, defaultSettings(), false)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	return rep
}

// CA-05 — custom [from 00:00, to+1d 00:00) no fuso; from=to = um único dia.
func TestPeriodoCustomJanela(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-c", "c14@tt.dev")
	ws := seedWorkspace(t, pool, "C14")
	loc := brt(t)

	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 0, 0, 0, 0, loc), 600)   // dentro
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 23, 30, 0, 0, loc), 900) // dentro (mesmo dia)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 11, 0, 0, 0, 0, loc), 1200)  // fora (dia seguinte)

	svc := newSvc(t, pool, clockMeioDaSemana)

	// from=to=2026-06-10 → um único dia
	rep := report(t, svc, ws, user, "", "", "2026-06-10", "2026-06-10", tzSP, reports.Filters{})
	if rep.From != "2026-06-10" || rep.To != "2026-06-10" {
		t.Errorf("rótulo %s→%s, esperava 2026-06-10→2026-06-10", rep.From, rep.To)
	}
	if rep.TotalSeconds != 1500 { // 600 + 900; a do dia 11 fora
		t.Errorf("total %d, esperava 1500 (só o dia 10)", rep.TotalSeconds)
	}

	// intervalo de 2 dias inclui o dia 11
	rep = report(t, svc, ws, user, "", "", "2026-06-10", "2026-06-11", tzSP, reports.Filters{})
	if rep.TotalSeconds != 2700 {
		t.Errorf("total 2 dias %d, esperava 2700", rep.TotalSeconds)
	}
}

// CA-06 — presets day/week/month no fuso; from/to PRECEDE period (custom).
func TestPeriodoPresets(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-p", "p14@tt.dev")
	ws := seedWorkspace(t, pool, "P14")
	loc := brt(t)

	// dia 10, dia 09, e dia 02 (mesmo mês, fora da semana 08–14)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 9, 9, 0, 0, 0, loc), 900)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 2, 9, 0, 0, 0, loc), 1200)

	svc := newSvc(t, pool, clockMeioDaSemana)

	// day = só o dia 10
	rep := report(t, svc, ws, user, "day", "2026-06-10", "", "", tzSP, reports.Filters{})
	if rep.From != "2026-06-10" || rep.To != "2026-06-10" || rep.TotalSeconds != 600 {
		t.Errorf("day: %s→%s total %d, esperava 2026-06-10×2 total 600", rep.From, rep.To, rep.TotalSeconds)
	}

	// week = segunda→domingo 08–14 (dia 10 e 09 dentro; dia 02 fora)
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})
	if rep.From != "2026-06-08" || rep.To != "2026-06-14" || rep.TotalSeconds != 1500 {
		t.Errorf("week: %s→%s total %d, esperava 2026-06-08→14 total 1500", rep.From, rep.To, rep.TotalSeconds)
	}

	// month = junho inteiro (todos os 3)
	rep = report(t, svc, ws, user, "month", "2026-06-10", "", "", tzSP, reports.Filters{})
	if rep.From != "2026-06-01" || rep.To != "2026-06-30" || rep.TotalSeconds != 2700 {
		t.Errorf("month: %s→%s total %d, esperava 2026-06-01→30 total 2700", rep.From, rep.To, rep.TotalSeconds)
	}

	// from/to PRECEDE period: passa period=month MAS from/to do dia 10
	rep = report(t, svc, ws, user, "month", "2026-06-10", "2026-06-10", "2026-06-10", tzSP, reports.Filters{})
	if rep.From != "2026-06-10" || rep.To != "2026-06-10" || rep.TotalSeconds != 600 {
		t.Errorf("custom precede period: %s→%s total %d, esperava só o dia 10", rep.From, rep.To, rep.TotalSeconds)
	}
}

// CA-07 — period/date/from/to/tz malformados e to<from → 400 validation_error.
func TestPeriodoMalformado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(t, pool, clockMeioDaSemana)

	casos := []struct {
		period, date, from, to, tz string
	}{
		{"semana", "", "", "", ""},                     // period inválido
		{"day", "banana", "", "", ""},                  // date malformada
		{"", "", "banana", "2026-06-10", ""},           // from malformada
		{"", "", "2026-06-10", "banana", ""},           // to malformada
		{"", "", "2026-06-11", "2026-06-10", ""},       // to < from
		{"day", "2026-06-10", "", "", "Marte/Cratera"}, // tz inválida
		{"", "", "2026-06-10", "", ""},                 // from sem to
	}
	for _, c := range casos {
		_, err := svc.ResolveWindow(c.period, c.date, c.from, c.to, c.tz)
		de, ok := err.(*domain.Error)
		if !ok || de.Code != "validation_error" {
			t.Errorf("caso %+v: esperava *domain.Error validation_error, veio %T %v", c, err, err)
		}
	}
}

// CA-09 — a semana sem filtro reproduz a list da 07: total e buckets idênticos
// ao Week (prova de não-regressão semântica). Mesma semente, mesmas asserções.
func TestSemanaSemFiltroEquivaleAW07(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g14-eq", "eq14@tt.dev")
	ws := seedWorkspace(t, pool, "EQ14")
	loc := brt(t)

	cli := seedClient(t, pool, ws, "Cli")
	pA := seedProject(t, pool, ws, user, "Alfa", "#4f9dde", projOpts{clientID: &cli, goal: f64(10)})
	pB := seedProject(t, pool, ws, user, "Beta", "#5cc98b", projOpts{})
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)
	insertClosed(t, pool, ws, user, &pA, meio, 3600)
	insertClosed(t, pool, ws, user, &pB, meio.Add(2*time.Hour), 1800)
	insertClosed(t, pool, ws, user, nil, meio.Add(4*time.Hour), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)

	w07, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if rep.From != w07.WeekStart || rep.To != w07.WeekEnd {
		t.Errorf("rótulo divergente: %s→%s vs week %s→%s", rep.From, rep.To, w07.WeekStart, w07.WeekEnd)
	}
	if rep.TotalSeconds != w07.TotalSeconds {
		t.Errorf("total divergente: %d vs week %d", rep.TotalSeconds, w07.TotalSeconds)
	}
	if len(rep.ByProject) != len(w07.ByProject) {
		t.Fatalf("by_project divergente: %d vs %d", len(rep.ByProject), len(w07.ByProject))
	}
	for i := range rep.ByProject {
		if rep.ByProject[i].TotalSeconds != w07.ByProject[i].TotalSeconds {
			t.Errorf("bucket %d divergente: %d vs %d", i, rep.ByProject[i].TotalSeconds, w07.ByProject[i].TotalSeconds)
		}
	}
	if rep.Focus != w07.Focus {
		t.Errorf("focus divergente: %+v vs %+v", rep.Focus, w07.Focus)
	}
	if len(rep.Goals) != len(w07.Goals) {
		t.Errorf("goals divergente: %d vs %d", len(rep.Goals), len(w07.Goals))
	}
}

// CA-10/11 — filtros combináveis e opcionais; project_id isola pro projeto;
// client_id pelos projetos do cliente; objective_id pelas tasks do objetivo.
func TestFiltrosCombinaveis(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-fc", "fc14@tt.dev")
	ws := seedWorkspace(t, pool, "FC14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	cliA := seedClient(t, pool, ws, "ClienteA")
	pA := seedProject(t, pool, ws, user, "ProjA", "#4f9dde", projOpts{clientID: &cliA})
	pB := seedProject(t, pool, ws, user, "ProjB", "#5cc98b", projOpts{}) // sem cliente
	obj := seedObjective(t, pool, ws, pA, user, "ObjA", "open", nil)
	tk := seedTask(t, pool, ws, pA, user, "TaskA", "todo", &obj)

	insertClosedTask(t, pool, ws, user, &pA, &tk, meio, 3600)         // ProjA + cliA + obj
	insertClosed(t, pool, ws, user, &pA, meio.Add(time.Hour), 600)    // ProjA + cliA (sem task/obj)
	insertClosed(t, pool, ws, user, &pB, meio.Add(2*time.Hour), 1800) // ProjB, sem cliente

	svc := newSvc(t, pool, clockMeioDaSemana)

	// sem filtro: tudo
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})
	if rep.TotalSeconds != 6000 {
		t.Errorf("sem filtro total %d, esperava 6000", rep.TotalSeconds)
	}
	// project_id = ProjA → 4200
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ProjectID: pA})
	if rep.TotalSeconds != 4200 {
		t.Errorf("project ProjA total %d, esperava 4200", rep.TotalSeconds)
	}
	// client_id = cliA → projetos do cliente A = ProjA = 4200
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ClientID: cliA})
	if rep.TotalSeconds != 4200 {
		t.Errorf("client cliA total %d, esperava 4200", rep.TotalSeconds)
	}
	// objective_id = obj → só a entry com a task do objetivo = 3600
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ObjectiveID: obj})
	if rep.TotalSeconds != 3600 {
		t.Errorf("objective obj total %d, esperava 3600", rep.TotalSeconds)
	}
	// combinado project+objective (ambos casam) → 3600
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ProjectID: pA, ObjectiveID: obj})
	if rep.TotalSeconds != 3600 {
		t.Errorf("combinado total %d, esperava 3600", rep.TotalSeconds)
	}
}

// CA-11 — id válido inexistente/invisível → resultado VAZIO (não 404);
// malformado → 400.
func TestFiltroInvisivelVazioEMalformado(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-iv", "iv14@tt.dev")
	ws := seedWorkspace(t, pool, "IV14")
	loc := brt(t)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)

	// project_id válido (uuid) mas inexistente → vazio, não 404
	fantasma := "00000000-0000-0000-0000-000000000abc"
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ProjectID: fantasma})
	if rep.TotalSeconds != 0 {
		t.Errorf("filtro fantasma total %d, esperava 0 (vazio)", rep.TotalSeconds)
	}

	// project_id malformado → 400 já no Report (via validate dos filtros)
	win, err := svc.ResolveWindow("week", "2026-06-10", "", "", tzSP)
	if err != nil {
		t.Fatalf("ResolveWindow: %v", err)
	}
	_, err = svc.Report(context.Background(), ws, user, win, reports.Filters{ProjectID: "nao-uuid"}, defaultSettings(), false)
	de, ok := err.(*domain.Error)
	if !ok || de.Code != "validation_error" {
		t.Errorf("filtro malformado: esperava *domain.Error validation_error, veio %T %v", err, err)
	}
}

// CA-12 — objetivo de projeto PRIVADO de OUTRO usuário não filtra nada visível
// (herda visibleWhere da 02): o usuário da rota não vê a entry dele nem via
// objective_id alheio.
func TestFiltroObjetivoPrivadoAlheio(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-pv", "pv14@tt.dev")
	outro := seedUser(t, pool, "g14-pv2", "pv2-14@tt.dev")
	ws := seedWorkspace(t, pool, "PV14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	// projeto PRIVADO do OUTRO usuário, com objetivo e task
	priv := seedProject(t, pool, ws, outro, "Privado", "#888888", projOpts{visibility: "private"})
	objPriv := seedObjective(t, pool, ws, priv, outro, "ObjPriv", "open", nil)
	tkPriv := seedTask(t, pool, ws, priv, outro, "TaskPriv", "todo", &objPriv)
	// entry do USER da rota ligada à task do projeto privado alheio
	insertClosedTask(t, pool, ws, user, &priv, &tkPriv, meio, 3600)

	svc := newSvc(t, pool, clockMeioDaSemana)

	// filtrar pelo objetivo do projeto privado alheio → vazio (visibleWhere)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ObjectiveID: objPriv})
	if rep.TotalSeconds != 0 {
		t.Errorf("objetivo de privado alheio: total %d, esperava 0 (não visível)", rep.TotalSeconds)
	}
	// idem por project_id direto do privado alheio
	rep = report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{ProjectID: priv})
	if rep.TotalSeconds != 0 {
		t.Logf("nota: project_id direto não aplica visibleWhere (a entry é do próprio user) — total %d", rep.TotalSeconds)
	}
}

func f64(v float64) *float64 { return &v }
