// entries_handlers_test.go — T-12: CRUD de entradas manuais nas rotas
// aninhadas (CA-14/14.1/15/16/24/24.1/24.2/29.1/29.2/29.3), atrás da
// cadeia sessão → membership → role gate da fase 02.
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// entryJSON espelha o objeto Entry do contrato §3.6 do plan.
type entryJSON struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	ProjectID       *string    `json:"project_id"`
	TaskID          *string    `json:"task_id"`
	Description     string     `json:"description"`
	Tags            []string   `json:"tags"`
	StartedAt       time.Time  `json:"started_at"`
	StoppedAt       *time.Time `json:"stopped_at"`
	DurationSeconds *int       `json:"duration_seconds"`
	Source          string     `json:"source"`
}

type entriesListJSON struct {
	Entries      []entryJSON `json:"entries"`
	TotalSeconds int         `json:"total_seconds"`
}

func decodeEntry(t *testing.T, rec *httptest.ResponseRecorder) entryJSON {
	t.Helper()
	var e entryJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("resposta fora do contrato de entry: %s", rec.Body.String())
	}
	return e
}

func decodeEntriesList(t *testing.T, rec *httptest.ResponseRecorder) entriesListJSON {
	t.Helper()
	var l entriesListJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatalf("resposta fora do contrato de listagem: %s", rec.Body.String())
	}
	return l
}

func entriesPath(wsID string) string {
	return "/api/v1/workspaces/" + wsID + "/entries"
}

