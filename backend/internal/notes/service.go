package notes

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/tasks"
)

// Note é o objeto do contrato (plan §5). Preview só na LISTA (D-05); Links
// resolvidos na leitura (CA-12, T-05).
type Note struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	ProjectID   *string    `json:"project_id"` // null = solta
	TaskID      *string    `json:"task_id"`    // null = não atrelada
	Title       string     `json:"title"`
	BodyMD      string     `json:"body_md"`
	Preview     string     `json:"preview,omitempty"` // só na LISTA
	Links       []NoteLink `json:"links"`             // resolvidos (CA-12)
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// NoteLink é um link resolvido pra exibir o chip (CA-12).
type NoteLink struct {
	ID       string `json:"id"`
	RefType  string `json:"ref_type"`
	RefID    string `json:"ref_id"`
	Label    string `json:"label"`            // nome da task / título da sub-nota
	Status   string `json:"status,omitempty"` // só p/ ref_type=task
	Position int    `json:"position"`
}

// CreateInput é o payload do POST /notes. ProjectID/TaskID opcionais (sem
// nenhum = nota solta — CA-01.2).
type CreateInput struct {
	ProjectID *string
	TaskID    *string
	Title     string
}

// PatchInput — title/body tri-state (ausente mantém; *Set distingue — D-09/CA-05).
type PatchInput struct {
	TitleSet bool
	Title    *string
	BodySet  bool
	BodyMD   *string
}

// Service é o domínio de notas (fase 26). Recebe o tasks.Service pro slash
// /task (cria task na mesma tx via tasks.CreateTx — D-04). A topologia é
// notes → tasks (uma direção, sem ciclo).
type Service struct {
	db    *pgxpool.Pool
	tasks *tasks.Service
}

// NewService liga o pool e o tasks.Service (pro slash /task).
func NewService(db *pgxpool.Pool, tasksSvc *tasks.Service) *Service {
	return &Service{db: db, tasks: tasksSvc}
}

// validTitle aplica CA-01.1/05.1: trim e ≤ 200 caracteres (vazio é permitido).
func validTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	if utf8.RuneCountInString(t) > 200 {
		return "", domain.Validation("título deve ter no máximo 200 caracteres")
	}
	return t, nil
}

// noteCols é a lista de colunas do objeto (sem preview/links — derivados).
const noteCols = `id, workspace_id, project_id, task_id, title, body_md, created_by, created_at, updated_at`

func scanNote(row pgx.Row) (Note, error) {
	var n Note
	err := row.Scan(&n.ID, &n.WorkspaceID, &n.ProjectID, &n.TaskID, &n.Title, &n.BodyMD,
		&n.CreatedBy, &n.CreatedAt, &n.UpdatedAt)
	if n.Links == nil {
		n.Links = []NoteLink{}
	}
	return n, err
}

