// cutter_test.go — TDD do corte (fase 16 T-02): limiar estrito (CA-02),
// marco e não-now (CA-03), limpeza do estado (CA-04), pausa nunca cortada
// (CA-05), idempotência (CA-06), corrida/lock (CA-07), isolamento (CA-08),
// hooks só no corte (CA-09). Clock injetável (RNF-03) — o teste avança o
// relógio em vez de esperar 2h reais.
package guard_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/guard"
	"timetrack.live/backend/internal/timer"
)

const base = "2026-06-20T12:00:00Z"

func baseTime(t *testing.T) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, base)
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	return tm
}

// seedUser cria um user e um workspace e devolve seus ids.
func seedUser(t *testing.T, pool *pgxpool.Pool, googleID string) (userID, wsID string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', 'U') RETURNING id`,
		googleID).Scan(&userID); err != nil {
		t.Fatalf("insert user %s: %v", googleID, err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		wsID, userID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	return userID, wsID
}

// seedRunning cria uma entry RODANDO (stopped_at NULL) começada em startedAt e
// a linha de timer_states correspondente (mode countup), espelhando o que o
// timer.Start faz. Devolve o id da entry.
func seedRunning(t *testing.T, pool *pgxpool.Pool, wsID, userID string, startedAt time.Time) string {
	t.Helper()
	ctx := context.Background()
	var entryID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		VALUES ($1, $2, $3, 'timer') RETURNING id`,
		wsID, userID, startedAt).Scan(&entryID); err != nil {
		t.Fatalf("insert running entry: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO timer_states (user_id, mode, entry_id, started_at)
		VALUES ($1, 'countup', $2, $3)`, userID, entryID, startedAt); err != nil {
		t.Fatalf("insert timer_state: %v", err)
	}
	return entryID
}

type entryRow struct {
	stoppedAt *time.Time
	duration  *int
	autoCut   bool
}

func loadEntry(t *testing.T, pool *pgxpool.Pool, id string) entryRow {
	t.Helper()
	var r entryRow
	if err := pool.QueryRow(context.Background(),
		`SELECT stopped_at, duration_seconds, auto_cut FROM time_entries WHERE id = $1`, id).
		Scan(&r.stoppedAt, &r.duration, &r.autoCut); err != nil {
		t.Fatalf("load entry %s: %v", id, err)
	}
	return r
}

func hasTimerState(t *testing.T, pool *pgxpool.Pool, userID string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM timer_states WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("contar timer_states: %v", err)
	}
	return n > 0
}

// CA-02 — limiar ESTRITO: exatamente 2h NÃO corta; 2h+1s corta.
func TestLimiarEstrito(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)

	uExato, wsExato := seedUser(t, pool, "g-exato")
	uVencido, wsVencido := seedUser(t, pool, "g-vencido")
	// exatamente 2h atrás → não corta
	eExato := seedRunning(t, pool, wsExato, uExato, now.Add(-2*time.Hour))
	// 2h+1s atrás → corta
	eVencido := seedRunning(t, pool, wsVencido, uVencido, now.Add(-2*time.Hour-time.Second))

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(cuts) != 1 {
		t.Fatalf("esperava 1 corte (só o vencido), veio %d", len(cuts))
	}
	if cuts[0].EntryID != eVencido {
		t.Errorf("o corte deveria ser da entry vencida, veio %s", cuts[0].EntryID)
	}
	if loadEntry(t, pool, eExato).stoppedAt != nil {
		t.Errorf("entry de EXATAMENTE 2h NÃO deveria ser cortada (limiar estrito)")
	}
	if loadEntry(t, pool, eVencido).stoppedAt == nil {
		t.Errorf("entry de 2h+1s DEVERIA ser cortada")
	}
}

// CA-03 — marco, não now: stopped_at = started+2h, duration=7200, auto_cut=true.
func TestCorteNoMarcoNaoAgora(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	started := now.Add(-5 * time.Hour) // 5h atrás: muito vencida
	u, ws := seedUser(t, pool, "g-marco")
	e := seedRunning(t, pool, ws, u, started)

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	if _, err := cutter.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	r := loadEntry(t, pool, e)
	if r.stoppedAt == nil {
		t.Fatalf("entry deveria estar cortada")
	}
	wantMark := started.Add(2 * time.Hour)
	if !r.stoppedAt.Equal(wantMark) {
		t.Errorf("stopped_at deveria ser o MARCO %v (started+2h), veio %v — NUNCA now (%v)",
			wantMark, *r.stoppedAt, now)
	}
	if r.stoppedAt.Equal(now) {
		t.Errorf("stopped_at NÃO pode ser now (%v) — seria a hora-fantasma", now)
	}
	if r.duration == nil || *r.duration != 7200 {
		t.Errorf("duration_seconds deveria ser 7200 (2h exatas), veio %v", r.duration)
	}
	if !r.autoCut {
		t.Errorf("auto_cut deveria ser true")
	}
}

