# plan.md — Fase 28 · Projeto vivo + Calendário por projeto

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> assinaturas + ordem de tarefas. **Estende** as fases 02 (`internal/projects`), 12 (`/board`,
> `internal/objectives`), 06 (`internal/calendar`), 24 (tab-shell) e 26 (notas, aditivo) — não as
> reescreve. Herda o baseline do `SDD.md §7` e a cadeia de middlewares
> (`RequireAuth` → `RequireMember` → `RequireWriteRole`). Nada aqui é código — é o contrato.

---

## 1. Arquitetura geral

```
Backend (estende; pacote novo internal/projectsummary + cal por projeto):
  migrations/0025_projects_deadline.sql        ← ALTER projects ADD deadline + idx_tasks_due (RNF-05)
  internal/projects/service.go (existe)         ← + deadline em CreateInput/PatchInput (tri-state) + no Project struct
  internal/projects/summary.go        (NOVO)    ← Summary(ws,userID,tz,archived): 3 queries fixas → cards vivos (CA-04..08)
  internal/projects/calendar.go       (NOVO)    ← ProjectCalendar(ws,userID,projectID,from,to): itens datados (CA-09..15.1)
  internal/api/projects_handlers.go (existe)    ← + handlers GET /projects/summary e /projects/{id}/calendar; deadline nos handlers de project
  internal/api/core_routes.go (existe)          ← + rotas summary e calendar no subrouter do workspace

Frontend (cards vivos + abas Overview/Calendar):
  lib/api.ts                                    ← deadline em Project/ProjectInput; ProjectSummary[]; ProjectCalendarItem[]; getProjectsSummary/getProjectCalendar
  lib/queries/projects.ts (existe)              ← + useProjectsSummary, useProjectCalendar; patch de project invalida summary
  lib/queries/calendar.ts (existe da 06/11)     ← reusa useCalendarEvents(from,to,tz) p/ o overlay (janela = monthRange(refDate) da fase 11, grade de 42 dias, NÃO o month estrito)
  components/projects/ProjectCard.tsx (existe 02) ← reescrito p/ card vivo (horas reais, pendentes, deadline, "hoje") — caminho REAL: components/projects/, NÃO components/cards/
  routes/cockpit/Projects.tsx (existe)          ← consome useProjectsSummary (1 request)
  routes/cockpit/ProjectDetail.tsx (existe 12/24) ← tab-shell ganha Overview + Calendar (default Overview)
  components/project/OverviewTab.tsx   (NOVO)   ← KPIs + Upcoming + Progress(+Goals) + Recent notes (CA-17/21)
  components/project/ProjectCalendarTab.tsx (NOVO) ← grade do mês (itens do projeto + overlay GCal) (CA-18)
  lib/calendar-layout.ts (existe da 11)         ← reusa monthGridDays (grade) + monthRange (janela do overlay GCal, 42 dias) (PUROS) — NÃO duplicar grade
```

## 2. Migração — `0025_projects_deadline.sql` (dona: fase 28)

```sql
-- 0025_projects_deadline — fase 28 (plan §2): prazo do projeto (card vivo, "tem algo hoje",
-- calendário do projeto). NULL = sem prazo (a maioria não tem — CA-02). + índice de due_date
-- em tasks pro due_today/calendário (RNF-05).
ALTER TABLE projects ADD COLUMN deadline date;   -- NULL default; sem CHECK (date já valida)

-- due_today (summary) e o kind:'task' do calendário filtram por due_date; sem este índice a
-- varredura cresce com as tasks. Parcial (só tasks com due_date — a maioria é NULL).
CREATE INDEX idx_tasks_due ON tasks (project_id, due_date) WHERE due_date IS NOT NULL;
```

- Numeração: na M3 as migrações vão de `0021` (fase 24) a `0026` (fase 29). Esta fase é a **`0025`**
  (PRD §7). NÃO "ler o próximo livre do disco" — sob build em worktrees as fases leriam o mesmo
  número e colidiriam (lição da M2). Conferir o diretório só pra sanity; o número é FIXO.
- **Sem migração de `objectives.target_date`** — essa coluna é da fase 29 (`0026_objectives_modes`).
  Esta fase trata `target_date` como **opcional/aditivo** (CA-15.1; ver D-05).
- `date` no Postgres já rejeita valores impossíveis no cast; a validação de formato `YYYY-MM-DD` é
  no service ANTES do SQL (CA-01.1 — nunca deixar o cast estourar pra `500`).
