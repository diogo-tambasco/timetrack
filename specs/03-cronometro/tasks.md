# tasks.md — Fase 03 · Cronômetro

> Tarefas atômicas, em ordem de execução. **1 commit por tarefa** (teste + código, mensagem
> PT-BR). TDD obrigatório: confirmar o **red** antes do green (`TDD.md §1`). Serviços externos
> sempre fake/stub; Postgres de teste efêmero, nunca o de prod.
> Pré-requisito: fases 00–02 verdes (server boot, sessão, projects/tasks).

---

## Backend — migrações

- **T-01 — Migração `time_entries`**
  - **Objetivo:** criar enum `entry_source` + tabela `time_entries` com check de horários,
    índices e o **unique parcial de 1 entry rodando por usuário**.
  - **Entradas:** DDL do `plan.md §2`; migrate-no-boot da fase 00.
  - **Entregável:** `backend/internal/db/migrations/NNNN_time_entries.sql`.
  - **Acceptance check:** teste `TestMigracaoTimeEntries` red→green — migra num Postgres de
    teste e **(CA-03)** a 2ª inserção com `stopped_at IS NULL` pro mesmo usuário falha por
    violação do índice único (inclusive em 2 transações concorrentes); hard delete de uma
    task referenciada zera `task_id` da entry (`ON DELETE SET NULL` — contrato com o
    hard delete de tasks da fase 02, D-03/CA-21 de lá).

- **T-02 — Migração `pomodoro_settings`**
  - **Objetivo:** tabela por usuário com defaults 25/5 e checks de faixa (15–90 / 1–60).
  - **Entradas:** DDL do `plan.md §2`.
  - **Entregável:** `NNNN_pomodoro_settings.sql`.
  - **Acceptance check:** teste `TestMigracaoPomodoroSettings` red→green — insert sem valores
    aplica defaults 25/5; insert com `break_minutes=0` falha no check.

- **T-03 — Migração `timer_states`**
  - **Objetivo:** enum `timer_mode` + tabela de estado de runtime (1 linha por usuário, campos
    `resume_*` pra retomar após pausa).
  - **Entradas:** DDL do `plan.md §2`.
  - **Entregável:** `NNNN_timer_states.sql`.
  - **Acceptance check:** teste `TestMigracaoTimerStates` red→green — 2ª linha pro mesmo
    `user_id` falha (PK); linha `mode='break'` aceita `entry_id` nulo; DELETE direto no banco
    da entry referenciada zera `resume_entry_id` (`ON DELETE SET NULL`, defesa — a API bloqueia
    antes com 409, ver T-12).

## Backend — domínio

- **T-04 — Repositório de entries + isolamento por workspace**
  - **Objetivo:** `internal/entries`: insert/list por dia/update/delete com filtro obrigatório
    de `workspace_id`.
  - **Entradas:** T-01; pool pgx da fase 00; fixtures de 2 workspaces (fase 01/02).
  - **Entregável:** `entries/repo.go` + testes.
  - **Acceptance check:** 2 testes red→green — **(CA-01)** `TestIsolamentoEntriesPorWorkspace`:
    entries criadas no workspace B nunca aparecem na listagem do workspace A, mesmo sendo do
    mesmo usuário; **(CA-01.1)** `TestListagemSoEntriesProprias`: com 2 usuários no MESMO
    workspace, a listagem de um nunca retorna entries do outro.

- **T-05 — `timer.Service`: start (count up) e stop**
  - **Objetivo:** iniciar count up criando entry `source='timer'` + linha em `timer_states`;
    stop fecha a entry com duração do relógio do servidor e remove o estado. Campos
    projeto/task/tags opcionais. Hook `OnChange` no-op (pronto pra fase 04).
  - **Entradas:** T-01, T-03, T-04; clock injetável (`func() time.Time`) pros testes.
  - **Entregável:** `timer/service.go` (`Start`, `Stop`, `State`) + testes.
  - **Acceptance check:** 5 testes red→green — **(CA-04)** start cria entry rodando com
    `started_at` do servidor e retorna estado; **(CA-06)** stop grava `stopped_at` e
    `duration_seconds` e estado vira nulo; **(CA-02)** `duration_seconds` enviado pelo cliente
    é ignorado (valor final = `stopped_at − started_at` do clock injetado); **(CA-03.1)**
    `user_id` no payload é ignorado — a entry nasce com o usuário da sessão; **(CA-20)** start
    sem `project_id`/`task_id` inicia normalmente.

