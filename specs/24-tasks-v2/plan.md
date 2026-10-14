# plan.md — Fase 24 · Tasks 2.0

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> migrações (SQL EXATO) + contratos de API + frontend + ordem de tarefas. **Estende a fase 12**
> (board/objetivos/tasks) e o baseline da fase 02 — **não** as reescreve. Herda a cadeia de
> middlewares (`RequireAuth` → `RequireMember` → `RequireWriteRole`) e os padrões de tri-state /
> gate / Route-irmão das fases 02/12/18/19. Nada aqui é código — é o contrato.

---

## 1. Arquitetura geral

```
Backend (estende fases 02/12; pacote novo internal/subtasks):
  migrations/0021_tasks_v2.sql                ← ALTER tasks: status+backlog, priority, notion_url, position (CA-01..04)
  migrations/0022_task_subtasks.sql           ← tabela task_subtasks + índices (CA-06..10)
  internal/tasks/service.go (existe)          ← + priority/notion_url/position em CreateInput/PatchInput;
                                                validStatus aceita 'backlog'; validação priority 0..3,
                                                notion_url ≤2048, position ≥0; INSERT/UPDATE ganham as 3 colunas
                                                (recalcObjectiveStatus/due hook INTOCADOS — só status já dispara)
  internal/subtasks/service.go      (NOVO)    ← subtasks.Service: CRUD + loadTaskGate (visibilidade→404,
                                                escrita→403) — espelha o gate da fase 12/board (CA-06..13)
  internal/api/subtasks_handlers.go (NOVO)    ← REST de sub-tasks (handlers finos)
  internal/api/core_routes.go (existe)        ← + sub-rota /tasks/{taskId}/subtasks DENTRO do bloco /tasks
                                                do mountCore (Core normal, sob RequireMember+RequireWriteRole —
                                                decisão de coordenação F; NÃO sibling em auth_routes)
  internal/objectives/board.go (existe)       ← + subtask_count/done (+ goal_count/done) no BoardTask via
                                                1 query agregada GROUP BY task_id; priority/notion_url/position
                                                já saem do SELECT de tasks; ORDER BY position (CA-14);
                                                + client_name no BoardProject via LEFT JOIN clients (CA-14.1)

Frontend (tab-shell + aba Tasks rica + Inter):
  styles/tokens.css (existe)                  ← --font-ui → Inter (CA-27); --font-mono intocado;
                                                + --notion-mark-bg / --notion-mark-ink (dark+light — CA-14/RNF-03)
  public/fonts/Inter-*.woff2 (NOVO)           ← fonte self-hosted (sem CDN)
  components/ui/ViewToggle.tsx (existe)        ← reescreve useView pra genérico de N valores (CA-16);
                                                Clients/Projects passam a chamar a versão genérica (sem mudança)
  lib/api.ts (existe)                          ← priority/notion_url/position em Task/TaskInput; Subtask + CRUD;
                                                subtask_count/done no BoardTask; client_name no BoardProject (CA-14.1)
  lib/queries/subtasks.ts            (NOVO)    ← useSubtasks(taskId) + mutações (invalidam subtasks + board)
  lib/notion.ts                      (NOVO)    ← notionTitle(url) puro (deriva título do slug — CA-24)
  components/board/  (existe da fase 12, estende):
     ProjectDetail.tsx (existe)               ← vira tab-shell: header + abas Overview/Tasks/Notes/Goals/Calendar
     ProjectTabs.tsx                 (NOVO)    ← barra de abas (fonte de integração 26/28/29 — D-13)
     ProjectOverview.tsx             (NOVO)    ← esqueleto (KPIs + status + goals do /board) — CA-15
     TasksTab.tsx                    (NOVO)    ← toolbar (seg Board/List/Table + New task) + as 3 visões
     TaskBoardColumns.tsx (existe)            ← 4 colunas + DnD HTML5 nativo + quick-add (CA-17/18) [substitui ‹›]
     TaskBoardCard.tsx               (NOVO)    ← card arrastável (nome, prio, meta: due/todos/⭐/time/ícone Notion)
     TaskListView.tsx                (NOVO)    ← lista agrupada por status + quick-add (CA-16/18)
     TaskTableView.tsx               (NOVO)    ← table nativa status/priority/due/todos/time + quick-add (CA-19)
     CenterPanel.tsx                 (NOVO)    ← popup central reusável (scrim+card+toggle side/full) — REUSADO
                                                 pelas fases 26/27 (decisão de coordenação A; NÃO usar .mscrim/.scard
                                                 cruas do mockup como contrato — são estilo interno do componente)
     TaskPanel.tsx                   (NOVO)    ← usa CenterPanel — título/props/desc/sidebar (CA-20/21/22)
     TaskPanelProps.tsx              (NOVO)    ← linhas de propriedade (Status/Priority/Due/Time[+Start mock
                                                 data-testid="task-start"]/Notion) (CA-20/26)
     SubtaskSidebar.tsx             (NOVO)    ← sidebar de sub-tasks (add/feito/⭐) (CA-23)
     NotionPreview.tsx              (NOVO)    ← card de preview do Notion (CA-24; mark "N" via tokens --notion-mark-*)
     ui/ViewToggle.tsx (existe)               ← reescreve useView pra genérico de N valores (CA-16/16.1)
     icons.ts (existe)                        ← + ícones board/table/list/expand/collapse/notion/ext/star/starO/bolt
```

