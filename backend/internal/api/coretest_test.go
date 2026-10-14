// coretest_test.go — harness compartilhado dos testes da fase 02: login
// pelo Google stubbado (fase 01), memberships por SQL e requisições JSON.
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/properties"
	"timetrack.live/backend/internal/subtasks"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/workspace"
)

// newCoreRouter monta o router REAL da aplicação (auth da fase 01 + domínio
// da fase 02) contra o pool de teste.
func newCoreRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasks.NewService(pool),
			Objectives: objectives.NewService(pool),
			Subtasks:   subtasks.NewService(pool),
			Properties: properties.NewService(pool),
		}),
	)
	return router, svc, stub
}

// newAuthService sobe o auth da fase 01 contra o stub do Google.
func newAuthService(t *testing.T, pool *pgxpool.Pool) (*auth.Service, *googlestub.Server) {
	t.Helper()
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
	return svc, stub
}

// loginVia roda o callback stubbado e devolve o cookie de sessão (cada
// user nasce com workspace pessoal + role owner — fase 01).
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

func userIDOf(t *testing.T, pool *pgxpool.Pool, googleID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE google_id = $1`, googleID).Scan(&id); err != nil {
		t.Fatalf("user %s não existe: %v", googleID, err)
	}
	return id
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

// addMember vincula um user já existente a um workspace com a role dada.
func addMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)`, wsID, userID, role); err != nil {
		t.Fatalf("vincular %s como %s: %v", userID, role, err)
	}
}

// doJSON dispara uma requisição autenticada com corpo JSON opcional.
func doJSON(router http.Handler, method, path string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// errCode extrai o error.code do envelope canônico.
func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("resposta fora do envelope canônico: %s", rec.Body.String())
	}
	return env.Error.Code
}
