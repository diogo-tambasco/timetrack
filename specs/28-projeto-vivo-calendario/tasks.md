# tasks.md — Fase 28 · Projeto vivo + Calendário por projeto

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-15).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → deadline em projects → summary →
> calendar → handlers/wiring), depois frontend (api/hooks → ProjectCard vivo + Projects →
> OverviewTab → ProjectCalendarTab → shell), E2E e gate. **Estende** as fases 02/12/06/11/14/24/26
> — não as reescreve (RNF-01/RNF-05 das fases anteriores).

---

## Bloco 1 — backend (T-01 → T-06)

- **T-01 — Migração `0025_projects_deadline`** (CA-01/02 backstop, RNF-05)
  - `backend/internal/db/migrations/0025_projects_deadline.sql` (plan §2): `ALTER TABLE projects
    ADD COLUMN deadline date` (NULL default) + `CREATE INDEX idx_tasks_due ON tasks (project_id,
    due_date) WHERE due_date IS NOT NULL`.
  - Número FIXO `0025` (PRD §7 / plan §2) — NÃO "ler o próximo livre". Conferir o diretório só p/
    sanity.
  - Teste (`migrations_*_test.go` ou novo, dbtest): coluna `projects.deadline` existe e aceita
    `NULL` + uma data; `idx_tasks_due` existe; projeto criado sem `deadline` fica `NULL` (CA-02);
    `GET`/leitura de projeto traz `deadline:null` default.
  - INDEX.md: registrar 0025 dona da fase 28 (orquestrador).

- **T-02 — `deadline` no CRUD de projetos (estende fase 02)** (CA-01/01.1/02/03)
  - `internal/projects/service.go`: `Project` struct + `Deadline *string`; `CreateInput.Deadline`;
    `PatchInput.DeadlineSet bool`/`Deadline *string` (tri-state — D-01); `validateDeadline`
    (`time.Parse("2006-01-02")` ANTES do SQL — CA-01.1); incluir `deadline` no INSERT/UPDATE e nas
    4 leituras (`List`, `Get`, e o `project` do `board.go` da fase 12). **NÃO** mexer no gate
    (`getForWrite` + `canEditMeta` byte-idênticos — os símbolos REAIS de `internal/projects/
    service.go`; **não** há `loadProjectGate` no pacote `projects` — CA-03/RNF-05).
  - Teste `service_test.go` (estende, dbtest): POST com `deadline` válido persiste e volta no Get;
    `deadline` malformado (`"2025-13-40"`, `"ontem"`) → 400 `validation_error`, NÃO persiste
    (CA-01.1); PATCH tri-state (`{"deadline":null}` remove; ausente mantém — CA-01); projeto sem
    deadline = `null` (CA-02); board (fase 12) agora traz `project.deadline` (campo aditivo, resto
    intocado — RNF-05); escrita por viewer/`access=view` segue 403/404 (CA-03, gate da fase 02 não
    mudou — pina que o caminho de erro é o mesmo). Suíte da fase 02/12 **continua verde** (só o
    campo novo).

- **T-03 — `Summary`: cards vivos (3 queries fixas)** (CA-04/04.1/05/06/08, RNF-04)
  - `internal/projects/summary.go`: `ProjectSummary` + `Summary(ctx, ws, userID, tz, archived)` —
    3 queries fixas (D-02): (1) projetos visíveis do ws (`visibility='shared' OR owner_user_id=$u`,
    `archived` flag — CA-04.1/08); (2) contadores `GROUP BY project_id` (`count`, `done`,
    `due_today` com `due_date=$today AND status<>'done'`); (3) soma de tempo `GROUP BY project_id`
    das entries fechadas do user. Montagem em memória O(n). `today` = `time.Now().In(loc)` no `tz`
    (UTC default; `time.LoadLocation` valida → 400 CA-06).
  - Teste `summary_test.go` (dbtest): contadores corretos (task_count/done_count/open_count) por
    projeto; `tracked_seconds` = bucket projeto do user (D-03), **NÃO soma entries de outro user**
    (mutação removendo `user_id` do SUM deve QUEBRAR — CA-09); `due_today_count` no `tz`
    (task vence 25/06: "hoje" em SP × UTC, com clock fake que diverge — CA-06); `tz` inválido →
    400; projeto 0 tasks → contadores 0 (CA-05); projeto `private` de OUTRO user NÃO aparece
    (mutação removendo a visibilidade deve QUEBRAR — CA-04.1); arquivado fora sem `?archived`,
    dentro com (CA-08); isolamento por workspace (mutação removendo `WHERE workspace_id` morre —
    CA-09).
  - **CA-07 (perf — exceção red→green, igual CA-18 da fase 12):** `backend/testdata/projects/
    summary_seed.sql` (30 projetos / 300 tasks / 1.000 entries, placeholders `{{WORKSPACE_ID}}`/
    `{{USER_ID}}`, padrão `seed500.sql`/`board_seed.sql`) + `TestSummarySob200ms` (melhor de 3 <
    200ms; nasce verde, pina o N+1/RNF-04).

