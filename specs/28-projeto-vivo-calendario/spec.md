# spec.md — Fase 28 · Projeto vivo (cards com vida + Overview) + Calendário por projeto

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Quinta fase da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`). Cobre **RF-21,
> RF-22, RF-23** (calendário do projeto), formaliza **RF-01** (a aba shell ganha Overview e
> Calendar) + entrega **RF-02** (Overview) + os **cards vivos** da tela de Projetos.
> **Fonte visual:** `mockups/projeto-workspace.html` — `renderList` (278-293), `renderProject`
> (296-312), `renderOverview` (314-366), `renderCal` (483-502). **Buildar pro mockup.**
>
> **Decisões travadas (MILESTONE-3 §3/§4/§9 — NÃO reabrir):**
> 1. Card de projeto vivo (deadline · % done · horas REAIS · "tem algo hoje") — mata o `0h`
>    hardcoded desde a fase 02.
> 2. Calendário do projeto = **read-only** (deadlines de task/objetivo/projeto + overlay do GCal
>    que já existe da fase 06). **Sem write-back** (decisão herdada).
> 3. **Sem associação evento↔projeto** — o overlay do GCal é o calendário `primary` do usuário,
>    sobreposto por data (limitação herdada da fase 06). Declarada explicitamente.
> 4. Única migração nova: **`0025_projects_deadline`** (`ALTER TABLE projects ADD COLUMN deadline`).

---

## 1. Mapa pro problema central (MILESTONE-3 §1/§5.1/§5.5)

A tela de Projetos mostra **cards mortos** (`0h` hardcoded desde a fase 02 — `MILESTONE-3 §1`).
O `/board` (fase 12) já tem os números REAIS (`tracked_seconds`, `task_count`, `done_count` por
objetivo e projeto), mas **por projeto** — chamar o `/board` N vezes na lista de projetos viola
o RNF-05 (<200ms). Esta fase:

1. Adiciona `projects.deadline` (migração `0025`) — alimenta o card vivo, o "tem algo hoje" e o
   calendário do projeto.
2. Cria um **endpoint de resumo** (`GET .../projects/summary`) — 1 request que devolve os
   contadores leves de TODOS os projetos visíveis do workspace, pra alimentar os cards vivos sem
   N+1.
3. Cria o **endpoint de calendário do projeto** (`GET .../projects/{id}/calendar?month=`) — agrega
   os itens datados do projeto (tasks com `due_date`, objetivos com `target_date` [fase 29,
   aditivo], `projects.deadline`) por mês.
4. No frontend, ADICIONA as abas **Overview** e **Calendar** ao tab-shell da fase 24, e faz os
   **cards vivos** na lista de projetos.

A fase **estende** as fases 12 (`/board`), 24 (tab-shell), 06 (`/calendar/events`) e 26 (notas,
aditivo) — não as reescreve. `target_date` de objetivo vem da fase 29 (dependência **fraca**: se a
29 não estiver no build, o calendário omite o `kind:'objective'` — ver §6/CA-15.1).

## 2. User stories

- **US-01** — Como dono/membro, quero ver os **cards de projeto vivos** (horas reais, % de tasks
  concluídas, prazo do projeto, "tem algo hoje"), pra saber o estado de cada projeto em 0 clique
  (object permanence — TDAH).
- **US-02** — Como usuário, quero **definir um prazo (deadline) no projeto** (ou deixar sem), pra
  marcar a entrega na grade do tempo (deadlines = marcos locais, decisão herdada).
- **US-03** — Como usuário, quero a aba **Overview** do projeto com KPIs (tempo total · esta semana
  · pendentes/backlog · % done), **próximos prazos**, **andamento por status + goals** e **notas
  recentes**, pra ter o pulso do projeto numa tela.
- **US-04** — Como usuário, quero a aba **Calendar** do projeto: um mês com os **itens datados do
  projeto** (deadlines de tasks/objetivos + prazo do projeto) na cor do projeto, **sobreposto** aos
  meus eventos do Google Calendar (read-only), navegável por mês.
- **US-05** — Como usuário, quero **clicar num item datado** (task/objetivo) no calendário ou em
  "Próximos prazos" e **abrir** a task/objetivo correspondente, pra agir num toque (RF-23).
- **US-06** — Como viewer do workspace (ou `access=view` num projeto), quero **só ler** o Overview,
  o calendário e os cards — sem editar deadline nem nada.

## 3. Critérios de aceite (EARS)

### `projects.deadline` — API (estende a fase 02)

> O CRUD de projetos é da fase 02 (`internal/projects`). Esta fase ADICIONA o campo `deadline` ao
> `CreateInput`/`PatchInput` (tri-state) e ao corpo do projeto, sem regredir o contrato.

- **CA-01** *(event-driven)* — QUANDO um `POST .../projects` ou `PATCH .../projects/{id}` inclui
  `deadline` com uma data ISO `YYYY-MM-DD` válida, o sistema DEVE persistir o prazo e devolver
  `deadline` no corpo do projeto (em `GET .../projects`, `GET .../projects/{id}` e
  `GET .../projects/{id}/board`). `deadline` é **tri-state** no PATCH (ausente mantém, `null`
  remove o prazo).
- **CA-01.1** *(unwanted)* — SE `deadline` for informado e **não** for uma data `YYYY-MM-DD`
  parseável (formato inválido, mês 13, dia 32, string não-data), o sistema NÃO DEVE escrever e
  DEVE retornar `400` com `error.code="validation_error"` (`time.Parse("2006-01-02", …)` antes do
  SQL, padrão `due_date` da fase 02 — nunca `500` por erro de cast no Postgres).
- **CA-02** *(state-driven)* — Um projeto sem `deadline` (coluna `NULL`) é **válido** (a maioria
  dos projetos não tem prazo — decisão herdada §3.5 do CLAUDE.md: "projeto pode não ter
  deadline"). O corpo do projeto traz `deadline: null` nesse caso.
- **CA-03** *(unwanted — herda fase 02)* — SE um usuário sem permissão de escrita no projeto
  (viewer do ws, `access=view`, ou projeto invisível/arquivado conforme as regras da fase 02)
  enviar `PATCH .../projects/{id}` com `deadline`, o sistema DEVE responder o erro da fase 02
  (`403`/`404`/`400`) e NÃO alterar o prazo. (O gate é o `getForWrite` [visibilidade→404] +
  `canEditMeta` [permissão→403] da fase 02 — os mesmos símbolos REAIS de `internal/projects/
  service.go`; **não** há `loadProjectGate` no pacote `projects`. Esta fase não muda a autorização,
  só adiciona o campo.)

### Resumo dos projetos (cards vivos) — API

> Endpoint NOVO `GET .../projects/summary` — 1 request com os contadores leves de TODOS os projetos
> visíveis, pra os cards vivos (RNF-05 sem N+1).

- **CA-04** *(event-driven)* — QUANDO um membro do workspace envia `GET .../projects/summary`, o
  sistema DEVE retornar, **num único request** (sem N+1 — RNF-04), a lista dos projetos
  **visíveis** ao usuário (não arquivados por padrão; ver CA-08) com, por projeto:
  `id, name, color, client_id, deadline, task_count, done_count, open_count,
  tracked_seconds, due_today_count`. Onde:
  - `task_count` = total de tasks do projeto; `done_count` = tasks `status='done'`;
    `open_count` = `task_count - done_count`.
  - `tracked_seconds` = soma das entries FECHADAS do **`user_id` da sessão** no workspace com
    `project_id` = este projeto (mesmo bucket "projeto" do `/board` — D-06 da fase 12; **não** das
    outras pessoas — CA-09).
  - `due_today_count` = nº de tasks do projeto com `due_date` = **hoje no fuso da requisição**
    (`?tz=IANA`) e `status != 'done'` — alimenta o "tem algo hoje" do card.
- **CA-04.1** *(state-driven — visibilidade)* — O summary DEVE listar apenas projetos que o usuário
  **enxerga** (visibilidade da fase 02: `visibility='shared' OR owner_user_id = $user`). Projetos
  `private` de **outro** usuário NÃO aparecem (mesmo o owner do workspace só vê os `shared` e os
  seus próprios — regra da fase 02, CA-09; ver §6 sobre owner-ws). Multi-tenant: só o
  `workspace_id` da rota (CA-09).
- **CA-05** *(state-driven)* — Quando um projeto tem 0 tasks, o summary devolve
  `task_count=0, done_count=0, open_count=0, due_today_count=0` e o `tracked_seconds` do projeto
  (que pode ser `> 0` se houver entries com `project_id` e sem `task_id`). O cliente deriva o
  `done_pct = round(done_count/task_count*100)`, **0 quando `task_count=0`** (não dividir por
  zero).
- **CA-06** *(event-driven — `?tz`)* — QUANDO a requisição inclui `?tz=America/Sao_Paulo` (IANA), o
  `due_today_count` DEVE ser computado com "hoje" **no fuso informado**; `tz` ausente → `UTC`
  (default, padrão da fase 03/14). SE `tz` for um IANA inválido, o sistema DEVE retornar `400`
  `validation_error` (validação `time.LoadLocation` antes do SQL, padrão das fases anteriores).
- **CA-07** *(perf — verificação informativa, fora do red→green)* — Com **30 projetos**, 300 tasks
  e 1.000 entries num workspace, `GET .../projects/summary` DEVE responder em **< 200 ms** no
  ambiente local de teste (melhor de 3 — RNF-05). Implementado com **3 queries fixas** (projetos
  visíveis; contadores de task por projeto via `GROUP BY project_id`; soma de tempo por projeto via
  `GROUP BY project_id`), montadas em memória em O(n) — **N+1 proibido**. Nasce verde; se falhar, a
  correção é índice/SQL — não segue o ciclo red→green (exceção documentada, igual CA-18 da fase 12).
- **CA-08** *(optional — arquivados)* — QUANDO a requisição inclui `?archived=true`, o summary DEVE
  incluir também os projetos arquivados (visíveis ao usuário); sem o flag, os arquivados ficam
  **fora** (padrão da listagem de projetos da fase 02). (O front da lista de Projetos hoje não
  mostra arquivados por padrão — RNF-05.)

### Calendário do projeto — API

> Endpoint NOVO `GET .../projects/{projectId}/calendar?month=YYYY-MM` (ou `from`/`to`) — agrega só
> os itens datados do **próprio projeto** (RF-21). O overlay do GCal NÃO entra neste endpoint — vem
> do `GET /api/v1/calendar/events` existente (fase 06), sobreposto no front (CA-13).

- **CA-09** *(ubíquo — teste de vazamento OBRIGATÓRIO)* — O sistema DEVE sempre filtrar os itens do
  calendário do projeto pelo `workspace_id` E pela visibilidade do projeto (`visibility='shared' OR
  owner_user_id = $user`): projeto invisível → `404` (CA-12); todo `SUM`/`SELECT` de tempo soma só
  entries do `user_id` da sessão. Referenciar um projeto de **outro** workspace → `404`. Vazar
  entre tenants = bug crítico (SDD §8.4).
- **CA-10** *(event-driven — itens datados)* — QUANDO um membro que enxerga o projeto envia
  `GET .../projects/{projectId}/calendar?month=2025-06`, o sistema DEVE retornar
  `{ "items": [ {…} ], "from": "2025-06-01", "to": "2025-06-30" }` com **todos os itens datados do
  projeto cuja data cai no intervalo `[from, to]`** (inclusivo), cada item com:
  `{ "kind": "task" | "objective" | "project_deadline", "id": "<uuid>", "name": "<string>",
    "date": "YYYY-MM-DD", "status": "<string|null>" }`. Onde:
  - `kind:"task"` — uma `task` do projeto com `due_date` no intervalo; `name = task.name`,
    `status = task.status`. **Inclui** tasks `done` (a marca de prazo da task no calendário não
    some quando ela é concluída — útil pra ver o que entregou no mês; o front pode estilizar
    `done` diferente). `id` = `tasks.id`.
  - `kind:"objective"` — um objetivo do projeto com `target_date` no intervalo (**fase 29,
    aditivo**: se a coluna `objectives.target_date` não existir no schema do build, este `kind`
    é **omitido** — CA-15.1); `name = objective.name`, `status = objective.status`. `id` =
    `objectives.id`.
  - `kind:"project_deadline"` — o `projects.deadline` do próprio projeto, se cair no intervalo;
    `name = project.name`, `status = null`. `id` = `projects.id`. **No máximo 1** por projeto.
- **CA-11** *(event-driven — janela `from`/`to`)* — A janela DEVE ser resolvida de:
  `?month=YYYY-MM` → `from` = dia 1 daquele mês, `to` = último dia do mês (calculado por
  `AddDate(0,1,0)` − 1 dia, **DST-safe**, padrão da fase 14); OU `?from=YYYY-MM-DD&to=YYYY-MM-DD`
  explícitos. SE `month`/`from`/`to` forem malformados, ou `to < from`, o sistema DEVE retornar
  `400` `validation_error`. SE nenhum dos dois for informado, default = **mês corrente** (no `tz`
  informado, ou UTC).
- **CA-12** *(unwanted — visibilidade/inexistência)* — SE a rota referenciar um projeto invisível
  ao usuário (private de outro — CA-09) ou inexistente no workspace, ou `projectId` malformado
  (não-uuid), o `GET .../calendar` DEVE retornar `404` `not_found` (mesma resposta — não vazar
  existência; `uuid.Validate(projectId)` antes do SQL pra nunca dar `500`).
- **CA-13** *(state-driven — overlay GCal é separado, SEM vínculo a projeto)* — O endpoint de
  calendário do projeto **NÃO** retorna eventos do Google Calendar. O overlay vem do endpoint
  **existente** `GET /api/v1/calendar/events?from&to&tz` (fase 06 — o calendário `primary` do
  usuário, **sem** associação a projeto), buscado pelo **front** e sobreposto por data. **Não há
  vínculo evento↔projeto** (limitação herdada da fase 06, declarada): TODO projeto vê os MESMOS
  eventos do GCal do usuário no overlay. SE o usuário não tiver o GCal conectado
  (`/calendar/status → {connected:false}`), o calendário do projeto ainda funciona (só os itens
  datados do projeto, sem overlay — degrada como na fase 06).
- **CA-14** *(state-driven — read-only / sem write-back)* — O calendário do projeto é
  **somente leitura**: NÃO existe rota de criar/editar/mover item de calendário, nem write-back pro
  GCal (decisão herdada). Mover uma deadline = editar o `due_date` da task / `deadline` do projeto
  pelas rotas que já existem (fase 12/02) — não pelo calendário.

### Roles, auth e isolamento — herdados

- **CA-15** *(unwanted)* — SE a requisição chegar sem cookie de sessão válido, qualquer rota desta
  fase (`projects.deadline`, `summary`, `calendar`) DEVE retornar `401` `unauthorized`.
- **CA-15.1** *(optional — `target_date` aditivo / dependência fraca da fase 29)* — A query de
  objetivos do calendário (CA-10, `kind:"objective"`) só roda **se** a coluna
  `objectives.target_date` existir no schema do build. Implementação: o pacote detecta a presença
  da coluna **uma vez no boot** (consulta a `information_schema.columns`, cacheada) — coluna
  ausente ⇒ a query é **pulada** e o calendário omite `kind:"objective"` (nada quebra). Quando a
  fase 29 entrar (migração `0026_objectives_modes`), o `kind:"objective"` passa a aparecer **sem
  mudança de código** nesta fase. (Build order 24→26→27→28→29: na ordem real, a 28 vem ANTES da 29
  — então no deploy desta fase o `kind:"objective"` nasce **omitido**, e "liga" quando a 29
  deployar. Declarado.)

### Frontend — cards vivos + Overview + Calendar

- **CA-16** *(event-driven — cards vivos)* — A lista de Projetos (`renderList`, mockup 278-293)
  DEVE renderizar cada projeto como um card que mostra: nome + barra de cor do projeto, **`X
  pendentes · Y no total`** (de `open_count`/`task_count` do summary), o **cliente**, e — quando há
  dado — o **tempo total real** (`tracked_seconds` formatado, ex.: `142h`, NÃO `0h` hardcoded), o
  **prazo** (`deadline` formatado, quando há) e um indicador **"tem algo hoje"** quando
  `due_today_count > 0`. Clicar no card navega pro detalhe do projeto (`/projetos/{id}`). Os dados
  vêm de **um** `GET .../projects/summary` (CA-04), não de N `/board`.
- **CA-17** *(event-driven — aba Overview)* — A aba **Overview** do projeto (`renderOverview`,
  mockup 314-366) DEVE exibir:
  - **KPIs**: `Total time` (= `tracked_seconds` do projeto, do `/board`), `This week`
    (horas da semana corrente — reusa `GET .../reports?period=week` ou `/reports/week` da fase
    07/14 filtrado por projeto; ver §6), `Open` (= `open_count`, com sub-rótulo
    `N in backlog` quando a fase 24 já distingue `backlog`), `Done` (= `done_pct%`, sub
    `done_count/task_count tasks`). Todos derivados do `/board` (e do relatório p/ "this week").
  - **Upcoming** (próximos prazos): as tasks do projeto com `due_date` futuro/hoje e
    `status != 'done'`, ordenadas por data asc, **clicáveis → abrem a task** (RF-23). Vazio →
    "No deadlines set." (à prova de culpa — RNF-03).
  - **Progress**: barra por status (`backlog/todo/doing/done`) com contagem, + lista de **Goals**
    do projeto com % (reusa `objStats`/`done_pct` do `/board`, fase 12).
  - **Recent notes**: as notas mais recentes do projeto (reusa a fase 26 se presente; **aditivo**:
    se a fase 26 não estiver no build, a seção é **omitida** — CA-21).
- **CA-18** *(event-driven — aba Calendar)* — A aba **Calendar** do projeto (`renderCal`, mockup
  483-502) DEVE renderizar uma **grade do mês** (6 linhas × 7 colunas, semana começando na
  **segunda** — `(getDay()+6)%7`, padrão do `lib/calendar-layout.ts` da fase 11) com os **itens
  datados do projeto** (do `GET .../calendar?month=`) no dia certo, cada chip na **cor do projeto**
  (`border-left` na cor, padrão do mockup 491). DEVE sobrepor os eventos do **Google Calendar**
  (do `GET /api/v1/calendar/events`, fase 06) read-only — buscados na janela da **grade de 42 dias**
  (`monthRange(refDate)` da fase 11), **não** no mês estrito, pra que eventos do GCal nos dias de
  *spillover* da grade (fim do mês anterior / início do próximo, que a grade exibe) também apareçam,
  espelhando a fase 11. (Os **itens do projeto** vêm do `?month=` e caem só nos dias do próprio mês —
  CA-10/11.) Navegável por mês (‹ Hoje ›). Clicar num item de task/objetivo **abre** a entidade
  (RF-23); evento do GCal abre o `html_link` (padrão da fase 11).
- **CA-19** *(state-driven — tab-shell estende a fase 24)* — As abas DEVEM ser
  **Overview · Tasks · Notes · Goals · Calendar** (mockup 310). Esta fase **adiciona** Overview e
  Calendar ao tab-shell já existente da fase 24 (Tasks/Notes/Goals); a aba **default** ao abrir um
  projeto é **Overview** (mockup `S.tab='overview'`). A escolha de aba NÃO precisa persistir entre
  navegações (o mockup reseta pra overview ao trocar de projeto — `go()`).
- **CA-20** *(state-driven — viewer/read-only)* — ENQUANTO o usuário for `viewer` (ou `access=view`
  no projeto, ou o projeto estiver arquivado), a UI do Overview/Calendar NÃO DEVE exibir ações de
  escrita (editar deadline, criar task pelo Upcoming, etc.) — só leitura. A permissão deriva de
  **`board.project.can_write`** (CA-16 da fase 12), não da role do ws sozinha. (Backend já nega —
  CA-03/herança fase 02.)
- **CA-21** *(optional — degrade aditivo das deps fracas)* — SE a fase 26 (notas) não estiver no
  build, a seção **Recent notes** do Overview é **omitida** (não quebra). SE a fase 29 (objetivos
  com modos / `target_date`) não estiver no build, o calendário do projeto **não** mostra
  `kind:"objective"` (o backend já omite — CA-15.1) e o Overview lista os Goals só por % de tasks
  (comportamento atual da fase 12). Nenhuma dep fraca ausente pode quebrar a fase.

### E2E

- **CA-22** — Logado (stub da fase 01) num projeto `shared` com escrita: o usuário **define um
  deadline** no projeto, vê os **cards vivos** na lista de Projetos (horas reais ≠ `0h`,
  `X pendentes`, indicador "hoje" se houver), abre o projeto, vê a aba **Overview** (KPIs, Upcoming
  com pelo menos uma task de prazo, Progress), abre a aba **Calendar**, vê o **item de prazo** do
  projeto e o **due da task** no dia certo, **clica no item da task** e abre a task; navega de mês e
  a grade muda. **O overlay do GCal no E2E** usa o `calendarstub` da fase 06 (eventos ancorados no
  `timeMin`) — o teste asserta que pelo menos um evento do stub aparece sobreposto (degrade
  testado: sem conexão, só os itens do projeto). **O gating de viewer NÃO é asserido neste E2E** (o
  `googlestub` autentica só o owner — limitação herdada da fase 12/02); o CA-20 é coberto por teste
  de componente jsdom (`viewer-gating`, `can_write=false` mockado) na T-12.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-01 (parte) | Tab-shell ganha abas Overview · Calendar (formaliza as 5 abas) | CA-19 |
| RF-02 | Overview: KPIs · Upcoming · Progress+Goals · Recent notes | CA-17, 21 |
| RF-21 | Calendário do projeto: itens datados (task/objetivo/deadline) por mês | CA-10, 11, 18 |
| RF-22 | Overlay read-only dos eventos do Google Calendar (fase 06) | CA-13, 18 |
| RF-23 | Clicar item datado abre a task/objetivo | CA-17, 18 |
| Cards vivos | `projects.deadline` + resumo (horas reais · % done · "tem algo hoje") | CA-01..08, 16, 22 |

> Mapa pro MILESTONE-3 §5: RF-21/22/23 (calendário) · RF-01 parcial (abas Overview/Calendar) ·
> RF-02 (Overview) · §1 cards vivos. (RF-03..06 são da fase 24; RF-24/25 Goals com modos são da
> fase 29 — esta fase só os consome via `target_date`/`done_pct`, aditivo.)

## 5. Requisitos não-funcionais

- **RNF-01 (não regredir o TRACKING)** — `MILESTONE-3 §6 RNF-01`: cronômetro/sync/Pomodoro/foco
  intocados. Esta fase só LÊ `time_entries` (agregados, igual fase 12); zero-diff no timer.
- **RNF-02 (à prova de culpa)** — `MILESTONE-3 §6 RNF-03`: Upcoming/Calendar mostram prazos sem
  vermelho acusatório; vazio = "No deadlines set." (não "você está atrasado"). Prazo passado pode
  ser destacado, mas o tom é reagendável (decisão herdada — nada zera, nada acusa).
- **RNF-03 (leveza)** — zero dependência Go nova além do baseline (chi + pgx); validação na mão com
  stdlib (`time.Parse`/`time.LoadLocation`). Frontend reusa TanStack Query + `lib/calendar-layout.ts`
  da fase 11 (`monthGridDays`/`weekDays` — funções PURAS) + `tokens.css` — **nenhuma lib de
  calendário/grid nova**.
- **RNF-04 (sem N+1)** — o summary (CA-04/07) e o calendário (CA-10) são **um** request cada, com
  número FIXO de queries (3 no summary; 3 no calendário: deadline do projeto + tasks com due +
  objetivos com target_date [se a coluna existe]), montados em memória. Loop de query por projeto/
  task **proibido**.
- **RNF-05 (perf)** — `summary` e `calendar` < 200ms local (CA-07); índices: o `idx_entries_task`
  (fase 12) e os índices da `0009` cobrem o tempo; `tasks (project_id)` já existe (fase 02);
  `tasks (due_date)` é criado na `0025` pra o calendário/`due_today` (ver plan §2).
- **RNF-06 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`); cor do projeto é
  dado de domínio via `lib/palette.ts` (exceção sancionada da fase 02). Chips do calendário, KPIs e
  barras usam tokens/classes existentes (`.tt-card`, `.tt-goal`, `.tt-dot`, `--accent`,
  `--accent-soft`, `.calgrid`/`.calcell`/`.caldow` — ver §6).
