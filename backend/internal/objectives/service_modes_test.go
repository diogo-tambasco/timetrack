// service_modes_test.go — fase 29 T-02: completion_mode + target_date no
// objectives.Service (validação + tri-state). Estende a fase 12: os testes da
// fase 12 (service_test.go) seguem verdes sem mudar asserção (regressão).
package objectives

import (
	"context"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
)

// TestCreateComModo: create com by_hours persiste o modo; sem completion_mode
// → default by_tasks (CA-02/D-07).
func TestCreateComModo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// by_hours + goal_hours persiste o modo
	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{
		Name: "Bater 40h", CompletionMode: "by_hours", GoalHours: ptr(40.0)})
	if err != nil {
		t.Fatalf("create by_hours: %v", err)
	}
	if o.CompletionMode != "by_hours" {
		t.Errorf("completion_mode esperado by_hours, veio %q", o.CompletionMode)
	}
	if o.TargetDate != nil {
		t.Errorf("target_date deveria ser nil, veio %v", *o.TargetDate)
	}

	// sem completion_mode → by_tasks (default, preserva cliente fase 12)
	o2, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "Legado"})
	if err != nil {
		t.Fatalf("create sem modo: %v", err)
	}
	if o2.CompletionMode != "by_tasks" {
		t.Errorf("completion_mode default esperado by_tasks, veio %q", o2.CompletionMode)
	}

	// by_date + target_date
	o3, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{
		Name: "Demo", CompletionMode: "by_date", TargetDate: ptr("2026-07-15")})
	if err != nil {
		t.Fatalf("create by_date: %v", err)
	}
	if o3.CompletionMode != "by_date" || o3.TargetDate == nil || *o3.TargetDate != "2026-07-15" {
		t.Errorf("by_date esperado target_date=2026-07-15, veio mode=%q td=%v", o3.CompletionMode, o3.TargetDate)
	}
}

// TestCreateModoInvalido: completion_mode/target_date inválido → 400 (não 500,
// não persiste — CA-02.1/03).
func TestCreateModoInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	if _, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{
		Name: "x", CompletionMode: "foo"}); !isValidation(err) {
		t.Errorf("completion_mode='foo' deveria dar validation_error, veio %v", err)
	}
	if _, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{
		Name: "x", CompletionMode: "by_date", TargetDate: ptr("2026-13-40")}); !isValidation(err) {
		t.Errorf("target_date malformado deveria dar validation_error, veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "objectives"); n != 0 {
		t.Errorf("inválidos não deviam persistir; %d objetivos no banco", n)
	}
}

// TestPatchModoTriState: PATCH target_date tri-state (ausente mantém, null
// limpa, valor define); PATCH completion_mode troca sem zerar goal_hours/
// target_date (CA-03.1/04). CA-04.1 (marco dormente): trocar by_date→by_tasks
// PRESERVA o target_date no banco (não zera).
func TestPatchModoTriState(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{
		Name: "obj", CompletionMode: "by_date", TargetDate: ptr("2026-07-15"), GoalHours: ptr(10.0)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// ausente mantém: patch só do nome NÃO mexe em target_date/completion_mode/goal_hours
	upd, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{Name: ptr("novo")})
	if err != nil {
		t.Fatalf("patch nome: %v", err)
	}
	if upd.TargetDate == nil || *upd.TargetDate != "2026-07-15" {
		t.Errorf("ausente deveria manter target_date, veio %v", upd.TargetDate)
	}
	if upd.CompletionMode != "by_date" {
		t.Errorf("ausente deveria manter modo by_date, veio %q", upd.CompletionMode)
	}

	// trocar modo by_date→by_tasks NÃO zera target_date nem goal_hours (CA-04/04.1 marco dormente)
	upd, err = svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{CompletionMode: ptr("by_tasks")})
	if err != nil {
		t.Fatalf("patch modo: %v", err)
	}
	if upd.CompletionMode != "by_tasks" {
		t.Errorf("modo deveria virar by_tasks, veio %q", upd.CompletionMode)
	}
	if upd.TargetDate == nil || *upd.TargetDate != "2026-07-15" {
		t.Errorf("CA-04.1: trocar modo NÃO deveria zerar target_date (marco dormente), veio %v", upd.TargetDate)
	}
	if upd.GoalHours == nil || *upd.GoalHours != 10 {
		t.Errorf("trocar modo NÃO deveria zerar goal_hours, veio %v", upd.GoalHours)
	}

	// null limpa o marco (tri-state)
	upd, err = svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{TargetDateSet: true, TargetDate: nil})
	if err != nil {
		t.Fatalf("patch target_date null: %v", err)
	}
	if upd.TargetDate != nil {
		t.Errorf("null deveria limpar o marco, veio %v", *upd.TargetDate)
	}

	// valor define
	upd, err = svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{TargetDateSet: true, TargetDate: ptr("2026-08-01")})
	if err != nil {
		t.Fatalf("patch target_date valor: %v", err)
	}
	if upd.TargetDate == nil || *upd.TargetDate != "2026-08-01" {
		t.Errorf("valor deveria definir target_date, veio %v", upd.TargetDate)
	}
}

// TestPatchModoInvalido: PATCH com completion_mode/target_date inválido → 400.
func TestPatchModoInvalido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, _ := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "obj"})
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{CompletionMode: ptr("foo")}); !isValidation(err) {
		t.Errorf("completion_mode='foo' no PATCH deveria dar validation_error, veio %v", err)
	}
	if _, err := svc.Patch(ctx, s.ws, s.pOwner, "member", s.shared, o.ID, PatchInput{TargetDateSet: true, TargetDate: ptr("2026-13-40")}); !isValidation(err) {
		t.Errorf("target_date malformado no PATCH deveria dar validation_error, veio %v", err)
	}
}