## 2. Migrações (donas: fase 24 — números FIXOS `0021`/`0022`, PRD §7)

> `0020_reminders` é o último no disco (confirmado). A fase 08 (conector, na fila) pega o próximo
> livre **depois** de 0022. INDEX.md: registrar 0021/0022 donas da fase 24.

### 2.1 `0021_tasks_v2.sql` — ALTER tasks (CA-01..04)

```sql
-- 0021_tasks_v2 — fase 24 (plan §2.1): status ganha 'backlog'; priority, notion_url, position.
-- ⚠️ PG17 PERMITE 'ALTER TYPE … ADD VALUE' DENTRO de uma transação SÓ se o valor novo NÃO for
--    usado no MESMO arquivo. O runner roda CADA .sql na própria tx (BEGIN; Exec(arquivo); COMMIT).
--    Logo: NENHUM statement abaixo pode referenciar 'backlog' (sem DEFAULT/WHERE/CHECK/UPDATE no
--    valor novo). O DEFAULT de status CONTINUA 'todo' (intocado). RNF-05.
ALTER TYPE task_status ADD VALUE 'backlog' BEFORE 'todo';

-- priority 0..3 (0=sem · 1=baixa · 2=média · 3=alta). CHECK é backstop; validação real no Go (D-07).
ALTER TABLE tasks ADD COLUMN priority smallint NOT NULL DEFAULT 0
    CHECK (priority >= 0 AND priority <= 3);

-- notion_url: texto livre opcional (NULL = sem doc). Sem validar formato (preview client-side).
ALTER TABLE tasks ADD COLUMN notion_url text;

-- position: ordenação manual no board/lista (DnD). Default 0; o service grava max+1 no create (D-03).
ALTER TABLE tasks ADD COLUMN position integer NOT NULL DEFAULT 0;

-- índice pro ORDER BY position por coluna (board) — CA-14/17.
CREATE INDEX idx_tasks_project_status_position ON tasks (project_id, status, position);
```

- **NÃO** colocar `'backlog'` em nenhum lugar deste arquivo (nem `CHECK (status IN (...))` — o enum
  já restringe; nem `DEFAULT 'backlog'`; nem `UPDATE … SET status='backlog'`). É a armadilha #3 do
  PRD §9. Teste de migração prova: enum tem os 4 valores na ordem certa; `DEFAULT` de status = `todo`;
  `priority` default 0 + CHECK barra `-1`/`4`; `position` default 0; índice existe.

### 2.2 `0022_task_subtasks.sql` — sub-tasks (CA-06..10)

```sql
-- 0022_task_subtasks — fase 24 (plan §2.2): sub-tasks "goals da task". workspace_id denormalizado
-- (D-09 herdado) pro filtro tenant trivial. NÃO referencia objectives (separado dos Goals do projeto).
CREATE TABLE task_subtasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    title        text NOT NULL,
    done         boolean NOT NULL DEFAULT false,
    is_goal      boolean NOT NULL DEFAULT false,
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_subtasks_task ON task_subtasks (task_id, position);
-- agregado de progresso/metas do board (CA-14/RNF-04): contagem por task com filtro is_goal.
CREATE INDEX idx_subtasks_task_goal ON task_subtasks (task_id) WHERE is_goal;
```

- `ON DELETE CASCADE` no `task_id`: deletar a task (CRUD fase 02) apaga as sub-tasks (CA-09).
- `workspace_id` denormalizado: filtro tenant em toda query da `subtasks.Service` sem JOIN extra
  (mas a **visibilidade** ainda exige carregar a task → projeto; ver D-08).
