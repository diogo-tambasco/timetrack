# plan.md — Fase 14 · Relatórios 2.0

> Como construir a `spec.md` desta pasta. **Estende** `internal/reports` da fase 07 (janela,
> byProject, focusMetrics, goals) e **não regride** a 06 (planned). Decisões D-xx + arquitetura +
> ordem de tarefas. Herda os contratos das fases 02/03/07/12 e o envelope de erro canônico.

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Rota nova `GET /workspaces/{id}/reports` (genérica) + `/reports/week` mantido como ALIAS.** O handler de `/reports/week` passa a delegar pro mesmo service com `period=week, date`. O `/reports` aceita `period` (day/week/month) OU `from`+`to` (custom), `tz`, filtros e `compare`. | Não-regressão (CA-04/09): o front da 07 e o `relatorios.spec` antigo seguem chamando `/reports/week` sem mudança. A 14 só adiciona. Generalizar a janela num lugar só evita duas lógicas de calendário. |
| D-02 | **`weekWindow` da 07 vira `periodWindow(period, date, from, to, loc)`** retornando `[start, end)` no fuso. `day` = `[D, D+1)`; `week` = a `weekWindow` atual (segunda→domingo); `month` = `[1º, 1º+1mês)` com aritmética de calendário no loc; custom = `[from, to+1d)`. Validação (`to<from`, datas/period/tz malformados) → `domain.Validation` (CA-07). | Reusa a aritmética provada (CA-06/07 da 07, `AddDate` no loc, nunca `time.Hour×N` — DST de fuso arbitrário). `from`/`to` precedem `period` (CA-06). |
| D-03 | **Agregação reusa as funções da 07** (`byProject`, `focusMetrics`, `goals`) sobre a query 1 já existente, agora com **filtros no WHERE** (project_id direto; client_id via `project_id IN (projetos do cliente)`; objective_id via `task_id IN (tasks do objetivo)`). O service ganha uma struct `Filters{ProjectID, ClientID, ObjectiveID}` e um `whereFilters(...)` que monta os predicados parametrizados. | RNF-05 (07 intocada): as funções continuam recebendo a `list []entryRow`; só a query que ENCHE a list ganha filtros. Filtros combináveis (CA-10/11). |
| D-04 | **Comparação `compare=previous` = 2ª passada no service** (mesma janela deslocada um período atrás, mesmos filtros), devolvendo só `total_seconds`+`kpis` no bloco `previous`. SEM compare → bloco nil. | CA-25/26: opt-in num request (o cliente não faz 2 chamadas; o servidor reusa o pipeline). A janela anterior usa o mesmo `periodWindow` com a data deslocada (week −7d; month `addMonths −1`; custom −duração). |
| D-05 | **Objetivos por entrega = 3 queries** no pacote (fase 12 dona das tabelas): (q-obj) objetivos dos projetos VISÍVEIS do ws com atividade-no-período OU meta — JOIN `objectives`×`projects` com `visibleWhere`; (q-tasks) `tasks_total`/`tasks_done` por objetivo (contagem CUMULATIVA, não filtrada por período — CA-17); (q-realized) tempo do período por objetivo via `time_entries.task_id → tasks.objective_id`, filtro duplo ws+user. Composição em Go: `progress_pct = round(tasks_done/tasks_total×100)` (inteiro 0–100, 0 se total 0). **A q-obj aplica os MESMOS filtros (`whereFilters`) ativos (CA-16.1):** com `project_id`/`client_id`/`objective_id`, a lista de objetivos é restrita aos projetos/objetivo que casam — o ramo "OU têm meta" NÃO sobrepõe o filtro (senão objetivos de outro projeto com meta vazam). | CA-16..19/16.1: entrega é cumulativa (tasks_done não depende da janela); tempo é do período. `status`/`goal_hours` vêm de `objectives` (a 14 só LÊ — a conclusão automática é da 12, CA-18). Tasks sem objetivo não viram item (CA-19) — o JOIN é por `objective_id NOT NULL`. `progress_pct` espelha o `donePct` da 12 (×100). |
| D-06 | **`availability` = jornada útil configurável.** `expected_seconds = work_hours_per_day×3600 × diasDoPeríodo` (todos os dias, ou só seg–sex se `workdays_only`); `tracked = total_seconds`; `untracked = max(expected−tracked,0)`; `tracked_pct = round(tracked/expected×100)` capado em 100, 0 se expected=0. `expected_basis` (string) descreve a suposição. A config vem de `report_settings` (q própria, defaults 8h/workdays_only=true). | CA-20/21: "não-rastreado" precisa de denominador; a jornada útil é a fonte mais simples e do controle do usuário. Registrado na resposta (`expected_basis`) pra ser honesto sobre a suposição. **Dias do período = ITERAR `start..end` no loc (`AddDate(+1d)` até `end`), contando seg–sex se `workdays_only`** (furo confirmado: `(end−start)/24h` quebra na transição de DST e não sabe classificar dia útil — usar a aritmética de calendário do §6, idêntica ao `weekWindow`). |
| D-07 | **Ângulo 4 (Planejado×Real da 06) no front:** quando o período é **Semana**, a página chama `usePlannedReport(ws, dateDaSemana)` como hoje (CA-33); em Dia/Mês/Custom, o card de Planejado×real é **ocultado** (o `/reports/planned` é semanal por construção — não vou estender a 06). | RNF-05: NÃO toco no backend de planned. O ângulo 4 só faz sentido semanal hoje; ampliá-lo é fase futura (§7 da spec). Documentado: nenhuma regressão, só visibilidade condicional. |
| D-08 | **`focus_by_hour` = 24 buckets** derivados da `list` em Go: pra cada entry de FOCO (≥1500s), soma `focus_seconds` no bucket `started_at.In(loc).Hour()`. KPIs (`deep_focus_pct`, `avg_session_seconds`) derivam de `focus`+contagem de entries — zero query nova. | CA-22/23/24: tudo deriva da query 1 já carregada. v1 buckeia pela hora de início (fatiar por hora real = refino futuro, §7). 24 buckets sempre (zeros) pro front não tratar ausências. |
| D-09 | **Front:** `useReport(ws, {period|from,to, filters, compare})` (TanStack Query, key inclui tudo + tz). Período = estado local (preset + refDate, ou range custom); navegação reusa `addDays/addWeeks/addMonths` (o `addMonths` com clamp da fase 11 — importar de `lib/calendar-layout.ts`, NÃO duplicar). Rótulo SEMPRE do `from/to` da resposta (CA-08/27). | D-01 da 07 (fonte única da janela = servidor) estendido pro período. Reuso do `addMonths` clampado evita o bug de fevereiro de novo. |
| D-10 | **Página reescrita** (`Reports.tsx`) com layout do `reports.jsx` (KPI strip + grid de cards + toggle Gráficos/Tabela), reusando `ProjectBars`/`FocusStats`/`GoalsSection`/`PlannedVsReal` da 07/06 e novos `KpiStrip`/`TrackedDonut`/`FocusByHour`/`ObjectivesSection`/`ReportTable`/`PeriodNav`/`ReportFilters`. **Sem `WeekNav` fork** — `PeriodNav` o substitui (WeekNav pode ser aposentado se nada mais usar — checar na T-10). | CA-27..32: o mockup já desenhou; portar com tokens. Componentes da 07 reusados sem fork (barras/foco/metas). |