- INDEX.md: registrar **0025 dona da fase 28**.

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **`deadline` é coluna de `projects`, editada pelo CRUD da fase 02** (não pacote novo) | É um campo simples do projeto (igual `weekly_hours_goal`). Reusa o caminho de escrita REAL da fase 02 — `getForWrite` (visibilidade→404, service.go:190) + `canEditMeta` (permissão→403, service.go:206), exatamente como `Patch`/`Create` fazem hoje (service.go:280/284). ⚠️ **NÃO existe `loadProjectGate` no pacote `projects`** (esse símbolo é do `internal/objectives`/`internal/tasks`) — não citar aqui. Esta fase só ADICIONA o campo (tri-state) ao `CreateInput`/`PatchInput`/`Project` struct e às leituras (`List`/`Get`/`board`); a autorização **não muda** (CA-03). |
| D-02 | **Summary = endpoint NOVO `GET .../projects/summary`, 3 queries fixas** (NÃO N× `/board`) | RNF-04/05: chamar `/board` por projeto na lista é N+1 e cada `/board` é 3 queries → 3N. O summary faz: (1) projetos visíveis do ws (1 query, `visibility='shared' OR owner_user_id=$u`, + `archived` flag); (2) contadores de task `GROUP BY project_id` (1 query: `count(*)`, `count(*) FILTER (WHERE status='done')`, `count(*) FILTER (WHERE due_date=$today AND status<>'done')`); (3) soma de tempo `GROUP BY project_id` das entries FECHADAS do user (1 query). Montadas em memória em O(n). |
| D-03 | **`tracked_seconds` do summary = bucket "projeto" do D-06 da fase 12** | Consistência: o card e o `/board` devem bater. Soma das entries FECHADAS do user com `project_id` = projeto (incluindo as sem `task_id`). NÃO soma por `task_id ∈ projeto` (esse é o bucket de task do `/board`, que pode divergir — D-06 da 12). O card mostra o tempo do **projeto**. |
| D-04 | **`due_today_count` no fuso da requisição** (`?tz`), default UTC | "Hoje" depende do fuso (uma task que vence 25/06 vira "hoje" 1h antes em SP que em UTC). Computar o "hoje" como `date` no `tz` informado (`time.Now().In(loc)` truncado a YYYY-MM-DD) e comparar com `tasks.due_date` (`date`). `time.LoadLocation` valida o tz ANTES do SQL (CA-06 → 400). Padrão das fases 03/14. |
| D-05 | **Calendário do projeto = endpoint NOVO `GET .../projects/{id}/calendar`, agregação por data; `target_date` DETECTADO em runtime (dependência fraca da fase 29)** | RF-21: itens datados do projeto. 3 queries: (a) `projects.deadline` do projeto se ∈ janela; (b) tasks do projeto com `due_date ∈ janela`; (c) objetivos do projeto com `target_date ∈ janela` **SE a coluna existir** (CA-15.1). A presença de `objectives.target_date` é checada **1× no boot** (`information_schema.columns`, cacheada num bool no `Service`), evitando 500 quando a fase 29 ainda não buildou. Coluna ausente ⇒ query (c) é pulada. Quando a 29 deployar, "liga" sem mudar este código. |
| D-06 | **Overlay GCal NÃO entra no endpoint do projeto** — vem do `GET /api/v1/calendar/events` (fase 06), buscado no front e sobreposto por data | A fase 06 não associa evento a projeto (limitação herdada). Misturar os eventos `primary` do usuário no endpoint do projeto mentiria que são "do projeto". Mantê-los separados deixa a limitação honesta (CA-13) e reusa o endpoint/cache da fase 06 sem ciclo de import (`projects` NÃO importa `calendar`). |
| D-07 | **DUAS janelas, fonte única cada**: (i) itens do PROJETO via `?month=YYYY-MM` (backend resolve `from`/`to` = 1º..último dia do mês, DST-safe, `AddDate(0,1,0)−1` dia); (ii) overlay do GCal via **`monthRange(refDate)`** da fase 11 (janela dos **42 dias da grade** seg→dom, ver `lib/calendar-layout.ts:267`) | O calendário do projeto desenha uma grade de **42 células** (`monthGridDays`, fase 11), incluindo dias de *spillover* (fim do mês anterior / início do próximo). Se o overlay do GCal usasse o `from`/`to` ESTRITO do mês (que o endpoint do projeto devolve), os eventos do GCal nos dias de spillover SUMIRIAM (inconsistência visual; diverge da fase 11). Por isso o overlay usa `monthRange(refDate)` (mesma janela que a grade exibe) — **NÃO** o `from`/`to` do endpoint do projeto. Os itens do projeto continuam com o `?month=` (só caem nos dias do próprio mês, sem spillover). ⚠️ O `?month=` do endpoint do projeto produz janela do MÊS; `calendarWindow` do `/calendar/events` (fase 06, `calendar_routes.go:116`) trata `to` como dia ÚLTIMO inclusivo e retorna `[from, to+1d)` — por isso o overlay deve receber o `to` da `monthRange` (última célula da grade), não o `to` estrito do mês. |
| D-08 | **Frontend reusa `lib/calendar-layout.ts` da fase 11** (`monthGridDays` p/ a grade + `monthRange` p/ a janela do overlay GCal, PUROS, tz-independentes) | Não reescrever a matemática de grade do mês (a fase 11 já a tem testada em 3 fusos). O `ProjectCalendarTab` posiciona os chips dos itens do projeto nos dias da grade que `monthGridDays` devolve, e busca o overlay do GCal na janela `monthRange(refDate)` (os 42 dias da grade — cobre os dias de spillover, D-07). |
| D-09 | **Cards vivos: `ProjectCard` reescrito, `Projects.tsx` consome `useProjectsSummary`** (1 request) | RNF-04. A página de Projetos da fase 02 fazia `useProjects` (lista crua, `0h` hardcoded no card). Troca por `useProjectsSummary` (CA-04). O card linka pro detalhe (já é assim desde a fase 12). |
| D-10 | **Tab-shell: adiciona Overview/Calendar à fase 24, default = Overview** | A fase 24 entrega o shell de abas (Tasks/Notes/Goals). Esta fase ADICIONA Overview e Calendar. Default ao abrir = Overview (mockup `S.tab='overview'`). O `ProjectDetail` da fase 12/24 ganha os 2 painéis novos; o `useBoard` (fase 12) alimenta os KPIs/Progress/Goals do Overview. |
| D-11 | **Overview: KPIs e Progress do `/board`; "This week" do relatório** | O `/board` (fase 12) já dá `tracked_seconds`, `task_count`, `done_count`, `done_pct` por objetivo e projeto. "This week" precisa de horas da semana corrente filtradas por projeto → reusa `GET .../reports?period=week&project_id=` (fase 14) **OU** `/reports/week` (fase 07) — escolha na T-10 (provável: `/reports` da 14, que aceita `project_id`). Recent notes = fase 26 (aditivo). |
| D-12 | **Rótulos das abas/KPIs em EN (paridade com o mockup)**; resto da UI em PT-BR | O mockup usa `Overview/Tasks/Notes/Goals/Calendar` e `Total time/This week/Open/Done/Upcoming/Recent notes/Progress`. Manter a paridade visual do mockup (fonte da verdade) é mais forte que a regra PT-BR pros rótulos fixos do shell. Mensagens dinâmicas e o resto seguem PT-BR. Decisão registrada (espelha a tensão já vivida; o Diogo validou o mockup em EN). |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) — idênticos às fases anteriores.

