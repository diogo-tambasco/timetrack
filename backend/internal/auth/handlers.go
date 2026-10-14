// handlers.go — handlers HTTP do fluxo de auth: callback do OAuth
// (T-06..T-08), logout e GET /api/v1/me (T-10/T-11).
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/reqctx"
	"timetrack.live/backend/internal/workspace"
)

// HandleCallback — GET /auth/google/callback. Ordem dos caminhos de falha
// antes do feliz: erro do Google (CA-06) → state anti-CSRF (CA-03).
func (s *Service) HandleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// fase 06 (verificação do build): lê o destino de retorno do connect e
	// EXPIRA o cookie JÁ — vale pra TODOS os caminhos de saída. Cancelar no
	// Google deixava o cookie vivo e o próximo login normal ia parar em
	// /calendario (viola CA-03.1).
	returnPath := ""
	if c, err := r.Cookie(returnCookieName); err == nil && safeReturnPath(c.Value) {
		returnPath = c.Value
	}
	s.clearReturnCookie(w)

	// CA-06: o Google retornou erro (ex.: access_denied) → volta pro login
	// sem nenhum efeito no banco.
	if q.Get("error") != "" {
		http.Redirect(w, r, "/login?error=google", http.StatusFound)
		return
	}

	// CA-03: o state da query tem que bater com o do cookie temporário.
	state := q.Get("state")
	cookie, err := r.Cookie(stateCookieName)
	if err != nil || state == "" || cookie.Value != state {
		respond.Error(w, http.StatusBadRequest, "invalid_state", "state do OAuth não confere")
		return
	}

	// Caminho feliz: exchange → userinfo → upsert transacional (CA-04/05).
	tok, err := s.Exchange(r.Context(), q.Get("code"))
	if err != nil {
		// troca do code falhou — mesmo destino do erro do Google (CA-06)
		http.Redirect(w, r, "/login?error=google", http.StatusFound)
		return
	}
	ui, err := s.Userinfo(r.Context(), tok)
	if err != nil {
		http.Redirect(w, r, "/login?error=google", http.StatusFound)
		return
	}

	// fase 06 (verificação do build): no fluxo de CONNECT (returnPath setado) a
	// conta que consentiu DEVE ser a mesma da sessão logada — senão o connect
	// trocaria o usuário silenciosamente (Gmail pessoal × corporativo) e
	// gravaria tokens da conta errada. Diverge → aborta sem recriar sessão.
	if returnPath != "" {
		if sid, ok := s.sessionGoogleID(r.Context(), r); ok && sid != ui.GoogleID {
			http.Redirect(w, r, returnPath+"?erro=conta", http.StatusFound)
			return
		}
	}

	// Tokens do Google cifrados ANTES de qualquer escrita (CA-14).
	enc, err := s.encryptToken(tok)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal", "falha ao proteger credenciais")
		return
	}

	userID, err := s.store.UpsertUserFromGoogle(r.Context(), workspace.GoogleProfile{
		GoogleID:  ui.GoogleID,
		Email:     ui.Email,
		Name:      ui.Name,
		AvatarURL: ui.AvatarURL,
	}, enc)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal", "falha ao registrar login")
		return
	}

	// fase 17 (D-05): resolve convites pendentes do email no 1º login (re-vínculo
	// inclusive — CA-28). Best-effort: roda em transação própria pós-commit do
	// login; erro é logado e ENGOLIDO (o login é prioritário — CA-24/27). Removida
	// esta chamada, o convite nunca materializa (RNF-05 pina por mutação).
	if s.cfg.ResolveInvites != nil {
		if err := s.cfg.ResolveInvites(r.Context(), userID, ui.Email); err != nil {
			slog.Error("falha ao resolver convites no login", "user_id", userID, "err", err)
		}
	}

	// Sessão de 30 dias: token em claro só no cookie, hash no banco (CA-08/09).
	session, err := s.createSession(r.Context(), userID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal", "falha ao criar sessão")
		return
	}
	s.clearStateCookie(w)

	// fase 06: o connect gravou tt_oauth_return=/calendario (lido e expirado no
	// topo); o login normal não grava → default "/".
	dest := "/"
	if returnPath != "" {
		dest = returnPath
	}

	http.SetCookie(w, s.sessionCookie(session, int(sessionTTL.Seconds())))
	http.Redirect(w, r, dest, http.StatusFound)
}

