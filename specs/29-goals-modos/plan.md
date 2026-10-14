# plan.md — Fase 29 · Goals (objetivos) com modos de conclusão

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> assinaturas + ordem de tarefas. **Estende a fase 12** (`objectives`, `/board`, recálculo) — não a
> reescreve. Herda o baseline (`SDD.md §7`), a cadeia de middlewares (RequireAuth → RequireMember →
> RequireWriteRole) e o gate auto-contido do pacote `objectives`. Nada aqui é código — é o contrato.

---

## 1. Arquitetura geral

```
Backend (estende a fase 12; SEM pacote novo):
  migrations/0026_objectives_modes.sql           ← ALTER objectives ADD completion_mode + target_date (CA-01)
  internal/objectives/service.go (existe)         ← + CompletionMode/TargetDate em Objective, CreateInput,
                                                     PatchInput (tri-state TargetDateSet); validModo; persistência
  internal/objectives/board.go (existe)           ← + completion_mode/target_date em BoardObjective (CA-10) +
                                                     progress_pct opcional coerente com o modo (CA-10.1)
  internal/tasks/objective_link.go (existe)       ← recalcObjectiveStatus passa a ramificar por completion_mode
                                                     (by_tasks intocado; by_hours por horas; by_date no-op) — CA-05/06/07
  internal/api/objectives_handlers.go (existe)    ← completion_mode/target_date nos handlers Create/Patch
                                                     (target_date tri-state via json.RawMessage — CA-03.1)
  internal/reports/objectives.go (existe, fase 14)← deriva "concluído by_hours" por modo (tracked>=meta),
                                                     não só do status cru — CA-06.3 / 29-H7 (cross-fase)

Frontend (aba Goals rica + New goal com modo):
  lib/api.ts (existe)                             ← + completion_mode/target_date em Objective/Board/Input/Patch
  lib/queries/objectives.ts (existe)              ← hooks intactos (já invalidam boardKey)
  routes/cockpit/tabs/GoalsTab.tsx       (NOVO)   ← aba Goals: visão lista Notion (CA-12) consumindo o /board
  components/board/GoalRow.tsx           (NOVO)   ← linha da tabela: nome, barra por modo, tasks, horas, status
  components/board/GoalModal.tsx         (NOVO)   ← New/Edit goal: seletor de modo + campos por modo (CA-13)
  components/board/ObjectiveCard.tsx (existe, 12) ← reforço do feedback de conclusão (CA-14) + badge de modo
                                                     + barra por MODO (progress_pct, não done_pct) — CA-12.2
  routes/cockpit/tabs/OverviewTab.tsx (fase 28)   ← lista de Goals deriva a barra por MODO (CA-12.2, cross-fase)
  components/reports/ObjectivesSection.tsx (14)   ← deriva "concluído by_hours" por modo (CA-06.3, cross-fase)
  components/reports/ReportTable.tsx (14)         ← idem (CA-06.3, cross-fase)
```

> **A aba Goals pendura no tab-shell da fase 24** (que introduz Overview·Tasks·Notes·Goals·Calendar). A
> fase 29 entrega o **conteúdo** da aba Goals. Se a 24 já criou um placeholder `GoalsTab`, esta fase o
> preenche. (A dependência 24→…→29 é dura no front; ver §9.)

## 2. Migração — `0026_objectives_modes.sql` (dona: fase 29)

```sql
-- 0026_objectives_modes — fase 29 (plan §2): Goals com modos de conclusão.
-- by_tasks = comportamento da fase 12 (% tasks done, auto-close a 100%) — DEFAULT,
-- preserva todos os objetivos existentes. by_hours = tracked_seconds vs goal_hours.
-- by_date = target_date é marco/milestone (fecho manual; alimenta o calendário, fase 28).
-- Migração ADITIVA: coluna com DEFAULT + coluna nullable. ALTER TABLE ADD COLUMN roda
-- na própria transação do runner (não é ALTER TYPE ADD VALUE — sem a restrição do §9.3
-- do PRD; não há enum novo nesta fase).
ALTER TABLE objectives
  ADD COLUMN completion_mode text NOT NULL DEFAULT 'by_tasks'
    CHECK (completion_mode IN ('by_tasks','by_hours','by_date')),
  ADD COLUMN target_date date;
```

