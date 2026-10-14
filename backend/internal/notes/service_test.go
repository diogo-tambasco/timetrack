package notes

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

// seedTask insere uma task num projeto (status default 'todo').
func seedTask(t *testing.T, pool *pgxpool.Pool, wsID, projID, createdBy, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		wsID, projID, name, createdBy).Scan(&id); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return id
}

// CA-01/01.2/02 — Create de nota de projeto / solta / atrelada a task.
func TestCreateNota(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	task := seedTask(t, pool, wsID, proj, owner, "Tarefa")

	// nota de projeto.
	n, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "  Decisões  "})
	if err != nil {
		t.Fatalf("create nota de projeto: %v", err)
	}
	if n.ProjectID == nil || *n.ProjectID != proj || n.Title != "Decisões" || n.BodyMD != "" ||
		n.CreatedBy != owner || n.WorkspaceID != wsID || n.TaskID != nil {
		t.Errorf("nota de projeto fora do contrato: %+v", n)
	}

	// nota solta (sem project_id nem task_id).
	loose, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{Title: "Solta"})
	if err != nil {
		t.Fatalf("create nota solta: %v", err)
	}
	if loose.ProjectID != nil || loose.TaskID != nil {
		t.Errorf("nota solta deveria ter project_id e task_id NULL: %+v", loose)
	}

	// nota atrelada a task → herda o project_id da task e grava task_id.
	att, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{TaskID: &task, Title: "Atrelada"})
	if err != nil {
		t.Fatalf("create nota atrelada: %v", err)
	}
	if att.ProjectID == nil || *att.ProjectID != proj || att.TaskID == nil || *att.TaskID != task {
		t.Errorf("nota atrelada deveria herdar project_id e gravar task_id: %+v", att)
	}
}

// CA-01.1/05.1 — título > 200 → 400 e não persiste.
func TestTituloGrande400(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)

	big := strings.Repeat("x", 201)
	if _, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: big}); !isDomain(err, "validation_error") {
		t.Errorf("título 201 chars → esperado 400 validation_error, veio %v", err)
	}
	list, _ := svc.List(ctx, wsID, owner, proj)
	if len(list) != 0 {
		t.Errorf("título inválido NÃO deveria persistir, veio %d notas", len(list))
	}

	// 200 chars é permitido.
	ok := strings.Repeat("y", 200)
	if _, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: ok}); err != nil {
		t.Errorf("título 200 chars deveria passar, veio %v", err)
	}
}

// CA-02 — task_id de outro ws / invisível / malformado → 400 invalid_reference.
func TestCreateTaskRefInvalida(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")

	// task de OUTRO ws.
	other := seedUser(t, pool, "g-other")
	ws2 := seedWorkspace(t, pool, "WS2")
	seedMember(t, pool, ws2, other, "owner")
	proj2 := seedProject(t, pool, ws2, other, "shared", false)
	taskOther := seedTask(t, pool, ws2, proj2, other, "Outra")

	if _, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{TaskID: &taskOther, Title: "x"}); !isDomain(err, "invalid_reference") {
		t.Errorf("task de outro ws → esperado 400 invalid_reference, veio %v", err)
	}
	// task private de outro (invisível) no MESMO ws.
	privP := seedProject(t, pool, wsID, owner, "private", false)
	taskPriv := seedTask(t, pool, wsID, privP, owner, "Priv")
	member := seedUser(t, pool, "g-member")
	seedMember(t, pool, wsID, member, "member")
	if _, err := svc.Create(ctx, wsID, member, "member", CreateInput{TaskID: &taskPriv, Title: "x"}); !isDomain(err, "invalid_reference") {
		t.Errorf("task invisível → esperado 400 invalid_reference, veio %v", err)
	}
	// task_id malformado.
	bad := "nao-e-uuid"
	if _, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{TaskID: &bad, Title: "x"}); !isDomain(err, "invalid_reference") {
		t.Errorf("task_id malformado → esperado 400 invalid_reference, veio %v", err)
	}
}

// CA-02 (ramo project_id de payload) — project_id de OUTRO ws (mesmo SHARED) no
// payload → 400 invalid_reference, nada persiste em WS1. Mutação a matar
// (gate.go loadProjectWriteGate): neutralizar o WHERE p.workspace_id = $1 →
// o projeto shared de WS2 vazaria e o Create teria sucesso.
func TestCreateProjectRefCrossWs(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	ws1 := seedWorkspace(t, pool, "WS1")
	seedMember(t, pool, ws1, owner, "owner")

	// projeto SHARED em OUTRO ws (WS2), dono = other.
	other := seedUser(t, pool, "g-other")
	ws2 := seedWorkspace(t, pool, "WS2")
	seedMember(t, pool, ws2, other, "owner")
	proj2 := seedProject(t, pool, ws2, other, "shared", false)

	// owner do WS1 tenta criar nota com project_id do WS2 → 400 invalid_reference.
	if _, err := svc.Create(ctx, ws1, owner, "owner", CreateInput{ProjectID: &proj2, Title: "x"}); !isDomain(err, "invalid_reference") {
		t.Errorf("project_id de outro ws → esperado 400 invalid_reference, veio %v", err)
	}
	// nada persistiu em WS1.
	if got := countNotes(t, pool, ws1); got != 0 {
		t.Errorf("Create cross-ws NÃO deveria persistir nota em WS1, veio %d", got)
	}
}

