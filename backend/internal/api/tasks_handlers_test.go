package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type taskJSON struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ProjectID   string  `json:"project_id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	DueDate     *string `json:"due_date"`
	Recurrence  *string `json:"recurrence"`
	Notes       string  `json:"notes"`
	CreatedBy   string  `json:"created_by"`
	Assignee    *string `json:"assignee"`
	RequestedBy *string `json:"requested_by"`
	CreatedAt   string  `json:"created_at"`
}

func criarTask(t *testing.T, router http.Handler, ws string, cookie *http.Cookie, body string) taskJSON {
	t.Helper()
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/tasks", cookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar task: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var tk taskJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &tk); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return tk
}

func listTasks(t *testing.T, router http.Handler, ws, query string, cookie *http.Cookie) []taskJSON {
	t.Helper()
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks"+query, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks%s: esperado 200, veio %d: %s", query, rec.Code, rec.Body.String())
	}
	var body struct {
		Tasks []taskJSON `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Tasks
}

// CA-16 — POST cria com status default "todo" e created_by da sessão;
// membro do projeto com access=edit também cria.
func TestCriarTaskDefaults(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idA := userIDOf(t, pool, "g-a")
	idM := userIDOf(t, pool, "g-m")
	addMember(t, pool, ws, idM, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Definir onboarding"}`)
	if tk.ID == "" || tk.WorkspaceID != ws || tk.ProjectID != p.ID ||
		tk.Name != "Definir onboarding" || tk.Status != "todo" || tk.CreatedBy != idA ||
		tk.DueDate != nil || tk.Recurrence != nil || tk.Notes != "" ||
		tk.Assignee != nil || tk.RequestedBy != nil || tk.CreatedAt == "" {
		t.Errorf("task fora do contrato §3.3: %+v", tk)
	}

	// membro do projeto com access=edit cria task (D-05)
	seedProjectMember(t, pool, p.ID, idM, "edit")
	tk2 := criarTask(t, router, ws, cookieM, `{"project_id":"`+p.ID+`","name":"Task do membro"}`)
	if tk2.CreatedBy != idM {
		t.Errorf("created_by deveria ser o membro da sessão, veio %q", tk2.CreatedBy)
	}
}

// CA-16.1 — sem permissão de escrita (access=view, ou membro do ws fora de
// project_members): POST/PATCH/DELETE → 403 e NADA muda.
func TestEscritaTaskSemPermissao403(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieView := loginVia(t, svc, stub, "g-view", "view@example.com", "Vio View")
	cookieFora := loginVia(t, svc, stub, "g-fora", "fora@example.com", "Filo Fora")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-view"), "member")
	addMember(t, pool, ws, userIDOf(t, pool, "g-fora"), "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	seedProjectMember(t, pool, p.ID, userIDOf(t, pool, "g-view"), "view")
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Intocável"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	for nome, cookie := range map[string]*http.Cookie{
		"access=view": cookieView, "fora de project_members": cookieFora,
	} {
		for _, caso := range []struct{ method, path, body string }{
			{http.MethodPost, base, `{"project_id":"` + p.ID + `","name":"Tentativa"}`},
			{http.MethodPatch, base + "/" + tk.ID, `{"name":"Hackeada"}`},
			{http.MethodDelete, base + "/" + tk.ID, ""},
		} {
			rec := doJSON(router, caso.method, caso.path, cookie, caso.body)
			if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
				t.Errorf("%s %s como %s: esperado 403 forbidden, veio %d: %s",
					caso.method, caso.path, nome, rec.Code, rec.Body.String())
			}
		}
	}

	// nada mudou: 1 task, nome intacto
	if n := dbtest.CountRows(t, pool, "tasks"); n != 1 {
		t.Errorf("contagem de tasks deveria seguir 1, há %d", n)
	}
	var nome string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM tasks WHERE id = $1`, tk.ID).Scan(&nome); err != nil || nome != "Intocável" {
		t.Errorf("task não deveria ter sido alterada (nome=%q, err=%v)", nome, err)
	}
}

// CA-16.2 — nome vazio, só espaços ou >200: 400 validation_error.
func TestTaskNomeInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	for _, nome := range []string{"", "   ", strings.Repeat("a", 201)} {
		rec := doJSON(router, http.MethodPost, base, cookie,
			`{"project_id":"`+p.ID+`","name":"`+nome+`"}`)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("POST nome %q: esperado 400 validation_error, veio %d", nome, rec.Code)
		}
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 0 {
		t.Fatalf("nada deveria ter sido persistido, há %d", n)
	}

	// fronteira: 200 caracteres passa
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"`+strings.Repeat("b", 200)+`"}`)

	// PATCH segue a mesma regra
	rec := doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookie, `{"name":"   "}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("PATCH nome inválido: esperado 400 validation_error, veio %d", rec.Code)
	}
}

// CA-17 — project_id inexistente e projeto private de outro usuário
// respondem 404 not_found IDÊNTICOS (não vazar existência).
func TestTaskProjetoInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	base := "/api/v1/workspaces/" + ws + "/tasks"

	privado := criarProjeto(t, router, ws, cookieM, `{"name":"Segredo","color":"#4f9dde","visibility":"private"}`)

	recInexistente := doJSON(router, http.MethodPost, base, cookieA,
		`{"project_id":"00000000-0000-0000-0000-000000000000","name":"X"}`)
	recPrivado := doJSON(router, http.MethodPost, base, cookieA,
		`{"project_id":"`+privado.ID+`","name":"X"}`)

	for nome, rec := range map[string]*struct {
		code int
		body string
	}{
		"inexistente":      {recInexistente.Code, recInexistente.Body.String()},
		"private de outro": {recPrivado.Code, recPrivado.Body.String()},
	} {
		if rec.code != http.StatusNotFound {
			t.Errorf("POST projeto %s: esperado 404, veio %d: %s", nome, rec.code, rec.body)
		}
	}
	if recInexistente.Body.String() != recPrivado.Body.String() {
		t.Errorf("respostas deveriam ser IDÊNTICAS:\n%s\n%s",
			recInexistente.Body.String(), recPrivado.Body.String())
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 0 {
		t.Errorf("nada deveria ter sido persistido, há %d", n)
	}
}

// CA-19 — filtros project_id e status são opcionais e combináveis.
func TestListarTasksComFiltros(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")

	p1 := criarProjeto(t, router, ws, cookie, `{"name":"P1","color":"#4f9dde","visibility":"shared"}`)
	p2 := criarProjeto(t, router, ws, cookie, `{"name":"P2","color":"#22c55e","visibility":"shared"}`)
	t1 := criarTask(t, router, ws, cookie, `{"project_id":"`+p1.ID+`","name":"t1"}`)
	t2 := criarTask(t, router, ws, cookie, `{"project_id":"`+p1.ID+`","name":"t2"}`)
	criarTask(t, router, ws, cookie, `{"project_id":"`+p2.ID+`","name":"t3"}`)

	// status muda via SQL — o PATCH de status é a T-10
	if _, err := pool.Exec(context.Background(),
		`UPDATE tasks SET status = 'doing' WHERE id = $1`, t2.ID); err != nil {
		t.Fatal(err)
	}

	if got := listTasks(t, router, ws, "", cookie); len(got) != 3 {
		t.Errorf("sem filtro: esperadas 3 tasks, vieram %d", len(got))
	}
	if got := listTasks(t, router, ws, "?project_id="+p1.ID, cookie); len(got) != 2 {
		t.Errorf("project_id=P1: esperadas 2, vieram %d", len(got))
	}
	if got := listTasks(t, router, ws, "?status=todo", cookie); len(got) != 2 {
		t.Errorf("status=todo: esperadas 2, vieram %d", len(got))
	}
	got := listTasks(t, router, ws, "?project_id="+p1.ID+"&status=todo", cookie)
	if len(got) != 1 || got[0].ID != t1.ID {
		t.Errorf("filtros combinados: esperada só t1, veio %+v", got)
	}
}

// CA-20 — due_date/recurrence/notes persistem (criação e edição);
// due_date fora do ISO 8601 → 400.
func TestTaskCamposOpcionais(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Onboarding",
		"due_date":"2026-06-15","recurrence":"FREQ=WEEKLY;BYDAY=MO","notes":"ver Figma"}`)
	if tk.DueDate == nil || *tk.DueDate != "2026-06-15" ||
		tk.Recurrence == nil || *tk.Recurrence != "FREQ=WEEKLY;BYDAY=MO" || tk.Notes != "ver Figma" {
		t.Errorf("os 3 campos deveriam persistir e voltar: %+v", tk)
	}

	// GET detalhe devolve o mesmo shape
	rec := doJSON(router, http.MethodGet, base+"/"+tk.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET detalhe: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var detalhe taskJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &detalhe); err != nil {
		t.Fatal(err)
	}
	if detalhe.DueDate == nil || *detalhe.DueDate != "2026-06-15" {
		t.Errorf("detalhe fora do contrato: %+v", detalhe)
	}

	// edição dos 3 campos (CA-20: "criada/editada"); null limpa a data
	rec = doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookie,
		`{"due_date":"2026-07-01","recurrence":"FREQ=DAILY","notes":"v2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH campos: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var editada taskJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &editada)
	if editada.DueDate == nil || *editada.DueDate != "2026-07-01" ||
		editada.Recurrence == nil || *editada.Recurrence != "FREQ=DAILY" || editada.Notes != "v2" {
		t.Errorf("PATCH deveria editar os 3 campos: %+v", editada)
	}
	rec = doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookie, `{"due_date":null}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &editada)
	if rec.Code != http.StatusOK || editada.DueDate != nil {
		t.Errorf("due_date:null deveria limpar a data (%d): %+v", rec.Code, editada)
	}

	// due_date fora do ISO → 400 validation_error
	for _, data := range []string{"15/06/2026", "2026-13-45", "ontem"} {
		rec := doJSON(router, http.MethodPost, base, cookie,
			`{"project_id":"`+p.ID+`","name":"X","due_date":"`+data+`"}`)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("due_date %q: esperado 400 validation_error, veio %d", data, rec.Code)
		}
	}
}

