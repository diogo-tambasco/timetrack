package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/db/dbtest"
)

// CA-13: logout apaga a sessão, zera o cookie e responde 204; o mesmo
// cookie depois disso leva 401 no /me.
func TestLogout(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(svc.RequireAuth)
		r.Get("/me", svc.HandleMe)
		r.Post("/auth/logout", svc.HandleLogout)
	})

	cookie := loginAndCookie(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}
	cleared := cookieByName(t, rec, "tt_session")
	if cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("esperado Set-Cookie zerando tt_session, veio %+v", cleared)
	}
	if n := dbtest.CountRows(t, pool, "sessions"); n != 0 {
		t.Errorf("sessão deveria ter sido apagada, sobraram %d", n)
	}

	// o mesmo cookie agora é inválido (revogação imediata)
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqMe.AddCookie(cookie)
	recMe := httptest.NewRecorder()
	r.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusUnauthorized {
		t.Errorf("GET /me após logout deveria dar 401, veio %d", recMe.Code)
	}
}
