# plan.md — Fase 12 · Gerenciador de tarefas (objetivos → tasks)

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> assinaturas + ordem de tarefas. **Estende a fase 02** (clientes/projetos/tasks) — não a
> reescreve. Herda o baseline do `SDD.md §7` e a cadeia de middlewares da fase 01/02
> (`RequireAuth` → `RequireMember` → `RequireWriteRole`). Nada aqui é código — é o contrato.

---

## 1. Arquitetura geral

```
Backend (estende a fase 02; novo pacote internal/objectives + agregação):
  migrations/0013_objectives.sql              ← tabela objectives + ALTER tasks ADD objective_id (RNF-03)
  internal/objectives/service.go     (NOVO)   ← objectives.Service: CRUD + auto-status (CA-01..08)
  internal/objectives/board.go       (NOVO)   ← Board: read agregado único projeto+objetivos+tasks+tempo (CA-16)
  internal/tasks/service.go (existe)          ← + objective_id em CreateInput/PatchInput + checagem de ref
                                                + recálculo de status do objetivo afetado (CA-06/09)
  internal/api/objectives_handlers.go (NOVO)  ← REST de objetivos + board (handlers finos)
  internal/api/core_routes.go (existe)        ← + subárvore /projects/{projectId}/objectives e /board

Frontend (o gerenciador):
  lib/api.ts                                  ← Objective/ObjectiveInput, Board, get/post/patch/delete + objective_id em Task
  lib/queries/objectives.ts          (NOVO)   ← useBoard + mutações de objetivo (invalida board)
  lib/queries/tasks.ts (existe)               ← mutações já invalidam; board é o consumidor novo
  components/board/
     ProjectBoard.tsx                (NOVO)   ← orquestra board↔lista, objetivos + Geral
     ObjectiveCard.tsx               (NOVO)   ← cabeçalho do objetivo: nome, progresso, barra de horas
     ObjectiveModal.tsx              (NOVO)   ← criar/editar objetivo (nome, descrição, goal_hours)
     TaskBoardColumns.tsx            (NOVO)   ← colunas todo/doing/done (board) — mover via select/botão
     TaskRow.tsx                     (NOVO)   ← linha de task (lista) com status, data, tempo, editar
     TaskModal.tsx                   (NOVO)   ← criar/editar task (nome, status, data, notas, objetivo)
     ProgressBar.tsx                 (NOVO)   ← % de tasks done + barra de horas (reusa .tt-goal)
  routes/cockpit/ProjectDetail.tsx   (NOVO)   ← rota /projetos/{id} com o ProjectBoard
  routes/cockpit/Projects.tsx (existe)        ← cartão de projeto vira link pro detalhe (sem reescrever a página)
```

## 2. Migração — `0013_objectives.sql` (dona: fase 12)

```sql
-- 0013_objectives — fase 12 (plan §2): camada objetivos entre projeto e task.
-- workspace_id denormalizado (igual tasks, D-09 da fase 02) pro filtro tenant
-- trivial (CA-15). Sem soft-delete: objetivo é open/done + hard delete; tasks
-- que o referenciam caem pra NULL (bucket "Geral", CA-07/11).
CREATE TYPE objective_status AS ENUM ('open', 'done');

CREATE TABLE objectives (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid NOT NULL REFERENCES projects(id),
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    status       objective_status NOT NULL DEFAULT 'open',
    -- numeric(6,2) → cabe a faixa declarada (0,9999] (numeric(5,2) só ia a 999.99 e
    -- estouraria com 22003/500 — furo da verificação da spec). NULL = sem meta.
    goal_hours   numeric(6,2) CHECK (goal_hours IS NULL OR (goal_hours > 0 AND goal_hours <= 9999)),
    position     integer NOT NULL DEFAULT 0,
    created_by   uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_objectives_project ON objectives (project_id, position);

ALTER TABLE tasks
    ADD COLUMN objective_id uuid REFERENCES objectives(id) ON DELETE SET NULL;
CREATE INDEX idx_tasks_objective ON tasks (objective_id);

-- soma de tempo por task do /board (CA-16/RNF-04): os índices da 0009 cobrem só
-- user_id/workspace_id+started_at; este cobre o GROUP BY task_id das entries fechadas.
CREATE INDEX idx_entries_task ON time_entries (task_id) WHERE duration_seconds IS NOT NULL;
```

