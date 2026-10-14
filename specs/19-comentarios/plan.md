# plan.md — Fase 19 · Comentários (thread) em tasks

> Como construir a spec. Decisões de design (D-xx) + arquitetura + contratos + ordem de tarefas.
> Reusa o gate de visibilidade da fase 02/12 (`tasks.loadProjectGate`/`projects.visibleWhere`), o
> harness de teste `coretest_test.go`, o `domain.Error` canônico, o `reqctx`, o board da fase 12
> (UI). Adiciona: migração `0019`, pacote `internal/comments`, rotas sob a task, painel de thread.

---

## 1. Arquitetura geral

```
Backend:
  db/migrations/0019_task_comments.sql       ← tabela task_comments + índice (task_id, created_at)
  internal/comments/               (NOVO pacote)
     service.go                               ← Create/List/Delete + gate de visibilidade/carve-out
     service_test.go                          ← TDD (dbtest): criar/listar/apagar, body, isolamento
  internal/api/comments_handlers.go (NOVO)    ← tradução HTTP (POST/GET/DELETE sob a task)
     comments_handlers_test.go                ← harness api: 201/400/403/404/401, ordem, carve-out
  internal/api/core_routes.go (existe)        ← + sub-rotas /tasks/{taskId}/comments no mountCore
  internal/api/auth_routes.go (existe)        ← (ver D-02) escopo do RequireWriteRole p/ o carve-out

Frontend:
  lib/api.ts (existe)                         ← + tipo Comment + listComments/createComment/deleteComment
  lib/queries/comments.ts          (NOVO)     ← TanStack: useComments(wsId, taskId) + mutations
  components/board/TaskThread.tsx  (NOVO)     ← painel da thread (lista + input), gating can_comment
  components/board/TaskModal.tsx (existe)     ← monta a <TaskThread/> no modo Editar (task existe)
  components/board/ProjectBoard.tsx (existe)  ← passa can_comment ao TaskModal (do board/can_write)
```

## 2. Migração — `0019_task_comments.sql` (dona: fase 19)

> **Número:** as fases 17 (`0017_invitations`) e 18 (`0018`, índices de assignee/requested_by) são
> donas dos números anteriores (`MILESTONE-2.md §4/§5`). A T-01 fixa o número lendo
> `backend/internal/db/migrations/` (próximo livre; ownership 0019 é fixo por design pra não colidir
> na worktree paralela). Conteúdo:

```sql
-- 0019_task_comments — fase 19 (plan §2): thread de comentários por task.
-- workspace_id denormalizado (igual tasks, D-09 da fase 02) → filtro tenant
-- direto. ON DELETE CASCADE no task_id: a thread morre com a task.

CREATE TABLE task_comments (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  author_id    uuid NOT NULL REFERENCES users(id),
  body         text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_comments_task ON task_comments (task_id, created_at);
```

- **`task_id` `ON DELETE CASCADE`** (CA-01.1): o hard delete de task da fase 02 (`tasks.Delete`)
  limpa a thread sem código novo. `workspace_id` e `author_id` **sem** cascade explícito de delete de
  user/ws (espelha `tasks`, que também não cascateia user/ws — o domínio nunca apaga user/ws no MVP).
- **Índice `(task_id, created_at)`** (RNF-01/CA-06): a thread lista por `task_id` ordenada por
  `created_at` — o índice serve os dois.
- INDEX.md: registrar **0019 dona da fase 19** (número final confirmado na T-01).

## 3. Backend — `internal/comments` (service)

### D-01 — pacote novo `comments`, gate de visibilidade reusado (não reescrito)

O service espelha o shape de `tasks.Service`/`clients.Service` (recebe `*pgxpool.Pool`, devolve
`domain.Error`). A **visibilidade** NÃO é reimplementada: toda query de comentário faz **JOIN com a
task e o projeto** aplicando o MESMO predicado `(p.visibility = 'shared' OR p.owner_user_id = $user)`
que `tasks.Get`/`loadProjectGate`/`projects.visibleWhere` usam. Assim, "task invisível → 404" cai de
graça (a task simplesmente não aparece no JOIN → `ErrNoRows` → `domain.NotFound`).

