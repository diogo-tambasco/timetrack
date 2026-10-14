package db

import (
	"context"
	"testing"
)

// Fase 24 T-01 — TestMigracaoTasksV2: a migração 0021 estende tasks com o status
// 'backlog' (BEFORE todo), priority (0..3, default 0, CHECK), notion_url (texto
// nullable) e position (default 0), + índice (project_id, status, position).
// ⚠️ A migração roda sem erro de tx (prova que ADD VALUE 'backlog' não foi usado
// no mesmo arquivo — PG17/PRD §9.3) e o DEFAULT de status CONTINUA 'todo'.
func TestMigracaoTasksV2(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Se a migração tentasse usar 'backlog' no mesmo arquivo, isto falharia.
	if err := Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrações reais falharam (0021 deve aplicar limpa): %v", err)
	}

	// enum task_status tem os 4 valores na ordem backlog,todo,doing,done (CA-01).
	// Resolve o OID do tipo pelo search_path corrente ('task_status'::regtype) pra não
	// pegar os enums de schemas irmãos do db-test (search_path isola por teste).
	rows, err := pool.Query(ctx, `
		SELECT e.enumlabel
		  FROM pg_enum e
		 WHERE e.enumtypid = 'task_status'::regtype
		 ORDER BY e.enumsortorder`)
	if err != nil {
		t.Fatalf("ler enum task_status: %v", err)
	}
	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			rows.Close()
			t.Fatalf("scan label: %v", err)
		}
		labels = append(labels, l)
	}
	rows.Close()
	want := []string{"backlog", "todo", "doing", "done"}
	if len(labels) != len(want) {
		t.Fatalf("enum task_status deveria ter %v, veio %v", want, labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("enum task_status[%d] = %q, esperado %q (ordem importa — backlog antes de todo)", i, labels[i], want[i])
		}
	}

	// seed pra exercitar os defaults
	var userID, wsID, projID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-v2', 'v2@b.c', 'V2') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS V2') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("insert ws: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsID, userID).Scan(&projID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// DEFAULT de status CONTINUA 'todo' (pina o furo "backlog vira default"); priority
	// default 0; position default 0; notion_url nullable (CA-01/02).
	var status string
	var priority, position int
	var notionNull bool
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'T', $3)
		 RETURNING status, priority, position, notion_url IS NULL`,
		wsID, projID, userID).Scan(&status, &priority, &position, &notionNull); err != nil {
		t.Fatalf("insert task com defaults: %v", err)
	}
	if status != "todo" {
		t.Errorf("status default deveria ser 'todo' (não 'backlog'), veio %q", status)
	}
	if priority != 0 {
		t.Errorf("priority default deveria ser 0, veio %d", priority)
	}
	if position != 0 {
		t.Errorf("position default deveria ser 0, veio %d", position)
	}
	if !notionNull {
		t.Errorf("notion_url deveria nascer NULL")
	}

	// CHECK de priority barra -1 e 4 (backstop — CA-02.1).
	if _, err := pool.Exec(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, priority)
		 VALUES ($1, $2, 'neg', $3, -1)`, wsID, projID, userID); err == nil {
		t.Error("priority -1 deveria violar o CHECK (0..3)")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, priority)
		 VALUES ($1, $2, 'big', $3, 4)`, wsID, projID, userID); err == nil {
		t.Error("priority 4 deveria violar o CHECK (0..3)")
	}

	// status 'backlog' agora é aceito no banco.
	if _, err := pool.Exec(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by, status)
		 VALUES ($1, $2, 'bl', $3, 'backlog')`, wsID, projID, userID); err != nil {
		t.Errorf("status 'backlog' deveria ser aceito após a 0021: %v", err)
	}

	// índice idx_tasks_project_status_position existe.
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM pg_index i
		 WHERE i.indrelid = 'tasks'::regclass
		   AND i.indexrelid = to_regclass('idx_tasks_project_status_position')`).Scan(new(int)); err != nil {
		t.Fatalf("índice idx_tasks_project_status_position não encontrado: %v", err)
	}
}
