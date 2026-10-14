// realtime_sink_test.go — T-03/CA-09 (lado adapter): unit white-box do
// wsSink com conn FAKE. O lado Hub do CA-09 já foi provado no T-01; aqui é
// o adapter real — buffer 8 cheio → Close chamado, TrySend nunca bloqueia.
package api

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeWSConn implementa wsConn sem rede.
type fakeWSConn struct {
	closed atomic.Int32
}

func (f *fakeWSConn) Write(context.Context, websocket.MessageType, []byte) error { return nil }
func (f *fakeWSConn) Ping(context.Context) error                                 { return nil }
func (f *fakeWSConn) Close(websocket.StatusCode, string) error {
	f.closed.Add(1)
	return nil
}

func TestAdapterBufferCheioFechaSemBloquear(t *testing.T) {
	fc := &fakeWSConn{}
	s := newWSSink(fc)
	// sem o pump (run) rodando, nada drena o canal — os 8 do buffer cabem
	for i := range 8 {
		if !s.TrySend([]byte("evt")) {
			t.Fatalf("envio %d deveria caber no buffer de 8 (D-05)", i+1)
		}
	}

	// o 9º não cabe: TrySend devolve false NA HORA (nunca bloqueia) e o
	// adapter fecha a conexão (D-05: fechar, não pular evento)
	done := make(chan bool, 1)
	go func() { done <- s.TrySend([]byte("evt-9")) }()
	select {
	case ok := <-done:
		if ok {
			t.Error("9º envio com buffer cheio deveria devolver false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TrySend BLOQUEOU com o buffer cheio — viola o contrato do Sink")
	}
	if fc.closed.Load() == 0 {
		t.Error("buffer cheio deveria fechar a conexão (Close não foi chamado)")
	}

	// Close é idempotente (hub e adapter podem ambos fechar)
	s.Close()
	if got := fc.closed.Load(); got != 1 {
		t.Errorf("Close deveria ser idempotente (conn.Close chamado %d vezes)", got)
	}
}