- **T-04 — `ProjectCalendar`: itens datados (detecção de `target_date`)** (CA-09/10/11/12/13/14/15.1)
  - `internal/projects/calendar.go`: `CalendarItem` + `ProjectCalendar(ctx, ws, userID, projectID,
    from, to)` — gate de visibilidade via **`s.Get(ctx, ws, userID, projectID)`** (existe em
    service.go:219: faz `uuid.Validate(projectID)` → 404 nunca 500, e aplica `visibleWhere`
    `(visibility='shared' OR owner_user_id=$2)` → 404 não-vaza — CA-09/12). ⚠️ **NÃO** chamar
    `loadProjectGate` (esse símbolo NÃO existe no pacote `projects` — só em `objectives`/`tasks`).
    Alternativa sancionada: replicar a regra inline na própria query do calendário (auto-contida,
    igual `objectives/gate.go`). 3 queries (D-05): (a) `projects.deadline` do projeto ∈ [from,to] →
    `kind:"project_deadline"`; (b)
    tasks `WHERE project_id=$ AND due_date BETWEEN $from AND $to` → `kind:"task"` (inclui `done` —
    CA-10); (c) objetivos `WHERE project_id=$ AND target_date BETWEEN ...` → `kind:"objective"`
    **SÓ se `s.hasObjectiveTargetDate`** (CA-15.1). `resolveWindow(month, from, to, tz)` (`?month`
    → 1º..último dia DST-safe via `AddDate`; `to<from`/malformado → 400 — CA-11). `s.hasObjectiveTargetDate`
    detectado 1× no `NewService` via `information_schema.columns`.
  - Teste `calendar_test.go` (dbtest): itens corretos por dia (task due, deadline do projeto);
    `?month` resolve `from`/`to` (último dia de mês de 30/31/fev — DST-safe); `?from&to` explícito;
    `to<from`/`month` malformado → 400 (CA-11); projeto invisível/`projectId` malformado → 404 (não
    500 — CA-12); **NÃO vaza item de outro projeto/workspace** (mutação removendo `project_id`/
    visibilidade morre — CA-09); task `done` ainda aparece (CA-10); **`kind:"objective"` aparece SE
    a coluna existe, some se não** (CA-15.1 — testar os dois ramos: criar a coluna num schema de
    teste auxiliar OU stubar `hasObjectiveTargetDate`); o endpoint NÃO retorna eventos do GCal
    (CA-13 — sem campo de evento na resposta).

- **T-05 — Handlers de summary + calendar + `deadline` nos handlers de project** (CA-01..15)
  - `internal/api/projects_handlers.go`: handlers finos pra `GET .../projects/summary` (lê `?tz`/
    `?archived`) e `GET .../projects/{projectId}/calendar` (lê `?month`/`?from`/`?to`/`?tz`);
    `deadline` adicionado aos handlers de POST/PATCH de project (tri-state via `json.RawMessage` no
    patch, padrão da fase 02). Erros canônicos (400/403/404/401).
  - Teste `projects_handlers_test.go` (harness `coretest`): summary 200 com a lista + campos; 400
    `tz` inválido; 403/404 herdados; calendar 200 com `{items,from,to}`; 400 `month` malformado;
    404 projeto invisível/malformado (CA-12); **401 sem sessão** table-driven nas rotas novas
    (CA-15); deadline no corpo de POST/PATCH/GET de project.

- **T-06 — Wiring no `core_routes.go`** (CA-15, RNF-04)
  - `mountCore`: rotas `GET .../projects/summary` e `GET .../projects/{projectId}/calendar` no
    subrouter do workspace (cadeia RequireAuth→RequireMember já aplicada; ambas são GET → passam pro
    viewer também). `cmd/server` liga o `Summary`/`ProjectCalendar` (mesmo `projects.Service` — sem
    construtor novo). `hasObjectiveTargetDate` resolvido no boot do service.
  - Teste: rotas vivas (smoke de roteamento no `router_test`/`coretest`); GET de summary/calendar
    por membro 200; sem sessão 401.

