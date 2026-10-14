// timer_resume_test.go — T-10: resume e skip da pausa (CA-09/10/11/26).
package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// CA-09 — resume cria uma entry NOVA clonando descrição, project_id,
// task_id e tags da pausada, com started_at novo do servidor.
func TestResumeClonaEntryEContinua(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
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

	original := mustStart(t, tsvc, userID, timer.StartInput{
		WorkspaceID: wsID, Description: "trabalho X",
		ProjectID: &projectID, TaskID: &taskID, Tags: []string{"deep", "lote-14"},
	})
	clock.Advance(600 * time.Second)
	if _, err := tsvc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause de setup: %v", err)
	}
	clock.Advance(30 * time.Second)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/resume", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("resume esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" {
		t.Fatalf("estado pós-resume deveria voltar pro trabalho, veio %s", rec.Body.String())
	}
	if body.State.Entry == nil || body.State.Entry.ID == original.State.Entry.ID {
		t.Fatal("resume deveria criar entry NOVA, não reabrir a pausada")
	}
	if body.State.Entry.Description != "trabalho X" {
		t.Errorf("descrição deveria ser clonada, veio %q", body.State.Entry.Description)
	}

	// os 4 campos clonados + started_at novo, direto do banco
	var desc string
	var gotProject, gotTask *string
	var tags []string
	var startedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT description, project_id, task_id, tags, started_at
		  FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&desc, &gotProject, &gotTask, &tags, &startedAt); err != nil {
		t.Fatalf("ler entry retomada: %v", err)
	}
	if desc != "trabalho X" || gotProject == nil || *gotProject != projectID ||
		gotTask == nil || *gotTask != taskID || len(tags) != 2 {
		t.Errorf("clone incompleto: desc=%q project=%v task=%v tags=%v", desc, gotProject, gotTask, tags)
	}
	queriaInicio := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC).Add(630 * time.Second)
	if !startedAt.Equal(queriaInicio) {
		t.Errorf("started_at da retomada deveria ser %v (relógio do servidor), veio %v", queriaInicio, startedAt)
	}
}

// CA-10 — countdown de 1200s pausado aos 300s retoma com o RESTANTE do
// alvo (900s), não com o alvo cheio.
func TestResumeRestauraRestanteDoAlvo(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	target := 1200
	mustStart(t, tsvc, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown", TargetSeconds: &target,
	})
	clock.Advance(300 * time.Second)
	if _, err := tsvc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause: %v", err)
	}
	clock.Advance(60 * time.Second)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/resume", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("resume esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State.Mode != "countdown" {
		t.Errorf("modo restaurado deveria ser countdown, veio %q", body.State.Mode)
	}
	if body.State.TargetSeconds == nil || *body.State.TargetSeconds != 900 {
		t.Errorf("alvo retomado deveria ser o restante 900, veio %v", body.State.TargetSeconds)
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 900 {
		t.Errorf("remaining no instante da retomada deveria ser 900, veio %v", body.State.RemainingSeconds)
	}
}

// CA-11 — skip encerra a pausa NA HORA (restavam 200s) e deixa o timer de
// trabalho rodando.
func TestSkipEncerraPausaNaHora(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	clock.Advance(60 * time.Second)
	brk := 5 // 300s de pausa
	if _, err := tsvc.Pause(ctx, userID, &brk); err != nil {
		t.Fatalf("pause: %v", err)
	}
	clock.Advance(100 * time.Second) // restam 200s — skip não espera

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/break/skip", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("skip esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" || body.State.Entry == nil {
		t.Fatalf("após o skip o trabalho deveria estar rodando, veio %s", rec.Body.String())
	}
	if countRunning(t, pool, userID) != 1 {
		t.Error("deveria haver exatamente 1 entry rodando após o skip")
	}
	var mode string
	if err := pool.QueryRow(ctx,
		`SELECT mode::text FROM timer_states WHERE user_id = $1`, userID).Scan(&mode); err != nil || mode != "countup" {
		t.Errorf("estado deveria voltar pro modo de trabalho, veio %q (err=%v)", mode, err)
	}
}

// CA-26 — resume/skip sem pausa ativa respondem 409 no_active_break —
// inclusive com timer de TRABALHO rodando (não é pausa).
func TestResumeSemPausaAtiva(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)

	for _, rota := range []string{"/api/v1/timer/resume", "/api/v1/timer/break/skip"} {
		rec := doJSON(router, http.MethodPost, rota, cookie, "")
		if rec.Code != http.StatusConflict || errCode(t, rec) != "no_active_break" {
			t.Errorf("%s ocioso: esperava 409 no_active_break, veio %d %s", rota, rec.Code, rec.Body.String())
		}
	}

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID})
	clock.Advance(10 * time.Second)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/resume", cookie, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "no_active_break" {
		t.Errorf("resume com trabalho rodando: esperava 409 no_active_break, veio %d %s",
			rec.Code, rec.Body.String())
	}
}
