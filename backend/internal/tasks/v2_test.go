// v2_test.go — fase 24 T-03: priority/notion_url/position + status backlog no
// tasks.Service. White-box (package tasks): exercita a Service real contra o
// dbtest. Reusa o seed/harness do objective_status_test.go.
package tasks

import (
	"context"
	"errors"
	"strings"
	"testing"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

func isValidation(err error) bool {
	var de *domain.Error
	return errors.As(err, &de) && de.Code == "validation_error"
}

func iptr(i int) *int       { return &i }
func sptr(s string) *string { return &s }

// CA-01/02: cria task com status:backlog, priority:2, notion_url → persiste e devolve.
func TestCreateComCamposV2(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	url := "https://www.notion.so/Churn-8f3a1c9b"
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{
		ProjectID: s.project, Name: "Investigar churn",
		Status: sptr("backlog"), Priority: iptr(2), NotionURL: &url,
	})
	if err != nil {
		t.Fatalf("create com campos v2: %v", err)
	}
	if task.Status != "backlog" {
		t.Errorf("status deveria ser backlog, veio %q", task.Status)
	}
	if task.Priority != 2 {
		t.Errorf("priority deveria ser 2, veio %d", task.Priority)
	}
	if task.NotionURL == nil || *task.NotionURL != url {
		t.Errorf("notion_url deveria ser %q, veio %v", url, task.NotionURL)
	}

	// CA-02 defaults: priority ausente → 0; status ausente → todo; notion ausente → nil.
	t2, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2"})
	if err != nil {
		t.Fatalf("create default: %v", err)
	}
	if t2.Priority != 0 || t2.Status != "todo" || t2.NotionURL != nil {
		t.Errorf("defaults errados: prio=%d status=%q notion=%v", t2.Priority, t2.Status, t2.NotionURL)
	}
}

// CA-02.1: priority -1 e 4 → 400 validation_error (no Create e no Patch).
func TestPriorityForaDaFaixa(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	for _, p := range []int{-1, 4} {
		_, _, err := svc.Create(ctx, s.ws, s.user, "owner",
			CreateInput{ProjectID: s.project, Name: "x", Priority: iptr(p)})
		if !isValidation(err) {
			t.Errorf("Create priority=%d deveria dar validation_error, veio %v", p, err)
		}
	}
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "ok"})
	if err != nil {
		t.Fatalf("create base: %v", err)
	}
	for _, p := range []int{-1, 4} {
		_, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{PrioritySet: true, Priority: iptr(p)})
		if !isValidation(err) {
			t.Errorf("Patch priority=%d deveria dar validation_error, veio %v", p, err)
		}
	}
}

// CA-04: notion_url > 2048 → 400; vazio/só-espaços → null.
func TestNotionURLValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	big := strings.Repeat("a", 2049)
	_, _, err := svc.Create(ctx, s.ws, s.user, "owner",
		CreateInput{ProjectID: s.project, Name: "x", NotionURL: &big})
	if !isValidation(err) {
		t.Errorf("notion_url > 2048 deveria dar validation_error, veio %v", err)
	}

	blank := "   "
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner",
		CreateInput{ProjectID: s.project, Name: "x", NotionURL: &blank})
	if err != nil {
		t.Fatalf("create notion vazio: %v", err)
	}
	if task.NotionURL != nil {
		t.Errorf("notion_url só-espaços deveria virar nil, veio %v", task.NotionURL)
	}
}