- Teste de migração: tabela/colunas/defaults existem; FK cascade zera ao deletar task pai; índices
  existem.

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **`ALTER TYPE … ADD VALUE 'backlog' BEFORE 'todo'` numa migração própria (`0021`), sem usar o valor no mesmo arquivo** | PG17 só permite ADD VALUE em tx se o valor não for usado no mesmo statement batch (PRD §9.3). O runner roda cada `.sql` na própria tx. Como nada na 0021 referencia `'backlog'` (DEFAULT segue `todo`, sem WHERE/CHECK/UPDATE no valor), a migração roda limpa. RNF-05. |
| D-02 | **priority/notion_url/position como colunas de `tasks` (não propriedades tipadas)** | São propriedades **nativas** (PRD §4.1 / §4.3 "Status/Priority/Due/Time/Notion são nativas, não entram em task_property_values"). Custom é da fase 25. Colunas = simples, indexáveis, no `BoardTask` de graça. |
| D-03 | **`position` no create = `max(position)+1` entre tasks do mesmo `(project_id, status)`** | Quick-add por coluna deve aparecer **no fim** daquela coluna (CA-02/18). Sem isso todo create cairia em `position=0` (default) e a ordem ficaria indefinida. O `PATCH position` (DnD) reescreve. (Reindex global por coluna no DnD é YAGNI nesta fase — basta `position` crescente; empate desempata por `created_at`.) |
| D-04 | **DnD do board grava SÓ `status` no MVP** (não reordenar `position` no drop) | RF-04 pede "persiste `status` (e `position`)", mas reordenar visualmente dentro da coluna exige reescrever positions de N cards (transação custosa). Decisão: o drop entre colunas faz `PATCH {status, position: max+1 da coluna destino}` (vai pro fim da coluna destino). Reordenar **dentro** da mesma coluna por drag = fora (sub-task do roadmap; `position` editável por PATCH segue disponível). Simples, satisfaz "muda status sem clicar em menu" + ordem estável. |
| D-05 | **Sub-tasks = pacote novo `internal/subtasks`, NÃO estender `tasks`** | Ciclo de vida e rotas próprias; mantém `tasks.Service` enxuto. `subtasks.Service` tem seu **próprio** `loadTaskGate` (carrega a task → resolve visibilidade+can_write do projeto, cópia da lógica do board da fase 12 — D-08). **NÃO** importa `tasks`/`projects`/`objectives` (sem ciclo; query SQL direta). |
| D-06 | **Sub-task NÃO referencia `objectives`** (PRD §4.2/§9-decisão-5) | A ⭐ (`is_goal`) é meta **da task**, separada dos Goals/objetivos do projeto. São dois níveis. Não unificar evita acoplamento e confusão de UI. |
| D-07 | **Validação na mão (stdlib)**, igual fases 02/12: trim+len (title 1–500, notion_url ≤2048), faixa (priority 0..3, position ≥0), `uuid.Validate` | Leveza (RNF-01); poucas regras não pagam lib. `priority`/`title`/`notion_url` validados ANTES do SQL → `400`, nunca `500` por CHECK/overflow. |
| D-08 | **`loadTaskGate` no `subtasks`: carrega a task casando `(workspace_id, task_id)` + resolve a visibilidade/can_write do projeto** (1 query com JOIN `tasks→projects` + LEFT JOIN `project_members`) | Sub-task herda a visibilidade e a permissão de escrita do **projeto da task** (CA-07/11/13). `uuid.Validate(taskId)`/`(subtaskId)` antes → 404, nunca 500. Carga da sub-task no PATCH/DELETE casa `(workspace_id, task_id, id)` (CA-13 — não vaza cross-task). |
| D-09 | **notion_url tri-state no PATCH de task** (igual `due_date`); priority/position/status presentes-ou-ausentes | `notion_url:null` limpa, ausente mantém, string define (CA-03). priority/position são inteiros sempre-com-default (não há "null"); status idem (enum). Usa `json.RawMessage` no handler pro tri-state do notion_url (padrão fase 02). |
| D-10 | **Carve-out viewer-assignee NÃO se estende a sub-tasks** (CA-11) | Sub-task é escrita de planejamento (estrutura da task), não "mudar o status da minha task". O carve-out das fases 18/19/20 é status/comentário/lembrete. Manter o gate de sub-task = can_write do projeto (loadProjectGate puro), sem regexp bypass. Decisão registrada. |
| D-11 | **Persistência de visão = `tt.view.board`** (mesma chave da fase 12, D-11 da 12) + **`useView` reescrito pra genérico de N valores** com **mapeamento do legado** | A aba Tasks **é** o board; reusa a chave que a fase 12 já criou (não inventar `tt.taskview` — RF-03 travado). MAS o `useView` real (`components/ui/ViewToggle.tsx`) só suporta 2 valores (`'cards'|'table'`, default `cards`) e faz fallback de desconhecido pra `cards`. Reescrever pra `useView<T extends string>(key, options, fallback)`; as 3 strings desta fase = `'board'|'list'|'table'` (default `'board'`). **Mapear o legado** ao ler: `'cards'`→`'board'`, `'table'`→`'list'` (a LISTA `TaskRow` da fase 12), pra não regredir quem já usava o board; a **Table nativa** nova fica só no valor novo `'table'`; desconhecido → `'board'` (CA-16/16.1). Consumidores `Clients`/`Projects` chamam a versão genérica com `(['cards','table'],'cards')` — comportamento idêntico. |
| D-12 | **Board grava via `PATCH status` + invalida `boardKey` no `onSuccess` local** (não mexer em `lib/queries/tasks.ts`) | Padrão D-12/§6 da fase 12 (aditivo/local, sem regredir a fase 02). A conclusão automática do objetivo chega no refetch do board (CA-17). |
| D-13 | **`ProjectDetail` vira tab-shell; `ProjectTabs` é a fonte de integração das fases 26/28/29** | Esta fase é dona do shell de abas; as fases seguintes **substituem** os placeholders Notes/Calendar/Goals **sem** tocar no shell. Isso sequencia: 24 entrega o shell + Tasks; 26/28/29 plugam suas abas. Declarado no §10 (deps & paralelização). |
| D-14 | **Fonte Inter self-hosted, troca em `tokens.css` (`--font-ui`)** | Decisão 11 do PRD. A fonte é uma var única; trocar o valor + adicionar `@font-face` cobre toda a UI. Mono pra números fica (`--font-mono`). Self-hosted (sem CDN) preserva offline/leveza (RNF-01) e o `check-pwa`. |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) · `invalid_reference` (400) — idênticos às fases 02/12.

