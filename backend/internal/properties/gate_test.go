// gate_test.go — fase 25 T-02: o gate de projeto AUTO-CONTIDO do pacote
// properties (D-03 alt. B, cópia da fase 12). Espelha tasks/objectives
// loadProjectGate sem importar tasks/projects/objectives: visibilidade→404,
// permissão→403, arquivado→400 invalid_reference. Inclui um teste de PARIDADE
// (mesma entrada → mesma decisão que o gate de objetivos).
package properties

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/objectives"
)

// gateSeed espelha o seedGate da fase 12: workspace com um projeto shared, um
// private e um shared arquivado; membros com access edit/view.
type gateSeed struct {
	ws       string
	pOwner   string // dono dos projetos
	mEdit    string // membro do projeto shared com access=edit
	mView    string // membro do projeto shared com access=view
	mNone    string // membro do workspace sem membership no projeto
	shared   string
	private  string
	archived string
}

func seedGate(t *testing.T, pool *pgxpool.Pool) gateSeed {
	t.Helper()
	ctx := context.Background()
	var s gateSeed
	mustScan := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", q, err)
		}
		return id
	}
	s.ws = mustScan(`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`)
	user := func(g string) string {
		return mustScan(`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', $1) RETURNING id`, g)
	}
	s.pOwner = user("owner")
	s.mEdit = user("edit")
	s.mView = user("view")
	s.mNone = user("none")
	for _, u := range []string{s.pOwner, s.mEdit, s.mView, s.mNone} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, s.ws, u); err != nil {
			t.Fatalf("seed ws member: %v", err)
		}
	}
	proj := func(name, vis string) string {
		return mustScan(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
			VALUES ($1, $2, '#4f9dde', $3, $4) RETURNING id`, s.ws, name, vis, s.pOwner)
	}
	s.shared = proj("Shared", "shared")
	s.private = proj("Private", "private")
	s.archived = proj("Archived", "shared")
	if _, err := pool.Exec(ctx, `UPDATE projects SET archived_at = now() WHERE id = $1`, s.archived); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
	for _, m := range []struct{ u, a string }{{s.mEdit, "edit"}, {s.mView, "view"}} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, $3)`, s.shared, m.u, m.a); err != nil {
			t.Fatalf("seed project member: %v", err)
		}
	}
	return s
}

func TestLoadProjectGateVisibilidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	if _, err := svc.loadProjectGate(ctx, s.ws, s.mNone, s.shared); err != nil {
		t.Errorf("shared deveria ser visível ao membro: %v", err)
	}
	if _, err := svc.loadProjectGate(ctx, s.ws, s.mNone, s.private); !isNotFound(err) {
		t.Errorf("private de outro deveria dar 404, veio %v", err)
	}
	if _, err := svc.loadProjectGate(ctx, s.ws, s.pOwner, s.private); err != nil {
		t.Errorf("dono deveria enxergar o próprio private: %v", err)
	}
	if _, err := svc.loadProjectGate(ctx, s.ws, s.pOwner, "nao-e-uuid"); !isNotFound(err) {
		t.Errorf("projectID malformado deveria dar 404, veio %v", err)
	}
}

func TestEnsureWritePermissao(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool)
	ctx := context.Background()

	cases := []struct {
		name    string
		user    string
		role    string
		project string
		block   bool
		wantErr func(error) bool
	}{
		{"dono do projeto escreve", s.pOwner, "member", s.shared, true, nil},
		{"owner do workspace escreve", s.mNone, "owner", s.shared, true, nil},
		{"membro access=edit escreve", s.mEdit, "member", s.shared, true, nil},
		{"membro access=view → 403", s.mView, "member", s.shared, true, isForbidden},
		{"membro sem membership → 403", s.mNone, "member", s.shared, true, isForbidden},
		{"private de outro → 404 (não vaza)", s.mNone, "member", s.private, true, isNotFound},
		{"arquivado bloqueado → 400 invalid_reference", s.pOwner, "member", s.archived, true, isInvalidRef},
		{"arquivado sem bloqueio → ok (delete)", s.pOwner, "member", s.archived, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.ensureWrite(ctx, s.ws, c.user, c.role, c.project, c.block)
			if c.wantErr == nil {
				if err != nil {
					t.Errorf("esperava ok, veio %v", err)
				}
				return
			}
			if !c.wantErr(err) {
				t.Errorf("erro inesperado: %v", err)
			}
		})
	}
}

