// board_perf_test.go — fase 12 CA-18: o /board responde < 200ms com 30 obj /
// 300 tasks / 1.000 entries (melhor de 3). Verificação INFORMATIVA fora do
// red→green (igual CA-32 da fase 02): nasce verde; se falhar, a correção é
// índice/SQL — pina que não há N+1 (RNF-04).
package objectives

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
)

func TestBoardSob200ms(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	raw, err := os.ReadFile("../../testdata/objectives/board_seed.sql")
	if err != nil {
		t.Fatalf("ler board_seed.sql: %v", err)
	}
	seed := string(raw)
	seed = strings.ReplaceAll(seed, "{{WORKSPACE_ID}}", s.ws)
	seed = strings.ReplaceAll(seed, "{{USER_ID}}", s.pOwner)
	seed = strings.ReplaceAll(seed, "{{PROJECT_ID}}", s.shared)
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatalf("aplicar seed: %v", err)
	}

	// warm-up (planos/cache)
	if _, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared); err != nil {
		t.Fatalf("board warm-up: %v", err)
	}
	best := time.Hour
	for i := 0; i < 3; i++ {
		start := time.Now()
		b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
		if err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(b.Objectives) != 30 {
			t.Fatalf("esperava 30 objetivos no board, veio %d", len(b.Objectives))
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 200*time.Millisecond {
		t.Errorf("board (30 obj/300 tasks/1000 entries): melhor de 3 = %v (> 200ms — N+1? RNF-04)", best)
	}
	t.Logf("board melhor de 3: %v", best)
}
