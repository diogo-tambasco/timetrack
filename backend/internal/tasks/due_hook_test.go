// due_hook_test.go — fase 20 T-05: a criação/cancelamento do reminder due na
// escrita da task, com o Hook REAL ligado (CA-11/12/12.1/13/14/15/28). Black-box
// (package tasks_test) pra importar reminders sem ciclo. O gatilho é a due_date
// MUDAR DE VALOR (não a presença do campo — CA-12.1).
package tasks_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reminders"
	"timetrack.live/backend/internal/tasks"
)

type dueSeed struct {
	ws, user, project string
	svc               *tasks.Service
}

func seedDue(t *testing.T, pool *pgxpool.Pool) dueSeed {
	t.Helper()
	ctx := context.Background()
	var s dueSeed
	scan := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
		return id
	}
	s.ws = scan(`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`)
	s.user = scan(`INSERT INTO users (google_id, email, name) VALUES ('u-due', 'udue@b.c', 'U') RETURNING id`)
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, s.ws, s.user); err != nil {
		t.Fatalf("seed ws member: %v", err)
	}
	s.project = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'P', '#4f9dde', 'shared', $2) RETURNING id`, s.ws, s.user)
	s.svc = tasks.NewService(pool)
	s.svc.SetReminderHook(reminders.NewHook(reminders.NewStore(pool)))
	return s
}

// dueReminders carrega os reminders due da task (qualquer status), ordenados por created_at.
func dueReminders(t *testing.T, pool *pgxpool.Pool, taskID string) []reminders.Reminder {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id, user_id, task_id, workspace_id, remind_at, kind, status
		  FROM reminders WHERE task_id = $1 AND kind = 'due'
		 ORDER BY created_at, id`, taskID)
	if err != nil {
		t.Fatalf("dueReminders: %v", err)
	}
	defer rows.Close()
	var out []reminders.Reminder
	for rows.Next() {
		var r reminders.Reminder
		if err := rows.Scan(&r.ID, &r.UserID, &r.TaskID, &r.WorkspaceID, &r.RemindAt, &r.Kind, &r.Status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func ptr(s string) *string { return &s }

// CA-11: task nasce com due_date → 1 reminder due pending pro created_by, remind_at = 09:00 local.
func TestDueNasceNoCreate(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()

	task, _, err := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "Com prazo", DueDate: ptr("2026-06-25"), TZ: "America/Sao_Paulo",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	pending := 0
	for _, r := range rs {
		if r.Status == "pending" {
			pending++
			if r.UserID != s.user {
				t.Errorf("due deveria ser pro created_by, veio %s", r.UserID)
			}
			want := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) // 09:00 SP = 12:00Z
			if !r.RemindAt.Equal(want) {
				t.Errorf("remind_at deveria ser %s (09:00 SP), veio %s", want, r.RemindAt.UTC())
			}
		}
	}
	if pending != 1 {
		t.Fatalf("deveria existir exatamente 1 due pending, veio %d", pending)
	}
}

// CA-28: Create SEM due_date não dispara nada (nenhum reminder due).
func TestDueNaoNasceSemDueDate(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	task, _, err := s.svc.Create(context.Background(), s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "Sem prazo",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rs := dueReminders(t, pool, task.ID); len(rs) != 0 {
		t.Errorf("task sem due_date não deveria gerar reminder due, veio %d", len(rs))
	}
}

// CA-12: Patch que MUDA a due_date pra OUTRA data → cancela o antigo, cria 1 novo, nunca 2 pendentes.
func TestDuePatchMudaData(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	task, _, _ := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "T", DueDate: ptr("2026-06-25"), TZ: "UTC",
	})
	// muda pra 2026-07-01
	if _, _, err := s.svc.Patch(ctx, s.ws, s.user, "owner", task.ID, tasks.PatchInput{
		DueDateSet: true, DueDate: ptr("2026-07-01"), TZ: "UTC",
	}); err != nil {
		t.Fatalf("Patch muda data: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	pending, canceled := 0, 0
	for _, r := range rs {
		switch r.Status {
		case "pending":
			pending++
			if !r.RemindAt.Equal(time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)) {
				t.Errorf("novo due deveria ter remind_at na data nova, veio %s", r.RemindAt.UTC())
			}
		case "canceled":
			canceled++
		}
	}
	if pending != 1 || canceled != 1 {
		t.Errorf("esperava 1 pending + 1 canceled, veio %d pending / %d canceled (CA-12)", pending, canceled)
	}
}

// CA-12.1: Patch que reescreve a MESMA due_date com o due antigo já SENT → NENHUM
// reminder novo; o sent permanece; nada re-dispara.
func TestDuePatchMesmaDataPreservaSent(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	task, _, _ := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "T", DueDate: ptr("2026-06-25"), TZ: "UTC",
	})
	// simula que o worker já despachou o due (vira sent).
	if _, err := pool.Exec(ctx,
		`UPDATE reminders SET status = 'sent' WHERE task_id = $1 AND kind = 'due'`, task.ID); err != nil {
		t.Fatalf("marcar sent: %v", err)
	}

	// Patch reescrevendo a MESMA due_date (e mexendo noutro campo qualquer) → NO-OP no reminder.
	if _, _, err := s.svc.Patch(ctx, s.ws, s.user, "owner", task.ID, tasks.PatchInput{
		Name: ptr("T editada"), DueDateSet: true, DueDate: ptr("2026-06-25"), TZ: "UTC",
	}); err != nil {
		t.Fatalf("Patch mesma data: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	if len(rs) != 1 || rs[0].Status != "sent" {
		t.Errorf("re-setar a MESMA due_date deveria preservar SÓ o due sent, veio %+v (CA-12.1)", rs)
	}

	// E uma RunOnce não re-dispara (não há pending no passado).
	var s2 spyNotice
	d := reminders.NewDispatcher(pool,
		reminders.WithClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }),
		reminders.WithOnNotice(s2.add))
	done, _ := d.RunOnce(ctx)
	if len(done) != 0 || s2.n != 0 {
		t.Errorf("nenhum aviso deveria re-disparar após re-setar a mesma data (CA-12.1), veio %d", s2.n)
	}
}