- Numeração: a M3 começa em `0021` (PRD §4); as fases 24→29 consomem `0021`..`0026`. A fase 29 é a
  **última** → `0026` (FIXO; não "ler o próximo livre" — sob worktrees colidiria). INDEX.md: registrar
  `0026_objectives_modes` dona da fase 29.
- **Sem enum novo** (`completion_mode` é `text` + CHECK, não um Postgres ENUM) → evita a armadilha do
  `ALTER TYPE … ADD VALUE` (§9.3 do PRD; o runner aplica cada `.sql` em UMA transação — `ADD VALUE`
  usado no mesmo arquivo quebraria, mas aqui não há `ADD VALUE`). `text`+CHECK é o padrão da fase 22/M3
  pra estados pequenos sem ENUM.
- `target_date date` nullable: NULL = sem marco. Igual `due_date` de task (fase 02) e `deadline` de
  projeto (fase 28). Validação `YYYY-MM-DD` no service ANTES do SQL (D-03).
- **Backstop:** o CHECK do `completion_mode` garante integridade no banco; a validação no service
  (D-03) garante `400 validation_error` em vez de `500` por violação de CHECK (CA-02.1).

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **SEM pacote novo — estende `internal/objectives` e `internal/tasks`** | O modo de conclusão é um atributo do objetivo + uma ramificação no recálculo que JÁ existe (`objective_link.go`). Criar pacote seria churn. O `objectives.Service` (CRUD/board) e o `recalcObjectiveStatus` inline no `tasks` (sem ciclo de import — D-03 da fase 12) ganham os campos/ramos. `tasks` continua **não** importando `objectives` (o recálculo lê `objectives.completion_mode`/`goal_hours` por SQL inline). |
| D-02 | **Recálculo ramifica por `completion_mode` DENTRO de `recalcObjectiveStatus`** (na transação da mutação de task, disparado pelos MESMOS gatilhos da fase 12 — D-04 da 12) | Mantém o ponto de auto-close único e atômico (`FOR UPDATE` da fase 12 permanece). A query lê `completion_mode`/`goal_hours` do objetivo na mesma trava e decide: `by_tasks` → regra atual (CA-05, byte-equivalente); `by_hours` → compara `Σ duration_seconds das tasks do objetivo` com `goal_hours×3600` (CA-06); `by_date` → no-op (CA-07). Idempotente como o original. |
| D-03 | **Validação na mão (stdlib)**, igual fase 12: `validCompletionMode` (switch nos 3 valores), `validTargetDate` (parse `2006-01-02`), ANTES do SQL | Leveza (RNF-01); evita `500` por CHECK/cast (CA-02.1/03). Espelha `validStatus`/`validGoal` da fase 12. |
| D-04 | **Auto-close por horas é recalculado nos gatilhos de TASK, não no fechamento de entry** | O recálculo da fase 12 só roda em mutação de task (D-04 da 12). Fechar uma entry (cronômetro) muda `tracked_seconds` mas **não** é mutação de task — então o `by_hours` **não** auto-fecha no instante do stop. **Convergência condicionada a uma mutação de task futura (CA-06.2, 29-H7):** se NENHUMA task do objetivo sofrer mutação depois de a meta ser batida, o `status` persistido pode NUNCA virar `done` por conta própria; carregar o `/board`/Goals **NÃO** converge o `status` (read-only). **Como a UI não mente mesmo assim:** a aba Goals/Overview deriva "atingiu a meta" do `tracked_seconds`/`progress_pct` cru do `/board` (CA-12, per-usuário) e os consumidores do `status` persistido (Relatórios, fase 14) derivam "concluído by_hours" por modo (CA-06.3) — nenhuma UI mostra um by_hours da meta-batida como "Open/atrasado". Amarrar o recálculo ao `timer.OnChange`/stop é o **elo timer↔task** (§9.2 do PRD), evolução futura — fora desta fase. **Razão de não fazer agora:** mexeria no `timer.Service`/`OnChange` (RNF-01 do PRD: não regredir o TRACKING) e exigiria resolver "qual objetivo a entry alimenta" no caminho quente do timer. |
| D-05 | **`/board` expõe `completion_mode` + `target_date` (obrigatório, aditivo) e `progress_pct` (OPCIONAL, derivado)** | Os crus (`completion_mode`, `target_date`, `tracked_seconds`, `goal_hours`, `done_pct`) já bastam pro client derivar a barra por modo (CA-12.1). `progress_pct` coerente com o modo é uma **comodidade** (CA-10.1): se incluído, clampar `[0,100]` e nunca dividir por zero; se não, o client deriva. **Decisão FIXA: INCLUIR** o `progress_pct` no board (uma linha de cálculo em memória, sem query nova) pra a aba Goals e o Overview não duplicarem a lógica de modo. Campos da fase 12 **byte-idênticos** (RNF-05). |
| D-06 | **`target_date` tri-state no PATCH** (`json.RawMessage`), igual `goal_hours` da fase 12 | A spec exige "ausente mantém, null limpa, valor define" (CA-03.1). Reusa o padrão exato do `patchObjectiveBody.GoalHours` (fase 12, `objectives_handlers.go`). `completion_mode` é ponteiro simples (sem semântica de null — trocar modo é sempre um valor válido). |
| D-07 | **`completion_mode` ausente no POST = `'by_tasks'`** (default no service, espelhando o DEFAULT da coluna) | Preserva o cliente da fase 12 (que nunca manda `completion_mode`): cria objetivo `by_tasks`, comportamento idêntico (CA-02). Zero regressão no `ObjectiveModal` da fase 12 se ele não for tocado. |
| D-08 | **Front: aba Goals é uma visão LISTA do `/board`** (não um endpoint próprio) | O `/board` já traz todos os objetivos com progresso + os campos novos (CA-10). A `GoalsTab` consome `useBoard(ws, projectId)` (mesma query do board da fase 12) e renderiza a tabela Notion (`renderMetas` do mockup). Zero query nova no front (RNF-05). |
| D-09 | **Gating viewer pela `board.project.can_write`** (CA-15), não pela role do ws | Padrão da fase 12 (CA-22): um `access=view` num projeto `shared` é `can_write=false`. A `GoalsTab`/`GoalModal`/`GoalRow` leem `board.project.can_write`. |
| D-10 | **Dopamina (CA-14) reforça o `ObjectiveCard`/linha existente** — sem lib de animação | O sinal de "concluído" da fase 12 já existe; esta fase adiciona um feedback visual curto (CSS transition/check no accent) ao virar `done`. Nada de lib (RNF-01); à prova de culpa (RNF-03 — sem vermelho). |
| D-11 | **A barra do `ObjectiveCard` (board↔lista, fase 12) e da Overview (fase 28) passa a derivar por MODO** (`progress_pct`/`goalProgress`), não `done_pct` (CA-12.2, 29-H6) | Sem isso, um `by_hours` fechado por horas (0 tasks done) mostraria barra de tasks a 0% sob um chip "Concluído" — inconsistente com a aba Goals e fere RNF-03. Como `progress_pct` por-modo já está no contrato do board (D-05, com o propósito de servir Overview E aba Goals) e no `by_tasks` `progress_pct==done_pct`, é só trocar a fonte da barra — **zero risco de regressão no by_tasks**, zero query nova. |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) — idênticos à fase 12. **As rotas da fase 12 são REUSADAS sem mudar de
shape** (só o payload ganha campos).

