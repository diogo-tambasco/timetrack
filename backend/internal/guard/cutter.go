// Package guard: a 1ª infra de BACKGROUND do Timetrack (fase 16). Um worker
// leve (RNF-01) varre as entries rodando e CORTA as que passaram de 2h sem
// pausa — fecha a entry NO MARCO das 2h (stopped_at = started_at + 2h, NÃO
// now), pra a métrica não ganhar horas-fantasma (US-02/CA-03).
//
// O corte REUSA as primitivas do timer.Service (advisory lock por usuário,
// fechar entry, limpar timer_states) mas vive num pacote NOVO (D-01): é uma
// preocupação diferente (background) e não pode inchar/arriscar o
// timer.Service (RNF-05). O clock é injetável (RNF-03), como no timer.
package guard

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/timer"
)

// cutAfter é o limiar do corte: uma entry rodando há MAIS de 2h (estrito —
// CA-02) é cortada. Fixo nesta fase (limite configurável = fase futura).
const cutAfter = 2 * time.Hour

// CutEvent descreve um corte aplicado nesta passada (pós-commit): alimenta o
// aviso in-app (notice) e o Web Push (D-05). StoppedAt é o MARCO (started+2h).
type CutEvent struct {
	UserID    string
	EntryID   string
	StoppedAt time.Time
}

// Cutter aplica o corte de timers esquecidos. Construir com NewCutter.
type Cutter struct {
	db      *pgxpool.Pool
	now     func() time.Time
	onTimer func(userID string, st *timer.State, at time.Time)
	onCut   func(c CutEvent)
}

// Option ajusta o Cutter na construção.
type Option func(*Cutter)

// WithClock injeta o relógio (testes — RNF-03); default time.Now.
func WithClock(now func() time.Time) Option {
	return func(c *Cutter) { c.now = now }
}

// WithOnTimer registra o hook do estado de timer pós-corte (= hub.BroadcastTimer):
// emite type:"timer" com estado NULO, igual a um Stop remoto — a UI converge
// pra ociosa pelo caminho da fase 04 (CA-12/D-05). Default no-op.
func WithOnTimer(fn func(userID string, st *timer.State, at time.Time)) Option {
	return func(c *Cutter) { c.onTimer = fn }
}

// WithOnCut registra o hook do aviso de corte (notice in-app + push — D-05),
// chamado SÓ pra entries efetivamente cortadas nesta passada (CA-09). Default
// no-op.
func WithOnCut(fn func(c CutEvent)) Option {
	return func(c *Cutter) { c.onCut = fn }
}

