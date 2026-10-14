// realtime_routes_test.go — T-03 da fase 04: o upgrade autenticado do
// GET /api/v1/ws (CA-05), o descarte de mensagens do cliente (CA-03) e a
// remoção do registro nos dois caminhos de morte da conexão (CA-10: close
// do cliente e timeout de ping — D-08, intervalos injetados).
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"timetrack.live/backend/internal/db/dbtest"
	"timetrack.live/backend/internal/realtime"
)

// dialWS abre uma conexão websocket autenticada contra o servidor de teste.
func dialWS(t *testing.T, ts *httptest.Server, cookie *http.Cookie) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header := http.Header{}
	if cookie != nil {
		header.Set("Cookie", cookie.Name+"="+cookie.Value)
	}
	c, _, err := websocket.Dial(ctx, ts.URL+"/api/v1/ws", &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// aguarda até o registro do hub atingir o tamanho esperado (registro e
// desregistro acontecem na goroutine do handler — corrida benigna).
func waitConns(t *testing.T, hub *realtime.Hub, userID string, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Conns(userID) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("registro do hub não chegou a %d conexões (tem %d)", want, hub.Conns(userID))
}

// CA-05 — sem cookie de sessão válido, o /ws recusa com 401 no envelope
// canônico ANTES de qualquer upgrade (o RequireAuth responde; o handler
// nem roda).
func TestWSSemSessaoRecusa401(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, _, _, _, _ := newRealtimeRouter(t, pool, nil, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/ws")
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperava 401, veio %d", resp.StatusCode)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil || env.Error.Code != "unauthorized" {
		t.Errorf("esperava envelope canônico unauthorized, veio code=%q err=%v", env.Error.Code, err)
	}
}

// CA-03 — o WS é downstream-only: mensagem do cliente é drenada e
// DESCARTADA (plan §6: sem CloseRead) — não gera resposta, não muta nada e
// a conexão segue viva: o broadcast seguinte ainda chega.
func TestMensagemDoClienteEIgnoradaSemDerrubarAConexao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, nil, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// tentativa de mutação por dentro do socket — deve ser ignorada
	if err := c.Write(ctx, websocket.MessageText,
		[]byte(`{"type":"start","payload":{"description":"hack"}}`)); err != nil {
		t.Fatalf("write do cliente: %v", err)
	}
	// nada mudou no banco: nenhuma entry nasceu da mensagem
	time.Sleep(150 * time.Millisecond)
	if n := dbtest.CountRows(t, pool, "time_entries"); n != 0 {
		t.Errorf("mensagem do cliente criou %d entry(s) — o WS nunca muta", n)
	}

	// a conexão segue viva e drenando: o broadcast seguinte chega, e a
	// PRIMEIRA coisa lida é ele (nenhuma resposta à mensagem do cliente)
	evt := []byte(`{"type":"timer","payload":{"state":null}}`)
	hub.Broadcast(userID, evt)
	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("a conexão deveria seguir viva após a mensagem do cliente: %v", err)
	}
	if string(got) != string(evt) {
		t.Errorf("primeira leitura deveria ser o broadcast (sem resposta à mensagem), veio %q", got)
	}
	if n := hub.Conns(userID); n != 1 {
		t.Errorf("conexão deveria seguir registrada (conns=%d)", n)
	}
}

// CA-03 (borda do tamanho — verificação adversarial do build, 11/06) — a
// letra diz QUALQUER mensagem: uma data message acima dos 32KiB do read
// limit default da coder/websocket também não pode derrubar a conexão
// (sem o SetReadLimit(-1) + drain por Reader, a lib fecha com
// StatusMessageTooBig na primeira mensagem grande).
func TestMensagemGrandeDoClienteNaoDerrubaAConexao(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, nil, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, make([]byte, 40_000)); err != nil {
		t.Fatalf("write de 40KB: %v", err)
	}

	// a conexão segue viva, registrada e drenando: o broadcast chega
	time.Sleep(150 * time.Millisecond)
	if n := hub.Conns(userID); n != 1 {
		t.Fatalf("mensagem de 40KB derrubou a conexão (conns=%d) — CA-03 fala em QUALQUER mensagem", n)
	}
	evt := []byte(`{"type":"timer","payload":{"state":null}}`)
	hub.Broadcast(userID, evt)
	if _, got, err := c.Read(ctx); err != nil || string(got) != string(evt) {
		t.Errorf("broadcast pós-mensagem-grande deveria chegar, veio %q err=%v", got, err)
	}
}

// CA-10 (braço HTTP) — fechar a conexão pelo cliente remove o registro do
// hub: broadcasts seguintes não tentam entregá-la.
func TestCloseDoClienteRemoveDoHub(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, nil, 0, 0)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	if err := c.Close(websocket.StatusNormalClosure, "tchau"); err != nil {
		t.Fatalf("close: %v", err)
	}
	waitConns(t, hub, userID, 0)
}

// CA-10 (braço ping — D-08) — conexão que não responde pong é fechada pelo
// servidor E removida do hub (device morto não acumula). Intervalo de ping
// encurtado por injeção; o cliente NUNCA chama Read, então a lib nunca
// processa os pings — sem pong.
func TestConexaoSemPongEFechadaERemovida(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, nil, 50*time.Millisecond, 100*time.Millisecond)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	// sem Read do lado do cliente → sem pong → o servidor fecha e remove
	waitConns(t, hub, userID, 0)

	// e a conexão foi de fato encerrada na rede, não só no registro
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := c.Read(ctx); err == nil {
		t.Error("a conexão deveria estar fechada pelo servidor")
	}
}

// CA-10 (contraprova do braço ping — verificação adversarial do build,
// 11/06): conexão SAUDÁVEL sobrevive a vários ciclos de ping — o cliente
// que responde pong (a lib responde sozinha enquanto há um Read ativo)
// segue registrado e recebendo eventos. Sem este teste, uma mutação que
// fecha TODA conexão a cada tick de ping passava a suíte inteira.
func TestConexaoSaudavelSobreviveAosPings(t *testing.T) {
	pool := dbtest.NewPool(t)
	router, svc, stub, _, hub := newRealtimeRouter(t, pool, nil, 30*time.Millisecond, 60*time.Millisecond)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cookie := loginVia(t, svc, stub, "g-ana", "ana@tt.dev", "Ana")
	userID := userIDOf(t, pool, "g-ana")

	c := dialWS(t, ts, cookie)
	waitConns(t, hub, userID, 1)

	// Read ativo no cliente: processa os pings (e entrega data messages)
	msgs := make(chan []byte, 4)
	go func() {
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				close(msgs)
				return
			}
			msgs <- data
		}
	}()

	time.Sleep(200 * time.Millisecond) // ≥6 ciclos de ping de 30ms
	if n := hub.Conns(userID); n != 1 {
		t.Fatalf("conexão saudável deveria sobreviver aos pings (conns=%d)", n)
	}
	evt := []byte(`{"type":"timer","payload":{"state":null}}`)
	hub.Broadcast(userID, evt)
	select {
	case got, ok := <-msgs:
		if !ok {
			t.Fatal("a conexão morreu durante os ciclos de ping")
		}
		if string(got) != string(evt) {
			t.Errorf("broadcast pós-pings esperado %q, veio %q", evt, got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("broadcast não chegou após os ciclos de ping")
	}
}
