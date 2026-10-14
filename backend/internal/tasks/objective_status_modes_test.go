// objective_status_modes_test.go — fase 29 T-04: recalcObjectiveStatus ramifica
// por completion_mode. by_tasks = byte-equivalente à fase 12 (regressão); by_hours
// fecha por horas (per-usuário — decisão E); by_date = no-op (fecho só manual).
package tasks

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

// mkClosedEntry semeia uma entry fechada (duration_seconds) numa task.
func mkClosedEntry(t *testing.T, pool *pgxpool.Pool, ws, user, project, taskID string, seconds int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO time_entries (workspace_id, user_id, project_id, task_id, started_at, stopped_at, duration_seconds, source)
		VALUES ($1, $2, $3, $4, now() - make_interval(secs => $5), now(), $5, 'manual')`,
		ws, user, project, taskID, seconds); err != nil {
		t.Fatalf("mkClosedEntry: %v", err)
	}
}

// mkUserMember cria um usuário e o adiciona como membro do workspace.
func mkUserMember(t *testing.T, pool *pgxpool.Pool, ws, googleID string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', $1) RETURNING id`,
		googleID).Scan(&id); err != nil {
		t.Fatalf("mkUserMember user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, ws, id); err != nil {
		t.Fatalf("mkUserMember member: %v", err)
	}
	return id
}

// TestRecalcByHoursAutoClose: objetivo by_hours com goal_hours bate a meta e
// uma mutação de task subsequente fecha (CA-06). O recálculo só roda em mutação
// de task, não no fechamento de entry (CA-06.2/29-H7).
func TestRecalcByHoursAutoClose(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	// objetivo by_hours, meta 2h.
	o := mkObjective(t, pool, s, s.project, "Horas")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2 WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}

	// task 1 (todo) com entry de 7300s (> 2h = 7200s) DO PRÓPRIO user.
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 7300)

	// (CA-06.2) sem mutação de task após a entry, o status persiste open (gatilho não disparou).
	if st := objStatus(t, pool, o); st != "open" {
		t.Fatalf("CA-06.2: sem mutação de task o status deveria persistir open, veio %q", st)
	}

	// mutação de task subsequente (criar OUTRA task do objetivo) → recálculo roda → done.
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_hours com meta batida deveria auto-concluir na mutação de task, veio %q", st)
	}

	// tirar as horas (deletar a entry) e disparar mutação de task → reabre (<meta).
	if _, err := pool.Exec(ctx, `DELETE FROM time_entries WHERE task_id=$1`, t1.ID); err != nil {
		t.Fatalf("delete entry: %v", err)
	}
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T3", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("by_hours abaixo da meta deveria reabrir, veio %q", st)
	}
}

// TestRecalcByHoursPerUsuario: entry de OUTRO usuário numa task do objetivo
// by_hours NÃO conta pro auto-close (só as do user da mutação — decisão E/29-H1).
// Mutação que REMOVE `WHERE e.user_id` passaria a contar a entry alheia e
// fecharia o objetivo indevidamente → este teste quebra.
func TestRecalcByHoursPerUsuario(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	other := mkUserMember(t, pool, s.ws, "other")

	o := mkObjective(t, pool, s, s.project, "Horas")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2 WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	// entry de 7300s mas do OUTRO usuário → NÃO conta pro auto-close do s.user.
	mkClosedEntry(t, pool, s.ws, other, s.project, t1.ID, 7300)

	// mutação de task (do s.user) → recálculo com o user da mutação → não fecha (0s do s.user).
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("horas de OUTRO usuário NÃO deveriam fechar o objetivo by_hours (per-usuário), veio %q", st)
	}
}

// TestRecalcByHoursSemMeta: by_hours sem goal_hours nunca auto-fecha (CA-06.1).
func TestRecalcByHoursSemMeta(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Horas sem meta")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours' WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 99999)
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("by_hours sem meta nunca auto-fecha, veio %q", st)
	}
}

// TestRecalcByDateNoOp: by_date é no-op sempre — marcar todas as tasks done NÃO
// fecha; só PATCH status manual fecha (CA-07/08).
func TestRecalcByDateNoOp(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Marco")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_date', target_date='2026-07-15' WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	done := "done"
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	t2, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	// marcar TODAS as tasks done → by_date continua open (no-op — CA-07).
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &done})
	svc.Patch(ctx, s.ws, s.user, "owner", t2.ID, PatchInput{Status: &done})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("by_date com 100%% tasks done NÃO deveria auto-fechar (no-op), veio %q", st)
	}
}

