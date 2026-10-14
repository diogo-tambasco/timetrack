// Package tasks: regras de domínio de task (fase 02, CA-15.2/16..22).
// Permissão de escrita em 2 níveis (D-05): metadados de projeto não entram
// aqui — escrita de task = dono do projeto, owner do workspace (em projeto
// visível) ou membro do projeto com access=edit. workspace_id denormalizado
// (D-09) mantém o filtro tenant trivial (CA-26).
package tasks

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Task é o objeto do contrato §3.3 do plan. objective_id (fase 12) é o vínculo
// opcional com um objetivo do projeto; null = bucket "Geral" (CA-09/11).
type Task struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ProjectID   string    `json:"project_id"`
	ObjectiveID *string   `json:"objective_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`   // fase 24: 0..3 (0 sem · 1 baixa · 2 média · 3 alta)
	NotionURL   *string   `json:"notion_url"` // fase 24: link de doc opcional (null = sem doc)
	Position    int       `json:"position"`   // fase 24: ordenação manual no board/lista
	DueDate     *string   `json:"due_date"`   // YYYY-MM-DD
	Recurrence  *string   `json:"recurrence"`
	Notes       string    `json:"notes"`
	CreatedBy   string    `json:"created_by"`
	Assignee    *string   `json:"assignee"`
	RequestedBy *string   `json:"requested_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateInput é o payload do POST /tasks. ObjectiveID opcional (CA-09).
// Assignee/RequestedBy (fase 18, CA-02) ligam os campos antes dormentes.
type CreateInput struct {
	ProjectID   string
	Name        string
	ObjectiveID *string
	Status      *string // fase 24: ∈ {backlog,todo,doing,done}; nil → todo (default da coluna)
	Priority    *int    // fase 24: 0..3; nil → 0
	NotionURL   *string // fase 24: link de doc; nil/vazio → sem doc
	DueDate     *string
	Recurrence  *string
	Notes       string
	Assignee    *string
	RequestedBy *string
	TZ          string // fase 20: fuso da requisição pro reminder due (09:00 local); vazio → UTC
}

// PatchInput — campos editáveis; DueDate, Recurrence, ObjectiveID, Assignee e
// RequestedBy são tri-state (ausente mantém, null limpa; o *Set distingue os
// dois — D-08 / fase 18 D-01).
type PatchInput struct {
	Name           *string
	Status         *string
	PrioritySet    bool // fase 24
	Priority       *int
	NotionURLSet   bool // fase 24: tri-state (ausente mantém, null limpa, string define — D-09)
	NotionURL      *string
	PositionSet    bool // fase 24
	Position       *int
	DueDateSet     bool
	DueDate        *string
	RecurrenceSet  bool
	Recurrence     *string
	Notes          *string
	ObjectiveIDSet bool
	ObjectiveID    *string
	AssigneeSet    bool
	Assignee       *string
	RequestedBySet bool
	RequestedBy    *string
	TZ             string // fase 20: fuso da requisição pro reminder due (09:00 local); vazio → UTC
}

// ReminderHook é o gancho fino do reminder due (fase 20, D-02): chamado DENTRO
// da tx do Create/Patch SOMENTE quando a due_date MUDOU DE VALOR. tx é a MESMA
// transação; o hook cancela o due pendente antigo e cria o novo (se dueDate !=
// nil), resolvendo o responsável (assignee ?? created_by) lendo a task na tx. tz
// vem da requisição. Default no-op (RNF-05) — o tasks.Service não importa
// reminders (sem ciclo): o boot injeta a impl via SetReminderHook.
type ReminderHook interface {
	OnDueDateSet(ctx context.Context, tx pgx.Tx, taskID, userID string, dueDate *string, tz string) error
}

// noopReminderHook é o default: não faz nada (quem não liga reminders — testes
// da 02/12 — segue intocado, CA-28).
type noopReminderHook struct{}

func (noopReminderHook) OnDueDateSet(context.Context, pgx.Tx, string, string, *string, string) error {
	return nil
}

type Service struct {
	db       *pgxpool.Pool
	reminder ReminderHook
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db, reminder: noopReminderHook{}}
}

// SetReminderHook injeta o gancho do reminder due (wiring no boot — fase 20).
// nil → no-op.
func (s *Service) SetReminderHook(h ReminderHook) {
	if h == nil {
		s.reminder = noopReminderHook{}
		return
	}
	s.reminder = h
}

// AssignNotify sinaliza ao handler (camada api) que uma operação atribuiu uma
// task a um TERCEIRO (mudança real de assignee, assignee != quem chamou) e que
// o aviso pós-commit deve ser disparado (notice + push — fase 16). O service
// NÃO importa realtime/push (sem ciclo): só devolve o sinal; o handler faz o
// fan-out via Core.OnAssign. nil = sem notificação (CA-14/15).
type AssignNotify struct {
	TargetUserID string
	TaskID       string
}

// assignNotify devolve o sinal de notificação se houve mudança real de
// assignee POR TERCEIRO (CA-13/14/15): old != new (por valor) E new != nil E
// new != quem chamou (sem auto-set). Caso contrário nil.
func assignNotify(old, new *string, userID, taskID string) *AssignNotify {
	if ptrEqual(old, new) || new == nil || *new == userID {
		return nil
	}
	return &AssignNotify{TargetUserID: *new, TaskID: taskID}
}

// validName aplica CA-16.2: trim e 1–200 caracteres.
func validName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 200 {
		return "", domain.Validation("nome deve ter entre 1 e 200 caracteres")
	}
	return trimmed, nil
}

// validDueDate aplica CA-20: data ISO 8601 YYYY-MM-DD ou nada.
func validDueDate(d *string) error {
	if d == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *d); err != nil {
		return domain.Validation("due_date deve ser uma data ISO 8601 (YYYY-MM-DD)")
	}
	return nil
}

// validRecurrence aplica CA-20.1: texto opaco de até 255 caracteres (D-08).
func validRecurrence(r *string) error {
	if r != nil && utf8.RuneCountInString(*r) > 255 {
		return domain.Validation("recurrence deve ter no máximo 255 caracteres")
	}
	return nil
}

// validTZ rejeita um tz não-IANA (fase 20): a rota passa o tz da requisição pro
// reminder due (09:00 local); um tz inválido vira 400 ANTES da escrita (não 500
// no meio da tx). Vazio → ok (UTC).
func validTZ(tz string) error {
	if tz == "" {
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return domain.Validation("tz deve ser um fuso IANA válido")
	}
	return nil
}

// validStatus aplica CA-18 (fase 02) + CA-01 (fase 24): enum fechado
// backlog|todo|doing|done.
func validStatus(s string) error {
	if s != "backlog" && s != "todo" && s != "doing" && s != "done" {
		return domain.Validation("status deve ser backlog, todo, doing ou done")
	}
	return nil
}

// validPriority aplica CA-02.1: inteiro 0..3 (validado no Go ANTES do SQL — D-07).
func validPriority(p *int) error {
	if p == nil {
		return nil
	}
	if *p < 0 || *p > 3 {
		return domain.Validation("priority deve estar entre 0 e 3")
	}
	return nil
}

// validPosition aplica D-07: inteiro ≥ 0.
func validPosition(p *int) error {
	if p != nil && *p < 0 {
		return domain.Validation("position deve ser ≥ 0")
	}
	return nil
}

// normalizeNotionURL aplica CA-04: texto livre ≤ 2048 (sem validar formato — o
// preview é client-side). Trim → vazio vira nil (sem doc); > 2048 → 400.
func normalizeNotionURL(u *string) (*string, error) {
	if u == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*u)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > 2048 {
		return nil, domain.Validation("notion_url deve ter no máximo 2048 caracteres")
	}
	return &trimmed, nil
}

// nextPosition resolve o position do create (D-03): max(position)+1 entre as
// tasks do mesmo (project_id, status) — a task nova vai pro FIM da coluna. Sem
// tasks no bucket → 0. Roda na tx do Create (rowQuerier).
func nextPosition(ctx context.Context, q rowQuerier, projectID, status string) (int, error) {
	var pos int
	err := q.QueryRow(ctx, `
		SELECT COALESCE(max(position), -1) + 1 FROM tasks
		 WHERE project_id = $1 AND status = $2`, projectID, status).Scan(&pos)
	return pos, err
}

// validateMember confere que um assignee/requested_by (quando presente) é
// membro do workspace da task (CA-04). nil → ok; uuid malformado → 400 (CA-05,
// não 500); uuid que não é membro do workspace → 400 GENÉRICO — a mesma
// mensagem pra "não existe em users" e pra "existe mas em outro workspace"
// (não vaza existência nem o tenant — CA-04/12). Roda ANTES do INSERT/UPDATE
// (RNF-03). Reusa rowQuerier (pool ou tx).
func validateMember(ctx context.Context, q rowQuerier, workspaceID string, u *string) error {
	if u == nil {
		return nil
	}
	if uuid.Validate(*u) != nil {
		return domain.Validation("assignee deve ser um membro do workspace")
	}
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT true FROM workspace_members
		 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, *u).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Validation("assignee deve ser um membro do workspace")
	}
	return err
}

// projectGate é o que a permissão de escrita precisa saber do projeto.
type projectGate struct {
	OwnerUserID string
	Archived    bool
	Access      *string // access em project_members, se houver
}

// loadProjectGate resolve o projeto pra escrita de task: visibilidade
// primeiro (CA-17 — inexistente e private de outro respondem o MESMO 404),
// depois permissão (CA-16.1, 403), depois arquivamento quando bloqueante
// (CA-15.2, 400).
func (s *Service) loadProjectGate(ctx context.Context, workspaceID, userID, role, projectID string, blockArchived bool) error {
	if uuid.Validate(projectID) != nil {
		return domain.NotFound("projeto não encontrado")
	}
	var g projectGate
	err := s.db.QueryRow(ctx, `
		SELECT p.owner_user_id, p.archived_at IS NOT NULL,
		       (SELECT access::text FROM project_members m
		         WHERE m.project_id = p.id AND m.user_id = $2)
		  FROM projects p
		 WHERE p.workspace_id = $1 AND p.id = $3
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)`,
		workspaceID, userID, projectID).Scan(&g.OwnerUserID, &g.Archived, &g.Access)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("projeto não encontrado")
	}
	if err != nil {
		return err
	}
	canWrite := g.OwnerUserID == userID || role == "owner" ||
		(g.Access != nil && *g.Access == "edit")
	if !canWrite {
		return domain.Forbidden("sem permissão de escrita neste projeto")
	}
	if blockArchived && g.Archived {
		return domain.InvalidReference("projeto arquivado é só leitura")
	}
	return nil
}

// checkViewerAssigneeGate é a autoridade ÚNICA do carve-out viewer-assignee
// (fase 18, D-03 / CA-10/10.1/11). Este é o padrão que as fases 19/20 reusam.
// Aplica, nesta ordem:
//  1. o viewer DEVE ser o assignee DESTA task (senão 403) — CA-11a;
//  2. o patch DEVE tocar APENAS o status (qualquer outro campo set → 403) — CA-11b;
//  3. herda o bloqueio de projeto arquivado: se o projeto da task está arquivado,
//     400 invalid_reference — o MESMO que owner/member (CA-10.1).
func (s *Service) checkViewerAssigneeGate(ctx context.Context, t Task, userID string, in PatchInput) error {
	if t.Assignee == nil || *t.Assignee != userID {
		return domain.Forbidden("viewer só edita a task em que é responsável")
	}
	// só status: nenhum outro campo do PatchInput pode estar setado (inclusive os
	// campos novos da fase 24 — priority/notion_url/position são escrita de
	// planejamento, fora do carve-out viewer-assignee, D-10/CA-11).
	onlyStatus := in.Status != nil &&
		in.Name == nil && in.Notes == nil &&
		!in.DueDateSet && !in.RecurrenceSet && !in.ObjectiveIDSet &&
		!in.AssigneeSet && !in.RequestedBySet &&
		!in.PrioritySet && !in.NotionURLSet && !in.PositionSet
	if !onlyStatus {
		return domain.Forbidden("viewer responsável só pode mudar o status da task")
	}
	// herda o bloqueio de projeto arquivado (CA-10.1) — checagem leve do
	// archived_at do projeto da task (não chama loadProjectGate; a task já foi
	// resolvida pelo Get sob visibilidade).
	var archived bool
	if err := s.db.QueryRow(ctx,
		`SELECT archived_at IS NOT NULL FROM projects WHERE id = $1`, t.ProjectID).Scan(&archived); err != nil {
		return err
	}
	if archived {
		return domain.InvalidReference("projeto arquivado é só leitura")
	}
	return nil
}

// taskCols devolve as colunas do contrato com o prefixo de alias dado
// (vazio em queries sem join; "t." quando há join com projects).
func taskCols(prefix string) string {
	cols := []string{"id", "workspace_id", "project_id", "objective_id", "name", "status",
		"priority", "notion_url", "position",
		"to_char(" + prefix + "due_date, 'YYYY-MM-DD')", "recurrence", "notes",
		"created_by", "assignee", "requested_by", "created_at"}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		if strings.HasPrefix(c, "to_char") {
			out += c
			continue
		}
		out += prefix + c
	}
	return out
}

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.WorkspaceID, &t.ProjectID, &t.ObjectiveID, &t.Name, &t.Status,
		&t.Priority, &t.NotionURL, &t.Position,
		&t.DueDate, &t.Recurrence, &t.Notes, &t.CreatedBy, &t.Assignee,
		&t.RequestedBy, &t.CreatedAt)
	return t, err
}

// Create cria a task com status default todo e created_by da sessão (CA-16).
// Devolve um *AssignNotify quando a task nasce atribuída a um TERCEIRO (fase 18,
// CA-13/14) — o handler dispara o aviso pós-commit.
func (s *Service) Create(ctx context.Context, workspaceID, userID, role string, in CreateInput) (Task, *AssignNotify, error) {
	name, err := validName(in.Name)
	if err != nil {
		return Task{}, nil, err
	}
	if err := validDueDate(in.DueDate); err != nil {
		return Task{}, nil, err
	}
	if err := validRecurrence(in.Recurrence); err != nil {
		return Task{}, nil, err
	}
	if err := validTZ(in.TZ); err != nil { // fase 20: tz inválido → 400 antes da escrita
		return Task{}, nil, err
	}
	// fase 24: status/priority/notion_url validados ANTES do SQL (D-07).
	if in.Status != nil {
		if err := validStatus(*in.Status); err != nil {
			return Task{}, nil, err
		}
	}
	if err := validPriority(in.Priority); err != nil {
		return Task{}, nil, err
	}
	notionURL, err := normalizeNotionURL(in.NotionURL)
	if err != nil {
		return Task{}, nil, err
	}
	if err := s.loadProjectGate(ctx, workspaceID, userID, role, in.ProjectID, true); err != nil {
		return Task{}, nil, err
	}

	// fase 24: status efetivo (nil → 'todo', o default da coluna — CA-02) e
	// priority efetivo (nil → 0). position = max+1 da coluna (project, status) — D-03.
	status := "todo"
	if in.Status != nil {
		status = *in.Status
	}
	priority := 0
	if in.Priority != nil {
		priority = *in.Priority
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Task{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := validateObjectiveRef(ctx, tx, workspaceID, in.ProjectID, in.ObjectiveID); err != nil {
		return Task{}, nil, err
	}
	if err := validateMember(ctx, tx, workspaceID, in.Assignee); err != nil {
		return Task{}, nil, err
	}
	if err := validateMember(ctx, tx, workspaceID, in.RequestedBy); err != nil {
		return Task{}, nil, err
	}
	position, err := nextPosition(ctx, tx, in.ProjectID, status)
	if err != nil {
		return Task{}, nil, err
	}
	t, err := scanTask(tx.QueryRow(ctx, `
		INSERT INTO tasks (workspace_id, project_id, objective_id, name, status, priority, notion_url, position, due_date, recurrence, notes, created_by, assignee, requested_by)
		VALUES ($1, $2, $3, $4, $5::task_status, $6, $7, $8, $9::date, $10, $11, $12, $13, $14)
		RETURNING `+taskCols(""),
		workspaceID, in.ProjectID, in.ObjectiveID, name, status, priority, notionURL, position,
		in.DueDate, in.Recurrence, in.Notes, userID, in.Assignee, in.RequestedBy))
	if err != nil {
		return Task{}, nil, err
	}
	// task nova entra no objetivo → recalcula (CA-06; D-04: criação é transição).
	// userID = quem cria → bucket de horas per-usuário do by_hours (fase 29 / decisão E).
	if err := recalcObjectiveStatus(ctx, tx, in.ObjectiveID, userID); err != nil {
		return Task{}, nil, err
	}
	// fase 20: a task nasce com due_date setada → cria o reminder due na MESMA tx
	// (gatilho = due_date != nil no Create — CA-11). NO-OP por default (RNF-05).
	if in.DueDate != nil {
		if err := s.reminder.OnDueDateSet(ctx, tx, t.ID, userID, t.DueDate, in.TZ); err != nil {
			return Task{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, nil, err
	}
	// notificação pós-commit (CA-13): old assignee era nil (task nova).
	return t, assignNotify(nil, t.Assignee, userID, t.ID), nil
}

// List devolve as tasks do workspace com filtros opcionais e combináveis
// (CA-19); tasks herdam a visibilidade do projeto (CA-22) — private de
// outro usuário fica fora da lista.
func (s *Service) List(ctx context.Context, workspaceID, userID string, projectID, status, objectiveID *string) ([]Task, error) {
	if projectID != nil && uuid.Validate(*projectID) != nil {
		return nil, domain.Validation("project_id inválido")
	}
	if status != nil {
		if err := validStatus(*status); err != nil {
			return nil, err
		}
	}
	// objective_id malformado no FILTRO → validation_error (CA-10; no payload é
	// invalid_reference — ver validateObjectiveRef).
	if objectiveID != nil && uuid.Validate(*objectiveID) != nil {
		return nil, domain.Validation("objective_id inválido")
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+taskCols(`t.`)+` FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1
		   AND ($3::uuid IS NULL OR t.project_id = $3)
		   AND ($4::task_status IS NULL OR t.status = $4)
		   AND ($5::uuid IS NULL OR t.objective_id = $5)
		 ORDER BY t.created_at, t.id`, workspaceID, userID, projectID, status, objectiveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// Get carrega o detalhe da task; em projeto private de outro usuário a
// resposta é o MESMO 404 de inexistente (CA-22).
func (s *Service) Get(ctx context.Context, workspaceID, userID, taskID string) (Task, error) {
	if uuid.Validate(taskID) != nil {
		return Task{}, domain.NotFound("task não encontrada")
	}
	t, err := scanTask(s.db.QueryRow(ctx, `
		SELECT `+taskCols(`t.`)+` FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $3`, workspaceID, userID, taskID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, domain.NotFound("task não encontrada")
	}
	return t, err
}

// Patch edita a task (CA-18/20); o gate do projeto bloqueia arquivado
// (CA-15.2). Devolve um *AssignNotify quando o assignee muda pra um TERCEIRO
// (fase 18, CA-13/14/15).
func (s *Service) Patch(ctx context.Context, workspaceID, userID, role, taskID string, in PatchInput) (Task, *AssignNotify, error) {
	if uuid.Validate(taskID) != nil { // espelha s.Get: uuid malformado → 404 antes do SQL
		return Task{}, nil, domain.NotFound("task não encontrada")
	}
	if err := validTZ(in.TZ); err != nil { // fase 20: tz inválido → 400 antes da escrita
		return Task{}, nil, err
	}

	// Todo o PATCH roda numa transação com SELECT … FOR UPDATE na linha da task:
	// serializa PATCHes parciais concorrentes na MESMA task. O popup da fase 24
	// dispara um PATCH POR CAMPO (nome/status/assignee/…) — sem o lock, o
	// read-modify-write da linha inteira perde update (o 2º PATCH lê o estado
	// ANTES do 1º commitar e regrava o campo do 1º com o valor velho; ex.:
	// requested_by zerava o assignee recém-setado → a task sumia de /minhas-tarefas).
	// FOR UPDATE OF t trava só a linha da task; a visibilidade herda do projeto
	// (private de outro → ErrNoRows → 404, igual ao s.Get).
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Task{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	t, err := scanTask(tx.QueryRow(ctx, `
		SELECT `+taskCols(`t.`)+` FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $3
		 FOR UPDATE OF t`, workspaceID, userID, taskID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, nil, domain.NotFound("task não encontrada")
	}
	if err != nil {
		return Task{}, nil, err
	}
	// Carve-out viewer-assignee (fase 18, D-03): o viewer chega aqui SÓ via
	// PATCH de task (o middleware abriu o buraco). O service é a autoridade
	// final: o viewer só pode mexer no STATUS da task ONDE É o assignee, e em
	// nenhum outro campo (CA-10/11). NÃO chama loadProjectGate (sua autoridade é
	// "ser o assignee", não can_write), MAS herda o bloqueio de projeto
	// arquivado (CA-10.1) — o mesmo 400 que owner/member levam.
	if role == "viewer" {
		if err := s.checkViewerAssigneeGate(ctx, t, userID, in); err != nil {
			return Task{}, nil, err
		}
	} else if err := s.loadProjectGate(ctx, workspaceID, userID, role, t.ProjectID, true); err != nil {
		return Task{}, nil, err
	}

	oldStatus := t.Status
	oldObjectiveID := t.ObjectiveID
	oldAssignee := t.Assignee
	oldRequestedBy := t.RequestedBy
	oldDueDate := t.DueDate // fase 20: gatilho do reminder due = due_date mudou de valor

	if in.Name != nil {
		if t.Name, err = validName(*in.Name); err != nil {
			return Task{}, nil, err
		}
	}
	if in.Status != nil {
		if err := validStatus(*in.Status); err != nil {
			return Task{}, nil, err
		}
		t.Status = *in.Status
	}
	// fase 24: campos BENIGNOS — priority/notion_url/position NÃO disparam recálculo
	// de objetivo nem hook de reminder (D-04); só editam a linha.
	if in.PrioritySet {
		if err := validPriority(in.Priority); err != nil {
			return Task{}, nil, err
		}
		if in.Priority != nil {
			t.Priority = *in.Priority
		}
	}
	if in.NotionURLSet {
		normalized, err := normalizeNotionURL(in.NotionURL)
		if err != nil {
			return Task{}, nil, err
		}
		t.NotionURL = normalized
	}
	if in.PositionSet {
		if err := validPosition(in.Position); err != nil {
			return Task{}, nil, err
		}
		if in.Position != nil {
			t.Position = *in.Position
		}
	}
	if in.DueDateSet {
		if err := validDueDate(in.DueDate); err != nil {
			return Task{}, nil, err
		}
		t.DueDate = in.DueDate
	}
	if in.RecurrenceSet {
		if err := validRecurrence(in.Recurrence); err != nil {
			return Task{}, nil, err
		}
		t.Recurrence = in.Recurrence
	}
	if in.Notes != nil {
		t.Notes = *in.Notes
	}
	if in.ObjectiveIDSet {
		t.ObjectiveID = in.ObjectiveID
	}
	if in.AssigneeSet {
		t.Assignee = in.Assignee
	}
	if in.RequestedBySet {
		t.RequestedBy = in.RequestedBy
	}

	// Valida membership SÓ quando o assignee/requested_by MUDA pra um novo valor
	// (CA-04). Reenviar o valor ATUAL (o TaskModal sempre reenvia os dois campos)
	// NÃO revalida — senão editar um campo qualquer de uma task atribuída a quem
	// SAIU do workspace (fase 17 RemoveMember: some de workspace_members, mas
	// tasks.assignee segue apontando pro users row) falharia com 400. Limpar (nil)
	// também não valida. Read-only → roda no pool; nada inválido chega ao UPDATE.
	if in.AssigneeSet && in.Assignee != nil && !ptrEqual(in.Assignee, oldAssignee) {
		if err := validateMember(ctx, tx, workspaceID, in.Assignee); err != nil {
			return Task{}, nil, err
		}
	}
	if in.RequestedBySet && in.RequestedBy != nil && !ptrEqual(in.RequestedBy, oldRequestedBy) {
		if err := validateMember(ctx, tx, workspaceID, in.RequestedBy); err != nil {
			return Task{}, nil, err
		}
	}

	// notificação pós-commit (CA-13/14/15): só se o assignee EFETIVO mudou pra um
	// terceiro. Calculado do old/new; emitido após o commit de cada ramo.
	notify := assignNotify(oldAssignee, t.Assignee, userID, t.ID)

	const updateSQL = `
		UPDATE tasks
		   SET name = $3, status = $4, due_date = $5::date, recurrence = $6, notes = $7,
		       objective_id = $8, assignee = $9, requested_by = $10,
		       priority = $11, notion_url = $12, position = $13
		 WHERE workspace_id = $1 AND id = $2
		RETURNING `

	statusChanged := t.Status != oldStatus
	objectiveChanged := !ptrEqual(t.ObjectiveID, oldObjectiveID)
	// fase 20 (CA-12.1): o reminder due nasce/cancela quando a due_date MUDA DE
	// VALOR (não por DueDateSet — re-setar a MESMA data é NO-OP, preserva o due
	// `sent`). null↔null e X↔X = false; X↔null, null↔X, X↔Y = true.
	dueChanged := in.DueDateSet && !ptrEqual(t.DueDate, oldDueDate)

	// Edição BENIGNA (sem mudar status nem objetivo nem a due_date de valor):
	// UPDATE simples, SEM recálculo (D-04) — preserva o fecho/reabertura manual do
	// objetivo (CA-06.1). Se SÓ a due_date mudou de valor, NÃO é mais benigna:
	// precisa de tx pro hook do reminder (D-02/§9 — senão o due não nasceria).
	if !statusChanged && !objectiveChanged && !dueChanged {
		updated, err := scanTask(tx.QueryRow(ctx, updateSQL+taskCols(""),
			workspaceID, taskID, t.Name, t.Status, t.DueDate, t.Recurrence, t.Notes,
			t.ObjectiveID, t.Assignee, t.RequestedBy, t.Priority, t.NotionURL, t.Position))
		if err != nil {
			return Task{}, nil, err
		}
		if err := tx.Commit(ctx); err != nil { // libera o FOR UPDATE do topo
			return Task{}, nil, err
		}
		return updated, notify, nil
	}

	// Transição (status, objetivo e/ou due_date mudaram): valida o novo vínculo,
	// grava, dispara o hook do reminder e recalcula os objetivos afetados na MESMA
	// transação já aberta no topo (com o FOR UPDATE da linha da task — sem segunda
	// Begin, que abandonaria o lock; CA-06/12/13/D-09).
	if objectiveChanged && t.ObjectiveID != nil {
		if err := validateObjectiveRef(ctx, tx, workspaceID, t.ProjectID, t.ObjectiveID); err != nil {
			return Task{}, nil, err
		}
	}
	updated, err := scanTask(tx.QueryRow(ctx, updateSQL+taskCols(""),
		workspaceID, taskID, t.Name, t.Status, t.DueDate, t.Recurrence, t.Notes,
		t.ObjectiveID, t.Assignee, t.RequestedBy, t.Priority, t.NotionURL, t.Position))
	if err != nil {
		return Task{}, nil, err
	}
	// fase 20: a due_date mudou de valor → cancela o due antigo e cria o novo (se
	// não-nula) na MESMA tx, depois do UPDATE (a task já está gravada com o
	// assignee/due_date correntes — CA-11/12/13/14). NO-OP por default (RNF-05).
	if dueChanged {
		if err := s.reminder.OnDueDateSet(ctx, tx, updated.ID, userID, updated.DueDate, in.TZ); err != nil {
			return Task{}, nil, err
		}
	}
	// Recalcula os objetivos afetados (destino + origem no move A→B), dedup e
	// EM ORDEM DE ID: cada recálculo trava a linha do objetivo (FOR UPDATE); a
	// ordem fixa garante que dois moves concorrentes do mesmo par travem na mesma
	// sequência → sem deadlock. SÓ recalcula se status/objetivo mudaram: uma
	// mudança SÓ de due_date (fase 20) entrou nesta tx pelo hook do reminder, mas
	// NÃO altera a composição do objetivo → recalcular regrediria o fecho manual
	// (CA-06.1). dueChanged sozinho não recalcula.
	affected := map[string]struct{}{}
	if statusChanged || objectiveChanged {
		if t.ObjectiveID != nil {
			affected[*t.ObjectiveID] = struct{}{} // objetivo atual (status e/ou recém-recebido)
		}
		if objectiveChanged && oldObjectiveID != nil {
			affected[*oldObjectiveID] = struct{}{} // origem do move também muda de composição
		}
	}
	ids := make([]string, 0, len(affected))
	for id := range affected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i := range ids {
		if err := recalcObjectiveStatus(ctx, tx, &ids[i], userID); err != nil {
			return Task{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, nil, err
	}
	return updated, notify, nil
}

// Delete remove a task DEFINITIVAMENTE (hard delete — CA-21); exige
// permissão de escrita (CA-16.1); arquivado NÃO bloqueia (CA-15.2 cobre
// só POST/PATCH).
func (s *Service) Delete(ctx context.Context, workspaceID, userID, role, taskID string) error {
	t, err := s.Get(ctx, workspaceID, userID, taskID)
	if err != nil {
		return err
	}
	if err := s.loadProjectGate(ctx, workspaceID, userID, role, t.ProjectID, false); err != nil {
		return err
	}
	// task sem objetivo: delete simples (nada pra recalcular).
	if t.ObjectiveID == nil {
		_, err = s.db.Exec(ctx,
			`DELETE FROM tasks WHERE workspace_id = $1 AND id = $2`, workspaceID, taskID)
		return err
	}
	// task de um objetivo: remover muda a composição → recalcula na MESMA tx (D-04/D-09).
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		`DELETE FROM tasks WHERE workspace_id = $1 AND id = $2`, workspaceID, taskID); err != nil {
		return err
	}
	if err := recalcObjectiveStatus(ctx, tx, t.ObjectiveID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