### 4.1 Sub-tasks (NOVO — mount Core normal, decisão de coordenação F)

Decisão de rota (**decisão de coordenação F — supersede a "Route IRMÃO" das versões anteriores deste
plan**): sub-tasks são montadas no **Core normal**, DENTRO de `mountCore` (`core_routes.go`), no bloco
`/tasks` que já existe — `r.Route("/{taskId}/subtasks", …)` aninhado em `wr.Route("/tasks", …)`. Isso
herda a cadeia do subrouter `/workspaces/{workspaceId}`: **`RequireMember` + `RequireWriteRole`**
(blanket — `auth_routes.go:86`). O `subtasks.Service` é injetado **no `Core`** (campo novo
`Subtasks *subtasks.Service`), NÃO via Option/sibling separado.

Por que mount Core (e não sibling como comments/reminders): o gate de sub-task é o **`can_write` do
projeto da task PURO** — **NÃO** há carve-out viewer-assignee pra sub-task (D-10/CA-11). Como o gate é
sempre "precisa escrever no projeto", a cadeia `RequireWriteRole` do Core (que já barra o viewer do
workspace) é **coerente** — não há cenário "viewer pode escrever sub-task" a destravar. O comments
(19)/reminders (20) usam sibling justamente porque TÊM carve-out viewer-assignee (precisam deixar o
viewer passar o `RequireWriteRole` e decidir fino no service); sub-task não tem esse carve-out, então
o mount Core é o lugar natural. O `loadTaskGate` do service **permanece a autoridade** pros casos finos
restantes que o `RequireWriteRole` não cobre: `access=view` no projeto → 403; projeto arquivado → 403;
task sem visibilidade/inexistente/cross-task/uuid malformado → 404 (CA-13). Nenhum bypass regexp toca
o path de sub-task (o carve-out da fase 18 é só pra `PATCH .../tasks/{id}` — path diferente).

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/tasks/{taskId}/subtasks` | membro que enxerga a task (invisível → 404) |
| `POST` | `/tasks/{taskId}/subtasks` | escrita no projeto da task (dono/owner-ws/`access=edit`); senão 403 |
| `PATCH` | `/tasks/{taskId}/subtasks/{subtaskId}` | idem |
| `DELETE` | `/tasks/{taskId}/subtasks/{subtaskId}` | idem (hard delete → 204) |

```json
// → POST .../tasks/{taskId}/subtasks   (title obrigatório; done/is_goal/position default)
{ "title": "Revisar com o Carlos" }

// ← 201
{ "id": "su1...", "task_id": "77aa...", "title": "Revisar com o Carlos",
  "done": false, "is_goal": false, "position": 1, "created_at": "2026-06-22T14:00:00Z" }

// ← 400 título inválido (CA-06.1)
{ "error": { "code": "validation_error", "message": "título deve ter entre 1 e 500 caracteres" } }

// ← 403 viewer/access=view (CA-11)   ← 404 task invisível/inexistente/cross-task (CA-13)

// → PATCH .../subtasks/{id}  { "done": true }            ← 200 (marca feita)
// → PATCH .../subtasks/{id}  { "is_goal": true }         ← 200 (marca ⭐ meta)
// → PATCH .../subtasks/{id}  { "title": "...", "position": 3 }
// → GET   .../tasks/{taskId}/subtasks  ← 200 { "subtasks": [ {…}, … ] }  (ordem por position — CA-07)
// → DELETE .../subtasks/{id}  ← 204 (CA-09)
```

`workspace_id` da sub-task = o da rota (gravado no INSERT, **não** vem do cliente). `taskId`/`subtaskId`
malformados → `404` (uuid.Validate antes — CA-13).

### 4.2 Tasks — campos novos (extensão das fases 02/12, RNF-05)

`POST .../tasks` e `PATCH .../tasks/{id}` ganham `priority`, `notion_url`, `position` + `status` agora
aceita `backlog`:

```json
// → POST  (tudo opcional além do já existente; status ∈ {backlog,todo,doing,done})
{ "project_id": "0d9e...", "name": "Investigar churn", "status": "backlog",
  "priority": 1, "notion_url": "https://www.notion.so/Churn-8f3a1c9b" }