## 2. Modelo de dados / migrações

- **Lê (fase 12, NÃO redefine):** `objectives(id, workspace_id, project_id, name, status,
  goal_hours, ...)` + `tasks.objective_id`. A 14 consome em `SELECT` apenas.
- **Cria — `0014_report_settings.sql`** (dona: fase 14; após `0013_objectives` da fase 12):

```sql
-- 0014_report_settings — fase 14: preferências do relatório POR USUÁRIO
-- (jornada útil esperada — base do rastreado×não-rastreado, CA-21). Espelha
-- calendar_settings (fase 11). Sem linha = defaults na leitura.
CREATE TABLE report_settings (
  user_id            uuid PRIMARY KEY REFERENCES users(id),
  work_hours_per_day numeric(4,2) NOT NULL DEFAULT 8   CHECK (work_hours_per_day > 0 AND work_hours_per_day <= 24),
  workdays_only      boolean      NOT NULL DEFAULT true,
  updated_at         timestamptz  NOT NULL DEFAULT now()
);
```

- INDEX.md: registrar **0014 dona da fase 14**; **0013 é da fase 12** (objetivos) — a 14 só lê.

## 3. Contratos

### 3.1 `GET /api/v1/workspaces/{workspaceId}/reports`

Query params (todos opcionais com defaults): `period` (`day|week|month`, default `week`) **ou**
`from`+`to` (YYYY-MM-DD, custom — precede `period`); `date` (referência do preset, default hoje no
fuso); `tz` (IANA, default UTC); `project_id`/`client_id`/`objective_id` (uuid); `compare`
(`previous`). Malformados / `to<from` → `400 validation_error`. Cadeia: 401 / 404 / viewer-lê.

