// settings_pomodoro_test.go — T-08: GET|PUT /api/v1/settings/pomodoro
// (CA-18/19/28). Rota por usuário, sessão via Google stubbado.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func decodeSettings(t *testing.T, rec *httptest.ResponseRecorder) (focus, brk int) {
	t.Helper()
	var body struct {
		FocusMinutes int `json:"focus_minutes"`
		BreakMinutes int `json:"break_minutes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta fora do contrato: %s", rec.Body.String())
	}
	return body.FocusMinutes, body.BreakMinutes
}

// CA-18 — PUT válido persiste por usuário e o GET seguinte devolve
// exatamente o que foi salvo.
func TestPutPomodoroPersisteEGetDevolve(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie,
		`{"focus_minutes":50,"break_minutes":15}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if f, b := decodeSettings(t, rec); f != 50 || b != 15 {
		t.Errorf("PUT deveria ecoar os valores salvos, veio %d/%d", f, b)
	}

	rec = doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET esperava 200, veio %d", rec.Code)
	}
	if f, b := decodeSettings(t, rec); f != 50 || b != 15 {
		t.Errorf("GET deveria devolver os valores persistidos 50/15, veio %d/%d", f, b)
	}
}

// CA-19 — usuário que nunca salvou recebe os defaults 25/5 no GET.
func TestGetPomodoroSemConfigDevolveDefaults(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	rec := doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if f, b := decodeSettings(t, rec); f != 25 || b != 5 {
		t.Errorf("defaults esperados 25/5, vieram %d/%d", f, b)
	}
}

// CA-28 — valores fora da faixa (focus 15–90, break 1–60) respondem 400 e
// a configuração anterior é PRESERVADA.
func TestPutPomodoroForaDaFaixaPreservaAnterior(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	if rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie,
		`{"focus_minutes":50,"break_minutes":15}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT inicial: %d", rec.Code)
	}

	for _, payload := range []string{
		`{"focus_minutes":10,"break_minutes":15}`, // focus < 15
		`{"focus_minutes":91,"break_minutes":15}`, // focus > 90
		`{"focus_minutes":50,"break_minutes":0}`,  // break < 1
		`{"focus_minutes":50,"break_minutes":61}`, // break > 60
	} {
		rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s deveria responder 400 validation_error, veio %d %s",
				payload, rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if f, b := decodeSettings(t, rec); f != 50 || b != 15 {
		t.Errorf("config anterior deveria seguir 50/15 após PUTs inválidos, veio %d/%d", f, b)
	}
}
