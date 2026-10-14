# spec.md — Fase 18 · Cross-assign & visões ("Minhas tasks" / "Pedidas por mim")

> Fonte de verdade da fase. Resolve a dor **P2** do `MILESTONE-2.md §2/§4`: "não consigo pedir/
> atribuir uma task pra outra pessoa". Liga os campos **`tasks.assignee`** e **`tasks.requested_by`**
> que **já existem** no schema (`0008_tasks.sql`) e na resposta JSON da task (`tasks.Task.Assignee`/
> `RequestedBy`), mas são **dormentes**: lidos, **nunca escritos** (`CreateInput`/`PatchInput` não os
> têm). Esta fase: aceita os dois campos no POST/PATCH de task (validados contra a membership),
> notifica o assignee quando ele é atribuído por terceiro (reusa o canal `notice` per-usuário da
> fase 16), e entrega duas visões cross-workspace: **"Minhas tasks"** e **"Pedidas por mim"**.
>
> **Decisões travadas (HITL, `MILESTONE-2.md §7`):**
> 1. **Cross-assign = liga `assignee`/`requested_by`** na API de tasks (tri-state como `due_date`/
>    `objective_id`; null limpa); uuids **validados contra a membership do workspace da task** (atribuir
>    a não-membro → 400 `validation_error`, sem vazar quem existe).
> 2. **Permissão** (matriz `§8`): member atribui/pede; **viewer 403** ao setar; **viewer PODE ser
>    assignee** (carve-out) e, como assignee, **muda o status só da task dele** + comenta nela.
> 3. **Notifica o assignee** em set por terceiro (não em auto-set) via `notice` per-usuário **+** Web
>    Push (fase 16). Sem broadcast por workspace. Novo kind `task_assigned`.
> 4. **Visões dedicadas** cross-workspace: `GET /api/v1/tasks/assigned` e `/api/v1/tasks/requested`
>    (rotas POR USUÁRIO, como `/api/v1/timer` — não aninhadas num workspace).
> 5. **Migração `0018` = só índices** (`assignee`, `requested_by`) pra a query cross-workspace.
>    **SEM coluna nova** (as colunas já existem em `0008`).

---

## 1. Objetivo

Tornar `assignee`/`requested_by` campos vivos: o Diogo (ou qualquer member) pode **atribuir** uma
task a um colega (`assignee`) e marcar quem **pediu** (`requested_by`); o alvo de uma atribuição
recebe um **aviso** (banner in-app + push) e encontra a task numa visão **"Minhas tasks"** que
cruza todos os workspaces que ele participa; quem distribui acompanha em **"Pedidas por mim"**.

Tudo **respeitando**: isolamento multi-tenant (filtro duplo `workspace_id` + visibilidade de
projeto), a matriz de permissões `§8` (com o único carve-out viewer-assignee), e a não-regressão
das fases 02 (CRUD de task) e 12 (board/objetivos). A propagação ao vivo (a task aparecendo na hora
no device do colega) **fica fora** (decisão HITL `MILESTONE-2.md §3`): a visão converge **no
reload**; só a **notificação** chega ao vivo, pelo `notice` per-usuário que já existe.

## 2. Definições precisas

- **assignee** — o user **responsável** por executar a task (quem recebe). uuid de um `users.id`
  que **é membro** do workspace da task. `null` = sem responsável.
- **requested_by** — o user que **pediu/solicitou** a task. uuid de um membro do workspace da task.
  `null` = sem solicitante. (Distingo de `created_by`, que é imutável e = quem criou a linha.)
- **set por terceiro** — uma operação que define `assignee = X` onde **X ≠ o user da sessão** que
  fez a chamada. Só essa dispara notificação (auto-atribuição não notifica — CA-14).
- **mudança real de assignee** — o valor **efetivo** de `assignee` após a operação difere do valor
  **antes** dela. Um PATCH que reenvia o mesmo `assignee` (ou não toca o campo) **não** é mudança
  real → não notifica (CA-15).
- **viewer-assignee** — um membro com role `viewer` que é o `assignee` **daquela** task específica.
  O carve-out de escrita é **escopado a essa task** (CA-09/10), nunca global.
