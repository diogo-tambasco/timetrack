// dispatcher_test.go — TDD do despacho (fase 20 T-03), espelha o cutter_test do
// guard: limiar inclusivo/seleção (CA-04), flip pending→sent (CA-05), aviso só
// no despacho real (CA-06), payload do notice (CA-07), idempotência 2× RunOnce +
// corrida com cancel (CA-08), isolamento por user (CA-09), wiring best-effort do
// push (CA-10). Clock injetável (RNF-03) — o teste seta now em vez de esperar.
package reminders_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/reminders"
)

// seedReminder insere um reminder direto no banco (sem passar pelo Store) e
// devolve o id, pra o teste controlar status/horário.
func seedReminder(t *testing.T, pool *pgxpool.Pool, userID, taskID, wsID string, remindAt time.Time, kind, status string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind, status)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, taskID, wsID, remindAt, kind, status).Scan(&id); err != nil {
		t.Fatalf("seedReminder: %v", err)
	}
	return id
}

func remStatus(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM reminders WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("status do reminder %s: %v", id, err)
	}
	return s
}

// spy coleta as chamadas dos hooks (thread-safe — o RunOnce é serial mas vale a
// disciplina). noticePayloads guarda os payloads pra checar o kind/title/task.
type spy struct {
	mu             sync.Mutex
	noticeUsers    []string
	noticePayloads []any
	pushUsers      []string
	pushPayloads   [][]byte
}

func (s *spy) onNotice(userID string, payload any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noticeUsers = append(s.noticeUsers, userID)
	s.noticePayloads = append(s.noticePayloads, payload)
}
func (s *spy) onPush(userID string, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushUsers = append(s.pushUsers, userID)
	s.pushPayloads = append(s.pushPayloads, payload)
}

func newDispatcher(pool *pgxpool.Pool, now time.Time, s *spy) *reminders.Dispatcher {
	return reminders.NewDispatcher(pool,
		reminders.WithClock(func() time.Time { return now }),
		reminders.WithOnNotice(s.onNotice),
		reminders.WithOnPush(s.onPush),
	)
}

// CA-04: remind_at == now despacha; futuro NÃO; sent/canceled nunca entram.
func TestRunOnceLimiarESelecao(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-limiar", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	atNow := seedReminder(t, pool, u, task, ws, now, "manual", "pending")               // == now → despacha
	atPast := seedReminder(t, pool, u, task, ws, now.Add(-time.Hour), "due", "pending") // passado → despacha
	atFuture := seedReminder(t, pool, u, task, ws, now.Add(time.Hour), "manual", "pending")
	alreadySent := seedReminder(t, pool, u, task, ws, now.Add(-time.Hour), "manual", "sent")
	canceled := seedReminder(t, pool, u, task, ws, now.Add(-time.Hour), "manual", "canceled")

	s := &spy{}
	done, err := newDispatcher(pool, now, s).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(done) != 2 {
		t.Fatalf("deveria despachar 2 (now + passado), veio %d", len(done))
	}
	if remStatus(t, pool, atNow) != "sent" {
		t.Errorf("remind_at == now deveria despachar (limiar inclusivo — CA-04)")
	}
	if remStatus(t, pool, atPast) != "sent" {
		t.Errorf("remind_at no passado deveria despachar")
	}
	if remStatus(t, pool, atFuture) != "pending" {
		t.Errorf("remind_at no futuro NÃO deveria despachar")
	}
	if remStatus(t, pool, alreadySent) != "sent" || remStatus(t, pool, canceled) != "canceled" {
		t.Errorf("sent/canceled não devem entrar na varredura nem mudar")
	}
}

