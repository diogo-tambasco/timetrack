// entries_continue_test.go — T-13: POST /entries/{id}/continue (CA-17)
// clona descrição/projeto/task/tags e inicia count up source='timer',
// aplicando a troca atômica do CA-05 quando já há timer rodando.
package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// CA-17 — continue de uma entry FECHADA cria entry RODANDO com os 4 campos
// clonados e horários novos do servidor; resposta = corpo do GET /timer.
func TestContinuarEntradaClonaEInicia(t *testing.T) {
	pool, router, clock, _, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	var projectID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'App', '#4f9dde', 'shared', $2) RETURNING id`, wsID, userID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3) RETURNING id`, wsID, projectID, userID).Scan(&taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	payload := fmt.Sprintf(`{"description":"recorrente","project_id":%q,"task_id":%q,
		"tags":["semanal"],"started_at":"2026-06-10T08:00:00Z","stopped_at":"2026-06-10T09:00:00Z"}`,
		projectID, taskID)
	rec := doJSON(router, http.MethodPost, entriesPath(wsID), cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar entry de setup: %d %s", rec.Code, rec.Body.String())
	}
	original := decodeEntry(t, rec)
	clock.Advance(3600 * time.Second) // continue acontece 1h depois

	rec = doJSON(router, http.MethodPost, entriesPath(wsID)+"/"+original.ID+"/continue", cookie, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("continue esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" || body.State.Entry == nil {
		t.Fatalf("continue deveria deixar um countup rodando, veio %s", rec.Body.String())
	}
	if body.State.Entry.ID == original.ID {
		t.Fatal("continue cria entry NOVA, não reabre a original")
	}

	// os 4 campos clonados + horários novos + source='timer', do banco
	var desc, source string
	var gotProject, gotTask *string
	var tags []string
	var startedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT description, project_id, task_id, tags, started_at, source
		  FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&desc, &gotProject, &gotTask, &tags, &startedAt, &source); err != nil {
		t.Fatalf("ler entry continuada: %v", err)
	}
	if desc != "recorrente" || gotProject == nil || *gotProject != projectID ||
		gotTask == nil || *gotTask != taskID || len(tags) != 1 || tags[0] != "semanal" {
		t.Errorf("clone incompleto: desc=%q project=%v task=%v tags=%v", desc, gotProject, gotTask, tags)
	}
	if source != "timer" {
		t.Errorf("source da continuada deveria ser 'timer', veio %q", source)
	}
	queriaInicio := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)
	if !startedAt.Equal(queriaInicio) {
		t.Errorf("started_at deveria ser o relógio do servidor %v, veio %v", queriaInicio, startedAt)
	}
}

// CA-17 + CA-05 — continue com OUTRO timer rodando para o atual e deixa
// exatamente 1 entry rodando (troca atômica).
func TestContinuarComTimerRodandoTroca(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)

	original := criarEntryManual(t, router, cookie, wsID, "antiga",
		"2026-06-10T08:00:00Z", "2026-06-10T09:00:00Z")
	rodando := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "atual"})
	clock.Advance(120 * time.Second)

	rec := doJSON(router, http.MethodPost, entriesPath(wsID)+"/"+original.ID+"/continue", cookie, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("continue esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, userID) != 1 {
		t.Error("deveria restar exatamente 1 entry rodando após o continue (CA-05)")
	}
	var dur *int
	if err := pool.QueryRow(context.Background(),
		`SELECT duration_seconds FROM time_entries WHERE id = $1`,
		rodando.State.Entry.ID).Scan(&dur); err != nil || dur == nil || *dur != 120 {
		t.Errorf("a entry que rodava deveria fechar com 120s, veio %v (err=%v)", dur, err)
	}
}

// CA-29.3 — continue em entry de OUTRO usuário do mesmo workspace → 404
// (continuar só as próprias entries) e nenhum timer inicia.
func TestContinuarEntryDeOutroUsuario(t *testing.T) {
	pool, router, _, env, anaCookie, _, wsAna := startStopSetup(t)

	e := criarEntryManual(t, router, anaCookie, wsAna, "da ana",
		"2026-06-10T08:00:00Z", "2026-06-10T09:00:00Z")
	biaCookie, biaID, _ := loginBia(t, pool, env)
	addMember(t, pool, wsAna, biaID, "member")

	rec := doJSON(router, http.MethodPost, entriesPath(wsAna)+"/"+e.ID+"/continue", biaCookie, "")
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("continue em entry alheia esperava 404, veio %d %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, biaID) != 0 {
		t.Error("nenhum timer deveria iniciar a partir de entry alheia")
	}
}
