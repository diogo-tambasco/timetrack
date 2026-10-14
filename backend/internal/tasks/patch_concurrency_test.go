// patch_concurrency_test.go — fase 24 (correção da verificação do build): o popup
// central (TaskPanel) dispara um PATCH POR CAMPO (nome, status, assignee,
// requested_by, …). PATCHes parciais concorrentes na MESMA task NÃO podem se
// atropelar: o Patch faz read-modify-write da LINHA INTEIRA, então sem
// serialização o 2º PATCH lê o estado ANTES do 1º commitar e regrava o campo do
// 1º com o valor velho (lost update — ex.: o PATCH de requested_by zerava o
// assignee recém-setado → a task sumia de /minhas-tarefas). Serializado por
// SELECT … FOR UPDATE em Patch. Roda muitas iterações pra forçar a corrida.
package tasks

import (
	"context"
	"sync"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func TestPatchConcorrentePreservaCamposDistintos(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedAssigned(t, pool)
	ctx := context.Background()

	taskID := mkTaskFull(t, pool, s.wsA, s.projAShared, "concorrente", nil, nil, nil, s.me)

	for i := 0; i < 30; i++ {
		// zera os dois campos antes de cada rodada
		if _, err := pool.Exec(ctx, `UPDATE tasks SET assignee = NULL, requested_by = NULL WHERE id = $1`, taskID); err != nil {
			t.Fatalf("reset: %v", err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		errs := make([]error, 2)
		go func() {
			defer wg.Done()
			_, _, errs[0] = svc.Patch(ctx, s.wsA, s.me, "owner", taskID, PatchInput{AssigneeSet: true, Assignee: &s.me})
		}()
		go func() {
			defer wg.Done()
			_, _, errs[1] = svc.Patch(ctx, s.wsA, s.me, "owner", taskID, PatchInput{RequestedBySet: true, RequestedBy: &s.other})
		}()
		wg.Wait()
		for _, e := range errs {
			if e != nil {
				t.Fatalf("iter %d: patch err: %v", i, e)
			}
		}
		// AMBOS os campos devem persistir — sem lost update
		var assignee, requestedBy *string
		if err := pool.QueryRow(ctx, `SELECT assignee, requested_by FROM tasks WHERE id = $1`, taskID).Scan(&assignee, &requestedBy); err != nil {
			t.Fatalf("read: %v", err)
		}
		if assignee == nil || *assignee != s.me {
			t.Fatalf("iter %d: assignee PERDIDO (lost update): got %v, want %s", i, assignee, s.me)
		}
		if requestedBy == nil || *requestedBy != s.other {
			t.Fatalf("iter %d: requested_by PERDIDO (lost update): got %v, want %s", i, requestedBy, s.other)
		}
	}
}