### 4.1 `projects.deadline` (estende a fase 02)

`POST .../projects` e `PATCH .../projects/{id}` ganham `deadline` (CA-01):

```json
// → PATCH (deadline tri-state: ausente mantém, null remove, "YYYY-MM-DD" define)
{ "deadline": "2025-09-30" }

// ← 200  (objeto de project da fase 02 + deadline)
{ "id": "0d9e...", "workspace_id": "8c...", "client_id": null, "name": "Acme",
  "color": "#4f9dde", "weekly_hours_goal": 18, "visibility": "shared",
  "owner_user_id": "me0...", "archived_at": null, "deadline": "2025-09-30",
  "created_at": "..." }

// ← 400 deadline malformado (CA-01.1)
{ "error": { "code": "validation_error", "message": "deadline deve ser uma data YYYY-MM-DD" } }
```

- `deadline` aparece também em `GET .../projects`, `GET .../projects/{id}` e no `project` do
  `GET .../projects/{id}/board` (campo aditivo — RNF-05 da fase 12 preservado: o board ganha 1
  campo a mais, nada existente muda).
- `project_id` (no summary) e a autorização do PATCH NÃO mudam (gate da fase 02).

### 4.2 Summary — cards vivos (CA-04..08)

`GET .../projects/summary?tz=America/Sao_Paulo&archived=false` → `200`:

