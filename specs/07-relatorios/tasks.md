# tasks.md — Fase 07 · Relatórios (ângulos 1–3)

> Tarefas atômicas, TDD red→green→refactor, **commit por tarefa** (`TDD.md §1`).
> Backend = `go test` (dbtest + clock fake) · Front = Vitest · E2E = Playwright.

---

## T-01 — Janela da semana + ângulo 1 (internal/reports)

- **Objetivo:** `reports.Service` (clock injetável) com `weekWindow` (D-01) e a agregação
  `by_project` + `total_seconds` (queries 1+2 do D-02).
- **RED:** `service_test.go` com dbtest + `entries.Insert`: (a) CA-06 janela segunda→domingo
  em tz (date no meio da semana, date numa segunda, date num domingo); (a.1) **defaults do
  CA-06** (verificação adversarial das specs): clock fake em instante que DIVERGE entre UTC
  e o fuso (ex.: segunda 01:00 UTC = domingo 22:00 America/Sao_Paulo) → `date` vazio +
  `tz=America/Sao_Paulo` resolve "hoje" NO FUSO (semana que fecha no domingo, não a que
  abre em UTC); `date` e `tz` vazios → semana de segunda em UTC — mata a mutação
  `s.now().Format(...)` sem `.In(loc)`; (b) CA-07 borda de
  fuso (domingo 23:30 America/Sao_Paulo = 02:30 UTC de segunda → semana que FECHA; segunda
  00:30 BRT → semana seguinte); (c) CA-09 buckets ordenados DESC com name/color/client e
  soma ≡ total; (d) CA-10 bucket null; (e) CA-11 projeto arquivado nomeado; (f) CA-01
  isolamento (entries de outro usuário no MESMO ws fora; outro ws fora); (g) CA-04 entry
  rodando fora.
- **Acceptance:** `go test ./internal/reports` verde com todos os casos acima.

## T-02 — Ângulo 2: focus metrics

- **Objetivo:** `focus` no Report (D-03/D-04): focus/fragment seconds+counts,
  `longest_seconds`, `switches`.
- **RED:** casos: CA-12 limiar (1499s fragmento, 1500s foco, mistura com somas/contagens
  exatas); CA-13 longest (e 0 em semana vazia); CA-14 switches (mesmo dia: A→B→A = 2;
  A→null→A = 2; dias diferentes A | B = 0; mesma sequência de projeto = 0; ordem por
  started_at com desempate por id).
- **Acceptance:** casos verdes; zero query nova (deriva da query 1).

## T-03 — Ângulo 3: goals

- **Objetivo:** `goals` no Report (query 3 do D-02 + realized da query 1).
- **RED:** casos: CA-15 (projeto com meta e 0 realizado ENTRA; arquivado com meta FORA;
  private de OUTRO usuário com meta FORA — `visibleWhere`; private do PRÓPRIO usuário
  ENTRA; realized só da semana/do usuário); CA-16 meta fracionária 12.25 → 44100.
- **Acceptance:** casos verdes.

## T-04 — Rota REST + perf

- **Objetivo:** `GET /workspaces/{id}/reports/week` (D-05/D-06): option `WithReports`,
  mount no subrouter, handler com `date`/`tz`, wiring no `cmd/server`.
- **RED:** `reports_handlers_test.go` (harness `coretest_test.go`): 401 sem sessão; 404
  ws alheio (CA-02); **viewer → 200** (CA-03); 400 `validation_error` pra date/tz
  malformados (CA-08); shape do JSON do contrato §3.1 (listas vazias = `[]`); RNF-02
  perf com 1.000+ entries semeadas (3 amostras < 200ms; exceção red→green documentada
  no cabeçalho — precedente do `timer_perf_test.go`).
- **Acceptance:** `make test-back` 100% verde.

## T-05 — Front: api + query + página com navegação e ângulo 1

- **Objetivo:** `lib/api.ts` (+tipos, `getWeekReport`), `lib/queries/reports.ts`
  (`useWeekReport` — D-07), `Reports.tsx` real com WeekNav (‹ ›, rótulo da RESPOSTA) e a
  seção do ângulo 1 (ProjectBars — D-08).
- **RED:** `Reports.spec.tsx` (padrão das páginas da 02: mock de `../../lib/api` +
  QueryClientProvider): CA-17 navegação (clicar ‹ refaz a busca com date − 7d; rótulo vem
  de week_start/end mockados); CA-18 barras com a cor do projeto e largura proporcional;
  CA-21 chamada sempre com `tz: browserTZ()`; bucket null como "Sem projeto".
- **Acceptance:** Vitest verde; `check-cores.sh` passa.

## T-06 — Front: ângulos 2 e 3 + estado vazio

- **Objetivo:** FocusStats (tempo focado × fragmentado com %, maior bloco, trocas) e
  GoalsSection (GoalBar da 02 reusada — CA-19) + estados vazios (CA-20).
- **RED:** casos novos no `Reports.spec.tsx`: CA-19 (GoalBar com fill proporcional na cor
  e accent ≥100% — reusa as asserções de comportamento do componente, sem fork); CA-20
  (resposta vazia → 3 estados vazios, sem NaN no DOM); formatação via `fmtDayTotal`.
- **Acceptance:** Vitest verde.

## T-07 — E2E: fluxo nº5 (recorte 3 ângulos)

- **Objetivo:** `frontend/e2e/relatorios.spec.ts` — arquivo NOVO **paralelo** (não muta o
  timer — plan §6): login stub → cria cliente+projeto único com meta 1h via API → semeia
  2 entries de hoje via `POST /entries` (40min e 10min) → `/relatorios` → ângulo 1 mostra
  o projeto com "0h50" e o cliente; ângulo 3 mostra a GoalBar do projeto com realizado
  "0h50" de "1h00"; seção de foco visível com números renderizados (sem igualdade global).
- **Acceptance:** Playwright verde (com `make up` derrubado antes).
- **CA coberto:** CA-22.

## T-08 — Gate + INDEX

- **Objetivo:** `make check-full` (GATE da fase) + `specs/INDEX.md` marcando a 07
  construída (e a convenção nova: rota `reports/week`, denormalização D-06).
- **Acceptance:** GATE VERDE; INDEX atualizado.
