// realtime_test.go — T-04 da fase 04: a ponta a ponta backend — mutação
// REST commitada → evento nos sockets do usuário (httptest + websocket
// REAL). O payload do evento é o MESMO objeto canônico do GET /timer
// (CA-02): byte-idêntico após TrimSpace nas duas pontas (o Encoder.Encode
// do respond.JSON termina o corpo HTTP com '\n' — plan §3.2), com o clock
// congelado entre a mutação e o GET.
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/timer"
)

// readEvent lê 1 mensagem do socket dentro do prazo.
func readEvent(t *testing.T, c *websocket.Conn, within time.Duration) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("esperava evento no socket, veio erro: %v", err)
	}
	return data
}

// assertNoEvent garante que NADA chega no socket dentro do prazo.
func assertNoEvent(t *testing.T, c *websocket.Conn, within time.Duration, who string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	if _, data, err := c.Read(ctx); err == nil {
		t.Errorf("%s não deveria receber evento, veio %q", who, data)
	}
}

// decodeEnvelope abre o envelope versionável do RF-03.
func decodeEnvelope(t *testing.T, raw []byte) (string, json.RawMessage) {
	t.Helper()
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("evento fora do envelope do RF-03: %s", raw)
	}
	return env.Type, env.Payload
}

// CA-02/06/08 + CA-01 na camada HTTP — POST /timer/start → os 2 sockets do
// usuário recebem exatamente 1 evento {"type":"timer",...} com payload
// byte-idêntico ao corpo do GET /timer; o socket do OUTRO usuário não
// recebe nada.
func TestMutacaoRESTEmiteEventoNosSocketsDoUsuario(t *testing.T) {
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, clock.Now, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	cookieAna := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	anaID := userIDOf(t, pool, "g-ana")
	anaWS := workspaceIDOf(t, pool, "g-ana")
	biaCookie := loginVia(t, svc, stub, "g-bia", "bia@tt.dev", "Bia")
	biaID := userIDOf(t, pool, "g-bia")

	devA := dialWS(t, ts, cookieAna) // "celular"
	devB := dialWS(t, ts, cookieAna) // "iPad"
	devBia := dialWS(t, ts, biaCookie)
	waitConns(t, hub, anaID, 2)
	waitConns(t, hub, biaID, 1)

	payload := fmt.Sprintf(`{"workspace_id":%q,"description":"sync entre devices"}`, anaWS)
	rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookieAna, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}

	get := doJSON(router, http.MethodGet, "/api/v1/timer", cookieAna, "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /timer esperava 200, veio %d", get.Code)
	}
	want := bytes.TrimSpace(get.Body.Bytes())

	for name, c := range map[string]*websocket.Conn{"device A": devA, "device B": devB} {
		evt := readEvent(t, c, 5*time.Second)
		typ, evtPayload := decodeEnvelope(t, evt)
		if typ != "timer" {
			t.Errorf("%s: type esperado \"timer\", veio %q", name, typ)
		}
		if got := bytes.TrimSpace(evtPayload); !bytes.Equal(got, want) {
			t.Errorf("%s: payload do evento deveria ser byte-idêntico ao GET /timer (CA-02):\nevento: %s\nget:    %s",
				name, got, want)
		}
		// exatamente 1 evento por mutação (CA-06): nada mais chega
		assertNoEvent(t, c, 300*time.Millisecond, name+" (2º evento)")
	}

	// CA-01: a mutação de Ana NUNCA chega a socket de Bia
	assertNoEvent(t, devBia, 300*time.Millisecond, "socket da Bia")
}

// CA-06 (pause) — POST /timer/pause → evento com state.mode='break' e
// remaining_seconds da config persistida: o dado que destrava o CA-12 no
// front (BreakOverlay abre em todos os devices).
func TestPauseEmiteEventoDeBreakComConfig(t *testing.T) {
	pool := dbtest.NewPool(t)
	clock := newFakeClock(time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))
	router, svc, stub, tsvc, hub := newRealtimeRouter(t, pool, clock.Now, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")
	wsID := workspaceIDOf(t, pool, "g-ana")
	ctx := context.Background()
	if _, err := tsvc.SaveSettings(ctx, userID, timer.Settings{FocusMinutes: 25, BreakMinutes: 15}); err != nil {
		t.Fatalf("salvar settings: %v", err)
	}

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	payload := fmt.Sprintf(`{"workspace_id":%q,"description":"foco"}`, wsID)
	if rec := doJSON(router, http.MethodPost, "/api/v1/timer/start", cookie, payload); rec.Code != http.StatusCreated {
		t.Fatalf("start esperava 201, veio %d: %s", rec.Code, rec.Body.String())
	}
	_ = readEvent(t, c, 5*time.Second) // consome o evento do start
	clock.Advance(600 * time.Second)

	if rec := doJSON(router, http.MethodPost, "/api/v1/timer/pause", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("pause esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}

	evt := readEvent(t, c, 5*time.Second)
	typ, evtPayload := decodeEnvelope(t, evt)
	if typ != "timer" {
		t.Errorf("type esperado \"timer\", veio %q", typ)
	}
	var body timerResp
	if err := json.Unmarshal(evtPayload, &body); err != nil {
		t.Fatalf("payload fora do contrato do timer: %s", evtPayload)
	}
	if body.State == nil || body.State.Mode != "break" {
		t.Fatalf("evento do pause deveria ter state.mode='break', veio %s", evtPayload)
	}
	if body.State.RemainingSeconds == nil || *body.State.RemainingSeconds != 900 {
		t.Errorf("remaining esperado 900 (break 15min da config), veio %v", body.State.RemainingSeconds)
	}
	if body.StoppedEntry != nil {
		t.Error("o evento nunca carrega stopped_entry (D-06) — quem espelha estado não fecha entry")
	}

	// e o payload segue sendo o objeto canônico: idêntico ao GET na sequência
	get := doJSON(router, http.MethodGet, "/api/v1/timer", cookie, "")
	if got, want := bytes.TrimSpace(evtPayload), bytes.TrimSpace(get.Body.Bytes()); !bytes.Equal(got, want) {
		t.Errorf("payload do pause deveria ser byte-idêntico ao GET /timer (CA-02):\nevento: %s\nget:    %s", got, want)
	}
}
