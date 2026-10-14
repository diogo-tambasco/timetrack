# tasks.md — Fase 17 · Membros & convites (workspace multiplayer)

> TDD: red → green → refactor. **Commit por tarefa.** `make check-full` é o gate final (T-13).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → service → gate → rotas → hook de
> login → wiring), depois o frontend (api/hooks → página), E2E, verificação adversarial e gate.
> **Paralelizável (worktree):** sim — `0017` é número fixo, pacote/rotas/UI quase só novos
> (plan §7). A regra de blocos está APOSENTADA (CLAUDE.md): a fase roda numa paulada, parando só
> no fim ou num bloqueador.

---

## Backend — convites + gestão de membros

- **T-01 — Migração `0017_invitations`** (CA-01/02/03, RNF-04 backstop)
  - Fixar o número lendo `backend/internal/db/migrations/` (próximo livre = `0017`; última presente
    = `0016_timer_guard`). Criar `invitations` (plan §2): colunas + CHECK de status + FK cascade no
    workspace + FK em `invited_by` + `token UNIQUE`; reusar o enum `workspace_role` (0003). Índice
    **único parcial** `ux_invitations_pending (workspace_id, lower(email)) WHERE status='pending'` e
    índice `idx_invitations_email_pending (lower(email)) WHERE status='pending'`.
  - **Teste** (estende `migrations_*_test.go` ou novo): tabela/colunas existem; `status` default
    `pending`; CHECK barra status inválido; FK cascade no workspace; o índice parcial barra 2
    pending pro mesmo `(workspace, lower(email))` e PERMITE um 2º pending depois de revogar o 1º.
  - INDEX.md: registrar 0017 dona da fase 17.
  - **Acceptance:** `go test ./internal/db/...` (teste da migração) verde; índice parcial provado.

- **T-02 — `invitations.Service` — criar/listar/revogar convite** (CA-03/04/05/06/07/08/09/13/14/15/22)
  - Pacote novo `internal/invitations`; `Service{db, now, inviteTTL}` (`now` injetável `WithClock`;
    `INVITE_TTL` default 14d — D-03). `Create` (normaliza email, valida role member|viewer + formato,
    checa já-membro → `already_member`, INSERT com token `crypto/rand` + `expires_at=now+TTL`,
    `ON CONFLICT ux_invitations_pending` → `invite_pending`), `List` (só pending, por `workspace_id`,
    sem token), `Revoke` (`id AND workspace_id`; pending→revoked; accepted→`invite_not_pending`;
    outro ws → `not_found`).
  - **`service_test.go`** (dbtest, RED primeiro): email normalizado lower/trim (CA-05); role owner no
    payload → 400 (CA-03); já-membro → 409 `already_member` (CA-07); 2º pending mesmo email → 409
    `invite_pending`, 1 linha só (CA-08); novo pending após revogar → criado (CA-09); `List` só vê os
    do workspace, sem token (CA-13/RNF-04); revoke de outro ws → 404 (CA-15); revoke de accepted →
    409 (CA-14); **isolamento:** mutação removendo `WHERE workspace_id` no List/Revoke morre (CA-22).
  - **Acceptance:** `go test ./internal/invitations/...` verde; mutação de isolamento mata teste.

- **T-03 — `invitations.Service` — add existente / remover / mudar role + invariante last_owner**
  (CA-10/11/12/16/17/18/19/20/22)
  - `AddExistingMember` (resolve `user_id` por `lower(email)` em `users`; inexistente →
    `user_not_found` 404; já-membro → `already_member` 409; INSERT membership role member|viewer →
    201). `RemoveMember`/`ChangeRole` numa transação sob `pg_advisory_xact_lock(hashtext(workspaceID))`
    com re-check `count(*) WHERE role='owner'` → operação que zeraria owners → `last_owner` 409;
    membro inexistente → `not_found` 404; role fora do enum → 400.
  - **Teste** (dbtest, RED): add existente cria membership (CA-10); add inexistente → 404 (CA-11);
    add já-membro → 409 (CA-12); remover membro comum OK (CA-16); remover único owner → 409
    `last_owner` (CA-18); rebaixar único owner → 409 (CA-19); promover 2º owner depois rebaixar o 1º
    → OK (CA-19); PATCH/DELETE de userId não-membro → 404 (CA-20); **concorrência:** 2 `RemoveMember`
    do owner sob lock → no máximo 1 sucesso, nunca 0 owners (CA-18); isolamento `WHERE workspace_id`
    (CA-22).
  - **Acceptance:** `go test ./internal/invitations/...` verde; teste de concorrência determinístico
    (martelado N×) nunca deixa o workspace sem owner.