> **Fim do bloco 1:** `make test-back` 100% verde (projects estendido + objectives/tasks/calendar
> intactos); Vitest/Playwright não rodados (bloco sem frontend). Handoff "retomar na T-07".

## Bloco 2 — frontend (T-07 → T-12)

- **T-07 — `lib/api.ts` + `lib/queries/projects.ts`** (CA-01/04/10/16)
  - `Project`/`ProjectInput` ganham `deadline:string|null`; tipos `ProjectSummary[]`,
    `ProjectCalendarItem`/`ProjectCalendar` (`{items,from,to}`); `getProjectsSummary(ws,tz,archived)`
    e `getProjectCalendar(ws,projectId,{month|from,to,tz})`. Hooks `useProjectsSummary`,
    `useProjectCalendar`; o patch de project invalida `projectsSummaryKey` + `boardKey`.
  - Teste `projects-api.test.ts`: chama os paths certos (`/projects/summary?tz=`,
    `/projects/{id}/calendar?month=`); hook de patch invalida a query de summary.

- **T-08 — `ProjectCard` vivo + `Projects.tsx`** (CA-05/16, RNF-07)
  - `components/projects/ProjectCard.tsx` reescrito (caminho REAL — **não** `components/cards/`, que
    não existe; o import vivo está em `routes/cockpit/Projects.tsx:8`; o spec do componente já é
    `components/projects/ProjectCard.spec.tsx`): nome + barra de cor; `open_count` pendentes ·
    `task_count` total; cliente; **`fmtH(tracked_seconds)`** (NÃO `0h` hardcoded); `deadline`
    formatado quando há; dot "tem algo hoje" quando `due_today_count>0`; link pro detalhe.
    `Projects.tsx` consome `useProjectsSummary` (1 request — CA-04/16); `done_pct` derivado no
    client (0 quando `task_count=0` — CA-05).
  - Specs (jsdom): card mostra horas reais (`142h`, não `0h`); `X pendentes · Y no total`; dot
    "hoje" com `due_today_count>0` e ausente com 0; `done_pct` 0 sem dividir por zero; sem deadline
    → sem chip de prazo; clicar navega pro detalhe; < md empilha (responsivo, padrão fase 22).

- **T-09 — `OverviewTab`** (CA-17/20/21)
  - `components/project/OverviewTab.tsx`: KPIs (Total time/This week/Open/Done — do `useBoard` +
    relatório da fase 14 p/ "This week"); Upcoming (tasks due futuras/hoje, `status!=done`, asc,
    clicáveis → `openTask` da fase 24 — RF-23; vazio "No deadlines set." — RNF-02); Progress por
    status + Goals (do board, reusa `done_pct`); Recent notes (fase 26, **aditivo** — omite se
    ausente — CA-21). Gating por `board.project.can_write` (CA-20).
  - Specs (jsdom, board mockado): KPIs com os números do board; Upcoming lista as tasks de prazo
    clicáveis (chama `openTask`); vazio mostra a mensagem à prova de culpa; Progress por status;
    Recent notes presente com a fase 26 mockada / **omitida** quando o hook de notas devolve
    indisponível (CA-21); ações de escrita escondidas com `can_write=false` (CA-20, padrão
    `viewer-gating.spec`).

- **T-10 — `ProjectCalendarTab`** (CA-13/18/21)
  - `components/project/ProjectCalendarTab.tsx`: grade do mês via `monthGridDays` (fase 11);
    chips dos **itens do projeto** (`useProjectCalendar`, buscados com `?month=`) na **cor do
    projeto** (`border-left`); overlay GCal via `useCalendarEvents(from,to,tz)` (fase 06) com a
    janela da **GRADE de 42 dias** = `monthRange(refDate)` da fase 11 (`lib/calendar-layout.ts:267`),
    **NÃO** o `from`/`to` estrito do endpoint do projeto (D-07 — senão os eventos do GCal nos dias de
    *spillover* da grade somem); ‹ Hoje › navega mês; clicar item task/objetivo → abre (RF-23),
    evento → `html_link` (fase 11). Degrada sem conexão GCal (CA-13). `kind:"objective"` só aparece
    quando o backend o devolve (CA-15.1/21).
  - Specs (jsdom): item de task/deadline no dia certo na cor do projeto; clicar task chama
    `openTask`; evento do GCal sobreposto (mock do `useCalendarEvents`); **o overlay GCal é buscado
    com a janela da GRADE (`monthRange`) e NÃO o mês estrito — asserir que `useCalendarEvents` recebe
    o `from`/`to` da grade de 42 dias (cobre os dias de spillover, não só os do mês — D-07/CA-18)**;
    navegar mês muda a janela (chama `getProjectCalendar` com o novo `month` E `useCalendarEvents`
    com o novo `monthRange`); sem conexão GCal → só itens do projeto (sem overlay, sem erro — CA-13);
    `kind:"objective"` ausente não quebra (CA-21).

