// reminder_routes_test.go — REST dos lembretes (fase 20 T-06): POST manual
// (201/400/403/404), DELETE (204 dono / 404 alheio / idempotente), GET
// /api/v1/reminders (só os meus, cross-ws, ordenado), 401 sem sessão nas 3 rotas.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/reminders"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/workspace"
)

type reminderJSON struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	TaskID      string `json:"task_id"`
	WorkspaceID string `json:"workspace_id"`
	RemindAt    string `json:"remind_at"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	TaskName    string `json:"task_name"`
}

// newReminderRouter monta o router REAL com auth (01) + core (02) + reminders (20).
func newReminderRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasks.NewService(pool),
			Objectives: objectives.NewService(pool),
		}),
		api.WithReminders(api.Reminders{Svc: reminders.NewService(reminders.NewStore(pool))}),
	)
	return router, svc, stub
}

func remindersTaskPath(ws, task string) string {
	return "/api/v1/workspaces/" + ws + "/tasks/" + task + "/reminders"
}

func setAssigneeSQL(t *testing.T, pool *pgxpool.Pool, taskID, userID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE tasks SET assignee = $2 WHERE id = $1`, taskID, userID); err != nil {
		t.Fatalf("set assignee SQL: %v", err)
	}
}

// CA-16/22: owner cria manual → 201 + reminder pending pro user da sessão.
func TestReminderCreate201(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	me := userIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Definir onboarding"}`)

	rec := doJSON(router, http.MethodPost, remindersTaskPath(ws, tk.ID), cookie,
		`{"remind_at":"2026-06-25T09:00:00-03:00"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST manual: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var rm reminderJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &rm); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	if rm.ID == "" || rm.Kind != "manual" || rm.Status != "pending" || rm.UserID != me || rm.TaskID != tk.ID {
		t.Errorf("reminder fora do contrato: %+v", rm)
	}
}

// CA-17: remind_at ausente/inválido/vazio → 400 validation_error.
func TestReminderCreateValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"T"}`)

	for _, body := range []string{`{}`, `{"remind_at":""}`, `{"remind_at":"amanhã"}`, `{"remind_at":"2026-06-25"}`} {
		rec := doJSON(router, http.MethodPost, remindersTaskPath(ws, tk.ID), cookie, body)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("body %q: esperado 400 validation_error, veio %d %s", body, rec.Code, rec.Body.String())
		}
	}
}

// CA-25: task invisível (private de outro) / taskId malformado → 404.
func TestReminderCreateTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	// projeto PRIVATE do A: o member M não enxerga
	p := criarProjeto(t, router, ws, cookieA, `{"name":"Secreto","color":"#4f9dde","visibility":"private"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"T"}`)

	rec := doJSON(router, http.MethodPost, remindersTaskPath(ws, tk.ID), cookieM,
		`{"remind_at":"2026-06-25T09:00:00Z"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("task invisível: esperado 404, veio %d", rec.Code)
	}
	// taskId malformado → 404
	rec = doJSON(router, http.MethodPost, remindersTaskPath(ws, "nao-é-uuid"), cookieA,
		`{"remind_at":"2026-06-25T09:00:00Z"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("taskId malformado: esperado 404, veio %d", rec.Code)
	}
}

