# spec.md — Fase 03 · Cronômetro

> Núcleo de cronometragem do Timetrack **num único device, via REST**. Count up, countdown,
> Pomodoro configurável, entrada manual, continuar entrada, **pause → pausa Pomodoro automática**
> (no próprio device — broadcast entre devices é a fase 04). O **servidor é a fonte da verdade
> do tempo**: toda duração é calculada no backend; o cliente só renderiza.
> Depende de: fase 01 (auth/sessão/workspace) e fase 02 (clients/projects/tasks).

---

## 1. Mapa pro problema central (`REQUISITOS.md §2`)

| Problema central | O que esta fase entrega |
|---|---|
| 1. Trackear tempo **rápido**, separando cliente/projeto | Barra de timer global (descrição + projeto + play/stop), continuar entrada em 1 clique, entrada manual |
| 3. Pausei → **inicia Pomodoro configurável (5/15/20)** | Pause → pausa Pomodoro automática **neste device**; config persistida por usuário. O "em todos os devices" entra na fase 04 |
| 2. Sync Google Calendar | **Fora** desta fase (06) — mas `source` da entry já reserva `'calendar'` |

## 2. User stories

- **US-01** — Como usuário, quero iniciar um cronômetro (count up) digitando uma descrição e
  escolhendo projeto/task/tags, pra registrar no que estou trabalhando sem fricção.
- **US-02** — Como usuário, quero parar o cronômetro e ter a duração calculada pelo servidor,
  pra confiar no registro mesmo se meu device dormir ou o relógio dele estiver errado.
- **US-03** — Como usuário, quero **pausar** o trabalho e cair automaticamente numa **pausa
  Pomodoro** com a duração que configurei (5/15/20 min ou custom), pra descansar sem apertar
  mais nada. (Comportamento central do produto.)
- **US-04** — Como usuário, quero **retomar** depois da pausa e continuar a mesma
  descrição/projeto/task, pra não redigitar contexto.
- **US-05** — Como usuário, quero rodar um **countdown** com duração escolhida, pra fazer
  blocos de tempo fechados (timeboxing).
- **US-06** — Como usuário, quero um **modo Pomodoro** que usa minha duração de foco
  configurada como alvo e emenda na pausa, pra ciclar foco/descanso sem pensar.
- **US-07** — Como usuário, quero criar e editar **entradas manuais** (início/fim), pra
  registrar tempo que esqueci de cronometrar.
- **US-08** — Como usuário, quero **continuar uma entrada anterior** (clona
  descrição/projeto/task/tags e inicia), pra repetir trabalhos recorrentes em 1 clique.
- **US-09** — Como usuário, quero ver a lista **"Hoje"** com as entradas do dia e a **barra de
  timer global sempre visível** no cockpit, pra saber o que rodou e o que está rodando.
- **US-10** — Como usuário, quero **configurar as durações do Pomodoro** (foco e pausa) nos
  Ajustes, persistidas na minha conta, pra valer em qualquer device que eu logar.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* O sistema DEVE sempre filtrar `time_entries` por `workspace_id`: a
  listagem de entries de um workspace NÃO DEVE retornar nenhuma entry de outro workspace,
  mesmo do mesmo usuário. *(vazamento = bug crítico)*
- **CA-01.1** *(ubíquo)* A listagem de entries DEVE retornar apenas entries do **próprio
  usuário da sessão** (v1 single-user por design): entries de outros membros do mesmo
  workspace NÃO aparecem — agregação por equipe e visibilidade de projetos privados de
  terceiros ficam pras fases 07+.
- **CA-02** *(ubíquo)* O sistema DEVE sempre calcular `duration_seconds` no servidor como
  `stopped_at − started_at` (em segundos inteiros); qualquer `duration_seconds` enviado pelo
  cliente DEVE ser ignorado.
- **CA-03** *(ubíquo)* O sistema DEVE garantir no máximo 1 entry rodando (`stopped_at IS NULL`)
  por usuário, inclusive sob 2 inserções concorrentes (constraint no banco, não só na aplicação).
- **CA-03.1** *(ubíquo)* Toda escrita de timer/entry é em **nome do usuário da sessão**:
  `user_id` vem do cookie; qualquer `user_id` no payload DEVE ser ignorado.

### 3.2 Timer — eventos (event-driven)

