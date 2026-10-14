// realtime_mutations_test.go — T-05 da fase 04: a tabela COMPLETA de
// mutações emitindo exatamente 1 evento por socket (CA-06) e a corrida
// determinística do ciclo pomodoro com 2 devices (CA-17 — braço inviável
// em E2E pelos mínimos de foco/pausa).
//
// EXCEÇÃO DOCUMENTADA AO RED ESTRUTURAL (tasks.md T-05): a fase 03 já chama
// onChange em toda mutação e o hub foi plugado no T-04 — a tabela nasce
// verde por construção. A validade foi provada por MUTAÇÃO de call site
// antes do commit: remover o s.onChange(...) do Start (service.go:294) e o
// do Stop (service.go:338), um por vez, fez os passos correspondentes da
// tabela falharem (evento nunca chega); restaurados, tudo verde.
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// readTimerEvent lê 1 evento, valida o envelope e devolve o payload decodificado.
func readTimerEvent(t *testing.T, c *websocket.Conn, who string) timerResp {
	t.Helper()
	evt := readEvent(t, c, 5*time.Second)
	typ, payload := decodeEnvelope(t, evt)
	if typ != "timer" {
		t.Errorf("%s: type esperado \"timer\", veio %q", who, typ)
	}
	var body timerResp
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("%s: payload fora do contrato do timer: %s", who, payload)
	}
	return body
}

// CA-06 — cada operação da tabela entrega exatamente 1 evento por socket
// com o estado esperado; a troca via start com timer ativo emite SÓ o
// estado final (a mutação é uma transação única na 03).
func TestTodaMutacaoEmiteUmEvento(t *testing.T) {
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC))
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, clock.Now, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")
	wsID := workspaceIDOf(t, pool, "g-ana")

	devA := dialWS(t, ts, cookie)
	devB := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 2)

	do := func(t *testing.T, method, path, payload string, wantCode int) timerResp {
		t.Helper()
		rec := doJSON(router, method, path, cookie, payload)
		if rec.Code != wantCode {
			t.Fatalf("%s %s esperava %d, veio %d: %s", method, path, wantCode, rec.Code, rec.Body.String())
		}
		return decodeTimer(t, rec)
	}

	var stoppedID string // alimentado pelo stop, usado pelo continue

	steps := []struct {
		name  string
		op    func(t *testing.T)
		check func(t *testing.T, got timerResp, who string)
	}{
		{
			name: "start",
			op: func(t *testing.T) {
				do(t, http.MethodPost, "/api/v1/timer/start",
					fmt.Sprintf(`{"workspace_id":%q,"description":"primeira"}`, wsID), http.StatusCreated)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "countup" || got.State.Entry == nil ||
					got.State.Entry.Description != "primeira" {
					t.Errorf("%s: evento do start deveria ser countup rodando com 'primeira', veio %+v", who, got.State)
				}
			},
		},
		{
			name: "troca via start com timer ativo",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				do(t, http.MethodPost, "/api/v1/timer/start",
					fmt.Sprintf(`{"workspace_id":%q,"description":"segunda"}`, wsID), http.StatusCreated)
			},
			check: func(t *testing.T, got timerResp, who string) {
				// 1 evento SÓ, do estado FINAL — um evento intermediário do
				// stop implícito (state null) faria este check falhar
				if got.State == nil || got.State.Entry == nil || got.State.Entry.Description != "segunda" {
					t.Errorf("%s: a troca emite só o estado final ('segunda' rodando), veio %+v", who, got.State)
				}
			},
		},
		{
			name: "pause",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				do(t, http.MethodPost, "/api/v1/timer/pause", "", http.StatusOK)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "break" || got.State.Resume == nil {
					t.Errorf("%s: evento do pause deveria ser break com resume, veio %+v", who, got.State)
				}
			},
		},
		{
			name: "resume",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				do(t, http.MethodPost, "/api/v1/timer/resume", "", http.StatusOK)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "countup" || got.State.Entry == nil ||
					got.State.Entry.Description != "segunda" {
					t.Errorf("%s: evento do resume deveria ser 'segunda' rodando de novo, veio %+v", who, got.State)
				}
			},
		},
		{
			name: "pause de novo",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				do(t, http.MethodPost, "/api/v1/timer/pause", "", http.StatusOK)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "break" {
					t.Errorf("%s: evento do 2º pause deveria ser break, veio %+v", who, got.State)
				}
			},
		},
		{
			name: "skip",
			op: func(t *testing.T) {
				do(t, http.MethodPost, "/api/v1/timer/break/skip", "", http.StatusOK)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "countup" || got.State.Entry == nil ||
					got.State.Entry.Description != "segunda" {
					t.Errorf("%s: evento do skip deveria ser 'segunda' rodando, veio %+v", who, got.State)
				}
			},
		},
		{
			name: "stop",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				body := do(t, http.MethodPost, "/api/v1/timer/stop", "", http.StatusOK)
				if body.StoppedEntry == nil {
					t.Fatalf("stop deveria devolver stopped_entry")
				}
				stoppedID = body.StoppedEntry.ID
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State != nil {
					t.Errorf("%s: evento do stop deveria ter state null, veio %+v", who, got.State)
				}
			},
		},
		{
			name: "continue",
			op: func(t *testing.T) {
				clock.Advance(60 * time.Second)
				do(t, http.MethodPost,
					fmt.Sprintf("/api/v1/workspaces/%s/entries/%s/continue", wsID, stoppedID), "", http.StatusCreated)
			},
			check: func(t *testing.T, got timerResp, who string) {
				if got.State == nil || got.State.Mode != "countup" || got.State.Entry == nil ||
					got.State.Entry.Description != "segunda" {
					t.Errorf("%s: evento do continue deveria clonar 'segunda' rodando, veio %+v", who, got.State)
				}
			},
		},
	}

	// "exatamente 1 evento" SEM matar a conexão (Read com ctx expirado fecha
	// a conn na coder/websocket): depois de cada mutação um MARCADOR é
	// broadcast direto no hub — FIFO por conexão garante que um 2º evento da
	// mutação apareceria ANTES do marcador na leitura seguinte.
	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			step.op(t)
			marker := fmt.Appendf(nil, `{"type":"marker-%d"}`, i)
			hub.Broadcast(userID, marker)
			for who, c := range map[string]*websocket.Conn{"device A": devA, "device B": devB} {
				got := readTimerEvent(t, c, who)
				step.check(t, got, who)
				if next := readEvent(t, c, 5*time.Second); !bytes.Equal(next, marker) {
					t.Errorf("%s: depois do evento de %q deveria vir SÓ o marcador (1 evento por mutação — CA-06), veio %s",
						who, step.name, next)
				}
			}
		})
	}
}