- **T-06 — `timer.Service`: countdown e modo pomodoro + validações de escrita**
  - **Objetivo:** modos `countdown` (alvo do payload) e `pomodoro` (alvo = `focus_minutes`
    persistido); validação de `project_id`/`task_id` pertencerem ao workspace (compartilhada
    com a criação manual).
  - **Entradas:** T-05; leitura de `pomodoro_settings` (T-02); repositórios da fase 02.
  - **Entregável:** extensão de `timer/service.go` + testes.
  - **Acceptance check:** 5 testes red→green — **(CA-12)** countdown de 1200s consultado 300s
    depois (clock injetado) retorna `remaining_seconds=900`; **(CA-13)** start `pomodoro` com
    `focus_minutes=50` salvo gera `target_seconds=3000`; **(CA-13.1)** stop de um countdown de
    1200s com clock injetado 3600s depois grava `duration_seconds=3600` (real, sem clamp ao
    alvo); **(CA-27)** start `countdown` sem `target_seconds` (ou ≤0) → `400`, nenhuma entry
    criada; **(CA-29)** `project_id` de outro workspace → erro `not_found`, nenhuma entry
    criada (cobre start **e** entrada manual, table-driven).

- **T-07 — `timer.Service`: trocar de timer com um rodando**
  - **Objetivo:** start com timer ativo para o atual (duração calculada) e inicia o novo na
    mesma transação.
  - **Entradas:** T-05.
  - **Entregável:** lógica de troca + teste.
  - **Acceptance check:** teste `TestStartComTimerRodandoTrocaAtomico` red→green — **(CA-05)**
    após o 2º start existe exatamente 1 entry rodando; a 1ª está fechada com duração certa.

- **T-08 — Configuração do Pomodoro (service + handlers REST)**
  - **Objetivo:** `GET|PUT /api/v1/settings/pomodoro` com upsert por usuário, defaults na
    leitura e validação de faixa.
  - **Entradas:** T-02; middleware de sessão (fase 01); router chi (fase 00).
  - **Entregável:** `api/settings_pomodoro.go` + testes httptest.
  - **Acceptance check:** 3 testes red→green — **(CA-18)** PUT válido persiste e o GET
    seguinte retorna os mesmos valores; **(CA-19)** GET sem linha salva retorna 25/5;
    **(CA-28)** PUT com `focus_minutes=10` → `400` e GET continua retornando o valor anterior.

- **T-09 — Pause → pausa Pomodoro automática (comportamento central)**
  - **Objetivo:** `Pause` fecha a entry de trabalho e cria estado `mode='break'` com
    `target_seconds` da config (ou override do payload) e `resume_*` preenchidos (entry pausada,
    modo anterior, restante do alvo).
  - **Entradas:** T-05, T-06, T-08.
  - **Entregável:** `timer/service.go::Pause` + handler `POST /api/v1/timer/pause` + testes.
  - **Acceptance check:** 4 testes red→green — **(CA-07)** pause com `break_minutes=15` salvo
    fecha a entry e retorna `mode='break'`, `remaining_seconds=900`; **(CA-08)** pause com
    `{"break_minutes":5}` no body usa 300s nesta pausa e a config persistida segue 15;
    **(CA-25)** pause sem timer rodando → `409 no_active_timer`; **(CA-25.1)** pause com
    `{"break_minutes":0}` (e `61`, table-driven) → `400` e a entry de trabalho segue rodando
    intacta.

- **T-10 — Resume e skip da pausa**
  - **Objetivo:** `Resume`/`SkipBreak` encerram a pausa e iniciam **nova** entry clonada
    (descrição/projeto/task/tags), restaurando modo e restante do alvo.
  - **Entradas:** T-09.
  - **Entregável:** `timer/service.go::Resume,SkipBreak` + handlers `POST /api/v1/timer/resume`
    e `POST /api/v1/timer/break/skip` + testes.
  - **Acceptance check:** 5 testes red→green — **(CA-09)** resume cria entry nova com os 4
    campos clonados e `started_at` novo; **(CA-10)** countdown pausado aos 300/1200s retoma com
    `target_seconds=900`; **(CA-11)** skip com 200s de pausa restantes encerra na hora e deixa
    timer de trabalho rodando; **(CA-26)** resume sem pausa ativa → `409 no_active_break`;
    **(CA-06.1)** stop durante a pausa encerra a pausa e deixa ocioso (`state: null`), sem
    retomar nem criar entry.

