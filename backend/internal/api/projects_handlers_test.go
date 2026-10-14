package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type projectJSON struct {
	ID              string   `json:"id"`
	WorkspaceID     string   `json:"workspace_id"`
	ClientID        *string  `json:"client_id"`
	Name            string   `json:"name"`
	Color           string   `json:"color"`
	WeeklyHoursGoal *float64 `json:"weekly_hours_goal"`
	Visibility      string   `json:"visibility"`
	OwnerUserID     string   `json:"owner_user_id"`
	ArchivedAt      *string  `json:"archived_at"`
	CreatedAt       string   `json:"created_at"`
}

func criarProjeto(t *testing.T, router http.Handler, ws string, cookie *http.Cookie, body string) projectJSON {
	t.Helper()
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/projects", cookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar projeto: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var p projectJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return p
}

// CA-06 — POST /projects válido cria com owner_user_id da sessão e 201.
func TestCriarProjeto201(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	me := userIDOf(t, pool, "g-owner")

	p := criarProjeto(t, router, ws, cookie,
		`{"name":"Acme — App mobile","color":"#4f9dde","weekly_hours_goal":18,"visibility":"shared"}`)
	if p.ID == "" || p.WorkspaceID != ws || p.Name != "Acme — App mobile" ||
		p.Color != "#4f9dde" || p.WeeklyHoursGoal == nil || *p.WeeklyHoursGoal != 18 ||
		p.Visibility != "shared" || p.OwnerUserID != me || p.ArchivedAt != nil || p.CreatedAt == "" {
		t.Errorf("objeto fora do contrato §3.2: %+v", p)
	}

	// validações chegam na borda HTTP com envelope canônico (CA-07)
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/projects", cookie,
		`{"name":"App","color":"azul","visibility":"shared"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("cor inválida: esperado 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}

	// visibility é obrigatória, sem default no servidor (D-10)
	rec = doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/projects", cookie,
		`{"name":"App","color":"#4f9dde"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("sem visibility: esperado 400 validation_error, veio %d", rec.Code)
	}
}

// CA-14 (+CA-09 na escrita) — metadados: dono 200; owner do ws 200 em
// shared; member comum 403 em shared; private de outro = 404 pra TODOS
// (inclusive owner do ws — privacidade absoluta).
func TestPatchProjetoSoDonoOuOwner(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	cookieM2 := loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m2"), "member")
	base := "/api/v1/workspaces/" + ws + "/projects"

	// M é dono do projeto shared
	shared := criarProjeto(t, router, ws, cookieM, `{"name":"Compartilhado","color":"#4f9dde","visibility":"shared"}`)

	caso := func(nome string, rec *httptest.ResponseRecorder, status int) {
		t.Helper()
		if rec.Code != status {
			t.Errorf("%s: esperado %d, veio %d: %s", nome, status, rec.Code, rec.Body.String())
		}
	}

	// dono edita metadados → 200 com objeto atualizado
	rec := doJSON(router, http.MethodPatch, base+"/"+shared.ID, cookieM, `{"name":"Renomeado"}`)
	caso("dono PATCH shared", rec, http.StatusOK)
	var atualizado projectJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &atualizado)
	if atualizado.Name != "Renomeado" {
		t.Errorf("PATCH não aplicou o nome: %+v", atualizado)
	}

	// owner do workspace (não-dono) → 200 em projeto shared
	caso("ws owner PATCH shared",
		doJSON(router, http.MethodPatch, base+"/"+shared.ID, cookieA, `{"color":"#22c55e"}`),
		http.StatusOK)

	// member comum (não-dono, não-owner) → 403 em shared
	rec = doJSON(router, http.MethodPatch, base+"/"+shared.ID, cookieM2, `{"name":"Hack"}`)
	caso("member comum PATCH shared", rec, http.StatusForbidden)
	if errCode(t, rec) != "forbidden" {
		t.Errorf("error.code esperado forbidden, veio %q", errCode(t, rec))
	}

	// projeto private de M: TODOS os outros recebem 404 — inclusive o
	// owner do workspace (CA-09, privacidade absoluta)
	privado := criarProjeto(t, router, ws, cookieM, `{"name":"Privado","color":"#4f9dde","visibility":"private"}`)
	for nome, cookie := range map[string]*http.Cookie{"ws owner": cookieA, "member comum": cookieM2} {
		rec := doJSON(router, http.MethodPatch, base+"/"+privado.ID, cookie, `{"name":"X"}`)
		caso(nome+" PATCH private de outro", rec, http.StatusNotFound)
		rec = doJSON(router, http.MethodDelete, base+"/"+privado.ID, cookie, "")
		caso(nome+" DELETE private de outro", rec, http.StatusNotFound)
	}

	// o dono segue editando o próprio private
	caso("dono PATCH private próprio",
		doJSON(router, http.MethodPatch, base+"/"+privado.ID, cookieM, `{"name":"Privado v2"}`),
		http.StatusOK)

	// inexistente → mesmo 404
	caso("PATCH inexistente",
		doJSON(router, http.MethodPatch, base+"/00000000-0000-0000-0000-000000000000", cookieM, `{"name":"X"}`),
		http.StatusNotFound)

	// validação no PATCH → 400
	rec = doJSON(router, http.MethodPatch, base+"/"+shared.ID, cookieM, `{"color":"azul"}`)
	caso("PATCH cor inválida", rec, http.StatusBadRequest)
}