- **RNF-07 (mobile)** — `MILESTONE-3 §6 RNF-06`: responsivo no breakpoint 768 da M2 (cards
  empilham, calendário rola/encolhe < md; grids 1-col, padrão da fase 22).
- **RNF-08 (Inter)** — a fonte Inter já foi trocada na fase 24 (`MILESTONE-3` decisão 11); esta
  fase **assume Inter** (mono só p/ números/tempo — `fmtH`, datas, % do calendário). NÃO troca
  fonte (a troca Geist→Inter é da fase 24).
- **Multi-tenant** — CA-09 é inegociável (SDD §8.4); recurso inexistente e invisível respondem o
  mesmo `404`.
- **PT-BR na UI**; identificadores de código em inglês (nomes de teste em PT-BR = exceção
  registrada). **Atenção:** o mockup usa rótulos EN (`Overview/Tasks/Upcoming/Done`) — manter a
  paridade visual do mockup é mais forte que a regra PT-BR pros rótulos das abas/KPIs (decisão
  registrada no plan §6; o resto da UI segue PT-BR).

## 6. Fora de escopo (fronteiras explícitas)

- **Write-back no Google Calendar** (entries/itens do projeto → GCal) → fora (decisão herdada,
  `MILESTONE-3 §8`). O overlay é **read-only** (CA-13/14).
