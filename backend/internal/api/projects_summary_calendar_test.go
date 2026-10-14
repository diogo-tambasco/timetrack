package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type summaryRow struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Color         string  `json:"color"`
	ClientID      *string `json:"client_id"`
	Deadline      *string `json:"deadline"`
	TaskCount     int     `json:"task_count"`
	DoneCount     int     `json:"done_count"`
	OpenCount     int     `json:"open_count"`
	TrackedSecond int64   `json:"tracked_seconds"`
	DueTodayCount int     `json:"due_today_count"`
}

type calItem struct {
	Kind   string  `json:"kind"`
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Date   string  `json:"date"`
	Status *string `json:"status"`
}

// Fase 28 T-05 (CA-01) — deadline no corpo de POST/PATCH/GET de project.
func TestProjectDeadlineNosHandlers(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana")
	ws := workspaceIDOf(t, pool, "g-owner")

	// POST com deadline.
	p := criarProjeto(t, router, ws, cookie,
		`{"name":"Acme","color":"#4f9dde","visibility":"shared","deadline":"2025-09-30"}`)
	// GET traz deadline.
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Deadline *string `json:"deadline"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Deadline == nil || *got.Deadline != "2025-09-30" {
		t.Errorf("GET deveria trazer deadline 2025-09-30, veio %v", got.Deadline)
	}

	// PATCH null remove.
	rec = doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookie, `{"deadline":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch null: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Deadline != nil {
		t.Errorf("PATCH null deveria remover o deadline, veio %v", *got.Deadline)
	}

	// PATCH malformado → 400.
	rec = doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookie, `{"deadline":"2025-13-40"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("deadline malformado: esperado 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
}

// Fase 28 T-05 (CA-04/06) — summary 200 com a lista; tz inválido → 400.
func TestProjectsSummaryHandler(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana")
	ws := workspaceIDOf(t, pool, "g-owner")

	p := criarProjeto(t, router, ws, cookie,
		`{"name":"Acme","color":"#4f9dde","visibility":"shared","deadline":"2025-09-30"}`)
	// 1 task done por SQL direto.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, status)
		 SELECT $1, $2, 'T', u.id, 'done' FROM users u WHERE u.google_id = 'g-owner'`,
		ws, p.ID); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/summary?tz=America/Sao_Paulo", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("summary: esperado 200, veio %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Projects []summaryRow `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	var row *summaryRow
	for i := range env.Projects {
		if env.Projects[i].ID == p.ID {
			row = &env.Projects[i]
		}
	}
	if row == nil {
		t.Fatal("projeto não veio no summary")
	}
	if row.TaskCount != 1 || row.DoneCount != 1 || row.OpenCount != 0 {
		t.Errorf("contadores errados no summary: %+v", row)
	}
	if row.Deadline == nil || *row.Deadline != "2025-09-30" {
		t.Errorf("summary deveria trazer deadline, veio %v", row.Deadline)
	}

	// tz inválido → 400.
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/summary?tz=Nao/Existe", cookie, "")
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("tz inválido: esperado 400, veio %d %s", rec.Code, rec.Body.String())
	}
}

// Fase 28 T-05 (CA-10/11/12) — calendar 200 com {items,from,to}; 400 month
// malformado; 404 projeto invisível/malformado.
func TestProjectCalendarHandler(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana")
	ws := workspaceIDOf(t, pool, "g-owner")

	p := criarProjeto(t, router, ws, cookie,
		`{"name":"Acme","color":"#4f9dde","visibility":"shared","deadline":"2025-06-30"}`)
	// task com due em junho.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, status, due_date)
		 SELECT $1, $2, 'Fechar OKRs', u.id, 'doing', '2025-06-25' FROM users u WHERE u.google_id = 'g-owner'`,
		ws, p.ID); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/"+p.ID+"/calendar?month=2025-06", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("calendar: esperado 200, veio %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		From  string    `json:"from"`
		To    string    `json:"to"`
		Items []calItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	if env.From != "2025-06-01" || env.To != "2025-06-30" {
		t.Errorf("janela errada: %s..%s", env.From, env.To)
	}
	var hasTask, hasDeadline bool
	for _, it := range env.Items {
		if it.Kind == "task" && it.Date == "2025-06-25" {
			hasTask = true
		}
		if it.Kind == "project_deadline" && it.Date == "2025-06-30" {
			hasDeadline = true
		}
	}
	if !hasTask || !hasDeadline {
		t.Errorf("itens esperados ausentes: task=%v deadline=%v (%+v)", hasTask, hasDeadline, env.Items)
	}

	// month malformado → 400.
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/"+p.ID+"/calendar?month=2025-13", cookie, "")
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("month malformado: esperado 400, veio %d %s", rec.Code, rec.Body.String())
	}

	// projeto malformado → 404.
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects/nao-uuid/calendar?month=2025-06", cookie, "")
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("projectId malformado: esperado 404, veio %d %s", rec.Code, rec.Body.String())
	}
}

// Fase 28 T-05/T-06 (CA-15) — 401 sem sessão nas rotas novas.
func TestSummaryCalendarSemSessao401(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana")
	ws := workspaceIDOf(t, pool, "g-owner")
	p := criarProjeto(t, router, ws, cookie, `{"name":"P","color":"#4f9dde","visibility":"shared"}`)

	for _, path := range []string{
		"/api/v1/workspaces/" + ws + "/projects/summary",
		"/api/v1/workspaces/" + ws + "/projects/" + p.ID + "/calendar?month=2025-06",
	} {
		rec := doJSON(router, http.MethodGet, path, nil, "")
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("%s sem sessão: esperado 401 unauthorized, veio %d %s", path, rec.Code, rec.Body.String())
		}
	}
}
