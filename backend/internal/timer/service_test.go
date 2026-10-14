package timer_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// fakeClock dá ao teste o controle do relógio do servidor (D-01): o tempo
// só anda quando o teste manda.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{t: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// t0 é um instante fixo qualquer — o relógio é todo injetado.
var t0 = time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)

// seedBase cria user + workspace + project (fixtures das fases 01/02).
func seedBase(t *testing.T, pool *pgxpool.Pool) (userID, wsID, projectID string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-ana', 'ana@tt.dev', 'Ana') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ('WS') RETURNING id`).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		wsID, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, color, visibility, owner_user_id)
		 VALUES ($1, 'App', '#4f9dde', 'shared', $2) RETURNING id`, wsID, userID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return userID, wsID, projectID
}

func newTimerService(t *testing.T) (*timer.Service, *fakeClock, *pgxpool.Pool, string, string, string) {
	t.Helper()
	pool := dbtest.NewPool(t)
	userID, wsID, projectID := seedBase(t, pool)
	clock := newFakeClock(t0)
	svc := timer.NewService(pool, timer.WithClock(clock.Now))
	return svc, clock, pool, userID, wsID, projectID
}

// CA-04 — start count up cria entry rodando com started_at do RELÓGIO DO
// SERVIDOR e retorna o estado canônico; o hook OnChange dispara com ele.
func TestStartCountUpCriaEntryRodando(t *testing.T) {
	svc, _, pool, userID, wsID, projectID := newTimerService(t)
	ctx := context.Background()

	var hookState *timer.State
	hookCalls := 0
	// assinatura +at da fase 04 (D-04) — edição mecânica sancionada (CA-04)
	svc.SetOnChange(func(uid string, st *timer.State, _ time.Time) {
		hookCalls++
		if uid != userID {
			t.Errorf("OnChange com userID errado: %s", uid)
		}
		hookState = st
	})

	res, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID,
		Description: "Revisão de laudas — lote 14",
		ProjectID:   &projectID,
		Tags:        []string{"deep-work"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	st := res.State
	if st == nil || st.Mode != "countup" {
		t.Fatalf("esperava state.mode='countup', veio %+v", st)
	}
	if st.Entry == nil {
		t.Fatal("state.entry deveria vir preenchido com a entry rodando")
	}
	if !st.Entry.StartedAt.Equal(t0) {
		t.Errorf("started_at deveria ser o relógio do servidor (%v), veio %v", t0, st.Entry.StartedAt)
	}
	if st.Entry.StoppedAt != nil {
		t.Error("entry recém-iniciada não pode ter stopped_at")
	}
	if st.Entry.Source != "timer" {
		t.Errorf("source esperado 'timer', veio %q", st.Entry.Source)
	}
	if st.ElapsedSeconds != 0 {
		t.Errorf("elapsed no instante do start deveria ser 0, veio %d", st.ElapsedSeconds)
	}
	if !res.ServerTime.Equal(t0) {
		t.Errorf("server_time deveria ser o relógio injetado, veio %v", res.ServerTime)
	}

	var running int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM time_entries WHERE user_id = $1 AND stopped_at IS NULL`,
		userID).Scan(&running); err != nil || running != 1 {
		t.Errorf("deveria existir exatamente 1 entry rodando, veio %d (err=%v)", running, err)
	}
	if n := dbtest.CountRows(t, pool, "timer_states"); n != 1 {
		t.Errorf("start deveria criar a linha de estado, veio %d", n)
	}
	if hookCalls != 1 || hookState == nil || hookState.Mode != "countup" {
		t.Errorf("OnChange deveria disparar 1x com o estado novo (calls=%d state=%+v)", hookCalls, hookState)
	}
}

