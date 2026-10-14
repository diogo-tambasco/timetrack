package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type clientJSON struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	ArchivedAt  *string `json:"archived_at"`
	CreatedAt   string  `json:"created_at"`
}

// CA-01 — POST /clients com nome válido cria e devolve 201 com o objeto;
// CA-02 na borda HTTP: payload inválido → 400 validation_error.
func TestCriarCliente201(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/clients"

	rec := doJSON(router, http.MethodPost, base, cookie, `{"name":"  Acme  "}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var c clientJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	if c.ID == "" || c.WorkspaceID != ws || c.Name != "Acme" ||
		c.ArchivedAt != nil || c.CreatedAt == "" {
		t.Errorf("objeto fora do contrato §3.1: %+v", c)
	}

	// CA-02 na rota: nome inválido → 400 validation_error
	rec = doJSON(router, http.MethodPost, base, cookie, `{"name":"   "}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("nome inválido: esperado 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}

	// JSON malformado → 400 validation_error (specs/INDEX.md)
	rec = doJSON(router, http.MethodPost, base, cookie, `{nope`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("JSON malformado: esperado 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-03 — GET /clients omite arquivados por default e os inclui só com
// ?archived=true.
func TestListarClientesSemArquivados(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/clients"

	for _, nome := range []string{"Ativo SA", "Antigo SA"} {
		if rec := doJSON(router, http.MethodPost, base, cookie, `{"name":"`+nome+`"}`); rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d %s", nome, rec.Code, rec.Body.String())
		}
	}
	// arquiva o "Antigo SA" direto no banco (o DELETE é coberto no CA-05)
	if _, err := pool.Exec(context.Background(),
		`UPDATE clients SET archived_at = now() WHERE name = 'Antigo SA'`); err != nil {
		t.Fatal(err)
	}

	var body struct {
		Clients []clientJSON `json:"clients"`
	}
	rec := doJSON(router, http.MethodGet, base, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("envelope fora do contrato: %v", err)
	}
	if len(body.Clients) != 1 || body.Clients[0].Name != "Ativo SA" {
		t.Errorf("default deveria listar só o não-arquivado, veio %+v", body.Clients)
	}

	rec = doJSON(router, http.MethodGet, base+"?archived=true", cookie, "")
	body.Clients = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Clients) != 2 {
		t.Errorf("?archived=true deveria incluir os arquivados (2), veio %d", len(body.Clients))
	}
}

// CA-04 — PATCH renomeia e devolve 200 com o objeto atualizado.
func TestRenomearCliente(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/clients"

	rec := doJSON(router, http.MethodPost, base, cookie, `{"name":"Acme"}`)
	var c clientJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}

	rec = doJSON(router, http.MethodPatch, base+"/"+c.ID, cookie, `{"name":"Acme Tech"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var atualizado clientJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &atualizado); err != nil {
		t.Fatal(err)
	}
	if atualizado.ID != c.ID || atualizado.Name != "Acme Tech" {
		t.Errorf("objeto atualizado fora do contrato: %+v", atualizado)
	}

	// id inexistente → 404 (mesma resposta de invisível — specs/INDEX.md)
	rec = doJSON(router, http.MethodPatch, base+"/00000000-0000-0000-0000-000000000000",
		cookie, `{"name":"X"}`)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("inexistente: esperado 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}

	// nome inválido no PATCH → 400 validation_error
	rec = doJSON(router, http.MethodPatch, base+"/"+c.ID, cookie, `{"name":""}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("nome inválido: esperado 400 validation_error, veio %d", rec.Code)
	}
}

// CA-05 — DELETE arquiva (nunca apaga a linha) e os projetos vinculados
// mantêm o client_id intacto.
func TestArquivarClienteMantemProjetos(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-owner", "owner@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-owner")
	userID := userIDOf(t, pool, "g-owner")
	base := "/api/v1/workspaces/" + ws + "/clients"
	ctx := context.Background()

	rec := doJSON(router, http.MethodPost, base, cookie, `{"name":"Acme"}`)
	var c clientJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}

	// projeto vinculado ao cliente (via SQL — handlers de projeto chegam na T-06)
	var projectID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, client_id, name, color, visibility, owner_user_id)
		VALUES ($1, $2, 'App', '#4f9dde', 'shared', $3) RETURNING id`,
		ws, c.ID, userID).Scan(&projectID); err != nil {
		t.Fatalf("seed projeto: %v", err)
	}

	rec = doJSON(router, http.MethodDelete, base+"/"+c.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}

	// linha continua existindo, com archived_at preenchido
	var arquivado bool
	if err := pool.QueryRow(ctx,
		`SELECT archived_at IS NOT NULL FROM clients WHERE id = $1`, c.ID).Scan(&arquivado); err != nil {
		t.Fatalf("cliente sumiu do banco (deveria ser soft-delete): %v", err)
	}
	if !arquivado {
		t.Error("archived_at deveria estar preenchido após o DELETE")
	}

	// projeto vinculado mantém o client_id
	var clientIDDoProjeto *string
	if err := pool.QueryRow(ctx,
		`SELECT client_id FROM projects WHERE id = $1`, projectID).Scan(&clientIDDoProjeto); err != nil {
		t.Fatal(err)
	}
	if clientIDDoProjeto == nil || *clientIDDoProjeto != c.ID {
		t.Errorf("client_id do projeto deveria ficar intacto, veio %v", clientIDDoProjeto)
	}

	// DELETE de id inexistente → 404
	rec = doJSON(router, http.MethodDelete, base+"/00000000-0000-0000-0000-000000000000", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("inexistente: esperado 404, veio %d", rec.Code)
	}
}
