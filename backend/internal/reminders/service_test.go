// service_test.go — TDD do gate do POST manual (fase 20, verificação do build).
// Cobre o isolamento multi-tenant do CreateManual: o gate filtra t.workspace_id
// = o ws da ROTA, não qualquer ws onde o user é membro.
package reminders_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/reminders"
)

// (verificação do build) CA-22: o gate do POST manual filtra t.workspace_id = o
// ws da ROTA. Sem este teste a mutação que remove esse filtro sobrevive — um
// membro de DOIS workspaces criaria um lembrete via a rota do ws A pra uma task
// que vive no ws B (cross-tenant). Aqui uA é membro de A e de B; criar lembrete
// via a rota de A pra uma task de B deve dar not_found (a task não é do ws da rota).
func TestCreateManualIsolaWorkspace(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := reminders.NewService(reminders.NewStore(pool))
	ctx := context.Background()

	uA, wsA, _ := seedTask(t, pool, "g-iso", "Tarefa do A")

	// wsB: o MESMO user uA é membro (owner), com um projeto shared e uma task.
	var wsB, projB, taskB string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ('WS B') RETURNING id`).Scan(&wsB); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, wsB, uA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, 'P', '#112233', 'shared') RETURNING id`, wsB, uA).Scan(&projB); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (workspace_id, project_id, name, created_by)
		 VALUES ($1, $2, 'Task do B', $3) RETURNING id`, wsB, projB, uA).Scan(&taskB); err != nil {
		t.Fatal(err)
	}

	// criar lembrete via a rota do ws A, mas pra a task do ws B → not_found
	// (o gate filtra t.workspace_id = wsA; taskB é do wsB — não casa).
	_, err := svc.CreateManual(ctx, wsA, uA, "owner", taskB, time.Now().UTC())
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "not_found" {
		t.Fatalf("CreateManual cross-ws (rota A, task B) deveria dar not_found, veio %v", err)
	}
	if n := dbtest.CountRows(t, pool, "reminders"); n != 0 {
		t.Errorf("nenhum reminder deveria ter sido criado cross-ws, há %d", n)
	}
}
