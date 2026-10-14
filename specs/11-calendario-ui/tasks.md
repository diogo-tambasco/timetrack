# tasks.md — Fase 11 · Calendário 2.0

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-13).
> Cada T-XX lista os CAs cobertos. Backend primeiro (settings), depois o núcleo puro de
> layout (TDD pesado), depois componentes/página/ajustes, E2E e gate.

---

## Bloco backend (preferências)

- **T-01 — Migração `0012_calendar_settings`** (CA-01/03 backstop, RNF-03)
  - `backend/internal/db/migrations/0012_calendar_settings.sql` (plan §2).
  - Teste: estende `migrations_*_test.go` (ou novo `migrations_calendar_test.go`) — tabela
    existe, defaults certos, CHECKs barram view/density/hora inválidas e `hour_end<=hour_start`.
  - INDEX.md: registrar 0012 dona da fase 11.

- **T-02 — `calendar.Settings` + Get/Save** (CA-01/02/03/04)
  - Campo `db *pgxpool.Pool` no `calendar.Service` (set em `NewService`); `internal/calendar/
    settings.go` com `Settings`, `GetSettings` (defaults se sem linha), `SaveSettings` (valida
    faixa/enum ANTES do SQL → `domain.Validation`; upsert `ON CONFLICT (user_id)`).
  - Teste `settings_test.go` no pacote calendar (dbtest): defaults; upsert; cada limite de
    validação (view, density, hour_start, hour_end, end<=start) não persiste; isolamento por
    user (mutação removendo `WHERE user_id` morre).

- **T-03 — Handlers + wiring `GET|PUT /api/v1/settings/calendar`** (CA-01/02/03/04)
  - `internal/api/settings_calendar.go` (handlers finos, espelho do pomodoro); montar em
    `mountCalendar` (ou junto, rota por-usuário no `/api/v1`).
  - Teste `settings_calendar_test.go` (harness de api): GET defaults 200; PUT válido 200 +
    persiste; PUT inválido 400 `validation_error`; sem sessão 401 nas duas.

## Bloco núcleo de layout (frontend puro — o coração)

- **T-04 — `lib/calendar-layout.ts` + testes** (CA-09/10/11/11.1/12/13.1/14/19.1/19.2, RNF-04)
  - Funções puras (plan §4): `splitByDay` (descarta segmento de duração zero — CA-11.1),
    `packColumns`, `layoutDay`/`layoutWeek` → `Positioned[]`, `allDayByDay` (end EXCLUSIVO,
    `[startDay,endDay)` — CA-13.1), `monthGridDays`/`inMonth`, `addMonths` (clamp — CA-19.1),
    `periodTitle` (do refDate — CA-19.2), `weekDays`/`dayWindow`, `weekRange`/`dayRange`/`monthRange`.
  - `calendar-layout.test.ts` (RED primeiro): **a lista de casos obrigatórios do plan §4 é a
    fonte** — incluindo os 5 casos dos findings da verificação adversarial (all-day exclusivo,
    fim em 00:00, addMonths clamp, título do mês, borda-a-borda).

- **T-05 — `lib/api.ts` + `lib/queries/calendar.ts`** (CA-01/02/23/24)
  - `CalendarSettings` type + `getCalendarSettings`/`putCalendarSettings`; hooks
    `useCalendarSettings`/`useSaveCalendarSettings` (invalida/seta cache).
  - Teste: `calendar-api.test.ts` estendido (chama os paths certos) — RED→green.

## Bloco componentes + página

