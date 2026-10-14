# tasks.md — Fase 04 · Sync em tempo real

> Tarefas atômicas, em ordem. **1 commit por tarefa** (teste + código, mensagem PT-BR). TDD
> red→green confirmado antes do green (`TDD.md §1`). WebSocket nos testes de unidade =
> transporte **fake** (TDD.md §3); o websocket real aparece nos testes de integração
> httptest (T-03/T-04) e no E2E (T-10).
> Blocos de sessão (regra do CLAUDE.md): **bloco 1 = T-01→T-05 (backend)** · **bloco 2 =
> T-06→T-11 (front + E2E + fechamento, gate `make check-full`)**.

---

## Backend

- **T-01 — Dep nova + `realtime.Hub` (transporte fake)**
  - **Objetivo:** `go get github.com/coder/websocket` (D-02) e o pacote `internal/realtime`:
    `Hub` com registro por usuário sobre a interface `Sink` (D-03), broadcast e remoção.
  - **Entradas:** plan §1 D-02/D-03/D-05.
  - **Entregável:** `realtime/hub.go` + `realtime/hub_test.go` (Sinks fake de canal).
  - **Acceptance check:** 4 testes red→green — **(CA-01)** broadcast do usuário A nunca
    chega a Sink do usuário B; **(CA-06/08)** 2 Sinks do mesmo usuário recebem exatamente 1
    evento cada por broadcast; **(CA-09)** Sink que recusa entrega (buffer cheio simulado no
    fake) é FECHADO e os demais recebem normalmente, sem o broadcast bloquear; **(CA-10)**
    após `Unregister` (ou fechamento pelo CA-09), novos broadcasts não alcançam o Sink e o
    registro não cresce (sem leak — contagem de Sinks assertada).

- **T-02 — Hook `OnChange` com o instante da mutação**
  - **Objetivo:** estender a assinatura pra `func(userID string, st *State, at time.Time)`
    (D-04 — extensão aditiva sancionada; `at` = `now` da mutação, o mesmo do `server_time`).
  - **Entradas:** `timer/service.go` (5 call sites pós-commit); o teste existente do hook.
  - **Entregável:** assinatura nova + chamadas passando o `now`; teste da 03 ajustado (edição
    mecânica sancionada — RF-05/D-04; o CA-04 registra a exceção).
  - **Acceptance check:** 3 testes red→green — **(CA-02 parcial)** o hook recebe `at` igual
    ao `server_time` do `Result` retornado (clock fake); **(CA-07 lado "sem commit")**
    mutação que falha (pause sem timer; start countdown sem alvo) NÃO chama o hook — red
    provado removendo a condição (mutação manual) antes do green; **(CA-07 lado "commit com
    409" — a exceção real)** pausa órfã (anular `resume_entry_id` via SQL direto, caminho de
    defesa da 03) → `Resume` responde conflito E o hook dispara exatamente 1x com `state`
    nil e o `at` da mutação — pina o emit da linha ~466 contra "correções" futuras.

- **T-03 — Endpoint `GET /api/v1/ws` (option `WithRealtime`)**
  - **Objetivo:** upgrade autenticado atrás do `RequireAuth`, registro/desregistro no hub,
    adapter `Sink` com buffer 8 (D-05) e ping 30s (D-08). Mensagens do cliente: drenadas e
    descartadas (CA-03).
  - **Entradas:** T-01; `api.WithAuth` (fase 01); harness `timertest_test.go`.
  - **Entregável:** `api/realtime_routes.go` + testes httptest (ping/buffer injetáveis).
  - **Acceptance check:** 5 testes red→green — **(CA-05)** sem cookie → `401` com envelope
    canônico, sem upgrade; **(CA-03)** mensagem enviada pelo cliente não gera resposta, não
    muta nada e a conexão segue viva (broadcast seguinte ainda chega — atenção plan §6: SEM
    `CloseRead`); **(CA-10 HTTP)** fechar a conexão remove o registro do hub (contagem via
    API de teste do hub); **(CA-10 braço ping)** com intervalo de ping encurtado via
    injeção, conexão que não responde pong é fechada E removida do hub; **(CA-09 adapter)**
    unit do adapter com conn fake: buffer 8 cheio → `Close` chamado, `TrySend` nunca
    bloqueia (o lado Hub já foi provado no T-01; aqui é o adapter real).

