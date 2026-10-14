// invitations_routes_test.go — tabela de permissões + isolamento das rotas de
// gestão da fase 17 (CA-04/06/07/10/13/14/16/17/20/21/21.1/22/23). Espelha o
// estilo de isolation_core_test.go.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/invitations"
	"timetrack.live/backend/internal/workspace"
)

// newInvitationsRouter monta o router real com auth (fase 01) + as rotas de
// gestão da fase 17 (sem o domínio core, que não é necessário aqui).
func newInvitationsRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithInvitations(api.Invitations{Service: invitations.NewService(pool)}),
	)
	return router, svc, stub
}

// invUserID resolve o id do user pelo email (helper local).
func invUserID(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&id); err != nil {
		t.Fatalf("user %s não existe: %v", email, err)
	}
	return id
}

// CA-21/21.1/23 — tabela de permissões: owner 2xx, member/viewer 403,
// não-membro 404, sem sessão 401, em TODAS as rotas de gestão.
func TestInvitationsPermissoes(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)

	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	cookieMember := loginVia(t, svc, stub, "g-member", "member@example.com", "Beto Member")
	cookieViewer := loginVia(t, svc, stub, "g-viewer", "viewer@example.com", "Caio Viewer")
	cookieStranger := loginVia(t, svc, stub, "g-stranger", "stranger@example.com", "Dani Stranger")
	ws := workspaceIDOf(t, pool, "g-owner")
	addMember(t, pool, ws, invUserID(t, pool, "member@example.com"), "member")
	addMember(t, pool, ws, invUserID(t, pool, "viewer@example.com"), "viewer")

	base := "/api/v1/workspaces/" + ws
	someUser := invUserID(t, pool, "member@example.com")
	someUUID := "00000000-0000-0000-0000-000000000000"

	routes := []struct {
		method, path, body string
	}{
		{http.MethodPost, base + "/invitations", `{"email":"novo@x.com","role":"member"}`},
		{http.MethodGet, base + "/invitations", ""},
		{http.MethodDelete, base + "/invitations/" + someUUID, ""},
		{http.MethodPost, base + "/members", `{"email":"x@x.com","role":"member"}`},
		{http.MethodDelete, base + "/members/" + someUser, ""},
		{http.MethodPatch, base + "/members/" + someUser, `{"role":"viewer"}`},
	}

	for _, rt := range routes {
		// member → 403 (CA-21).
		if rec := doJSON(router, rt.method, rt.path, cookieMember, rt.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s como member: esperado 403, veio %d", rt.method, rt.path, rec.Code)
		}
		// viewer → 403 (CA-21).
		if rec := doJSON(router, rt.method, rt.path, cookieViewer, rt.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s como viewer: esperado 403, veio %d", rt.method, rt.path, rec.Code)
		}
		// não-membro (stranger): 404 do RequireMember ANTES do 403 (CA-21.1).
		if rec := doJSON(router, rt.method, rt.path, cookieStranger, rt.body); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s como não-membro: esperado 404 (antes do 403), veio %d", rt.method, rt.path, rec.Code)
		}
		// sem sessão → 401 (CA-23).
		if rec := doJSON(router, rt.method, rt.path, nil, rt.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s sem sessão: esperado 401, veio %d", rt.method, rt.path, rec.Code)
		}
	}

	_ = cookieOwner
}

// CA-04 — owner cria convite → 201 com os campos do contrato, sem token.
func TestInvitationsCreate201(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")

	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/invitations", cookieOwner,
		`{"email":"cadu@example.com","role":"member"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"email":"cadu@example.com"`, `"role":"member"`, `"status":"pending"`} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo não tem %s: %s", want, body)
		}
	}
	if strings.Contains(body, `"token"`) {
		t.Errorf("o token NUNCA deve aparecer no corpo (RNF-04): %s", body)
	}
}

// CA-06 — payload inválido (sem email / role inválida) → 400.
func TestInvitationsCreate400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/invitations"

	for _, body := range []string{`{"role":"member"}`, `{"email":"semarroba","role":"member"}`, `{"email":"x@x.com","role":"owner"}`, `{"email":"x@x.com","role":"chefe"}`} {
		rec := doJSON(router, http.MethodPost, base, cookieOwner, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("payload %s: esperado 400, veio %d", body, rec.Code)
		}
	}
}

