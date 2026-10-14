// objectives_handlers_test.go — fase 12 T-06: REST de objetivos + board via o
// harness coretest (router real). CA-01..08/13/13.1/14/16.
package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func bodyID(t *testing.T, body []byte) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("corpo sem id: %s", body)
	}
	return v.ID
}

func TestObjectivesHandlersFluxo(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	// cria projeto shared
	rec := doJSON(router, http.MethodPost, base+"/projects", cookie,
		`{"name":"P","color":"#4f9dde","visibility":"shared"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar projeto: %d %s", rec.Code, rec.Body.String())
	}
	pid := bodyID(t, rec.Body.Bytes())
	objBase := base + "/projects/" + pid + "/objectives"

	// GET vazio → 200 {objectives:[]}
	rec = doJSON(router, http.MethodGet, objBase, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list vazio: %d %s", rec.Code, rec.Body.String())
	}

	// POST objetivo com meta → 201
	rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"Lançar v2","goal_hours":12}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar objetivo: %d %s", rec.Code, rec.Body.String())
	}
	oid := bodyID(t, rec.Body.Bytes())

	// POST inválido: nome vazio → 400; goal_hours > 9999 → 400
	if rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"   "}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("nome vazio deveria dar 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
	if rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"x","goal_hours":10000}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("goal_hours>9999 deveria dar 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}

	// PATCH goal_hours:null remove; status:done fecha
	if rec = doJSON(router, http.MethodPatch, objBase+"/"+oid, cookie, `{"goal_hours":null}`); rec.Code != http.StatusOK {
		t.Fatalf("patch goal null: %d %s", rec.Code, rec.Body.String())
	}
	var patched struct {
		GoalHours *float64 `json:"goal_hours"`
		Status    string   `json:"status"`
	}
	json.Unmarshal(rec.Body.Bytes(), &patched)
	if patched.GoalHours != nil {
		t.Errorf("goal_hours:null deveria remover a meta, veio %v", *patched.GoalHours)
	}
	if rec = doJSON(router, http.MethodPatch, objBase+"/"+oid, cookie, `{"status":"done"}`); rec.Code != http.StatusOK {
		t.Fatalf("patch status done: %d %s", rec.Code, rec.Body.String())
	}

	// GET board → 200 com project.can_write
	rec = doJSON(router, http.MethodGet, base+"/projects/"+pid+"/board", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	var board struct {
		Project struct {
			CanWrite bool `json:"can_write"`
		} `json:"project"`
		Objectives []struct {
			ID string `json:"id"`
		} `json:"objectives"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatalf("board fora do shape: %v", err)
	}
	if !board.Project.CanWrite {
		t.Error("board.project.can_write deveria ser true pro dono")
	}
	if len(board.Objectives) != 1 {
		t.Errorf("board deveria ter 1 objetivo, veio %d", len(board.Objectives))
	}

	// DELETE → 204
	if rec = doJSON(router, http.MethodDelete, objBase+"/"+oid, cookie, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestObjectivesHandlersAuthEIsolamento(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	// dois projetos do dono
	pA := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie, `{"name":"A","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	pB := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie, `{"name":"B","color":"#000000","visibility":"shared"}`).Body.Bytes())
	// objetivo no B
	oB := bodyID(t, doJSON(router, http.MethodPost, base+"/projects/"+pB+"/objectives", cookie, `{"name":"do B"}`).Body.Bytes())

	// CA-13.1: objetivo do B via rota do A → 404
	if rec := doJSON(router, http.MethodPatch, base+"/projects/"+pA+"/objectives/"+oB, cookie, `{"name":"hack"}`); rec.Code != http.StatusNotFound {
		t.Errorf("objetivo de outro projeto via rota errada deveria dar 404, veio %d", rec.Code)
	}

	// 401 sem sessão
	if rec := doJSON(router, http.MethodGet, base+"/projects/"+pA+"/objectives", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem sessão deveria dar 401, veio %d", rec.Code)
	}

	// 403 viewer: outro user, viewer do mesmo workspace, tenta escrever
	vcookie := loginVia(t, svc, stub, "viewer", "viewer@b.c", "Viewer")
	addMember(t, pool, ws, userIDOf(t, pool, "viewer"), "viewer")
	if rec := doJSON(router, http.MethodPost, base+"/projects/"+pA+"/objectives", vcookie, `{"name":"x"}`); rec.Code != http.StatusForbidden {
		t.Errorf("viewer escrevendo deveria dar 403, veio %d %s", rec.Code, rec.Body.String())
	}
}