```json
{
  "from": "2026-06-08",
  "to":   "2026-06-14",
  "total_seconds": 138900,
  "by_project": [ { "project_id":"uuid","name":"Lauda","color":"#b07cd6",
                    "client_id":"uuid","client_name":"Lauda.ai","total_seconds":46800 },
                  { "project_id":null,"name":null,"color":null,
                    "client_id":null,"client_name":null,"total_seconds":5700 } ],
  "focus": { "focus_seconds":120000,"fragment_seconds":18900,
             "focus_count":18,"fragment_count":12,"longest_seconds":7200,"switches":21 },
  "goals": [ { "project_id":"uuid","name":"Freela PM","color":"#c9806a",
               "goal_seconds":36000,"realized_seconds":12600 } ],
  "objectives": [ { "project_id":"uuid","project_name":"Lauda","project_color":"#b07cd6",
                    "objective_id":"uuid","objective_name":"Lançar v1","status":"open",
                    "tasks_total":8,"tasks_done":3,"progress_pct":38,
                    "realized_seconds":28800,"goal_seconds":72000 } ],
  "availability": { "tracked_seconds":138900,"expected_seconds":144000,
                    "untracked_seconds":5100,"tracked_pct":96,
                    "expected_basis":"8h/dia × 5 dias úteis" },
  "focus_by_hour": [ {"hour":0,"focus_seconds":0}, "...", {"hour":23,"focus_seconds":0} ],
  "kpis": { "tracked_seconds":138900,"deep_focus_pct":86,
            "avg_session_seconds":4630,"untracked_seconds":5100 },
  "previous": { "total_seconds":120000,
                "kpis": { "tracked_seconds":120000,"deep_focus_pct":80,
                          "avg_session_seconds":4000,"untracked_seconds":24000 } }
}
```

- `by_project`/`goals` = idênticos à 07 (D-06 da 07: denormalizado, ordenado, listas vazias `[]`).
- `objectives`: ordenado `project_name ASC, objective_name ASC`; `progress_pct` inteiro 0–100
  (round); `goal_seconds` omitido/0 quando objetivo sem `goal_hours`.
- `focus_by_hour`: **sempre 24 itens** (CA-23).
- `previous`: presente só com `compare=previous`; **null/ausente** caso contrário (CA-26).
- Listas vazias = `[]`; objetos zerados (não null) em `focus`/`availability`/`kpis` — sem NaN no
  front (CA-32).

### 3.2 `GET /api/v1/workspaces/{workspaceId}/reports/week?date&tz` (ALIAS — D-01)

Inalterado em shape (07). Internamente = `reports?period=week&date&tz` sem filtros/compare. Prova
de não-regressão: CA-09 assere igualdade byte com a resposta da 07 pra a mesma semana.

### 3.3 `GET|PUT /api/v1/settings/reports` (preferências — por usuário)

