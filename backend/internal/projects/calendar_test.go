package projects_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/projects"
)

func findItem(items []projects.CalendarItem, kind, id string) *projects.CalendarItem {
	for i := range items {
		if items[i].Kind == kind && items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// CA-10/11 — itens datados do projeto por mês: task due, deadline do projeto;
// ?month resolve from/to (último dia do mês). task done ainda aparece.
func TestProjectCalendarItens(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Acme", Color: "#4f9dde", Visibility: "shared", Deadline: ptr("2025-06-30"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// task due em junho (done — ainda aparece — CA-10), task due em junho (doing),
	// task due em JULHO (fora da janela).
	tDone := mkTask(t, pool, ws, p.ID, owner, "done", ptr("2025-06-20"))
	tDoing := mkTask(t, pool, ws, p.ID, owner, "doing", ptr("2025-06-25"))
	mkTask(t, pool, ws, p.ID, owner, "todo", ptr("2025-07-10"))

	cal, err := svc.ProjectCalendar(ctx, ws, owner, p.ID, "2025-06-01", "2025-06-30")
	if err != nil {
		t.Fatalf("calendar: %v", err)
	}
	if findItem(cal, "task", tDone) == nil {
		t.Error("task done deveria aparecer (CA-10)")
	}
	if it := findItem(cal, "task", tDoing); it == nil || it.Date != "2025-06-25" {
		t.Errorf("task doing deveria aparecer em 2025-06-25, veio %v", it)
	}
	// task de julho fora.
	for _, it := range cal {
		if it.Kind == "task" && it.Date == "2025-07-10" {
			t.Error("task de julho NÃO deveria aparecer na janela de junho")
		}
	}
	// deadline do projeto em junho.
	if it := findItem(cal, "project_deadline", p.ID); it == nil || it.Date != "2025-06-30" {
		t.Errorf("project_deadline deveria aparecer em 2025-06-30, veio %v", it)
	}
	// NÃO retorna eventos do GCal (CA-13) — só os 3 kinds.
	for _, it := range cal {
		if it.Kind != "task" && it.Kind != "objective" && it.Kind != "project_deadline" {
			t.Errorf("kind inesperado %q (sem evento de GCal — CA-13)", it.Kind)
		}
	}
}

// CA-11 — ?month resolve from/to corretamente (último dia de fevereiro, 30 e 31).
func TestResolveWindowMonth(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	p, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P", Color: "#4f9dde", Visibility: "shared"})

	// fevereiro 2025 (28 dias).
	from, to, err := svc.ProjectCalendarWindow("2025-02", "", "", "")
	if err != nil {
		t.Fatalf("window fev: %v", err)
	}
	if from != "2025-02-01" || to != "2025-02-28" {
		t.Errorf("fev 2025 deveria ser 02-01..02-28, veio %s..%s", from, to)
	}
	// abril (30 dias).
	from, to, _ = svc.ProjectCalendarWindow("2025-04", "", "", "")
	if from != "2025-04-01" || to != "2025-04-30" {
		t.Errorf("abr 2025 deveria ser 04-01..04-30, veio %s..%s", from, to)
	}
	// dezembro (31 dias).
	from, to, _ = svc.ProjectCalendarWindow("2025-12", "", "", "")
	if from != "2025-12-01" || to != "2025-12-31" {
		t.Errorf("dez 2025 deveria ser 12-01..12-31, veio %s..%s", from, to)
	}
	// from/to explícitos.
	from, to, _ = svc.ProjectCalendarWindow("", "2025-06-05", "2025-06-15", "")
	if from != "2025-06-05" || to != "2025-06-15" {
		t.Errorf("explícito deveria ser 06-05..06-15, veio %s..%s", from, to)
	}
	_ = p
}

// 28-FIND-MWD-1 — branch DEFAULT de ProjectCalendarWindow (sem month, sem
// from/to): (a) tz inválido → validation_error (mata a mutação que engole o erro
// de LoadLocation e cai pra UTC); (b) tz vazio → 1º..último dia do MÊS CORRENTE
// derivado de time.Now() (pure-function, determinístico via cálculo espelhado).
func TestProjectCalendarWindowDefault(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ctx := context.Background()
	_ = ctx

	// (a) tz inválido no branch default → validation_error (não engole pra UTC).
	_, _, err := svc.ProjectCalendarWindow("", "", "", "Nao/Existe")
	assertDomainCode(t, err, "validation_error", "default tz inválido")

	// (b) tz vazio → mês corrente em UTC (default loc). Calcula o esperado com a
	// MESMA lógica de calendar.go (time.Date(year, month, 1...) + AddDate).
	now := time.Now().In(time.UTC)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, 0).AddDate(0, 0, -1)
	wantFrom := first.Format("2006-01-02")
	wantTo := last.Format("2006-01-02")
	from, to, err := svc.ProjectCalendarWindow("", "", "", "")
	if err != nil {
		t.Fatalf("default window: %v", err)
	}
	if from != wantFrom || to != wantTo {
		t.Errorf("default deveria ser %s..%s (mês corrente UTC), veio %s..%s", wantFrom, wantTo, from, to)
	}
}

// CA-11 — month/from/to malformados ou to<from → 400.
func TestProjectCalendarJanelaInvalida(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	_ = context.Background()

	for _, c := range []struct{ month, from, to string }{
		{"2025-13", "", ""},              // mês inválido
		{"2025-6", "", ""},               // formato curto
		{"naodata", "", ""},              // não-data
		{"", "2025-06-30", "2025-06-01"}, // to < from
		{"", "ontem", "hoje"},            // from/to não-data
	} {
		_, _, err := svc.ProjectCalendarWindow(c.month, c.from, c.to, "")
		assertDomainCode(t, err, "validation_error", "Window inválida("+c.month+c.from+")")
	}
}

// CA-09/12 — projeto invisível/inexistente/projectId malformado → 404 (não 500),
// não vaza itens de outro projeto/workspace.
func TestProjectCalendarVisibilidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	outro := seedUser(t, pool, "g-outro")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, outro, "member")
	ctx := context.Background()

	// projectId malformado → 404.
	_, err := svc.ProjectCalendar(ctx, ws, owner, "nao-uuid", "2025-06-01", "2025-06-30")
	assertDomainCode(t, err, "not_found", "calendar projectId malformado")

	// inexistente → 404.
	_, err = svc.ProjectCalendar(ctx, ws, owner, "00000000-0000-0000-0000-000000000000", "2025-06-01", "2025-06-30")
	assertDomainCode(t, err, "not_found", "calendar projeto inexistente")

	// private do "outro" — owner não vê → 404.
	priv, _ := svc.Create(ctx, ws, outro, projects.CreateInput{Name: "Priv", Color: "#4f9dde", Visibility: "private"})
	mkTask(t, pool, ws, priv.ID, outro, "todo", ptr("2025-06-10"))
	_, err = svc.ProjectCalendar(ctx, ws, owner, priv.ID, "2025-06-01", "2025-06-30")
	assertDomainCode(t, err, "not_found", "calendar projeto invisível")

	// projeto de OUTRO workspace → 404.
	wsB := seedWorkspace(t, pool)
	pB, _ := svc.Create(ctx, wsB, owner, projects.CreateInput{Name: "B", Color: "#4f9dde", Visibility: "shared"})
	_, err = svc.ProjectCalendar(ctx, ws, owner, pB.ID, "2025-06-01", "2025-06-30")
	assertDomainCode(t, err, "not_found", "calendar projeto de outro ws")
}

