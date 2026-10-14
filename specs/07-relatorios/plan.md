# plan.md — Fase 07 · Relatórios (ângulos 1–3)

> Como implementar a `spec.md` desta pasta. Herda o baseline (`SDD.md §7`) e os contratos
> das fases 02/03: `time_entries` (filtro duplo workspace+user), recorte por tz IANA com
> `ParseInLocation` (padrão do `entries.ListDay`), `visibleWhere` de projects, GoalBar/
> ProjectChip no front, envelope de erro canônico.

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Período = semana segunda→domingo no fuso do cliente.** Janela `[segunda 00:00, segunda+7d 00:00)` calculada com `time.ParseInLocation` + aritmética de calendário no fuso (mesmo padrão do `ListDay` da 03). | A meta de projeto é semanal (`weekly_hours_goal`) — semana fecha o ângulo 3 sem conversão inventada. Segunda-feira é a convenção BR/ISO. |
| D-02 | **Agregação em 3 queries + composição em Go** no pacote novo `internal/reports`: (1) entries fechadas da janela ordenadas por `started_at ASC` (filtro duplo ws+user); (2) metadados dos projetos referenciados + clientes (JOIN simples por id, SEM filtro de visibilidade); (3) projetos com meta (não-arquivados + `visibleWhere`). Focus metrics e realized derivam da query 1 em Go puro. | Volume é pequeno (semana de 1 usuário); Go puro deixa o limiar/switches unit-testáveis sem SQL acrobático. A query 2 sem visibilidade é deliberada: a entry é DO usuário — ele já conhece o projeto que cronometrou; esconder o nome do próprio histórico seria perda de dado (registrado: projeto que virou private depois continua nomeado no relatório do próprio tempo). A query 3 COM `visibleWhere` espelha a 02 (metas de privados alheios não vazam — CA-15). |
| D-03 | **Limiar de foco = 1500s (25min), constante `focusThresholdSeconds`** no pacote. | Pomodoro clássico; configurável fica fora (spec §6). Constante nomeada + caso de borda testado (1500 exato = foco). |
| D-04 | **`switches` por dia local:** entries da query 1 agrupadas pelo dia de `started_at` no fuso; dentro do dia, cada par consecutivo com `project_id` diferente (nil ≠ id conta) soma 1. | "Pula-pula" é fenômeno intra-dia; troca entre dias é só a rotina. Nil como "projeto distinto" porque largar o projeto pra tarefa avulsa É troca de contexto. |
| D-05 | **Rota `GET /workspaces/{id}/reports/week`** montada no subrouter do workspace (`auth_routes.go`, padrão `mountCore`/`mountEntries`) via option nova `api.WithReports(svc)`. Leitura → viewer passa (CA-03) de graça pelo `RequireWriteRole` existente (só barra POST/PATCH/PUT/DELETE). | Cadeia de auth provada das fases 01/02, zero middleware novo. |
| D-06 | **Resposta embute name/color/client_name** (denormalizado) em `by_project` e `goals`. | O front não precisa de N queries de join; o relatório é um snapshot autocontido — e o E2E assere texto direto. |
| D-07 | **Front:** `useWeekReport(ws, dateISO)` (TanStack Query, key por `[reports, ws, date, tz]`); navegação = estado local `weekOffset` → `date` calculado no cliente (`hoje + 7×offset` dias), rótulo SEMPRE derivado de `week_start/week_end` da resposta. | Fonte única da janela é o servidor (CA-17); o cliente só aponta um dia dentro da semana alvo — imune a divergência de cálculo de segunda-feira nas duas pontas. |
| D-08 | **Barras em CSS puro** (div com width %), cor via `color` do projeto (dado de domínio — exceção sancionada do check-cores na 02, mesma do ProjectChip/GoalBar) e token neutro pro bucket sem projeto. GoalBar da 02 REUSADA no ângulo 3 sem fork. | RNF-01 (zero dep). Componentes provados. |

## 2. Modelo de dados / migrações

**Nenhuma migração** (RNF-04). Tudo deriva de `time_entries` + `projects` + `clients`.

## 3. Contratos

### 3.1 `GET /api/v1/workspaces/{workspaceId}/reports/week?date=YYYY-MM-DD&tz=IANA`

- `date` ausente → hoje no fuso; `tz` ausente → UTC; malformados → `400 validation_error`
  (CA-06/08). Cadeia: 401 sem sessão · 404 não-membro · viewer lê (CA-02/03).

