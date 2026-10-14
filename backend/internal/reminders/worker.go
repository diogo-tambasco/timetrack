package reminders

import (
	"context"
	"log/slog"
	"time"
)

// defaultTick é o intervalo padrão da varredura (RNF-01): 60s. Como o guard,
// numa VPS com poucos usuários o custo é desprezível (1 query indexada por tick).
const defaultTick = 60 * time.Second

// Worker é o ticker de background: a cada intervalo roda uma passada do
// Dispatcher. 1 goroutine, encerra no shutdown via context (CA-29). Espelha o
// guard.Worker. Construir com NewWorker.
type Worker struct {
	dispatcher *Dispatcher
	interval   time.Duration
	log        *slog.Logger
}

// NewWorker cria o worker. interval <= 0 → 60s (RNF-01). log nil → descartado.
func NewWorker(dispatcher *Dispatcher, interval time.Duration, log *slog.Logger) *Worker {
	if interval <= 0 {
		interval = defaultTick
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Worker{dispatcher: dispatcher, interval: interval, log: log}
}

// Run roda o ticker até o ctx ser cancelado (shutdown — CA-29). NÃO roda uma
// passada imediata no boot (espera o 1º tick — espelha o guard): evita competir
// com a migração recém-aplicada e o pico de boot. Cada passada loga os despachos.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("reminder worker iniciado", "intervalo", w.interval.String())
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("reminder worker encerrado")
			return
		case <-ticker.C:
			done, err := w.dispatcher.RunOnce(ctx)
			if err != nil {
				// erro de varredura é logado e a próxima passada tenta de novo
				// (idempotente — CA-08); nunca derruba o worker.
				w.log.Error("reminder: varredura falhou", "err", err)
				continue
			}
			for _, d := range done {
				w.log.Info("lembrete despachado", "user", d.UserID, "task", d.TaskID)
			}
		}
	}
}
