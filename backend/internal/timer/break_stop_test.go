// break_stop_test.go — T-10: stop DURANTE a pausa (CA-06.1).
package timer_test

import (
	"context"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// CA-06.1 — stop no meio da pausa encerra a pausa e deixa o estado OCIOSO:
// sem retomar, sem criar entry nova (a pausada já foi fechada no pause).
func TestStopDurantePausaDeixaOcioso(t *testing.T) {
	svc, clock, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(600 * time.Second)
	if _, err := svc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause: %v", err)
	}
	clock.Advance(60 * time.Second) // pausa em andamento

	res, err := svc.Stop(ctx, userID)
	if err != nil {
		t.Fatalf("stop durante a pausa: %v", err)
	}
	if res.State != nil {
		t.Errorf("estado deveria ficar ocioso (nil), veio %+v", res.State)
	}
	if res.StoppedEntry != nil {
		t.Errorf("stop na pausa não fecha entry nenhuma (a pausada já fechou no pause), veio %+v", res.StoppedEntry)
	}

	// nada foi retomado nem criado: segue 1 única entry (a fechada no pause)
	if n := dbtest.CountRows(t, pool, "time_entries"); n != 1 {
		t.Errorf("deveria existir só a entry fechada no pause, vieram %d", n)
	}
	var running int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&running); err != nil || running != 0 {
		t.Errorf("nenhuma entry deveria estar rodando, vieram %d (err=%v)", running, err)
	}
	if n := dbtest.CountRows(t, pool, "timer_states"); n != 0 {
		t.Errorf("a linha de runtime deveria sumir, sobraram %d", n)
	}

	// e o GET segue reportando ocioso (CA-23)
	st, err := svc.State(ctx, userID)
	if err != nil || st.State != nil {
		t.Errorf("estado consultado deveria ser nulo (err=%v state=%+v)", err, st.State)
	}
}
