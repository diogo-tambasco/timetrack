// Package invitations: convites de workspace + gestão de membros (fase 17 —
// fundação do multiplayer). É a 1ª camada que ESCREVE em workspace_members
// por API (até aqui só o 1º login criava o owner). Pacote FOLHA: usa
// *pgxpool.Pool + crypto/rand direto; NÃO importa workspace nem auth (sem
// ciclo — o login chama ResolveForEmail por função injetada em
// auth.Config.ResolveInvites, plan §D-01/D-05). O Service devolve *domain.Error
// (a camada api mapeia pro envelope canônico via writeDomainError).
package invitations

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/domain"
)

// defaultInviteTTL — janela padrão de validade de um convite pending (14 dias,
// D-03). O boot pode sobrescrever via INVITE_TTL.
const defaultInviteTTL = 336 * time.Hour

// Invitation é o objeto do contrato (plan §4) — o token NUNCA é exposto (RNF-04).
type Invitation struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// Member é o retorno do add de um user existente (CA-10).
type Member struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// Service agrega as queries de convite/membro. now/inviteTTL injetáveis.
type Service struct {
	db        *pgxpool.Pool
	now       func() time.Time
	inviteTTL time.Duration
}

// Option ajusta o Service na construção.
type Option func(*Service)

// WithClock injeta o relógio (testes de expiração — espelha timer/guard).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithTTL define a janela de validade do convite (0/negativo → default 14d).
func WithTTL(d time.Duration) Option {
	return func(s *Service) {
		if d > 0 {
			s.inviteTTL = d
		}
	}
}

// NewService cria o serviço com clock real e TTL default.
func NewService(db *pgxpool.Pool, opts ...Option) *Service {
	s := &Service{db: db, now: time.Now, inviteTTL: defaultInviteTTL}
	for _, o := range opts {
		o(s)
	}
	if s.inviteTTL <= 0 {
		s.inviteTTL = defaultInviteTTL
	}
	return s
}

// normEmail normaliza o email (lower/trim) — CA-05. O índice parcial e toda
// comparação de já-membro/resolução usam lower(email) dos dois lados.
func normEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// validInviteRole — convite/add direto só aceitam member|viewer (CA-03/10);
// owner só por PATCH (promoção).
func validInviteRole(role string) bool { return role == "member" || role == "viewer" }

// userNotFound é o 404 com code CUSTOM (CA-11) — o construtor domain.NotFound
// hardcoda code "not_found", então usamos o literal pra distinguir do 404 puro.
func userNotFound() *domain.Error {
	return &domain.Error{
		Status:  http.StatusNotFound,
		Code:    "user_not_found",
		Message: "usuário não encontrado — use o convite por email",
	}
}

// newToken gera o token opaco do convite (≥128 bits, base64url) — RNF-03.
// Não importa auth (folha); espelha o NewSessionToken (32 bytes crypto/rand).
func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Create cria um convite pending (CA-04). Normaliza o email, valida role+
// formato, barra já-membro (CA-07, cobre o auto-convite — o owner é membro),
// e trata o pending duplicado como 409 invite_pending (CA-08, nunca 500/2 linhas).
func (s *Service) Create(ctx context.Context, workspaceID, invitedByUserID, email, role string) (Invitation, error) {
	em := normEmail(email)
	if em == "" || !strings.Contains(em, "@") {
		return Invitation{}, domain.Validation("email inválido")
	}
	if !validInviteRole(role) {
		return Invitation{}, domain.Validation("role deve ser member ou viewer")
	}

	already, err := s.isMemberByEmail(ctx, workspaceID, em)
	if err != nil {
		return Invitation{}, err
	}
	if already {
		return Invitation{}, domain.Conflict("already_member", "esse email já é membro do workspace")
	}

	expiresAt := s.now().Add(s.inviteTTL)
	var inv Invitation
	err = s.db.QueryRow(ctx, `
		INSERT INTO invitations (workspace_id, email, role, token, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, role, status, expires_at, created_at`,
		workspaceID, em, role, newToken(), invitedByUserID, expiresAt).
		Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		// ON CONFLICT no índice parcial ux_invitations_pending → já há 1 pending
		// pra esse (workspace, email): CA-08 (nunca 500, nunca 2 linhas).
		if isUniqueViolation(err, "ux_invitations_pending") {
			return Invitation{}, domain.Conflict("invite_pending", "já existe um convite pendente para esse email")
		}
		return Invitation{}, err
	}
	return inv, nil
}

