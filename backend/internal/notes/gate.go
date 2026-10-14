// Package notes: notas nativas .md (1 linha/nota no Postgres) + links do slash
// (fase 26). Camada de captura escrita NOVA sobre a infra de projetos/tasks/
// visibilidade das fases 02/12/24. O gate de visibilidade é uma CÓPIA do JOIN
// `(p.visibility='shared' OR p.owner_user_id=$u)` de tasks/comments (D-03) —
// NÃO importa projects/tasks pra ler; os pacotes da fase 02 ficam byte-idênticos.
package notes

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// noteGate é a nota JÁ resolvida (visível) com o que a escrita precisa saber.
type noteGate struct {
	ProjectID   *string // nil = nota solta
	CreatedBy   string
	OwnerUserID *string // dono do projeto (nil pra nota solta)
	Archived    bool
	Access      *string // access em project_members do user, se houver
}

// loadVisibleNote resolve a nota pra LEITURA por uma única query (cópia do
// padrão de tasks/comments — D-03):
//   - nota de PROJETO: visibilidade `(visibility='shared' OR owner_user_id=$u)`
//     → 404 quando invisível (CA-13/14, não vaza).
//   - nota SOLTA (project_id NULL): created_by==$u → ok; senão 404 (CA-11.1).
//
// uuid.Validate(noteID) antes do SQL → 404 nunca 500. NÃO decide escrita — isso
// é o requireNoteWrite (chamado pelo loadNoteGate só pra Create/Patch/Delete).
func (s *Service) loadVisibleNote(ctx context.Context, q rowQuerier, workspaceID, userID, noteID string) (noteGate, error) {
	if uuid.Validate(noteID) != nil {
		return noteGate{}, domain.NotFound("nota não encontrada")
	}
	var g noteGate
	// LEFT JOIN projects: a nota pode ser solta (project_id NULL → p.* NULL); a
	// visibilidade do projeto entra na cláusula do JOIN, então uma nota de
	// projeto INVISÍVEL também casa p.* NULL aqui — distinguir "solta" de
	// "projeto invisível" pelo n.project_id (não pelo p.id).
	err := q.QueryRow(ctx, `
		SELECT n.project_id, n.created_by, p.owner_user_id,
		       (p.id IS NOT NULL AND p.archived_at IS NOT NULL) AS archived,
		       (SELECT m.access::text FROM project_members m
		         WHERE m.project_id = p.id AND m.user_id = $2)   AS access
		  FROM notes n
		  LEFT JOIN projects p ON p.id = n.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE n.workspace_id = $1 AND n.id = $3`,
		workspaceID, userID, noteID).Scan(&g.ProjectID, &g.CreatedBy, &g.OwnerUserID, &g.Archived, &g.Access)
	if errors.Is(err, pgx.ErrNoRows) {
		return noteGate{}, domain.NotFound("nota não encontrada")
	}
	if err != nil {
		return noteGate{}, err
	}

	if g.ProjectID == nil {
		// nota SOLTA: só o criador (CA-11.1). Role do ws não dá acesso.
		if g.CreatedBy != userID {
			return noteGate{}, domain.NotFound("nota não encontrada")
		}
		return g, nil
	}
	// nota de PROJETO: o LEFT JOIN não resolveu o projeto (p.id NULL) ⇒ projeto
	// invisível ao user (private de outro) → 404 (CA-13/14, não vaza).
	if g.OwnerUserID == nil {
		return noteGate{}, domain.NotFound("nota não encontrada")
	}
	return g, nil
}

// loadNoteGate resolve a nota pra ESCRITA: visibilidade (404) → permissão (403)
// → arquivado (400 invalid_reference quando blockArchived). Nota solta já foi
// validada por created_by em loadVisibleNote.
//
// blockArchived POR OPERAÇÃO (26-H7): Create/Patch chamam com true; Delete com
// false (apagar nota de projeto arquivado é permitido → 204, CA-06.1).
func (s *Service) loadNoteGate(ctx context.Context, q rowQuerier, workspaceID, userID, role, noteID string, blockArchived bool) (noteGate, error) {
	g, err := s.loadVisibleNote(ctx, q, workspaceID, userID, noteID)
	if err != nil {
		return noteGate{}, err
	}
	if g.ProjectID == nil {
		// nota solta: o criador escreve (loadVisibleNote já garantiu).
		return g, nil
	}
	canWrite := (g.OwnerUserID != nil && *g.OwnerUserID == userID) || role == "owner" ||
		(g.Access != nil && *g.Access == "edit")
	if !canWrite {
		return noteGate{}, domain.Forbidden("sem permissão de escrita nesta nota")
	}
	if blockArchived && g.Archived {
		return noteGate{}, domain.InvalidReference("projeto arquivado é só leitura")
	}
	return g, nil
}

// loadProjectWriteGate resolve um project_id "cru" (do payload OU herdado de uma
// task) pra ESCRITA, espelhando tasks.loadProjectGate: visibilidade → quando é
// ref de PAYLOAD vira invalid_reference (400); permissão → 403; arquivado → 400
// invalid_reference. notFoundAsRef controla se "não visível/inexistente"
// responde como ref de payload (invalid_reference) ou como rota (not_found).
func (s *Service) loadProjectWriteGate(ctx context.Context, q rowQuerier, workspaceID, userID, role, projectID string, blockArchived, notFoundAsRef bool) error {
	notVisible := func() error {
		if notFoundAsRef {
			return domain.InvalidReference("project_id não encontrado")
		}
		return domain.NotFound("projeto não encontrado")
	}
	if uuid.Validate(projectID) != nil {
		return notVisible()
	}
	var ownerUserID string
	var archived bool
	var access *string
	err := q.QueryRow(ctx, `
		SELECT p.owner_user_id, p.archived_at IS NOT NULL,
		       (SELECT m.access::text FROM project_members m
		         WHERE m.project_id = p.id AND m.user_id = $2)
		  FROM projects p
		 WHERE p.workspace_id = $1 AND p.id = $3
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)`,
		workspaceID, userID, projectID).Scan(&ownerUserID, &archived, &access)
	if errors.Is(err, pgx.ErrNoRows) {
		return notVisible()
	}
	if err != nil {
		return err
	}
	canWrite := ownerUserID == userID || role == "owner" || (access != nil && *access == "edit")
	if !canWrite {
		return domain.Forbidden("sem permissão de escrita neste projeto")
	}
	if blockArchived && archived {
		return domain.InvalidReference("projeto arquivado é só leitura")
	}
	return nil
}

// rowQuerier abstrai pool e tx (igual tasks.rowQuerier).
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
