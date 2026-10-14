# tasks.md — Fase 14 · Relatórios 2.0

> TDD: red → green → refactor. **Commit por tarefa** (`TDD.md §1`). Cada T-XX lista os CAs
> cobertos. Backend primeiro (generalização preserva a 07), depois objetivos/availability/foco-
> hora/kpis/compare/settings, depois front, E2E, verificação adversarial e gate.
> **Pré-condição:** fase 12 (objetivos) construída — `objectives` + `tasks.objective_id`.
> **Blocos (regra ≤10 tarefas / contexto):** backend (T-01→T-07) · front (T-08→T-11) ·
> E2E+verificação+gate (T-12→T-14). Gate `make check-full` só no fim (T-14).

---

## Bloco backend (generalização + agregações)

- **T-01 — `periodWindow` + filtros (generaliza a 07, sem regredir)** (CA-04/05/06/07/09/10/11/12)
  - `internal/reports/service.go`: `periodWindow(period, date, from, to, loc) → (start, end)`
    (D-02, reusa `weekWindow`); struct `Filters{ProjectID, ClientID, ObjectiveID}` + `whereFilters`
    parametrizado (D-03, project direto / client via projetos do cliente / objective via tasks do
    objetivo, com `visibleWhere`). `weekWindow`/`byProject`/`focusMetrics`/`goals` **PRESERVADOS**.
  - **RED:** `period_test.go`: (a) CA-05 janela `[from,to+1d)` (from=to = 1 dia); (b) CA-06 presets
    day/week/month no fuso (`from/to` precede `period`); (c) CA-07 malformados e `to<from` → 400;
    (d) CA-10/11 filtros project/client/objective combináveis; id válido invisível → vazio (não
    404); malformado → 400; (e) CA-12 objetivo de privado alheio não filtra nada; (f) CA-09 a
    semana sem filtro = mesma `list` da 07.
  - **Acceptance:** `go test ./internal/reports` verde; **service_test.go/planned_test.go da 07/06
    INTOCADOS e verdes** (prova RNF-05).

- **T-02 — Seção `objectives` (progresso por ENTREGA)** (CA-16/17/18/19)
  - `internal/reports/objectives.go` (D-05): 3 queries — objetivos visíveis com atividade-no-
    período OU meta; `tasks_total`/`tasks_done` CUMULATIVO por objetivo; `realized_seconds` do
    período via `task_id → objective_id`. Composição: `progress_pct = round(done/total×100)`, 0 se
    total 0; `goal_seconds` só com `goal_hours`.
  - **RED:** casos: CA-16 shape (project_*, objective_*, status, contagens, progress, realized,
    goal); CA-17 contagem cumulativa × tempo do período (task `done` fora do período + entry
    dentro — pina os dois grãos); CA-18 `status='done'` com <100% marcado; 100% com `status='open'`
    reportado como open; CA-19 tasks sem objetivo NÃO viram item (mas tempo segue em by_project);
    isolamento user (realized só do usuário) + visibilidade (objetivo de privado alheio fora).
  - **Acceptance:** casos verdes; assume schema da fase 12.

- **T-03 — `availability` (rastreado × não-rastreado)** (CA-20/21)
  - `availability` no Report (D-06): `expected = work_hours_per_day×3600 × diasDoPeríodo`
    (todos ou só seg–sex se `workdays_only`); tracked = total; untracked = max(exp−tracked,0);
    tracked_pct round capado 100, 0 se exp=0; `expected_basis` string. Config recebida (default 8h/
    workdays_only=true até a T-05 ligar o settings).
  - **RED:** casos: semana útil 5 dias × 8h → expected 144000; `workdays_only=false` conta 7 dias;
    custom 1 dia sábado + workdays_only → expected 0 → tracked_pct 0 (sem div/0); untracked clampa
    em 0 quando tracked>expected.
  - **Acceptance:** casos verdes.

- **T-04 — `focus_by_hour` + `kpis`** (CA-22/23/24)
  - Deriva da `list` em Go (D-08): 24 buckets de `focus_seconds` por hora local do `started_at`
    (só entries ≥1500s); `kpis` (tracked, deep_focus_pct, avg_session_seconds, untracked) — div/0 → 0.
  - **RED:** casos: CA-22 entry de foco às 16h local soma no bucket 16; fragmento (<1500s) NÃO
    soma; CA-23 sempre 24 buckets (zeros); CA-24 deep_focus_pct/avg_session com 0 entries → 0
    (sem NaN); bucket por fuso (entry UTC que vira hora diferente no loc).
  - **Acceptance:** casos verdes; zero query nova (deriva da query 1).

