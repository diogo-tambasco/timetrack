# plan.md — Fase 04 · Sync em tempo real

> Como implementar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7` (§7.3 = tempo
> real é núcleo) e TODOS os contratos da fase 03: `timer.Service` transacional, `TimerState`
> canônico do `GET /timer`, `timerStore.apply()` com anti-stale, re-sync em
> `visibilitychange`/focus/60s. A tabela `timer_states` segue **interna** — esta fase consome
> só o hook `OnChange` e o `State` (INDEX.md).

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **WS downstream-only.** O socket só ENTREGA estado; nenhuma mutação entra por ele (mensagens do cliente são lidas e descartadas — CA-03). Toda escrita continua nas rotas REST da 03, atrás da cadeia sessão → membership → role → advisory lock. | Superfície de ataque e de teste mínima: zero parsing de comando, zero validação nova, zero caminho de escrita duplicado. O WS vira um "GET /timer empurrado" — e o CA-02 garante que o cliente não distingue a origem. |
| D-02 | **Lib WS Go: `github.com/coder/websocket`** (sucessora mantida da `nhooyr.io/websocket`). | A stdlib não tem WebSocket. Alternativas: `gorilla/websocket` (maior, estilo callback, já foi arquivada) e `gobwas/ws` (low-level demais). A coder/websocket é mínima, context-based (combina com chi/http nativo) e roda em `httptest.Server` — necessário pro teste de integração. **Única dep Go nova da fase (RNF-01), sancionada aqui.** |
| D-03 | **Hub transport-agnostic em `internal/realtime`.** O Hub conhece só a interface `Sink` (`TrySend([]byte) bool` + `Close()`); a camada `api` adapta a conexão websocket real pra um Sink com buffer. | TDD.md §3 manda testar broadcast com **transporte fake** determinístico. O Hub fica 100% testável sem rede; o adapter websocket é fino e coberto pelo teste de integração httptest (T-04). |
| D-04 | **Extensão ADITIVA do hook:** `SetOnChange(func(userID string, st *timer.State, at time.Time))` — o `at` é o `now` **da mutação** (o mesmo que vira `server_time` no `Result`). | O CA-02 exige `server_time` = relógio da mutação: é ele que o `apply()` do front usa pra descartar stale e ordenar WS × REST. Carimbar no broadcast (tempo do hub) quebraria essa ordenação com clock fake nos testes e criaria corrida real (evento carimbado depois de um GET mais novo). Únicos consumidores hoje: o no-op e 1 teste da fase 03 → mudança de assinatura barata, **exceção registrada** ao "sem refatorar" do RNF-05 da 03 (extensão de assinatura, não de comportamento; a suíte da 03 muda só nesse teste). |
| D-05 | **Backpressure por conexão: buffer pequeno (8 eventos) e, cheio, a conexão é FECHADA** (não bloqueia, não descarta evento isolado). | CA-09: um cliente travado não pode atrasar o hub nem os outros devices. Fechar (em vez de pular eventos) é o único modo seguro num protocolo de estado: pular um evento deixaria o cliente com estado velho SEM saber; fechado, ele reconecta e o CA-14 (re-sync no open) restaura a verdade. |
| D-06 | **Envelope versionável** `{"type":"timer","payload":{state,server_time}}`; o payload é a serialização EXATA do `timer.Result` (sem `stopped_entry` — irrelevante pra quem só espelha estado). Tipos desconhecidos são ignorados pelo cliente (CA-18). | Decisão do Diogo limitou a v1 ao timer; o envelope deixa entries/settings entrarem em fase futura sem quebrar clientes no ar. |
| D-07 | **Cliente WS dentro do runtime existente:** `lib/realtime.ts` exporta `connectTimerSocket({makeUrl, onResult, ...})` puro (makeUrl chamado a cada tentativa, backoff com jitter injetável, parse, dispose — testável com WebSocket stubado); `attachTimerSync()` (timerStore.ts) o liga e desliga junto com o interval e os listeners. | Um único runtime do timer = um único teardown (CA-19). O `apply()` continua sendo o único ponto de entrada de estado (CA-11) — zero caminho novo de mutação de store. |
| D-08 | **Ping do servidor a cada 30s; sem pong em 2 ciclos → fecha.** Reconexão no cliente: backoff exponencial 1s→2s→4s→…→30s (teto) com jitter, re-agendado no `onclose`. | Libera conexões de devices mortos (iPad dormiu) sem esperar timeout de TCP; o teto + jitter evita tempestade de reconexão (CA-16). |

## 2. Modelo de dados / migrações

**Nenhuma migração.** Esta fase não toca o schema — estado de conexão vive em memória no
processo (1 instância única, decisão de deploy do baseline). Restart do server derruba os
sockets; os clientes reconectam (backoff) e re-sincronizam (CA-14) — sem estado perdido,
porque a verdade está no Postgres da fase 03.

## 3. Contratos

### 3.1 `GET /api/v1/ws` — upgrade autenticado (CA-05)

- Atrás do `RequireAuth` da fase 01, dentro do `/api/v1` (mesmo subrouter; nova option
  `api.WithRealtime(hub)` monta a rota).
- Sem sessão → `401 {"error":{"code":"unauthorized",...}}` **antes** do upgrade (o
  middleware já responde; o handler nem roda).
- Com sessão → upgrade WebSocket; a conexão fica registrada no hub sob o `user_id` do
  contexto até fechar.
- Origem: mesma origem do app (PWA servida pelo próprio backend em prod; em dev/preview o
  proxy do Vite encaminha). `OriginPatterns` restrito ao host da requisição.

### 3.2 Mensagens (servidor → cliente; CA-02/CA-03/CA-18)

```json
{ "type": "timer", "payload": { "state": { "...": "TimerState canônico" },
                                "server_time": "2026-06-11T14:00:00Z" } }
