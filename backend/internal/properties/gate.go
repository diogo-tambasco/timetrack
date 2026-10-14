// Package properties: schema de propriedades custom por projeto (fase 25,
// "database por projeto"). O gate de projeto é AUTO-CONTIDO aqui (D-03 alt. B,
// herdado da fase 12): replica a regra de escrita/visibilidade de
// tasks.loadProjectGate SEM importar tasks/projects/objectives (zero churn de
// construtor, zero ciclo de import; RNF-06). Mantém os pacotes da fase 02/12
// byte-idênticos.
package properties

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Service expõe o CRUD de defs + set/clear de value por task (fase 25).
type Service struct{ db *pgxpool.Pool }

// NewService cria o serviço. O gate é interno (D-03 alt. B) — NÃO injeta
// ProjectGate nem recebe o projects.Service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// projectGate é o estado do projeto que governa permissão/visibilidade.
type projectGate struct {
	ownerUserID string
	archived    bool
	access      *string // access em project_members, se houver
}

// loadProjectGate carrega o projeto aplicando a visibilidade (CA-15): private
// de outro usuário e inexistente respondem o MESMO 404 (não vaza existência);
// projectID malformado também → 404 (nunca 500). É a porta dos READs (ListDefs)
// e o primeiro passo dos writes (ensureWrite).
func (s *Service) loadProjectGate(ctx context.Context, workspaceID, userID, projectID string) (projectGate, error) {
	if uuid.Validate(projectID) != nil {
		return projectGate{}, domain.NotFound("projeto não encontrado")
	}
	var g projectGate
	err := s.db.QueryRow(ctx, `
		SELECT p.owner_user_id, p.archived_at IS NOT NULL,
		       (SELECT access::text FROM project_members m
		         WHERE m.project_id = p.id AND m.user_id = $2)
		  FROM projects p
		 WHERE p.workspace_id = $1 AND p.id = $3
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)`,
		workspaceID, userID, projectID).Scan(&g.ownerUserID, &g.archived, &g.access)
	if errors.Is(err, pgx.ErrNoRows) {
		return projectGate{}, domain.NotFound("projeto não encontrado")
	}
	if err != nil {
		return projectGate{}, err
	}
	return g, nil
}

// canWrite aplica a regra de escrita da fase 02 (CA-01/07): dono do projeto,
// owner do workspace, ou membro do projeto com access=edit.
func (g projectGate) canWrite(userID, role string) bool {
	return g.ownerUserID == userID || role == "owner" ||
		(g.access != nil && *g.access == "edit")
}

// ensureWrite carrega o gate e exige permissão de escrita: visibilidade→404,
// permissão→403, arquivado (quando blockArchived)→400 invalid_reference (código
// REAL da fase 12; NÃO existe code `archived`). Devolve o gate pro chamador.
func (s *Service) ensureWrite(ctx context.Context, workspaceID, userID, role, projectID string, blockArchived bool) (projectGate, error) {
	g, err := s.loadProjectGate(ctx, workspaceID, userID, projectID)
	if err != nil {
		return projectGate{}, err
	}
	if !g.canWrite(userID, role) {
		return projectGate{}, domain.Forbidden("sem permissão de escrita neste projeto")
	}
	if blockArchived && g.archived {
		return projectGate{}, domain.InvalidReference("projeto arquivado é só leitura")
	}
	return g, nil
}
