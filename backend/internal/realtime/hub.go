// Package realtime: hub de broadcast por usuário da fase 04 (D-03).
// Transport-agnostic: o Hub conhece só a interface Sink; a camada api
// adapta a conexão websocket real. Testável com transporte fake (TDD.md §3).
package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"timetrack.live/backend/internal/timer"
)

// Sink é o destino de eventos de UMA conexão. TrySend NUNCA bloqueia:
// false = consumidor não drena (buffer cheio no adapter — D-05) e o hub
// FECHA a conexão (CA-09) — pular um evento deixaria o cliente com estado
// velho SEM saber; fechado, ele reconecta e o re-sync do open (CA-14)
// restaura a verdade.
type Sink interface {
	TrySend(msg []byte) bool
	Close()
}

// Hub registra as conexões abertas por usuário e entrega cada evento a
// TODAS as conexões do dono (CA-06/CA-08) — e SÓ a elas (CA-01).
type Hub struct {
	mu    sync.Mutex
	users map[string]map[Sink]struct{}
}

// NewHub cria o hub vazio. Estado de conexão vive só em memória (plan §2):
// restart derruba os sockets e os clientes reconectam + re-sincronizam.
func NewHub() *Hub {
	return &Hub{users: make(map[string]map[Sink]struct{})}
}

// Register adiciona a conexão ao registro do usuário.
func (h *Hub) Register(userID string, s Sink) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.users[userID]
	if set == nil {
		set = make(map[Sink]struct{})
		h.users[userID] = set
	}
	set[s] = struct{}{}
}

// Unregister remove a conexão: broadcasts seguintes não a alcançam e o
// registro não acumula conexões mortas (CA-10).
func (h *Hub) Unregister(userID string, s Sink) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.remove(userID, s)
}

// remove tira o Sink do registro (chamar com o mutex tomado).
func (h *Hub) remove(userID string, s Sink) {
	set := h.users[userID]
	delete(set, s)
	if len(set) == 0 {
		delete(h.users, userID)
	}
}

// Broadcast entrega msg a todas as conexões do usuário. Conexão que não
// drena é fechada e removida NA HORA (CA-09), sem bloquear nem atrasar as
// demais — TrySend é não-bloqueante por contrato.
func (h *Hub) Broadcast(userID string, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.users[userID] {
		if !s.TrySend(msg) {
			s.Close()
			h.remove(userID, s)
		}
	}
}

// BroadcastTimer adapta o hook OnChange do timer.Service (D-04) pro
// broadcast: o payload é a serialização EXATA do timer.Result — sem
// stopped_entry, irrelevante pra quem só espelha estado (D-06) — embrulhada
// no envelope versionável do RF-03. O server_time é o `at` da MUTAÇÃO
// (CA-02): é ele que o apply() do front usa pra ordenar WS × REST.
// Wiring: tsvc.SetOnChange(hub.BroadcastTimer) no boot (plan §3.3).
func (h *Hub) BroadcastTimer(userID string, st *timer.State, at time.Time) {
	evt, err := json.Marshal(struct {
		Type    string       `json:"type"`
		Payload timer.Result `json:"payload"`
	}{Type: "timer", Payload: timer.Result{State: st, ServerTime: at}})
	if err != nil {
		return // inalcançável: Result é sempre serializável
	}
	h.Broadcast(userID, evt)
}

// BroadcastNotice empurra um aviso (envelope type:"notice") às conexões /ws
// do dono — fase 16, CA-10/RNF-05. ADITIVO: reusa o mesmo Broadcast([]byte)
// por usuário, só com outro envelope type. O BroadcastTimer/Sink/Broadcast
// existentes NÃO mudam (CA-18). O payload identifica o aviso (ex.:
// {"kind":"timer_auto_cut","entry_id","stopped_at"}); o cliente que só trata
// type:"timer" ignora este envelope sem derrubar a conexão (CA-11/CA-18 da 04).
func (h *Hub) BroadcastNotice(userID string, payload any) {
	evt, err := json.Marshal(struct {
		Type    string `json:"type"`
		Payload any    `json:"payload"`
	}{Type: "notice", Payload: payload})
	if err != nil {
		return
	}
	h.Broadcast(userID, evt)
}

// BroadcastPresence empurra um envelope type:"presence" às conexões /ws do
// dono — fase 21, CA-19/RNF-02/04. ADITIVO: reusa o mesmo Broadcast([]byte) por
// usuário, só com outro envelope type (espelha o BroadcastNotice da fase 16). O
// fan-out POR WORKSPACE vive no presence.Service (resolução de membership +
// visibilidade), que chama este método uma vez por (membro, workspace) — o Hub
// NÃO indexa por workspace (Opção B do plan §1, RNF-02). O
// BroadcastTimer/BroadcastNotice/Broadcast/Sink existentes NÃO mudam (CA-19); o
// cliente que só trata type:"timer" ignora este envelope sem derrubar a conexão.
func (h *Hub) BroadcastPresence(userID string, payload any) {
	evt, err := json.Marshal(struct {
		Type    string `json:"type"`
		Payload any    `json:"payload"`
	}{Type: "presence", Payload: payload})
	if err != nil {
		return
	}
	h.Broadcast(userID, evt)
}

// Conns conta as conexões registradas do usuário (API de teste — CA-10).
func (h *Hub) Conns(userID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.users[userID])
}

// Users conta as entradas do mapa de usuários (API de teste — CA-10): pina
// a limpeza da entrada vazia no remove, que o Conns sozinho não enxerga
// (len de set removido e de set vazio são ambos 0).
func (h *Hub) Users() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.users)
}
