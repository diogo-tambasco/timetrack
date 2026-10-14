// service_test.go — fase 21 T-03/T-04: presence.Service.OnTimer (fan-out por
// workspace, escopado ao workspace DA ENTRY quando running:true — CA-02/02.1) +
// isolamento entre workspaces (CA-03), self-exclusion (D-03), running derivado
// (CA-04/08/09), timer sem projeto (CA-07), best-effort (CA-10) e visibilidade
// priv/shared (CA-05/06). DBTest + broadcast FAKE que captura (memberID, payload).
package presence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/entries"
	"timetrack.live/backend/internal/presence"
	"timetrack.live/backend/internal/timer"
)

// capture é um broadcast fake: registra cada chamada (memberID, payload). É o
// espelho do Sink fake da fase 04, mas no nível do presence.Service.
type capture struct {
	mu    sync.Mutex
	calls []captured
}

type captured struct {
	member  string
	payload presence.PresencePayload
}

func (c *capture) fn(member string, payload any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, _ := payload.(presence.PresencePayload)
	c.calls = append(c.calls, captured{member: member, payload: p})
}

// to devolve as chamadas destinadas a um membro.
func (c *capture) to(member string) []presence.PresencePayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []presence.PresencePayload
	for _, cl := range c.calls {
		if cl.member == member {
			out = append(out, cl.payload)
		}
	}
	return out
}

// inWS devolve as chamadas pra um membro num workspace específico.
func (c *capture) inWS(member, ws string) []presence.PresencePayload {
	var out []presence.PresencePayload
	for _, p := range c.to(member) {
		if p.WorkspaceID == ws {
			out = append(out, p)
		}
	}
	return out
}

// — helpers de seed —

func mkUser(t *testing.T, pool *pgxpool.Pool, gid string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (google_id, email, name) VALUES ($1, $1 || '@b.c', $1) RETURNING id`,
		gid).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", gid, err)
	}
	return id
}

func mkWorkspace(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("insert ws %s: %v", name, err)
	}
	return id
}

func mkMember(t *testing.T, pool *pgxpool.Pool, wsID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		wsID, userID, role); err != nil {
		t.Fatalf("insert member: %v", err)
	}
}

func mkProject(t *testing.T, pool *pgxpool.Pool, wsID, ownerID, name, visibility string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
		 VALUES ($1, $2, $3, '#112233', $4) RETURNING id`,
		wsID, ownerID, name, visibility).Scan(&id); err != nil {
		t.Fatalf("insert project %s: %v", name, err)
	}
	return id
}

// runningState monta um State de trabalho com uma entry rodando no ws/projeto
// dados — espelha o que o timer.Service entrega no OnChange.
func runningState(wsID, userID string, projectID *string, startedAt time.Time) *timer.State {
	return &timer.State{
		Mode: "countup",
		Entry: &entries.Entry{
			ID:          "entry-id-irrelevante",
			WorkspaceID: wsID,
			UserID:      userID,
			ProjectID:   projectID,
			StartedAt:   startedAt,
		},
		ElapsedSeconds: 0,
	}
}

func newService(pool *pgxpool.Pool, cap *capture, now time.Time) *presence.Service {
	return presence.NewService(pool, cap.fn, presence.WithClock(func() time.Time { return now }))
}

// CA-02/04/05 — A roda timer em projeto shared no ws W com B membro → B recebe
// presence running:true, project=<nome>, since=started_at. (CA-05 detalhado no
// T-04; aqui o caminho feliz do fan-out.)
func TestOnTimerFanOutSharedRunning(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	a := mkUser(t, pool, "fanout-a")
	b := mkUser(t, pool, "fanout-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	proj := mkProject(t, pool, ws, a, "Site", "shared")
	started := time.Now().UTC().Add(-3 * time.Minute)

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, &proj, started), time.Now().UTC())
	_ = ctx

	got := cap.inWS(b, ws)
	if len(got) != 1 {
		t.Fatalf("B deveria receber 1 presence de A no ws, recebeu %d", len(got))
	}
	p := got[0]
	if p.UserID != a || !p.Running {
		t.Errorf("payload inesperado: %+v", p)
	}
	if p.Project == nil || *p.Project != "Site" {
		t.Errorf("projeto shared deveria aparecer com nome: %+v", p.Project)
	}
	if p.Since == nil || !p.Since.Equal(started) {
		t.Errorf("since deveria ser o started_at (%v), veio %v", started, p.Since)
	}
}