### 4.1 Objetivos — campos novos (aditivos, RNF-05)

`POST .../projects/{projectId}/objectives`:

```json
// → request (completion_mode/target_date opcionais; default by_tasks)
{ "name": "Bater 40h no onboarding", "completion_mode": "by_hours", "goal_hours": 40 }

// ← 201  (objeto da fase 12 + completion_mode + target_date)
{ "id": "ob1...", "workspace_id": "8c...", "project_id": "0d9e...",
  "name": "Bater 40h no onboarding", "description": "", "status": "open",
  "goal_hours": 40, "completion_mode": "by_hours", "target_date": null,
  "position": 2, "created_by": "me0...", "created_at": "2026-06-22T14:00:00Z" }

// → request (marco por data)
{ "name": "Demo pro cliente", "completion_mode": "by_date", "target_date": "2026-07-15" }

// ← 400 modo inválido (CA-02.1)
{ "error": { "code": "validation_error", "message": "completion_mode deve ser by_tasks, by_hours ou by_date" } }

// ← 400 target_date malformado (CA-03)
{ "error": { "code": "validation_error", "message": "target_date deve estar no formato YYYY-MM-DD" } }
```

`PATCH .../objectives/{objectiveId}` aceita o subconjunto da fase 12
(`{name, description, goal_hours, status, position}`) **+** `completion_mode` (ponteiro; ausente
mantém) **+** `target_date` (**tri-state** via `json.RawMessage`: ausente mantém, `null` limpa, valor
define — CA-03.1). `goal_hours` segue tri-state (fase 12). Trocar `completion_mode` não toca
`goal_hours`/`target_date` salvo se vierem no payload (CA-04).

