package db

import (
	"context"
	"testing"
)

// Fase 19 T-01 — TestMigracaoTaskComments: a migração 0019 cria a tabela
// task_comments (thread de comentários por task) com as colunas e nullability
// declaradas (CA-01), FK task_id ON DELETE CASCADE (CA-01.1) e o índice
// (task_id, created_at) pra varredura ordenada da thread (RNF-01).
func TestMigracaoTaskComments(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// colunas + nullability declaradas (CA-01).
	for _, c := range []struct {
		name     string
		dataType string
		nullable string
	}{
		{"id", "uuid", "NO"},
		{"task_id", "uuid", "NO"},
		{"workspace_id", "uuid", "NO"},
		{"author_id", "uuid", "NO"},
		{"body", "text", "NO"},
		{"created_at", "timestamp with time zone", "NO"},
	} {
		var dataType, isNullable string
		if err := pool.QueryRow(ctx,
			`SELECT data_type, is_nullable FROM information_schema.columns
			  WHERE table_name = 'task_comments' AND table_schema = current_schema() AND column_name = $1`, c.name).Scan(&dataType, &isNullable); err != nil {
			t.Fatalf("coluna %s: %v", c.name, err)
		}
		if dataType != c.dataType {
			t.Errorf("coluna %s deveria ser %s, veio %q", c.name, c.dataType, dataType)
		}
		if isNullable != c.nullable {
			t.Errorf("coluna %s deveria ter is_nullable=%q, veio %q", c.name, c.nullable, isNullable)
		}
	}

	// índice (task_id, created_at) presente. Consultamos pelo OID resolvido a
	// partir da relação task_comments DO SCHEMA CORRENTE (search_path), em vez do
	// view pg_indexes — que varre TODOS os schemas e arrisca "could not open
	// relation with OID" quando um schema de teste irmão é dropado em paralelo.
	var idxOID string
	if err := pool.QueryRow(ctx, `
		SELECT i.indexrelid::regclass::text
		  FROM pg_index i
		 WHERE i.indrelid = 'task_comments'::regclass
		   AND i.indexrelid = to_regclass('idx_task_comments_task')`).Scan(&idxOID); err != nil {
		t.Fatalf("índice idx_task_comments_task não encontrado no schema corrente: %v", err)
	}

	// CA-01.1 — FK task_id é ON DELETE CASCADE: criar task + 2 comentários,
	// apagar a task, contar 0 comentários daquela task_id.
	var userID, wsID, projID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-cmt', 'cmt@b.c', 'Cmt') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Cmt') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3) RETURNING id`, wsID, projID, userID).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx,
			`INSERT INTO task_comments (task_id, workspace_id, author_id, body)
			 VALUES ($1, $2, $3, 'oi')`, taskID, wsID, userID); err != nil {
			t.Fatalf("insert comment: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("delete task (cascade): %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM task_comments WHERE task_id = $1`, taskID).Scan(&n); err != nil {
		t.Fatalf("contar comentários após cascade: %v", err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE deveria remover os comentários da task removida, sobraram %d", n)
	}

	// body NOT NULL: insert sem body é rejeitado.
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T2', $3) RETURNING id`, wsID, projID, userID).Scan(&taskID); err != nil {
		t.Fatalf("insert task 2: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_comments (task_id, workspace_id, author_id, body)
		 VALUES ($1, $2, $3, NULL)`, taskID, wsID, userID); err == nil {
		t.Errorf("body NULL deveria violar o NOT NULL")
	}

	// FK author_id a users: apagar o user referenciado deve falhar.
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_comments (task_id, workspace_id, author_id, body)
		 VALUES ($1, $2, $3, 'tem autor')`, taskID, wsID, userID); err != nil {
		t.Fatalf("insert comment com author válido: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err == nil {
		t.Errorf("apagar user referenciado por author_id deveria violar a FK")
	}
}
