// core_routes.go — wiring da fase 02: pendura clients/projects/tasks no
// subrouter /api/v1/workspaces/{workspaceId} (D-01), atrás da cadeia
// RequireAuth → RequireMember → RequireWriteRole (D-02).
package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/api/respond"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/properties"
	"timetrack.live/backend/internal/subtasks"
	"timetrack.live/backend/internal/tasks"
)

// Core agrega os services do domínio core (fase 02 + objetivos da fase 12 +
// sub-tasks da fase 24).
type Core struct {
	Clients    *clients.Service
	Projects   *projects.Service
	Tasks      *tasks.Service
	Objectives *objectives.Service
	Subtasks   *subtasks.Service   // fase 24: sub-tasks "goals da task" (mount Core — decisão F)
	Properties *properties.Service // fase 25: propriedades tipadas custom por projeto
	// OnAssign (fase 18) é o fan-out da notificação de atribuição (notice + push),
	// injetado no boot (cmd/server). nil = no-op (testes do service não precisam
	// do hub). O service NÃO importa realtime/push — devolve o sinal e o handler
	// dispara isto pós-commit.
	OnAssign func(targetUserID, taskID, byUserID string)
}

// WithCore liga as rotas do domínio core no router.
func WithCore(core Core) Option {
	return func(c *routerCfg) {
		c.core = &core
	}
}

func mountCore(wr chi.Router, c *Core) {
	wr.Route("/clients", func(r chi.Router) {
		r.Get("/", c.handleListClients)
		r.Post("/", c.handleCreateClient)
		r.Patch("/{clientId}", c.handlePatchClient)
		r.Delete("/{clientId}", c.handleDeleteClient)
	})
	wr.Route("/projects", func(r chi.Router) {
		r.Get("/", c.handleListProjects)
		r.Post("/", c.handleCreateProject)
		// fase 28: resumo (cards vivos) — rota estática antes do {projectId}
		r.Get("/summary", c.handleProjectsSummary)
		r.Get("/{projectId}", c.handleGetProject)
		r.Patch("/{projectId}", c.handlePatchProject)
		r.Delete("/{projectId}", c.handleDeleteProject)
		r.Put("/{projectId}/members", c.handleReplaceMembers)
		// fase 28: calendário do projeto (itens datados, read-only)
		r.Get("/{projectId}/calendar", c.handleProjectCalendar)
		// fase 12: objetivos + board aninhados no projeto (cadeia de visibilidade)
		if c.Objectives != nil {
			r.Get("/{projectId}/board", c.handleBoard)
			r.Route("/{projectId}/objectives", func(o chi.Router) {
				o.Get("/", c.handleListObjectives)
				o.Post("/", c.handleCreateObjective)
				o.Patch("/{objectiveId}", c.handlePatchObjective)
				o.Delete("/{objectiveId}", c.handleDeleteObjective)
			})
		}
		// fase 25: defs de propriedade aninhadas no projeto (cadeia de visibilidade —
		// D-01) + values aninhados no projeto/task. O gate fino (loadProjectGate) cobre
		// access=view→403, arquivado→400, invisível→404; pertencimento def↔projeto↔task→404.
		if c.Properties != nil {
			r.Route("/{projectId}/properties", func(p chi.Router) {
				p.Get("/", c.handleListPropertyDefs)
				p.Post("/", c.handleCreatePropertyDef)
				p.Patch("/{defId}", c.handlePatchPropertyDef)
				p.Delete("/{defId}", c.handleDeletePropertyDef)
			})
			r.Put("/{projectId}/tasks/{taskId}/properties/{defId}", c.handleSetPropertyValue)
			r.Delete("/{projectId}/tasks/{taskId}/properties/{defId}", c.handleClearPropertyValue)
		}
	})
	wr.Route("/tasks", func(r chi.Router) {
		r.Get("/", c.handleListTasks)
		r.Post("/", c.handleCreateTask)
		r.Get("/{taskId}", c.handleGetTask)
		r.Patch("/{taskId}", c.handlePatchTask)
		r.Delete("/{taskId}", c.handleDeleteTask)
		// fase 24: sub-tasks "goals da task" aninhadas na task (mount Core, sob a
		// cadeia RequireMember+RequireWriteRole do subrouter — decisão F). O gate
		// fino (loadTaskGate) cobre access=view→403, arquivado→400, invisível→404.
		if c.Subtasks != nil {
			r.Route("/{taskId}/subtasks", func(s chi.Router) {
				s.Get("/", c.handleListSubtasks)
				s.Post("/", c.handleCreateSubtask)
				s.Patch("/{subtaskId}", c.handlePatchSubtask)
				s.Delete("/{subtaskId}", c.handleDeleteSubtask)
			})
		}
	})
}

// writeDomainError mapeia *domain.Error pro envelope canônico; qualquer
// outro erro vira 500 sem vazar detalhe.
func writeDomainError(w http.ResponseWriter, err error) {
	var de *domain.Error
	if errors.As(err, &de) {
		respond.Error(w, de.Status, de.Code, de.Message)
		return
	}
	respond.Error(w, http.StatusInternalServerError, "internal", "erro interno")
}

// decodeJSON aplica a convenção do specs/INDEX.md: payload que não parseia
// é 400 validation_error.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := jsonDecode(r, dst); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation_error", "payload inválido")
		return false
	}
	return true
}
