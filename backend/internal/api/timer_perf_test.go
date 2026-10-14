// timer_perf_test.go — RNF-04 (T-20): check leve de latência do
// GET /api/v1/timer no ambiente local (< 100ms, sem benchmark formal).
// TODAS as 3 amostras precisam caber no limite — com n=3, o p95
// nearest-rank que a letra do RNF-04 pede É o máximo do sample, então
// assertar o pior caso honra a spec ao mesmo custo (verificação
// adversarial do bloco 3: mínimo-de-3 deixava cauda passar).
//
// Assim como TestListagensSob200ms (fase 02), este smoke fica FORA do
// ciclo red→green (exceção documentada no tasks.md T-20): nasce verde e
// depende da máquina. Se falhar, investigar antes de fechar a fase.
package api_test

import (
	"net/http"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

func TestGetTimerSob100ms(t *testing.T) {
	_, router, clock, tsvc, cookie, userID, wsID := pauseSetup(t)

	// estado RODANDO: o GET faz o trabalho real (timer_states + entry)
	mustStart(t, tsvc, userID, timer.StartInput{WorkspaceID: wsID, Description: "perf RNF-04"})
	clock.Advance(90 * time.Second)

	pior := time.Duration(0)
	for tentativa := range 3 {
		inicio := time.Now()
		rec := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
		decorrido := time.Since(inicio)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /timer (tentativa %d): esperado 200, veio %d: %s",
				tentativa+1, rec.Code, rec.Body.String())
		}
		if decorrido > pior {
			pior = decorrido
		}
		if decorrido > 100*time.Millisecond {
			t.Errorf("GET /timer (tentativa %d) acima do limite do RNF-04: %s > 100ms — investigar antes de fechar a fase",
				tentativa+1, decorrido)
		}
	}
	t.Logf("GET /timer: pior de 3 = %s", pior)
}
