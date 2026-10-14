package projects_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/projects"
)

func seedMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		wsID, userID, role); err != nil {
		t.Fatalf("seed member: %v", err)
	}
}

func mkTask(t *testing.T, pool *pgxpool.Pool, wsID, projectID, userID, status string, dueDate *string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, status, due_date)
		 VALUES ($1, $2, 'T', $3, $4, $5) RETURNING id`,
		wsID, projectID, userID, status, dueDate).Scan(&id); err != nil {
		t.Fatalf("mkTask: %v", err)
	}
	return id
}

func mkEntry(t *testing.T, pool *pgxpool.Pool, wsID, userID, projectID string, taskID *string, seconds int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, now() - make_interval(secs => $5), now(), $5, 'manual')`,
		wsID, userID, projectID, taskID, seconds); err != nil {
		t.Fatalf("mkEntry: %v", err)
	}
}

func findSummary(list []projects.ProjectSummary, id string) *projects.ProjectSummary {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// CA-04/05 — contadores corretos por projeto; projeto 0 tasks → contadores 0;
// tracked_seconds = bucket projeto do user.
func TestSummaryContadores(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	p, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Acme", Color: "#4f9dde", Visibility: "shared", Deadline: ptr("2025-09-30"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	vazio, err := svc.Create(ctx, ws, owner, projects.CreateInput{
		Name: "Vazio", Color: "#4f9dde", Visibility: "shared",
	})
	if err != nil {
		t.Fatalf("create vazio: %v", err)
	}

	// 3 tasks: 1 done, 2 todo. Tempo: 2 entries no projeto (uma sem task).
	t1 := mkTask(t, pool, ws, p.ID, owner, "done", nil)
	mkTask(t, pool, ws, p.ID, owner, "todo", nil)
	mkTask(t, pool, ws, p.ID, owner, "todo", nil)
	mkEntry(t, pool, ws, owner, p.ID, &t1, 3600)
	mkEntry(t, pool, ws, owner, p.ID, nil, 1800) // entry sem task, conta no bucket projeto

	list, err := svc.Summary(ctx, ws, owner, "", false)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	sp := findSummary(list, p.ID)
	if sp == nil {
		t.Fatal("projeto Acme não veio no summary")
	}
	if sp.TaskCount != 3 || sp.DoneCount != 1 || sp.OpenCount != 2 {
		t.Errorf("contadores errados: task=%d done=%d open=%d (esperado 3/1/2)", sp.TaskCount, sp.DoneCount, sp.OpenCount)
	}
	if sp.TrackedSeconds != 5400 {
		t.Errorf("tracked_seconds deveria ser 5400 (3600+1800), veio %d", sp.TrackedSeconds)
	}
	if sp.Deadline == nil || *sp.Deadline != "2025-09-30" {
		t.Errorf("deadline deveria vir, veio %v", sp.Deadline)
	}

	sv := findSummary(list, vazio.ID)
	if sv == nil {
		t.Fatal("projeto Vazio não veio")
	}
	if sv.TaskCount != 0 || sv.DoneCount != 0 || sv.OpenCount != 0 || sv.DueTodayCount != 0 {
		t.Errorf("projeto vazio deveria ter contadores 0, veio %+v", sv)
	}
}

// CA-09 — tracked_seconds NÃO soma entries de outro user (mutação removendo
// user_id do SUM deve QUEBRAR).
func TestSummaryTrackedSoDoUser(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	outro := seedUser(t, pool, "g-outro")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, outro, "member")
	ctx := context.Background()

	p, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P", Color: "#4f9dde", Visibility: "shared"})
	mkEntry(t, pool, ws, owner, p.ID, nil, 3600)
	mkEntry(t, pool, ws, outro, p.ID, nil, 9999) // de OUTRO user, não deve somar

	list, err := svc.Summary(ctx, ws, owner, "", false)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	sp := findSummary(list, p.ID)
	if sp == nil || sp.TrackedSeconds != 3600 {
		t.Errorf("tracked deveria ser só o do user (3600), veio %v", sp)
	}
}

