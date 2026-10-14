package db

import (
	"context"
	"testing"
)

// Fase 18 T-01 — TestMigracaoTaskAssignment: a migração 0018 cria SÓ os índices
// idx_tasks_assignee e idx_tasks_requested_by (parciais, WHERE ... IS NOT NULL)
// pra a varredura cross-workspace das visões "Minhas tasks"/"Pedidas por mim"
// (CA-01). NENHUMA coluna nova: assignee/requested_by já existem em 0008
// (uuid nullable, FK a users) e seguem assim (não regrediram).
func TestMigracaoTaskAssignment(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// os 2 índices da fase 18 existem E são PARCIAIS. Consultamos pg_index
	// pelo OID do índice resolvido a partir da relação tasks DO SCHEMA CORRENTE
	// (search_path), em vez do view pg_indexes — que varre TODOS os schemas e
	// resolve cada OID via pg_get_indexdef (corre risco de "could not open
	// relation with OID" quando um schema de teste irmão é dropado em paralelo).
	for _, idx := range []string{"idx_tasks_assignee", "idx_tasks_requested_by"} {
		var partial bool
		err := pool.QueryRow(ctx, `
			SELECT i.indpred IS NOT NULL
			  FROM pg_index i
			 WHERE i.indrelid = 'tasks'::regclass
			   AND i.indexrelid = to_regclass($1)`, idx).Scan(&partial)
		if err != nil {
			t.Fatalf("índice %s não encontrado no schema corrente: %v", idx, err)
		}
		if !partial {
			t.Errorf("índice %s deveria ser PARCIAL (WHERE ... IS NOT NULL)", idx)
		}
	}

	// as colunas assignee/requested_by seguem uuid-nullable FK a users (não regrediram).
	for _, col := range []string{"assignee", "requested_by"} {
		var dataType, isNullable string
		if err := pool.QueryRow(ctx,
			`SELECT data_type, is_nullable FROM information_schema.columns
			  WHERE table_name = 'tasks' AND table_schema = current_schema() AND column_name = $1`, col).Scan(&dataType, &isNullable); err != nil {
			t.Fatalf("coluna %s: %v", col, err)
		}
		if dataType != "uuid" {
			t.Errorf("%s deveria ser uuid, veio %q", col, dataType)
		}
		if isNullable != "YES" {
			t.Errorf("%s deveria ser nullable, veio is_nullable=%q", col, isNullable)
		}
	}

	// FK a users segue existindo (apagar user referenciado por assignee deve falhar).
	var userID, wsID, projID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-asg', 'asg@b.c', 'Asg') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Asg') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, assignee)
		 VALUES ($1, $2, 'T', $3, $3)`, wsID, projID, userID); err != nil {
		t.Fatalf("insert task com assignee válido: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err == nil {
		t.Errorf("apagar user referenciado por assignee/created_by deveria violar a FK")
	}
}
