// service_test.go — T-01 da fase 07: janela da semana (CA-06/06-defaults/07)
// e ângulo 1 (CA-09/10/11) + invariantes (CA-01 isolamento, CA-04 rodando
// fora). dbtest + entries.Insert pra semear; clock injetável (WithClock).
package reports_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/reports"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $2, $2) RETURNING id`,
		googleID, email).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return id
}

func seedWorkspace(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed workspace %s: %v", name, err)
	}
	return id
}

func seedClient(t *testing.T, pool *pgxpool.Pool, wsID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO clients (workspace_id, name) VALUES ($1, $2) RETURNING id`,
		wsID, name).Scan(&id); err != nil {
		t.Fatalf("seed client %s: %v", name, err)
	}
	return id
}

type projOpts struct {
	clientID   *string
	goal       *float64
	visibility string // default shared
	archived   bool
}

func seedProject(t *testing.T, pool *pgxpool.Pool, wsID, ownerID, name, color string, o projOpts) string {
	t.Helper()
	vis := o.visibility
	if vis == "" {
		vis = "shared"
	}
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal,
		                      visibility, owner_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		wsID, o.clientID, name, color, o.goal, vis, ownerID).Scan(&id); err != nil {
		t.Fatalf("seed project %s: %v", name, err)
	}
	if o.archived {
		if _, err := pool.Exec(context.Background(),
			`UPDATE projects SET archived_at = now() WHERE id = $1`, id); err != nil {
			t.Fatalf("arquivar projeto %s: %v", name, err)
		}
	}
	return id
}

// insertClosed semeia uma entry FECHADA com a duração dada.
func insertClosed(t *testing.T, pool *pgxpool.Pool, wsID, userID string, projectID *string, start time.Time, durSec int) entries.Entry {
	t.Helper()
	stop := start.Add(time.Duration(durSec) * time.Second)
	e, err := entries.Insert(context.Background(), pool, entries.Entry{
		WorkspaceID: wsID, UserID: userID, ProjectID: projectID,
		Description: "seed", Tags: []string{}, StartedAt: start,
		StoppedAt: &stop, DurationSeconds: &durSec, Source: "manual",
	})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	return e
}

// insertRunning semeia uma entry RODANDO (sem stopped_at) — CA-04.
func insertRunning(t *testing.T, pool *pgxpool.Pool, wsID, userID string, start time.Time) {
	t.Helper()
	if _, err := entries.Insert(context.Background(), pool, entries.Entry{
		WorkspaceID: wsID, UserID: userID, Description: "rodando",
		Tags: []string{}, StartedAt: start, Source: "timer",
	}); err != nil {
		t.Fatalf("seed entry rodando: %v", err)
	}
}

const tzSP = "America/Sao_Paulo"

// quarta 12:00 UTC, meio da semana 2026-06-08 → 2026-06-14 (em SP e em UTC)
var clockMeioDaSemana = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func newSvc(t *testing.T, pool *pgxpool.Pool, clock time.Time) *reports.Service {
	t.Helper()
	return reports.NewService(pool, reports.WithClock(func() time.Time { return clock }))
}

func brt(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(tzSP)
	if err != nil {
		t.Fatalf("tz: %v", err)
	}
	return loc
}

// CA-06 — semana segunda→domingo que contém date no fuso: o MESMO
// week_start pra date no meio da semana, numa segunda e num domingo; e a
// janela [seg 00:00, seg+7d) no fuso comprovada por entries nas bordas.
func TestJanelaSemanaSegundaADomingo(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-w", "w@tt.dev")
	ws := seedWorkspace(t, pool, "W")
	loc := brt(t)

	// bordas da semana 08–14 em SP: dentro, dentro, fora (seg seguinte)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 8, 0, 0, 0, 0, loc), 600)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 14, 23, 30, 0, 0, loc), 900)
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 15, 0, 0, 0, 0, loc), 1200)

	svc := newSvc(t, pool, clockMeioDaSemana)
	for _, date := range []string{"2026-06-10", "2026-06-08", "2026-06-14"} {
		rep, err := svc.Week(ctx, ws, user, date, tzSP)
		if err != nil {
			t.Fatalf("Week(%s): %v", date, err)
		}
		if rep.WeekStart != "2026-06-08" || rep.WeekEnd != "2026-06-14" {
			t.Errorf("date %s: semana veio %s→%s, esperava 2026-06-08→2026-06-14",
				date, rep.WeekStart, rep.WeekEnd)
		}
		if rep.TotalSeconds != 1500 { // 600 + 900; a de seg seguinte FORA
			t.Errorf("date %s: total %d, esperava 1500", date, rep.TotalSeconds)
		}
	}
}

// CA-06 (defaults — verificação adversarial das specs): date ausente →
// HOJE NO FUSO (clock que diverge UTC×SP); tz ausente → UTC.
func TestJanelaDefaults(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-d", "d@tt.dev")
	ws := seedWorkspace(t, pool, "D")

	// seg 2026-06-08 01:00 UTC = DOMINGO 2026-06-07 22:00 em SP
	svc := newSvc(t, pool, time.Date(2026, 6, 8, 1, 0, 0, 0, time.UTC))

	// date vazio + tz SP → "hoje" é domingo 07/06 em SP → semana que FECHA
	rep, err := svc.Week(ctx, ws, user, "", tzSP)
	if err != nil {
		t.Fatalf("Week default SP: %v", err)
	}
	if rep.WeekStart != "2026-06-01" || rep.WeekEnd != "2026-06-07" {
		t.Errorf("default em SP: %s→%s, esperava 2026-06-01→2026-06-07 (mata s.now() sem .In(loc))",
			rep.WeekStart, rep.WeekEnd)
	}

	// date e tz vazios → hoje em UTC (segunda 08/06) → semana seguinte
	rep, err = svc.Week(ctx, ws, user, "", "")
	if err != nil {
		t.Fatalf("Week default UTC: %v", err)
	}
	if rep.WeekStart != "2026-06-08" || rep.WeekEnd != "2026-06-14" {
		t.Errorf("default em UTC: %s→%s, esperava 2026-06-08→2026-06-14",
			rep.WeekStart, rep.WeekEnd)
	}
}

// CA-07 — recorte por started_at NO FUSO: domingo 23:30 em SP (= 02:30 UTC
// de segunda) pertence à semana que FECHA; segunda 00:30 em SP à seguinte.
func TestBordaDeFusoNaSemana(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-b", "b@tt.dev")
	ws := seedWorkspace(t, pool, "B")
	loc := brt(t)

	// domingo 14/06 23:30 SP = 15/06 02:30 UTC
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 14, 23, 30, 0, 0, loc), 600)
	// segunda 15/06 00:30 SP
	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 15, 0, 30, 0, 0, loc), 900)

	svc := newSvc(t, pool, clockMeioDaSemana)

	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.TotalSeconds != 600 {
		t.Errorf("semana 08–14 em SP: total %d, esperava 600 (só o domingo 23:30)", rep.TotalSeconds)
	}

	rep, err = svc.Week(ctx, ws, user, "2026-06-15", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.TotalSeconds != 900 {
		t.Errorf("semana 15–21 em SP: total %d, esperava 900 (só a segunda 00:30)", rep.TotalSeconds)
	}
}

// CA-09/CA-10 — buckets por projeto ordenados por total DESC (empate: name
// ASC; bucket nulo por último), com name/color/client resolvidos; soma dos
// buckets ≡ total_seconds; sem-projeto vira bucket project_id null.
func TestBucketsPorProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-p", "p@tt.dev")
	ws := seedWorkspace(t, pool, "P")
	loc := brt(t)

	cliente := seedClient(t, pool, ws, "Lauda.ai")
	pLauda := seedProject(t, pool, ws, user, "Lauda", "#b07cd6", projOpts{clientID: &cliente})
	pAlfa := seedProject(t, pool, ws, user, "Alfa", "#4f9dde", projOpts{})
	pBeta := seedProject(t, pool, ws, user, "Beta", "#5cc98b", projOpts{})

	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)
	insertClosed(t, pool, ws, user, &pLauda, meio, 3600)
	insertClosed(t, pool, ws, user, &pLauda, meio.Add(2*time.Hour), 1800) // Lauda: 5400
	insertClosed(t, pool, ws, user, &pAlfa, meio.Add(4*time.Hour), 1200)  // empate 1200
	insertClosed(t, pool, ws, user, &pBeta, meio.Add(5*time.Hour), 1200)  // empate 1200
	insertClosed(t, pool, ws, user, nil, meio.Add(6*time.Hour), 600)      // sem projeto

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}

	if rep.TotalSeconds != 5400+1200+1200+600 {
		t.Fatalf("total %d, esperava %d", rep.TotalSeconds, 5400+1200+1200+600)
	}
	soma := 0
	for _, b := range rep.ByProject {
		soma += b.TotalSeconds
	}
	if soma != rep.TotalSeconds {
		t.Errorf("soma dos buckets %d ≠ total %d", soma, rep.TotalSeconds)
	}

	if len(rep.ByProject) != 4 {
		t.Fatalf("esperava 4 buckets, veio %d", len(rep.ByProject))
	}
	// ordem: Lauda 5400 · Alfa 1200 (empate, name ASC) · Beta 1200 · null 600
	b0, b1, b2, b3 := rep.ByProject[0], rep.ByProject[1], rep.ByProject[2], rep.ByProject[3]
	if b0.ProjectID == nil || *b0.ProjectID != pLauda || b0.TotalSeconds != 5400 {
		t.Errorf("bucket[0] esperava Lauda 5400, veio %+v", b0)
	}
	if b0.Name == nil || *b0.Name != "Lauda" || b0.Color == nil || *b0.Color != "#b07cd6" {
		t.Errorf("bucket[0] sem name/color resolvidos: %+v", b0)
	}
	if b0.ClientID == nil || *b0.ClientID != cliente || b0.ClientName == nil || *b0.ClientName != "Lauda.ai" {
		t.Errorf("bucket[0] sem client resolvido: %+v", b0)
	}
	if b1.Name == nil || *b1.Name != "Alfa" || b2.Name == nil || *b2.Name != "Beta" {
		t.Errorf("empate não ordenou por name ASC: [1]=%+v [2]=%+v", b1, b2)
	}
	if b3.ProjectID != nil || b3.Name != nil || b3.Color != nil || b3.ClientName != nil {
		t.Errorf("bucket nulo deveria ter campos nulos: %+v", b3)
	}
	if b3.TotalSeconds != 600 {
		t.Errorf("bucket nulo esperava 600s, veio %d", b3.TotalSeconds)
	}
}

// CA-11 — projeto ARQUIVADO com tempo na semana aparece nomeado (o
// arquivamento não quebra vínculo — CA-05 da fase 02).
func TestProjetoArquivadoNomeado(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-a", "a@tt.dev")
	ws := seedWorkspace(t, pool, "A")
	loc := brt(t)

	arq := seedProject(t, pool, ws, user, "Arquivado Vivo", "#e0a13c", projOpts{archived: true})
	insertClosed(t, pool, ws, user, &arq, time.Date(2026, 6, 9, 10, 0, 0, 0, loc), 2400)

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if len(rep.ByProject) != 1 {
		t.Fatalf("esperava 1 bucket, veio %d", len(rep.ByProject))
	}
	b := rep.ByProject[0]
	if b.Name == nil || *b.Name != "Arquivado Vivo" || b.Color == nil || *b.Color != "#e0a13c" {
		t.Errorf("projeto arquivado sem name/color: %+v", b)
	}
}

// CA-01 — só entries DO USUÁRIO no workspace DA ROTA: outro membro do
// MESMO ws fora; outro ws do MESMO usuário fora.
func TestIsolamentoWorkspaceEUsuario(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-i1", "i1@tt.dev")
	outro := seedUser(t, pool, "g-i2", "i2@tt.dev")
	ws := seedWorkspace(t, pool, "I")
	wsB := seedWorkspace(t, pool, "IB")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 9, 0, 0, 0, loc)

	insertClosed(t, pool, ws, user, nil, meio, 600)                    // ENTRA
	insertClosed(t, pool, ws, outro, nil, meio.Add(time.Hour), 1800)   // outro usuário, mesmo ws: FORA
	insertClosed(t, pool, wsB, user, nil, meio.Add(2*time.Hour), 2400) // mesmo usuário, outro ws: FORA

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.TotalSeconds != 600 {
		t.Errorf("total %d, esperava 600 (vazou entry de outro usuário/ws)", rep.TotalSeconds)
	}
}

// CA-04 — entry RODANDO fica fora de todas as agregações.
func TestEntryRodandoFora(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-r", "r@tt.dev")
	ws := seedWorkspace(t, pool, "R")
	loc := brt(t)

	insertClosed(t, pool, ws, user, nil, time.Date(2026, 6, 10, 9, 0, 0, 0, loc), 600)
	insertRunning(t, pool, ws, user, time.Date(2026, 6, 10, 11, 0, 0, 0, loc))

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.TotalSeconds != 600 {
		t.Errorf("total %d, esperava 600 (entry rodando entrou na agregação)", rep.TotalSeconds)
	}
	if len(rep.ByProject) != 1 {
		t.Errorf("esperava 1 bucket, veio %d", len(rep.ByProject))
	}
}

// ───────────────────────── T-02: ângulo 2 (focus) ─────────────────────────

// CA-12 — limiar INCLUSIVO de 25min: 1499s é fragmento, 1500s é foco;
// somas e contagens exatas dos dois lados.
func TestFocusLimiar(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-f1", "f1@tt.dev")
	ws := seedWorkspace(t, pool, "F1")
	loc := brt(t)
	meio := time.Date(2026, 6, 10, 8, 0, 0, 0, loc)

	insertClosed(t, pool, ws, user, nil, meio, 1499)                  // fragmento (borda)
	insertClosed(t, pool, ws, user, nil, meio.Add(1*time.Hour), 1500) // foco (borda inclusiva)
	insertClosed(t, pool, ws, user, nil, meio.Add(2*time.Hour), 3600) // foco
	insertClosed(t, pool, ws, user, nil, meio.Add(4*time.Hour), 200)  // fragmento

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	f := rep.Focus
	if f.FocusSeconds != 5100 || f.FocusCount != 2 {
		t.Errorf("foco: %d s em %d blocos, esperava 5100 em 2", f.FocusSeconds, f.FocusCount)
	}
	if f.FragmentSeconds != 1699 || f.FragmentCount != 2 {
		t.Errorf("fragmentos: %d s em %d, esperava 1699 em 2", f.FragmentSeconds, f.FragmentCount)
	}
}

// CA-13 — longest_seconds = maior entry da semana; 0 em semana vazia (e
// nenhum NaN/contagem fantasma nas demais métricas).
func TestLongestEZeroEmSemanaVazia(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-f2", "f2@tt.dev")
	ws := seedWorkspace(t, pool, "F2")
	loc := brt(t)

	svc := newSvc(t, pool, clockMeioDaSemana)

	// semana vazia
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.Focus.LongestSeconds != 0 || rep.Focus.Switches != 0 ||
		rep.Focus.FocusCount != 0 || rep.Focus.FragmentCount != 0 {
		t.Errorf("semana vazia deveria zerar focus, veio %+v", rep.Focus)
	}
	if rep.TotalSeconds != 0 || len(rep.ByProject) != 0 {
		t.Errorf("semana vazia: total %d, buckets %d", rep.TotalSeconds, len(rep.ByProject))
	}

	// com entries: longest é a maior
	meio := time.Date(2026, 6, 10, 8, 0, 0, 0, loc)
	insertClosed(t, pool, ws, user, nil, meio, 1200)
	insertClosed(t, pool, ws, user, nil, meio.Add(time.Hour), 5400)
	insertClosed(t, pool, ws, user, nil, meio.Add(3*time.Hour), 900)

	rep, err = svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if rep.Focus.LongestSeconds != 5400 {
		t.Errorf("longest %d, esperava 5400", rep.Focus.LongestSeconds)
	}
}

// CA-14 — switches: trocas de projeto entre entries CONSECUTIVAS do MESMO
// dia local (nil conta como projeto distinto); primeira entry do dia não
// conta; mesma sequência = 0.
func TestSwitches(t *testing.T) {
	ctx := context.Background()
	loc := brt(t)
	svc := func(pool *pgxpool.Pool) *reports.Service { return newSvc(t, pool, clockMeioDaSemana) }

	caso := func(nome string, want int, seed func(ws, user, pA, pB string, p *pgxpool.Pool)) {
		t.Run(nome, func(t *testing.T) {
			p := dbtest.NewPool(t)
			user := seedUser(t, p, "g-sw-"+nome, nome+"@tt.dev")
			ws := seedWorkspace(t, p, "SW")
			pA := seedProject(t, p, ws, user, "A", "#4f9dde", projOpts{})
			pB := seedProject(t, p, ws, user, "B", "#5cc98b", projOpts{})
			seed(ws, user, pA, pB, p)
			rep, err := svc(p).Week(ctx, ws, user, "2026-06-10", tzSP)
			if err != nil {
				t.Fatalf("Week: %v", err)
			}
			if rep.Focus.Switches != want {
				t.Errorf("switches %d, esperava %d", rep.Focus.Switches, want)
			}
		})
	}

	dia := func(d, h int) time.Time { return time.Date(2026, 6, d, h, 0, 0, 0, loc) }

	// mesmo dia A→B→A = 2
	caso("ABA", 2, func(ws, user, pA, pB string, p *pgxpool.Pool) {
		insertClosed(t, p, ws, user, &pA, dia(10, 8), 600)
		insertClosed(t, p, ws, user, &pB, dia(10, 10), 600)
		insertClosed(t, p, ws, user, &pA, dia(10, 12), 600)
	})
	// A→null→A = 2 (largar o projeto pra tarefa avulsa É troca)
	caso("AnullA", 2, func(ws, user, pA, _ string, p *pgxpool.Pool) {
		insertClosed(t, p, ws, user, &pA, dia(10, 8), 600)
		insertClosed(t, p, ws, user, nil, dia(10, 10), 600)
		insertClosed(t, p, ws, user, &pA, dia(10, 12), 600)
	})
	// dias diferentes: A (qua) | B (qui) = 0 (não há troca entre dias)
	caso("diasDiferentes", 0, func(ws, user, pA, pB string, p *pgxpool.Pool) {
		insertClosed(t, p, ws, user, &pA, dia(10, 8), 600)
		insertClosed(t, p, ws, user, &pB, dia(11, 8), 600)
	})
	// mesma sequência de projeto = 0
	caso("mesmoProjeto", 0, func(ws, user, pA, _ string, p *pgxpool.Pool) {
		insertClosed(t, p, ws, user, &pA, dia(10, 8), 600)
		insertClosed(t, p, ws, user, &pA, dia(10, 10), 600)
		insertClosed(t, p, ws, user, &pA, dia(10, 12), 600)
	})
	// empate de started_at: C → {A,B} no MESMO instante = 2 trocas QUALQUER
	// que seja a ordem dos empatados — a contagem é invariante por
	// construção (pinar a ordem id-a-id não é determinístico nem com o
	// ORDER BY: registro da verificação adversarial do build)
	caso("empate", 2, func(ws, user, pA, pB string, p *pgxpool.Pool) {
		pC := seedProject(t, p, ws, user, "C", "#e0a13c", projOpts{})
		insertClosed(t, p, ws, user, &pC, dia(10, 8), 600)
		insertClosed(t, p, ws, user, &pA, dia(10, 10), 600)
		insertClosed(t, p, ws, user, &pB, dia(10, 10), 600)
	})
}

// ───────────────────────── T-03: ângulo 3 (goals) ─────────────────────────

// CA-15 — goals lista projetos com meta, NÃO-arquivados e VISÍVEIS
// (visibleWhere da 02): 0-realizado ENTRA; arquivado FORA; private alheio
// FORA; private próprio ENTRA; realized só da semana e só do usuário.
func TestGoals(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-g1", "g1@tt.dev")
	outro := seedUser(t, pool, "g-g2", "g2@tt.dev")
	ws := seedWorkspace(t, pool, "G")
	loc := brt(t)

	goal10, goal5, goal8, goal6, goal25 := 10.0, 5.0, 8.0, 6.0, 2.5
	pMetas := seedProject(t, pool, ws, user, "Metas", "#4f9dde", projOpts{goal: &goal10})
	seedProject(t, pool, ws, user, "Zerado", "#5cc98b", projOpts{goal: &goal5}) // 0 realizado: ENTRA
	pArq := seedProject(t, pool, ws, user, "Arquivado", "#e0a13c", projOpts{goal: &goal8, archived: true})
	seedProject(t, pool, ws, outro, "Privado Alheio", "#b07cd6", projOpts{goal: &goal6, visibility: "private"})
	seedProject(t, pool, ws, user, "Privado Meu", "#c9806a", projOpts{goal: &goal25, visibility: "private"})

	meio := time.Date(2026, 6, 10, 8, 0, 0, 0, loc)
	insertClosed(t, pool, ws, user, &pMetas, meio, 3600)                    // ENTRA no realized
	insertClosed(t, pool, ws, user, &pMetas, meio.Add(2*time.Hour), 3600)   // ENTRA
	insertClosed(t, pool, ws, user, &pMetas, meio.AddDate(0, 0, -7), 7200)  // semana PASSADA: fora
	insertClosed(t, pool, ws, outro, &pMetas, meio.Add(4*time.Hour), 10800) // outro usuário: fora
	insertClosed(t, pool, ws, user, &pArq, meio.Add(5*time.Hour), 1800)     // projeto arquivado: bucket sim, goal não

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}

	if len(rep.Goals) != 3 {
		t.Fatalf("esperava 3 goals (Metas, Privado Meu, Zerado), veio %d: %+v", len(rep.Goals), rep.Goals)
	}
	// ordem name ASC
	if rep.Goals[0].Name != "Metas" || rep.Goals[1].Name != "Privado Meu" || rep.Goals[2].Name != "Zerado" {
		t.Errorf("ordem por name ASC quebrada: %+v", rep.Goals)
	}
	g := rep.Goals[0]
	if g.GoalSeconds != 36000 {
		t.Errorf("Metas: goal_seconds %d, esperava 36000", g.GoalSeconds)
	}
	if g.RealizedSeconds != 7200 {
		t.Errorf("Metas: realized %d, esperava 7200 (só a semana, só o usuário)", g.RealizedSeconds)
	}
	if rep.Goals[1].GoalSeconds != 9000 { // 2.5h
		t.Errorf("Privado Meu: goal_seconds %d, esperava 9000", rep.Goals[1].GoalSeconds)
	}
	if rep.Goals[2].RealizedSeconds != 0 {
		t.Errorf("Zerado: realized %d, esperava 0", rep.Goals[2].RealizedSeconds)
	}
}

// CA-16 — meta fracionária exata: 12.25h → 44100s (round, nunca truncar).
func TestGoalFracionaria(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	user := seedUser(t, pool, "g-g3", "g3@tt.dev")
	ws := seedWorkspace(t, pool, "GF")

	frac := 12.25
	seedProject(t, pool, ws, user, "Fração", "#4f9dde", projOpts{goal: &frac})
	// 8.2 NÃO é exato em float64 (8.2×3600 = 29519.999…): round dá 29520,
	// truncamento daria 29519 — pin do "nunca truncar" (verificação
	// adversarial do build: 12.25 sozinho era float-exato e int() passava)
	inexata := 8.2
	seedProject(t, pool, ws, user, "Inexata", "#5cc98b", projOpts{goal: &inexata})

	svc := newSvc(t, pool, clockMeioDaSemana)
	rep, err := svc.Week(ctx, ws, user, "2026-06-10", tzSP)
	if err != nil {
		t.Fatalf("Week: %v", err)
	}
	if len(rep.Goals) != 2 {
		t.Fatalf("esperava 2 goals, veio %+v", rep.Goals)
	}
	if rep.Goals[0].GoalSeconds != 44100 { // Fração (name ASC)
		t.Errorf("12.25h deveria virar 44100s, veio %d", rep.Goals[0].GoalSeconds)
	}
	if rep.Goals[1].GoalSeconds != 29520 { // Inexata — mata int() truncado
		t.Errorf("8.2h deveria virar 29520s (round), veio %d", rep.Goals[1].GoalSeconds)
	}
}