```go
type Comment struct {
    ID          string    `json:"id"`
    TaskID      string    `json:"task_id"`
    WorkspaceID string    `json:"workspace_id"`
    AuthorID    string    `json:"author_id"`
    AuthorName  string    `json:"author_name"`   // join com users.name (CA-02/06)
    Body        string    `json:"body"`
    CreatedAt   time.Time `json:"created_at"`
}

type Service struct{ db *pgxpool.Pool }
func NewService(db *pgxpool.Pool) *Service

// Create resolve a task (visibilidade + carve-out), valida o body, insere.
func (s *Service) Create(ctx, workspaceID, userID, role, taskID, body string) (Comment, error)
// List devolve a thread da task visível, ASC por created_at.
func (s *Service) List(ctx, workspaceID, userID, taskID string) ([]Comment, error)
// Delete remove se autor ou owner; senão 403; comment fora da task → 404.
func (s *Service) Delete(ctx, workspaceID, userID, role, taskID, commentID string) error
```

### D-02 — o problema do `RequireWriteRole` global (decisão central)

`auth_routes.go` aplica `wr.Use(RequireWriteRole)` no **subrouter inteiro** do workspace
(`/api/v1/workspaces/{workspaceId}`): **todo** `POST`/`PATCH`/`PUT`/`DELETE` de um **viewer** é
barrado com `403` **antes** de chegar ao handler. Isso mataria o carve-out viewer-assignee (CA-14):
o `POST .../comments` de um viewer-assignee nunca chegaria ao service.

**Decisão:** as rotas de comentário **devem ficar fora do alcance do `RequireWriteRole` global**,
porque o gate de escrita delas é **condicional por task** (member ⇒ visibilidade; viewer ⇒ ser
assignee), não a regra binária "viewer nunca escreve". Duas opções avaliadas:

- **(A) Sub-grupo sem o middleware global** — montar `/tasks/{taskId}/comments` num grupo chi que
  **não herda** `RequireWriteRole` (chi: um `Group`/`Route` filho não re-aplica o middleware do pai
  — na verdade herda; logo a montagem das comments precisa ser **irmã** do subrouter que tem o
  `RequireWriteRole`, ou o `RequireWriteRole` precisa **pular** o path de comments). A forma limpa:
  manter o `RequireWriteRole` no subrouter `wr`, mas montar as rotas de comments com a verificação de
  escrita **delegada ao service** e fazer o `RequireWriteRole` **ignorar** o método nas rotas de
  comments. Isso exige tocar o middleware → **rejeitado** (regride a fase 02, CA-18.2).
- **(B, ESCOLHIDA) Montar as rotas de comments num `Route` filho do workspace que aplica RequireAuth
  + RequireMember mas NÃO RequireWriteRole.** Como `wr` já tem `RequireWriteRole` no `Use`, criamos a
  montagem das comments **antes** do `wr.Use(RequireWriteRole)` OU num **subrouter paralelo**
  `wr2 := api.Route("/workspaces/{workspaceId}/tasks/{taskId}/comments", …)` que reusa
  `ws.RequireMember` mas **não** o write-role. O service faz o gate fino (visibilidade + autor/owner +
  carve-out).

  **Forma concreta (recomendada):** em `mountAuth`, dentro do `Route("/workspaces/{workspaceId}")`,
  **antes** de `wr.Use(RequireWriteRole)`, registrar um sub-`Group` que aplica só `RequireMember`
  (já no `wr` antes do write-role `Use`) — mas chi aplica `Use` a TODO o `Route` independente da
  ordem de registro das rotas. Então a forma robusta é um **segundo `Route` irmão**:

  ```go
  // dentro de api.Route("/api/v1") {  (auth_routes.go)
  api.Route("/workspaces/{workspaceId}", func(wr chi.Router) {
      wr.Use(ws.RequireMember)
      wr.Use(RequireWriteRole)            // fase 02 — INTOCADO
      ... rotas existentes ...
  })
  if cfg.comments != nil {               // fase 19: subrouter IRMÃO, sem write-role
      api.Route("/workspaces/{workspaceId}/tasks/{taskId}/comments", func(cr chi.Router) {
          cr.Use(ws.RequireMember)       // membership sim; write-role NÃO (carve-out no service)
          cr.Get("/", cfg.comments.handleList)
          cr.Post("/", cfg.comments.handleCreate)
          cr.Delete("/{commentId}", cfg.comments.handleDelete)
      })
  }
  ```

  Esse subrouter irmão **reusa** `RequireMember` (membership + `reqctx.Membership` no contexto, com
  `Role`) e deixa a decisão de escrita pro `comments.Service`. O `RequireWriteRole` do subrouter
  original **não muda** (CA-18.2). **Armadilha:** o subrouter irmão precisa do MESMO `RequireAuth`
  (já está, pois ambos vivem dentro de `api.Route("/api/v1")` com `api.Use(svc.RequireAuth)`).

