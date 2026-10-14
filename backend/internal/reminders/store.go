// Package reminders: a 2ª infra de BACKGROUND do Timetrack (fase 20). Um worker
// próprio (Worker/Dispatcher) varre os reminders pendentes vencidos e dispara o
// aviso (in-app + Web Push), espelhando o formato do guard.Worker/guard.Cutter
// da fase 16 — mas SEM importar nem tocar o pacote guard (CA-26/RNF-05). Além do
// despacho, este pacote é dono do repo (Store), da criação do reminder due na
// escrita da task (due.go) e do helper de horário 09:00-local.
package reminders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Reminder é o objeto do contrato (plan §3). TaskName só é preenchido na
// ListByUser (join com tasks — CA-20).
type Reminder struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	TaskID      string    `json:"task_id"`
	WorkspaceID string    `json:"workspace_id"`
	RemindAt    time.Time `json:"remind_at"`
	Kind        string    `json:"kind"`   // "due" | "manual"
	Status      string    `json:"status"` // "pending" | "sent" | "canceled"
	TaskName    string    `json:"task_name,omitempty"`
}

// Querier é o subconjunto comum de *pgxpool.Pool e pgx.Tx: deixa Create/
// CancelDuePending rodarem na MESMA transação do Create/Patch de task (CA-28)
// OU numa conexão própria (a rota manual passa o pool).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// Create insere um reminder pending e devolve o objeto criado. q = a tx do
// Create/Patch de task (due) OU o pool (manual). Não preenche TaskName.
func (s *Store) Create(ctx context.Context, q Querier, r Reminder) (Reminder, error) {
	out := r
	err := q.QueryRow(ctx, `
		INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at`,
		r.UserID, r.TaskID, r.WorkspaceID, r.RemindAt, r.Kind).
		Scan(&out.ID, &out.Status, new(time.Time))
	return out, err
}

// CancelDuePending marca canceled o reminder due PENDENTE da task (idempotente;
// 0..1 linhas — o unique parcial garante no máximo 1). Roda na tx do Patch
// (CA-12/13). NÃO toca due sent nem reminders manual.
func (s *Store) CancelDuePending(ctx context.Context, q Querier, taskID string) error {
	_, err := q.Exec(ctx, `
		UPDATE reminders SET status = 'canceled'
		 WHERE task_id = $1 AND kind = 'due' AND status = 'pending'`, taskID)
	return err
}

// ListByUser devolve os reminders do user (cross-workspace), do status pedido
// (default pending), ordenados por remind_at, com o nome da task por join
// (CA-20). SEMPRE filtra por r.user_id (CA-21 — isolamento). Vazio → [] não-nil.
func (s *Store) ListByUser(ctx context.Context, userID, status string) ([]Reminder, error) {
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "sent" && status != "canceled" {
		return nil, domain.Validation("status deve ser pending, sent ou canceled")
	}
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.user_id, r.task_id, r.workspace_id, r.remind_at, r.kind, r.status, t.name
		  FROM reminders r
		  JOIN tasks t ON t.id = r.task_id
		 WHERE r.user_id = $1 AND r.status = $2
		 ORDER BY r.remind_at, r.id`, userID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Reminder{}
	for rows.Next() {
		var r Reminder
		if err := rows.Scan(&r.ID, &r.UserID, &r.TaskID, &r.WorkspaceID,
			&r.RemindAt, &r.Kind, &r.Status, &r.TaskName); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// Cancel marca canceled o reminder do user da sessão (soft — não apaga; CA-18).
// found = "existe alguma linha com esse id E user_id" (independe de status, pra
// cancelar um já-canceled/sent ser 204 idempotente). Reminder de OUTRO user /
// inexistente → found=false → 404 (CA-19, não-vaza). taskID, quando não-vazio,
// é exigido coerente (o reminder pertence à task da URL — senão found=false).
func (s *Store) Cancel(ctx context.Context, userID, reminderID, taskID string) (bool, error) {
	if uuid.Validate(reminderID) != nil {
		return false, nil // id malformado = inexistente (404 não-vaza)
	}
	if taskID != "" && uuid.Validate(taskID) != nil {
		// taskID malformado da URL = incoerente: 404 não-vaza, NÃO 500 do cast
		// $3::uuid estourando no Postgres (22P02). Espelha o guard do reminderID.
		return false, nil
	}
	// O UPDATE só casa pending; o EXISTS decide 204×404 (independe de status).
	if _, err := s.db.Exec(ctx, `
		UPDATE reminders SET status = 'canceled'
		 WHERE id = $1 AND user_id = $2 AND status = 'pending'
		   AND ($3 = '' OR task_id = $3::uuid)`, reminderID, userID, taskID); err != nil {
		return false, err
	}
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT true FROM reminders
		 WHERE id = $1 AND user_id = $2 AND ($3 = '' OR task_id = $3::uuid)`,
		reminderID, userID, taskID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists, nil
}
