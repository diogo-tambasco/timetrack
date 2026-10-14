// board_modes_test.go — fase 29 T-03: completion_mode/target_date/progress_pct
// no /board (aditivo). progressByMode é função PURA testada por tabela. Os
// campos da fase 12 ficam byte-idênticos (regressão).
package objectives

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// TestProgressByModePuro: tabela de casos da função pura (CA-10.1).
func TestProgressByModePuro(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		donePct int
		tracked int64
		goal    *float64
		want    int
	}{
		{"by_tasks usa done_pct", "by_tasks", 33, 90000, ptr(40.0), 33},
		{"by_date usa done_pct", "by_date", 50, 90000, ptr(40.0), 50},
		{"by_hours 1.8h/4h = 45", "by_hours", 0, 6480, ptr(4.0), 45}, // 1.8h = 6480s; 6480/(4*3600)=45%
		{"by_hours sem meta = 0", "by_hours", 80, 6480, nil, 0},
		{"by_hours tracked>meta clampa 100", "by_hours", 0, 100000, ptr(4.0), 100},
		{"by_hours zero tracked = 0", "by_hours", 0, 0, ptr(4.0), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := progressByMode(c.mode, c.donePct, c.tracked, c.goal); got != c.want {
				t.Errorf("progressByMode(%q, %d, %d, %v) = %d, want %d", c.mode, c.donePct, c.tracked, c.goal, got, c.want)
			}
		})
	}
}

// TestBoardCamposModo: o board traz completion_mode/target_date/progress_pct
// corretos por objetivo (CA-10/10.1). Os campos da fase 12 inalterados.
func TestBoardCamposModo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	p := s.shared

	// by_hours: meta 4h, tracked 1.8h → progress_pct 45.
	oH, _ := svc.Create(ctx, s.ws, s.pOwner, "member", p, CreateInput{
		Name: "Horas", CompletionMode: "by_hours", GoalHours: ptr(4.0)})
	tH := mkTask(t, pool, s, &oH.ID, "todo")
	mkEntry(t, pool, s.ws, s.pOwner, &p, &tH, 6480) // 1.8h

	// by_date com target_date → progress_pct usa done_pct das tasks.
	oD, _ := svc.Create(ctx, s.ws, s.pOwner, "member", p, CreateInput{
		Name: "Marco", CompletionMode: "by_date", TargetDate: ptr("2026-07-15")})
	mkTask(t, pool, s, &oD.ID, "done")
	mkTask(t, pool, s, &oD.ID, "todo") // done_pct = 50

	// by_tasks (default): progress_pct == done_pct.
	oT, _ := svc.Create(ctx, s.ws, s.pOwner, "member", p, CreateInput{Name: "Tasks"})
	mkTask(t, pool, s, &oT.ID, "done")
	mkTask(t, pool, s, &oT.ID, "done")
	mkTask(t, pool, s, &oT.ID, "todo") // done_pct = 67

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", p)
	if err != nil {
		t.Fatalf("board: %v", err)
	}

	bH := findObj(b, oH.ID)
	if bH == nil || bH.CompletionMode != "by_hours" {
		t.Fatalf("objetivo by_hours não encontrado/incorreto: %+v", bH)
	}
	if bH.TargetDate != nil {
		t.Errorf("by_hours target_date deveria ser nil, veio %v", *bH.TargetDate)
	}
	if bH.ProgressPct != 45 {
		t.Errorf("by_hours progress_pct esperado 45, veio %d", bH.ProgressPct)
	}

	bD := findObj(b, oD.ID)
	if bD == nil || bD.CompletionMode != "by_date" {
		t.Fatalf("objetivo by_date não encontrado/incorreto: %+v", bD)
	}
	if bD.TargetDate == nil || *bD.TargetDate != "2026-07-15" {
		t.Errorf("by_date target_date esperado 2026-07-15, veio %v", bD.TargetDate)
	}
	if bD.ProgressPct != bD.DonePct || bD.ProgressPct != 50 {
		t.Errorf("by_date progress_pct deveria == done_pct == 50, veio progress=%d done=%d", bD.ProgressPct, bD.DonePct)
	}

	bT := findObj(b, oT.ID)
	if bT == nil || bT.CompletionMode != "by_tasks" {
		t.Fatalf("objetivo by_tasks não encontrado/incorreto: %+v", bT)
	}
	if bT.ProgressPct != bT.DonePct {
		t.Errorf("by_tasks progress_pct deveria == done_pct, veio progress=%d done=%d", bT.ProgressPct, bT.DonePct)
	}
}

// TestBoardByHoursSemMeta: by_hours sem goal_hours → progress_pct 0 (sem div/0).
func TestBoardByHoursSemMeta(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()
	p := s.shared

	o, _ := svc.Create(ctx, s.ws, s.pOwner, "member", p, CreateInput{Name: "Horas sem meta", CompletionMode: "by_hours"})
	tk := mkTask(t, pool, s, &o.ID, "todo")
	mkEntry(t, pool, s.ws, s.pOwner, &p, &tk, 7200)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", p)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	bo := findObj(b, o.ID)
	if bo == nil || bo.ProgressPct != 0 {
		t.Errorf("by_hours sem meta deveria dar progress_pct 0, veio %+v", bo)
	}
}