// List devolve os convites PENDING do workspace da rota (CA-13), recentes
// primeiro, SEM o token (RNF-04). Filtra por workspace_id (CA-22).
func (s *Service) List(ctx context.Context, workspaceID string) ([]Invitation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, email, role, status, expires_at, created_at
		  FROM invitations
		 WHERE workspace_id = $1 AND status = 'pending'
		 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Invitation{}
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// Revoke marca um convite pending como revoked (CA-14, soft — preserva
// auditoria). accepted → invite_not_pending; já revoked → no-op idempotente;
// convite de outro workspace/inexistente → not_found (CA-15). Filtra
// id AND workspace_id (CA-22 — não vaza existência entre workspaces).
func (s *Service) Revoke(ctx context.Context, workspaceID, inviteID string) error {
	if !canonicalUUID(inviteID) {
		return domain.NotFound("convite não encontrado")
	}
	var status string
	err := s.db.QueryRow(ctx, `
		SELECT status FROM invitations
		 WHERE id = $1 AND workspace_id = $2`, inviteID, workspaceID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("convite não encontrado")
	}
	if err != nil {
		return err
	}
	switch status {
	case "revoked":
		return nil // idempotente (CA-14)
	case "accepted":
		return domain.Conflict("invite_not_pending", "convite já aceito não pode ser revogado")
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE invitations SET status = 'revoked'
		 WHERE id = $1 AND workspace_id = $2`, inviteID, workspaceID); err != nil {
		return err
	}
	return nil
}

// AddExistingMember adiciona direto quem JÁ tem conta (fluxo B, CA-10).
// Resolve o user_id por lower(email); inexistente → user_not_found 404 (CA-11);
// já-membro → already_member 409 (CA-12). role member|viewer (owner por PATCH).
func (s *Service) AddExistingMember(ctx context.Context, workspaceID, email, role string) (Member, error) {
	em := normEmail(email)
	if em == "" || !strings.Contains(em, "@") {
		return Member{}, domain.Validation("email inválido")
	}
	if !validInviteRole(role) {
		return Member{}, domain.Validation("role deve ser member ou viewer")
	}

	var m Member
	err := s.db.QueryRow(ctx, `
		SELECT id, name, email FROM users WHERE lower(email) = $1`, em).
		Scan(&m.UserID, &m.Name, &m.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, userNotFound()
	}
	if err != nil {
		return Member{}, err
	}

	tag, err := s.db.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING`, workspaceID, m.UserID, role)
	if err != nil {
		return Member{}, err
	}
	if tag.RowsAffected() == 0 {
		return Member{}, domain.Conflict("already_member", "esse usuário já é membro do workspace")
	}
	m.Role = role
	return m, nil
}

// RemoveMember remove a membership (CA-16). Sob advisory lock por workspace +
// re-check de owners: remover o ÚNICO owner → last_owner 409 (CA-18). Membro
// inexistente → not_found 404 (CA-20). Filtra workspace_id (CA-22).
func (s *Service) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	if !canonicalUUID(userID) {
		return domain.NotFound("membro não encontrado")
	}
	return s.withWorkspaceLock(ctx, workspaceID, func(tx pgx.Tx) error {
		var role string
		err := tx.QueryRow(ctx, `
			SELECT role FROM workspace_members
			 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NotFound("membro não encontrado")
		}
		if err != nil {
			return err
		}
		if role == "owner" {
			owners, err := countOwners(ctx, tx, workspaceID)
			if err != nil {
				return err
			}
			if owners <= 1 {
				return domain.Conflict("last_owner", "não é possível remover o último owner do workspace")
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM workspace_members
			 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID); err != nil {
			return err
		}
		return nil
	})
}

// ChangeRole muda a role de um membro (CA-17). owner|member|viewer aqui
// (promover a owner é OK). Sob lock + re-check: rebaixar o ÚNICO owner →
// last_owner 409 (CA-19). Não-membro → not_found 404 (CA-20). Filtra
// workspace_id (CA-22).
func (s *Service) ChangeRole(ctx context.Context, workspaceID, userID, role string) error {
	if role != "owner" && role != "member" && role != "viewer" {
		return domain.Validation("role deve ser owner, member ou viewer")
	}
	if !canonicalUUID(userID) {
		return domain.NotFound("membro não encontrado")
	}
	return s.withWorkspaceLock(ctx, workspaceID, func(tx pgx.Tx) error {
		var current string
		err := tx.QueryRow(ctx, `
			SELECT role FROM workspace_members
			 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NotFound("membro não encontrado")
		}
		if err != nil {
			return err
		}
		// rebaixar o último owner é proibido (CA-19); promover/manter não.
		if current == "owner" && role != "owner" {
			owners, err := countOwners(ctx, tx, workspaceID)
			if err != nil {
				return err
			}
			if owners <= 1 {
				return domain.Conflict("last_owner", "não é possível rebaixar o último owner do workspace")
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE workspace_members SET role = $3
			 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID, role); err != nil {
			return err
		}
		return nil
	})
}

// ResolveForEmail materializa as memberships dos convites válidos do email no
// 1º login (hook do auth.HandleCallback, D-05). Roda em transação PRÓPRIA
// pós-commit do login, sob advisory lock por user_id; ON CONFLICT DO NOTHING
// (CA-26) e marca os convites accepted. Best-effort: o caller engole o erro.
func (s *Service) ResolveForEmail(ctx context.Context, userID, email string) error {
	em := normEmail(email)
	if em == "" || userID == "" {
		return nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// serializa resoluções concorrentes do MESMO user (re-login simultâneo).
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		WITH inv AS (
			SELECT id, workspace_id, role FROM invitations
			 WHERE lower(email) = $1
			   AND status = 'pending'
			   AND (expires_at IS NULL OR expires_at > now())
			 FOR UPDATE
		), ins AS (
			INSERT INTO workspace_members (workspace_id, user_id, role)
			SELECT workspace_id, $2, role FROM inv
			ON CONFLICT (workspace_id, user_id) DO NOTHING
		)
		UPDATE invitations SET status = 'accepted'
		 WHERE id IN (SELECT id FROM inv)`, em, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// isMemberByEmail verifica se algum membro do workspace tem esse email
// (normalizado) — base de já-membro (CA-07) e do auto-convite. Filtra
// workspace_id (CA-22).
func (s *Service) isMemberByEmail(ctx context.Context, workspaceID, email string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM workspace_members wm
			  JOIN users u ON u.id = wm.user_id
			 WHERE wm.workspace_id = $1 AND lower(u.email) = $2)`,
		workspaceID, email).Scan(&exists)
	return exists, err
}

// withWorkspaceLock roda fn numa transação sob pg_advisory_xact_lock(workspaceID)
// — serializa as mutações do quadro (evita 2 owners saindo ao mesmo tempo →
// 0 owners; CA-18/19). domain.Error volta como está (rollback).
func (s *Service) withWorkspaceLock(ctx context.Context, workspaceID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, workspaceID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// countOwners conta os owners do workspace dentro da transação (re-check do
// invariante sob lock).
func countOwners(ctx context.Context, tx pgx.Tx, workspaceID string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `
		SELECT count(*) FROM workspace_members
		 WHERE workspace_id = $1 AND role = 'owner'`, workspaceID).Scan(&n)
	return n, err
}

// canonicalUUID exige a forma 8-4-4-4-12 (igual ao entries): id não-canônico
// no path responde o MESMO not_found, sem 500 no cast do Postgres.
func canonicalUUID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == strings.ToLower(id)
}

// isUniqueViolation reconhece um 23505 cujo constraint contém o nome dado.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, constraint)
	}
	return false
}