- **CA-04** QUANDO o usuário envia `POST /api/v1/timer/start` em modo `countup` com descrição e
  projeto válidos, o sistema DEVE criar uma `time_entry` com `source='timer'`,
  `started_at` = relógio do servidor e `stopped_at` nulo, e retornar `201` com o estado do timer.
- **CA-05** QUANDO o usuário inicia um timer com outro já rodando, o sistema DEVE parar o atual
  (gravando `stopped_at` e `duration_seconds`) e iniciar o novo na mesma operação, restando
  exatamente 1 entry rodando.
- **CA-06** QUANDO o usuário envia `POST /api/v1/timer/stop` com timer rodando, o sistema DEVE
  gravar `stopped_at` (relógio do servidor), calcular `duration_seconds` e retornar `200` com a
  entry finalizada e estado `null`.
- **CA-06.1** QUANDO `POST /api/v1/timer/stop` chega **durante uma pausa Pomodoro**, o sistema
  DEVE encerrar a pausa e deixar o estado ocioso (`state: null`), **sem retomar** nem criar
  nova entry.
- **CA-07** QUANDO o usuário envia `POST /api/v1/timer/pause` com timer de trabalho rodando, o
  sistema DEVE parar a entry (com duração calculada) **e iniciar automaticamente a pausa
  Pomodoro** com o `break_minutes` persistido do usuário, retornando `200` com estado
  `mode='break'` e `remaining_seconds = break_minutes × 60`.
- **CA-08** QUANDO o pause inclui `break_minutes` no payload (preset 5/15/20 ou custom), o
  sistema DEVE usar esse valor **só nesta pausa**, sem alterar a configuração persistida.
- **CA-09** QUANDO o usuário envia `POST /api/v1/timer/resume` durante uma pausa, o sistema
  DEVE encerrar a pausa e criar uma **nova** entry clonando descrição, `project_id`, `task_id` e
  `tags` da entry pausada, com novo `started_at` do servidor.
- **CA-10** QUANDO o usuário retoma um `countdown` ou um foco de Pomodoro pausado, o novo
  timer DEVE iniciar com `target_seconds` igual ao **tempo restante** no momento do pause
  (ex.: countdown de 1200s pausado aos 300s decorridos → retoma com alvo 900s).
- **CA-11** QUANDO o usuário envia `POST /api/v1/timer/break/skip` durante uma pausa, o sistema
  DEVE encerrar a pausa imediatamente e retomar o trabalho (mesma clonagem do resume).
- **CA-12** QUANDO o usuário inicia modo `countdown` com `target_seconds` válido, o
  `GET /api/v1/timer` DEVE retornar `remaining_seconds` calculado pelo relógio do servidor
  (`target_seconds − elapsed_seconds`, nunca negativo).
- **CA-13** QUANDO o usuário inicia modo `pomodoro`, o sistema DEVE usar o `focus_minutes`
  persistido do usuário como alvo (`target_seconds = focus_minutes × 60`).
- **CA-13.1** QUANDO o stop/pause chega **depois** do alvo de countdown/foco vencido (overrun —
  ex.: iPad dormiu), o sistema DEVE gravar a duração **real** (`stopped_at − started_at`),
  **sem clampar ao alvo** (decisão de produto da v1; o front mitiga via CA-37.1).

### 3.3 Entradas manuais e continuar (event-driven)

- **CA-14** QUANDO o usuário cria uma entrada manual
  (`POST /api/v1/workspaces/{workspaceId}/entries`) com `started_at`/`stopped_at` válidos, o
  sistema DEVE persistir com `source='manual'` e `duration_seconds` calculado no servidor,
  retornando `201`.
- **CA-14.1** QUANDO a listagem (`GET /api/v1/workspaces/{workspaceId}/entries`) é chamada sem
  `from`/`to`, o recorte "hoje" DEVE usar o fuso IANA do parâmetro `tz` (ausente → UTC): uma
  entry iniciada às 22:00 BRT (01:00 UTC do dia seguinte) DEVE aparecer no "Hoje" de
  `America/Sao_Paulo`, e não no dia UTC seguinte.
- **CA-15** QUANDO o usuário edita os horários de uma entry
  (`PATCH /api/v1/workspaces/{workspaceId}/entries/{id}`), o sistema DEVE recalcular
  `duration_seconds` no servidor e retornar a entry atualizada.