- Numeração: `0012_calendar_settings` é o último → esta fase começa em **`0013`** (conferir o
  diretório antes de criar). INDEX.md: registrar **0013 dona da fase 12**.
- `goal_hours numeric(6,2)` → comporta a faixa declarada `(0, 9999]` (`numeric(5,2)` só ia a
  `999.99` → estouro `22003`/`500` em `goal_hours=9999`, furo confirmado na verificação da spec).
  O teto **9999** é validado no service ANTES do SQL (D-07) pra garantir `400 validation_error`;
  o `CHECK (goal_hours IS NULL OR (goal_hours > 0 AND goal_hours <= 9999))` na migração é o
  backstop (piso **e** teto).
- `ON DELETE SET NULL` em `tasks.objective_id`: a única forma de remover objetivo (CA-07) sem
  apagar tasks. Espelha o `time_entries.task_id ON DELETE SET NULL` da fase 03.

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Objetivos aninhados no projeto na URL**: `/api/v1/workspaces/{workspaceId}/projects/{projectId}/objectives[/{objectiveId}]` e `/projects/{projectId}/board` | Objetivo só existe dentro de um projeto; a URL torna a cadeia de visibilidade explícita (reusa o `loadProjectGate` da fase 02 com o `projectId` da rota). Impossível "esquecer" o filtro de projeto/workspace (CA-13/15). PATCH/DELETE de objetivo carregam o `projectId` na rota pra revalidar o gate (sem confiar só no `objective_id`). |
| D-02 | **Pacote novo `internal/objectives`**, NÃO estender `internal/projects` | Mantém o `projects.Service` da fase 02 intocado (RNF-05); objetivo tem ciclo de vida e regras próprias (auto-status). O `objectives.Service` tem o **seu próprio** `loadProjectGate` (cópia da lógica da fase 02 — D-03 alt. B, decidido; NÃO importa `projects`/`tasks`, sem ciclo). |
| D-03 | **Gate de escrita REPLICADO no `objectives` (alternativa B — DECIDIDO; sem extração)**: o `tasks.Service.loadProjectGate` resolve "visibilidade→404, permissão→403, arquivado→400". O `objectives.Service` precisa da MESMA regra, mas **NÃO** se extrai nada de `tasks`/`projects` — o pacote `objectives` ganha um helper interno `loadProjectGate` (cópia das ~25 linhas de `tasks/service.go:110-138`: dono ∥ `role=="owner"` ∥ `project_members.access=="edit"`; visibilidade `(visibility='shared' OR owner_user_id=$user)`; arquivado quando `blockArchived`). `tasks.Service` e `projects.Service` ficam **byte-idênticos** (RNF-05 garantido de fato; zero churn de construtor; zero risco de ciclo). Um teste de **paridade** (T-02) prova que as duas rotas dão o mesmo erro pra mesma entrada. | A verificação adversarial da spec mostrou que a extração (alt. A) é mais invasiva num build autônomo: muda `tasks.NewService` em 3 call sites e ADICIONA lógica nova a `projects` (que só tem `canEditMeta` owner-only, sem o ramo `access=edit`) — risco de regressão. ~25 linhas duplicadas é o preço barato de manter a fase 02 intocada. Extração vira refactor opcional FUTURO. |
| D-04 | **Recálculo de status DISPARADO POR TRANSIÇÃO (sem coluna nova — DECIDIDO)**: a coluna `status` é a verdade persistida; o fecho/reabertura manual (`PATCH status` — CA-05) é um write direto. O recálculo automático (CA-06) roda **na transação da mutação de task** (D-09), mas **SOMENTE** quando a mutação muda composição/done-ness do objetivo: task **criada** nele, **deletada** dele, com **`status` alterado**, ou com **`objective_id` alterado** (movida pra dentro/fora). Um `PATCH` de task que toca **só** `name`/`notes`/`due_date`/`recurrence` **NÃO** dispara recálculo. Quando dispara, para um objetivo com **`task_count ≥ 1`**: `done_count == task_count` **e** `status='open'` → grava `done`; `done_count < task_count` **e** `status='done'` → grava `open`; senão no-op. **`task_count == 0` → no-op** (objetivo vazio nunca é tocado pelo recálculo — nem fecha nem reabre). | Resolve a contradição achada na verificação: um recompute ABSOLUTO em TODA mutação destruía o fecho/reabertura manual a cada edição benigna (renomear uma task reabriria um objetivo fechado manual a <100%). Disparar só em transição de `status`/`objective_id`/criação/deleção preserva o override manual contra edições benignas (CA-06.1) **sem** coluna `closed_manually` (RNF-01). O override manual só é sobrescrito quando uma transição real cruza o limiar (marcar task done/doing). A regra é idempotente, então não importa qual `status`-change disparou. |
| D-05 | **`Board` = read agregado único** (`/projects/{id}/board`), NÃO endpoints separados por objetivo | RNF-04: evita N+1. 3 queries fixas (objetivos do projeto; tasks do projeto com `objective_id`; soma de `duration_seconds GROUP BY task_id` das entries do user/projeto), montadas em memória em O(n). O cliente recebe a árvore pronta. |
| D-06 | **Bucketização do tempo — DOIS buckets INDEPENDENTES (CA-16/17)**: `tracked_seconds` por **task** = `SUM(duration_seconds)` das entries FECHADAS do user/workspace cujo `task_id ∈ {tasks deste projeto}` — **SEM** filtrar `project_id` da entry. `tracked_seconds` por **objetivo** = soma das suas tasks. `tracked_seconds` do **projeto** = soma das entries FECHADAS do user/workspace com `project_id` = projeto (incluindo as sem `task_id`). **Os dois buckets NÃO se comparam por invariante**: como `project_id` e `task_id` de uma entry são colunas independentes (schema da fase 03 não força concordância), `Σ tasks` pode **exceder** o `tracked` do projeto (entry com `task_id` do projeto mas `project_id` nulo/diverso) — isso é esperado. | A verificação da spec mostrou que "projeto ≥ Σ" é FALSO e que escopar a soma por task por `entry.project_id` PERDERIA o tempo de entries com `task_id` do projeto mas `project_id` nulo/diverso (timer iniciado numa task sem project_id). Pinar a soma por task em `task_id ∈ tasks do projeto` bate com CA-16 (por `task_id`) e CA-17 (entry sem task não infla task/objetivo; projeto = por `project_id`). Task deletada (`task_id`→NULL na entry) some da task/objetivo mas segue no projeto via `project_id` (CA-07/17). |
| D-07 | **Validação na mão (stdlib)**, igual fase 02: trim+len pro nome (1–200), `goal_hours` em `(0, 9999]`, switch pro enum de status, `uuid.Validate` pro `objective_id` | Leveza (RNF-01); 4 regras não pagam uma lib. |
| D-08 | **`objective_id` tri-state no PATCH de task** (igual `due_date`/`recurrence` da fase 02): `json.RawMessage` no handler distingue ausente×null; o service ganha `ObjectiveIDSet bool` + `ObjectiveID *string` | A spec exige "ausente mantém, null desassocia" (CA-09). Espelha o padrão já provado de tri-state da fase 02 (não inventar). |
| D-09 | **Recálculo de status na MESMA transação** da mutação da task | Sem corrida: criar/mover/concluir task e recalcular o(s) objetivo(s) afetado(s) é atômico. Mover task de objetivo A→B recalcula **os dois** (A pode ter virado 100%, B pode ter saído). |
| D-10 | **Front: rota de detalhe nova `/projetos/{id}`** com o `ProjectBoard`; a página Projetos da fase 02 fica (cartões/tabela) e o cartão **linka** pro detalhe | Não reescreve a `Projects.tsx` (RNF-05); o board é grande demais pra caber no cartão expandido. O `ProjectTasks.tsx` da fase 02 (lista inline no cartão) pode ser aposentado ou mantido — decidir na T-09 (provável: aposentar, o board substitui). |
| D-11 | **Board ↔ Lista por toggle persistido** (`localStorage tt.view.board`), padrão `ViewToggle` da fase 02 | Regra canônica do DESIGN-additions; reusa o componente existente. |
| D-12 | **Mover task = `PATCH status`** (board) / select de status (lista) — sem DnD | RNF-01/§6: nada de lib de drag. A coluna é um destino visual; o clique/select dispara o PATCH e a invalidação reflete (CA-20). |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) · `invalid_reference` (400) — idênticos à fase 02.

