// gate.go — fase 24 (D-05/D-08): o gate de sub-task é AUTO-CONTIDO (cópia da
// lógica do board da fase 12), SEM importar tasks/projects/objectives. Carrega a
// task casando (workspace_id, id), resolve a visibilidade/can_write do PROJETO da
// task numa única query (JOIN tasks→projects + subquery do access em
// project_members). SEM carve-out viewer-assignee (D-10/CA-11) — can_write puro.
package subtasks

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// taskGate é o estado do projeto da task que governa permissão/visibilidade.
type taskGate struct {
	ownerUserID string
	archived    bool
	access      *string
}

// loadTaskGate resolve o gate da task pra leitura/escrita de sub-task:
//   - uuid.Validate(taskID) → 404 (nunca 500);
//   - carrega a task casando (workspace_id, id) + JOIN projects com a visibilidade
//     (p.visibility='shared' OR p.owner_user_id=$u); senão → 404 (não vaza);
//   - se requireWrite: can_write = dono ∥ role=="owner" ∥ access=="edit"; senão → 403;
//   - se requireWrite && blockArchived && projeto arquivado → 400 invalid_reference.
//
// Resumo de códigos (decisão G): invisível/inexistente/outro-ws/uuid malformado
// → 404; sem escrita → 403; projeto arquivado → 400 invalid_reference.
func (s *Service) loadTaskGate(ctx context.Context, workspaceID, userID, role, taskID string, requireWrite, blockArchived bool) (taskGate, error) {
	if uuid.Validate(taskID) != nil {
		return taskGate{}, domain.NotFound("task não encontrada")
	}
	var g taskGate
	err := s.db.QueryRow(ctx, `
		SELECT p.owner_user_id, p.archived_at IS NOT NULL,
		       (SELECT access::text FROM project_members m
		         WHERE m.project_id = p.id AND m.user_id = $2)
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		 WHERE t.workspace_id = $1 AND t.id = $3
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)`,
		workspaceID, userID, taskID).Scan(&g.ownerUserID, &g.archived, &g.access)
	if errors.Is(err, pgx.ErrNoRows) {
		return taskGate{}, domain.NotFound("task não encontrada")
	}
	if err != nil {
		return taskGate{}, err
	}
	if !requireWrite {
		return g, nil
	}
	canWrite := g.ownerUserID == userID || role == "owner" ||
		(g.access != nil && *g.access == "edit")
	if !canWrite {
		return taskGate{}, domain.Forbidden("sem permissão de escrita neste projeto")
	}
	if blockArchived && g.archived {
		return taskGate{}, domain.InvalidReference("projeto arquivado é só leitura")
	}
	return g, nil
}
