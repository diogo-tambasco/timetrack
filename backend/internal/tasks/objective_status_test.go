// objective_status_test.go — fase 12 T-04: vínculo task↔objetivo (CA-09/09.1/
// 11) + recálculo de status POR TRANSIÇÃO (CA-05/06/06.1). White-box (package
// tasks): exercita a Service real contra o dbtest e confere o status do
// objetivo por SQL.
package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

type taskSeed struct {
	ws, user, project, other string // other = 2º projeto (cross-project)
}

func seedTasks(t *testing.T, pool *pgxpool.Pool) taskSeed {
	t.Helper()
	ctx := context.Background()
	var s taskSeed
	scan := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
		return id
	}
	s.ws = scan(`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`)
	s.user = scan(`INSERT INTO users (google_id, email, name) VALUES ('u', 'u@b.c', 'U') RETURNING id`)
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, s.ws, s.user); err != nil {
		t.Fatalf("seed ws member: %v", err)
	}
	s.project = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'P', '#4f9dde', 'shared', $2) RETURNING id`, s.ws, s.user)
	s.other = scan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'Other', '#000000', 'shared', $2) RETURNING id`, s.ws, s.user)
	return s
}

func mkObjective(t *testing.T, pool *pgxpool.Pool, s taskSeed, projectID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO objectives (workspace_id, project_id, name, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		s.ws, projectID, name, s.user).Scan(&id); err != nil {
		t.Fatalf("mkObjective: %v", err)
	}
	return id
}

func objStatus(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var st string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM objectives WHERE id = $1`, id).Scan(&st); err != nil {
		t.Fatalf("objStatus: %v", err)
	}
	return st
}

func setObjStatus(t *testing.T, pool *pgxpool.Pool, id, st string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE objectives SET status = $2 WHERE id = $1`, id, st); err != nil {
		t.Fatalf("setObjStatus: %v", err)
	}
}

func isInvalidRef(err error) bool {
	var de *domain.Error
	return errors.As(err, &de) && de.Code == "invalid_reference"
}

func TestObjectiveIDRefValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")
	oOther := mkObjective(t, pool, s, s.other, "O-other")

	// objective_id válido do mesmo projeto → persiste
	task, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T", ObjectiveID: &o})
	if err != nil || task.ObjectiveID == nil || *task.ObjectiveID != o {
		t.Fatalf("objective_id válido deveria persistir; veio %v (err=%v)", task.ObjectiveID, err)
	}
	// objective_id de OUTRO projeto → invalid_reference
	if _, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "X", ObjectiveID: &oOther}); !isInvalidRef(err) {
		t.Errorf("objective_id de outro projeto deveria dar invalid_reference, veio %v", err)
	}
	// objective_id malformado → invalid_reference
	bad := "nao-e-uuid"
	if _, _, err := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "X", ObjectiveID: &bad}); !isInvalidRef(err) {
		t.Errorf("objective_id malformado deveria dar invalid_reference, veio %v", err)
	}
	// PATCH objective_id:null desassocia (vira Geral)
	upd, _, err := svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{ObjectiveIDSet: true, ObjectiveID: nil})
	if err != nil || upd.ObjectiveID != nil {
		t.Errorf("objective_id:null deveria desassociar, veio %v (err=%v)", upd.ObjectiveID, err)
	}
}

func TestAutoStatusTransicao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	t2, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})

	// 2 tasks todo → objetivo open
	if st := objStatus(t, pool, o); st != "open" {
		t.Fatalf("objetivo com tasks todo deveria estar open, veio %q", st)
	}
	done := "done"
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &done})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("50%% done ainda é open, veio %q", st)
	}
	// última task done → 100% → objetivo done (auto, CA-06)
	svc.Patch(ctx, s.ws, s.user, "owner", t2.ID, PatchInput{Status: &done})
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("100%% done deveria auto-concluir, veio %q", st)
	}
	// uma task volta a doing → reabre
	doing := "doing"
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &doing})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("deixou de ser 100%% → deveria reabrir, veio %q", st)
	}
}

// TestAutoStatusConcorrente pina a verificação adversarial do build (CA-06):
// marcar as duas (últimas) tasks de um objetivo como done EM PARALELO precisa
// auto-concluir o objetivo. Sem o lock no recálculo, sob READ COMMITTED cada
// transação conta d=1,n=2 (não enxerga o done ainda não commitado da irmã) e o
// objetivo fica preso em open a 100% — estado persistente e incoerente. O loop
// expõe a corrida (sem o fix falha na maioria das iterações); com o FOR UPDATE
// em recalcObjectiveStatus o resultado é determinístico: sempre done.
func TestAutoStatusConcorrente(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	done := "done"

	const N = 25
	for i := 0; i < N; i++ {
		o := mkObjective(t, pool, s, s.project, "O")
		t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
		t2, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})

		var wg sync.WaitGroup
		for _, id := range []string{t1.ID, t2.ID} {
			id := id
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, _ = svc.Patch(ctx, s.ws, s.user, "owner", id, PatchInput{Status: &done})
			}()
		}
		wg.Wait()

		if st := objStatus(t, pool, o); st != "done" {
			t.Fatalf("iteração %d: 2 tasks done concorrentes deveriam auto-concluir o objetivo, veio %q", i, st)
		}
	}
}

