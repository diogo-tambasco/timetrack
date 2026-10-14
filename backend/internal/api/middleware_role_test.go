package api_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/workspace"
)

// newGateRouter monta a MESMA cadeia do subrouter de workspace do
// auth_routes.go (RequireAuth → RequireMember → RequireWriteRole) com
// rotas-sonda — as rotas reais chegam nas T-04+.
func newGateRouter(svc interface {
	RequireAuth(http.Handler) http.Handler
}, store *workspace.Store) http.Handler {
	probe := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(svc.RequireAuth)
		v1.Route("/workspaces/{workspaceId}", func(wr chi.Router) {
			wr.Use(store.RequireMember)
			wr.Use(api.RequireWriteRole)
			wr.Get("/probe", probe)
			wr.Post("/probe", probe)
			wr.Patch("/probe", probe)
			wr.Put("/probe", probe)
			wr.Delete("/probe", probe)
		})
	})
	return r
}

// CA-23 — viewer em qualquer método de escrita recebe 403 forbidden; o
// gate não barra owner/member.
func TestViewerEscritaProibida(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := newAuthService(t, pool)
	store := workspace.NewStore(pool)
	router := newGateRouter(svc, store)

	cookieA := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	cookieB := loginVia(t, svc, stub, "g-viewer", "viewer@example.com", "Beto Viewer")
	wsA := workspaceIDOf(t, pool, "g-owner")
	addMember(t, pool, wsA, userIDOf(t, pool, "g-viewer"), "viewer")

	base := "/api/v1/workspaces/" + wsA + "/probe"
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		rec := doJSON(router, method, base, cookieB, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s como viewer: esperado 403, veio %d: %s", method, rec.Code, rec.Body.String())
			continue
		}
		if code := errCode(t, rec); code != "forbidden" {
			t.Errorf("%s como viewer: error.code esperado forbidden, veio %q", method, code)
		}
	}

	// owner não é barrado pelo gate
	if rec := doJSON(router, http.MethodPost, base, cookieA, ""); rec.Code != http.StatusOK {
		t.Errorf("POST como owner: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-24 — viewer autenticado consegue GET (200) atrás do gate.
func TestViewerLeituraPermitida(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := newAuthService(t, pool)
	store := workspace.NewStore(pool)
	router := newGateRouter(svc, store)

	loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	cookieB := loginVia(t, svc, stub, "g-viewer", "viewer@example.com", "Beto Viewer")
	wsA := workspaceIDOf(t, pool, "g-owner")
	addMember(t, pool, wsA, userIDOf(t, pool, "g-viewer"), "viewer")

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+wsA+"/probe", cookieB, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET como viewer: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
}

// newOwnerGateRouter monta a cadeia com o RequireOwner APÓS o RequireWriteRole
// (fase 17 T-04): owner passa, member e viewer recebem 403 mesmo no GET.
func newOwnerGateRouter(svc interface {
	RequireAuth(http.Handler) http.Handler
}, store *workspace.Store) http.Handler {
	probe := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(svc.RequireAuth)
		v1.Route("/workspaces/{workspaceId}", func(wr chi.Router) {
			wr.Use(store.RequireMember)
			wr.Use(api.RequireWriteRole)
			wr.Group(func(o chi.Router) {
				o.Use(api.RequireOwner)
				o.Get("/probe", probe)
				o.Post("/probe", probe)
			})
		})
	})
	return r
}

// CA-21 (fase 17) — gestão é Owner-only: owner passa; member e viewer recebem
// 403 forbidden (inclusive no GET, que o RequireWriteRole não barra).
func TestRequireOwnerGate(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := newAuthService(t, pool)
	store := workspace.NewStore(pool)
	router := newOwnerGateRouter(svc, store)

	cookieOwner := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	cookieMember := loginVia(t, svc, stub, "g-member", "member@example.com", "Beto Member")
	cookieViewer := loginVia(t, svc, stub, "g-viewer", "viewer@example.com", "Caio Viewer")
	wsA := workspaceIDOf(t, pool, "g-owner")
	addMember(t, pool, wsA, userIDOf(t, pool, "g-member"), "member")
	addMember(t, pool, wsA, userIDOf(t, pool, "g-viewer"), "viewer")

	base := "/api/v1/workspaces/" + wsA + "/probe"

	// owner passa (GET e POST).
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := doJSON(router, method, base, cookieOwner, ""); rec.Code != http.StatusOK {
			t.Errorf("%s como owner: esperado 200, veio %d: %s", method, rec.Code, rec.Body.String())
		}
	}

	// member e viewer → 403 forbidden em TODAS as rotas de gestão (incl. GET).
	for _, c := range []struct {
		who    string
		cookie *http.Cookie
	}{{"member", cookieMember}, {"viewer", cookieViewer}} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			rec := doJSON(router, method, base, c.cookie, "")
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s como %s: esperado 403, veio %d: %s", method, c.who, rec.Code, rec.Body.String())
				continue
			}
			if code := errCode(t, rec); code != "forbidden" {
				t.Errorf("%s como %s: error.code esperado forbidden, veio %q", method, c.who, code)
			}
		}
	}
}

// CA-25 — verificação de WIRING (regressão, nasce verde por reuso do
// RequireAuth da fase 01 — exceção documentada no tasks.md): sem cookie
// de sessão, a cadeia responde 401 antes de qualquer handler da fase.
func TestSemSessao401(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := newAuthService(t, pool)
	store := workspace.NewStore(pool)
	router := newGateRouter(svc, store)

	rec := doJSON(router, http.MethodGet,
		"/api/v1/workspaces/00000000-0000-0000-0000-000000000000/probe", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão: esperado 401, veio %d: %s", rec.Code, rec.Body.String())
	}
	if code := errCode(t, rec); code != "unauthorized" {
		t.Errorf("error.code esperado unauthorized, veio %q", code)
	}
}
