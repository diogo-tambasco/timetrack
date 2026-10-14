// board_props_test.go — fase 25 T-05: o /board carrega property_defs (schema do
// projeto) + property_values por task, SEM N+1 (CA-16/17), com value de select
// órfão de option OMITIDO (CA-13) e isolamento por workspace+projeto (CA-15).
package objectives

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

func insertDef(t *testing.T, pool *pgxpool.Pool, ws, projectID, name, typ, optionsJSON string, position int) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO project_property_defs (project_id, workspace_id, name, type, options, position)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6) RETURNING id`,
		projectID, ws, name, typ, optionsJSON, position).Scan(&id); err != nil {
		t.Fatalf("insertDef(%s): %v", name, err)
	}
	return id
}

func insertVal(t *testing.T, pool *pgxpool.Pool, ws, taskID, defID, valueJSON string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO task_property_values (task_id, def_id, workspace_id, value)
		VALUES ($1, $2, $3, $4::jsonb)`, taskID, defID, ws, valueJSON); err != nil {
		t.Fatalf("insertVal(%s): %v", defID, err)
	}
}

func taskValue(task BoardTask, defID string) (string, bool) {
	for _, v := range task.PropertyValues {
		if v.DefID == defID {
			return string(v.Value), true
		}
	}
	return "", false
}

func TestBoardTrazPropertyDefsEValues(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// 2 defs no projeto: text (pos 0) + select (pos 1).
	dText := insertDef(t, pool, s.ws, s.shared, "Nota", "text", "[]", 0)
	dSel := insertDef(t, pool, s.ws, s.shared, "Cliente", "select",
		`[{"id":"op_a","label":"Lauda"},{"id":"op_b","label":"Arkan"}]`, 1)

	t1 := mkTask(t, pool, s, nil, "todo")
	t2 := mkTask(t, pool, s, nil, "todo") // sem value (CA-17)
	insertVal(t, pool, s.ws, t1, dText, `"oi"`)
	insertVal(t, pool, s.ws, t1, dSel, `"op_a"`)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	// CA-16: property_defs ordenado por position.
	if len(b.Project.PropertyDefs) != 2 || b.Project.PropertyDefs[0].ID != dText || b.Project.PropertyDefs[1].ID != dSel {
		t.Fatalf("property_defs ordenado errado: %+v", b.Project.PropertyDefs)
	}
	// CA-16: task1 traz os 2 values; task2 traz [] (CA-17).
	var bt1, bt2 *BoardTask
	for i := range b.General.Tasks {
		switch b.General.Tasks[i].ID {
		case t1:
			bt1 = &b.General.Tasks[i]
		case t2:
			bt2 = &b.General.Tasks[i]
		}
	}
	if bt1 == nil || bt2 == nil {
		t.Fatal("tasks não vieram no Geral")
	}
	if v, ok := taskValue(*bt1, dText); !ok || v != `"oi"` {
		t.Errorf("value de text errado: %q ok=%v", v, ok)
	}
	if v, ok := taskValue(*bt1, dSel); !ok || v != `"op_a"` {
		t.Errorf("value de select errado: %q ok=%v", v, ok)
	}
	if len(bt2.PropertyValues) != 0 {
		t.Errorf("task sem value deveria ter property_values [], veio %+v", bt2.PropertyValues)
	}
}

// TestBoardPropertyValuesEmObjetivo (T-14 #3): property_values também chegam nas
// tasks DENTRO de um objetivo (não só no Geral). O attach roda antes do split
// objetivo/Geral; mover o attach pra dentro do ramo do Geral faria a task-em-objetivo
// vir com property_values null — esta cobertura mata essa mutação.
func TestBoardPropertyValuesEmObjetivo(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	o, err := svc.Create(ctx, s.ws, s.pOwner, "member", s.shared, CreateInput{Name: "Obj"})
	if err != nil {
		t.Fatalf("create obj: %v", err)
	}
	dText := insertDef(t, pool, s.ws, s.shared, "Nota", "text", "[]", 0)
	tObj := mkTask(t, pool, s, &o.ID, "todo") // task DENTRO do objetivo
	insertVal(t, pool, s.ws, tObj, dText, `"em-objetivo"`)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	obj := findObj(b, o.ID)
	if obj == nil || len(obj.Tasks) != 1 {
		t.Fatalf("objetivo/task não vieram: %+v", obj)
	}
	if v, ok := taskValue(obj.Tasks[0], dText); !ok || v != `"em-objetivo"` {
		t.Errorf("task em objetivo deveria trazer o property_value, veio %q ok=%v (pv=%+v)", v, ok, obj.Tasks[0].PropertyValues)
	}
}

