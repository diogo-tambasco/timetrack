package guard

import (
	"context"
	"log/slog"
	"time"
)

// defaultTick é o intervalo padrão da varredura (RNF-01): 60s. Numa VPS com
// poucos usuários o custo é desprezível (1 query indexada por tick).
const defaultTick = 60 * time.Second

// Worker é o ticker de background: a cada intervalo roda uma passada do
// Cutter. 1 goroutine, encerra no shutdown via context (CA-18). Construir com
// NewWorker.
type Worker struct {
	cutter   *Cutter
	interval time.Duration
	log      *slog.Logger
}

// NewWorker cria o worker. interval <= 0 → 60s (RNF-01). log nil → descartado.
func NewWorker(cutter *Cutter, interval time.Duration, log *slog.Logger) *Worker {
	if interval <= 0 {
		interval = defaultTick
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Worker{cutter: cutter, interval: interval, log: log}
}

// Run roda o ticker até o ctx ser cancelado (shutdown — CA-18). NÃO roda uma
// passada imediata no boot (espera o 1º tick — D-02): evita competir com a
// migração recém-aplicada e o pico de boot. Cada passada loga os cortes.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("guard worker iniciado", "intervalo", w.interval.String())
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("guard worker encerrado")
			return
		case <-ticker.C:
			cuts, err := w.cutter.RunOnce(ctx)
			if err != nil {
				// erro de varredura é logado e a próxima passada tenta de novo
				// (idempotente — CA-06); nunca derruba o worker.
				w.log.Error("guard: varredura falhou", "err", err)
				continue
			}
			for _, c := range cuts {
				w.log.Info("timer auto-cortado", "user", c.UserID, "entry", c.EntryID, "stopped_at", c.StoppedAt)
			}
		}
	}
}