- **CA-16** QUANDO o usuário envia `DELETE /api/v1/workspaces/{workspaceId}/entries/{id}` de
  uma entry sua do workspace, o sistema DEVE removê-la e ela NÃO DEVE mais aparecer na
  listagem (`204`).
- **CA-17** QUANDO o usuário envia `POST /api/v1/workspaces/{workspaceId}/entries/{id}/continue`,
  o sistema DEVE clonar descrição, `project_id`, `task_id` e `tags` da entry e iniciar um count
  up com `source='timer'` (aplicando CA-05 se houver timer rodando).

### 3.4 Configuração do Pomodoro (event-driven / optional)

- **CA-18** QUANDO o usuário envia `PUT /api/v1/settings/pomodoro` com valores válidos, o
  sistema DEVE persistir por usuário e o `GET` seguinte DEVE retornar exatamente os valores salvos.
- **CA-19** ONDE o usuário nunca salvou configuração de Pomodoro, o sistema DEVE responder o
  `GET` e aplicar nas pausas os defaults `focus_minutes=25`, `break_minutes=5`.
- **CA-20** ONDE o start não informa `project_id` nem `task_id`, o timer DEVE iniciar mesmo
  assim (projeto/task/tags/descrição são opcionais na entry).

### 3.5 Estado do timer (state-driven)

- **CA-21** ENQUANTO um timer estiver rodando, `GET /api/v1/timer` DEVE retornar o modo, a entry
  ativa, `elapsed_seconds` calculado pelo relógio do servidor e `server_time`, permitindo ao
  cliente renderizar o tick localmente sem requisição por segundo.
- **CA-22** ENQUANTO uma pausa Pomodoro estiver ativa, `GET /api/v1/timer` DEVE retornar
  `mode='break'`, `remaining_seconds` e a referência à entry pausada (pra retomar com contexto).
- **CA-22.1** ENQUANTO uma pausa estiver **expirada** sem resume/skip/stop, `GET /api/v1/timer`
  DEVE continuar retornando `mode='break'` com `remaining_seconds = 0` (**nunca** negativo),
  até o usuário (ou o auto-resume do CA-34.1) agir.
- **CA-23** ENQUANTO não houver timer nem pausa ativa, `GET /api/v1/timer` DEVE retornar
  `200 {"state": null}`.

### 3.6 Erros e proteções (unwanted)

- **CA-24** SE uma entrada manual chegar com `stopped_at` ≤ `started_at`, o sistema NÃO DEVE
  persistir nada e DEVE retornar `400 {"error":{"code":"validation_error", ...}}`.
- **CA-24.1** SE `PATCH` ou `DELETE` atingirem a entry **rodando** (`stopped_at IS NULL`), o
  sistema NÃO DEVE alterar nada e DEVE retornar `409 {"error":{"code":"entry_running", ...}}` —
  mudança no timer ativo passa pelas rotas `/timer` (stop), nunca pelo CRUD.
- **CA-24.2** SE `DELETE` atingir a entry referenciada como contexto de retomada
  (`resume_entry_id`) de uma **pausa ativa**, o sistema DEVE retornar
  `409 {"error":{"code":"entry_in_use", ...}}` sem remover (proteção do resume).
- **CA-25** SE `POST /api/v1/timer/pause` chegar sem timer rodando, o sistema DEVE retornar
  `409 {"error":{"code":"no_active_timer", ...}}`.
- **CA-25.1** SE `POST /api/v1/timer/pause` chegar com `break_minutes` fora de 1–60, o sistema
  DEVE retornar `400` e o timer de trabalho DEVE **continuar rodando intacto** (nenhuma entry
  é fechada).
- **CA-26** SE `POST /api/v1/timer/resume` (ou `break/skip`) chegar sem pausa ativa, o sistema
  DEVE retornar `409 {"error":{"code":"no_active_break", ...}}`.
- **CA-27** SE `POST /api/v1/timer/start` em modo `countdown` chegar sem `target_seconds` ou com
  valor ≤ 0, o sistema DEVE retornar `400` sem criar entry.
- **CA-28** SE `PUT /api/v1/settings/pomodoro` chegar com valores fora dos limites
  (`focus_minutes` fora de 15–90 ou `break_minutes` fora de 1–60), o sistema DEVE retornar `400`
  e preservar a configuração anterior.
