package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/notes"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/subtasks"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/workspace"
)

type noteJSON struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ProjectID   *string `json:"project_id"`
	TaskID      *string `json:"task_id"`
	Title       string  `json:"title"`
	BodyMD      string  `json:"body_md"`
	Preview     string  `json:"preview"`
	Links       []struct {
		ID       string `json:"id"`
		RefType  string `json:"ref_type"`
		RefID    string `json:"ref_id"`
		Label    string `json:"label"`
		Status   string `json:"status"`
		Position int    `json:"position"`
	} `json:"links"`
}

// newNotesRouter monta o router REAL com auth (01) + core (02/24) + notes (26),
// exercitando o wiring do subrouter IRMÃO da fase 26.
func newNotesRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	tasksSvc := tasks.NewService(pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasksSvc,
			Objectives: objectives.NewService(pool),
			Subtasks:   subtasks.NewService(pool),
		}),
		api.WithNotes(api.Notes{Svc: notes.NewService(pool, tasksSvc)}),
	)
	return router, svc, stub
}

func notesPath(ws string) string { return "/api/v1/workspaces/" + ws + "/notes" }

func criarNota(t *testing.T, router http.Handler, ws string, cookie *http.Cookie, body string) noteJSON {
	t.Helper()
	rec := doJSON(router, http.MethodPost, notesPath(ws), cookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar nota: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var n noteJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return n
}

func listNotes(t *testing.T, router http.Handler, ws, projectID string, cookie *http.Cookie) []noteJSON {
	t.Helper()
	path := notesPath(ws)
	if projectID != "" {
		path += "?project_id=" + projectID
	}
	rec := doJSON(router, http.MethodGet, path, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET notas: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Notes []noteJSON `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Notes
}

// Smoke do wiring (plan §3 D-02): o router NÃO panica e a rota das notas roteia
// (GET 200 com lista vazia) — prova que o subrouter IRMÃO de /notes coexiste com
// /tasks/{id}/comments e com o /workspaces/{id}/tasks da fase 02 (chi Mount sob
// wildcard).
func TestRoteamentoNotas(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	if list := listNotes(t, router, ws, p.ID, cookie); list == nil || len(list) != 0 {
		t.Fatalf("notas novas deveriam vir [] vazias, veio %+v", list)
	}
	// a task da fase 02 segue roteando (não engolida).
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Task"}`)
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks/"+tk.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks/{id} deveria seguir roteando, veio %d", rec.Code)
	}
}

// CA-01/01.2/03/04 — POST de nota de projeto e solta; GET lista; GET por id.
func TestNotasCriarListarGet(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	n := criarNota(t, router, ws, cookie, `{"project_id":"`+p.ID+`","title":"Decisões"}`)
	if n.ProjectID == nil || *n.ProjectID != p.ID || n.Title != "Decisões" || n.WorkspaceID != ws {
		t.Errorf("nota fora do contrato: %+v", n)
	}

	// nota solta (sem project_id) → 201, project_id null.
	loose := criarNota(t, router, ws, cookie, `{"title":"Solta"}`)
	if loose.ProjectID != nil {
		t.Errorf("nota solta deveria ter project_id null: %+v", loose)
	}

	// list por projeto: 1 nota (a solta não aparece).
	list := listNotes(t, router, ws, p.ID, cookie)
	if len(list) != 1 || list[0].Title != "Decisões" {
		t.Fatalf("list projeto deveria ter 1 nota, veio %+v", list)
	}
	// list soltas (sem project_id): 1 nota.
	mine := listNotes(t, router, ws, "", cookie)
	if len(mine) != 1 || mine[0].Title != "Solta" {
		t.Fatalf("list soltas deveria ter 1, veio %+v", mine)
	}

	// GET por id com body.
	rec := doJSON(router, http.MethodGet, notesPath(ws)+"/"+n.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET por id: esperado 200, veio %d", rec.Code)
	}
}