// CA-05: despachado vira sent; remind_at/kind/task_id/user_id intactos.
func TestRunOnceFlipPreservaCampos(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-flip", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	id := seedReminder(t, pool, u, task, ws, at, "due", "pending")

	s := &spy{}
	if _, err := newDispatcher(pool, now, s).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	var status, kind, taskID, userID string
	var remindAt time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT status, kind, task_id, user_id, remind_at FROM reminders WHERE id = $1`, id).
		Scan(&status, &kind, &taskID, &userID, &remindAt); err != nil {
		t.Fatalf("reload reminder: %v", err)
	}
	if status != "sent" {
		t.Errorf("status deveria ser sent, veio %q", status)
	}
	if kind != "due" || taskID != task || userID != u || !remindAt.Equal(at) {
		t.Errorf("flip deveria preservar kind/task_id/user_id/remind_at (CA-05)")
	}
}

// CA-06: aviso só pros despachados; passada sem vencido = zero notice/push.
func TestRunOnceAvisoSoNoDespacho(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-aviso", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	seedReminder(t, pool, u, task, ws, now.Add(time.Hour), "manual", "pending") // só futuro

	s := &spy{}
	done, err := newDispatcher(pool, now, s).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("passada sem vencido não deveria despachar nada, veio %d", len(done))
	}
	if len(s.noticeUsers) != 0 || len(s.pushUsers) != 0 {
		t.Errorf("passada sem vencido = zero notice (%d) e zero push (%d) — CA-06", len(s.noticeUsers), len(s.pushUsers))
	}
}

// CA-07: payload do notice = {kind:"reminder", task_id, title=nome da task}.
func TestRunOncePayloadDoNotice(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-payload", "Entregar relatório")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	seedReminder(t, pool, u, task, ws, now.Add(-time.Minute), "manual", "pending")

	s := &spy{}
	if _, err := newDispatcher(pool, now, s).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(s.noticePayloads) != 1 {
		t.Fatalf("esperava 1 notice, veio %d", len(s.noticePayloads))
	}
	m, ok := s.noticePayloads[0].(map[string]any)
	if !ok {
		t.Fatalf("payload do notice deveria ser map, veio %T", s.noticePayloads[0])
	}
	if m["kind"] != "reminder" || m["task_id"] != task || m["title"] != "Entregar relatório" {
		t.Errorf("payload do notice errado (CA-07): %v", m)
	}
}

// CA-08 (idempotência): 2× RunOnce → exatamente 1 notice/push pro reminder.
func TestRunOnceIdempotente(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-idem", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	id := seedReminder(t, pool, u, task, ws, now.Add(-time.Minute), "manual", "pending")

	s := &spy{}
	d := newDispatcher(pool, now, s)
	ctx := context.Background()
	d.RunOnce(ctx)
	d.RunOnce(ctx) // 2ª passada: já sent → não acha pending

	if remStatus(t, pool, id) != "sent" {
		t.Errorf("reminder deveria estar sent")
	}
	if len(s.noticeUsers) != 1 || len(s.pushUsers) != 1 {
		t.Errorf("2× RunOnce deveria render exatamente 1 notice (%d) e 1 push (%d) — CA-08", len(s.noticeUsers), len(s.pushUsers))
	}
}

// CA-08 (corrida): cancelar o reminder ANTES do despacho → o FOR UPDATE/status
// aborta, zero aviso. Simula a corrida marcando canceled entre a seleção e o
// commit (aqui: cancelado antes do RunOnce, mas selecionado pela leitura barata
// — provado pinando que o flip não acontece). Usamos um now que selecionaria o
// reminder mas o status mudou.
func TestRunOnceCorridaComCancel(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-corrida", "T")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	// pending e vencido: passaria na seleção...
	id := seedReminder(t, pool, u, task, ws, now.Add(-time.Minute), "manual", "pending")
	// ...mas é cancelado antes do despacho (o FOR UPDATE re-check vê canceled).
	if _, err := pool.Exec(context.Background(),
		`UPDATE reminders SET status = 'canceled' WHERE id = $1`, id); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	s := &spy{}
	done, err := newDispatcher(pool, now, s).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(done) != 0 || len(s.noticeUsers) != 0 {
		t.Errorf("reminder cancelado não deveria ser despachado nem avisado (CA-08)")
	}
	if remStatus(t, pool, id) != "canceled" {
		t.Errorf("o flip não deveria sobrescrever o canceled")
	}
}

// CA-09: 2 usuários com reminders vencidos → cada notice/push vai pro DONO certo.
func TestRunOnceIsolamentoPorUser(t *testing.T) {
	pool := dbtest.NewPool(t)
	uA, wsA, taskA := seedTask(t, pool, "g-iso-a", "Tarefa A")
	uB, wsB, taskB := seedTask(t, pool, "g-iso-b", "Tarefa B")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	seedReminder(t, pool, uA, taskA, wsA, now.Add(-time.Minute), "manual", "pending")
	seedReminder(t, pool, uB, taskB, wsB, now.Add(-time.Minute), "manual", "pending")

	s := &spy{}
	if _, err := newDispatcher(pool, now, s).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(s.noticeUsers) != 2 || len(s.pushUsers) != 2 {
		t.Fatalf("esperava 2 notice/2 push, veio %d/%d", len(s.noticeUsers), len(s.pushUsers))
	}
	// cada aviso vai pro DONO do reminder, com o título da task DELE.
	for i, payload := range s.noticePayloads {
		m := payload.(map[string]any)
		owner := s.noticeUsers[i]
		switch owner {
		case uA:
			if m["title"] != "Tarefa A" {
				t.Errorf("aviso de A deveria levar 'Tarefa A', veio %v", m["title"])
			}
		case uB:
			if m["title"] != "Tarefa B" {
				t.Errorf("aviso de B deveria levar 'Tarefa B', veio %v", m["title"])
			}
		default:
			t.Errorf("aviso foi pro user errado: %s (CA-09)", owner)
		}
	}
}

// CA-10 (wiring best-effort): o onPush recebe (userID, payload) corretos por
// reminder — o best-effort real é do Sender (fase 16); aqui prova o wiring.
func TestRunOnceWiringDoPush(t *testing.T) {
	pool := dbtest.NewPool(t)
	u, ws, task := seedTask(t, pool, "g-push", "Pagar boleto")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	seedReminder(t, pool, u, task, ws, now.Add(-time.Minute), "manual", "pending")

	s := &spy{}
	if _, err := newDispatcher(pool, now, s).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(s.pushUsers) != 1 || s.pushUsers[0] != u {
		t.Fatalf("onPush deveria receber o user dono, veio %v", s.pushUsers)
	}
	// o payload é o JSON do push.Payload com kind reminder e body = nome da task.
	body := string(s.pushPayloads[0])
	if !strings.Contains(body, `"kind":"reminder"`) || !strings.Contains(body, "Pagar boleto") {
		t.Errorf("push payload deveria levar kind reminder + nome da task, veio %s", body)
	}
}
