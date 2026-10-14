// summary_perf_test.go — fase 28 CA-07 (perf): o /projects/summary responde
// < 200ms com 30 projetos / 300 tasks / 1.000 entries (melhor de 3).
// Verificação INFORMATIVA fora do red→green (igual CA-18 da fase 12): nasce
// verde; se falhar, a correção é índice/SQL — pina que não há N+1 (RNF-04).
package projects_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/projects"
)

func TestSummarySob200ms(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	raw, err := os.ReadFile("../../testdata/projects/summary_seed.sql")
	if err != nil {
		t.Fatalf("ler summary_seed.sql: %v", err)
	}
	seed := strings.ReplaceAll(string(raw), "{{WORKSPACE_ID}}", ws)
	seed = strings.ReplaceAll(seed, "{{USER_ID}}", owner)
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatalf("aplicar seed: %v", err)
	}

	// warm-up (planos/cache)
	if _, err := svc.Summary(ctx, ws, owner, "America/Sao_Paulo", false); err != nil {
		t.Fatalf("summary warm-up: %v", err)
	}
	best := time.Hour
	for i := 0; i < 3; i++ {
		start := time.Now()
		list, err := svc.Summary(ctx, ws, owner, "America/Sao_Paulo", false)
		if err != nil {
			t.Fatalf("summary: %v", err)
		}
		if len(list) != 30 {
			t.Fatalf("esperava 30 projetos no summary, veio %d", len(list))
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 200*time.Millisecond {
		t.Errorf("summary (30 proj/300 tasks/1000 entries): melhor de 3 = %v (> 200ms — N+1? RNF-04)", best)
	}
	t.Logf("summary melhor de 3: %v", best)
}
