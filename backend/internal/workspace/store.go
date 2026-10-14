// Package workspace: queries de users, workspaces e memberships (fase 01).
// As fases 02/03 penduram seus domínios no workspace validado daqui.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GoogleProfile é o perfil mínimo vindo do userinfo do Google.
// (Struct própria pra não importar o pacote auth — api importa auth,
// auth importa workspace; workspace é folha.)
type GoogleProfile struct {
	GoogleID  string
	Email     string
	Name      string
	AvatarURL string
}

// EncryptedToken são os tokens do Google JÁ cifrados (AES-256-GCM no
// pacote auth) prontos pra repouso em oauth_tokens (CA-14).
type EncryptedToken struct {
	AccessTokenEnc  []byte
	RefreshTokenEnc []byte // nil quando o Google não devolveu refresh (CA-14.1)
	ExpiresAt       time.Time
	Scopes          []string
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// UpsertUserFromGoogle materializa o login (CA-04/CA-05) numa única transação:
//   - google_id conhecido → atualiza name/avatar_url, nada é criado;
//   - email conhecido com google_id novo (conta Google recriada) →
//     re-vincula o google_id ao user existente, nada é criado;
//   - desconhecido → cria user + workspace pessoal + membership owner.
//
// O upsert de oauth_tokens entra na MESMA transação (plan §2), com
// COALESCE não-destrutivo no refresh_token_enc (CA-14.1).
func (s *Store) UpsertUserFromGoogle(ctx context.Context, p GoogleProfile, tok EncryptedToken) (userID string, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("abrir transação de login: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// serializa logins concorrentes do MESMO usuário: dois callbacks
	// simultâneos de 1º login caíam juntos no INSERT (unique_violation em
	// google_id → 500). Lock consultivo por google_id, solto no fim da
	// transação; usuários diferentes não se bloqueiam.
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1))`, p.GoogleID); err != nil {
		return "", fmt.Errorf("serializar login: %w", err)
	}

	// login repetido: google_id já existe (CA-05)
	err = tx.QueryRow(ctx, `
		UPDATE users SET name = $2, avatar_url = $3
		 WHERE google_id = $1
		 RETURNING id`, p.GoogleID, p.Name, p.AvatarURL).Scan(&userID)
	if err != nil && err != pgx.ErrNoRows {
		return "", fmt.Errorf("atualizar user por google_id: %w", err)
	}

	if err == pgx.ErrNoRows {
		// conta Google recriada: mesmo email, google_id novo → re-vincula
		err = tx.QueryRow(ctx, `
			UPDATE users SET google_id = $1, name = $2, avatar_url = $3
			 WHERE email = $4
			 RETURNING id`, p.GoogleID, p.Name, p.AvatarURL, p.Email).Scan(&userID)
		if err != nil && err != pgx.ErrNoRows {
			return "", fmt.Errorf("re-vincular google_id por email: %w", err)
		}
	}

	if err == pgx.ErrNoRows {
		// 1º login: user + workspace pessoal + owner, tudo ou nada (CA-04)
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (google_id, email, name, avatar_url)
			VALUES ($1, $2, $3, $4)
			RETURNING id`, p.GoogleID, p.Email, p.Name, p.AvatarURL).Scan(&userID); err != nil {
			return "", fmt.Errorf("criar user: %w", err)
		}
		var wsID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO workspaces (name) VALUES ($1)
			RETURNING id`, personalWorkspaceName(p.Name)).Scan(&wsID); err != nil {
			return "", fmt.Errorf("criar workspace pessoal: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO workspace_members (workspace_id, user_id, role)
			VALUES ($1, $2, 'owner')`, wsID, userID); err != nil {
			return "", fmt.Errorf("criar membership owner: %w", err)
		}
	}

	// scopes = UNIÃO dos antigos com os novos (D-02.1 da fase 06): um login
	// normal DEPOIS do connect do calendar reflete só os scopes do pedido
	// atual no campo `scope` do token; substituir total derrubaria o
	// calendar.readonly e desconectaria o Calendar a cada login (CA-03.2).
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_tokens
			(user_id, provider, access_token_enc, refresh_token_enc, token_expires_at, scopes, updated_at)
		VALUES ($1, 'google', $2, $3, $4, $5, now())
		ON CONFLICT (user_id, provider) DO UPDATE SET
			access_token_enc  = EXCLUDED.access_token_enc,
			refresh_token_enc = COALESCE(EXCLUDED.refresh_token_enc, oauth_tokens.refresh_token_enc),
			token_expires_at  = EXCLUDED.token_expires_at,
			scopes            = (SELECT array_agg(DISTINCT s)
			                       FROM unnest(oauth_tokens.scopes || EXCLUDED.scopes) AS s),
			updated_at        = now()`,
		userID, tok.AccessTokenEnc, tok.RefreshTokenEnc, tok.ExpiresAt, tok.Scopes); err != nil {
		return "", fmt.Errorf("gravar oauth_tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit do login: %w", err)
	}
	return userID, nil
}

// User é o perfil exposto pelo GET /api/v1/me (contrato §3.3 da fase 01).
type User struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

// Membership é um workspace do usuário com a role dele.
type Membership struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// UserWithWorkspaces carrega o user e seus workspaces+role (CA-10).
func (s *Store) UserWithWorkspaces(ctx context.Context, userID string) (User, []Membership, error) {
	var u User
	if err := s.db.QueryRow(ctx, `
		SELECT id, email, name, avatar_url FROM users WHERE id = $1`, userID).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL); err != nil {
		return User{}, nil, fmt.Errorf("carregar user: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT w.id, w.name, m.role
		  FROM workspace_members m
		  JOIN workspaces w ON w.id = m.workspace_id
		 WHERE m.user_id = $1
		 ORDER BY w.created_at`, userID)
	if err != nil {
		return User{}, nil, fmt.Errorf("carregar workspaces: %w", err)
	}
	defer rows.Close()

	memberships := []Membership{}
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.ID, &m.Name, &m.Role); err != nil {
			return User{}, nil, err
		}
		memberships = append(memberships, m)
	}
	return u, memberships, rows.Err()
}