- **membership** — vínculo `(workspace_id, user_id)` em `workspace_members` (role owner|member|
  viewer); `workspace.MemberRole(ws,user)` devolve a role ou `""` (não-membro).

## 3. User stories

- **US-01** — Como member, quero **atribuir** uma task a um colega (definir o `assignee`), pra
  delegar quem executa.
- **US-02** — Como member, quero registrar **quem pediu** a task (`requested_by`), pra rastrear a
  origem da demanda.
- **US-03** — Como pessoa atribuída, quero **ser avisado** (banner no app + push) quando alguém me
  atribui uma task, pra não depender de ficar olhando o board.
- **US-04** — Como pessoa atribuída, quero uma visão **"Minhas tasks"** que junte tudo o que é meu
  **em qualquer workspace** que eu participe, pra ter uma fila única.
- **US-05** — Como quem distribui, quero **"Pedidas por mim"** pra acompanhar as tasks que eu
  solicitei a outros.
- **US-06** — Como **viewer** (ex.: namorada num workspace de leitura), quero **receber uma task** e
  **mudar o status só dela** (marcar feita), sem ganhar permissão de escrita no resto.

## 4. Critérios de aceite (EARS — cada um vira teste)

### Migração & modelo de dados

- **CA-01** (migração) — a fase 18 É dona da migração **`0018`** (nome lógico `task_assignment_idx`;
  número fixado na T-01 lendo `backend/internal/db/migrations/` — próximo livre após `0017`*).
  Ela **só cria índices**: `idx_tasks_assignee` em `tasks (assignee)` e `idx_tasks_requested_by` em
  `tasks (requested_by)`. **NENHUMA coluna nova** (as colunas `assignee uuid REFERENCES users(id)` e
  `requested_by uuid REFERENCES users(id)` já existem em `0008_tasks`). Os índices podem ser
  parciais (`WHERE assignee IS NOT NULL` / `WHERE requested_by IS NOT NULL`) pra a varredura
  cross-workspace das visões. Teste: índices existem; as colunas seguem como em `0008` (uuid
  nullable, FK a `users`).
  *(\*as fases anteriores da M2 são donas de `0017` (17, invitations); o número segue a sequência
  real presente.)*

### Backend — aceitar assignee/requested_by no POST/PATCH

- **CA-02** (Create aceita) — `POST /workspaces/{id}/tasks` DEVE aceitar `assignee` e/ou
  `requested_by` (uuids) no corpo. Quando ausentes, a task nasce com ambos `null` (comportamento
  atual, não regride). `CreateInput` ganha `Assignee *string`, `RequestedBy *string`; o INSERT
  grava os dois.
- **CA-03** (Patch tri-state) — `PATCH /workspaces/{id}/tasks/{taskId}` DEVE tratar `assignee` e
  `requested_by` como **tri-state** (igual a `due_date`/`objective_id` hoje): campo **ausente** →
  mantém o valor; valor **`null`** → limpa; valor uuid → define. O handler usa `json.RawMessage` pra
  distinguir ausente de null (espelha o `patchTaskBody` atual); `PatchInput` ganha `AssigneeSet
  bool`/`Assignee *string` e `RequestedBySet bool`/`RequestedBy *string`.
- **CA-04** (validação de membership) — QUANDO `assignee` (ou `requested_by`) é um uuid que **NÃO é
  membro do workspace da task**, a resposta DEVE ser **400 `validation_error`** (a operação não
  grava nada). Aplica-se tanto a um uuid de um user de **outro** workspace quanto a um uuid que não
  existe em `users`. A mensagem **não vaza** se o user existe ou em qual workspace está (mensagem
  genérica tipo "assignee deve ser um membro do workspace"). A validação roda **antes** do
  INSERT/UPDATE.
- **CA-05** (uuid malformado) — `assignee`/`requested_by` com uuid **mal-formado** (não 8-4-4-4-12)
  → **400 `validation_error`** (não 500), seguindo a convenção dos outros campos uuid (`objective_id`).
- **CA-06** (null limpa) — um PATCH com `assignee: null` numa task que tinha assignee DEVE deixar
  `assignee = null` e a resposta refletir `"assignee": null`. Idem `requested_by`.
