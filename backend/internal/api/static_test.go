package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func staticRouter() http.Handler {
	return NewRouter(WithStaticFS(fstest.MapFS{
		"index.html":           {Data: []byte("<html>shell timetrack</html>")},
		"assets/app.js":        {Data: []byte("console.log('app')")},
		"manifest.webmanifest": {Data: []byte(`{"name":"Timetrack"}`)},
	}))
}

// CA-19: rota sem extensão fora de /api e /healthz → index.html 200.
func TestSPA_RotaDesconhecida_ServeIndex(t *testing.T) {
	for _, path := range []string{"/", "/ajustes", "/projetos/abc"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		staticRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: esperado 200, veio %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "shell timetrack") {
			t.Errorf("%s: esperava o index.html, veio %q", path, rec.Body.String())
		}
	}
}

// CA-19: arquivos existentes são servidos como estáticos.
func TestSPA_Asset_ServeArquivo(t *testing.T) {
	cases := map[string]string{
		"/assets/app.js":        "console.log('app')",
		"/manifest.webmanifest": `{"name":"Timetrack"}`,
	}
	for path, want := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		staticRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: esperado 200, veio %d", path, rec.Code)
		}
		if rec.Body.String() != want {
			t.Errorf("%s: corpo esperado %q, veio %q", path, want, rec.Body.String())
		}
	}
}

// CA-19: o fallback SPA NÃO engole o 404 JSON de /api/v1/ (CA-07).
func TestSPA_NaoEngoleAPIv1(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nao-existe", nil)
	rec := httptest.NewRecorder()
	staticRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"not_found"`) {
		t.Errorf("esperava envelope JSON de erro, veio %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "shell timetrack") {
		t.Error("fallback SPA engoliu a rota de API")
	}
}
