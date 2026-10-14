// service_test.go — fase 25 T-04: CRUD de defs + set/clear de value. dbtest real.
// Cobre CA-01/01.1/02/03/04/05/06/07/08/09/09.1/10/11/12/15. Mutação que remove
// project_id/workspace_id do WHERE deve morrer (isolamento/pertencimento).
package properties

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

// mkTask cria uma task no projeto e devolve o id.
func mkTask(t *testing.T, pool *pgxpool.Pool, ws, projectID, userID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tasks (workspace_id, project_id, name, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		ws, projectID, name, userID).Scan(&id); err != nil {
		t.Fatalf("mkTask: %v", err)
	}
	return id
}

func mustCreateDef(t *testing.T, svc *Service, ws, userID, role, projectID string, in CreateDefInput) PropertyDef {
	t.Helper()
	d, err := svc.CreateDef(context.Background(), ws, userID, role, projectID, in)
	if err != nil {
		t.Fatalf("CreateDef(%s): %v", in.Name, err)
	}
	return d
}

func TestCreateDefPorTipoEPosicao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// CA-01: cria por tipo; position incrementa no fim da lista.
	txt := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Nota", Type: "text"})
	if txt.Position != 0 || txt.Type != "text" || len(txt.Options) != 0 {
		t.Errorf("text def inesperado: %+v", txt)
	}
	num := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Estimativa", Type: "number"})
	if num.Position != 1 {
		t.Errorf("2ª def deveria ter position 1, veio %d", num.Position)
	}
	// CA-03: select gera id pras options.
	sel := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{
		Name: "Cliente", Type: "select",
		Options: []Option{{Label: "Lauda"}, {Label: "Arkan"}},
	})
	if len(sel.Options) != 2 || sel.Options[0].ID == "" || sel.Options[0].Label != "Lauda" {
		t.Errorf("select options sem id/label: %+v", sel.Options)
	}

	// CA-01.1: nome vazio/só espaço/>80 → validation.
	for _, bad := range []string{"", "   ", string(make([]byte, 81))} {
		if _, err := svc.CreateDef(ctx, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: bad, Type: "text"}); !isValidationErr(err) {
			t.Errorf("nome %q deveria dar validation_error, veio %q", bad, codeOf(err))
		}
	}
	// CA-02: tipo fora dos 6 + person/multi_select → validation.
	for _, bad := range []string{"foo", "person", "multi_select"} {
		if _, err := svc.CreateDef(ctx, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "X", Type: bad}); !isValidationErr(err) {
			t.Errorf("type %q deveria dar validation_error, veio %q", bad, codeOf(err))
		}
	}
	// CA-03: select sem options / label duplicado / label vazio → validation.
	if _, err := svc.CreateDef(ctx, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "S", Type: "select"}); !isValidationErr(err) {
		t.Errorf("select sem options deveria dar validation_error, veio %q", codeOf(err))
	}
	if _, err := svc.CreateDef(ctx, s.ws, s.pOwner, "member", s.shared, CreateDefInput{
		Name: "S", Type: "select", Options: []Option{{Label: "A"}, {Label: "A"}},
	}); !isValidationErr(err) {
		t.Errorf("select label duplicado deveria dar validation_error, veio %q", codeOf(err))
	}
}

func TestListDefsPorPosition(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	a := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "A", Type: "text"})
	b := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "B", Type: "text"})
	// CA-04: ordenado por position (a antes de b).
	defs, err := svc.ListDefs(ctx, s.ws, s.mNone, s.shared)
	if err != nil {
		t.Fatalf("ListDefs: %v", err)
	}
	if len(defs) != 2 || defs[0].ID != a.ID || defs[1].ID != b.ID {
		t.Errorf("ordem por position errada: %+v", defs)
	}
	// CA-15: projeto invisível (private de outro) → 404.
	if _, err := svc.ListDefs(ctx, s.ws, s.mNone, s.private); !isNotFound(err) {
		t.Errorf("ListDefs de projeto invisível deveria dar 404, veio %q", codeOf(err))
	}
}