- **T-11 — Tab-shell: Overview + Calendar no `ProjectDetail`** (CA-19)
  - `routes/cockpit/ProjectDetail.tsx` (estende 12/24): abas **Overview · Tasks · Notes · Goals ·
    Calendar** (mockup 310); default = **Overview**; resetar a aba ao trocar de projeto (não
    persistir — CA-19). Pendurar `OverviewTab`/`ProjectCalendarTab` nos painéis novos; reusar o
    `useBoard` já carregado.
  - Specs (jsdom): as 5 abas renderizam; default Overview ao abrir; trocar pra Calendar mostra a
    grade; trocar de projeto volta pra Overview.

- **T-12 — Gating viewer (componente)** (CA-20)
  - `viewer-gating` (estende o padrão das fases 02/12): com `board.project.can_write=false`, o
    Overview/Calendar não exibem editar-deadline / criar-task pelo Upcoming; com `can_write=true`,
    exibem. (O E2E não loga como viewer — limitação herdada; este teste de componente é a cobertura
    do CA-20 — CA-22.)
  - Spec (jsdom): controle member/viewer num arquivo (padrão `viewer-gating.spec.tsx`).

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright não
> rodado ainda (E2E é o bloco 3). Handoff "retomar na T-13".

## Bloco 3 — E2E + verificação + gate (T-13 → T-15)

- **T-13 — E2E `projeto-vivo.spec.ts`** (CA-22)
  - Logado+escrita (stub fase 01) num projeto `shared`: define `deadline` no projeto; vê os **cards
    vivos** na lista de Projetos (horas reais ≠ `0h`, `X pendentes`, dot "hoje" se houver); abre o
    projeto → aba **Overview** (KPIs, Upcoming com ≥1 task de prazo, Progress); abre **Calendar** →
    vê o item de **prazo do projeto** e o **due da task** no dia certo + **≥1 evento do GCal**
    sobreposto (via `calendarstub` da fase 06); **clica no item da task** → abre a task; navega de
    mês → grade muda. **SEM** segundo perfil viewer no E2E (stub autentica só o owner — CA-22): o
    gating de viewer é coberto pela T-12 (jsdom). Reusar `loginViaStub`; nomes únicos (banco
    reusado, idempotente); resetar a conexão de calendar via helper da fase 06 se necessário.

- **T-14 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: isolamento multi-tenant/visibilidade no summary E calendar,
    `due_today` por fuso / clock, agregação de tempo bucket-projeto vs board, `target_date` aditivo
    [coluna ausente×presente], janela `?month` DST/último-dia, overlay GCal sem vínculo a projeto,
    regressão fase 02/12 — `deadline` aditivo no board) + refutação dupla. Commitar/limpar a tree
    ANTES de rodar (os agentes de mutação rodam na mesma working tree — lição da M2). Corrigir
    confirmados red→green. Registrar na seção abaixo.

- **T-15 — Gate + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Liberar a :8080**
    de dev server alheio antes do E2E/gate (lição registrada). **SEM deploy** (o deploy da M3 é
    único, no fim da milestone — não deployar por fase, padrão M1/M2).
  - Handoff (CLAUDE.md "retomar..."), INDEX (28 construída, 0025 dona), KICKOFF da fase 29, commit,
    briefing pro Diogo. Registrar a dependência fraca: `kind:"objective"` do calendário nasce
    **omitido** e liga quando a fase 29 deployar (CA-15.1).

---

## Blocos (regra de organização — o build é UMA PAULADA, CLAUDE.md)

- **Bloco 1 — backend:** T-01 → T-06 (migração, deadline em projects, summary, calendar, handlers,
  wiring). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-07 → T-12 (api/hooks, ProjectCard vivo + Projects, OverviewTab,
  ProjectCalendarTab, shell, gating). Suíte: Vitest + `make test-back` (regressão).
- **Bloco 3 — E2E + fechamento:** T-13 → T-15 (E2E, verificação adversarial, gate `make
  check-full` + handoff). **Deploy é único no fim da M3.**

`make check-full` é o gate final (T-15). Os blocos 1 e 2 fecham com a suíte do que existe verde +
handoff + commit.

---

## Furos da verificação adversarial (ETAPA 2 — aplicados)

