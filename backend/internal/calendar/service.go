// Package calendar — fase 06 (LEITURA): lê a agenda principal do Google por
// usuário. Token cifrado carregado de oauth_tokens (fase 01), renovado e
// re-persistido transparentemente (CA-09). NENHUMA tabela nova (RNF-03);
// nada de write-back (fora de escopo).
package calendar

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/workspace"
)

// ReadonlyScope — escopo OAuth de leitura da agenda (D-06).
const ReadonlyScope = "https://www.googleapis.com/auth/calendar.readonly"

// defaultAPIBase — base da Calendar API v3 (vazio no NewService → este).
const defaultAPIBase = "https://www.googleapis.com/calendar/v3"

// Service lê a agenda principal do Google por usuário.
type Service struct {
	db       *pgxpool.Pool // fase 11: persistência das preferências de calendário (settings.go)
	store    *workspace.Store
	oauthCfg oauth2.Config // ClientID/Secret/Endpoint.TokenURL — pro refresh
	tokenKey []byte        // AES-256-GCM (mesma chave da fase 01)
	apiBase  string
	httpc    *http.Client
}

// Option ajusta o Service.
type Option func(*Service)

// WithHTTPClient injeta o cliente HTTP (testes/timeout custom).
func WithHTTPClient(c *http.Client) Option {
	return func(s *Service) { s.httpc = c }
}

// NewService monta o serviço. apiBase vazio → Calendar API real.
func NewService(db *pgxpool.Pool, oauthCfg oauth2.Config, tokenKey []byte, apiBase string, opts ...Option) *Service {
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	s := &Service{
		db:       db,
		store:    workspace.NewStore(db),
		oauthCfg: oauthCfg,
		tokenKey: tokenKey,
		apiBase:  apiBase,
		httpc:    &http.Client{Timeout: 15 * time.Second},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// hasReadonly — o token concede leitura de calendar?
func hasReadonly(st workspace.StoredToken) bool {
	for _, sc := range st.Scopes {
		if sc == ReadonlyScope {
			return true
		}
	}
	return false
}

// Status (CA-04): o usuário conectou o Google Calendar (tem o scope de leitura)?
func (s *Service) Status(ctx context.Context, userID string) (bool, error) {
	st, ok, err := s.store.LoadGoogleToken(ctx, userID)
	if err != nil {
		return false, err
	}
	return ok && hasReadonly(st), nil
}

// Events (CA-05..CA-12): eventos da agenda primary na janela [from, to).
// Renova o access expirado e re-persiste (CA-09); refresh revogado →
// calendar_reconnect_required (CA-10).
func (s *Service) Events(ctx context.Context, userID string, from, to time.Time) ([]Event, error) {
	st, ok, err := s.store.LoadGoogleToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok || !hasReadonly(st) { // CA-06: nem chega no Google
		return nil, domain.Conflict("calendar_not_connected", "conecte o Google Calendar")
	}

	access, err := auth.Open(s.tokenKey, st.AccessTokenEnc)
	if err != nil {
		return nil, err // decifra falhou = token corrompido (interno/500)
	}
	var refresh string
	if len(st.RefreshTokenEnc) > 0 {
		r, err := auth.Open(s.tokenKey, st.RefreshTokenEnc)
		if err != nil {
			return nil, err
		}
		refresh = string(r)
	}

	original := &oauth2.Token{
		AccessToken:  string(access),
		RefreshToken: refresh,
		Expiry:       st.ExpiresAt,
	}
	tok, err := s.oauthCfg.TokenSource(ctx, original).Token()
	if err != nil { // refresh falhou (invalid_grant/revogado) — CA-10
		return nil, domain.Conflict("calendar_reconnect_required", "reconecte o Google Calendar")
	}
	if tok.AccessToken != original.AccessToken { // renovou → re-persiste SÓ o access (CA-09)
		if enc, e := auth.Seal(s.tokenKey, []byte(tok.AccessToken)); e == nil {
			_ = s.store.SaveGoogleAccessToken(ctx, userID, enc, tok.Expiry)
		}
	}

	return s.listPrimary(ctx, tok.AccessToken, from, to)
}
