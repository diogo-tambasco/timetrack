// presence_routes_test.go — fase 21 T-05: GET /workspaces/{id}/presence
// (snapshot REST). 200 lista própria (running shared com nome / private sem
// nome) · 404 não-membro/ws inexistente/id malformado · 401 sem sessão ·
// isolamento de tenant (CA-11/12/13/14). Harness real (auth da 01 + presence).
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/presence"
	"timetrack.live/backend/internal/workspace"
)

// newPresenceRouter monta o router real com auth da 01 + presence da 21.
func newPresenceRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, func(sub, email, name string) *http.Cookie) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithPresence(presence.NewService(pool, func(string, any) {})),
	)
	login := func(sub, email, name string) *http.Cookie {
		return loginVia(t, svc, stub, sub, email, name)
	}
	return router, login
}

type presenceListJSON struct {
	Members []struct {
		UserID      string  `json:"user_id"`
		Name        string  `json:"name"`
		WorkspaceID string  `json:"workspace_id"`
		Running     bool    `json:"running"`
		Project     *string `json:"project"`
		Since       *string `json:"since"`
	} `json:"members"`
}

func seedRunningEntry(t *testing.T, pool *pgxpool.Pool, wsID, userID string, projectID *string, started time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, started_at, source)
		VALUES ($1, $2, $3, '', $4, 'timer')`,
		wsID, userID, projectID, started); err != nil {
		t.Fatalf("seed running entry: %v", err)
	}
}

func seedProject(t *testing.T, pool *pgxpool.Pool, wsID, ownerID, name, visibility string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, $3, '#112233', $4) RETURNING id`,
		wsID, ownerID, name, visibility).Scan(&id); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return id
}

// CA-11 — 200 lista os membros do ws com timer rodando; shared mostra nome,
// private omite; ocioso não aparece.
func TestPresenceSnapshot200ComVisibilidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, login := newPresenceRouter(t, pool)
	cookie := login("pres-viewer", "pres-viewer@b.c", "Viewer")
	ws := workspaceIDOf(t, pool, "pres-viewer")

	// dois colegas rodando, um ocioso
	a := mkUser(t, pool, "pres-a")
	b := mkUser(t, pool, "pres-b")
	c := mkUser(t, pool, "pres-c")
	addMember(t, pool, ws, a, "member")
	addMember(t, pool, ws, b, "member")
	addMember(t, pool, ws, c, "member")
	pShared := seedProject(t, pool, ws, a, "Site", "shared")
	pPrivate := seedProject(t, pool, ws, b, "Segredo", "private")
	started := time.Now().UTC().Add(-4 * time.Minute)
	seedRunningEntry(t, pool, ws, a, &pShared, started)
	seedRunningEntry(t, pool, ws, b, &pPrivate, started)

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/presence", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var out presenceListJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("corpo inválido: %s", rec.Body.String())
	}
	if len(out.Members) != 2 {
		t.Fatalf("esperava 2 rodando, veio %d: %+v", len(out.Members), out.Members)
	}
	byUser := map[string]*string{}
	for _, m := range out.Members {
		if !m.Running || m.WorkspaceID != ws || m.Since == nil {
			t.Errorf("item fora do contrato: %+v", m)
		}
		byUser[m.UserID] = m.Project
	}
	if byUser[a] == nil || *byUser[a] != "Site" {
		t.Errorf("A (shared) deveria mostrar o nome do projeto: %v", byUser[a])
	}
	if byUser[b] != nil {
		t.Errorf("B (private de B, viewer não dono) NÃO deveria mostrar o nome: %v", *byUser[b])
	}
}

// CA-11 — lista vazia quando ninguém está rodando → {"members":[]}.
func TestPresenceSnapshotVazio(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, login := newPresenceRouter(t, pool)
	cookie := login("pres-viewer", "pres-viewer@b.c", "Viewer")
	ws := workspaceIDOf(t, pool, "pres-viewer")

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/presence", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	var out presenceListJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("corpo inválido: %s", rec.Body.String())
	}
	if len(out.Members) != 0 {
		t.Errorf("ninguém rodando → members vazio, veio %+v", out.Members)
	}
}

// CA-12 — 404 pra não-membro / ws inexistente / id malformado.
func TestPresenceSnapshot404NaoMembro(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, login := newPresenceRouter(t, pool)
	cookie := login("pres-viewer", "pres-viewer@b.c", "Viewer")

	// ws de OUTRO usuário (não-membro)
	other := mkUser(t, pool, "pres-other")
	otherWS := mkWorkspaceFor(t, pool, other)
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+otherWS+"/presence", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("não-membro deveria dar 404, veio %d", rec.Code)
	}
	// ws inexistente (uuid válido mas sem linha)
	rec = doJSON(router, http.MethodGet,
		"/api/v1/workspaces/00000000-0000-0000-0000-000000000000/presence", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("ws inexistente deveria dar 404, veio %d", rec.Code)
	}
	// id malformado
	rec = doJSON(router, http.MethodGet, "/api/v1/workspaces/nao-uuid/presence", cookie, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("id malformado deveria dar 404, veio %d", rec.Code)
	}
}

// CA-13 — 401 sem sessão.
func TestPresenceSnapshot401SemSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, login := newPresenceRouter(t, pool)
	_ = login("pres-viewer", "pres-viewer@b.c", "Viewer")
	ws := workspaceIDOf(t, pool, "pres-viewer")

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/presence", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sem sessão deveria dar 401, veio %d", rec.Code)
	}
	if code := errCode(t, rec); code != "unauthorized" {
		t.Errorf("error.code deveria ser unauthorized, veio %q", code)
	}
}

// CA-14 — isolamento de tenant: entry rodando de um membro em OUTRO ws dele NÃO
// aparece no snapshot de W.
func TestPresenceSnapshotIsoladoPorTenant(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, login := newPresenceRouter(t, pool)
	cookie := login("pres-viewer", "pres-viewer@b.c", "Viewer")
	ws := workspaceIDOf(t, pool, "pres-viewer")

	a := mkUser(t, pool, "pres-iso-a")
	addMember(t, pool, ws, a, "member")
	// A também é dono de OUTRO ws e está rodando lá
	otherWS := mkWorkspaceFor(t, pool, a)
	seedRunningEntry(t, pool, otherWS, a, nil, time.Now().UTC())

	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/presence", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	var out presenceListJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Members) != 0 {
		t.Errorf("entry de A em OUTRO ws não pode aparecer no snapshot de W: %+v", out.Members)
	}
}

// — helpers locais —

// mkUser cria um user solto (sem ws) e devolve o id.
func mkUser(t *testing.T, pool *pgxpool.Pool, gid string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', $1) RETURNING id`,
		gid).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", gid, err)
	}
	return id
}

// mkWorkspaceFor cria um ws e adiciona o user como owner; devolve o ws id.
func mkWorkspaceFor(t *testing.T, pool *pgxpool.Pool, userID string) string {
	t.Helper()
	var wsID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ('outro') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	addMember(t, pool, wsID, userID, "owner")
	return wsID
}