// CA-03/D-09: notion_url tri-state no PATCH — null limpa, ausente mantém, string define.
func TestNotionURLTriState(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	url := "https://www.notion.so/Doc-abc123"
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner",
		CreateInput{ProjectID: s.project, Name: "x", NotionURL: &url})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// ausente → mantém (patch só do name).
	upd, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{Name: sptr("y")})
	if err != nil {
		t.Fatalf("patch name: %v", err)
	}
	if upd.NotionURL == nil || *upd.NotionURL != url {
		t.Errorf("notion_url ausente deveria manter %q, veio %v", url, upd.NotionURL)
	}

	// null → limpa.
	upd, _, err = svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{NotionURLSet: true, NotionURL: nil})
	if err != nil {
		t.Fatalf("patch notion null: %v", err)
	}
	if upd.NotionURL != nil {
		t.Errorf("notion_url null deveria limpar, veio %v", upd.NotionURL)
	}

	// string → define.
	url2 := "https://www.notion.so/Doc2-def456"
	upd, _, err = svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{NotionURLSet: true, NotionURL: &url2})
	if err != nil {
		t.Fatalf("patch notion set: %v", err)
	}
	if upd.NotionURL == nil || *upd.NotionURL != url2 {
		t.Errorf("notion_url deveria virar %q, veio %v", url2, upd.NotionURL)
	}
}

// CA-05/D-04: PATCH só priority NÃO recalcula objetivo (edição benigna).
// Objetivo manual-done com a task ainda doing < 100% deve continuar done.
func TestPatchPriorityNaoRecalculaObjetivo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	task, _, err := svc.Create(ctx, s.ws, s.user, "owner",
		CreateInput{ProjectID: s.project, Name: "T", ObjectiveID: &o})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// força o objetivo a done manualmente (task ainda todo → < 100%).
	setObjStatus(t, pool, o, "done")

	// patch SÓ priority → benigno, NÃO recalcula (senão reabriria o objetivo).
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{PrioritySet: true, Priority: iptr(3)}); err != nil {
		t.Fatalf("patch priority: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("patch só priority NÃO deveria recalcular o objetivo (esperado done, veio %q)", st)
	}

	// idem só notion_url e só position.
	url := "https://www.notion.so/x-1"
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{NotionURLSet: true, NotionURL: &url}); err != nil {
		t.Fatalf("patch notion: %v", err)
	}
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{PositionSet: true, Position: iptr(5)}); err != nil {
		t.Fatalf("patch position: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("patch notion/position NÃO deveria recalcular o objetivo (esperado done, veio %q)", st)
	}
}

// CA-05: PATCH status backlog→done recalcula (auto-conclui o objetivo em 100%).
func TestPatchStatusBacklogRecalcula(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	task, _, err := svc.Create(ctx, s.ws, s.user, "owner",
		CreateInput{ProjectID: s.project, Name: "T", Status: sptr("backlog"), ObjectiveID: &o})
	if err != nil {
		t.Fatalf("create backlog: %v", err)
	}
	if st := objStatus(t, pool, o); st != "open" {
		t.Fatalf("objetivo deveria estar open com a task em backlog, veio %q", st)
	}
	// backlog conta como não-done → mover pra done fecha o objetivo (100%).
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{Status: sptr("done")}); err != nil {
		t.Fatalf("patch status done: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("mover a última task pra done deveria fechar o objetivo, veio %q", st)
	}
}

// CA-01: validStatus aceita backlog e rejeita lixo no Patch.
func TestValidStatusBacklog(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{Status: sptr("backlog")}); err != nil {
		t.Errorf("status backlog deveria ser aceito, veio %v", err)
	}
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{Status: sptr("xyz")}); !isValidation(err) {
		t.Errorf("status xyz deveria dar validation_error, veio %v", err)
	}
}

// D-03: position no create = max+1 por (project_id, status). 2 creates no mesmo status → 0,1.
func TestNextPositionPorStatus(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	a, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "A", Status: sptr("todo")})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "B", Status: sptr("todo")})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if a.Position != 0 {
		t.Errorf("1ª task no status todo deveria ter position 0, veio %d", a.Position)
	}
	if b.Position != 1 {
		t.Errorf("2ª task no status todo deveria ter position 1, veio %d", b.Position)
	}
	// status diferente reinicia a contagem (max+1 por (project_id, status)).
	c, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "C", Status: sptr("backlog")})
	if err != nil {
		t.Fatalf("create C: %v", err)
	}
	if c.Position != 0 {
		t.Errorf("1ª task no status backlog deveria ter position 0, veio %d", c.Position)
	}
}