type spyNotice struct{ n int }

func (s *spyNotice) add(string, any) { s.n++ }

// CA-13: Patch due_date → null cancela o due pendente, não cria novo. due sent intacto.
func TestDuePatchLimpaCancela(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	task, _, _ := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "T", DueDate: ptr("2026-06-25"), TZ: "UTC",
	})
	if _, _, err := s.svc.Patch(ctx, s.ws, s.user, "owner", task.ID, tasks.PatchInput{
		DueDateSet: true, DueDate: nil, TZ: "UTC", // limpa
	}); err != nil {
		t.Fatalf("Patch limpa: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	for _, r := range rs {
		if r.Status == "pending" {
			t.Errorf("limpar a due_date deveria cancelar o due pendente (CA-13), achei pending")
		}
	}
	if len(rs) != 1 || rs[0].Status != "canceled" {
		t.Errorf("esperava 1 due canceled após limpar, veio %+v", rs)
	}
}

// CA-12.1 (null↔null): Patch due_date→null numa task SEM due = NO-OP no reminder.
func TestDuePatchNullParaNullNoop(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	task, _, _ := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "Sem prazo",
	})
	if _, _, err := s.svc.Patch(ctx, s.ws, s.user, "owner", task.ID, tasks.PatchInput{
		DueDateSet: true, DueDate: nil, TZ: "UTC",
	}); err != nil {
		t.Fatalf("Patch null→null: %v", err)
	}
	if rs := dueReminders(t, pool, task.ID); len(rs) != 0 {
		t.Errorf("null→null não deveria criar nem cancelar nada, veio %d", len(rs))
	}
}

// CA-14: assignee setado (SQL) → o due vai pro assignee, não pro created_by.
func TestDueRespeitaAssignee(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	// 2º membro do ws (assignee).
	var assignee string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('u-assignee', 'ua@b.c', 'A') RETURNING id`).Scan(&assignee); err != nil {
		t.Fatalf("insert assignee: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, s.ws, assignee); err != nil {
		t.Fatalf("add assignee member: %v", err)
	}
	task, _, err := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "Pra A", DueDate: ptr("2026-06-25"), TZ: "UTC", Assignee: &assignee,
	})
	if err != nil {
		t.Fatalf("Create com assignee: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	if len(rs) != 1 || rs[0].UserID != assignee {
		t.Errorf("due deveria ir pro assignee %s, veio %+v (CA-14)", assignee, rs)
	}
}

// CA-15: due_date no passado → due pending no passado, despachado na 1ª RunOnce.
func TestDuePassadoDisparaJa(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedDue(t, pool)
	ctx := context.Background()
	task, _, err := s.svc.Create(ctx, s.ws, s.user, "owner", tasks.CreateInput{
		ProjectID: s.project, Name: "Vencida", DueDate: ptr("2020-01-01"), TZ: "UTC",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rs := dueReminders(t, pool, task.ID)
	if len(rs) != 1 || rs[0].Status != "pending" {
		t.Fatalf("due de data passada deveria nascer pending, veio %+v", rs)
	}
	var s2 spyNotice
	d := reminders.NewDispatcher(pool, reminders.WithOnNotice(s2.add)) // clock real → agora > 2020
	done, _ := d.RunOnce(ctx)
	if len(done) != 1 || s2.n != 1 {
		t.Errorf("due vencido deveria ser despachado na 1ª RunOnce (CA-15), veio %d", len(done))
	}
}
