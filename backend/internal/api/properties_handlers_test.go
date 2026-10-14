// properties_handlers_test.go — fase 25 T-06: REST de defs + values via o harness
// coretest (router real). CA-01..12/14/16: 201/200/204, validações 400, viewer 403,
// arquivado 400 invalid_reference, pertencimento 404, 401 sem sessão, board 200 com
// property_defs + property_values.
package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func TestPropertiesHandlersFluxo(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	pid := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie,
		`{"name":"P","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	tid := bodyID(t, doJSON(router, http.MethodPost, base+"/tasks", cookie,
		`{"project_id":"`+pid+`","name":"T"}`).Body.Bytes())
	propBase := base + "/projects/" + pid + "/properties"

	// GET vazio → 200 {properties:[]}
	if rec := doJSON(router, http.MethodGet, propBase, cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("list vazio: %d %s", rec.Code, rec.Body.String())
	}

	// POST select com options → 201 + persiste
	rec := doJSON(router, http.MethodPost, propBase, cookie,
		`{"name":"Cliente","type":"select","options":[{"label":"Lauda"},{"label":"Arkan"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar def: %d %s", rec.Code, rec.Body.String())
	}
	var def struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Options []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"options"`
	}
	json.Unmarshal(rec.Body.Bytes(), &def)
	if def.ID == "" || def.Type != "select" || len(def.Options) != 2 || def.Options[0].ID == "" {
		t.Fatalf("def select inesperada: %s", rec.Body.String())
	}
	optID := def.Options[0].ID

	// POST inválidos → 400 validation_error
	for _, bad := range []string{
		`{"name":"","type":"text"}`,    // nome vazio (CA-01.1)
		`{"name":"X","type":"person"}`, // tipo adiado (CA-02)
		`{"name":"X","type":"foo"}`,    // tipo inválido (CA-02)
		`{"name":"S","type":"select"}`, // select sem options (CA-03)
	} {
		if rec := doJSON(router, http.MethodPost, propBase, cookie, bad); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("payload %s deveria dar 400 validation_error, veio %d %s", bad, rec.Code, rec.Body.String())
		}
	}

	// PATCH renomeia → 200
	if rec := doJSON(router, http.MethodPatch, propBase+"/"+def.ID, cookie, `{"name":"Conta"}`); rec.Code != http.StatusOK {
		t.Fatalf("patch def: %d %s", rec.Code, rec.Body.String())
	}

	// PUT value (option válida) → 200 + persiste
	valURL := base + "/projects/" + pid + "/tasks/" + tid + "/properties/" + def.ID
	if rec := doJSON(router, http.MethodPut, valURL, cookie, `{"value":"`+optID+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("set value: %d %s", rec.Code, rec.Body.String())
	}
	// PUT value inválido (option inexistente) → 400
	if rec := doJSON(router, http.MethodPut, valURL, cookie, `{"value":"op_zzz"}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("value inválido deveria dar 400, veio %d %s", rec.Code, rec.Body.String())
	}

	// board → 200 com property_defs + property_values
	rec = doJSON(router, http.MethodGet, base+"/projects/"+pid+"/board", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	var board struct {
		Project struct {
			PropertyDefs []struct {
				ID string `json:"id"`
			} `json:"property_defs"`
		} `json:"project"`
		General struct {
			Tasks []struct {
				PropertyValues []struct {
					DefID string          `json:"def_id"`
					Value json.RawMessage `json:"value"`
				} `json:"property_values"`
			} `json:"tasks"`
		} `json:"general"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatalf("board fora do shape: %v", err)
	}
	if len(board.Project.PropertyDefs) != 1 {
		t.Errorf("board deveria ter 1 property_def, veio %d", len(board.Project.PropertyDefs))
	}
	if len(board.General.Tasks) != 1 || len(board.General.Tasks[0].PropertyValues) != 1 {
		t.Errorf("board task deveria ter 1 property_value, veio %s", rec.Body.String())
	}

	// PUT {value:null} → 204 e remove a linha (CA-09.1)
	if rec := doJSON(router, http.MethodPut, valURL, cookie, `{"value":null}`); rec.Code != http.StatusNoContent {
		t.Fatalf("set value null deveria dar 204, veio %d %s", rec.Code, rec.Body.String())
	}
	// idempotente: DELETE value sem value → 204 (CA-10)
	if rec := doJSON(router, http.MethodDelete, valURL, cookie, ""); rec.Code != http.StatusNoContent {
		t.Errorf("clear value idempotente deveria dar 204, veio %d", rec.Code)
	}

	// DELETE def → 204
	if rec := doJSON(router, http.MethodDelete, propBase+"/"+def.ID, cookie, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete def: %d %s", rec.Code, rec.Body.String())
	}
}

// TestSetValueCampoAusente (T-14 #2): PUT sem o campo `value` (`{}`) → 400 com a
// mensagem "value ausente" (guard do handler, CA-09.1), DISTINTO de `{value:null}`
// → 204 (clear). Sem o guard, `{}` ainda daria 400 (validateValue(nil) falha), mas
// com a mensagem "valor inválido" — assertar a mensagem mata a mutação que remove o
// guard.
func TestSetValueCampoAusente(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	pid := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie, `{"name":"P","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	tid := bodyID(t, doJSON(router, http.MethodPost, base+"/tasks", cookie, `{"project_id":"`+pid+`","name":"T"}`).Body.Bytes())
	defID := bodyID(t, doJSON(router, http.MethodPost, base+"/projects/"+pid+"/properties", cookie, `{"name":"N","type":"text"}`).Body.Bytes())
	valURL := base + "/projects/" + pid + "/tasks/" + tid + "/properties/" + defID

	// value ausente (`{}`) → 400 + mensagem "value ausente" (guard do handler)
	rec := doJSON(router, http.MethodPut, valURL, cookie, `{}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Fatalf("`{}` deveria dar 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Error struct{ Message string } `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Message != "value ausente" {
		t.Errorf("mensagem deveria ser \"value ausente\" (guard do handler), veio %q", env.Error.Message)
	}
	// contraste: `{value:null}` → 204 (clear idempotente)
	if rec := doJSON(router, http.MethodPut, valURL, cookie, `{"value":null}`); rec.Code != http.StatusNoContent {
		t.Errorf("`{value:null}` deveria dar 204, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestPropertiesHandlersAuthPermissaoIsolamento(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	pA := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie, `{"name":"A","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	pB := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie, `{"name":"B","color":"#000000","visibility":"shared"}`).Body.Bytes())
	defB := bodyID(t, doJSON(router, http.MethodPost, base+"/projects/"+pB+"/properties", cookie, `{"name":"B","type":"text"}`).Body.Bytes())
	taskA := bodyID(t, doJSON(router, http.MethodPost, base+"/tasks", cookie, `{"project_id":"`+pA+`","name":"TA"}`).Body.Bytes())

	// CA-12: def do B via rota do A → 404 (PATCH/DELETE)
	if rec := doJSON(router, http.MethodPatch, base+"/projects/"+pA+"/properties/"+defB, cookie, `{"name":"hack"}`); rec.Code != http.StatusNotFound {
		t.Errorf("def de outro projeto via rota errada deveria dar 404, veio %d", rec.Code)
	}
	// CA-12: value com def do B via rota do A → 404
	if rec := doJSON(router, http.MethodPut, base+"/projects/"+pA+"/tasks/"+taskA+"/properties/"+defB, cookie, `{"value":"x"}`); rec.Code != http.StatusNotFound {
		t.Errorf("value com def de outro projeto deveria dar 404, veio %d", rec.Code)
	}
	// CA-12: id malformado → 404 (não 500)
	if rec := doJSON(router, http.MethodPatch, base+"/projects/"+pA+"/properties/nao-uuid", cookie, `{"name":"x"}`); rec.Code != http.StatusNotFound {
		t.Errorf("defId malformado deveria dar 404, veio %d", rec.Code)
	}

	// CA-14: 401 sem sessão nas rotas novas (table-driven)
	noSess := []struct{ method, url, body string }{
		{http.MethodGet, base + "/projects/" + pA + "/properties", ""},
		{http.MethodPost, base + "/projects/" + pA + "/properties", `{"name":"x","type":"text"}`},
		{http.MethodPut, base + "/projects/" + pA + "/tasks/" + taskA + "/properties/" + defB, `{"value":"x"}`},
		{http.MethodDelete, base + "/projects/" + pA + "/tasks/" + taskA + "/properties/" + defB, ""},
	}
	for _, c := range noSess {
		if rec := doJSON(router, c.method, c.url, nil, c.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s sem sessão deveria dar 401, veio %d", c.method, c.url, rec.Code)
		}
	}

	// CA-07: viewer escrevendo def → 403
	vcookie := loginVia(t, svc, stub, "viewer", "viewer@b.c", "Viewer")
	addMember(t, pool, ws, userIDOf(t, pool, "viewer"), "viewer")
	if rec := doJSON(router, http.MethodPost, base+"/projects/"+pA+"/properties", vcookie, `{"name":"x","type":"text"}`); rec.Code != http.StatusForbidden {
		t.Errorf("viewer criando def deveria dar 403, veio %d %s", rec.Code, rec.Body.String())
	}

	// CA-11: escrita em projeto ARQUIVADO → 400 invalid_reference (NÃO 'archived')
	defArch := bodyID(t, doJSON(router, http.MethodPost, base+"/projects/"+pA+"/properties", cookie, `{"name":"N","type":"text"}`).Body.Bytes())
	if _, err := pool.Exec(t.Context(), `UPDATE projects SET archived_at = now() WHERE id = $1`, pA); err != nil {
		t.Fatalf("arquivar projeto: %v", err)
	}
	if rec := doJSON(router, http.MethodPost, base+"/projects/"+pA+"/properties", cookie, `{"name":"Z","type":"text"}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("CreateDef em arquivado deveria dar 400 invalid_reference, veio %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(router, http.MethodPut, base+"/projects/"+pA+"/tasks/"+taskA+"/properties/"+defArch, cookie, `{"value":"x"}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Errorf("SetValue em arquivado deveria dar 400 invalid_reference, veio %d %s", rec.Code, rec.Body.String())
	}
}
