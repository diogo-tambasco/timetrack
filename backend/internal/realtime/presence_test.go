// presence_test.go — fase 21 T-02: Hub.BroadcastPresence (aditivo — CA-19,
// RNF-02/04). Entrega {"type":"presence","payload":...} só às conexões do user;
// não afeta o consumidor de timer/notice. Reusa o fakeSink da fase 04
// (hub_test.go). hub_test.go/notice_test.go da 04/16 seguem verdes sem editar.
package realtime_test

import (
	"encoding/json"
	"testing"

	"timetrack.live/backend/internal/realtime"
)

// CA-19 — BroadcastPresence entrega o envelope presence às conexões do dono, e
// SÓ a elas (não vaza pra outro user). Espelha o BroadcastNotice (fase 16).
func TestBroadcastPresenceEntregaSoAoDono(t *testing.T) {
	hub := realtime.NewHub()
	a1, a2, b1 := &fakeSink{}, &fakeSink{}, &fakeSink{}
	hub.Register("user-a", a1)
	hub.Register("user-a", a2)
	hub.Register("user-b", b1)

	hub.BroadcastPresence("user-a", map[string]any{
		"user_id":      "user-x",
		"workspace_id": "ws-1",
		"running":      true,
		"project":      "Site",
	})

	if len(b1.msgs) != 0 {
		t.Errorf("o presence de A NÃO pode vazar pro user B: %q", b1.msgs)
	}
	if len(a1.msgs) != 1 || len(a2.msgs) != 1 {
		t.Fatalf("as conexões de A deveriam receber o presence (a1=%d a2=%d)", len(a1.msgs), len(a2.msgs))
	}

	// envelope = {"type":"presence","payload":{...}}
	var env struct {
		Type    string `json:"type"`
		Payload struct {
			UserID      string `json:"user_id"`
			WorkspaceID string `json:"workspace_id"`
			Running     bool   `json:"running"`
			Project     string `json:"project"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(a1.msgs[0], &env); err != nil {
		t.Fatalf("envelope fora do contrato: %s", a1.msgs[0])
	}
	if env.Type != "presence" {
		t.Errorf("type deveria ser \"presence\", veio %q", env.Type)
	}
	if env.Payload.UserID != "user-x" || env.Payload.WorkspaceID != "ws-1" ||
		!env.Payload.Running || env.Payload.Project != "Site" {
		t.Errorf("payload do presence fora do esperado: %+v", env.Payload)
	}
}

// RNF-04 — o envelope do presence é DISTINTO do timer e do notice: type
// "presence" ≠ "timer" ≠ "notice" (o cliente que só trata "timer" ignora este
// envelope sem derrubar a conexão).
func TestPresenceNaoUsaTypeTimerNemNotice(t *testing.T) {
	hub := realtime.NewHub()
	s := &fakeSink{}
	hub.Register("u", s)
	hub.BroadcastPresence("u", map[string]any{"running": false})

	if len(s.msgs) != 1 {
		t.Fatalf("deveria ter chegado 1 envelope, veio %d", len(s.msgs))
	}
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(s.msgs[0], &env); err != nil {
		t.Fatalf("envelope inválido: %s", s.msgs[0])
	}
	if env.Type == "timer" || env.Type == "notice" {
		t.Errorf("presence não pode usar o type %q", env.Type)
	}
	if env.Type != "presence" {
		t.Errorf("type deveria ser \"presence\", veio %q", env.Type)
	}
}
