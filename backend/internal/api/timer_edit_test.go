// timer_edit_test.go — T-00 da fase 27: contrato HTTP da allowance
// PATCH /api/v1/timer (edição de rótulo/projeto/tags da entry rodando,
// decisão HITL 23/06). Rota POR USUÁRIO; corpo SEM workspace_id; timing
// intocado (CA-13.1); 404/409 canônicos.
package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// CA-13 (HTTP) — PATCH /timer edita description+project_id da entry rodando e
// devolve o Result canônico (mesmo shape do GET /timer); timing intocado.
func TestPatchTimerEditaEntryRodando(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "antigo"})
	clock.Advance(90 * time.Second)

	// projeto do próprio workspace
	var projectID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'App', '#4f9dde', 'shared', $2) RETURNING id`, wsID, userID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	payload := fmt.Sprintf(`{"description":"novo","project_id":%q}`, projectID)
	rec := doJSON(router, http.MethodPatch, "/api/v1/timer", cookie, payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Entry == nil {
		t.Fatalf("resposta deveria trazer o State da entry rodando, veio %s", rec.Body.String())
	}
	if body.State.Entry.Description != "novo" {
		t.Errorf("descrição deveria virar 'novo', veio %q", body.State.Entry.Description)
	}
	if body.State.Mode != "countup" {
		t.Errorf("mode deveria seguir countup, veio %q", body.State.Mode)
	}
	if body.State.ElapsedSeconds != 90 {
		t.Errorf("elapsed deveria ser 90 (cronômetro não zerou), veio %d", body.State.ElapsedSeconds)
	}

	// a entry segue rodando no banco com a descrição nova e SEM stopped_at
	var (
		desc    string
		stopped *time.Time
	)
	if err := pool.QueryRow(ctx,
		`SELECT description, stopped_at FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&desc, &stopped); err != nil {
		t.Fatalf("ler entry rodando: %v", err)
	}
	if desc != "novo" || stopped != nil {
		t.Errorf("entry rodando: desc=%q stopped=%v (esperava 'novo', rodando)", desc, stopped)
	}
}

// CA-13 (HTTP) — PATCH /timer ocioso → 409 no_active_timer.
func TestPatchTimerOcioso409(t *testing.T) {
	_, router, _, _, cookie, _, _ := pauseSetup(t)

	rec := doJSON(router, http.MethodPatch, "/api/v1/timer", cookie, `{"description":"x"}`)
	if rec.Code != http.StatusConflict || errCode(t, rec) != "no_active_timer" {
		t.Errorf("PATCH ocioso: esperava 409 no_active_timer, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-13.2 (HTTP) — project_id de outro workspace → 404 not_found.
func TestPatchTimerProjetoAlheio404(t *testing.T) {
	pool, router, _, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "rodando"})

	var otherWs, otherProj string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ('Outro') RETURNING id`).Scan(&otherWs); err != nil {
		t.Fatalf("seed outro ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'Alheio', '#4f9dde', 'shared', $2) RETURNING id`, otherWs, userID).Scan(&otherProj); err != nil {
		t.Fatalf("seed projeto alheio: %v", err)
	}

	payload := fmt.Sprintf(`{"project_id":%q}`, otherProj)
	rec := doJSON(router, http.MethodPatch, "/api/v1/timer", cookie, payload)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("project alheio: esperava 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-13.1 (HTTP) — PATCH /timer não aceita campos de tempo: enviar started_at
// é ignorado (decode descarta), a entry segue rodando intacta. (O caminho de
// edição de TIMING segue via entries.Patch → 409 entry_running — pinado em
// TestPatchDeleteEntryRodando.)
func TestPatchTimerIgnoraCamposDeTempo(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	started := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "antigo"})
	startedAt := started.State.Entry.StartedAt
	clock.Advance(60 * time.Second)

	// started_at/stopped_at/duration_seconds no payload são descartados pelo decode
	rec := doJSON(router, http.MethodPatch, "/api/v1/timer", cookie,
		`{"description":"novo","started_at":"2020-01-01T00:00:00Z","stopped_at":"2020-01-01T01:00:00Z","duration_seconds":3600}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}

	var (
		dbStarted time.Time
		dbStopped *time.Time
		dbDur     *int
	)
	if err := pool.QueryRow(ctx,
		`SELECT started_at, stopped_at, duration_seconds FROM time_entries WHERE id = $1`,
		started.State.Entry.ID).Scan(&dbStarted, &dbStopped, &dbDur); err != nil {
		t.Fatalf("ler entry: %v", err)
	}
	if !dbStarted.Equal(startedAt) {
		t.Errorf("started_at deveria ser imutável (%v), veio %v", startedAt, dbStarted)
	}
	if dbStopped != nil || dbDur != nil {
		t.Errorf("entry deveria seguir rodando: stopped=%v dur=%v", dbStopped, dbDur)
	}
}
