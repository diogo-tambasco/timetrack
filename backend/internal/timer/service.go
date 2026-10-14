// Package timer: o serviço de cronômetro da fase 03. TODA mutação passa por
// aqui, roda em transação serializada por usuário e retorna o MESMO
// TimerState canônico do GET /timer (D-05/RNF-05) — a fase 04 só assina o
// hook OnChange pra broadcast WebSocket, sem refatorar nada.
//
// O servidor é o relógio (D-01): started_at/stopped_at saem de s.now(),
// nunca do cliente. O clock é injetável pros testes.
package timer

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/entries"
)

// State é o TimerState canônico — contrato público entre as fases
// (specs/INDEX.md); timer_states é detalhe interno (D-03).
// Mode no state usa os valores da API: countup|countdown|pomodoro|break
// (o enum interno 'pomodoro_focus' vira "pomodoro" — plan §3.2).
type State struct {
	Mode             string         `json:"mode"`
	Entry            *entries.Entry `json:"entry"`
	ElapsedSeconds   int            `json:"elapsed_seconds"`
	TargetSeconds    *int           `json:"target_seconds"`
	RemainingSeconds *int           `json:"remaining_seconds"`
	Resume           *Resume        `json:"resume,omitempty"`
	// BreakKind ("short"|"long") rotula a pausa pra UI sem recalcular
	// (CA-06.1 da fase 15). É LIDO da coluna persistida break_is_long do
	// timer_states (gravada no pause), NUNCA recomputado por
	// focus_count % cycles — o override manual diverge. Vazio fora de break.
	BreakKind string `json:"break_kind,omitempty"`
}

// Resume é o contexto de retomada exposto durante a pausa (CA-22).
// Mode aqui usa o valor INTERNO ('pomodoro_focus' incluso): é o que o front
// lê pra decidir o auto-resume do ciclo pomodoro (CA-34.1, plan §3.1) —
// exceção deliberada à tradução do state.mode.
type Resume struct {
	EntryID       string `json:"entry_id"`
	Mode          string `json:"mode"`
	TargetSeconds *int   `json:"target_seconds"`
}

// Result é o retorno uniforme de toda operação (RNF-05): serializado
// direto como corpo das respostas REST.
type Result struct {
	State        *State         `json:"state"`
	StoppedEntry *entries.Entry `json:"stopped_entry,omitempty"`
	ServerTime   time.Time      `json:"server_time"`
}

// StartInput é o payload do POST /timer/start (plan §3.2). Não existem
// campos user_id nem duration_seconds: o usuário vem da sessão (CA-03.1) e
// a duração é sempre aritmética do relógio do servidor (CA-02) — valores
// extras no JSON são descartados pelo decode.
type StartInput struct {
	WorkspaceID   string   `json:"workspace_id"`
	Description   string   `json:"description"`
	ProjectID     *string  `json:"project_id"`
	TaskID        *string  `json:"task_id"`
	Tags          []string `json:"tags"`
	Mode          string   `json:"mode"`           // "" → countup
	TargetSeconds *int     `json:"target_seconds"` // exigido em countdown (CA-27)
}

// Service executa as operações do timer. Construir com NewService.
type Service struct {
	db       *pgxpool.Pool
	now      func() time.Time
	onChange func(userID string, state *State, at time.Time)
}

// Option ajusta o Service na construção.
type Option func(*Service)

// WithClock injeta o relógio (testes); default time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService cria o serviço com clock real e OnChange no-op.
func NewService(db *pgxpool.Pool, opts ...Option) *Service {
	s := &Service{
		db:       db,
		now:      time.Now,
		onChange: func(string, *State, time.Time) {}, // no-op até o boot plugar o hub WS
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// SetOnChange registra o hook chamado após TODA mutação COMMITADA com o
// estado novo (nil = ocioso) e o instante DA MUTAÇÃO — o mesmo now que
// vira server_time no Result (D-04 da fase 04): é ele que o apply() do
// front usa pra ordenar WS × REST. O hub WebSocket pluga aqui (RF-05).
func (s *Service) SetOnChange(fn func(userID string, state *State, at time.Time)) {
	s.onChange = fn
}

// apiModeOf traduz o enum interno timer_mode pro contrato da API:
// 'pomodoro_focus' nunca vaza como state.mode — vira "pomodoro" (plan §3.2).
func apiModeOf(dbMode string) string {
	if dbMode == "pomodoro_focus" {
		return "pomodoro"
	}
	return dbMode
}

// breakKindOf mapeia a coluna persistida break_is_long pro rótulo do
// TimerState (CA-06.1 — lido, nunca recomputado).
func breakKindOf(isLong bool) string {
	if isLong {
		return "long"
	}
	return "short"
}

// lockUser serializa as mutações de timer do usuário na transação: 2
// operações concorrentes não entrelaçam (o unique parcial do banco é o
// backstop do CA-03; o lock evita o erro chegar ao usuário).
func lockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, userID)
	return err
}