// CA-17 (braço determinístico do ciclo) — pausa pomodoro EXPIRADA + 2
// POST /timer/resume concorrentes (os 2 devices disparando o auto-resume ao
// mesmo tempo): exatamente 1 entry nova, 1 resposta 409 no_active_break e
// exatamente 1 evento por socket — o commit do vencedor (o 409 do perdedor
// não commita nada, logo não emite — CA-07). É o braço com risco de entry
// duplicada (área do bug HIGH do bloco 2 da 03).
func TestCorridaDoCicloPomodoroResumesConcorrentes(t *testing.T) {
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC))
	router, svc, stub, tsvc, hub := newRealtimeRouter(t, pool, clock.Now, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")
	wsID := workspaceIDOf(t, pool, "g-ana")
	if _, err := tsvc.SaveSettings(context.Background(), userID,
		timer.Settings{FocusMinutes: 25, BreakMinutes: 5}); err != nil {
		t.Fatalf("salvar settings: %v", err)
	}

	devA := dialWS(t, ts, cookie)
	devB := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 2)

	// foco pomodoro rodando → pausa automática de 5min
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie,
		fmt.Sprintf(`{"workspace_id":%q,"description":"ciclo","mode":"pomodoro"}`, wsID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("start pomodoro esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	clock.Advance(10 * time.Minute)
	if rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("pause esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range []*websocket.Conn{devA, devB} { // consome start + pause
		_ = readEvent(t, c, 5*time.Second)
		_ = readEvent(t, c, 5*time.Second)
	}

	// pausa EXPIRADA: 6min > alvo de 5min → os 2 devices disparam o resume
	clock.Advance(6 * time.Minute)
	before := dbtest.CountRows(t, pool, "time_entries")

	recs := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs <- doJSON(router, http.MethodPost, "/api/v1/timer/resume", cookie, "")
		}()
	}
	wg.Wait()
	close(recs)

	var got []int
	var loser *httptest.ResponseRecorder
	for rec := range recs {
		got = append(got, rec.Code)
		if rec.Code == http.StatusConflict {
			loser = rec
		}
	}
	if !((got[0] == http.StatusOK && got[1] == http.StatusConflict) ||
		(got[0] == http.StatusConflict && got[1] == http.StatusOK)) {
		t.Fatalf("a corrida deveria dar exatamente 1×200 + 1×409, veio %v", got)
	}
	if code := errCode(t, loser); code != "no_active_break" {
		t.Errorf("o perdedor deveria receber 409 no_active_break, veio %q", code)
	}

	// exatamente 1 entry nova (o foco retomado) — nada duplicado
	if after := dbtest.CountRows(t, pool, "time_entries"); after != before+1 {
		t.Errorf("a corrida deveria criar exatamente 1 entry nova, criou %d", after-before)
	}

	// exatamente 1 evento por socket: o commit do vencedor, foco retomado
	for who, c := range map[string]*websocket.Conn{"device A": devA, "device B": devB} {
		got := readTimerEvent(t, c, who)
		if got.State == nil || got.State.Mode != "pomodoro" || got.State.Entry == nil ||
			got.State.Entry.Description != "ciclo" {
			t.Errorf("%s: o evento da corrida deveria ser o foco 'ciclo' retomado, veio %+v", who, got.State)
		}
		if got.State != nil && (got.State.RemainingSeconds == nil || *got.State.RemainingSeconds != 900) {
			t.Errorf("%s: foco retomado deveria ter remaining 900 (25min − 10min focados), veio %v",
				who, got.State.RemainingSeconds)
		}
		assertNoEvent(t, c, 250*time.Millisecond, who+" (evento extra da corrida)")
	}
}