// List devolve as notas de um projeto visível (projectID != "") ou as notas
// SOLTAS do user (projectID == "" — CA-03.1), ordem updated_at desc, com
// preview derivado (D-05) e links resolvidos (CA-12, T-05).
func (s *Service) List(ctx context.Context, workspaceID, userID, projectID string) ([]Note, error) {
	var rows pgx.Rows
	var err error
	if projectID == "" {
		// notas soltas do user (CA-03.1): só project_id NULL E created_by=$u.
		// Mutação removendo created_by=$u → vazaria solta de outro (CA-14).
		rows, err = s.db.Query(ctx, `
			SELECT `+noteCols+` FROM notes
			 WHERE workspace_id = $1 AND project_id IS NULL AND created_by = $2
			 ORDER BY updated_at DESC, created_at DESC, id`, workspaceID, userID)
	} else {
		// notas do projeto visível ao user. Projeto invisível/inexistente → 404
		// (ref de ROTA — CA-13). Checa visibilidade ANTES (senão lista vazia
		// mascararia projeto invisível).
		if err := s.requireVisibleProject(ctx, workspaceID, userID, projectID); err != nil {
			return nil, err
		}
		rows, err = s.db.Query(ctx, `
			SELECT `+noteCols+` FROM notes
			 WHERE workspace_id = $1 AND project_id = $2
			 ORDER BY updated_at DESC, created_at DESC, id`, workspaceID, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		n.Preview = mdStrip(n.BodyMD)
		list = append(list, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.resolveLinks(ctx, workspaceID, userID, list); err != nil {
		return nil, err
	}
	return list, nil
}

// Get devolve o objeto COMPLETO (body_md + links). Invisível/malformado → 404
// (CA-04).
func (s *Service) Get(ctx context.Context, workspaceID, userID, noteID string) (Note, error) {
	if _, err := s.loadVisibleNote(ctx, s.db, workspaceID, userID, noteID); err != nil {
		return Note{}, err
	}
	n, err := scanNote(s.db.QueryRow(ctx,
		`SELECT `+noteCols+` FROM notes WHERE workspace_id = $1 AND id = $2`, workspaceID, noteID))
	if err != nil {
		return Note{}, err
	}
	one := []Note{n}
	if err := s.resolveLinks(ctx, workspaceID, userID, one); err != nil {
		return Note{}, err
	}
	return one[0], nil
}

// Create cria a nota. O gate de escrita roda contra o project_id EFETIVO
// (payload OU herdado do task_id — 26-H3/CA-02.1):
//   - task_id no payload → resolve (visibilidade + mesmo ws senão 400
//     invalid_reference) e HERDA o project_id (tasks.project_id é NOT NULL →
//     nota vira de PROJETO, nunca solta);
//   - project_id efetivo não-nulo → gate de ESCRITA contra ESSE id (viewer/
//     access=view que só vê a task → 403);
//   - project_id efetivo nulo (nem payload nem task) → nota solta sem gate de
//     projeto (CA-01.2).
func (s *Service) Create(ctx context.Context, workspaceID, userID, role string, in CreateInput) (Note, error) {
	title, err := validTitle(in.Title)
	if err != nil {
		return Note{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Note{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// project_id EFETIVO: payload OU herdado da task.
	var effProjectID *string
	var taskID *string
	if in.TaskID != nil {
		pid, err := s.resolveTaskProject(ctx, tx, workspaceID, userID, *in.TaskID)
		if err != nil {
			return Note{}, err
		}
		effProjectID = &pid
		taskID = in.TaskID
	} else if in.ProjectID != nil {
		effProjectID = in.ProjectID
	}

	// gate de escrita contra o project_id EFETIVO (nota de projeto). Nulo =
	// nota solta (sem gate de projeto — CA-01.2).
	if effProjectID != nil {
		if err := s.loadProjectWriteGate(ctx, tx, workspaceID, userID, role, *effProjectID, true, true); err != nil {
			return Note{}, err
		}
	}

	n, err := scanNote(tx.QueryRow(ctx, `
		INSERT INTO notes (workspace_id, project_id, task_id, title, body_md, created_by)
		VALUES ($1, $2, $3, $4, '', $5)
		RETURNING `+noteCols, workspaceID, effProjectID, taskID, title, userID))
	if err != nil {
		return Note{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Note{}, err
	}
	return n, nil
}

// Patch atualiza title/body (tri-state). Gate de escrita com blockArchived=true
// (CA-05/11/14). updated_at=now().
func (s *Service) Patch(ctx context.Context, workspaceID, userID, role, noteID string, in PatchInput) (Note, error) {
	var newTitle *string
	if in.TitleSet && in.Title != nil {
		t, err := validTitle(*in.Title)
		if err != nil {
			return Note{}, err
		}
		newTitle = &t
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Note{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.loadNoteGate(ctx, tx, workspaceID, userID, role, noteID, true); err != nil {
		return Note{}, err
	}

	// COALESCE: $3/$4 NULL mantêm o valor atual (tri-state ausente). body_md ''
	// é válido (limpa). Quando TitleSet com nil → também mantém (Title nil =
	// ausente; o handler só seta TitleSet quando a chave veio).
	n, err := scanNote(tx.QueryRow(ctx, `
		UPDATE notes
		   SET title = COALESCE($3, title),
		       body_md = COALESCE($4, body_md),
		       updated_at = now()
		 WHERE workspace_id = $1 AND id = $2
		RETURNING `+noteCols, workspaceID, noteID, newTitle, boolNil(in.BodySet, in.BodyMD)))
	if err != nil {
		return Note{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Note{}, err
	}
	one := []Note{n}
	if err := s.resolveLinks(ctx, workspaceID, userID, one); err != nil {
		return Note{}, err
	}
	return one[0], nil
}

// Delete remove a nota (hard delete — CA-06); gate com blockArchived=FALSE
// (apagar nota de projeto arquivado é permitido — CA-06.1/26-H7). Os links caem
// por cascade. 2º delete do mesmo id → 404 (loadNoteGate não acha → not_found).
func (s *Service) Delete(ctx context.Context, workspaceID, userID, role, noteID string) error {
	if _, err := s.loadNoteGate(ctx, s.db, workspaceID, userID, role, noteID, false); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `DELETE FROM notes WHERE workspace_id = $1 AND id = $2`, workspaceID, noteID)
	return err
}

// requireVisibleProject: o projeto existe e é visível ao user? (mesmo JOIN do
// gate). ErrNoRows / uuid malformado → NotFound (ref de rota — CA-13).
func (s *Service) requireVisibleProject(ctx context.Context, workspaceID, userID, projectID string) error {
	if pgxValidUUID(projectID) != nil {
		return domain.NotFound("projeto não encontrado")
	}
	var ok bool
	err := s.db.QueryRow(ctx, `
		SELECT true FROM projects p
		 WHERE p.workspace_id = $1 AND p.id = $2
		   AND (p.visibility = 'shared' OR p.owner_user_id = $3)`,
		workspaceID, projectID, userID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("projeto não encontrado")
	}
	return err
}

// resolveTaskProject valida o task_id (visibilidade + mesmo ws) e devolve o
// project_id da task. Inválido/invisível/outro-ws/malformado → 400
// invalid_reference (ref de PAYLOAD — CA-02).
func (s *Service) resolveTaskProject(ctx context.Context, q rowQuerier, workspaceID, userID, taskID string) (string, error) {
	if pgxValidUUID(taskID) != nil {
		return "", domain.InvalidReference("task_id não encontrada")
	}
	var projectID string
	err := q.QueryRow(ctx, `
		SELECT t.project_id
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $3`,
		workspaceID, userID, taskID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.InvalidReference("task_id não encontrada")
	}
	if err != nil {
		return "", err
	}
	return projectID, nil
}

// boolNil devolve p só quando set==true; senão nil (pra COALESCE manter o
// valor no UPDATE quando o campo não veio no PATCH).
func boolNil(set bool, p *string) *string {
	if !set {
		return nil
	}
	return p
}