// criarEntryManual cria uma entrada manual fechada via POST e devolve o id.
func criarEntryManual(t *testing.T, router http.Handler, cookie *http.Cookie, wsID, desc, started, stopped string) entryJSON {
	t.Helper()
	payload := fmt.Sprintf(`{"description":%q,"started_at":%q,"stopped_at":%q}`, desc, started, stopped)
	rec := doJSON(router, http.MethodPost, entriesPath(wsID), cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar entry de setup esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	return decodeEntry(t, rec)
}

// CA-14 (+CA-02 no POST) — entrada manual válida persiste com
// source='manual' e duration_seconds calculado no SERVIDOR; o valor do
// payload é ignorado.
func TestCreateEntryManual(t *testing.T) {
	_, router, _, _, cookie, _, wsID := pauseSetup(t)

	payload := `{"description":"Call Arkan","tags":["call"],
		"started_at":"2026-06-10T09:00:00Z","stopped_at":"2026-06-10T09:45:00Z",
		"duration_seconds": 999}`
	rec := doJSON(router, http.MethodPost, entriesPath(wsID), cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST manual esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	e := decodeEntry(t, rec)
	if e.Source != "manual" {
		t.Errorf("source deveria ser 'manual', veio %q", e.Source)
	}
	if e.DurationSeconds == nil || *e.DurationSeconds != 2700 {
		t.Errorf("duração do SERVIDOR esperada 2700 (999 do payload ignorado), veio %v", e.DurationSeconds)
	}
	if e.Description != "Call Arkan" || len(e.Tags) != 1 {
		t.Errorf("campos do payload deveriam persistir, veio %+v", e)
	}
}

// CA-14.1 — recorte "hoje" segue o fuso IANA de tz: entry iniciada às
// 22:00 BRT (01:00 UTC do dia seguinte) aparece no "Hoje" de
// America/Sao_Paulo e NÃO aparece no recorte UTC (sem tz).
func TestListEntriesHojePorTimezone(t *testing.T) {
	// clock do harness: 2026-06-10 14:00 UTC = 11:00 BRT
	_, router, _, _, cookie, _, wsID := pauseSetup(t)

	criarEntryManual(t, router, cookie, wsID, "fim de noite BRT",
		"2026-06-11T01:00:00Z", "2026-06-11T01:30:00Z")

	rec := doJSON(router, http.MethodGet, entriesPath(wsID)+"?tz=America/Sao_Paulo", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET com tz esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	comTz := decodeEntriesList(t, rec)
	if len(comTz.Entries) != 1 || comTz.Entries[0].Description != "fim de noite BRT" {
		t.Errorf("no fuso BRT a entry é de HOJE e deveria aparecer, veio %+v", comTz.Entries)
	}
	if comTz.TotalSeconds != 1800 {
		t.Errorf("total_seconds esperado 1800, veio %d", comTz.TotalSeconds)
	}

	rec = doJSON(router, http.MethodGet, entriesPath(wsID), cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET sem tz esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	semTz := decodeEntriesList(t, rec)
	if len(semTz.Entries) != 0 {
		t.Errorf("no recorte UTC a entry é de AMANHÃ e não deveria aparecer, veio %+v", semTz.Entries)
	}

	// filtro malformado = 400 validation_error (decisão da fase 02)
	rec = doJSON(router, http.MethodGet, entriesPath(wsID)+"?tz=Marte/Cratera", cookie, "")
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("tz inválida esperava 400 validation_error, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-15 — PATCH mudando stopped_at recalcula duration_seconds no servidor.
func TestPatchEntryRecalculaDuracao(t *testing.T) {
	_, router, _, _, cookie, _, wsID := pauseSetup(t)

	e := criarEntryManual(t, router, cookie, wsID, "ajustável",
		"2026-06-10T09:00:00Z", "2026-06-10T09:45:00Z")

	rec := doJSON(router, http.MethodPatch, entriesPath(wsID)+"/"+e.ID, cookie,
		`{"stopped_at":"2026-06-10T10:30:00Z"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeEntry(t, rec)
	if got.DurationSeconds == nil || *got.DurationSeconds != 5400 {
		t.Errorf("duração recalculada esperada 5400 (09:00→10:30), veio %v", got.DurationSeconds)
	}
}

// CA-16 — DELETE responde 204 e a entry some da listagem.
func TestDeleteEntrySomeDaListagem(t *testing.T) {
	_, router, _, _, cookie, _, wsID := pauseSetup(t)

	e := criarEntryManual(t, router, cookie, wsID, "descartável",
		"2026-06-10T09:00:00Z", "2026-06-10T09:30:00Z")

	rec := doJSON(router, http.MethodDelete, entriesPath(wsID)+"/"+e.ID, cookie, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE esperava 204, veio %d: %s", rec.Code, rec.Body.String())
	}
	list := decodeEntriesList(t, doJSON(router, http.MethodGet, entriesPath(wsID), cookie, ""))
	if len(list.Entries) != 0 {
		t.Errorf("a entry excluída não deveria mais aparecer, veio %+v", list.Entries)
	}
}

// CA-24 — stopped_at ≤ started_at (e horários ausentes) → 400 e NADA
// persistido.
func TestCreateEntryHorariosInvalidos(t *testing.T) {
	pool, router, _, _, cookie, _, wsID := pauseSetup(t)

	for _, payload := range []string{
		`{"description":"x","started_at":"2026-06-10T10:00:00Z","stopped_at":"2026-06-10T09:00:00Z"}`,
		`{"description":"x","started_at":"2026-06-10T10:00:00Z","stopped_at":"2026-06-10T10:00:00Z"}`,
		`{"description":"x","started_at":"2026-06-10T10:00:00Z"}`,
		`{"description":"x"}`,
	} {
		rec := doJSON(router, http.MethodPost, entriesPath(wsID), cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s esperava 400 validation_error, veio %d %s", payload, rec.Code, rec.Body.String())
		}
	}
	if n := dbtest.CountRows(t, pool, "time_entries"); n != 0 {
		t.Errorf("nenhuma entry deveria ter sido persistida, há %d", n)
	}
}

// CA-29 (POST manual, HTTP) — project_id de outro workspace → 404, nada
// persistido (anti-vazamento na escrita).
func TestCreateEntryRefDeOutroWorkspace(t *testing.T) {
	pool, router, _, env, anaCookie, _, wsAna := startStopSetup(t)
	ctx := context.Background()

	_, biaID, biaWs := loginBia(t, pool, env)
	var projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'Alheio', '#4f9dde', 'shared', $2) RETURNING id`,
		biaWs, biaID).Scan(&projectID); err != nil {
		t.Fatalf("seed projeto alheio: %v", err)
	}

	payload := fmt.Sprintf(`{"description":"x","project_id":%q,
		"started_at":"2026-06-10T09:00:00Z","stopped_at":"2026-06-10T09:30:00Z"}`, projectID)
	rec := doJSON(router, http.MethodPost, entriesPath(wsAna), anaCookie, payload)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("ref cruzada esperava 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}
	if n := dbtest.CountRows(t, pool, "time_entries"); n != 0 {
		t.Errorf("nada deveria ser persistido com ref de outro workspace, há %d", n)
	}
}

// CA-24.1 — PATCH e DELETE na entry RODANDO → 409 entry_running e nada
// muda (mudança no timer ativo passa pelas rotas /timer).
func TestPatchDeleteEntryRodando(t *testing.T) {
	pool, router, _, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	started := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "rodando"})
	entryID := started.State.Entry.ID

	rec := doJSON(router, http.MethodPatch, entriesPath(wsID)+"/"+entryID, cookie,
		`{"description":"hackeada"}`)
	if rec.Code != http.StatusConflict || errCode(t, rec) != "entry_running" {
		t.Errorf("PATCH na rodando esperava 409 entry_running, veio %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodDelete, entriesPath(wsID)+"/"+entryID, cookie, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "entry_running" {
		t.Errorf("DELETE na rodando esperava 409 entry_running, veio %d %s", rec.Code, rec.Body.String())
	}

	var desc string
	var stoppedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT description, stopped_at FROM time_entries WHERE id = $1`, entryID).
		Scan(&desc, &stoppedAt); err != nil {
		t.Fatalf("a entry rodando deveria seguir existindo: %v", err)
	}
	if desc != "rodando" || stoppedAt != nil {
		t.Errorf("a entry rodando deveria seguir INTACTA, veio desc=%q stopped=%v", desc, stoppedAt)
	}
}

// CA-24.2 — DELETE na entry que é resume_entry_id de pausa ATIVA → 409
// entry_in_use (proteção do resume), sem remover.
func TestDeleteEntryEmUsoPelaPausa(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	started := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "pausada"})
	clock.Advance(60 * time.Second)
	if _, err := tsvc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause de setup: %v", err)
	}
	entryID := started.State.Entry.ID

	rec := doJSON(router, http.MethodDelete, entriesPath(wsID)+"/"+entryID, cookie, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "entry_in_use" {
		t.Errorf("DELETE no contexto da pausa esperava 409 entry_in_use, veio %d %s",
			rec.Code, rec.Body.String())
	}
	var existe bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM time_entries WHERE id = $1)`, entryID).Scan(&existe); err != nil || !existe {
		t.Errorf("a entry em uso pela pausa deveria seguir existindo (existe=%v err=%v)", existe, err)
	}
}

// CA-29.1 — {workspaceId} de workspace que o usuário NÃO integra → 404 em
// leitura E escrita, sem expor nenhum dado.
func TestEntriesWorkspaceQueNaoIntegra(t *testing.T) {
	pool, router, _, env, anaCookie, _, wsAna := startStopSetup(t)

	e := criarEntryManual(t, router, anaCookie, wsAna, "da ana",
		"2026-06-10T09:00:00Z", "2026-06-10T09:30:00Z")
	biaCookie, _, _ := loginBia(t, pool, env)

	rotas := []struct{ method, path, body string }{
		{http.MethodGet, entriesPath(wsAna), ""},
		{http.MethodPost, entriesPath(wsAna),
			`{"description":"x","started_at":"2026-06-10T09:00:00Z","stopped_at":"2026-06-10T09:30:00Z"}`},
		{http.MethodPatch, entriesPath(wsAna) + "/" + e.ID, `{"description":"x"}`},
		{http.MethodDelete, entriesPath(wsAna) + "/" + e.ID, ""},
	}
	for _, r := range rotas {
		rec := doJSON(router, r.method, r.path, biaCookie, r.body)
		if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
			t.Errorf("%s %s sem membership: esperava 404 not_found, veio %d %s",
				r.method, r.path, rec.Code, rec.Body.String())
		}
	}
}

// CA-29.2 — escrita como VIEWER → 403 forbidden (leitura passa).
func TestEntriesComoViewer(t *testing.T) {
	pool, router, _, env, anaCookie, _, wsAna := startStopSetup(t)

	e := criarEntryManual(t, router, anaCookie, wsAna, "da ana",
		"2026-06-10T09:00:00Z", "2026-06-10T09:30:00Z")
	biaCookie, biaID, _ := loginBia(t, pool, env)
	addMember(t, pool, wsAna, biaID, "viewer")

	escritas := []struct{ method, path, body string }{
		{http.MethodPost, entriesPath(wsAna),
			`{"description":"x","started_at":"2026-06-10T09:00:00Z","stopped_at":"2026-06-10T09:30:00Z"}`},
		{http.MethodPatch, entriesPath(wsAna) + "/" + e.ID, `{"description":"x"}`},
		{http.MethodDelete, entriesPath(wsAna) + "/" + e.ID, ""},
		// o CA-29.2 lista o continue EXPLICITAMENTE — sem esta linha, mover a
		// rota pra fora do role gate passava na suíte (provado por mutação na
		// verificação adversarial do bloco)
		{http.MethodPost, entriesPath(wsAna) + "/" + e.ID + "/continue", ""},
	}
	for _, r := range escritas {
		rec := doJSON(router, r.method, r.path, biaCookie, r.body)
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
			t.Errorf("%s %s como viewer: esperava 403 forbidden, veio %d %s",
				r.method, r.path, rec.Code, rec.Body.String())
		}
	}
	rec := doJSON(router, http.MethodGet, entriesPath(wsAna), biaCookie, "")
	if rec.Code != http.StatusOK {
		t.Errorf("viewer LÊ a listagem (role gate só barra escrita), veio %d %s",
			rec.Code, rec.Body.String())
	}
}

// CA-29.3 (+CA-01.1 na listagem HTTP) — entry de OUTRO usuário do MESMO
// workspace: PATCH/DELETE respondem 404 e a listagem não a inclui.
func TestEntriesDeOutroUsuarioMesmoWorkspace(t *testing.T) {
	pool, router, _, env, anaCookie, _, wsAna := startStopSetup(t)

	e := criarEntryManual(t, router, anaCookie, wsAna, "da ana",
		"2026-06-10T09:00:00Z", "2026-06-10T09:30:00Z")
	biaCookie, biaID, _ := loginBia(t, pool, env)
	addMember(t, pool, wsAna, biaID, "member")

	rec := doJSON(router, http.MethodPatch, entriesPath(wsAna)+"/"+e.ID, biaCookie,
		`{"description":"da bia agora"}`)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("PATCH em entry alheia esperava 404, veio %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodDelete, entriesPath(wsAna)+"/"+e.ID, biaCookie, "")
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("DELETE em entry alheia esperava 404, veio %d %s", rec.Code, rec.Body.String())
	}

	list := decodeEntriesList(t, doJSON(router, http.MethodGet, entriesPath(wsAna), biaCookie, ""))
	if len(list.Entries) != 0 {
		t.Errorf("a listagem da Bia não inclui entries da Ana (CA-01.1), veio %+v", list.Entries)
	}
}