// TestRecalcByTasksInalterado: o ramo by_tasks (default) é byte-equivalente à
// fase 12 — 100% done auto-conclui; <100% reabre (CA-05). Pin explícito do modo.
func TestRecalcByTasksInalterado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Tasks") // default by_tasks
	done := "done"
	doing := "doing"
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	t2, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &done})
	svc.Patch(ctx, s.ws, s.user, "owner", t2.ID, PatchInput{Status: &done})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_tasks 100%% → done, veio %q", st)
	}
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &doing})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("by_tasks <100%% → reabre, veio %q", st)
	}
}

// TestRecalcByTasksVazioOpenNaoFecha (T-13/#1): objetivo by_tasks VAZIO (0 tasks)
// em open NÃO auto-fecha (CA-05: task_count=0 → no-op). Deletar a última task de
// um objetivo open (que nunca teve task done) dispara o recálculo num objetivo
// vazio+open — a guarda `agg.n >= 1` (objective_link.go:84) impede o fecho. Mutação
// que remove `agg.n >= 1` faria agg.d=agg.n (0=0) & open → done indevidamente.
func TestRecalcByTasksVazioOpenNaoFecha(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Tasks") // default by_tasks, open
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	// deleta a ÚNICA task (todo, nunca done) → recálculo no objetivo agora VAZIO+open.
	if err := svc.Delete(ctx, s.ws, s.user, "owner", t1.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("objetivo by_tasks VAZIO+open NÃO deveria auto-fechar (n=0 → no-op), veio %q", st)
	}
}

// TestRecalcByHoursBordaExataFecha (T-13/#2): a meta EXATAMENTE batida fecha o
// by_hours (limiar INCLUSIVO secs >= goal×3600 — CA-06). Entry de 7200s == 2h.
// Mutação >= → > (objective_link.go:86) deixaria a borda exata sem fechar.
func TestRecalcByHoursBordaExataFecha(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Horas")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2 WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 7200) // EXATAMENTE 2h
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_hours com a meta EXATAMENTE batida (7200s==2h) deveria fechar (>= inclusivo), veio %q", st)
	}
}

// TestRecalcByHoursBordaExataNaoReabre (T-13/#3, LOW): na borda exata secs==meta
// o by_hours done NÃO reabre (a reabertura é secs < meta — exclusiva, CA-06).
// Mutação < → <= (objective_link.go:87) reabriria na borda.
func TestRecalcByHoursBordaExataNaoReabre(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Horas") // by_tasks/open por ora
	// task + entry de EXATAMENTE 2h ANTES de virar by_hours/done (assim o recálculo
	// final vê secs==meta com status=done — isola a borda da REABERTURA, sem um
	// recalc intermediário com secs=0 reabrindo por outro motivo).
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 7200) // EXATAMENTE 2h
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2, status='done' WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	// mutação de task com secs==meta já presente → recalc by_hours vê 7200 e done:
	// reabertura é `secs < meta` EXCLUSIVO → 7200<7200=false → fica done.
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_hours done com secs==meta NÃO deveria reabrir (reabertura é < exclusivo), veio %q", st)
	}
}

// TestRecalcByHoursMultiplasEntries (T-13/#4): 1 task com 2 entries do mesmo user
// → o secs SOMA as duas (4000+4000=8000 >= 7200). Pina que secs é SUM sobre as
// entries da task (não MAX/COUNT) e que o fan-out do join não infla. Mutação que
// trocasse SUM por outro agregado (ou juntasse n/d/secs num join único) seria pega.
func TestRecalcByHoursMultiplasEntries(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Horas")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_hours', goal_hours=2 WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	// 2 entries na MESMA task: cada uma < meta, mas a SOMA bate a meta.
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 4000)
	mkClosedEntry(t, pool, s.ws, s.user, s.project, t1.ID, 4000)
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_hours: 2 entries (4000+4000=8000>=7200) deveriam SOMAR e fechar, veio %q", st)
	}
}

// TestRecalcByDateNaoReabre (T-13/#5): by_date é no-op nos DOIS sentidos — um
// marco fechado manualmente NÃO reabre por tasks ficarem incompletas (CA-07).
// Mutação que adicionasse um ramo by_date de reabertura (d<n & done → open) seria
// pega aqui.
func TestRecalcByDateNaoReabre(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()

	o := mkObjective(t, pool, s, s.project, "Marco")
	if _, err := pool.Exec(ctx,
		`UPDATE objectives SET completion_mode='by_date', target_date='2026-07-15', status='done' WHERE id=$1`, o); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	// marco fechado manualmente (done) com tasks INCOMPLETAS (d<n).
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	// mutação de task subsequente (não-done) dispara o recálculo → by_date no-op → fica done.
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("by_date done NÃO deveria reabrir por tasks incompletas (no-op nos dois sentidos), veio %q", st)
	}
}
