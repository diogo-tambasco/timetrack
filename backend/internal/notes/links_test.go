package notes

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
)

func findLink(links []NoteLink, refType, refID string) *NoteLink {
	for i := range links {
		if links[i].RefType == refType && links[i].RefID == refID {
			return &links[i]
		}
	}
	return nil
}

// CA-07/09 — /task cria task no projeto da nota (status 'todo') + link; DeleteLink
// remove só o link e a task sobrevive.
func TestCreateLinkTask(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})

	res, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "task", "  Publicar OKRs  ")
	if err != nil {
		t.Fatalf("CreateLink task: %v", err)
	}
	if res.Task == nil || res.Task.Status != "todo" || res.Task.ProjectID != proj || res.Task.Name != "Publicar OKRs" {
		t.Errorf("task do slash fora do contrato (status deve ser 'todo'): %+v", res.Task)
	}
	if res.Link.RefType != "task" || res.Link.RefID != res.Task.ID || res.Link.Label != "Publicar OKRs" {
		t.Errorf("link fora do contrato: %+v", res.Link)
	}

	// o link resolve no GET da nota-mãe.
	got, _ := svc.Get(ctx, wsID, owner, note.ID)
	if l := findLink(got.Links, "task", res.Task.ID); l == nil || l.Label != "Publicar OKRs" || l.Status != "todo" {
		t.Errorf("link da task deveria resolver no GET: %+v", got.Links)
	}

	// DeleteLink remove só o link; a task sobrevive.
	if err := svc.DeleteLink(ctx, wsID, owner, "owner", note.ID, res.Link.ID); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	var nTasks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE id = $1`, res.Task.ID).Scan(&nTasks); err != nil {
		t.Fatal(err)
	}
	if nTasks != 1 {
		t.Errorf("DeleteLink NÃO deveria apagar a task, restaram %d", nTasks)
	}
	got, _ = svc.Get(ctx, wsID, owner, note.ID)
	if len(got.Links) != 0 {
		t.Errorf("link deveria sumir após DeleteLink, veio %+v", got.Links)
	}
	// 2º delete / link de outra nota / malformado → 404.
	if err := svc.DeleteLink(ctx, wsID, owner, "owner", note.ID, res.Link.ID); !isDomain(err, "not_found") {
		t.Errorf("2º DeleteLink → esperado 404, veio %v", err)
	}
	if err := svc.DeleteLink(ctx, wsID, owner, "owner", note.ID, "nao-uuid"); !isDomain(err, "not_found") {
		t.Errorf("DeleteLink malformado → esperado 404, veio %v", err)
	}
}

// CA-08/08.1 — /note cria sub-nota + link; kind inválido → 400 sem criar nada.
func TestCreateLinkNote(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})

	res, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "note", "Sub-detalhes")
	if err != nil {
		t.Fatalf("CreateLink note: %v", err)
	}
	if res.Note == nil || res.Note.Title != "Sub-detalhes" || res.Note.ProjectID == nil || *res.Note.ProjectID != proj {
		t.Errorf("sub-nota fora do contrato (mesmo projeto da mãe): %+v", res.Note)
	}
	if res.Link.RefType != "note" || res.Link.RefID != res.Note.ID || res.Link.Label != "Sub-detalhes" {
		t.Errorf("link da sub-nota fora do contrato: %+v", res.Link)
	}

	// kind inválido → 400, nada criado.
	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "goal", "x"); !isDomain(err, "validation_error") {
		t.Errorf("kind inválido → esperado 400 validation_error, veio %v", err)
	}
	got, _ := svc.Get(ctx, wsID, owner, note.ID)
	if len(got.Links) != 1 {
		t.Errorf("kind inválido NÃO deveria criar link, veio %d links", len(got.Links))
	}
}

// CA-10 — /task em nota solta → 400 e nenhuma task; /note em nota solta → sub-nota
// SOLTA + link, e o chip da sub-nota solta RESOLVE pro criador (26-H2).
func TestCreateLinkNotaSolta(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	loose, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{Title: "Solta"})

	// /task em nota solta → 400, nenhuma task criada.
	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", loose.ID, "task", "Tarefa"); !isDomain(err, "validation_error") {
		t.Errorf("/task em nota solta → esperado 400 validation_error, veio %v", err)
	}
	var nTasks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE workspace_id = $1`, wsID).Scan(&nTasks); err != nil {
		t.Fatal(err)
	}
	if nTasks != 0 {
		t.Errorf("/task em nota solta NÃO deveria criar task, veio %d", nTasks)
	}

	// /note em nota solta → sub-nota TAMBÉM solta + link.
	res, err := svc.CreateLink(ctx, wsID, owner, "owner", loose.ID, "note", "Sub solta")
	if err != nil {
		t.Fatalf("/note em nota solta: %v", err)
	}
	if res.Note == nil || res.Note.ProjectID != nil {
		t.Errorf("sub-nota de mãe solta deveria ser SOLTA (project_id NULL): %+v", res.Note)
	}

	// 26-H2: o chip da sub-nota SOLTA resolve pro criador no GET da nota-mãe.
	got, _ := svc.Get(ctx, wsID, owner, loose.ID)
	if l := findLink(got.Links, "note", res.Note.ID); l == nil || l.Label != "Sub solta" {
		t.Errorf("sub-nota solta do criador deveria RESOLVER (26-H2): %+v", got.Links)
	}
}

