package auth

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/invitations"
)

// Fase 17 T-06 — resolução de convites no 1º login PELO callback real
// (auth.Service.HandleCallback → cfg.ResolveInvites). A prova é PELO callback
// (RNF-05): uma mutação que remove a chamada a ResolveInvites do HandleCallback
// mata estes testes (um teste só de ResolveForEmail isolado não cobriria).

// stubServiceComResolver é o stubServiceComDB com o hook de resolução ligado
// num invitations.Service real (espelha o cmd/server).
func stubServiceComResolver(t *testing.T, pool *pgxpool.Pool) (*Service, *googlestub.Server) {
	t.Helper()
	stub := googlestub.New(fixturesDir)
	t.Cleanup(stub.Close)

	invSvc := invitations.NewService(pool)
	svc := NewService(Config{
		ClientID:       "client-id-teste",
		ClientSecret:   "client-secret-teste",
		RedirectURL:    "http://localhost:8080/auth/google/callback",
		TokenKey:       testTokenKey,
		Endpoint:       stub.Endpoint(),
		UserinfoURL:    stub.UserinfoURL(),
		DB:             pool,
		ResolveInvites: invSvc.ResolveForEmail,
	})
	return svc, stub
}

// seedOwnerWorkspace cria um owner + workspace e devolve o workspace_id (o
// convidador). Usa o store da fase 01 indiretamente por SQL.
func seedOwnerWorkspace(t *testing.T, pool *pgxpool.Pool, name string) (ownerID, wsID string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-inv-owner', 'invowner@b.c', 'Owner') RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, wsID, ownerID); err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	return ownerID, wsID
}

// seedInvitation insere um convite pending (token único) e devolve o id.
func seedInvitation(t *testing.T, pool *pgxpool.Pool, wsID, invitedBy, email, role, token, status string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO invitations (workspace_id, email, role, token, invited_by, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '14 days')
		RETURNING id`, wsID, email, role, token, invitedBy, status).Scan(&id); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return id
}

func roleInWorkspace(t *testing.T, pool *pgxpool.Pool, wsID, userID string) string {
	t.Helper()
	var role string
	if err := pool.QueryRow(context.Background(),
		`SELECT role FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, wsID, userID).Scan(&role); err != nil {
		return ""
	}
	return role
}

func inviteStatus(t *testing.T, pool *pgxpool.Pool, inviteID string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM invitations WHERE id=$1`, inviteID).Scan(&s); err != nil {
		t.Fatalf("status do convite: %v", err)
	}
	return s
}

// CA-24/RNF-05 — login do email convidado materializa a membership com a role
// do convite, e o convite vira accepted. Prova PELO HandleCallback real (a
// mutação que remove a chamada a ResolveInvites mata este teste).
func TestResolveConviteNoLogin(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComResolver(t, pool)
	owner, ws := seedOwnerWorkspace(t, pool, "WS do Owner")
	inviteID := seedInvitation(t, pool, ws, owner, "cadu@example.com", "member", "tok-resolve-1", "pending")

	// 1º login do convidado (email do convite, case/espaço variados pra provar
	// a normalização CA-24).
	stub.SetUserinfo(userinfoJSON("g-cadu", "  Cadu@Example.COM ", "Cadu", ""))
	loginStubbado(t, svc)

	var caduID string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE google_id='g-cadu'`).Scan(&caduID); err != nil {
		t.Fatalf("user cadu não foi criado no login: %v", err)
	}
	if got := roleInWorkspace(t, pool, ws, caduID); got != "member" {
		t.Errorf("membership do convite não materializada, role=%q", got)
	}
	if got := inviteStatus(t, pool, inviteID); got != "accepted" {
		t.Errorf("convite deveria virar accepted, veio %q", got)
	}
}

// CA-25 — convite expirado/revogado NÃO materializa no login.
func TestResolveConviteExpiradoRevogado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComResolver(t, pool)
	owner, ws := seedOwnerWorkspace(t, pool, "WS")
	ctx := context.Background()

	// revogado.
	seedInvitation(t, pool, ws, owner, "rev@example.com", "member", "tok-rev", "revoked")
	// expirado: insere e força expires_at no passado.
	expID := seedInvitation(t, pool, ws, owner, "exp@example.com", "member", "tok-exp", "pending")
	if _, err := pool.Exec(ctx, `UPDATE invitations SET expires_at = now() - interval '1 day' WHERE id=$1`, expID); err != nil {
		t.Fatalf("forçar expiração: %v", err)
	}

	// login do revogado → sem membership.
	stub.SetUserinfo(userinfoJSON("g-rev", "rev@example.com", "Rev", ""))
	loginStubbado(t, svc)
	var revID string
	pool.QueryRow(ctx, `SELECT id FROM users WHERE google_id='g-rev'`).Scan(&revID)
	if got := roleInWorkspace(t, pool, ws, revID); got != "" {
		t.Errorf("convite revogado NÃO deveria materializar, role=%q", got)
	}

	// login do expirado → sem membership; convite segue pending (inerte).
	stub.SetUserinfo(userinfoJSON("g-exp", "exp@example.com", "Exp", ""))
	loginStubbado(t, svc)
	var expUserID string
	pool.QueryRow(ctx, `SELECT id FROM users WHERE google_id='g-exp'`).Scan(&expUserID)
	if got := roleInWorkspace(t, pool, ws, expUserID); got != "" {
		t.Errorf("convite expirado NÃO deveria materializar, role=%q", got)
	}
	if got := inviteStatus(t, pool, expID); got != "pending" {
		t.Errorf("convite expirado deveria seguir pending (inerte), veio %q", got)
	}
}