- **T-05 — `report.Settings` + Get/Save + comparação `previous`** (CA-25/26/29-backend)
  - `internal/reports/settings.go` (espelha `calendar.settings` da 11): `Settings{work_hours_per_day,
    workdays_only}`, `GetSettings` (defaults sem linha), `SaveSettings` (valida `0<hpd≤24` ANTES do
    SQL → `domain.Validation`; upsert). `report_settings` usa a migração da T-07. `Report` ganha o
    bloco `previous` (D-04): 2ª passada com a janela deslocada (week −7d; month addMonths −1 com
    clamp; custom −duração), mesmos filtros, devolvendo total+kpis; sem `compare` → nil.
  - **RED:** `settings_test.go`: defaults; upsert; validação não persiste; isolamento por user
    (mutação removendo `WHERE user_id` morre). `previous`: week→semana anterior; month→mês anterior
    (round-trip do clamp); custom→janela de mesmo tamanho; sem compare → previous nil.
  - **Acceptance:** casos verdes.

- **T-06 — Migração `0014_report_settings`** (CA-21/29 backstop, RNF-04)
  - `backend/internal/db/migrations/0014_report_settings.sql` (plan §2). Teste: tabela existe,
    defaults 8/true, CHECK barra `work_hours_per_day` fora de `(0,24]`.
  - INDEX.md: registrar 0014 dona da fase 14 (após 0013 da fase 12).

- **T-07 — Rota genérica `/reports` + alias `/reports/week` + settings + perf** (CA-02/04/07/08/27, RNF-03)
  - `api/reports_routes.go`: handler genérico `GET /reports` (period|from&to&tz&filtros&compare),
    montado no subrouter do workspace; `/reports/week` **delega** pro mesmo service
    (`period=week`) sem mudar o shape (D-01); `/reports/planned` **INTOCADO**. `api/
    settings_reports.go` (GET|PUT `/settings/reports`, rota por-usuário); wiring no `cmd/server`.
  - **RED:** `reports_handlers_test.go` (harness `coretest_test.go`): 401/404/viewer-200 no
    `/reports`; 400 para period/from/to/tz/filtros malformados (CA-07); shape do §3.1 (listas `[]`,
    24 buckets, previous só com compare); **`/reports/week` shape idêntico ao da 07 (CA-04/09)**;
    settings GET defaults/PUT válido/PUT inválido 400; RNF-03 perf `period=month&compare=previous`
    com 1.000+ entries (3 amostras <200ms — exceção red→green documentada no cabeçalho).
  - **Acceptance:** `make test-back` 100% verde; suítes 06/07 sem edição.

## Bloco frontend

- **T-08 — Front base: api + query + período + filtros + toggle** (CA-08/27/28)
  - `lib/api.ts` (tipos §3.4 + `getReport`, `get/putReportSettings`; mantém `getWeekReport`);
    `lib/queries/reports.ts` (`useReport` — key com period/from/to/filtros/compare/tz;
    `useReportSettings`/`useSaveReportSettings`); `Reports.tsx` reescrita com `PeriodNav`
    (Dia/Semana/Mês/Custom, ‹ Hoje ›, `addMonths` importado da 11) + `ReportFilters` (selects
    projeto/cliente/objetivo + toggle compare) + `ChartsTableToggle`.
  - **RED:** `Reports.spec.tsx` (mock de `../../lib/api` + QueryClientProvider): CA-27 troca de
    preset refaz a busca e o rótulo vem do `from/to` da resposta; navegação por passo coerente;
    CA-28 filtro refaz a busca; toggle alterna; chamada sempre com `tz: browserTZ()`.
  - **Acceptance:** Vitest verde; `check-cores.sh` passa.

- **T-09 — KPIs + donut + foco por hora** (CA-20/22/24/30)
  - `KpiStrip` (faixa + delta vs. `previous` quando compare ligado — **fórmula do CA-30:
    `round((atual−anterior)/anterior×100)` com sinal se anterior>0; "novo" se anterior=0 e atual>0;
    "0%" se ambos 0 — NUNCA NaN/Infinity**), `TrackedDonut` (conic-gradient, % + tracked/untracked),
    `FocusByHour` (24 barras de intensidade). Tokens só.
  - **RED:** casos: KPIs renderizam valores; **delta com sinal (anterior>0), "novo" (anterior=0,
    atual>0), "0%" (ambos 0)** — sem NaN; delta só com compare; donut com `tracked_pct`; 24 barras;
    sem hex literal (check-cores).
  - **Acceptance:** Vitest verde.

