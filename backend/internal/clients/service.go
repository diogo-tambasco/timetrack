// Package clients: regras de domínio de cliente (fase 02, CA-01..05).
// Soft-delete via archived_at (D-03) — apagar quebraria o histórico das
// fases 03/07. Toda query filtra por workspace_id (CA-26).
package clients

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// Client é o objeto do contrato §3.1 do plan.
type Client struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// validName aplica CA-02/CA-01: trim e 1–120 caracteres; persiste o trim.
func validName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 120 {
		return "", domain.Validation("nome deve ter entre 1 e 120 caracteres")
	}
	return trimmed, nil
}

// Create cria um cliente no workspace (CA-01).
func (s *Service) Create(ctx context.Context, workspaceID, name string) (Client, error) {
	trimmed, err := validName(name)
	if err != nil {
		return Client{}, err
	}
	var c Client
	err = s.db.QueryRow(ctx, `
		INSERT INTO clients (workspace_id, name) VALUES ($1, $2)
		RETURNING id, workspace_id, name, archived_at, created_at`,
		workspaceID, trimmed).
		Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.ArchivedAt, &c.CreatedAt)
	return c, err
}

// Rename renomeia um cliente do workspace (CA-04). Inexistente (ou id fora
// do formato uuid) responde o mesmo not_found — não vaza existência.
func (s *Service) Rename(ctx context.Context, workspaceID, clientID, name string) (Client, error) {
	trimmed, err := validName(name)
	if err != nil {
		return Client{}, err
	}
	if uuid.Validate(clientID) != nil {
		return Client{}, domain.NotFound("cliente não encontrado")
	}
	var c Client
	err = s.db.QueryRow(ctx, `
		UPDATE clients SET name = $3
		 WHERE workspace_id = $1 AND id = $2
		RETURNING id, workspace_id, name, archived_at, created_at`,
		workspaceID, clientID, trimmed).
		Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.ArchivedAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, domain.NotFound("cliente não encontrado")
	}
	return c, err
}

// List devolve os clientes do workspace; arquivados só entram com
// includeArchived (CA-03).
func (s *Service) List(ctx context.Context, workspaceID string, includeArchived bool) ([]Client, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, workspace_id, name, archived_at, created_at
		  FROM clients
		 WHERE workspace_id = $1 AND ($2 OR archived_at IS NULL)
		 ORDER BY created_at, id`, workspaceID, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Client{}
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.ArchivedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// Archive arquiva o cliente (CA-05): preenche archived_at, NUNCA apaga a
// linha — projetos vinculados ficam intactos. Idempotente.
func (s *Service) Archive(ctx context.Context, workspaceID, clientID string) error {
	if uuid.Validate(clientID) != nil {
		return domain.NotFound("cliente não encontrado")
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE clients SET archived_at = COALESCE(archived_at, now())
		 WHERE workspace_id = $1 AND id = $2`, workspaceID, clientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("cliente não encontrado")
	}
	return nil
}
