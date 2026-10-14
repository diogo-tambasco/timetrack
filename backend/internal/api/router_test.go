package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func doRequest(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := NewRouter()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

type errEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// CA-01: GET /healthz → 200 {"status":"ok"} com Content-Type JSON.
func TestHealthz_200JSON(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status esperado 200, veio %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type esperado application/json, veio %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`corpo esperado {"status":"ok"}, veio %s`, rec.Body.String())
	}
}

// CA-07a: rota inexistente sob /api/v1/ → 404 com envelope canônico.
func TestAPIv1_RotaInexistente_404Envelope(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/api/v1/nao-existe")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status esperado 404, veio %d", rec.Code)
	}
	var env errEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	if env.Error.Code != "not_found" {
		t.Errorf("code esperado not_found, veio %q", env.Error.Code)
	}
	if env.Error.Message != "rota não encontrada" {
		t.Errorf("message esperada 'rota não encontrada', veio %q", env.Error.Message)
	}
}

// CA-07b: método não permitido em rota existente → 405 com envelope canônico.
// Exemplo canônico exercitável nesta fase: DELETE /healthz.
func TestHealthz_MetodoErrado_405Envelope(t *testing.T) {
	rec := doRequest(t, http.MethodDelete, "/healthz")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status esperado 405, veio %d", rec.Code)
	}
	var env errEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	if env.Error.Code != "method_not_allowed" {
		t.Errorf("code esperado method_not_allowed, veio %q", env.Error.Code)
	}
	if env.Error.Message != "método não permitido" {
		t.Errorf("message esperada 'método não permitido', veio %q", env.Error.Message)
	}
}