`{ "work_hours_per_day": 8, "workdays_only": true }`. GET sem linha → defaults; PUT valida
(`0 < hpd ≤ 24`) ANTES do SQL → `domain.Validation` (CA-29 backstop nos CHECKs). Rota por-usuário
no `/api/v1` atrás de `RequireAuth` (espelha `settings/calendar` da 11 e `settings/pomodoro` da 03).

### 3.4 Tipos front (`lib/api.ts`, extensão aditiva)

`PeriodReport`, `ReportObjective`, `ReportAvailability`, `FocusHourBucket`, `ReportKpis`,
`ReportPrevious`, `ReportSettings` + `getReport(ws, params)`, `getReportSettings`/
`putReportSettings`. `WeekReport`/`getWeekReport` da 07 **mantidos** (o alias segue vivo).

## 4. Libs

**Nenhuma nova** (RNF-01). SQL via pgx; donut em `conic-gradient`, barras/tabela em CSS com
tokens; `addMonths` reusado de `lib/calendar-layout.ts` (fase 11).

## 5. Onde vive o quê

```
backend/internal/
  reports/
    service.go          + periodWindow (D-02, generaliza weekWindow), Filters,
                          whereFilters (D-03), Report(ctx, ws, user, win, filters, compare),
                          availability (D-06), focusByHour+kpis (D-08), previous (D-04).
                          weekWindow/byProject/focusMetrics/goals da 07 PRESERVADOS (reuso).
    objectives.go (NOVO) objectivesSection (D-05): 3 queries (visível/atividade-ou-meta,
                          contagem cumulativa de tasks, realized do período) + composição.
    settings.go   (NOVO) report.Settings + GetSettings/SaveSettings (espelha calendar.settings).
    *_test.go            períodos/bordas (CA-05..09), filtros (CA-10..12), objetivos
                          (CA-16..19), availability (CA-20/21), focus_by_hour/kpis (CA-22..24),
                          compare (CA-25/26), isolamento (CA-01), perf (RNF-03). planned_test
                          e service_test da 07 INTOCADOS (CA-04).
  api/
    reports_routes.go   + WithReportSettings/mount; handler genérico /reports; /reports/week
                          vira alias (delega — D-01); /reports/planned INTOCADO.
    settings_reports.go (NOVO) handlers GET|PUT /settings/reports (finos, espelho pomodoro/cal).
    reports_handlers_test.go + casos do /reports (shape, filtros, compare, 400, viewer, perf).
  cmd/server/           wiring das settings de relatório.
frontend/src/
  lib/api.ts            tipos + getReport + get/putReportSettings (§3.4)
  lib/queries/reports.ts useReport (D-09) + useReportSettings/useSaveReportSettings
  routes/cockpit/Reports.tsx  reescrita (período + filtros + KPIs + cards + tabela) — D-10
  routes/cockpit/Reports.spec.tsx  CA-27..33
  routes/cockpit/Settings.tsx + <ReportGroup/> (CA-29)
  components/reports/
    PeriodNav.tsx (NOVO) seletor Dia/Semana/Mês/Custom + ‹ Hoje › (rótulo da resposta)
    ReportFilters.tsx (NOVO) selects projeto/cliente/objetivo + toggle compare
    KpiStrip.tsx (NOVO) faixa de KPIs (+delta com compare)
    TrackedDonut.tsx (NOVO) donut conic-gradient (CA-20/30)
    FocusByHour.tsx (NOVO) barras de intensidade por hora (CA-22/30)
    ObjectivesSection.tsx (NOVO) progresso por entrega (CA-16..19)
    ReportTable.tsx (NOVO) tabela densa on-demand (CA-31): linha/projeto SÓ com colunas do
                          contrato por projeto — projeto, cliente, rastreado (by_project),
                          meta (goals), objetivos concluídos/total (objectives por project_id).
                          NÃO fabricar não-rastreado/foco%/sessões por projeto (são de workspace).
    ChartsTableToggle.tsx (NOVO) Gráficos↔Tabela
    ProjectBars/FocusStats/GoalsSection/PlannedVsReal  REUSADOS (07/06), sem fork
  components/settings/ReportGroup.tsx (NOVO)
frontend/e2e/relatorios-v2.spec.ts (NOVO)  CA-34 — paralelo, escopo por projeto único
```

