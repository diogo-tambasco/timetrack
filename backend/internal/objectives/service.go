// service.go — fase 12 T-03: CRUD de objetivos. Toda escrita passa pelo gate
// auto-contido (ensureWrite: 404/403/400); validação na mão (D-07). Patch/
// Delete carregam o objetivo SEMPRE casando (workspace_id, project_id da rota,
// id) — CA-13.1: objetivo de projeto vizinho via rota errada → 404 (não vaza).
package objectives

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// Objective é o objeto do contrato (§5 do plan). Fase 29: CompletionMode
// (by_tasks|by_hours|by_date) + TargetDate (YYYY-MM-DD ou null) — aditivos.
type Objective struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	ProjectID      string    `json:"project_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	GoalHours      *float64  `json:"goal_hours"`
	CompletionMode string    `json:"completion_mode"` // fase 29: by_tasks|by_hours|by_date
	TargetDate     *string   `json:"target_date"`     // fase 29: YYYY-MM-DD ou null
	Position       int       `json:"position"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateInput é o payload do POST (description e goal_hours opcionais). Fase 29:
// CompletionMode ("" → by_tasks, default — D-07) + TargetDate (nil → sem marco).
type CreateInput struct {
	Name           string
	Description    string
	GoalHours      *float64
	CompletionMode string
	TargetDate     *string
}

// PatchInput — campos editáveis (CA-04). GoalHours é tri-state (GoalSet
// distingue ausente×null; null remove a meta). Position idem. Fase 29:
// CompletionMode (ponteiro; ausente mantém) + TargetDate tri-state
// (TargetDateSet distingue ausente×null; null limpa o marco — CA-03.1).
type PatchInput struct {
	Name           *string
	Description    *string
	Status         *string
	GoalSet        bool
	GoalHours      *float64
	PositionSet    bool
	Position       *int
	CompletionMode *string
	TargetDateSet  bool
	TargetDate     *string
}

const objectiveCols = `id, workspace_id, project_id, name, description, status,
                       goal_hours, completion_mode, to_char(target_date, 'YYYY-MM-DD'),
                       position, created_by, created_at`

func scanObjective(row pgx.Row) (Objective, error) {
	var o Objective
	err := row.Scan(&o.ID, &o.WorkspaceID, &o.ProjectID, &o.Name, &o.Description,
		&o.Status, &o.GoalHours, &o.CompletionMode, &o.TargetDate, &o.Position,
		&o.CreatedBy, &o.CreatedAt)
	return o, err
}

// validName: trim e 1–200 caracteres (CA-01.1).
func validName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 200 {
		return "", domain.Validation("nome deve ter entre 1 e 200 caracteres")
	}
	return trimmed, nil
}

// validGoal: NULL = sem meta; senão (0, 9999]. Teto e piso validados ANTES do
// SQL pra nunca deixar o numeric(6,2) estourar pra 500 (CA-02). O piso checa o
// valor JÁ ARREDONDADO a 2 casas (math.Round(*100)): um positivo minúsculo
// (0 < g < 0.005) arredondaria pra 0.00 e violaria o CHECK > 0 — 400, não 500.
func validGoal(goal *float64) error {
	if goal != nil && (*goal <= 0 || *goal > 9999 || math.Round(*goal*100) < 1) {
		return domain.Validation("goal_hours deve ser maior que 0 e no máximo 9999")
	}
	return nil
}

func validStatus(s string) error {
	if s != "open" && s != "done" {
		return domain.Validation("status deve ser open ou done")
	}
	return nil
}

// validCompletionMode: switch nos 3 valores (fase 29 — CA-02.1). Validado ANTES
// do SQL pra nunca deixar o CHECK estourar pra 500. Espelha validStatus.
func validCompletionMode(m string) error {
	switch m {
	case "by_tasks", "by_hours", "by_date":
		return nil
	}
	return domain.Validation("completion_mode deve ser by_tasks, by_hours ou by_date")
}

// validTargetDate: nil ok (sem marco); senão parse YYYY-MM-DD (CA-03). Validado
// ANTES do SQL → 400 em vez de 500 por cast inválido.
func validTargetDate(d *string) error {
	if d == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *d); err != nil {
		return domain.Validation("target_date deve estar no formato YYYY-MM-DD")
	}
	return nil
}