- **T-04 — `RequireOwner` (gate aditivo)** (CA-21/21.1)
  - `api/middleware_role.go`: **só ADICIONAR** `RequireOwner` (403 se `Role != "owner"`).
    `RequireWriteRole` **intocado**.
  - **Teste** (`middleware_role_test.go` estendido): owner passa; member → 403; viewer → 403;
    asserção de que o envelope é `forbidden`. (A ordem 404-antes-de-403 é provada no T-05 com a
    cadeia real.)
  - **Acceptance:** `go test ./internal/api/...` do middleware verde.

- **T-05 — Rotas `mountInvitations` (gestão Owner-only)** (CA-04/06/07/10/13/14/16/17/20/21/21.1/22/23)
  - `internal/api/invitations_routes.go`: `Invitations` agrega `*invitations.Service`;
    `WithInvitations(svc) Option`; `mountInvitations(wr, svc)` cria um `wr.Group` com `o.Use(
    RequireOwner)` e pendura `POST/GET /invitations`, `DELETE /invitations/{inviteId}`,
    `POST /members`, `DELETE /members/{userId}`, `PATCH /members/{userId}` — handlers finos
    (decode → service → `writeDomainError`/`respond.JSON`), espelhando `push_routes.go`/
    `core_routes.go`. Montar em `auth_routes.go` dentro do subrouter do workspace, **depois** do
    `mountCore`.
  - **`invitations_routes_test.go`** (harness `coretest_test.go`/`newCoreRouter`): tabela de
    permissões — owner 2xx, member 403, viewer 403 em TODAS as rotas de gestão (CA-21); não-membro →
    404 (CA-21.1); sem sessão → 401 (CA-23); payloads inválidos → 400 (CA-06); 409s certos
    (already_member/invite_pending/last_owner/invite_not_pending); isolamento ids cruzados de 2
    workspaces → 404 (CA-22). Espelha o estilo de `isolation_core_test.go`.
  - **Acceptance:** `go test ./internal/api/...` verde; a tabela owner/member/viewer cobre as 6 rotas.

- **T-06 — Hook de resolução no login PELO callback de auth (`ResolveForEmail` + `auth.Config.ResolveInvites`)** (CA-24/25/26/27/28, RNF-02/05)
  - **⚠️ Furo HIGH (verificação da spec):** o login real é `auth.Service.HandleCallback` →
    `s.store.UpsertUserFromGoogle`, e `s.store` é construído DENTRO de `auth.NewService`
    (`auth/oauth.go:50`: `workspace.NewStore(cfg.DB)`) — instância diferente da do routing. Ligar a
    resolução no `NewStore` do `api.WithAuth` deixaria o login real no-op. **Decisão (opção (b)):**
    ligar no callback, **NÃO** no store; `workspace.NewStore` fica **byte-idêntico** (sem tocar os 13
    call sites, incl. `auth/oauth.go:50` e `calendar/service.go:51`).
  - `invitations.Service.ResolveForEmail(ctx, userID, email)`: transação **própria pós-commit** sobre
    o pool, sob `pg_advisory_xact_lock(hashtext(googleID))` (ou por user_id); erro logado e engolido.
    Query: materializa `workspace_members` dos convites válidos (`pending` + não-expirado) do
    `lower(email)`, `ON CONFLICT (workspace_id,user_id) DO NOTHING`, e marca esses convites
    `accepted` (plan §3.5 SQL). Assinatura casa o tipo do campo `auth.Config.ResolveInvites`.
  - `internal/auth/oauth.go`: `auth.Config` ganha o campo
    `ResolveInvites func(ctx context.Context, userID, email string) error` (default no-op se nil) —
    ajuste **sancionado**. `auth.NewService` **NÃO muda** o `workspace.NewStore(cfg.DB)` interno
    (linha 50). `internal/auth/handlers.go`: `HandleCallback` chama `cfg.ResolveInvites(ctx, userID,
    ui.Email)` **logo após `UpsertUserFromGoogle` retornar** (~linha 78; já tem `ui.Email` + userID;
    no-op se nil). **`workspace/store.go` INTOCADO.**
  - **Teste** (dbtest + callback real): convite pending + 1º login do email → membership materializada
    com a role do convite, convite vira `accepted` (CA-24); convite expirado/revogado → nada (CA-25);
    user já membro + convite pending → sem 2ª membership, convite vira accepted (CA-26); **login sem
    convites = comportamento idêntico** (CA-27 — suítes da fase 01 verdes sem editar nada; `NewStore`
    intocado); re-vínculo por email + convite → entra (CA-28). **RNF-05 (mutação pinada PELO callback
    real):** `TestResolveConviteNoLogin` exercita `auth.Service.HandleCallback` com
    `Config.ResolveInvites` ligado (`invSvc.ResolveForEmail` ou spy) e confirma a materialização +
    aceite; **uma mutação que remove a chamada a `ResolveInvites` do `HandleCallback` mata o teste**
    (um teste só de `ResolveForEmail` isolado NÃO cobre o furo — é complementar, não suficiente).
  - **Acceptance:** `go test ./internal/auth/... ./internal/workspace/... ./internal/invitations/...`
    verde; suítes de callback/login da fase 01 verdes sem editar asserções; a mutação no
    `HandleCallback` mata o teste pinado.