Verificação adversarial das specs (ancorada no código real). 3 furos CONFIRMADOS 2/2 — todos
aplicados em spec.md/plan.md/tasks.md ANTES do build.

- **28-H1 (MEDIUM · CONTRATO/CORRETUDE) — `loadProjectGate` NÃO existe no pacote `projects`.**
  Problema: plan §5/§7 e tasks T-04 mandavam "reusar o `loadProjectGate` já interno do
  `projects.Service`" pra resolver visibilidade→404 no calendário; mas esse símbolo só existe em
  `internal/objectives/gate.go:37` e `internal/tasks/service.go` — o `projects.Service` tem
  `Get`/`getForWrite`/`canEditMeta`/`visibleWhere` (confirmado em `service.go:190/206/215/219`). Um
  builder ao pé da letra ou não compila ou inventa um gate divergente.
  Resolução: troquei TODAS as menções a `loadProjectGate` no contexto de `projects` pelos símbolos
  REAIS. Gate de visibilidade de LEITURA do calendário (CA-09/12) = **`s.Get(ctx,ws,userID,projectID)`**
  (faz `uuid.Validate`→404 nunca 500, aplica `visibleWhere`→404 não-vaza), com a alternativa
  sancionada de replicar a regra inline (igual `objectives/gate.go`). Gate de ESCRITA do `deadline`
  (CA-03/D-01) = **`getForWrite` + `canEditMeta`** (o caminho real de `Patch`/`Create`,
  service.go:280/284). Editado: spec.md CA-03; plan.md D-01, §5 (assinatura `ProjectCalendar` +
  topologia de import), §7 (armadilha de vazamento); tasks.md T-02 e T-04.

- **28-H4 (MEDIUM · CONSISTÊNCIA CROSS-FASE) — janela do overlay GCal usava o mês estrito, não a
  grade de 42 dias.** Problema: D-07/§6/CA-18 mandavam o `ProjectCalendarTab` buscar o overlay do
  GCal com os MESMOS `from`/`to` que o endpoint do projeto (`?month=`) devolve = 1º..último dia do
  MÊS. Mas a grade é `monthGridDays` = **42 dias** (`calendar-layout.ts:204`, seg→dom, com
  spillover), e o `/calendar/events` (fase 06, `calendar_routes.go:116`) trata `to` como dia último
  inclusivo `[from, to+1d)`. Resultado: eventos do GCal nos dias de spillover da grade somem
  (inconsistência visual, diverge da fase 11).
  Resolução: o overlay passa a usar **`monthRange(refDate)`** da fase 11 (`calendar-layout.ts:267` →
  primeira/última célula da grade de 42 dias) — fonte única DST-safe da janela do overlay —, enquanto
  os itens do projeto seguem no `?month=` (caem só nos dias do próprio mês). Editado: plan.md D-07,
  D-08, §1 (linhas de `lib/queries/calendar.ts` e `lib/calendar-layout.ts`), §6 (linha "Overlay GCal
  alinhado" + tabela Tab Calendar), §7 (nova armadilha); spec.md CA-18 (explicita que o overlay cobre
  os 42 dias da grade); tasks.md T-10 (descrição + novo teste jsdom que asserta `useCalendarEvents`
  recebendo a janela da GRADE, cobrindo spillover).

- **28-H7 (LOW · CONTRATO/CORRETUDE frontend path) — `ProjectCard` está em `components/projects/`,
  não `components/cards/`.** Problema: plan §1 e §6 mandavam editar `components/cards/ProjectCard.tsx`
  (existe 02); a pasta `components/cards/` não existe — o arquivo real é
  `frontend/src/components/projects/ProjectCard.tsx` (+ `ProjectCard.spec.tsx`), importado em
  `routes/cockpit/Projects.tsx:8`. Builder seguindo o plan criaria um arquivo órfão em `cards/`.
  Resolução: corrigi o path nos 3 pontos — plan.md §1 (bloco de arquitetura) e §6 (tabela "Card
  vivo"), e tasks.md T-08. (spec.md não cita path — não alterado, conforme o fix.)

### Decisões de coordenação cross-fase aplicadas

- **(G — erros canônicos)** O gate de visibilidade do calendário responde `404 not_found` pra
  projeto invisível/inexistente/`projectId` malformado (CA-09/12) — códigos reais do envelope
  (`s.Get` devolve `domain.NotFound`). Nenhum código inventado nesta fase.
- **(H — caminhos reais)** Path do `ProjectCard` ancorado em `components/projects/` (não
  `components/cards/` nem `src/stores/`), conforme as decisões de coordenação.