// CA-20.1 — recurrence com mais de 255 caracteres: 400 validation_error.
func TestTaskRecurrenceLonga400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	rec := doJSON(router, http.MethodPost, base, cookie,
		`{"project_id":"`+p.ID+`","name":"X","recurrence":"`+strings.Repeat("r", 256)+`"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Fatalf("recurrence 256: esperado 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 0 {
		t.Errorf("nada deveria ter sido persistido, há %d", n)
	}

	// fronteira: 255 passa
	criarTask(t, router, ws, cookie,
		`{"project_id":"`+p.ID+`","name":"X","recurrence":"`+strings.Repeat("r", 255)+`"}`)
}

// CA-15.2 — projeto arquivado é só leitura pra tasks: POST/PATCH → 400
// invalid_reference; GET lista e detalhe seguem 200.
func TestTaskEmProjetoArquivado400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	base := "/api/v1/workspaces/" + ws + "/tasks"

	p := criarProjeto(t, router, ws, cookie, `{"name":"Velho","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Sobrevivente"}`)
	if rec := doJSON(router, http.MethodDelete, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookie, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("arquivar projeto: %d", rec.Code)
	}

	rec := doJSON(router, http.MethodPost, base, cookie, `{"project_id":"`+p.ID+`","name":"Nova"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("POST em arquivado: esperado 400 invalid_reference, veio %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookie, `{"name":"Editada"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("PATCH em arquivado: esperado 400 invalid_reference, veio %d: %s", rec.Code, rec.Body.String())
	}

	// leitura segue aberta (CA-15.1)
	if got := listTasks(t, router, ws, "?project_id="+p.ID, cookie); len(got) != 1 {
		t.Errorf("task de projeto arquivado deveria seguir listável, vieram %d", len(got))
	}
	if rec := doJSON(router, http.MethodGet, base+"/"+tk.ID, cookie, ""); rec.Code != http.StatusOK {
		t.Errorf("detalhe de task em projeto arquivado: esperado 200, veio %d", rec.Code)
	}
}
