# tasks.md — Fase 19 · Comentários (thread) em tasks

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-11).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → service → handlers → wiring),
> depois o frontend (api/queries/thread/modal), E2E, verificação adversarial e gate.
> Reusa: `dbtest` (fase 01), `coretest_test.go` (`newCoreRouter`/`loginVia`/`addMember`/`doJSON`/
> `errCode`), `domain.Error`, `reqctx`, gate de visibilidade `visibleWhere`/`loadProjectGate`
> (fase 02/12), board/TaskModal (fase 12), TanStack Query (`lib/queries/*`).

---

## Bloco backend (thread + permissões)

- **T-01 — Migração `0019_task_comments`** (CA-01/01.1, RNF-01)
  - Fixar o número lendo `backend/internal/db/migrations/` (próximo livre; ownership 0019 fixo —
    `MILESTONE-2.md §4`). Tabela `task_comments` (`id` PK, `task_id` FK `tasks` `ON DELETE CASCADE`,
    `workspace_id` FK `workspaces`, `author_id` FK `users`, `body text NOT NULL`, `created_at`
    timestamptz default now) + índice `(task_id, created_at)` — plan §2.
  - **Entregável:** o `.sql` + teste de migração (novo `migrations_comments_test.go` no padrão de
    `migrations_core_test.go`).
  - **Acceptance:** teste verde — tabela/colunas existem com a nullability declarada; FK `task_id` é
    `ON DELETE CASCADE` (inserir task+comentário, deletar task, contar 0 comentários — CA-01.1);
    índice `(task_id, created_at)` presente.
  - INDEX.md: registrar 0019 (número final) dona da fase 19.

- **T-02 — `comments.Service.Create` — criar + body + visibilidade + carve-out** (CA-02/03/04/05/11/13/13.1/14/15)
  - Pacote novo `internal/comments`; `Service{db}`; `validBody` (TrimSpace, não-vazio, ≤ 4000 runes
    `utf8.RuneCountInString`); `Create(ctx, wsID, userID, role, taskID, body)`: query de gate
    (visibilidade + member_can/viewer_can por `assignee` — plan §3 D-03; `viewer_can` =
    **`COALESCE(t.assignee = $userID, false)`**, null-safe), depois INSERT com
    `workspace_id` da ROTA e `body` trimado, RETURNING + join `users.name`.
  - **Entregável:** `service.go` + `service_test.go` (dbtest, RED primeiro).
  - **Acceptance:** testes verdes — member em task shared cria (201-equiv, objeto com `author_name`/
    `body` trimado — CA-02/05); body vazio/só-espaço → `Validation` (CA-03); 4001 runes → `Validation`,
    4000 aceito (CA-04); member em task private de outro → `NotFound` (CA-11/15); viewer não-assignee
    → `Forbidden` (CA-13); **viewer não-assignee numa task visível com `assignee IS NULL` → `Forbidden`,
    NÃO 500 (CA-13.1)** — mutação trocando o `COALESCE` por `t.assignee = $userID` cru reproduz o 500 e
    morre; viewer-assignee → cria (CA-14); viewer-assignee em OUTRA task visível
    (não-assignee) → `Forbidden` (CA-14, "só nessa task").

- **T-03 — `comments.Service.List` — thread ordenada + visibilidade** (CA-06/06.1/12)
  - `List(ctx, wsID, userID, taskID)`: checa visibilidade da task (sub-query, invisível → `NotFound`),
    depois `SELECT … JOIN users … WHERE workspace_id AND task_id ORDER BY created_at, id`.
  - **Acceptance:** testes verdes — N comentários voltam ASC por `created_at` (desempate `id` —
    CA-06); vazio → `[]` não-nil; comentários de OUTRA task não aparecem (CA-06.1, mutação removendo
    `task_id` morre); `List` de task private de outro → `NotFound` (CA-12).

- **T-04 — `comments.Service.Delete` — autor/owner + 404 não-vaza** (CA-07/08/09/10/14.1)
  - `Delete(ctx, wsID, userID, role, taskID, commentID)`: `uuid.Validate` (→ NotFound); query casando
    `c.id`+`c.task_id`+`workspace_id`+visibilidade → `author_id`; `canDelete = autor || owner`; senão
    `Forbidden`; DELETE.
  - **Acceptance:** testes verdes — autor apaga o próprio (CA-07) e 2º DELETE → NotFound (idempotência);
    owner apaga o de outro (CA-08); member não-autor não apaga o de outro → `Forbidden` (CA-09);
    viewer-assignee não apaga o de outro autor na task dele → `Forbidden` (CA-14.1); commentId de
    outra task / inexistente / malformado → `NotFound` (CA-10).