// CA-06 — due_today_count no tz da requisição; tz inválido → 400.
func TestSummaryDueTodayTz(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, ws, owner, "owner")
	ctx := context.Background()

	p, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "P", Color: "#4f9dde", Visibility: "shared"})
	// task vence HOJE no fuso de SP.
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load SP: %v", err)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	mkTask(t, pool, ws, p.ID, owner, "todo", &today)
	// task done que vence hoje NÃO conta.
	mkTask(t, pool, ws, p.ID, owner, "done", &today)

	list, err := svc.Summary(ctx, ws, owner, "America/Sao_Paulo", false)
	if err != nil {
		t.Fatalf("summary SP: %v", err)
	}
	sp := findSummary(list, p.ID)
	if sp == nil || sp.DueTodayCount != 1 {
		t.Errorf("due_today_count deveria ser 1 (só a não-done), veio %v", sp)
	}

	// tz inválido → 400.
	_, err = svc.Summary(ctx, ws, owner, "Nao/Existe", false)
	assertDomainCode(t, err, "validation_error", "Summary(tz inválido)")
}

// CA-04.1 — projeto private de OUTRO user NÃO aparece (mutação removendo a
// visibilidade deve QUEBRAR); CA-08 — arquivado fora sem ?archived.
func TestSummaryVisibilidadeEArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	ws := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	outro := seedUser(t, pool, "g-outro")
	seedMember(t, pool, ws, owner, "owner")
	seedMember(t, pool, ws, outro, "member")
	ctx := context.Background()

	// private do "outro" — owner NÃO deve ver.
	priv, _ := svc.Create(ctx, ws, outro, projects.CreateInput{Name: "Priv outro", Color: "#4f9dde", Visibility: "private"})
	// shared — todos veem.
	sh, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "Shared", Color: "#4f9dde", Visibility: "shared"})
	// arquivado.
	arq, _ := svc.Create(ctx, ws, owner, projects.CreateInput{Name: "Arq", Color: "#4f9dde", Visibility: "shared"})
	if err := svc.Archive(ctx, ws, owner, "owner", arq.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	list, err := svc.Summary(ctx, ws, owner, "", false)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if findSummary(list, priv.ID) != nil {
		t.Error("private de outro user NÃO deveria aparecer no summary")
	}
	if findSummary(list, sh.ID) == nil {
		t.Error("shared deveria aparecer")
	}
	if findSummary(list, arq.ID) != nil {
		t.Error("arquivado NÃO deveria aparecer sem ?archived")
	}

	// com ?archived inclui o arquivado.
	listArq, err := svc.Summary(ctx, ws, owner, "", true)
	if err != nil {
		t.Fatalf("summary archived: %v", err)
	}
	if findSummary(listArq, arq.ID) == nil {
		t.Error("arquivado deveria aparecer com ?archived=true")
	}
}

// CA-09 — isolamento por workspace (mutação removendo WHERE workspace_id morre).
func TestSummaryIsolamentoWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := projects.NewService(pool)
	wsA := seedWorkspace(t, pool)
	wsB := seedWorkspace(t, pool)
	owner := seedUser(t, pool, "g-owner")
	seedMember(t, pool, wsA, owner, "owner")
	seedMember(t, pool, wsB, owner, "owner")
	ctx := context.Background()

	pA, _ := svc.Create(ctx, wsA, owner, projects.CreateInput{Name: "A", Color: "#4f9dde", Visibility: "shared"})
	pB, _ := svc.Create(ctx, wsB, owner, projects.CreateInput{Name: "B", Color: "#4f9dde", Visibility: "shared"})

	list, err := svc.Summary(ctx, wsA, owner, "", false)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if findSummary(list, pA.ID) == nil {
		t.Error("projeto do ws A deveria aparecer")
	}
	if findSummary(list, pB.ID) != nil {
		t.Error("projeto do ws B NÃO deveria vazar no summary do ws A")
	}
}