func TestPatchDefNomePositionOptions(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	sel := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{
		Name: "Cliente", Type: "select", Options: []Option{{Label: "Lauda"}},
	})
	existingID := sel.Options[0].ID

	// CA-05: renomeia + reordena + adiciona option (preserva id existente, gera id da nova).
	newName := "Conta"
	newPos := 5
	patched, err := svc.PatchDef(ctx, s.ws, s.pOwner, "member", s.shared, sel.ID, PatchDefInput{
		Name:        &newName,
		PositionSet: true, Position: &newPos,
		OptionsSet: true, Options: []Option{{ID: existingID, Label: "Lauda"}, {Label: "Arkan"}},
	})
	if err != nil {
		t.Fatalf("PatchDef: %v", err)
	}
	if patched.Name != "Conta" || patched.Position != 5 {
		t.Errorf("patch name/position não aplicou: %+v", patched)
	}
	if len(patched.Options) != 2 || patched.Options[0].ID != existingID || patched.Options[1].ID == "" {
		t.Errorf("options: id existente não preservado ou novo sem id: %+v", patched.Options)
	}

	// CA-05/D-04: `type` no payload é ignorado (não erra, não muda o tipo).
	// PatchDefInput não tem campo type → naturalmente imutável. Confirma que o
	// tipo continua select após um patch só de nome.
	again, err := svc.PatchDef(ctx, s.ws, s.pOwner, "member", s.shared, sel.ID, PatchDefInput{Name: ptr("Outro")})
	if err != nil || again.Type != "select" {
		t.Errorf("type deveria seguir select (imutável): %+v err=%v", again, err)
	}
}

func TestDeleteDefCascadeValues(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	def := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Nota", Type: "text"})
	task := mkTask(t, pool, s.ws, s.shared, s.pOwner, "T")
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, def.ID, json.RawMessage(`"oi"`)); err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	// CA-06: deletar a def remove os values (CASCADE) e NÃO apaga a task.
	if err := svc.DeleteDef(ctx, s.ws, s.pOwner, "member", s.shared, def.ID); err != nil {
		t.Fatalf("DeleteDef: %v", err)
	}
	var nVal, nTask int
	pool.QueryRow(ctx, `SELECT count(*) FROM task_property_values WHERE def_id = $1`, def.ID).Scan(&nVal)
	pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE id = $1`, task).Scan(&nTask)
	if nVal != 0 {
		t.Errorf("values deveriam sumir no cascade, sobraram %d", nVal)
	}
	if nTask != 1 {
		t.Errorf("a task NÃO deveria ser apagada (CA-06)")
	}
}

func TestSetValueUpsertEValidacao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	num := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Est", Type: "number"})
	task := mkTask(t, pool, s.ws, s.shared, s.pOwner, "T")

	// CA-08/D-05: upsert — 1º set, 2º set sobrescreve (1 linha só).
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, num.ID, json.RawMessage(`10`)); err != nil {
		t.Fatalf("SetValue 1: %v", err)
	}
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, num.ID, json.RawMessage(`20`)); err != nil {
		t.Fatalf("SetValue 2: %v", err)
	}
	var n int
	var val string
	pool.QueryRow(ctx, `SELECT count(*), max(value::text) FROM task_property_values WHERE task_id=$1 AND def_id=$2`, task, num.ID).Scan(&n, &val)
	if n != 1 || val != "20" {
		t.Errorf("upsert deveria deixar 1 linha com value 20, veio n=%d val=%s", n, val)
	}

	// CA-09: value inválido pro tipo → validation (não escreve).
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, num.ID, json.RawMessage(`"abc"`)); !isValidationErr(err) {
		t.Errorf("number com string deveria dar validation_error, veio %q", codeOf(err))
	}
}

func TestClearValueENull(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	def := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Nota", Type: "text"})
	task := mkTask(t, pool, s.ws, s.shared, s.pOwner, "T")

	// CA-09.1: SetValue com value:null (sem value prévio) → cleared, idempotente.
	_, cleared, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, def.ID, json.RawMessage(`null`))
	if err != nil || !cleared {
		t.Fatalf("SetValue null sem value prévio: cleared=%v err=%v", cleared, err)
	}
	// grava, depois null limpa.
	svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, def.ID, json.RawMessage(`"oi"`))
	_, cleared, err = svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, task, def.ID, json.RawMessage(`null`))
	if err != nil || !cleared {
		t.Fatalf("SetValue null com value: cleared=%v err=%v", cleared, err)
	}
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM task_property_values WHERE task_id=$1 AND def_id=$2`, task, def.ID).Scan(&n)
	if n != 0 {
		t.Errorf("null deveria limpar a linha, sobraram %d", n)
	}
	// CA-10: ClearValue idempotente — sem value também → sem erro.
	if err := svc.ClearValue(ctx, s.ws, s.pOwner, "member", s.shared, task, def.ID); err != nil {
		t.Errorf("ClearValue idempotente deveria ser ok, veio %v", err)
	}
}

func TestEscritaPermissaoEArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	def := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "Nota", Type: "text"})
	task := mkTask(t, pool, s.ws, s.shared, s.pOwner, "T")

	// CA-07: viewer/access=view → 403 em CreateDef/SetValue.
	if _, err := svc.CreateDef(ctx, s.ws, s.mView, "member", s.shared, CreateDefInput{Name: "X", Type: "text"}); !isForbidden(err) {
		t.Errorf("view CreateDef deveria dar 403, veio %q", codeOf(err))
	}
	if _, _, err := svc.SetValue(ctx, s.ws, s.mView, "member", s.shared, task, def.ID, json.RawMessage(`"x"`)); !isForbidden(err) {
		t.Errorf("view SetValue deveria dar 403, veio %q", codeOf(err))
	}
	// CA-11: viewer ClearValue → 403.
	if err := svc.ClearValue(ctx, s.ws, s.mView, "member", s.shared, task, def.ID); !isForbidden(err) {
		t.Errorf("view ClearValue deveria dar 403, veio %q", codeOf(err))
	}

	// CA-11: escrita em projeto ARQUIVADO → 400 invalid_reference (NÃO 'archived').
	archTask := mkTask(t, pool, s.ws, s.archived, s.pOwner, "AT")
	// (a def precisa existir no arquivado pra alcançar o gate de value; cria via SQL pois CreateDef bloqueia arquivado)
	var archDef string
	pool.QueryRow(ctx, `INSERT INTO project_property_defs (project_id, workspace_id, name, type) VALUES ($1,$2,'N','text') RETURNING id`, s.archived, s.ws).Scan(&archDef)
	if _, err := svc.CreateDef(ctx, s.ws, s.pOwner, "member", s.archived, CreateDefInput{Name: "N", Type: "text"}); !isInvalidRef(err) {
		t.Errorf("CreateDef em arquivado deveria dar invalid_reference, veio %q", codeOf(err))
	}
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.archived, archTask, archDef, json.RawMessage(`"x"`)); !isInvalidRef(err) {
		t.Errorf("SetValue em arquivado deveria dar invalid_reference, veio %q", codeOf(err))
	}
	if err := svc.ClearValue(ctx, s.ws, s.pOwner, "member", s.archived, archTask, archDef); !isInvalidRef(err) {
		t.Errorf("ClearValue em arquivado deveria dar invalid_reference, veio %q", codeOf(err))
	}
}

func TestPertencimentoDefTaskProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// projeto B no MESMO workspace.
	var projB string
	pool.QueryRow(ctx, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id) VALUES ($1,'B','#4f9dde','shared',$2) RETURNING id`, s.ws, s.pOwner).Scan(&projB)
	defA := mustCreateDef(t, svc, s.ws, s.pOwner, "member", s.shared, CreateDefInput{Name: "A", Type: "text"})
	defB := mustCreateDef(t, svc, s.ws, s.pOwner, "member", projB, CreateDefInput{Name: "B", Type: "text"})
	taskA := mkTask(t, pool, s.ws, s.shared, s.pOwner, "TA")
	taskB := mkTask(t, pool, s.ws, projB, s.pOwner, "TB")

	// CA-12: def de B alcançada pela rota de A → 404 (mutação que remove project_id morre).
	if _, err := svc.PatchDef(ctx, s.ws, s.pOwner, "member", s.shared, defB.ID, PatchDefInput{Name: ptr("z")}); !isNotFound(err) {
		t.Errorf("PatchDef defB via rota A deveria dar 404, veio %q", codeOf(err))
	}
	if err := svc.DeleteDef(ctx, s.ws, s.pOwner, "member", s.shared, defB.ID); !isNotFound(err) {
		t.Errorf("DeleteDef defB via rota A deveria dar 404, veio %q", codeOf(err))
	}
	// CA-12: task de B + def de A via rota de A → 404 (task não pertence ao projeto da rota).
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, taskB, defA.ID, json.RawMessage(`"x"`)); !isNotFound(err) {
		t.Errorf("SetValue taskB via rota A deveria dar 404, veio %q", codeOf(err))
	}
	// CA-12: def de B + task de A via rota de A → 404 (def não pertence ao projeto da rota).
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, taskA, defB.ID, json.RawMessage(`"x"`)); !isNotFound(err) {
		t.Errorf("SetValue defB via rota A deveria dar 404, veio %q", codeOf(err))
	}
	// CA-12: ids malformados → 404 (não 500).
	if _, err := svc.PatchDef(ctx, s.ws, s.pOwner, "member", s.shared, "nao-uuid", PatchDefInput{Name: ptr("z")}); !isNotFound(err) {
		t.Errorf("defID malformado deveria dar 404, veio %q", codeOf(err))
	}
	if _, _, err := svc.SetValue(ctx, s.ws, s.pOwner, "member", s.shared, "nao-uuid", defA.ID, json.RawMessage(`"x"`)); !isNotFound(err) {
		t.Errorf("taskID malformado deveria dar 404, veio %q", codeOf(err))
	}
}

func isValidationErr(err error) bool { return codeIs(err, "validation_error") }
func ptr(s string) *string           { return &s }
