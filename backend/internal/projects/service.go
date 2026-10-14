// Package projects: regras de domínio de projeto (fase 02, CA-06..15) —
// validações na mão (D-07), visibilidade resolvida na query (D-04) e
// soft-delete via archived_at (D-03). Toda query filtra workspace_id (CA-26).
package projects

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Project é o objeto do contrato §3.2 do plan (sem members — a listagem
// não os embute; o detalhe adiciona à parte).
type Project struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	ClientID        *string    `json:"client_id"`
	Name            string     `json:"name"`
	Color           string     `json:"color"`
	WeeklyHoursGoal *float64   `json:"weekly_hours_goal"`
	Visibility      string     `json:"visibility"`
	OwnerUserID     string     `json:"owner_user_id"`
	ArchivedAt      *time.Time `json:"archived_at"`
	CreatedAt       time.Time  `json:"created_at"`
	Deadline        *string    `json:"deadline"` // fase 28: "YYYY-MM-DD" ou null (CA-01/02)
}

// Member é um membro do projeto (CA-11) — o dono é implícito e nunca entra.
type Member struct {
	UserID string `json:"user_id"`
	Access string `json:"access"` // edit | view
}

// CreateInput é o payload do POST /projects (visibility obrigatória — D-10).
// Members só é aceito em shared (CA-13) e persiste na mesma transação da
// criação (CA-11.1).
type CreateInput struct {
	Name            string
	Color           string
	Visibility      string
	ClientID        *string
	WeeklyHoursGoal *float64
	Members         []Member
	Deadline        *string // fase 28: nil = sem prazo (CA-01/02)
}

// PatchInput é o subconjunto editável (plan §3.2). ClientID e
// WeeklyHoursGoal são tri-state: ausente mantém, null remove (o *Set
// distingue os dois).
type PatchInput struct {
	Name            *string
	Color           *string
	Visibility      *string
	ClientIDSet     bool
	ClientID        *string
	GoalSet         bool
	WeeklyHoursGoal *float64
	MembersSet      bool
	Members         []Member
	DeadlineSet     bool // fase 28: tri-state (ausente mantém, null remove, data define — CA-01)
	Deadline        *string
}

type Service struct {
	db *pgxpool.Pool
	// hasObjectiveTargetDate é detectado 1× no boot (fase 28 D-05/CA-15.1): se a
	// coluna objectives.target_date (fase 29) não existe, a query (c) do
	// calendário é pulada e kind:"objective" é omitido. Liga sem mudar código
	// quando a 29 deployar.
	hasObjectiveTargetDate bool
}

func NewService(db *pgxpool.Pool) *Service {
	s := &Service{db: db}
	// detecção no boot (cacheada). Erro silencioso → assume ausente (degrada).
	if db != nil {
		var exists bool
		// table_schema = current_schema() escopa a detecção ao schema atual (prod =
		// public; dbtest = schema isolado do teste). Sem isso, a coluna criada pela
		// fase 29 em QUALQUER schema do mesmo banco dava false-positive (28-TD-1).
		_ = db.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				 WHERE table_name = 'objectives' AND column_name = 'target_date'
				   AND table_schema = current_schema())`).Scan(&exists)
		s.hasObjectiveTargetDate = exists
	}
	return s
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validName aplica CA-06.1: trim e 1–120 caracteres; persiste o trim.
func validName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 120 {
		return "", domain.Validation("nome deve ter entre 1 e 120 caracteres")
	}
	return trimmed, nil
}

// validColor aplica CA-07 (regex da spec).
func validColor(color string) error {
	if !colorRe.MatchString(color) {
		return domain.Validation("color deve estar no formato #RRGGBB")
	}
	return nil
}

// validGoal aplica CA-07: NULL = sem meta; senão (0,168].
func validGoal(goal *float64) error {
	if goal != nil && (*goal <= 0 || *goal > 168) {
		return domain.Validation("weekly_hours_goal deve ser maior que 0 e no máximo 168")
	}
	return nil
}

func validVisibility(v string) error {
	if v != "private" && v != "shared" {
		return domain.Validation("visibility deve ser private ou shared")
	}
	return nil
}

// validateDeadline aplica CA-01.1: nil = sem prazo; senão precisa ser uma data
// YYYY-MM-DD parseável (time.Parse ANTES do SQL — nunca 500 por cast no Postgres).
func validateDeadline(s *string) error {
	if s == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *s); err != nil {
		return domain.Validation("deadline deve ser uma data YYYY-MM-DD")
	}
	return nil
}

// checkClientRef aplica CA-08/CA-08.1: client_id precisa existir NO MESMO
// workspace e não estar arquivado. Inexistente, de outro workspace e id
// fora do formato uuid respondem o MESMO invalid_reference.
func (s *Service) checkClientRef(ctx context.Context, workspaceID string, clientID *string) error {
	if clientID == nil {
		return nil
	}
	if uuid.Validate(*clientID) != nil {
		return domain.InvalidReference("client_id não pertence a este workspace")
	}
	var archivedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT archived_at FROM clients WHERE workspace_id = $1 AND id = $2`,
		workspaceID, *clientID).Scan(&archivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InvalidReference("client_id não pertence a este workspace")
	}
	if err != nil {
		return err
	}
	if archivedAt != nil {
		return domain.InvalidReference("client_id referencia um cliente arquivado")
	}
	return nil
}

