package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

// seedUser insere um user direto no banco e devolve o id.
func seedUser(t *testing.T, pool *pgxpool.Pool, googleID, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO users (google_id, email, name) VALUES ($1, $2, 'Teste')
		RETURNING id`, googleID, email).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

// apiProbe monta /api/v1/probe atrás do RequireAuth e devolve um flag de
// execução do handler.
func apiProbe(svc *Service) (http.Handler, *bool) {
	executed := false
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(svc.RequireAuth)
		r.Get("/probe", func(w http.ResponseWriter, _ *http.Request) {
			executed = true
			w.WriteHeader(http.StatusOK)
		})
	})
	return r, &executed
}

// CA-11: sem cookie, ou com token inexistente → 401 unauthorized e o
// handler protegido NÃO executa.
func TestApiNegaSemSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	router, executed := apiProbe(svc)

	cases := []struct {
		name   string
		cookie *http.Cookie
	}{
		{"sem cookie", nil},
		{"token inexistente", &http.Cookie{Name: "tt_session", Value: "token-que-nao-existe"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
			if c.cookie != nil {
				req.AddCookie(c.cookie)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("esperado 401, veio %d", rec.Code)
			}
			if code := decodeErrorCode(t, rec); code != "unauthorized" {
				t.Errorf("error.code esperado unauthorized, veio %q", code)
			}
			if *executed {
				t.Error("handler protegido executou sem sessão válida")
			}
		})
	}
}

// CA-12: sessão expirada → 401, linha apagada de sessions e cookie zerado.
func TestSessaoExpirada(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	router, executed := apiProbe(svc)
	ctx := context.Background()

	userID := seedUser(t, pool, "g-expirado", "exp@example.com")
	token, hash := NewSessionToken()
	if _, err := pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`, hash, userID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
	req.AddCookie(&http.Cookie{Name: "tt_session", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, veio %d", rec.Code)
	}
	if *executed {
		t.Error("handler protegido executou com sessão expirada")
	}
	if n := dbtest.CountRows(t, pool, "sessions"); n != 0 {
		t.Errorf("sessão expirada deveria ter sido apagada, sobraram %d", n)
	}
	cleared := cookieByName(t, rec, "tt_session")
	if cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("esperado Set-Cookie zerando tt_session (Max-Age=0), veio %+v", cleared)
	}
}