// sessionGoogleID devolve o google_id do usuário da sessão atual (cookie
// tt_session válido), ou ok=false. Usado pra impedir que o connect troque a
// conta logada (verificação do build da fase 06).
func (s *Service) sessionGoogleID(ctx context.Context, r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	var googleID string
	if err := s.cfg.DB.QueryRow(ctx, `
		SELECT u.google_id FROM sessions s
		  JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		HashToken(cookie.Value)).Scan(&googleID); err != nil {
		return "", false
	}
	return googleID, true
}

// returnCookieName guarda pra onde voltar após o callback (fase 06).
const returnCookieName = "tt_oauth_return"

// safeReturnPath aceita só caminho absoluto interno (não "//host", que o
// browser trata como URL protocol-relative — open redirect).
func safeReturnPath(v string) bool {
	return strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//")
}

// StartConsent (fase 06) inicia o fluxo OAuth de CONNECT: state anti-CSRF +
// cookie de retorno + redirect ao consentimento do Google com prompt=consent
// e os escopos extras (ex.: calendar.readonly). Exige sessão a montante.
func (s *Service) StartConsent(w http.ResponseWriter, r *http.Request, returnPath string, extraScopes ...string) {
	state, _ := NewSessionToken()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
	if safeReturnPath(returnPath) {
		http.SetCookie(w, &http.Cookie{
			Name:     returnCookieName,
			Value:    returnPath,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   s.cfg.CookieSecure,
		})
	}
	http.Redirect(w, r, s.BuildConsentURL(state, extraScopes...), http.StatusFound)
}

// clearReturnCookie expira o cookie de retorno após o callback.
func (s *Service) clearReturnCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     returnCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}

// HandleMe — GET /api/v1/me (CA-10): quem está logado e seus workspaces.
func (s *Service) HandleMe(w http.ResponseWriter, r *http.Request) {
	user, workspaces, err := s.store.UserWithWorkspaces(r.Context(), reqctx.UserID(r.Context()))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal", "falha ao carregar perfil")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"workspaces": workspaces,
	})
}

// HandleLogout — POST /api/v1/auth/logout (CA-13): revogação imediata.
func (s *Service) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if _, err := s.cfg.DB.Exec(r.Context(),
			`DELETE FROM sessions WHERE token_hash = $1`, HashToken(cookie.Value)); err != nil {
			respond.Error(w, http.StatusInternalServerError, "internal", "falha ao encerrar sessão")
			return
		}
	}
	http.SetCookie(w, s.sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

// encryptToken cifra access/refresh com AES-256-GCM (CA-14). Refresh
// ausente → nil, preservado no upsert via COALESCE (CA-14.1).
func (s *Service) encryptToken(tok *oauth2.Token) (workspace.EncryptedToken, error) {
	accessEnc, err := Seal(s.cfg.TokenKey, []byte(tok.AccessToken))
	if err != nil {
		return workspace.EncryptedToken{}, err
	}
	var refreshEnc []byte
	if tok.RefreshToken != "" {
		if refreshEnc, err = Seal(s.cfg.TokenKey, []byte(tok.RefreshToken)); err != nil {
			return workspace.EncryptedToken{}, err
		}
	}

	scopes := baseScopes
	if granted, ok := tok.Extra("scope").(string); ok && granted != "" {
		scopes = strings.Fields(granted)
	}
	return workspace.EncryptedToken{
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		ExpiresAt:       tok.Expiry,
		Scopes:          scopes,
	}, nil
}
