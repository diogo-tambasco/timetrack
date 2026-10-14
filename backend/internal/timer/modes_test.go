// modes_test.go — T-06: countdown, modo pomodoro e validações de escrita
// (CA-12/13/13.1/27/29).
package timer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/timer"
)

func intPtr(n int) *int { return &n }

// CA-12 — countdown de 1200s consultado 300s depois: remaining_seconds
// calculado pelo RELÓGIO DO SERVIDOR (1200 − 300 = 900).
func TestCountdownRemainingCalculadoNoServidor(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	res, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown", TargetSeconds: intPtr(1200),
	})
	if err != nil {
		t.Fatalf("start countdown: %v", err)
	}
	if res.State.Mode != "countdown" || res.State.TargetSeconds == nil || *res.State.TargetSeconds != 1200 {
		t.Fatalf("estado inicial esperado countdown/1200, veio %+v", res.State)
	}

	clock.Advance(300 * time.Second)
	st, err := svc.State(ctx, userID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if st.State.ElapsedSeconds != 300 {
		t.Errorf("elapsed esperado 300, veio %d", st.State.ElapsedSeconds)
	}
	if st.State.RemainingSeconds == nil || *st.State.RemainingSeconds != 900 {
		t.Errorf("remaining esperado 900, veio %v", st.State.RemainingSeconds)
	}
}

// CA-13 — start em modo pomodoro usa o focus_minutes PERSISTIDO do usuário
// como alvo (50 min → 3000s); na API o modo aparece como "pomodoro", nunca
// o enum interno.
func TestPomodoroUsaFocusPersistido(t *testing.T) {
	svc, _, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO pomodoro_settings (user_id, focus_minutes) VALUES ($1, 50)`, userID); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	res, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Mode: "pomodoro"})
	if err != nil {
		t.Fatalf("start pomodoro: %v", err)
	}
	if res.State.Mode != "pomodoro" {
		t.Errorf("state.mode na API deveria ser 'pomodoro' (sem vazar o enum interno), veio %q", res.State.Mode)
	}
	if res.State.TargetSeconds == nil || *res.State.TargetSeconds != 3000 {
		t.Errorf("target esperado 3000 (focus 50min), veio %v", res.State.TargetSeconds)
	}

	// no banco o enum interno é 'pomodoro_focus' (plan §3.2)
	var dbMode string
	if err := pool.QueryRow(ctx,
		`SELECT mode::text FROM timer_states WHERE user_id = $1`, userID).Scan(&dbMode); err != nil || dbMode != "pomodoro_focus" {
		t.Errorf("mode no banco esperado 'pomodoro_focus', veio %q (err=%v)", dbMode, err)
	}
}

// CA-13.1 — stop DEPOIS do alvo vencido (overrun): grava a duração REAL,
// sem clampar ao alvo (decisão de produto da v1).
func TestStopOverrunGravaDuracaoReal(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown", TargetSeconds: intPtr(1200),
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(3600 * time.Second)

	res, err := svc.Stop(ctx, userID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if res.StoppedEntry.DurationSeconds == nil || *res.StoppedEntry.DurationSeconds != 3600 {
		t.Errorf("overrun deveria gravar a duração real 3600 (sem clamp a 1200), veio %v",
			res.StoppedEntry.DurationSeconds)
	}
}

// CA-27 — countdown sem target_seconds (ou ≤ 0) é validation_error e
// NENHUMA entry/estado é criado.
func TestCountdownSemAlvoFalha(t *testing.T) {
	svc, _, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	cases := []struct {
		nome   string
		target *int
	}{
		{"sem target", nil},
		{"target zero", intPtr(0)},
		{"target negativo", intPtr(-10)},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			_, err := svc.Start(ctx, userID, timer.StartInput{
				WorkspaceID: wsID, Mode: "countdown", TargetSeconds: tc.target,
			})
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != "validation_error" {
				t.Fatalf("esperava validation_error, veio %v", err)
			}
		})
	}
	if n := dbtest.CountRows(t, pool, "time_entries"); n != 0 {
		t.Errorf("nenhuma entry deveria existir após starts inválidos, vieram %d", n)
	}
	if n := dbtest.CountRows(t, pool, "timer_states"); n != 0 {
		t.Errorf("nenhum estado deveria existir após starts inválidos, vieram %d", n)
	}
}

// CA-29 — project_id/task_id de OUTRO workspace respondem not_found e nada
// é criado (anti-vazamento na escrita). Table-driven cobrindo o start do
// timer E a validação compartilhada usada pela entrada manual (T-12).
func TestRefsDeOutroWorkspaceNaoVazam(t *testing.T) {
	svc, _, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	// outro workspace DO MESMO usuário, com projeto e task
	var wsB, projB, taskB string
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('B') RETURNING id`).Scan(&wsB); err != nil {
		t.Fatalf("seed ws B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'Fora', '#222222', 'shared', $2) RETURNING id`, wsB, userID).Scan(&projB); err != nil {
		t.Fatalf("seed projeto B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3) RETURNING id`, wsB, projB, userID).Scan(&taskB); err != nil {
		t.Fatalf("seed task B: %v", err)
	}

	inexistente := "00000000-0000-0000-0000-000000000999"
	cases := []struct {
		nome    string
		project *string
		task    *string
	}{
		{"project de outro workspace", &projB, nil},
		{"task de outro workspace", nil, &taskB},
		{"project inexistente", &inexistente, nil},
		{"project com id fora do formato", strPtr("nao-e-uuid"), nil},
	}
	for _, tc := range cases {
		t.Run("start: "+tc.nome, func(t *testing.T) {
			_, err := svc.Start(ctx, userID, timer.StartInput{
				WorkspaceID: wsID, ProjectID: tc.project, TaskID: tc.task,
			})
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != "not_found" {
				t.Fatalf("esperava not_found, veio %v", err)
			}
		})
		t.Run("validação compartilhada (entrada manual): "+tc.nome, func(t *testing.T) {
			err := entries.ValidateRefs(ctx, pool, wsID, tc.project, tc.task)
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != "not_found" {
				t.Fatalf("esperava not_found, veio %v", err)
			}
		})
	}

	if n := dbtest.CountRows(t, pool, "time_entries"); n != 0 {
		t.Errorf("nenhuma entry deveria ter sido criada, vieram %d", n)
	}
}

func strPtr(s string) *string { return &s }
