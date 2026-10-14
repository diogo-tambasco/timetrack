// timer_startstop_test.go — T-11.1: contrato HTTP de POST /timer/start e
// POST /timer/stop (CA-04/06 na camada REST) + membership/role do
// workspace_id do payload (CA-29.1/29.2) — a rota de timer é POR USUÁRIO e
// não passa pelo RequireMember; a validação mora no handler de start.
package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// authEnv expõe o auth do harness pra logar um 2º usuário nos testes de
// membership (o pauseSetup esconde svc/stub).
type authEnv struct {
	svc  *auth.Service
	stub *googlestub.Server
}

func startStopSetup(t *testing.T) (*pgxpool.Pool, http.Handler, *fakeClock, *authEnv, *http.Cookie, string, string) {
	t.Helper()
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC))
	router, svc, stub, _ := newTimerRouter(t, pool, clock.Now)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")
	wsID := workspaceIDOf(t, pool, "g-ana")
	return pool, router, clock, &authEnv{svc, stub}, cookie, userID, wsID
}

func loginBia(t *testing.T, pool *pgxpool.Pool, env *authEnv) (*http.Cookie, string, string) {
	t.Helper()
	cookie := loginVia(t, env.svc, env.stub, "g-bia", "bia@tt.dev", "Bia")
	return cookie, userIDOf(t, pool, "g-bia"), workspaceIDOf(t, pool, "g-bia")
}

// CA-04 (HTTP) — start válido responde 201 com corpo IDÊNTICO ao do
// GET /timer (mesmo Result canônico, relógio congelado).
func TestStartHTTPRespondeCorpoDoGet(t *testing.T) {
	pool, router, _, _, cookie, userID, wsID := pauseSetup(t)

	payload := fmt.Sprintf(`{"workspace_id":%q,"description":"via REST","tags":["a"]}`, wsID)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" || body.State.Entry == nil {
		t.Fatalf("estado pós-start deveria ser countup com entry, veio %s", rec.Body.String())
	}
	if body.State.Entry.Description != "via REST" {
		t.Errorf("descrição do payload deveria valer, veio %q", body.State.Entry.Description)
	}
	if countRunning(t, pool, userID) != 1 {
		t.Error("deveria haver exatamente 1 entry rodando após o start")
	}

	get := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET pós-start esperava 200, veio %d", get.Code)
	}
	if rec.Body.String() != get.Body.String() {
		t.Errorf("corpo do start deveria ser IDÊNTICO ao do GET /timer:\nstart: %s\nget:   %s",
			rec.Body.String(), get.Body.String())
	}
}

// CA-06 (HTTP) — stop responde 200 com stopped_entry (duração do relógio
// do servidor) e state null.
func TestStopHTTPFechaEntryEDeixaOcioso(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	clock.Advance(120 * time.Second)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/stop", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stop esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State != nil {
		t.Errorf("estado pós-stop deveria ser null, veio %+v", body.State)
	}
	if body.StoppedEntry == nil || body.StoppedEntry.DurationSeconds == nil ||
		*body.StoppedEntry.DurationSeconds != 120 {
		t.Errorf("stopped_entry deveria fechar com 120s, veio %+v", body.StoppedEntry)
	}
}

// CA-25 (HTTP, stop) — stop sem timer nem pausa ativa → 409 no_active_timer.
func TestStopHTTPOcioso(t *testing.T) {
	_, router, _, _, cookie, _, _ := pauseSetup(t)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/stop", cookie, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "no_active_timer" {
		t.Errorf("stop ocioso: esperava 409 no_active_timer, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-27 (HTTP) — countdown sem target_seconds (ou ≤0) → 400 e nenhuma
// entry criada.
func TestStartHTTPCountdownSemAlvo(t *testing.T) {
	pool, router, _, _, cookie, userID, wsID := pauseSetup(t)

	for _, payload := range []string{
		fmt.Sprintf(`{"workspace_id":%q,"mode":"countdown"}`, wsID),
		fmt.Sprintf(`{"workspace_id":%q,"mode":"countdown","target_seconds":0}`, wsID),
		fmt.Sprintf(`{"workspace_id":%q,"mode":"countdown","target_seconds":-5}`, wsID),
	} {
		rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s deveria responder 400 validation_error, veio %d %s",
				payload, rec.Code, rec.Body.String())
		}
	}
	if countRunning(t, pool, userID) != 0 {
		t.Error("nenhuma entry deveria existir após starts inválidos")
	}
}

// CA-29 (HTTP) — project_id de OUTRO workspace → 404 not_found, nada criado.
func TestStartHTTPRefDeOutroWorkspace(t *testing.T) {
	pool, router, _, env, anaCookie, anaID, wsAna := startStopSetup(t)
	ctx := context.Background()

	// projeto vive no workspace da Bia (outro tenant)
	_, biaID, biaWs := loginBia(t, pool, env)
	var projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'Alheio', '#4f9dde', 'shared', $2) RETURNING id`,
		biaWs, biaID).Scan(&projectID); err != nil {
		t.Fatalf("seed projeto alheio: %v", err)
	}

	payload := fmt.Sprintf(`{"workspace_id":%q,"project_id":%q}`, wsAna, projectID)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", anaCookie, payload)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("ref cruzada esperava 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, anaID) != 0 {
		t.Error("nenhuma entry deveria ser criada com ref de outro workspace")
	}
}

// CA-29.1 — start com workspace_id de workspace que o usuário NÃO integra
// → 404 sem expor nenhum dado (mesmo envelope de inexistente/malformado).
func TestStartHTTPWorkspaceQueNaoIntegra(t *testing.T) {
	pool, router, _, env, _, _, wsAna := startStopSetup(t)

	biaCookie, biaID, _ := loginBia(t, pool, env)

	for _, wsID := range []string{wsAna, "00000000-0000-0000-0000-00000000dead", "nao-uuid"} {
		payload := fmt.Sprintf(`{"workspace_id":%q,"description":"x"}`, wsID)
		rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", biaCookie, payload)
		if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
			t.Errorf("workspace %q sem membership: esperava 404 not_found, veio %d %s",
				wsID, rec.Code, rec.Body.String())
		}
	}
	if countRunning(t, pool, biaID) != 0 {
		t.Error("nenhuma entry deveria ser criada sem membership")
	}
}

// CA-29.2 — start como VIEWER do workspace → 403 forbidden.
func TestStartHTTPComoViewer(t *testing.T) {
	pool, router, _, env, _, _, wsAna := startStopSetup(t)

	biaCookie, biaID, _ := loginBia(t, pool, env)
	addMember(t, pool, wsAna, biaID, "viewer")

	payload := fmt.Sprintf(`{"workspace_id":%q,"description":"x"}`, wsAna)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", biaCookie, payload)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Errorf("viewer esperava 403 forbidden, veio %d %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, biaID) != 0 {
		t.Error("viewer não cria entry")
	}
}
