package db

import (
	"context"
	"testing"
)

// Fase 15 T-01 — TestMigracaoPomodoroCycles: a migração 0015 estende
// pomodoro_settings (+5 colunas de ciclo/auto-start/som) e timer_states
// (+focus_count, +break_is_long PERSISTIDO — CA-06.1, não recomputar).
// Os defaults de auto-start = true/true PRESERVAM o ciclo automático da
// fase 03 (CA-01/D-05); default false REGREDIRIA o cycle_test.go.
func TestMigracaoPomodoroCycles(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var userA, userB, wsID, entryID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-pc-a', 'pca@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-pc-b', 'pcb@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'timer') RETURNING id`, wsID, userA).Scan(&entryID); err != nil {
		t.Fatalf("insert entry: %v", err)
	}

	// pomodoro_settings: insert sem valores aplica os defaults novos.
	// CA-01/D-05: auto-start = true/true PRESERVA a fase 03; som = false.
	var cycles, longBrk int
	var autoBrk, autoFocus, sound bool
	if err := pool.QueryRow(ctx,
		`INSERT INTO pomodoro_settings (user_id) VALUES ($1)
		 RETURNING cycles_before_long, long_break_minutes, auto_start_break, auto_start_focus, sound_enabled`,
		userA).Scan(&cycles, &longBrk, &autoBrk, &autoFocus, &sound); err != nil {
		t.Fatalf("insert settings com defaults novos: %v", err)
	}
	if cycles != 4 || longBrk != 15 {
		t.Errorf("defaults esperados cycles=4 long_break=15, vieram %d/%d", cycles, longBrk)
	}
	if !autoBrk || !autoFocus {
		t.Errorf("auto-start defaults DEVEM ser true/true (preservam a fase 03 — D-05), vieram %v/%v", autoBrk, autoFocus)
	}
	if sound {
		t.Errorf("sound_enabled default DEVE ser false, veio %v", sound)
	}

	// CHECKs barram cycles fora de 1..12 e long_break fora de 1..120.
	for _, c := range []struct {
		desc string
		sql  string
	}{
		{"cycles = 0", `INSERT INTO pomodoro_settings (user_id, cycles_before_long) VALUES ($1, 0)`},
		{"cycles = 13", `INSERT INTO pomodoro_settings (user_id, cycles_before_long) VALUES ($1, 13)`},
		{"long_break = 0", `INSERT INTO pomodoro_settings (user_id, long_break_minutes) VALUES ($1, 0)`},
		{"long_break = 121", `INSERT INTO pomodoro_settings (user_id, long_break_minutes) VALUES ($1, 121)`},
	} {
		if _, err := pool.Exec(ctx, c.sql, userB); err == nil {
			t.Errorf("%s deveria violar o CHECK", c.desc)
		}
	}

	// timer_states: focus_count default 0, break_is_long default false.
	if _, err := pool.Exec(ctx,
		`INSERT INTO timer_states (user_id, mode, entry_id, started_at)
		 VALUES ($1, 'countup', $2, now())`, userA, entryID); err != nil {
		t.Fatalf("insert timer_state: %v", err)
	}
	var focusCount int
	var breakIsLong bool
	if err := pool.QueryRow(ctx,
		`SELECT focus_count, break_is_long FROM timer_states WHERE user_id = $1`,
		userA).Scan(&focusCount, &breakIsLong); err != nil {
		t.Fatalf("ler focus_count/break_is_long: %v", err)
	}
	if focusCount != 0 {
		t.Errorf("focus_count default deveria ser 0, veio %d", focusCount)
	}
	if breakIsLong {
		t.Errorf("break_is_long default deveria ser false, veio %v", breakIsLong)
	}
}
