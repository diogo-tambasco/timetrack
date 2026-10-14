// timer_state_test.go — T-11: GET /api/v1/timer (estado canônico —
// CA-21/22/22.1/23) e exigência de sessão em TODAS as rotas da fase
// (CA-30, table-driven). Os casos pomodoro RODANDO e pausa EXPIRADA são
// obrigatórios NA CAMADA HTTP (no service já têm teste em
// timer/isolation_test.go — aqui o contrato REST).
package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// CA-21 — rodando (countup): GET /timer devolve modo, a entry ativa,
// elapsed_seconds do relógio do servidor e server_time.
func TestGetTimerRodandoCountup(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)

	started := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "lote 14"})
	clock.Advance(5070 * time.Second)

	rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" {
		t.Fatalf("estado deveria ser countup rodando, veio %s", rec.Body.String())
	}
	if body.State.Entry == nil || body.State.Entry.ID != started.State.Entry.ID ||
		body.State.Entry.Description != "lote 14" {
		t.Errorf("a entry ativa deveria vir no estado, veio %+v", body.State.Entry)
	}
	if body.State.ElapsedSeconds != 5070 {
		t.Errorf("elapsed do relógio do servidor esperado 5070, veio %d", body.State.ElapsedSeconds)
	}
	queria := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC).Add(5070 * time.Second)
	if !body.ServerTime.Equal(queria) {
		t.Errorf("server_time esperado %v, veio %v", queria, body.ServerTime)
	}
}

// CA-21 (caso obrigatório do bloco) — pomodoro RODANDO na camada HTTP: o
// state.mode responde "pomodoro" (tradução do enum interno) com alvo do
// focus_minutes persistido e remaining calculado pelo servidor.
func TestGetTimerPomodoroRodando(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	if _, err := tsvc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 5}); err != nil {
		t.Fatalf("salvar settings: %v", err)
	}
	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Mode: "pomodoro", Description: "foco"})
	clock.Advance(300 * time.Second)

	rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "pomodoro" {
		t.Fatalf("state.mode deveria ser 'pomodoro' (nunca o enum interno), veio %s", rec.Body.String())
	}
	if body.State.TargetSeconds == nil || *body.State.TargetSeconds != 1500 {
		t.Errorf("alvo esperado 1500 (25min persistidos), veio %v", body.State.TargetSeconds)
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 1200 {
		t.Errorf("remaining esperado 1200 (1500−300), veio %v", body.State.RemainingSeconds)
	}
	if body.State.Entry == nil {
		t.Error("pomodoro rodando é modo de TRABALHO: a entry ativa vem no estado")
	}
}

// CA-22 — em pausa: mode='break', remaining_seconds e o bloco resume
// referenciando a entry pausada (pra retomar com contexto).
func TestGetTimerEmPausa(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	target := 1200
	started := mustStart(t, tsvc, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown", TargetSeconds: &target, Description: "bloco",
	})
	clock.Advance(300 * time.Second)
	if _, err := tsvc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause de setup: %v", err)
	}
	clock.Advance(28 * time.Second)

	rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "break" {
		t.Fatalf("estado deveria ser mode='break', veio %s", rec.Body.String())
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 272 {
		t.Errorf("remaining esperado 272 (300−28, default 5min), veio %v", body.State.RemainingSeconds)
	}
	if body.State.Entry != nil {
		t.Error("durante a pausa NÃO há entry rodando no estado")
	}
	if body.State.Resume == nil || body.State.Resume.EntryID != started.State.Entry.ID {
		t.Fatalf("resume deveria referenciar a entry pausada, veio %+v", body.State.Resume)
	}
	if body.State.Resume.Mode != "countdown" {
		t.Errorf("resume.mode deveria preservar o modo pausado, veio %q", body.State.Resume.Mode)
	}
	if body.State.Resume.TargetSeconds == nil || *body.State.Resume.TargetSeconds != 900 {
		t.Errorf("restante do alvo esperado 900 (1200−300), veio %v", body.State.Resume.TargetSeconds)
	}
}

// CA-22.1 (caso obrigatório do bloco) — pausa EXPIRADA sem
// resume/skip/stop na camada HTTP: segue mode='break' com remaining=0,
// NUNCA negativo, até alguém agir.
func TestGetTimerPausaExpirada(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	clock.Advance(60 * time.Second)
	brk := 5 // pausa de 300s
	if _, err := tsvc.Pause(ctx, userID, &brk); err != nil {
		t.Fatalf("pause de setup: %v", err)
	}
	clock.Advance(400 * time.Second) // 100s além do fim da pausa

	rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /timer esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "break" {
		t.Fatalf("pausa expirada SEGUE em mode='break' até o usuário agir, veio %s", rec.Body.String())
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 0 {
		t.Errorf("remaining de pausa expirada deveria clampar em 0, veio %v", body.State.RemainingSeconds)
	}
	if body.State.Resume == nil {
		t.Error("o contexto de retomada segue disponível na pausa expirada")
	}
}

// CA-23 — ocioso: 200 {"state": null} com server_time.
func TestGetTimerOcioso(t *testing.T) {
	_, router, _, _, cookie, _, _ := pauseSetup(t)

	rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /timer ocioso esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State != nil {
		t.Errorf("ocioso deveria responder state null, veio %+v", body.State)
	}
	if body.ServerTime.IsZero() {
		t.Error("server_time vem em TODA resposta de estado (RNF-02)")
	}
}

// CA-30 — TODAS as rotas da fase exigem sessão: sem cookie → 401
// unauthorized (table-driven, inclusive as rotas que entram em
// T-11.1/T-12/T-13 — o RequireAuth cobre o subrouter inteiro).
func TestRotasDaFaseExigemSessao(t *testing.T) {
	_, router, _, _, _, _, wsID := pauseSetup(t)
	entryID := "00000000-0000-0000-0000-000000000001"

	rotas := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/timer"},
		{http.MethodPost, "/api/v1/timer/start"},
		{http.MethodPost, "/api/v1/timer/stop"},
		{http.MethodPost, "/api/v1/timer/pause"},
		{http.MethodPost, "/api/v1/timer/resume"},
		{http.MethodPost, "/api/v1/timer/break/skip"},
		{http.MethodGet, "/api/v1/settings/pomodoro"},
		{http.MethodPut, "/api/v1/settings/pomodoro"},
		{http.MethodGet, "/api/v1/workspaces/" + wsID + "/entries"},
		{http.MethodPost, "/api/v1/workspaces/" + wsID + "/entries"},
		{http.MethodPatch, "/api/v1/workspaces/" + wsID + "/entries/" + entryID},
		{http.MethodDelete, "/api/v1/workspaces/" + wsID + "/entries/" + entryID},
		{http.MethodPost, "/api/v1/workspaces/" + wsID + "/entries/" + entryID + "/continue"},
	}
	for _, r := range rotas {
		rec := doJSON(router, r.method, r.path, nil, "")
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("%s %s sem sessão: esperava 401 unauthorized, veio %d %s",
				r.method, r.path, rec.Code, rec.Body.String())
		}
	}
}