- **Associação evento GCal ↔ projeto** → fora (limitação herdada da fase 06 — o `/calendar/events`
  é o `primary` do usuário, sem `project_id`). TODO projeto vê os mesmos eventos no overlay.
  Calendários secundários do Google também ficam fora (a fase 06 só lê `primary`).
- **Grade de horas (Dia/Semana, time-grid)** do projeto → fora desta fase. O calendário do projeto
  é só **vista de mês** (mockup `renderCal` = mês). O `/calendario` global (fase 11) segue dono das
  views Dia/Semana/Mês com horários.
- **Objetivos com modos (`completion_mode`/`target_date` write)** → **fase 29**. Esta fase só
  **consome** `target_date` no calendário (aditivo, omitido se ausente — CA-15.1) e `done_pct` do
  objetivo no Overview/Progress; não cria a coluna nem a UI de modos.
- **Roadmap Now/Next/Later** entre projetos / `projects.horizon` → fora (`MILESTONE-3 §8`).
- **Reagendar por drag** no calendário do projeto → fora (RF-04 drag é só do Board de tasks, fase
  24). Mudar prazo = editar `due_date`/`deadline` pelas rotas existentes.
- **Notas (tabela `notes`)** — esta fase **não** cria a tabela nem a UI de notas (é a fase 26); só
  **consome** as notas do projeto na seção "Recent notes" do Overview (aditivo, omitido se a fase
  26 não estiver no build — CA-21).
- **Tasks 2.0 / propriedades tipadas / Spotlight** → fases 24/25/27. Esta fase usa o tab-shell e o
  `openTask` que a fase 24 já entrega; não os reimplementa.
- **Notificação de prazo** (push quando o `deadline` do projeto chega) → fora (a fase 20 cobre
  lembretes de **task** `due`; deadline de **projeto** não dispara lembrete nesta fase — candidato
  a futuro).
