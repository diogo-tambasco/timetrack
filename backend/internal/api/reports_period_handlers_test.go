// reports_period_handlers_test.go — T-07 da fase 14: contrato HTTP do
// relatório de período genérico (/reports) e das settings (/settings/reports),
// + RNF-03 (perf mês com compare). NÃO toca os testes da 07 (reports_handlers_
// test.go INTOCADO — CA-04). Reusa newReportsRouter/doJSON/errCode/addMember.
//
// Exceção red→green no caso de perf (precedente do RNF-02 da 07): a asserção de
// latência não tem red estrutural — a validade vem do volume e do limite.
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
)

func periodPath(wsID, query string) string {
	p := "/api/v1/workspaces/" + wsID + "/reports"
	if query != "" {
		p += "?" + query
	}
	return p
}

// CA-02 — cadeia 01/02 no /reports: sem sessão 401; ws alheio 404; viewer lê.
func TestPeriodReportCadeiaAuth(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)

	ana := h.login("g-pr-ana", "pra@tt.dev", "Ana")
	_ = h.login("g-pr-bia", "prb@tt.dev", "Bia")
	wsBia := workspaceIDOf(t, pool, "g-pr-bia")

	if rec := doJSON(router, http.MethodGet, periodPath(wsBia, ""), nil, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem sessão: %d, esperava 401", rec.Code)
	}
	if rec := doJSON(router, http.MethodGet, periodPath(wsBia, ""), ana, ""); rec.Code != http.StatusNotFound {
		t.Errorf("ws alheio: %d, esperava 404", rec.Code)
	}

	// viewer lê
	_ = h.login("g-pr-dono", "prd@tt.dev", "Dono")
	viewer := h.login("g-pr-view", "prv@tt.dev", "View")
	wsDono := workspaceIDOf(t, pool, "g-pr-dono")
	addMember(t, pool, wsDono, userIDOf(t, pool, "g-pr-view"), "viewer")
	if rec := doJSON(router, http.MethodGet, periodPath(wsDono, "tz=America/Sao_Paulo"), viewer, ""); rec.Code != http.StatusOK {
		t.Errorf("viewer: %d, esperava 200 (%s)", rec.Code, rec.Body.String())
	}
}

// CA-07 — period/date/from/to/tz/filtros malformados → 400 validation_error.
func TestPeriodReportMalformado(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ana := h.login("g-pr-400", "pr400@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-pr-400")

	for _, q := range []string{
		"period=semana", "date=banana", "from=banana&to=2026-06-10",
		"from=2026-06-11&to=2026-06-10", "tz=Marte/Cratera",
		"project_id=nao-uuid", "client_id=nao-uuid", "objective_id=nao-uuid",
		"from=2026-06-10", // from sem to
	} {
		rec := doJSON(router, http.MethodGet, periodPath(ws, q), ana, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, esperava 400 (%s)", q, rec.Code, rec.Body.String())
			continue
		}
		if code := errCode(t, rec); code != "validation_error" {
			t.Errorf("%s: code %q, esperava validation_error", q, code)
		}
	}
}

