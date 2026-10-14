// subtasks_handlers_test.go — fase 24 T-06/T-07: REST de sub-tasks montado no
// Core (sob RequireMember+RequireWriteRole — decisão F) + os campos novos de
// task (priority/notion_url/position) nos handlers de task. Harness coretest.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
)

// subtaskSeed: workspace shared do owner + 1 task + 1 projeto arquivado/task +
// 1 projeto private/task (do owner). Reusa o login stubbado; svc/stub ficam
// disponíveis pra logar outros users no MESMO router (multi-user).
type subtaskSeed struct {
	router       http.Handler
	svc          *auth.Service
	stub         *googlestub.Server
	ownerCookie  *http.Cookie
	ws           string
	task         string
	taskArchived string
	taskPrivate  string
}

func seedSubtaskHTTP(t *testing.T, pool *pgxpool.Pool) subtaskSeed {
	t.Helper()
	router, svc, stub := newCoreRouter(t, pool)
	ctx := context.Background()
	ownerCookie := loginVia(t, svc, stub, "g-st-own", "stown@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "g-st-own")
	owner := userIDOf(t, pool, "g-st-own")

	var proj, archived, priv, task, taskA, taskP string
	mk := func(dst *string, q string, args ...any) {
		if err := pool.QueryRow(ctx, q, args...).Scan(dst); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
	}
	mk(&proj, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id) VALUES ($1,'P','#4f9dde','shared',$2) RETURNING id`, ws, owner)
	mk(&archived, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id, archived_at) VALUES ($1,'A','#000000','shared',$2, now()) RETURNING id`, ws, owner)
	mk(&priv, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id) VALUES ($1,'Pr','#111111','private',$2) RETURNING id`, ws, owner)
	mk(&task, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'T',$3) RETURNING id`, ws, proj, owner)
	mk(&taskA, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'TA',$3) RETURNING id`, ws, archived, owner)
	mk(&taskP, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'TP',$3) RETURNING id`, ws, priv, owner)
	return subtaskSeed{router: router, svc: svc, stub: stub, ownerCookie: ownerCookie, ws: ws, task: task, taskArchived: taskA, taskPrivate: taskP}
}

func subPath(ws, task string) string {
	return "/api/v1/workspaces/" + ws + "/tasks/" + task + "/subtasks"
}

// CA-06/07/08/09: POST 201 + GET ordenado + PATCH + DELETE.
func TestSubtaskCRUDHTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)

	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, s.task), s.ownerCookie, `{"title":"Revisar"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST subtask = %d, esperava 201 (%s)", rec.Code, rec.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		TaskID   string `json:"task_id"`
		Title    string `json:"title"`
		Done     bool   `json:"done"`
		IsGoal   bool   `json:"is_goal"`
		Position int    `json:"position"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	if created.ID == "" || created.Title != "Revisar" || created.Done || created.IsGoal {
		t.Errorf("POST devolveu %+v", created)
	}

	// 2ª sub-task (pra ordem).
	doJSON(s.router, http.MethodPost, subPath(s.ws, s.task), s.ownerCookie, `{"title":"Segunda"}`)

	rec = doJSON(s.router, http.MethodGet, subPath(s.ws, s.task), s.ownerCookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d (%s)", rec.Code, rec.Body.String())
	}
	var listed struct {
		Subtasks []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"subtasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	if len(listed.Subtasks) != 2 || listed.Subtasks[0].Title != "Revisar" {
		t.Errorf("GET deveria vir [Revisar, Segunda] ordenado, veio %+v", listed.Subtasks)
	}

	// PATCH done.
	rec = doJSON(s.router, http.MethodPatch, subPath(s.ws, s.task)+"/"+created.ID, s.ownerCookie, `{"done":true,"is_goal":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d (%s)", rec.Code, rec.Body.String())
	}
	// DELETE → 204.
	rec = doJSON(s.router, http.MethodDelete, subPath(s.ws, s.task)+"/"+created.ID, s.ownerCookie, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("DELETE = %d, esperava 204 (%s)", rec.Code, rec.Body.String())
	}
}

// CA-06.1: title vazio → 400 validation_error.
func TestSubtaskTituloInvalidoHTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, s.task), s.ownerCookie, `{"title":"   "}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("title vazio deveria dar 400 validation_error, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// decisão G: POST em projeto arquivado → 400 invalid_reference.
func TestSubtaskProjetoArquivado400HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, s.taskArchived), s.ownerCookie, `{"title":"x"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("projeto arquivado deveria dar 400 invalid_reference, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-11: POST por viewer do ws → 403 (barrado pelo RequireWriteRole do Core).
func TestSubtaskViewerWS403HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	// loga um 2º user no MESMO router/auth e o adiciona como VIEWER do ws do owner.
	viewerCookie := loginVia(t, s.svc, s.stub, "g-st-vie", "stvie@b.c", "Viewer")
	addMember(t, pool, s.ws, userIDOf(t, pool, "g-st-vie"), "viewer")

	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, s.task), viewerCookie, `{"title":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer do ws deveria dar 403, veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-11: POST por membro com access=view no projeto → 403 (gate fino do service).
func TestSubtaskAccessView403HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	memberCookie := loginVia(t, s.svc, s.stub, "g-st-av", "stav@b.c", "AV")
	mid := userIDOf(t, pool, "g-st-av")
	addMember(t, pool, s.ws, mid, "member")
	var projID string
	if err := pool.QueryRow(context.Background(), `SELECT project_id FROM tasks WHERE id = $1`, s.task).Scan(&projID); err != nil {
		t.Fatalf("project_id: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO project_members (project_id, user_id, access) VALUES ($1,$2,'view')`, projID, mid); err != nil {
		t.Fatalf("seed access view: %v", err)
	}
	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, s.task), memberCookie, `{"title":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("access=view deveria dar 403 (gate fino), veio %d/%s", rec.Code, rec.Body.String())
	}
}

// CA-13: task invisível (private de outro) → 404; cross-task → 404.
func TestSubtaskInvisivelE404HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	// um 2º membro (member, não vê o private do owner) → 404 ao acessar o private.
	memberCookie := loginVia(t, s.svc, s.stub, "g-st-mem", "stmem@b.c", "Mem")
	addMember(t, pool, s.ws, userIDOf(t, pool, "g-st-mem"), "member")

	rec := doJSON(s.router, http.MethodGet, subPath(s.ws, s.taskPrivate), memberCookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("task private de outro deveria dar 404, veio %d/%s", rec.Code, rec.Body.String())
	}
	// uuid malformado → 404 (não 500).
	rec = doJSON(s.router, http.MethodGet, subPath(s.ws, "nao-uuid"), s.ownerCookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("taskId malformado deveria dar 404, veio %d", rec.Code)
	}
}

// CA-13 (pertencimento, HTTP): sub-task da task B alcançada via rota da task A → 404.
func TestSubtaskCrossTask404HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	ctx := context.Background()
	// 2ª task no mesmo projeto do s.task.
	var projID, taskB string
	if err := pool.QueryRow(ctx, `SELECT project_id FROM tasks WHERE id = $1`, s.task).Scan(&projID); err != nil {
		t.Fatalf("project_id: %v", err)
	}
	owner := userIDOf(t, pool, "g-st-own")
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1,$2,'B',$3) RETURNING id`, s.ws, projID, owner).Scan(&taskB); err != nil {
		t.Fatalf("taskB: %v", err)
	}
	// cria sub-task na taskB.
	rec := doJSON(s.router, http.MethodPost, subPath(s.ws, taskB), s.ownerCookie, `{"title":"de B"}`)
	var sub struct {
		ID string `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &sub)
	// PATCH/DELETE da subB pela ROTA da task A (s.task) → 404.
	rec = doJSON(s.router, http.MethodPatch, subPath(s.ws, s.task)+"/"+sub.ID, s.ownerCookie, `{"done":true}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH cross-task deveria dar 404, veio %d", rec.Code)
	}
	rec = doJSON(s.router, http.MethodDelete, subPath(s.ws, s.task)+"/"+sub.ID, s.ownerCookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("DELETE cross-task deveria dar 404, veio %d", rec.Code)
	}
}

