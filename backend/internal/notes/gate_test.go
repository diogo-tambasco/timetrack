package notes

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/tasks"
)

// --- helpers de seed (SQL direto; não há API de invite no MVP) ---

func seedUser(t *testing.T, pool *pgxpool.Pool, googleID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', $1) RETURNING id`,
		googleID).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", googleID, err)
	}
	return id
}

func seedWorkspace(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed ws %s: %v", name, err)
	}
	return id
}

func seedMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		wsID, userID, role); err != nil {
		t.Fatalf("seed member: %v", err)
	}
}

func seedProject(t *testing.T, pool *pgxpool.Pool, wsID, ownerID, visibility string, archived bool) string {
	t.Helper()
	var id string
	q := `INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility, archived_at)
	      VALUES ($1, $2, 'P', '#112233', $3, CASE WHEN $4 THEN now() ELSE NULL END) RETURNING id`
	if err := pool.QueryRow(context.Background(), q, wsID, ownerID, visibility, archived).Scan(&id); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return id
}

func seedProjectMember(t *testing.T, pool *pgxpool.Pool, projID, userID, access string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO project_members (project_id, user_id, access) VALUES ($1, $2, $3)`,
		projID, userID, access); err != nil {
		t.Fatalf("seed project member: %v", err)
	}
}

// seedNote insere uma nota direto no banco (projeto = "" → nota solta).
func seedNote(t *testing.T, pool *pgxpool.Pool, wsID, createdBy, projectID string) string {
	t.Helper()
	var id string
	var proj any
	if projectID != "" {
		proj = projectID
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO notes (workspace_id, project_id, created_by) VALUES ($1, $2, $3) RETURNING id`,
		wsID, proj, createdBy).Scan(&id); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	return id
}

func newSvc(pool *pgxpool.Pool) *Service {
	return NewService(pool, tasks.NewService(pool))
}

func isDomain(err error, code string) bool {
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}

// CA-11/11.1/13/14 — loadNoteGate cobre a matriz de visibilidade/escrita.
func TestLoadNoteGate(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	member := seedUser(t, pool, "g-member") // ws member, sem access no projeto
	editor := seedUser(t, pool, "g-editor") // access=edit no projeto
	viewer := seedUser(t, pool, "g-viewer") // ws member com access=view
	other := seedUser(t, pool, "g-other")   // user de OUTRO ws
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, member, "member")
	seedMember(t, pool, wsID, editor, "member")
	seedMember(t, pool, wsID, viewer, "viewer")

	shared := seedProject(t, pool, wsID, owner, "shared", false)
	seedProjectMember(t, pool, shared, editor, "edit")
	seedProjectMember(t, pool, shared, viewer, "view")

	privateP := seedProject(t, pool, wsID, owner, "private", false)
	archived := seedProject(t, pool, wsID, owner, "shared", true)

	noteShared := seedNote(t, pool, wsID, owner, shared)
	notePrivate := seedNote(t, pool, wsID, owner, privateP)
	noteArchived := seedNote(t, pool, wsID, owner, archived)
	noteLooseOwner := seedNote(t, pool, wsID, owner, "")  // nota solta do owner
	noteLooseOther := seedNote(t, pool, wsID, member, "") // nota solta do member

	// nota de projeto private de OUTRO (que owner enxerga; member não).
	// member NÃO enxerga notePrivate → 404.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, member, "member", notePrivate, false); !isDomain(err, "not_found") {
		t.Errorf("member em nota de projeto private → esperado 404, veio %v", err)
	}
	// leitura (loadVisibleNote): member não vê notePrivate → 404; owner vê.
	if _, err := svc.loadVisibleNote(ctx, pool, wsID, member, notePrivate); !isDomain(err, "not_found") {
		t.Errorf("loadVisibleNote member em private → esperado 404, veio %v", err)
	}
	if _, err := svc.loadVisibleNote(ctx, pool, wsID, owner, notePrivate); err != nil {
		t.Errorf("loadVisibleNote owner em private próprio → esperado ok, veio %v", err)
	}

	// access=view (viewer) ENXERGA (leitura ok) mas ESCREVE → 403.
	if _, err := svc.loadVisibleNote(ctx, pool, wsID, viewer, noteShared); err != nil {
		t.Fatalf("viewer deveria ENXERGAR a nota shared (leitura): %v", err)
	}
	if _, err := svc.loadNoteGate(ctx, pool, wsID, viewer, "viewer", noteShared, true); !isDomain(err, "forbidden") {
		t.Errorf("viewer em escrita de nota shared → esperado 403, veio %v", err)
	}
	// member do ws SEM access no projeto também não escreve → 403.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, member, "member", noteShared, true); !isDomain(err, "forbidden") {
		t.Errorf("member sem access em escrita → esperado 403, veio %v", err)
	}

	// dono escreve → ok.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", noteShared, true); err != nil {
		t.Errorf("owner em escrita → esperado ok, veio %v", err)
	}
	// access=edit escreve → ok.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, editor, "member", noteShared, true); err != nil {
		t.Errorf("editor (access=edit) em escrita → esperado ok, veio %v", err)
	}

	// projeto arquivado + blockArchived=true → 400 invalid_reference (NÃO 403).
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", noteArchived, true); !isDomain(err, "invalid_reference") {
		t.Errorf("escrita em projeto arquivado (blockArchived=true) → esperado 400 invalid_reference, veio %v", err)
	}
	// arquivado + blockArchived=false (DELETE) → ok (CA-06.1/26-H7).
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", noteArchived, false); err != nil {
		t.Errorf("DELETE em projeto arquivado (blockArchived=false) → esperado ok, veio %v", err)
	}

	// nota solta de OUTRO usuário → 404 (CA-11.1). owner não vê a solta do member.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", noteLooseOther, false); !isDomain(err, "not_found") {
		t.Errorf("nota solta de outro user → esperado 404, veio %v", err)
	}
	// member tampouco vê a solta de outro (mesmo sendo o criador desta) — sua própria ok:
	if _, err := svc.loadNoteGate(ctx, pool, wsID, member, "member", noteLooseOther, false); err != nil {
		t.Errorf("criador da nota solta deveria ler/escrever: %v", err)
	}
	// nota solta do owner: owner ok; member → 404.
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", noteLooseOwner, true); err != nil {
		t.Errorf("owner na própria solta → ok, veio %v", err)
	}
	if _, err := svc.loadNoteGate(ctx, pool, wsID, member, "member", noteLooseOwner, false); !isDomain(err, "not_found") {
		t.Errorf("member na solta do owner → esperado 404, veio %v", err)
	}

	// noteID malformado → 404 (não 500).
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", "nao-e-uuid", false); !isDomain(err, "not_found") {
		t.Errorf("noteID malformado → esperado 404, veio %v", err)
	}

	// nota de OUTRO workspace → 404 (isolamento). Cria ws2 com nota.
	ws2 := seedWorkspace(t, pool, "WS2")
	seedMember(t, pool, ws2, other, "owner")
	proj2 := seedProject(t, pool, ws2, other, "shared", false)
	note2 := seedNote(t, pool, ws2, other, proj2)
	if _, err := svc.loadNoteGate(ctx, pool, wsID, owner, "owner", note2, false); !isDomain(err, "not_found") {
		t.Errorf("nota de outro workspace → esperado 404, veio %v", err)
	}
}
