package objectives

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// Fase 28 T-02 (CA-01, RNF-05) — o /board (fase 12) traz project.deadline como
// campo aditivo: null sem prazo, "YYYY-MM-DD" quando definido. Resto do board
// intocado (suíte da fase 12 continua verde).
func TestBoardTrazDeadline(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// projeto shared sem deadline → null no board.
	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if b.Project.Deadline != nil {
		t.Errorf("board sem deadline deveria ser null, veio %v", *b.Project.Deadline)
	}

	// define o deadline direto no banco e relê.
	if _, err := pool.Exec(ctx,
		`UPDATE projects SET deadline = '2025-09-30' WHERE id = $1`, s.shared); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	b, err = svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board 2: %v", err)
	}
	if b.Project.Deadline == nil || *b.Project.Deadline != "2025-09-30" {
		t.Errorf("board deveria trazer deadline 2025-09-30, veio %v", b.Project.Deadline)
	}
}
