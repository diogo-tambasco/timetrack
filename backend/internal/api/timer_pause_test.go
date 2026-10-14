// timer_pause_test.go — T-09: pause → pausa Pomodoro automática
// (CA-07/08/25/25.1) — comportamento central do produto.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// timerResp espelha o Result canônico do timer.Service no JSON.
type timerResp struct {
	State *struct {
		Mode             string `json:"mode"`
		ElapsedSeconds   int    `json:"elapsed_seconds"`
		TargetSeconds    *int   `json:"target_seconds"`
		RemainingSeconds *int   `json:"remaining_seconds"`
		Entry            *struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		} `json:"entry"`
		Resume *struct {
			EntryID       string `json:"entry_id"`
			Mode          string `json:"mode"`
			TargetSeconds *int   `json:"target_seconds"`
		} `json:"resume"`
	} `json:"state"`
	StoppedEntry *struct {
		ID              string `json:"id"`
		DurationSeconds *int   `json:"duration_seconds"`
	} `json:"stopped_entry"`
	ServerTime time.Time `json:"server_time"`
}

func decodeTimer(t *testing.T, rec *httptest.ResponseRecorder) timerResp {
	t.Helper()
	var body timerResp
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta fora do contrato do timer: %s", rec.Body.String())
	}
	return body
}

// pauseSetup sobe o router com clock injetável e um usuário logado.
func pauseSetup(t *testing.T) (*pgxpool.Pool, http.Handler, *fakeClock, *timer.Service, *http.Cookie, string, string) {
	t.Helper()
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC))
	router, svc, stub, tsvc := newTimerRouter(t, pool, clock.Now)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")
	wsID := workspaceIDOf(t, pool, "g-ana")
	return pool, router, clock, tsvc, cookie, userID, wsID
}

func mustStart(t *testing.T, tsvc *timer.Service, userID string, in timer.StartInput) timer.Result {
	t.Helper()
	res, err := tsvc.Start(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("start de setup: %v", err)
	}
	return res
}

func countRunning(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&n); err != nil {
		t.Fatalf("contar rodando: %v", err)
	}
	return n
}

// CA-07 — pause com break_minutes=15 PERSISTIDO fecha a entry de trabalho
// e inicia a pausa: mode='break', remaining = 15×60 = 900.
func TestPauseFechaEntryEIniciaPausaConfigurada(t *testing.T) {
	pool, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	if _, err := tsvc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 15}); err != nil {
		t.Fatalf("salvar settings: %v", err)
	}
	started := mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	clock.Advance(1500 * time.Second)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pause esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "break" {
		t.Fatalf("estado deveria ser mode='break', veio %s", rec.Body.String())
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 900 {
		t.Errorf("remaining esperado 900 (break 15min), veio %v", body.State.RemainingSeconds)
	}
	if body.State.Entry != nil {
		t.Error("durante a pausa NÃO há entry rodando no estado (semântica do TimerState)")
	}
	if body.StoppedEntry == nil || body.StoppedEntry.DurationSeconds == nil || *body.StoppedEntry.DurationSeconds != 1500 {
		t.Errorf("stopped_entry deveria fechar com 1500s, veio %+v", body.StoppedEntry)
	}
	if body.State.Resume == nil || body.State.Resume.EntryID != started.State.Entry.ID {
		t.Errorf("resume deveria referenciar a entry pausada, veio %+v", body.State.Resume)
	}
	if countRunning(t, pool, userID) != 0 {
		t.Error("nenhuma entry deveria seguir rodando durante a pausa")
	}
}

// CA-08 — break_minutes no payload vale SÓ nesta pausa; a config
// persistida não muda.
func TestPauseComOverrideNaoAlteraConfig(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)
	ctx := context.Background()

	if _, err := tsvc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 15}); err != nil {
		t.Fatalf("salvar settings: %v", err)
	}
	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID})
	clock.Advance(60 * time.Second)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, `{"break_minutes":5}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 300 {
		t.Errorf("remaining desta pausa esperado 300 (override 5min), veio %v", body.State.RemainingSeconds)
	}

	got, err := tsvc.GetSettings(ctx, userID)
	if err != nil || got.BreakMinutes != 15 {
		t.Errorf("config persistida deveria seguir 15, veio %+v (err=%v)", got, err)
	}
}

// CA-25 — pause sem timer rodando responde 409 no_active_timer.
func TestPauseSemTimerRodando(t *testing.T) {
	_, router, _, _, cookie, _, _ := pauseSetup(t)

	rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, "")
	if rec.Code != http.StatusConflict || errCode(t, rec) != "no_active_timer" {
		t.Errorf("esperava 409 no_active_timer, veio %d %s", rec.Code, rec.Body.String())
	}
}

// CA-25.1 — break_minutes fora de 1–60 responde 400 e o timer de trabalho
// segue RODANDO intacto (nenhuma entry é fechada).
func TestPauseForaDaFaixaNaoFechaNada(t *testing.T) {
	pool, router, _, tsvc, cookie, userID, wsID := pauseSetup(t)

	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "intacto"})

	for _, payload := range []string{`{"break_minutes":0}`, `{"break_minutes":61}`} {
		rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s deveria responder 400 validation_error, veio %d %s",
				payload, rec.Code, rec.Body.String())
		}
	}

	if countRunning(t, pool, userID) != 1 {
		t.Error("a entry de trabalho deveria seguir rodando após pauses inválidos")
	}
	var mode string
	if err := pool.QueryRow(context.Background(),
		`SELECT mode::text FROM timer_states WHERE user_id = $1`, userID).Scan(&mode); err != nil || mode != "countup" {
		t.Errorf("estado deveria seguir countup, veio %q (err=%v)", mode, err)
	}
}
