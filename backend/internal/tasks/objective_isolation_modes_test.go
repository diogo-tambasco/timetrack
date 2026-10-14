// objective_isolation_modes_test.go — fase 29 T-06: isolamento multi-tenant do
// auto-close by_hours (CA-09.2). O `secs` do recálculo só soma entries de tasks
// DESTE objetivo (mutação que remove `WHERE e.task_id IN (tasks do objetivo)`
// quebra) E só do user da mutação (mutação que remove `WHERE e.user_id` quebra —
// per-usuário, decisão E / 29-H1).
package tasks

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// TestRecalcByHoursSoSomaTasksDoObjetivo: horas numa task de OUTRO objetivo (do
// mesmo user) NÃO contam pro auto-close do objetivo by_hours. Mutação que remove
// o `WHERE e.task_id IN (tasks do objetivo)` somaria as horas alheias e fecharia
// o objetivo indevidamente → este teste quebra.
func TestRecalcByHoursSoSomaTasksDoObjetivo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	// objetivo by_hours alvo (meta 2h), sem horas próprias.
	alvo := mkObjective(t, pool, s, s.project, "Alvo")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2 WHERE id=$1`, alvo); err != nil {
		t.Fatalf("set mode alvo: %v", err)
	}

	// OUTRO objetivo (no mesmo projeto) com uma task gorda de horas (do mesmo user).
	outro := mkObjective(t, pool, s, s.project, "Outro")
	tOutro, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T-outro", ObjectiveID: &outro})
	mkClosedEntry(t, pool, s.ws, s.user, s.project, tOutro.ID, 99999) // muito > 2h, mas de OUTRO objetivo

	// mutação de task NO ALVO (criar task sem horas) → recalcula o alvo → NÃO fecha.
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T-alvo", ObjectiveID: &alvo})
	if st := objStatus(t, pool, alvo); st != "open" {
		t.Errorf("horas de OUTRO objetivo NÃO deveriam fechar o objetivo by_hours alvo, veio %q", st)
	}
}
