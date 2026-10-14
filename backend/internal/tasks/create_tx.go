// create_tx.go — fase 26 (coordenação D / 26-H4): CreateTx é um método ADITIVO
// pro slash /task das Notas. Aceita uma pgx.Tx já aberta (a transação atômica do
// notes.CreateLink — D-04) e faz SOMENTE validName + loadProjectGate
// (blockArchived=true) + INSERT na tx, com status no DEFAULT do enum ('todo' —
// coordenação C; NÃO crava 'backlog'). SEM objective/assignee/due/recurrence/
// notes/tz, SEM recalcObjectiveStatus, SEM reminder.OnDueDateSet, SEM
// AssignNotify — nada disso se aplica ao slash. tasks.Create fica BYTE-IDÊNTICO
// (RNF-05). tasks NÃO importa notes (a direção é notes → tasks).
package tasks

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// CreateTx cria a task no projeto dado dentro da transação `tx` fornecida pelo
// caller (o slash /task das Notas). status = DEFAULT 'todo'. Roda o
// loadProjectGate(blockArchived=true) por segurança (mesma regra de escrita do
// Create) — é o mesmo projeto/role que o gate da nota já validou, então não
// falha por permissão; o gate aqui só repete a invariante de escrita.
func (s *Service) CreateTx(ctx context.Context, tx pgx.Tx, workspaceID, userID, role, projectID, name string) (Task, error) {
	trimmed, err := validName(name)
	if err != nil {
		return Task{}, err
	}
	if err := s.loadProjectGate(ctx, workspaceID, userID, role, projectID, true); err != nil {
		return Task{}, err
	}
	// status no DEFAULT 'todo' (coordenação C / 26-H1: o INSERT NÃO seta status
	// — a coluna fica no default do enum). position = max+1 da coluna 'todo'.
	const status = "todo"
	position, err := nextPosition(ctx, tx, projectID, status)
	if err != nil {
		return Task{}, err
	}
	t, err := scanTask(tx.QueryRow(ctx, `
		INSERT INTO tasks (workspace_id, project_id, name, created_by, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+taskCols(""),
		workspaceID, projectID, trimmed, userID, position))
	if err != nil {
		return Task{}, err
	}
	return t, nil
}
