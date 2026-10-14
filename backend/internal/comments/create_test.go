package comments_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

func deCode(err error) string {
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// CA-02/05 — member numa task shared cria; objeto traz author_name e body
// trimado; author_id/workspace_id/task_id corretos; created_at preenchido.
func TestCreateMemberTaskVisivel(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	member := seedUser(t, pool, "g-m", "m@b.c", "Member Name")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, member, "member")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	c, err := svc.Create(ctx, ws, member, "member", task, "  oi mundo  ")
	if err != nil {
		t.Fatalf("member em task shared deveria criar: %v", err)
	}
	if c.ID == "" || c.TaskID != task || c.WorkspaceID != ws || c.AuthorID != member ||
		c.AuthorName != "Member Name" || c.Body != "oi mundo" || c.CreatedAt.IsZero() {
		t.Errorf("comentário fora do contrato: %+v", c)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 1 {
		t.Fatalf("deveria haver 1 comentário, há %d", n)
	}
}

// CA-03 — body vazio / só-espaço → validation_error; nada gravado.
func TestCreateBodyVazio(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	for _, b := range []string{"", "   ", "\t\n "} {
		_, err := svc.Create(ctx, ws, owner, "owner", task, b)
		if deCode(err) != "validation_error" {
			t.Errorf("Create(%q): esperado validation_error, veio %v", b, err)
		}
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("nenhum comentário deveria persistir, há %d", n)
	}
}

// CA-04 — > 4000 runes → validation_error; exatamente 4000 aceito; contagem
// por rune (emoji conta 1, não por bytes).
func TestCreateLimiteRunes(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil)

	// 4001 runes → 400.
	if _, err := svc.Create(ctx, ws, owner, "owner", task, strings.Repeat("a", 4001)); deCode(err) != "validation_error" {
		t.Errorf("4001 runes: esperado validation_error, veio %v", err)
	}
	// exatamente 4000 → aceito.
	if _, err := svc.Create(ctx, ws, owner, "owner", task, strings.Repeat("b", 4000)); err != nil {
		t.Errorf("4000 runes deveria ser aceito: %v", err)
	}
	// 4000 emojis (4 bytes cada = 16000 bytes) por RUNE → aceito.
	if _, err := svc.Create(ctx, ws, owner, "owner", task, strings.Repeat("😀", 4000)); err != nil {
		t.Errorf("4000 emojis (runes) deveria ser aceito: %v", err)
	}
	// 4001 emojis → 400.
	if _, err := svc.Create(ctx, ws, owner, "owner", task, strings.Repeat("😀", 4001)); deCode(err) != "validation_error" {
		t.Errorf("4001 emojis: esperado validation_error, veio %v", err)
	}
}

// CA-11/15 — member em task de projeto private de OUTRO user → NotFound
// (mesma resposta de inexistente; não vaza). Task inexistente idem.
func TestCreateTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	other := seedUser(t, pool, "g-x", "x@b.c", "Outro")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, other, "member")
	// projeto PRIVADO do owner; other é membro do ws mas não enxerga o projeto.
	priv := seedProject(t, pool, ws, owner, "Privado", "private")
	task := seedTask(t, pool, ws, priv, owner, nil)

	if _, err := svc.Create(ctx, ws, other, "member", task, "tentando"); deCode(err) != "not_found" {
		t.Errorf("member em task private de outro: esperado not_found, veio %v", err)
	}
	// task inexistente (uuid válido mas sem linha) → mesmo not_found.
	if _, err := svc.Create(ctx, ws, owner, "owner", "00000000-0000-0000-0000-000000000000", "x"); deCode(err) != "not_found" {
		t.Errorf("task inexistente: esperado not_found, veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("nada deveria persistir, há %d", n)
	}
}

// CA-13 — viewer numa task visível onde NÃO é assignee (assignee de outro) →
// Forbidden; nada gravado.
func TestCreateViewerNaoAssignee403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	viewer := seedUser(t, pool, "g-v", "v@b.c", "Viewer")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, viewer, "viewer")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	// assignee = owner (não o viewer).
	task := seedTask(t, pool, ws, proj, owner, &owner)

	if _, err := svc.Create(ctx, ws, viewer, "viewer", task, "deixa eu falar"); deCode(err) != "forbidden" {
		t.Errorf("viewer não-assignee: esperado forbidden, veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("nada deveria persistir, há %d", n)
	}
}

// CA-13.1 — gate null-safe: viewer numa task visível com assignee IS NULL (o
// caso NORMAL) → Forbidden, NUNCA 500. Mutação trocando COALESCE por
// `t.assignee = $userID` cru reproduz o 500 (Scan de NULL pra bool) e mata
// este teste.
func TestCreateViewerAssigneeNulo403NaoQuebra(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	viewer := seedUser(t, pool, "g-v", "v@b.c", "Viewer")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, viewer, "viewer")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	task := seedTask(t, pool, ws, proj, owner, nil) // assignee IS NULL

	_, err := svc.Create(ctx, ws, viewer, "viewer", task, "oi")
	if deCode(err) != "forbidden" {
		t.Fatalf("viewer em task com assignee NULL: esperado forbidden (NÃO 500), veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_comments"); n != 0 {
		t.Fatalf("nada deveria persistir, há %d", n)
	}
}

// CA-14 — viewer-assignee comenta na task DELE (201). E só nela: comentar em
// OUTRA task visível onde não é assignee segue Forbidden.
func TestCreateViewerAssigneeOK(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := comments.NewService(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-o", "o@b.c", "Owner")
	viewer := seedUser(t, pool, "g-v", "v@b.c", "Viewer")
	ws := seedWorkspace(t, pool, "WS")
	addMember(t, pool, ws, owner, "owner")
	addMember(t, pool, ws, viewer, "viewer")
	proj := seedProject(t, pool, ws, owner, "Proj", "shared")
	mine := seedTask(t, pool, ws, proj, owner, &viewer) // assignee = viewer
	other := seedTask(t, pool, ws, proj, owner, nil)    // não atribuída ao viewer

	c, err := svc.Create(ctx, ws, viewer, "viewer", mine, "respondendo")
	if err != nil {
		t.Fatalf("viewer-assignee deveria comentar na task dele: %v", err)
	}
	if c.AuthorID != viewer || c.Body != "respondendo" {
		t.Errorf("comentário do viewer-assignee fora do contrato: %+v", c)
	}
	// "só nessa task": outra task visível onde não é assignee → 403.
	if _, err := svc.Create(ctx, ws, viewer, "viewer", other, "tentando em outra"); deCode(err) != "forbidden" {
		t.Errorf("viewer-assignee em OUTRA task: esperado forbidden, veio %v", err)
	}
}
