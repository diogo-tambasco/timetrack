// service.go — CRUD de entradas manuais (T-12, CA-14/15/16/24/24.1/24.2).
// Mutações rodam em transação serializada pelo MESMO advisory lock do
// timer.Service (hashtext(user_id)): o status "rodando" de uma entry não
// muda no meio de um PATCH/DELETE.
package entries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Service expõe o CRUD de entries por cima do repositório. Construir com
// NewService; clock injetável pros testes (recorte "hoje" usa s.now).
type Service struct {
	db  *pgxpool.Pool
	now func() time.Time
}

// ServiceOption ajusta o Service na construção.
type ServiceOption func(*Service)

// WithClock injeta o relógio (testes); default time.Now.
func WithClock(now func() time.Time) ServiceOption {
	return func(s *Service) { s.now = now }
}

// NewService cria o serviço com clock real.
func NewService(db *pgxpool.Pool, opts ...ServiceOption) *Service {
	s := &Service{db: db, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// lockUserTx serializa as mutações de entry com as do timer.Service (mesma
// chave de advisory lock — ver timer.lockUser).
func lockUserTx(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, userID)
	return err
}

// CreateInput é o payload do POST de entrada manual (plan §3.6). Não há
// duration_seconds: valor enviado pelo cliente é descartado no decode e a
// duração é sempre calculada no servidor (CA-02).
type CreateInput struct {
	Description string     `json:"description"`
	ProjectID   *string    `json:"project_id"`
	TaskID      *string    `json:"task_id"`
	Tags        []string   `json:"tags"`
	StartedAt   *time.Time `json:"started_at"`
	StoppedAt   *time.Time `json:"stopped_at"`
}

// Create persiste uma entrada manual fechada (CA-14): horários validados
// (CA-24), refs do workspace validadas (CA-29) e duração do servidor.
func (s *Service) Create(ctx context.Context, workspaceID, userID string, in CreateInput) (Entry, error) {
	if in.StartedAt == nil || in.StoppedAt == nil {
		return Entry{}, domain.Validation("started_at e stopped_at são obrigatórios")
	}
	if !in.StoppedAt.After(*in.StartedAt) {
		return Entry{}, domain.Validation("stopped_at deve ser depois de started_at")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Entry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ValidateRefs(ctx, tx, workspaceID, in.ProjectID, in.TaskID); err != nil {
		return Entry{}, err
	}

	started := in.StartedAt.UTC()
	stopped := in.StoppedAt.UTC()
	dur := int(stopped.Sub(started).Seconds())
	e, err := Insert(ctx, tx, Entry{
		WorkspaceID:     workspaceID,
		UserID:          userID, // SEMPRE o usuário da sessão (CA-03.1)
		ProjectID:       in.ProjectID,
		TaskID:          in.TaskID,
		Description:     in.Description,
		Tags:            in.Tags,
		StartedAt:       started,
		StoppedAt:       &stopped,
		DurationSeconds: &dur,
		Source:          "manual",
	})
	if err != nil {
		return Entry{}, err
	}
	return e, tx.Commit(ctx)
}

// ListInput são os filtros do GET (plan §3.6): from/to em YYYY-MM-DD no
// fuso de tz; ausentes valem "hoje" nesse fuso (CA-14.1).
type ListInput struct {
	From, To, TZ string
}

// ListDay resolve a janela [from 00:00, to+1d 00:00) no fuso IANA de tz
// (ausente → UTC) e devolve as entries + total de segundos fechados.
// Filtros malformados respondem 400 validation_error (decisão da fase 02).
func (s *Service) ListDay(ctx context.Context, workspaceID, userID string, in ListInput) ([]Entry, int, error) {
	loc := time.UTC
	if in.TZ != "" {
		l, err := time.LoadLocation(in.TZ)
		if err != nil {
			return nil, 0, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	today := s.now().In(loc).Format("2006-01-02")
	from, to := in.From, in.To
	if from == "" {
		from = today
	}
	if to == "" {
		to = today
	}
	fromT, err := time.ParseInLocation("2006-01-02", from, loc)
	if err != nil {
		return nil, 0, domain.Validation("from deve ser uma data YYYY-MM-DD")
	}
	toT, err := time.ParseInLocation("2006-01-02", to, loc)
	if err != nil {
		return nil, 0, domain.Validation("to deve ser uma data YYYY-MM-DD")
	}

	list, err := List(ctx, s.db, workspaceID, userID, fromT, toT.AddDate(0, 0, 1))
	if err != nil {
		return nil, 0, err
	}
	total := 0
	for _, e := range list {
		if e.DurationSeconds != nil {
			total += *e.DurationSeconds
		}
	}
	return list, total, nil
}

// Get carrega uma entry própria (404 pra qualquer outra — CA-29.3); é a
// porta do continue (T-13), que clona o contexto antes do timer.Start.
func (s *Service) Get(ctx context.Context, workspaceID, userID, id string) (Entry, error) {
	return GetOwned(ctx, s.db, workspaceID, userID, id)
}

// PatchInput são os campos parciais do PATCH (plan §3.6). ProjectIDSet/
// TaskIDSet distinguem "ausente" de "null" (null remove o vínculo).
type PatchInput struct {
	Description  *string
	Tags         *[]string
	ProjectIDSet bool
	ProjectID    *string
	TaskIDSet    bool
	TaskID       *string
	StartedAt    *time.Time
	StoppedAt    *time.Time
}

// Patch edita uma entry própria fechada (CA-15): horários revalidados,
// duração SEMPRE recalculada no servidor. Entry rodando → 409 entry_running
// (CA-24.1 — mudança no timer ativo passa pelas rotas /timer).
func (s *Service) Patch(ctx context.Context, workspaceID, userID, id string, in PatchInput) (Entry, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Entry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserTx(ctx, tx, userID); err != nil {
		return Entry{}, err
	}

	e, err := GetOwned(ctx, tx, workspaceID, userID, id) // 404 (CA-29.3)
	if err != nil {
		return Entry{}, err
	}
	if e.StoppedAt == nil {
		return Entry{}, domain.Conflict("entry_running",
			"a entry está rodando — pare o timer pelas rotas /timer")
	}

	if in.Description != nil {
		e.Description = *in.Description
	}
	if in.Tags != nil {
		e.Tags = *in.Tags
	}
	if in.ProjectIDSet {
		e.ProjectID = in.ProjectID
	}
	if in.TaskIDSet {
		e.TaskID = in.TaskID
	}
	if in.StartedAt != nil {
		e.StartedAt = in.StartedAt.UTC()
	}
	if in.StoppedAt != nil {
		t := in.StoppedAt.UTC()
		e.StoppedAt = &t
	}
	if !e.StoppedAt.After(e.StartedAt) {
		return Entry{}, domain.Validation("stopped_at deve ser depois de started_at")
	}
	if err := ValidateRefs(ctx, tx, workspaceID, e.ProjectID, e.TaskID); err != nil {
		return Entry{}, err
	}
	dur := int(e.StoppedAt.Sub(e.StartedAt).Seconds())
	e.DurationSeconds = &dur

	e, err = Update(ctx, tx, e)
	if err != nil {
		return Entry{}, err
	}
	return e, tx.Commit(ctx)
}

// Remove exclui uma entry própria (CA-16). Rodando → 409 entry_running
// (CA-24.1); contexto de retomada de pausa ATIVA → 409 entry_in_use
// (CA-24.2, proteção do resume).
func (s *Service) Remove(ctx context.Context, workspaceID, userID, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserTx(ctx, tx, userID); err != nil {
		return err
	}

	e, err := GetOwned(ctx, tx, workspaceID, userID, id) // 404 (CA-29.3)
	if err != nil {
		return err
	}
	if e.StoppedAt == nil {
		return domain.Conflict("entry_running",
			"a entry está rodando — pare o timer pelas rotas /timer")
	}
	var inUse bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM timer_states
		               WHERE mode = 'break' AND resume_entry_id = $1)`, e.ID).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return domain.Conflict("entry_in_use",
			"a entry é o contexto de retomada de uma pausa ativa")
	}
	if err := Delete(ctx, tx, workspaceID, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