// TestGateParidadeComObjetivos: o gate de properties decide IGUAL ao gate de
// objectives (fase 12) pras mesmas entradas — prova que a cópia (D-03) não
// divergiu. Compara o code do erro (ou ok) pros casos canônicos.
func TestGateParidadeComObjetivos(t *testing.T) {
	pool := dbtest.NewPool(t)
	s := seedGate(t, pool)
	ctx := context.Background()

	propSvc := NewService(pool)
	objSvc := objectives.NewService(pool)

	// objectives.Create chama ensureWrite(blockArchived=true) internamente; o code
	// do erro reflete a MESMA decisão de gate. Comparamos os codes.
	cases := []struct {
		name    string
		user    string
		role    string
		project string
	}{
		{"dono escreve", s.pOwner, "member", s.shared},
		{"view → 403", s.mView, "member", s.shared},
		{"sem membership → 403", s.mNone, "member", s.shared},
		{"private → 404", s.mNone, "member", s.private},
		{"arquivado → 400", s.pOwner, "member", s.archived},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, propErr := propSvc.ensureWrite(ctx, s.ws, c.user, c.role, c.project, true)
			_, objErr := objSvc.Create(ctx, s.ws, c.user, c.role, c.project,
				objectives.CreateInput{Name: "paridade"})
			if codeOf(propErr) != codeOf(objErr) {
				t.Errorf("divergência de gate: properties=%q vs objectives=%q", codeOf(propErr), codeOf(objErr))
			}
		})
	}
}

// TestGateCrossWorkspaceNaoVaza (T-14 #1, HIGH): o middleware RequireMember só
// valida a membership no workspace da ROTA — não casa o projeto da URL com esse
// workspace. O `WHERE p.workspace_id=$1` do gate é a ÚNICA defesa que impede um
// membro do WS-A de alcançar um projeto SHARED do WS-B (project ids são UUID global;
// visibility='shared' passa a cláusula de visibilidade pra qualquer um). A mutação
// que remove `p.workspace_id=$1` DEVE morrer aqui (sem este teste, sobrevivia).
func TestGateCrossWorkspaceNaoVaza(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := NewService(pool)
	s := seedGate(t, pool) // s.pOwner é membro do s.ws
	ctx := context.Background()

	// workspace B distinto, com um projeto SHARED próprio (dono = userB).
	var wsB, userB, projBShared string
	mustB := func(q string, args ...any) string {
		var id string
		if err := pool.QueryRow(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed B (%s): %v", q, err)
		}
		return id
	}
	wsB = mustB(`INSERT INTO workspaces (name) VALUES ('WS B') RETURNING id`)
	userB = mustB(`INSERT INTO users (google_id, email, name) VALUES ('crossB','crossB@b.c','B') RETURNING id`)
	projBShared = mustB(`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		VALUES ($1, 'PB shared', '#4f9dde', 'shared', $2) RETURNING id`, wsB, userB)

	// membro do WS-A alcançando um projeto SHARED do WS-B PELA rota do WS-A → 404
	// (loadProjectGate filtra por workspace_id — não vaza cross-tenant — CA-15).
	if _, err := svc.loadProjectGate(ctx, s.ws, s.pOwner, projBShared); !isNotFound(err) {
		t.Errorf("loadProjectGate(wsA, projeto shared de wsB) deveria dar 404, veio %v", err)
	}
	if _, err := svc.ensureWrite(ctx, s.ws, s.pOwner, "owner", projBShared, false); !isNotFound(err) {
		t.Errorf("ensureWrite(wsA, projeto shared de wsB) deveria dar 404, veio %v", err)
	}
	if _, err := svc.ListDefs(ctx, s.ws, s.pOwner, projBShared); !isNotFound(err) {
		t.Errorf("ListDefs(wsA, projeto shared de wsB) deveria dar 404, veio %v", err)
	}
}

func isNotFound(err error) bool   { return codeIs(err, "not_found") }
func isForbidden(err error) bool  { return codeIs(err, "forbidden") }
func isInvalidRef(err error) bool { return codeIs(err, "invalid_reference") }

func codeIs(err error, code string) bool { return codeOf(err) == code }

// codeOf devolve o code do *domain.Error, ou "" se err == nil (ok).
func codeOf(err error) string {
	if err == nil {
		return ""
	}
	var de *domain.Error
	if !errors.As(err, &de) {
		return "non_domain"
	}
	return de.Code
}