```
- `payload` = exatamente o corpo do `GET /api/v1/timer` (state nulo quando ocioso).
  *(Nos asserts byte a byte: comparar após `bytes.TrimSpace` nas duas pontas — o
  `respond.JSON` usa `Encoder.Encode`, que termina o corpo HTTP com `\n`; o payload
  extraído do envelope não o tem.)*
- O cliente trata `type !== "timer"` (e JSON inválido) como no-op (CA-18).
- Mensagens cliente→servidor: lidas e descartadas (CA-03); o servidor nunca responde a elas.

### 3.3 Hook estendido (RF-05/D-04)

```go
// fase 03 (atual):  SetOnChange(func(userID string, st *State))
// fase 04 (novo):   SetOnChange(func(userID string, st *State, at time.Time))
```
Chamado **após o commit** de toda mutação (já é assim na 03 — CA-07 só ganha teste), com o
`now` da própria mutação. `cmd/server` liga: `tsvc.SetOnChange(hub.BroadcastTimer)`.
São **5 call sites** em `timer/service.go` (292/336/414/466/519) — atenção ao 5º
(`resumeFromBreak`, pausa órfã, linha ~466): ele commita, emite `onChange(userID, nil)` E
retorna `409 no_active_break`. É o caso real do "evento ⟺ commit" do CA-07 — **não remover
o emit** "corrigindo" pelo 409; o comportamento certo é o do código.

## 4. Libs (leveza é requisito)

**Go — 1 nova (sancionada em D-02):** `github.com/coder/websocket`. Resto igual à 03.
**Front — nenhuma:** `WebSocket` nativo, `URL` derivada de `location` (`ws://`/`wss://` por
protocolo). Testes Vitest com classe `WebSocket` stubada (fake timers pro backoff) — zero
dependência nova, mesmo padrão do stub de fetch puro da 03.

## 5. Onde vive o quê

