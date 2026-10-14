package db

import (
	"context"
	"testing"
)

// Fase 20 T-01 — TestMigracaoReminders: a migração 0020 cria a tabela reminders
// (quem é lembrado, qual task, quando, tipo due|manual, status) com as colunas/
// defaults declaradas (CA-01), o índice parcial pending pra varredura barata
// (CA-02), o unique parcial due-pending (CA-03), FKs ON DELETE CASCADE e os
// CHECKs de kind/status.
func TestMigracaoReminders(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// colunas + nullability declaradas (CA-01).
	for _, c := range []struct {
		name     string
		dataType string
		nullable string
	}{
		{"id", "uuid", "NO"},
		{"user_id", "uuid", "NO"},
		{"task_id", "uuid", "NO"},
		{"workspace_id", "uuid", "NO"},
		{"remind_at", "timestamp with time zone", "NO"},
		{"kind", "text", "NO"},
		{"status", "text", "NO"},
		{"created_at", "timestamp with time zone", "NO"},
	} {
		var dataType, isNullable string
		if err := pool.QueryRow(ctx,
			`SELECT data_type, is_nullable FROM information_schema.columns
			  WHERE table_name = 'reminders' AND table_schema = current_schema() AND column_name = $1`, c.name).Scan(&dataType, &isNullable); err != nil {
			t.Fatalf("coluna %s: %v", c.name, err)
		}
		if dataType != c.dataType {
			t.Errorf("coluna %s deveria ser %s, veio %q", c.name, c.dataType, dataType)
		}
		if isNullable != c.nullable {
			t.Errorf("coluna %s deveria ter is_nullable=%q, veio %q", c.name, c.nullable, isNullable)
		}
	}

	// seed comum: user + ws + project + task.
	var userA, userB, wsID, projID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-rem-a', 'rema@b.c', 'Ana') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("insert user A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-rem-b', 'remb@b.c', 'Bia') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("insert user B: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Rem') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userA).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3) RETURNING id`, wsID, projID, userA).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// status default 'pending' (CA-01): insert sem o campo → pending.
	var remID, status string
	if err := pool.QueryRow(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		 VALUES ($1, $2, $3, now(), 'manual') RETURNING id, status`,
		userA, taskID, wsID).Scan(&remID, &status); err != nil {
		t.Fatalf("insert reminder com default de status: %v", err)
	}
	if status != "pending" {
		t.Errorf("status default deveria ser pending, veio %q", status)
	}

	// CHECK de kind: valor fora do enum é rejeitado.
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		 VALUES ($1, $2, $3, now(), 'lembrete')`, userA, taskID, wsID); err == nil {
		t.Errorf("kind fora do enum (due|manual) deveria violar o CHECK")
	}
	// CHECK de status: valor fora do enum é rejeitado.
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind, status)
		 VALUES ($1, $2, $3, now(), 'manual', 'enviado')`, userA, taskID, wsID); err == nil {
		t.Errorf("status fora do enum deveria violar o CHECK")
	}

	// CA-02 — índice parcial idx_reminders_pending presente E parcial (tem
	// indpred). Consultado pelo OID resolvido da relação reminders do schema
	// corrente (search_path), não pelo pg_indexes que varre TODOS os schemas
	// (cross-schema flake quando um schema irmão é dropado em paralelo).
	var hasPred bool
	if err := pool.QueryRow(ctx, `
		SELECT i.indpred IS NOT NULL
		  FROM pg_index i
		 WHERE i.indrelid = 'reminders'::regclass
		   AND i.indexrelid = to_regclass('idx_reminders_pending')`).Scan(&hasPred); err != nil {
		t.Fatalf("índice idx_reminders_pending não encontrado no schema corrente: %v", err)
	}
	if !hasPred {
		t.Errorf("idx_reminders_pending deveria ser PARCIAL (WHERE status='pending')")
	}

	// idx_reminders_user presente (CA-20 — listagem por user).
	if err := pool.QueryRow(ctx, `
		SELECT 1
		  FROM pg_index i
		 WHERE i.indrelid = 'reminders'::regclass
		   AND i.indexrelid = to_regclass('idx_reminders_user')`).Scan(new(int)); err != nil {
		t.Fatalf("índice idx_reminders_user não encontrado: %v", err)
	}

	// CA-03 — unique parcial due-pending: 2 inserts due pendentes da MESMA task →
	// o 2º viola o unique. Outras tasks/kinds/status não colidem.
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		 VALUES ($1, $2, $3, now(), 'due')`, userA, taskID, wsID); err != nil {
		t.Fatalf("1º reminder due pendente: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		 VALUES ($1, $2, $3, now(), 'due')`, userB, taskID, wsID); err == nil {
		t.Errorf("2º reminder due pendente da MESMA task deveria violar o unique parcial")
	}
	// mas um due CANCELED da mesma task NÃO colide (o unique é só pending).
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind, status)
		 VALUES ($1, $2, $3, now(), 'due', 'canceled')`, userA, taskID, wsID); err != nil {
		t.Errorf("due canceled da mesma task NÃO deveria colidir com o pending: %v", err)
	}

	// CA-01 — FK task_id ON DELETE CASCADE: apagar a task limpa os reminders.
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("delete task (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM reminders WHERE task_id = $1`, taskID).Scan(&n); err != nil {
		t.Fatalf("contar reminders após cascade da task: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE da task deveria limpar os reminders, sobraram %d", n)
	}

	// FK user_id ON DELETE CASCADE: apagar o user limpa seus reminders.
	var task2 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T2', $3) RETURNING id`, wsID, projID, userA).Scan(&task2); err != nil {
		t.Fatalf("insert task 2: %v", err)
	}
	var userC string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-rem-c', 'remc@b.c', 'Cau') RETURNING id`).Scan(&userC); err != nil {
		t.Fatalf("insert user C: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		 VALUES ($1, $2, $3, now(), 'manual')`, userC, task2, wsID); err != nil {
		t.Fatalf("insert reminder do user C: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userC); err != nil {
		t.Fatalf("delete user C (cascade): %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM reminders WHERE user_id = $1`, userC).Scan(&n); err != nil {
		t.Fatalf("contar reminders após cascade do user: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE do user deveria limpar os reminders, sobraram %d", n)
	}
}
