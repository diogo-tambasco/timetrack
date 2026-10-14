package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
)

func cookieByName(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// CA-02: callback com code+state válidos → 302 pra /, cookie tt_session
// presente, exatamente 1 sessão nova, state cookie zerado.
func TestCallbackSucesso(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)

	rec := callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=state-valido", "state-valido")

	if rec.Code != http.StatusFound {
		t.Fatalf("esperado 302, veio %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location esperada /, veio %q", loc)
	}

	session := cookieByName(t, rec, "tt_session")
	if session == nil || session.Value == "" {
		t.Fatal("Set-Cookie de tt_session ausente ou vazio")
	}
	if n := dbtest.CountRows(t, pool, "sessions"); n != 1 {
		t.Errorf("esperada exatamente 1 sessão, tem %d", n)
	}

	state := cookieByName(t, rec, "tt_oauth_state")
	if state == nil || state.Value != "" || state.MaxAge >= 0 {
		t.Errorf("cookie tt_oauth_state deveria ser zerado (Max-Age=0, valor vazio), veio %+v", state)
	}
}

// CA-08: o banco só guarda o hash SHA-256 do token de sessão — o valor do
// cookie nunca aparece em sessions.token_hash.
func TestSessaoSoGuardaHash(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	rec := callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=s1", "s1")
	session := cookieByName(t, rec, "tt_session")
	if session == nil {
		t.Fatal("cookie tt_session ausente")
	}

	var emClaro bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash = $1)`, session.Value).Scan(&emClaro); err != nil {
		t.Fatal(err)
	}
	if emClaro {
		t.Error("token de sessão em claro encontrado em sessions.token_hash")
	}

	var hashGravado bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash = $1)`, HashToken(session.Value)).Scan(&hashGravado); err != nil {
		t.Fatal(err)
	}
	if !hashGravado {
		t.Error("HashToken(cookie) deveria existir em sessions.token_hash")
	}
}

// CA-09: atributos do cookie de sessão + expires_at ≈ now+30d no banco.
func TestCookieDeSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	rec := callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=s1", "s1")
	c := cookieByName(t, rec, "tt_session")
	if c == nil {
		t.Fatal("cookie tt_session ausente")
	}
	if !c.HttpOnly {
		t.Error("cookie de sessão deve ser HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Error("cookie de sessão deve ser SameSite=Lax")
	}
	if c.Path != "/" {
		t.Errorf("cookie de sessão deve ter Path=/, veio %q", c.Path)
	}
	if c.MaxAge != 2592000 {
		t.Errorf("cookie de sessão deve ter Max-Age=2592000 (30d), veio %d", c.MaxAge)
	}
	if c.Secure {
		t.Error("sem COOKIE_SECURE, cookie não deve ser Secure")
	}

	var expiresAt time.Time
	if err := pool.QueryRow(ctx, `SELECT expires_at FROM sessions`).Scan(&expiresAt); err != nil {
		t.Fatal(err)
	}
	want := time.Now().Add(30 * 24 * time.Hour)
	if diff := expiresAt.Sub(want); diff < -5*time.Minute || diff > 5*time.Minute {
		t.Errorf("expires_at deveria ser ≈ now+30d, veio %v (desvio %v)", expiresAt, diff)
	}

	// com COOKIE_SECURE=true o cookie sai Secure
	svcSec, _ := stubServiceComDB(t, dbtest.NewPool(t), true)
	recSec := callbackRequest(t, svcSec, "code="+googlestub.CodeOK+"&state=s2", "s2")
	cSec := cookieByName(t, recSec, "tt_session")
	if cSec == nil || !cSec.Secure {
		t.Error("com COOKIE_SECURE=true o cookie de sessão deve ser Secure")
	}
}

// CA-14: tokens do Google cifrados com AES-256-GCM em repouso — o bytea
// difere do token em claro e Open devolve o original.
func TestTokensGoogleCifrados(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=s1", "s1")

	var accessEnc, refreshEnc []byte
	if err := pool.QueryRow(ctx,
		`SELECT access_token_enc, refresh_token_enc FROM oauth_tokens`).Scan(&accessEnc, &refreshEnc); err != nil {
		t.Fatalf("oauth_tokens deveria ter 1 linha após o login: %v", err)
	}

	if string(accessEnc) == "ya29.fixture-access-token" {
		t.Error("access_token gravado em claro")
	}
	if string(refreshEnc) == "1//fixture-refresh-token" {
		t.Error("refresh_token gravado em claro")
	}

	plainAccess, err := Open(testTokenKey, accessEnc)
	if err != nil || string(plainAccess) != "ya29.fixture-access-token" {
		t.Errorf("Open(access) deveria devolver o original, veio %q (err=%v)", plainAccess, err)
	}
	plainRefresh, err := Open(testTokenKey, refreshEnc)
	if err != nil || string(plainRefresh) != "1//fixture-refresh-token" {
		t.Errorf("Open(refresh) deveria devolver o original, veio %q (err=%v)", plainRefresh, err)
	}
}

// CA-14.1: login repetido SEM refresh_token na resposta do Google (caso
// normal) não sobrescreve nem apaga o refresh_token_enc existente.
func TestLoginRepetidoPreservaRefreshToken(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, _ := stubServiceComDB(t, pool, false)
	ctx := context.Background()

	// 1º login: fixture com refresh_token
	callbackRequest(t, svc, "code="+googlestub.CodeOK+"&state=s1", "s1")
	// 2º login: fixture SEM refresh_token (token_no_refresh.json)
	callbackRequest(t, svc, "code="+googlestub.CodeNoRefresh+"&state=s2", "s2")

	var accessEnc, refreshEnc []byte
	if err := pool.QueryRow(ctx,
		`SELECT access_token_enc, refresh_token_enc FROM oauth_tokens`).Scan(&accessEnc, &refreshEnc); err != nil {
		t.Fatal(err)
	}

	if refreshEnc == nil {
		t.Fatal("refresh_token_enc foi apagado pelo 2º login")
	}
	plainRefresh, err := Open(testTokenKey, refreshEnc)
	if err != nil || string(plainRefresh) != "1//fixture-refresh-token" {
		t.Errorf("refresh original deveria permanecer, veio %q (err=%v)", plainRefresh, err)
	}

	// e o access foi atualizado pro do 2º login (upsert não-destrutivo)
	plainAccess, err := Open(testTokenKey, accessEnc)
	if err != nil || string(plainAccess) != "ya29.fixture-access-token-relogin" {
		t.Errorf("access deveria ser o do 2º login, veio %q (err=%v)", plainAccess, err)
	}
}