## 6. Pontos de integração e armadilhas conhecidas

- **NÃO-REGRESSÃO é o risco nº1 (RNF-05):** `service_test.go`/`planned_test.go` da 07/06 NÃO
  podem mudar de asserção. `/reports/week` delega mas devolve o MESMO shape (CA-09 pina a
  igualdade). Rodar a suíte da 07 sem editar é o teste de fumaça da generalização.
- **Dependência da fase 12:** se a fase 12 ainda não construiu `objectives`/`tasks.objective_id`,
  as queries de objetivo quebram — esta fase **assume a 12 pronta** (ordem 12→13→14, MILESTONE-1
  §7). O dbtest semeia `objectives`/`tasks` via INSERT (não há API de seed própria). Caso a 12
  derive de spec diferente, alinhar os nomes de coluna ANTES de codar a T-04.
- **Contagem de entrega cumulativa vs. tempo do período (CA-17):** `tasks_done`/`tasks_total` são
  contados sobre TODAS as tasks do objetivo (sem janela); só `realized_seconds` usa a janela. Um
  caso de teste com task `done` FORA do período + entry DENTRO pina os dois grãos.
- **`expected_seconds` e dias do período (D-06):** contar dias iterando `start..end` no loc
  (`AddDate(+1d)` até `end`), classificando seg–sex se `workdays_only`. Custom de 1 dia num
  sábado com `workdays_only` → `expected=0` → `tracked_pct=0` (sem divisão por zero — caso de
  teste). `expected_basis` é string informativa, não usada em cálculo.
- **`avg_session_seconds` divisão por zero (CA-24):** 0 entries → 0 (não NaN). Idem
  `deep_focus_pct`/`tracked_pct` com denominador 0.
- **Comparação e `addMonths` (D-04):** o mês anterior usa clamp do dia (mar/31 −1mês = fev/28) —
  reusar a aritmética do `periodWindow(month)` ancorada no 1º dia evita o off-by. Custom anterior
  = `[from−duração, from)`. Teste de round-trip do mês.
- **Filtros e visibilidade (CA-11/12):** `objective_id`/`project_id`/`client_id` válidos mas
  invisíveis/inexistentes → resultado **vazio**, nunca 404 (não vaza). O `whereFilters` aplica
  `visibleWhere` quando o filtro é por objetivo/projeto que poderia ser de outro usuário.
- **Perf (RNF-03):** semear 1.000+ entries + dezenas de objetivos/tasks; medir
  `period=month&compare=previous&objective filtros` (pior caso: 2 janelas + objetivos). 3 amostras
  < 200ms; exceção red→green documentada no cabeçalho (precedente 07/02/03).
- **E2E paralelo com banco reusado:** `relatorios-v2.spec.ts` escopa tudo ao projeto único do
  teste (nome com `Date.now()`); números globais só presença, nunca igualdade — padrão do
  `relatorios.spec` da 07. Semear objetivo+tasks via INSERT/API; entries via `POST /entries`.
- **`addMonths` no front:** importar de `lib/calendar-layout.ts` (fase 11) — NÃO reimplementar
  (o bug do fevereiro já foi corrigido e testado lá).
- **Ângulo 4 condicional (D-07):** ocultar o card de Planejado×real fora do preset Semana — NÃO
  é regressão (o card só existia na visão semanal); documentar pra não re-litigar.

## 7. Ordem de execução (ver tasks.md)

Backend primeiro: generalizar a janela + filtros (preserva a 07) → objetivos por entrega →
availability/foco-hora/kpis → comparação → settings → rota genérica + alias + perf. Depois front:
api/query → período/filtros/toggle + KPIs → donut/foco-hora/objetivos/tabela → ajustes → E2E →
verificação adversarial → gate. Commit por tarefa. `make check-full` é o gate final.
