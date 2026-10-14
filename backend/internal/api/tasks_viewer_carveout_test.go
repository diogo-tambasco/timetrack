package api_test

import (
	"context"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// Fase 18 T-04 — carve-out viewer-assignee (CA-08/09/10/10.1/11/24).
// O viewer só escreve numa exceção ESTREITA: PATCH só-status na task ONDE ELE é
// o assignee. Tudo o mais → 403. O carve-out herda o bloqueio de projeto
// arquivado (400). owner/member não dependem da exceção (CA-08).
func TestViewerAssigneeCarveOut(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vio Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	// task X atribuída ao viewer; task Y SEM assignee
	taskX := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"X","assignee":"`+idV+`"}`)
	taskY := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Y"}`)

	// CA-10: viewer-assignee PATCH só status → 200 e persiste
	rec := doJSON(router, http.MethodPatch, base+"/"+taskX.ID, cookieV, `{"status":"done"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer-assignee status-only: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM tasks WHERE id = $1`, taskX.ID).Scan(&status); err != nil || status != "done" {
		t.Errorf("status deveria ter persistido done, veio %q (err %v)", status, err)
	}

	// CA-11b: viewer-assignee tentando OUTRO campo na própria task → 403
	for _, body := range []string{
		`{"name":"Renomeada"}`,
		`{"due_date":"2026-08-01"}`,
		`{"notes":"nota"}`,
		`{"assignee":null}`,
		`{"status":"todo","name":"junto"}`, // status + outro campo também é 403
		// status + CADA campo: cada combo mata a remoção da cláusula correspondente
		// do onlyStatus (CA-11b — o carve-out não pode escalar pra esses campos).
		`{"status":"done","notes":"x"}`,
		`{"status":"done","due_date":"2026-08-01"}`,
		`{"status":"done","recurrence":"weekly"}`,
		`{"status":"done","objective_id":null}`,
		`{"status":"done","requested_by":null}`,
	} {
		rec := doJSON(router, http.MethodPatch, base+"/"+taskX.ID, cookieV, body)
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
			t.Errorf("viewer-assignee PATCH %s: esperado 403, veio %d: %s", body, rec.Code, rec.Body.String())
		}
	}

	// CA-11a: viewer NÃO-assignee mudando status de Y → 403
	rec = doJSON(router, http.MethodPatch, base+"/"+taskY.ID, cookieV, `{"status":"done"}`)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Errorf("viewer não-assignee PATCH status: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}

	// CA-09: viewer POST task → 403 (barrado no middleware, não chega no service)
	rec = doJSON(router, http.MethodPost, base, cookieV, `{"project_id":"`+p.ID+`","name":"Nova"}`)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Errorf("viewer POST: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}
	// CA-09: viewer DELETE task → 403 (middleware)
	rec = doJSON(router, http.MethodDelete, base+"/"+taskX.ID, cookieV, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer DELETE: esperado 403, veio %d", rec.Code)
	}

	// CA-08: owner segue editando qualquer campo normalmente
	rec = doJSON(router, http.MethodPatch, base+"/"+taskX.ID, cookieA, `{"name":"Owner editou"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("owner PATCH qualquer campo: esperado 200, veio %d", rec.Code)
	}
}

// CA-10.1 — carve-out viewer-assignee herda o bloqueio de projeto arquivado:
// PATCH só-status do viewer-assignee num projeto arquivado → 400 invalid_reference
// (o MESMO que owner/member levam pelo loadProjectGate). Mutação que pulasse o
// bloqueio (deixando o viewer-assignee gravar status em arquivado) mata o teste.
func TestViewerAssigneeProjetoArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vio Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"Velho","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"
	taskX := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"X","assignee":"`+idV+`"}`)

	// arquiva o projeto
	if rec := doJSON(router, http.MethodDelete, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookieA, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("arquivar projeto: %d", rec.Code)
	}

	// owner/member levam 400 invalid_reference; o viewer-assignee deve levar o MESMO
	rec := doJSON(router, http.MethodPatch, base+"/"+taskX.ID, cookieV, `{"status":"done"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("viewer-assignee status em projeto arquivado: esperado 400 invalid_reference, veio %d: %s", rec.Code, rec.Body.String())
	}
	// e nada gravou
	var status string
	_ = pool.QueryRow(context.Background(), `SELECT status FROM tasks WHERE id = $1`, taskX.ID).Scan(&status)
	if status != "todo" {
		t.Errorf("status não deveria ter mudado em projeto arquivado, veio %q", status)
	}
}