### 4.1 Objetivos

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/projects/{projectId}/objectives` | membro que enxerga o projeto (invisível → 404) |
| `POST` | `/projects/{projectId}/objectives` | escrita no projeto (dono/owner-ws/membro edit — CA-01); viewer 403 |
| `PATCH` | `/projects/{projectId}/objectives/{objectiveId}` | idem |
| `DELETE` | `/projects/{projectId}/objectives/{objectiveId}` | idem (hard delete, tasks → Geral — CA-07) |

`POST .../objectives`:

```json
// → request (description e goal_hours opcionais; position auto no fim)
{ "name": "Lançar onboarding v2", "description": "fluxo completo", "goal_hours": 12 }

// ← 201
{ "id": "ob1...", "workspace_id": "8c...", "project_id": "0d9e...",
  "name": "Lançar onboarding v2", "description": "fluxo completo", "status": "open",
  "goal_hours": 12, "position": 2, "created_by": "me0...",
  "created_at": "2026-06-19T14:00:00Z" }

// ← 400 nome inválido (CA-01.1)
{ "error": { "code": "validation_error", "message": "nome deve ter entre 1 e 200 caracteres" } }

// ← 403 viewer / access=view (CA-08/12)        ← 404 projeto invisível (CA-13)
```

`GET .../objectives` → `200 { "objectives": [ {…}, {…} ] }` (ordem por `position` — CA-03).
`PATCH .../objectives/{id}` aceita subconjunto de `{name, description, goal_hours, status,
position}`; `goal_hours: null` remove (CA-04); `status: "done"|"open"` fecha/reabre manual
(CA-05). `DELETE` → `204` (CA-07).

### 4.2 Board (read agregado — CA-16)

`GET .../projects/{projectId}/board` → `200`:

```json
{
  "project": { "id": "0d9e...", "name": "...", "color": "#4f9dde",
               "weekly_hours_goal": 18, "archived_at": null,
               "tracked_seconds": 54000, "can_write": true },
  "objectives": [
    { "id": "ob1...", "name": "Lançar onboarding v2", "description": "...",
      "status": "done", "goal_hours": 12, "position": 0,
      "task_count": 3, "done_count": 3, "done_pct": 100, "tracked_seconds": 40000,
      "tasks": [ { /* Task com objective_id + tracked_seconds */ }, ... ] }
  ],
  "general": {                          // bucket "Geral" — tasks sem objetivo
    "task_count": 2, "done_count": 0, "tracked_seconds": 1200,
    "tasks": [ { /* Task com objective_id:null + tracked_seconds */ } ]
  }
}
```

- Task no board = o objeto da fase 02 + `objective_id` + `tracked_seconds:int`.
- `done_pct = round(done_count/task_count*100)`, `0` quando `task_count=0` (CA-16.1).
- **Um** request; backend faz 3 queries fixas (D-05), zero N+1 (RNF-04).
- Visibilidade: projeto invisível → `404` (CA-13). Tempo só do `user_id` da sessão (CA-15).
- `project.can_write` (boolean) é computado pelo gate do pacote `objectives` (D-03) pro user da
  sessão: `true` sse dono ∥ owner-ws ∥ `access=edit`, **e** projeto não arquivado — o front usa
  no gating do CA-22 (a role do ws sozinha não basta). Tempo por task soma `task_id ∈ tasks do
  projeto`, SEM filtrar `project_id` da entry (D-06).

### 4.3 Tasks — campos novos (extensão da fase 02, RNF-05)

`POST .../tasks` e `PATCH .../tasks/{id}` ganham `objective_id` (CA-09):

```json
// → POST (objective_id opcional; null/ausente = Geral)
{ "project_id": "0d9e...", "name": "Definir telas", "objective_id": "ob1..." }