// CA-12: sem sessão → 401 nas rotas novas.
func TestSubtaskSemSessao401HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, subPath(s.ws, s.task), ""},
		{http.MethodPost, subPath(s.ws, s.task), `{"title":"x"}`},
		{http.MethodPatch, subPath(s.ws, s.task) + "/" + "00000000-0000-0000-0000-000000000000", `{"done":true}`},
		{http.MethodDelete, subPath(s.ws, s.task) + "/" + "00000000-0000-0000-0000-000000000000", ""},
	} {
		rec := doJSON(s.router, tc.method, tc.path, nil, tc.body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s sem sessão = %d, esperava 401", tc.method, tc.path, rec.Code)
		}
	}
}

// CA-02/03: POST/PATCH de task com priority/notion_url retornam os campos.
func TestTaskCamposV2HTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedSubtaskHTTP(t, pool)
	// project_id do s.task (precisamos do projeto shared) — pega via a task.
	var projID string
	if err := pool.QueryRow(context.Background(), `SELECT project_id FROM tasks WHERE id = $1`, s.task).Scan(&projID); err != nil {
		t.Fatalf("project_id: %v", err)
	}
	body := `{"project_id":"` + projID + `","name":"Churn","status":"backlog","priority":2,"notion_url":"https://www.notion.so/Churn-8f3a1c9b"}`
	rec := doJSON(s.router, http.MethodPost, "/api/v1/workspaces/"+s.ws+"/tasks", s.ownerCookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST task = %d (%s)", rec.Code, rec.Body.String())
	}
	var task struct {
		ID        string  `json:"id"`
		Status    string  `json:"status"`
		Priority  int     `json:"priority"`
		NotionURL *string `json:"notion_url"`
		Position  int     `json:"position"`
	}
	json.Unmarshal(rec.Body.Bytes(), &task)
	if task.Status != "backlog" || task.Priority != 2 || task.NotionURL == nil {
		t.Errorf("POST task v2 devolveu %+v", task)
	}
	// PATCH notion_url:null limpa.
	rec = doJSON(s.router, http.MethodPatch, "/api/v1/workspaces/"+s.ws+"/tasks/"+task.ID, s.ownerCookie, `{"notion_url":null,"priority":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH task = %d (%s)", rec.Code, rec.Body.String())
	}
	var patched struct {
		Priority  int     `json:"priority"`
		NotionURL *string `json:"notion_url"`
	}
	json.Unmarshal(rec.Body.Bytes(), &patched)
	if patched.NotionURL != nil || patched.Priority != 3 {
		t.Errorf("PATCH task v2 devolveu %+v", patched)
	}
}
