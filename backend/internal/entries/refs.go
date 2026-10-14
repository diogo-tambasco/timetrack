// refs.go — validação COMPARTILHADA de referências do payload (CA-29):
// project_id/task_id precisam pertencer ao workspace. Usada pelo
// timer.Service (start/resume) e pelo CRUD de entradas manuais (T-12) —
// anti-vazamento na escrita: fora do workspace = o MESMO not_found de
// inexistente.
package entries

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// ValidateRefs confere que project_id e task_id (quando presentes)
// pertencem ao workspace. Qualquer referência cruzada, inexistente ou fora
// do formato uuid responde not_found (CA-29 — não vazar existência).
func ValidateRefs(ctx context.Context, q Querier, workspaceID string, projectID, taskID *string) error {
	if err := checkRef(ctx, q, "projects", workspaceID, projectID,
		domain.NotFound("projeto não encontrado")); err != nil {
		return err
	}
	return checkRef(ctx, q, "tasks", workspaceID, taskID,
		domain.NotFound("task não encontrada"))
}

func checkRef(ctx context.Context, q Querier, table, workspaceID string, id *string, notFound error) error {
	if id == nil {
		return nil
	}
	if uuid.Validate(*id) != nil {
		return notFound
	}
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT true FROM `+table+` WHERE workspace_id = $1 AND id = $2`,
		workspaceID, *id).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}
