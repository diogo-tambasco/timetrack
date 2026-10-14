// hook_test.go — T-02 da fase 04: o hook OnChange carrega o instante da
// mutação (D-04) e obedece "evento ⟺ commit" (CA-07) nos dois lados:
// mutação sem commit não emite; commit emite MESMO quando a resposta é 409
// (pausa órfã — o caso real herdado da 03, service.go resumeFromBreak).
package timer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/domain"
	"timetrack.live/backend/internal/timer"
)

// hookRecorder captura toda chamada do OnChange (assinatura da fase 04).
type hookRecorder struct {
	calls  int
	states []*timer.State
	ats    []time.Time
}

func (h *hookRecorder) fn() func(string, *timer.State, time.Time) {
	return func(_ string, st *timer.State, at time.Time) {
		h.calls++
		h.states = append(h.states, st)
		h.ats = append(h.ats, at)
	}
}

// CA-02 (parcial) — em TODA mutação commitada, o `at` entregue ao hook é
// IGUAL ao server_time do Result retornado (o relógio DA MUTAÇÃO, não o
// instante do broadcast): é ele que ordena WS × REST no apply() do front.
func TestHookRecebeOInstanteDaMutacao(t *testing.T) {
	svc, clock, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()
	rec := &hookRecorder{}
	svc.SetOnChange(rec.fn())

	assertAt := func(op string, res timer.Result, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		if rec.calls == 0 {
			t.Fatalf("%s: hook não disparou", op)
		}
		got := rec.ats[len(rec.ats)-1]
		if !got.Equal(res.ServerTime) {
			t.Errorf("%s: at do hook (%v) ≠ server_time do Result (%v)", op, got, res.ServerTime)
		}
	}

	res, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	assertAt("start", res, err)

	clock.Advance(90 * time.Second)
	res, err = svc.Pause(ctx, userID, nil)
	assertAt("pause", res, err)

	clock.Advance(30 * time.Second)
	res, err = svc.Resume(ctx, userID)
	assertAt("resume", res, err)

	clock.Advance(45 * time.Second)
	res, err = svc.Stop(ctx, userID)
	assertAt("stop", res, err)

	if rec.calls != 4 {
		t.Errorf("4 mutações deveriam emitir 4 eventos, emitiram %d", rec.calls)
	}
}

// CA-02 (a letra "não ao instante do broadcast" — verificação adversarial
// do build, 11/06): com um clock que AVANÇA a cada Now(), um call site que
// recompute s.now() no emit (em vez de repassar o now capturado no topo da
// mutação) produziria at ≠ server_time. O fakeClock estável do teste acima
// não distingue essa regressão — este distingue.
func TestAtEhDoTopoDaMutacaoNaoDoEmit(t *testing.T) {
	pool := dbtest.NewPool(t)
	userID, wsID, _ := seedBase(t, pool)
	var mu sync.Mutex
	current := t0
	stepping := func() time.Time { // cada leitura do relógio avança 1s
		mu.Lock()
		defer mu.Unlock()
		now := current
		current = current.Add(time.Second)
		return now
	}
	svc := timer.NewService(pool, timer.WithClock(stepping))
	rec := &hookRecorder{}
	svc.SetOnChange(rec.fn())
	ctx := context.Background()

	res, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !rec.ats[0].Equal(res.ServerTime) {
		t.Errorf("start: at (%v) ≠ server_time (%v) — algum caminho recomputou o relógio", rec.ats[0], res.ServerTime)
	}
	res, err = svc.Stop(ctx, userID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !rec.ats[1].Equal(res.ServerTime) {
		t.Errorf("stop: at (%v) ≠ server_time (%v) — algum caminho recomputou o relógio", rec.ats[1], res.ServerTime)
	}
}

// CA-07 lado "sem commit" — mutação que falha na validação ou em conflito
// SEM efeito não chama o hook: nenhum evento sem transação commitada.
func TestMutacaoSemCommitNaoEmiteEvento(t *testing.T) {
	svc, _, _, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()
	rec := &hookRecorder{}
	svc.SetOnChange(rec.fn())

	// pause sem timer rodando → 409 no_active_timer, nada commitado
	if _, err := svc.Pause(ctx, userID, nil); err == nil {
		t.Fatal("pause sem timer deveria falhar")
	}
	// start countdown sem alvo → 400 validation_error, nada criado
	if _, err := svc.Start(ctx, userID, timer.StartInput{
		WorkspaceID: wsID, Mode: "countdown",
	}); err == nil {
		t.Fatal("start countdown sem target deveria falhar")
	}

	if rec.calls != 0 {
		t.Errorf("mutações sem commit emitiram %d evento(s) — CA-07 violado", rec.calls)
	}
}

// CA-07 lado "commit com 409" — a exceção REAL herdada da 03: o resume de
// pausa órfã (resume_entry_id NULL, defesa em profundidade) COMMITA a
// remoção do estado, EMITE state nil com o at da mutação E responde
// 409 no_active_break. Este teste PINA o emit do resumeFromBreak contra
// "correções" futuras que o removam por causa do 409.
func TestPausaOrfaCommitaEmiteEResponde409(t *testing.T) {
	svc, clock, pool, userID, wsID, _ := newTimerService(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, userID, timer.StartInput{WorkspaceID: wsID, Description: "foco"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(60 * time.Second)
	if _, err := svc.Pause(ctx, userID, nil); err != nil {
		t.Fatalf("pause: %v", err)
	}
	// corrupção fora da API (único caminho até a pausa órfã — fase 03)
	if _, err := pool.Exec(ctx,
		`UPDATE timer_states SET resume_entry_id = NULL WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("anular resume_entry_id: %v", err)
	}

	rec := &hookRecorder{}
	svc.SetOnChange(rec.fn())
	clock.Advance(10 * time.Second)
	mutationNow := clock.Now()

	_, err := svc.Resume(ctx, userID)
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "no_active_break" {
		t.Fatalf("resume de pausa órfã deveria responder 409 no_active_break, veio %v", err)
	}

	if rec.calls != 1 {
		t.Fatalf("a remoção da pausa órfã COMMITA e deveria emitir exatamente 1 evento, emitiu %d", rec.calls)
	}
	if rec.states[0] != nil {
		t.Errorf("o evento da pausa órfã deveria carregar state nil, veio %+v", rec.states[0])
	}
	if !rec.ats[0].Equal(mutationNow) {
		t.Errorf("at do evento deveria ser o relógio da mutação (%v), veio %v", mutationNow, rec.ats[0])
	}

	// e o estado realmente morreu no banco (commit aconteceu)
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM timer_states WHERE user_id = $1`, userID).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("timer_states deveria zerar após a pausa órfã morrer (rows=%d err=%v)", rows, err)
	}
}
