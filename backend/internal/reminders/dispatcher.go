package reminders

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/push"
)

// Dispatched descreve um reminder despachado nesta passada (pós-commit):
// alimenta os hooks de aviso e os testes. Espelha o guard.CutEvent.
type Dispatched struct {
	UserID   string
	TaskID   string
	TaskName string
}

// Dispatcher varre os reminders pendentes vencidos e despacha cada um, sob a
// MESMA família de advisory lock por usuário do timer/guard (RNF-02). Clock
// injetável (RNF-03). Espelha o guard.Cutter — SEM importar guard. Construir com
// NewDispatcher.
type Dispatcher struct {
	db       *pgxpool.Pool
	now      func() time.Time
	onNotice func(userID string, payload any)    // = hub.BroadcastNotice
	onPush   func(userID string, payload []byte) // = pushSender.SendToUser (best-effort)
}

// Option ajusta o Dispatcher na construção.
type Option func(*Dispatcher)

// WithClock injeta o relógio (testes — RNF-03); default time.Now.
func WithClock(now func() time.Time) Option {
	return func(d *Dispatcher) { d.now = now }
}

// WithOnNotice registra o hook do aviso in-app (= hub.BroadcastNotice): chamado
// SÓ pros reminders efetivamente despachados (CA-06), pós-commit. Default no-op.
func WithOnNotice(fn func(userID string, payload any)) Option {
	return func(d *Dispatcher) { d.onNotice = fn }
}

// WithOnPush registra o hook do Web Push (= pushSender.SendToUser): best-effort,
// pós-commit (CA-10). Default no-op.
func WithOnPush(fn func(userID string, payload []byte)) Option {
	return func(d *Dispatcher) { d.onPush = fn }
}

// NewDispatcher cria o despachante com clock real e hooks no-op.
func NewDispatcher(db *pgxpool.Pool, opts ...Option) *Dispatcher {
	d := &Dispatcher{
		db:       db,
		now:      time.Now,
		onNotice: func(string, any) {},
		onPush:   func(string, []byte) {},
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// pending é um reminder candidato ao despacho (seleção leve, fora da transação
// — RNF-01). A verdade é reconferida sob o lock (CA-08).
type pending struct {
	id     string
	userID string
}

// RunOnce varre os reminders pending vencidos (remind_at <= now, INCLUSIVO —
// CA-04) e despacha cada um na MESMA transação serializada por usuário. Devolve
// os despachados (pra teste). Best-effort: erro num reminder não derruba os
// outros. Os hooks (onNotice/onPush) rodam SÓ pros que commitaram o flip (CA-06),
// pós-commit (fora do lock — RNF-01).
func (d *Dispatcher) RunOnce(ctx context.Context) ([]Dispatched, error) {
	now := d.now().UTC()

	// Seleção (CA-04): só a fatia pending vencida (idx_reminders_pending
	// parcial). Limiar INCLUSIVO (<=) — o due nasce no horário-alvo. Leitura
	// barata, fora de transação; a verdade é reconferida sob o lock.
	rows, err := d.db.Query(ctx, `
		SELECT id, user_id FROM reminders
		 WHERE status = 'pending' AND remind_at <= $1
		 ORDER BY user_id`, now)
	if err != nil {
		return nil, err
	}
	var cands []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.userID); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var done []Dispatched
	for _, p := range cands {
		evt, ok, err := d.dispatchOne(ctx, p)
		if err != nil {
			// best-effort: erro num reminder não para os outros.
			continue
		}
		if ok {
			done = append(done, evt)
		}
	}

	// Pós-commit (CA-06): aviso só pros REALMENTE despachados. Fora do lock e da
	// tx — push/notice lento nunca segura a varredura (RNF-01). O user_id é
	// sempre o DONO do reminder (CA-09).
	for _, evt := range done {
		d.onNotice(evt.UserID, noticePayload(evt))
		d.onPush(evt.UserID, pushPayload(evt))
	}
	return done, nil
}

// dispatchOne despacha UM reminder candidato sob o advisory lock do usuário
// (CA-08), espelhando guard.cutOne. Devolve (evt, true) se flipou pending→sent,
// (_, false) se o reminder sumiu/já não-pending no intervalo (cancelado ou
// consumido por outra passada concorrente — idempotência/corrida CA-08).
func (d *Dispatcher) dispatchOne(ctx context.Context, p pending) (Dispatched, bool, error) {
	tx, err := d.db.Begin(ctx)
	if err != nil {
		return Dispatched{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// MESMO lock do timer.Service/guard (hashtext(user_id)) — serializa com um
	// cancel concorrente do usuário e com outra passada (CA-08).
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1::text))`, p.userID); err != nil {
		return Dispatched{}, false, err
	}

	// Re-confere sob o lock + FOR UPDATE (CA-08): se o reminder foi cancelado ou
	// já enviado por outra passada no intervalo, não está mais pending → rollback,
	// nada feito. Filtra por user_id também (isolamento — CA-09). Carrega o nome
	// da task pro payload (join).
	var taskID, taskName string
	err = tx.QueryRow(ctx, `
		SELECT r.task_id, t.name
		  FROM reminders r
		  JOIN tasks t ON t.id = r.task_id
		 WHERE r.id = $1 AND r.user_id = $2 AND r.status = 'pending'
		 FOR UPDATE OF r`, p.id, p.userID).Scan(&taskID, &taskName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Dispatched{}, false, nil // cancelado/consumido no intervalo — nada a fazer
	}
	if err != nil {
		return Dispatched{}, false, err
	}

	// Flip pending→sent (CA-05). A fonte da verdade da idempotência: a 2ª passada
	// não acha mais pending. remind_at/kind/task_id/user_id ficam intactos.
	if _, err := tx.Exec(ctx, `
		UPDATE reminders SET status = 'sent'
		 WHERE id = $1 AND user_id = $2 AND status = 'pending'`, p.id, p.userID); err != nil {
		return Dispatched{}, false, err
	}

	// COMMIT antes do envio (D-04): nunca reenviar em loop. Crash entre commit e
	// envio = sent sem aviso (perda silenciosa aceita — push é best-effort).
	if err := tx.Commit(ctx); err != nil {
		return Dispatched{}, false, err
	}
	return Dispatched{UserID: p.userID, TaskID: taskID, TaskName: taskName}, true, nil
}

// noticePayload monta o payload do aviso in-app (CA-07): kind reminder + task +
// título (nome da task). O front roteia por kind (distingue do timer_auto_cut).
func noticePayload(evt Dispatched) map[string]any {
	return map[string]any{
		"kind":    "reminder",
		"task_id": evt.TaskID,
		"title":   evt.TaskName,
	}
}

// pushPayload monta o corpo do Web Push do lembrete (CA-07/10): vira o JSON que
// o service worker mostra via showNotification. Mesma mecânica do pushPayload do
// corte (fase 16).
func pushPayload(evt Dispatched) []byte {
	b, _ := json.Marshal(push.Payload{
		Title: "Lembrete",
		Body:  evt.TaskName,
		Kind:  "reminder",
	})
	return b
}