// CA-02.1/26-H3 — viewer/access=view que ENXERGA a task posta {task_id} → 403,
// nada persiste. Mutação que pule o gate no caminho task_id-inferido (ou decida
// por "tem project_id no payload?") DEVE quebrar este teste.
func TestViewerComTaskID403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	viewer := seedUser(t, pool, "g-viewer")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, viewer, "viewer")

	shared := seedProject(t, pool, wsID, owner, "shared", false) // viewer ENXERGA (shared)
	task := seedTask(t, pool, wsID, shared, owner, "Visível")

	// viewer vê a task (shared) mas não tem escrita → 403.
	if _, err := svc.Create(ctx, wsID, viewer, "viewer", CreateInput{TaskID: &task, Title: "burlando"}); !isDomain(err, "forbidden") {
		t.Errorf("viewer com {task_id} de task visível → esperado 403 forbidden, veio %v", err)
	}
	// nada persistiu.
	list, _ := svc.List(ctx, wsID, owner, shared)
	if len(list) != 0 {
		t.Errorf("viewer com 403 NÃO deveria persistir nota, veio %d", len(list))
	}
	// E um member com access=view também: 403.
	memberV := seedUser(t, pool, "g-mv")
	seedMember(t, pool, wsID, memberV, "member")
	seedProjectMember(t, pool, shared, memberV, "view")
	if _, err := svc.Create(ctx, wsID, memberV, "member", CreateInput{TaskID: &task, Title: "burlando2"}); !isDomain(err, "forbidden") {
		t.Errorf("member access=view com {task_id} → esperado 403, veio %v", err)
	}
}

// CA-03/03.1 — list por projeto ordenado updated_at desc + preview; list soltas
// só do user. Mutação removendo created_by=$u do WHERE de soltas DEVE quebrar.
func TestListProjetoESoltas(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	member := seedUser(t, pool, "g-member")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, member, "member")
	proj := seedProject(t, pool, wsID, owner, "shared", false)

	a, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "A"})
	_, _ = svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "B"})
	// patcha A → updated_at maior → A primeiro.
	body := "## Título\nCorpo com **negrito** e [link](http://x). Lista:\n- item"
	if _, err := svc.Patch(ctx, wsID, owner, "owner", a.ID, PatchInput{BodySet: true, BodyMD: &body}); err != nil {
		t.Fatalf("patch A: %v", err)
	}

	list, err := svc.List(ctx, wsID, owner, proj)
	if err != nil {
		t.Fatalf("list projeto: %v", err)
	}
	if len(list) != 2 || list[0].Title != "A" {
		t.Fatalf("ordem updated_at desc deveria pôr A primeiro, veio %+v", titles(list))
	}
	// preview com markdown removido, sem '#', '**', sem 'http' do link.
	pv := list[0].Preview
	if strings.Contains(pv, "#") || strings.Contains(pv, "**") || strings.Contains(pv, "http") || strings.Contains(pv, "[") {
		t.Errorf("preview deveria remover markdown, veio %q", pv)
	}
	if !strings.Contains(pv, "Título") || !strings.Contains(pv, "negrito") || !strings.Contains(pv, "link") {
		t.Errorf("preview deveria conter o texto, veio %q", pv)
	}

	// notas soltas: owner cria uma, member cria outra. owner só vê a sua.
	_, _ = svc.Create(ctx, wsID, owner, "owner", CreateInput{Title: "minha solta"})
	_, _ = svc.Create(ctx, wsID, member, "member", CreateInput{Title: "solta do member"})
	mine, _ := svc.List(ctx, wsID, owner, "")
	if len(mine) != 1 || mine[0].Title != "minha solta" {
		t.Errorf("list soltas deveria devolver SÓ a do user, veio %+v", titles(mine))
	}
}

// CA-13 (ramo List cross-ws) — List ?project_id= apontando pra projeto SHARED de
// OUTRO ws → 404 not_found (ref de ROTA). Mutação a matar (service.go
// requireVisibleProject): neutralizar o WHERE p.workspace_id = $1 → o projeto
// shared de WS2 vazaria e List não daria 404.
func TestListProjectCrossWs(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	ws1 := seedWorkspace(t, pool, "WS1")
	seedMember(t, pool, ws1, owner, "owner")

	// projeto SHARED em OUTRO ws.
	other := seedUser(t, pool, "g-other")
	ws2 := seedWorkspace(t, pool, "WS2")
	seedMember(t, pool, ws2, other, "owner")
	proj2 := seedProject(t, pool, ws2, other, "shared", false)

	// owner do WS1 lista por um projeto de WS2 → 404.
	if _, err := svc.List(ctx, ws1, owner, proj2); !isDomain(err, "not_found") {
		t.Errorf("List ?project_id de outro ws → esperado 404 not_found, veio %v", err)
	}
}

