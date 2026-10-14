# plan.md — Fase 18 · Cross-assign & visões

> Como construir a spec. Decisões de arquitetura (D-xx) + migração + contratos de API + ordem de
> tarefas. **Reusa** o `tasks.Service`/`tasks_handlers.go` (fase 02/12), o `RequireWriteRole` (fase
> 02), o `hub.BroadcastNotice` + `push.Sender.SendToUser` + `noticeStore`/`realtime.ts` (fase 16), e
> o `workspace.MemberRole` (fase 01/02). **Adiciona** índices (0018), os campos no Create/Patch, a
> validação de membership, o carve-out viewer-assignee escopado, as duas rotas de visão e a UI.

---

## 1. Arquitetura geral

```
Backend:
  db/migrations/0018_task_assignment_idx.sql   ← SÓ índices em assignee + requested_by (sem coluna)
  internal/tasks/service.go (existe)           ← + Assignee/RequestedBy em CreateInput/PatchInput
                                                   + validateMember(assignee/requested_by)
                                                   + carve-out viewer-assignee (Patch, escopado)
                                                   + detecção de "mudança real de assignee" → CutEvent-like
  internal/tasks/assigned.go        (NOVO)     ← queries cross-ws: Assigned(userID) / Requested(userID)
     assigned_test.go                          ← TDD das visões (isolamento, visibilidade, ordem)
  internal/api/tasks_handlers.go (existe)      ← decodifica assignee/requested_by (RawMessage tri-state);
                                                   dispara onAssign pós-commit (notice + push)
  internal/api/assigned_routes.go   (NOVO)     ← GET /api/v1/tasks/assigned + /tasks/requested (por-usuário)
     assigned_routes_test.go                   ← 200/401, shape, isolamento
  internal/api/auth_routes.go (existe)         ← monta as 2 rotas no /api/v1 (por-usuário, antes do ws)
  cmd/server/main.go (existe)                  ← injeta onAssign (hub.BroadcastNotice + go sender.SendToUser)

Frontend:
  lib/api.ts (existe)                          ← + assignee/requested_by em TaskInput; getAssignedTasks/
                                                   getRequestedTasks + tipos AssignedTask
  lib/noticeStore.ts (existe)                  ← + ramo kind 'task_assigned' (banner) — aditivo
  components/board/TaskModal.tsx (existe)      ← + selects de Responsável (assignee) e Solicitante
                                                   (requested_by), populados pelos membros do ws
  lib/queries/members.ts (existe? senão NOVO)  ← lista de membros do ws p/ os selects (reusa GET ws)
  routes/cockpit/MyTasks.tsx        (NOVO)     ← "Minhas tasks" (assigned) + "Pedidas por mim" (requested)
  App.tsx (existe)                             ← + rotas /minhas-tarefas (e aba "pedidas")
  components/SideNav.tsx (existe)              ← + item de nav "Minhas tasks"
```

## 2. Migração — `0018_task_assignment_idx.sql` (dona: fase 18)

> **Número:** a fase 17 é dona de `0017_invitations`. A T-01 fixa `0018` lendo
> `backend/internal/db/migrations/` (próximo livre). **NÃO** há `ALTER TABLE ... ADD COLUMN`: as
> colunas `assignee`/`requested_by` já existem em `0008_tasks.sql` (uuid nullable, FK a `users(id)`).

```sql
-- 0018_task_assignment_idx — fase 18 (plan §2): índices p/ as visões cross-workspace.
-- As colunas tasks.assignee / tasks.requested_by já existem (0008_tasks). Sem coluna nova.
CREATE INDEX idx_tasks_assignee     ON tasks (assignee)     WHERE assignee IS NOT NULL;
CREATE INDEX idx_tasks_requested_by ON tasks (requested_by) WHERE requested_by IS NOT NULL;
```

- Índices **parciais** (`WHERE ... IS NOT NULL`): a vasta maioria das tasks tem os campos nulos; o
  índice só precisa cobrir as atribuídas — menor e mais barato pra `GET /tasks/assigned`.
- INDEX.md: registrar **0018 dona da fase 18** (número final confirmado na T-01). Atualizar a tabela
  de migrações donas com o nome lógico.

