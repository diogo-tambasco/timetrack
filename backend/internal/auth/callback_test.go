package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
)

// callbackRequest monta um GET /auth/google/callback com query e cookie de
// state opcionais e devolve o recorder após o handler rodar.
func callbackRequest(t *testing.T, svc *Service, query, stateCookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?"+query, nil)
	if stateCookie != "" {
		req.AddCookie(&http.Cookie{Name: stateCookieName, Value: stateCookie})
	}
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)
	return rec
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("corpo de erro não é o envelope canônico: %v", err)
	}
	return body.Error.Code
}

func assertSemEfeitosNoBanco(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"users", "sessions"} {
		if n := dbtest.CountRows(t, pool, table); n != 0 {
			t.Errorf("%s deveria ter 0 linhas após callback falho, tem %d", table, n)
		}
	}
}

// CA-03: state divergente/ausente → 400 invalid_state, zero efeitos no banco.
func TestCallbackStateInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)

	cases := []struct {
		name        string
		query       string
		stateCookie string
	}{
		{"state divergente do cookie", "code=" + googlestub.CodeOK + "&state=state-da-query", "state-do-cookie"},
		{"cookie de state ausente", "code=" + googlestub.CodeOK + "&state=state-da-query", ""},
		{"state ausente na query", "code=" + googlestub.CodeOK, "state-do-cookie"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := callbackRequest(t, svc, c.query, c.stateCookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("esperado 400, veio %d", rec.Code)
			}
			if code := decodeErrorCode(t, rec); code != "invalid_state" {
				t.Errorf("error.code esperado invalid_state, veio %q", code)
			}
		})
	}
	assertSemEfeitosNoBanco(t, pool)
}

// CA-06: Google retornou erro (ex.: access_denied) → 302 /login?error=google,
// zero efeitos no banco.
func TestCallbackErroGoogle(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)

	rec := callbackRequest(t, svc, "error=access_denied", "qualquer")
	if rec.Code != http.StatusFound {
		t.Fatalf("esperado 302, veio %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?error=google" {
		t.Errorf("Location esperada /login?error=google, veio %q", loc)
	}
	assertSemEfeitosNoBanco(t, pool)
}