func TestBoardSelectOrfaoOmitido(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	dSel := insertDef(t, pool, s.ws, s.shared, "Cliente", "select",
		`[{"id":"op_a","label":"Lauda"}]`, 0)
	t1 := mkTask(t, pool, s, nil, "todo")
	insertVal(t, pool, s.ws, t1, dSel, `"op_a"`)

	// remove a option op_a do schema (mantém o value órfão no banco).
	if _, err := pool.Exec(ctx, `UPDATE project_property_defs SET options = '[{"id":"op_b","label":"Arkan"}]'::jsonb WHERE id = $1`, dSel); err != nil {
		t.Fatalf("update options: %v", err)
	}

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	for i := range b.General.Tasks {
		if b.General.Tasks[i].ID != t1 {
			continue
		}
		if _, ok := taskValue(b.General.Tasks[i], dSel); ok {
			t.Errorf("value de select órfão de option deveria ser OMITIDO (CA-13), veio presente")
		}
	}
}

func TestBoardPropsIsolamentoWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	// def legítima no projeto A.
	insertDef(t, pool, s.ws, s.shared, "Nota", "text", "[]", 0)

	// outro workspace + projeto + def (NÃO deve vazar pro board do projeto A — CA-15).
	var wsB, userB, projB string
	pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ('WS B') RETURNING id`).Scan(&wsB)
	pool.QueryRow(ctx, `INSERT INTO users (google_id, email, name) VALUES ('gb','gb@b.c','B') RETURNING id`).Scan(&userB)
	pool.QueryRow(ctx, `INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id) VALUES ($1,'PB','#4f9dde','shared',$2) RETURNING id`, wsB, userB).Scan(&projB)
	insertDef(t, pool, wsB, projB, "LeakDef", "text", "[]", 0)

	b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	// a mutação que remove workspace_id/project_id da query de defs faria "LeakDef" vazar.
	for _, d := range b.Project.PropertyDefs {
		if d.Name == "LeakDef" {
			t.Errorf("def de outro workspace/projeto VAZOU pro board (CA-15): %+v", d)
		}
	}
	if len(b.Project.PropertyDefs) != 1 {
		t.Errorf("board do projeto A deveria ter só a def dele, veio %d", len(b.Project.PropertyDefs))
	}
}

// TestBoardComPropsSob200ms — CA-18 (perf, exceção red→green, igual CA-18 da fase
// 12): 10 defs / 300 tasks / ~3.000 values num projeto; /board < 200ms (melhor de 3).
// Nasce verde; pina o N+1/RNF-02 (defs em 1 query, values em 1 query).
func TestBoardComPropsSob200ms(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	raw, err := os.ReadFile("../../testdata/properties/board_props_seed.sql")
	if err != nil {
		t.Fatalf("ler board_props_seed.sql: %v", err)
	}
	seed := strings.NewReplacer(
		"{{WORKSPACE_ID}}", s.ws,
		"{{USER_ID}}", s.pOwner,
		"{{PROJECT_ID}}", s.shared,
	).Replace(string(raw))
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatalf("aplicar seed: %v", err)
	}

	// warm-up.
	if _, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared); err != nil {
		t.Fatalf("board warm-up: %v", err)
	}
	best := time.Hour
	for i := 0; i < 3; i++ {
		start := time.Now()
		b, err := svc.Board(ctx, s.ws, s.pOwner, "member", s.shared)
		if err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(b.Project.PropertyDefs) != 10 {
			t.Fatalf("esperava 10 defs no board, veio %d", len(b.Project.PropertyDefs))
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 200*time.Millisecond {
		t.Errorf("board (10 defs/300 tasks/3000 values): melhor de 3 = %v (> 200ms — N+1? RNF-02)", best)
	}
	t.Logf("board com props melhor de 3: %v", best)
}
