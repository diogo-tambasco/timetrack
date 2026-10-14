// reports_handlers_test.go — T-04 da fase 07: contrato HTTP do relatório
// semanal (CA-02/03/08 + shape §3.1) e RNF-02 (perf com 1.000+ entries).
//
// Exceção red→green documentada no caso de perf (precedente do
// timer_perf_test.go da 03): asserção de latência não tem red estrutural —
// a validade vem do volume semeado e do limite apertado.
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
	"timetrack.live/backend/internal/reports"
	"timetrack.live/backend/internal/workspace"
)

// newReportsRouter monta o router real com auth da 01 + reports da 07.
func newReportsRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *struct {
	login func(sub, email, name string) *http.Cookie
}) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithReports(reports.NewService(pool)),
	)
	h := &struct {
		login func(sub, email, name string) *http.Cookie
	}{}
	h.login = func(sub, email, name string) *http.Cookie {
		return loginVia(t, svc, stub, sub, email, name)
	}
	return router, h
}

func reportPath(wsID, query string) string {
	p := "/api/v1/workspaces/" + wsID + "/reports/week"
	if query != "" {
		p += "?" + query
	}
	return p
}

type reportJSON struct {
	WeekStart    string `json:"week_start"`
	WeekEnd      string `json:"week_end"`
	TotalSeconds int    `json:"total_seconds"`
	ByProject    []struct {
		ProjectID    *string `json:"project_id"`
		Name         *string `json:"name"`
		Color        *string `json:"color"`
		ClientID     *string `json:"client_id"`
		ClientName   *string `json:"client_name"`
		TotalSeconds int     `json:"total_seconds"`
	} `json:"by_project"`
	Focus struct {
		FocusSeconds    int `json:"focus_seconds"`
		FragmentSeconds int `json:"fragment_seconds"`
		FocusCount      int `json:"focus_count"`
		FragmentCount   int `json:"fragment_count"`
		LongestSeconds  int `json:"longest_seconds"`
		Switches        int `json:"switches"`
	} `json:"focus"`
	Goals []struct {
		ProjectID       string `json:"project_id"`
		Name            string `json:"name"`
		Color           string `json:"color"`
		GoalSeconds     int    `json:"goal_seconds"`
		RealizedSeconds int    `json:"realized_seconds"`
	} `json:"goals"`
}

// CA-02 — cadeia da 01/02: sem sessão 401; workspace alheio 404 uniforme.
func TestReportsCadeiaAuth(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)

	ana := h.login("g-rep-ana", "ana@tt.dev", "Ana")
	_ = h.login("g-rep-bia", "bia@tt.dev", "Bia")
	wsBia := workspaceIDOf(t, pool, "g-rep-bia")

	// sem sessão → 401
	rec := doJSON(router, http.MethodGet, reportPath(wsBia, ""), nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sem sessão: %d, esperava 401", rec.Code)
	}

	// workspace alheio → 404 uniforme (não vaza existência)
	rec = doJSON(router, http.MethodGet, reportPath(wsBia, ""), ana, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("ws alheio: %d, esperava 404", rec.Code)
	}
}

// CA-03 — viewer PODE ler o relatório (GET passa no RequireWriteRole).
func TestReportsViewerLe(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)

	_ = h.login("g-rep-dono", "dono@tt.dev", "Dono")
	viewer := h.login("g-rep-view", "view@tt.dev", "View")
	wsDono := workspaceIDOf(t, pool, "g-rep-dono")
	addMember(t, pool, wsDono, userIDOf(t, pool, "g-rep-view"), "viewer")

	rec := doJSON(router, http.MethodGet, reportPath(wsDono, "tz=America/Sao_Paulo"), viewer, "")
	if rec.Code != http.StatusOK {
		t.Errorf("viewer: %d, esperava 200 (corpo: %s)", rec.Code, rec.Body.String())
	}
}

// CA-08 — date/tz malformados → 400 validation_error (convenção do INDEX).
func TestReportsParamsMalformados(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)

	ana := h.login("g-rep-400", "p400@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-rep-400")

	for _, q := range []string{"date=12-06-2026", "date=banana", "tz=Marte/Cratera"} {
		rec := doJSON(router, http.MethodGet, reportPath(ws, q), ana, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, esperava 400", q, rec.Code)
			continue
		}
		if code := errCode(t, rec); code != "validation_error" {
			t.Errorf("%s: code %q, esperava validation_error", q, code)
		}
	}
}

