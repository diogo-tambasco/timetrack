package googlestub_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"timetrack.live/backend/internal/auth/googlestub"
)

const fixtures = "../../../testdata/google"

// O connect da fase 06 pede o escopo de calendar; o stub TEM que diferenciar
// (senão o E2E concederia o token base e mascararia o caminho de calendar).
func TestAuthorizeDiferenciaScopeCalendar(t *testing.T) {
	srv := googlestub.New(fixtures)
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // não seguir o redirect
	}}

	codeFor := func(scope string) string {
		u := srv.URL() + "/authorize?redirect_uri=" + url.QueryEscape("http://app/cb") +
			"&state=s&scope=" + url.QueryEscape(scope)
		resp, err := client.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		loc, _ := url.Parse(resp.Header.Get("Location"))
		return loc.Query().Get("code")
	}

	if got := codeFor("openid email profile"); got != googlestub.CodeOK {
		t.Errorf("sem calendar: code esperado %q, veio %q", googlestub.CodeOK, got)
	}
	if got := codeFor("openid email https://www.googleapis.com/auth/calendar.readonly"); got != googlestub.CodeCalendar {
		t.Errorf("com calendar: code esperado %q, veio %q", googlestub.CodeCalendar, got)
	}
}

// O code-calendar tem que devolver um token COM o scope de calendar E refresh.
func TestTokenCalendarTemScopeERefresh(t *testing.T) {
	srv := googlestub.New(fixtures)
	defer srv.Close()

	resp, err := http.PostForm(srv.URL()+"/token", url.Values{"code": {googlestub.CodeCalendar}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token code-calendar status %d", resp.StatusCode)
	}
	var tok struct {
		Scope        string `json:"scope"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tok.Scope, "calendar.readonly") {
		t.Errorf("token de calendar sem o scope: %q", tok.Scope)
	}
	if tok.RefreshToken == "" {
		t.Error("token de calendar precisa de refresh_token")
	}
}
