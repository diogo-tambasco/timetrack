package comments_test

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/db/dbtest"
)

// CA-06 — N comentários voltam ASC por created_at (desempate id), com
// author_name; lista vazia = [] não-nil.
func TestListOrdenadoASC(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	// thread vazia → [] (não nil).
	empty, err := svc.List(ctx, ws, owner, task)
	if err != nil {
		t.Fatalf("List vazia: %v", err)
	}
	if empty == nil {
		t.Fatal("List vazia deveria devolver [] não-nil")
	}
	if len(empty) != 0 {
		t.Fatalf("List vazia deveria ter 0 itens, tem %d", len(empty))
	}

	// 3 comentários em ordem; created_at via now() default — inserimos com
	// pausa lógica via ordem de criação pela API.
	bodies := []string{"primeiro", "segundo", "terceiro"}
	for _, b := range bodies {
		if _, err := svc.Create(ctx, ws, owner, "owner", task, b); err != nil {
			t.Fatalf("criar %q: %v", b, err)
		}
	}

	list, err := svc.List(ctx, ws, owner, task)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("esperado 3 comentários, veio %d", len(list))
	}
	for i, c := range list {
		if c.Body != bodies[i] {
			t.Errorf("ordem ASC errada na posição %d: esperado %q, veio %q", i, bodies[i], c.Body)
		}
		if c.AuthorName != "Owner" {
			t.Errorf("author_name esperado Owner, veio %q", c.AuthorName)
		}
	}
}

// CA-06.1 — a lista NUNCA inclui comentário de OUTRA task (mesmo do mesmo
// workspace/projeto). Mutação removendo o WHERE task_id morre aqui.
func TestListSoDaTask(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	taskA := seedTask(t, pool, ws, proj, owner, nil)
	taskB := seedTask(t, pool, ws, proj, owner, nil)

	if _, err := svc.Create(ctx, ws, owner, "owner", taskA, "sou da A"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, ws, owner, "owner", taskB, "sou da B"); err != nil {
		t.Fatal(err)
	}

	listA, err := svc.List(ctx, ws, owner, taskA)
	if err != nil {
		t.Fatal(err)
	}
	if len(listA) != 1 || listA[0].Body != "sou da A" {
		t.Fatalf("List(A) deveria ter só o comentário da A, veio %+v", listA)
	}
	listB, err := svc.List(ctx, ws, owner, taskB)
	if err != nil {
		t.Fatal(err)
	}
	if len(listB) != 1 || listB[0].Body != "sou da B" {
		t.Fatalf("List(B) deveria ter só o comentário da B, veio %+v", listB)
	}
}

// CA-12 — List de task private de OUTRO user → NotFound (não vaza); uuid
// malformado → NotFound.
func TestListTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	other := seedUser(t, pool, "g-x", "x@b.c", "Outro")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, other, "member")
	priv := seedProject(t, pool, ws, owner, "Privado", "private")
	task := seedTask(t, pool, ws, priv, owner, nil)
	if _, err := svc.Create(ctx, ws, owner, "owner", task, "secreto"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.List(ctx, ws, other, task); deCode(err) != "not_found" {
		t.Errorf("List de task private de outro: esperado not_found, veio %v", err)
	}
	if _, err := svc.List(ctx, ws, owner, "nao-e-uuid"); deCode(err) != "not_found" {
		t.Errorf("List com taskID malformado: esperado not_found, veio %v", err)
	}
}
