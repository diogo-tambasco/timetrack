// service_test.go — fase 24 T-04: CRUD de sub-tasks ("goals da task") + gate
// (loadTaskGate: visibilidade→404, escrita→403, arquivado→400). White-box.
// O gate é cópia da lógica do board da fase 12 (D-05/D-08): subtasks NÃO importa
// tasks/projects/objectives.
package subtasks

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

func code(err error) string {
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

func bptr(b bool) *bool     { return &b }
func sptr(s string) *string { return &s }
func ipt(i int) *int        { return &i }

type seed struct {
	ws, owner, member, viewer, outsider string
	project, archived, privateP         string // projetos: shared / arquivado / private do owner
	task, taskArchived, taskPrivate     string
}

func mkSeed(t *testing.T, pool *pgxpool.Pool) seed {
	t.Helper()
	ctx := context.Background()
	var s seed
	scan := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
		return id
	}
	s.ws = scan(`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`)
	s.owner = scan(`INSERT INTO users (google_id, email, name) VALUES ('own', 'own@b.c', 'Own') RETURNING id`)
	s.member = scan(`INSERT INTO users (google_id, email, name) VALUES ('mem', 'mem@b.c', 'Mem') RETURNING id`)
	s.viewer = scan(`INSERT INTO users (google_id, email, name) VALUES ('vie', 'vie@b.c', 'Vie') RETURNING id`)
	s.outsider = scan(`INSERT INTO users (google_id, email, name) VALUES ('out', 'out@b.c', 'Out') RETURNING id`)
	exec := func(q string, args ...any) {
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("seed exec (%s): %v", q, err)
		}
	}
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1,$2,'owner')`, s.ws, s.owner)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1,$2,'member')`, s.ws, s.member)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1,$2,'viewer')`, s.ws, s.viewer)

	s.project = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1,'P','#4f9dde','shared',$2) RETURNING id`, s.ws, s.owner)
	s.archived = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id, archived_at)
		VALUES ($1,'Arq','#000000','shared',$2, now()) RETURNING id`, s.ws, s.owner)
	s.privateP = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1,'Priv','#111111','private',$2) RETURNING id`, s.ws, s.owner)

	s.task = scan(`INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'T',$3) RETURNING id`, s.ws, s.project, s.owner)
	s.taskArchived = scan(`INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'TA',$3) RETURNING id`, s.ws, s.archived, s.owner)
	s.taskPrivate = scan(`INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'TP',$3) RETURNING id`, s.ws, s.privateP, s.owner)
	return s
}

// CA-06/07/10: create + list (ordem por position) + título trim.
func TestCreateListOrdem(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()

	a, err := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "  Primeira  "})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if a.Title != "Primeira" {
		t.Errorf("title deveria ser trimado, veio %q", a.Title)
	}
	if a.Done || a.IsGoal {
		t.Errorf("done/is_goal deveriam nascer false")
	}
	if a.Position != 0 {
		t.Errorf("1ª sub-task deveria ter position 0, veio %d", a.Position)
	}
	b, err := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "Segunda"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if b.Position != 1 {
		t.Errorf("2ª sub-task deveria ter position 1, veio %d", b.Position)
	}

	list, err := svc.List(ctx, s.ws, s.owner, s.task)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Errorf("lista deveria vir ordenada por position [A,B], veio %+v", list)
	}
}

// CA-06.1: título vazio/só-espaços/>500 → 400.
func TestTituloInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()

	for _, bad := range []string{"", "   ", strings.Repeat("a", 501)} {
		if _, err := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: bad}); code(err) != "validation_error" {
			t.Errorf("título %q deveria dar validation_error, veio %v", bad, err)
		}
	}
	a, err := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "ok"})
	if err != nil {
		t.Fatalf("create base: %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.owner, "owner", s.task, a.ID, PatchInput{Title: sptr("   ")}); code(err) != "validation_error" {
		t.Errorf("patch título vazio deveria dar validation_error, veio %v", err)
	}
}

// CA-08: patch done/is_goal/title/position.
func TestPatch(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	a, _ := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "x"})

	upd, err := svc.Patch(ctx, s.ws, s.owner, "owner", s.task, a.ID, PatchInput{Done: bptr(true), IsGoal: bptr(true)})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if !upd.Done || !upd.IsGoal {
		t.Errorf("done/is_goal deveriam virar true, veio %+v", upd)
	}
	upd, err = svc.Patch(ctx, s.ws, s.owner, "owner", s.task, a.ID, PatchInput{Title: sptr("y"), Position: ipt(3)})
	if err != nil {
		t.Fatalf("patch2: %v", err)
	}
	if upd.Title != "y" || upd.Position != 3 {
		t.Errorf("title/position deveriam atualizar, veio %+v", upd)
	}
	// done preservado (tri-state por presença): patch só do title não zera done.
	if !upd.Done {
		t.Errorf("done deveria ser preservado num patch que não o toca")
	}
}