// stateRow é a linha de timer_states (detalhe interno — D-03).
// focus_count/break_is_long são runtime do ciclo Pomodoro 2.0 (fase 15):
// focus_count = focos completos desde a última pausa longa/reset;
// break_is_long = kind da pausa atual PERSISTIDO (CA-06.1, não recomputar).
type stateRow struct {
	Mode                string
	EntryID             *string
	TargetSeconds       *int
	StartedAt           time.Time
	ResumeEntryID       *string
	ResumeMode          *string
	ResumeTargetSeconds *int
	FocusCount          int
	BreakIsLong         bool
}

func loadStateRow(ctx context.Context, q entries.Querier, userID string, forUpdate bool) (stateRow, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var row stateRow
	err := q.QueryRow(ctx, `
		SELECT mode, entry_id, target_seconds, started_at,
		       resume_entry_id, resume_mode, resume_target_seconds,
		       focus_count, break_is_long
		  FROM timer_states WHERE user_id = $1`+suffix, userID).
		Scan(&row.Mode, &row.EntryID, &row.TargetSeconds, &row.StartedAt,
			&row.ResumeEntryID, &row.ResumeMode, &row.ResumeTargetSeconds,
			&row.FocusCount, &row.BreakIsLong)
	return row, err
}

// workState monta o TimerState de um modo de trabalho: elapsed/remaining
// são sempre aritmética do relógio do servidor (CA-21), remaining nunca
// negativo (CA-12).
func workState(apiMode string, e *entries.Entry, target *int, startedAt, now time.Time) *State {
	elapsed := int(now.Sub(startedAt).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	st := &State{Mode: apiMode, Entry: e, ElapsedSeconds: elapsed, TargetSeconds: target}
	if target != nil {
		rem := *target - elapsed
		if rem < 0 {
			rem = 0
		}
		st.RemainingSeconds = &rem
	}
	return st
}

// closeEntry fecha a entry rodando com stopped_at = relógio do servidor e
// duration_seconds calculado NO BANCO a partir do started_at persistido
// (CA-02/CA-06). Overrun não clampa ao alvo (CA-13.1).
func closeEntry(ctx context.Context, q entries.Querier, entryID string, now time.Time) (entries.Entry, error) {
	var e entries.Entry
	err := q.QueryRow(ctx, `
		UPDATE time_entries
		   SET stopped_at = $2,
		       duration_seconds = floor(extract(epoch FROM ($2::timestamptz - started_at)))::int
		 WHERE id = $1 AND stopped_at IS NULL
		RETURNING id, workspace_id, user_id, project_id, task_id, description,
		          tags, started_at, stopped_at, duration_seconds, source`, entryID, now).
		Scan(&e.ID, &e.WorkspaceID, &e.UserID, &e.ProjectID, &e.TaskID,
			&e.Description, &e.Tags, &e.StartedAt, &e.StoppedAt, &e.DurationSeconds, &e.Source)
	return e, err
}

// closeRunning fecha a entry rodando do usuário, se houver (CA-05 — a
// troca acontece na MESMA transação do start novo). Sem entry rodando,
// devolve nil sem erro.
func closeRunning(ctx context.Context, q entries.Querier, userID string, now time.Time) (*entries.Entry, error) {
	var id string
	err := q.QueryRow(ctx, `
		SELECT id FROM time_entries
		 WHERE user_id = $1 AND stopped_at IS NULL FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e, err := closeEntry(ctx, q, id, now)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Start inicia um timer pro usuário da sessão (CA-04/12/13/20/27/29).
// Modos: countup (default) · countdown (alvo do payload) · pomodoro
// (alvo = focus_minutes persistido; vira 'pomodoro_focus' no banco).
func (s *Service) Start(ctx context.Context, userID string, in StartInput) (Result, error) {
	apiMode := in.Mode
	if apiMode == "" {
		apiMode = "countup"
	}
	dbMode := apiMode
	var target *int
	switch apiMode {
	case "countup":
	case "countdown":
		// CA-27: countdown exige alvo positivo — nada é criado sem ele
		if in.TargetSeconds == nil || *in.TargetSeconds <= 0 {
			return Result{}, domain.Validation("countdown exige target_seconds maior que zero")
		}
		target = in.TargetSeconds
	case "pomodoro":
		dbMode = "pomodoro_focus" // nunca gravar 'pomodoro' no banco (plan §3.2)
	default:
		return Result{}, domain.Validation("mode deve ser countup, countdown ou pomodoro")
	}
	now := s.now().UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUser(ctx, tx, userID); err != nil {
		return Result{}, err
	}
	if apiMode == "pomodoro" {
		cfg, err := loadFullSettings(ctx, tx, userID)
		if err != nil {
			return Result{}, err
		}
		t := cfg.FocusMinutes * 60 // CA-13
		target = &t
	}
	// CA-29: refs de outro workspace = not_found, nada é criado
	if err := entries.ValidateRefs(ctx, tx, in.WorkspaceID, in.ProjectID, in.TaskID); err != nil {
		return Result{}, err
	}
	// CA-05: timer já rodando → fecha o atual e inicia o novo na MESMA
	// transação (tudo ou nada)
	if _, err := closeRunning(ctx, tx, userID, now); err != nil {
		return Result{}, err
	}

	e, err := entries.Insert(ctx, tx, entries.Entry{
		WorkspaceID: in.WorkspaceID,
		UserID:      userID, // SEMPRE o usuário da sessão (CA-03.1)
		ProjectID:   in.ProjectID,
		TaskID:      in.TaskID,
		Description: in.Description,
		Tags:        in.Tags,
		StartedAt:   now,
		Source:      "timer",
	})
	if err != nil {
		return Result{}, err
	}
	// CA-07: novo start começa o ciclo do zero (focus_count=0,
	// break_is_long=false) — espelha o reset de resume_*.
	if _, err := tx.Exec(ctx, `
		INSERT INTO timer_states (user_id, mode, entry_id, target_seconds, started_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		   SET mode = excluded.mode, entry_id = excluded.entry_id,
		       target_seconds = excluded.target_seconds, started_at = excluded.started_at,
		       resume_entry_id = NULL, resume_mode = NULL, resume_target_seconds = NULL,
		       focus_count = 0, break_is_long = false`,
		userID, dbMode, e.ID, target, now); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	st := workState(apiMode, &e, target, now, now)
	s.onChange(userID, st, now)
	return Result{State: st, ServerTime: now}, nil
}

// EditCurrentInput edita SÓ rótulo/projeto/tags da entry RODANDO (allowance
// da decisão HITL 23/06 — fase 27/27-H1). NÃO há campos de tempo:
// started_at/stopped_at/duration/mode/target são imutáveis numa entry em
// andamento (CA-13.1). ProjectIDSet distingue "ausente" de "null" (null
// remove o vínculo), como entries.PatchInput.
type EditCurrentInput struct {
	Description  *string
	Tags         *[]string
	ProjectIDSet bool
	ProjectID    *string
}

// EditCurrent edita a entry RODANDO do usuário da sessão (fase 27/CA-13).
// Sob o advisory lock do timer; carrega a entry de TRABALHO (mode != 'break')
// com FOR UPDATE; valida o project_id novo via entries.ValidateRefs no
// WORKSPACE da entry; UPDATE só de description/project_id/tags (timing
// INTOCADO — não recalcula duration, não toca started_at/stopped_at);
// dispara s.onChange(State novo) → broadcast da fase 04. Sem entry de
// trabalho rodando (ocioso OU durante 'break') → 409 no_active_timer.
// project de outro ws/inexistente → 404 not_found.
func (s *Service) EditCurrent(ctx context.Context, userID string, in EditCurrentInput) (Result, error) {
	now := s.now().UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUser(ctx, tx, userID); err != nil {
		return Result{}, err
	}

	row, err := loadStateRow(ctx, tx, userID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, domain.Conflict("no_active_timer", "não há timer rodando")
	}
	if err != nil {
		return Result{}, err
	}
	if row.Mode == "break" {
		// durante a pausa não há entry de trabalho aberta pra editar
		return Result{}, domain.Conflict("no_active_timer", "não há timer rodando")
	}

	e, err := entries.GetByID(ctx, tx, *row.EntryID)
	if err != nil {
		return Result{}, err
	}

	// aplica só os campos editáveis (tri-state no project_id) — timing intocado
	if in.Description != nil {
		e.Description = *in.Description
	}
	if in.Tags != nil {
		e.Tags = *in.Tags
	}
	if in.ProjectIDSet {
		e.ProjectID = in.ProjectID
	}

	// CA-13.2: project_id novo precisa pertencer ao workspace DA ENTRY (não
	// vaza existência — mesmo not_found de inexistente); task_id segue como
	// está (não é editável aqui, mas revalidar é barato e mantém o invariante)
	if err := entries.ValidateRefs(ctx, tx, e.WorkspaceID, e.ProjectID, e.TaskID); err != nil {
		return Result{}, err
	}

	// UPDATE com filtro duplo workspace+user; started_at/stopped_at/duration
	// passam IGUAIS (a entry rodando tem stopped_at=nil; não se mexe nele)
	e, err = entries.Update(ctx, tx, e)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	// elapsed/remaining derivam do relógio do servidor (nada do tempo muda)
	st := workState(apiModeOf(row.Mode), &e, row.TargetSeconds, row.StartedAt, now)
	s.onChange(userID, st, now)
	return Result{State: st, ServerTime: now}, nil
}

// Stop para o timer rodando: fecha a entry com a duração do relógio do
// servidor e deixa o estado ocioso (CA-06).
func (s *Service) Stop(ctx context.Context, userID string) (Result, error) {
	now := s.now().UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUser(ctx, tx, userID); err != nil {
		return Result{}, err
	}

	row, err := loadStateRow(ctx, tx, userID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, domain.Conflict("no_active_timer", "não há timer nem pausa ativa")
	}
	if err != nil {
		return Result{}, err
	}

	var stopped *entries.Entry
	if row.Mode != "break" {
		e, err := closeEntry(ctx, tx, *row.EntryID, now)
		if err != nil {
			return Result{}, err
		}
		stopped = &e
	}
	// durante a pausa, stop só encerra a pausa — sem retomar nem criar
	// entry (CA-06.1); a entry pausada já foi fechada no pause
	if _, err := tx.Exec(ctx,
		`DELETE FROM timer_states WHERE user_id = $1`, userID); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	s.onChange(userID, nil, now)
	return Result{State: nil, StoppedEntry: stopped, ServerTime: now}, nil
}

// Pause é o comportamento central do produto (CA-07/08/25/25.1): fecha a
// entry de trabalho e emenda AUTOMATICAMENTE na pausa Pomodoro, guardando
// em resume_* o contexto pra retomar (entry, modo e restante do alvo).
// breakMinutes do payload vale SÓ nesta pausa (CA-08); nil usa a config
// persistida do usuário.
func (s *Service) Pause(ctx context.Context, userID string, breakMinutes *int) (Result, error) {
	// CA-25.1: faixa validada ANTES de tocar em qualquer coisa — inválido
	// não fecha entry nenhuma
	if breakMinutes != nil && (*breakMinutes < 1 || *breakMinutes > 60) {
		return Result{}, domain.Validation("break_minutes deve estar entre 1 e 60")
	}
	now := s.now().UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUser(ctx, tx, userID); err != nil {
		return Result{}, err
	}

	row, err := loadStateRow(ctx, tx, userID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, domain.Conflict("no_active_timer", "não há timer rodando")
	}
	if err != nil {
		return Result{}, err
	}
	if row.Mode == "break" {
		// pausar uma pausa não existe: não há timer de TRABALHO rodando
		return Result{}, domain.Conflict("no_active_timer", "não há timer rodando")
	}

	cfg, err := loadFullSettings(ctx, tx, userID)
	if err != nil {
		return Result{}, err
	}

	// CA-05: só foco Pomodoro conta pro ciclo — incrementa o contador de
	// focos completos. countup/countdown não mexem no contador.
	newCount := row.FocusCount
	if row.Mode == "pomodoro_focus" {
		newCount++
	}

	// CA-06/06.1: escolha curta×longa + kind PERSISTIDO (break_is_long).
	// - override manual (breakMinutes no payload) = SEMPRE curta (CA-08), kind short;
	// - foco no múltiplo de cycles_before_long (newCount>0 && newCount%cycles==0) → LONGA;
	// - senão → curta.
	var brkSeconds int
	var breakIsLong bool
	switch {
	case breakMinutes != nil:
		brkSeconds = *breakMinutes * 60 // override só desta pausa (CA-08)
		breakIsLong = false             // override é sempre curta, manual
	case row.Mode == "pomodoro_focus" && newCount > 0 && newCount%cfg.CyclesBeforeLong == 0:
		brkSeconds = cfg.LongBreakMinutes * 60 // pausa longa (CA-06)
		breakIsLong = true
	default:
		brkSeconds = cfg.BreakMinutes * 60 // pausa curta (CA-07)
		breakIsLong = false
	}

	stopped, err := closeEntry(ctx, tx, *row.EntryID, now)
	if err != nil {
		return Result{}, err
	}

	// restante do alvo pra retomada (CA-10): countdown/pomodoro_focus
	// guardam target − elapsed (nunca negativo); countup não tem alvo
	var resumeTarget *int
	if row.TargetSeconds != nil {
		rem := *row.TargetSeconds - int(now.Sub(row.StartedAt).Seconds())
		if rem < 0 {
			rem = 0
		}
		resumeTarget = &rem
	}
	// grava o contador do ciclo E o kind da pausa (CA-06.1 — persistido,
	// lido no GET/resume sem recomputar).
	if _, err := tx.Exec(ctx, `
		UPDATE timer_states
		   SET mode = 'break', entry_id = NULL, target_seconds = $2, started_at = $3,
		       resume_entry_id = $4, resume_mode = $5, resume_target_seconds = $6,
		       focus_count = $7, break_is_long = $8
		 WHERE user_id = $1`,
		userID, brkSeconds, now, stopped.ID, row.Mode, resumeTarget,
		newCount, breakIsLong); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	st := workState("break", nil, &brkSeconds, now, now)
	st.Resume = &Resume{EntryID: stopped.ID, Mode: row.Mode, TargetSeconds: resumeTarget}
	st.BreakKind = breakKindOf(breakIsLong)
	s.onChange(userID, st, now)
	return Result{State: st, StoppedEntry: &stopped, ServerTime: now}, nil
}

// Resume encerra a pausa e retoma o trabalho criando uma entry NOVA
// clonada da pausada (CA-09), restaurando modo e restante do alvo (CA-10).
func (s *Service) Resume(ctx context.Context, userID string) (Result, error) {
	return s.resumeFromBreak(ctx, userID)
}

// SkipBreak encerra a pausa IMEDIATAMENTE (sem esperar zerar) e retoma o
// trabalho — mesma clonagem do Resume (CA-11). A diferença é só de
// intenção na UI; no domínio as duas operações são idênticas.
func (s *Service) SkipBreak(ctx context.Context, userID string) (Result, error) {
	return s.resumeFromBreak(ctx, userID)
}

// resumeFromBreak é o caminho comum de resume/skip: valida a pausa, clona
// a entry referida em resume_entry_id e devolve o estado de trabalho.
func (s *Service) resumeFromBreak(ctx context.Context, userID string) (Result, error) {
	now := s.now().UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUser(ctx, tx, userID); err != nil {
		return Result{}, err
	}

	row, err := loadStateRow(ctx, tx, userID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, domain.Conflict("no_active_break", "não há pausa ativa")
	}
	if err != nil {
		return Result{}, err
	}
	if row.Mode != "break" {
		return Result{}, domain.Conflict("no_active_break", "não há pausa ativa")
	}
	if row.ResumeEntryID == nil {
		// defesa em profundidade: a entry de contexto sumiu por fora da API
		// (o caminho normal é o 409 entry_in_use do DELETE — CA-24.2);
		// pausa órfã morre e o usuário recomeça com um start
		if _, err := tx.Exec(ctx,
			`DELETE FROM timer_states WHERE user_id = $1`, userID); err != nil {
			return Result{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Result{}, err
		}
		// CA-07 da fase 04 (evento ⟺ commit): a remoção COMMITOU — os
		// outros devices precisam saber que a pausa morreu, mesmo com o
		// 409 na resposta. NÃO remover este emit "corrigindo" pelo erro.
		s.onChange(userID, nil, now)
		return Result{}, domain.Conflict("no_active_break", "o contexto de retomada não existe mais")
	}

	src, err := entries.GetByID(ctx, tx, *row.ResumeEntryID)
	if err != nil {
		return Result{}, err
	}
	// clona os 4 campos de contexto (CA-09); horários são sempre novos
	e, err := entries.Insert(ctx, tx, entries.Entry{
		WorkspaceID: src.WorkspaceID,
		UserID:      userID,
		ProjectID:   src.ProjectID,
		TaskID:      src.TaskID,
		Description: src.Description,
		Tags:        src.Tags,
		StartedAt:   now,
		Source:      "timer",
	})
	if err != nil {
		return Result{}, err
	}

	resumeMode := "countup"
	if row.ResumeMode != nil {
		resumeMode = *row.ResumeMode
	}
	target := row.ResumeTargetSeconds // restante do alvo (CA-10); nil em countup
	// ciclo pomodoro canônico (US-06): foco que rodou até o fim guarda
	// restante 0 — retomar com alvo 0 degenerava num loop foco-de-1s ↔
	// pausa-cheia (auto-pause do CA-34 redisparava na hora). O novo foco
	// renasce com o alvo cheio; pause manual no meio segue o CA-10.
	if resumeMode == "pomodoro_focus" && (target == nil || *target <= 0) {
		cfg, err := loadFullSettings(ctx, tx, userID)
		if err != nil {
			return Result{}, err
		}
		full := cfg.FocusMinutes * 60
		target = &full
	}
	// CA-07: encerrar uma pausa LONGA reseta o ciclo (focus_count=0); pausa
	// CURTA encerrada preserva o contador. O kind vem da COLUNA persistida
	// (row.BreakIsLong) — NÃO recomputado por focus_count % cycles (furo
	// confirmado: diverge no override manual num múltiplo).
	newCount := row.FocusCount
	if row.BreakIsLong {
		newCount = 0
	}
	if _, err := tx.Exec(ctx, `
		UPDATE timer_states
		   SET mode = $2, entry_id = $3, target_seconds = $4, started_at = $5,
		       resume_entry_id = NULL, resume_mode = NULL, resume_target_seconds = NULL,
		       focus_count = $6, break_is_long = false
		 WHERE user_id = $1`,
		userID, resumeMode, e.ID, target, now, newCount); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	st := workState(apiModeOf(resumeMode), &e, target, now, now)
	s.onChange(userID, st, now)
	return Result{State: st, ServerTime: now}, nil
}

// State devolve o estado canônico do timer do usuário (CA-21/22/23) — a
// única fonte do front; toda mutação retorna este mesmo objeto.
func (s *Service) State(ctx context.Context, userID string) (Result, error) {
	now := s.now().UTC()
	row, err := loadStateRow(ctx, s.db, userID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{State: nil, ServerTime: now}, nil // ocioso (CA-23)
	}
	if err != nil {
		return Result{}, err
	}
	st, err := s.stateOf(ctx, s.db, row, now)
	if err != nil {
		return Result{}, err
	}
	return Result{State: st, ServerTime: now}, nil
}

// stateOf materializa o TimerState a partir da linha de runtime.
func (s *Service) stateOf(ctx context.Context, q entries.Querier, row stateRow, now time.Time) (*State, error) {
	if row.Mode == "break" {
		// pausa Pomodoro (CA-22). CA-06.1: o kind vem da COLUNA persistida
		// break_is_long — lido, nunca recomputado.
		st := workState("break", nil, row.TargetSeconds, row.StartedAt, now)
		st.BreakKind = breakKindOf(row.BreakIsLong)
		if row.ResumeEntryID != nil {
			st.Resume = &Resume{
				EntryID:       *row.ResumeEntryID,
				Mode:          *row.ResumeMode,
				TargetSeconds: row.ResumeTargetSeconds,
			}
		}
		return st, nil
	}
	e, err := entries.GetByID(ctx, q, *row.EntryID)
	if err != nil {
		return nil, err
	}
	return workState(apiModeOf(row.Mode), &e, row.TargetSeconds, row.StartedAt, now), nil
}