// ← 201  (objeto da task da fase 12 + priority/notion_url/position)
{ "id":"77aa...","project_id":"0d9e...","objective_id":null,"name":"Investigar churn",
  "status":"backlog","priority":1,"notion_url":"https://www.notion.so/Churn-8f3a1c9b",
  "position":3,"due_date":null,"recurrence":null,"notes":"","created_by":"me0...",
  "assignee":null,"requested_by":null,"created_at":"..." }

// ← 400 priority fora de 0..3 (CA-02.1)
{ "error": { "code": "validation_error", "message": "priority deve estar entre 0 e 3" } }

// → PATCH { "notion_url": null }   ← 200 desvincula (tri-state — CA-03/D-09)
// → PATCH { "status": "done" }     ← 200 + recalcula objetivo (CA-05, fase 12 herdado)
// → PATCH { "priority": 3 }        ← 200 (edição benigna — NÃO recalcula objetivo)
```

- `status` aceita os 4 valores; `validStatus` no Go (`internal/tasks/service.go:174`) ganha `backlog`.
- O recálculo de objetivo (`recalcObjectiveStatus`) e o `due hook` (fase 20) **não mudam** — só
  `status`/`objective_id`/`due_date` os disparam, exatamente como hoje (CA-05). `priority`/`notion_url`/
  `position` entram na lista de "campos benignos" do `Patch` (não disparam recálculo — D-04 fase 12).
- `notion_url` vazio/só-espaços após trim → grava `null` (CA-04). `> 2048` → `400`.

### 4.3 Board — campos novos no `BoardTask` e `BoardProject` (extensão da fase 12, CA-14/14.1)

`GET .../projects/{projectId}/board`: cada `BoardTask` ganha `priority`, `notion_url`, `position`
(colunas de tasks, já saem do SELECT) e os agregados de sub-task numa **query única** GROUP BY task_id;
o `BoardProject` ganha `client_name` via LEFT JOIN no SELECT de metadados (CA-14.1):

```json
{ "project": { /* BoardProject (fase 12) + ↓ */ "client_name": "Lauda.ai" /* ou null */ },
  /* … */
  "task": { /* BoardTask = Task inteira + tracked_seconds (fase 12) + ↓ */
    "priority": 2, "notion_url": null, "position": 1,
    "subtask_count": 3, "subtask_done": 1, "goal_count": 2, "goal_done": 1 } }
```

- **`client_name` no `BoardProject` (CA-14.1):** o SELECT de metadados do projeto em `board.go`
  (`SELECT p.id, p.name, p.color, … FROM projects p WHERE p.id = $3`) ganha `LEFT JOIN clients c ON
  c.id = p.client_id` e a coluna `c.name` → `b.Project.ClientName *string` (NULL quando sem cliente).
  Aditivo (não muda nenhum campo existente). É a fonte única do chip de cliente no header (CA-15) —
  sem 2º fetch. `BoardProject` struct (Go) ganha `ClientName *string \`json:"client_name"\``; o TS
  espelha `client_name: string | null`.
- A 4ª query do board (além das 3 da fase 12) = `SELECT task_id, count(*) AS sc,
  count(*) FILTER (WHERE done) AS sd, count(*) FILTER (WHERE is_goal) AS gc,
  count(*) FILTER (WHERE is_goal AND done) AS gd FROM task_subtasks WHERE task_id = ANY($tasks do
  projeto) GROUP BY task_id`. Zero N+1 (RNF-04). Tasks ausentes do resultado → contagens 0.
- O board ORDER BY `position` asc (desempate `created_at`) em cada coluna/bucket (CA-14).

## 5. Backend — assinaturas

