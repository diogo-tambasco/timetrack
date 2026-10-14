// worker_test.go — TDD do ticker (fase 16 T-03): Run com intervalo curto
// roda RunOnce ≥1× e RETORNA ao cancelar o ctx (sem goroutine zumbi — CA-18).
package guard_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/guard"
)

// TestWorkerTicaECortaEParaNoShutdown: o worker com intervalo de ms corta a
// entry vencida (RunOnce roda no tick) e Run RETORNA quando o ctx é cancelado.
func TestWorkerTicaECortaEParaNoShutdown(t *testing.T) {
	pool := dbtest.NewPool(t)
	now := baseTime(t)
	u, ws := seedUser(t, pool, "g-worker")
	e := seedRunning(t, pool, ws, u, now.Add(-3*time.Hour))

	var mu sync.Mutex
	var cutCount int
	cutter := guard.NewCutter(pool,
		guard.WithClock(func() time.Time { return now }),
		guard.WithOnCut(func(guard.CutEvent) {
			mu.Lock()
			cutCount++
			mu.Unlock()
		}),
	)
	w := guard.NewWorker(cutter, 5*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// espera o corte acontecer (RunOnce rodou ≥1× num tick)
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		c := cutCount
		mu.Unlock()
		if c >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("worker não cortou a entry vencida dentro do prazo (RunOnce não rodou)")
		case <-time.After(5 * time.Millisecond):
		}
	}

	// a entry venceu e foi cortada no banco
	if loadEntry(t, pool, e).stoppedAt == nil {
		t.Errorf("a entry vencida deveria estar cortada após os ticks do worker")
	}

	// cancelar o ctx faz Run RETORNAR (sem goroutine zumbi — CA-18)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run NÃO retornou após cancelar o ctx (goroutine zumbi)")
	}
}

// TestWorkerIntervaloDefault: intervalo <= 0 cai no default (não explode).
func TestWorkerIntervaloDefault(t *testing.T) {
	pool := dbtest.NewPool(t)
	cutter := guard.NewCutter(pool)
	w := guard.NewWorker(cutter, 0, nil) // 0 → default 60s, não roda no prazo do teste

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run com intervalo default não retornou ao cancelar o ctx")
	}
}
