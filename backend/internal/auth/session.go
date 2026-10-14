// session.go — sessão opaca em Postgres (D-2/D-3): cookie httpOnly de 30
// dias com token aleatório; o banco só conhece o hash SHA-256.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	sessionCookieName = "tt_session"
	sessionTTL        = 30 * 24 * time.Hour // CA-09: 30 dias
)

// createSession grava uma sessão nova e devolve o token em claro (vai só
// pro cookie — CA-08).
func (s *Service) createSession(ctx context.Context, userID string) (string, error) {
	token, hash := NewSessionToken()
	expiresAt := time.Now().Add(sessionTTL)
	if _, err := s.cfg.DB.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`, hash, userID, expiresAt); err != nil {
		return "", fmt.Errorf("criar sessão: %w", err)
	}
	return token, nil
}

// sessionCookie monta o cookie de sessão com os atributos do CA-09.
func (s *Service) sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	}
}

// clearStateCookie zera o cookie temporário do state após o callback.
func (s *Service) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}