// CA-09 — NÃO vaza item de outro projeto (filtro project_id na query de tasks).
func TestProjectCalendarNaoVazaOutroProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	p1, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P1", Color: "#4f9dde", Visibility: "shared"})
	p2, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P2", Color: "#4f9dde", Visibility: "shared"})
	tOutro := mkTask(t, pool, ws, p2.ID, owner, "todo", ptr("2025-06-15"))
	mkTask(t, pool, ws, p1.ID, owner, "todo", ptr("2025-06-10"))

	cal, err := svc.ProjectCalendar(ctx, ws, owner, p1.ID, "2025-06-01", "2025-06-30")
	if err != nil {
		t.Fatalf("calendar: %v", err)
	}
	if findItem(cal, "task", tOutro) != nil {
		t.Error("task do projeto P2 NÃO deveria vazar no calendário de P1")
	}
}

// CA-15.1 + 29-H8/CA-11 — kind:"objective" aparece pra objetivos by_date com
// target_date ∈ janela (a coluna agora existe sempre — migração 0026 da fase 29).
// Um objetivo NÃO-by_date com target_date residual (marco dormente, CA-04.1) NÃO
// vira marco: a query (c) gateia por completion_mode='by_date'. Mutação que
// remove o filtro `completion_mode='by_date'` faz o marco dormente vazar → quebra.
func TestProjectCalendarObjectiveTargetDate(t *testing.T) {
	pool := dbtest.NewPool(t)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	svc := projects.NewService(pool)
	p, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P", Color: "#4f9dde", Visibility: "shared"})

	// objetivo by_date com target_date ∈ janela → vira marco (CA-11).
	var byDateID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by, completion_mode, target_date)
		 VALUES ($1, $2, 'Roadmap Q3', $3, 'by_date', '2025-06-30') RETURNING id`,
		ws, p.ID, owner).Scan(&byDateID); err != nil {
		t.Fatalf("insert objective by_date: %v", err)
	}
	// objetivo by_tasks com target_date residual (marco dormente — CA-04.1): NÃO vira marco.
	var dormID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO objectives (workspace_id, project_id, name, created_by, completion_mode, target_date)
		 VALUES ($1, $2, 'Dormente', $3, 'by_tasks', '2025-06-20') RETURNING id`,
		ws, p.ID, owner).Scan(&dormID); err != nil {
		t.Fatalf("insert objective dormente: %v", err)
	}

	cal, err := svc.ProjectCalendar(ctx, ws, owner, p.ID, "2025-06-01", "2025-06-30")
	if err != nil {
		t.Fatalf("calendar: %v", err)
	}
	if it := findItem(cal, "objective", byDateID); it == nil || it.Date != "2025-06-30" {
		t.Errorf("objetivo by_date deveria virar marco em 2025-06-30, veio %v", it)
	}
	if it := findItem(cal, "objective", dormID); it != nil {
		t.Errorf("CA-04.1/29-H8: marco dormente (by_tasks com target_date residual) NÃO deveria aparecer, veio %v", it)
	}
}
