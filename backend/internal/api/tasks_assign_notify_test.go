package api_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/workspace"
)

type assignCall struct{ target, taskID, by string }

// newCoreRouterWithAssignSpy monta o router com um spy de OnAssign (fase 18 T-05).
func newCoreRouterWithAssignSpy(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server, *[]assignCall, *sync.Mutex) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	var mu sync.Mutex
	calls := []assignCall{}
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasks.NewService(pool),
			Objectives: objectives.NewService(pool),
			OnAssign: func(target, taskID, by string) {
				mu.Lock()
				calls = append(calls, assignCall{target, taskID, by})
				mu.Unlock()
			},
		}),
	)
	return router, svc, stub, &calls, &mu
}

// Fase 18 T-05 — CA-13/14/15: notifica SÓ em set-por-terceiro e em mudança real.
func TestAssignNotificaSoEmMudancaRealPorTerceiro(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, calls, mu := newCoreRouterWithAssignSpy(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	loginVia(t, svc, stub, "g-b", "b@example.com", "Beto Membro")
	ws := workspaceIDOf(t, pool, "g-a")
	idA := userIDOf(t, pool, "g-a")
	idB := userIDOf(t, pool, "g-b")
	addMember(t, pool, ws, idB, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	base := "/api/v1/workspaces/" + ws + "/tasks"

	count := func() int { mu.Lock(); defer mu.Unlock(); return len(*calls) }

	// CA-13: A cria task atribuindo a B (terceiro) → 1 notificação pro B
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Delegada","assignee":"`+idB+`"}`)
	if count() != 1 {
		t.Fatalf("CA-13: criar atribuindo a B deveria notificar 1×, veio %d", count())
	}
	mu.Lock()
	last := (*calls)[0]
	mu.Unlock()
	if last.target != idB || last.taskID != tk.ID || last.by != idA {
		t.Errorf("payload da notificação errado: %+v (esperado target=B task=%s by=A)", last, tk.ID)
	}

	// CA-14: A atribui task a SI MESMO → NÃO notifica
	tk2 := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Minha","assignee":"`+idA+`"}`)
	if count() != 1 {
		t.Errorf("CA-14: auto-set não deveria notificar, total veio %d", count())
	}

	// CA-15: PATCH reenviando o MESMO assignee (B) na tk → NÃO re-notifica
	doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookieA, `{"assignee":"`+idB+`"}`)
	if count() != 1 {
		t.Errorf("CA-15: reenviar o mesmo assignee não deveria notificar, total %d", count())
	}

	// CA-15: PATCH mudando SÓ notes (assignee intocado) → NÃO notifica
	doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookieA, `{"notes":"v2"}`)
	if count() != 1 {
		t.Errorf("CA-15: mudar só notes não deveria notificar, total %d", count())
	}

	// CA-13: PATCH reatribuindo a tk2 (era A) → B (terceiro) → notifica
	doJSON(router, http.MethodPatch, base+"/"+tk2.ID, cookieA, `{"assignee":"`+idB+`"}`)
	if count() != 2 {
		t.Errorf("CA-13: reatribuir de A→B deveria notificar, total %d", count())
	}

	// CA-15: PATCH limpando o assignee (B → null) → NÃO notifica (new é nil)
	doJSON(router, http.MethodPatch, base+"/"+tk.ID, cookieA, `{"assignee":null}`)
	if count() != 2 {
		t.Errorf("limpar assignee não deveria notificar (new=nil), total %d", count())
	}
}
