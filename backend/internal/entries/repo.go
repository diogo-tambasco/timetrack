// Package entries: repositório de time_entries (fase 03, T-04). TODA query
// filtra por workspace_id (CA-01, constituição §8.4) E por user_id (CA-01.1
// — a v1 só lista/edita entries do próprio usuário). As funções recebem um
// Querier pra rodar tanto no pool quanto dentro das transações do
// timer.Service (D-05).
package entries

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"timetrack.live/backend/internal/domain"
)

// canonicalUUID exige a forma 8-4-4-4-12 — uuid.Validate sozinho aceita
// 'urn:uuid:...', que o Postgres rejeita no cast (22P02 → 500 em vez do
// 404 uniforme do CA-29.3). Forma não-canônica responde o MESMO not_found.
func canonicalUUID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == strings.ToLower(id)
}

// Entry é o objeto do contrato §3.1/§3.6 do plan. UserID fica fora do JSON:
// escrita é sempre em nome do usuário da sessão (CA-03.1).
type Entry struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	UserID          string     `json:"-"`
	ProjectID       *string    `json:"project_id"`
	TaskID          *string    `json:"task_id"`
	Description     string     `json:"description"`
	Tags            []string   `json:"tags"`
	StartedAt       time.Time  `json:"started_at"`
	StoppedAt       *time.Time `json:"stopped_at"`
	DurationSeconds *int       `json:"duration_seconds"`
	Source          string     `json:"source"`
}

// Querier é o subconjunto de pgxpool.Pool/pgx.Tx que o repositório usa.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const entryCols = `id, workspace_id, user_id, project_id, task_id, description,
                   tags, started_at, stopped_at, duration_seconds, source`

func scanEntry(row pgx.Row) (Entry, error) {
	var e Entry
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.UserID, &e.ProjectID, &e.TaskID,
		&e.Description, &e.Tags, &e.StartedAt, &e.StoppedAt, &e.DurationSeconds, &e.Source)
	return e, err
}

// Insert persiste a entry e devolve a linha completa.
func Insert(ctx context.Context, q Querier, e Entry) (Entry, error) {
	if e.Tags == nil {
		e.Tags = []string{}
	}
	return scanEntry(q.QueryRow(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, project_id, task_id,
		                          description, tags, started_at, stopped_at,
		                          duration_seconds, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+entryCols,
		e.WorkspaceID, e.UserID, e.ProjectID, e.TaskID, e.Description, e.Tags,
		e.StartedAt, e.StoppedAt, e.DurationSeconds, e.Source))
}

// List devolve as entries do usuário no workspace com started_at dentro de
// [from, to), mais recente primeiro (CA-35). Filtro duplo obrigatório:
// workspace_id (CA-01) e user_id (CA-01.1).
func List(ctx context.Context, q Querier, workspaceID, userID string, from, to time.Time) ([]Entry, error) {
	rows, err := q.Query(ctx, `
		SELECT `+entryCols+` FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2
		   AND started_at >= $3 AND started_at < $4
		 ORDER BY started_at DESC, id`, workspaceID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// GetOwned carrega a entry SE ela for do workspace e do usuário; qualquer
// outra combinação (outro workspace, outro usuário, inexistente, id fora do
// formato uuid) responde o MESMO not_found (CA-29.3 — não vazar existência).
func GetOwned(ctx context.Context, q Querier, workspaceID, userID, id string) (Entry, error) {
	if !canonicalUUID(id) {
		return Entry{}, domain.NotFound("entrada não encontrada")
	}
	e, err := scanEntry(q.QueryRow(ctx, `
		SELECT `+entryCols+` FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2 AND id = $3`,
		workspaceID, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, domain.NotFound("entrada não encontrada")
	}
	return e, err
}

// GetByID carrega a entry por id SEM filtro de tenant — uso interno do
// timer.Service, cujos ids vêm de timer_states do próprio usuário. Rotas
// REST nunca chegam aqui direto: usam GetOwned.
func GetByID(ctx context.Context, q Querier, id string) (Entry, error) {
	e, err := scanEntry(q.QueryRow(ctx, `
		SELECT `+entryCols+` FROM time_entries WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, domain.NotFound("entrada não encontrada")
	}
	return e, err
}

// Update persiste os campos editáveis da entry própria (T-12): o WHERE
// mantém o filtro duplo workspace+user (CA-01/01.1) + id.
func Update(ctx context.Context, q Querier, e Entry) (Entry, error) {
	return scanEntry(q.QueryRow(ctx, `
		UPDATE time_entries
		   SET description = $4, project_id = $5, task_id = $6, tags = $7,
		       started_at = $8, stopped_at = $9, duration_seconds = $10
		 WHERE workspace_id = $1 AND user_id = $2 AND id = $3
		RETURNING `+entryCols,
		e.WorkspaceID, e.UserID, e.ID, e.Description, e.ProjectID, e.TaskID,
		e.Tags, e.StartedAt, e.StoppedAt, e.DurationSeconds))
}

// Delete remove a entry própria; fora do workspace/usuário = not_found
// (CA-16/29.3). As proteções 409 (entry rodando / em uso) moram na camada
// de serviço (T-12).
func Delete(ctx context.Context, q Querier, workspaceID, userID, id string) error {
	if !canonicalUUID(id) {
		return domain.NotFound("entrada não encontrada")
	}
	tag, err := q.Exec(ctx, `
		DELETE FROM time_entries
		 WHERE workspace_id = $1 AND user_id = $2 AND id = $3`,
		workspaceID, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("entrada não encontrada")
	}
	return nil
}
