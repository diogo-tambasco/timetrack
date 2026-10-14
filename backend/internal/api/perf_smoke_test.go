package api_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

// CA-32 — smoke informativo de regressão de performance (T-12): com 500
// registros (50 clientes / 150 projetos / 300 tasks), cada listagem da
// fase responde em < 200 ms no Postgres efêmero local, valendo a MELHOR
// de 3 execuções.
//
// Este é o único teste da fase FORA do ciclo red→green (exceção documentada
// na spec §3/CA-32 e no tasks.md): nasce verde e depende da máquina — não
// se aplica o anti-cheat do TDD.md. Se falhar, a correção esperada é
// índice na migração.
func TestListagensSob200ms(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCoreRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-perf", "perf@example.com", "Perf Owner")
	ws := workspaceIDOf(t, pool, "g-perf")
	user := userIDOf(t, pool, "g-perf")

	seedCore500(t, pool, ws, user)

	// sanity da proporção fixa do CA-32 — o seed é parte do contrato
	if n := dbtest.CountRows(t, pool, "clients"); n != 50 {
		t.Fatalf("seed deveria criar 50 clientes, criou %d", n)
	}
	if n := dbtest.CountRows(t, pool, "projects"); n != 150 {
		t.Fatalf("seed deveria criar 150 projetos, criou %d", n)
	}
	if n := dbtest.CountRows(t, pool, "tasks"); n != 300 {
		t.Fatalf("seed deveria criar 300 tasks, criou %d", n)
	}

	for _, rota := range []string{"/clients", "/projects", "/tasks"} {
		melhor := time.Duration(1<<63 - 1)
		for tentativa := range 3 {
			inicio := time.Now()
			rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+rota, cookie, "")
			decorrido := time.Since(inicio)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s (tentativa %d): esperado 200, veio %d: %s",
					rota, tentativa+1, rec.Code, rec.Body.String())
			}
			if decorrido < melhor {
				melhor = decorrido
			}
		}
		t.Logf("GET %s: melhor de 3 = %s", rota, melhor)
		if melhor > 200*time.Millisecond {
			t.Errorf("GET %s acima do limite do CA-32: %s > 200ms (correção esperada: índice na migração)",
				rota, melhor)
		}
	}
}

// seedCore500 carrega o seed reutilizável de backend/testdata/core/ e o
// aplica ao workspace/usuário dados (placeholders por substituição — o
// arquivo roda num único Exec, protocolo simples).
func seedCore500(t *testing.T, pool *pgxpool.Pool, wsID, userID string) {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/core/seed500.sql")
	if err != nil {
		t.Fatalf("ler seed500.sql: %v", err)
	}
	sql := strings.ReplaceAll(string(raw), "{{WORKSPACE_ID}}", wsID)
	sql = strings.ReplaceAll(sql, "{{USER_ID}}", userID)
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatalf("aplicar seed500.sql: %v", err)
	}
}