// Create cria o projeto com owner_user_id = usuário da sessão (CA-06).
func (s *Service) Create(ctx context.Context, workspaceID, ownerUserID string, in CreateInput) (Project, error) {
	name, err := validName(in.Name)
	if err != nil {
		return Project{}, err
	}
	if err := validColor(in.Color); err != nil {
		return Project{}, err
	}
	if err := validGoal(in.WeeklyHoursGoal); err != nil {
		return Project{}, err
	}
	if err := validVisibility(in.Visibility); err != nil {
		return Project{}, err
	}
	if err := validateDeadline(in.Deadline); err != nil {
		return Project{}, err
	}
	if err := s.checkClientRef(ctx, workspaceID, in.ClientID); err != nil {
		return Project{}, err
	}
	if in.Visibility == "private" && len(in.Members) > 0 {
		return Project{}, domain.Validation("projeto privado não tem membros")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p, err := scanProject(tx.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal, visibility, owner_user_id, deadline)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+projectCols,
		workspaceID, in.ClientID, name, in.Color, in.WeeklyHoursGoal, in.Visibility, ownerUserID, in.Deadline))
	if err != nil {
		return Project{}, err
	}
	// CA-11.1: members entram na MESMA transação da criação — tudo ou nada
	if _, err := s.replaceMembersTx(ctx, tx, workspaceID, p, in.Members); err != nil {
		return Project{}, err
	}
	return p, tx.Commit(ctx)
}

const projectCols = `id, workspace_id, client_id, name, color, weekly_hours_goal,
                     visibility, owner_user_id, archived_at, created_at,
                     to_char(deadline, 'YYYY-MM-DD')`

func scanProject(row pgx.Row) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.WorkspaceID, &p.ClientID, &p.Name, &p.Color,
		&p.WeeklyHoursGoal, &p.Visibility, &p.OwnerUserID, &p.ArchivedAt, &p.CreatedAt,
		&p.Deadline)
	return p, err
}

