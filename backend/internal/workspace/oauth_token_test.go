package workspace_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/workspace"
)

const calScope = "https://www.googleapis.com/auth/calendar.readonly"

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// upsertScopes cria/atualiza o user g-tok com os scopes dados (refresh fixo
// "ref", access "acc-<último scope>"), exercitando o caminho real do login.
func upsertScopes(t *testing.T, store *workspace.Store, scopes []string) string {
	t.Helper()
	id, err := store.UpsertUserFromGoogle(context.Background(),
		workspace.GoogleProfile{GoogleID: "g-tok", Email: "tok@example.com", Name: "Tok"},
		workspace.EncryptedToken{
			AccessTokenEnc:  []byte("acc-" + scopes[len(scopes)-1]),
			RefreshTokenEnc: []byte("ref"),
			ExpiresAt:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
			Scopes:          scopes,
		})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return id
}

func TestLoadGoogleToken(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := workspace.NewStore(pool)

	if _, ok, err := store.LoadGoogleToken(context.Background(),
		"00000000-0000-0000-0000-000000000000"); err != nil || ok {
		t.Fatalf("token inexistente devia dar ok=false sem erro (ok=%v err=%v)", ok, err)
	}

	id := upsertScopes(t, store, []string{"openid", "email"})
	st, ok, err := store.LoadGoogleToken(context.Background(), id)
	if err != nil || !ok {
		t.Fatalf("LoadGoogleToken: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(st.AccessTokenEnc, []byte("acc-email")) {
		t.Errorf("access blob inesperado: %q", st.AccessTokenEnc)
	}
	if !bytes.Equal(st.RefreshTokenEnc, []byte("ref")) {
		t.Errorf("refresh blob inesperado: %q", st.RefreshTokenEnc)
	}
	if len(st.Scopes) != 2 {
		t.Errorf("scopes inesperados: %v", st.Scopes)
	}
}

func TestSaveGoogleAccessTokenNaoTocaRefreshNemScopes(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := workspace.NewStore(pool)
	id := upsertScopes(t, store, []string{"openid", calScope})

	novoExpiry := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := store.SaveGoogleAccessToken(context.Background(), id, []byte("novo-access"), novoExpiry); err != nil {
		t.Fatalf("SaveGoogleAccessToken: %v", err)
	}

	st, _, _ := store.LoadGoogleToken(context.Background(), id)
	if !bytes.Equal(st.AccessTokenEnc, []byte("novo-access")) {
		t.Errorf("access não foi atualizado: %q", st.AccessTokenEnc)
	}
	if !bytes.Equal(st.RefreshTokenEnc, []byte("ref")) {
		t.Errorf("SaveGoogleAccessToken tocou o refresh: %q", st.RefreshTokenEnc)
	}
	if !hasScope(st.Scopes, calScope) {
		t.Errorf("SaveGoogleAccessToken apagou scopes: %v", st.Scopes)
	}
}

// CA-03.2: conectar o calendar e depois logar normal NÃO pode derrubar o
// calendar.readonly — o upsert faz merge (união) dos scopes.
func TestUpsertMergeDeScopes(t *testing.T) {
	pool := dbtest.NewPool(t)
	store := workspace.NewStore(pool)

	upsertScopes(t, store, []string{"openid", "email", "profile"})      // login base
	id := upsertScopes(t, store, []string{"openid", "email", calScope}) // connect calendar
	upsertScopes(t, store, []string{"openid", "email", "profile"})      // login normal depois

	st, _, _ := store.LoadGoogleToken(context.Background(), id)
	if !hasScope(st.Scopes, calScope) {
		t.Fatalf("re-login apagou o calendar.readonly: %v", st.Scopes)
	}
}
