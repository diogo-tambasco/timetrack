package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// Suporte à T-18 (CA-28): a seção "Membros do projeto" do modal precisa
// da lista de membros do workspace, e a fase 01 não expôs nenhuma fonte —
// o detalhe do workspace (§3.5) passa a embutir `members` (extensão
// ADITIVA: nenhum campo existente do contrato muda). Decisão registrada
// no handoff do bloco 3.
func TestWorkspaceDetalheListaMembros(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)

	cookieA := loginVia(t, svc, stub, "g-ana", "ana@example.com", "Ana Owner")
	_ = loginVia(t, svc, stub, "g-bruno", "bruno@example.com", "Bruno Member")
	ws := workspaceIDOf(t, pool, "g-ana")
	bruno := userIDOf(t, pool, "g-bruno")
	addMember(t, pool, ws, bruno, "member")

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws, cookieA, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Workspace struct {
			ID      string `json:"id"`
			Members []struct {
				UserID string `json:"user_id"`
				Name   string `json:"name"`
				Email  string `json:"email"`
				Role   string `json:"role"`
			} `json:"members"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}

	if len(body.Workspace.Members) != 2 {
		t.Fatalf("esperados 2 membros (owner + member), vieram %d: %s",
			len(body.Workspace.Members), rec.Body.String())
	}
	// owner primeiro (ordem estável pra UI), depois por nome
	first, second := body.Workspace.Members[0], body.Workspace.Members[1]
	if first.Role != "owner" || first.Name != "Ana Owner" || first.Email != "ana@example.com" {
		t.Errorf("primeiro membro deveria ser a owner com name/email: %+v", first)
	}
	if second.UserID != bruno || second.Role != "member" ||
		second.Name != "Bruno Member" || second.Email != "bruno@example.com" {
		t.Errorf("segundo membro deveria ser o Bruno (member): %+v", second)
	}
}
