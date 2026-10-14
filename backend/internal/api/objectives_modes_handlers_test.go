// objectives_modes_handlers_test.go — fase 29 T-05: handlers com completion_mode
// + target_date (tri-state via json.RawMessage). CA-02/02.1/03/03.1/04/10.
package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

type objBody struct {
	ID             string   `json:"id"`
	CompletionMode string   `json:"completion_mode"`
	TargetDate     *string  `json:"target_date"`
	GoalHours      *float64 `json:"goal_hours"`
}

func decodeObj(t *testing.T, body []byte) objBody {
	t.Helper()
	var v objBody
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("corpo fora do shape: %s", body)
	}
	return v
}

func TestObjectivesModosHandlers(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	pid := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie,
		`{"name":"P","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	objBase := base + "/projects/" + pid + "/objectives"

	// POST by_hours + goal → 201 com modo no corpo (CA-02/10).
	rec := doJSON(router, http.MethodPost, objBase, cookie, `{"name":"Horas","completion_mode":"by_hours","goal_hours":40}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST by_hours: %d %s", rec.Code, rec.Body.String())
	}
	o := decodeObj(t, rec.Body.Bytes())
	if o.CompletionMode != "by_hours" {
		t.Errorf("completion_mode no corpo esperado by_hours, veio %q", o.CompletionMode)
	}

	// POST by_date + target_date → 201.
	rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"Marco","completion_mode":"by_date","target_date":"2026-07-15"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST by_date: %d %s", rec.Code, rec.Body.String())
	}
	oD := decodeObj(t, rec.Body.Bytes())
	if oD.CompletionMode != "by_date" || oD.TargetDate == nil || *oD.TargetDate != "2026-07-15" {
		t.Errorf("by_date esperado target_date=2026-07-15, veio mode=%q td=%v", oD.CompletionMode, oD.TargetDate)
	}

	// POST modo inválido → 400.
	if rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"x","completion_mode":"foo"}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("completion_mode='foo' deveria dar 400, veio %d %s", rec.Code, rec.Body.String())
	}
	// POST target_date malformado → 400.
	if rec = doJSON(router, http.MethodPost, objBase, cookie, `{"name":"x","completion_mode":"by_date","target_date":"2026-13-40"}`); rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
		t.Errorf("target_date malformado deveria dar 400, veio %d %s", rec.Code, rec.Body.String())
	}

	// PATCH target_date:null limpa (tri-state); ausente mantém.
	if rec = doJSON(router, http.MethodPatch, objBase+"/"+oD.ID, cookie, `{"target_date":null}`); rec.Code != http.StatusOK {
		t.Fatalf("PATCH target_date null: %d %s", rec.Code, rec.Body.String())
	}
	if p := decodeObj(t, rec.Body.Bytes()); p.TargetDate != nil {
		t.Errorf("target_date:null deveria limpar, veio %v", *p.TargetDate)
	}
	// PATCH só do nome NÃO mexe no completion_mode.
	if rec = doJSON(router, http.MethodPatch, objBase+"/"+oD.ID, cookie, `{"name":"renomeado"}`); rec.Code != http.StatusOK {
		t.Fatalf("PATCH nome: %d %s", rec.Code, rec.Body.String())
	}
	if p := decodeObj(t, rec.Body.Bytes()); p.CompletionMode != "by_date" {
		t.Errorf("ausente deveria manter modo by_date, veio %q", p.CompletionMode)
	}
	// PATCH completion_mode troca o modo.
	if rec = doJSON(router, http.MethodPatch, objBase+"/"+oD.ID, cookie, `{"completion_mode":"by_tasks"}`); rec.Code != http.StatusOK {
		t.Fatalf("PATCH modo: %d %s", rec.Code, rec.Body.String())
	}
	if p := decodeObj(t, rec.Body.Bytes()); p.CompletionMode != "by_tasks" {
		t.Errorf("modo deveria virar by_tasks, veio %q", p.CompletionMode)
	}

	// board 200 com completion_mode/target_date/progress_pct (CA-10).
	rec = doJSON(router, http.MethodGet, base+"/projects/"+pid+"/board", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	var board struct {
		Objectives []struct {
			CompletionMode string  `json:"completion_mode"`
			TargetDate     *string `json:"target_date"`
			ProgressPct    int     `json:"progress_pct"`
		} `json:"objectives"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatalf("board shape: %v", err)
	}
	if len(board.Objectives) < 1 {
		t.Fatalf("board deveria ter objetivos, veio %d", len(board.Objectives))
	}
	foundHours := false
	for _, b := range board.Objectives {
		if b.CompletionMode == "" {
			t.Errorf("board objetivo sem completion_mode")
		}
		if b.CompletionMode == "by_hours" {
			foundHours = true
		}
	}
	if !foundHours {
		t.Error("board deveria ter o objetivo by_hours")
	}
}

// TestObjectivesModosHandlersTargetDateTriState (T-13/#7/#8): a camada HTTP
// exercita os DOIS ramos não-cobertos do tri-state de target_date no PATCH —
// (#7) definir um VALOR (in.TargetDate=&v) e (#8) preservar o marco num PATCH
// só do nome (TargetDateSet só quando o campo vem no corpo).
func TestObjectivesModosHandlersTargetDateTriState(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "owner", "owner@b.c", "Owner")
	ws := workspaceIDOf(t, pool, "owner")
	base := "/api/v1/workspaces/" + ws

	pid := bodyID(t, doJSON(router, http.MethodPost, base+"/projects", cookie,
		`{"name":"P","color":"#4f9dde","visibility":"shared"}`).Body.Bytes())
	objBase := base + "/projects/" + pid + "/objectives"

	oD := decodeObj(t, doJSON(router, http.MethodPost, objBase, cookie,
		`{"name":"Marco","completion_mode":"by_date","target_date":"2026-07-15"}`).Body.Bytes())

	// #7: PATCH com target_date VALOR (não-null) DEFINE o marco via HTTP (ramo
	// in.TargetDate = &v). Mutação &v→nil limparia em vez de definir.
	rec := doJSON(router, http.MethodPatch, objBase+"/"+oD.ID, cookie, `{"target_date":"2026-08-01"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH target_date valor: %d %s", rec.Code, rec.Body.String())
	}
	if p := decodeObj(t, rec.Body.Bytes()); p.TargetDate == nil || *p.TargetDate != "2026-08-01" {
		t.Errorf("PATCH target_date:\"2026-08-01\" deveria DEFINIR o marco, veio %v", p.TargetDate)
	}

	// #8: PATCH só do nome PRESERVA o target_date existente (TargetDateSet só
	// quando o campo vem no corpo). Mutação que sempre seta TargetDateSet limparia.
	rec = doJSON(router, http.MethodPatch, objBase+"/"+oD.ID, cookie, `{"name":"renomeado"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH nome: %d %s", rec.Code, rec.Body.String())
	}
	if p := decodeObj(t, rec.Body.Bytes()); p.TargetDate == nil || *p.TargetDate != "2026-08-01" {
		t.Errorf("PATCH só do nome deveria PRESERVAR o marco (2026-08-01), veio %v", p.TargetDate)
	}
}