## 3. Backend — service (`tasks/service.go`) + handlers

### D-01 — campos aditivos no Create/Patch

`CreateInput` ganha `Assignee *string`, `RequestedBy *string`. `PatchInput` ganha (tri-state, padrão
do `due_date`/`objective_id`): `AssigneeSet bool` + `Assignee *string`; `RequestedBySet bool` +
`RequestedBy *string`. O INSERT da `Create` passa a gravar `assignee`/`requested_by`; o UPDATE da
`Patch` aplica a regra tri-state (set+nil = limpa; set+valor = define; não-set = mantém). O handler
`tasks_handlers.go` ganha, no `patchTaskBody`, `Assignee json.RawMessage` e `RequestedBy
json.RawMessage` (mesma mecânica do `DueDate`/`ObjectiveID` atuais — `nil`=ausente, `"null"`=limpa).

### D-02 — validação de membership (CA-04/05/12)

Função `validateMember(ctx, q, workspaceID, *uuid) error`:
- `nil` → ok (sem assignee/requested_by).
- uuid malformado → `domain.Validation("assignee deve ser um membro do workspace")` (CA-05).
- `SELECT 1 FROM workspace_members WHERE workspace_id=$1 AND user_id=$2` — sem linha →
  `domain.Validation(...)` **genérica** (não vaza existência/workspace — CA-04/12). A mensagem é a
  mesma pra "não existe" e "existe mas em outro ws".
- Roda **dentro da mesma transação** da escrita, **antes** do INSERT/UPDATE de fato, **depois** do
  `loadProjectGate` (visibilidade/permissão primeiro → 404/403 ganham de 400 — CA-12).
- Reusa o pool/`tx` já abertos no `Create`/`Patch` (a `Patch` já abre `tx` no caminho de transição;
  no caminho benigno, se só assignee/requested_by mudou, o service abre `tx` pra validar+gravar —
  ver D-03).

### D-03 — onde o carve-out viewer-assignee vive (CA-10/11)

O `RequireWriteRole` (middleware, subrouter) barra **viewer** em **todo** POST/PATCH/PUT/DELETE
**antes** de chegar no handler. Pra o viewer-assignee mudar o status, há duas opções:

- **Opção A (escolhida): mover a exceção pro service, mantendo o middleware como pré-filtro, MAS
  abrindo um buraco escopado.** Problema: o middleware já respondeu 403 antes do service rodar. Logo
  o `RequireWriteRole` precisa **deixar passar** o PATCH de task de um viewer e delegar a decisão
  final ao service. Pra **não** afrouxar o gate pro resto (clients/projects/objectives), a exceção é
  **estreita**: o `RequireWriteRole` libera o viewer **somente** em `PATCH /workspaces/{id}/tasks/
  {taskId}` (método + shape de rota), e o `tasks.Patch` aplica a regra fina (CA-10): viewer só pode
  se for o assignee **daquela** task **e** o patch só toca `status`. Qualquer outra coisa → o service
  responde `domain.Forbidden(...)` (403). Os outros recursos e os outros métodos de task (POST,
  DELETE) seguem 403 no middleware.
  - Implementação: o `RequireWriteRole` ganha um **bypass condicional** — se `role=="viewer"` e a
    rota casa `PATCH /tasks/{taskId}` (chi route pattern), **não** barra; senão, barra como hoje. O
    service é a autoridade final. (Mantém a assinatura do middleware — CA-24 — só estende o switch.)
  - **Regra fina no service** (`tasks.Patch`, no topo, quando `role=="viewer"`): carrega a task
    (`Get`, já aplica visibilidade → 404 se invisível); se `task.Assignee == nil || *task.Assignee
    != userID` → `domain.Forbidden`. Se for o assignee, **só** aceita o patch se **apenas** `Status`
    está setado (todos os outros campos de `PatchInput` nil/false) — senão `domain.Forbidden`. Não
    chama `loadProjectGate` (o viewer não tem can_write; a autoridade aqui é "ser o assignee"), **MAS
    herda o bloqueio de projeto arquivado** (CA-10.1): faz uma checagem leve do `archived_at` do
    projeto da task (`SELECT archived_at FROM projects WHERE id=$1` — barato, ou reusa o campo se o
    `Get` já o trouxer) e, se arquivado, responde `domain.Validation/invalid_reference` (**400**,
    igual ao `loadProjectGate(blockArchived=true)` que owner/member levam). Isso impede que o carve-
    out fique mais permissivo que owner/member num projeto arquivado.