// D-04 — atomicidade: o INSERT do link falha → a transação inteira reverte (a
// task NÃO persiste). Forçamos a falha pré-criando um link colidindo com o uq.
func TestCreateLinkAtomicidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})

	tasksBefore := countTasks(t, pool, wsID)

	// injeta uma falha: um trigger temporário que rejeita INSERT em note_links.
	// (simula falha no passo final → a task criada na MESMA tx deve reverter.)
	if _, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_note_link() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'falha de teste no link'; END; $$ LANGUAGE plpgsql;
		CREATE TRIGGER trg_fail_link BEFORE INSERT ON note_links
		  FOR EACH ROW EXECUTE FUNCTION fail_note_link();`); err != nil {
		t.Fatalf("criar trigger: %v", err)
	}
	defer pool.Exec(ctx, `DROP TRIGGER IF EXISTS trg_fail_link ON note_links; DROP FUNCTION IF EXISTS fail_note_link();`)

	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "task", "Vai falhar"); err == nil {
		t.Fatal("CreateLink deveria falhar com o trigger de erro")
	}
	if got := countTasks(t, pool, wsID); got != tasksBefore {
		t.Errorf("a task NÃO deveria persistir após falha no link (rollback total), tinha %d veio %d", tasksBefore, got)
	}
}

// D-04 (ramo /note) — atomicidade: o INSERT do link falha → a transação inteira
// reverte (a SUB-NOTA criada na MESMA tx NÃO persiste). Espelha
// TestCreateLinkAtomicidade, mas para kind="note". Mutação a matar (links.go):
// QueryRow do INSERT da sub-nota em s.db em vez de tx → sub-nota nasce fora da
// tx e sobrevive ao rollback (contagem de notas sobe +1).
func TestCreateLinkNoteAtomicidade(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})

	notesBefore := countNotes(t, pool, wsID)

	// mesmo trigger de TestCreateLinkAtomicidade: faz o INSERT em note_links falhar.
	if _, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_note_link() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'falha de teste no link'; END; $$ LANGUAGE plpgsql;
		CREATE TRIGGER trg_fail_link BEFORE INSERT ON note_links
		  FOR EACH ROW EXECUTE FUNCTION fail_note_link();`); err != nil {
		t.Fatalf("criar trigger: %v", err)
	}
	defer pool.Exec(ctx, `DROP TRIGGER IF EXISTS trg_fail_link ON note_links; DROP FUNCTION IF EXISTS fail_note_link();`)

	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "note", "Sub orfa"); err == nil {
		t.Fatal("CreateLink note deveria falhar com o trigger de erro")
	}
	if got := countNotes(t, pool, wsID); got != notesBefore {
		t.Errorf("a sub-nota NÃO deveria persistir após falha no link (rollback total), tinha %d veio %d", notesBefore, got)
	}
}

