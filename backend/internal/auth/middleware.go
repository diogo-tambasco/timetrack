// middleware.go — RequireAuth protege todo /api/v1/* (CA-11/CA-12).
// Desenho extensível (D-9): hoje só autentica por cookie de sessão; a fase
// 08 adiciona um segundo autenticador (header Authorization: Bearer tt_...)
// como mais um ramo de authenticate, sem tocar nos handlers.
package auth

import (
	"errors"
	"net/http"
	"time"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
)

var errNoSession = errors.New("sessão ausente ou expirada")

// RequireAuth nega com 401 qualquer requisição sem credencial válida e
// injeta o user_id no contexto pras camadas seguintes.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.authenticate(w, r)
		if err != nil {
			respond.Error(w, http.StatusUnauthorized, "unauthorized", "sessão ausente ou expirada")
			return
		}
		next.ServeHTTP(w, r.WithContext(reqctx.WithUserID(r.Context(), userID)))
	})
}

// authenticate resolve a credencial da requisição. Único esquema da fase
// 01: cookie de sessão. (Fase 08 pluga o token tt_ aqui.)
func (s *Service) authenticate(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", errNoSession
	}

	var userID string
	var expiresAt time.Time
	err = s.cfg.DB.QueryRow(r.Context(), `
		SELECT user_id, expires_at FROM sessions WHERE token_hash = $1`,
		HashToken(cookie.Value)).Scan(&userID, &expiresAt)
	if err != nil {
		return "", errNoSession
	}

	// CA-12: expirada → apaga a linha e zera o cookie do browser.
	if time.Now().After(expiresAt) {
		_, _ = s.cfg.DB.Exec(r.Context(),
			`DELETE FROM sessions WHERE token_hash = $1`, HashToken(cookie.Value))
		http.SetCookie(w, s.sessionCookie("", -1))
		return "", errNoSession
	}
	return userID, nil
}
