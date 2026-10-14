package invitations

import (
	"context"
	"sync"
	"testing"
)

// Fase 17 T-03 — membros: add existente / remover / mudar role + invariante
// do último owner.

// CA-10 — add de quem já tem conta cria a membership na hora.
func TestAddExistingCriaMembership(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	m, err := svc.AddExistingMember(context.Background(), ws, "BOB@b.c", "member")
	if err != nil {
		t.Fatalf("AddExistingMember: %v", err)
	}
	if m.UserID != bob || m.Role != "member" || m.Email != "bob@b.c" {
		t.Errorf("retorno inesperado: %+v", m)
	}
	if got := memberRole(t, pool, ws, bob); got != "member" {
		t.Errorf("membership não criada, role=%q", got)
	}
}

// CA-11 — add de quem não tem conta → 404 user_not_found (code custom).
func TestAddExistingInexistente(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	_, err := svc.AddExistingMember(context.Background(), ws, "ninguem@b.c", "member")
	if got := domainCode(err); got != "user_not_found" {
		t.Errorf("add inexistente deveria dar user_not_found, veio %q", got)
	}
}

// CA-12 — add de quem já é membro → 409 already_member, sem 2ª linha.
func TestAddExistingJaMembro(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, bob, "viewer")

	_, err := svc.AddExistingMember(context.Background(), ws, "bob@b.c", "member")
	if got := domainCode(err); got != "already_member" {
		t.Errorf("add já-membro deveria dar already_member, veio %q", got)
	}
	// role não muda (continua viewer — AddExistingMember não rebaixa/promove).
	if got := memberRole(t, pool, ws, bob); got != "viewer" {
		t.Errorf("role do membro não deveria mudar, veio %q", got)
	}
}

// CA-10 — role owner no payload de add → 400.
func TestAddExistingRoleOwnerRejeitada(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	if _, err := svc.AddExistingMember(context.Background(), ws, "bob@b.c", "owner"); domainCode(err) != "validation_error" {
		t.Errorf("add com role owner deveria dar validation_error, veio %v", err)
	}
}

// CA-16 — remover um membro comum funciona.
func TestRemoveMembroComum(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, bob, "member")

	if err := svc.RemoveMember(context.Background(), ws, bob); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if got := memberRole(t, pool, ws, bob); got != "" {
		t.Errorf("membro deveria ter sido removido, role=%q", got)
	}
}

// CA-20 — remover/patch de userId não-membro → 404 not_found.
func TestRemovePatchNaoMembro(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ghost := seedUser(t, pool, "g-g", "ghost@b.c", "Ghost") // existe, mas não é membro
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	if got := domainCode(svc.RemoveMember(ctx, ws, ghost)); got != "not_found" {
		t.Errorf("remover não-membro deveria dar not_found, veio %q", got)
	}
	if got := domainCode(svc.ChangeRole(ctx, ws, ghost, "member")); got != "not_found" {
		t.Errorf("patch de não-membro deveria dar not_found, veio %q", got)
	}
}

// CA-18 — remover o ÚNICO owner → 409 last_owner, sem remover.
func TestRemoveUltimoOwner(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	if got := domainCode(svc.RemoveMember(context.Background(), ws, owner)); got != "last_owner" {
		t.Errorf("remover único owner deveria dar last_owner, veio %q", got)
	}
	if got := memberRole(t, pool, ws, owner); got != "owner" {
		t.Errorf("owner não deveria ter sido removido, role=%q", got)
	}
}

// CA-19 — rebaixar o ÚNICO owner → 409 last_owner, sem alterar.
func TestRebaixarUltimoOwner(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")

	if got := domainCode(svc.ChangeRole(context.Background(), ws, owner, "member")); got != "last_owner" {
		t.Errorf("rebaixar único owner deveria dar last_owner, veio %q", got)
	}
	if got := memberRole(t, pool, ws, owner); got != "owner" {
		t.Errorf("owner não deveria ter sido rebaixado, role=%q", got)
	}
}

// CA-17/19 — promover um 2º owner DEPOIS rebaixar o 1º é OK (há 2 owners no
// meio do caminho).
func TestPromoverDepoisRebaixar(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, bob, "member")
	ctx := context.Background()

	if err := svc.ChangeRole(ctx, ws, bob, "owner"); err != nil {
		t.Fatalf("promover bob a owner: %v", err)
	}
	if err := svc.ChangeRole(ctx, ws, owner, "member"); err != nil {
		t.Errorf("rebaixar o 1º owner com 2 owners deveria ser OK, veio %v", err)
	}
	if got := memberRole(t, pool, ws, owner); got != "member" {
		t.Errorf("owner deveria ter sido rebaixado a member, role=%q", got)
	}
}

// CA-17 — role fora do enum → 400 validation_error.
func TestChangeRoleInvalida(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, bob, "member")

	if got := domainCode(svc.ChangeRole(context.Background(), ws, bob, "gerente")); got != "validation_error" {
		t.Errorf("role fora do enum deveria dar validation_error, veio %q", got)
	}
}