```json
{ "projects": [
  { "id": "0d9e...", "name": "Acme", "color": "#4f9dde", "client_id": null,
    "deadline": "2025-09-30",
    "task_count": 6, "done_count": 1, "open_count": 5,
    "tracked_seconds": 511200, "due_today_count": 2 },
  { "id": "ak...", "name": "Arkan", "color": "#c98a3a", "client_id": "cli...",
    "deadline": null, "task_count": 2, "done_count": 2, "open_count": 0,
    "tracked_seconds": 129600, "due_today_count": 0 }
] }
```

- 1 request, 3 queries fixas (D-02), zero N+1 (RNF-04).
- Só projetos **visíveis** (CA-04.1); arquivados fora salvo `?archived=true` (CA-08).
- `tracked_seconds` = bucket projeto do user (D-03); `due_today_count` no `tz` (D-04); `tz`
  inválido → `400` (CA-06); `tz` ausente → UTC.
- O front deriva `done_pct = round(done_count/task_count*100)` (0 quando `task_count=0` — CA-05).

### 4.3 Calendário do projeto (CA-09..15.1)

`GET .../projects/{projectId}/calendar?month=2025-06&tz=America/Sao_Paulo` → `200`:

```json
{ "from": "2025-06-01", "to": "2025-06-30",
  "items": [
    { "kind": "task", "id": "lh-t1", "name": "Fechar OKRs do trimestre",
      "date": "2025-06-25", "status": "doing" },
    { "kind": "task", "id": "lh-t2", "name": "Sprint review", "date": "2025-06-20",
      "status": "done" },
    { "kind": "objective", "id": "lh-o1", "name": "Roadmap Q3", "date": "2025-06-30",
      "status": "open" },                       // omitido se objectives.target_date não existe (CA-15.1)
    { "kind": "project_deadline", "id": "0d9e...", "name": "Acme",
      "date": "2025-06-30", "status": null }
  ] }
```

- Aceita `?month=YYYY-MM` OU `?from=&to=` (CA-11); malformado/`to<from` → `400`; default = mês
  corrente. `projectId` malformado/invisível/inexistente → `404` (CA-12).
- 3 queries fixas (D-05): deadline do projeto · tasks com due ∈ janela · objetivos com target_date
  ∈ janela (se a coluna existe). Visibilidade pelo gate (404 — CA-09/12).
- **Não** retorna eventos do GCal (CA-13) — o front busca `GET /api/v1/calendar/events?from&to&tz`
  (fase 06) com os mesmos `from`/`to` e sobrepõe.

### 4.4 Endpoints reusados (sem mudança nesta fase)

- `GET .../projects/{id}/board` (fase 12) — alimenta KPIs/Progress/Goals do Overview (ganha só o
  campo `deadline` no `project`).
- `GET .../reports?period=week&project_id={id}` (fase 14) — "This week" do Overview.
- `GET /api/v1/calendar/events?from&to&tz` + `GET /api/v1/calendar/status` (fase 06) — overlay do
  Calendar.
- Notas do projeto (fase 26, aditivo) — "Recent notes" do Overview (CA-21).

## 5. Backend — assinaturas

