package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/api"
	"timetrack.live/backend/internal/auth"
	"timetrack.live/backend/internal/auth/googlestub"
	"timetrack.live/backend/internal/clients"
	"timetrack.live/backend/internal/comments"
	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/objectives"
	"timetrack.live/backend/internal/projects"
	"timetrack.live/backend/internal/tasks"
	"timetrack.live/backend/internal/workspace"
)

type commentJSON struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	WorkspaceID string `json:"workspace_id"`
	AuthorID    string `json:"author_id"`
	AuthorName  string `json:"author_name"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
}

// newCommentsRouter monta o router REAL com auth (01) + core (02) + comments
// (19) — exercita o wiring do subrouter IRMÃO da fase 19.
func newCommentsRouter(t *testing.T, pool *pgxpool.Pool) (http.Handler, *auth.Service, *googlestub.Server) {
	t.Helper()
	svc, stub := newAuthService(t, pool)
	router := api.NewRouter(
		api.WithAuth(svc, workspace.NewStore(pool)),
		api.WithCore(api.Core{
			Clients:    clients.NewService(pool),
			Projects:   projects.NewService(pool),
			Tasks:      tasks.NewService(pool),
			Objectives: objectives.NewService(pool),
		}),
		api.WithComments(api.Comments{Svc: comments.NewService(pool)}),
	)
	return router, svc, stub
}

func commentsPath(ws, task string) string {
	return "/api/v1/workspaces/" + ws + "/tasks/" + task + "/comments"
}

// criarComment dispara o POST e exige 201.
func criarComment(t *testing.T, router http.Handler, ws, task string, cookie *http.Cookie, body string) commentJSON {
	t.Helper()
	rec := doJSON(router, http.MethodPost, commentsPath(ws, task), cookie, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar comentário: esperado 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	var cm commentJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &cm); err != nil {
		t.Fatalf("corpo fora do contrato: %v", err)
	}
	return cm
}

func listComments(t *testing.T, router http.Handler, ws, task string, cookie *http.Cookie) []commentJSON {
	t.Helper()
	rec := doJSON(router, http.MethodGet, commentsPath(ws, task), cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET comentários: esperado 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Comments []commentJSON `json:"comments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Comments
}

