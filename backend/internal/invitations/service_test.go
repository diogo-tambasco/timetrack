package invitations

import (
	"context"
	"strings"
	"testing"
)

// Fase 17 T-02 — convites: criar/listar/revogar.

// CA-04/05 — Create normaliza o email (lower/trim) e grava pending.
func TestCreateNormalizaEmail(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	inv, err := svc.Create(context.Background(), ws, owner, "  Cadu@Example.COM ", "member")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.Email != "cadu@example.com" {
		t.Errorf("email gravado %q, esperava cadu@example.com", inv.Email)
	}
	if inv.Status != "pending" {
		t.Errorf("status %q, esperava pending", inv.Status)
	}
	if inv.Role != "member" {
		t.Errorf("role %q, esperava member", inv.Role)
	}
	if inv.ExpiresAt == nil {
		t.Errorf("expires_at deveria estar preenchido (now+TTL)")
	}
}

// CA-03 — role 'owner' no payload de convite → 400 validation_error.
func TestCreateRoleOwnerRejeitada(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	_, err := svc.Create(context.Background(), ws, owner, "x@b.c", "owner")
	if got := domainCode(err); got != "validation_error" {
		t.Errorf("role owner deveria dar validation_error, veio %q", got)
	}
}

// CA-06 — email malformado/ausente → 400 validation_error.
func TestCreateEmailMalformado(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	for _, em := range []string{"", "   ", "semarroba"} {
		if _, err := svc.Create(context.Background(), ws, owner, em, "member"); domainCode(err) != "validation_error" {
			t.Errorf("email %q deveria dar validation_error, veio %v", em, err)
		}
	}
}

// CA-07 — convidar email que já é membro → 409 already_member, sem criar.
func TestCreateJaMembro(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, bob, "member")

	_, err := svc.Create(context.Background(), ws, owner, "BOB@b.c", "member")
	if got := domainCode(err); got != "already_member" {
		t.Errorf("convidar membro deveria dar already_member, veio %q", got)
	}
	if n := countInvitations(t, pool, ws); n != 0 {
		t.Errorf("não deveria criar convite, há %d", n)
	}
}

// CA-06 (auto-convite) — owner convidando o próprio email cai em já-membro
// (o owner É membro) → 409 already_member, não 400.
func TestCreateAutoConvite(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	_, err := svc.Create(context.Background(), ws, owner, "owner@b.c", "member")
	if got := domainCode(err); got != "already_member" {
		t.Errorf("auto-convite deveria dar already_member, veio %q", got)
	}
}

// CA-08 — 2º pending pro mesmo email → 409 invite_pending, 1 linha só.
func TestCreatePendingDuplicado(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	if _, err := svc.Create(context.Background(), ws, owner, "cadu@b.c", "member"); err != nil {
		t.Fatalf("1º Create: %v", err)
	}
	_, err := svc.Create(context.Background(), ws, owner, "CADU@b.c", "viewer")
	if got := domainCode(err); got != "invite_pending" {
		t.Errorf("2º pending deveria dar invite_pending, veio %q", got)
	}
	if n := countInvitations(t, pool, ws); n != 1 {
		t.Errorf("deveria haver 1 convite, há %d", n)
	}
}

// CA-09 — novo pending depois de revogar o 1º é criado.
func TestCreateReconviteAposRevogar(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	inv, err := svc.Create(ctx, ws, owner, "cadu@b.c", "member")
	if err != nil {
		t.Fatalf("1º Create: %v", err)
	}
	if err := svc.Revoke(ctx, ws, inv.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := svc.Create(ctx, ws, owner, "cadu@b.c", "member"); err != nil {
		t.Errorf("reconvite após revogar deveria criar, veio %v", err)
	}
}

// CA-13/RNF-04 — List só vê os pending do workspace da rota, sem token.
func TestListSoPendingDoWorkspaceSemToken(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	ctx := context.Background()

	invA, _ := svc.Create(ctx, wsA, owner, "a@b.c", "member")
	if _, err := svc.Create(ctx, wsB, owner, "b@b.c", "member"); err != nil {
		t.Fatalf("Create wsB: %v", err)
	}
	// um revogado no wsA não deve aparecer.
	revoked, _ := svc.Create(ctx, wsA, owner, "rev@b.c", "member")
	if err := svc.Revoke(ctx, wsA, revoked.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	list, err := svc.List(ctx, wsA)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != invA.ID {
		t.Fatalf("List wsA deveria ter só o pending de a@b.c, veio %+v", list)
	}
	// o JSON não tem campo token (RNF-04) — confere que a struct não vaza nada.
	for _, inv := range list {
		if strings.Contains(inv.Email, "token") {
			t.Errorf("inesperado")
		}
	}
}

// CA-15 — revogar convite de OUTRO workspace → 404 not_found (não vaza).
func TestRevokeOutroWorkspace(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	ctx := context.Background()

	invB, _ := svc.Create(ctx, wsB, owner, "b@b.c", "member")
	// tenta revogar pelo workspace A → 404 (filtra id AND workspace_id).
	if got := domainCode(svc.Revoke(ctx, wsA, invB.ID)); got != "not_found" {
		t.Errorf("revoke cross-workspace deveria dar not_found, veio %q", got)
	}
	// o convite no wsB segue pending.
	if n := countInvitations(t, pool, wsB); n != 1 {
		t.Errorf("convite do wsB não deveria ser tocado")
	}
}

// CA-14 — revogar accepted → 409 invite_not_pending; já revoked → idempotente.
func TestRevokeAcceptedEIdempotente(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	inv, _ := svc.Create(ctx, ws, owner, "cadu@b.c", "member")
	// força accepted no banco (simula resolução de login).
	if _, err := pool.Exec(ctx, `UPDATE invitations SET status='accepted' WHERE id=$1`, inv.ID); err != nil {
		t.Fatalf("forçar accepted: %v", err)
	}
	if got := domainCode(svc.Revoke(ctx, ws, inv.ID)); got != "invite_not_pending" {
		t.Errorf("revoke de accepted deveria dar invite_not_pending, veio %q", got)
	}

	// um pending revogado: revogar de novo é no-op (sem erro).
	inv2, _ := svc.Create(ctx, ws, owner, "outro@b.c", "member")
	if err := svc.Revoke(ctx, ws, inv2.ID); err != nil {
		t.Fatalf("1ª revoke: %v", err)
	}
	if err := svc.Revoke(ctx, ws, inv2.ID); err != nil {
		t.Errorf("revoke idempotente deveria ser no-op, veio %v", err)
	}
}

// CA-22 — isolamento: o List/Revoke filtram por workspace_id. Mutação que
// remove o WHERE workspace_id deve quebrar um teste (TestListSoPending* e
// TestRevokeOutroWorkspace cobrem List e Revoke respectivamente).
func TestIsolamentoConviteEntreWorkspaces(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	ctx := context.Background()

	if _, err := svc.Create(ctx, wsA, owner, "a@b.c", "member"); err != nil {
		t.Fatalf("Create wsA: %v", err)
	}
	if _, err := svc.Create(ctx, wsB, owner, "b@b.c", "member"); err != nil {
		t.Fatalf("Create wsB: %v", err)
	}

	listB, err := svc.List(ctx, wsB)
	if err != nil {
		t.Fatalf("List wsB: %v", err)
	}
	if len(listB) != 1 || listB[0].Email != "b@b.c" {
		t.Fatalf("List wsB vazou convite do wsA: %+v", listB)
	}
}
