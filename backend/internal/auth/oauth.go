// oauth.go — fluxo OAuth do Google: URL de autorização com escopos
// extensíveis (D-8) e endpoints injetáveis pra stub em teste (D-1).
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"timetrack.live/backend/internal/workspace"
)

// Escopos base da fase 01; Calendar entra por auth incremental na fase 06.
var baseScopes = []string{"openid", "email", "profile"}

const stateCookieName = "tt_oauth_state"

// Config do serviço de auth. Endpoint/UserinfoURL são injetáveis pra teste
// (TDD.md §3: Google sempre stubbado).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CookieSecure bool
	TokenKey     []byte          // 32 bytes — AES-256-GCM (D-5)
	Endpoint     oauth2.Endpoint // vazio → Google real
	UserinfoURL  string          // vazio → userinfo real do Google
	DB           *pgxpool.Pool   // users/workspaces/sessões (T-07/T-08)

	// ResolveInvites (fase 17, D-05) é chamado pelo HandleCallback LOGO APÓS
	// UpsertUserFromGoogle retornar (já com ui.Email + userID), pra materializar
	// convites pendentes no 1º login. Default no-op se nil. O cmd/server injeta
	// invitations.Service.ResolveForEmail. NÃO passa pelo store (NewStore fica
	// byte-idêntico — evita os 13 call sites; CA-27/34).
	ResolveInvites func(ctx context.Context, userID, email string) error
}

type Service struct {
	cfg   Config
	store *workspace.Store
}

func NewService(cfg Config) *Service {
	if cfg.Endpoint.AuthURL == "" {
		cfg.Endpoint = endpoints.Google
	}
	if cfg.UserinfoURL == "" {
		cfg.UserinfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	svc := &Service{cfg: cfg}
	if cfg.DB != nil {
		svc.store = workspace.NewStore(cfg.DB)
	}
	return svc
}

func (s *Service) oauth2Config(extraScopes ...string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.ClientID,
		ClientSecret: s.cfg.ClientSecret,
		RedirectURL:  s.cfg.RedirectURL,
		Scopes:       append(append([]string{}, baseScopes...), extraScopes...),
		Endpoint:     s.cfg.Endpoint,
	}
}

// BuildAuthURL monta a URL de autorização (CA-01). Escopos extras entram
// sem mudar a assinatura (CA-07 — auth incremental da fase 06). NÃO força
// prompt=consent — o login normal não deve re-consentir a cada acesso
// (fase 06 CA-01.1).
func (s *Service) BuildAuthURL(state string, extraScopes ...string) string {
	return s.oauth2Config(extraScopes...).AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
}

// BuildConsentURL é o BuildAuthURL do CONNECT (fase 06): força
// prompt=consent pra garantir o refresh token no re-consentimento (sem ele
// o Google pode não devolver refresh e o sync do calendar quebra em ~1h).
func (s *Service) BuildConsentURL(state string, extraScopes ...string) string {
	return s.oauth2Config(extraScopes...).AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// Exchange troca o authorization code por tokens no endpoint configurado
// (Google real em prod, stub em teste — D-1).
func (s *Service) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return s.oauth2Config().Exchange(ctx, code)
}

// Userinfo é o perfil mínimo que o callback precisa (CA-02).
type Userinfo struct {
	GoogleID  string
	Email     string
	Name      string
	AvatarURL string
}

// Userinfo busca o perfil OpenID Connect com o access token do exchange.
func (s *Service) Userinfo(ctx context.Context, tok *oauth2.Token) (Userinfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.UserinfoURL, nil)
	if err != nil {
		return Userinfo{}, err
	}
	resp, err := s.oauth2Config().Client(ctx, tok).Do(req)
	if err != nil {
		return Userinfo{}, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Userinfo{}, fmt.Errorf("userinfo respondeu %d", resp.StatusCode)
	}

	var raw struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Userinfo{}, fmt.Errorf("userinfo: corpo inválido: %w", err)
	}
	return Userinfo{
		GoogleID:  raw.Sub,
		Email:     raw.Email,
		Name:      raw.Name,
		AvatarURL: raw.Picture,
	}, nil
}

// HandleLogin — GET /auth/google/login: gera o state anti-CSRF (D-4),
// grava em cookie temporário e redireciona pro Google.
func (s *Service) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, _ := NewSessionToken() // 32 bytes de crypto/rand ≥ 128 bits

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
	http.Redirect(w, r, s.BuildAuthURL(state), http.StatusFound)
}
