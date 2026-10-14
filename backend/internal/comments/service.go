// Package comments: thread de comentários por task (fase 19). Camada de
// colaboração NOVA sobre a infra de tasks/visibilidade das fases 02/12.
// Toda query herda o gate de visibilidade do projeto (visibleWhere/
// loadProjectGate) por JOIN; o carve-out viewer-assignee é avaliado AQUI
// no service (não no RequireWriteRole global — plan §3 D-02/D-03).
package comments

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Comment é o objeto do contrato (plan §3). AuthorName vem por JOIN com
// users.name (CA-02/06) — sem N+1.
type Comment struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	WorkspaceID string    `json:"workspace_id"`
	AuthorID    string    `json:"author_id"`
	AuthorName  string    `json:"author_name"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// validBody aplica CA-03/04/05: TrimSpace primeiro; vazio → 400; > 4000 runes
// (utf8.RuneCountInString, não bytes) → 400; persiste o trim.
func validBody(b string) (string, error) {
	t := strings.TrimSpace(b)
	if t == "" {
		return "", domain.Validation("comentário não pode ser vazio")
	}
	if utf8.RuneCountInString(t) > 4000 {
		return "", domain.Validation("comentário deve ter no máximo 4000 caracteres")
	}
	return t, nil
}

// Create resolve a task (visibilidade + carve-out por uma única query — plan
// §3 D-03), valida o body, insere com workspace_id da ROTA (não do payload —
// CA-16) e devolve o objeto com author_name.
func (s *Service) Create(ctx context.Context, workspaceID, userID, role, taskID, body string) (Comment, error) {
	trimmed, err := validBody(body)
	if err != nil {
		return Comment{}, err
	}
	if uuid.Validate(taskID) != nil {
		return Comment{}, domain.NotFound("task não encontrada")
	}

	// Gate: a task é visível ao user E ele pode comentar?
	//  - member/owner: basta ver a task (member_can).
	//  - viewer: só se é o assignee DESTA task (viewer_can). COALESCE força
	//    false quando assignee IS NULL (o caso NORMAL) — senão NULL = $u é
	//    SQL NULL e o Scan pra bool dá 500 onde o esperado é 403 (CA-13.1).
	var memberCan, viewerCan bool
	err = s.db.QueryRow(ctx, `
		SELECT ($3 <> 'viewer')                        AS member_can,
		       COALESCE(t.assignee = $2, false)        AS viewer_can
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $4`,
		workspaceID, userID, role, taskID).Scan(&memberCan, &viewerCan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, domain.NotFound("task não encontrada")
	}
	if err != nil {
		return Comment{}, err
	}
	if !(memberCan || viewerCan) {
		return Comment{}, domain.Forbidden("sem permissão para comentar nesta task")
	}

	c, err := scanComment(s.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO task_comments (task_id, workspace_id, author_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING id, task_id, workspace_id, author_id, body, created_at
		)
		SELECT ins.id, ins.task_id, ins.workspace_id, ins.author_id, u.name, ins.body, ins.created_at
		  FROM ins JOIN users u ON u.id = ins.author_id`,
		taskID, workspaceID, userID, trimmed))
	if err != nil {
		return Comment{}, err
	}
	return c, nil
}

// List devolve a thread da task visível, ASC por created_at (desempate id).
// A visibilidade é checada ANTES (senão GET de task invisível listaria —
// CA-12); leitura NÃO exige carve-out (qualquer membro que VÊ a task lê a
// thread — matriz §8). Vazio → [] não-nil.
func (s *Service) List(ctx context.Context, workspaceID, userID, taskID string) ([]Comment, error) {
	if err := s.requireVisibleTask(ctx, workspaceID, userID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.task_id, c.workspace_id, c.author_id, u.name, c.body, c.created_at
		  FROM task_comments c
		  JOIN users u ON u.id = c.author_id
		 WHERE c.workspace_id = $1 AND c.task_id = $2
		 ORDER BY c.created_at, c.id`, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// Delete remove o comentário se o user é o AUTOR ou o OWNER do workspace
// (nunca por ser assignee — o carve-out é só de criar/comentar, CA-14.1).
// Comment de outra task / invisível / ws alheio / id malformado → 404
// (não vaza). 204 ao apagar; 2º DELETE do mesmo id → 404 (idempotente).
func (s *Service) Delete(ctx context.Context, workspaceID, userID, role, taskID, commentID string) error {
	if uuid.Validate(commentID) != nil {
		return domain.NotFound("comentário não encontrado")
	}
	if uuid.Validate(taskID) != nil {
		return domain.NotFound("task não encontrada")
	}
	var authorID string
	err := s.db.QueryRow(ctx, `
		SELECT c.author_id
		  FROM task_comments c
		  JOIN tasks t    ON t.id = c.task_id AND t.workspace_id = $1
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE c.id = $4 AND c.task_id = $3 AND c.workspace_id = $1`,
		workspaceID, userID, taskID, commentID).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("comentário não encontrado")
	}
	if err != nil {
		return err
	}
	if !(authorID == userID || role == "owner") {
		return domain.Forbidden("só o autor ou o dono apaga o comentário")
	}
	_, err = s.db.Exec(ctx,
		`DELETE FROM task_comments WHERE id = $1 AND workspace_id = $2`, commentID, workspaceID)
	return err
}

// requireVisibleTask: a task existe e é visível ao user (mesmo JOIN do gate)?
// Sem carve-out (leitura). ErrNoRows / uuid malformado → NotFound (não vaza).
func (s *Service) requireVisibleTask(ctx context.Context, workspaceID, userID, taskID string) error {
	if uuid.Validate(taskID) != nil {
		return domain.NotFound("task não encontrada")
	}
	var ok bool
	err := s.db.QueryRow(ctx, `
		SELECT true
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $3`,
		workspaceID, userID, taskID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("task não encontrada")
	}
	return err
}

func scanComment(row pgx.Row) (Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.TaskID, &c.WorkspaceID, &c.AuthorID, &c.AuthorName, &c.Body, &c.CreatedAt)
	return c, err
}
