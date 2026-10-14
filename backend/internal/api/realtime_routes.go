// realtime_routes.go — wiring da fase 04: GET /api/v1/ws autenticado faz o
// upgrade WebSocket e registra a conexão no hub sob o usuário da sessão
// (CA-05 é resolvido pelo RequireAuth antes do handler rodar). O socket é
// downstream-only (D-01): mensagens do cliente são drenadas e DESCARTADAS
// (CA-03 — NUNCA Conn.CloseRead, plan §6: ele derruba a conexão na 1ª data
// message); eventos saem pelo adapter wsSink (buffer 8 — D-05) e o ping do
// servidor libera device morto (D-08).
package api

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"timetrack.live/backend/internal/realtime"
	"timetrack.live/backend/internal/reqctx"
)

// Defaults do D-08: ping a cada 30s; sem pong em 2 ciclos → fecha.
const (
	defaultWSPingInterval = 30 * time.Second
	defaultWSPingTimeout  = 2 * defaultWSPingInterval
	wsWriteTimeout        = 10 * time.Second
)

// WithRealtime liga o endpoint /ws da fase 04 no router.
func WithRealtime(hub *realtime.Hub) Option {
	return func(c *routerCfg) { c.hub = hub }
}

// WithWSPing injeta os prazos do ping do /ws (testes — D-08; 0 = default).
func WithWSPing(interval, timeout time.Duration) Option {
	return func(c *routerCfg) {
		c.wsPingInterval = interval
		c.wsPingTimeout = timeout
	}
}

func mountRealtime(r chi.Router, cfg *routerCfg) {
	interval, timeout := cfg.wsPingInterval, cfg.wsPingTimeout
	if interval <= 0 {
		interval = defaultWSPingInterval
	}
	if timeout <= 0 {
		timeout = defaultWSPingTimeout
	}
	r.Get("/ws", handleWS(cfg.hub, interval, timeout))
}

// handleWS — upgrade, registro no hub e os dois laços da conexão: leitura
// (drena e descarta — CA-03) e escrita (eventos do sink + ping — D-08).
// Qualquer morte — close do cliente, sem pong, backpressure (D-05), erro de
// escrita — desregistra do hub e derruba a conexão (CA-10).
func handleWS(hub *realtime.Hub, pingInterval, pingTimeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// origem: default da lib — Origin ausente (cliente não-browser) ou
		// igual ao Host da requisição (plan §3.1); sem OriginPatterns extras
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		userID := reqctx.UserID(r.Context())
		sink := newWSSink(realConn{c})
		hub.Register(userID, sink)
		defer hub.Unregister(userID, sink)
		defer sink.Close()

		ctx := r.Context()

		// leitura: drenar TUDO que o cliente mandar, sem responder (CA-03 —
		// QUALQUER mensagem, de qualquer tamanho). SetReadLimit(-1) +
		// Reader/io.Copy descartam por streaming sem materializar o payload
		// (c.Read faria io.ReadAll e o limit default de 32KiB fecharia a
		// conexão com StatusMessageTooBig). O mesmo loop processa os pongs.
		c.SetReadLimit(-1)
		readDead := make(chan struct{})
		go func() {
			defer close(readDead)
			for {
				_, r, err := c.Reader(ctx)
				if err != nil {
					return
				}
				if _, err := io.Copy(io.Discard, r); err != nil {
					return
				}
			}
		}()

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-readDead: // cliente fechou ou a conexão morreu
				return
			case <-sink.dead: // hub fechou o sink (backpressure — D-05)
				return
			case msg := <-sink.ch:
				wctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
				err := c.Write(wctx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			case <-ticker.C:
				pctx, cancel := context.WithTimeout(ctx, pingTimeout)
				err := c.Ping(pctx)
				cancel()
				if err != nil { // sem pong no prazo → device morto (D-08)
					return
				}
			}
		}
	}
}

// wsConn é o recorte de *websocket.Conn que o adapter usa — interface pro
// unit com conn fake (realtime_sink_test.go).
type wsConn interface {
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
	Ping(ctx context.Context) error
	Close(code websocket.StatusCode, reason string) error
}

// realConn adapta *websocket.Conn pro wsConn. Close derruba o transporte NA
// HORA (CloseNow): quem chega aqui (backpressure, sem pong, teardown) tem
// peer não-cooperante por hipótese — o handshake gracioso da lib esperaria
// segundos segurando conexão morta no registro.
type realConn struct{ c *websocket.Conn }

func (r realConn) Write(ctx context.Context, typ websocket.MessageType, p []byte) error {
	return r.c.Write(ctx, typ, p)
}
func (r realConn) Ping(ctx context.Context) error           { return r.c.Ping(ctx) }
func (r realConn) Close(websocket.StatusCode, string) error { return r.c.CloseNow() }

// wsSink adapta a conexão pro contrato do hub (D-03/D-05): buffer de 8
// eventos, TrySend nunca bloqueia, buffer cheio FECHA a conexão — pular um
// evento deixaria o cliente com estado velho sem saber; fechado, ele
// reconecta e o re-sync do open (CA-14) restaura a verdade.
type wsSink struct {
	conn wsConn
	ch   chan []byte
	dead chan struct{}
	once sync.Once
}

func newWSSink(c wsConn) *wsSink {
	return &wsSink{conn: c, ch: make(chan []byte, 8), dead: make(chan struct{})}
}

func (s *wsSink) TrySend(msg []byte) bool {
	select {
	case s.ch <- msg:
		return true
	default:
		s.Close()
		return false
	}
}

// Close é idempotente: hub (backpressure) e handler (teardown) podem ambos
// fechar. Derrubar o transporte desbloqueia leitura e escrita pendentes.
func (s *wsSink) Close() {
	s.once.Do(func() {
		close(s.dead)
		_ = s.conn.Close(websocket.StatusPolicyViolation, "consumidor lento")
	})
}