- **T-07 — Wiring no boot** (CA-24, RNF-05)
  - `internal/config/config.go`: `INVITE_TTL` (duration, default 336h; 0/inválido → default).
  - `cmd/server/main.go`: construir `invitations.NewService(db, ...)`, ligar `invSvc.ResolveForEmail`
    em **`auth.Config.ResolveInvites`** (antes de `auth.NewService`, NÃO no `workspace.NewStore`), e
    `api.WithInvitations(invSvc)` no router (plan §6/D-05).
  - **Teste:** `config_test.go` (default/parse do `INVITE_TTL`); o boot liga sem quebrar (exceção ao
    red estrutural documentada — espelha o `tsvc.SetOnChange` da fase 04).
  - **Acceptance:** `go build ./...` + `go test ./internal/config/...` verde; o servidor sobe.

## Frontend — UI de gestão na página `/workspace`

- **T-08 — `lib/api.ts` + hooks** (CA-30/31/32/33)
  - `lib/api.ts`: tipo `Invitation` + `listInvitations`/`createInvitation`/`revokeInvitation`/
    `addMember`/`removeMember`/`changeRole` (plan §4). `lib/queries/workspace.ts`: `invitationsKey`,
    `useInvitations(ws, {enabled})` + mutations que invalidam `invitationsKey`/`workspaceKey`.
  - **Teste** (jsdom, mock de `../../lib/api`): cada hook chama a rota certa com o corpo certo;
    mutations invalidam as queries certas (padrão dos hooks da fase 02).
  - **Acceptance:** `vitest run` dos hooks verde.

- **T-09 — Bloco de gestão na `Workspace.tsx` (SÓ owner) — convites** (CA-29/30/31)
  - Sob `useRole(ws.id) === 'owner'`, adicionar: form de convite (email + select role + enviar →
    `useCreateInvitation`; validação client espelha o backend — email com `@`); lista de pendentes
    (`useInvitations`, enabled só p/ owner) com email/role/"expira em…" + botão **revogar**
    (`useRevokeInvitation`). **JSX de leitura (nome + membros) INTOCADO** (CA-34). Estados de erro
    (já-membro/inválido) mostrados sem quebrar.
  - **Specs** (jsdom): owner vê o form + lista; member/viewer NÃO veem (CA-29); enviar convite chama
    a API e limpa o form (CA-30); revogar remove o item (CA-31); email vazio/sem @ não chama
    (CA-30).
  - **Acceptance:** `vitest run` da página verde; member/viewer sem controles.

- **T-10 — Gestão de membros na `Workspace.tsx` (SÓ owner) — role/remover/add existente** (CA-32/33)
  - Por membro: seletor de role (owner/member/viewer → `useChangeRole`) e **remover** (com
    confirmação → `useRemoveMember`); a UI **oculta/desabilita** remover/rebaixar do **último owner**
    (CA-32; o backend é a verdade — CA-18/19). Form/fluxo de **adicionar existente** por email →
    `useAddMember`; 404 `user_not_found` → oferece "enviar convite por email" (encadeia
    `createInvitation` — CA-33/plan §6).
  - **Specs** (jsdom): trocar role chama `PATCH`; remover chama `DELETE`; último owner sem ação de
    remover/rebaixar; add existente → 404 mostra o fallback de convite (CA-33).
  - **Acceptance:** `vitest run` da página verde.

- **T-11 — E2E `membros.spec.ts`** (CA-35)
  - Logado como owner (`loginViaStub`, `diogo.fixture@example.com`): convidar `cadu@example.com` →
    convite aparece nos pendentes; **simular o 1º login do convidado** — opção A: 2º contexto
    Playwright com o googlestub apontado pro email convidado (estender o stub/fixture pra um 2º
    usuário); opção B: helper `e2e/helpers.ts` que materializa o login do email + roda a resolução
    via SQL (`seedInvitedLogin`) — **fixar a opção na T** (preferir a que não acopla ao stub;
    documentar). Após a resolução: o novo membro **aparece na lista de membros** com a role do
    convite e o convite some dos pendentes. Owner **adiciona existente** por email → membership na
    hora. Owner **muda role** e **remove** um membro. **Non-owner** (member, via `seedWorkspaceMember`
    + troca de workspace ativo OU contexto próprio) abre `/workspace` → **sem controles de gestão**
    (CA-29). Reusar `loginViaStub`/`seedWorkspaceMember`; liberar a :8080 antes do E2E.
  - **Acceptance:** `membros.spec.ts` verde; o caminho convite→1º-login→membership passa ponta a
    ponta.