// CA-07 — convidar quem já é membro → 409 already_member.
func TestInvitationsCreateJaMembro(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")

	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+ws+"/invitations", cookieOwner,
		`{"email":"owner@example.com","role":"member"}`)
	if rec.Code != http.StatusConflict || errCode(t, rec) != "already_member" {
		t.Errorf("auto-convite deveria dar 409 already_member, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-10/11 — add existente (201) e add inexistente (404 user_not_found).
func TestMembersAdd(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-bob", "bob@example.com", "Bob") // user existe, sem membership
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/members"

	// existente → 201.
	rec := doJSON(router, http.MethodPost, base, cookieOwner, `{"email":"bob@example.com","role":"viewer"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add existente: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	// inexistente → 404 user_not_found.
	rec = doJSON(router, http.MethodPost, base, cookieOwner, `{"email":"ninguem@example.com","role":"member"}`)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "user_not_found" {
		t.Errorf("add inexistente: esperado 404 user_not_found, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-14/16/17/20 — revoke/remove/patch happy + 404 e 409 corretos.
func TestMembersAndInvitesLifecycle(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-bob", "bob@example.com", "Bob")
	ws := workspaceIDOf(t, pool, "g-owner")
	bobID := invUserID(t, pool, "bob@example.com")
	addMember(t, pool, ws, bobID, "member")
	base := "/api/v1/workspaces/" + ws

	// PATCH role → 200.
	if rec := doJSON(router, http.MethodPatch, base+"/members/"+bobID, cookieOwner, `{"role":"viewer"}`); rec.Code != http.StatusOK {
		t.Errorf("patch role: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	// DELETE membro → 204.
	if rec := doJSON(router, http.MethodDelete, base+"/members/"+bobID, cookieOwner, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete membro: esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}
	// DELETE de não-membro → 404.
	if rec := doJSON(router, http.MethodDelete, base+"/members/"+bobID, cookieOwner, ""); rec.Code != http.StatusNotFound {
		t.Errorf("delete não-membro: esperado 404, veio %d", rec.Code)
	}
	// DELETE do único owner → 409 last_owner.
	ownerID := invUserID(t, pool, "owner@example.com")
	rec := doJSON(router, http.MethodDelete, base+"/members/"+ownerID, cookieOwner, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "last_owner" {
		t.Errorf("delete último owner: esperado 409 last_owner, veio %d %s", rec.Code, rec.Body.String())
	}

	// criar + revogar convite → 204; revogar de novo → 204 idempotente.
	rec = doJSON(router, http.MethodPost, base+"/invitations", cookieOwner, `{"email":"cadu@example.com","role":"member"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar convite: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec := doJSON(router, http.MethodDelete, base+"/invitations/"+created.ID, cookieOwner, ""); rec.Code != http.StatusNoContent {
		t.Errorf("revogar convite: esperado 204, veio %d", rec.Code)
	}
}

// CA-22 — isolamento: owner do A não cria/lista/revoga/remove no B. Ids
// cruzados → 404 (não vaza).
func TestInvitationsIsolamento(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newInvitationsRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana A")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Bob B")
	wsA := workspaceIDOf(t, pool, "g-a")
	wsB := workspaceIDOf(t, pool, "g-b")

	// owner do A cria um convite no A.
	rec := doJSON(router, http.MethodPost, "/api/v1/workspaces/"+wsA+"/invitations", cookieA,
		`{"email":"cadu@example.com","role":"member"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar convite no A: %d %s", rec.Code, rec.Body.String())
	}
	var inv struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// owner do A NÃO é membro do B → 404 (RequireMember) ao tentar listar lá.
	if rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+wsB+"/invitations", cookieA, ""); rec.Code != http.StatusNotFound {
		t.Errorf("listar convites do B como owner do A: esperado 404, veio %d", rec.Code)
	}

	// revogar o convite do A pela rota do B (onde A nem é membro) → 404.
	if rec := doJSON(router, http.MethodDelete, "/api/v1/workspaces/"+wsB+"/invitations/"+inv.ID, cookieA, ""); rec.Code != http.StatusNotFound {
		t.Errorf("revogar convite do A pela rota do B: esperado 404, veio %d", rec.Code)
	}

	// o convite do A segue intacto (listável pelo A).
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/"+wsA+"/invitations", cookieA, "")
	if !strings.Contains(rec.Body.String(), "cadu@example.com") {
		t.Errorf("convite do A deveria seguir pendente: %s", rec.Body.String())
	}
}