// ← 201  (objeto da task da fase 02 + objective_id)
{ "id": "77aa...", "project_id": "0d9e...", "objective_id": "ob1...", "name": "Definir telas",
  "status": "todo", "due_date": null, "recurrence": null, "notes": "",
  "created_by": "me0...", "assignee": null, "requested_by": null, "created_at": "..." }

// ← 400 objetivo de outro projeto / inexistente (CA-09.1)
{ "error": { "code": "invalid_reference", "message": "objective_id não pertence a este projeto" } }

// → PATCH { "objective_id": null }   ← 200 desassocia (vira Geral, tri-state — CA-09/D-08)
// → GET /tasks?objective_id=ob1...   ← 200 só tasks do objetivo (CA-10)
```

- A criação/edição/deleção de task que **muda `status`/`objective_id`** recalcula o status do(s)
  objetivo(s) afetado(s) na mesma transação (CA-06/D-04/D-09); edição benigna (só `name`/`notes`/
  `due_date`/`recurrence`) **não** recalcula.
- `project_id` de task permanece **IMUTÁVEL** nesta fase (o `PatchInput` da fase 02 **não** tem
  `ProjectID`; **não** adicionar). A checagem CA-09.1 no PATCH usa o `project_id` **persistido** da
  task — não há caminho "mudar `project_id` + `objective_id` juntos".

## 5. Backend — assinaturas

```go
// internal/objectives
type Objective struct {
    ID, WorkspaceID, ProjectID, Name, Description, Status string
    GoalHours *float64 `json:"goal_hours"`
    Position  int
    CreatedBy string
    CreatedAt time.Time
}
type CreateInput struct { Name, Description string; GoalHours *float64 }
type PatchInput struct {
    Name, Description, Status *string
    GoalSet bool; GoalHours *float64        // tri-state (null remove — CA-04)
    PositionSet bool; Position *int
}
func NewService(db *pgxpool.Pool) *Service   // gate é interno do pacote (D-03, alt. B — NÃO injeta ProjectGate)
func (s *Service) List(ctx, ws, userID, projectID string) ([]Objective, error)        // CA-03
func (s *Service) Create(ctx, ws, userID, role, projectID string, in CreateInput) (Objective, error) // CA-01/02/08
func (s *Service) Patch(ctx, ws, userID, role, projectID, objectiveID string, in PatchInput) (Objective, error) // CA-04/05/13.1
func (s *Service) Delete(ctx, ws, userID, role, projectID, objectiveID string) error  // CA-07/13.1
func (s *Service) Board(ctx, ws, userID, projectID string) (Board, error)             // CA-16 (read agregado)