```go
// internal/projects (estende)
type Project struct {
    // ...campos da fase 02...
    Deadline *string `json:"deadline"`   // "YYYY-MM-DD" ou null (CA-01/02)
}
type CreateInput struct {
    // ...campos da fase 02...
    Deadline *string                     // nil = sem prazo
}
type PatchInput struct {
    // ...campos da fase 02...
    DeadlineSet bool; Deadline *string    // tri-state (CA-01: ausente mantém, null remove)
}
// validateDeadline(s *string) error — time.Parse("2006-01-02", *s) antes do SQL (CA-01.1)

// internal/projects/summary.go (NOVO)
type ProjectSummary struct {
    ID, Name, Color string
    ClientID *string `json:"client_id"`
    Deadline *string `json:"deadline"`
    TaskCount, DoneCount, OpenCount int
    TrackedSeconds int `json:"tracked_seconds"`
    DueTodayCount  int `json:"due_today_count"`
}
func (s *Service) Summary(ctx, ws, userID, tz string, archived bool) ([]ProjectSummary, error) // CA-04..08
//   3 queries fixas (D-02): projetos visíveis; contadores GROUP BY project_id; tempo GROUP BY project_id.
//   "today" = time.Now().In(loc).Format("2006-01-02"); loc de tz (UTC default), inválido → domain.Validation.

// internal/projects/calendar.go (NOVO)
type CalendarItem struct {
    Kind   string  `json:"kind"`   // "task" | "objective" | "project_deadline"
    ID     string  `json:"id"`
    Name   string  `json:"name"`
    Date   string  `json:"date"`   // YYYY-MM-DD
    Status *string `json:"status"` // task.status / objective.status / null p/ deadline
}
func (s *Service) ProjectCalendar(ctx, ws, userID, projectID, from, to string) ([]CalendarItem, error) // CA-09..15.1
//   GATE DE VISIBILIDADE (visibilidade→404) — chamar s.Get(ctx, ws, userID, projectID), que JÁ EXISTE
//   em internal/projects/service.go:219: faz uuid.Validate(projectID) (→ domain.NotFound, CA-12, nunca
//   500) E aplica visibleWhere `(visibility='shared' OR owner_user_id=$2)` → domain.NotFound 404 que
//   NÃO vaza existência (CA-09/12). É exatamente o gate de visibilidade de LEITURA necessário (read-only,
//   sem role). ⚠️ NÃO existe `loadProjectGate` no pacote `projects` — esse símbolo vive só em
//   internal/objectives/gate.go:37 (e em internal/tasks/service.go). NÃO chamar `loadProjectGate` aqui.
//   query (a) deadline do projeto ∈ [from,to]; (b) tasks WHERE project_id=$ AND due_date BETWEEN ...;
//   (c) objetivos WHERE project_id=$ AND target_date BETWEEN ...  SÓ se s.hasObjectiveTargetDate (D-05).

// s.hasObjectiveTargetDate bool — detectado 1× no NewService/boot via information_schema.columns (D-05/CA-15.1).
// resolveWindow(month, from, to, tz) (from, to string, err error) — ?month→1º..último dia (DST-safe), CA-11.
```

- **Validação (CA-01.1/06/11)** ANTES do SQL → `domain.Validation(...)`. `deadline`:
  `time.Parse("2006-01-02", *s)`; `tz`: `time.LoadLocation`; `month`: `time.Parse("2006-01",…)` +
  `AddDate(0,1,0).AddDate(0,0,-1)` pro último dia; `to<from` → 400.
- **Topologia de import (sem ciclo)**: `internal/projects` ganha `summary.go`/`calendar.go` no MESMO
  pacote (reusam `s.Get`/`getForWrite`/`visibleWhere` já internos do `projects.Service` — **NÃO**
  existe `loadProjectGate` no pacote `projects`; o gate de visibilidade de leitura do calendário é o
  `s.Get` da fase 02, ver §5). **NÃO** importa `objectives`, `tasks` nem `calendar` — as queries de
  task/objetivo do calendário são SQL inline (lê `tasks`/`objectives` por SQL, não pelos pacotes).
  Zero churn nos outros pacotes. (Caso queira evitar o SELECT extra do `s.Get`, alternativa
  sancionada: replicar a regra inline `visibility='shared' OR owner_user_id=$user` direto na query do
  calendário — auto-contida, igual `objectives/gate.go` faz; mas o caminho default é reusar `s.Get`.)
- **`hasObjectiveTargetDate`** (D-05): no `NewService`, `SELECT 1 FROM information_schema.columns
  WHERE table_name='objectives' AND column_name='target_date'` → bool cacheado. (Alternativa
  considerada e descartada: rodar a query (c) sempre e tratar o erro de coluna inexistente — frágil
  e gera erro de log; a detecção no boot é limpa.)

## 6. Frontend — telas e componentes