// List devolve os objetivos do projeto ordenados por position (CA-03);
// projeto invisível → 404 (CA-13).
func (s *Service) List(ctx context.Context, workspaceID, userID, projectID string) ([]Objective, error) {
	if _, err := s.loadProjectGate(ctx, workspaceID, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+objectiveCols+` FROM objectives
		 WHERE workspace_id = $1 AND project_id = $2
		 ORDER BY position, created_at, id`, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Objective{}
	for rows.Next() {
		o, err := scanObjective(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

// Create insere o objetivo no fim da lista (position auto), status open,
// created_by da sessão (CA-01); escrita exige permissão e projeto não
// arquivado (CA-08/13).
func (s *Service) Create(ctx context.Context, workspaceID, userID, role, projectID string, in CreateInput) (Objective, error) {
	name, err := validName(in.Name)
	if err != nil {
		return Objective{}, err
	}
	if err := validGoal(in.GoalHours); err != nil {
		return Objective{}, err
	}
	// fase 29: completion_mode ausente → by_tasks (default, D-07); valida o modo
	// e o target_date ANTES do SQL (CA-02.1/03).
	mode := in.CompletionMode
	if mode == "" {
		mode = "by_tasks"
	}
	if err := validCompletionMode(mode); err != nil {
		return Objective{}, err
	}
	if err := validTargetDate(in.TargetDate); err != nil {
		return Objective{}, err
	}
	if _, err := s.ensureWrite(ctx, workspaceID, userID, role, projectID, true); err != nil {
		return Objective{}, err
	}
	return scanObjective(s.db.QueryRow(ctx, `
		INSERT INTO objectives (workspace_id, project_id, name, description, goal_hours, completion_mode, target_date, position, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date,
		        COALESCE((SELECT max(position) + 1 FROM objectives WHERE project_id = $2), 0),
		        $8)
		RETURNING `+objectiveCols,
		workspaceID, projectID, name, in.Description, in.GoalHours, mode, in.TargetDate, userID))
}

// getInProject carrega o objetivo casando (workspace, projeto da rota, id) —
// CA-13.1: objetivo de outro projeto/workspace ou inexistente → 404; id
// malformado também (uuid.Validate antes → nunca 500).
func (s *Service) getInProject(ctx context.Context, workspaceID, projectID, objectiveID string) (Objective, error) {
	if uuid.Validate(objectiveID) != nil {
		return Objective{}, domain.NotFound("objetivo não encontrado")
	}
	o, err := scanObjective(s.db.QueryRow(ctx, `
		SELECT `+objectiveCols+` FROM objectives
		 WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
		workspaceID, projectID, objectiveID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Objective{}, domain.NotFound("objetivo não encontrado")
	}
	return o, err
}

// Patch aplica o subconjunto presente (CA-04); status:done/open fecha/reabre
// manualmente (CA-05). Escrita em projeto arquivado → 400 (CA-13).
func (s *Service) Patch(ctx context.Context, workspaceID, userID, role, projectID, objectiveID string, in PatchInput) (Objective, error) {
	if _, err := s.ensureWrite(ctx, workspaceID, userID, role, projectID, true); err != nil {
		return Objective{}, err
	}
	o, err := s.getInProject(ctx, workspaceID, projectID, objectiveID)
	if err != nil {
		return Objective{}, err
	}

	if in.Name != nil {
		if o.Name, err = validName(*in.Name); err != nil {
			return Objective{}, err
		}
	}
	if in.Description != nil {
		o.Description = *in.Description
	}
	if in.Status != nil {
		if err := validStatus(*in.Status); err != nil {
			return Objective{}, err
		}
		o.Status = *in.Status
	}
	if in.GoalSet {
		if err := validGoal(in.GoalHours); err != nil {
			return Objective{}, err
		}
		o.GoalHours = in.GoalHours
	}
	if in.PositionSet && in.Position != nil {
		o.Position = *in.Position
	}
	// fase 29: completion_mode ponteiro (ausente mantém); target_date tri-state
	// (ausente mantém, null limpa, valor define — CA-03.1). Trocar o modo NÃO
	// zera target_date/goal_hours (só reescreve se vier no payload — CA-04/04.1).
	if in.CompletionMode != nil {
		if err := validCompletionMode(*in.CompletionMode); err != nil {
			return Objective{}, err
		}
		o.CompletionMode = *in.CompletionMode
	}
	if in.TargetDateSet {
		if err := validTargetDate(in.TargetDate); err != nil {
			return Objective{}, err
		}
		o.TargetDate = in.TargetDate
	}

	return scanObjective(s.db.QueryRow(ctx, `
		UPDATE objectives
		   SET name = $4, description = $5, status = $6, goal_hours = $7, position = $8,
		       completion_mode = $9, target_date = $10::date
		 WHERE workspace_id = $1 AND project_id = $2 AND id = $3
		RETURNING `+objectiveCols,
		workspaceID, projectID, objectiveID, o.Name, o.Description, o.Status, o.GoalHours, o.Position,
		o.CompletionMode, o.TargetDate))
}

// Delete remove o objetivo (hard delete — CA-07); as tasks que o referenciam
// viram "Geral" via ON DELETE SET NULL. Arquivado NÃO bloqueia delete (igual
// task da fase 02).
func (s *Service) Delete(ctx context.Context, workspaceID, userID, role, projectID, objectiveID string) error {
	if _, err := s.ensureWrite(ctx, workspaceID, userID, role, projectID, false); err != nil {
		return err
	}
	if _, err := s.getInProject(ctx, workspaceID, projectID, objectiveID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`DELETE FROM objectives WHERE workspace_id = $1 AND project_id = $2 AND id = $3`,
		workspaceID, projectID, objectiveID)
	return err
}