// CA-09: delete → linha some.
func TestDelete(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	a, _ := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "x"})
	if err := svc.Delete(ctx, s.ws, s.owner, "owner", s.task, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "task_subtasks"); n != 0 {
		t.Errorf("sub-task deveria sumir, sobraram %d", n)
	}
}

// CA-11: escrita por access=view e por viewer do ws → 403.
func TestEscritaSemPermissao403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	// member com access=view no projeto → 403.
	if _, err := pool.Exec(ctx, `INSERT INTO project_members (project_id, user_id, access) VALUES ($1,$2,'view')`, s.project, s.member); err != nil {
		t.Fatalf("seed access view: %v", err)
	}
	if _, err := svc.Create(ctx, s.ws, s.member, "member", s.task, CreateInput{Title: "x"}); code(err) != "forbidden" {
		t.Errorf("access=view deveria dar 403, veio %v", err)
	}
	// viewer do workspace (sem membership no projeto) → 403.
	if _, err := svc.Create(ctx, s.ws, s.viewer, "viewer", s.task, CreateInput{Title: "x"}); code(err) != "forbidden" {
		t.Errorf("viewer do ws deveria dar 403, veio %v", err)
	}
}

// CA-11: PATCH e DELETE por access=view / viewer-do-ws também → 403 (não só Create).
// O middleware RequireWriteRole (mount Core, decisão F) só barra ws-viewer; contra um
// MEMBRO com project access=view o gate fino do service é a ÚNICA autoridade → estes
// testes matam a mutação que troca requireWrite true→false em Patch/Delete.
func TestEscritaPatchDeleteSemPermissao403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	a, err := svc.Create(ctx, s.ws, s.owner, "owner", s.task, CreateInput{Title: "base"})
	if err != nil {
		t.Fatalf("create base: %v", err)
	}
	// member com access=view no projeto → PATCH e DELETE 403.
	if _, err := pool.Exec(ctx, `INSERT INTO project_members (project_id, user_id, access) VALUES ($1,$2,'view')`, s.project, s.member); err != nil {
		t.Fatalf("seed access view: %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.member, "member", s.task, a.ID, PatchInput{Done: bptr(true)}); code(err) != "forbidden" {
		t.Errorf("PATCH por access=view deveria dar 403, veio %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.member, "member", s.task, a.ID); code(err) != "forbidden" {
		t.Errorf("DELETE por access=view deveria dar 403, veio %v", err)
	}
	// viewer do ws (sem membership no projeto) → PATCH e DELETE 403.
	if _, err := svc.Patch(ctx, s.ws, s.viewer, "viewer", s.task, a.ID, PatchInput{Done: bptr(true)}); code(err) != "forbidden" {
		t.Errorf("PATCH por viewer do ws deveria dar 403, veio %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.viewer, "viewer", s.task, a.ID); code(err) != "forbidden" {
		t.Errorf("DELETE por viewer do ws deveria dar 403, veio %v", err)
	}
}

// decisão G: PATCH e DELETE de sub-task em projeto ARQUIVADO → 400 invalid_reference
// (não só Create). A sub-task é inserida direto (o projeto pode ter sido arquivado
// DEPOIS de criada) → mata a mutação que troca blockArchived true→false em Patch/Delete.
func TestProjetoArquivadoPatchDelete400(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	var subID string
	if err := pool.QueryRow(ctx, `INSERT INTO task_subtasks (task_id, workspace_id, title, position) VALUES ($1,$2,'antiga',0) RETURNING id`, s.taskArchived, s.ws).Scan(&subID); err != nil {
		t.Fatalf("seed sub em task arquivada: %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.owner, "owner", s.taskArchived, subID, PatchInput{Done: bptr(true)}); code(err) != "invalid_reference" {
		t.Errorf("PATCH em projeto arquivado deveria dar 400 invalid_reference, veio %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.owner, "owner", s.taskArchived, subID); code(err) != "invalid_reference" {
		t.Errorf("DELETE em projeto arquivado deveria dar 400 invalid_reference, veio %v", err)
	}
}

// D-10/CA-11: viewer que é ASSIGNEE da task NÃO cria sub-task (sem carve-out) → 403.
func TestViewerAssigneeNaoCriaSubtask403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE tasks SET assignee = $2 WHERE id = $1`, s.task, s.viewer); err != nil {
		t.Fatalf("seed assignee: %v", err)
	}
	if _, err := svc.Create(ctx, s.ws, s.viewer, "viewer", s.task, CreateInput{Title: "x"}); code(err) != "forbidden" {
		t.Errorf("viewer-assignee NÃO deveria criar sub-task (sem carve-out — D-10), esperado 403, veio %v", err)
	}
}

// decisão G: escrita em task de projeto ARQUIVADO → 400 invalid_reference.
func TestProjetoArquivado400(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	if _, err := svc.Create(ctx, s.ws, s.owner, "owner", s.taskArchived, CreateInput{Title: "x"}); code(err) != "invalid_reference" {
		t.Errorf("projeto arquivado deveria dar 400 invalid_reference, veio %v", err)
	}
}

// CA-13: task invisível (private de outro) → 404.
func TestTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	// outsider não enxerga o projeto private do owner → 404 (não vaza 403).
	if _, err := svc.Create(ctx, s.ws, s.outsider, "member", s.taskPrivate, CreateInput{Title: "x"}); code(err) != "not_found" {
		t.Errorf("task private de outro deveria dar 404, veio %v", err)
	}
	if _, err := svc.List(ctx, s.ws, s.outsider, s.taskPrivate); code(err) != "not_found" {
		t.Errorf("list de task invisível deveria dar 404, veio %v", err)
	}
}

// CA-13: taskId/subtaskId malformado → 404 (não 500).
func TestUUIDMalformado404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	if _, err := svc.List(ctx, s.ws, s.owner, "nao-uuid"); code(err) != "not_found" {
		t.Errorf("taskId malformado deveria dar 404, veio %v", err)
	}
	if _, err := svc.Create(ctx, s.ws, s.owner, "owner", "nao-uuid", CreateInput{Title: "x"}); code(err) != "not_found" {
		t.Errorf("create com taskId malformado deveria dar 404, veio %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.owner, "owner", s.task, "nao-uuid", PatchInput{Done: bptr(true)}); code(err) != "not_found" {
		t.Errorf("subtaskId malformado deveria dar 404, veio %v", err)
	}
}

// CA-13 (pertencimento): sub-task da task B alcançada via rota da task A → 404.
func TestSubtaskCrossTask404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	// 2ª task no mesmo projeto.
	var taskB string
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'B',$3) RETURNING id`, s.ws, s.project, s.owner).Scan(&taskB); err != nil {
		t.Fatalf("seed taskB: %v", err)
	}
	subB, err := svc.Create(ctx, s.ws, s.owner, "owner", taskB, CreateInput{Title: "de B"})
	if err != nil {
		t.Fatalf("create subB: %v", err)
	}
	// PATCH/DELETE da subB pela ROTA da task A → 404 (mutação que tira task_id=$rota do WHERE morre).
	if _, err := svc.Patch(ctx, s.ws, s.owner, "owner", s.task, subB.ID, PatchInput{Done: bptr(true)}); code(err) != "not_found" {
		t.Errorf("PATCH cross-task deveria dar 404, veio %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.owner, "owner", s.task, subB.ID); code(err) != "not_found" {
		t.Errorf("DELETE cross-task deveria dar 404, veio %v", err)
	}
}

// CA-13/RNF: isolamento por workspace — sub-task de task em OUTRO ws via rota
// com o ws errado → 404 (mutação que remove WHERE workspace_id morre).
func TestIsolamentoWorkspace404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := mkSeed(t, pool)
	ctx := context.Background()
	// outro ws com o MESMO owner como membro.
	var ws2, proj2, task2 string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ('WS2') RETURNING id`).Scan(&ws2); err != nil {
		t.Fatalf("ws2: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1,$2,'owner')`, ws2, s.owner); err != nil {
		t.Fatalf("member ws2: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id) VALUES ($1,'P2','#222222','shared',$2) RETURNING id`, ws2, s.owner).Scan(&proj2); err != nil {
		t.Fatalf("proj2: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'T2',$3) RETURNING id`, ws2, proj2, s.owner).Scan(&task2); err != nil {
		t.Fatalf("task2: %v", err)
	}
	sub2, err := svc.Create(ctx, ws2, s.owner, "owner", task2, CreateInput{Title: "no ws2"})
	if err != nil {
		t.Fatalf("create sub2: %v", err)
	}
	// acessar task2/sub2 pela rota do ws errado (s.ws) → 404.
	if _, err := svc.List(ctx, s.ws, s.owner, task2); code(err) != "not_found" {
		t.Errorf("list de task de outro ws via ws errado deveria dar 404, veio %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.owner, "owner", task2, sub2.ID, PatchInput{Done: bptr(true)}); code(err) != "not_found" {
		t.Errorf("patch de sub-task de outro ws via ws errado deveria dar 404, veio %v", err)
	}
}
