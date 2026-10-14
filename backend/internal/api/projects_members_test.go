package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

type memberJSON struct {
	UserID string `json:"user_id"`
	Access string `json:"access"`
}

// membersInDB lê project_members direto do banco, ordenado por user_id.
func membersInDB(t *testing.T, pool *pgxpool.Pool, projectID string) []memberJSON {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT user_id, access FROM project_members WHERE project_id = $1 ORDER BY user_id`, projectID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	list := []memberJSON{}
	for rows.Next() {
		var m memberJSON
		if err := rows.Scan(&m.UserID, &m.Access); err != nil {
			t.Fatal(err)
		}
		list = append(list, m)
	}
	return list
}

func seedProjectMember(t *testing.T, pool *pgxpool.Pool, projectID, userID, access string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, $3)`,
		projectID, userID, access); err != nil {
		t.Fatal(err)
	}
}

// CA-11 — PUT substitui a lista INTEIRA (não é merge); o dono é implícito
// e é filtrado se vier na lista; o owner do workspace também pode.
func TestSubstituirMembrosDoProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	loginVia(t, svc, stub, "g-m3", "m3@example.com", "Mel Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM := userIDOf(t, pool, "g-m")
	idM2 := userIDOf(t, pool, "g-m2")
	idM3 := userIDOf(t, pool, "g-m3")
	for _, id := range []string{idM, idM2, idM3} {
		addMember(t, pool, ws, id, "member")
	}

	p := criarProjeto(t, router, ws, cookieM, `{"name":"Squad","color":"#4f9dde","visibility":"shared"}`)
	membersURL := "/api/v1/workspaces/" + ws + "/projects/" + p.ID + "/members"

	// dono define a lista inicial
	rec := doJSON(router, http.MethodPut, membersURL, cookieM,
		`{"members":[{"user_id":"`+idM2+`","access":"edit"},{"user_id":"`+idM3+`","access":"view"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT members: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Members []memberJSON `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	sort.Slice(body.Members, func(i, j int) bool { return body.Members[i].UserID < body.Members[j].UserID })
	if len(body.Members) != 2 {
		t.Fatalf("resposta deveria ter os 2 membros persistidos, veio %+v", body.Members)
	}
	if got := membersInDB(t, pool, p.ID); len(got) != 2 {
		t.Fatalf("banco deveria ter 2 membros, tem %+v", got)
	}

	// substituição total: nova lista com só 1 → o outro sai
	rec = doJSON(router, http.MethodPut, membersURL, cookieM,
		`{"members":[{"user_id":"`+idM2+`","access":"view"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT substitutivo: %d %s", rec.Code, rec.Body.String())
	}
	got := membersInDB(t, pool, p.ID)
	if len(got) != 1 || got[0].UserID != idM2 || got[0].Access != "view" {
		t.Errorf("PUT deveria substituir a lista inteira, banco tem %+v", got)
	}

	// dono na lista é filtrado (implícito, não entra — D-06)
	rec = doJSON(router, http.MethodPut, membersURL, cookieM,
		`{"members":[{"user_id":"`+idM+`","access":"edit"},{"user_id":"`+idM3+`","access":"edit"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT com dono na lista: %d %s", rec.Code, rec.Body.String())
	}
	got = membersInDB(t, pool, p.ID)
	if len(got) != 1 || got[0].UserID != idM3 {
		t.Errorf("dono deveria ser filtrado da lista, banco tem %+v", got)
	}

	// owner do workspace também administra members de projeto shared (CA-11)
	rec = doJSON(router, http.MethodPut, membersURL, cookieA, `{"members":[]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT pelo ws owner: %d %s", rec.Code, rec.Body.String())
	}
	if got := membersInDB(t, pool, p.ID); len(got) != 0 {
		t.Errorf("lista vazia deveria zerar os membros, banco tem %+v", got)
	}

	// access fora do enum → 400 validation_error
	rec = doJSON(router, http.MethodPut, membersURL, cookieM,
		`{"members":[{"user_id":"`+idM2+`","access":"admin"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("access inválido: esperado 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}

	// PATCH com members em projeto shared também substitui (CA-13 contempla
	// members no PATCH; em shared a semântica é a do PUT)
	rec = doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookieM,
		`{"members":[{"user_id":"`+idM2+`","access":"edit"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH com members: %d %s", rec.Code, rec.Body.String())
	}
	got = membersInDB(t, pool, p.ID)
	if len(got) != 1 || got[0].UserID != idM2 || got[0].Access != "edit" {
		t.Errorf("PATCH com members deveria substituir a lista, banco tem %+v", got)
	}
}

// CA-11.1 — POST de projeto shared com members persiste na MESMA transação
// e devolve os members no 201.
func TestCriarProjetoComMembros(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM2 := userIDOf(t, pool, "g-m2")
	addMember(t, pool, ws, idM2, "member")

	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/projects", cookieA,
		`{"name":"Com squad","color":"#4f9dde","visibility":"shared",
		  "members":[{"user_id":"`+idM2+`","access":"edit"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST com members: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		projectJSON
		Members []memberJSON `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Members) != 1 || created.Members[0].UserID != idM2 || created.Members[0].Access != "edit" {
		t.Errorf("201 deveria devolver os members persistidos, veio %+v", created.Members)
	}
	if got := membersInDB(t, pool, created.ID); len(got) != 1 {
		t.Errorf("members deveriam estar persistidos, banco tem %+v", got)
	}
}

// CA-12 — user_id fora do workspace: NADA persiste (nem o projeto no POST,
// nem mudança na lista no PUT) e a resposta é 400 invalid_reference.
func TestMembroForaDoWorkspace400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-x", "x@example.com", "Xeno Estranho") // tem ws próprio, NÃO é membro do wsA
	loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idX := userIDOf(t, pool, "g-x")
	idM2 := userIDOf(t, pool, "g-m2")
	addMember(t, pool, ws, idM2, "member")

	antes := dbtest.CountRows(t, pool, "projects")
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/projects", cookieA,
		`{"name":"Vazado","color":"#4f9dde","visibility":"shared",
		  "members":[{"user_id":"`+idX+`","access":"edit"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Fatalf("POST com estranho: esperado 400 invalid_reference, veio %d: %s", rec.Code, rec.Body.String())
	}
	if dbtest.CountRows(t, pool, "projects") != antes {
		t.Error("POST inválido NÃO deveria ter criado o projeto (tudo ou nada)")
	}

	p := criarProjeto(t, router, ws, cookieA, `{"name":"Squad","color":"#4f9dde","visibility":"shared"}`)
	seedProjectMember(t, pool, p.ID, idM2, "view")

	rec = doJSON(router, http.MethodPut, "/api/v1/workspaces/"+ws+"/projects/"+p.ID+"/members", cookieA,
		`{"members":[{"user_id":"`+idX+`","access":"edit"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_reference" {
		t.Fatalf("PUT com estranho: esperado 400 invalid_reference, veio %d: %s", rec.Code, rec.Body.String())
	}
	got := membersInDB(t, pool, p.ID)
	if len(got) != 1 || got[0].UserID != idM2 || got[0].Access != "view" {
		t.Errorf("PUT inválido NÃO deveria ter mexido na lista, banco tem %+v", got)
	}
}

// CA-13 — projeto private (ou virando private) com lista de members não
// vazia: 400 validation_error no POST, no PATCH e no PUT.
func TestPrivadoComMembros400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM2 := userIDOf(t, pool, "g-m2")
	addMember(t, pool, ws, idM2, "member")
	base := "/api/v1/workspaces/" + ws + "/projects"

	// POST private + members → 400, nada persiste
	antes := dbtest.CountRows(t, pool, "projects")
	rec := doJSON(router, http.MethodPost, base, cookieA,
		`{"name":"Privado","color":"#4f9dde","visibility":"private",
		  "members":[{"user_id":"`+idM2+`","access":"edit"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Fatalf("POST private+members: esperado 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
	if dbtest.CountRows(t, pool, "projects") != antes {
		t.Error("POST inválido não deveria criar o projeto")
	}

	// PATCH virando private + members não vazia → 400 e projeto intacto
	p := criarProjeto(t, router, ws, cookieA, `{"name":"Squad","color":"#4f9dde","visibility":"shared"}`)
	rec = doJSON(router, http.MethodPatch, base+"/"+p.ID, cookieA,
		`{"visibility":"private","members":[{"user_id":"`+idM2+`","access":"edit"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Fatalf("PATCH→private+members: esperado 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
	var vis string
	if err := pool.QueryRow(context.Background(),
		`SELECT visibility FROM projects WHERE id = $1`, p.ID).Scan(&vis); err != nil || vis != "shared" {
		t.Errorf("PATCH inválido não deveria ter aplicado nada (visibility=%s, err=%v)", vis, err)
	}

	// PUT members não vazia em projeto private → 400; lista vazia → 200
	priv := criarProjeto(t, router, ws, cookieA, `{"name":"Só meu","color":"#4f9dde","visibility":"private"}`)
	rec = doJSON(router, http.MethodPut, base+"/"+priv.ID+"/members", cookieA,
		`{"members":[{"user_id":"`+idM2+`","access":"view"}]}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Fatalf("PUT em private: esperado 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodPut, base+"/"+priv.ID+"/members", cookieA, `{"members":[]}`)
	if rec.Code != http.StatusOK {
		t.Errorf("PUT lista vazia em private deveria ser 200 (no-op), veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-13.1 — PATCH que muda visibility pra private SEM campo members
// remove todas as linhas de project_members na mesma transação.
func TestPatchPrivateRemoveMembros(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-m2", "m2@example.com", "Max Member")
	loginVia(t, svc, stub, "g-m3", "m3@example.com", "Mel Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM2 := userIDOf(t, pool, "g-m2")
	idM3 := userIDOf(t, pool, "g-m3")
	addMember(t, pool, ws, idM2, "member")
	addMember(t, pool, ws, idM3, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"Squad","color":"#4f9dde","visibility":"shared"}`)
	seedProjectMember(t, pool, p.ID, idM2, "edit")
	seedProjectMember(t, pool, p.ID, idM3, "view")

	rec := doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/projects/"+p.ID, cookieA,
		`{"visibility":"private"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH→private: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if got := membersInDB(t, pool, p.ID); len(got) != 0 {
		t.Errorf("private nunca mantém membros órfãos — banco tem %+v", got)
	}
}
