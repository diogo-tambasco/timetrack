package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// CA-18 — PATCH de status dentro do enum atualiza; fora do enum é 400.
func TestPatchStatusTask(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Mover"}`)
	url := "/api/v1/workspaces/" + ws + "/tasks/" + tk.ID

	rec := doJSON(router, http.MethodPatch, url, cookie, `{"status":"doing"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status doing: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var atualizada taskJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &atualizada); err != nil {
		t.Fatal(err)
	}
	if atualizada.Status != "doing" {
		t.Errorf("status deveria ser doing, veio %q", atualizada.Status)
	}
	var noBanco string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM tasks WHERE id = $1`, tk.ID).Scan(&noBanco); err != nil || noBanco != "doing" {
		t.Errorf("status no banco deveria ser doing, veio %q (err=%v)", noBanco, err)
	}

	// fora do enum → 400 validation_error, status intacto
	rec = doJSON(router, http.MethodPatch, url, cookie, `{"status":"feito"}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("status feito: esperado 400 validation_error, veio %d: %s", rec.Code, rec.Body.String())
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM tasks WHERE id = $1`, tk.ID).Scan(&noBanco); err != nil || noBanco != "doing" {
		t.Errorf("status inválido não deveria ter mudado nada, veio %q", noBanco)
	}
}

// CA-21 — DELETE remove a linha de verdade (hard delete, sem archived_at).
func TestDeletarTaskHard(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Descartável"}`)

	rec := doJSON(router, http.MethodDelete, "/api/v1/workspaces/"+ws+"/tasks/"+tk.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: esperado 204, veio %d: %s", rec.Code, rec.Body.String())
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 0 {
		t.Errorf("hard delete deveria remover a linha, há %d", n)
	}

	// repetir o DELETE → 404 (já não existe)
	rec = doJSON(router, http.MethodDelete, "/api/v1/workspaces/"+ws+"/tasks/"+tk.ID, cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("DELETE repetido: esperado 404, veio %d", rec.Code)
	}
}

// CA-22 — task herda a visibilidade do projeto: em projeto private de
// outro usuário, some da lista e o detalhe responde 404.
func TestTaskHerdaVisibilidadeDoProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	addMember(t, pool, ws, userIDOf(t, pool, "g-m"), "member")

	privado := criarProjeto(t, router, ws, cookieM, `{"name":"Segredo","color":"#4f9dde","visibility":"private"}`)
	escondida := criarTask(t, router, ws, cookieM, `{"project_id":"`+privado.ID+`","name":"Task secreta"}`)
	compartilhado := criarProjeto(t, router, ws, cookieM, `{"name":"Aberto","color":"#22c55e","visibility":"shared"}`)
	visivel := criarTask(t, router, ws, cookieM, `{"project_id":"`+compartilhado.ID+`","name":"Task pública"}`)

	// na lista de A (owner do ws): só a task do projeto shared
	got := listTasks(t, router, ws, "", cookieA)
	if len(got) != 1 || got[0].ID != visivel.ID {
		t.Errorf("lista de A deveria ter só a task do shared, veio %+v", got)
	}
	// o dono vê as duas
	if got := listTasks(t, router, ws, "", cookieM); len(got) != 2 {
		t.Errorf("lista do dono deveria ter 2 tasks, veio %d", len(got))
	}

	// detalhe da task escondida pra A → 404 not_found (idêntico a inexistente)
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks/"+escondida.ID, cookieA, "")
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("detalhe de task em private de outro: esperado 404 not_found, veio %d: %s",
			rec.Code, rec.Body.String())
	}
	// pro dono → 200
	if rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks/"+escondida.ID, cookieM, ""); rec.Code != http.StatusOK {
		t.Errorf("detalhe pro dono: esperado 200, veio %d", rec.Code)
	}

	// escrita por A na task escondida → mesmo 404 (não vaza nem pro ws owner)
	rec = doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/tasks/"+escondida.ID, cookieA, `{"name":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH de task invisível: esperado 404, veio %d", rec.Code)
	}
}
