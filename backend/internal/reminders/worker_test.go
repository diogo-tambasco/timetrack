// worker_test.go — TDD do ticker (fase 20 T-04), espelha o guard/worker_test:
// Run com intervalo curto despacha o reminder vencido (RunOnce roda ≥1×) e
// RETORNA ao cancelar o ctx (sem goroutine zumbi — CA-29); intervalo 0 → default
// sem explodir.
package reminders_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reminders"
)

func TestWorkerTicaEDespachaEParaNoShutdown(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-worker", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	id := seedReminder(t, pool, u, task, ws, now.Add(-time.Minute), "manual", "pending")

	var mu sync.Mutex
	var count int
	d := reminders.NewDispatcher(pool,
		reminders.WithClock(func() time.Time { return now }),
		reminders.WithOnNotice(func(string, any) {
			mu.Lock()
			count++
			mu.Unlock()
		}),
	)
	w := reminders.NewWorker(d, 5*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// espera o despacho acontecer (RunOnce rodou ≥1× num tick)
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		c := count
		mu.Unlock()
		if c >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("worker não despachou o reminder vencido dentro do prazo (RunOnce não rodou)")
		case <-time.After(5 * time.Millisecond):
		}
	}

	if remStatus(t, pool, id) != "sent" {
		t.Errorf("o reminder vencido deveria estar sent após os ticks do worker")
	}

	// cancelar o ctx faz Run RETORNAR (sem goroutine zumbi — CA-29)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run NÃO retornou após cancelar o ctx (goroutine zumbi)")
	}
}

func TestWorkerIntervaloDefault(t *testing.T) {
	pool := dbtest.NewPool(t)
	d := reminders.NewDispatcher(pool)
	w := reminders.NewWorker(d, 0, nil) // 0 → default 60s, não roda no prazo do teste

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
