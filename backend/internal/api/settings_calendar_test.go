// settings_calendar_test.go — fase 11 T-03: GET|PUT /api/v1/settings/calendar
// (CA-01/02/03/04). Rota POR USUÁRIO (user da sessão, sem workspace na URL).
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func decodeCalSettings(t *testing.T, rec *httptest.ResponseRecorder) (view, density string, hs, he int) {
	t.Helper()
	var body struct {
		DefaultView string `json:"default_view"`
		HourStart   int    `json:"hour_start"`
		HourEnd     int    `json:"hour_end"`
		Density     string `json:"density"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta fora do contrato: %s", rec.Body.String())
	}
	return body.DefaultView, body.Density, body.HourStart, body.HourEnd
}

// CA-01 — sem config salva, GET devolve os defaults week/6/23/comfortable.
func TestGetCalendarSettingsDefaults(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-cal1", "cal1@tt.dev", "Cal")

	rec := doJSON(router, http.MethodGet, "/api/v1/settings/calendar", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if v, d, hs, he := decodeCalSettings(t, rec); v != "week" || d != "comfortable" || hs != 6 || he != 23 {
		t.Errorf("defaults esperados week/6/23/comfortable, vieram %s/%d/%d/%s", v, hs, he, d)
	}
}

// CA-02 — PUT válido persiste por usuário e o GET seguinte ecoa.
func TestPutCalendarSettingsPersiste(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-cal2", "cal2@tt.dev", "Cal")

	rec := doJSON(router, http.MethodPut, "/api/v1/settings/calendar", cookie,
		`{"default_view":"day","hour_start":8,"hour_end":20,"density":"compact"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if v, d, hs, he := decodeCalSettings(t, rec); v != "day" || d != "compact" || hs != 8 || he != 20 {
		t.Errorf("PUT deveria ecoar o salvo, veio %s/%d/%d/%s", v, hs, he, d)
	}

	rec = doJSON(router, http.MethodGet, "/api/v1/settings/calendar", cookie, "")
	if v, d, hs, he := decodeCalSettings(t, rec); v != "day" || d != "compact" || hs != 8 || he != 20 {
		t.Errorf("GET deveria devolver o persistido, veio %s/%d/%d/%s", v, hs, he, d)
	}
}

// CA-03 — PUT inválido responde 400 validation_error e PRESERVA o anterior.
func TestPutCalendarSettingsInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCalendarRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-cal3", "cal3@tt.dev", "Cal")

	if rec := doJSON(router, http.MethodPut, "/api/v1/settings/calendar", cookie,
		`{"default_view":"day","hour_start":8,"hour_end":20,"density":"compact"}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT inicial: %d", rec.Code)
	}

	for _, payload := range []string{
		`{"default_view":"year","hour_start":8,"hour_end":20,"density":"compact"}`,
		`{"default_view":"day","hour_start":8,"hour_end":20,"density":"cozy"}`,
		`{"default_view":"day","hour_start":-1,"hour_end":20,"density":"compact"}`,
		`{"default_view":"day","hour_start":8,"hour_end":25,"density":"compact"}`,
		`{"default_view":"day","hour_start":20,"hour_end":20,"density":"compact"}`,
	} {
		rec := doJSON(router, http.MethodPut, "/api/v1/settings/calendar", cookie, payload)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("%s deveria responder 400 validation_error, veio %d %s", payload, rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(router, http.MethodGet, "/api/v1/settings/calendar", cookie, "")
	if v, _, hs, he := decodeCalSettings(t, rec); v != "day" || hs != 8 || he != 20 {
		t.Errorf("config anterior deveria seguir day/8/20 após PUTs inválidos, veio %s/%d/%d", v, hs, he)
	}
}

// CA-04 — sem sessão, as duas rotas respondem 401 canônico.
func TestCalendarSettingsSemSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, _, _ := newCalendarRouter(t, pool)

	for _, m := range []struct {
		method, body string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{"default_view":"day","hour_start":8,"hour_end":20,"density":"compact"}`},
	} {
		rec := doJSON(router, m.method, "/api/v1/settings/calendar", nil, m.body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sem sessão esperava 401, veio %d", m.method, rec.Code)
		}
	}
}