// NewCutter cria o cortador com clock real e hooks no-op.
func NewCutter(db *pgxpool.Pool, opts ...Option) *Cutter {
	c := &Cutter{
		db:      db,
		now:     time.Now,
		onTimer: func(string, *timer.State, time.Time) {},
		onCut:   func(CutEvent) {},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// candidate é uma entry rodando candidata ao corte (seleção leve, fora da
// transação — RNF-01). A verdade é reconferida sob o lock (CA-07).
type candidate struct {
	entryID string
	userID  string
}

// RunOnce varre as entries rodando vencidas (now − started_at > 2h, estrito)
// e corta cada uma na MESMA transação serializada por usuário do timer.Service.
// Devolve os cortes aplicados (pra teste). Best-effort por usuário: um erro
// num corte não derruba os outros. Os hooks (onTimer/onCut) rodam SÓ pras
// entries que commitaram o corte (CA-09), pós-commit.
func (c *Cutter) RunOnce(ctx context.Context) ([]CutEvent, error) {
	now := c.now().UTC()
	threshold := now.Add(-cutAfter)

	// Seleção (CA-02): só entries rodando (stopped_at IS NULL) iniciadas
	// ANTES de now−2h (estrito → exatamente 2h NÃO corta). Leitura barata
	// e indexada (uq_entries_one_running / idx_entries_user). Pausa não tem
	// entry rodando, então nunca aparece aqui (CA-05).
	rows, err := c.db.Query(ctx, `
		SELECT id, user_id FROM time_entries
		 WHERE stopped_at IS NULL AND started_at < $1
		 ORDER BY user_id`, threshold)
	if err != nil {
		return nil, err
	}
	cands := []candidate{}
	for rows.Next() {
		var cd candidate
		if err := rows.Scan(&cd.entryID, &cd.userID); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, cd)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var cuts []CutEvent
	for _, cd := range cands {
		evt, cut, err := c.cutOne(ctx, cd, now)
		if err != nil {
			// best-effort: erro num usuário não para os outros (CA-15 análogo)
			continue
		}
		if cut {
			cuts = append(cuts, evt)
		}
	}

	// Pós-commit (CA-09): aviso só pras entries REALMENTE cortadas. Fora da
	// transação e do lock — push/notice lento nunca segura a varredura (RNF-01).
	for _, evt := range cuts {
		c.onTimer(evt.UserID, nil, now)
		c.onCut(evt)
	}
	return cuts, nil
}

// cutOne corta UMA entry candidata sob o advisory lock do usuário (CA-07/08).
// Devolve (evt, true) se cortou, (_, false) se a entry sumiu/já parou no
// intervalo (idempotência/corrida — CA-06/07). O corte fecha a entry no MARCO
// (started+2h — CA-03), marca auto_cut e limpa timer_states (CA-04).
func (c *Cutter) cutOne(ctx context.Context, cd candidate, now time.Time) (CutEvent, bool, error) {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return CutEvent{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// MESMO lock do timer.Service (hashtext(user_id)) — serializa com
	// stop/pause/resume simultâneos do usuário (CA-07).
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1::text))`, cd.userID); err != nil {
		return CutEvent{}, false, err
	}

	// Re-confere sob o lock + FOR UPDATE (CA-07): se o usuário parou/retomou
	// no intervalo entre a seleção e aqui, a entry não está mais rodando →
	// aborta sem fechar nada em dobro nem ressuscitar entry parada. Filtra por
	// user_id também (CA-08 — isolamento).
	var startedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT started_at FROM time_entries
		 WHERE id = $1 AND user_id = $2 AND stopped_at IS NULL
		 FOR UPDATE`, cd.entryID, cd.userID).Scan(&startedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CutEvent{}, false, nil // parada no intervalo — nada a fazer
	}
	if err != nil {
		return CutEvent{}, false, err
	}

	// Defesa: se a entry ainda não venceu (re-checagem do limiar sob o lock),
	// não corta. Caminho normal só chega aqui com entry vencida, mas o re-check
	// fecha a janela contra started_at adiantado por outra mutação.
	mark := startedAt.Add(cutAfter)
	if !startedAt.Before(now.Add(-cutAfter)) {
		return CutEvent{}, false, nil
	}

	// Corte (CA-03): fecha NO MARCO (started+2h), duração = 7200, auto_cut=true.
	// NUNCA stopped_at=now (seria a hora-fantasma — a dor inteira da fase).
	if _, err := tx.Exec(ctx, `
		UPDATE time_entries
		   SET stopped_at = started_at + interval '2 hours',
		       duration_seconds = 7200,
		       auto_cut = true
		 WHERE id = $1 AND user_id = $2 AND stopped_at IS NULL`,
		cd.entryID, cd.userID); err != nil {
		return CutEvent{}, false, err
	}

	// Limpa o estado do timer (CA-04): o usuário fica ocioso, igual ao Stop.
	// Sem linha (entry órfã) = no-op — a entry segue a fonte do "rodando".
	if _, err := tx.Exec(ctx,
		`DELETE FROM timer_states WHERE user_id = $1`, cd.userID); err != nil {
		return CutEvent{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CutEvent{}, false, err
	}
	return CutEvent{UserID: cd.userID, EntryID: cd.entryID, StoppedAt: mark}, true, nil
}
