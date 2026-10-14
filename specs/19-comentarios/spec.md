# spec.md — Fase 19 · Comentários (thread) em tasks

> Fonte de verdade da fase. Resolve a dor **P3** do `MILESTONE-2.md §2/§4` ("Não dá pra conversar
> sobre uma task"): adiciona uma **thread de comentários por task** — uma camada de colaboração
> NOVA, sobre a infra de tasks/visibilidade das fases 02/12.
>
> **Decisões travadas com o Diogo (HITL, `MILESTONE-2.md §7.3` e matriz §8):**
> 1. **Thread = tabela nova `task_comments`** (autor + timestamp + texto). A coluna `tasks.notes`
>    (`NOT NULL DEFAULT ''`, já existe na migração `0008_tasks`) **PERMANECE** como a descrição/nota
>    da task — o thread NÃO substitui `notes`, é uma camada paralela.
> 2. **Permissão (matriz §8):** **member** comenta nas tasks **visíveis** (visibilidade herdada do
>    projeto — `loadProjectGate`/`visibleWhere` da fase 02/12); **viewer** comenta **só na task onde
>    é `assignee`** (carve-out escopado §8); o **autor** apaga o próprio comentário, o **owner** apaga
>    qualquer um. Tudo o mais que um viewer tentar = `403`.
> 3. **Tempo real fora:** comentário novo aparece **no reload** (decisão HITL `§3 Fora da M2` —
>    fan-out de domínio ao vivo é future). Esta fase NÃO toca o Hub/realtime nem notifica.
> 4. **Migração `0019_task_comments`** — dona fixa da fase 19 (`MILESTONE-2.md §4/§5`).

---

## 1. Objetivo

Quando uma task é compartilhada entre membros (fase 17/18), as pessoas precisam **conversar sobre
ela sem sair do contexto**. Hoje a única "nota" é o campo `tasks.notes` — texto único, sobrescrito,
sem autoria nem ordem. Esta fase entrega uma **thread**:

1. **Camada de dados nova** (`task_comments`): cada comentário tem autor, corpo e timestamp; vários
   comentários por task, ordenados por criação.
2. **API REST** aninhada na task (`GET`/`POST`/`DELETE`), atrás da cadeia de
   sessão→membership→visibilidade do projeto que já existe.
3. **Permissão por matriz**: member comenta no que vê; viewer comenta **só na task que lhe foi
   atribuída** (carve-out); apaga quem é autor (ou o owner).
4. **UI**: painel de thread no detalhe da task (board da fase 12) — lista de comentários (autor +
   timestamp + texto) + input de novo comentário.

`tasks.notes`, board, objetivos, timer, relatórios, push/notice **seguem intactos**: esta fase
**adiciona** um pacote/rotas/UI quase só novos (RNF-05).

## 2. Definições precisas

- **Task visível** — a regra ÚNICA da fase 02/12 (`projects/service.go::visibleWhere`,
  `tasks/service.go::loadProjectGate`/`Get`): a task é visível ao user se o projeto dela for
  `visibility = 'shared'` **OU** `owner_user_id = user`. Task em projeto private de outro = invisível
  → `404` (não vaza). Toda query de comentário herda esse JOIN.
- **Assignee da task** — `tasks.assignee` (uuid, FK `users`; coluna já existe em `0008`, escrita pela
  **fase 18**). Quando `assignee = user`, o viewer ganha o carve-out de comentar **só nessa task**.
  Se a fase 18 não tiver rodado, `assignee` é sempre `NULL` → nenhum viewer comenta (degradação
  segura, sem regressão).
- **body** — o texto do comentário. **Válido** = string **não-vazia após `TrimSpace`** e com **≤ 4000
  runes** (`utf8.RuneCountInString`). O corpo é **persistido com `TrimSpace` aplicado** (sem espaço de
  borda). Vazio/só-espaço → `400 validation_error`; acima de 4000 → `400 validation_error`.
- **Autor** — `author_id = user_id` da sessão no POST. Imutável (não há edição de comentário nesta
  fase — ver Fora de escopo).
- **Owner** — membro do workspace com `role = 'owner'` (a role da rota, `reqctx.Membership.Role`).

## 3. User stories

- **US-01** — Como membro de um workspace, quero **comentar numa task que vejo**, pra discutir o
  trabalho sem editar a descrição da task.
- **US-02** — Como qualquer membro, quero **ler a thread inteira em ordem cronológica** (mais antigo
  → mais novo), com **quem** escreveu e **quando**, pra acompanhar a conversa.
- **US-03** — Como autor de um comentário, quero **apagar o meu próprio comentário** (corrigir um
  erro); como **owner**, quero poder **apagar qualquer comentário** (moderação).
- **US-04** — Como **viewer** que recebeu uma task (sou o assignee), quero **comentar nela** pra
  responder a quem me pediu — mesmo sendo viewer (só nessa task).
- **US-05** — Como dono de um projeto **privado**, quero que **ninguém de fora** veja nem comente nas
  minhas tasks (não vaza nem a existência).

## 4. Critérios de aceite (EARS — cada um vira teste)

### Migração & modelo de dados

- **CA-01** (migração) — a fase 19 É dona da migração **`0019_task_comments`** (próximo número livre
  após `0018`*): cria a tabela `task_comments` com colunas `id uuid PK DEFAULT gen_random_uuid()`,
  `task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE`, `workspace_id uuid NOT NULL
  REFERENCES workspaces(id)` (denormalizado p/ filtro tenant, igual `tasks`), `author_id uuid NOT
  NULL REFERENCES users(id)`, `body text NOT NULL`, `created_at timestamptz NOT NULL DEFAULT now()`.
  Cria o índice **`(task_id, created_at)`** (varredura ordenada da thread — RNF-01). O teste de
  migração afirma: tabela e colunas existem com a nullability declarada; FK `task_id` é `ON DELETE
  CASCADE`; índice `(task_id, created_at)` presente.
  *(\*o número exato é fixado na T-01 lendo `backend/internal/db/migrations/`. As fases 17 e 18 são
  donas de `0017`/`0018`; se 18 não tiver rodado na worktree, a 19 ainda fixa `0019` por ownership —
  `MILESTONE-2.md §4`. Ver plan §2 e tasks T-01.)*
- **CA-01.1** (cascade real) — apagar uma task (hard delete da fase 02, `Delete`) DEVE **remover em
  cascata** os comentários dela (a thread não sobrevive à task). Teste: criar task + 2 comentários,
  deletar a task, contar 0 comentários daquela `task_id`.

### Backend — criar comentário (`internal/comments`)

- **CA-02** (criar) — `POST /api/v1/workspaces/{id}/tasks/{taskId}/comments` autenticado, de um
  **member** numa **task visível**, com body `{"body":"texto"}` válido, DEVE **persistir** o
  comentário com `author_id = user da sessão`, `task_id`, `workspace_id` (denormalizado, **resolvido
  a partir da task**, não do payload), `body` com `TrimSpace`, `created_at = now()` e responder
  **`201`** com o objeto criado `{id, task_id, workspace_id, author_id, author_name, body,
  created_at}` (`author_name` = nome do user, resolvido por join com `users`).
- **CA-03** (body vazio) — body **ausente**, **vazio** (`""`) ou **só espaço** (após `TrimSpace`) →
  **`400 validation_error`**; nenhuma linha gravada.
- **CA-04** (limite de tamanho) — body com **> 4000 runes** → **`400 validation_error`**; nenhuma
  linha gravada. Exatamente 4000 runes é **aceito**. (Contagem por rune, `utf8.RuneCountInString`,
  não por byte — emoji conta 1.)
- **CA-05** (trim na persistência) — um body `"  oi  "` é gravado como `"oi"` (TrimSpace aplicado
  ANTES do INSERT) e devolvido assim. (Casa com a validação do CA-03: não dá pra burlar o vazio com
  espaços.)

### Backend — listar thread

- **CA-06** (listar ordenado) — `GET /api/v1/workspaces/{id}/tasks/{taskId}/comments` numa task
  visível DEVE devolver **todos os comentários daquela task**, ordenados por **`created_at` ASC**
  (mais antigo primeiro; desempate por `id` pra ordem determinística), no envelope
  `{"comments":[...]}`. Lista vazia = `{"comments":[]}` (`[]`, não `null`). Cada item traz
  `author_name`.
- **CA-06.1** (só daquela task) — a lista NUNCA inclui comentário de **outra task** (mesmo do mesmo
  workspace/projeto). Teste: 2 tasks com comentários distintos; cada `GET` só vê os seus. Mutação
  removendo o `WHERE task_id` morre.

### Backend — apagar comentário

- **CA-07** (autor apaga o próprio) — `DELETE
  /api/v1/workspaces/{id}/tasks/{taskId}/comments/{commentId}` pelo **autor** do comentário DEVE
  removê-lo e responder **`204`** (idempotência: 2º DELETE do mesmo id → `404`, já não existe).
- **CA-08** (owner apaga qualquer) — o **owner** do workspace DEVE poder apagar **qualquer**
  comentário da thread (mesmo de outro autor) → `204`.
- **CA-09** (member NÃO apaga o de outro) — um **member** (não-owner, não-autor) que tenta apagar o
  comentário **de outra pessoa** → **`403 forbidden`**; o comentário permanece. (Apagar o próprio →
  `204`, CA-07.)
- **CA-10** (comment inexistente / de outra task) — DELETE com `commentId` inexistente, malformado,
  ou que pertence a **outra task** que não a `{taskId}` da URL → **`404`** (não vaza; o id tem que
  casar `task_id` da rota).

### Permissões — visibilidade & carve-out viewer-assignee (matriz `MILESTONE-2.md §8`)

- **CA-11** (task invisível → 404 no POST) — comentar (POST) numa task de **projeto private de outro
  user** → **`404`** (MESMO 404 de task inexistente — não vaza existência). Idem `GET` (CA-12) e
  `DELETE`. A resolução de visibilidade reusa o gate da fase 02/12 (`visibleWhere`), não reinventa.
- **CA-12** (task invisível → 404 no GET) — `GET` da thread de uma task **invisível** ou de
  **workspace alheio** (id na URL de outro ws) → **`404`**; zero vazamento de comentários.
- **CA-13** (viewer 403 em task que não é dele) — um **viewer** que tenta **comentar** (POST) numa
  task visível onde **NÃO é o `assignee`** → **`403 forbidden`** (a regra base "viewer não escreve"
  da fase 02 vale). Nenhuma linha gravada.
- **CA-13.1** (gate null-safe — assignee NULL não vira 500) — um **viewer** que tenta **comentar**
  (POST) numa task visível cujo **`assignee IS NULL`** (o caso NORMAL — a escrita do assignee é da
  fase 18) DEVE receber **`403 forbidden`**, **NUNCA `500`**. A coluna do gate `viewer_can` é
  resolvida com `COALESCE(t.assignee = $userID, false)` (plan §3 D-03): `NULL = $userID` é `NULL` em
  SQL e quebraria o `Scan` para `bool` Go com 500; o `COALESCE` força `false`. Teste: task visível,
  `assignee IS NULL`, viewer POSTa → 403 (e nenhuma linha gravada); mutação trocando o `COALESCE` por
  `t.assignee = $userID` cru reproduz o 500 e morre.
- **CA-14** (viewer OK na task dele — carve-out) — um **viewer** que é o **`assignee`** de uma task
  visível DEVE conseguir **comentar** (POST → `201`) nela. **Só nessa task**: comentar em qualquer
  outra (mesmo visível) onde não é assignee segue `403` (CA-13). Esse é o **único** carve-out de
  escrita do viewer nesta fase.
- **CA-14.1** (carve-out NÃO vaza pra delete de terceiros) — o viewer-assignee pode apagar o
  **próprio** comentário (CA-07 vale por autoria, não por role), mas **NÃO** apaga comentário de
  outro autor na sua task → `403` (ele não é owner; o carve-out é só de **criar/comentar**, não de
  moderar). (Casa com a matriz: "Apagar comentário: viewer = próprio".)
- **CA-15** (member comenta só em task visível) — um member comenta em task `shared` (→ `201`), mas
  em task de projeto **private de outro** → `404` (CA-11). A visibilidade é a fronteira do member,
  não a role.

### Isolamento multi-tenant

- **CA-16** (isolamento por workspace) — toda query de comentário (criar/listar/apagar) filtra por
  **`workspace_id`** (+ visibilidade do projeto). Um comentário criado no workspace **A** NUNCA
  aparece numa requisição do workspace **B** (mesmo que o user seja membro dos dois). Teste: 2
  workspaces, comentário no A; `GET` no B (com o `taskId` do A na URL de B) → `404`, lista do B vazia.
- **CA-16.1** (mutação mata o teste) — remover o predicado **`WHERE workspace_id`** (ou o JOIN de
  visibilidade) de qualquer query do `comments.Service` DEVE **quebrar** um teste de isolamento
  (Constituição §8.4 / `MILESTONE-2.md §3`). O `workspace_id` denormalizado na tabela é o backstop;
  a query usa o `workspace_id` da rota, não confia no payload.

### Autenticação

- **CA-17** (401 sem sessão) — `GET`/`POST`/`DELETE` de comentários **sem sessão** → **`401`**
  canônico `{"error":{"code":"unauthorized",...}}` (a cadeia `RequireAuth` da fase 01 barra antes do
  handler; teste table-driven nas 3 rotas).

### Não-regressão

- **CA-18** (fase 02/12 intactas) — as rotas e o service de **tasks** (`tasks/service.go`,
  `tasks_handlers.go`), o **board** (`objectives/board.go`), o gate de visibilidade
  (`loadProjectGate`/`visibleWhere`) e as migrações `0008`/`0013` seguem **byte-compatíveis**: esta
  fase **adiciona** o pacote `internal/comments`, a migração `0019`, rotas novas sob a task, e um
  painel de UI novo — **sem editar** asserções das suítes das fases 02/12. As suítes Go/Vitest/
  Playwright dessas fases seguem verdes.
- **CA-18.1** (`tasks.notes` preservado) — o campo `tasks.notes` segue como a descrição da task (lido
  e escrito por `Create`/`Patch` da fase 02/12, exibido no `TaskModal`); o thread é **separado**. Um
  teste afirma que criar/editar a `notes` de uma task **não** cria comentário, e comentar **não**
  altera `notes`.
- **CA-18.2** (carve-out não regride o gate global) — a exceção viewer-assignee é **escopada por
  task no service de comments** (não no `RequireWriteRole` global — ver plan §3): um viewer segue
  recebendo `403` em escrita de domínio (task/objetivo/projeto/cliente da fase 02) **que NÃO seja o
  carve-out de status-only que a fase 18 abre** (as suítes de `viewer-gating`/`middleware_role`
  seguem verdes). Teste **robusto a pós-merge com a fase 18**: viewer-assignee comenta (`201`) mas
  ainda recebe `403` ao tentar um **PATCH de domínio que continua barrado independentemente da fase
  18** — concretamente: (a) PATCH de um campo **FORA** do carve-out status-only da 18 (`name`/`notes`/
  `due_date`/`objective_id`) na **própria** task → `403`; **OU** (b) PATCH/POST numa task onde **NÃO
  é assignee** → `403`. **Não usar** o caso status-only na própria task (que a fase 18 CA-10 passa a
  retornar `200`) — esse colidiria pós-merge. O ponto provado é que o `RequireWriteRole` global
  **não foi afrouxado** pela fase 19.

### E2E (Playwright)

- **CA-19** — Logado (stub) como **owner/member** num projeto shared com uma task: abrir o detalhe da
  task (board da fase 12), escrever um comentário no input, enviar → o comentário **aparece na thread**
  com o nome do autor e o texto; um **segundo** comentário aparece **abaixo** do primeiro (ordem
  ASC); apagar o **próprio** comentário → some da lista; reload mostra o estado persistido. (O E2E
  cobre SÓ este fluxo do owner/member — o gating viewer-assignee é provado fora do E2E, CA-19.1.)
- **CA-19.1** (viewer-assignee — provado SEM E2E) — o E2E **não consegue** logar como o viewer: o
  `googlestub` autentica **um único usuário** (o owner/dono da sessão), não há login multi-usuário
  (limitação já registrada em `tarefas.spec.ts:8-9`). Logo o gating viewer-assignee é provado em
  **dois níveis fora do Playwright**:
  1. **Teste de componente (jsdom — T-08):** monta `<TaskThread/>` com `me.user.id` mockado: quando
     `currentUserId === task.assignee` (viewer-assignee), o input/botão "Comentar" **está
     disponível** (`can_comment = true`); quando `currentUserId !== task.assignee` numa task onde
     `can_write = false`, o input **não permite** comentar (desabilitado/ausente, `can_comment =
     false`). Cobre o gating de UI por `can_comment` sem depender de login real.
  2. **Teste de API Go (T-06, `comments_handlers_test.go`):** via o subrouter irmão, um POST de
     **viewer-assignee** → `201` (carve-out passa o write-role global, prova D-02); de **viewer
     não-assignee** → `403`. Cobre o gate de backend do carve-out sem depender do E2E.
  Espelha a decisão registrada no `tarefas.spec.ts` (gating de role provado por componente+API, não
  por E2E multi-usuário).

## 5. Requisitos não-funcionais

- **RNF-01 (listagem barata)** — a thread lê por `task_id` sobre o índice **`(task_id, created_at)`**
  (uma varredura ordenada, sem `ORDER BY` em memória). O join com `users` (nome do autor) e o join de
  visibilidade com `projects` são por PK/índice. Sem N+1 (uma query lista a thread inteira com
  `author_name`).
- **RNF-02 (clock/tempo)** — `created_at = now()` no banco; a API devolve `timestamptz` em ISO 8601
  (RFC3339), o front formata por fuso do navegador (padrão das outras listagens). Sem aritmética de
  data no service.
- **RNF-03 (sem dependência nova)** — **zero** dependência Go ou JS nova (reusa `pgx`, `chi`,
  `google/uuid`, `domain`, TanStack Query, `lib/api.ts`). O pacote `internal/comments` espelha o
  shape do `tasks`/`clients` existentes.
- **RNF-04 (não-regressão)** — fases 02/12 (tasks/board/visibilidade) e a cadeia de middleware
  (`RequireAuth`→`RequireMember`→`RequireWriteRole`) **intocadas no contrato** (CA-18/18.2). As rotas
  de comentário entram num ponto que **não** sujeita o POST/DELETE de viewer-assignee ao gate global
  (plan §3) — a exceção é avaliada no service.
- **RNF-05 (gate de fase)** — `make check-full` (Go test + Vitest + Playwright + builds + smoke do
  compose) **verde**.

## 6. Pré-condições externas

- **Nenhuma.** A fase é autônoma: não depende de credencial, provider externo, nem de tarefa do
  Diogo. A migração roda no boot; testes usam o `dbtest` (fase 01) e o `googlestub` (E2E). A coluna
  `tasks.assignee` (usada pelo carve-out) **já existe** desde `0008`; a **escrita** dela é da fase 18,
  mas o E2E/teste do carve-out **semeia o assignee por SQL** (não exige a API da 18) — a fase 19 não
  tem dependência dura de build com a 18 (ver plan §7).

## 7. Fora de escopo (fronteiras explícitas)

- **Edição de comentário** — comentário é imutável (criar/ler/apagar). Editar é evolução futura.
- **Tempo real** — comentário novo NÃO aparece ao vivo pros outros (decisão HITL `§3 Fora da M2`);
  aparece **no reload**. Esta fase NÃO toca o Hub/`realtime.ts`/`BroadcastNotice`.
- **Notificação de comentário novo** (push/notice ao assignee/autores da thread) — fora; o canal
  `notice`/push existe (fase 16) mas só lembretes/cross-assign o usam (fases 18/20). Comentário não
  notifica nesta fase.
- **Menções (`@fulano`), anexos, reações, markdown/rich text** — fora; `body` é texto plano.
- **Threading aninhado (responder a um comentário)** — fora; a thread é **linear** (lista plana por
  task).
- **Escrita de `assignee`/`requested_by`** (cross-assign) — é da **fase 18**; a 19 só **lê**
  `assignee` pro carve-out.
- **Paginação da thread** — fora; lista tudo (threads de task são curtas; paginar é futuro se doer).
- **Contagem/badge de comentários no card do board** — fora desta spec (candidato a polish; o board
  da fase 12 não muxa de contrato — CA-18).