// getForWrite carrega o projeto pra escrita aplicando a visibilidade
// (D-04): private de outro usuário responde o MESMO not_found de
// inexistente — inclusive pro owner do workspace (CA-09).
func (s *Service) getForWrite(ctx context.Context, workspaceID, userID, projectID string) (Project, error) {
	if uuid.Validate(projectID) != nil {
		return Project{}, domain.NotFound("projeto não encontrado")
	}
	p, err := scanProject(s.db.QueryRow(ctx, `
		SELECT `+projectCols+` FROM projects
		 WHERE workspace_id = $1 AND id = $3 AND `+visibleWhere,
		workspaceID, userID, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, domain.NotFound("projeto não encontrado")
	}
	return p, err
}

// canEditMeta aplica CA-14: metadados só pelo dono do projeto ou pelo
// owner do workspace (a visibilidade já foi resolvida no getForWrite).
func canEditMeta(p Project, userID, role string) error {
	if p.OwnerUserID != userID && role != "owner" {
		return domain.Forbidden("apenas o dono do projeto ou o owner do workspace editam metadados")
	}
	return nil
}

// visibleWhere é a regra ÚNICA de visibilidade (D-04, CA-09/10): shared é
// de todos; private só do dono — sem exceção pro owner do workspace.
const visibleWhere = `(visibility = 'shared' OR owner_user_id = $2)`

// Get carrega o detalhe aplicando visibilidade (CA-09); invisível responde
// o MESMO 404 de inexistente. Arquivado responde normalmente (CA-15.1).
func (s *Service) Get(ctx context.Context, workspaceID, userID, projectID string) (Project, error) {
	if uuid.Validate(projectID) != nil {
		return Project{}, domain.NotFound("projeto não encontrado")
	}
	p, err := scanProject(s.db.QueryRow(ctx, `
		SELECT `+projectCols+` FROM projects
		 WHERE workspace_id = $1 AND id = $3 AND `+visibleWhere,
		workspaceID, userID, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, domain.NotFound("projeto não encontrado")
	}
	return p, err
}

// List devolve os projetos visíveis ao usuário (CA-09/10); arquivados só
// com includeArchived (CA-15).
func (s *Service) List(ctx context.Context, workspaceID, userID string, includeArchived bool) ([]Project, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+projectCols+` FROM projects
		 WHERE workspace_id = $1 AND `+visibleWhere+` AND ($3 OR archived_at IS NULL)
		 ORDER BY created_at, id`, workspaceID, userID, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// Members devolve a lista persistida de membros do projeto (CA-11).
func (s *Service) Members(ctx context.Context, projectID string) ([]Member, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, access FROM project_members
		 WHERE project_id = $1 ORDER BY user_id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Access); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// Patch altera metadados (CA-14): visibilidade via getForWrite, permissão
// via canEditMeta, validações por campo presente.
func (s *Service) Patch(ctx context.Context, workspaceID, userID, role, projectID string, in PatchInput) (Project, error) {
	p, err := s.getForWrite(ctx, workspaceID, userID, projectID)
	if err != nil {
		return Project{}, err
	}
	if err := canEditMeta(p, userID, role); err != nil {
		return Project{}, err
	}

	if in.Name != nil {
		if p.Name, err = validName(*in.Name); err != nil {
			return Project{}, err
		}
	}
	if in.Color != nil {
		if err := validColor(*in.Color); err != nil {
			return Project{}, err
		}
		p.Color = *in.Color
	}
	if in.Visibility != nil {
		if err := validVisibility(*in.Visibility); err != nil {
			return Project{}, err
		}
		p.Visibility = *in.Visibility
	}
	if in.ClientIDSet {
		if err := s.checkClientRef(ctx, workspaceID, in.ClientID); err != nil {
			return Project{}, err
		}
		p.ClientID = in.ClientID
	}
	if in.GoalSet {
		if err := validGoal(in.WeeklyHoursGoal); err != nil {
			return Project{}, err
		}
		p.WeeklyHoursGoal = in.WeeklyHoursGoal
	}
	if in.DeadlineSet {
		if err := validateDeadline(in.Deadline); err != nil {
			return Project{}, err
		}
		p.Deadline = in.Deadline
	}
	// CA-13: private (ou virando private) não aceita lista de members
	if p.Visibility == "private" && in.MembersSet && len(in.Members) > 0 {
		return Project{}, domain.Validation("projeto privado não tem membros")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanProject(tx.QueryRow(ctx, `
		UPDATE projects
		   SET name = $3, color = $4, visibility = $5, client_id = $6, weekly_hours_goal = $7,
		       deadline = $8
		 WHERE workspace_id = $1 AND id = $2
		RETURNING `+projectCols,
		workspaceID, projectID, p.Name, p.Color, p.Visibility, p.ClientID, p.WeeklyHoursGoal,
		p.Deadline))
	if err != nil {
		return Project{}, err
	}

	if updated.Visibility == "private" {
		// CA-13.1: virou private → zera project_members na MESMA transação
		if _, err := tx.Exec(ctx,
			`DELETE FROM project_members WHERE project_id = $1`, updated.ID); err != nil {
			return Project{}, err
		}
	} else if in.MembersSet {
		// PATCH com members em shared substitui a lista (semântica do PUT)
		if _, err := s.replaceMembersTx(ctx, tx, workspaceID, updated, in.Members); err != nil {
			return Project{}, err
		}
	}
	return updated, tx.Commit(ctx)
}

// ReplaceMembers é o PUT substitutivo (D-06/CA-11): troca a lista inteira
// numa transação; mesma permissão de metadados (dono ou owner do ws).
func (s *Service) ReplaceMembers(ctx context.Context, workspaceID, userID, role, projectID string, members []Member) ([]Member, error) {
	p, err := s.getForWrite(ctx, workspaceID, userID, projectID)
	if err != nil {
		return nil, err
	}
	if err := canEditMeta(p, userID, role); err != nil {
		return nil, err
	}
	if p.Visibility == "private" && len(members) > 0 {
		return nil, domain.Validation("projeto privado não tem membros")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	persisted, err := s.replaceMembersTx(ctx, tx, workspaceID, p, members)
	if err != nil {
		return nil, err
	}
	return persisted, tx.Commit(ctx)
}

// replaceMembersTx valida (CA-12/13) e substitui project_members dentro
// da transação do chamador — POST, PATCH e PUT convergem aqui.
func (s *Service) replaceMembersTx(ctx context.Context, tx pgx.Tx, workspaceID string, p Project, members []Member) ([]Member, error) {
	valid, err := validMembersList(ctx, tx, workspaceID, p.OwnerUserID, members)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM project_members WHERE project_id = $1`, p.ID); err != nil {
		return nil, err
	}
	for _, m := range valid {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, access)
			VALUES ($1, $2, $3)`, p.ID, m.UserID, m.Access); err != nil {
			return nil, err
		}
	}
	return valid, nil
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// validMembersList aplica as regras da lista: dono é implícito (filtrado),
// access ∈ edit|view, sem duplicata, e TODO user_id precisa ser membro do
// workspace (CA-12 — tudo ou nada).
func validMembersList(ctx context.Context, q rowQuerier, workspaceID, ownerUserID string, members []Member) ([]Member, error) {
	valid := []Member{}
	seen := map[string]bool{}
	ids := []string{}
	for _, m := range members {
		if m.UserID == ownerUserID {
			continue
		}
		if m.Access != "edit" && m.Access != "view" {
			return nil, domain.Validation("access deve ser edit ou view")
		}
		if uuid.Validate(m.UserID) != nil {
			return nil, domain.InvalidReference("user_id não é membro do workspace")
		}
		if seen[m.UserID] {
			return nil, domain.Validation("user_id duplicado na lista de membros")
		}
		seen[m.UserID] = true
		valid = append(valid, m)
		ids = append(ids, m.UserID)
	}
	if len(ids) > 0 {
		var n int
		if err := q.QueryRow(ctx, `
			SELECT count(*) FROM workspace_members
			 WHERE workspace_id = $1 AND user_id = ANY($2::uuid[])`,
			workspaceID, ids).Scan(&n); err != nil {
			return nil, err
		}
		if n != len(ids) {
			return nil, domain.InvalidReference("user_id não é membro do workspace")
		}
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].UserID < valid[j].UserID })
	return valid, nil
}

// Archive arquiva o projeto sem apagar tasks (CA-15). Idempotente.
func (s *Service) Archive(ctx context.Context, workspaceID, userID, role, projectID string) error {
	p, err := s.getForWrite(ctx, workspaceID, userID, projectID)
	if err != nil {
		return err
	}
	if err := canEditMeta(p, userID, role); err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		UPDATE projects SET archived_at = COALESCE(archived_at, now())
		 WHERE workspace_id = $1 AND id = $2`, workspaceID, projectID)
	return err
}