- **T-11 — `GET /api/v1/timer` + auth nas rotas de timer**
  - **Objetivo:** endpoint de estado canônico (rodando / pausa / ocioso, com `server_time` e
    segundos calculados no servidor) e exigência de sessão em todas as rotas da fase.
  - **Entradas:** T-05–T-10; middleware de sessão.
  - **Entregável:** `api/timer.go` (GET + wiring de start/stop/pause/resume/skip) + testes httptest.
  - **Acceptance check:** 5 testes red→green — **(CA-21)** rodando: retorna modo, entry,
    `elapsed_seconds` do clock injetado e `server_time`; **(CA-22)** em pausa: `mode='break'`,
    `remaining_seconds` e bloco `resume` com a entry pausada; **(CA-22.1)** pausa expirada
    (clock injetado além do alvo) sem ação: segue `mode='break'` com `remaining_seconds=0`,
    nunca negativo; **(CA-23)** ocioso: `200 {"state":null}`; **(CA-30)** sem cookie de
    sessão: `401 unauthorized` (table-driven em todas as rotas da fase).

- **T-11.1 — Contrato HTTP de start/stop + membership/role no timer**
  - **Objetivo:** cobrir na camada REST (httptest, não só no service) os contratos de
    `POST /timer/start` e `POST /timer/stop`: status codes, mapeamento de erros do service e
    corpo idêntico ao do `GET /timer`; validar membership do `workspace_id` do payload e role
    gate (reuso da fase 02).
  - **Entradas:** T-11; fixtures de membership/roles (fase 01/02).
  - **Entregável:** `api/timer_test.go` (table-driven) — sem código novo de produção além do
    wiring de membership/role no handler de start.
  - **Acceptance check:** suíte table-driven red→green — **(CA-04/06 na camada HTTP)** start
    válido → `201` com corpo igual ao do `GET /timer`; stop → `200` com `stopped_entry` e
    `state:null`; **(CA-27 HTTP)** countdown sem `target_seconds` → `400`; **(CA-29 HTTP)**
    `project_id` de outro workspace → `404`; **(CA-29.1)** start com `workspace_id` de
    workspace que o usuário não integra → `404` sem expor dados; **(CA-29.2)** start como
    `viewer` do workspace → `403`; **(CA-25 HTTP)** stop ocioso → `409 no_active_timer`.

- **T-12 — CRUD de entradas manuais (REST, rotas aninhadas)**
  - **Objetivo:** `GET/POST /api/v1/workspaces/{workspaceId}/entries`,
    `PATCH/DELETE .../entries/{id}` com `source='manual'`, duração sempre recalculada no
    servidor, recorte "hoje" por timezone (`tz`) e validações — atrás da cadeia
    sessão → membership → role gate da fase 02.
  - **Entradas:** T-04, T-06 (validação compartilhada); middlewares de sessão (fase 01) e
    membership/role (fase 02).
  - **Entregável:** `api/entries.go` + testes httptest.
  - **Acceptance check:** 10 testes red→green — **(CA-14)** POST manual válido → `201`,
    `source='manual'`, `duration_seconds` calculado; **(CA-14.1)** entry iniciada às 22:00 BRT
    (01:00 UTC do dia seguinte): com `tz=America/Sao_Paulo` ela aparece no "Hoje" default
    desse dia BRT; sem `tz` (recorte UTC) ela NÃO aparece — provando que o recorte segue o
    fuso; **(CA-15)** PATCH mudando `stopped_at` recalcula a duração; **(CA-16)** DELETE →
    `204` e some da listagem; **(CA-24)** POST com `stopped_at ≤ started_at` → `400` e nada
    persistido; **(CA-24.1)** PATCH e DELETE na entry rodando → `409 entry_running`, nada muda;
    **(CA-24.2)** DELETE na entry que é `resume_entry_id` de pausa ativa → `409 entry_in_use`;
    **(CA-29.1)** rota com `{workspaceId}` de workspace que o usuário não integra → `404` sem
    expor dados; **(CA-29.2)** escrita como `viewer` → `403`; **(CA-29.3)** PATCH/DELETE em
    entry de outro usuário do mesmo workspace → `404`.

- **T-13 — Continuar entrada anterior**
  - **Objetivo:** `POST /api/v1/workspaces/{workspaceId}/entries/{id}/continue` clona
    descrição/projeto/task/tags e inicia count up (`source='timer'`), reaproveitando a troca
    do T-07 (só entries próprias — CA-29.3, coberto no T-12).
  - **Entradas:** T-07, T-12.
  - **Entregável:** handler + teste.
  - **Acceptance check:** teste `TestContinuarEntradaClonaEInicia` red→green — **(CA-17)**
    continue de uma entry fechada cria entry rodando com os 4 campos iguais e horários novos.

## Frontend

