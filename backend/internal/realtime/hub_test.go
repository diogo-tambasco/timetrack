package realtime_test

import (
	"bytes"
	"testing"

	"timetrack.live/backend/internal/realtime"
)

// fakeSink — transporte fake determinístico (TDD.md §3): entrega em slice,
// sem canal nem goroutine. reject simula o consumidor que não drena
// (buffer cheio no adapter real — D-05).
type fakeSink struct {
	msgs   [][]byte
	reject bool
	closed int
}

func (f *fakeSink) TrySend(m []byte) bool {
	if f.reject {
		return false
	}
	f.msgs = append(f.msgs, m)
	return true
}

func (f *fakeSink) Close() { f.closed++ }

// CA-01 — broadcast do usuário A NUNCA chega a Sink do usuário B
// (vazamento entre usuários = bug crítico).
func TestBroadcastNuncaVazaEntreUsuarios(t *testing.T) {
	hub := realtime.NewHub()
	a1, a2, b1 := &fakeSink{}, &fakeSink{}, &fakeSink{}
	hub.Register("user-a", a1)
	hub.Register("user-a", a2)
	hub.Register("user-b", b1)

	hub.Broadcast("user-a", []byte(`{"type":"timer"}`))

	if len(b1.msgs) != 0 {
		t.Errorf("socket do usuário B recebeu broadcast de A: %q", b1.msgs)
	}
	if len(a1.msgs) != 1 || len(a2.msgs) != 1 {
		t.Errorf("sockets de A deveriam receber o evento (a1=%d a2=%d)", len(a1.msgs), len(a2.msgs))
	}
}

// CA-06/CA-08 — TODAS as conexões do usuário (devices e abas, inclusive a
// que originou a mutação) recebem EXATAMENTE 1 evento por broadcast.
func TestTodasAsConexoesRecebemExatamenteUmEvento(t *testing.T) {
	hub := realtime.NewHub()
	a1, a2 := &fakeSink{}, &fakeSink{}
	hub.Register("user-a", a1)
	hub.Register("user-a", a2)

	msg := []byte(`{"type":"timer","payload":{"state":null}}`)
	hub.Broadcast("user-a", msg)

	for i, s := range []*fakeSink{a1, a2} {
		if len(s.msgs) != 1 {
			t.Fatalf("sink %d deveria receber exatamente 1 evento, recebeu %d", i+1, len(s.msgs))
		}
		if !bytes.Equal(s.msgs[0], msg) {
			t.Errorf("sink %d recebeu payload diferente do broadcast: %q", i+1, s.msgs[0])
		}
	}
}

// CA-09 — Sink que não drena (TrySend false) é FECHADO e removido sem
// bloquear o broadcast nem atrasar as demais conexões.
func TestConsumidorLentoEFechadoSemAtrasarOsDemais(t *testing.T) {
	hub := realtime.NewHub()
	ok1, lento, ok2 := &fakeSink{}, &fakeSink{reject: true}, &fakeSink{}
	hub.Register("user-a", ok1)
	hub.Register("user-a", lento)
	hub.Register("user-a", ok2)

	hub.Broadcast("user-a", []byte(`evt-1`))

	if lento.closed != 1 {
		t.Errorf("o sink lento deveria ser fechado (closed=%d)", lento.closed)
	}
	if len(ok1.msgs) != 1 || len(ok2.msgs) != 1 {
		t.Errorf("os sinks saudáveis deveriam receber normalmente (ok1=%d ok2=%d)",
			len(ok1.msgs), len(ok2.msgs))
	}
	if n := hub.Conns("user-a"); n != 2 {
		t.Errorf("o sink fechado deveria sair do registro (conns=%d, esperado 2)", n)
	}

	// o fechado não recebe mais nada nos broadcasts seguintes
	hub.Broadcast("user-a", []byte(`evt-2`))
	if lento.closed != 1 {
		t.Errorf("sink já removido não pode ser fechado de novo (closed=%d)", lento.closed)
	}
	if len(ok1.msgs) != 2 || len(ok2.msgs) != 2 {
		t.Errorf("broadcast seguinte deveria alcançar só os saudáveis (ok1=%d ok2=%d)",
			len(ok1.msgs), len(ok2.msgs))
	}
}

// CA-10 — após Unregister, broadcasts não alcançam o Sink e o registro não
// cresce (sem leak de conexões acumuladas).
func TestUnregisterRemoveDoRegistroSemLeak(t *testing.T) {
	hub := realtime.NewHub()
	a1, a2 := &fakeSink{}, &fakeSink{}
	hub.Register("user-a", a1)
	hub.Register("user-a", a2)
	if n := hub.Conns("user-a"); n != 2 {
		t.Fatalf("registro deveria ter 2 conexões, tem %d", n)
	}

	hub.Unregister("user-a", a1)

	hub.Broadcast("user-a", []byte(`evt`))
	if len(a1.msgs) != 0 {
		t.Errorf("sink desregistrado recebeu broadcast: %q", a1.msgs)
	}
	if len(a2.msgs) != 1 {
		t.Errorf("sink que ficou deveria receber o evento (recebeu %d)", len(a2.msgs))
	}
	if n := hub.Conns("user-a"); n != 1 {
		t.Errorf("registro deveria ter 1 conexão após o unregister, tem %d", n)
	}

	// ciclo completo register/unregister não acumula nada — nem a ENTRADA
	// do usuário no mapa (verificação adversarial do build, 11/06: Conns
	// devolve 0 tanto pra entrada removida quanto pra vazia que ficou)
	hub.Unregister("user-a", a2)
	if n := hub.Conns("user-a"); n != 0 {
		t.Errorf("registro deveria zerar, tem %d", n)
	}
	if n := hub.Users(); n != 0 {
		t.Errorf("o mapa de usuários deveria zerar após o ciclo (tem %d entradas) — leak de 1 entrada por usuário que já conectou", n)
	}
}