// CA-04 — limpa o estado: timer_states do user some após o corte; entry
// rodando ÓRFÃ (sem timer_states) ainda é cortada.
func TestLimpaEstadoEEntryOrfa(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)

	// user com timer_states → some após o corte
	u, ws := seedUser(t, pool, "g-estado")
	e := seedRunning(t, pool, ws, u, now.Add(-3*time.Hour))
	if !hasTimerState(t, pool, u) {
		t.Fatalf("timer_states deveria existir antes do corte")
	}

	// user com entry rodando ÓRFÃ (sem timer_states) → ainda cortada
	uOrfa, wsOrfa := seedUser(t, pool, "g-orfa")
	var eOrfa string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		VALUES ($1, $2, $3, 'timer') RETURNING id`,
		wsOrfa, uOrfa, now.Add(-3*time.Hour)).Scan(&eOrfa); err != nil {
		t.Fatalf("insert entry órfã: %v", err)
	}

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 2 {
		t.Fatalf("esperava 2 cortes (com e sem estado), veio %d", len(cuts))
	}
	if hasTimerState(t, pool, u) {
		t.Errorf("timer_states do user deveria ser removido após o corte (CA-04)")
	}
	if loadEntry(t, pool, e).stoppedAt == nil {
		t.Errorf("entry com estado deveria ser cortada")
	}
	if loadEntry(t, pool, eOrfa).stoppedAt == nil {
		t.Errorf("entry rodando ÓRFÃ deveria ser cortada (a entry é a fonte do rodando)")
	}
}

// CA-05 — pausa NUNCA é cortada (sem entry rodando) e entry manual já fechada
// não entra na varredura.
func TestPausaEManualNaoCortadas(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	ctx := context.Background()

	// user em break: timer_states mode='break', SEM entry rodando
	uBreak, wsBreak := seedUser(t, pool, "g-break")
	if _, err := pool.Exec(ctx, `
		INSERT INTO timer_states (user_id, mode, started_at, target_seconds)
		VALUES ($1, 'break', $2, 300)`, uBreak, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("insert timer_state break: %v", err)
	}
	// e uma entry FECHADA dele (a entry pausada, há 3h) — não pode ser tocada
	var eFechada string
	if err := pool.QueryRow(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, 1800, 'timer') RETURNING id`,
		wsBreak, uBreak, now.Add(-3*time.Hour), now.Add(-3*time.Hour).Add(30*time.Minute)).Scan(&eFechada); err != nil {
		t.Fatalf("insert entry fechada: %v", err)
	}

	// user com entry MANUAL já fechada, começada há muito
	uManual, wsManual := seedUser(t, pool, "g-manual")
	var eManual string
	if err := pool.QueryRow(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, 7200, 'manual') RETURNING id`,
		wsManual, uManual, now.Add(-10*time.Hour), now.Add(-8*time.Hour)).Scan(&eManual); err != nil {
		t.Fatalf("insert entry manual: %v", err)
	}

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 0 {
		t.Fatalf("pausa/entry fechada não deveriam ser cortadas, veio %d cortes", len(cuts))
	}
	if !hasTimerState(t, pool, uBreak) {
		t.Errorf("o break NÃO deveria ser tocado")
	}
	if r := loadEntry(t, pool, eFechada); r.autoCut || r.duration == nil || *r.duration != 1800 {
		t.Errorf("a entry fechada da pausa não pode mudar (auto_cut=%v dur=%v)", r.autoCut, r.duration)
	}
	if r := loadEntry(t, pool, eManual); r.autoCut {
		t.Errorf("a entry manual fechada não pode ser auto-cortada")
	}
}

// CA-06 — idempotência: 2ª passada sobre o mesmo estado não corta de novo
// nem reemite aviso.
func TestIdempotencia(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	u, ws := seedUser(t, pool, "g-idem")
	seedRunning(t, pool, ws, u, now.Add(-3*time.Hour))

	var notices int
	cutter := guard.NewCutter(pool,
		guard.WithClock(func() time.Time { return now }),
		guard.WithOnCut(func(guard.CutEvent) { notices++ }),
	)
	ctx := context.Background()
	cuts1, err := cutter.RunOnce(ctx)
	if err != nil {
		t.Fatalf("1ª passada: %v", err)
	}
	cuts2, err := cutter.RunOnce(ctx)
	if err != nil {
		t.Fatalf("2ª passada: %v", err)
	}
	if len(cuts1) != 1 {
		t.Errorf("1ª passada deveria cortar 1, veio %d", len(cuts1))
	}
	if len(cuts2) != 0 {
		t.Errorf("2ª passada NÃO deveria cortar nada (idempotência), veio %d", len(cuts2))
	}
	if notices != 1 {
		t.Errorf("aviso deveria sair só 1 vez (CA-06/09), veio %d", notices)
	}
}

// CA-07 — corrida/lock: se o usuário PARA o timer no intervalo entre a seleção
// e o corte (entry já fechada), o FOR UPDATE re-check aborta — nada fecha em
// dobro. Simula fechando a entry ANTES da passada (o candidato sumiu da
// re-checagem sob o lock).
func TestCorridaForUpdateAborta(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	ctx := context.Background()
	u, ws := seedUser(t, pool, "g-corrida")
	started := now.Add(-3 * time.Hour)
	e := seedRunning(t, pool, ws, u, started)

	// o usuário parou: a entry foi fechada (stopped_at preenchido) e o
	// timer_states removido, ANTES da varredura agir. A seleção do RunOnce já
	// não vai pegá-la (stopped_at IS NULL filtra), e mesmo que pegasse, o FOR
	// UPDATE re-check abortaria.
	userStop := started.Add(40 * time.Minute)
	if _, err := pool.Exec(ctx,
		`UPDATE time_entries SET stopped_at = $2, duration_seconds = 2400 WHERE id = $1`,
		e, userStop); err != nil {
		t.Fatalf("simular stop do usuário: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM timer_states WHERE user_id = $1`, u); err != nil {
		t.Fatalf("remover timer_state: %v", err)
	}

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 0 {
		t.Fatalf("entry já parada pelo usuário não pode ser cortada de novo, veio %d", len(cuts))
	}
	r := loadEntry(t, pool, e)
	if r.stoppedAt == nil || !r.stoppedAt.Equal(userStop) {
		t.Errorf("o stopped_at do usuário (%v) NÃO pode ser sobrescrito pelo worker, veio %v", userStop, r.stoppedAt)
	}
	if r.autoCut {
		t.Errorf("a entry parada pelo usuário não pode virar auto_cut")
	}
	if r.duration == nil || *r.duration != 2400 {
		t.Errorf("duration do stop do usuário deveria ficar 2400, veio %v", r.duration)
	}
}