// Shape do contrato §3.1 — semana vazia: listas SEMPRE [] (nunca null),
// focus zerado, week_start/week_end refletindo date+tz.
func TestReportsShapeVazio(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)

	ana := h.login("g-rep-shape", "shape@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-rep-shape")

	rec := doJSON(router, http.MethodGet,
		reportPath(ws, "date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	// listas vazias têm que vir como [] no JSON CRU (não null)
	raw := rec.Body.String()
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &asMap); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if string(asMap["by_project"]) != "[]" {
		t.Errorf("by_project deveria ser [], veio %s", asMap["by_project"])
	}
	if string(asMap["goals"]) != "[]" {
		t.Errorf("goals deveria ser [], veio %s", asMap["goals"])
	}

	var rep reportJSON
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.WeekStart != "2026-06-08" || rep.WeekEnd != "2026-06-14" {
		t.Errorf("semana %s→%s, esperava 2026-06-08→2026-06-14", rep.WeekStart, rep.WeekEnd)
	}
	if rep.TotalSeconds != 0 || rep.Focus.LongestSeconds != 0 {
		t.Errorf("semana vazia: total %d, longest %d", rep.TotalSeconds, rep.Focus.LongestSeconds)
	}
}

// Shape com dados — denormalização (name/color/client_name) atravessa a
// camada HTTP; goals com realized.
func TestReportsShapeComDados(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ctx := context.Background()

	ana := h.login("g-rep-dados", "dados@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-rep-dados")
	user := userIDOf(t, pool, "g-rep-dados")

	var clienteID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO clients (workspace_id, name) VALUES ($1, 'Lauda.ai') RETURNING id`,
		ws).Scan(&clienteID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	var projID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal,
		                      visibility, owner_user_id)
		VALUES ($1, $2, 'Lauda', '#b07cd6', 10, 'shared', $3) RETURNING id`,
		ws, clienteID, user).Scan(&projID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	// terça 2026-06-09 12:00 UTC, 1h — dentro da semana 08–14
	if _, err := pool.Exec(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, tags,
		                          started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, 'x', '{}', '2026-06-09T12:00:00Z', '2026-06-09T13:00:00Z', 3600, 'manual')`,
		ws, user, projID); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	rec := doJSON(router, http.MethodGet,
		reportPath(ws, "date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var rep reportJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rep.ByProject) != 1 || rep.ByProject[0].Name == nil || *rep.ByProject[0].Name != "Lauda" ||
		rep.ByProject[0].ClientName == nil || *rep.ByProject[0].ClientName != "Lauda.ai" {
		t.Errorf("by_project sem denormalização: %+v", rep.ByProject)
	}
	if len(rep.Goals) != 1 || rep.Goals[0].GoalSeconds != 36000 || rep.Goals[0].RealizedSeconds != 3600 {
		t.Errorf("goals errados: %+v", rep.Goals)
	}
	if rep.Focus.FocusCount != 1 || rep.Focus.FocusSeconds != 3600 {
		t.Errorf("focus errado: %+v", rep.Focus)
	}
}

// RNF-02 — relatório da semana com 1.000+ entries no workspace responde
// nas 3 amostras em < 200ms (p95 nearest-rank de n=3 = máximo — padrão do
// RNF-04 da fase 03).
func TestReportsPerf1kEntries(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ctx := context.Background()

	ana := h.login("g-rep-perf", "perf@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-rep-perf")
	user := userIDOf(t, pool, "g-rep-perf")

	var projID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal,
		                      visibility, owner_user_id)
		VALUES ($1, NULL, 'Perf', '#4f9dde', 10, 'shared', $2) RETURNING id`,
		ws, user).Scan(&projID); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// 1.200 entries fechadas espalhadas pela semana 08–14 (generate_series)
	if _, err := pool.Exec(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, tags,
		                          started_at, stopped_at, duration_seconds, source)
		SELECT $1, $2, CASE WHEN i % 3 = 0 THEN NULL ELSE $3::uuid END, 'perf '||i, '{}',
		       '2026-06-08T03:00:00Z'::timestamptz + (i || ' minutes')::interval,
		       '2026-06-08T03:00:00Z'::timestamptz + (i || ' minutes')::interval + interval '5 minutes',
		       300, 'manual'
		  FROM generate_series(1, 1200) i`, ws, user, projID); err != nil {
		t.Fatalf("seed 1200 entries: %v", err)
	}

	for i := range 3 {
		inicio := time.Now()
		rec := doJSON(router, http.MethodGet,
			reportPath(ws, "date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
		dur := time.Since(inicio)
		if rec.Code != http.StatusOK {
			t.Fatalf("amostra %d: status %d", i, rec.Code)
		}
		if dur >= 200*time.Millisecond {
			t.Errorf("amostra %d: %v, limite 200ms (RNF-02)", i, dur)
		}
	}

	// sanidade do volume: o relatório realmente agregou as 1200
	rec := doJSON(router, http.MethodGet,
		reportPath(ws, "date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
	var rep reportJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.TotalSeconds != 1200*300 {
		t.Errorf("total %d, esperava %d", rep.TotalSeconds, 1200*300)
	}
}
