package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// Fase 18 T-03 — CA-04/05/12: validação de membership do assignee/requested_by.
//   - uuid de user de OUTRO workspace → 400 validation_error (não vaza)
//   - uuid inexistente em users → 400 validation_error (mesma msg genérica)
//   - uuid malformado → 400 validation_error (não 500 — CA-05)
//   - projeto private de outro → 404 (gate de projeto ganha do 400 — CA-12)
//   - membro válido → 200/201
func TestAssigneeValidacaoMembership(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	// C existe mas é de OUTRO workspace (login cria o workspace pessoal dele)
	loginVia(t, svc, stub, "g-c", "c@example.com", "Carla Outra")
	ws := workspaceIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	idC := userIDOf(t, pool, "g-c")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	// membro válido → 201 com assignee gravado
	tkOK := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"OK","assignee":"`+idB+`"}`)
	if tkOK.Assignee == nil || *tkOK.Assignee != idB {
		t.Errorf("assignee de membro válido deveria gravar: %+v", tkOK)
	}

	naoMembro := idC
	inexistente := "00000000-0000-0000-0000-000000000000"
	malformado := "nao-e-uuid"

	// POST com assignee inválido → 400 validation_error e NADA grava
	for nome, uid := range map[string]string{
		"não-membro (outro ws)": naoMembro, "inexistente": inexistente, "malformado": malformado,
	} {
		rec := doJSON(router, http.MethodPost, base, cookieA,
			`{"project_id":"`+p.ID+`","name":"X","assignee":"`+uid+`"}`)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("POST assignee %s: esperado 400 validation_error, veio %d: %s", nome, rec.Code, rec.Body.String())
		}
	}
	// idem requested_by
	rec := doJSON(router, http.MethodPost, base, cookieA,
		`{"project_id":"`+p.ID+`","name":"X","requested_by":"`+naoMembro+`"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("POST requested_by não-membro: esperado 400, veio %d", rec.Code)
	}

	// só a task OK foi gravada
	if n := dbtest.CountRows(t, pool, "tasks"); n != 1 {
		t.Errorf("só a task válida deveria existir, há %d", n)
	}

	// PATCH com assignee inválido → 400
	patchBase := base + "/" + tkOK.ID
	for nome, uid := range map[string]string{
		"não-membro": naoMembro, "inexistente": inexistente, "malformado": malformado,
	} {
		rec := doJSON(router, http.MethodPatch, patchBase, cookieA, `{"assignee":"`+uid+`"}`)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("PATCH assignee %s: esperado 400, veio %d: %s", nome, rec.Code, rec.Body.String())
		}
	}

	// CA-12: setar assignee numa task de projeto PRIVATE de outro → 404 (gate ganha do 400).
	cookieB := loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	privado := criarProjeto(t, router, ws, cookieB, `{"name":"Segredo","color":"#22c55e","visibility":"private"}`)
	tkPriv := criarTask(t, router, ws, cookieB, `{"project_id":"`+privado.ID+`","name":"Secreta"}`)
	// A (não vê o projeto private de B) tenta atribuir → 404 not_found, não 400/403
	rec = doJSON(router, http.MethodPatch, base+"/"+tkPriv.ID, cookieA, `{"assignee":"`+idB+`"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH assignee em task de projeto private de outro: esperado 404, veio %d: %s", rec.Code, rec.Body.String())
	}
}

// Fase 18 (verificação do build) — CA-04 refinado: o PATCH só revalida a membership
// quando o assignee/requested_by MUDA de valor. Reenviar o valor ATUAL (o TaskModal
// sempre reenvia os dois campos, inicializados de task.assignee/requested_by) de um
// membro que SAIU do workspace (fase 17 RemoveMember deleta só a linha de
// workspace_members; tasks.assignee segue apontando pro users row) NÃO pode falhar —
// senão a task vira INEDITÁVEL pela única UI que a edita. Mas MUDAR pra um novo
// não-membro segue 400 (CA-04 preservado).
func TestPatchAssigneeInalteradoNaoRevalida(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA,
		`{"project_id":"`+p.ID+`","name":"T","assignee":"`+idB+`"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks/" + tk.ID

	// B sai do workspace (fase 17 RemoveMember deleta só a linha de workspace_members)
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, ws, idB); err != nil {
		t.Fatal(err)
	}

	// o TaskModal reenvia o assignee atual (B, agora ex-membro) ao editar o nome →
	// deve PASSAR (200), não 400, porque o assignee NÃO mudou.
	rec := doJSON(router, http.MethodPatch, base, cookieA,
		`{"name":"Renomeada","assignee":"`+idB+`","requested_by":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH reenviando assignee inalterado (ex-membro): esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var v taskJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Assignee == nil || *v.Assignee != idB || v.Name != "Renomeada" {
		t.Errorf("assignee inalterado deveria seguir B e o nome ter mudado: %+v", v)
	}

	// MAS mudar o assignee pra um NOVO não-membro ainda deve falhar 400 (CA-04).
	loginVia(t, svc, stub, "g-c", "c@example.com", "Carla Outra")
	idC := userIDOf(t, pool, "g-c")
	rec = doJSON(router, http.MethodPatch, base, cookieA, `{"assignee":"`+idC+`"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("mudar assignee pra NÃO-membro deveria dar 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
}