// CA-05/06 — PATCH atualiza; DELETE 204; 2º DELETE 404.
func TestNotasPatchDelete(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	n := criarNota(t, router, ws, cookie, `{"project_id":"`+p.ID+`","title":"v1"}`)

	rec := doJSON(router, http.MethodPatch, notesPath(ws)+"/"+n.ID, cookie, `{"title":"v2","body_md":"corpo"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var up noteJSON
	json.Unmarshal(rec.Body.Bytes(), &up)
	if up.Title != "v2" || up.BodyMD != "corpo" {
		t.Errorf("PATCH não aplicou: %+v", up)
	}

	rec = doJSON(router, http.MethodDelete, notesPath(ws)+"/"+n.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: esperado 204, veio %d", rec.Code)
	}
	rec = doJSON(router, http.MethodDelete, notesPath(ws)+"/"+n.ID, cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("2º DELETE: esperado 404, veio %d", rec.Code)
	}
}

// CA-01.1/02 — título > 200 → 400 validation_error; task_id inválido → 400
// invalid_reference.
func TestNotasErros400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	big := `{"project_id":"` + p.ID + `","title":"` + repeat("x", 201) + `"}`
	rec := doJSON(router, http.MethodPost, notesPath(ws), cookie, big)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("título grande: esperado 400 validation_error, veio %d/%s", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodPost, notesPath(ws), cookie,
		`{"task_id":"00000000-0000-0000-0000-000000000000","title":"x"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("task_id inválido: esperado 400 invalid_reference, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-11 — viewer em escrita de nota de projeto → 403; LEITURA → 200 (CA-03/04).
func TestNotasViewer403LeituraOK(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vic Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	n := criarNota(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","title":"do owner"}`)

	// viewer POST de nota de projeto → 403.
	rec := doJSON(router, http.MethodPost, notesPath(ws), cookieV, `{"project_id":"`+p.ID+`","title":"viewer"}`)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Errorf("viewer POST → esperado 403 forbidden, veio %d/%s", rec.Code, rec.Body.String())
	}
	// viewer LÊ (200): list e GET por id.
	list := listNotes(t, router, ws, p.ID, cookieV)
	if len(list) != 1 {
		t.Errorf("viewer deveria LER a lista (CA-03), veio %d", len(list))
	}
	rec = doJSON(router, http.MethodGet, notesPath(ws)+"/"+n.ID, cookieV, "")
	if rec.Code != http.StatusOK {
		t.Errorf("viewer GET por id → esperado 200 (CA-04), veio %d", rec.Code)
	}
}

// CA-02.1/26-H3 — viewer que ENXERGA a task posta POST /notes com {task_id} (sem
// project_id) → 403 (o gate roda contra o project_id herdado).
func TestNotasViewerComTaskID403(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vic Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Visível"}`)

	rec := doJSON(router, http.MethodPost, notesPath(ws), cookieV, `{"task_id":"`+tk.ID+`","title":"burlando"}`)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Errorf("viewer com {task_id} → esperado 403, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-13/11.1 — projeto invisível na LISTAGEM → 404; nota solta de outro user → 404.
func TestNotas404Invisivel(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM := userIDOf(t, pool, "g-m")
	addMember(t, pool, ws, idM, "member")

	// projeto PRIVADO da Ana; Mia não enxerga.
	priv := criarProjeto(t, router, ws, cookieA, `{"name":"Secreto","color":"#4f9dde","visibility":"private"}`)
	rec := doJSON(router, http.MethodGet, notesPath(ws)+"?project_id="+priv.ID, cookieM, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("list por projeto invisível → esperado 404, veio %d", rec.Code)
	}

	// nota solta da Ana; Mia tenta GET → 404 (CA-11.1).
	loose := criarNota(t, router, ws, cookieA, `{"title":"solta da Ana"}`)
	rec = doJSON(router, http.MethodGet, notesPath(ws)+"/"+loose.ID, cookieM, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET de nota solta de outro → esperado 404, veio %d", rec.Code)
	}
}

// CA-07/10/09 — POST link /task 201 com status 'todo'; /task em nota solta → 400;
// DELETE link por {linkId} → 204.
func TestNotasLinks(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	n := criarNota(t, router, ws, cookie, `{"project_id":"`+p.ID+`","title":"Mãe"}`)

	rec := doJSON(router, http.MethodPost, notesPath(ws)+"/"+n.ID+"/links", cookie, `{"kind":"task","name":"Publicar"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST link /task: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Link struct {
			ID string `json:"id"`
		} `json:"link"`
		Task struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"task"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Task.Status != "todo" {
		t.Errorf("task do slash deveria ter status 'todo', veio %q", res.Task.Status)
	}
	// a task aparece na aba Tasks (lista de tasks do projeto da fase 02).
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks?project_id="+p.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tasks: veio %d", rec.Code)
	}

	// DELETE link por {linkId} → 204.
	rec = doJSON(router, http.MethodDelete, notesPath(ws)+"/"+n.ID+"/links/"+res.Link.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE link: esperado 204, veio %d", rec.Code)
	}

	// /task em nota solta → 400.
	loose := criarNota(t, router, ws, cookie, `{"title":"Solta"}`)
	rec = doJSON(router, http.MethodPost, notesPath(ws)+"/"+loose.ID+"/links", cookie, `{"kind":"task","name":"x"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("/task em nota solta → esperado 400 validation_error, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-15 — 401 sem sessão em TODAS as rotas novas (table-driven).
func TestNotas401SemSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newNotesRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	n := criarNota(t, router, ws, cookie, `{"project_id":"`+p.ID+`","title":"x"}`)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, notesPath(ws), ""},
		{http.MethodPost, notesPath(ws), `{"project_id":"` + p.ID + `","title":"x"}`},
		{http.MethodGet, notesPath(ws) + "/" + n.ID, ""},
		{http.MethodPatch, notesPath(ws) + "/" + n.ID, `{"title":"x"}`},
		{http.MethodDelete, notesPath(ws) + "/" + n.ID, ""},
		{http.MethodPost, notesPath(ws) + "/" + n.ID + "/links", `{"kind":"note","title":"x"}`},
		{http.MethodDelete, notesPath(ws) + "/" + n.ID + "/links/00000000-0000-0000-0000-000000000000", ""},
	} {
		rec := doJSON(router, tc.method, tc.path, nil, tc.body) // sem cookie
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("%s %s sem sessão: esperado 401 unauthorized, veio %d", tc.method, tc.path, rec.Code)
		}
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
