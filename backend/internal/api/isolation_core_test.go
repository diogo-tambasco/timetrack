package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// uuid válido que não existe em lugar nenhum — referência "inexistente"
// pra comparar com a resposta dos recursos de outro workspace.
const uuidInexistente = "00000000-0000-0000-0000-0000000000aa"

func criarCliente(t *testing.T, router http.Handler, ws string, cookie *http.Cookie, body string) clientJSON {
	t.Helper()
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/clients", cookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar cliente: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var c clientJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return c
}

func listClients(t *testing.T, router http.Handler, ws, query string, cookie *http.Cookie) []clientJSON {
	t.Helper()
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/clients"+query, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /clients%s: esperado 200, veio %d: %s", query, rec.Code, rec.Body.String())
	}
	var body struct {
		Clients []clientJSON `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Clients
}

// CA-26 — teste de vazamento multi-tenant (crítico). Dois workspaces
// seedados; o usuário do A referenciando ids REAIS do B (na URL e no
// payload) recebe a MESMA resposta de um recurso inexistente, em clientes,
// projetos, membros de projeto e tasks; rota de workspace alheio é 404.
//
// Red→green desta tarefa (exceção documentada no tasks.md): a suíte nasce
// verde porque os filtros existem desde o bloco 1 — o red foi validado por
// MUTAÇÃO MANUAL (remover o filtro workspace_id de clients.Rename, do
// visibleWhere de projects.getForWrite e do tasks.List faz casos da tabela
// falharem; mutações revertidas antes do commit).
//
// Nota de contrato: CA-26 manda payload cruzado responder como inexistente.
// O código exato vem dos CAs específicos — client_id é 400 invalid_reference
// (CA-08) e project_id é 404 not_found (CA-17); o que NÃO pode acontecer é
// a resposta divergir da de um id inexistente (vazaria existência). A
// tabela compara os dois lado a lado.
func TestIsolamentoEntreWorkspaces(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)

	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Dona do A")
	cookieB := loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Dono do B")
	wsA := workspaceIDOf(t, pool, "g-a")
	wsB := workspaceIDOf(t, pool, "g-b")
	userB := userIDOf(t, pool, "g-b")

	// seed do workspace B (tudo criado pelo B)
	clientB := criarCliente(t, router, wsB, cookieB, `{"name":"Cliente do B"}`)
	projectB := criarProjeto(t, router, wsB, cookieB,
		`{"name":"Projeto do B","color":"#4f9dde","visibility":"shared"}`)
	taskB := criarTask(t, router, wsB, cookieB,
		`{"project_id":"`+projectB.ID+`","name":"Task do B"}`)

	// seed do workspace A (alvos legítimos pro payload cruzado)
	clientA := criarCliente(t, router, wsA, cookieA, `{"name":"Cliente do A"}`)
	projectA := criarProjeto(t, router, wsA, cookieA,
		`{"name":"Projeto do A","color":"#22c55e","visibility":"shared"}`)
	taskA := criarTask(t, router, wsA, cookieA,
		`{"project_id":"`+projectA.ID+`","name":"Task do A"}`)

	corpoProjeto := `{"name":"Invasor","color":"#4f9dde","visibility":"shared"}`

	casos := []struct {
		nome       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		// — ids reais do B na URL, dentro do workspace A → 404 —
		{"patch cliente do B via wsA", http.MethodPatch, "/api/v1/workspaces/" + wsA + "/clients/" + clientB.ID, `{"name":"Hijack"}`, http.StatusNotFound, "not_found"},
		{"delete cliente do B via wsA", http.MethodDelete, "/api/v1/workspaces/" + wsA + "/clients/" + clientB.ID, "", http.StatusNotFound, "not_found"},
		{"get projeto do B via wsA", http.MethodGet, "/api/v1/workspaces/" + wsA + "/projects/" + projectB.ID, "", http.StatusNotFound, "not_found"},
		{"patch projeto do B via wsA", http.MethodPatch, "/api/v1/workspaces/" + wsA + "/projects/" + projectB.ID, `{"name":"Hijack"}`, http.StatusNotFound, "not_found"},
		{"delete projeto do B via wsA", http.MethodDelete, "/api/v1/workspaces/" + wsA + "/projects/" + projectB.ID, "", http.StatusNotFound, "not_found"},
		{"put members do projeto do B via wsA", http.MethodPut, "/api/v1/workspaces/" + wsA + "/projects/" + projectB.ID + "/members", `{"members":[]}`, http.StatusNotFound, "not_found"},
		{"get task do B via wsA", http.MethodGet, "/api/v1/workspaces/" + wsA + "/tasks/" + taskB.ID, "", http.StatusNotFound, "not_found"},
		{"patch task do B via wsA", http.MethodPatch, "/api/v1/workspaces/" + wsA + "/tasks/" + taskB.ID, `{"name":"Hijack"}`, http.StatusNotFound, "not_found"},
		{"delete task do B via wsA", http.MethodDelete, "/api/v1/workspaces/" + wsA + "/tasks/" + taskB.ID, "", http.StatusNotFound, "not_found"},

		// — rota de workspace que A não integra → 404 uniforme (RequireMember) —
		{"listar clientes do wsB", http.MethodGet, "/api/v1/workspaces/" + wsB + "/clients", "", http.StatusNotFound, "not_found"},
		{"criar cliente no wsB", http.MethodPost, "/api/v1/workspaces/" + wsB + "/clients", `{"name":"Invasor"}`, http.StatusNotFound, "not_found"},
		{"listar projetos do wsB", http.MethodGet, "/api/v1/workspaces/" + wsB + "/projects", "", http.StatusNotFound, "not_found"},
		{"criar projeto no wsB", http.MethodPost, "/api/v1/workspaces/" + wsB + "/projects", corpoProjeto, http.StatusNotFound, "not_found"},
		{"detalhe de projeto do wsB", http.MethodGet, "/api/v1/workspaces/" + wsB + "/projects/" + projectB.ID, "", http.StatusNotFound, "not_found"},
		{"listar tasks do wsB", http.MethodGet, "/api/v1/workspaces/" + wsB + "/tasks", "", http.StatusNotFound, "not_found"},
		{"criar task no wsB", http.MethodPost, "/api/v1/workspaces/" + wsB + "/tasks", `{"project_id":"` + projectB.ID + `","name":"Invasor"}`, http.StatusNotFound, "not_found"},

		// — ids reais do B no PAYLOAD, dentro do workspace A → mesma resposta
		//   de inexistente (códigos dos CAs específicos: 08/12/17) —
		{"client_id do B no POST de projeto", http.MethodPost, "/api/v1/workspaces/" + wsA + "/projects", `{"name":"X","color":"#4f9dde","visibility":"shared","client_id":"` + clientB.ID + `"}`, http.StatusBadRequest, "invalid_reference"},
		{"client_id inexistente no POST de projeto", http.MethodPost, "/api/v1/workspaces/" + wsA + "/projects", `{"name":"X","color":"#4f9dde","visibility":"shared","client_id":"` + uuidInexistente + `"}`, http.StatusBadRequest, "invalid_reference"},
		{"client_id do B no PATCH de projeto", http.MethodPatch, "/api/v1/workspaces/" + wsA + "/projects/" + projectA.ID, `{"client_id":"` + clientB.ID + `"}`, http.StatusBadRequest, "invalid_reference"},
		{"project_id do B no POST de task", http.MethodPost, "/api/v1/workspaces/" + wsA + "/tasks", `{"project_id":"` + projectB.ID + `","name":"X"}`, http.StatusNotFound, "not_found"},
		{"project_id inexistente no POST de task", http.MethodPost, "/api/v1/workspaces/" + wsA + "/tasks", `{"project_id":"` + uuidInexistente + `","name":"X"}`, http.StatusNotFound, "not_found"},
		{"user do B na lista de membros", http.MethodPut, "/api/v1/workspaces/" + wsA + "/projects/" + projectA.ID + "/members", `{"members":[{"user_id":"` + userB + `","access":"edit"}]}`, http.StatusBadRequest, "invalid_reference"},
		{"user inexistente na lista de membros", http.MethodPut, "/api/v1/workspaces/" + wsA + "/projects/" + projectA.ID + "/members", `{"members":[{"user_id":"` + uuidInexistente + `","access":"edit"}]}`, http.StatusBadRequest, "invalid_reference"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := doJSON(router, c.method, c.path, cookieA, c.body)
			if rec.Code != c.wantStatus {
				t.Fatalf("%s %s: esperado %d, veio %d: %s",
					c.method, c.path, c.wantStatus, rec.Code, rec.Body.String())
			}
			if got := errCode(t, rec); got != c.wantCode {
				t.Errorf("%s %s: esperado error.code=%q, veio %q",
					c.method, c.path, c.wantCode, got)
			}
		})
	}

	// — listagens do A não vazam NADA do B —
	t.Run("listagens do A sem dados do B", func(t *testing.T) {
		if cs := listClients(t, router, wsA, "", cookieA); len(cs) != 1 || cs[0].ID != clientA.ID {
			t.Errorf("GET /clients do A deveria ter só o cliente do A, veio %+v", cs)
		}
		if ps := listProjects(t, router, wsA, cookieA); len(ps) != 1 || ps[0].ID != projectA.ID {
			t.Errorf("GET /projects do A deveria ter só o projeto do A, veio %+v", ps)
		}
		if ts := listTasks(t, router, wsA, "", cookieA); len(ts) != 1 || ts[0].ID != taskA.ID {
			t.Errorf("GET /tasks do A deveria ter só a task do A, veio %+v", ts)
		}
		// filtro project_id apontando pro projeto do B → lista vazia (o
		// filtro tenant prevalece; nada do B atravessa)
		if ts := listTasks(t, router, wsA, "?project_id="+projectB.ID, cookieA); len(ts) != 0 {
			t.Errorf("filtro com project_id do B deveria voltar vazio, veio %+v", ts)
		}
	})

	// — nada das tentativas persistiu: o workspace B está intacto —
	t.Run("workspace B intacto", func(t *testing.T) {
		var nome string
		if err := pool.QueryRow(context.Background(),
			`SELECT name FROM clients WHERE id = $1`, clientB.ID).Scan(&nome); err != nil || nome != "Cliente do B" {
			t.Errorf("cliente do B deveria seguir intacto, veio %q (err=%v)", nome, err)
		}
		var arquivado *string
		if err := pool.QueryRow(context.Background(),
			`SELECT name, archived_at::text FROM projects WHERE id = $1`, projectB.ID).Scan(&nome, &arquivado); err != nil ||
			nome != "Projeto do B" || arquivado != nil {
			t.Errorf("projeto do B deveria seguir intacto, veio %q archived_at=%v (err=%v)", nome, arquivado, err)
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM tasks WHERE workspace_id = $1`, wsB).Scan(&n); err != nil || n != 1 {
			t.Errorf("wsB deveria seguir com exatamente 1 task, veio %d (err=%v)", n, err)
		}
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM project_members WHERE project_id = $1`, projectB.ID).Scan(&n); err != nil || n != 0 {
			t.Errorf("projeto do B não deveria ter ganho membros, veio %d (err=%v)", n, err)
		}
	})
}
