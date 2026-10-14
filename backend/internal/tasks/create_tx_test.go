// create_tx_test.go — fase 26 T-04: CreateTx aditivo pro slash /task (26-H4/
// coord. C+D). Prova: rollback não persiste (atomicidade); commit cria com
// status DEFAULT 'todo' (mutação que crave outro valor quebra — 26-H1); nome
// trimado; nome inválido sem persistir; projeto arquivado → 400.
package tasks

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// CreateTx numa tx com ROLLBACK → a task NÃO persiste (base do CA-07).
func TestCreateTxRollbackNaoPersiste(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	task, err := svc.CreateTx(ctx, tx, s.ws, s.user, "owner", s.project, "Na tx")
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if task.ProjectID != s.project || task.Name != "Na tx" {
		t.Errorf("task fora do contrato: %+v", task)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE id = $1`, task.ID).Scan(&n); err != nil {
		t.Fatalf("contar: %v", err)
	}
	if n != 0 {
		t.Errorf("rollback deveria descartar a task, sobraram %d", n)
	}
}

// CreateTx com COMMIT → task com status 'todo' (o DEFAULT). MUTAÇÃO: se o INSERT
// setar 'backlog'/outro valor, o assert status=='todo' DEVE quebrar (26-H1).
func TestCreateTxCommitStatusTodo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	task, err := svc.CreateTx(ctx, tx, s.ws, s.user, "owner", s.project, "  Publicar OKRs  ")
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var status, name string
	if err := pool.QueryRow(ctx, `SELECT status, name FROM tasks WHERE id = $1`, task.ID).Scan(&status, &name); err != nil {
		t.Fatalf("ler task: %v", err)
	}
	if status != "todo" {
		t.Errorf("status do slash deveria ser DEFAULT 'todo', veio %q", status)
	}
	if name != "Publicar OKRs" {
		t.Errorf("nome deveria ser trimado, veio %q", name)
	}
	if task.Status != "todo" {
		t.Errorf("Task.Status devolvida deveria ser 'todo', veio %q", task.Status)
	}
}

// CreateTx com nome inválido → erro de validação sem persistir.
func TestCreateTxNomeInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	tx, _ := pool.Begin(ctx)
	defer tx.Rollback(ctx)
	if _, err := svc.CreateTx(ctx, tx, s.ws, s.user, "owner", s.project, "   "); !isValidation(err) {
		t.Errorf("nome vazio → esperado validation_error, veio %v", err)
	}
}

// CreateTx em projeto arquivado → 400 invalid_reference.
func TestCreateTxProjetoArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `UPDATE projects SET archived_at = now() WHERE id = $1`, s.project); err != nil {
		t.Fatalf("arquivar: %v", err)
	}
	tx, _ := pool.Begin(ctx)
	defer tx.Rollback(ctx)
	if _, err := svc.CreateTx(ctx, tx, s.ws, s.user, "owner", s.project, "x"); !isInvalidRef(err) {
		t.Errorf("CreateTx em projeto arquivado → esperado invalid_reference, veio %v", err)
	}
}
