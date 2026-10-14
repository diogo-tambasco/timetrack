package reminders

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// dueRemindAt computa o horário-alvo do reminder due: 09:00 do dia da due_date,
// NO FUSO da requisição (§2.2). tz vazio → UTC (default herdado das outras
// rotas); tz IANA inválido → erro (a rota valida antes, mas o helper é defensivo
// e testável). É tz-correto (time.Date com a loc), igual ao calendarWindow —
// NUNCA computar em UTC e "ajustar".
func dueRemindAt(dueDate, tz string) (time.Time, error) {
	loc := time.UTC
	if tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return time.Time{}, domain.Validation("tz deve ser um fuso IANA válido")
		}
		loc = l
	}
	d, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		return time.Time{}, domain.Validation("due_date deve ser uma data ISO 8601 (YYYY-MM-DD)")
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, loc), nil
}

// Hook implementa o tasks.ReminderHook: cria/cancela o reminder due dentro da
// MESMA transação do Create/Patch de task (CA-28). É chamado SOMENTE quando a
// due_date MUDOU DE VALOR (o gatilho mora no tasks.Service — CA-12.1). Quando
// chamado:
//   - cancela qualquer due PENDENTE pré-existente da task (upsert lógico);
//   - se dueDate != nil, cria um novo due pending pro responsável
//     (assignee ?? created_by, lido na tx) com remind_at = 09:00 local.
//
// Quando dueDate == nil (limpar): só cancela, não cria (CA-13).
type Hook struct{ store *Store }

// NewHook cria o hook do reminder due ligado ao Store.
func NewHook(store *Store) *Hook { return &Hook{store: store} }

// OnDueDateSet roda dentro da tx do Create/Patch. taskID identifica a task (já
// gravada com a due_date e o assignee correntes); tz vem da requisição. O
// requestUserID é passado por consistência com o padrão de hook do projeto, mas
// o RESPONSÁVEL do reminder é resolvido da TASK (assignee ?? created_by — CA-14),
// não de quem chamou.
func (h *Hook) OnDueDateSet(ctx context.Context, tx pgx.Tx, taskID, requestUserID string, dueDate *string, tz string) error {
	// cancela o due pendente antigo (idempotente; 0..1) — vale pra mudar E limpar.
	if err := h.store.CancelDuePending(ctx, tx, taskID); err != nil {
		return err
	}
	if dueDate == nil {
		return nil // limpar a due_date só cancela (CA-13)
	}
	remindAt, err := dueRemindAt(*dueDate, tz)
	if err != nil {
		return err
	}
	// responsável = assignee ?? created_by, lido na tx (a task já está gravada).
	var ownerID, workspaceID string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(assignee, created_by), workspace_id
		  FROM tasks WHERE id = $1`, taskID).Scan(&ownerID, &workspaceID); err != nil {
		return err
	}
	_, err = h.store.Create(ctx, tx, Reminder{
		UserID:      ownerID,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		RemindAt:    remindAt,
		Kind:        "due",
	})
	return err
}