- **CA-07** (campos no contrato de resposta) — a `Task` retornada de POST/PATCH/GET/List **já**
  expõe `assignee` e `requested_by` (existem no JSON hoje). Esta fase **não muda o shape** — só passa
  a poder preenchê-los. (RNF de não-regressão: o JSON da task da fase 02/12 não ganha nem perde
  chave.)

### Permissões (matriz `MILESTONE-2.md §8`)

- **CA-08** (member seta) — um **member** (ou owner) DEVE conseguir setar `assignee`/`requested_by`
  em POST e PATCH (sujeito ao gate de projeto da fase 02/12 — visibilidade/`can_write`). O fluxo
  passa pelo `RequireWriteRole` atual sem mudança.
- **CA-09** (viewer 403 ao setar) — um **viewer** que tenta **setar** `assignee` ou `requested_by`
  (em POST ou PATCH, em qualquer task) DEVE receber **403** — herda o `RequireWriteRole` (viewer é
  barrado em todo método de escrita). Mutação que removesse esse gate deve quebrar um teste.
- **CA-10** (carve-out viewer-assignee — status) — um **viewer** que é o `assignee` **daquela** task
  DEVE conseguir **mudar o status** dela (PATCH só do `status`) — exceção **escopada por-task** ao
  `RequireWriteRole`. Precisão:
  - A exceção vale **somente** quando: a role é `viewer` **E** o `assignee` atual da task (lido do
    banco, sob o gate de visibilidade) **==** o user da sessão **E** o PATCH altera **apenas
    `status`** (nenhum outro campo: nem name, due_date, notes, objective_id, assignee, requested_by).
  - Fora desse recorte o viewer segue **403** (CA-11). Owner/member não dependem da exceção (já podem).
- **CA-10.1** (carve-out herda o bloqueio de projeto arquivado) — o carve-out viewer-assignee **NÃO**
  pode ser mais permissivo que owner/member: QUANDO o projeto da task está **arquivado**
  (`projects.archived_at IS NOT NULL`), um PATCH de status do viewer-assignee DEVE responder **400
  `invalid_reference`** — **o mesmo** resultado que owner/member levam pelo `loadProjectGate
  (blockArchived=true)` da fase 02 (CA-15.2). Como o ramo viewer-assignee **não** chama
  `loadProjectGate` (a autoridade é "ser o assignee", D-03), ele DEVE aplicar uma checagem leve do
  `archived_at` do projeto da task (1 `SELECT archived_at` — a task já foi carregada pelo `Get`) e
  abortar com 400 antes de gravar o status. Mutação que pulasse esse bloqueio (deixando o viewer-
  assignee mudar status num projeto arquivado) deve quebrar um teste.
- **CA-11** (carve-out NÃO vaza pra outras tasks/campos) — um **viewer** que é assignee da task A
  DEVE receber **403** ao: (a) mudar o status de uma task B onde **não** é assignee; (b) editar
  qualquer campo **além de status** na task A (ex.: renomear, mudar due_date, reatribuir o assignee).
  Mutação que ampliasse o carve-out pra qualquer task ou qualquer campo deve quebrar um teste.
- **CA-12** (404 não-vaza na atribuição) — setar assignee numa task de um projeto **private de
  outro** (ou inexistente) DEVE responder **404** (o gate de visibilidade da fase 02 roda primeiro —
  `loadProjectGate`), não 400/403 — não vaza a existência da task.

### Notificação (reusa o canal `notice` per-usuário da fase 16)

- **CA-13** (notifica em set por terceiro) — QUANDO uma operação define `assignee = X` e **X ≠ o
  user da sessão** (set por terceiro) **e** isso é uma **mudança real** (CA-15), o servidor DEVE,
  **pós-commit**, (a) empurrar um `notice` per-usuário pro **X** via
  `hub.BroadcastNotice(X, payload)` com **`{"kind":"task_assigned","task_id":<id>,"by":<sessionUser>}`**
  e (b) enviar um **Web Push** pras subscriptions de **X** via `sender.SendToUser(ctx, X, payload)`
  (best-effort). Reusa **integralmente** o canal da fase 16 — fan-out já é pro userID alvo, **sem**
  broadcast por workspace.
- **CA-14** (auto-set não notifica) — QUANDO o user da sessão atribui a si mesmo (`assignee == user
  da sessão`), **nenhuma** notificação é emitida (sem `notice`, sem push). Você não se avisa.