Tokens de `tokens.css` (fase 00) + classes do mockup (`.calgrid`/`.calcell`/`.caldow`/`.kpi`/
`.trow`/`.tt-card`/`.tt-goal`/`.chip`/`.eyebrow`). Reusa `lib/calendar-layout.ts` (fase 11),
`lib/palette.ts` (cor do projeto), `fmtH`/`dataBr` (formatação de horas/data já existentes nos
componentes do `/hoje`/relatórios), `statusIcon`/`prioIcon` (fase 24).

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Card vivo | `components/projects/ProjectCard.tsx` (reescreve o da fase 02 — caminho REAL; **não** `components/cards/`, que não existe; o import vivo está em `routes/cockpit/Projects.tsx:8`) | nome + barra de cor; `open_count` pendentes · `task_count` total; cliente; `fmtH(tracked_seconds)` (NÃO `0h`); `deadline` (quando há); dot "hoje" quando `due_today_count>0`. Link pro detalhe (CA-16) |
| Lista de Projetos | `routes/cockpit/Projects.tsx` (estende) | troca `useProjects` por `useProjectsSummary` (1 request — CA-04/16) |
| Tab Overview | `components/project/OverviewTab.tsx` (NOVO) | KPIs (board + "This week" do relatório) · Upcoming (tasks due futuras, clicáveis → `openTask` — RF-23) · Progress por status + Goals (do board) · Recent notes (fase 26, aditivo — CA-17/21) |
| Tab Calendar | `components/project/ProjectCalendarTab.tsx` (NOVO) | grade do mês (`monthGridDays` da fase 11) · chips dos itens (do `?month=`) na cor do projeto (`border-left`) · overlay GCal via `useCalendarEvents(from,to,tz)` com a janela da **grade** (`monthRange(refDate)` da fase 11), NÃO o `from`/`to` estrito do endpoint do projeto (D-07 — cobre os dias de spillover) · ‹ Hoje › navega mês · clicar item task/obj → abre; evento → `html_link` (CA-18) |
| ProjectDetail (shell) | `routes/cockpit/ProjectDetail.tsx` (estende 12/24) | tab-shell ganha Overview + Calendar; default Overview (CA-19) |
| API + hooks | `lib/api.ts` (estende), `lib/queries/projects.ts` (+useProjectsSummary/useProjectCalendar) | `deadline` em Project/Input; tipos Summary/CalendarItem; patch de project invalida `projectsSummaryKey` + `boardKey` |

- **Gating viewer (CA-20):** Overview/Calendar leem `board.project.can_write` (do `/board`, fase
  12) pra esconder editar-deadline/ações de escrita — não a role do ws sozinha. Padrão de teste
  `viewer-gating.spec` (fase 02/12).
- **Overlay GCal alinhado à GRADE (não ao mês):** o `ProjectCalendarTab` chama
  `useCalendarEvents(from, to, tz)` com a janela da **grade de 42 dias** — `monthRange(refDate)` da
  fase 11 (`lib/calendar-layout.ts:267` → `{from: primeira célula, to: última célula}`), NÃO o
  `from`/`to` estrito do mês que o endpoint do projeto devolve (D-07). Assim os eventos do GCal nos
  dias de *spillover* (que a grade mostra) também aparecem — igual à fase 11. Os **itens do projeto**
  usam o `?month=` (ficam só nos dias do próprio mês). Se `/calendar/status` = desconectado, o overlay
  some (degrada — CA-13).
- **À prova de culpa (RNF-02):** Upcoming/Calendar não pintam prazo passado de vermelho acusatório;
  vazio = "No deadlines set." (mockup). Prazo de hoje pode ser destacado no accent (não no
  `--danger`).

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** chamar `/board` N vezes pra montar os cards vivos (RNF-04/D-02) — usar o `summary` (1
  request, 3 queries).
- **NÃO** somar `tracked_seconds` do card por `task_id ∈ projeto` (D-03) — é o bucket **projeto**
  (`WHERE project_id=$`), pra bater com o `/board`. (O bucket por-task pode divergir — D-06 da 12.)
- **NÃO** somar tempo de **outros usuários** no summary/calendar (CA-09) — filtro `user_id` da
  sessão em toda soma.
- **NÃO** computar `due_today` em UTC fixo (D-04/CA-06) — usar o `tz` da requisição; `tz` inválido
  → `400`, nunca somar errado.
- **NÃO** misturar eventos do GCal no endpoint do projeto (D-06/CA-13) — o overlay é separado, sem
  vínculo a projeto (limitação herdada honesta).
- **NÃO** rodar a query de `objectives.target_date` sem checar se a coluna existe (D-05/CA-15.1) —
  `hasObjectiveTargetDate` detectado no boot; coluna ausente ⇒ pular a query (build order 28 antes
  de 29 → o `kind:objective` nasce omitido, sem 500).
- **NÃO** importar `objectives`/`tasks`/`calendar` no pacote `projects` (§5) — queries inline; sem
  ciclo.
