// cycle2_test.go — fase 15 T-03: ciclo Pomodoro 2.0 (foco curto ×N → pausa
// longa). O contador de focos vive no timer_states (runtime interno, como
// resume_*); o break_kind é PERSISTIDO (break_is_long) e exposto no
// TimerState. Espelho do TestCicloPomodoroCompletoRenasceComFocoCheio da
// fase 03 (que segue valendo — o renascer do foco cheio NÃO é tocado).
package timer_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// completaFoco roda um pomodoro_focus até o fim e pausa (auto-pause do ciclo).
// Devolve o State da pausa resultante.
func completaFoco(t *testing.T, svc *timer.Service, clock *fakeClock, userID, wsID string, focusSeconds int) *timer.State {
	t.Helper()
	ctx := context.Background()
	// avança 1s antes do start pra fechar qualquer entry rodando da operação
	// anterior com stopped_at > started_at (CHECK entry_times_valid).
	clock.Advance(time.Second)
	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Mode: "pomodoro", Description: "foco"}); err != nil {
		t.Fatalf("start pomodoro: %v", err)
	}
	clock.Advance(time.Duration(focusSeconds) * time.Second)
	res, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	return res.State
}

// CA-08.1 — ciclo de N+1 com cycles_before_long=2: foco1→pausa curta,
// foco2→pausa LONGA; retomar da longa zera o ciclo → foco3→pausa curta de
// novo. Determinístico com clock fake. Pina o break_kind e o reset (CA-06/07).
func TestCicloDoisFocosPausaLonga(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 2, LongBreakMinutes: 15,
		AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}

	// foco 1 completo (1500s) → pausa CURTA (focus_count=1, 1 % 2 != 0)
	p1 := completaFoco(t, svc, clock, userID, wsID, 1500)
	if p1.BreakKind != "short" {
		t.Errorf("pausa 1 deveria ser curta (focus_count=1), veio %q", p1.BreakKind)
	}
	if p1.TargetSeconds == nil || *p1.TargetSeconds != 5*60 {
		t.Errorf("pausa 1 curta deveria ter alvo 300, veio %v", p1.TargetSeconds)
	}
	// retoma da curta (zera a pausa) — ciclo NÃO reseta
	clock.Advance(300 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume da pausa 1: %v", err)
	}

	// foco 2 completo → pausa LONGA (focus_count=2, 2 % 2 == 0)
	clock.Advance(1500 * time.Second)
	p2res, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause 2: %v", err)
	}
	if p2res.State.BreakKind != "long" {
		t.Errorf("pausa 2 deveria ser LONGA (focus_count=2, múltiplo de 2), veio %q", p2res.State.BreakKind)
	}
	if p2res.State.TargetSeconds == nil || *p2res.State.TargetSeconds != 15*60 {
		t.Errorf("pausa 2 longa deveria ter alvo 900, veio %v", p2res.State.TargetSeconds)
	}
	// GET seguinte DEVE reportar o mesmo kind (persistido, não recomputado)
	getP2, _ := svc.State(ctx, userID)
	if getP2.State.BreakKind != "long" {
		t.Errorf("GET da pausa 2 deveria reportar long (persistido), veio %q", getP2.State.BreakKind)
	}

	// retoma da LONGA → ciclo RESETA (focus_count volta a 0)
	clock.Advance(900 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume da pausa longa: %v", err)
	}

	// foco 3 completo → pausa CURTA de novo (focus_count=1 após reset)
	clock.Advance(1500 * time.Second)
	p3, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause 3: %v", err)
	}
	if p3.State.BreakKind != "short" {
		t.Errorf("pausa 3 (após reset da longa) deveria voltar a ser curta, veio %q", p3.State.BreakKind)
	}
}

