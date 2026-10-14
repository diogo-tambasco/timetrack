// store_test.go — TDD do repo (fase 20 T-02): Create persiste pending;
// CancelDuePending só o due pendente (não toca sent/manual); ListByUser só os
// do user com task_name (mutação removendo WHERE user_id morre — CA-21); Cancel
// de outro user → found=false (CA-19); Cancel idempotente em já-canceled (CA-18).
package reminders_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reminders"
)

// seedTask cria user + ws (owner) + project + task e devolve (userID, wsID, taskID).
func seedTask(t *testing.T, pool *pgxpool.Pool, googleID, taskName string) (userID, wsID, taskID string) {
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
	var projID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`, wsID, projID, taskName, userID).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return userID, wsID, taskID
}

func statusOf(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM reminders WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("status do reminder %s: %v", id, err)
	}
	return s
}

func TestStoreCreatePersistePending(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := reminders.NewStore(pool)
	u, ws, task := seedTask(t, pool, "g-create", "Task X")
	at := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	r, err := store.Create(context.Background(), pool, reminders.Reminder{
		UserID: u, TaskID: task, WorkspaceID: ws, RemindAt: at, Kind: "manual",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID == "" {
		t.Errorf("Create deveria devolver o id gerado")
	}
	if r.Status != "pending" {
		t.Errorf("reminder novo deveria nascer pending, veio %q", r.Status)
	}
	if statusOf(t, pool, r.ID) != "pending" {
		t.Errorf("reminder não persistiu pending")
	}
}

// CancelDuePending marca canceled SÓ o due pendente; não toca due sent nem manual.
func TestStoreCancelDuePending(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := reminders.NewStore(pool)
	u, ws, task := seedTask(t, pool, "g-canceldue", "T")
	ctx := context.Background()
	at := time.Now().UTC()

	due, _ := store.Create(ctx, pool, reminders.Reminder{UserID: u, TaskID: task, WorkspaceID: ws, RemindAt: at, Kind: "due"})
	manual, _ := store.Create(ctx, pool, reminders.Reminder{UserID: u, TaskID: task, WorkspaceID: ws, RemindAt: at, Kind: "manual"})

	if err := store.CancelDuePending(ctx, pool, task); err != nil {
		t.Fatalf("CancelDuePending: %v", err)
	}
	if got := statusOf(t, pool, due.ID); got != "canceled" {
		t.Errorf("o due pendente deveria virar canceled, veio %q", got)
	}
	if got := statusOf(t, pool, manual.ID); got != "pending" {
		t.Errorf("CancelDuePending NÃO deveria tocar o manual, veio %q", got)
	}

	// idempotente: 2ª chamada sem due pendente é no-op (não explode).
	if err := store.CancelDuePending(ctx, pool, task); err != nil {
		t.Fatalf("CancelDuePending idempotente: %v", err)
	}
}

// ListByUser só vê os do user e traz task_name; isolamento por user (CA-21).
func TestStoreListByUserIsolamento(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := reminders.NewStore(pool)
	ctx := context.Background()
	uA, wsA, taskA := seedTask(t, pool, "g-list-a", "Tarefa da Ana")
	uB, wsB, taskB := seedTask(t, pool, "g-list-b", "Tarefa da Bia")

	earlier := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	later := time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)
	store.Create(ctx, pool, reminders.Reminder{UserID: uA, TaskID: taskA, WorkspaceID: wsA, RemindAt: later, Kind: "manual"})
	store.Create(ctx, pool, reminders.Reminder{UserID: uA, TaskID: taskA, WorkspaceID: wsA, RemindAt: earlier, Kind: "due"})
	store.Create(ctx, pool, reminders.Reminder{UserID: uB, TaskID: taskB, WorkspaceID: wsB, RemindAt: earlier, Kind: "manual"})

	list, err := store.ListByUser(ctx, uA, "pending")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("A deveria ter 2 reminders, veio %d", len(list))
	}
	// CA-21: nenhum reminder do user B aparece.
	for _, r := range list {
		if r.UserID != uA {
			t.Errorf("ListByUser de A vazou reminder do user %s (CA-21)", r.UserID)
		}
		if r.TaskName != "Tarefa da Ana" {
			t.Errorf("task_name esperado 'Tarefa da Ana', veio %q", r.TaskName)
		}
	}
	// ordenado por remind_at: o earlier (due) vem antes.
	if !list[0].RemindAt.Equal(earlier) {
		t.Errorf("ListByUser deveria ordenar por remind_at (earlier primeiro)")
	}

	// B só vê o seu.
	listB, _ := store.ListByUser(ctx, uB, "pending")
	if len(listB) != 1 || listB[0].UserID != uB {
		t.Errorf("B deveria ver só o seu reminder, veio %d", len(listB))
	}

	// filtro por status: sent vazio → [].
	sent, err := store.ListByUser(ctx, uA, "sent")
	if err != nil {
		t.Fatalf("ListByUser sent: %v", err)
	}
	if sent == nil || len(sent) != 0 {
		t.Errorf("ListByUser sent deveria ser [] não-nil, veio %v", sent)
	}

	// status default = pending (vazio).
	def, _ := store.ListByUser(ctx, uA, "")
	if len(def) != 2 {
		t.Errorf("ListByUser('') deveria default pra pending (2), veio %d", len(def))
	}

	// status inválido → 400.
	if _, err := store.ListByUser(ctx, uA, "lixo"); err == nil {
		t.Errorf("ListByUser com status inválido deveria dar erro de validação")
	}
}

// Cancel: do dono pending → found+canceled; de outro user → found=false (CA-19);
// idempotente em já-canceled (CA-18); coerência com a task da URL.
func TestStoreCancel(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := reminders.NewStore(pool)
	ctx := context.Background()
	uA, wsA, taskA := seedTask(t, pool, "g-cancel-a", "T")
	uB, _, _ := seedTask(t, pool, "g-cancel-b", "T")

	r, _ := store.Create(ctx, pool, reminders.Reminder{UserID: uA, TaskID: taskA, WorkspaceID: wsA, RemindAt: time.Now().UTC(), Kind: "manual"})

	// outro user tentando cancelar → found=false, não toca o reminder (CA-19).
	found, err := store.Cancel(ctx, uB, r.ID, "")
	if err != nil {
		t.Fatalf("Cancel outro user: %v", err)
	}
	if found {
		t.Errorf("Cancel de OUTRO user deveria devolver found=false (CA-19)")
	}
	if statusOf(t, pool, r.ID) != "pending" {
		t.Errorf("Cancel de outro user NÃO deveria tocar o reminder")
	}

	// dono cancela → found=true, vira canceled (CA-18).
	found, err = store.Cancel(ctx, uA, r.ID, "")
	if err != nil {
		t.Fatalf("Cancel dono: %v", err)
	}
	if !found {
		t.Errorf("Cancel do dono deveria devolver found=true")
	}
	if statusOf(t, pool, r.ID) != "canceled" {
		t.Errorf("Cancel do dono deveria virar canceled")
	}

	// idempotente: cancelar de novo → ainda found=true (existe), status segue canceled.
	found, err = store.Cancel(ctx, uA, r.ID, "")
	if err != nil {
		t.Fatalf("Cancel idempotente: %v", err)
	}
	if !found {
		t.Errorf("Cancel idempotente de já-canceled deveria devolver found=true (204)")
	}

	// inexistente → found=false.
	found, _ = store.Cancel(ctx, uA, "00000000-0000-0000-0000-000000000000", "")
	if found {
		t.Errorf("Cancel de id inexistente deveria devolver found=false")
	}
	// id malformado → found=false (404 não-vaza).
	found, _ = store.Cancel(ctx, uA, "nao-é-uuid", "")
	if found {
		t.Errorf("Cancel de id malformado deveria devolver found=false")
	}

	// coerência com a task da URL: reminder de taskA, mas taskID errado → found=false.
	r2, _ := store.Create(ctx, pool, reminders.Reminder{UserID: uA, TaskID: taskA, WorkspaceID: wsA, RemindAt: time.Now().UTC(), Kind: "manual"})
	found, _ = store.Cancel(ctx, uA, r2.ID, "11111111-1111-1111-1111-111111111111")
	if found {
		t.Errorf("Cancel com taskID que não bate deveria devolver found=false")
	}
	if statusOf(t, pool, r2.ID) != "pending" {
		t.Errorf("Cancel com taskID errado NÃO deveria cancelar")
	}

	// (verificação do build) taskID MALFORMADO na URL → found=false (404), e
	// NÃO 500 do cast $3::uuid estourando no Postgres (22P02 — CA-19 não-vaza).
	found, err = store.Cancel(ctx, uA, r2.ID, "nao-é-uuid")
	if err != nil {
		t.Fatalf("Cancel com taskID malformado NÃO deveria dar erro (500), e sim found=false: %v", err)
	}
	if found {
		t.Errorf("Cancel com taskID malformado deveria devolver found=false (404)")
	}
	if statusOf(t, pool, r2.ID) != "pending" {
		t.Errorf("Cancel com taskID malformado NÃO deveria cancelar")
	}
}
