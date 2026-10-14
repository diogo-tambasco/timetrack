// objectives_test.go — T-02 da fase 14: seção objectives (progresso por
// ENTREGA, CA-16/17/18/19). Reusa os helpers de service_test.go e period_test.go
// (seedObjective/seedTask/insertClosedTask). A contagem de tasks é CUMULATIVA
// (sem janela — CA-17); só realized_seconds usa o período.
//
// Red→green: objectivesSection nasceu no T-01 (a orquestração do Report a chama),
// mas SEM cobertura dos grãos de entrega/visibilidade. Estes casos pinam os
// comportamentos novos (cumulativa×período, status manual, Geral≠objetivo,
// isolamento). Mutações verificadas: trocar a q-tasks por uma janelada faz
// TestObjectivesContagemCumulativa quebrar; remover o visibleWhere da q-obj faz
// TestObjectivesIsolamentoEVisibilidade quebrar.
package reports_test

import (
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reports"
)

// objByID acha o item de objetivo na seção (helper).
func objByID(items []reports.ObjectiveItem, id string) *reports.ObjectiveItem {
	for i := range items {
		if items[i].ObjectiveID == id {
			return &items[i]
		}
	}
	return nil
}

// CA-16 — shape completo: project_*, objective_*, status, contagens, progress,
// realized, goal. Objetivo com 2 tasks (1 done) e meta → progress 50%, goal.
func TestObjectivesShape(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-os", "os14@tt.dev")
	ws := seedWorkspace(t, pool, "OS14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	proj := seedProject(t, pool, ws, user, "Lauda", "#b07cd6", projOpts{})
	obj := seedObjective(t, pool, ws, proj, user, "Lançar v1", "open", f64(2)) // meta 2h = 7200s
	tDone := seedTask(t, pool, ws, proj, user, "t1", "done", &obj)
	_ = seedTask(t, pool, ws, proj, user, "t2", "todo", &obj)
	insertClosedTask(t, pool, ws, user, &proj, &tDone, meio, 1800) // realized 1800

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if len(rep.Objectives) != 1 {
		t.Fatalf("esperava 1 objetivo, veio %d: %+v", len(rep.Objectives), rep.Objectives)
	}
	o := rep.Objectives[0]
	if o.ProjectID != proj || o.ProjectName != "Lauda" || o.ProjectColor != "#b07cd6" {
		t.Errorf("projeto denormalizado errado: %+v", o)
	}
	if o.ObjectiveID != obj || o.ObjectiveName != "Lançar v1" || o.Status != "open" {
		t.Errorf("objetivo errado: %+v", o)
	}
	if o.TasksTotal != 2 || o.TasksDone != 1 || o.ProgressPct != 50 {
		t.Errorf("progresso errado: total %d done %d pct %d, esperava 2/1/50", o.TasksTotal, o.TasksDone, o.ProgressPct)
	}
	if o.RealizedSeconds != 1800 {
		t.Errorf("realized %d, esperava 1800", o.RealizedSeconds)
	}
	if o.GoalSeconds != 7200 {
		t.Errorf("goal %d, esperava 7200", o.GoalSeconds)
	}
}

// CA-17 — contagem CUMULATIVA × tempo do período: uma task done FORA do
// período conta na entrega (cumulativa); o tempo do período só conta a entry
// DENTRO. Pina os dois grãos.
func TestObjectivesContagemCumulativaVsTempoPeriodo(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-cum", "cum14@tt.dev")
	ws := seedWorkspace(t, pool, "CUM14")
	loc := brt(t)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	obj := seedObjective(t, pool, ws, proj, user, "Obj", "open", nil)
	tDoneFora := seedTask(t, pool, ws, proj, user, "doneFora", "done", &obj)
	tDentro := seedTask(t, pool, ws, proj, user, "dentro", "todo", &obj)

	// entry da task done FORA do período (mês anterior): conta na entrega,
	// NÃO no realized da semana 08–14.
	insertClosedTask(t, pool, ws, user, &proj, &tDoneFora, time.Date(2026, 5, 20, 9, 0, 0, 0, loc), 3600)
	// entry da task de dentro, na semana: conta no realized
	insertClosedTask(t, pool, ws, user, &proj, &tDentro, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil {
		t.Fatalf("objetivo não veio: %+v", rep.Objectives)
	}
	// entrega cumulativa: 2 tasks, 1 done (a de fora também conta na contagem)
	if o.TasksTotal != 2 || o.TasksDone != 1 || o.ProgressPct != 50 {
		t.Errorf("entrega: total %d done %d pct %d, esperava 2/1/50 (cumulativa)", o.TasksTotal, o.TasksDone, o.ProgressPct)
	}
	// tempo do período: só a entry DENTRO (600); a de fora (3600) não conta
	if o.RealizedSeconds != 600 {
		t.Errorf("realized %d, esperava 600 (só a entry da semana)", o.RealizedSeconds)
	}
}

// CA-18 — status='done' com <100% vem marcado done (fechamento manual); 100%
// de tasks done mas status='open' vem com pct=100 e status=open (reporta, não
// conclui).
func TestObjectivesStatusManual(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-st", "st14@tt.dev")
	ws := seedWorkspace(t, pool, "ST14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})

	// objetivo A: status=done, mas só 1 de 2 tasks done (<100%)
	objDone := seedObjective(t, pool, ws, proj, user, "A-done-manual", "done", nil)
	tA := seedTask(t, pool, ws, proj, user, "a1", "done", &objDone)
	_ = seedTask(t, pool, ws, proj, user, "a2", "todo", &objDone)
	insertClosedTask(t, pool, ws, user, &proj, &tA, meio, 600) // atividade pra entrar

	// objetivo B: status=open, mas 100% das tasks done
	objOpen := seedObjective(t, pool, ws, proj, user, "B-open-cheio", "open", nil)
	tB := seedTask(t, pool, ws, proj, user, "b1", "done", &objOpen)
	insertClosedTask(t, pool, ws, user, &proj, &tB, meio.Add(time.Hour), 600)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	a := objByID(rep.Objectives, objDone)
	if a == nil || a.Status != "done" || a.ProgressPct != 50 {
		t.Errorf("A: esperava status=done pct=50, veio %+v", a)
	}
	b := objByID(rep.Objectives, objOpen)
	if b == nil || b.Status != "open" || b.ProgressPct != 100 {
		t.Errorf("B: esperava status=open pct=100, veio %+v", b)
	}
}

// CA-19 — tasks SEM objetivo não viram item objectives (mas o tempo segue em
// by_project). O bucket "Geral" não vira objetivo-fantasma.
func TestObjectivesSemObjetivoNaoViraItem(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-ge", "ge14@tt.dev")
	ws := seedWorkspace(t, pool, "GE14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	proj := seedProject(t, pool, ws, user, "Proj", "#4f9dde", projOpts{})
	// task SEM objetivo + entry
	tGeral := seedTask(t, pool, ws, proj, user, "geral", "todo", nil)
	insertClosedTask(t, pool, ws, user, &proj, &tGeral, meio, 1200)
	// entry sem nenhuma task no mesmo projeto
	insertClosed(t, pool, ws, user, &proj, meio.Add(time.Hour), 300)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	if len(rep.Objectives) != 0 {
		t.Errorf("esperava 0 objetivos (só Geral), veio %d: %+v", len(rep.Objectives), rep.Objectives)
	}
	// o tempo segue contando em by_project (1200 + 300 = 1500)
	if rep.TotalSeconds != 1500 {
		t.Errorf("total %d, esperava 1500 (tempo do Geral segue contando)", rep.TotalSeconds)
	}
}

// CA-16/CA-01 — realized_seconds só do USUÁRIO; objetivo de projeto privado
// alheio NÃO aparece (visibleWhere).
func TestObjectivesIsolamentoEVisibilidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-oi", "oi14@tt.dev")
	outro := seedUser(t, pool, "g14-oi2", "oi2-14@tt.dev")
	ws := seedWorkspace(t, pool, "OI14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	// projeto SHARED com objetivo: realized do user vs do outro
	proj := seedProject(t, pool, ws, user, "Shared", "#4f9dde", projOpts{})
	obj := seedObjective(t, pool, ws, proj, user, "Compartilhado", "open", nil)
	tk := seedTask(t, pool, ws, proj, user, "t", "todo", &obj)
	insertClosedTask(t, pool, ws, user, &proj, &tk, meio, 600)                  // do user → conta
	insertClosedTask(t, pool, ws, outro, &proj, &tk, meio.Add(time.Hour), 9999) // do outro → fora

	// projeto PRIVADO do outro com objetivo (não deve aparecer)
	priv := seedProject(t, pool, ws, outro, "Privado", "#888888", projOpts{visibility: "private"})
	objPriv := seedObjective(t, pool, ws, priv, outro, "Secreto", "open", f64(5)) // tem meta

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})

	o := objByID(rep.Objectives, obj)
	if o == nil || o.RealizedSeconds != 600 {
		t.Errorf("realized do user: esperava 600 (sem o do outro), veio %+v", o)
	}
	if objByID(rep.Objectives, objPriv) != nil {
		t.Errorf("objetivo de projeto privado alheio vazou na seção: %+v", rep.Objectives)
	}
}

// CA-16.1 — um filtro ativo (project_id/objective_id/client_id) restringe a
// LISTA de objetivos: um objetivo de OUTRO projeto que só "tem meta" (entra pelo
// ramo "OU têm meta") NÃO pode vazar sob o filtro. Furo confirmado na
// verificação do build (sem este teste, neutralizar os predicados de filtro da
// q-obj em objectives.go deixava a suíte VERDE). Cobre os 3 ramos de filtro.
func TestObjectivesFiltroRestringeLista(t *testing.T) {
	pool := dbtest.NewPool(t)
	user := seedUser(t, pool, "g14-ofl", "ofl14@tt.dev")
	ws := seedWorkspace(t, pool, "OFL14")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	// projA (sob clienteA): objetivo ATIVO no período (entra por atividade)
	cliA := seedClient(t, pool, ws, "ClienteA")
	projA := seedProject(t, pool, ws, user, "ProjA", "#4f9dde", projOpts{clientID: &cliA})
	objA := seedObjective(t, pool, ws, projA, user, "ObjA", "open", nil)
	tkA := seedTask(t, pool, ws, projA, user, "tA", "todo", &objA)
	insertClosedTask(t, pool, ws, user, &projA, &tkA, meio, 1800)

	// projB (SEM cliente): objetivo só com META, ZERO atividade (entra só pelo
	// ramo "OU têm meta") — é o que NÃO pode vazar sob um filtro de projA
	projB := seedProject(t, pool, ws, user, "ProjB", "#5cc98b", projOpts{})
	objB := seedObjective(t, pool, ws, projB, user, "ObjB-meta", "open", f64(5)) // meta, sem entries

	svc := newSvc(t, pool, clockMeioDaSemana)

	// baseline (sem filtro): os DOIS aparecem (objB pelo ramo "OU têm meta")
	base := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, reports.Filters{})
	if objByID(base.Objectives, objA) == nil || objByID(base.Objectives, objB) == nil {
		t.Fatalf("baseline sem filtro deveria listar objA E objB (meta): %+v", base.Objectives)
	}

	// cada filtro de projA restringe a lista: objA presente, objB-meta FORA
	for _, f := range []struct {
		nome string
		flt  reports.Filters
	}{
		{"project_id", reports.Filters{ProjectID: projA}},
		{"objective_id", reports.Filters{ObjectiveID: objA}},
		{"client_id", reports.Filters{ClientID: cliA}},
	} {
		rep := report(t, svc, ws, user, "week", "2026-06-10", "", "", tzSP, f.flt)
		if objByID(rep.Objectives, objA) == nil {
			t.Errorf("filtro %s: objA deveria estar na lista, veio %+v", f.nome, rep.Objectives)
		}
		if objByID(rep.Objectives, objB) != nil {
			t.Errorf("filtro %s: objB-meta de OUTRO projeto vazou na lista: %+v", f.nome, rep.Objectives)
		}
	}
}
