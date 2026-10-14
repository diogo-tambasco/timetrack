// swap_test.go — T-07: start com timer rodando troca atomicamente (CA-05).
package timer_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// CA-05 — start com outro timer rodando PARA o atual (gravando duração do
// relógio do servidor) e inicia o novo na MESMA operação: resta exatamente
// 1 entry rodando.
func TestStartComTimerRodandoTrocaAtomico(t *testing.T) {
	svc, clock, pool, userID, wsID, projectID := newTimerService(t)
	ctx := context.Background()

	primeiro, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "primeiro"})
	if err != nil {
		t.Fatalf("primeiro start: %v", err)
	}
	clock.Advance(600 * time.Second)

	segundo, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Description: "segundo", ProjectID: &projectID,
	})
	if err != nil {
		t.Fatalf("segundo start deveria trocar o timer, não falhar: %v", err)
	}
	if segundo.State == nil || segundo.State.Entry.Description != "segundo" {
		t.Fatalf("estado deveria apontar o novo timer, veio %+v", segundo.State)
	}

	// exatamente 1 entry rodando — e é a nova
	var running int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&running); err != nil || running != 1 {
		t.Fatalf("deveria restar exatamente 1 entry rodando, veio %d (err=%v)", running, err)
	}
	var runningID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&runningID); err != nil || runningID != segundo.State.Entry.ID {
		t.Errorf("a entry rodando deveria ser a do segundo start (err=%v)", err)
	}

	// a primeira foi fechada com a duração certa (600s)
	var dur int
	if err := pool.QueryRow(ctx,
		`SELECT duration_seconds FROM time_entries WHERE id = $1`,
		primeiro.State.Entry.ID).Scan(&dur); err != nil || dur != 600 {
		t.Errorf("primeira entry deveria fechar com 600s, veio %d (err=%v)", dur, err)
	}
}
