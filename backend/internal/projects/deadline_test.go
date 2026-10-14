package projects_test

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/projects"
)

// Fase 28 T-02 (CA-01) — POST com deadline válido persiste e volta no Get;
// PATCH tri-state (null remove, ausente mantém).
func TestProjetoDeadlineCRUD(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	ctx := context.Background()

	// Create com deadline → persiste e volta no corpo.
	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Acme", Color: "#4f9dde", Visibility: "shared", Deadline: ptr("2025-09-30"),
	})
	if err != nil {
		t.Fatalf("create com deadline: %v", err)
	}
	if p.Deadline == nil || *p.Deadline != "2025-09-30" {
		t.Fatalf("deadline no create deveria ser 2025-09-30, veio %v", p.Deadline)
	}

	// Get traz o deadline.
	got, err := svc.Get(ctx, ws, owner, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Deadline == nil || *got.Deadline != "2025-09-30" {
		t.Errorf("Get deveria trazer deadline 2025-09-30, veio %v", got.Deadline)
	}

	// PATCH ausente (sem DeadlineSet) mantém o deadline.
	nome := "Acme 2"
	patched, err := svc.Patch(ctx, ws, owner, "owner", p.ID, projects.PatchInput{Name: &nome})
	if err != nil {
		t.Fatalf("patch sem deadline: %v", err)
	}
	if patched.Deadline == nil || *patched.Deadline != "2025-09-30" {
		t.Errorf("PATCH ausente deveria manter o deadline, veio %v", patched.Deadline)
	}

	// PATCH definindo nova data.
	patched, err = svc.Patch(ctx, ws, owner, "owner", p.ID, projects.PatchInput{DeadlineSet: true, Deadline: ptr("2026-01-15")})
	if err != nil {
		t.Fatalf("patch nova data: %v", err)
	}
	if patched.Deadline == nil || *patched.Deadline != "2026-01-15" {
		t.Errorf("PATCH deveria definir 2026-01-15, veio %v", patched.Deadline)
	}

	// PATCH com null remove.
	patched, err = svc.Patch(ctx, ws, owner, "owner", p.ID, projects.PatchInput{DeadlineSet: true, Deadline: nil})
	if err != nil {
		t.Fatalf("patch remove deadline: %v", err)
	}
	if patched.Deadline != nil {
		t.Errorf("PATCH null deveria remover o deadline, veio %v", *patched.Deadline)
	}
}

// CA-02 — projeto sem deadline = null válido.
func TestProjetoSemDeadlineNull(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	ctx := context.Background()

	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Sem Prazo", Color: "#4f9dde", Visibility: "shared",
	})
	if err != nil {
		t.Fatalf("create sem deadline: %v", err)
	}
	if p.Deadline != nil {
		t.Errorf("projeto sem deadline deveria ser null, veio %v", *p.Deadline)
	}
}

// CA-01.1 — deadline malformado → validation_error, NÃO persiste.
func TestProjetoDeadlineMalformado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	ctx := context.Background()

	for _, dl := range []string{"2025-13-40", "ontem", "30/09/2025", "2025-9-30", ""} {
		_, err := svc.Create(ctx, ws, owner, projects.CreateInput{
			Name: "App", Color: "#4f9dde", Visibility: "shared", Deadline: ptr(dl),
		})
		assertDomainCode(t, err, "validation_error", "Create(deadline "+dl+")")
	}
	if n := dbtest.CountRows(t, pool, "projects"); n != 0 {
		t.Fatalf("nada deveria ter sido persistido, há %d", n)
	}

	// PATCH malformado também é rejeitado e nada muda.
	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared", Deadline: ptr("2025-09-30"),
	})
	if err != nil {
		t.Fatalf("create base: %v", err)
	}
	_, err = svc.Patch(ctx, ws, owner, "owner", p.ID, projects.PatchInput{DeadlineSet: true, Deadline: ptr("2025-13-40")})
	assertDomainCode(t, err, "validation_error", "Patch(deadline malformado)")
	got, _ := svc.Get(ctx, ws, owner, p.ID)
	if got.Deadline == nil || *got.Deadline != "2025-09-30" {
		t.Errorf("PATCH malformado não deveria alterar o deadline, veio %v", got.Deadline)
	}
}

// CA-03 — escrita por usuário sem permissão segue o gate da fase 02 (não muda).
// Viewer (role member, não-dono) → 403; projeto invisível → 404; o deadline
// não altera o caminho de erro.
func TestProjetoDeadlineGateInalterado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	outro := seedUser(t, pool, "g-outro")
	ctx := context.Background()

	// projeto shared do owner; "outro" é membro do ws mas não é dono nem owner-ws.
	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// outro (role member) tenta editar deadline → forbidden (canEditMeta).
	_, err = svc.Patch(ctx, ws, outro, "member", p.ID, projects.PatchInput{DeadlineSet: true, Deadline: ptr("2025-09-30")})
	assertDomainCode(t, err, "forbidden", "Patch deadline por não-dono")

	// projeto private de outro → 404 (invisível) ao tentar editar.
	priv, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Privado", Color: "#4f9dde", Visibility: "private",
	})
	if err != nil {
		t.Fatalf("create private: %v", err)
	}
	_, err = svc.Patch(ctx, ws, outro, "member", priv.ID, projects.PatchInput{DeadlineSet: true, Deadline: ptr("2025-09-30")})
	assertDomainCode(t, err, "not_found", "Patch deadline em projeto invisível")
}

// 28-DEADLINE-F1 (CA-03) — o gate de escrita roda ANTES da validação do deadline:
// um não-dono PATCHando deadline MALFORMADO num projeto que ele só VÊ deve receber
// 403 (gate), NÃO 400 (validação). Pina a ordem; mover validateDeadline pra antes
// de canEditMeta vira 400 (RED).
func TestProjetoDeadlineGateAntesDaValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	outro := seedUser(t, pool, "g-outro")
	ctx := context.Background()

	// projeto shared do owner; "outro" só vê (role member, não-dono).
	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "App", Color: "#4f9dde", Visibility: "shared",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// PATCH com deadline MALFORMADO por não-dono → 403 (gate antes da validação),
	// nunca 400.
	_, err = svc.Patch(ctx, ws, outro, "member", p.ID,
		projects.PatchInput{DeadlineSet: true, Deadline: ptr("2025-13-40")})
	assertDomainCode(t, err, "forbidden", "Patch deadline malformado por não-dono = gate primeiro")
}
