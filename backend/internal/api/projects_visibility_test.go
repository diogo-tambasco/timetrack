package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func listProjects(t *testing.T, router http.Handler, ws string, cookie *http.Cookie) []projectJSON {
	t.Helper()
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/projects", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /projects: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Projects []projectJSON `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Projects
}

// CA-09 — projeto private só existe pro dono: fora da listagem dos outros
// e 404 em detalhe/PATCH/DELETE/PUT members — INCLUSIVE pro owner do
// workspace (privacidade absoluta sobre poder administrativo).
func TestProjetoPrivadoInvisivelParaOutros(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vivi Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	addMember(t, pool, ws, userIDOf(t, pool, "g-v"), "viewer")
	base := "/api/v1/workspaces/" + ws + "/projects"

	privado := criarProjeto(t, router, ws, cookieM, `{"name":"Segredo de M","color":"#4f9dde","visibility":"private"}`)

	// fora da listagem de todos que não são o dono
	for nome, cookie := range map[string]*http.Cookie{"ws owner": cookieA, "viewer": cookieV} {
		for _, p := range listProjects(t, router, ws, cookie) {
			if p.ID == privado.ID {
				t.Errorf("projeto private de M vazou na listagem de %s", nome)
			}
		}
	}
	// o dono vê o próprio private na lista
	achou := false
	for _, p := range listProjects(t, router, ws, cookieM) {
		if p.ID == privado.ID {
			achou = true
		}
	}
	if !achou {
		t.Error("o dono deveria ver o próprio projeto private na listagem")
	}

	// detalhe: 404 idêntico a inexistente pra quem não é o dono
	for nome, cookie := range map[string]*http.Cookie{"ws owner": cookieA, "viewer": cookieV} {
		rec := doJSON(router, http.MethodGet, base+"/"+privado.ID, cookie, "")
		if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
			t.Errorf("detalhe do private como %s: esperado 404 not_found, veio %d: %s",
				nome, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(router, http.MethodGet, base+"/"+privado.ID, cookieM, ""); rec.Code != http.StatusOK {
		t.Errorf("detalhe do private pro dono: esperado 200, veio %d", rec.Code)
	}

	// escrita pelo owner do ws: PATCH/DELETE/PUT members → o MESMO 404
	// (a rota PUT .../members nasce na T-08; a visibilidade lá mantém o 404)
	for _, caso := range []struct{ method, path, body string }{
		{http.MethodPatch, base + "/" + privado.ID, `{"name":"X"}`},
		{http.MethodDelete, base + "/" + privado.ID, ""},
		{http.MethodPut, base + "/" + privado.ID + "/members", `{"members":[]}`},
	} {
		rec := doJSON(router, caso.method, caso.path, cookieA, caso.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s como ws owner: esperado 404, veio %d", caso.method, caso.path, rec.Code)
		}
	}
}

// CA-10 — projeto shared aparece na listagem e no detalhe pra owner,
// member e viewer do workspace.
func TestProjetoCompartilhadoVisivelPorTodos(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vivi Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	addMember(t, pool, ws, userIDOf(t, pool, "g-v"), "viewer")
	base := "/api/v1/workspaces/" + ws + "/projects"

	shared := criarProjeto(t, router, ws, cookieM, `{"name":"De todos","color":"#22c55e","visibility":"shared"}`)

	for nome, cookie := range map[string]*http.Cookie{
		"owner": cookieA, "member dono": cookieM, "viewer": cookieV,
	} {
		achou := false
		for _, p := range listProjects(t, router, ws, cookie) {
			if p.ID == shared.ID {
				achou = true
			}
		}
		if !achou {
			t.Errorf("%s deveria ver o projeto shared na listagem", nome)
		}
		if rec := doJSON(router, http.MethodGet, base+"/"+shared.ID, cookie, ""); rec.Code != http.StatusOK {
			t.Errorf("detalhe do shared como %s: esperado 200, veio %d", nome, rec.Code)
		}
	}
}