- **NÃO** deixar o `deadline` estourar o cast do Postgres (CA-01.1) — `time.Parse` ANTES do SQL →
  `400`, nunca `500`.
- **NÃO** vazar projeto invisível no summary/calendar (CA-04.1/09/12) — `visibility='shared' OR
  owner_user_id=$user` em TODA query; no calendário, o gate é `s.Get` da fase 02 (visibilidade→404,
  service.go:219) — **NÃO** `loadProjectGate` (esse símbolo não existe no pacote `projects`).
- **NÃO** errar o último dia do mês em DST (CA-11) — `AddDate(0,1,0).AddDate(0,0,-1)`, NUNCA
  `+30 dias` ou `(end-start)/24h` (furo da fase 14).
- **NÃO** duplicar a matemática de grade do mês no front (D-08) — reusar `lib/calendar-layout.ts`
  da fase 11 (testada em 3 fusos).
- **NÃO** buscar o overlay do GCal com o `from`/`to` ESTRITO do mês que o endpoint do projeto
  devolve (D-07/CA-18) — usar `monthRange(refDate)` da fase 11 (janela dos 42 dias da grade), senão
  os eventos do GCal nos dias de *spillover* da grade somem (diverge da fase 11). Os itens do projeto
  ficam no `?month=`; só o overlay usa a janela maior.
- **NÃO** reescrever o CRUD de projeto da fase 02 (D-01) — só ADICIONAR `deadline` (tri-state) ao
  `CreateInput`/`PatchInput`/`Project`/leituras; gate intocado.
- **NÃO** persistir aba selecionada (CA-19) — default Overview; resetar ao trocar de projeto
  (mockup `go()`).
- **NÃO** criar tabela `notes` nem UI de notas (é a fase 26) — só consumir, aditivo (CA-21).
- **NÃO** introduzir lib de calendário/grid nova (RNF-03) — reusar fase 11 + classes do mockup.
- **NÃO** adicionar write-back/edição de item no calendário (CA-14) — read-only.

## 8. Ordem de execução (ver tasks.md)

Backend primeiro (migração → deadline em projects → summary → calendar → handlers/wiring), depois
o frontend (api/hooks → ProjectCard vivo + Projects → OverviewTab → ProjectCalendarTab → shell),
depois E2E e gate. Commit por tarefa. `make check-full` é o gate final.

## 9. Dependências & paralelização

- **Depende de (build order 24 → 26 → 27 → 28 → 29):**
  - **fase 24** (tab-shell, `openTask`, `statusIcon`/`prioIcon`, status `backlog`) — **dura**: o
    Overview/Calendar penduram no shell e usam `openTask`. A 28 vem DEPOIS da 24.
  - **fase 12** (`/board`, `objectives`, `done_pct`) — **dura** (já em produção): KPIs/Progress/Goals
    do Overview.
  - **fase 06** (`/calendar/events`/`status`, `calendarstub`) — **dura** (já em produção): overlay.
  - **fase 11** (`lib/calendar-layout.ts`) — **dura** (já em produção): grade do mês no front.
  - **fase 14** (`/reports?period=week&project_id`) — **dura** (já em produção): "This week".
  - **fase 26** (notas) — **FRACA/aditiva**: "Recent notes" omitido se ausente (CA-21). A 28 não
    bloqueia se a 26 não estiver no build (mas a ordem sugerida coloca a 26 antes).
  - **fase 29** (`objectives.target_date`) — **FRACA/aditiva**: `kind:"objective"` omitido se a
    coluna não existe (CA-15.1). A 28 vem ANTES da 29 → no deploy desta fase o `kind:objective`
    nasce **off** e liga quando a 29 deployar.
- **Paralelização:** a fase 28 **não é worktree-paralelizável** com a 24/26 (depende de artefatos
  delas no front — `ProjectDetail` shell, `openTask`, notas). Migração `0025` é independente das
  outras migrações da M3 (`0021..0024`, `0026`) — sem contenção de número se construída em ordem.
  Backend (summary/calendar) e frontend desta fase **dentro** da fase podem ser fatiados, mas o
  build TDD é sequencial e stateful (cada tarefa edita arquivos compartilhados — `projects/service.go`,
  `api/projects_handlers.go`, `Projects.tsx`/`ProjectDetail.tsx`) → **não** paralelizar por workflow
  ingenuamente (conflito de arquivo). Se o contexto apertar, delegar a fase inteira a 1
  agente-executor (contexto isolado).