// CA-08 — isolamento: 2 usuários vencidos, só o do tenant certo é tocado; o
// corte de um nunca mexe na entry/estado do outro. (A mutação que remove o
// WHERE user_id do UPDATE/DELETE deve quebrar este teste.)
func TestIsolamentoEntreUsuarios(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)

	uA, wsA := seedUser(t, pool, "g-iso-a")
	uB, wsB := seedUser(t, pool, "g-iso-b")
	eA := seedRunning(t, pool, wsA, uA, now.Add(-3*time.Hour))
	eB := seedRunning(t, pool, wsB, uB, now.Add(-4*time.Hour))

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 2 {
		t.Fatalf("ambos vencidos deveriam ser cortados (cada um no seu marco), veio %d", len(cuts))
	}
	// cada corte some no seu próprio marco; o de A nunca fecha a entry de B
	rA := loadEntry(t, pool, eA)
	rB := loadEntry(t, pool, eB)
	if rA.stoppedAt == nil || !rA.stoppedAt.Equal(now.Add(-3*time.Hour).Add(2*time.Hour)) {
		t.Errorf("entry A deveria fechar no marco started_A+2h, veio %v", rA.stoppedAt)
	}
	if rB.stoppedAt == nil || !rB.stoppedAt.Equal(now.Add(-4*time.Hour).Add(2*time.Hour)) {
		t.Errorf("entry B deveria fechar no marco started_B+2h, veio %v", rB.stoppedAt)
	}
	// o timer_states de cada um foi removido, mas só o seu
	if hasTimerState(t, pool, uA) || hasTimerState(t, pool, uB) {
		t.Errorf("ambos os timer_states deveriam ser removidos (cada um pelo seu corte)")
	}
}