### D-03 — gate de escrita no service (visibilidade + carve-out viewer-assignee)

`Create` (e o gate de quem pode comentar) é uma **única query** que resolve a task visível e o papel
do user numa tacada (CA-11/13/14/15):

```sql
-- existsWritable: a task é visível ao user E ele pode comentar?
SELECT
  ($role <> 'viewer')                              AS member_can,   -- member/owner: basta ver
  COALESCE(t.assignee = $userID, false)            AS viewer_can    -- viewer: só se é assignee
FROM tasks t
JOIN projects p ON p.id = t.project_id
 AND (p.visibility = 'shared' OR p.owner_user_id = $userID)  -- visibilidade (fase 02)
WHERE t.workspace_id = $workspaceID AND t.id = $taskID
```

> **`COALESCE(t.assignee = $userID, false)` (NÃO `t.assignee = $userID` cru).** O caso NORMAL é
> `assignee IS NULL` (a escrita do assignee é da fase 18); `NULL = $userID` em SQL é `NULL` (não
> `false`), e um `bool` `NULL` faz o `Scan` para `*bool`/`bool` Go falhar → **500** onde o esperado é
> **403** (viewer não-assignee). O `COALESCE(..., false)` (ou `t.assignee IS NOT NULL AND t.assignee =
> $userID`) força `viewer_can = false` quando o assignee é nulo — espelha o padrão `IS NOT NULL` do
> `tasks.loadProjectGate` (fase 02). Vale pra esta query de gate do `Create` (e p/ qualquer query que
> toque `assignee` num gate de escrita).

- `ErrNoRows` (task não existe / invisível / ws alheio) → `domain.NotFound("task não encontrada")`
  (CA-11/12/15/16). **Mesma mensagem** de task inexistente — não vaza.
- Linha encontrada: `canComment := (role != 'viewer') || viewerCan`. `false` → `domain.Forbidden`
  (CA-13). `true` → segue pro INSERT.
- O carve-out é **escopado por task** (a query é por `t.id` e `t.assignee = user`): comentar em outra
  task onde o viewer não é assignee re-roda a query e dá `false` → 403 (CA-14, "só nessa task").
- `workspace_id` do INSERT = o **da rota** (`reqctx.Membership.WorkspaceID`), não do payload (CA-16).
  O `body` entra **com `TrimSpace`** (CA-05) após `validBody`.

`validBody`:
```go
func validBody(b string) (string, error) {
    t := strings.TrimSpace(b)
    if t == "" { return "", domain.Validation("comentário não pode ser vazio") }
    if utf8.RuneCountInString(t) > 4000 { return "", domain.Validation("comentário ≤ 4000 caracteres") }
    return t, nil
}
```

