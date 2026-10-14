// Package googlestub sobe um Google OAuth de mentira servindo as fixtures
// de backend/testdata/google (TDD.md §3: nunca bater no Google real).
// Usado pelos testes da fase 01 e pelo E2E (T-14), que precisa de um
// Google falso injetado no backend via env de teste.
package googlestub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
)

// Codes que o endpoint de token reconhece; qualquer outro devolve
// token_error.json com 400 (mesmo shape de erro do Google real).
const (
	CodeOK        = "code-ok"         // → token.json (com refresh_token)
	CodeNoRefresh = "code-no-refresh" // → token_no_refresh.json (CA-14.1)
	CodeCalendar  = "code-calendar"   // → token_calendar.json (fase 06: scope calendar.readonly)
)

// Server é o Google falso: /authorize, /token e /userinfo.
type Server struct {
	srv          *httptest.Server
	dir          string
	userinfoBody []byte // override do /userinfo (nil → fixture userinfo.json)
}

// SetUserinfo substitui a resposta do /userinfo desta instância — pros
// subcasos de upsert (nome vazio, e-mail re-vinculado, perfil atualizado).
func (s *Server) SetUserinfo(body []byte) { s.userinfoBody = body }

// New sobe o stub em porta efêmera (testes Go).
func New(fixturesDir string) *Server {
	s := &Server{dir: fixturesDir}
	s.srv = httptest.NewServer(s.handler())
	return s
}

// NewHandler devolve só o http.Handler do stub — o cmd/googlestub serve
// numa porta fixa pro E2E (endpoint injetado no backend via env).
func NewHandler(fixturesDir string) http.Handler {
	return (&Server{dir: fixturesDir}).handler()
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /authorize", s.handleAuthorize)
	mux.HandleFunc("POST /token", s.handleToken)
	mux.HandleFunc("GET /userinfo", s.handleUserinfo)
	return mux
}

func (s *Server) Close()      { s.srv.Close() }
func (s *Server) URL() string { return s.srv.URL }

// Endpoint devolve o oauth2.Endpoint pra injetar no auth.Config.
func (s *Server) Endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:  s.srv.URL + "/authorize",
		TokenURL: s.srv.URL + "/token",
	}
}

// UserinfoURL devolve a URL do userinfo pra injetar no auth.Config.
func (s *Server) UserinfoURL() string {
	return s.srv.URL + "/userinfo"
}

// handleAuthorize imita o consentimento: devolve o browser direto pro
// redirect_uri com code fixo e o mesmo state — fecha o ciclo no E2E.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("redirect_uri")
	if redirect == "" {
		http.Error(w, "redirect_uri ausente", http.StatusBadRequest)
		return
	}
	target, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "redirect_uri inválida", http.StatusBadRequest)
		return
	}
	// fase 06: o connect pede o escopo de calendar → concede um token COM
	// calendar.readonly (code distinto); login normal segue com code-ok.
	code := CodeOK
	if strings.Contains(r.URL.Query().Get("scope"), "calendar") {
		code = CodeCalendar
	}

	q := target.Query()
	q.Set("code", code)
	q.Set("state", r.URL.Query().Get("state"))
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form inválido", http.StatusBadRequest)
		return
	}
	switch r.PostFormValue("code") {
	case CodeOK:
		s.serveFixture(w, "token.json", http.StatusOK)
	case CodeNoRefresh:
		s.serveFixture(w, "token_no_refresh.json", http.StatusOK)
	case CodeCalendar:
		s.serveFixture(w, "token_calendar.json", http.StatusOK)
	default:
		s.serveFixture(w, "token_error.json", http.StatusBadRequest)
	}
}

func (s *Server) handleUserinfo(w http.ResponseWriter, r *http.Request) {
	const bearer = "Bearer "
	authz := r.Header.Get("Authorization")
	if len(authz) <= len(bearer) || authz[:len(bearer)] != bearer {
		http.Error(w, "token ausente", http.StatusUnauthorized)
		return
	}
	if s.userinfoBody != nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(s.userinfoBody)
		return
	}
	s.serveFixture(w, "userinfo.json", http.StatusOK)
}

func (s *Server) serveFixture(w http.ResponseWriter, name string, status int) {
	body, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		http.Error(w, "fixture ausente: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