// (verificação do build, HIGH) CA-06 — A pode rodar timer num projeto PRIVATE de
// um CO-MEMBRO (timer.Start aceita qualquer projeto do ws, sem checar dono): o
// nome NÃO pode vazar pros co-membros NÃO-donos. O dono (B) vê o nome; o não-dono
// (C) NÃO. Sem o fix (cache do nome por-workspace pelo 1º observador), C vazaria.
func TestOnTimerProjetoPrivadoDeOutroNaoVazaProNaoDono(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "leak-a")
	b := mkUser(t, pool, "leak-b") // DONO do projeto private
	c := mkUser(t, pool, "leak-c") // co-membro NÃO-dono
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	mkMember(t, pool, ws, c, "member")
	proj := mkProject(t, pool, ws, b, "SegredoDoB", "private") // dono = B
	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, &proj, time.Now().UTC()), time.Now().UTC())

	gotB := cap.inWS(b, ws)
	if len(gotB) != 1 || gotB[0].Project == nil || *gotB[0].Project != "SegredoDoB" {
		t.Errorf("o DONO (B) do projeto private deveria ver o nome: %+v", gotB)
	}
	gotC := cap.inWS(c, ws)
	if len(gotC) != 1 {
		t.Fatalf("C deveria receber 1 presence: %+v", gotC)
	}
	if gotC[0].Project != nil {
		t.Errorf("VAZAMENTO: C (não-dono) recebeu o nome do projeto private de B: %v", *gotC[0].Project)
	}
}

// (verificação do build) CA-10 — best-effort: um PÂNICO no broadcast NÃO propaga
// pro caminho do timer (o recover do OnTimer engole). Mutação removendo o recover
// faz este teste entrar em pânico e falhar.
func TestOnTimerRecuperaDePanico(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "panic-a")
	b := mkUser(t, pool, "panic-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	proj := mkProject(t, pool, ws, a, "P", "shared")
	svc := presence.NewService(pool, func(string, any) { panic("boom no broadcast") })
	svc.OnTimer(a, runningState(ws, a, &proj, time.Now().UTC()), time.Now().UTC())
}

// CA-02 / D-03 — A NÃO recebe presence de si mesmo.
func TestOnTimerNaoMandaPraSiMesmo(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "self-a")
	b := mkUser(t, pool, "self-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	proj := mkProject(t, pool, ws, a, "P", "shared")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, &proj, time.Now().UTC()), time.Now().UTC())

	if got := cap.to(a); len(got) != 0 {
		t.Errorf("A não pode receber presence de si: %+v", got)
	}
	if got := cap.to(b); len(got) != 1 {
		t.Errorf("B deveria receber o presence de A: %d", len(got))
	}
}

// CA-03 — isolamento: C, membro de um ws SEM interseção com A, NUNCA recebe
// presence de A. Mutação removendo o WHERE de membership faria C receber.
func TestOnTimerIsolamentoEntreWorkspaces(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "iso-a")
	c := mkUser(t, pool, "iso-c")
	w1 := mkWorkspace(t, pool, "W1")
	w2 := mkWorkspace(t, pool, "W2")
	mkMember(t, pool, w1, a, "owner")
	mkMember(t, pool, w2, c, "owner")
	proj := mkProject(t, pool, w1, a, "P", "shared")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(w1, a, &proj, time.Now().UTC()), time.Now().UTC())

	if got := cap.to(c); len(got) != 0 {
		t.Errorf("C (de outro ws disjunto) NÃO pode receber presence de A: %+v", got)
	}
}

// CA-02.1 — A em W1∩W2, rodando um timer num projeto de W1 → co-membro de W1
// recebe running:true; co-membro SÓ de W2 recebe NADA (running:true). Mutação
// que troca o filtro m1.workspace_id=$entry por todas as memberships de A faz o
// membro de W2 receber running:true e mata este teste.
func TestOnTimerRunningEscopadoAoWorkspaceDaEntry(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "scope-a")
	b1 := mkUser(t, pool, "scope-b1") // só W1
	b2 := mkUser(t, pool, "scope-b2") // só W2
	w1 := mkWorkspace(t, pool, "W1")
	w2 := mkWorkspace(t, pool, "W2")
	mkMember(t, pool, w1, a, "owner")
	mkMember(t, pool, w2, a, "owner")
	mkMember(t, pool, w1, b1, "member")
	mkMember(t, pool, w2, b2, "member")
	proj := mkProject(t, pool, w1, a, "Site W1", "shared")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(w1, a, &proj, time.Now().UTC()), time.Now().UTC())

	// b1 (co-membro de W1) recebe running:true
	if got := cap.inWS(b1, w1); len(got) != 1 || !got[0].Running {
		t.Errorf("b1 (W1) deveria receber running:true: %+v", got)
	}
	// b2 (só W2) NÃO recebe running:true
	for _, p := range cap.to(b2) {
		if p.Running {
			t.Errorf("b2 (só W2) NÃO pode receber running:true de A rodando em W1: %+v", p)
		}
	}
}