- **CA-29** SE o payload referenciar `project_id` ou `task_id` de **outro workspace**, o sistema
  NÃO DEVE criar/atualizar nada e DEVE retornar `404 {"error":{"code":"not_found", ...}}`
  (anti-vazamento na escrita).
- **CA-29.1** SE o `workspace_id` da requisição (payload do `timer/start` ou `{workspaceId}`
  das rotas de entries) referenciar workspace do qual o **usuário da sessão não é membro**, o
  sistema DEVE retornar `404 {"error":{"code":"not_found", ...}}` **sem expor nenhum dado**
  (middleware de membership da fase 02). *(vazamento entre workspaces = bug crítico)*
- **CA-29.2** SE o usuário tiver role `viewer` no workspace, qualquer escrita desta fase
  (`timer/start` naquele workspace, `POST`/`PATCH`/`DELETE` de entries, `continue`) DEVE
  retornar `403 {"error":{"code":"forbidden", ...}}` (role gate da fase 02).
- **CA-29.3** SE `PATCH`/`DELETE`/`continue` referenciarem entry de **outro usuário** (mesmo
  no mesmo workspace), o sistema DEVE retornar `404` — edição/exclusão/continuar só nas
  próprias entries (consistente com CA-01.1).
- **CA-30** SE qualquer rota desta fase for chamada sem cookie de sessão válido, o sistema DEVE
  retornar `401 {"error":{"code":"unauthorized", ...}}`.

### 3.7 Frontend (Vitest / Playwright)

- **CA-31** ENQUANTO um timer estiver rodando, a barra de timer global do cockpit DEVE exibir a
  descrição, o chip do projeto (`.tt-chip` + `.tt-dot` na cor do projeto), o tempo decorrido em
  mono (`h:mm:ss`) e o botão stop com indicador pulsante (`.tt-pulse`).
- **CA-32** QUANDO o usuário pausa pela UI, a interface DEVE trocar pro estado de pausa
  ("modo descanso") exibindo a contagem regressiva da pausa e as ações retomar / pular pausa.
- **CA-33** QUANDO um countdown chega a zero no cliente, o front DEVE acionar o stop
  automaticamente (`POST /timer/stop`), sem intervenção do usuário.
- **CA-34** QUANDO o foco do Pomodoro chega a zero no cliente, o front DEVE acionar o pause
  automaticamente (`POST /timer/pause`), emendando na pausa configurada.
- **CA-34.1** QUANDO a **pausa** chega a zero no cliente E o contexto de retomada é
  `pomodoro_focus`, o front DEVE acionar **exatamente 1** `POST /timer/resume` automaticamente
  (ciclo foco ↔ pausa do US-06, sem intervenção).
- **CA-34.2** QUANDO a pausa chega a zero E o contexto de retomada é `countup`/`countdown`, o
  front NÃO DEVE retomar sozinho: exibe o estado "pausa encerrada" com as ações
  retomar/parar (o servidor segue reportando `mode='break'` com `remaining_seconds=0` — CA-22.1).
- **CA-35** A lista **"Hoje"** DEVE exibir as entries do dia (mais recente primeiro) com hora de
  início, duração em mono e botão **continuar** por linha; a entry rodando DEVE aparecer
  destacada em accent com botão stop.
- **CA-35.1** A lista "Hoje" DEVE chamar a listagem enviando
  `tz = Intl.DateTimeFormat().resolvedOptions().timeZone` do navegador (recorte do dia no fuso
  do usuário — CA-14.1).
- **CA-36** QUANDO o usuário edita os horários de uma entry pela UI (modal de edição), a duração
  exibida na lista DEVE refletir o valor recalculado pelo servidor após salvar.
- **CA-37** QUANDO a página recarrega com um timer rodando, a barra DEVE restaurar o estado a
  partir do `GET /api/v1/timer` exibindo o `elapsed` correto (tolerância ≤ 2s), sem zerar.
- **CA-37.1** QUANDO a aba volta a ficar visível (`visibilitychange`/focus) com um alvo já
  vencido (countdown/foco/pausa), o front DEVE re-sincronizar com `GET /timer` e disparar
  **imediatamente e exatamente 1 vez** a transição pendente (stop / pause / regra do CA-34.1) —
  mitigação do overrun do CA-13.1.
