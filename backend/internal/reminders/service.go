package reminders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// Service é a camada de aplicação dos reminders (rotas manual/cancelar/listar).
// O gate de permissão por-task espelha o comments.Service (fase 19): a task é
// visível ao user (visibilidade herdada do projeto — 404 não-vaza); member/owner
// criam; viewer só na task onde é assignee (carve-out §8). O Store é a
// persistência.
type Service struct{ store *Store }

func NewService(store *Store) *Service { return &Service{store: store} }

// CreateManual cria um reminder manual PRA SI (user_id = sessão — §8), na task
// visível ao user, respeitando o carve-out viewer-assignee (CA-16/22/23/25).
// remind_at vem do corpo (RFC3339; ausente/inválido já barrou no handler).
func (s *Service) CreateManual(ctx context.Context, workspaceID, userID, role, taskID string, remindAt time.Time) (Reminder, error) {
	if uuid.Validate(taskID) != nil {
		return Reminder{}, domain.NotFound("task não encontrada")
	}
	// Gate (espelha comments.Service.Create): a task é visível E o user pode agir?
	//  - member/owner: basta ver a task (member_can).
	//  - viewer: só se é o assignee DESTA task (viewer_can). COALESCE força false
	//    quando assignee IS NULL (o caso normal) — senão NULL=$u vira SQL NULL e o
	//    Scan pra bool dá 500 onde o esperado é 403 (CA-23).
	var memberCan, viewerCan bool
	err := s.store.db.QueryRow(ctx, `
		SELECT ($3 <> 'viewer')                 AS member_can,
		       COALESCE(t.assignee = $2, false) AS viewer_can
		  FROM tasks t
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE t.workspace_id = $1 AND t.id = $4`,
		workspaceID, userID, role, taskID).Scan(&memberCan, &viewerCan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reminder{}, domain.NotFound("task não encontrada")
	}
	if err != nil {
		return Reminder{}, err
	}
	if !(memberCan || viewerCan) {
		return Reminder{}, domain.Forbidden("sem permissão para criar lembrete nesta task")
	}

	return s.store.Create(ctx, s.store.db, Reminder{
		UserID:      userID,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		RemindAt:    remindAt,
		Kind:        "manual",
	})
}

// Cancel cancela o reminder do user da sessão (soft — CA-18/19). found decide
// 204 (existe e é dele) × 404 (de outro/inexistente, não-vaza). taskID exige a
// coerência do reminder com a task da URL.
func (s *Service) Cancel(ctx context.Context, userID, taskID, reminderID string) (bool, error) {
	return s.store.Cancel(ctx, userID, reminderID, taskID)
}

// List devolve os reminders do user (cross-ws), do status pedido (CA-20/21).
func (s *Service) List(ctx context.Context, userID, status string) ([]Reminder, error) {
	return s.store.ListByUser(ctx, userID, status)
}