// CA-26 — user que loga já sendo membro + convite pending: não duplica
// membership e MESMO ASSIM marca o convite accepted (consumido).
func TestResolveJaMembroNaoDuplica(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComResolver(t, pool)
	owner, ws := seedOwnerWorkspace(t, pool, "WS")
	ctx := context.Background()

	// cria o convidado ANTES e já o torna membro 'viewer' do ws.
	var caduID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-cadu', 'cadu@example.com', 'Cadu') RETURNING id`).Scan(&caduID); err != nil {
		t.Fatalf("seed cadu: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'viewer')`, ws, caduID); err != nil {
		t.Fatalf("seed membership prévia: %v", err)
	}
	inviteID := seedInvitation(t, pool, ws, owner, "cadu@example.com", "member", "tok-dup", "pending")

	// agora o cadu loga (re-vínculo via email já mapeado; google_id já existe).
	stub.SetUserinfo(userinfoJSON("g-cadu", "cadu@example.com", "Cadu", ""))
	loginStubbado(t, svc)

	// 1 só membership (a role original 'viewer' fica — ON CONFLICT DO NOTHING).
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, ws, caduID).Scan(&n)
	if n != 1 {
		t.Errorf("membership não pode duplicar, há %d", n)
	}
	if got := roleInWorkspace(t, pool, ws, caduID); got != "viewer" {
		t.Errorf("role original deveria ser preservada (DO NOTHING), veio %q", got)
	}
	// mesmo assim o convite é consumido.
	if got := inviteStatus(t, pool, inviteID); got != "accepted" {
		t.Errorf("convite deveria virar accepted mesmo com já-membro, veio %q", got)
	}
}

// CA-27 — login sem convites pendentes segue idêntico: cria user + workspace
// pessoal + membership owner, sem efeito colateral da fase 17.
func TestLoginSemConvitesNaoRegride(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComResolver(t, pool)

	stub.SetUserinfo(userinfoJSON("g-solo", "solo@example.com", "Solo", ""))
	loginStubbado(t, svc)

	var role, wsName string
	if err := pool.QueryRow(context.Background(), `
		SELECT m.role, w.name FROM workspace_members m
		  JOIN workspaces w ON w.id = m.workspace_id
		  JOIN users u ON u.id = m.user_id
		 WHERE u.google_id = 'g-solo'`).Scan(&role, &wsName); err != nil {
		t.Fatalf("login solo não criou workspace pessoal: %v", err)
	}
	if role != "owner" {
		t.Errorf("login solo deveria ser owner do próprio ws, veio %q", role)
	}
	if wsName != "Workspace de Solo" {
		t.Errorf("workspace pessoal esperado 'Workspace de Solo', veio %q", wsName)
	}
}

// CA-28 — re-vínculo por email (google_id novo, email conhecido) também resolve
// convites: um user antigo que recriou a conta Google e tinha convite pendente
// entra no ws ao re-logar.
func TestResolveConviteNoReVinculo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc, stub := stubServiceComResolver(t, pool)
	owner, ws := seedOwnerWorkspace(t, pool, "WS")
	ctx := context.Background()

	// user antigo com google_id velho, ainda SEM membership no ws do owner.
	var velhoID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-velho', 'recriado@example.com', 'Velho') RETURNING id`).Scan(&velhoID); err != nil {
		t.Fatalf("seed user antigo: %v", err)
	}
	inviteID := seedInvitation(t, pool, ws, owner, "recriado@example.com", "member", "tok-revinc", "pending")

	// re-login com google_id NOVO mas mesmo email → re-vincula + resolve convite.
	stub.SetUserinfo(userinfoJSON("g-novo", "recriado@example.com", "Recriado", ""))
	loginStubbado(t, svc)

	if got := roleInWorkspace(t, pool, ws, velhoID); got != "member" {
		t.Errorf("convite no re-vínculo deveria materializar, role=%q", got)
	}
	if got := inviteStatus(t, pool, inviteID); got != "accepted" {
		t.Errorf("convite deveria virar accepted no re-vínculo, veio %q", got)
	}
}
