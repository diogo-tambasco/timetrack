// service_test.go — fase 12 T-03: CRUD de objetivos + validação (CA-01/01.1/
// 02/03/04/05/07/08/13/13.1/15).
package objectives

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

func ptr[T any](v T) *T { return &v }

func TestCreateListOrdemPorPosition(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	for _, name := range []string{"A", "B", "C"} {
		if _, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: name}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	list, err := svc.List(ctx, s.ws, s.pOwner, s.shared)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("esperava 3 objetivos, veio %d", len(list))
	}
	for i, want := range []struct {
		name string
		pos  int
	}{{"A", 0}, {"B", 1}, {"C", 2}} {
		if list[i].Name != want.name || list[i].Position != want.pos {
			t.Errorf("objetivo[%d] = (%s,%d), esperava (%s,%d)", i, list[i].Name, list[i].Position, want.name, want.pos)
		}
		if list[i].Status != "open" {
			t.Errorf("objetivo nasce open, veio %q", list[i].Status)
		}
	}
}

func TestCreateValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	bad := []struct {
		name string
		in   CreateInput
	}{
		{"nome vazio", CreateInput{Name: "   "}},
		{"nome >200", CreateInput{Name: string(make([]byte, 201))}},
		{"goal 0", CreateInput{Name: "x", GoalHours: ptr(0.0)}},
		{"goal negativo", CreateInput{Name: "x", GoalHours: ptr(-1.0)}},
		{"goal >9999", CreateInput{Name: "x", GoalHours: ptr(10000.0)}},
		// sub-0.005 positivo: > 0 passa no guard ingênuo, mas arredonda pra 0.00
		// no numeric(6,2) e viola o CHECK → 400, NUNCA 500 (CA-02).
		{"goal arredonda pra 0", CreateInput{Name: "x", GoalHours: ptr(0.004)}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, c.in); !isValidation(err) {
				t.Errorf("esperava validation_error, veio %v", err)
			}
		})
	}
	// nenhum dos inválidos persistiu
	if n := dbtest.CountRows(t, pool, "objectives"); n != 0 {
		t.Errorf("validações não deviam persistir; %d objetivos no banco", n)
	}
	// goal_hours no teto 9999 cabe (numeric(6,2)) e persiste
	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "teto", GoalHours: ptr(9999.0)})
	if err != nil {
		t.Fatalf("goal_hours=9999 deveria criar: %v", err)
	}
	if o.GoalHours == nil || *o.GoalHours != 9999 {
		t.Errorf("goal_hours esperado 9999, veio %v", o.GoalHours)
	}
}

// TestPatchGoalArredondaParaZero: o PATCH também precisa barrar o goal sub-0.005
// antes do SQL (CA-02 — 400, nunca 500 por violar o CHECK do numeric(6,2)).
func TestPatchGoalArredondaParaZero(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "obj"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID,
		PatchInput{GoalSet: true, GoalHours: ptr(0.004)}); !isValidation(err) {
		t.Errorf("goal sub-0.005 no PATCH deveria dar validation_error, veio %v", err)
	}
}

func TestPatchTriStateGoalHours(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "obj", GoalHours: ptr(12.0)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// ausente mantém: patch só do nome NÃO mexe em goal_hours
	upd, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Name: ptr("novo nome")})
	if err != nil {
		t.Fatalf("patch nome: %v", err)
	}
	if upd.Name != "novo nome" || upd.GoalHours == nil || *upd.GoalHours != 12 {
		t.Errorf("ausente deveria manter goal=12; veio nome=%q goal=%v", upd.Name, upd.GoalHours)
	}
	// null remove
	upd, err = svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{GoalSet: true, GoalHours: nil})
	if err != nil {
		t.Fatalf("patch goal null: %v", err)
	}
	if upd.GoalHours != nil {
		t.Errorf("null deveria remover a meta, veio %v", *upd.GoalHours)
	}
}

func TestPatchStatusManual(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "obj"})
	// fecho manual mesmo sem 100% das tasks (CA-05)
	upd, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Status: ptr("done")})
	if err != nil || upd.Status != "done" {
		t.Errorf("fecho manual deveria dar done, veio %q (err=%v)", upd.Status, err)
	}
	// reabre
	upd, err = svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Status: ptr("open")})
	if err != nil || upd.Status != "open" {
		t.Errorf("reabertura manual deveria dar open, veio %q (err=%v)", upd.Status, err)
	}
	// status inválido → 400
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Status: ptr("concluido")}); !isValidation(err) {
		t.Errorf("status fora do enum deveria dar validation_error, veio %v", err)
	}
}

func TestDeleteHardTasksViramGeral(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "obj"})
	var taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, objective_id)
		 VALUES ($1, $2, 'T', $3, $4) RETURNING id`, s.ws, s.shared, s.pOwner, o.ID).Scan(&taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.pOwner, "member", s.shared, o.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := dbtest.CountRows(t, pool, "objectives"); n != 0 {
		t.Errorf("delete hard deveria zerar objectives, veio %d", n)
	}
	var orphanNull bool
	if err := pool.QueryRow(ctx, `SELECT objective_id IS NULL FROM tasks WHERE id = $1`, taskID).Scan(&orphanNull); err != nil {
		t.Fatalf("task deveria sobreviver: %v", err)
	}
	if !orphanNull {
		t.Error("task deveria virar Geral (objective_id NULL) após delete do objetivo")
	}
}

func TestEscritaGatePorViewEViewer(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// membro com access=view → 403
	if _, err := svc.Create(ctx, s.ws, s.mView, "member", s.shared, CreateInput{Name: "x"}); !isForbidden(err) {
		t.Errorf("access=view deveria dar 403, veio %v", err)
	}
	// projeto private de outro → 404 (não vaza)
	if _, err := svc.Create(ctx, s.ws, s.mNone, "member", s.private, CreateInput{Name: "x"}); !isNotFound(err) {
		t.Errorf("private de outro deveria dar 404, veio %v", err)
	}
}

func TestObjetivoCrossProjectDaRota(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// objetivo no projeto PRIVATE (pOwner dono dos dois projetos)
	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.private, CreateInput{Name: "do B"})
	if err != nil {
		t.Fatalf("create no private: %v", err)
	}
	// alcançado pela rota do projeto SHARED → 404 (CA-13.1, não vaza/edita vizinho)
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Name: ptr("hack")}); !isNotFound(err) {
		t.Errorf("objetivo de outro projeto via rota errada deveria dar 404, veio %v", err)
	}
	if err := svc.Delete(ctx, s.ws, s.pOwner, "member", s.shared, o.ID); !isNotFound(err) {
		t.Errorf("delete cross-project deveria dar 404, veio %v", err)
	}
	// objectiveId malformado no path → 404 (não 500)
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, "nao-e-uuid", PatchInput{Name: ptr("x")}); !isNotFound(err) {
		t.Errorf("objectiveId malformado deveria dar 404, veio %v", err)
	}
}

func isValidation(err error) bool { return codeIs(err, "validation_error") }