- **Opção B (rejeitada): subrouter separado pra `PATCH /tasks/{id}` sem `RequireWriteRole`.** Mais
  invasivo no `auth_routes.go`/`core_routes.go` e duplica a montagem das rotas de task. A A é
  cirúrgica.

> **Decisão registrada:** o carve-out é **por-task + só-status**, decidido no service; o middleware
> só abre o buraco de *rota* (PATCH task) pro viewer, nunca pros outros recursos/métodos. Mutação que
> ampliasse (qualquer task, ou qualquer campo) tem que matar um teste (CA-11).

### D-04 — detecção de "mudança real de assignee" + fan-out (CA-13/14/15)

O service (`Create` e `Patch`) calcula, **dentro da tx**, o `assignee` **antes** e **depois**. Após
o `Commit`, devolve ao handler um sinal `assignedTo *string` (o novo assignee) **somente** quando:
houve mudança real (`old != new`, ambos comparados por valor) **e** `new != nil` **e** `*new !=
userID` (não auto-set). O service NÃO importa `realtime`/`push` (sem ciclo): devolve o sinal; o
**handler** (camada api) faz o fan-out, espelhando o `onCut` do `guard` (fase 16, `cmd/server/
main.go`). Padrão: o `Core` ganha um campo opcional `OnAssign func(targetUserID, taskID, byUserID
string)` injetado no boot = `func(...){ hub.BroadcastNotice(target, {...}); go sender.SendToUser(...)
}`. `OnAssign == nil` → no-op (testes do service não precisam do hub).

- `Create.return` e `Patch.return` ganham, alé da `Task`, um `*assignSignal` (interno) que o handler
  consome. Alternativa mais simples: o handler relê o assignee antigo? Não — o service é quem está na
  tx e sabe o `old`. O service expõe via um campo no retorno ou um segundo valor (`(Task, *Notify,
  error)`). **Decisão:** segundo valor de retorno `notify *AssignNotify` (`{TargetUserID, TaskID}`)
  nas duas funções; nil = sem notificação. O handler chama `c.OnAssign` se `notify != nil`.

### D-05 — visões cross-workspace (`tasks/assigned.go`)

```go
type AssignedTask struct {
    Task                              // embute o Task atual (assignee/requested_by incluídos)
    Project   struct{ ID, Name, Color string } `json:"project"`
    Workspace struct{ ID, Name string }        `json:"workspace"`
    Client    *string                          `json:"client"` // nome do cliente ou null
}
func (s *Service) Assigned(ctx, userID string) ([]AssignedTask, error)
func (s *Service) Requested(ctx, userID string) ([]AssignedTask, error)
```

Query (uma só, sem N+1 — RNF-04):
```sql
SELECT <task cols>, p.id, p.name, p.color, w.id, w.name, c.name
  FROM tasks t
  JOIN projects   p ON p.id = t.project_id
  JOIN workspaces w ON w.id = t.workspace_id
  JOIN workspace_members wm ON wm.workspace_id = t.workspace_id AND wm.user_id = $1  -- membership
  LEFT JOIN clients c ON c.id = p.client_id
 WHERE t.assignee = $1                                   -- (ou t.requested_by = $1 na outra visão)
   AND (p.visibility = 'shared' OR p.owner_user_id = $1) -- visibilidade (CA-20)
 ORDER BY t.due_date NULLS LAST, t.created_at, t.id      -- estável (CA-22)
```
- O `JOIN workspace_members ... user_id = $1` garante a participação (CA-21) — sem ele, uma task
  cujo workspace o user **deixou** vazaria. O `WHERE t.assignee = $1` + visibilidade são os outros
  dois filtros que a mutação-teste deve provar load-bearing.