// CA-08 — pausa (mode='break') derruba a presence: running:false em todas as
// memberships de A.
func TestOnTimerBreakDerrubaPresence(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "brk-a")
	b := mkUser(t, pool, "brk-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	brk := &timer.State{Mode: "break"}
	svc.OnTimer(a, brk, time.Now().UTC())

	got := cap.inWS(b, ws)
	if len(got) != 1 {
		t.Fatalf("B deveria receber 1 presence (running:false) no break: %d", len(got))
	}
	if got[0].Running {
		t.Errorf("pausa deveria reportar running:false: %+v", got[0])
	}
	if got[0].Since != nil || got[0].Project != nil {
		t.Errorf("since/project devem ser nil quando !running: %+v", got[0])
	}
}

// CA-09 / CA-04 / CA-16 — stop/corte (state==nil) → running:false em TODAS as
// memberships de A (idempotente — co-membros de W1 E de W2 recebem false).
func TestOnTimerNilDerrubaEmTodasMemberships(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "nil-a")
	b1 := mkUser(t, pool, "nil-b1")
	b2 := mkUser(t, pool, "nil-b2")
	w1 := mkWorkspace(t, pool, "W1")
	w2 := mkWorkspace(t, pool, "W2")
	mkMember(t, pool, w1, a, "owner")
	mkMember(t, pool, w2, a, "owner")
	mkMember(t, pool, w1, b1, "member")
	mkMember(t, pool, w2, b2, "member")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, nil, time.Now().UTC())

	for _, b := range []struct {
		id, ws string
	}{{b1, w1}, {b2, w2}} {
		got := cap.inWS(b.id, b.ws)
		if len(got) != 1 || got[0].Running {
			t.Errorf("%s (ws %s) deveria receber running:false: %+v", b.id, b.ws, got)
		}
	}
	// A nunca recebe de si
	if got := cap.to(a); len(got) != 0 {
		t.Errorf("A não pode receber presence de si no stop: %+v", got)
	}
}

// CA-07 — timer sem projeto (Entry.ProjectID==nil) → running:true sem project.
func TestOnTimerSemProjeto(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "nop-a")
	b := mkUser(t, pool, "nop-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, nil, time.Now().UTC()), time.Now().UTC())

	got := cap.inWS(b, ws)
	if len(got) != 1 || !got[0].Running {
		t.Fatalf("B deveria receber running:true sem projeto: %+v", got)
	}
	if got[0].Project != nil {
		t.Errorf("timer sem projeto não pode ter project: %+v", got[0].Project)
	}
}

// CA-06 (T-04) — projeto PRIVATE de A não vaza o nome: B recebe running:true SEM
// project. Mutação ignorando a visibilidade (sempre inclui o nome) mata o teste.
func TestOnTimerProjetoPrivadoNaoVazaNome(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "priv-a")
	b := mkUser(t, pool, "priv-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	proj := mkProject(t, pool, ws, a, "Segredo", "private")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, &proj, time.Now().UTC()), time.Now().UTC())

	got := cap.inWS(b, ws)
	if len(got) != 1 || !got[0].Running {
		t.Fatalf("B deveria receber running:true: %+v", got)
	}
	if got[0].Project != nil {
		t.Errorf("projeto PRIVADO de A NÃO pode vazar o nome pro membro B: %+v", *got[0].Project)
	}
}

// CA-05 (T-04) — projeto SHARED inclui o nome (espelho do privado).
func TestOnTimerProjetoSharedMostraNome(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "shr-a")
	b := mkUser(t, pool, "shr-b")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	mkMember(t, pool, ws, b, "member")
	proj := mkProject(t, pool, ws, a, "Público", "shared")

	cap := &capture{}
	svc := newService(pool, cap, time.Now().UTC())
	svc.OnTimer(a, runningState(ws, a, &proj, time.Now().UTC()), time.Now().UTC())

	got := cap.inWS(b, ws)
	if len(got) != 1 || got[0].Project == nil || *got[0].Project != "Público" {
		t.Fatalf("projeto shared deveria aparecer com nome: %+v", got)
	}
}

// CA-10 / RNF-01 — best-effort: um erro de query (pool fechado) NÃO entra em
// panic e NÃO propaga; nenhum broadcast acontece.
func TestOnTimerBestEffortNaoPanica(t *testing.T) {
	pool := dbtest.NewPool(t)
	a := mkUser(t, pool, "be-a")
	ws := mkWorkspace(t, pool, "W")
	mkMember(t, pool, ws, a, "owner")
	pool.Close() // força erro de query no fan-out

	cap := &capture{}
	svc := presence.NewService(pool, cap.fn)
	// não pode entrar em panic
	svc.OnTimer(a, runningState(ws, a, nil, time.Now().UTC()), time.Now().UTC())

	if got := len(cap.calls); got != 0 {
		t.Errorf("nenhum broadcast deveria acontecer com pool fechado, veio %d", got)
	}
}