- **T-12 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: **isolamento multi-tenant** convite/membro entre 2 workspaces ·
    **invariante último-owner** sob concorrência (lock + re-check) · **gate de permissões** owner/
    member/viewer + ordem 404-antes-de-403 · **normalização de email** (lower/trim, unique parcial,
    case na resolução) · **resolução no login** (best-effort, não regride o login, idempotência
    já-membro, expirado/revogado não materializa) · **não-regressão** fases 01/02/10) + refutação
    dupla. Corrigir confirmados red→green. Registrar no fim deste arquivo.

- **T-13 — Gate + handoff** (sem deploy isolado — deploy da M2 é decisão do orquestrador)
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Antes do E2E,
    liberar a :8080** de qualquer dev server alheio (CLAUDE.md).
  - Handoff (CLAUDE.md "retomar…"), INDEX (17 construída), commit, briefing. O **deploy** segue a
    política da M2 (único no fim da milestone, ou conforme o orquestrador decidir — NÃO deployar por
    fase sem ordem). NÃO tocar Caddy/PrimaOS/portas 80-443.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

> Verificação adversarial ancorada no código real (fases 01/02/10 já construídas). **3 furos
> confirmados 2/2 aplicados** na spec/plan/tasks ANTES do build (Regra Nº1). Os findings refutados/
> descartados **não** entram aqui.

- **[HIGH] Resolver de convite ligado no STORE ERRADO.** O login real é
  `auth.Service.HandleCallback` → `s.store.UpsertUserFromGoogle`, e `s.store` é construído DENTRO de
  `auth.NewService` (`auth/oauth.go:50`: `workspace.NewStore(cfg.DB)`) — instância diferente da
  passada em `api.WithAuth(...)`. Ligar a resolução no `NewStore` do routing deixaria o login real
  **no-op**. **Correção (opção (b) travada):** `workspace.NewStore` fica **INTOCADO** (assinatura
  byte-idêntica — evita os 13 call sites, incl. `auth/oauth.go:50` e `calendar/service.go:51`);
  `auth.Config` ganha o campo `ResolveInvites func(ctx, userID, email) error` (default no-op);
  `auth.Service.HandleCallback` chama `ResolveInvites` LOGO APÓS `UpsertUserFromGoogle` retornar
  (já tem `ui.Email` + userID); `cmd/server` injeta `invSvc.ResolveForEmail`. **spec** CA-24/26/27/
  28/34/RNF-05 reescritos (resolução pelo callback real, `NewStore` intocado); **plan** D-01/D-05/§6
  (apontam `auth/oauth.go:50` e `auth/handlers.go:78`, declaram `NewStore` intocado, descrevem
  `auth.Config.ResolveInvites`, assinatura de `ResolveForEmail` sem `pgx.Tx`) + armadilha nova em
  §8 + §7 paralelização; **tasks** T-06 (editar `auth.Config`/`HandleCallback`, não o store; PINAR
  teste que prova a resolução PELO callback real → mutação que remove a chamada mata o teste) e T-07
  (injetar em `auth.Config.ResolveInvites`, não no `NewStore`).
- **[MEDIUM] `domain.NotFound("user_not_found", ...)` não compila.** O construtor real é
  `domain.NotFound(msg string)` e HARDCODA `Code:"not_found"` (`internal/domain/errors.go:30`). Pra
  ter code custom `user_not_found` (CA-11), usar o LITERAL
  `&domain.Error{Status: http.StatusNotFound, Code: "user_not_found", Message: "…"}` (padrão de
  `calendar/client.go:45`). **plan** §3 ("Add inexistente") corrigido + nota de que os 404
  `not_found` puros (CA-15/16/20) seguem usando `domain.NotFound(msg)`; **spec** CA-11 ajustada
  (code custom via literal, não o `not_found` genérico).
- **[LOW] Auto-convite: CA-06 dizia 400, plan resolve 409 `already_member` (contradição).**
  **Correção:** alinhar CA-06 ao 409 — o owner É membro, então auto-convite cai em já-membro → 409
  `already_member` (CA-07). **spec** CA-06 reescrita (removida a frase "→ 400" do auto-convite; 400
  fica restrito a email malformado/ausente e role inválida member|viewer). Plan §3 já resolvia 409;
  agora spec e plan concordam.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> Suítes finais, findings do `verify-build`, correções aplicadas.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> Suítes finais, findings do `verify-build`, correções aplicadas.
