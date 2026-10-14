// timer_task_link_test.go — Fase 30 (elo timer↔task), T-01/T-02: testes de
// NÃO-REGRESSÃO do contrato existente. O backend do timer JÁ suporta o elo
// (StartInput aceita project_id+task_id; ValidateRefs checa cada ref contra o
// workspace SEM cruzar task↔project — D-01). NENHUM código de produção novo:
// estes testes só pinam o contrato que o frontend da fase 30 consome (RNF-01/
// RNF-06). Nascem VERDES contra o código atual; uma mutação que remova o TaskID
// do Insert (T-01) ou troque domain.NotFound por domain.Validation no refs.go
// (T-02) quebraria.
package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// T-01 / CA-01/03 — start com project_id E task_id grava AMBOS na entry rodando.
// O front da fase 30 sempre manda {project_id: task.project_id, task_id: task.id}
// da MESMA task (CA-20); este teste pina que o vínculo é persistido coerente —
// matar o `TaskID:` do entries.Insert (service.go) faz este teste falhar.
func TestStartComTaskGravaProjectETaskID(t *testing.T) {
	pool, router, _, _, cookie, userID, wsID := startStopSetup(t)

	p := criarProjeto(t, router, wsID, cookie,
		`{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, wsID, cookie,
		`{"project_id":"`+p.ID+`","name":"Definir telas"}`)

	payload := fmt.Sprintf(`{"workspace_id":%q,"project_id":%q,"task_id":%q,"description":%q}`,
		wsID, p.ID, tk.ID, tk.Name)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start por task esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" || body.State.Entry == nil {
		t.Fatalf("estado pós-start deveria ser countup com entry, veio %s", rec.Body.String())
	}

	// CA-01/03: a entry rodando grava project_id E task_id (ambos do payload),
	// coerentes com a task de origem (project_id = task.project_id).
	var gotProj, gotTask *string
	if err := pool.QueryRow(context.Background(),
		`SELECT project_id, task_id FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&gotProj, &gotTask); err != nil {
		t.Fatalf("ler entry rodando: %v", err)
	}
	if gotProj == nil || *gotProj != p.ID {
		t.Errorf("entry deveria gravar project_id=%s, veio %v", p.ID, gotProj)
	}
	if gotTask == nil || *gotTask != tk.ID {
		t.Errorf("entry deveria gravar task_id=%s, veio %v", tk.ID, gotTask)
	}
}

// T-01 / CA-19 — start por task é um countup NORMAL: não mexe no ciclo Pomodoro.
// O novo start reseta o ciclo do zero (focus_count=0, break_is_long=false),
// idêntico a um start sem task (RNF-01 — o elo só adiciona os refs).
func TestStartComTaskNaoMudaCiclo(t *testing.T) {
	pool, router, _, _, cookie, userID, wsID := startStopSetup(t)

	p := criarProjeto(t, router, wsID, cookie,
		`{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, wsID, cookie,
		`{"project_id":"`+p.ID+`","name":"Definir telas"}`)

	payload := fmt.Sprintf(`{"workspace_id":%q,"project_id":%q,"task_id":%q}`, wsID, p.ID, tk.ID)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start por task esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeTimer(t, rec)
	if body.State == nil || body.State.Mode != "countup" {
		t.Fatalf("start por task deveria ser countup, veio %s", rec.Body.String())
	}

	var focusCount int
	var breakIsLong bool
	if err := pool.QueryRow(context.Background(),
		`SELECT focus_count, break_is_long FROM timer_states WHERE user_id = $1`,
		userID).Scan(&focusCount, &breakIsLong); err != nil {
		t.Fatalf("ler timer_states: %v", err)
	}
	if focusCount != 0 || breakIsLong {
		t.Errorf("start por task não deveria mexer no ciclo: focus_count=%d break_is_long=%v",
			focusCount, breakIsLong)
	}
}

// T-02 / CA-02 — task de OUTRO workspace → 404 not_found (HERDADO da fase 03 via
// ValidateRefs → domain.NotFound), nenhuma entry criada. O elo NÃO introduz
// vazamento nem troca o código de erro pra invalid_reference (RNF-01).
func TestStartTaskDeOutroWorkspace404(t *testing.T) {
	pool, router, _, env, anaCookie, anaID, wsAna := startStopSetup(t)

	// projeto + task vivem no workspace da Bia (outro tenant)
	biaCookie, _, biaWs := loginBia(t, pool, env)
	p := criarProjeto(t, router, biaWs, biaCookie,
		`{"name":"Alheio","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, biaWs, biaCookie,
		`{"project_id":"`+p.ID+`","name":"Tarefa alheia"}`)

	payload := fmt.Sprintf(`{"workspace_id":%q,"task_id":%q}`, wsAna, tk.ID)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", anaCookie, payload)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("task de outro workspace esperava 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, anaID) != 0 {
		t.Error("nenhuma entry deveria ser criada com task de outro workspace")
	}
}

// T-02 / CA-02 — task com uuid MALFORMADO → 404 not_found (via uuid.Validate no
// ValidateRefs), nunca 500. Pina que o caminho do elo não estoura.
func TestStartTaskMalformada404(t *testing.T) {
	pool, router, _, _, cookie, userID, wsID := startStopSetup(t)

	payload := fmt.Sprintf(`{"workspace_id":%q,"task_id":"nao-uuid"}`, wsID)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload)
	if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
		t.Errorf("task malformada esperava 404 not_found, veio %d %s", rec.Code, rec.Body.String())
	}
	if countRunning(t, pool, userID) != 0 {
		t.Error("nenhuma entry deveria ser criada com task malformada")
	}
}