// loadProjectGate é INTERNO do pacote objectives (D-03 alt. B — cópia de tasks/service.go,
// NÃO importa projects nem tasks): visibilidade→404, permissão(dono ∥ owner-ws ∥ access=edit)→403,
// arquivado quando blockArchived→400. Devolve também canWrite/archived pro Board montar can_write.
//
// CA-13.1 (pertencimento): Patch/Delete/List-de-um carregam o objetivo SEMPRE com
//   WHERE workspace_id=$ws AND project_id=$projectIdDaRota AND id=$objectiveId
//   (uuid.Validate(objectiveId) antes → 404, nunca 500); pgx.ErrNoRows → domain.NotFound.
//   Objetivo de OUTRO projeto (mesmo ws) alcançado por rota de A → 404 (não vaza).
//
// recalcObjectiveStatus(ctx, tx, objectiveID) — roda na transação da mutação de task (D-04/D-09),
// SÓ quando a mutação muda status/objective_id/criação/deleção (edição benigna NÃO chama):
//   SELECT count(*) AS n, count(*) FILTER (WHERE status='done') AS d FROM tasks WHERE objective_id=$1
//   if n == 0: return (no-op — objetivo vazio nunca é tocado)
//   if d == n  AND status='open' : UPDATE ... SET status='done'    (auto-conclui — CA-06)
//   if d <  n  AND status='done' : UPDATE ... SET status='open'    (auto-reabre — CA-06)
//   else: no-op (idempotente; preserva override manual — CA-06.1)
// Mover task A→B: chama recalc(A) e recalc(B).
```

- **Validação (CA-01.1/02/05)** no `Create`/`Patch` ANTES do SQL → `domain.Validation(...)`.
  `goal_hours`: rejeitar `<=0` **ou** `>9999` com `domain.Validation` antes do SQL (CA-02 — nunca
  deixar o `numeric` estourar pra `500`).
- **Topologia de import (sem ciclo, alt. B do D-03)**: `objectives` tem seu **próprio**
  `loadProjectGate` (não importa `projects` nem `tasks`). O recálculo de status vive como **SQL
  inline no `tasks.Service`** (helper que recalcula por `objective_id` afetado, sem importar
  `objectives`). `tasks` **não** importa `objectives`; `objectives` **não** importa `tasks`. Os
  dois pacotes da fase 02 (`tasks`, `projects`) ficam **byte-idênticos** salvo o `objective_id` em
  `tasks` (campo + checagem CA-09.1 + chamada do recálculo). Registrado.

## 6. Frontend — telas e componentes

Tokens de `tokens.css` (fase 00). Reusa `ViewToggle`, `.tt-card`, `.tt-goal`, `.tt-dot`,
`StatusPill`, paleta `lib/palette.ts` (cor do projeto), formatação de horas do `/hoje`/relatórios.

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Rota detalhe do projeto | `routes/cockpit/ProjectDetail.tsx` | `/projetos/{id}`; carrega `useBoard(ws, id)`; header com nome+cor+tempo total; toggle board↔lista |
| Orquestrador | `components/board/ProjectBoard.tsx` | lista objetivos (cada um `ObjectiveCard` + suas tasks) + bucket "Geral"; modo board/lista |
| Cartão de objetivo | `components/board/ObjectiveCard.tsx` | nome, `ProgressBar` (% done + barra de horas), ações de escrita (editar/concluir/excluir) gated por `canWrite` (CA-22) |
| Progresso | `components/board/ProgressBar.tsx` | % de tasks done (sempre) + barra de horas via `.tt-goal` SÓ com `goal_hours` (CA-23, regra do `.tt-goal` da fase 02) |
| Colunas | `components/board/TaskBoardColumns.tsx` | 3 colunas todo/doing/done; mover = `PATCH status` (CA-20), sem DnD (D-12) |
| Linha de task | `components/board/TaskRow.tsx` | nome, status, data, `tracked_seconds` formatado, editar |
| Modal de task | `components/board/TaskModal.tsx` | nome, status, data, notas, **select de objetivo** (com "Geral") — criar/editar (CA-21) |
| Modal de objetivo | `components/board/ObjectiveModal.tsx` | nome, descrição, meta de horas — validação `goal_hours>0` no client antes do POST/PATCH (CA-21) |
| API + hooks | `lib/api.ts` (estende), `lib/queries/objectives.ts` (novo: `useBoard`, mutações) | mutações de objetivo e task invalidam a query do board → reflexo sem reload (CA-20/21) |

- `useBoard` é a query central; mutações de objetivo (hooks novos em `lib/queries/objectives.ts`)
  invalidam `boardKey(ws, projectId)`. Para as mutações de **task** no board (CA-20/21): **FIXO —
  o `ProjectBoard`/`ProjectDetail` invalida `boardKey` no `onSuccess` local** das chamadas
  `useCreateTask`/`usePatchTask`/`useDeleteTask` (via `mutate(vars, { onSuccess })`). **NÃO** mexer
  em `lib/queries/tasks.ts` (não regredir a fase 02 — a invalidação extra é aditiva e local). A
  conclusão automática do CA-06 chega no refetch do board.
- Gating viewer (CA-22): a UI lê **`board.project.can_write`** (CA-16, computado no backend pelo
  gate — D-03) — **não** `useRole(ws)` sozinho (que não distingue `access=view`/`edit` num projeto
  `shared`). Padrão de teste `viewer-gating.spec` da fase 02, com `can_write` mockado.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** apagar tasks ao deletar objetivo (CA-07) — é `ON DELETE SET NULL`, viram "Geral".
- **NÃO** recomputar status de forma ABSOLUTA em TODA mutação de task (furo da verificação) —
  o recálculo dispara **só** em criação/deleção/mudança de `status`/`objective_id` (D-04); edição
  benigna (`name`/`notes`/`due_date`/`recurrence`) **não** recalcula, senão destrói o
  fecho/reabertura manual (CA-06.1).
- **NÃO** deixar o recálculo de status fora da transação da mutação de task (D-09) — corrida.
- **NÃO** tocar objetivo **sem tasks** no recálculo (CA-06/D-04) — `task_count == 0` é no-op
  (nem auto-conclui nem auto-reabre).
- **NÃO** deixar `goal_hours` estourar o `numeric` (CA-02) — validar `0 < goal_hours <= 9999`
  ANTES do SQL → `400`, nunca `500`.
- **NÃO** fazer N+1 no `/board` (RNF-04/D-05) — 3 queries fixas, montagem em memória.
- **NÃO** escopar a soma de tempo por task por `entry.project_id` (D-06) — é por
  `task_id ∈ tasks do projeto`; o filtro por `project_id` é só do bucket do **projeto**.
- **NÃO** afirmar a invariante `projeto ≥ Σ objetivos` (D-06) — buckets independentes, podem
  divergir.
- **NÃO** somar tempo de **outros usuários** em nenhum agregado (CA-15) — filtro `user_id` da
  sessão em **toda** soma do board.
- **NÃO** vincular `objective_id` de outro projeto/workspace nem malformado (CA-09.1) — checar
  `objective.project_id == task.project_id` na mesma transação; `uuid.Validate` → `invalid_reference`.
- **NÃO** carregar objetivo (Patch/Delete) sem casar o `project_id` da rota (CA-13.1) — `WHERE
  workspace_id AND project_id AND id`; objetivo de projeto vizinho via rota errada → `404`.
- **NÃO** vazar projeto invisível: objetivo herda a visibilidade do projeto (CA-13) — passar
  pelo `loadProjectGate` interno SEMPRE, antes de tocar `objectives`.
- **NÃO** criar ciclo de import `tasks ↔ objectives` (D-03/§5) — recálculo via SQL inline no
  `tasks`; o gate é cópia interna no `objectives` (NÃO importar `projects`/`tasks`).
- **NÃO** reescrever/extrair `tasks.Service`/`projects.Service`/`Projects.tsx` (RNF-05/D-03 alt.
  B) — `objectives` é auto-contido; só linkar pro detalhe novo (D-10).
- **NÃO** introduzir lib de DnD/board (RNF-01/§6) — mover = PATCH explícito.
- **NÃO** tratar `tasks.recurrence` como motor (D-08 fase 02) — segue texto opaco.

## 8. Ordem de execução (ver tasks.md)

Backend primeiro (migração → gate extraído → objectives.Service + auto-status → board →
objective_id em tasks → handlers/wiring), depois o frontend (api/queries → componentes → rota
detalhe → gating), depois E2E e gate. Commit por tarefa. `make check-full` é o gate final.
