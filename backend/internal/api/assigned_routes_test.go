package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type assignedRespTask struct {
	taskJSON
	Project struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"project"`
	Workspace struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Client *string `json:"client"`
}

func getAssigned(t *testing.T, router http.Handler, path string, cookie *http.Cookie) []assignedRespTask {
	t.Helper()
	rec := doJSON(router, http.MethodGet, path, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: esperado 200, veio %d: %s", path, rec.Code, rec.Body.String())
	}
	var body struct {
		Tasks []assignedRespTask `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return body.Tasks
}

// Fase 18 T-07 — CA-18/19: GET /tasks/assigned + /tasks/requested por usuário.
// 200 + shape com sessão; 401 sem sessão; isolamento (A só vê o dele).
func TestAssignedRoutes(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieB := loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idA := userIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	// A cria task atribuída a B, pedida por A
	tk := criarTask(t, router, ws, cookieA,
		`{"project_id":"`+p.ID+`","name":"Pra B","assignee":"`+idB+`","requested_by":"`+idA+`"}`)

	// /tasks/assigned de B vê a task
	gotB := getAssigned(t, router, "/api/v1/tasks/assigned", cookieB)
	if len(gotB) != 1 || gotB[0].ID != tk.ID {
		t.Fatalf("B deveria ver 1 task atribuída, veio %+v", gotB)
	}
	if gotB[0].Project.Name != "App" || gotB[0].Workspace.ID != ws {
		t.Errorf("contexto cross-ws ausente: %+v", gotB[0])
	}
	// requested_by de A: aparece a task pedida por A
	gotReqA := getAssigned(t, router, "/api/v1/tasks/requested", cookieA)
	if len(gotReqA) != 1 || gotReqA[0].ID != tk.ID {
		t.Errorf("A deveria ver 1 task pedida por si, veio %+v", gotReqA)
	}

	// isolamento: A não tem nada ATRIBUÍDO (a task é de B)
	gotA := getAssigned(t, router, "/api/v1/tasks/assigned", cookieA)
	if len(gotA) != 0 {
		t.Errorf("A não tem tasks atribuídas a si, veio %d: %+v", len(gotA), gotA)
	}

	// 401 sem sessão
	for _, path := range []string{"/api/v1/tasks/assigned", "/api/v1/tasks/requested"} {
		rec := doJSON(router, http.MethodGet, path, nil, "")
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("GET %s sem sessão: esperado 401 unauthorized, veio %d", path, rec.Code)
		}
	}
}

// garante que o WHERE assignee=$me não vaza nada (sanity de isolamento via API).
func TestAssignedRoutesVazio(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-z", "z@example.com", "Zé Solo")
	got := getAssigned(t, router, "/api/v1/tasks/assigned", cookie)
	if got == nil || len(got) != 0 {
		t.Errorf("sem atribuições deveria devolver [] vazio, veio %+v", got)
	}
	_ = context.Background()
}