- **T-05 — Isolamento multi-tenant** (CA-16/16.1)
  - Teste dedicado no `service_test.go`: 2 workspaces (mesmo user membro dos dois), comentário no A;
    `List`/`Delete`/`Create` com o `taskId` do A sob `workspace_id` do B → `NotFound`/lista vazia.
  - **Acceptance:** mutação removendo `WHERE workspace_id` (ou o JOIN de visibilidade) de Create/List/
    Delete **mata** um teste (CA-16.1). Espelha `TestIsolamentoEntreWorkspaces` da fase 02.

- **T-06 — Handlers HTTP + wiring do subrouter irmão** (CA-02/06/07/13.1/17/19.1, CA-18.2)
  - `internal/api/comments_handlers.go` (`Comments{Svc}`, `WithComments` Option, handlers
    `handleCreate`/`handleList`/`handleDelete` — tradução fina, `reqctx`+`chi.URLParam`,
    `writeDomainError`, `decodeJSON`). Wiring em `auth_routes.go`: subrouter **IRMÃO**
    `/api/v1/workspaces/{workspaceId}/tasks/{taskId}/comments` com **só** `ws.RequireMember` (NÃO o
    `RequireWriteRole`) — plan §3 D-02. `routerCfg` ganha `comments`.
  - **Entregável:** handlers + `comments_handlers_test.go` (harness `newCoreRouter` estendido p/
    montar comments, ou novo harness espelho).
  - **Acceptance:** testes verdes — POST válido 201 + persiste; body inválido 400; viewer não-assignee
    POST 403 (e numa task `assignee IS NULL` → **403, NÃO 500** — CA-13.1); viewer-assignee POST 201
    (carve-out passa o subrouter irmão — prova D-02 e o nível 2 do CA-19.1); GET lista
    ASC 200; DELETE autor 204 / outro 403/404; **401 table-driven** nas 3 rotas sem sessão (CA-17);
    **viewer-assignee segue 403 num PATCH de domínio que NÃO é o carve-out status-only da fase 18**
    — PATCH de campo fora do status-only (`name`/`notes`/`due_date`/`objective_id`) na própria task,
    OU PATCH/POST numa task onde NÃO é assignee (rota da fase 02, prova CA-18.2 robusta a pós-merge
    com a 18 — o write-role global não foi afrouxado; **não** usar o status-only na própria task, que
    a 18 passa a aceitar 200).

## Bloco frontend + E2E + gate

- **T-07 — `lib/api.ts` + `lib/queries/comments.ts`** (CA-06)
  - Tipo `Comment`; `listComments`/`createComment`/`deleteComment` (desembrulham `{comments}`);
    `commentsKey`/`useComments`/`useCreateComment`/`useDeleteComment` (TanStack, invalidam a key no
    onSuccess) — espelho de `lib/queries/tasks.ts`, **sem** tocar os hooks de task (RNF-04).
  - **Acceptance:** spec (vitest) do client/hook — `useComments` busca e mapeia; `useCreateComment`
    POSTa e invalida; tipos compilam.

- **T-08 — `TaskThread.tsx` + `can_comment` + `assignee` no board** (CA-19.1 nível 1, CA-08/09 gating UI)
  - `components/board/TaskThread.tsx` (lista autor+timestamp+body, input + "Comentar", ✕ no próprio/
    quando owner; `useComments`/mutations; ordem ASC; `role="log"`; sem polling). Verificar o shape
    da `BoardTask`: se `assignee` não estiver exposto no front, adicionar (aditivo no TS — plan §6
    D-05). `can_comment = can_write || (task.assignee === currentUserId)` (currentUserId do `/me`/
    `useAuthStore`).
  - **Acceptance:** specs (jsdom) — thread renderiza comentários ASC; enviar chama createComment;
    body vazio desabilita o botão; ✕ aparece só no próprio (autor) e em todos pro owner; comentário
    desconhecido/erro não quebra. **Gating viewer-assignee (CA-19.1 nível 1, sem E2E):** com
    `me.user.id === task.assignee` (viewer-assignee), o input/botão "Comentar" **está disponível**
    (`can_comment=true`); com `me.user.id !== task.assignee` e `can_write=false`, o input **não
    permite** comentar (`can_comment=false`, desabilitado/ausente).

