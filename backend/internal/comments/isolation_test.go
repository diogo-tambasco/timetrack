package comments_test

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/db/dbtest"
)

// CA-16/16.1 — isolamento multi-tenant: um comentário do workspace A NUNCA
// aparece numa requisição do workspace B (mesmo com o user membro dos dois).
// Toda query do service filtra workspace_id; a mutação removendo o WHERE
// workspace_id (ou o JOIN de visibilidade) mata um teste daqui.
func TestIsolamentoEntreWorkspaces(t *testing.T) {
	poolDB := dbtest.NewPool(t)
	svc := comments.NewService(poolDB)
	ctx := context.Background()

	user := seedUser(t, poolDB, "g-u", "u@b.c", "User")
	wsA := seedWorkspace(t, poolDB, "WS A")
	wsB := seedWorkspace(t, poolDB, "WS B")
	addMember(t, poolDB, wsA, user, "owner")
	addMember(t, poolDB, wsB, user, "owner")
	projA := seedProject(t, poolDB, wsA, user, "Proj A", "shared")
	taskA := seedTask(t, poolDB, wsA, projA, user, nil)

	c, err := svc.Create(ctx, wsA, user, "owner", taskA, "comentário do A")
	if err != nil {
		t.Fatalf("criar no A: %v", err)
	}

	// Create com o taskId do A sob o workspace_id do B → NotFound (a task A não
	// existe no ws B).
	if _, err := svc.Create(ctx, wsB, user, "owner", taskA, "tentando no B"); deCode(err) != "not_found" {
		t.Errorf("Create taskA sob wsB: esperado not_found, veio %v", err)
	}
	// List com o taskId do A sob o ws B → NotFound (não vaza a thread do A).
	if _, err := svc.List(ctx, wsB, user, taskA); deCode(err) != "not_found" {
		t.Errorf("List taskA sob wsB: esperado not_found, veio %v", err)
	}
	// Delete do comentário do A sob o ws B → NotFound.
	if err := svc.Delete(ctx, wsB, user, "owner", taskA, c.ID); deCode(err) != "not_found" {
		t.Errorf("Delete commentA sob wsB: esperado not_found, veio %v", err)
	}

	// o comentário segue no A (nada foi apagado por engano) e List(A) o vê.
	listA, err := svc.List(ctx, wsA, user, taskA)
	if err != nil {
		t.Fatal(err)
	}
	if len(listA) != 1 || listA[0].ID != c.ID {
		t.Fatalf("List(A) deveria ter só o comentário do A, veio %+v", listA)
	}
	if n := dbtest.CountRows(t, poolDB, "task_comments"); n != 1 {
		t.Fatalf("o comentário do A deveria permanecer, há %d", n)
	}
}