- **T-06 — `EventBlock` + `EventPopover`** (CA-21/22)
  - Bloco accent posicionado; clique abre popover (título, horário/"dia inteiro", "Abrir no
    Google" só com `html_link`, target/rel; Esc/fora fecha).
  - Specs: render do bloco (accent, posição via props), popover (link certo, sem link sem
    html_link, fecha no Esc).

- **T-07 — `TimeGrid` + `NowLine`** (CA-09..17, 24)
  - Grade hora×dia (Dia=1col, Semana=7), coluna de horas 0–24, faixa all-day no topo, blocos
    posicionados (consome `Positioned[]` da T-04), `NowLine` só na coluna de hoje, auto-scroll
    no mount, densidade aplicada.
  - Specs (jsdom): evento no `top/height` esperado; sobreposto em colunas; all-day na faixa;
    NowLine presente só com hoje na semana e ausente sem hoje; auto-scroll chamado.

- **T-08 — `MonthGrid` + aposentar `WeekCalendar` da 06** (CA-18, D-09)
  - 6×7 células (`monthGridDays`), nº do dia, chips + "+N", hoje destacado, fora-do-mês
    atenuado; clique na célula → Dia; clique no chip → popover.
  - Spec: 42 células; semanas parciais atenuadas; "+N" quando estoura; hoje destacado.
  - Reescrever `Calendar.spec.tsx` da 06 pro novo layout (a fidelidade dos dados segue
    assertada; muda a forma — desvio registrado no plan D-09); remover `WeekCalendar.tsx`/spec
    se nada mais depender.

- **T-09 — `CalendarNav` + `Calendar.tsx` reescrito** (CA-05/06/07/08/19/20/23/26)
  - `CalendarNav` (‹ Hoje › + título + segmented Dia/Semana/Mês); `Calendar.tsx` mantém os 3
    estados da 06 (CA-26), view inicial = `default_view` (CA-23), `refDate`, busca a janela
    conforme a view, renderiza TimeGrid/MonthGrid.
  - Specs: troca de view muda a grade/janela buscada; nav muda o período; estados
    conectar/erro/reconectar preservados; abre na `default_view`.

- **T-10 — `CalendarGroup` nos Ajustes** (CA-02/03/25)
  - Grupo "Calendário" em `Settings.tsx`: select view, horas start/end (validação `end>start`
    no client antes do PUT), toggle densidade; salva via `useSaveCalendarSettings`.
  - Spec: form inválido não chama API e sinaliza erro; válido chama PUT com o corpo certo.

## Bloco E2E + gate

- **T-11 — E2E `calendario.spec.ts`** (CA-27)
  - Logado+conectado (stub da 06): grade da semana com eventos da fixture posicionados; troca
    Dia/Mês; clique em evento → popover "Abrir no Google"; muda view padrão nos Ajustes →
    reload `/calendario` abre na nova view (persistência backend, banco reusado → nome/seed
    idempotente).
  - Reusar `e2e/helpers.ts` (loginViaStub) + stub de calendar (cmd/calendarstub / fixtures da 06).

- **T-12 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: layout/tempo, fuso/DST/meia-noite, sobreposição/colunas,
    persistência/validação, regressão da 06) + refutação. Corrigir confirmados. Registrar.

- **T-13 — Gate + deploy + smoke + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose).
  - Deploy pela rotina do KICKOFF (rsync + `docker compose build && up -d` em `/opt/timetrack`;
    **NÃO** tocar no Caddy do PrimaOS). Smoke honesto (`/healthz` 200, `/calendario` serve,
    vizinhos intactos).
  - Handoff (CLAUDE.md "retomar..."), INDEX (11 construída), KICKOFF da próxima (volta pra 08),
    commit, briefing.

> **Blocos (regra ≤10 tarefas / contexto):** backend+layout (T-01→T-05) · componentes/página
> (T-06→T-10) · E2E+verificação+deploy (T-11→T-13). Gate `make check-full` só no fim (T-13).

---

## Registro — verificação adversarial da SPEC (15/06, ETAPA 1, pré-código)

Workflow ultracode `verify-cal-spec` (58 agentes: 6 dimensões + refutação dupla). 26 findings
brutos → **6 confirmados** (5 HIGH, 1 MEDIUM), **todos aplicados na spec/plan ANTES de codar**:
1. **All-day end EXCLUSIVO** (HIGH, achado por 2 dimensões / 4 votos) → CA-13.1 + `allDayByDay`
   `[startDay,endDay)` + casos de teste.
2. **Timed fim em 00:00 → segmento fantasma** (HIGH) → CA-11.1 (`splitByDay` descarta duração zero).
3. **±1 mês sem clamp pula fevereiro** (HIGH) → CA-19.1 (`addMonths` com clamp).
4. **Título do mês derivado da grade de 42 dias** (HIGH) → CA-19.2 (`periodTitle` do refDate).
5. **`density`→`--row-h` global regride /hoje e /workspace** (MEDIUM) → CA-24.1 (`--cal-row-h` escopado).
Os 20 restantes foram refutados (fora de escopo declarado / já cobertos / preferência sem
impacto). **O build (próxima sessão) deve começar pela T-01.**

## Registro — BUILD + verificação adversarial do BUILD (15/06, sessão 15)

Build TDD T-01→T-13 completo numa só sessão (contexto não apertou; regra de blocos não
acionada), commit por tarefa. `make check-full` **GATE VERDE** + deploy em produção.
Suítes: Go 100% · Vitest 189/189 · Playwright 21/21.

Verificação adversarial do BUILD (workflow ultracode `verify-cal-build`, 9 agentes, 6
dimensões + refutação dupla): 3 findings brutos → **1 confirmado (MEDIUM), corrigido**:
- **MEDIUM (CA-21):** o backdrop do `EventPopover` é IRMÃO do botão do chip (não filho), e não
  tinha `stopPropagation` → na view **Mês** dispensar o popover pelo clique-fora borbulhava pra
  `month-cell` (`onClick`→`onPickDay`) e navegava pro Dia sem querer. Corrigido com
  `stopPropagation()` no backdrop + teste de regressão no `MonthGrid.spec`.
- **Refutados (2):** rótulo de hora inline no segmento cross-midnight mostrando o start original
  (cosmético — nenhum CA governa o conteúdo do label); gap de DST exatamente à meia-noite
  descartando o segmento (regime extinto no Brasil desde 2019, não reproduzível no fuso do Diogo,
  nenhum CA cobre).
