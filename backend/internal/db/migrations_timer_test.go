package db

import (
	"context"
	"testing"
)

// Fase 03 T-01 — TestMigracaoTimeEntries: enum entry_source + tabela
// time_entries (plan.md §2). O unique parcial garante NO BANCO o "máx 1
// entry rodando por usuário" (CA-03), inclusive sob concorrência; hard
// delete de task zera task_id (contrato com o hard delete da fase 02).
func TestMigracaoTimeEntries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}
	var ok bool
	if err := pool.QueryRow(ctx,
		"SELECT to_regclass('time_entries') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("tabela time_entries não existe (err=%v)", err)
	}

	// seed mínimo das fases 01/02 pras FKs
	var userA, userB, wsID, projectID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-a', 'a@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-b', 'b@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'App', '#4f9dde', 'shared', $2) RETURNING id`, wsID, userA).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'Onboarding', $3) RETURNING id`, wsID, projectID, userA).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// colunas canônicas + defaults: description '', tags '{}', source no enum
	var entryID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, started_at, source)
		 VALUES ($1, $2, $3, $4, now(), 'timer') RETURNING id`,
		wsID, userA, projectID, taskID).Scan(&entryID); err != nil {
		t.Fatalf("insert entry rodando: %v", err)
	}
	var desc string
	var tagsLen int
	if err := pool.QueryRow(ctx,
		`SELECT description, cardinality(tags) FROM time_entries WHERE id = $1`,
		entryID).Scan(&desc, &tagsLen); err != nil || desc != "" || tagsLen != 0 {
		t.Errorf("defaults esperados description='' e tags vazio (desc=%q len=%d err=%v)", desc, tagsLen, err)
	}

	// enum entry_source: os 4 valores do roadmap entram; fora deles falha
	for _, src := range []string{"manual", "calendar", "claude"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO time_entries (workspace_id, user_id, started_at, stopped_at, duration_seconds, source)
			 VALUES ($1, $2, now() - interval '1 hour', now(), 3600, $3)`, wsID, userA, src); err != nil {
			t.Errorf("source %q deveria ser aceito: %v", src, err)
		}
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'whatsapp')`, wsID, userB); err == nil {
		t.Error("source 'whatsapp' deveria falhar no enum entry_source")
	}

	// CHECK entry_times_valid: stopped_at ≤ started_at não entra
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, stopped_at, source)
		 VALUES ($1, $2, now(), now(), 'manual')`, wsID, userA); err == nil {
		t.Error("stopped_at = started_at deveria violar o check entry_times_valid")
	}

	// CA-03: 2ª entry rodando do MESMO usuário viola o unique parcial...
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'timer')`, wsID, userA); err == nil {
		t.Error("2ª entry rodando do mesmo usuário deveria violar uq_entries_one_running")
	}
	// ...mas entry rodando de OUTRO usuário entra normalmente
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'timer')`, wsID, userB); err != nil {
		t.Errorf("entry rodando de outro usuário deveria ser aceita: %v", err)
	}

	// fechar a entry libera o unique parcial (só vale com stopped_at IS NULL)
	if _, err := pool.Exec(ctx,
		`UPDATE time_entries SET stopped_at = started_at + interval '1 minute',
		        duration_seconds = 60 WHERE id = $1`, entryID); err != nil {
		t.Fatalf("fechar entry rodando: %v", err)
	}

	// CA-03 sob CONCORRÊNCIA: 2 transações inserindo entry rodando pro mesmo
	// user — a 2ª bloqueia no índice e falha quando a 1ª commita.
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("tx1: %v", err)
	}
	if _, err := tx1.Exec(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
		 VALUES ($1, $2, now(), 'timer')`, wsID, userA); err != nil {
		t.Fatalf("insert na tx1: %v", err)
	}
	tx2err := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			tx2err <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()
		_, err = tx2.Exec(ctx,
			`INSERT INTO time_entries (workspace_id, user_id, started_at, source)
			 VALUES ($1, $2, now(), 'timer')`, wsID, userA)
		tx2err <- err
	}()
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}
	if err := <-tx2err; err == nil {
		t.Error("insert concorrente da tx2 deveria falhar no unique parcial após o commit da tx1")
	}

	// contrato com a fase 02: hard delete da task zera task_id (SET NULL)
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("hard delete da task: %v", err)
	}
	var taskNull bool
	if err := pool.QueryRow(ctx,
		`SELECT task_id IS NULL FROM time_entries WHERE id = $1`, entryID).Scan(&taskNull); err != nil || !taskNull {
		t.Errorf("task_id deveria virar NULL após o hard delete da task (err=%v)", err)
	}
}

// Fase 03 T-02 — TestMigracaoPomodoroSettings: config por usuário com
// defaults 25/5 e checks de faixa no banco (focus 15–90, break 1–60).
func TestMigracaoPomodoroSettings(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var userA, userB string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-a', 'a@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-b', 'b@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}

	// insert sem valores aplica os defaults 25/5 (CA-19)
	var focus, brk int
	if err := pool.QueryRow(ctx,
		`INSERT INTO pomodoro_settings (user_id) VALUES ($1)
		 RETURNING focus_minutes, break_minutes`, userA).Scan(&focus, &brk); err != nil {
		t.Fatalf("insert settings com defaults: %v", err)
	}
	if focus != 25 || brk != 5 {
		t.Errorf("defaults esperados 25/5, vieram %d/%d", focus, brk)
	}

	// checks de faixa barram fora de 15–90 / 1–60 (CA-28 no banco)
	if _, err := pool.Exec(ctx,
		`INSERT INTO pomodoro_settings (user_id, break_minutes) VALUES ($1, 0)`, userB); err == nil {
		t.Error("break_minutes=0 deveria violar o check de faixa")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO pomodoro_settings (user_id, focus_minutes) VALUES ($1, 10)`, userB); err == nil {
		t.Error("focus_minutes=10 deveria violar o check de faixa")
	}

	// PK em user_id: 1 config por usuário
	if _, err := pool.Exec(ctx,
		`INSERT INTO pomodoro_settings (user_id) VALUES ($1)`, userA); err == nil {
		t.Error("2ª linha pro mesmo usuário deveria violar a PK")
	}
}