- **CA-15** (só em mudança real) — um PATCH que **não muda** o `assignee` efetivo (campo ausente, ou
  reenvia o mesmo valor) **NÃO** emite notificação, mesmo que outros campos mudem. Idempotência:
  reaplicar o mesmo PATCH não re-notifica.
- **CA-16** (push best-effort/desligado) — o envio de push segue a degradação da fase 16: sem chaves
  VAPID configuradas → `SendToUser` é **no-op**; uma subscription 404/410 é limpa; falha de uma não
  impede a outra nem o `notice` in-app nem o commit (o commit já aconteceu — notificação é pós-commit).
- **CA-17** (front trata `task_assigned`) — o `noticeStore.pushNotice` (fase 16) DEVE ganhar o ramo
  `kind === 'task_assigned'` (mostra o banner "Você recebeu uma nova task") **sem** alterar o ramo
  `timer_auto_cut` nem o tratamento de `type:"timer"`/anti-stale no `realtime.ts`/`timerStore`. Kind
  desconhecido segue ignorado.

### Visões cross-workspace (rotas POR USUÁRIO)

- **CA-18** ("Minhas tasks") — `GET /api/v1/tasks/assigned` autenticado DEVE devolver **todas** as
  tasks onde `assignee = user da sessão`, **em qualquer workspace que o user participe** e cujo
  projeto seja **visível** pra ele (shared ∥ owner do projeto — mesma regra de visibilidade da fase
  02). Cada item traz o contexto cross-ws: a task + **projeto** (id, name, color) + **workspace** (id,
  name) + **cliente** (name, se houver). Sem sessão → **401** canônico.
- **CA-19** ("Pedidas por mim") — `GET /api/v1/tasks/requested` autenticado DEVE devolver as tasks
  onde `requested_by = user da sessão`, **cross-workspace**, com a mesma visibilidade e o mesmo shape
  enriquecido do CA-18. Sem sessão → **401**.
- **CA-20** (visibilidade nas visões) — uma task atribuída a mim num **projeto private de outro**
  user (eu não sou dono do projeto) **NÃO** aparece em "Minhas tasks" (o join filtra `visibility =
  'shared' OR owner_user_id = $me`). Edge: se a task está num projeto private **meu** (sou owner),
  aparece.
- **CA-21** (isolamento das visões) — `GET /tasks/assigned`/`requested` DEVE filtrar por
  `assignee`/`requested_by` = **o user da sessão** e por membership (a participação no workspace via
  join com `workspace_members`). Uma mutação que removesse o `WHERE assignee = $me` (vazando tasks de
  outros) ou o gate de membership/visibilidade deve quebrar um teste.
- **CA-22** (ordenação determinística) — as visões DEVEM ter ordem estável (ex.: `due_date` NULLS
  LAST, depois `created_at, id`) pra a UI e o teste serem determinísticos. Listas vazias = `[]`
  (nunca `null`), seguindo a convenção das outras listas.

### Não-regressão (fases 02 e 12)

- **CA-23** — o CRUD de task da fase 02 (`tasks_handlers.go` / `tasks/service.go`) e o board da fase
  12 (`GET /workspaces/{id}/projects/{projectId}/board`, `objective_link.go`, recálculo de status)
  seguem **byte-compatíveis**: os campos novos são **aditivos** no `CreateInput`/`PatchInput`; o
  recálculo de objetivo (CA-06 da fase 12, `recalcObjectiveStatus` sob `FOR UPDATE`) **não muda**; as
  suítes `tasks_handlers_test.go`, `tasks_visibility_test.go`, `objective_status_test.go` e o board
  seguem verdes sem editar asserções (salvo extensões explicitamente sancionadas no `plan §desvios`).
- **CA-24** — o gate `RequireWriteRole` da fase 02 (`middleware_role.go`) **não muda de assinatura**;
  o carve-out viewer-assignee mora **no service** (`tasks.Patch`, escopado por-task), não numa
  reescrita do middleware. Os testes do role gate da fase 02 (`middleware_role_test.go`) seguem
  verdes.

### E2E (Playwright)