```go
// internal/subtasks
type Subtask struct {
    ID, TaskID, Title string
    Done, IsGoal      bool
    Position          int
    CreatedAt         time.Time
}
type CreateInput struct { Title string }
type PatchInput struct {
    Title       *string
    Done        *bool
    IsGoal      *bool
    Position    *int
}
func NewService(db *pgxpool.Pool) *Service                                          // gate interno (D-05/D-08)
func (s *Service) List(ctx, ws, userID, taskID string) ([]Subtask, error)          // CA-07/13
func (s *Service) Create(ctx, ws, userID, role, taskID string, in CreateInput) (Subtask, error) // CA-06/11/13
func (s *Service) Patch(ctx, ws, userID, role, taskID, subtaskID string, in PatchInput) (Subtask, error) // CA-08/13
func (s *Service) Delete(ctx, ws, userID, role, taskID, subtaskID string) error    // CA-09/13

// loadTaskGate(ctx, ws, userID, role, taskID, blockArchived) (can_write bool, err) — INTERNO:
//   uuid.Validate(taskID) → 404; carrega a task JOIN projects (+LEFT JOIN project_members) casando
//   t.workspace_id=$ws AND t.id=$taskID; visibilidade (p.visibility='shared' OR p.owner_user_id=$u)
//   → senão domain.NotFound (404); can_write = dono ∥ role=="owner" ∥ access=="edit"; sem escrita →
//   domain.Forbidden (403). **Escrita em projeto arquivado (blockArchived) → 400 invalid_reference**
//   (código canônico REAL — NÃO existe código 'archived'; espelha EXATAMENTE o gate da fase 12:
//   `objectives/gate.go:67` `ensureWrite … blockArchived → 400` e `tasks/service.go:239/251`
//   `loadProjectGate … blockArchived → invalid_reference`). pgx.ErrNoRows → domain.NotFound (404).
//   Resumo de códigos (decisão de coordenação G): visibilidade/inexistente/outro-ws/uuid malformado
//   → 404 not_found; sem permissão de escrita → 403 forbidden; projeto arquivado → 400
//   invalid_reference; sessão ausente → 401 (cadeia RequireAuth).
//
// CA-13 (pertencimento): Patch/Delete carregam a sub-task com
//   WHERE workspace_id=$ws AND task_id=$taskIdDaRota AND id=$subtaskId
//   (uuid.Validate(subtaskId) antes → 404). Sub-task de OUTRA task via rota → 404 (não vaza).

// internal/tasks (estende — NÃO reescreve):
//   CreateInput += Priority *int (nil→0), NotionURL *string (nil→sem doc)
//   PatchInput  += PrioritySet bool; Priority *int
//                  NotionURLSet bool; NotionURL *string   (tri-state — D-09)
//                  PositionSet bool;  Position *int
//   validStatus(): adicionar case "backlog"
//   validações novas ANTES do SQL (CA-02.1/04): priority 0..3, notion_url len ≤2048, position ≥0
//   INSERT/UPDATE de tasks: +priority, +notion_url, +position (no create: position = nextPosition(tx,
//     project_id, status) = COALESCE(max(position),-1)+1 — D-03)
//   ⚠️ recalcObjectiveStatus / due hook (ReminderHook): INTOCADOS — só status/objective_id/due_date
//     disparam; priority/notion_url/position entram nos "campos benignos" (não recalculam).
```

- **Topologia de import (sem ciclo)**: `subtasks` **não** importa `tasks`/`projects`/`objectives`
  (faz SQL direto pra carregar a task e resolver o gate — cópia da lógica do board da fase 12, como o
  `objectives` copiou o gate na fase 12, D-03). `tasks` **não** importa `subtasks` (o board agrega
  sub-task via SQL inline em `objectives/board.go`, que já é dono do read agregado). `objectives/
  board.go` faz a 4ª query (GROUP BY task_id) sem importar `subtasks` (SQL direto na mesma pool).

## 6. Frontend — telas e componentes

Tokens de `tokens.css`. Reusa `.tt-card`, `.tt-goal`, `.chip`, `.tt-btn--accent`, `StatusPill`/cores
de status (mockup linha 197 — já tokens), paleta `lib/palette.ts` (cor do projeto), formatação de
horas (`fmtH`).

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Tab-shell | `components/board/ProjectDetail.tsx` (estende) + `ProjectTabs.tsx` | header (cor + nome + chip de cliente quando `board.project.client_name != null` — CA-14.1/15) + abas Overview/Tasks/Notes/Goals/Calendar; só Tasks rica; Notes/Goals/Calendar placeholders (D-13) |
| Overview esqueleto | `components/board/ProjectOverview.tsx` | KPIs (total time / this week=0h por ora / open / done%) + andamento por status + goals — tudo do `/board` (CA-15); reusa `ProgressBar` da 12 |
| Aba Tasks | `components/board/TasksTab.tsx` | toolbar (seg Board/List/Table persist `tt.view.board` + "New task") + render da visão; gating por `board.project.can_write` (CA-25) |
| Board + DnD | `components/board/TaskBoardColumns.tsx` (reescreve a da 12 sem regredir contrato) + `TaskBoardCard.tsx` | 4 colunas; card `draggable`, `ondragover`/`ondrop` → `dropStatus(k)` puro → `PATCH status`+position; quick-add por coluna (CA-17/18) |
| List | `components/board/TaskListView.tsx` | grupos por status + quick-add por grupo; status 1-clique cicla (CA-16/18) |
| Table | `components/board/TaskTableView.tsx` | colunas status/priority/due/todos/time; status/priority 1-clique ciclam; quick-add (CA-19) |
| **Popup central (componente da fase 24)** | `components/board/CenterPanel.tsx` (NOVO — popup central reusável; chrome side↔full) | **Componente React** dono do scrim + card central + toggle expand/collapse, usando tokens.css. É o popup central que as **fases 26 (nota) e 27 (Spotlight) REUSAM** (decisão de coordenação A). NÃO referenciar classes cruas do mockup (`.mscrim`/`.scard`/`.swrap`) como contrato — elas são detalhe de estilo interno do componente. |
| Popup da task | `components/board/TaskPanel.tsx` | **usa `CenterPanel`** (side/full toggle — CA-22, ícones expand/collapse), título grande (Enter→corpo CA-21), TaskPanelProps, descrição textarea, SubtaskSidebar |
| Propriedades | `components/board/TaskPanelProps.tsx` | Status/Priority chips 1-clique; Due date-picker; Time = tracked real + **botão Start MOCK/desabilitado com `data-testid="task-start"`** (elemento estável; a fase 30 troca só a ação — decisão B; CA-20/26); Notion (input↔preview) |
| Sub-tasks | `components/board/SubtaskSidebar.tsx` | add (Enter), checkbox done, ⭐ is_goal; cabeçalho feitas/total + metas (CA-23) |
| Preview Notion | `components/board/NotionPreview.tsx` + `lib/notion.ts` | mark "N" usa `var(--notion-mark-bg)`/`var(--notion-mark-ink)` (tokens novos — RNF-03, ZERO hex literal) + `notionTitle(url)` (regex case-insensitive `-[0-9a-fA-F]{6,}$`) + notion.so + abrir/desvincular (CA-24) |
| API + hooks | `lib/api.ts` (estende), `lib/queries/subtasks.ts` (novo) | priority/notion_url/position em Task/TaskInput; client_name no BoardProject; Subtask CRUD; useSubtasks invalida subtasks+board |
| Persistência de visão | `components/ui/ViewToggle.tsx` (reescreve `useView`) | `useView<T>` genérico (3 valores `board/list/table`, default `board`) + mapeamento do legado (`cards→board`, `table→list`) — CA-16/16.1 |
| Fonte | `styles/tokens.css` + `public/fonts/Inter-*.woff2` | `--font-ui`→Inter; `@font-face`; mono intocado (CA-27); + tokens `--notion-mark-bg`/`--notion-mark-ink` |