- **T-04 — Integração: mutação REST → evento nos sockets (httptest + ws real)**
  - **Objetivo:** provar a ponta a ponta backend: `httptest.Server` com o router REAL,
    2 conexões websocket do usuário + 1 de outro usuário; mutações REST disparam os eventos.
  - **Entradas:** T-01–T-03; `coder/websocket` como cliente de teste; clock fake.
  - **Entregável:** `api/realtime_test.go`.
  - **Acceptance check:** 2 testes red→green — **(CA-02/06/08)** `POST /timer/start` → os 2
    sockets do usuário recebem exatamente 1 evento `{"type":"timer",...}` cujo `payload` é
    byte-idêntico ao corpo do `GET /timer` na sequência **após `bytes.TrimSpace` nas duas
    pontas** (o `Encoder.Encode` do respond.JSON termina o corpo HTTP com `\n` — plan §3.2;
    clock congelado entre mutação e GET), e o socket do outro usuário recebe NADA — CA-01
    na camada HTTP; **(CA-06 pause)** `POST /timer/pause` → evento com `state.mode='break'`
    e `remaining_seconds` da config (o dado que destrava o CA-12 no front).

- **T-05 — Broadcast em TODAS as mutações + corrida do ciclo + wiring no `cmd/server`**
  - **Objetivo:** tabela completa de mutações emitindo evento (start, stop, pause, resume,
    skip, continue — troca atômica emite só o estado FINAL), a corrida determinística do
    CA-17 no braço do ciclo, e o `SetOnChange(hub.BroadcastTimer)` ligado no boot.
  - **Entradas:** T-02, T-04; `entries` continue (fase 03 T-13).
  - **Entregável:** teste table-driven + teste de corrida + wiring.
  - **Acceptance check:** **exceção documentada ao red estrutural** (a 03 já chama
    `onChange` em toda mutação — com o hub plugado no T-04 a tabela nasce verde por
    construção; validade provada por MUTAÇÃO: remover o `onChange` de 1 call site → a
    tabela falha, confirmar antes do commit): **(CA-06)** `TestTodaMutacaoEmiteUmEvento` —
    cada operação da tabela entrega exatamente 1 evento por socket com o estado esperado
    (stop → `state:null`; continue → rodando com os campos clonados; troca via start com
    timer ativo → 1 evento só, do estado final); **(CA-17 determinístico — braço do ciclo)**
    pausa pomodoro EXPIRADA (clock fake além do alvo) + 2 `POST /timer/resume` concorrentes
    → exatamente 1 entry nova criada, 1 resposta `409 no_active_break`, e exatamente 1
    evento por socket (é o braço com risco de entry duplicada — área do bug HIGH do bloco 2
    da 03; inviável em E2E pelos mínimos 15/1min).

## Frontend

- **T-06 — `lib/realtime.ts` (cliente WS puro)**
  - **Objetivo:** `connectTimerSocket({makeUrl, onResult})`: abre o socket, parseia o
    envelope, backoff exponencial 1s→30s com jitter no `onclose`, `dispose()` total.
    WebSocket **stubado** nos testes (classe fake; zero dep — RNF-01).
  - **Entradas:** plan §3.2, D-07/D-08.
  - **Entregável:** `frontend/src/lib/realtime.ts` + `realtime.spec.ts` (fake timers).
  - **Acceptance check:** 4 testes red→green — **(CA-11 parcial)** mensagem `type:"timer"`
    chama `onResult` com o `payload`; **(CA-18)** JSON inválido e `type` desconhecido são
    ignorados sem fechar o socket nem chamar `onResult`; **(CA-16)** após `close`, reconecta
    com atrasos crescentes até o teto (fake timers; o jitter é **injetável** — nos testes
    vira determinístico pra sequência ser assertável) e NENHUM `fetch` periódico é criado;
    **(CA-19)** `dispose()` fecha o socket e cancela o reagendamento
    (avançar os timers não cria conexão nova).

- **T-07 — Integração no `attachTimerSync` (runtime único)**
  - **Objetivo:** o `attachTimerSync()` passa a abrir o socket: `open → load()` (re-sync),
    `onResult → apply()`, teardown devolvido fecha TUDO (interval + listeners + socket).
  - **Entradas:** T-06; `timerStore.ts` da 03.
  - **Entregável:** extensão do `timerStore.ts` + testes no `timerStore.spec.ts`.
  - **Acceptance check:** 4 testes red→green — **(CA-14)** open (e RE-open após queda)
    dispara `GET /timer`; **(CA-11)** evento aplicado via `apply()` COM anti-stale: evento
    com `server_time` antigo não regride o estado (reuso do teste-padrão da 03 com fonte
    WS); **(CA-15)** com o socket CAÍDO (connect falha, backoff agendado), mutações do
    store e re-sync via REST seguem funcionando normalmente (stub de fetch — WS fora do ar
    ≠ app degradado); **(CA-19)** o teardown do `attachTimerSync` desliga o socket junto.
    Se os specs da 03 que chamam `attachTimerSync` precisarem de stub de `WebSocket`, a
    mudança é SÓ no setup (exceção do CA-04 — nenhuma asserção da 03 muda).