- **T-10 — Objetivos + tabela + ângulos 1–3/4 reusados + vazio** (CA-13/14/15/16..19/31/32/33)
  - `ObjectivesSection` (progresso por entrega: barra de % de tasks + realizado/meta; status done
    marcado); `ReportTable` (tabela densa on-demand — `tt-table`; **colunas SÓ do contrato por
    projeto: projeto, cliente, rastreado, meta, objetivos concluídos/total — NÃO fabricar
    não-rastreado/foco%/sessões por projeto, CA-31**); reuso de `ProjectBars`/`FocusStats`/
    `GoalsSection` (07) e `PlannedVsReal` (06) — este **só no preset Semana** (D-07); estados
    vazios (CA-32 — sem NaN).
  - **RED:** casos: objetivo com 1/2 done = 50% e realizado; status done marcado; tasks sem
    objetivo não listadas; tabela mostra linha por projeto (rastreado + objetivos concluídos/total,
    SEM colunas fabricadas); período sem dados → vazios sem NaN; Planejado×real presente só em
    Semana (oculto em Dia/Mês/Custom — D-07, CA-33).
  - **Acceptance:** Vitest verde; aposentar `WeekNav` se nada mais usar.

- **T-11 — `ReportGroup` nos Ajustes** (CA-29)
  - Grupo "Relatórios" em `Settings.tsx`: `work_hours_per_day` (1–24) + `workdays_only` (toggle),
    salva via `useSaveReportSettings`.
  - **RED:** spec: form inválido (hpd ≤0 ou >24) não chama API e sinaliza erro; válido chama PUT
    com o corpo certo.
  - **Acceptance:** Vitest verde.

## Bloco E2E + verificação + gate

- **T-12 — E2E `relatorios-v2.spec.ts`** (CA-34)
  - Arquivo NOVO **paralelo** (não muta o timer): login stub → cliente+projeto único com 1
    objetivo (2 tasks, 1 `done`) e meta, semeado via API/INSERT → 2 entries de hoje via
    `POST /entries` (uma ≥25min ligada à task, uma <25min) → `/relatorios` preset Semana mostra o
    projeto no ângulo 1; filtra por aquele projeto (números escopados); seção objetivos mostra
    1/2 (50%) + realizado; troca pro preset Mês (período/rótulo mudam); **liga comparar → o KPI
    mostra "novo"** (período anterior do projeto novo é vazio, `anterior=0` — CA-30/34, sem delta
    numérico/NaN); abre a Tabela (linha do projeto: rastreado + objetivos concluídos/total). Escopo
    por projeto único = determinístico (banco reusado).
  - **Acceptance:** Playwright verde (com `make up`/dev servers na :8080 derrubados antes).

- **T-13 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: janela/período/fuso, filtros/visibilidade, objetivos-por-
    entrega vs. tempo, availability/div-zero, comparação/addMonths, **não-regressão 06/07**,
    front/tokens) + refutação dupla. Corrigir confirmados. Registrar.

- **T-14 — Gate + INDEX + handoff (SEM deploy — único no fim da M1)**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). `specs/INDEX.md`
    marcando a 14 construída (convenções novas: rota `/reports` genérica + alias `/reports/week`,
    `0014_report_settings`, seção objetivos por entrega, availability). **SEM deploy** (decisão do
    Diogo 20/06 — o deploy é ÚNICO no fim das 4 fases da M1, depois da 16). Handoff (CLAUDE.md,
    INDEX, memória), commit, segue DIRETO pra fase 15.

---

## Notas de não-regressão (RNF-05 — checar a cada commit)

- `internal/reports/service_test.go` e `planned_test.go` **não mudam de asserção** — se a
  generalização exigir tocar a 07, é sinal de regressão; pare e reavalie.
- `/reports/week` e `/reports/planned` devolvem o MESMO shape de antes (CA-04/09 pinam).
- O ângulo 4 (`PlannedVsReal` + `usePlannedReport`) não muda de contrato; só fica condicional ao
  preset Semana no front (D-07).

---

## Registro — furos confirmados na verificação adversarial da SPEC (PASSO 0, sessão 18, 20/06)