```
backend/internal/
  realtime/   hub.go (Hub, Sink, registro por usuário, broadcast, fecha lento)
              hub_test.go (transporte fake — CA-01/06/08/09/10)
  timer/      service.go (assinatura do hook +at — D-04; nada mais muda)
              service_test.go (closure do hook ganha o 3º arg — edição sancionada CA-04)
  api/        realtime_routes.go (option WithRealtime, handler do upgrade, adapter Sink
              com buffer 8 + ping 30s — intervalo/timeout INJETÁVEIS pros testes) ·
              auth_routes.go (a rota /ws pendura no subrouter /api/v1 existente — o
              mountAuth ganha o ramo, igual ao mountTimer da 03) ·
              timertest_test.go (harness ganha o hub no newTimerRouter) ·
              realtime_test.go (httptest + ws real — T-04)
backend/cmd/server/  wiring hub → SetOnChange
frontend/src/lib/
  realtime.ts     connectTimerSocket({url, onResult, ...}): backoff, parse, dispose
  timerStore.ts   attachTimerSync() ganha o socket (open → load(); message → apply();
                  teardown fecha e cancela reagendamento)
frontend/vite.config.ts  proxy '/api' vira objeto { target, ws: true } em server E preview
frontend/e2e/cronometro.spec.ts  ganha os testes de sync (2 contextos) — ver §6
```

## 6. Pontos de integração e armadilhas conhecidas

- **Vite dev/preview:** o shorthand `'/api': 'http://localhost:8080'` NÃO encaminha
  WebSocket — virar `{ target: 'http://localhost:8080', ws: true }` nos dois blocos
  (`server` e `preview`; o E2E roda no preview). **NÃO adicionar `changeOrigin: true`**
  (o shorthand atual o implica, mas no objeto novo ele deve ser OMITIDO): com changeOrigin
  o Host encaminhado vira `localhost:8080` enquanto o Origin do browser segue `:4173`/`:5173`
  — e o origin check default da coder/websocket (só autoriza Origin == host da requisição)
  reprova com 403 SÓ no dev/preview, com REST funcionando. Pesadelo de diagnosticar.
- **NÃO usar `Conn.CloseRead()`** no handler: é o idiom documentado da lib pra conexão
  write-only, mas ele FECHA a conexão com `StatusPolicyViolation` na primeira data message
  do cliente — viola o CA-03. Drenar com loop manual de `c.Read(ctx)` descartando o
  retorno (o mesmo loop processa os pongs do D-08).
- **Service Worker:** WebSocket não passa pelo SW — nenhuma mudança no workbox. O
  `offline.spec.ts` continua válido **porque esta fase não adiciona persistência nem cache**
  (spec §6 — proibição explícita).
- **E2E com timer global:** os testes de sync entram **no `cronometro.spec.ts` existente**
  (serial) — regra estabelecida no bloco 3 da fase 03: todo teste que muta o timer do
  usuário único do stub vive num único arquivo serial; um arquivo novo em worker paralelo
  brigaria pelo timer. O "2º device" é `browser.newContext()` + `loginViaStub` dentro do
  mesmo teste (mesmo usuário do googlestub = exatamente o cenário desejado).
- **Transições automáticas com 2 devices (CA-17):** nada de eleição de líder — todos os
  devices disparam (CA-33/34/34.1 da 03), o servidor serializa no advisory lock, o segundo
  recebe `409` e o `catch → load()` que JÁ existe no `checkAutoTransition` re-sincroniza. O
  broadcast do vencedor chega via `apply()` e rearma o `firedKey`. A fase 04 só **prova**
  esse comportamento (teste); não muda a lógica.
- **Suíte da fase 03:** intacta exceto o teste do hook (assinatura +`at` — D-04). O CA-04
  exige a suíte 03 verde sem nenhuma outra edição.
- **Fase 05 (Modo Foco):** consome o `timerStore` já sincronizado — zero trabalho extra lá.
- **Deploy (09):** 1 processo único — sem fan-out entre instâncias (fora de escopo até
  existir mais de uma instância, o que o MVP não prevê).