// CA-18 (concorrência) — 2 RemoveMember do MESMO owner em paralelo: o lock +
// re-check garantem no máximo 1 sucesso, e o workspace NUNCA fica sem owner.
// Com 2 owners, exatamente um pode sair; o outro vê owners=1 e é barrado.
func TestRemoveOwnerConcorrente(t *testing.T) {
	svc, pool := newService(t)
	ownerA := seedUser(t, pool, "g-oa", "oa@b.c", "OwnerA")
	ownerB := seedUser(t, pool, "g-ob", "ob@b.c", "OwnerB")
	ws := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, ws, ownerA, "owner")
	seedMember(t, pool, ws, ownerB, "owner")
	ctx := context.Background()

	// martelado N× pra cobrir a janela de corrida.
	for i := 0; i < 25; i++ {
		// re-seed pra cada iteração: 2 owners frescos.
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_members WHERE workspace_id=$1`, ws)
		seedMember(t, pool, ws, ownerA, "owner")
		seedMember(t, pool, ws, ownerB, "owner")

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); errs[0] = svc.RemoveMember(ctx, ws, ownerA) }()
		go func() { defer wg.Done(); errs[1] = svc.RemoveMember(ctx, ws, ownerB) }()
		wg.Wait()

		var owners int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND role='owner'`, ws).Scan(&owners); err != nil {
			t.Fatalf("contar owners: %v", err)
		}
		if owners < 1 {
			t.Fatalf("iteração %d: workspace ficou SEM owner (errs=%v)", i, errs)
		}
	}
}

// CA-22 — isolamento: remover/patch filtram por workspace_id. Remover o owner
// de um WS pela rota de OUTRO WS não acha (404), e não toca o membership real.
func TestIsolamentoMembroEntreWorkspaces(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	seedMember(t, pool, wsB, bob, "member") // bob é membro SÓ do B
	ctx := context.Background()

	// remover bob pela rota do A → 404 (ele não é membro do A).
	if got := domainCode(svc.RemoveMember(ctx, wsA, bob)); got != "not_found" {
		t.Errorf("remover membro de outro ws deveria dar not_found, veio %q", got)
	}
	// bob continua membro do B.
	if got := memberRole(t, pool, wsB, bob); got != "member" {
		t.Errorf("membership do wsB não deveria ser tocada, role=%q", got)
	}
}

// CA-22 (verificação adversarial do build) — o WHERE workspace_id do UPDATE de
// ChangeRole tem que ser load-bearing: com bob membro de AMBOS os workspaces,
// rebaixá-lo no A NÃO pode alterar a role dele no B. Sem este teste, uma mutação
// que afrouxe o filtro (ex.: `workspace_id = $1 OR true`) sobrevive à suíte (o
// SELECT-gate de 404 protege os outros testes, que semeiam o membro num ws só).
func TestChangeRoleIsolaWorkspace(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	seedMember(t, pool, wsA, bob, "member") // bob é membro de AMBOS
	seedMember(t, pool, wsB, bob, "member")
	ctx := context.Background()

	if err := svc.ChangeRole(ctx, wsA, bob, "viewer"); err != nil {
		t.Fatalf("ChangeRole no A: %v", err)
	}
	if got := memberRole(t, pool, wsA, bob); got != "viewer" {
		t.Errorf("role de bob no A deveria virar viewer, veio %q", got)
	}
	if got := memberRole(t, pool, wsB, bob); got != "member" {
		t.Errorf("VAZAMENTO cross-ws: role de bob no B mudou pra %q (deveria seguir member)", got)
	}
}

// CA-22 (verificação adversarial do build) — simétrico ao de cima pro DELETE de
// RemoveMember: com bob membro de AMBOS, removê-lo do A NÃO pode tirá-lo do B.
func TestRemoveMembroIsolaWorkspace(t *testing.T) {
	svc, pool := newService(t)
	owner := seedUser(t, pool, "g-o", "owner@b.c", "Owner")
	bob := seedUser(t, pool, "g-b", "bob@b.c", "Bob")
	wsA := seedWorkspace(t, pool, "WS A")
	wsB := seedWorkspace(t, pool, "WS B")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	seedMember(t, pool, wsA, bob, "member") // bob é membro de AMBOS
	seedMember(t, pool, wsB, bob, "member")
	ctx := context.Background()

	if err := svc.RemoveMember(ctx, wsA, bob); err != nil {
		t.Fatalf("RemoveMember no A: %v", err)
	}
	if got := memberRole(t, pool, wsA, bob); got != "" {
		t.Errorf("bob deveria ter saído do A, role=%q", got)
	}
	if got := memberRole(t, pool, wsB, bob); got != "member" {
		t.Errorf("VAZAMENTO cross-ws: bob foi removido do B (role=%q, deveria seguir member)", got)
	}
}