func TestObjetivoVazioNaoTocado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	done := "done"
	task, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T", ObjectiveID: &o})
	svc.Patch(ctx, s.ws, s.user, "owner", task.ID, PatchInput{Status: &done})
	if st := objStatus(t, pool, o); st != "done" {
		t.Fatalf("1 task done → objetivo done, veio %q", st)
	}
	// deletar a última task → objetivo fica SEM tasks → recálculo no-op (não reabre)
	if err := svc.Delete(ctx, s.ws, s.user, "owner", task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("objetivo sem tasks NÃO deveria ser reaberto (n=0 no-op), veio %q", st)
	}
}

func TestManualClosePersisteEdicaoBenigna(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	// fecho manual a <100% (CA-05)
	setObjStatus(t, pool, o, "done")
	// edição BENIGNA (só nome) NÃO dispara recálculo → continua done (CA-06.1)
	name := "renomeada"
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Name: &name}); err != nil {
		t.Fatalf("patch nome: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("fecho manual a <100%% deveria sobreviver a edição benigna, veio %q", st)
	}
	// mudar o STATUS de uma task (transição real) dispara o recálculo → reabre (<100%)
	doing := "doing"
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &doing})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("transição real deveria reabrir, veio %q", st)
	}
}

func TestManualOpenPersisteEdicaoBenigna(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	done := "done"
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &done}) // 100% → auto done
	// reabertura manual a 100% (CA-05)
	setObjStatus(t, pool, o, "open")
	// edição benigna → continua open (CA-06.1)
	notes := "anotação"
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Notes: &notes})
	if st := objStatus(t, pool, o); st != "open" {
		t.Errorf("reabertura manual a 100%% deveria sobreviver a edição benigna, veio %q", st)
	}
}

// (verificação do build da fase 20) CA-06.1 × PATCH due-only: a fase 20 fez uma
// mudança SÓ de due_date (valor) abrir tx (pro ReminderHook). O recálculo de
// objetivo DEVE seguir suprimido nesse caminho (só status/objetivo recalculam) —
// senão um simples PATCH de prazo numa task de objetivo fechado-manualmente a
// <100% REABRIRIA o objetivo. Os testes de edição benigna acima usam Name/Notes,
// que NÃO abrem tx; sem este teste a mutação que remove a supressão (popular o
// affected map incondicionalmente, como era pré-fase-20) sobrevive à suíte.
func TestManualClosePersistePatchDueDate(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	o := mkObjective(t, pool, s, s.project, "O")

	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &o})
	svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &o})
	// fecho manual a <100% (2 tasks, nenhuma done)
	setObjStatus(t, pool, o, "done")
	// PATCH SÓ da due_date (nil → valor = mudança real) → abre tx pro reminder hook,
	// MAS não pode recalcular o objetivo (CA-06.1).
	due := "2026-12-31"
	if _, _, err := svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{DueDateSet: true, DueDate: &due}); err != nil {
		t.Fatalf("patch due-only: %v", err)
	}
	if st := objStatus(t, pool, o); st != "done" {
		t.Errorf("fecho manual a <100%% deveria sobreviver a PATCH due-only (recalc suprimido), veio %q", st)
	}
}

func TestMoveRecalcaOsDois(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedTasks(t, pool)
	ctx := context.Background()
	oA := mkObjective(t, pool, s, s.project, "A")
	oB := mkObjective(t, pool, s, s.project, "B")

	done := "done"
	// A: T1 done, T2 todo → A open
	t1, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T1", ObjectiveID: &oA})
	svc.Patch(ctx, s.ws, s.user, "owner", t1.ID, PatchInput{Status: &done})
	t2, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T2", ObjectiveID: &oA})
	// B: T3 done → B done
	t3, _, _ := svc.Create(ctx, s.ws, s.user, "owner", CreateInput{ProjectID: s.project, Name: "T3", ObjectiveID: &oB})
	svc.Patch(ctx, s.ws, s.user, "owner", t3.ID, PatchInput{Status: &done})
	if objStatus(t, pool, oA) != "open" || objStatus(t, pool, oB) != "done" {
		t.Fatalf("setup: A=%q B=%q (esperava open/done)", objStatus(t, pool, oA), objStatus(t, pool, oB))
	}
	// move T2 (todo) de A → B: A passa a 100% (só T1 done) → done; B cai a 50% → reabre
	svc.Patch(ctx, s.ws, s.user, "owner", t2.ID, PatchInput{ObjectiveIDSet: true, ObjectiveID: &oB})
	if st := objStatus(t, pool, oA); st != "done" {
		t.Errorf("A deveria auto-concluir após a saída da task todo, veio %q", st)
	}
	if st := objStatus(t, pool, oB); st != "open" {
		t.Errorf("B deveria reabrir após receber task todo, veio %q", st)
	}
}