// MemberRole devolve a role do user no workspace, ou "" quando não é
// membro — workspace inexistente ou id malformado respondem IGUAL (não
// vazar existência). Usado pelo handler de timer/start (fase 03), cuja
// rota é por usuário e não passa pelo RequireMember.
func (s *Store) MemberRole(ctx context.Context, workspaceID, userID string) (string, error) {
	if uuid.Validate(workspaceID) != nil {
		return "", nil
	}
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT role FROM workspace_members
		 WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// StoredToken são os tokens do Google cifrados em repouso, como saem do
// oauth_tokens (fase 06: caminho de LEITURA, espelho do EncryptedToken).
type StoredToken struct {
	AccessTokenEnc  []byte
	RefreshTokenEnc []byte // nil quando ausente
	ExpiresAt       time.Time
	Scopes          []string
}

// LoadGoogleToken carrega o token Google cifrado do usuário (fase 06). ok=false
// (sem erro) quando não há linha — usuário ainda não conectou nada.
func (s *Store) LoadGoogleToken(ctx context.Context, userID string) (StoredToken, bool, error) {
	var t StoredToken
	var expires *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT access_token_enc, refresh_token_enc, token_expires_at, scopes
		  FROM oauth_tokens
		 WHERE user_id = $1 AND provider = 'google'`, userID).
		Scan(&t.AccessTokenEnc, &t.RefreshTokenEnc, &expires, &t.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredToken{}, false, nil
	}
	if err != nil {
		return StoredToken{}, false, fmt.Errorf("carregar oauth_tokens: %w", err)
	}
	if expires != nil {
		t.ExpiresAt = *expires
	}
	return t, true, nil
}

// SaveGoogleAccessToken persiste SÓ o access renovado + expiry pós-refresh
// (CA-09): NÃO toca refresh_token_enc nem scopes (a resposta de refresh do
// Google omite ambos; re-derivar zeraria o calendar.readonly).
func (s *Store) SaveGoogleAccessToken(ctx context.Context, userID string, accessEnc []byte, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE oauth_tokens
		   SET access_token_enc = $2, token_expires_at = $3, updated_at = now()
		 WHERE user_id = $1 AND provider = 'google'`, userID, accessEnc, expiresAt)
	if err != nil {
		return fmt.Errorf("atualizar access token: %w", err)
	}
	return nil
}

// personalWorkspaceName — "Workspace de <primeiro nome>", com fallback
// "Workspace pessoal" quando o Google não manda nome (D-10).
func personalWorkspaceName(fullName string) string {
	fields := strings.Fields(fullName)
	if len(fields) == 0 {
		return "Workspace pessoal"
	}
	return "Workspace de " + fields[0]
}