- **T-08 — Proxy WS no Vite (dev + preview)**
  - **Objetivo:** `'/api'` vira `{ target: 'http://localhost:8080', ws: true }` em `server`
    E `preview` (o E2E roda no preview; sem isso o socket morre no proxy).
  - **Entradas:** plan §6.
  - **Entregável:** `vite.config.ts` atualizado.
  - **Acceptance check:** acceptance é o E2E do T-10 conectando de verdade pelo preview
    (sem teste próprio — config); `npm run build` segue verde.

- **T-09 — REMOVIDA (verificação adversarial das specs, 11/06)**
  - O unit "apply de break → BreakOverlay renderiza" não tem red possível: esse caminho de
    render já é comportamento pinado pela suíte da 03 (a UI deriva 100% do store). O CA-12
    é provado no T-10/CA-20 (E2E real), que ABSORVEU a asserção única desta tarefa: o
    device passivo não emite nenhuma mutação própria. Numeração preservada de propósito
    (blocos e referências não mudam).

## E2E + fechamento

- **T-10 — E2E de sync: 2 contextos no `cronometro.spec.ts`**
  - **Objetivo:** os fluxos centrais com 2 "devices" (2 `browser.newContext()`, mesmo
    usuário do stub — cenário desejado), DENTRO do arquivo serial existente (plan §6: timer
    global = 1 arquivo serial).
  - **Entradas:** T-05, T-07, T-08; `ensureIdle`/`loginViaStub` existentes.
  - **Entregável:** testes novos em `frontend/e2e/cronometro.spec.ts`.
  - **Acceptance check:** 3 testes E2E — **exceção ao red estrutural documentada** (mesmo
    precedente do T-19 da 03: as entradas T-05–T-08 são pré-requisito, então o app já
    estará pronto; red honesto = rodar o spec ANTES de qualquer ajuste que ele exija) —
    **(CA-21 + CA-13)** start no A → barra do B rodando com descrição+chip em ≤3s sem
    reload; stop no A → barra do B volta a ociosa (continue herda o caminho do start —
    nota na matriz); **(CA-20 + CA-12 + CA-13 resume)** pause no A → B exibe BreakOverlay
    com countdown E não emite nenhuma mutação própria (monitorar `page.on('request')` do
    B: zero POST /timer/* — asserção absorvida da ex-T-09); retomar no A → B volta a
    rodando; **(CA-17 resultado fim a fim)** countdown de 2s iniciado **via API**
    (`page.request.post /timer/start` com `mode:'countdown'` — a fase 03 não tem UI de
    start de countdown; a barra só inicia countup) com A e B abertos → ao zerar, exatamente
    1 entry fechada no banco (asserção via listagem), nenhum dialog/erro visível nos 2
    devices, ambos ociosos. (O braço do ciclo do CA-17 é determinístico no T-05.)
  - **Aviso de robustez:** asserções do B sempre com `expect.poll`/auto-retry ≤3s — o sync é
    assíncrono; nada de `waitForTimeout` fixo como única espera.

- **T-11 — Fechamento da fase**
  - **Objetivo:** suíte completa verde numa execução única, `make check-full` GATE,
    gofmt/go vet limpos, INDEX marcando a fase 04 construída.
  - **Entradas:** T-01–T-10.
  - **Entregável:** commit de fechamento + INDEX.
  - **Acceptance check:** **(CA-04)** suíte da fase 03 verde SEM nenhuma edição além da
    sancionada no T-02 (diff da pasta de testes da 03 = só o hook); **(CA-15)** confirmado
    pelo `offline.spec.ts` intacto e verde (app funcional sem WS); matriz abaixo sem CA
    órfão; `check-full` GATE VERDE.

---

## Matriz CA → tarefa (todo critério coberto por ≥1 tarefa)

| CA | Tarefa | | CA | Tarefa |
|---|---|---|---|---|
| CA-01 | T-01 + T-04 | | CA-11 | T-06 + T-07 |
| CA-02 | T-02 + T-04 | | CA-12 | T-10/CA-20 (E2E; render já pinado pela suíte da 03) |
| CA-03 | T-03 | | CA-13 | T-10 (start/stop no CA-21; resume no CA-20; continue herda o caminho do start) |
| CA-04 | T-02 + T-11 | | CA-14 | T-07 |
| CA-05 | T-03 | | CA-15 | T-07 (WS caído, REST vivo) + T-11 (offline.spec intacto — condição MAIS forte: tudo caído) |
| CA-06 | T-01 + T-04 + T-05 | | CA-16 | T-06 |
| CA-07 | T-02 (ambos os lados: sem-commit e commit-com-409) | | CA-17 | T-05 (corrida determinística do ciclo) + T-10 (resultado E2E, braço countdown) |
| CA-08 | T-01 + T-04 | | CA-18 | T-06 |
| CA-09 | T-01 (hub) + T-03 (adapter) | | CA-19 | T-06 + T-07 |
| CA-10 | T-01 + T-03 (incl. braço ping) | | CA-20 | T-10 |
| | | | CA-21 | T-10 |