`List` — task visível primeiro (mesmo JOIN; invisível → `NotFound`), depois a thread:
```sql
SELECT c.id, c.task_id, c.workspace_id, c.author_id, u.name, c.body, c.created_at
  FROM task_comments c
  JOIN users u ON u.id = c.author_id
 WHERE c.workspace_id = $workspaceID AND c.task_id = $taskID
 ORDER BY c.created_at, c.id      -- ASC determinístico (CA-06)
```
Mas a **visibilidade** precisa ser checada ANTES (senão um GET de task invisível listaria — CA-12).
Forma: rodar a query de visibilidade da task (sub-`existsVisible`, sem o carve-out — leitura é
liberada a qualquer membro que VÊ a task) e, se ok, listar. `List` NÃO exige carve-out (qualquer
membro que vê a task lê a thread — matriz §8 "Ver … = todos leitura"). O `WHERE c.task_id` casado com
a task já visível garante CA-06.1/CA-16.

`Delete` — resolve o comentário **casado com a task da rota** e com a visibilidade:
```sql
SELECT c.author_id
  FROM task_comments c
  JOIN tasks t   ON t.id = c.task_id AND t.workspace_id = $workspaceID
  JOIN projects p ON p.id = t.project_id
   AND (p.visibility = 'shared' OR p.owner_user_id = $userID)
 WHERE c.id = $commentID AND c.task_id = $taskID AND c.workspace_id = $workspaceID
```
- `ErrNoRows` (id inexistente / de outra task / task invisível / ws alheio) → `domain.NotFound`
  (CA-10/11/16). `uuid.Validate(commentID)!=nil` → `NotFound` antes do SQL (espelha `tasks.Get`).
- Achou: `canDelete := author_id == userID || role == "owner"`. `false` → `domain.Forbidden`
  (CA-09/14.1). `true` → `DELETE FROM task_comments WHERE id=$1 AND workspace_id=$2` → `204`.
