# tasks.md — Fase 18 · Cross-assign & visões

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-13).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → campos no service → validação →
> carve-out → visões → wiring), depois o frontend (api/notice/modal/telas/nav), E2E, verificação
> adversarial e gate.
> **Paulada única** (regra de blocos APOSENTADA — `CLAUDE.md`): T-01→T-13 num fluxo só; o commit
> por tarefa garante os checkpoints. Backend (T-01→T-07) antes do front (T-08→T-13).

---

## Backend (campos + validação + carve-out + visões)

- **T-01 — Migração `0018_task_assignment_idx`** (CA-01)
  - Fixar o número lendo `backend/internal/db/migrations/` (próximo livre após `0017`; nome lógico
    `task_assignment_idx`). **SÓ índices**: `idx_tasks_assignee` em `tasks(assignee)` parcial
    (`WHERE assignee IS NOT NULL`) e `idx_tasks_requested_by` em `tasks(requested_by)` parcial.
    **NENHUMA coluna nova** (já existem em `0008`).
  - Teste (estende `migrations_*_test.go` ou novo): os 2 índices existem; as colunas `assignee`/
    `requested_by` seguem uuid-nullable FK a `users` (não regrediram).
  - INDEX.md: registrar 0018 (número final) dona da fase 18.
  - **Acceptance:** `make test-back` do pacote de migrações verde; `pg_indexes` mostra os 2 índices.

