// Package subtasks: sub-tasks "goals da task" (fase 24, CA-06..13). Camada de
// planejamento NOVA sobre tasks/visibilidade das fases 02/12. NÃO importa
// tasks/projects/objectives (D-05): o gate (loadTaskGate, gate.go) é cópia da
// lógica de can_write/visibilidade do board da fase 12, via SQL direto. SEM
// carve-out viewer-assignee (D-10/CA-11) — escrita de sub-task = can_write do
// projeto da task PURO.
package subtasks

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Subtask é o objeto do contrato (plan §4.1/§5).
type Subtask struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	IsGoal    bool      `json:"is_goal"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput — POST: só title (done/is_goal/position default).
type CreateInput struct {
	Title string
}

// PatchInput — PATCH: subconjunto de {title, done, is_goal, position} (tri-state
// por presença; nil = não toca — CA-08).
type PatchInput struct {
	Title    *string
	Done     *bool
	IsGoal   *bool
	Position *int
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// validTitle aplica CA-06.1: TrimSpace; vazio → 400; > 500 runes → 400; persiste o trim.
func validTitle(s string) (string, error) {
	t := strings.TrimSpace(s)
	if t == "" || utf8.RuneCountInString(t) > 500 {
		return "", domain.Validation("título deve ter entre 1 e 500 caracteres")
	}
	return t, nil
}

// scanSubtask lê uma linha na ordem fixa das colunas (subtaskCols).
func scanSubtask(row pgx.Row) (Subtask, error) {
	var st Subtask
	err := row.Scan(&st.ID, &st.TaskID, &st.Title, &st.Done, &st.IsGoal, &st.Position, &st.CreatedAt)
	return st, err
}

const subtaskCols = `id, task_id, title, done, is_goal, position, created_at`

// List devolve as sub-tasks da task ordenadas por position (CA-07). A task
// precisa ser VISÍVEL ao user (gate de leitura — D-08); invisível/inexistente/
// outro-ws/uuid malformado → 404 (CA-13).
func (s *Service) List(ctx context.Context, workspaceID, userID, taskID string) ([]Subtask, error) {
	if _, err := s.loadTaskGate(ctx, workspaceID, userID, "", taskID, false, false); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+subtaskCols+` FROM task_subtasks
		 WHERE workspace_id = $1 AND task_id = $2
		 ORDER BY position, created_at, id`, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Subtask{}
	for rows.Next() {
		st, err := scanSubtask(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, st)
	}
	return list, rows.Err()
}

// Create insere a sub-task no FIM da lista da task (position = max+1) com
// workspace_id da ROTA (não do cliente — CA-06). Exige escrita no projeto da
// task (CA-11); projeto arquivado → 400 (decisão G).
func (s *Service) Create(ctx context.Context, workspaceID, userID, role, taskID string, in CreateInput) (Subtask, error) {
	title, err := validTitle(in.Title)
	if err != nil {
		return Subtask{}, err
	}
	if _, err := s.loadTaskGate(ctx, workspaceID, userID, role, taskID, true, true); err != nil {
		return Subtask{}, err
	}
	var pos int
	if err := s.db.QueryRow(ctx, `
		SELECT COALESCE(max(position), -1) + 1 FROM task_subtasks
		 WHERE task_id = $1`, taskID).Scan(&pos); err != nil {
		return Subtask{}, err
	}
	return scanSubtask(s.db.QueryRow(ctx, `
		INSERT INTO task_subtasks (task_id, workspace_id, title, position)
		VALUES ($1, $2, $3, $4)
		RETURNING `+subtaskCols, taskID, workspaceID, title, pos))
}

// Patch atualiza os campos presentes da sub-task (tri-state por presença —
// CA-08). A sub-task casa (workspace_id, task_id da ROTA, id) — cross-task/
// cross-ws → 404 (CA-13). Exige escrita (CA-11); arquivado → 400.
func (s *Service) Patch(ctx context.Context, workspaceID, userID, role, taskID, subtaskID string, in PatchInput) (Subtask, error) {
	if _, err := s.loadTaskGate(ctx, workspaceID, userID, role, taskID, true, true); err != nil {
		return Subtask{}, err
	}
	if uuid.Validate(subtaskID) != nil {
		return Subtask{}, domain.NotFound("sub-task não encontrada")
	}
	st, err := s.loadSubtask(ctx, workspaceID, taskID, subtaskID)
	if err != nil {
		return Subtask{}, err
	}
	if in.Title != nil {
		if st.Title, err = validTitle(*in.Title); err != nil {
			return Subtask{}, err
		}
	}
	if in.Done != nil {
		st.Done = *in.Done
	}
	if in.IsGoal != nil {
		st.IsGoal = *in.IsGoal
	}
	if in.Position != nil {
		if *in.Position < 0 {
			return Subtask{}, domain.Validation("position deve ser ≥ 0")
		}
		st.Position = *in.Position
	}
	return scanSubtask(s.db.QueryRow(ctx, `
		UPDATE task_subtasks
		   SET title = $4, done = $5, is_goal = $6, position = $7
		 WHERE workspace_id = $1 AND task_id = $2 AND id = $3
		RETURNING `+subtaskCols,
		workspaceID, taskID, subtaskID, st.Title, st.Done, st.IsGoal, st.Position))
}

// Delete remove a sub-task (hard delete — CA-09). Casa (workspace_id, task_id,
// id) — cross-task/cross-ws → 404 (CA-13). Exige escrita (CA-11); arquivado → 400.
func (s *Service) Delete(ctx context.Context, workspaceID, userID, role, taskID, subtaskID string) error {
	if _, err := s.loadTaskGate(ctx, workspaceID, userID, role, taskID, true, true); err != nil {
		return err
	}
	if uuid.Validate(subtaskID) != nil {
		return domain.NotFound("sub-task não encontrada")
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM task_subtasks
		 WHERE workspace_id = $1 AND task_id = $2 AND id = $3`, workspaceID, taskID, subtaskID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("sub-task não encontrada")
	}
	return nil
}

// loadSubtask carrega a sub-task casando (workspace_id, task_id da ROTA, id) —
// pertencimento estrito (CA-13: sub-task de outra task/ws via rota → 404).
func (s *Service) loadSubtask(ctx context.Context, workspaceID, taskID, subtaskID string) (Subtask, error) {
	st, err := scanSubtask(s.db.QueryRow(ctx, `
		SELECT `+subtaskCols+` FROM task_subtasks
		 WHERE workspace_id = $1 AND task_id = $2 AND id = $3`, workspaceID, taskID, subtaskID))
	if err == pgx.ErrNoRows {
		return Subtask{}, domain.NotFound("sub-task não encontrada")
	}
	return st, err
}
