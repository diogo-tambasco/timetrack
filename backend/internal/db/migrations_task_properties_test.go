package db

import (
	"context"
	"strings"
	"testing"
)

// Fase 25 T-01 — TestMigracaoTaskProperties: a migração 0023 cria
// project_property_defs (o SCHEMA de propriedades custom do projeto) +
// task_property_values (o VALOR de uma def numa task, jsonb tipado).
//   - project_property_defs: type CHECK com os 8 tipos (schema comporta
//     person/multi_select; o SERVICE da v1 é quem barra os 2 — CA-02);
//     options default '[]'; position default 0; índice (project_id, position);
//     FK project_id ON DELETE CASCADE.
//   - task_property_values: PK (task_id, def_id); 2 FKs ON DELETE CASCADE
//     (task→tasks, def→defs); value jsonb NOT NULL; índice (def_id).
//   - deletar def → CASCADE limpa só os values daquela def, NÃO apaga a task
//     (CA-06); deletar task → CASCADE limpa os values dela (CA-08); PK impede
//     2ª linha pro mesmo par (upsert — D-05).
func TestMigracaoTaskProperties(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// tabelas existem.
	for _, tbl := range []string{"project_property_defs", "task_property_values"} {
		var ok bool
		if err := pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", tbl).Scan(&ok); err != nil || !ok {
			t.Fatalf("tabela %s não existe (err=%v)", tbl, err)
		}
	}

	// seed: user + ws + project + 2 tasks.
	var userID, wsID, projID, taskID, taskID2 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-prop', 'prop@b.c', 'Prop') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Prop') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T1', $3) RETURNING id`, wsID, projID, userID).Scan(&taskID); err != nil {
		t.Fatalf("insert task1: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T2', $3) RETURNING id`, wsID, projID, userID).Scan(&taskID2); err != nil {
		t.Fatalf("insert task2: %v", err)
	}

	// def: defaults options='[]', position=0.
	var defID string
	var options string
	var position int
	if err := pool.QueryRow(ctx,
		`INSERT INTO project_property_defs (project_id, workspace_id, name, type)
		 VALUES ($1, $2, 'Cliente', 'text')
		 RETURNING id, options::text, position`,
		projID, wsID).Scan(&defID, &options, &position); err != nil {
		t.Fatalf("insert def com defaults: %v", err)
	}
	if strings.TrimSpace(options) != "[]" {
		t.Errorf("options default deveria ser '[]', veio %q", options)
	}
	if position != 0 {
		t.Errorf("position default deveria ser 0, veio %d", position)
	}

	// CHECK do type: aceita os 8 tipos (schema comporta a fatia 25.1), barra 'foo'.
	for _, ty := range []string{"text", "number", "select", "multi_select", "date", "person", "checkbox", "url"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO project_property_defs (project_id, workspace_id, name, type) VALUES ($1, $2, $3, $4)`,
			projID, wsID, "d-"+ty, ty); err != nil {
			t.Errorf("CHECK deveria aceitar type=%q (service barra person/multi_select — CA-02): %v", ty, err)
		}
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO project_property_defs (project_id, workspace_id, name, type) VALUES ($1, $2, 'x', 'foo')`,
		projID, wsID); err == nil {
		t.Error("CHECK deveria barrar type='foo'")
	}

	// value jsonb NOT NULL.
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_property_values (task_id, def_id, workspace_id, value) VALUES ($1, $2, $3, '"abc"'::jsonb)`,
		taskID, defID, wsID); err != nil {
		t.Fatalf("insert value: %v", err)
	}
	// PK (task_id, def_id): 2ª linha pro mesmo par → erro (upsert é a escrita normal — D-05).
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_property_values (task_id, def_id, workspace_id, value) VALUES ($1, $2, $3, '"dup"'::jsonb)`,
		taskID, defID, wsID); err == nil {
		t.Error("PK (task_id, def_id) deveria impedir 2ª linha pro mesmo par")
	}
	// value NOT NULL.
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_property_values (task_id, def_id, workspace_id, value) VALUES ($1, $2, $3, NULL)`,
		taskID2, defID, wsID); err == nil {
		t.Error("value deveria ser NOT NULL")
	}

	// índices existem.
	for _, idx := range []string{"idx_prop_defs_project", "idx_prop_values_def"} {
		if err := pool.QueryRow(ctx, `
			SELECT 1 FROM pg_class WHERE relkind = 'i' AND relname = $1`, idx).Scan(new(int)); err != nil {
			t.Fatalf("índice %s não encontrado: %v", idx, err)
		}
	}

	// CA-08: deletar a TASK → CASCADE limpa os values dela.
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("delete task (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM task_property_values WHERE task_id = $1`, taskID).Scan(&n); err != nil {
		t.Fatalf("contar values após delete task: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE da task deveria limpar os values, sobraram %d", n)
	}

	// CA-06: deletar a DEF → CASCADE limpa os values dela, NÃO apaga a task.
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_property_values (task_id, def_id, workspace_id, value) VALUES ($1, $2, $3, '"v2"'::jsonb)`,
		taskID2, defID, wsID); err != nil {
		t.Fatalf("insert value task2: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM project_property_defs WHERE id = $1`, defID); err != nil {
		t.Fatalf("delete def (cascade): %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM task_property_values WHERE def_id = $1`, defID).Scan(&n); err != nil {
		t.Fatalf("contar values após delete def: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE da def deveria limpar os values, sobraram %d", n)
	}
	// a task 2 sobrevive.
	var taskAlive bool
	if err := pool.QueryRow(ctx, `SELECT exists(SELECT 1 FROM tasks WHERE id = $1)`, taskID2).Scan(&taskAlive); err != nil {
		t.Fatalf("checar task viva: %v", err)
	}
	if !taskAlive {
		t.Error("deletar a def NÃO deveria apagar a task (CA-06)")
	}

	// FK project_id ON DELETE CASCADE: apagar o projeto leva o schema junto.
	// (apaga a task remanescente antes — tasks.project_id NÃO cascateia, é o
	// project_property_defs.project_id que cascateia, é o que estamos provando.)
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID2); err != nil {
		t.Fatalf("delete task2: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projID); err != nil {
		t.Fatalf("delete project (cascade): %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM project_property_defs WHERE project_id = $1`, projID).Scan(&n); err != nil {
		t.Fatalf("contar defs após delete project: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE do projeto deveria limpar as defs, sobraram %d", n)
	}
}