// Shape do §3.1 — período vazio: from/to ecoados; listas [] (não null); 24
// buckets em focus_by_hour; previous AUSENTE sem compare (CA-08/26/32).
func TestPeriodReportShapeVazio(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ana := h.login("g-pr-sv", "prsv@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-pr-sv")

	rec := doJSON(router, http.MethodGet,
		periodPath(ws, "period=week&date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &asMap); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	for _, k := range []string{"by_project", "goals", "objectives"} {
		if string(asMap[k]) != "[]" {
			t.Errorf("%s deveria ser [], veio %s", k, asMap[k])
		}
	}
	if _, has := asMap["previous"]; has {
		t.Errorf("previous deveria estar AUSENTE sem compare, veio %s", asMap["previous"])
	}

	var rep struct {
		From        string `json:"from"`
		To          string `json:"to"`
		FocusByHour []struct {
			Hour         int `json:"hour"`
			FocusSeconds int `json:"focus_seconds"`
		} `json:"focus_by_hour"`
	}
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.From != "2026-06-08" || rep.To != "2026-06-14" {
		t.Errorf("período %s→%s, esperava 2026-06-08→14", rep.From, rep.To)
	}
	if len(rep.FocusByHour) != 24 {
		t.Errorf("focus_by_hour com %d buckets, esperava 24", len(rep.FocusByHour))
	}
}

// CA-04/09 — /reports/week (07) segue respondendo o MESMO shape do /reports?
// period=week pro mesmo período (prova de não-regressão semântica via HTTP).
func TestWeekAliasNaoRegride(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ctx := context.Background()
	ana := h.login("g-pr-al", "pral@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-pr-al")
	user := userIDOf(t, pool, "g-pr-al")

	if _, err := pool.Exec(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, tags,
		                          started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, NULL, 'x', '{}', '2026-06-09T12:00:00Z', '2026-06-09T13:00:00Z', 3600, 'manual')`,
		ws, user); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	// /reports/week (07)
	recW := doJSON(router, http.MethodGet,
		"/api/v1/workspaces/"+ws+"/reports/week?date=2026-06-10&tz=America/Sao_Paulo", ana, "")
	var w struct {
		WeekStart    string `json:"week_start"`
		WeekEnd      string `json:"week_end"`
		TotalSeconds int    `json:"total_seconds"`
	}
	if err := json.Unmarshal(recW.Body.Bytes(), &w); err != nil {
		t.Fatalf("decode week: %v", err)
	}
	if w.WeekStart != "2026-06-08" || w.WeekEnd != "2026-06-14" || w.TotalSeconds != 3600 {
		t.Errorf("/reports/week mudou: %+v", w)
	}

	// /reports?period=week — mesmo total e mesmas datas (em from/to)
	recP := doJSON(router, http.MethodGet,
		periodPath(ws, "period=week&date=2026-06-10&tz=America/Sao_Paulo"), ana, "")
	var p struct {
		From         string `json:"from"`
		To           string `json:"to"`
		TotalSeconds int    `json:"total_seconds"`
	}
	if err := json.Unmarshal(recP.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode period: %v", err)
	}
	if p.From != w.WeekStart || p.To != w.WeekEnd || p.TotalSeconds != w.TotalSeconds {
		t.Errorf("/reports?period=week divergiu de /reports/week: %+v vs %+v", p, w)
	}
}

// CA-25 — compare=previous traz o bloco previous com total+kpis.
func TestPeriodReportCompare(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ctx := context.Background()
	ana := h.login("g-pr-cmp", "prcmp@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-pr-cmp")
	user := userIDOf(t, pool, "g-pr-cmp")

	// semana atual 08–14 (entry dia 09) e anterior 01–07 (entry dia 03)
	if _, err := pool.Exec(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, description, tags, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, 'a', '{}', '2026-06-09T12:00:00Z', '2026-06-09T12:10:00Z', 600, 'manual'),
		       ($1, $2, 'b', '{}', '2026-06-03T12:00:00Z', '2026-06-03T12:30:00Z', 1800, 'manual')`,
		ws, user); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := doJSON(router, http.MethodGet,
		periodPath(ws, "period=week&date=2026-06-10&tz=UTC&compare=previous"), ana, "")
	var rep struct {
		TotalSeconds int `json:"total_seconds"`
		Previous     *struct {
			TotalSeconds int `json:"total_seconds"`
			Kpis         struct {
				TrackedSeconds int `json:"tracked_seconds"`
			} `json:"kpis"`
		} `json:"previous"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.TotalSeconds != 600 {
		t.Errorf("atual %d, esperava 600", rep.TotalSeconds)
	}
	if rep.Previous == nil || rep.Previous.TotalSeconds != 1800 || rep.Previous.Kpis.TrackedSeconds != 1800 {
		t.Errorf("previous errado: %+v", rep.Previous)
	}
}

// CA-29 — /settings/reports: GET defaults; PUT válido; PUT inválido 400.
func TestReportSettingsHTTP(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ana := h.login("g-pr-set", "prset@tt.dev", "Ana")

	// GET sem linha → defaults 8/true
	rec := doJSON(router, http.MethodGet, "/api/v1/settings/reports", ana, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settings: %d (%s)", rec.Code, rec.Body.String())
	}
	var st struct {
		WorkHoursPerDay float64 `json:"work_hours_per_day"`
		WorkdaysOnly    bool    `json:"workdays_only"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.WorkHoursPerDay != 8 || !st.WorkdaysOnly {
		t.Errorf("defaults %+v, esperava 8/true", st)
	}

	// PUT válido
	rec = doJSON(router, http.MethodPut, "/api/v1/settings/reports", ana,
		`{"work_hours_per_day":6,"workdays_only":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT válido: %d (%s)", rec.Code, rec.Body.String())
	}
	rec = doJSON(router, http.MethodGet, "/api/v1/settings/reports", ana, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.WorkHoursPerDay != 6 || st.WorkdaysOnly {
		t.Errorf("após PUT %+v, esperava 6/false", st)
	}

	// PUT inválido → 400 validation_error
	rec = doJSON(router, http.MethodPut, "/api/v1/settings/reports", ana,
		`{"work_hours_per_day":0,"workdays_only":true}`)
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("PUT inválido: %d code=%q, esperava 400 validation_error", rec.Code, errCode(t, rec))
	}
}

// RNF-03 — mês com compare=previous (pior caso: 2 janelas + objetivos) com
// 1.000+ entries responde em <200ms (3 amostras).
func TestPeriodReportPerfMesCompare(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, h := newReportsRouter(t, pool)
	ctx := context.Background()
	ana := h.login("g-pr-perf", "prperf@tt.dev", "Ana")
	ws := workspaceIDOf(t, pool, "g-pr-perf")
	user := userIDOf(t, pool, "g-pr-perf")

	var projID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, client_id, name, color, weekly_hours_goal, visibility, owner_user_id)
		VALUES ($1, NULL, 'Perf', '#4f9dde', 10, 'shared', $2) RETURNING id`,
		ws, user).Scan(&projID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	// alguns objetivos + tasks (a seção objectives roda em todo request)
	for i := 0; i < 10; i++ {
		var objID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO objectives (workspace_id, project_id, name, status, created_by)
			VALUES ($1, $2, $3, 'open', $4) RETURNING id`,
			ws, projID, fmt.Sprintf("obj%d", i), user).Scan(&objID); err != nil {
			t.Fatalf("seed obj: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO tasks (workspace_id, project_id, name, status, objective_id, created_by)
			SELECT $1, $2, 'tk'||g, (CASE WHEN g%2=0 THEN 'done' ELSE 'todo' END)::task_status, $3, $4
			  FROM generate_series(1,5) g`, ws, projID, objID, user); err != nil {
			t.Fatalf("seed tasks: %v", err)
		}
	}
	// 1.200 entries em junho/maio (cobre período + previous)
	if _, err := pool.Exec(ctx, `
		INSERT INTO time_entries (workspace_id, user_id, project_id, description, tags,
		                          started_at, stopped_at, duration_seconds, source)
		SELECT $1, $2, $3::uuid, 'perf '||i, '{}',
		       '2026-05-15T03:00:00Z'::timestamptz + (i || ' minutes')::interval,
		       '2026-05-15T03:00:00Z'::timestamptz + (i || ' minutes')::interval + interval '5 minutes',
		       300, 'manual'
		  FROM generate_series(1, 1200) i`, ws, user, projID); err != nil {
		t.Fatalf("seed 1200 entries: %v", err)
	}

	q := "period=month&date=2026-06-10&tz=America/Sao_Paulo&compare=previous"
	for i := range 3 {
		inicio := time.Now()
		rec := doJSON(router, http.MethodGet, periodPath(ws, q), ana, "")
		dur := time.Since(inicio)
		if rec.Code != http.StatusOK {
			t.Fatalf("amostra %d: status %d (%s)", i, rec.Code, rec.Body.String())
		}
		if dur >= 200*time.Millisecond {
			t.Errorf("amostra %d: %v, limite 200ms (RNF-03)", i, dur)
		}
	}
}