- **T-02 — `CreateInput`/`PatchInput` aceitam assignee/requested_by** (CA-02/03/06/07)
  - `tasks/service.go`: `CreateInput` += `Assignee *string`, `RequestedBy *string`; `PatchInput` +=
    `AssigneeSet bool`/`Assignee *string` + `RequestedBySet bool`/`RequestedBy *string` (tri-state).
    INSERT da `Create` grava os dois; UPDATE da `Patch` aplica tri-state. `tasks_handlers.go`:
    `handleCreateTask` decodifica os campos; `patchTaskBody` ganha `Assignee`/`RequestedBy
    json.RawMessage (mesma mecânica do due_date/objective_id).
  - Teste (`tasks_handlers_test.go` estendido, RED primeiro): POST com assignee/requested_by de
    membro grava e retorna; PATCH define; PATCH `null` limpa (CA-06); PATCH ausente mantém; shape da
    `Task` não muda (CA-07).
  - **Acceptance:** novos casos verdes; suíte existente de tasks intacta.

- **T-03 — Validação de membership do assignee/requested_by** (CA-04/05/12)
  - `tasks/service.go`: `validateMember(ctx, q, workspaceID, *string) error` (nil→ok; malformado→400;
    `SELECT 1 FROM workspace_members WHERE workspace_id AND user_id` sem linha → 400 genérico). Chamar
    em `Create`/`Patch` **na tx, antes** do INSERT/UPDATE, **depois** do `loadProjectGate`.
  - Teste (RED): atribuir a uuid de outro ws → 400 `validation_error`; uuid inexistente → 400; uuid
    malformado → 400 (não 500 — CA-05); projeto private de outro → **404** (gate de projeto ganha do
    400 — CA-12); membro válido → 200. Mensagem não vaza (genérica).
  - **Acceptance:** mutação removendo a checagem (aceitar não-membro) mata teste.

- **T-04 — Carve-out viewer-assignee (escopado por-task, só status)** (CA-08/09/10/10.1/11/24)
  - `middleware_role.go`: `RequireWriteRole` ganha bypass condicional — viewer passa **só** em
    `PATCH /tasks/{taskId}` (casar método + route pattern do chi); resto segue 403. Assinatura
    intocada (CA-24). `tasks/service.go::Patch`: no topo, se `role=="viewer"` → carrega a task
    (visibilidade → 404), exige `task.Assignee != nil && *task.Assignee == userID` (senão 403) e que
    o patch toque **apenas** `Status` (qualquer outro campo set → 403). Antes de gravar, **herda o
    bloqueio de projeto arquivado** (CA-10.1): checagem leve do `archived_at` do projeto da task →
    se arquivado, **400 `invalid_reference`** (mesmo resultado de owner/member pelo
    `loadProjectGate(blockArchived=true)`).
  - Teste (RED): viewer-assignee PATCH só status → 200 e persiste (CA-10); viewer-assignee PATCH só
    status em **projeto arquivado** → **400 `invalid_reference`** (igual member — CA-10.1); viewer-
    assignee PATCH name/due_date/assignee → 403 (CA-11b); viewer não-assignee PATCH status → 403
    (CA-11a); viewer POST task / setar assignee → 403 (CA-09); member/owner seguem 200 (CA-08).
    Mutação ampliando o carve-out (qualquer task OU qualquer campo) mata teste; mutação pulando o
    bloqueio de arquivado mata o teste do CA-10.1.
  - **Acceptance:** `middleware_role_test.go` da fase 02 segue verde; novos casos verdes.

- **T-05 — Notificação em set por terceiro (notice + push), só em mudança real** (CA-13/14/15/16)
  - `tasks/service.go`: `Create`/`Patch` calculam `old`/`new` assignee na tx; retornam `notify
    *AssignNotify{TargetUserID, TaskID}` (segundo valor) **só** quando mudança real **e** `new !=
    nil` **e** `*new != userID`. `Core` (api) ganha `OnAssign func(target, taskID, by string)`; os
    handlers chamam `c.OnAssign` **pós-commit** se `notify != nil`. `cmd/server/main.go` injeta
    `OnAssign = func(...){ hub.BroadcastNotice(target, {kind:"task_assigned",task_id,by}); go
    sender.SendToUser(ctx, target, pushPayload) }` (espelha o `onCut` do guard).
  - Teste (RED): A atribui a B → `OnAssign(B, …)` chamado 1× (spy); A atribui a si → **não** chama
    (CA-14); PATCH reenviando o mesmo assignee → não chama (CA-15); PATCH que muda só notes (assignee
    intocado) → não chama (CA-15); push best-effort/no-op sem VAPID herdado da 16 (CA-16, via
    `SendToUser` já testado na 16 — aqui só prova o disparo).
  - **Acceptance:** spy de `OnAssign` confirma exatamente os casos; o handler não bloqueia no push.

- **T-06 — Visões cross-workspace `Assigned`/`Requested`** (CA-18/19/20/21/22)
  - `tasks/assigned.go` (NOVO): `AssignedTask` (task + project{id,name,color} + workspace{id,name} +
    client *string). `Assigned(ctx,userID)`/`Requested(ctx,userID)`: 1 query com joins
    `tasks→projects→workspaces→workspace_members(user=$me)→clients(left)`, `WHERE assignee=$me` (ou
    `requested_by=$me`) `AND (visibility='shared' OR owner=$me)`, `ORDER BY due_date NULLS LAST,
    created_at, id`. Lista vazia → `[]`.
  - `assigned_test.go` (dbtest, RED): atribuídas a mim em 2 workspaces aparecem nas duas (cross-ws —
    CA-18); private de outro NÃO aparece, private meu aparece (CA-20); de outro user não vaza pra mim
    (CA-21); ordem estável (CA-22); `Requested` filtra por requested_by (CA-19). Mutação removendo
    `WHERE assignee=$me` OU o join de membership OU o filtro de visibilidade mata teste (CA-21).
  - **Acceptance:** isolamento provado por mutação; shape com contexto cross-ws.

- **T-07 — Rotas `GET /api/v1/tasks/assigned` + `/tasks/requested` + wiring** (CA-18/19)
  - `internal/api/assigned_routes.go` (NOVO): handlers finos por-usuário (`reqctx.UserID`), chamam
    `Tasks.Assigned`/`Requested`, respondem `{"tasks":[…]}`. `auth_routes.go`: montar as 2 no
    `/api/v1` por-usuário (junto do `mountPush`/`mountTimer`, antes do subrouter `/workspaces/{id}`).
  - `assigned_routes_test.go` (harness api): 200 + shape com sessão; **401** sem sessão; isolamento
    (user A só vê o dele).
  - **Acceptance:** 401 sem sessão; payload correto; suíte de rotas verde.

## Frontend + E2E + gate

- **T-08 — `api.ts` + `noticeStore` ramo task_assigned** (CA-17, CA-02/03)
  - `lib/api.ts`: `TaskInput` += `assignee?: string|null`, `requested_by?: string|null`; tipo
    `AssignedTask` + `getAssignedTasks()`/`getRequestedTasks()` (GET `/tasks/assigned`/`requested`,
    desembrulha `{tasks}`). `lib/noticeStore.ts`: `pushNotice` ganha o ramo `kind==='task_assigned'`
    (banner "Você recebeu uma nova task") **sem** tocar o ramo `timer_auto_cut`.
  - `noticeStore.spec` (estende): `task_assigned` mostra banner; `timer_auto_cut` segue funcionando;
    kind desconhecido ignorado (regressão fase 16 preservada).
  - **Acceptance:** specs verdes; `realtime.ts` intocado (só o store ganhou ramo).

- **T-09 — Selects de Responsável/Solicitante no `TaskModal`** (CA-02/03 UI)
  - `TaskModal.tsx`: 2 selects (Responsável = assignee, Solicitante = requested_by) populados pelos
    **membros do workspace** (reusa o `members` de `GET /workspaces/{id}` — fase 02; hook
    `useWorkspaceMembers` se já existir, senão criar em `lib/queries`). Opção "—" = null. Submit
    inclui `assignee`/`requested_by` no create/patch (null quando "—").
  - `TaskModal.spec` (estende): selects renderizam os membros; submit envia os campos; "—" → null.
  - **Acceptance:** modal envia os campos certos; sem regressão do modal da fase 12.

- **T-10 — Telas "Minhas tasks" / "Pedidas por mim" + nav** (CA-18/19 UI)
  - `routes/cockpit/MyTasks.tsx` (NOVO): duas abas (assigned/requested) consumindo
    `getAssignedTasks`/`getRequestedTasks`; cada item mostra task + projeto (chip cor) + workspace +
    cliente; estado vazio amigável. `App.tsx`: rota `/minhas-tarefas`. `SideNav.tsx`: item "Minhas
    tasks".
  - `MyTasks.spec` (jsdom): renderiza as tasks da resposta mockada com contexto cross-ws; aba troca a
    query; vazio mostra placeholder.
  - **Acceptance:** specs verdes; nav navega.

- **T-11 — E2E `cross-assign.spec.ts`** (CA-25/26)
  - Logado (stub): seed de um colega **B** no workspace por SQL (`e2e/helpers.ts`, padrão
    `seedWorkspaceMember`). A (owner) abre uma task → atribui a B → salva → a task mostra B. Verificar
    "Minhas tasks de B" e a mudança de status via a sessão de B (segundo contexto/login do stub, se
    viável; senão por chamada de API com a sessão de B dentro do teste). Viewer-assignee: B=viewer
    muda status da dele (200) e leva 403 noutra (pode ser asserção de API). Reusar `loginViaStub`.
  - **Acceptance:** o fluxo A→B→"minhas tasks"→status passa; o carve-out viewer é exercido.

- **T-12 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: isolamento multi-tenant das visões · carve-out viewer-assignee
    (escopo por-task/só-status, sem vazar) · validação de membership (não-vaza, antes do SQL) ·
    notificação só-em-mudança-real/só-por-terceiro/pós-commit · não-regressão fase 02/12/16 (board,
    recálculo de objetivo, notice timer_auto_cut, anti-stale) · tri-state null/ausente) + refutação
    dupla. Corrigir confirmados red→green. Registrar no fim deste arquivo.

- **T-13 — Gate + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Antes do E2E,
    liberar a :8080** de dev server alheio (regra do CLAUDE.md). **Sem deploy isolado** se a fase
    rodar dentro de uma paulada M2 (deploy é único no fim — política do CLAUDE.md); se rodar sozinha,
    seguir a rotina do KICKOFF.
  - Handoff (CLAUDE.md "retomar…"), INDEX (18 construída), commit, briefing pro Diogo.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

**1 furo confirmado 2/2 aplicado** (ancorado no código real da fase 02/12). Refutados/descartados
**não** entram neste registro.

- **[MEDIUM] Carve-out viewer-assignee não tratava projeto ARQUIVADO** — o ramo viewer-assignee
  (status-only PATCH) pula o `loadProjectGate` (D-03), então não herdava o `blockArchived=true` que
  owner/member levam (`tasks.Patch` → `loadProjectGate` → 400 `invalid_reference` num projeto
  arquivado, CA-15.2 da fase 02). Um viewer-assignee mudaria status num projeto arquivado enquanto
  owner/member levam 400 — carve-out **mais permissivo** que escrita normal. **Correção:** novo
  **CA-10.1** (spec) exigindo que o carve-out herde o bloqueio de arquivado (400 `invalid_reference`,
  via checagem leve do `archived_at` do projeto da task, sem chamar o `loadProjectGate`); **D-03**
  (plan) e o desvio vetado atualizados pra aplicar o mesmo bloqueio; **T-04** (tasks) ganhou o caso
  de teste "viewer-assignee + status-only + projeto arquivado → 400 (igual member)" + a nota de
  mutação.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> Suítes finais, findings do `verify-build`, correções aplicadas.