```json
{
  "week_start": "2026-06-08",
  "week_end":   "2026-06-14",
  "total_seconds": 18300,
  "by_project": [
    { "project_id": "uuid",
      "name": "Lauda", "color": "#b07cd6",
      "client_id": "uuid", "client_name": "Lauda.ai",
      "total_seconds": 12600 },
    { "project_id": null, "name": null, "color": null,
      "client_id": null, "client_name": null,
      "total_seconds": 5700 }
  ],
  "focus": {
    "focus_seconds": 15000, "fragment_seconds": 3300,
    "focus_count": 5, "fragment_count": 4,
    "longest_seconds": 5400, "switches": 7
  },
  "goals": [
    { "project_id": "uuid", "name": "Freela PM", "color": "#c9806a",
      "goal_seconds": 36000, "realized_seconds": 12600 }
  ]
}
```

- `by_project`: ordenado `total_seconds DESC` (empate: name ASC, bucket nulo por último);
  soma dos buckets ≡ `total_seconds` (CA-09).
- `goals`: ordenado `name ASC`; inclui 0 realizado (CA-15); `goal_seconds` =
  `round(weekly_hours_goal × 3600)` (CA-16).
- Listas vazias = `[]` (nunca null) — padrão das listagens da 02/03.

### 3.2 Tipos front (`lib/api.ts`, extensão aditiva)

`WeekReport`, `ReportProjectBucket`, `ReportFocus`, `ReportGoal` espelhando o JSON acima;
`getWeekReport(ws, {date, tz})`.

## 4. Libs

**Nenhuma nova** (RNF-01). SQL via pgx existente; barras/tiles em CSS com tokens.

## 5. Onde vive o quê

```
backend/internal/
  reports/
    service.go        Service{db, now} (clock injetável, padrão entries.Service):
                      Week(ctx, wsID, userID, date, tz) → Report
                      weekWindow (D-01) · agregação (D-02) · focus (D-03/D-04)
    service_test.go   janela/tz/bordas (CA-06/07) · buckets (CA-09/10/11) ·
                      focus (CA-12/13/14) · goals (CA-15/16) · isolamento (CA-01) ·
                      rodando fora (CA-04) — dbtest + entries.Insert pra semear
  api/
    reports_routes.go option WithReports + mountReports no subrouter do workspace
    reports_handlers.go handler do GET (400/shape — CA-08, contrato §3.1)
    reports_handlers_test.go 401/404/viewer-200/400/shape + RNF-02 (perf 1k entries)
backend/cmd/server/   wiring WithReports(reports.NewService(pool))
frontend/src/
  lib/api.ts          tipos + getWeekReport (§3.2)
  lib/queries/reports.ts useWeekReport (D-07)
  routes/cockpit/Reports.tsx página real (substitui StubPage): navegação + 3 seções
  routes/cockpit/Reports.spec.tsx render das seções/vazio/navegação (CA-17..21)
  components/reports/  WeekNav.tsx · ProjectBars.tsx · FocusStats.tsx · GoalsSection.tsx
                       (componentes pequenos; specs ficam no Reports.spec.tsx — página é a
                       unidade testada, padrão das páginas da 02)
frontend/e2e/relatorios.spec.ts  E2E novo (CA-22) — arquivo PARALELO (não muta timer)
```

## 6. Pontos de integração e armadilhas conhecidas

- **E2E paralelo com banco compartilhado:** o `cronometro.spec.ts` roda em paralelo criando
  entries do MESMO usuário → toda asserção do E2E é **escopada ao projeto único** do teste
  (nome com `Date.now()`); números globais (focus/total geral) só presença/render, nunca
  igualdade. Entries semeadas por `POST /entries` (API manual — não toca o timer).
- **`weekly_hours_goal` é float** (12.25 válido) → `goal_seconds` com `math.Round` antes do
  cast (CA-16); nunca truncar.
- **Semana com horário de verão:** aritmética de calendário no fuso (`time.Date` no loc,
  +7 dias de calendário), nunca `time.Hour×168`. (BR aboliu DST, mas tz IANA é input
  arbitrário — ex.: America/New_York.)
- **Entries cruzando a meia-noite/semana:** pertencem à janela do `started_at` (CA-07) —
  consistente com a lista Hoje da 03; a duração NÃO é fatiada entre semanas (decisão: o
  relatório conta entries, não fatias — documentado aqui pra não re-litigar).
- **`switches` e ordem estável:** query 1 ordena `started_at ASC, id` (desempate
  determinístico — duas entries manuais podem ter o mesmo started_at).
- **Viewer no front:** página é só leitura — nenhum gating novo necessário (o `useRole`
  não entra; nada de escrita na tela).
- **RNF-02 (perf):** teste semeia 1.000+ entries via INSERT em lote (generate_series ou
  loop com `entries.Insert`) e assere as 3 amostras < 200ms (padrão do RNF-04 da 03 —
  exceção red→green documentada no cabeçalho, precedente das fases 02/03).
- **Reports.tsx hoje é StubPage** — substituição limpa; a rota `/relatorios` e o item do
  rail já existem (fase 00).