// CA-08 (reforço do isolamento por user_id): um user vencido e um user NÃO
// vencido. Só o vencido é cortado; o estado do não-vencido fica intacto. Uma
// mutação removendo o WHERE user_id do DELETE removeria o timer_states do
// não-vencido também e mataria este teste.
func TestSoOVencidoTemEstadoRemovido(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)

	uVencido, wsV := seedUser(t, pool, "g-iso2-venc")
	uVivo, wsViv := seedUser(t, pool, "g-iso2-vivo")
	seedRunning(t, pool, wsV, uVencido, now.Add(-3*time.Hour))            // vencido
	eVivo := seedRunning(t, pool, wsViv, uVivo, now.Add(-30*time.Minute)) // 30min: NÃO vence

	cutter := guard.NewCutter(pool, guard.WithClock(func() time.Time { return now }))
	cuts, err := cutter.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 1 {
		t.Fatalf("só o vencido deveria ser cortado, veio %d", len(cuts))
	}
	if hasTimerState(t, pool, uVencido) {
		t.Errorf("o estado do vencido deveria sumir")
	}
	if !hasTimerState(t, pool, uVivo) {
		t.Errorf("o estado do user NÃO-vencido deve ficar intacto (isolamento)")
	}
	if loadEntry(t, pool, eVivo).stoppedAt != nil {
		t.Errorf("a entry do user não-vencido NÃO pode ser cortada")
	}
}

// CA-09 — hooks só no corte real: onTimer e onCut são chamados EXATAMENTE para
// as entries cortadas nesta passada (spy). Passada sem corte = zero chamada.
func TestHooksSoNoCorteReal(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)

	uVencido, wsV := seedUser(t, pool, "g-hook-venc")
	uVivo, wsViv := seedUser(t, pool, "g-hook-vivo")
	seedRunning(t, pool, wsV, uVencido, now.Add(-3*time.Hour))
	seedRunning(t, pool, wsViv, uVivo, now.Add(-10*time.Minute)) // não vence

	var mu sync.Mutex
	var onTimerUsers []string
	var onCutUsers []string
	var onTimerNilState bool = true
	cutter := guard.NewCutter(pool,
		guard.WithClock(func() time.Time { return now }),
		guard.WithOnTimer(func(userID string, st *timer.State, _ time.Time) {
			mu.Lock()
			defer mu.Unlock()
			onTimerUsers = append(onTimerUsers, userID)
			if st != nil {
				onTimerNilState = false
			}
		}),
		guard.WithOnCut(func(c guard.CutEvent) {
			mu.Lock()
			defer mu.Unlock()
			onCutUsers = append(onCutUsers, c.UserID)
		}),
	)
	if _, err := cutter.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(onTimerUsers) != 1 || onTimerUsers[0] != uVencido {
		t.Errorf("onTimer deveria ser chamado SÓ pro user vencido, veio %v", onTimerUsers)
	}
	if !onTimerNilState {
		t.Errorf("onTimer do corte deveria emitir estado NULO (ocioso, como um Stop remoto)")
	}
	if len(onCutUsers) != 1 || onCutUsers[0] != uVencido {
		t.Errorf("onCut deveria ser chamado SÓ pro user vencido, veio %v", onCutUsers)
	}
}

// CA-09 (reforço) — passada SEM corte não dispara hook nenhum.
func TestPassadaSemCorteNaoEmite(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	u, ws := seedUser(t, pool, "g-semcorte")
	seedRunning(t, pool, ws, u, now.Add(-30*time.Minute)) // 30min: não vence

	var hooks int
	cutter := guard.NewCutter(pool,
		guard.WithClock(func() time.Time { return now }),
		guard.WithOnTimer(func(string, *timer.State, time.Time) { hooks++ }),
		guard.WithOnCut(func(guard.CutEvent) { hooks++ }),
	)
	cuts, err := cutter.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(cuts) != 0 || hooks != 0 {
		t.Errorf("passada sem corte: 0 cortes e 0 hooks, veio %d cortes / %d hooks", len(cuts), hooks)
	}
}
