package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
)

// loginAndCookie completa o login stubbado e devolve o cookie de sessão.
func loginAndCookie(t *testing.T, svc *Service) *http.Cookie {
	t.Helper()
	rec := callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=ok", "ok")
	c := cookieByName(t, rec, "tt_session")
	if c == nil || c.Value == "" {
		t.Fatalf("login stubbado não devolveu cookie de sessão (status %d)", rec.Code)
	}
	return c
}

// apiRouter monta as rotas autenticadas da fase no padrão do app real.
func apiRouter(svc *Service) chi.Router {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(svc.RequireAuth)
		r.Get("/me", svc.HandleMe)
	})
	return r
}

// CA-10: com sessão válida, GET /api/v1/me → 200 com o user autenticado e
// seus workspaces com a respectiva role.
func TestMeComSessaoValida(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	router := apiRouter(svc)

	cookie := loginAndCookie(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		User struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		Workspaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"workspaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("corpo fora do contrato §3.3: %v", err)
	}

	if body.User.ID == "" {
		t.Error("user.id vazio")
	}
	if body.User.Email != "diogo.fixture@example.com" {
		t.Errorf("email esperado da fixture, veio %q", body.User.Email)
	}
	if body.User.Name != "Diogo Fixture" {
		t.Errorf("name esperado da fixture, veio %q", body.User.Name)
	}
	if body.User.AvatarURL != "https://lh3.googleusercontent.com/a/fixture-avatar" {
		t.Errorf("avatar_url esperado da fixture, veio %q", body.User.AvatarURL)
	}

	if len(body.Workspaces) != 1 {
		t.Fatalf("esperado 1 workspace, veio %d", len(body.Workspaces))
	}
	ws := body.Workspaces[0]
	if ws.ID == "" || ws.Name != "Workspace de Diogo" || ws.Role != "owner" {
		t.Errorf("workspace fora do contrato: %+v", ws)
	}
}
