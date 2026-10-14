package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// Fase 18 T-02 — CA-02: POST aceita assignee/requested_by (uuids de membros) e
// grava; quando ausentes a task nasce com ambos null (não regride).
func TestCriarTaskComAssignee(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)

	tk := criarTask(t, router, ws, cookieA,
		`{"project_id":"`+p.ID+`","name":"Delegada","assignee":"`+idB+`","requested_by":"`+idB+`"}`)
	if tk.Assignee == nil || *tk.Assignee != idB || tk.RequestedBy == nil || *tk.RequestedBy != idB {
		t.Errorf("assignee/requested_by deveriam persistir: %+v", tk)
	}

	// ausentes → ambos null (CA-02 não regride)
	tk2 := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Sem responsável"}`)
	if tk2.Assignee != nil || tk2.RequestedBy != nil {
		t.Errorf("assignee/requested_by deveriam nascer null quando ausentes: %+v", tk2)
	}
}

// Fase 18 T-02 — CA-03/06: PATCH tri-state de assignee/requested_by.
// uuid define; null limpa; ausente mantém. Shape da Task não muda (CA-07).
func TestPatchTaskAssigneeTriState(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idA := userIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"T"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks/" + tk.ID

	// define assignee=B, requested_by=A
	rec := doJSON(router, http.MethodPatch, base, cookieA,
		`{"assignee":"`+idB+`","requested_by":"`+idA+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH define: 200 esperado, veio %d: %s", rec.Code, rec.Body.String())
	}
	var v taskJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Assignee == nil || *v.Assignee != idB || v.RequestedBy == nil || *v.RequestedBy != idA {
		t.Errorf("PATCH deveria definir assignee=B/requested_by=A: %+v", v)
	}

	// ausente mantém: patch só do nome não mexe no assignee
	rec = doJSON(router, http.MethodPatch, base, cookieA, `{"name":"Renomeada"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Assignee == nil || *v.Assignee != idB || v.RequestedBy == nil || *v.RequestedBy != idA {
		t.Errorf("PATCH sem tocar assignee deveria manter os valores: %+v", v)
	}

	// null limpa o assignee (mas mantém requested_by — só o campo enviado muda)
	rec = doJSON(router, http.MethodPatch, base, cookieA, `{"assignee":null}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if rec.Code != http.StatusOK || v.Assignee != nil {
		t.Errorf("assignee:null deveria limpar (%d): %+v", rec.Code, v)
	}
	if v.RequestedBy == nil || *v.RequestedBy != idA {
		t.Errorf("requested_by não enviado deveria seguir A: %+v", v)
	}

	// null limpa requested_by também
	rec = doJSON(router, http.MethodPatch, base, cookieA, `{"requested_by":null}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.RequestedBy != nil {
		t.Errorf("requested_by:null deveria limpar: %+v", v)
	}

	// shape da Task inalterado (CA-07): persistiu no banco
	var dbAssignee *string
	if err := pool.QueryRow(context.Background(),
		`SELECT assignee FROM tasks WHERE id = $1`, tk.ID).Scan(&dbAssignee); err != nil {
		t.Fatal(err)
	}
	if dbAssignee != nil {
		t.Errorf("assignee deveria estar null no banco, veio %v", *dbAssignee)
	}
}

// Fase 18 (verificação do build) — CA-23: o ramo de TRANSIÇÃO do Patch (mudança de
// status, que faz o UPDATE em tx + recalcObjectiveStatus) DEVE preservar
// assignee/requested_by no write-back. É o fluxo nuclear do cross-assign ("o
// assignee marca a própria task como done"). Mutação que zerasse os campos no
// UPDATE de transição passaria verde sem este teste (a task ficaria done, mas sem
// responsável — não-asserido em lugar nenhum).
func TestPatchStatusPreservaAssignee(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA,
		`{"project_id":"`+p.ID+`","name":"T","assignee":"`+idB+`","requested_by":"`+idB+`"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks/" + tk.ID

	// PATCH só status (todo→done) → ramo de TRANSIÇÃO (UPDATE em tx + recalc)
	rec := doJSON(router, http.MethodPatch, base, cookieA, `{"status":"done"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status: 200 esperado, veio %d: %s", rec.Code, rec.Body.String())
	}
	var v taskJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Status != "done" {
		t.Errorf("status deveria ser done: %+v", v)
	}
	if v.Assignee == nil || *v.Assignee != idB || v.RequestedBy == nil || *v.RequestedBy != idB {
		t.Errorf("transição de status deveria PRESERVAR assignee/requested_by: %+v", v)
	}
	// confirma no banco, não só na resposta
	var dbA, dbR *string
	if err := pool.QueryRow(context.Background(),
		`SELECT assignee, requested_by FROM tasks WHERE id = $1`, tk.ID).Scan(&dbA, &dbR); err != nil {
		t.Fatal(err)
	}
	if dbA == nil || *dbA != idB || dbR == nil || *dbR != idB {
		t.Errorf("banco deveria preservar assignee/requested_by após transição: a=%v r=%v", dbA, dbR)
	}
}