- **T-14 — API client + `useTimerStore` (zustand)**
  - **Objetivo:** **estender** o `lib/api.ts` (criado na fase 01, estendido na 02) com as
    chamadas de timer/entries/settings e criar o store do timer: estado = resposta
    do `GET /timer` + `clockOffset` (skew via `server_time`); tick local de 1s; auto-transições
    no zero (countdown → stop; foco pomodoro → pause; pausa de ciclo pomodoro → resume);
    re-sync + transição pendente no `visibilitychange`. Sem polling por segundo (RNF-02/03).
  - **Entradas:** contratos do `plan.md §3`; API mockada com **stub de `fetch` puro** (zero
    dependência nova — RNF-01 veta MSW, inclusive como dev-dependency).
  - **Entregável:** extensão de `frontend/src/lib/api.ts` + `frontend/src/lib/timerStore.ts`
    (novo) + testes Vitest.
  - **Acceptance check:** 4 testes Vitest (fake timers) red→green — **(CA-33)** countdown
    chegando a zero dispara exatamente 1 `POST /timer/stop`; **(CA-34)** foco pomodoro chegando
    a zero dispara exatamente 1 `POST /timer/pause`; **(CA-34.1)** pausa chegando a zero com
    `resume.mode='pomodoro_focus'` dispara exatamente 1 `POST /timer/resume` (e NÃO dispara
    quando o contexto é countup/countdown); **(CA-37.1)** `visibilitychange` pra visível com
    alvo já vencido re-sincroniza via `GET /timer` e dispara a transição pendente exatamente
    1 vez.

- **T-15 — `TimerBar` (barra global do cockpit)**
  - **Objetivo:** barra sempre visível no topo: input de descrição, seletor projeto/task,
    dígitos mono `h:mm:ss`, play/stop (`.tt-iconbtn--go`/`--stop`) e `.tt-pulse` quando rodando.
    Referência: `TimerBar` de `cockpit.jsx`/`parts.jsx`, tokens de `tokens.css`.
  - **Entradas:** T-14; lista de projetos (fase 02).
  - **Entregável:** `components/timer/TimerBar.tsx` + teste Vitest.
  - **Acceptance check:** teste `TimerBar.rodando` red→green — **(CA-31)** com estado rodando
    mockado, renderiza descrição, chip do projeto (`.tt-chip`/`.tt-dot` na cor), tempo decorrido
    em mono e botão stop com `.tt-pulse`.

- **T-16 — `BreakOverlay` (modo descanso no cockpit)**
  - **Objetivo:** estado de pausa: contagem regressiva, presets 5/15/20 visíveis, ações
    retomar / pular pausa / parar; estado "pausa encerrada" quando o countdown zera sem ciclo
    pomodoro. Referência de comportamento: `FocoBreak` de `foco.jsx` (versão compacta;
    fullscreen é fase 05).
  - **Entradas:** T-14.
  - **Entregável:** `components/timer/BreakOverlay.tsx` + testes Vitest.
  - **Acceptance check:** 2 testes red→green — **(CA-32)** `BreakOverlay.aposPause`: após
    pausar pela UI (API mockada), a interface troca pro modo descanso exibindo a contagem
    regressiva e as ações retomar/pular pausa; **(CA-34.2)** `BreakOverlay.pausaEncerrada`:
    pausa zerada com contexto countup/countdown NÃO chama resume sozinha e exibe o estado
    "pausa encerrada" com ações retomar/parar.

- **T-17 — `TodayList` (lista "Hoje")**
  - **Objetivo:** entries do dia (GET /entries default hoje), mais recente primeiro, com hora de
    início, duração mono, botão continuar por linha, entry rodando destacada em accent e total
    do dia. Referência: `Timesheet` de `parts.jsx`.
  - **Entradas:** T-13, T-14.
  - **Entregável:** `components/timer/TodayList.tsx` + rota "Hoje" no cockpit + teste Vitest.
  - **Acceptance check:** 2 testes red→green — **(CA-35)** `TodayList.render`: com fixture de
    3 entries (1 rodando), renderiza início + duração + botão continuar nas fechadas e destaque
    accent + stop na rodando; **(CA-35.1)** `TodayList.timezone`: a chamada de listagem inclui
    `tz=` igual a `Intl.DateTimeFormat().resolvedOptions().timeZone` (stub de fetch).