func countNotes(t *testing.T, pool *pgxpool.Pool, wsID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notes WHERE workspace_id = $1`, wsID).Scan(&n); err != nil {
		t.Fatalf("contar notas: %v", err)
	}
	return n
}

// CA-11/14 (escalação de privilégio) — um VIEWER que apenas ENXERGA a nota de
// projeto SHARED não pode criar nem deletar link (403 em ambos). Mutação a matar
// (links.go ~linha 48): trocar loadNoteGate(...,true) por loadVisibleNote(...) no
// CreateLink (descarta o write-check) → viewer conseguiria criar o link.
func TestLinkViewerSemEscrita403(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	viewer := seedUser(t, pool, "g-viewer") // ws role viewer
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, viewer, "viewer")

	proj := seedProject(t, pool, wsID, owner, "shared", false) // viewer ENXERGA (shared)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})

	// viewer vê a nota (leitura ok), mas CreateLink (escrita) → 403.
	if _, err := svc.CreateLink(ctx, wsID, viewer, "viewer", note.ID, "note", "burlando"); !isDomain(err, "forbidden") {
		t.Errorf("viewer CreateLink → esperado 403 forbidden, veio %v", err)
	}
	// nenhuma sub-nota / link nasceu (o gate barra antes de qualquer INSERT).
	got, _ := svc.Get(ctx, wsID, owner, note.ID)
	if len(got.Links) != 0 {
		t.Errorf("viewer 403 NÃO deveria criar link, veio %+v", got.Links)
	}

	// owner cria um link de verdade; viewer tenta DeleteLink → 403.
	res, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "task", "Tarefa")
	if err != nil {
		t.Fatalf("owner CreateLink: %v", err)
	}
	if err := svc.DeleteLink(ctx, wsID, viewer, "viewer", note.ID, res.Link.ID); !isDomain(err, "forbidden") {
		t.Errorf("viewer DeleteLink → esperado 403 forbidden, veio %v", err)
	}
	// o link sobreviveu (viewer não conseguiu apagar).
	got, _ = svc.Get(ctx, wsID, owner, note.ID)
	if findLink(got.Links, "task", res.Task.ID) == nil {
		t.Errorf("link NÃO deveria ter sido apagado pelo viewer")
	}
}

// CA-06/26-H7 — CreateLink (task E note) contra nota de projeto ARQUIVADO → 400
// invalid_reference (projeto arquivado é só leitura). Mutação a matar (links.go
// ~linha 48): loadNoteGate(...,true) → (...,false) descarta o block-archived →
// CreateLink passaria no arquivado.
func TestCreateLinkProjetoArquivado(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	// projeto ATIVO: cria a nota, depois arquiva.
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})
	if _, err := pool.Exec(ctx, `UPDATE projects SET archived_at = now() WHERE id = $1`, proj); err != nil {
		t.Fatalf("arquivar: %v", err)
	}

	// CreateLink kind=task em projeto arquivado → 400.
	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "task", "Tarefa"); !isDomain(err, "invalid_reference") {
		t.Errorf("CreateLink task em arquivado → esperado 400 invalid_reference, veio %v", err)
	}
	// CreateLink kind=note em projeto arquivado → 400.
	if _, err := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "note", "Sub"); !isDomain(err, "invalid_reference") {
		t.Errorf("CreateLink note em arquivado → esperado 400 invalid_reference, veio %v", err)
	}
	// nada nasceu (gate barra antes de qualquer INSERT).
	got, _ := svc.Get(ctx, wsID, owner, note.ID)
	if len(got.Links) != 0 {
		t.Errorf("CreateLink em arquivado NÃO deveria criar link, veio %+v", got.Links)
	}
}

// CA-12/D-07 — resolução OMITE link cujo alvo (task) virou private de outro user.
func TestResolveOmiteTaskInvisivel(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	member := seedUser(t, pool, "g-member")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, member, "member")
	// owner cria projeto shared + nota + /task. member (que enxerga shared) lê o chip.
	proj := seedProject(t, pool, wsID, owner, "shared", false)
	note, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &proj, Title: "Mãe"})
	res, _ := svc.CreateLink(ctx, wsID, owner, "owner", note.ID, "task", "Visível")

	// member vê o chip (projeto shared).
	got, _ := svc.Get(ctx, wsID, member, note.ID)
	if findLink(got.Links, "task", res.Task.ID) == nil {
		t.Fatalf("member deveria ver o chip da task shared")
	}

	// agora a task vai pra um projeto PRIVATE de owner → member não enxerga mais.
	priv := seedProject(t, pool, wsID, owner, "private", false)
	if _, err := pool.Exec(ctx, `UPDATE tasks SET project_id = $1 WHERE id = $2`, priv, res.Task.ID); err != nil {
		t.Fatalf("mover task: %v", err)
	}
	got, _ = svc.Get(ctx, wsID, member, note.ID)
	if findLink(got.Links, "task", res.Task.ID) != nil {
		t.Errorf("chip de task que virou private NÃO deveria resolver pro member (CA-12)")
	}
	// owner (dono do private) ainda vê.
	got, _ = svc.Get(ctx, wsID, owner, note.ID)
	if findLink(got.Links, "task", res.Task.ID) == nil {
		t.Errorf("owner do projeto private ainda deveria ver o chip")
	}
}

// 26-H2 — sub-nota de outro user / de projeto invisível é OMITIDA da resolução.
func TestResolveOmiteSubNotaInvisivel(t *testing.T) {
	pool := dbtest.NewPool(t)
	svc := newSvc(pool)
	ctx := context.Background()

	owner := seedUser(t, pool, "g-owner")
	member := seedUser(t, pool, "g-member")
	wsID := seedWorkspace(t, pool, "WS")
	seedMember(t, pool, wsID, owner, "owner")
	seedMember(t, pool, wsID, member, "member")

	// owner cria nota de projeto shared + sub-nota; depois move a sub pra um
	// projeto private → member (que vê a mãe shared) não deve mais ver o chip.
	shared := seedProject(t, pool, wsID, owner, "shared", false)
	noteShared, _ := svc.Create(ctx, wsID, owner, "owner", CreateInput{ProjectID: &shared, Title: "Mãe shared"})
	// cria a sub-nota via /note (nasce no MESMO projeto shared) e depois move pra private.
	subRes, _ := svc.CreateLink(ctx, wsID, owner, "owner", noteShared.ID, "note", "Sub que some")
	priv := seedProject(t, pool, wsID, owner, "private", false)
	if _, err := pool.Exec(ctx, `UPDATE notes SET project_id = $1 WHERE id = $2`, priv, subRes.Note.ID); err != nil {
		t.Fatalf("mover sub-nota: %v", err)
	}
	// member enxerga a nota-mãe shared, mas o chip da sub que foi pra private some.
	got, _ := svc.Get(ctx, wsID, member, noteShared.ID)
	if findLink(got.Links, "note", subRes.Note.ID) != nil {
		t.Errorf("chip de sub-nota que virou private NÃO deveria resolver pro member")
	}
	// owner ainda vê (dono do private).
	got, _ = svc.Get(ctx, wsID, owner, noteShared.ID)
	if findLink(got.Links, "note", subRes.Note.ID) == nil {
		t.Errorf("owner do projeto private ainda deveria ver o chip da sub")
	}
}

func countTasks(t *testing.T, pool *pgxpool.Pool, wsID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tasks WHERE workspace_id = $1`, wsID).Scan(&n); err != nil {
		t.Fatalf("contar tasks: %v", err)
	}
	return n
}