- Lista vazia → `[]` (handler garante slice não-nil).

## 4. Contratos de API

### `POST /api/v1/workspaces/{id}/tasks` (estende — CA-02)
Body ganha `assignee?: string|null`, `requested_by?: string|null`. Validações: 400
`validation_error` se algum for uuid de não-membro/malformado (CA-04/05); 404 se projeto invisível
(CA-12, gate de projeto primeiro); 403 viewer (CA-09). Resposta: `Task` (shape inalterado, com os
campos preenchidos). Pós-commit: notifica se set-por-terceiro (CA-13).

### `PATCH /api/v1/workspaces/{id}/tasks/{taskId}` (estende — CA-03/06)
Body ganha `assignee` e `requested_by` como **tri-state** (`json.RawMessage`: ausente mantém, `null`
limpa, uuid define). Mesmas validações. Carve-out viewer-assignee (CA-10): viewer-assignee patchando
**só status** → 200; senão 403. Notifica se mudança real de assignee por terceiro (CA-13/15).

### `GET /api/v1/tasks/assigned` (NOVO — CA-18) · POR USUÁRIO
Sem param de workspace (cruza workspaces). Atrás do `RequireAuth`. Resposta:
```json
{ "tasks": [ { "id":"…","project_id":"…","status":"todo", … ,
              "project":{"id","name","color"},
              "workspace":{"id","name"},
              "client":"Acme"|null } ] }
```
401 sem sessão.

### `GET /api/v1/tasks/requested` (NOVO — CA-19) · POR USUÁRIO
Idêntico ao assigned, mas filtra `requested_by = eu`. Mesmo shape. 401 sem sessão.

### Mensagem WebSocket (reusa fase 16 — CA-13/17)
`{"type":"notice","payload":{"kind":"task_assigned","task_id":"<uuid>","by":"<uuid>"}}` — empurrado
por `hub.BroadcastNotice(targetUserID, payload)`. O front (`noticeStore.pushNotice`) trata o
`kind:"task_assigned"` (banner) além do `timer_auto_cut`.

## 5. Pontos de integração com o baseline

- **`tasks/service.go`** (fase 02/12) — estendido (campos + validação + carve-out + visões). O
  recálculo de objetivo (`recalcObjectiveStatus`, `objective_link.go`) é **intocado**: a mudança de
  `assignee`/`requested_by` **não** afeta o status do objetivo (não é status nem objetivo de task) →
  cai no caminho "edição benigna" do `Patch` quando só esses campos mudam (CA-23). **Atenção:** hoje
  o caminho benigno faz um UPDATE simples **fora** de transação; pra validar membership +
  notificação atômicas, quando `assignee`/`requested_by` entram no patch o service abre `tx` (mesmo
  no caminho sem mudança de status/objetivo). Garantir que o UPDATE benigno preserve o não-recálculo.
- **`tasks_handlers.go`** (fase 02) — `handleCreateTask`/`handlePatchTask` decodificam os campos
  novos e disparam `c.OnAssign` pós-commit. **Contende com a fase 19** (que também toca subrotas de
  task) — ver §dependências.
- **`middleware_role.go`** (fase 02) — `RequireWriteRole` ganha o bypass condicional pro viewer em
  `PATCH /tasks/{taskId}` (D-03). Mínimo; assinatura intocada.
- **`auth_routes.go`** (fase 01) — monta `GET /tasks/assigned` e `/tasks/requested` no `/api/v1`
  **por-usuário** (junto do `mountTimer`/`mountPush`, antes do subrouter `/workspaces/{id}`).
- **`hub.go` (fase 16) / `push` (fase 16)** — **intocados**: só **chamamos** `BroadcastNotice` e
  `SendToUser` (contratos existentes). `cmd/server/main.go` injeta o `OnAssign` (espelha o `onCut`).
- **`noticeStore.ts` / `realtime.ts` (fase 16)** — `pushNotice` ganha o ramo `task_assigned`
  (aditivo); o ramo `timer_auto_cut`, o `type:"timer"` e o anti-stale **não mudam** (CA-17).