// TestRoteamentoComentarios — smoke do wiring (plan §3 D-02): a construção do
// router NÃO panica e a rota da thread roteia corretamente (GET 200 com lista
// vazia numa task visível), provando que o subrouter IRMÃO não colide com o
// /workspaces/{workspaceId} (chi Mount sob wildcard).
func TestRoteamentoComentarios(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")

	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Task"}`)

	list := listComments(t, router, ws, tk.ID, cookie)
	if list == nil {
		t.Fatal("GET de thread vazia deveria devolver [] não-nil")
	}
	if len(list) != 0 {
		t.Fatalf("thread nova deveria estar vazia, veio %d", len(list))
	}

	// E o /workspaces/{id}/tasks/{id} da fase 02 (rota IRMÃ) segue roteando: GET
	// da task individual responde 200 (não foi engolido pelo subrouter de
	// comments).
	rec := doJSON(router, http.MethodGet, "/api/v1/workspaces/"+ws+"/tasks/"+tk.ID, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks/{id} (fase 02) deveria seguir roteando: veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-02/06 — POST válido 201 + persiste; GET lista ASC; o objeto traz
// author_name e body trimado.
func TestComentariosCriarEListar(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	idA := userIDOf(t, pool, "g-a")

	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Task"}`)

	c1 := criarComment(t, router, ws, tk.ID, cookie, `{"body":"  primeiro  "}`)
	if c1.AuthorID != idA || c1.AuthorName != "Ana Owner" || c1.Body != "primeiro" ||
		c1.WorkspaceID != ws || c1.TaskID != tk.ID || c1.CreatedAt == "" {
		t.Errorf("comentário fora do contrato: %+v", c1)
	}
	criarComment(t, router, ws, tk.ID, cookie, `{"body":"segundo"}`)

	list := listComments(t, router, ws, tk.ID, cookie)
	if len(list) != 2 || list[0].Body != "primeiro" || list[1].Body != "segundo" {
		t.Fatalf("lista deveria vir ASC [primeiro, segundo], veio %+v", list)
	}
}

// CA-03 — body vazio/ausente → 400 validation_error.
func TestComentarioBodyInvalido400(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Task"}`)

	for _, b := range []string{`{"body":""}`, `{"body":"   "}`, `{}`} {
		rec := doJSON(router, http.MethodPost, commentsPath(ws, tk.ID), cookie, b)
		if rec.Code != http.StatusBadRequest || errCode(t, rec) != "validation_error" {
			t.Errorf("POST %s: esperado 400 validation_error, veio %d/%s", b, rec.Code, rec.Body.String())
		}
	}
}

// CA-07 — autor apaga o próprio (204); outro membro apaga o de outro → 403;
// id inexistente → 404.
func TestComentarioDeleteAutorOutro(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM := userIDOf(t, pool, "g-m")
	addMember(t, pool, ws, idM, "member")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Task"}`)
	c := criarComment(t, router, ws, tk.ID, cookieM, `{"body":"do membro"}`)

	// outro (owner Ana) NÃO é autor mas é OWNER → 204 (CA-08).
	rec := doJSON(router, http.MethodDelete, commentsPath(ws, tk.ID)+"/"+c.ID, cookieA, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("owner deveria apagar qualquer (204), veio %d: %s", rec.Code, rec.Body.String())
	}
	// 2º DELETE → 404.
	rec = doJSON(router, http.MethodDelete, commentsPath(ws, tk.ID)+"/"+c.ID, cookieA, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("2º DELETE: esperado 404, veio %d", rec.Code)
	}

	// member não-autor apagando o de outro → 403.
	c2 := criarComment(t, router, ws, tk.ID, cookieA, `{"body":"do owner"}`)
	rec = doJSON(router, http.MethodDelete, commentsPath(ws, tk.ID)+"/"+c2.ID, cookieM, "")
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Fatalf("member apagando o de outro: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-13/13.1/14 — viewer-assignee POSTa (201, carve-out passa o subrouter
// IRMÃO — prova D-02 e nível 2 do CA-19.1); viewer não-assignee → 403; viewer
// numa task com assignee NULL → 403, NÃO 500.
func TestComentarioViewerCarveOut(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vic Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	// task atribuída ao viewer.
	mine := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Do viewer","assignee":"`+idV+`"}`)
	// task NÃO atribuída (assignee NULL) — o caso normal do CA-13.1.
	other := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Sem dono"}`)

	// viewer-assignee → 201.
	rec := doJSON(router, http.MethodPost, commentsPath(ws, mine.ID), cookieV, `{"body":"respondendo"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("viewer-assignee deveria comentar (201), veio %d: %s", rec.Code, rec.Body.String())
	}
	// viewer numa task com assignee NULL → 403, NÃO 500 (CA-13.1).
	rec = doJSON(router, http.MethodPost, commentsPath(ws, other.ID), cookieV, `{"body":"tentando"}`)
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "forbidden" {
		t.Fatalf("viewer em task assignee NULL: esperado 403 (NÃO 500), veio %d: %s", rec.Code, rec.Body.String())
	}
}

// CA-11/12 — task de projeto private de OUTRO → 404 no GET e no POST (não
// vaza).
func TestComentarioTaskInvisivel404(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieM := loginVia(t, svc, stub, "g-m", "m@example.com", "Mia Member")
	ws := workspaceIDOf(t, pool, "g-a")
	idM := userIDOf(t, pool, "g-m")
	addMember(t, pool, ws, idM, "member")

	// projeto PRIVADO da Ana; Mia (membro do ws) não enxerga.
	p := criarProjeto(t, router, ws, cookieA, `{"name":"Secreto","color":"#4f9dde","visibility":"private"}`)
	tk := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Task secreta"}`)

	rec := doJSON(router, http.MethodGet, commentsPath(ws, tk.ID), cookieM, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET de task private de outro: esperado 404, veio %d", rec.Code)
	}
	rec = doJSON(router, http.MethodPost, commentsPath(ws, tk.ID), cookieM, `{"body":"oi"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST em task private de outro: esperado 404, veio %d", rec.Code)
	}
}

// CA-17 — 401 sem sessão nas 3 rotas (table-driven).
func TestComentarios401SemSessao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookie := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	ws := workspaceIDOf(t, pool, "g-a")
	p := criarProjeto(t, router, ws, cookie, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	tk := criarTask(t, router, ws, cookie, `{"project_id":"`+p.ID+`","name":"Task"}`)
	c := criarComment(t, router, ws, tk.ID, cookie, `{"body":"oi"}`)

	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, commentsPath(ws, tk.ID), ""},
		{http.MethodPost, commentsPath(ws, tk.ID), `{"body":"x"}`},
		{http.MethodDelete, commentsPath(ws, tk.ID) + "/" + c.ID, ""},
	} {
		rec := doJSON(router, tc.method, tc.path, nil, tc.body) // sem cookie
		if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthorized" {
			t.Errorf("%s %s sem sessão: esperado 401 unauthorized, veio %d: %s",
				tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

// CA-18.2 — o write-role global NÃO foi afrouxado pela fase 19: o
// viewer-assignee comenta (201) MAS segue 403 num PATCH de domínio que
// permanece barrado independentemente da fase 18 — concretamente um PATCH de
// campo FORA do carve-out status-only (name) na PRÓPRIA task, E um POST numa
// task onde NÃO é assignee. NÃO usa o status-only (que a fase 18 abre p/ 200).
func TestComentarioCarveOutNaoAfrouxaWriteRoleGlobal(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub := newCommentsRouter(t, pool)
	cookieA := loginVia(t, svc, stub, "g-a", "a@example.com", "Ana Owner")
	cookieV := loginVia(t, svc, stub, "g-v", "v@example.com", "Vic Viewer")
	ws := workspaceIDOf(t, pool, "g-a")
	idV := userIDOf(t, pool, "g-v")
	addMember(t, pool, ws, idV, "viewer")

	p := criarProjeto(t, router, ws, cookieA, `{"name":"App","color":"#4f9dde","visibility":"shared"}`)
	mine := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Do viewer","assignee":"`+idV+`"}`)
	other := criarTask(t, router, ws, cookieA, `{"project_id":"`+p.ID+`","name":"Sem dono"}`)

	// viewer-assignee comenta na task dele → 201 (carve-out da fase 19).
	rec := doJSON(router, http.MethodPost, commentsPath(ws, mine.ID), cookieV, `{"body":"posso comentar"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("viewer-assignee deveria comentar (201), veio %d: %s", rec.Code, rec.Body.String())
	}

	// MAS PATCH de campo FORA do status-only (name) na PRÓPRIA task → 403
	// (independente da fase 18; o write-role global segue barrando).
	rec = doJSON(router, http.MethodPatch, "/api/v1/workspaces/"+ws+"/tasks/"+mine.ID, cookieV, `{"name":"renomeei"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer-assignee PATCH name na própria task: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}

	// E POST de comentário numa task onde NÃO é assignee → 403 (não afrouxou).
	rec = doJSON(router, http.MethodPost, commentsPath(ws, other.ID), cookieV, `{"body":"tentando"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer não-assignee POST: esperado 403, veio %d: %s", rec.Code, rec.Body.String())
	}
}
