package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/workspace"
)

// loginVia roda o fluxo de login stubbado pro perfil dado e devolve o
// cookie de sessão (cada user nasce com seu workspace pessoal).
func loginVia(t *testing.T, svc *auth.Service, stub *googlestub.Server, sub, email, name string) *http.Cookie {
	t.Helper()
	stub.SetUserinfo(fmt.Appendf(nil,
		`{"sub":%q,"email":%q,"email_verified":true,"name":%q,"picture":""}`, sub, email, name))

	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code="+googlestub.CodeOK+"&state=ok", nil)
	req.AddCookie(&http.Cookie{Name: "tt_oauth_state", Value: "ok"})
	rec := httptest.NewRecorder()
	svc.HandleCallback(rec, req)

	for _, c := range rec.Result().Cookies() {
		if c.Name == "tt_session" && c.Value != "" {
			return c
		}
	}
	t.Fatalf("login de %s não devolveu sessão (status %d: %s)", email, rec.Code, rec.Body.String())
	return nil
}

func workspaceIDOf(t *testing.T, pool *pgxpool.Pool, googleID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		SELECT w.id FROM workspaces w
		  JOIN workspace_members m ON m.workspace_id = w.id
		  JOIN users u ON u.id = m.user_id
		 WHERE u.google_id = $1`, googleID).Scan(&id); err != nil {
		t.Fatalf("workspace de %s não existe: %v", googleID, err)
	}
	return id
}

// CA-15: user B requisitando o workspace de user A recebe 404 not_found
// sem nenhum dado; A no próprio workspace recebe 200 com role owner.
func TestIsolamentoEntreWorkspaces(t *testing.T) {
	pool := dbtest.NewPool(t)
	stub := googlestub.New("../../testdata/google")
	t.Cleanup(stub.Close)

	svc := auth.NewService(auth.Config{
		ClientID:     "client-id-teste",
		ClientSecret: "client-secret-teste",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		TokenKey:     []byte("0123456789abcdef0123456789abcdef"),
		Endpoint:     stub.Endpoint(),
		UserinfoURL:  stub.UserinfoURL(),
		DB:           pool,
	})
	store := workspace.NewStore(pool)

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(svc.RequireAuth)
		r.Route("/workspaces/{workspaceId}", func(r chi.Router) {
			r.Use(store.RequireMember)
			r.Get("/", store.HandleGetWorkspace)
		})
	})

	cookieA := loginVia(t, svc, stub, "g-user-a", "a@example.com", "Ana Alves")
	cookieB := loginVia(t, svc, stub, "g-user-b", "b@example.com", "Beto Braga")
	wsA := workspaceIDOf(t, pool, "g-user-a")

	// B tenta ler o workspace de A → 404, sem vazar nada
	reqB := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsA, nil)
	reqB.AddCookie(cookieB)
	recB := httptest.NewRecorder()
	router.ServeHTTP(recB, reqB)

	if recB.Code != http.StatusNotFound {
		t.Fatalf("B no workspace de A: esperado 404, veio %d: %s", recB.Code, recB.Body.String())
	}
	var envB map[string]json.RawMessage
	if err := json.NewDecoder(recB.Body).Decode(&envB); err != nil {
		t.Fatal(err)
	}
	if _, soErro := envB["error"]; !soErro || len(envB) != 1 {
		t.Errorf("resposta 404 deveria conter só o envelope de erro, veio %v", envB)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	raw, _ := json.Marshal(envB)
	_ = json.Unmarshal(raw, &env)
	if env.Error.Code != "not_found" {
		t.Errorf("error.code esperado not_found, veio %q", env.Error.Code)
	}

	// A no próprio workspace → 200 com role owner
	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsA, nil)
	reqA.AddCookie(cookieA)
	recA := httptest.NewRecorder()
	router.ServeHTTP(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Fatalf("A no próprio workspace: esperado 200, veio %d: %s", recA.Code, recA.Body.String())
	}
	var body struct {
		Workspace struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Role      string `json:"role"`
			CreatedAt string `json:"created_at"`
		} `json:"workspace"`
	}
	if err := json.NewDecoder(recA.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Workspace.ID != wsA || body.Workspace.Role != "owner" ||
		body.Workspace.Name != "Workspace de Ana" || body.Workspace.CreatedAt == "" {
		t.Errorf("workspace fora do contrato §3.5: %+v", body.Workspace)
	}

	// workspace inexistente → mesmo 404 (não vaza existência)
	reqX := httptest.NewRequest(http.MethodGet,
		"/api/v1/workspaces/00000000-0000-0000-0000-000000000000", nil)
	reqX.AddCookie(cookieA)
	recX := httptest.NewRecorder()
	router.ServeHTTP(recX, reqX)
	if recX.Code != http.StatusNotFound {
		t.Errorf("workspace inexistente: esperado 404, veio %d", recX.Code)
	}
}