// CA-04 — get de nota invisível/malformada → 404.
func TestGetNota404(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	member := seedUser(t, pool, "g-member")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, member, "member")
	priv := seedProject(t, pool, wsID, owner, "private", false)
	n, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &priv, Title: "secreta"})

	if _, err := svc.Get(ctx, wsID, member, n.ID); !isDomain(err, "not_found") {
		t.Errorf("get de nota invisível → esperado 404, veio %v", err)
	}
	if _, err := svc.Get(ctx, wsID, owner, "nao-uuid"); !isDomain(err, "not_found") {
		t.Errorf("get malformado → esperado 404, veio %v", err)
	}
	// dono lê normalmente, com body completo.
	got, err := svc.Get(ctx, wsID, owner, n.ID)
	if err != nil || got.Title != "secreta" {
		t.Errorf("dono deveria ler a própria, veio %v / %+v", err, got)
	}
}

// CA-05 — patch title+body atualiza updated_at; body ” limpa.
func TestPatchNota(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	n, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "v1"})

	nt := "v2"
	nb := "corpo novo"
	up, err := svc.Patch(ctx, wsID, owner, "owner", n.ID, PatchInput{TitleSet: true, Title: &nt, BodySet: true, BodyMD: &nb})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if up.Title != "v2" || up.BodyMD != "corpo novo" || !up.UpdatedAt.After(n.UpdatedAt) {
		t.Errorf("patch deveria atualizar title/body/updated_at: %+v (era %v)", up, n.UpdatedAt)
	}
	// patch só do body (title ausente mantém).
	clear := ""
	up2, _ := svc.Patch(ctx, wsID, owner, "owner", n.ID, PatchInput{BodySet: true, BodyMD: &clear})
	if up2.Title != "v2" || up2.BodyMD != "" {
		t.Errorf("patch só body deveria manter title e limpar body: %+v", up2)
	}
}

// CA-06/06.1/26-H7 — Create/Patch em projeto arquivado → 400; DELETE → 204.
func TestArquivadoMatriz(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	// projeto ainda ATIVO: cria a nota, depois arquiva.
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	n, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "antes"})
	if _, err := pool.Exec(ctx, `UPDATE projects SET archived_at = now() WHERE id = $1`, proj); err != nil {
		t.Fatalf("arquivar: %v", err)
	}

	// Create em projeto arquivado → 400 invalid_reference.
	if _, err := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "nova"}); !isDomain(err, "invalid_reference") {
		t.Errorf("Create em arquivado → esperado 400 invalid_reference, veio %v", err)
	}
	// Patch em projeto arquivado → 400 invalid_reference.
	nt := "edit"
	if _, err := svc.Patch(ctx, wsID, owner, "owner", n.ID, PatchInput{TitleSet: true, Title: &nt}); !isDomain(err, "invalid_reference") {
		t.Errorf("Patch em arquivado → esperado 400 invalid_reference, veio %v", err)
	}
	// DELETE em projeto arquivado → 204 (sem erro).
	if err := svc.Delete(ctx, wsID, owner, "owner", n.ID); err != nil {
		t.Errorf("Delete em arquivado → esperado ok (204), veio %v", err)
	}
	// 2º delete → 404.
	if err := svc.Delete(ctx, wsID, owner, "owner", n.ID); !isDomain(err, "not_found") {
		t.Errorf("2º delete → esperado 404, veio %v", err)
	}
}

// CA-14 — isolamento por workspace: query de lista soltas filtra por ws E user.
// Mutação removendo o WHERE workspace_id quebra (nota solta de outro ws vazaria).
func TestIsolamentoWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	ws1 := seedWorkspace(t, pool, "WS1")
	ws2 := seedWorkspace(t, pool, "WS2")
	seedMember(t, pool, ws1, owner, "owner")
	seedMember(t, pool, ws2, owner, "owner")

	// o MESMO user tem nota solta nos dois ws.
	_, _ = svc.Create(ctx, ws1, owner, "owner", CreateInput{Title: "no ws1"})
	_, _ = svc.Create(ctx, ws2, owner, "owner", CreateInput{Title: "no ws2"})

	l1, _ := svc.List(ctx, ws1, owner, "")
	if len(l1) != 1 || l1[0].Title != "no ws1" {
		t.Errorf("list soltas de ws1 deveria trazer SÓ 'no ws1', veio %+v", titles(l1))
	}
}

func titles(list []Note) []string {
	out := make([]string, len(list))
	for i := range list {
		out[i] = list[i].Title
	}
	return out
}