// Fase 03 T-03 — TestMigracaoTimerStates: enum timer_mode + estado de
// runtime do timer (1 linha por usuário — PK). É detalhe INTERNO da fase
// (INDEX.md): o contrato público é o TimerState do GET /timer. O ON DELETE
// SET NULL é defesa em profundidade — a API bloqueia antes com 409 (T-12).
func TestMigracaoTimerStates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	var userID, wsID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-a', 'a@b.c', 'Ana') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	var entryID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO time_entries (workspace_id, user_id, started_at, stopped_at, duration_seconds, source)
		 VALUES ($1, $2, now() - interval '25 minutes', now(), 1500, 'timer') RETURNING id`,
		wsID, userID).Scan(&entryID); err != nil {
		t.Fatalf("insert entry: %v", err)
	}

	// linha de pausa: entry_id NULO (pausa não tem entry rodando) com os
	// campos resume_* apontando o que retomar
	if _, err := pool.Exec(ctx,
		`INSERT INTO timer_states (user_id, mode, entry_id, target_seconds, started_at,
		                           resume_entry_id, resume_mode, resume_target_seconds)
		 VALUES ($1, 'break', NULL, 300, now(), $2, 'pomodoro_focus', 900)`,
		userID, entryID); err != nil {
		t.Fatalf("linha mode='break' com entry_id nulo deveria ser aceita: %v", err)
	}

	// PK em user_id: 2ª linha pro mesmo usuário falha
	if _, err := pool.Exec(ctx,
		`INSERT INTO timer_states (user_id, mode, started_at) VALUES ($1, 'countup', now())`,
		userID); err == nil {
		t.Error("2ª linha de estado pro mesmo usuário deveria violar a PK")
	}

	// enum timer_mode fechado
	if _, err := pool.Exec(ctx,
		`UPDATE timer_states SET mode = 'stopwatch' WHERE user_id = $1`, userID); err == nil {
		t.Error("mode 'stopwatch' deveria falhar no enum timer_mode")
	}

	// DELETE direto no banco da entry referenciada zera resume_entry_id
	// (defesa em profundidade — caminho normal é o 409 da API)
	if _, err := pool.Exec(ctx, `DELETE FROM time_entries WHERE id = $1`, entryID); err != nil {
		t.Fatalf("delete da entry referenciada: %v", err)
	}
	var resumeNull bool
	if err := pool.QueryRow(ctx,
		`SELECT resume_entry_id IS NULL FROM timer_states WHERE user_id = $1`,
		userID).Scan(&resumeNull); err != nil || !resumeNull {
		t.Errorf("resume_entry_id deveria virar NULL após o delete da entry (err=%v)", err)
	}
}