`GET .../objectives` → `200 { "objectives": [ {…} ] }` (cada objetivo agora com `completion_mode`/
`target_date`; ordem por `position` — CA-03 da fase 12 intocada).

### 4.2 Board (read agregado — aditivo, CA-10)

`GET .../projects/{projectId}/board` → `200` (estrutura da fase 12 + 3 campos por objetivo):

```json
{
  "project": { "id": "...", "name": "...", "color": "#4f9dde",
               "weekly_hours_goal": 18, "archived_at": null,
               "tracked_seconds": 54000, "can_write": true },
  "objectives": [
    { "id": "ob1...", "name": "Bater 40h no onboarding", "description": "...",
      "status": "open", "goal_hours": 40, "position": 0,
      "completion_mode": "by_hours", "target_date": null,        // ← NOVO (CA-10)
      "task_count": 3, "done_count": 1, "done_pct": 33,
      "tracked_seconds": 90000, "progress_pct": 62,              // ← NOVO opcional (CA-10.1): 90000/(40*3600)=62%
      "tasks": [ { /* BoardTask da fase 12 */ } ] }
  ],
  "general": { "task_count": 2, "done_count": 0, "tracked_seconds": 1200, "tasks": [ ... ] }
}
```

- `completion_mode` (`by_tasks|by_hours|by_date`) e `target_date` (`YYYY-MM-DD`|null) são **aditivos**.
- `progress_pct` (FIXO incluir — D-05): `by_tasks` → `done_pct`; `by_hours` → `round(tracked_seconds /
  (goal_hours×3600) × 100)` clampado `[0,100]`, `0` se `goal_hours` nulo; `by_date` → `done_pct`. Nunca
  divide por zero, sempre `[0,100]`.
- Nenhum campo existente muda (RNF-05). Mesma **3 queries fixas** da fase 12 (D-05 da 12) — só +2
  colunas scaneadas no SELECT de objetivos + cálculo em memória do `progress_pct`. Sem N+1, `< 200ms`.

## 5. Backend — assinaturas