- **T-18 — `EntryModal` (nova entrada / edição manual) + `PomodoroSettings` (Ajustes)**
  - **Objetivo:** modal de criar/editar entry manual (descrição, projeto, task, tags,
    início/fim) e grupo Pomodoro nos Ajustes (slider foco 15–90, presets pausa 5/15/20 + custom),
    persistindo via API. Referências: botão "Nova entrada" de `cockpit.jsx`, `SGroup Pomodoro`
    de `settings.jsx`.
  - **Entradas:** T-12, T-08, T-14.
  - **Entregável:** `components/timer/EntryModal.tsx`, `components/settings/Pomodoro.tsx`
    (seção montada em `routes/cockpit/Settings.tsx` da fase 00) + testes
    Vitest (validações de formulário e chamadas de API corretas).
  - **Acceptance check:** testes Vitest dos 2 componentes red→green (formulário inválido não
    chama API; submit chama POST/PATCH/PUT com payload do contrato). Os CAs E2E ficam no T-19.

- **T-19 — E2E Playwright (fluxos centrais da fase)**
  - **Objetivo:** suíte E2E contra app real local (backend + Postgres de teste), cobrindo os
    critérios de ponta a ponta + o fluxo nº1 do `TDD.md §4` (cria projeto → timer → para →
    entrada com projeto certo).
  - **Entradas:** T-14–T-18; seed de teste (usuário + workspace + projeto).
  - **Entregável:** `frontend/e2e/cronometro.spec.ts`.
  - **Acceptance check:** 3 testes E2E red→green — **(CA-36)** editar horários de uma entry
    pelo modal atualiza a duração exibida na lista (valor recalculado pelo servidor);
    **(CA-37)** reload com timer rodando restaura a barra com elapsed correto (tolerância ≤2s);
    **(CA-38)** mudar a pausa nos Ajustes pra 15 min → próxima pausa inicia com 15:00.

- **T-20 — Fechamento da fase**
  - **Objetivo:** suíte completa verde (`go test ./...` + Vitest + Playwright), `gofmt`/`go vet`
    limpos, status da fase 03 marcado no `specs/INDEX.md`.
  - **Entradas:** T-01–T-19.
  - **Entregável:** commit de fechamento + INDEX atualizado.
  - **Acceptance check:** todas as suítes verdes numa execução única; matriz abaixo sem CA
    órfão; **(RNF-04)** check leve de latência no teste de integração do `GET /timer` —
    asserção de que a resposta local fica < 100ms (sem benchmark formal; falhou → investigar
    antes de fechar).

---

## Matriz CA → tarefa (todo critério coberto por ≥1 tarefa)

| CA | Tarefa | | CA | Tarefa |
|---|---|---|---|---|
| CA-01 | T-04 | | CA-22 | T-11 |
| CA-01.1 | T-04 | | CA-22.1 | T-11 |
| CA-02 | T-05 | | CA-23 | T-11 |
| CA-03 | T-01 | | CA-24 | T-12 |
| CA-03.1 | T-05 | | CA-24.1 | T-12 |
| CA-04 | T-05 † | | CA-24.2 | T-12 |
| CA-05 | T-07 † | | CA-25 | T-09 † |
| CA-06 | T-05 † | | CA-25.1 | T-09 |
| CA-06.1 | T-10 | | CA-26 | T-10 |
| CA-07 | T-09 | | CA-27 | T-06* † |
| CA-08 | T-09 | | CA-28 | T-08 |
| CA-09 | T-10 | | CA-29 | T-06 † |
| CA-10 | T-10 | | CA-29.1 | T-11.1 + T-12 |
| CA-11 | T-10 | | CA-29.2 | T-11.1 + T-12 |
| CA-12 | T-06 | | CA-29.3 | T-12 |
| CA-13 | T-06 | | CA-30 | T-11 |
| CA-13.1 | T-06 | | CA-31 | T-15 |
| CA-14 | T-12 | | CA-32 | T-16 |
| CA-14.1 | T-12 | | CA-33 | T-14 |
| CA-15 | T-12 | | CA-34 | T-14 |
| CA-16 | T-12 | | CA-34.1 | T-14 |
| CA-17 | T-13 | | CA-34.2 | T-16 |
| CA-18 | T-08 | | CA-35 | T-17 |
| CA-19 | T-08 | | CA-35.1 | T-17 |
| CA-20 | T-05 | | CA-36 | T-19 |
| CA-21 | T-11 | | CA-37 | T-19 |
| | | | CA-37.1 | T-14 |
| | | | CA-38 | T-19 |

\* CA-27 (countdown sem `target_seconds` → 400): teste de validação no start do T-06.
† Cobertos no nível do `timer.Service` (unit) na tarefa indicada **e** no nível do contrato
HTTP (httptest) no T-11.1 — start/stop são o comportamento central da fase, as duas camadas
têm teste próprio.
