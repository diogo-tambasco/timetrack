// gate_test.go — fase 12 T-02: o gate de projeto AUTO-CONTIDO do pacote
// objectives (D-03 alt. B). Espelha a regra de tasks.loadProjectGate sem
// importar tasks/projects: visibilidade→404, permissão→403, arquivado→400.
package objectives

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
)

// seedGate planta um workspace com um projeto shared e um private + membros
// com access edit/view, devolvendo os ids relevantes.
type gateSeed struct {
	ws       string
	pOwner   string // dono dos projetos
	mEdit    string // membro do projeto shared com access=edit
	mView    string // membro do projeto shared com access=view
	mNone    string // membro do workspace sem membership no projeto
	shared   string // projeto shared (visível a todos do ws)
	private  string // projeto private (só o dono enxerga)
	archived string // projeto shared arquivado
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

	// shared visível a qualquer membro do ws
	if _, err := svc.loadProjectGate(ctx, s.ws, s.mNone, s.shared); err != nil {
		t.Errorf("shared deveria ser visível ao membro: %v", err)
	}
	// private de outro → 404 (mesma resposta de inexistente)
	if _, err := svc.loadProjectGate(ctx, s.ws, s.mNone, s.private); !isNotFound(err) {
		t.Errorf("private de outro deveria dar 404, veio %v", err)
	}
	// dono enxerga o próprio private
	if _, err := svc.loadProjectGate(ctx, s.ws, s.pOwner, s.private); err != nil {
		t.Errorf("dono deveria enxergar o próprio private: %v", err)
	}
	// projectID malformado → 404 (não 500)
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
		{"arquivado bloqueado → 400", s.pOwner, "member", s.archived, true, isInvalidRef},
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

func isNotFound(err error) bool   { return codeIs(err, "not_found") }
func isForbidden(err error) bool  { return codeIs(err, "forbidden") }
func isInvalidRef(err error) bool { return codeIs(err, "invalid_reference") }

func codeIs(err error, code string) bool {
	var de *domain.Error
	if !errors.As(err, &de) {
		return false
	}
	return de.Code == code
}