// CA-23: viewer comum → 403; viewer ASSIGNEE da task → 201 (carve-out §8).
func TestReminderViewerCarveOut(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vic Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")
	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	// task comum → viewer 403
	t1 := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Comum"}`)
	rec := doJSON(router, http.MethodPost, remindersTaskPath(ws, t1.ID), cookieV,
		`{"remind_at":"2026-06-25T09:00:00Z"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer em task comum: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}

	// task onde o viewer é assignee → 201 (carve-out)
	t2 := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Dele"}`)
	setAssigneeSQL(t, pool, t2.ID, idV)
	rec = doJSON(router, http.MethodPost, remindersTaskPath(ws, t2.ID), cookieV,
		`{"remind_at":"2026-06-25T09:00:00Z"}`)
	if rec.Code != http.StatusCreated {
		t.Errorf("viewer-assignee: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-18/19: DELETE do dono → 204 + soft-cancel; de outro user → 404; idempotente.
func TestReminderDelete(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"T"}`)

	rec := doJSON(router, http.MethodPost, remindersTaskPath(ws, tk.ID), cookieA, `{"remind_at":"2026-06-25T09:00:00Z"}`)
	var rm reminderJSON
	json.Unmarshal(rec.Body.Bytes(), &rm)

	// outro user (member) tenta cancelar o reminder do A → 404 (não-vaza)
	rec = doJSON(router, http.MethodDelete, remindersTaskPath(ws, tk.ID)+"/"+rm.ID, cookieM, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete de reminder alheio: esperado 404, veio %d", rec.Code)
	}

	// dono cancela → 204 + status canceled
	rec = doJSON(router, http.MethodDelete, remindersTaskPath(ws, tk.ID)+"/"+rm.ID, cookieA, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete do dono: esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM reminders WHERE id = $1`, rm.ID).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "canceled" {
		t.Errorf("delete deveria fazer soft-cancel, status=%q", status)
	}

	// idempotente: 2º DELETE → 204 (existe e é dele)
	rec = doJSON(router, http.MethodDelete, remindersTaskPath(ws, tk.ID)+"/"+rm.ID, cookieA, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete idempotente: esperado 204, veio %d", rec.Code)
	}

	// id inexistente → 404
	rec = doJSON(router, http.MethodDelete, remindersTaskPath(ws, tk.ID)+"/00000000-0000-0000-0000-000000000000", cookieA, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete de id inexistente: esperado 404, veio %d", rec.Code)
	}
}

// CA-20/21: GET /api/v1/reminders só os do user, cross-ws, ordenado por remind_at.
func TestReminderListMine(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newReminderRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieB := loginVia(t, svc, stub, "g-b", "b@example.com", "Bia Owner")
	wsA := workspaceIDOf(t, pool, "g-a")
	wsB := workspaceIDOf(t, pool, "g-b")
	pA := criarProjeto(t, router, wsA, cookieA, `{"name":"PA","color":"#4f9dde","visibility":"shared"}`)
	pB := criarProjeto(t, router, wsB, cookieB, `{"name":"PB","color":"#4f9dde","visibility":"shared"}`)
	tA := criarTask(t, router, wsA, cookieA, `{"project_id":"`+pA.ID+`","name":"Tarefa A"}`)
	tB := criarTask(t, router, wsB, cookieB, `{"project_id":"`+pB.ID+`","name":"Tarefa B"}`)

	// A cria 2 (datas fora de ordem) ; B cria 1.
	doJSON(router, http.MethodPost, remindersTaskPath(wsA, tA.ID), cookieA, `{"remind_at":"2026-07-01T09:00:00Z"}`)
	doJSON(router, http.MethodPost, remindersTaskPath(wsA, tA.ID), cookieA, `{"remind_at":"2026-06-25T09:00:00Z"}`)
	doJSON(router, http.MethodPost, remindersTaskPath(wsB, tB.ID), cookieB, `{"remind_at":"2026-06-26T09:00:00Z"}`)

	rec := doJSON(router, http.MethodGet, "/api/v1/reminders", cookieA, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /reminders: esperado 200, veio %d", rec.Code)
	}
	var body struct {
		Reminders []reminderJSON `json:"reminders"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Reminders) != 2 {
		t.Fatalf("A deveria ver 2 reminders, veio %d", len(body.Reminders))
	}
	// CA-21: nenhum do B; e cada item leva task_name.
	for _, r := range body.Reminders {
		if r.TaskName != "Tarefa A" {
			t.Errorf("CA-21: vazou reminder de outra task/user — task_name %q", r.TaskName)
		}
	}
	// ordenado por remind_at: o de 25/06 vem antes do de 01/07.
	if body.Reminders[0].RemindAt > body.Reminders[1].RemindAt {
		t.Errorf("GET /reminders deveria ordenar por remind_at, veio %v", body.Reminders)
	}

	// status inválido → 400.
	rec = doJSON(router, http.MethodGet, "/api/v1/reminders?status=lixo", cookieA, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status inválido: esperado 400, veio %d", rec.Code)
	}
}

// CA-24: sem sessão → 401 nas três rotas.
func TestReminderSemSessao401(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, _, _ := newReminderRouter(t, pool)
	uuid := "00000000-0000-0000-0000-000000000000"

	for _, c := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/workspaces/" + uuid + "/tasks/" + uuid + "/reminders"},
		{http.MethodDelete, "/api/v1/workspaces/" + uuid + "/tasks/" + uuid + "/reminders/" + uuid},
		{http.MethodGet, "/api/v1/reminders"},
	} {
		rec := doJSON(router, c.method, c.path, nil, "")
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("%s %s sem sessão: esperado 401 unauthorized, veio %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}
