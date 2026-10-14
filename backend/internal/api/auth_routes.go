// auth_routes.go — wiring da fase 01 no router: rotas /auth/google/* fora
// do /api/v1 (navegação de browser) e o subrouter /api/v1 inteiro atrás do
// RequireAuth. As fases 02/03 penduram sub-rotas no subrouter do workspace.
package api

import (
	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/workspace"
)

// WithAuth liga o fluxo OAuth, o middleware de sessão e as rotas
// autenticadas da fase 01.
func WithAuth(svc *auth.Service, ws *workspace.Store) Option {
	return func(c *routerCfg) {
		c.auth = svc
		c.workspace = ws
	}
}

func mountAuth(r chi.Router, cfg *routerCfg) {
	svc, ws := cfg.auth, cfg.workspace
	r.Get("/auth/google/login", svc.HandleLogin)
	r.Get("/auth/google/callback", svc.HandleCallback)

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(svc.RequireAuth)
		api.Get("/me", svc.HandleMe)
		api.Post("/auth/logout", svc.HandleLogout)
		if cfg.timer != nil {
			mountTimer(api, cfg.timer, ws) // fase 03: rotas POR USUÁRIO (INDEX.md)
		}
		if cfg.hub != nil {
			mountRealtime(api, cfg) // fase 04: GET /ws atrás do RequireAuth (CA-05)
		}
		if cfg.calendar != nil {
			mountCalendar(api, cfg) // fase 06: leitura do Google Calendar (por usuário)
		}
		if cfg.push != nil {
			mountPush(api, cfg.push) // fase 16: subscribe + key (por usuário)
		}
		if cfg.reminders != nil {
			// fase 20: "Meus lembretes" — cross-workspace, POR USUÁRIO (fora do
			// subrouter /workspaces/{id}, espelha /api/v1/timer).
			api.Get("/reminders", cfg.reminders.handleListMine)
		}
		if cfg.core != nil {
			// fase 18: "Minhas tasks"/"Pedidas por mim" — cross-workspace, POR
			// USUÁRIO (fora do subrouter /workspaces/{id}).
			mountAssigned(api, cfg.core.Tasks)
		}
		if cfg.reports != nil {
			// fase 14: preferências do relatório por usuário (jornada útil)
			api.Get("/settings/reports", handleGetReportSettings(cfg.reports))
			api.Put("/settings/reports", handlePutReportSettings(cfg.reports))
		}
		if cfg.comments != nil {
			// fase 19 (plan §3 D-02): subrouter IRMÃO da task, com membership
			// (RequireMember) mas SEM RequireWriteRole — o gate de escrita da
			// thread é condicional por task (member⇒visibilidade; viewer⇒ser
			// assignee) e mora no comments.Service. Registrado FORA do
			// /workspaces/{workspaceId} pra não cair sob o wildcard daquele Mount
			// (chi panica em Mount sob wildcard existente).
			api.Route("/workspaces/{workspaceId}/tasks/{taskId}/comments", func(cr chi.Router) {
				cr.Use(ws.RequireMember)
				cr.Get("/", cfg.comments.handleList)
				cr.Post("/", cfg.comments.handleCreate)
				cr.Delete("/{commentId}", cfg.comments.handleDelete)
			})
		}
		if cfg.notes != nil {
			// fase 26 (plan §3 D-02): subrouter IRMÃO do workspace, com membership
			// (RequireMember) mas SEM RequireWriteRole — o gate de escrita da nota é
			// condicional (member que enxerga LÊ; escreve só com access=edit/owner) e
			// mora no notes.Service. Registrado FORA do /workspaces/{workspaceId} pra
			// não cair sob o wildcard daquele Mount (chi panica em Mount sob wildcard
			// existente) — igual às fases 19/20.
			api.Route("/workspaces/{workspaceId}/notes", func(nr chi.Router) {
				nr.Use(ws.RequireMember)
				nr.Get("/", cfg.notes.handleList)
				nr.Post("/", cfg.notes.handleCreate)
				nr.Get("/{noteId}", cfg.notes.handleGet)
				nr.Patch("/{noteId}", cfg.notes.handlePatch)
				nr.Delete("/{noteId}", cfg.notes.handleDelete)
				nr.Post("/{noteId}/links", cfg.notes.handleCreateLink)
				nr.Delete("/{noteId}/links/{linkId}", cfg.notes.handleDeleteLink)
			})
		}
		if cfg.reminders != nil {
			// fase 20 (plan §4): subrouter IRMÃO da task, com RequireMember mas SEM
			// RequireWriteRole — o carve-out viewer-assignee (viewer cria lembrete só
			// na task onde é assignee) mora no reminders.Service. Registrado FORA do
			// /workspaces/{workspaceId} pra não cair sob o wildcard daquele Mount
			// (chi panica em Mount sob wildcard existente) — igual à fase 19.
			api.Route("/workspaces/{workspaceId}/tasks/{taskId}/reminders", func(rr chi.Router) {
				rr.Use(ws.RequireMember)
				rr.Post("/", cfg.reminders.handleCreate)
				rr.Delete("/{reminderId}", cfg.reminders.handleDelete)
			})
		}
		api.Route("/workspaces/{workspaceId}", func(wr chi.Router) {
			wr.Use(ws.RequireMember)
			wr.Use(RequireWriteRole) // fase 02: 403 pra viewer em escrita (CA-23)
			wr.Get("/", ws.HandleGetWorkspace)
			if cfg.core != nil {
				mountCore(wr, cfg.core)
			}
			if cfg.invitations != nil {
				mountInvitations(wr, cfg.invitations) // fase 17: gestão Owner-only
			}
			if cfg.entries != nil {
				mountEntries(wr, cfg.entries, cfg.timer) // fase 03: CRUD + continue
			}
			if cfg.presence != nil {
				mountPresence(wr, cfg.presence) // fase 21: snapshot de presence (GET, leitura)
			}
			if cfg.reports != nil {
				mountReports(wr, cfg.reports) // fase 07: relatório semanal
				if cfg.calendar != nil {
					// fase 06 ângulo 4: Planejado × Real (precisa reports + calendar)
					wr.Get("/reports/planned", handlePlannedReport(cfg.reports, cfg.calendar))
				}
			}
		})
	})
}
