// edit_current_test.go — T-00 da fase 27: allowance EditCurrent (decisão HITL
// 23/06). Edita SÓ description/project_id/tags da entry RODANDO; timing
// (started_at/stopped_at/duration/mode/target) fica INTOCADO (CA-13.1);
// dispara OnChange 1× (CA-13/04); valida project_id no ws da entry (CA-13.2);
// ocioso/break → 409 no_active_timer; isolamento por usuário.
package timer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/timer"
)

func strptr(s string) *string { return &s }

// CA-13/13.1 — EditCurrent edita description/project_id/tags da entry RODANDO,
// e os campos de TEMPO ficam exatamente como estavam.
func TestEditCurrentEditaRotuloProjetoTagsTimingIntocado(t *testing.T) {
	svc, clock, pool, userID, wsID, projectID := newTimerService(t)
	ctx := context.Background()

	startRes, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Description: "antigo", Tags: []string{"x"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	before := startRes.State.Entry
	clock.Advance(120 * time.Second)

	res, err := svc.EditCurrent(ctx, userID, timer.EditCurrentInput{
		Description:  strptr("novo rótulo"),
		Tags:         &[]string{"a", "b"},
		ProjectIDSet: true,
		ProjectID:    &projectID,
	})
	if err != nil {
		t.Fatalf("EditCurrent: %v", err)
	}
	if res.State == nil || res.State.Entry == nil {
		t.Fatalf("EditCurrent deveria devolver o State da entry rodando, veio %+v", res.State)
	}
	got := res.State.Entry
	if got.Description != "novo rótulo" {
		t.Errorf("descrição deveria virar 'novo rótulo', veio %q", got.Description)
	}
	if got.ProjectID == nil || *got.ProjectID != projectID {
		t.Errorf("project_id deveria ser %q, veio %v", projectID, got.ProjectID)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Errorf("tags deveriam ser [a b], veio %v", got.Tags)
	}

	// CA-13.1: timing INTOCADO — started_at/stopped_at/duration/mode
	if !got.StartedAt.Equal(before.StartedAt) {
		t.Errorf("started_at deveria ser imutável (%v), veio %v", before.StartedAt, got.StartedAt)
	}
	if got.StoppedAt != nil {
		t.Errorf("a entry segue rodando — stopped_at deveria ser nil, veio %v", got.StoppedAt)
	}
	if got.DurationSeconds != nil {
		t.Errorf("a entry segue rodando — duration deveria ser nil, veio %v", got.DurationSeconds)
	}
	if res.State.Mode != "countup" {
		t.Errorf("mode deveria seguir countup, veio %q", res.State.Mode)
	}
	// o elapsed reflete o relógio (120s passados), NÃO um reset
	if res.State.ElapsedSeconds != 120 {
		t.Errorf("elapsed deveria ser 120 (cronômetro não zerou), veio %d", res.State.ElapsedSeconds)
	}

	// e no banco a entry continua rodando com os campos novos
	var (
		dbDesc    string
		dbStopped *time.Time
		dbDur     *int
	)
	if err := pool.QueryRow(ctx,
		`SELECT description, stopped_at, duration_seconds FROM time_entries WHERE id = $1`,
		got.ID).Scan(&dbDesc, &dbStopped, &dbDur); err != nil {
		t.Fatalf("ler entry: %v", err)
	}
	if dbDesc != "novo rótulo" || dbStopped != nil || dbDur != nil {
		t.Errorf("no banco: desc=%q stopped=%v dur=%v (esperava novo rótulo, rodando)", dbDesc, dbStopped, dbDur)
	}
}

// CA-13/04 — OnChange dispara EXATAMENTE 1× com o State novo (rótulo editado).
func TestEditCurrentDisparaOnChangeUmaVez(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "antigo"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(30 * time.Second)

	rec := &hookRecorder{}
	svc.SetOnChange(rec.fn())

	res, err := svc.EditCurrent(ctx, userID, timer.EditCurrentInput{Description: strptr("editado")})
	if err != nil {
		t.Fatalf("EditCurrent: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("EditCurrent deveria emitir exatamente 1 evento, emitiu %d", rec.calls)
	}
	if rec.states[0] == nil || rec.states[0].Entry == nil || rec.states[0].Entry.Description != "editado" {
		t.Errorf("o evento deveria carregar o State com a descrição nova, veio %+v", rec.states[0])
	}
	if !rec.ats[0].Equal(res.ServerTime) {
		t.Errorf("at do evento (%v) deveria ser o server_time da mutação (%v)", rec.ats[0], res.ServerTime)
	}
}

// CA-13.2 — project_id de OUTRO workspace → 404 not_found; o timer não muda
// (rollback).
func TestEditCurrentProjetoDeOutroWorkspace404(t *testing.T) {
	svc, _, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "rodando"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// projeto num workspace alheio
	var otherWs, otherProj string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ('Outro') RETURNING id`).Scan(&otherWs); err != nil {
		t.Fatalf("seed outro ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'Alheio', '#4f9dde', 'shared', $2) RETURNING id`, otherWs, userID).Scan(&otherProj); err != nil {
		t.Fatalf("seed projeto alheio: %v", err)
	}

	_, err := svc.EditCurrent(ctx, userID, timer.EditCurrentInput{
		Description: strptr("tentativa"), ProjectIDSet: true, ProjectID: &otherProj,
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "not_found" {
		t.Fatalf("project de outro ws deveria dar 404 not_found, veio %v", err)
	}

	// rollback: a descrição segue a antiga e nenhum project_id foi gravado
	var (
		dbDesc string
		dbProj *string
	)
	if err := pool.QueryRow(ctx,
		`SELECT description, project_id FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&dbDesc, &dbProj); err != nil {
		t.Fatalf("ler entry: %v", err)
	}
	if dbDesc != "rodando" || dbProj != nil {
		t.Errorf("EditCurrent que falha não pode editar nada (desc=%q proj=%v)", dbDesc, dbProj)
	}
}

// CA-13 — sem timer rodando (ocioso) → 409 no_active_timer.
func TestEditCurrentOcioso409(t *testing.T) {
	svc, _, _, userID, _, _ := newTimerService(t)
	ctx := context.Background()

	_, err := svc.EditCurrent(ctx, userID, timer.EditCurrentInput{Description: strptr("x")})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "no_active_timer" {
		t.Fatalf("EditCurrent ocioso deveria dar 409 no_active_timer, veio %v", err)
	}
}

// CA-13 — durante 'break' (pausa) não há entry de trabalho → 409 no_active_timer.
func TestEditCurrentDuranteBreak409(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(60 * time.Second)
	if _, err := svc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause: %v", err)
	}

	_, err := svc.EditCurrent(ctx, userID, timer.EditCurrentInput{Description: strptr("x")})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "no_active_timer" {
		t.Fatalf("EditCurrent durante break deveria dar 409 no_active_timer, veio %v", err)
	}
}

// Isolamento — EditCurrent edita só o timer do PRÓPRIO usuário (mata a mutação
// que remove o filtro user_id de loadStateRow/Update — espelha
// TestIsolamentoTimerEntreUsuarios).
func TestEditCurrentIsolamentoPorUsuario(t *testing.T) {
	svc, clock, pool, ana, wsID, _ := newTimerService(t)
	ctx := context.Background()

	var bia string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-bia', 'bia@tt.dev', 'Bia') RETURNING id`).Scan(&bia); err != nil {
		t.Fatalf("seed bia: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`,
		wsID, bia); err != nil {
		t.Fatalf("membership da bia: %v", err)
	}

	anaRes, err := svc.Start(ctx, ana, timer.StartInput{WorkspaceID: wsID, Description: "ana"})
	if err != nil {
		t.Fatalf("start ana: %v", err)
	}
	biaRes, err := svc.Start(ctx, bia, timer.StartInput{WorkspaceID: wsID, Description: "bia"})
	if err != nil {
		t.Fatalf("start bia: %v", err)
	}
	clock.Advance(10 * time.Second)

	// Bia edita o SEU timer — não pode tocar o da Ana
	if _, err := svc.EditCurrent(ctx, bia, timer.EditCurrentInput{Description: strptr("bia editado")}); err != nil {
		t.Fatalf("EditCurrent da bia: %v", err)
	}

	var anaDesc, biaDesc string
	if err := pool.QueryRow(ctx, `SELECT description FROM time_entries WHERE id = $1`, anaRes.State.Entry.ID).Scan(&anaDesc); err != nil {
		t.Fatalf("ler ana: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT description FROM time_entries WHERE id = $1`, biaRes.State.Entry.ID).Scan(&biaDesc); err != nil {
		t.Fatalf("ler bia: %v", err)
	}
	if anaDesc != "ana" {
		t.Errorf("a edição da Bia NÃO pode tocar a entry da Ana (desc=%q)", anaDesc)
	}
	if biaDesc != "bia editado" {
		t.Errorf("a edição da Bia deveria editar a entry dela (desc=%q)", biaDesc)
	}
}