// CA-15 — DELETE arquiva (archived_at), tasks ficam intactas; some da
// listagem default e volta com ?archived=true.
func TestArquivarProjetoMantemTasks(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	me := userIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/projects"
	ctx := context.Background()

	p := criarProjeto(t, router, ws, cookie, `{"name":"Com tasks","color":"#4f9dde","visibility":"shared"}`)

	// task do projeto via SQL (handlers de task chegam na T-09)
	var taskID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO tasks (workspace_id, project_id, name, created_by)
		VALUES ($1, $2, 'Task viva', $3) RETURNING id`, ws, p.ID, me).Scan(&taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	rec := doJSON(router, http.MethodDelete, base+"/"+p.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}

	var arquivado bool
	if err := pool.QueryRow(ctx,
		`SELECT archived_at IS NOT NULL FROM projects WHERE id = $1`, p.ID).Scan(&arquivado); err != nil || !arquivado {
		t.Errorf("projeto deveria existir com archived_at preenchido (err=%v)", err)
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 1 {
		t.Errorf("a task do projeto arquivado deveria continuar existindo, há %d", n)
	}

	var lista struct {
		Projects []projectJSON `json:"projects"`
	}
	rec = doJSON(router, http.MethodGet, base, cookie, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista.Projects) != 0 {
		t.Errorf("listagem default não deveria incluir arquivado, veio %+v", lista.Projects)
	}

	rec = doJSON(router, http.MethodGet, base+"?archived=true", cookie, "")
	lista.Projects = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista.Projects) != 1 || lista.Projects[0].ID != p.ID {
		t.Errorf("?archived=true deveria incluir o arquivado, veio %+v", lista.Projects)
	}
}

// CA-15.1 — detalhe de projeto arquivado responde 200 com archived_at
// preenchido (só as listagens omitem por default).
func TestDetalheProjetoArquivado200(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/projects"

	p := criarProjeto(t, router, ws, cookie, `{"name":"Aposentado","color":"#4f9dde","visibility":"shared"}`)
	if rec := doJSON(router, http.MethodDelete, base+"/"+p.ID, cookie, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("arquivar: %d", rec.Code)
	}

	rec := doJSON(router, http.MethodGet, base+"/"+p.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET detalhe arquivado: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var detalhe projectJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &detalhe); err != nil {
		t.Fatal(err)
	}
	if detalhe.ID != p.ID || detalhe.ArchivedAt == nil {
		t.Errorf("detalhe deveria vir com archived_at preenchido: %+v", detalhe)
	}
}