Workflow ultracode `verify-m1-specs` (176 agentes, 6 dim × refutação dupla, ancorado no código
real). Fase 14: **5 confirmados 2/2, aplicados ANTES de codar:**

1. **(internal-consistency, plan D-06)** "Dias do período = `(end−start)/24h`" era DST-bugado e não
   sabia classificar dia útil. → **D-06 + §6:** iterar `start..end` com `AddDate(+1d)`, contando
   seg–sex se `workdays_only` (aritmética de calendário, igual `weekWindow`).
2. **(internal-consistency, CA-16)** `progress_pct = tasks_done/tasks_total` omitia o ×100 (daria
   0.x). → **CA-16 + D-05:** `round(tasks_done/tasks_total × 100)`, inteiro 0–100 (espelha o
   `donePct` da fase 12).
3. **(completeness-gaps, CA-16.1 NOVO)** sob filtro `project_id`/`objective_id`, objetivos de OUTRO
   projeto com meta vazavam (ramo "OU têm meta" ignorava o filtro). → **CA-16.1 + D-05:** a q-obj
   aplica os mesmos `whereFilters`; o filtro restringe a LISTA.
4. **(testability, CA-30/CA-34)** delta dos KPIs sem fórmula → div/0 com período anterior vazio. →
   **CA-30:** `round((atual−anterior)/anterior×100)` se anterior>0; "novo" se anterior=0 e atual>0;
   "0%" se ambos 0. **CA-34:** o E2E (projeto novo) afirma "novo", não um delta numérico.
5. **(design-fidelity, CA-31)** a tabela pedia colunas por-projeto (não-rastreado/foco%/sessões)
   que o contrato NÃO tem por projeto (o mockup FABRICAVA). → **CA-31:** linha/projeto só com
   colunas do contrato (projeto, cliente, rastreado, meta, objetivos concluídos/total); KPIs
   agregados ficam na faixa de KPIs, não na tabela.

## Registro — build (executor) + verificação adversarial do BUILD (T-13/T-14, sessão 18, 20/06)

**Build delegado a 1 agente-executor** (Agent tool, contexto isolado): T-01→T-12, 13 commits TDD,
non-regressão 06/07 preservada (zero-diff em `service_test.go`/`planned_test.go`/`reports_handlers_
test.go` e no pacote `calendar/`). **Decisão sancionada:** `/reports/week` NÃO delega ao service
novo (mantém `svc.Week` intocado — mais seguro p/ não-regressão; CA-09 prova a equivalência via
`TestWeekAliasNaoRegride`).

**Achados pós-build, corrigidos no main loop:**
- **`zz_dstprobe_test.go` (untracked) loopava infinito** no gap de DST midnight de 2017 em
  America/Sao_Paulo (`time.Date(...Day()+1...)` no gap → loop) → o `go test ./...` isolado do
  executor cacheou e não pegou; o **gate completo** pegou (timeout 600s). **Removido** (scratch
  sem assertion, sem valor de spec; cobertura real de DST/`periodDays` está em `availability_test`/
  `kpis_test`).
- **2 regressões de E2E front** das mudanças sancionadas da fase (o executor só rodou o
  `relatorios-v2.spec` isolado): `relatorios.spec` (07) `week-label`→`period-label` (WeekNav
  aposentado pelo PeriodNav — D-10); `tema.spec` `getByRole('switch')` escopado por aria-label
  (ReportGroup adicionou um switch no /ajustes — espelha o escopo do `cronometro.spec` na fase 11).
- **Verificação adversarial do BUILD** (workflow ultracode, 15 agentes, 7 dim × refutação dupla):
  **2 confirmados (mesmo issue), 2 refutados.** Confirmado: **CA-16.1 estava implementado correto
  mas SEM teste** — mutação neutralizando os predicados de filtro da q-obj deixava a suíte verde →
  `TestObjectivesFiltroRestringeLista` (3 ramos de filtro; red→green por mutação). **Refutados:**
  `periodDays`/`daysBetween` over-count por 1 em DST-midnight (LOW, nil impact — BR sem DST desde
  2019, nenhum CA cobre; e o "fix" sugerido — `time.Date(...Day()+1...)` — é o loop infinito do
  scratch test, por isso o `AddDate` (termina, só erra no gap midnight de 2017) **fica**).

`make check-full` **GATE VERDE** — Go 100% · Vitest 259/259 · Playwright 25/25. **NÃO deployada**
(deploy único no fim da M1).
