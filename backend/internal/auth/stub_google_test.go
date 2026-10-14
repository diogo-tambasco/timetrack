package auth

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth/googlestub"
)

// fixturesDir é o caminho das fixtures do Google a partir deste pacote.
const fixturesDir = "../../testdata/google"

// stubService sobe o Google falso e devolve um Service apontando pra ele.
// Helper compartilhado pelos testes de callback (T-06..T-08).
func stubService(t *testing.T, cookieSecure bool) (*Service, *googlestub.Server) {
	t.Helper()
	stub := googlestub.New(fixturesDir)
	t.Cleanup(stub.Close)

	svc := NewService(Config{
		ClientID:     "client-id-teste",
		ClientSecret: "client-secret-teste",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		CookieSecure: cookieSecure,
		Endpoint:     stub.Endpoint(),
		UserinfoURL:  stub.UserinfoURL(),
	})
	return svc, stub
}

// testTokenKey: chave AES-256 determinística dos testes (32 bytes).
var testTokenKey = []byte("0123456789abcdef0123456789abcdef")

// stubServiceComDB é o stubService com Postgres de teste plugado — usado
// pelos testes de callback que assertam efeitos (ou a ausência deles) no banco.
func stubServiceComDB(t *testing.T, pool *pgxpool.Pool, cookieSecure bool) (*Service, *googlestub.Server) {
	t.Helper()
	stub := googlestub.New(fixturesDir)
	t.Cleanup(stub.Close)

	svc := NewService(Config{
		ClientID:     "client-id-teste",
		ClientSecret: "client-secret-teste",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		CookieSecure: cookieSecure,
		TokenKey:     testTokenKey,
		Endpoint:     stub.Endpoint(),
		UserinfoURL:  stub.UserinfoURL(),
		DB:           pool,
	})
	return svc, stub
}

// T-05: o exchange contra o stub devolve access+refresh token e o userinfo
// devolve google_id/email/name/avatar das fixtures (TDD.md §3 — Google
// sempre stubbado). Infra dos testes de T-06..T-08.
func TestStubGoogle(t *testing.T) {
	svc, _ := stubService(t, false)
	ctx := context.Background()

	// exchange feliz: fixture token.json
	tok, err := svc.Exchange(ctx, googlestub.CodeOK)
	if err != nil {
		t.Fatalf("exchange contra o stub falhou: %v", err)
	}
	if tok.AccessToken != "ya29.fixture-access-token" {
		t.Errorf("access_token esperado da fixture, veio %q", tok.AccessToken)
	}
	if tok.RefreshToken != "1//fixture-refresh-token" {
		t.Errorf("refresh_token esperado da fixture, veio %q", tok.RefreshToken)
	}

	// login repetido: fixture sem refresh_token (CA-14.1 usa esse caminho)
	tokNoRefresh, err := svc.Exchange(ctx, googlestub.CodeNoRefresh)
	if err != nil {
		t.Fatalf("exchange no-refresh falhou: %v", err)
	}
	if tokNoRefresh.RefreshToken != "" {
		t.Errorf("fixture sem refresh_token deveria vir vazia, veio %q", tokNoRefresh.RefreshToken)
	}

	// code inválido: fixture de erro vira erro do exchange
	if _, err := svc.Exchange(ctx, "code-invalido"); err == nil {
		t.Error("exchange com code inválido deveria falhar (token_error.json)")
	}

	// userinfo com o token do exchange devolve o perfil das fixtures
	ui, err := svc.Userinfo(ctx, tok)
	if err != nil {
		t.Fatalf("userinfo contra o stub falhou: %v", err)
	}
	if ui.GoogleID != "g-fixture-110248495921238986420" {
		t.Errorf("google_id esperado da fixture, veio %q", ui.GoogleID)
	}
	if ui.Email != "diogo.fixture@example.com" {
		t.Errorf("email esperado da fixture, veio %q", ui.Email)
	}
	if ui.Name != "Diogo Fixture" {
		t.Errorf("name esperado da fixture, veio %q", ui.Name)
	}
	if ui.AvatarURL != "https://lh3.googleusercontent.com/a/fixture-avatar" {
		t.Errorf("avatar_url esperado da fixture, veio %q", ui.AvatarURL)
	}
}
