// settings_pomodoro_cycles_test.go — fase 15 T-04: GET|PUT
// /api/v1/settings/pomodoro estendido (Pomodoro 2.0). GET devolve os 7
// campos com defaults (CA-01); PUT completo persiste (CA-02); PUT curto
// (corpo legado da fase 03) NÃO zera os campos novos (CA-02.1, merge);
// inválido 400 validation_error (CA-03); 401 sem sessão (CA-01/02/04).
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type pomodoroBody struct {
	FocusMinutes     int  `json:"focus_minutes"`
	BreakMinutes     int  `json:"break_minutes"`
	CyclesBeforeLong int  `json:"cycles_before_long"`
	LongBreakMinutes int  `json:"long_break_minutes"`
	AutoStartBreak   bool `json:"auto_start_break"`
	AutoStartFocus   bool `json:"auto_start_focus"`
	SoundEnabled     bool `json:"sound_enabled"`
}

func decodePomodoro(t *testing.T, rec *httptest.ResponseRecorder) pomodoroBody {
	t.Helper()
	var body pomodoroBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta fora do contrato: %s", rec.Body.String())
	}
	return body
}

// CA-01 — GET sem config devolve os 7 defaults: 25/5/4/15/true/true/false.
// auto-start true/true PRESERVA o ciclo automático da fase 03.
func TestGetPomodoroDefaultsSeteCampos(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	rec := doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	got := decodePomodoro(t, rec)
	want := pomodoroBody{
		FocusMinutes: 25, BreakMinutes: 5, CyclesBeforeLong: 4,
		LongBreakMinutes: 15, AutoStartBreak: true, AutoStartFocus: true, SoundEnabled: false,
	}
	if got != want {
		t.Errorf("defaults esperados %+v, vieram %+v", want, got)
	}
}

// CA-02 — PUT completo (7 campos) persiste e o GET seguinte devolve igual.
func TestPutPomodoroCompletoPersiste(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	body := `{"focus_minutes":50,"break_minutes":10,"cycles_before_long":3,
	          "long_break_minutes":30,"auto_start_break":false,
	          "auto_start_focus":false,"sound_enabled":true}`
	rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	want := pomodoroBody{
		FocusMinutes: 50, BreakMinutes: 10, CyclesBeforeLong: 3,
		LongBreakMinutes: 30, AutoStartBreak: false, AutoStartFocus: false, SoundEnabled: true,
	}
	if got := decodePomodoro(t, rec); got != want {
		t.Errorf("PUT deveria ecoar %+v, veio %+v", want, got)
	}
	rec = doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if got := decodePomodoro(t, rec); got != want {
		t.Errorf("GET deveria devolver %+v persistido, veio %+v", want, got)
	}
}

// CA-02.1 (merge) — um PUT legado só com {focus,break} NÃO zera os campos
// novos: parte do persistido atual. O front da fase 03 não pode apagar os
// ciclos/auto-start/som.
func TestPutPomodoroLegadoNaoZeraCamposNovos(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	// primeiro salva uma config 2.0 distinta dos defaults
	full := `{"focus_minutes":50,"break_minutes":10,"cycles_before_long":3,
	          "long_break_minutes":30,"auto_start_break":false,
	          "auto_start_focus":true,"sound_enabled":true}`
	if rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie, full); rec.Code != http.StatusOK {
		t.Fatalf("PUT full: %d %s", rec.Code, rec.Body.String())
	}

	// agora um PUT CURTO (corpo legado da fase 03) muda só focus/break
	short := `{"focus_minutes":40,"break_minutes":8}`
	rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie, short)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT curto: %d %s", rec.Code, rec.Body.String())
	}
	got := decodePomodoro(t, rec)
	// focus/break atualizados; o resto PRESERVADO da config 2.0 anterior
	want := pomodoroBody{
		FocusMinutes: 40, BreakMinutes: 8, CyclesBeforeLong: 3,
		LongBreakMinutes: 30, AutoStartBreak: false, AutoStartFocus: true, SoundEnabled: true,
	}
	if got != want {
		t.Errorf("PUT legado deveria preservar os campos novos: esperava %+v, veio %+v", want, got)
	}
}

// CA-03 — PUT inválido (cycles=13) responde 400 validation_error e nada
// persiste.
func TestPutPomodoroInvalido400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _ := newTimerRouter(t, pool, nil)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")

	for _, payload := range []string{
		`{"focus_minutes":25,"break_minutes":5,"cycles_before_long":13,"long_break_minutes":15}`,
		`{"focus_minutes":25,"break_minutes":5,"cycles_before_long":4,"long_break_minutes":121}`,
	} {
		rec := doJSON(router, http.MethodPut, "/api/v1/settings/pomodoro", cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s deveria responder 400 validation_error, veio %d %s",
				payload, rec.Code, rec.Body.String())
		}
	}
	// config segue nos defaults (nada persistiu)
	rec := doJSON(router, http.MethodGet, "/api/v1/settings/pomodoro", cookie, "")
	if got := decodePomodoro(t, rec); got.CyclesBeforeLong != 4 || got.LongBreakMinutes != 15 {
		t.Errorf("config deveria seguir nos defaults após inválidos, veio %+v", got)
	}
}

// CA-01/02/04 — sem sessão, GET e PUT respondem 401 canônico.
func TestPomodoroSemSessao401(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, _, _, _ := newTimerRouter(t, pool, nil)

	for _, m := range []struct {
		method, body string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{"focus_minutes":25,"break_minutes":5}`},
	} {
		rec := doJSON(router, m.method, "/api/v1/settings/pomodoro", nil, m.body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sem sessão deveria responder 401, veio %d", m.method, rec.Code)
		}
	}
}
