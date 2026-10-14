package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/calendar"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/presence"
	"timetrack.live/backend/internal/realtime"
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/timer"
	"timetrack.live/backend/internal/workspace"
)

// Option configura o NewRouter sem quebrar quem não precisa de estáticos.
type Option func(*routerCfg)

type routerCfg struct {
	staticFS       fs.FS
	auth           *auth.Service
	workspace      *workspace.Store
	core           *Core
	comments       *Comments
	notes          *Notes
	reminders      *Reminders
	timer          *timer.Service
	entries        *entries.Service
	reports        *reports.Service
	calendar       *calendar.Service
	hub            *realtime.Hub
	push           *Push
	presence       *presence.Service
	invitations    *Invitations
	wsPingInterval time.Duration
	wsPingTimeout  time.Duration
}

// WithStaticFS liga o serving do frontend (dist/) com fallback SPA (CA-19):
// rota sem extensão fora de /api e /healthz → index.html; arquivo existente
// → estático; /api/* mantém o 404 JSON canônico.
func WithStaticFS(fsys fs.FS) Option {
	return func(c *routerCfg) { c.staticFS = fsys }
}

// serveStatic resolve uma requisição não casada pelas rotas de API.
func serveStatic(fsys fs.FS, w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}

	if f, err := fsys.Open(name); err == nil {
		f.Close()
		http.ServeFileFS(w, r, fsys, name)
		return
	}

	// Fallback SPA: só pra rota sem extensão de arquivo.
	if path.Ext(name) == "" {
		http.ServeFileFS(w, r, fsys, "index.html")
		return
	}

	http.NotFound(w, r)
}
