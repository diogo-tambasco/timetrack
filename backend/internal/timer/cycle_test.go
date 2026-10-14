// cycle_test.go — correção da verificação adversarial do bloco 2 (F1):
// no ciclo CANÔNICO do US-06 (foco roda até o fim → auto-pause CA-34 →
// pausa zera → auto-resume CA-34.1), o foco completado guarda restante 0;
// retomar com alvo 0 degenerava o ciclo num loop foco-de-1s ↔ pausa-cheia.
// O resume de pomodoro_focus com restante 0 DEVE renascer com o alvo
// cheio (focus_minutes × 60) — ciclo novo, como o US-06 promete.
package timer_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

func TestCicloPomodoroCompletoRenasceComFocoCheio(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 5}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Mode: "pomodoro", Description: "ciclo"}); err != nil {
		t.Fatalf("start pomodoro: %v", err)
	}

	// foco roda ATÉ O FIM (1500s) — é o que o auto-pause do CA-34 faz
	clock.Advance(1500 * time.Second)
	pause, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause no fim do foco: %v", err)
	}
	if pause.State.Resume == nil || pause.State.Resume.TargetSeconds == nil ||
		*pause.State.Resume.TargetSeconds != 0 {
		t.Fatalf("premissa: foco completado guarda restante 0, veio %+v", pause.State.Resume)
	}

	// pausa roda até o fim — é o que o auto-resume do CA-34.1 faz
	clock.Advance(300 * time.Second)
	res, err := svc.Resume(ctx, userID)
	if err != nil {
		t.Fatalf("resume do ciclo: %v", err)
	}
	if res.State.Mode != "pomodoro" {
		t.Fatalf("o 2º foco deveria ser pomodoro, veio %q", res.State.Mode)
	}
	if res.State.TargetSeconds == nil || *res.State.TargetSeconds != 1500 {
		t.Errorf("o 2º foco deveria renascer com o alvo CHEIO 1500 (US-06), veio %v — alvo 0 degenera o ciclo em foco-de-1s ↔ pausa-cheia", res.State.TargetSeconds)
	}
	if res.State.RemainingSeconds == nil || *res.State.RemainingSeconds != 1500 {
		t.Errorf("remaining do 2º foco deveria ser 1500, veio %v", res.State.RemainingSeconds)
	}

	// pause MANUAL no meio do foco segue o CA-10 à risca: restante real
	clock.Advance(600 * time.Second)
	pause2, err := svc.Pause(ctx, userID, nil)
	if err != nil {
		t.Fatalf("pause manual: %v", err)
	}
	if pause2.State.Resume.TargetSeconds == nil || *pause2.State.Resume.TargetSeconds != 900 {
		t.Errorf("pause no meio do foco guarda o restante real 900 (CA-10), veio %v", pause2.State.Resume.TargetSeconds)
	}
}