- **T-09 — Montar a thread no `TaskModal` + passar `can_comment` do `ProjectBoard`** (CA-18/18.1)
  - `TaskModal` no modo **Editar** (task com id) renderiza `<TaskThread/>` abaixo do form; modo criar
    NÃO mostra. `ProjectBoard` calcula `canComment` (do `can_write` + assignee×currentUser) e passa
    ao modal. `tasks.notes` (campo Notas do modal) **intocado** (CA-18.1).
  - **Acceptance:** specs do board/modal seguem verdes (CA-18); novo spec: modo Editar mostra a
    thread, modo criar não; editar Notas não cria comentário (CA-18.1).

- **T-10 — E2E `comentarios.spec.ts`** (CA-19 — SÓ o fluxo owner/member)
  - Logado (stub) como owner/member num projeto shared com task: abrir detalhe (board fase 12) →
    escrever comentário → aparece com nome+texto; 2º comentário aparece **abaixo** (ASC); apagar o
    próprio → some; reload persiste.
  - **NÃO** há ramo viewer no E2E: o `googlestub` autentica **um único usuário** (não há login
    multi-usuário — `tarefas.spec.ts:8-9`), então "logado como o viewer" é **inexecutável** em
    Playwright. O gating viewer-assignee é coberto por T-08 (componente jsdom, CA-19.1 nível 1) +
    T-06 (API Go, CA-19.1 nível 2). Registrar essa decisão no cabeçalho do spec, espelhando o
    `tarefas.spec.ts`.
  - Reusar `loginViaStub`; liberar a :8080 antes do E2E (server alheio quebra `reuseExistingServer`).
  - **Acceptance:** `comentarios.spec.ts` verde; suítes E2E das fases 02/12 (`core-projetos.spec`,
    `tarefas.spec`) seguem verdes (CA-18).

- **T-11 — Verificação adversarial do build + gate + handoff**
  - Workflow ultracode (dimensões: visibilidade/404-não-vaza · carve-out viewer-assignee escopado por
    task · autor/owner no delete · isolamento multi-tenant (mutação WHERE workspace_id/task_id) ·
    body trim/limite-runes · não-regressão fase 02/12 + write-role global) + refutação dupla.
    Corrigir confirmados red→green. Registrar no fim deste arquivo.
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke do compose). INDEX (19
    construída), handoff (CLAUDE.md "retomar…"), commit por tarefa.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

**3 furos confirmados 2/2 aplicados** (ancorados no código real). Os refutados/descartados da
verificação NÃO entram aqui.

- **[HIGH] Gate de escrita null-unsafe (assignee NULL → 500 em vez de 403).** A query de gate do
  `Create` usava `t.assignee = $userID` cru; com `assignee IS NULL` (o caso NORMAL — a escrita do
  assignee é da fase 18) o predicado vira SQL `NULL` (não `false`) e quebra o `Scan` para `bool` Go →
  **500** onde o esperado é **403** (viewer não-assignee). **Aplicado:** plan §3 D-03 troca por
  `COALESCE(t.assignee = $userID, false)` (espelha o padrão `IS NOT NULL` do `loadProjectGate` da
  fase 02) + nota; **CA-13.1 novo** na spec (viewer numa task visível com `assignee IS NULL` → 403,
  nunca 500; mutação pro `=` cru reproduz o 500 e morre); T-02/T-06 cobrem o caso.
- **[HIGH] E2E "logado como o viewer" é inexecutável.** O `googlestub` autentica **um único usuário**
  (o owner), não há login multi-usuário (`tarefas.spec.ts:8-9`); o ramo viewer do E2E do CA-19.1 não
  rodava. **Aplicado:** CA-19.1 reescrito — o gating viewer-assignee é provado por **teste de
  componente jsdom** (T-08: `me.user.id === assignee` libera o input; `≠` com `can_write=false`
  bloqueia) + **teste de API Go** (T-06: viewer-assignee POST 201, viewer não-assignee 403); o E2E
  (T-10) cobre **só** o fluxo owner/member. CA-19 reescrito p/ owner/member; T-08/T-10 alinhados.
- **[MEDIUM] CA-18.2 colidia com a fase 18.** O caso "viewer-assignee comenta (201) mas 403 ao PATCH
  status-only na própria task" quebraria pós-merge: a fase 18 (CA-10) faz o PATCH status-only do
  viewer-assignee na própria task retornar **200**. **Aplicado:** CA-18.2 reescrito p/ um PATCH que
  permanece 403 independentemente da 18 — PATCH de campo FORA do carve-out status-only
  (`name`/`notes`/`due_date`/`objective_id`) na própria task, OU PATCH/POST numa task onde NÃO é
  assignee — com nota explícita de **não** usar o status-only (que a 18 abre). T-06 alinhado.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> PENDENTE — suítes finais, findings do `verify-build`, correções aplicadas. Preenchido no BUILD da fase.