```go
// internal/objectives — campos novos no Objective e nos inputs (estende a fase 12).
type Objective struct {
    ID, WorkspaceID, ProjectID, Name, Description, Status string
    GoalHours      *float64 `json:"goal_hours"`
    CompletionMode string   `json:"completion_mode"`   // NOVO: by_tasks|by_hours|by_date
    TargetDate     *string  `json:"target_date"`       // NOVO: YYYY-MM-DD ou null
    Position       int
    CreatedBy      string
    CreatedAt      time.Time
}
type CreateInput struct {
    Name, Description string
    GoalHours      *float64
    CompletionMode string   // "" → by_tasks (default, D-07)
    TargetDate     *string  // nil → sem marco
}
type PatchInput struct {                  // estende o da fase 12
    Name, Description, Status *string
    GoalSet bool; GoalHours *float64      // tri-state (fase 12)
    PositionSet bool; Position *int       // (fase 12)
    CompletionMode *string                // NOVO: ausente mantém
    TargetDateSet bool; TargetDate *string // NOVO: tri-state (ausente mantém, nil-com-Set limpa) — CA-03.1
}

// validação (D-03, ANTES do SQL):
func validCompletionMode(m string) error  // switch by_tasks/by_hours/by_date; senão domain.Validation (CA-02.1)
func validTargetDate(d *string) error      // nil ok; parse "2006-01-02"; senão domain.Validation (CA-03)

// Create/Patch ganham os campos; a query de scan/insert/update inclui completion_mode + target_date.
// objectiveCols += ", completion_mode, to_char(target_date,'YYYY-MM-DD')" (scan p/ *string).
// O loadProjectGate/getInProject/visibilidade da fase 12 ficam INTOCADOS (CA-09/09.1).

// internal/objectives/board.go — BoardObjective ganha 3 campos:
type BoardObjective struct {
    /* ... campos da fase 12 ... */
    CompletionMode string  `json:"completion_mode"`  // NOVO (CA-10)
    TargetDate     *string `json:"target_date"`      // NOVO (CA-10)
    ProgressPct    int     `json:"progress_pct"`     // NOVO (CA-10.1, D-05)
}
// O SELECT de objetivos do Board passa a trazer completion_mode/target_date; após montar
// task_count/done_count/done_pct/tracked_seconds, calcula progress_pct por modo (helper puro
// progressByMode(mode, donePct, trackedSeconds, goalHours) — clampa [0,100], 0 se goalHours nulo).

// internal/tasks/objective_link.go — recalcObjectiveStatus ramifica por completion_mode (D-02):
//   trava a linha (FOR UPDATE — fase 12) e lê completion_mode, goal_hours na MESMA query:
//     SELECT completion_mode, goal_hours,
//            count(t.*) AS n, count(t.*) FILTER (WHERE t.status='done') AS d,
//            COALESCE(SUM(e.duration_seconds) FILTER (WHERE e.duration_seconds IS NOT NULL),0) AS secs
//       FROM objectives o
//       LEFT JOIN tasks t ON t.objective_id = o.id
//       LEFT JOIN time_entries e ON e.task_id = t.id AND e.user_id = $2  -- horas POR-USUÁRIO (igual o board, decisão E)
//      WHERE o.id = $1 GROUP BY o.completion_mode, o.goal_hours
//   (o `secs` filtra e.user_id = <user da mutação> — MESMO bucket per-usuário do /board, board.go:106)
//   ramos:
//     by_tasks → regra da fase 12 (n>=1; d==n & open→done; d<n & done→open; senão no-op)  -- CA-05
//     by_hours → goal_hours não-nulo & secs >= goal_hours*3600 & open → done;
//                goal_hours não-nulo & secs <  goal_hours*3600 & done → open;
//                goal_hours nulo → no-op (CA-06.1); senão no-op                            -- CA-06
//     by_date  → no-op SEMPRE (CA-07)
//   idempotente; preserva fecho manual (CA-08) salvo quando o limiar do modo cruza.
```