- **CA-25** — Logado (stub), com um colega **B** seeded no workspace por SQL (padrão
  `seedWorkspaceMember`): o user A (member/owner) abre uma task, **atribui a B**, salva. O fluxo
  prova ponta-a-ponta: (1) a task agora mostra B como responsável; (2) **B** (segundo contexto/
  sessão do stub, se viável; senão verificado por chamada de API com a sessão de B) vê a task em
  **"Minhas tasks"**; (3) **B member** muda o status da sua task (marca feita) e persiste no reload.
  (A notificação push real não é exercitada headless — tem unit no backend; o banner in-app pode ser
  exercitado disparando o `notice` no canal, como no `timer-guard.spec` da 16.)
- **CA-26** (E2E viewer-assignee) — com **B = viewer** e assignee da task X: B consegue mudar o
  status de X (200), e recebe **403** ao tentar editar outro campo de X ou mudar o status de outra
  task Y (onde não é assignee). Pode ser um teste de API dentro do E2E (sessão de B) ou unit no
  backend cobrindo o carve-out (CA-10/11) — a fronteira fica no `plan`/`tasks`.

## 5. Requisitos não-funcionais

- **RNF-01 (multi-tenant)** — TODA query desta fase filtra por `workspace_id` (na escrita de task,
  via `loadProjectGate` + workspace_id da rota) e por participação/visibilidade (nas visões
  cross-ws). Vazamento entre workspaces = bug crítico (Constituição §8.4 — mutação removendo o filtro
  mata teste — CA-21).
- **RNF-02 (notificação pós-commit, não-bloqueante)** — `notice`/push só disparam **depois** do
  commit do PATCH/POST (CA-13); o push roda em goroutine própria (best-effort, fase 16) e **nunca**
  segura a resposta HTTP nem o handler.
- **RNF-03 (validação antes do SQL)** — a checagem de membership do assignee/requested_by roda
  **antes** do INSERT/UPDATE (CA-04), em 1 query indexada (`workspace_members` por
  `(workspace_id, user_id)` — PK), sem custo relevante.
- **RNF-04 (visões cross-ws indexadas)** — `GET /tasks/assigned`/`requested` apoiam-se nos índices da
  migração 0018 (`idx_tasks_assignee`/`idx_tasks_requested_by`); a query junta `tasks` →
  `projects` → `workspaces` → `clients` (left) → `workspace_members` (do user). Sem N+1 (1 query).
- **RNF-05 (não-regressão fase 02/12/16)** — `tasks_handlers.go`/`tasks/service.go` ganham campos
  aditivos; `hub.go`/`push` (fase 16) **intocados** (só **uso** de `BroadcastNotice`/`SendToUser`,
  contratos já existentes); `realtime.ts`/`noticeStore.ts` ganham um ramo de kind, sem mexer no
  `timer_auto_cut`.
- **RNF-06 (deps)** — **zero** dependência nova (Go ou JS). Tudo reusa pacotes existentes.

## 6. Pré-condições externas

Nenhuma nova. As chaves VAPID (push) são as mesmas da fase 16 (config de deploy, não dependência de
terceiro). Sem VAPID → push desligado, `notice` in-app segue (CA-16).

## 7. Fora de escopo (fronteiras explícitas)

- **Propagação ao vivo da atribuição** — a task atribuída **não** aparece na hora no board/visão do
  colega via WebSocket; converge **no reload** (decisão HITL `MILESTONE-2.md §3`). Só a
  **notificação** (banner/push) chega ao vivo. Broadcast de mutação de domínio por workspace = future.
- **`created_by` mutável** — `created_by` continua imutável (= quem criou); só `assignee`/
  `requested_by` são editáveis.
- **Reatribuição em massa / múltiplos assignees** — 1 assignee por task (a coluna é singular);
  multi-assignee é fora.
- **Notificação de `requested_by`** — só o `assignee` é notificado (é quem precisa agir); marcar
  `requested_by` não avisa ninguém.
- **Filtros/ordenação ricos nas visões** (por projeto/status/data na UI) — as visões entregam a
  lista completa cross-ws com ordem estável; filtragem fina é evolução (a UI pode filtrar no cliente).
- **Comentar como viewer-assignee** — é da **fase 19** (thread); aqui o carve-out cobre só **mudar
  status**. A matriz `§8` lista os dois; a fase 18 entrega a metade "status".
- **Lembretes por atribuição** — fase 20.
