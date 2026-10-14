package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testService(cookieSecure bool) *Service {
	return NewService(Config{
		ClientID:     "client-id-teste",
		ClientSecret: "client-secret-teste",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		CookieSecure: cookieSecure,
	})
}

// CA-01: login redireciona pro Google com client_id, redirect_uri, escopos
// base, state aleatório (≥128 bits) gravado em cookie temporário.
func TestLoginRedireciona(t *testing.T) {
	svc := testService(false)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()
	svc.HandleLogin(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("esperado 302, veio %d", rec.Code)
	}

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location inválida: %v", err)
	}
	q := loc.Query()
	if q.Get("client_id") != "client-id-teste" {
		t.Errorf("client_id errado: %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "http://localhost:8080/auth/google/callback" {
		t.Errorf("redirect_uri errada: %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type esperado code, veio %q", q.Get("response_type"))
	}
	if got := q.Get("scope"); got != "openid email profile" {
		t.Errorf("scope esperado 'openid email profile', veio %q", got)
	}
	if q.Get("access_type") != "offline" {
		t.Errorf("access_type esperado offline, veio %q", q.Get("access_type"))
	}
	if q.Get("include_granted_scopes") != "true" {
		t.Errorf("include_granted_scopes esperado true, veio %q", q.Get("include_granted_scopes"))
	}

	state := q.Get("state")
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil || len(raw) < 16 {
		t.Errorf("state deve ter ≥128 bits de entropia em base64url, veio %q (err=%v)", state, err)
	}

	// cookie temporário do state
	res := rec.Result()
	var stateCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "tt_oauth_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("cookie tt_oauth_state ausente")
	}
	if stateCookie.Value != state {
		t.Error("cookie de state deve guardar o mesmo valor enviado ao Google")
	}
	if !stateCookie.HttpOnly {
		t.Error("cookie de state deve ser HttpOnly")
	}
	if stateCookie.MaxAge != 600 {
		t.Errorf("cookie de state deve ter Max-Age=600, veio %d", stateCookie.MaxAge)
	}
	if stateCookie.SameSite != http.SameSiteLaxMode {
		t.Error("cookie de state deve ser SameSite=Lax")
	}
	if stateCookie.Secure {
		t.Error("sem COOKIE_SECURE, cookie não deve ser Secure")
	}

	// com COOKIE_SECURE=true o cookie sai Secure
	svcSec := testService(true)
	recSec := httptest.NewRecorder()
	svcSec.HandleLogin(recSec, httptest.NewRequest(http.MethodGet, "/auth/google/login", nil))
	for _, c := range recSec.Result().Cookies() {
		if c.Name == "tt_oauth_state" && !c.Secure {
			t.Error("com COOKIE_SECURE=true o cookie de state deve ser Secure")
		}
	}
}

// CA-07: escopo extra entra na URL sem mudar assinatura/contrato.
func TestBuildAuthURLEscopoExtra(t *testing.T) {
	svc := testService(false)

	authURL := svc.BuildAuthURL("state-x", "https://www.googleapis.com/auth/calendar")
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	scope := u.Query().Get("scope")
	if !strings.Contains(scope, "openid") || !strings.Contains(scope, "email") || !strings.Contains(scope, "profile") {
		t.Errorf("escopos base sumiram: %q", scope)
	}
	if !strings.Contains(scope, "https://www.googleapis.com/auth/calendar") {
		t.Errorf("escopo extra ausente: %q", scope)
	}
	if u.Query().Get("state") != "state-x" {
		t.Errorf("state errado: %q", u.Query().Get("state"))
	}
}
