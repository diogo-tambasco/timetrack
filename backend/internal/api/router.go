// Package api monta o router HTTP do Timetrack: /healthz, o prefixo /api/v1
// com envelope de erro canônico e (em produção) os estáticos do frontend.
package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// NewRouter monta o router base da fase 00. As fases seguintes penduram
// rotas no mesmo router (auth na 01, domínio na 02/03).
func NewRouter(opts ...Option) chi.Router {
	var cfg routerCfg
	for _, o := range opts {
		o(&cfg)
	}

	r := chi.NewRouter()

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		isAPI := strings.HasPrefix(req.URL.Path, "/api/") || req.URL.Path == "/healthz"
		if cfg.staticFS != nil && !isAPI {
			serveStatic(cfg.staticFS, w, req)
			return
		}
		WriteError(w, http.StatusNotFound, "not_found", "rota não encontrada")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "método não permitido")
	})

	r.Get("/healthz", handleHealthz)

	if cfg.auth != nil {
		mountAuth(r, &cfg)
	}

	return r
}