- **Atenção (CA-05 — não regredir):** o ramo `by_tasks` DEVE produzir o **mesmo** resultado que o
  `UPDATE` atual da fase 12. A reescrita com `LEFT JOIN time_entries` muda a forma da query (o `count`
  precisa ser `count(t.id)` ou `count(*) FILTER (WHERE t.id IS NOT NULL)` pra não contar a linha do
  `LEFT JOIN` quando o objetivo tem 0 tasks → `n=0`, e o `SUM` precisa de `DISTINCT`/agregação correta
  pra horas se uma task tiver N entries — **cuidado com fan-out do join**: `SUM(e.duration_seconds)` num
  `tasks ⋈ time_entries` está correto (cada entry conta 1×), mas `count(t.id)`/`count(... FILTER status)`
  **inflam** se uma task tem N entries (a task aparece N vezes). **Solução FIXA (D-02):** calcular `n`/`d`
  numa subquery sobre `tasks` (sem o join de entries) e `secs` numa subquery sobre `time_entries ⋈ tasks
  do objetivo **filtrando `e.user_id = <user da mutação>`** (decisão E — per-usuário, igual o board) —
  **duas subqueries**, não um join único, pra evitar o fan-out. Ver T-04.
- **Topologia de import (sem ciclo, herdada da fase 12):** `tasks` lê `objectives.completion_mode`/
  `goal_hours` por SQL inline (já lê `objectives` por SQL em `validateObjectiveRef`); **não** importa o
  pacote `objectives`. `objectives` não importa `tasks`. Byte-intocados salvo os campos novos.

## 6. Frontend — telas e componentes

Tokens de `tokens.css`. Reusa `.tt-goal`, `.tt-card`, `StatusPill`/chip, paleta `lib/palette.ts`,
formatação de horas (`fmtH` do mockup ≈ helper de horas das fases 13/14). A aba Goals pendura no
tab-shell da fase 24.

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Aba Goals | `routes/cockpit/tabs/GoalsTab.tsx` | consome `useBoard(ws, projectId)`; toolbar (texto + **New goal** gated por `can_write`); visão lista Notion (`renderMetas`); vazio → "No goals yet." (mockup linha 470) |
| Linha de goal | `components/board/GoalRow.tsx` | colunas Goal · Progresso · Tasks · Horas · Status (mockup 473–479); barra `.tt-goal` por modo (CA-12.1, função pura `goalProgress(obj)`); chip Done (accent) / Open |
| Modal de goal | `components/board/GoalModal.tsx` | New/Edit: **seletor de modo** (by_tasks/by_hours/by_date) + campos por modo (nome sempre; goal_hours em by_hours; target_date em by_date); validação client antes do POST/PATCH (CA-13); PATCH só com o diff |
| Reforço de conclusão + barra por modo | `components/board/ObjectiveCard.tsx` (existe, fase 12) | feedback visual de `done` (accent + check, sem vermelho — CA-14/RNF-03) + badge discreto do modo; **a barra do card deriva de `progress_pct`/`goalProgress` por MODO (CA-12.2), não `done_pct`** — proíbe chip "Concluído" sobre barra 0% num by_hours fechado por horas |
| Barra por modo na Overview | `routes/cockpit/tabs/OverviewTab.tsx`/`ProjectOverview` (fase 28) | a lista de Goals da Overview consome `progress_pct` por-modo do board (CA-12.2) em vez de `done_pct` — coerente com a aba Goals; troca trivial, zero regressão no by_tasks (`progress_pct==done_pct`) |
| API + tipos | `lib/api.ts` (estende) | `completion_mode`/`target_date` em `Objective`, `BoardObjective` (+`progress_pct`), `ObjectiveInput`, `ObjectivePatch` |

- `useBoard` segue a query central (fase 12); `useCreateObjective`/`usePatchObjective`/
  `useDeleteObjective` (fase 12) já invalidam `boardKey` → a aba Goals reflete sem reload (CA-13). **NÃO
  mexer em `lib/queries/objectives.ts`** além do que já existe (aditivo só nos tipos da `api.ts`).
- **Barra por modo (CA-12.1, função PURA testável `goalProgress`):** `by_tasks` → `done_pct`;
  `by_hours` → `progress_pct` do board (ou derivado: `goal_hours ? clamp(round(tracked/(goal_hours*3600)
  *100),0,100) : 0`); `by_date` → `done_pct` informativo. `>=100%` → `var(--accent)`; senão cor do
  projeto. Sem meta em `by_hours` → barra 0, sem `/Xh` na coluna Horas. **Nunca vermelho** (RNF-03).
- **Seletor de modo (GoalModal):** 3 opções; ao escolher `by_hours` mostra o campo `goal_hours`; ao
  escolher `by_date` mostra o date-picker de `target_date`; `by_tasks` esconde os dois. Validação:
  nome não-vazio sempre; `goal_hours` em `(0,9999]` se `by_hours`; `target_date` `YYYY-MM-DD` se
  `by_date`. Inválido → não chama API, sinaliza (CA-13).
- Gating viewer (CA-15): tudo via `board.project.can_write` (D-09).
- Mobile (RNF-06): a tabela Notion vira cartões `< md` (padrão `ReportTable`→cartões da fase 22).

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** regredir o ramo `by_tasks` do recálculo (CA-05/RNF-05) — DEVE ser byte-equivalente à fase 12;
  teste de regressão obrigatório (a suíte da fase 12, incl. `objective_status_test.go` e
  `TestAutoStatusConcorrente`, continua verde sem mudança de asserção).
- **CUIDADO com o fan-out do join `tasks ⋈ time_entries`** (D-02/§5) — `count(t.id)`/`count FILTER
  status` inflam se uma task tem N entries. Usar **duas subqueries** (`n`/`d` sobre `tasks`; `secs`
  sobre `time_entries ⋈ tasks do objetivo`), não um join único. Senão `by_tasks` e `by_hours` mentem.
- **NÃO** auto-fechar `by_date` por nada automático (CA-07) — marco fecha SÓ por `PATCH status` manual.
  Marcar tasks `done` num `by_date` NÃO o fecha.
- **NÃO** auto-fechar `by_hours` sem `goal_hours` (CA-06.1) — sem meta não há limiar; no-op.
- **NÃO** somar horas de **outros usuários** no `by_hours` (CA-09.2) — o `tracked_seconds` do objetivo já
  filtra `user_id` da sessão no board; o recálculo de horas no `recalcObjectiveStatus` também DEVE
  filtrar `e.user_id` (o tempo do objetivo é por-usuário, igual o board da fase 12). **Atenção:** o
  recálculo roda na transação da mutação de task, com o `user_id` de quem fez a mutação. **Decisão FIXA
  (coordenação cross-fase E):** o recálculo `by_hours` SOMA SÓ as entries do `user_id` da sessão (a
  mesma que fez a mutação de task), **MESMO bucket per-usuário do `/board`** (CA-16 da 12,
  `board.go:90/106` — `WHERE e.user_id = $2`). **Coerência total:** o `status` persistido (all-users
  seria incoerente com o que a UI mostra) e o `progress_pct` renderizado (per-user, vindo do board)
  usam o MESMO total. O `tracked` do recálculo `by_hours` **FILTRA `e.user_id`** — idêntico ao board.
  Isto alinha com o single-user-first do CLAUDE.md (dogfooding solo) e zera a incoerência "chip Done
  sobre barra <100%". **Soma all-users (esforço total da entrega entre membros) = FUTURO/multiplayer**
  (§9 do PRD), fora desta fase. Teste pina o filtro `e.user_id` (mutação que o REMOVE quebra).
- **NÃO** deixar `completion_mode`/`target_date` estourar o CHECK/cast pra `500` (CA-02.1/03) — validar
  no service ANTES do SQL.
- **NÃO** dividir por zero no `progress_pct` (CA-10.1) — `goal_hours` nulo → 0; clampar `[0,100]`.
- **NÃO** mudar o shape dos campos da fase 12 no `/board`/objetivo (RNF-05) — só ADICIONAR campos.
- **NÃO** tirar o `FOR UPDATE` do recálculo (fase 12) — a serialização por objetivo permanece (corrida
  do CA-06 da 12 já corrigida com ele).
- **NÃO** amarrar o auto-close por horas ao fechamento de entry (CA-06.2/§6 fora-de-escopo) — fica no
  gatilho de task; é o elo timer↔task (§9.2 do PRD), fase futura.
- **NÃO** marcar marco vencido como erro/vermelho (RNF-03) — `by_date` vencido aberto = **Open**
  discreto.
- **NÃO** unificar Goals (objetivos do projeto) com sub-tasks "goals da task" (`task_subtasks` da fase
  24) — níveis diferentes (§3.5 do PRD).
- **NÃO** criar ciclo de import `tasks ↔ objectives` — recálculo lê `objectives` por SQL inline.

## 8. Dependências & paralelização

- **Depende de (dura):**
  1. **Fase 12** (`objectives.Service`, `/board`, `recalcObjectiveStatus`) — esta fase **estende** esses
     três artefatos. Eles existem em produção (M1 deployada).
  2. **Fase 24** (tab-shell Overview·Tasks·Notes·Goals·Calendar) — a aba **Goals** pendura no shell da
     24. Se a 24 criou um placeholder `GoalsTab`, a 29 o preenche; senão a 29 cria o arquivo e a 24/28
     já o linka. **Dependência de front dura** (a 29 não tem onde montar a aba sem o shell).
  3. **Fase 28** (Overview + calendário do projeto) — **consumidora** de `target_date` (CA-11): o
     calendário/Upcoming da 28 lê o `target_date` que a 29 grava. A 29 só **garante o dado** no
     contrato; a renderização é teste da 28. **O agregador de calendário da 28 DEVE gatear marcos por
     `completion_mode='by_date'`** (`WHERE … AND completion_mode='by_date' AND target_date BETWEEN …` —
     29-H8): um `by_tasks`/`by_hours` com `target_date` residual (marco dormente, CA-04.1 da 29) NÃO
     vira marco. **A Overview da 28 (lista de Goals) DEVE derivar a barra/% por MODO** (consumir o
     `progress_pct` por-modo do board — CA-10.1, ou `goalProgress`), não `done_pct` — CA-12.2 da 29.
     (Estas duas exigências são do contrato da 28; registradas aqui pra a spec da 28 referenciar `0026`
     como dona de `target_date`/`completion_mode`.) Build order do PRD: 24→26→27→28→**29** — a 28 vem
     ANTES da 29 no front, mas a 28 só **lê** `objectives.target_date`/`completion_mode` (que a 29 cria).
     **Resolução:** o endpoint
     de calendário da 28 deve tratar `target_date` como **opcional** (a coluna nasce na 29); se a 28 for
     buildada antes da 29, o `target_date` simplesmente não existe ainda e o calendário só mostra
     `due_date`/`deadline`. A 29, ao criar a coluna, "liga" o marco no calendário sem mexer na 28.
     **Documentar a contenção:** se a build order for 28→29, a 28 referencia `objectives.target_date`
     que **ainda não existe** → a 28 deve OU criar a coluna ela mesma (migração da 28) OU tratar a
     ausência. **Decisão FIXA pra evitar colisão de migração:** a coluna `target_date` é DONA da fase
     **29** (`0026`); a fase 28 NÃO a cria — o calendário da 28 agrega `target_date` **se a coluna
     existir** (ou a 28 é buildada já assumindo a 29 — ver §9 do PRD: "build order 24→26→27→28→29";
     então a 28 escreve o agregador esperando a coluna, e a 29 a cria 1 fase depois). **O orquestrador
     do build resolve a ordem; a spec da 28 deve referenciar `0026` como dona de `target_date`.**
- **Migração FIXA:** `0026_objectives_modes` (dona: fase 29). Não "ler o próximo livre".
- **Paralelização:** **NÃO totalmente paralelizável** com a fase 28 (compartilham `objectives` no banco
  e o tab-shell no front; a 28 lê `target_date` que a 29 cria). É **paralelizável com 25/26/27** se cada
  uma tiver sua migração fixa (`0023`/`0024`/`0025` vs `0026`) e tocar arquivos distintos — porém o
  **front** colide no tab-shell (24) e na `api.ts`/`Objective` (que 14/28 também tocam). **Recomendação
  pro build:** sequencial após a 28 (ordem do PRD 24→26→27→28→29), ou worktree isolada SÓ pro backend da
  29 (migração + service + board + recálculo, arquivos próprios) e o front da aba Goals integrado no
  main loop (toca `api.ts` e o tab-shell — arquivos compartilhados). O recálculo em `objective_link.go`
  é o único ponto de contenção cross-fase (a 12 é dona; a 29 estende) — **não paralelizar a edição de
  `objective_link.go` com outra fase**.

## 9. Ordem de execução (ver tasks.md)

Backend primeiro (migração → service campos/validação → board campos/progress_pct → recálculo por modo
→ handlers/tri-state), depois frontend (api/tipos → GoalRow/goalProgress → GoalModal → GoalsTab + montar
no shell → reforço de conclusão), depois E2E e gate. Commit por tarefa. `make check-full` é o gate
final. **Regra Nº1:** os testes (red) vêm antes do código (green) em cada tarefa.
