package comments_test

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/db/dbtest"
)

// CA-07 — o autor apaga o próprio comentário (204); 2º DELETE do mesmo id →
// NotFound (idempotência: o id já não existe).
func TestDeleteAutor(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	member := seedUser(t, pool, "g-m", "m@b.c", "Member")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, member, "member")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	c, err := svc.Create(ctx, ws, member, "member", task, "meu comentário")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, ws, member, "member", task, c.ID); err != nil {
		t.Fatalf("autor deveria apagar o próprio: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("comentário deveria ter sido removido, sobraram %d", n)
	}
	// 2º DELETE → NotFound.
	if err := svc.Delete(ctx, ws, member, "member", task, c.ID); deCode(err) != "not_found" {
		t.Errorf("2º DELETE do mesmo id: esperado not_found, veio %v", err)
	}
}

// CA-08 — o owner do workspace apaga QUALQUER comentário (de outro autor).
func TestDeleteOwnerApagaQualquer(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	member := seedUser(t, pool, "g-m", "m@b.c", "Member")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, member, "member")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	c, err := svc.Create(ctx, ws, member, "member", task, "comentário do membro")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, ws, owner, "owner", task, c.ID); err != nil {
		t.Fatalf("owner deveria apagar o comentário de outro: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("comentário deveria ter sido removido, sobraram %d", n)
	}
}

// CA-09 — member não-owner não-autor que tenta apagar o de OUTRO → Forbidden;
// o comentário permanece.
func TestDeleteMemberNaoApagaDeOutro(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	m1 := seedUser(t, pool, "g-m1", "m1@b.c", "M1")
	m2 := seedUser(t, pool, "g-m2", "m2@b.c", "M2")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, m1, "member")
	addMember(t, pool, ws, m2, "member")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	c, err := svc.Create(ctx, ws, m1, "member", task, "do m1")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, ws, m2, "member", task, c.ID); deCode(err) != "forbidden" {
		t.Errorf("member apagando o de outro: esperado forbidden, veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 1 {
		t.Fatalf("comentário deveria permanecer, há %d", n)
	}
	// o autor (m1) consegue apagar o próprio (CA-07).
	if err := svc.Delete(ctx, ws, m1, "member", task, c.ID); err != nil {
		t.Errorf("autor deveria apagar o próprio: %v", err)
	}
}

// CA-14.1 — o viewer-assignee NÃO apaga comentário de OUTRO autor na task dele
// (não é owner; o carve-out é só de criar/comentar) → Forbidden. Mas apaga o
// PRÓPRIO (por autoria).
func TestDeleteViewerAssigneeNaoApagaDeTerceiro(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	viewer := seedUser(t, pool, "g-v", "v@b.c", "Viewer")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, viewer, "viewer")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, &viewer) // assignee = viewer

	// o owner comenta na task do viewer.
	cOwner, err := svc.Create(ctx, ws, owner, "owner", task, "comentário do owner")
	if err != nil {
		t.Fatal(err)
	}
	// o viewer-assignee comenta (carve-out, CA-14) — esse ele pode apagar.
	cViewer, err := svc.Create(ctx, ws, viewer, "viewer", task, "comentário do viewer")
	if err != nil {
		t.Fatal(err)
	}

	// viewer NÃO apaga o do owner.
	if err := svc.Delete(ctx, ws, viewer, "viewer", task, cOwner.ID); deCode(err) != "forbidden" {
		t.Errorf("viewer-assignee apagando o de terceiro: esperado forbidden, veio %v", err)
	}
	// viewer apaga o PRÓPRIO (autoria).
	if err := svc.Delete(ctx, ws, viewer, "viewer", task, cViewer.ID); err != nil {
		t.Errorf("viewer-assignee deveria apagar o próprio: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 1 {
		t.Fatalf("só o do owner deveria sobrar, há %d", n)
	}
}

// CA-10 — commentId de OUTRA task, inexistente, ou malformado → NotFound
// (não vaza; o id tem que casar a task da rota).
func TestDeleteComentarioForaDaTask404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	taskA := seedTask(t, pool, ws, proj, owner, nil)
	taskB := seedTask(t, pool, ws, proj, owner, nil)

	c, err := svc.Create(ctx, ws, owner, "owner", taskA, "sou da A")
	if err != nil {
		t.Fatal(err)
	}

	// id real, mas pedindo sob a task B → NotFound (não casa task_id).
	if err := svc.Delete(ctx, ws, owner, "owner", taskB, c.ID); deCode(err) != "not_found" {
		t.Errorf("DELETE de comentário sob a task errada: esperado not_found, veio %v", err)
	}
	// id inexistente → NotFound.
	if err := svc.Delete(ctx, ws, owner, "owner", taskA, "00000000-0000-0000-0000-000000000000"); deCode(err) != "not_found" {
		t.Errorf("DELETE de id inexistente: esperado not_found, veio %v", err)
	}
	// commentId malformado → NotFound (antes do SQL).
	if err := svc.Delete(ctx, ws, owner, "owner", taskA, "nao-uuid"); deCode(err) != "not_found" {
		t.Errorf("DELETE de id malformado: esperado not_found, veio %v", err)
	}
	// o comentário da A permanece (não foi apagado por engano).
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 1 {
		t.Fatalf("comentário da A deveria permanecer, há %d", n)
	}
}

// CA-11/12/16.1 (verificação do build) — o JOIN de VISIBILIDADE do Delete é
// load-bearing: um WS OWNER NÃO enxerga projeto PRIVATE de outro membro
// (projects: private só do dono, sem exceção pro owner do ws), então tentar
// apagar comentário lá dá not_found (não vaza existência, não apaga). Create e
// List já cobrem a visibilidade; sem ESTE teste a mutação que remove o predicado
// de visibilidade do Delete (ex.: `OR true`) sobrevive à suíte (o gate de autoria
// `role=='owner'` curto-circuita e o WS owner apagaria o que não deveria ver).
func TestDeleteTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	wsOwner := seedUser(t, pool, "g-o", "o@b.c", "WS Owner")
	dono := seedUser(t, pool, "g-d", "d@b.c", "Dono Privado")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, wsOwner, "owner")
	addMember(t, pool, ws, dono, "member")
	// projeto PRIVATE do dono (member) — o WS owner NÃO vê (CA-12).
	proj := seedProject(t, pool, ws, dono, "Secreto", "private")
	task := seedTask(t, pool, ws, proj, dono, nil)

	// o dono (vê o próprio private) comenta.
	c, err := svc.Create(ctx, ws, dono, "member", task, "comentário privado")
	if err != nil {
		t.Fatalf("o dono do projeto private deveria comentar: %v", err)
	}

	// o WS OWNER tenta apagar → not_found (não enxerga o projeto private de outro).
	if err := svc.Delete(ctx, ws, wsOwner, "owner", task, c.ID); deCode(err) != "not_found" {
		t.Errorf("WS owner apagando comentário em projeto private de outro: esperado not_found, veio %v", err)
	}
	// o comentário permanece (não vazou nem apagou).
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 1 {
		t.Fatalf("comentário NÃO deveria ter sido apagado, há %d", n)
	}
}