// CA-05 — countup pausado NÃO incrementa o contador de focos: só foco conta
// pro ciclo. Com cycles=1 (toda pausa de foco seria longa), um countup no
// meio não muda o ciclo.
func TestCountupNaoIncrementaCiclo(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()
	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 2, LongBreakMinutes: 15,
		AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}

	// countup e pausa: não conta pro ciclo, pausa é curta
	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Mode: "countup"}); err != nil {
		t.Fatalf("start countup: %v", err)
	}
	clock.Advance(60 * time.Second)
	cuPause, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause countup: %v", err)
	}
	if cuPause.State.BreakKind != "short" {
		t.Errorf("pausa de countup deveria ser curta (não conta pro ciclo), veio %q", cuPause.State.BreakKind)
	}
	clock.Advance(300 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// agora 2 focos completos → a 2ª pausa deveria ser longa (countup não
	// somou ao contador). Se o countup tivesse incrementado, a 1ª pausa de
	// foco já seria longa.
	completaFoco(t, svc, clock, userID, wsID, 1500)
	clock.Advance(300 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume foco1: %v", err)
	}
	clock.Advance(1500 * time.Second)
	p2, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause foco2: %v", err)
	}
	if p2.State.BreakKind != "long" {
		t.Errorf("após 2 focos (countup não conta), a 2ª pausa de foco deveria ser longa, veio %q", p2.State.BreakKind)
	}
}

// CA-06/06.1 (override manual num passo que daria longa) — com cycles=2, no
// 2º foco (que daria pausa LONGA), um override manual de break_minutes força
// pausa CURTA, e o break_kind PERSISTIDO é "short" no pause E no GET (não
// recomputado por focus_count % cycles — furo confirmado).
func TestOverrideManualNoMultiploEhCurta(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()
	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 2, LongBreakMinutes: 15,
		AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}

	// foco 1 completo → pausa curta (focus_count=1). Resumir da CURTA
	// PRESERVA o contador (o ciclo continua) — não usar Start (resetaria).
	completaFoco(t, svc, clock, userID, wsID, 1500)
	clock.Advance(300 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume 1 (foco2, count preservado em 1): %v", err)
	}

	// foco 2 corre — ao pausar, newCount=2 (múltiplo de 2) DARIA pausa LONGA,
	// mas o override manual de 7min força CURTA. Aqui mora o furo: persistir
	// short, NÃO recomputar long do focus_count=2.
	clock.Advance(1500 * time.Second)
	override := 7
	pres, err := svc.Pause(ctx, userID, &override)
	if err != nil {
		t.Fatalf("pause com override: %v", err)
	}
	if pres.State.BreakKind != "short" {
		t.Errorf("override manual num passo de pausa-longa DEVE ser curta, veio %q", pres.State.BreakKind)
	}
	if pres.State.TargetSeconds == nil || *pres.State.TargetSeconds != 7*60 {
		t.Errorf("override deveria valer 420s, veio %v", pres.State.TargetSeconds)
	}
	// GET seguinte: persistido como short (NÃO recomputa long do focus_count=2)
	get, _ := svc.State(ctx, userID)
	if get.State.BreakKind != "short" {
		t.Errorf("GET deveria reportar short PERSISTIDO (focus_count=2 não recomputa long), veio %q", get.State.BreakKind)
	}
}

// CA-07 — stop fora de pausa reseta o ciclo: depois de 1 foco completo,
// stop e novo ciclo começa do zero (a próxima pausa de foco volta a curta
// mesmo que o foco anterior tivesse avançado o contador).
func TestStopReseteaCiclo(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()
	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 1, LongBreakMinutes: 15,
		AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}

	// com cycles=1, todo foco completo dá pausa LONGA. foco1 → longa.
	p1 := completaFoco(t, svc, clock, userID, wsID, 1500)
	if p1.BreakKind != "long" {
		t.Errorf("com cycles=1 a 1ª pausa de foco deveria ser longa, veio %q", p1.BreakKind)
	}
	// retoma da longa (reseta) e PARA (stop) — limpa a linha
	clock.Advance(900 * time.Second)
	if _, err := svc.Resume(ctx, userID); err != nil {
		t.Fatalf("resume da longa: %v", err)
	}
	clock.Advance(time.Second)
	if _, err := svc.Stop(ctx, userID); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// novo ciclo após stop: foco completo → longa de novo (contador = 1 do zero)
	p2 := completaFoco(t, svc, clock, userID, wsID, 1500)
	if p2.BreakKind != "long" {
		t.Errorf("após stop o ciclo recomeça do zero; cycles=1 → 1ª pausa longa, veio %q", p2.BreakKind)
	}
}