- **CA-38** QUANDO o usuário altera a duração da pausa nos Ajustes (preset ou custom) e salva,
  a próxima pausa iniciada DEVE usar o novo valor.

## 4. Requisitos funcionais (resumo)

- **RF-01** Timer com 3 modos: `countup`, `countdown` (alvo escolhido), `pomodoro` (alvo =
  `focus_minutes` do usuário). Operações: start, stop, pause, resume, skip break.
- **RF-02** Pause de trabalho → pausa Pomodoro automática (duração persistida ou override por
  chamada). Resume/skip → nova entry clonada da pausada (restante do alvo quando aplicável).
- **RF-03** Entry carrega `description`, `project_id?`, `task_id?`, `tags[]`, `source`
  (`timer|manual|calendar|claude` — os 2 últimos só reservados nesta fase).
- **RF-04** CRUD de entries manuais (criar/editar/excluir) + continuar entrada anterior —
  sempre em nome do usuário da sessão; editar/excluir/continuar só as próprias entries;
  `viewer` não escreve (403).
- **RF-05** `GET /api/v1/timer` devolve o estado canônico do timer (única fonte pro front).
- **RF-06** `pomodoro_settings` por usuário: `focus_minutes` (default 25), `break_minutes`
  (default 5; presets 5/15/20 + custom).
- **RF-07** UI cockpit: barra de timer global sempre visível no topo + lista "Hoje" + modal de
  entrada/edição manual + grupo Pomodoro nos Ajustes.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** nenhuma dependência Go nova além do baseline (chi, pgx, google/uuid);
  nenhuma lib JS nova além do baseline (zustand). Sem lib de datas no front (Intl/Date nativos).
- **RNF-02 Relógio:** o cliente nunca acumula tempo como verdade — deriva a exibição de
  `started_at`/`server_time` (offset de skew calculado a cada resposta). Após qualquer re-sync,
  desvio de exibição ≤ 1s — **meta de implementação**; o E2E do CA-37 usa tolerância ≤ 2s de
  propósito (instabilidade de ambiente de teste). Os dois números são intencionais e distintos.
- **RNF-03 Sem polling agressivo:** tick de exibição é local (1 `setInterval`); re-sync com o
  servidor só em `visibilitychange`/focus da aba e a cada 60s no máximo.
- **RNF-04 Performance:** `GET /api/v1/timer` responde em < 100ms (p95) no ambiente local.
- **RNF-05 Pronto pra fase 04:** toda mutação de timer passa por um único serviço de domínio que
  retorna o objeto `TimerState` canônico (idêntico ao do GET) — a fase 04 só assina esse retorno
  pra broadcast via WebSocket, **sem refatorar** esta fase.
- **RNF-06 Multi-tenant:** toda query desta fase filtra por `workspace_id` (constituição §8.4),
  e toda rota com workspace passa pela cadeia sessão → membership → role gate da fase 02
  (CA-29.1/29.2).

## 6. Fora de escopo (fronteiras explícitas)

- **Sync em tempo real entre dispositivos** (WebSocket, broadcast de start/stop/pause,
  reconciliação ao reconectar) → **fase 04**. Aqui o estado vem por REST e vale pro device atual.
- **Modo Foco fullscreen** (flip-clock iPad, Wake Lock, seletor touch) → **fase 05**. A UI de
  pausa desta fase é a versão compacta do cockpit; `foco.jsx` é só referência de comportamento.
- **Google Calendar 2-way** → **fase 06** (`source='calendar'` apenas reservado no enum).
- **Relatórios / agregações** (4 ângulos, metas) → **fase 07**.
- **API token `tt_` / conector Claude** → **fase 08** (`source='claude'` apenas reservado).
- **Deploy real na VPS** → **fase 09**.
- **Multiplayer real** (criar/pedir tasks entre colaboradores, presença) → fora do MVP.
- **Agrupamento de entries na lista "Hoje"** (consolidar entries de mesma
  descrição+projeto+task, estilo Toggl) → **fora da v1**: a lista é plana, ordenada por
  `started_at DESC` (CA-35). Decisão explícita, não esquecimento.
- **Entries de outros membros do workspace** na listagem/relatos → fora desta fase (CA-01.1);
  agregação por equipe entra com relatórios (07) ou multiplayer.
- **Notificações push/som** de fim de Pomodoro → fora desta fase (UI in-app apenas).
- **Offline-first do timer** → fora desta fase.