- **DnD HTML5 nativo** (RF-04): o card tem `draggable=true`/`onDragStart` (guarda o id num ref/state)
  e a coluna `onDragOver`(preventDefault)/`onDrop` (chama `dropStatus(targetStatus)` → `PATCH
  {status, position}` + invalida). `dropStatus` é **função pura testável** (recebe dragId+status,
  devolve a mutação) — é assim que o CA-17 vira teste jsdom (Playwright drag é flaky — CA-28).
- **Gating viewer (CA-25)**: tudo lê **`board.project.can_write`** (CA-16 da fase 12). `can_write=false`
  → `draggable=false`, sem quick-add/New task, chips não-clicáveis, date read-only, sidebar/notion
  read-only. Padrão `viewer-gating.spec` da fase 12.
- **Persistência de visão**: `tt.view.board` (D-11). O popup (panel) NÃO persiste estado entre
  reloads; abrir uma task = state local (`panel:{type,id}`), igual o mockup.
- **Mutações otimistas vs invalidação**: editar título/status/priority/due/notion no popup faz `PATCH`
  + invalida `boardKey` E `subtasksKey(taskId)`; sub-task CRUD invalida `subtasksKey` E `boardKey` (pro
  cabeçalho/contagem do board atualizar). Reflexo sem reload (CA-20/23).

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** usar `'backlog'` em qualquer statement da `0021` (DEFAULT/WHERE/CHECK/UPDATE) — quebra o
  `ALTER TYPE … ADD VALUE` na tx do PG17 (PRD §9.3 / D-01). O DEFAULT de status fica `todo`.
- **NÃO** renomear o valor `doing` no banco — só o **rótulo** vira "In progress" no front (CA-01).
- **NÃO** referenciar `objectives` na tabela/serviço de sub-tasks (D-06) — a ⭐ é meta da task, não
  do projeto.
- **NÃO** estender o carve-out viewer-assignee a sub-tasks (D-10/CA-11) — gate = can_write do projeto
  puro.
- **NÃO** carregar sub-task (Patch/Delete) sem casar `(workspace_id, task_id da rota, id)` (CA-13) —
  sub-task de outra task via rota errada → 404; `uuid.Validate` antes → 404 nunca 500.
- **NÃO** disparar `recalcObjectiveStatus`/due hook em mudança de priority/notion_url/position (D-04
  fase 12) — só status/objective_id/due_date disparam. Não tocar o helper existente.
- **NÃO** deixar `position` cair em 0 no create (D-03) — `max+1` por `(project_id, status)`, senão a
  ordem do board é indefinida.
- **NÃO** reordenar N cards no drop (D-04) — o drop entre colunas grava `status` + `position=max+1` da
  coluna destino; reordenar dentro da coluna por drag = fora.