- Idempotência (CA-07): o `SELECT` casa o id; 2º DELETE não acha → `NotFound`/404. (Não há "DELETE
  silencioso já-apagado" porque o id já não existe.)

### D-04 — sem transação pesada

Create/Delete são single-statement após o gate (uma query de gate + um INSERT/DELETE). NÃO há
recálculo de objetivo nem advisory lock (comentário não afeta status de objetivo — diferente de
task). Mantém leve (RNF-01). Concorrência: dois comentários simultâneos na mesma task são INSERTs
independentes (sem corrida); apagar um comentário já apagado cai no `NotFound`.

## 4. Backend — handlers HTTP (`comments_handlers.go`)

Espelha `tasks_handlers.go` (tradução fina; tudo decidido no service):

```go
type Comments struct{ Svc *comments.Service }
func WithComments(c Comments) Option

// POST  body {"body":"texto"}; member/viewer-assignee em task visível.
func (c *Comments) handleCreate(w, r)   // 201 + Comment | 400 | 403 | 404 | 401
// GET   lista ASC
func (c *Comments) handleList(w, r)     // 200 {"comments":[...]} | 404 | 401
// DELETE
func (c *Comments) handleDelete(w, r)   // 204 | 403 | 404 | 401
```

- `workspaceId` e `taskId` vêm do path (`chi.URLParam`); o `commentId` no DELETE idem.
- `reqctx.MembershipFrom(ctx)` dá `WorkspaceID` + `Role`; `reqctx.UserID(ctx)` o autor.
- Erros do service → `writeDomainError` (o mapeador canônico de `core_routes.go`, reusado).
- `decodeJSON` (convenção INDEX) pro corpo do POST.

## 5. Contratos de API (REST)

> Todos atrás de `RequireAuth` (401 sem sessão — CA-17) e `RequireMember` (404 se não-membro do ws —
> herdado). **Não** atrás de `RequireWriteRole` (D-02; gate de escrita no service).

### `GET /api/v1/workspaces/{workspaceId}/tasks/{taskId}/comments`
- **200** `{"comments":[{id,task_id,workspace_id,author_id,author_name,body,created_at}, …]}` (ASC).
  Vazio → `{"comments":[]}`.
- **404** task invisível/inexistente/ws alheio (não vaza). **401** sem sessão.

### `POST /api/v1/workspaces/{workspaceId}/tasks/{taskId}/comments`
- Body: `{"body":"texto do comentário"}`.
- **201** `{id,task_id,workspace_id,author_id,author_name,body,created_at}` (body trimado).
- **400** `validation_error` (body vazio/só-espaço/> 4000 runes, ou JSON inválido).
- **403** `forbidden` (viewer não-assignee na task — CA-13).
- **404** task invisível/inexistente/ws alheio. **401** sem sessão.

### `DELETE /api/v1/workspaces/{workspaceId}/tasks/{taskId}/comments/{commentId}`
- **204** (autor ou owner). **403** `forbidden` (member/viewer apagando o de outro — CA-09/14.1).
- **404** comment inexistente / de outra task / task invisível / ws alheio. **401** sem sessão.

**Sem mensagens WebSocket** (tempo real fora — Fora de escopo).

## 6. Frontend — thread no detalhe da task

### `lib/api.ts` (estende)
- Tipo `Comment { id; task_id; workspace_id; author_id; author_name; body; created_at }`.
- `listComments(wsId, taskId): Promise<Comment[]>` (desembrulha `{comments}`),
  `createComment(wsId, taskId, body): Promise<Comment>`, `deleteComment(wsId, taskId, commentId)`.

### `lib/queries/comments.ts` (NOVO)
- `commentsKey(wsId, taskId)`; `useComments(wsId, taskId, enabled)` (query); `useCreateComment` /
  `useDeleteComment` (mutations que `invalidateQueries(commentsKey)` no `onSuccess`). Padrão idêntico
  a `lib/queries/tasks.ts` (não toca os hooks de task — RNF-04).

### `components/board/TaskThread.tsx` (NOVO)
- Props: `{ wsId, taskId, canComment }`. Lista os comentários (autor `author_name` + `created_at`
  formatado por fuso local via `lib/format` + `body`), input de novo comentário + botão "Comentar"
  (desabilitado se body vazio ou `!canComment`); cada comentário do **próprio autor** mostra um ✕
  (apagar) — o front resolve "sou autor" comparando `author_id` com o user atual
  (`useAuthStore`/`/me`); o owner vê o ✕ em todos (gating de UI; a verdade é no backend — CA-08/09).
- `role="log"`/lista acessível; ordem ASC (o backend já entrega ASC).
- Estado de carregamento/erro discreto (sem bloquear). NÃO faz polling (tempo real fora).

### `can_comment` no front (gating de UI — CA-19.1)
- O board (`ProjectBoard`/`TaskModal`) hoje só conhece `can_write` (do `/board`). Pra o gating fino
  do viewer-assignee, a UI precisa saber se **pode comentar nesta task**: `can_comment = can_write ||
  (task.assignee === currentUserId)`. `can_write` cobre member/owner; o `||` cobre o viewer-assignee.
  - O `assignee` na `BoardTask` é **lido** do contrato (a task já tem `assignee` no JSON — fase 02; o
    board precisa expô-lo — ver D-05). O `currentUserId` vem do `/me`/`useAuthStore`.
  - **Degradação:** se o backend negar (403), o input mostra o erro; o gating de UI é conveniência, a
    autoridade é o backend.

### D-05 — `assignee` no board/task do front
- A `BoardTask` (contrato `/board`) **pode não expor** `assignee` hoje. Para o `can_comment` do
  viewer-assignee no front, a T-08 verifica e, se faltar, **adiciona `assignee` ao shape lido**
  (campo já existe no JSON da task da fase 02; o board é que pode não propagar). Isso é **aditivo** no
  TS (não muda o backend do board — CA-18). Se a fase 18 já tiver exposto `assignee` no board, reusa.

### `TaskModal.tsx` (estende — monta a thread)
- No **modo Editar** (a task existe → tem id), renderiza `<TaskThread wsId taskId={task.id}
  canComment={canComment} />` abaixo do form de metadados. No modo **criar** (sem id) NÃO mostra
  thread (não há task ainda). `ProjectBoard` passa `canComment` calculado.

## 7. Dependências da fase + paralelização (`MILESTONE-2.md §9`)

- **Depende de:** fases **02 e 12** (tasks, visibilidade `visibleWhere`/`loadProjectGate`, board/
  TaskModal). Ambas **já construídas** (INDEX). **NÃO** depende dura da fase 18: o carve-out lê
  `tasks.assignee` (coluna existe desde `0008`); os testes/E2E **semeiam o assignee por SQL**. Se a
  18 rodar antes, o assignee terá API; se não, o thread funciona igual (viewer sem assignee = sem
  carve-out, degradação segura).
- **PARALELIZÁVEL: MUITO** (`MILESTONE-2.md §9`). Pacote backend novo (`internal/comments`), migração
  própria (`0019`), rotas novas (subrouter irmão), UI quase só nova (`TaskThread`,
  `lib/queries/comments.ts`). **Contenção mínima** com outras worktrees:
  - `core_routes.go`/`auth_routes.go`: a fase 19 adiciona um subrouter **irmão** + um `cfg.comments`
    no `routerCfg` — **não edita** o `mountCore`/`RequireWriteRole` existente (linhas distintas;
    merge trivial). A fase 18 toca `tasks_handlers.go`/`tasks/service.go` (assignee no PATCH/POST) —
    **arquivos diferentes** dos da 19.
  - `lib/api.ts`: append de tipos/funções (a 18 também faz append; conflito textual possível mas
    trivial — blocos distintos).
  - `TaskModal.tsx`: a 18 adiciona o picker de assignee; a 19 monta a thread. **Contenção possível**
    no mesmo arquivo — coordenar (regiões distintas do componente). Worktree: ok com revisão de
    merge.
- **Ordem recomendada:** pode rodar em worktree paralela com 17/18/20 (`MILESTONE-2.md §9`). A
  consistência das migrações fixas (0019) impede colisão de número.

## 8. Armadilhas conhecidas (vetadas)

- **NÃO** montar as rotas de comments atrás do `RequireWriteRole` global (D-02) — mataria o carve-out
  viewer-assignee (POST de viewer → 403 antes do service). Subrouter **irmão** sem o write-role; gate
  fino no service.
- **NÃO** editar o `RequireWriteRole`/`middleware_role.go` (CA-18.2) — ele segue barrando viewer em
  toda escrita de domínio (tasks/objetivos/projetos/clientes). A exceção é **escopada no service de
  comments**, não global.
- **NÃO** confiar no `workspace_id`/`task_id` do **payload** — usar sempre os do **path**
  (`reqctx`/`chi.URLParam`); o `workspace_id` do INSERT é o da rota (CA-16/16.1).
- **NÃO** pular a checagem de visibilidade no `List`/`Delete` (CA-12/CA-10) — listar/apagar comentário
  de task invisível tem que dar 404 igual ao POST. O JOIN de visibilidade é em TODA query.
- **NÃO** validar o body só por `len(bytes)` (CA-04) — usar `utf8.RuneCountInString` (emoji/acento);
  e **TrimSpace ANTES** de medir/gravar (CA-03/05) — senão "   " passa como não-vazio.
- **NÃO** vazar a thread de outra task no `List` (CA-06.1) — `WHERE c.task_id = $taskID` casado com a
  task **já resolvida como visível**; mutação removendo o `task_id` deve matar teste.
- **NÃO** deixar o viewer-assignee **apagar comentário de terceiro** (CA-14.1) — `canDelete` é
  `autor || owner`, nunca `assignee`. O carve-out é só de **criar/comentar**.
- **NÃO** tocar o Hub/`realtime.ts`/`BroadcastNotice` (tempo real fora) — comentário não notifica nem
  faz broadcast nesta fase.
- **NÃO** mexer em `tasks.notes` (CA-18.1) — a descrição da task segue independente; o thread é
  paralelo.
- **NÃO** adicionar dependência nova (RNF-03) — reusa `pgx`/`chi`/`uuid`/`domain`/TanStack.

## 9. Ordem de execução (ver tasks.md)

Backend primeiro (migração → service Create/List/Delete TDD com gate+carve-out → handlers → wiring do
subrouter irmão), depois o frontend (api.ts → queries → TaskThread → montar no TaskModal + can_comment),
depois E2E, verificação adversarial e gate. Commit por tarefa. `make check-full` é o gate final.