// CA-06 — stop grava stopped_at e duration_seconds calculados pelo relógio
// do servidor; o estado vira nulo e a linha de runtime some.
func TestStopFechaEntryComDuracaoDoServidor(t *testing.T) {
	svc, clock, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(5070 * time.Second)

	res, err := svc.Stop(ctx, userID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if res.State != nil {
		t.Errorf("após o stop o estado deveria ser nulo, veio %+v", res.State)
	}
	e := res.StoppedEntry
	if e == nil {
		t.Fatal("stop deveria devolver a entry finalizada")
	}
	if e.StoppedAt == nil || !e.StoppedAt.Equal(t0.Add(5070*time.Second)) {
		t.Errorf("stopped_at deveria ser t0+5070s, veio %v", e.StoppedAt)
	}
	if e.DurationSeconds == nil || *e.DurationSeconds != 5070 {
		t.Errorf("duration_seconds esperado 5070, veio %v", e.DurationSeconds)
	}
	if n := dbtest.CountRows(t, pool, "timer_states"); n != 0 {
		t.Errorf("stop deveria remover a linha de runtime, sobraram %d", n)
	}

	// estado consultado depois segue ocioso (CA-23 na visão do service)
	st, err := svc.State(ctx, userID)
	if err != nil || st.State != nil {
		t.Errorf("estado pós-stop deveria ser nulo (err=%v state=%+v)", err, st.State)
	}
}

// CA-02 — duration_seconds (e qualquer campo estranho) enviado pelo cliente
// é DESCARTADO no decode do payload: a duração final é sempre
// stopped_at − started_at do relógio do servidor.
func TestDuracaoDoClienteIgnorada(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	// payload como chega do cliente — com duration_seconds malicioso
	var in timer.StartInput
	payload := `{"workspace_id":"` + wsID + `","description":"x","duration_seconds":999999}`
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, err := svc.Start(ctx, userID, in); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(90 * time.Second)
	res, err := svc.Stop(ctx, userID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if res.StoppedEntry.DurationSeconds == nil || *res.StoppedEntry.DurationSeconds != 90 {
		t.Errorf("duração deveria ser 90s (do relógio do servidor), veio %v — o cliente NÃO manda na duração",
			res.StoppedEntry.DurationSeconds)
	}
}

// CA-03.1 — user_id no payload é ignorado: a entry nasce SEMPRE em nome do
// usuário da sessão.
func TestUserIDDoPayloadIgnorado(t *testing.T) {
	svc, _, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	var bia string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, name) VALUES ('g-bia', 'bia@tt.dev', 'Bia') RETURNING id`).Scan(&bia); err != nil {
		t.Fatalf("seed bia: %v", err)
	}

	var in timer.StartInput
	payload := `{"workspace_id":"` + wsID + `","description":"x","user_id":"` + bia + `"}`
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := svc.Start(ctx, userID, in); err != nil {
		t.Fatalf("start: %v", err)
	}

	var owner string
	if err := pool.QueryRow(ctx,
		`SELECT user_id FROM time_entries WHERE stopped_at IS NULL`).Scan(&owner); err != nil {
		t.Fatalf("ler entry rodando: %v", err)
	}
	if owner != userID {
		t.Errorf("entry deveria nascer do usuário da sessão (%s), nasceu de %s", userID, owner)
	}
}

// CA-20 — projeto/task/tags/descrição são opcionais: start "pelado" inicia.
func TestStartSemProjetoNemTask(t *testing.T) {
	svc, _, _, userID, wsID, _ := newTimerService(t)

	res, err := svc.Start(context.Background(), userID, timer.StartInput{WorkspaceID: wsID})
	if err != nil {
		t.Fatalf("start sem projeto/task deveria iniciar: %v", err)
	}
	e := res.State.Entry
	if e.ProjectID != nil || e.TaskID != nil {
		t.Errorf("entry sem projeto/task deveria ter os dois nulos: %+v", e)
	}
	if e.Description != "" || len(e.Tags) != 0 {
		t.Errorf("descrição/tags deveriam ficar vazias: %+v", e)
	}
}