- **NÃO** validar formato de URL do notion_url (CA-04) — texto livre ≤2048; preview client-side via
  `notionTitle` (regex de hash **case-insensitive** `-[0-9a-fA-F]{6,}$` — IDs do Notion vêm em
  maiúsculas/mistas; lowercase-only vazaria o hash no título — CA-24).
- **NÃO** fazer Start funcional (CA-26) — botão mock/desabilitado, MAS **identificável** (`data-testid=
  "task-start"`, decisão B): a fase 30 troca só a ação (`onClick → timerStore.start`) desse mesmo
  elemento; não há "slot/componente provido pela 24" pra fase 30 — é o mesmo botão.
- **NÃO** construir propriedades custom / Add property / Notes / slash / Spotlight / Calendar / Goals
  com modos (fases 25/26/27/28/29) — abas placeholder; o popup mostra só props nativas.
- **NÃO** N+1 no board pra sub-task (CA-14/RNF-04) — 1 query agregada GROUP BY task_id.
- **NÃO** importar lib de DnD/markdown (RNF-01) — HTML5 nativo + textarea.
- **NÃO** quebrar `check-cores` com hex literal nem EDITAR o gate. O `scripts/check-cores.sh` usa
  **regex ANCORADA por arquivo exato** (`^frontend/src/lib/palette\.ts:`, `^frontend/src/styles/
  tokens\.css:`), **NÃO substring de path** — adicionar arquivo à allowlist exigiria editar o script
  (mudança de gate, não-prescrita). O "N" do Notion (fundo branco `#fff` + glifo `#111` no mockup)
  vira **dois tokens novos OBRIGATÓRIOS** em `tokens.css` (dark **e** light): `--notion-mark-bg` e
  `--notion-mark-ink`; o `NotionPreview` usa `var(...)`. ZERO hex literal fora do `tokens.css` →
  `check-cores` verde sem tocar o script (RNF-03).
- **NÃO** usar CDN pra Inter (RNF-01/check-pwa) — woff2 self-hosted em `public/fonts/`.
- **NÃO** regredir o `mobile.spec` (M2) — popup ocupa a tela < md; sidebar vira seção abaixo.

## 8. Ordem de execução (ver tasks.md)

Backend primeiro (migrações → tasks ALTER no service → subtasks.Service + gate → handlers/wiring →
board estendido), depois frontend (api/queries → tab-shell → board/DnD/quick-add → popup + sidebar +
notion → list/table → Inter → gating), depois E2E e gate. Commit por tarefa. `make check-full` é o
gate final.

---

## 9. Libs novas

- **Backend:** nenhuma (chi + pgx + `google/uuid` já no baseline).
- **Frontend:** nenhuma lib JS nova (DnD = HTML5 nativo; markdown = textarea/render trivial). Adiciona
  **arquivos de fonte Inter** (`.woff2`) em `public/fonts/` — não é dependência npm, é asset.

## 10. Dependências & paralelização

- **Depende de:** fases 02 (tasks CRUD), 12 (board/objetivos/`loadProjectGate`/`tracked_seconds`/
  `can_write`/`tt.view.board`), 18 (carve-out — só pra **não** colidir; D-10 mantém o gate de subtask
  fora dele), 20 (due hook — não tocar), M2/22 (breakpoint 768). NÃO depende de nenhuma fase da M3
  (é a primeira).
- **É dona do tab-shell** (`ProjectDetail`/`ProjectTabs`) que **26 (Notes)**, **28 (Calendar/Overview
  rica)** e **29 (Goals)** estendem. Portanto, no build da M3: **a 24 vem primeiro** e estabelece o
  shell; 26/28/29 substituem os placeholders sem reescrever o shell. A 25 (propriedades) liga os
  `customRows` do popup e a coluna da Table.
- **Paralelização (worktrees):** o **backend** da 24 (migrações 0021/0022 + subtasks + board) é
  worktree-isolável de outras fases (migrações com número FIXO). O **frontend** da 24 toca arquivos
  que 25/26/28/29 também tocarão (`ProjectDetail`, `TaskPanel`, `TasksTab`) → essas fases têm
  **dependência dura** do frontend da 24 (precisam do shell pronto). Conclusão: **a 24 NÃO se
  paraleliza com as outras fases da M3 no frontend**; é a fundação sequencial. Dentro da 24, o build
  é sequencial (TDD stateful), salvo as 2 migrações (independentes entre si).
- **Migrações FIXAS:** `0021_tasks_v2`, `0022_task_subtasks` (PRD §7). NÃO "ler o próximo livre do
  disco". As fases seguintes da M3: 26=`0024_notes`(+note_links), 28=`0025_projects_deadline`,
  29=`0026_objectives_modes`, 25=`0023_task_properties` (numeração do PRD §7 — a 25 vai por último mas
  pega o 0023). ⚠️ **Nota de contenção:** se 25 for buildada DEPOIS de 26/28/29, seu número 0023 fica
  "no meio" do disco — registrar no INDEX a numeração FIXA por fase (não por ordem de build).
