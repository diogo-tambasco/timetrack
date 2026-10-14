// notice_test.go — fase 16 T-07: Hub.BroadcastNotice (aditivo — CA-10/RNF-05).
// Entrega {"type":"notice","payload":...} só às conexões do user; não afeta o
// consumidor de timer. Reusa o fakeSink da fase 04 (hub_test.go).
package realtime_test

import (
	"encoding/json"
	"testing"

	"timetrack.live/backend/internal/realtime"
)

// CA-10 — BroadcastNotice entrega o envelope notice às conexões do dono, e SÓ
// a elas (não vaza pra outro user).
func TestBroadcastNoticeEntregaSoAoDono(t *testing.T) {
	hub := realtime.NewHub()
	a1, a2, b1 := &fakeSink{}, &fakeSink{}, &fakeSink{}
	hub.Register("user-a", a1)
	hub.Register("user-a", a2)
	hub.Register("user-b", b1)

	hub.BroadcastNotice("user-a", map[string]any{
		"kind":     "timer_auto_cut",
		"entry_id": "e-1",
	})

	if len(b1.msgs) != 0 {
		t.Errorf("o notice de A NÃO pode vazar pro user B: %q", b1.msgs)
	}
	if len(a1.msgs) != 1 || len(a2.msgs) != 1 {
		t.Fatalf("as conexões de A deveriam receber o notice (a1=%d a2=%d)", len(a1.msgs), len(a2.msgs))
	}

	// envelope = {"type":"notice","payload":{...}}
	var env struct {
		Type    string `json:"type"`
		Payload struct {
			Kind    string `json:"kind"`
			EntryID string `json:"entry_id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(a1.msgs[0], &env); err != nil {
		t.Fatalf("envelope fora do contrato: %s", a1.msgs[0])
	}
	if env.Type != "notice" {
		t.Errorf("type deveria ser \"notice\", veio %q", env.Type)
	}
	if env.Payload.Kind != "timer_auto_cut" || env.Payload.EntryID != "e-1" {
		t.Errorf("payload do aviso fora do esperado: %+v", env.Payload)
	}
}

// RNF-05 — o envelope do notice é DISTINTO do timer: type "notice" ≠ "timer".
// (Garante que o cliente que só trata "timer" vai ignorar este sem confundir.)
func TestNoticeNaoUsaTypeTimer(t *testing.T) {
	hub := realtime.NewHub()
	s := &fakeSink{}
	hub.Register("u", s)
	hub.BroadcastNotice("u", map[string]any{"kind": "timer_auto_cut"})

	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(s.msgs[0], &env); err != nil {
		t.Fatalf("envelope inválido: %s", s.msgs[0])
	}
	if env.Type == "timer" {
		t.Errorf("o notice NÃO pode usar o type \"timer\" (confundiria o consumidor de estado)")
	}
}