- **`TaskModal.tsx` (fase 12)** — ganha os selects de Responsável/Solicitante populados pelos membros
  do workspace (reusa o `members` que `GET /workspaces/{id}` já devolve — fase 02). **Contende com a
  fase 19** (que adiciona o thread no detalhe da task).
- **`App.tsx`/`SideNav.tsx`** — nova rota/aba. **Contende com a fase 23** (controle remoto) e a fase
  17 em `App.tsx`.

## 6. Libs / deps

- **Zero dependência nova** (Go ou JS). Tudo reusa: `google/uuid` (já no `go.mod`), `pgx`,
  `webpush-go` (fase 16, só uso indireto via `push.Sender`), TanStack Query, zustand.

## 7. Dependências da fase + paralelização (PRD §9)

- **Depende de:** fase **02** (CRUD/visibilidade de task), fase **12** (board/objetivos/TaskModal),
  fase **16** (`notice`/push/`noticeStore`/`realtime.ts`). Não depende da fase 17 pra buildar (a
  membership pode ser seeded por SQL nos testes, como hoje), mas o **uso real** dos campos brilha com
  17 (convidar gente). **Não** depende de 19/20/21.
- **PARALELIZÁVEL (worktree): SIM, com cuidado.** Migração própria (0018) e pacote de visões
  (`assigned.go`) quase só novos. **Contenções a vigiar** (PRD §9): `tasks_handlers.go` (contende com
  a **fase 19**, que adiciona subrotas de comentário sob a task) e `App.tsx` (contende com **23** e
  **17**). Estratégia: na worktree da 18, tocar `tasks_handlers.go` só no `handleCreateTask`/
  `handlePatchTask` (não na montagem de rotas, que fica em `core_routes.go`); a montagem das visões
  por-usuário vai no `auth_routes.go` (longe das subrotas de task da 19). O merge resolve com
  conflito mínimo se cada fase respeitar seu território.

## 8. Desvios / armadilhas conhecidas (vetadas)

- **NÃO** afrouxar o `RequireWriteRole` pra viewer em geral (CA-09/11) — o bypass é **só** `PATCH
  /tasks/{taskId}`, e o service é a autoridade final (viewer-assignee + só-status). Mutação que
  ampliasse mata teste.
- **NÃO** deixar o carve-out viewer-assignee mais permissivo que owner/member num projeto **arquivado**
  (CA-10.1) — o ramo viewer-assignee não chama `loadProjectGate`, então DEVE aplicar o **mesmo**
  bloqueio de `archived_at` (400 `invalid_reference`); senão owner/member levam 400 e o viewer-
  assignee gravaria status. Mutação que pulasse o check mata teste.
- **NÃO** validar membership do assignee **depois** do INSERT/UPDATE (CA-04) — validar **antes**, na
  mesma tx; senão grava um assignee inválido e tem que reverter.
- **NÃO** notificar em auto-set nem em PATCH sem mudança real (CA-14/15) — comparar `old != new` por
  **valor**; auto-set (`new == sessionUser`) nunca notifica.
- **NÃO** vazar existência de user/workspace na validação (CA-04/12) — mensagem genérica; e o gate de
  **projeto** (404) roda antes do 400 de membership.
- **NÃO** disparar `notice`/push **antes** do commit (RNF-02) — pós-commit, push em goroutine
  (best-effort fase 16); o handler não espera o push.
- **NÃO** esquecer o `JOIN workspace_members ... user_id=$me` nas visões (CA-21) — sem ele, uma task
  num workspace que o user deixou (ou nunca participou, atribuída por engano em outro tenant) vaza.
- **NÃO** quebrar o caminho "edição benigna" do `Patch` (CA-23) — mudar só `assignee`/`requested_by`
  **não** recalcula objetivo; preservar o não-recálculo (D-04 da fase 12) ao abrir a tx pra validar.
- **NÃO** mexer no `hub.go`/`push.Sender`/`BroadcastTimer`/anti-stale (CA-17/RNF-05) — só **usar** os
  contratos da fase 16.
- **NÃO** introduzir N+1 nas visões — 1 query com os joins (RNF-04).
