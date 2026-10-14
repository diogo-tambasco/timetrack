package db

import (
	"context"
	"testing"
)

// Fase 26 T-01 — TestMigracaoNotes: a migração 0024 cria as tabelas notes e
// note_links com colunas/defaults declarados (CA-01/06/14), os FKs com a
// política de delete correta (project CASCADE, task SET NULL, note CASCADE),
// o CHECK do ref_type e o índice único uq_note_links (D-08).
func TestMigracaoNotes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam: %v", err)
	}

	// colunas + nullability de notes.
	for _, c := range []struct {
		name, dataType, nullable string
	}{
		{"id", "uuid", "NO"},
		{"workspace_id", "uuid", "NO"},
		{"project_id", "uuid", "YES"},
		{"task_id", "uuid", "YES"},
		{"title", "text", "NO"},
		{"body_md", "text", "NO"},
		{"created_by", "uuid", "NO"},
		{"created_at", "timestamp with time zone", "NO"},
		{"updated_at", "timestamp with time zone", "NO"},
	} {
		var dataType, isNullable string
		if err := pool.QueryRow(ctx,
			`SELECT data_type, is_nullable FROM information_schema.columns
			  WHERE table_name = 'notes' AND table_schema = current_schema() AND column_name = $1`, c.name).Scan(&dataType, &isNullable); err != nil {
			t.Fatalf("notes.%s: %v", c.name, err)
		}
		if dataType != c.dataType {
			t.Errorf("notes.%s deveria ser %s, veio %q", c.name, c.dataType, dataType)
		}
		if isNullable != c.nullable {
			t.Errorf("notes.%s deveria ter is_nullable=%q, veio %q", c.name, c.nullable, isNullable)
		}
	}

	// colunas de note_links.
	for _, c := range []struct {
		name, dataType, nullable string
	}{
		{"id", "uuid", "NO"},
		{"note_id", "uuid", "NO"},
		{"ref_type", "text", "NO"},
		{"ref_id", "uuid", "NO"},
		{"position", "integer", "NO"},
		{"workspace_id", "uuid", "NO"},
	} {
		var dataType, isNullable string
		if err := pool.QueryRow(ctx,
			`SELECT data_type, is_nullable FROM information_schema.columns
			  WHERE table_name = 'note_links' AND table_schema = current_schema() AND column_name = $1`, c.name).Scan(&dataType, &isNullable); err != nil {
			t.Fatalf("note_links.%s: %v", c.name, err)
		}
		if dataType != c.dataType {
			t.Errorf("note_links.%s deveria ser %s, veio %q", c.name, c.dataType, dataType)
		}
		if isNullable != c.nullable {
			t.Errorf("note_links.%s deveria ter is_nullable=%q, veio %q", c.name, c.nullable, isNullable)
		}
	}

	// índices presentes (resolvidos pela relação do schema corrente — robusto a
	// schemas de teste irmãos dropados em paralelo).
	for _, idx := range []struct{ rel, name string }{
		{"notes", "idx_notes_project_updated"},
		{"notes", "idx_notes_loose"},
		{"note_links", "idx_note_links_note"},
		{"note_links", "uq_note_links"},
	} {
		var oid string
		if err := pool.QueryRow(ctx, `
			SELECT i.indexrelid::regclass::text
			  FROM pg_index i
			 WHERE i.indrelid = $1::regclass
			   AND i.indexrelid = to_regclass($2)`, idx.rel, idx.name).Scan(&oid); err != nil {
			t.Fatalf("índice %s não encontrado em %s: %v", idx.name, idx.rel, err)
		}
	}

	// fixtures: user/ws/project/task pra exercitar defaults, FKs e cascades.
	var userID, wsID, projID, taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-note', 'note@b.c', 'Note') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS Note') RETURNING id`).Scan(&wsID); err != nil {
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

	// defaults: title='', body_md='', created_at/updated_at preenchidos.
	var noteID, title, body string
	if err := pool.QueryRow(ctx,
		`INSERT INTO notes (workspace_id, project_id, task_id, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id, title, body_md`,
		wsID, projID, taskID, userID).Scan(&noteID, &title, &body); err != nil {
		t.Fatalf("insert note com defaults: %v", err)
	}
	if title != "" || body != "" {
		t.Errorf("defaults deveriam ser title='' body_md='', veio title=%q body=%q", title, body)
	}

	// default de note_links.position = 0.
	var pos int
	if err := pool.QueryRow(ctx,
		`INSERT INTO note_links (note_id, ref_type, ref_id, workspace_id)
		 VALUES ($1, 'task', $2, $3) RETURNING position`,
		noteID, taskID, wsID).Scan(&pos); err != nil {
		t.Fatalf("insert note_link com default position: %v", err)
	}
	if pos != 0 {
		t.Errorf("position default deveria ser 0, veio %d", pos)
	}

	// CHECK do ref_type: valor fora de {task,note} é rejeitado.
	if _, err := pool.Exec(ctx,
		`INSERT INTO note_links (note_id, ref_type, ref_id, workspace_id)
		 VALUES ($1, 'goal', $2, $3)`, noteID, taskID, wsID); err == nil {
		t.Errorf("ref_type fora de {task,note} deveria violar o CHECK")
	}

	// uq_note_links: link DUPLICADO da mesma (note_id, ref_type, ref_id) é
	// rejeitado pelo índice único — único teste do uq (26-H8: a colisão é
	// inalcançável pela API; aqui é integridade de schema via INSERT direto).
	if _, err := pool.Exec(ctx,
		`INSERT INTO note_links (note_id, ref_type, ref_id, workspace_id)
		 VALUES ($1, 'task', $2, $3)`, noteID, taskID, wsID); err == nil {
		t.Errorf("link duplicado (note_id, ref_type, ref_id) deveria violar uq_note_links")
	}

	// ON DELETE CASCADE da note_id: apagar a nota apaga seus links (não a task).
	if _, err := pool.Exec(ctx, `DELETE FROM notes WHERE id = $1`, noteID); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	var nLinks, nTasks int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM note_links WHERE note_id = $1`, noteID).Scan(&nLinks); err != nil {
		t.Fatalf("contar links após delete da nota: %v", err)
	}
	if nLinks != 0 {
		t.Errorf("CASCADE da note_id deveria apagar os links da nota, sobraram %d", nLinks)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE id = $1`, taskID).Scan(&nTasks); err != nil {
		t.Fatalf("contar task após delete da nota: %v", err)
	}
	if nTasks != 1 {
		t.Errorf("apagar a nota NÃO deveria apagar a task ligada, restaram %d", nTasks)
	}

	// notes.task_id ON DELETE SET NULL: apagar a task zera task_id da nota.
	var note2 string
	if err := pool.QueryRow(ctx,
		`INSERT INTO notes (workspace_id, project_id, task_id, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`, wsID, projID, taskID, userID).Scan(&note2); err != nil {
		t.Fatalf("insert note 2: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		t.Fatalf("delete task (SET NULL): %v", err)
	}
	var taskRef *string
	if err := pool.QueryRow(ctx, `SELECT task_id FROM notes WHERE id = $1`, note2).Scan(&taskRef); err != nil {
		t.Fatalf("ler task_id após delete da task: %v", err)
	}
	if taskRef != nil {
		t.Errorf("task_id deveria virar NULL ao apagar a task (SET NULL), veio %v", *taskRef)
	}

	// notes.project_id ON DELETE CASCADE: apagar o projeto apaga as notas DO
	// projeto, mas NÃO a nota solta (project_id NULL).
	var noteLoose string
	if err := pool.QueryRow(ctx,
		`INSERT INTO notes (workspace_id, project_id, created_by)
		 VALUES ($1, NULL, $2) RETURNING id`, wsID, userID).Scan(&noteLoose); err != nil {
		t.Fatalf("insert nota solta: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projID); err != nil {
		t.Fatalf("delete project (CASCADE): %v", err)
	}
	var nProjNotes, nLoose int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notes WHERE project_id = $1`, projID).Scan(&nProjNotes); err != nil {
		t.Fatalf("contar notas do projeto após cascade: %v", err)
	}
	if nProjNotes != 0 {
		t.Errorf("CASCADE da project_id deveria apagar as notas do projeto, sobraram %d", nProjNotes)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notes WHERE id = $1`, noteLoose).Scan(&nLoose); err != nil {
		t.Fatalf("contar nota solta após cascade do projeto: %v", err)
	}
	if nLoose != 1 {
		t.Errorf("nota solta NÃO deveria ser afetada pelo delete do projeto, sobraram %d", nLoose)
	}
}
