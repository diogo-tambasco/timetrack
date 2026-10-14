# plan.md — Fase 17 · Membros & convites (workspace multiplayer)

> Como construir a spec. Decisões de arquitetura (D-xx) + modelo de dados + contratos + ordem de
> tarefas. **Reusa**: re-vínculo por email da fase 01 (`workspace/store.go::UpsertUserFromGoogle`),
> `GET /workspaces/{id}` com `members[]` (fase 02, `workspace/handlers.go`), `RequireMember`
> (fase 01), `RequireWriteRole` (fase 02), enum `workspace_role` (0003), página `/workspace`
> (fase 10). **Adiciona**: migração `0017_invitations`, pacote `internal/invitations`, gate
> Owner-only, rotas no subrouter do workspace, hook de resolução no login, UI de gestão.

---

## 1. Arquitetura geral

```
Backend:
  db/migrations/0017_invitations.sql        ← tabela invitations + índice único parcial pending
  internal/invitations/            (NOVO pacote)
     service.go                              ← Create/List/Revoke (convites) +
                                               AddExistingMember/RemoveMember/ChangeRole (membros) +
                                               ResolveForEmail (hook do login)
     service_test.go / *_test.go             ← TDD (normalização, unique, último-owner, isolamento,
                                               resolução no login)
  internal/api/invitations_routes.go (NOVO)  ← mountInvitations: rotas de gestão no subrouter do ws,
                                               atrás do gate Owner-only (RequireOwner)
     invitations_routes_test.go              ← 201/400/403/404/409; isolamento; tabela de permissões
  internal/api/middleware_role.go (existe)   ← + RequireOwner (aditivo; RequireWriteRole intocado)
  internal/auth/oauth.go (existe)             ← auth.Config ganha campo ResolveInvites (default no-op);
                                               auth.NewService NÃO muda o NewStore(cfg.DB) interno
  internal/auth/handlers.go (existe)          ← HandleCallback chama ResolveInvites LOGO APÓS
                                               UpsertUserFromGoogle retornar (já tem ui.Email + userID)
  internal/workspace/store.go (existe)       ← INTOCADO — NewStore mantém assinatura byte-idêntica
                                               (resolução NÃO passa pelo store; ver D-01/D-05)
  internal/api/auth_routes.go (existe)        ← monta mountInvitations no subrouter do workspace
  cmd/server/main.go (existe)                ← constrói invitations.Service e injeta ResolveForEmail
                                               em auth.Config.ResolveInvites + WithInvitations no router

Frontend:
  lib/api.ts (existe)                        ← + tipos Invitation + createInvitation/listInvitations/
                                               revokeInvitation/addMember/removeMember/changeRole
  lib/queries/workspace.ts (existe)          ← + invitationsKey + hooks (useInvitations + mutations)
  routes/cockpit/Workspace.tsx (existe)      ← bloco de gestão SÓ pra owner (form convite + pendentes
                                               + gestão de membros); leitura intocada (CA-29/34)
  e2e/helpers.ts (existe)                    ← + helper de convite/segundo-login no stub (E2E CA-35)
  e2e/membros.spec.ts             (NOVO)     ← E2E da fase
```

## 2. Migração — `0017_invitations.sql` (dona: fase 17)

> Número: `0016_timer_guard` é a última migração presente (confirmado em
> `backend/internal/db/migrations/`). **`0017` é o próximo livre** — fixado na T-01 lendo a pasta.
> Reusa o enum `workspace_role` (`owner|member|viewer`) já criado na 0003 (não recriar).

SQL (em prosa — a T-01 escreve o `.sql`):

- `CREATE TABLE invitations` com:
  - `id uuid PRIMARY KEY DEFAULT gen_random_uuid()`
  - `workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE`
  - `email text NOT NULL` (o service grava sempre `lower(trim(...))`)
  - `role workspace_role NOT NULL` (o CHECK do enum não impede `owner`; o **service** rejeita
    `owner` no payload — CA-03; opcionalmente um CHECK `role <> 'owner'` reforça no banco)
  - `token text NOT NULL UNIQUE` (opaco, gerado server-side)
  - `status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','revoked'))`
  - `invited_by uuid NOT NULL REFERENCES users(id)`
  - `created_at timestamptz NOT NULL DEFAULT now()`
  - `expires_at timestamptz` (NULL = sem expiração; o service preenche com `now()+TTL`)
- **Índice único parcial (CA-02)**:
  `CREATE UNIQUE INDEX ux_invitations_pending ON invitations (workspace_id, lower(email)) WHERE
  status='pending';` — garante no máximo 1 pending por (workspace, email), case-insensitive.
- **Índice de resolução no login (RNF-02)**:
  `CREATE INDEX idx_invitations_email_pending ON invitations (lower(email)) WHERE status='pending';`
  — a varredura do login busca por email normalizado entre pendentes.
- INDEX.md: registrar **0017 dona da fase 17**.

## 3. Backend

### D-01 — pacote novo `internal/invitations` (não inchar `workspace`)

O `workspace.Store` é a camada de queries do login/membership (folha — não importa `api`). Convites
+ gestão de membros é um domínio novo, então vive num **pacote novo `internal/invitations`** com seu
próprio `Service` (recebe `*pgxpool.Pool`). Por que separado e não dentro de `workspace`:
1. Mantém `workspace.Store` enxuto (CA-34/RNF-05) — só a extensão do `UpsertUserFromGoogle` o toca.
2. O `Service` retorna `*domain.Error` (pacote `internal/domain`, fase 02), que o `api`
   converte pelo `writeDomainError` existente → envelope canônico, **nunca 500 vazado**.
3. **Evita ciclo + liga no caminho de login REAL:** o login de produção é
   `auth.Service.HandleCallback` → `s.store.UpsertUserFromGoogle`, e esse `s.store` é construído
   **dentro** de `auth.NewService` (`auth/oauth.go:50`: `workspace.NewStore(cfg.DB)`) — uma instância
   **diferente** da passada em `api.WithAuth(...)`. **Por isso a resolução NÃO é injetada no
   `workspace.NewStore`** (faria o login real ficar no-op) **e** o `NewStore` mantém a assinatura
   **byte-idêntica** (evita tocar os 13 call sites — incl. `auth/oauth.go:50` e `calendar/
   service.go:51`). Solução adotada (opção (b) da verificação): **`auth.Config` ganha um campo
   `ResolveInvites func(ctx context.Context, userID, email string) error`** (default no-op se nil);
   `auth.Service.HandleCallback` chama `ResolveInvites` **logo após `UpsertUserFromGoogle` retornar**
   (o callback já tem `ui.Email` e o `userID`); o `cmd/server` injeta `invSvc.ResolveForEmail` em
   `auth.Config.ResolveInvites`. `invitations.Service` **não importa `workspace` nem `auth`** (folha;
   usa `*pgxpool.Pool`/`crypto/rand`) — sem ciclo. (Padrão idêntico ao
   `tsvc.SetOnChange(hub.BroadcastTimer)` da fase 04 — injeção pra ligar sem acoplar.)

### D-02 — `Service` (assinaturas)

```go
type Service struct { db *pgxpool.Pool; now func() time.Time; inviteTTL time.Duration }

// Convites (Owner-only — o gate é no router; o service NÃO confia, mas o
// caminho de produção sempre passa pelo RequireOwner).
func (s *Service) Create(ctx, workspaceID, invitedByUserID, email, role string) (Invitation, error)
func (s *Service) List(ctx, workspaceID string) ([]Invitation, error)   // só pending (CA-13)
func (s *Service) Revoke(ctx, workspaceID, inviteID string) error       // pending->revoked

// Membros existentes.
func (s *Service) AddExistingMember(ctx, workspaceID, email, role string) (Member, error)
func (s *Service) RemoveMember(ctx, workspaceID, userID string) error    // invariante last_owner
func (s *Service) ChangeRole(ctx, workspaceID, userID, role string) error// invariante last_owner

// Hook do login (chamado pelo auth.HandleCallback, APÓS UpsertUserFromGoogle retornar;
// roda em transação própria sobre o pool, sob advisory lock por google_id — D-05).
// Assinatura casa o tipo do campo auth.Config.ResolveInvites.
func (s *Service) ResolveForEmail(ctx context.Context, userID, email string) error
```

- **Normalização (CA-05):** `normEmail(e) = strings.ToLower(strings.TrimSpace(e))` aplicada em
  `Create`, `AddExistingMember`, e na busca de `ResolveForEmail`. Validação de formato mínima
  (`strings.Contains(e,"@")` + não-vazio) antes do SQL — senão `domain.Validation`.
- **Role válido (CA-03/CA-06):** `Create`/`AddExistingMember` aceitam só `member|viewer`;
  `ChangeRole` aceita `owner|member|viewer`. Fora disso → `domain.Validation`.
- **Já-membro (CA-07/CA-12):** antes de criar convite/membro, checar
  `SELECT 1 FROM workspace_members wm JOIN users u ON u.id=wm.user_id WHERE wm.workspace_id=$1 AND
  lower(u.email)=$2` → existe → `domain.Conflict("already_member", ...)`.
- **Pending duplicado (CA-08):** `Create` tenta o INSERT; `ON CONFLICT (ux_invitations_pending)` →
  **decisão: responder 409 `invite_pending`** (mais previsível que "retorna o existente"; a UI já
  mostra o pending na lista). Nunca 500, nunca 2 linhas.
- **Auto-convite (CA-06):** se `normEmail(email) == normEmail(emailDoSolicitante)` → 400. O service
  recebe o email do solicitante? Não — recebe só `invitedByUserID`; então a checagem é: o
  solicitante é owner (logo já é membro) → cair na regra "já-membro" cobre o auto-convite (o owner é
  membro). **Decisão:** basta a checagem de já-membro (CA-07) — convidar o próprio email = convidar
  um membro = 409. (Registra-se que não há um 400 separado de "auto-convite"; é o 409 de já-membro.)
- **Add inexistente (CA-11):** `AddExistingMember` resolve `user_id` por `lower(email)` em `users`;
  sem linha → **404 com code `user_not_found`**. **⚠️ Construtor:** `domain.NotFound(msg)` recebe
  **só a mensagem** e HARDCODA `Code:"not_found"` (`internal/domain/errors.go:30`) — não aceita code
  custom. Pra ter `code:"user_not_found"`, usar o **literal**:
  `&domain.Error{Status: http.StatusNotFound, Code: "user_not_found", Message: "usuário não
  encontrado — use o convite por email"}` (padrão de `calendar/client.go:45`). Os 404 `not_found`
  PUROS (CA-15 revoke de outro ws, CA-16/CA-20 membro inexistente) usam o `domain.NotFound(msg)`
  existente (esses estão OK — code `not_found`). Alternativa "criar convite pending automaticamente"
  **descartada** (D-04 abaixo).
- **Último owner (CA-18/19):** `RemoveMember`/`ChangeRole` rodam numa transação sob
  `pg_advisory_xact_lock(hashtext(workspaceID))` (serializa mutações do quadro do workspace, evita
  corrida de "2 owners saindo ao mesmo tempo"). Dentro do lock:
  `SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND role='owner'` e a role-alvo; se
  a operação levaria `owners` a 0 → `domain.Conflict("last_owner", ...)` (409), rollback.

### D-03 — `inviteTTL` (config opcional)

`INVITE_TTL` (env, duration, default **14 dias** = `336h`). 0/inválido → default. Não obrigatório
pro boot. `expires_at = now() + inviteTTL` na criação. (`now` injetável pro teste de expiração —
espelha o `WithClock` do `timer.Service`/`guard.Cutter`.)

### D-04 — `POST /members` de email inexistente → 404, NÃO auto-convite

Avaliado: "se o email não existe em `users`, criar um convite pending no endpoint de membro".
**Descartado** — borra a fronteira dos dois fluxos (o usuário pediu os dois **explícitos**) e torna
o endpoint imprevisível (às vezes membership, às vezes convite, mesmo verbo/rota). Decisão: `POST
/members` é **só** pra quem já tem conta; inexistente → 404 com mensagem que orienta o convite. A UI
(CA-33) pode encadear: 404 → oferece o botão "convidar por email" (que chama `POST /invitations`).

### D-05 — resolução no login PELO callback de auth, pós-commit (atomicidade + caminho real)

> **Furo HIGH corrigido (verificação da spec, sessão 19):** o login de produção é
> `auth.Service.HandleCallback` (`auth/handlers.go`, ~linha 78) → `s.store.UpsertUserFromGoogle`, e
> `s.store` é construído **dentro** de `auth.NewService` (`auth/oauth.go:50`:
> `workspace.NewStore(cfg.DB)`) — instância diferente da do routing. Ligar a resolução no `NewStore`
> passado em `api.WithAuth(...)` deixaria o login real **no-op**. **Decisão (opção (b)):** ligar a
> resolução no **callback de auth**, não no store.

O ponto de injeção é o **`auth.Config`**: ganha um campo
`ResolveInvites func(ctx context.Context, userID, email string) error` (default no-op se nil). O
`auth.Service.HandleCallback`, **logo após `UpsertUserFromGoogle` retornar** (já tem `ui.Email` do
`userinfo` e o `userID` materializado), chama `cfg.ResolveInvites(ctx, userID, ui.Email)`. O
`workspace.NewStore` fica **byte-idêntico** (assinatura intocada — CA-27/34/RNF-05). O
`cmd/server` injeta `invSvc.ResolveForEmail` em `auth.Config.ResolveInvites`.

`ResolveForEmail` roda em **transação própria** (sobre o `*pgxpool.Pool`), sob
`pg_advisory_xact_lock(hashtext(google_id))` quando disponível (ou por user_id), de modo que um erro
de resolução **não desfaz o login** (que já commitou no `UpsertUserFromGoogle`). A query:

```sql
-- materializa memberships dos convites válidos do email:
WITH inv AS (
  SELECT id, workspace_id, role FROM invitations
   WHERE lower(email) = lower($email)
     AND status = 'pending'
     AND (expires_at IS NULL OR expires_at > now())
   FOR UPDATE
), ins AS (
  INSERT INTO workspace_members (workspace_id, user_id, role)
  SELECT workspace_id, $userID, role FROM inv
  ON CONFLICT (workspace_id, user_id) DO NOTHING     -- CA-26 (já-membro não duplica)
)
UPDATE invitations SET status='accepted'
 WHERE id IN (SELECT id FROM inv);                    -- consome o convite (CA-24/26)
```

- **Best-effort (CA-24):** a resolução roda **pós-commit do login**, em transação própria, então um
  erro nela **não desfaz** o login (atomicidade trivial — CA-27). Erro é **logado e engolido** (o
  login já está commitado e é prioritário). A query é simples e indexada (RNF-02).
- **CA-27/CA-34 (não-regressão):** `workspace.NewStore` **não muda de assinatura** (nenhum call site
  afetado; o `handlers.go` de auth ganha **a chamada** a `ResolveInvites`, o `oauth.go` ganha **o
  campo** no `Config`). Login sem convites = comportamento idêntico (o no-op default não faz nada).
- **RNF-05 (mutação pinada):** a prova é **PELO callback real** — um teste exercita
  `auth.Service.HandleCallback` (com `Config.ResolveInvites` ligado num spy/`invSvc.ResolveForEmail`)
  e confirma que o convite foi materializado/aceito. Uma mutação que **remove a chamada a
  `ResolveInvites` do `HandleCallback`** mata esse teste. Um teste só de `ResolveForEmail` isolado
  **não** cobre o furo (não prova que o callback chama o hook) — é complementar, não suficiente.

### D-06 — `RequireOwner` (gate aditivo)

`api/middleware_role.go` ganha (sem tocar `RequireWriteRole`):

```go
func RequireOwner(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if reqctx.MembershipFrom(r.Context()).Role != "owner" {
      respond.Error(w, http.StatusForbidden, "forbidden", "só o dono gere membros e convites")
      return
    }
    next.ServeHTTP(w, r)
  })
}
```

Aplicado **só** ao subrouter de invitations/members (não ao subrouter inteiro do workspace — senão
quebraria leitura/escrita de tasks pra member). Roda **depois** do `RequireMember` (404 não-membro
antes do 403 — CA-21.1) e do `RequireWriteRole` (que já está no subrouter; member passa nele e é
barrado pelo `RequireOwner`; viewer é barrado já no `RequireWriteRole` em métodos de escrita, e no
`RequireOwner` no GET — ordem garante 403 nos dois). **Atenção:** `GET /invitations` é leitura, então
`RequireWriteRole` não barra member nela — o `RequireOwner` é quem barra (CA-21).

## 4. Contratos de API (REST)

Todas aninhadas em `/api/v1/workspaces/{workspaceId}`, atrás de `RequireAuth → RequireMember →
RequireWriteRole → RequireOwner` (o `RequireOwner` no subrouter de invitations/members).

| Método | Rota | Corpo | Sucesso | Erros |
|---|---|---|---|---|
| `POST` | `/workspaces/{id}/invitations` | `{email, role}` | `201 {id,email,role,status,expires_at,created_at}` | 400 validation_error · 403 forbidden · 404 not_found(não-membro) · 409 already_member · 409 invite_pending |
| `GET` | `/workspaces/{id}/invitations` | — | `200 {invitations:[{id,email,role,status,expires_at,created_at}]}` | 401 · 403 · 404 |
| `DELETE` | `/workspaces/{id}/invitations/{inviteId}` | — | `204` | 403 · 404 not_found(outro ws/inexistente) · 409 invite_not_pending |
| `POST` | `/workspaces/{id}/members` | `{email, role}` | `201 {user_id,name,email,role}` | 400 · 403 · 404 user_not_found · 409 already_member |
| `DELETE` | `/workspaces/{id}/members/{userId}` | — | `204` | 403 · 404 not_found · 409 last_owner |
| `PATCH` | `/workspaces/{id}/members/{userId}` | `{role}` | `200 {user_id,role}` | 400 · 403 · 404 not_found · 409 last_owner |

- **Envelope de erro:** `{"error":{"code","message"}}` (convenção INDEX). `validation_error` (400),
  `forbidden` (403), `not_found`/`user_not_found` (404), `already_member`/`invite_pending`/
  `last_owner`/`invite_not_pending` (409), `unauthorized` (401).
- **`token` NUNCA** aparece em nenhuma resposta (RNF-04).
- **Resolução de convite no login** = **não é rota** (hook do `UpsertUserFromGoogle`, D-05).

### Frontend (`lib/api.ts` — adições)

```ts
export interface Invitation {
  id: string; email: string; role: 'member' | 'viewer';
  status: 'pending' | 'accepted' | 'revoked';
  expires_at: string | null; created_at: string;
}
export function listInvitations(ws: string): Promise<Invitation[]>;
export function createInvitation(ws: string, b: {email: string; role: 'member'|'viewer'}): Promise<Invitation>;
export function revokeInvitation(ws: string, id: string): Promise<void>;
export function addMember(ws: string, b: {email: string; role: 'member'|'viewer'}): Promise<WorkspaceMember>;
export function removeMember(ws: string, userId: string): Promise<void>;
export function changeRole(ws: string, userId: string, role: WorkspaceRole): Promise<void>;
```

Hooks em `lib/queries/workspace.ts`: `invitationsKey(ws)`, `useInvitations(ws, {enabled: isOwner})`,
`useCreateInvitation`/`useRevokeInvitation`/`useAddMember`/`useRemoveMember`/`useChangeRole` —
mutações invalidam `invitationsKey(ws)` e/ou `workspaceKey(ws)` (a lista de membros do
`GET /workspaces/{id}`).

## 5. Libs / deps

- **Zero dep nova** (RNF-06). Reusa: `chi` (router), `pgx` (db + `pgx.Tx` no hook), `crypto/rand`
  via `auth.NewSessionToken` pro `token` opaco (ou um helper local em `invitations` espelhando-o —
  decisão na T pra não importar `auth` e criar ciclo: `invitations` é folha; usa `crypto/rand`
  direto, não `auth`). Front: TanStack Query + `lib/api.ts` existentes.

## 6. Pontos de integração com o baseline (ADITIVO vs INTOCADO)

- **`workspace/store.go::NewStore` + `UpsertUserFromGoogle`** — **INTOCADO** (assinatura e corpo
  byte-idênticos; a resolução NÃO passa pelo store — D-01/D-05). As suítes `callback_*_test.go`,
  `concurrent_login_test.go`, `oauth_token_test.go` seguem verdes sem editar nada.
- **`internal/auth/oauth.go` (`Config` + `NewService`)** — **ADITIVO**: `auth.Config` ganha o campo
  `ResolveInvites func(ctx context.Context, userID, email string) error` (default no-op se nil).
  `auth.NewService` **continua** construindo `s.store = workspace.NewStore(cfg.DB)` (linha 50, sem
  mudança). Só o campo novo no `Config`.
- **`internal/auth/handlers.go::HandleCallback`** — **ADITIVO**: logo após `UpsertUserFromGoogle`
  retornar (~linha 78, já com `ui.Email` + `userID`), chama `s.cfg.ResolveInvites(ctx, userID,
  ui.Email)` (no-op se nil). Resto do callback (sessão, cookie, oauth_tokens) **byte-idêntico**.
- **`workspace/handlers.go::HandleGetWorkspace`** — **INTOCADO**. A UI de gestão consome a mesma
  resposta (`members[]`); a lista de convites vem da rota nova `GET /invitations`.
- **`workspace/middleware.go::RequireMember`** — **INTOCADO** (404 não-membro roda antes do owner).
- **`api/middleware_role.go::RequireWriteRole`** — **INTOCADO**; `RequireOwner` é **adicionado** ao
  mesmo arquivo.
- **`api/auth_routes.go::mountAuth`** — **ADITIVO**: dentro do `wr.Route("/workspaces/{workspaceId}")`
  (depois de `mountCore`), `if cfg.invitations != nil { mountInvitations(wr, cfg.invitations) }`.
  `mountInvitations` cria um sub-grupo com `r.Group(func(o chi.Router){ o.Use(RequireOwner); ... })`
  pras rotas de gestão.
- **`cmd/server/main.go`** — **ADITIVO**: constrói `invitations.NewService(db, ...)`, liga
  `invSvc.ResolveForEmail` em `auth.Config.ResolveInvites` (antes de `auth.NewService`), e
  `api.WithInvitations(invSvc)` no router.
- **`routes/cockpit/Workspace.tsx`** — **ADITIVO**: bloco de gestão renderizado só se `useRole(ws.id)
  === 'owner'`; o JSX de leitura (nome + membros) **não muda** (CA-29/34).

### Decisão UI (CA-33) — form único com fallback

O form de "adicionar pessoa" tenta primeiro o caminho conforme um toggle, OU encadeia: digita email
+ role → "Adicionar"; o front chama `POST /members`; se **404 user_not_found**, mostra "Essa pessoa
ainda não tem conta — enviar convite?" → botão chama `POST /invitations`. Mantém 1 campo de email,
cobre os dois fluxos sem confundir. (Detalhe final na T-10; o importante é cobrir os dois.)

## 7. Dependências da fase + paralelização (PRD §9)

- **Depende de:** fase **01** (auth/re-vínculo/`UpsertUserFromGoogle`/`RequireMember`/`RequireAuth`),
  fase **02** (`GET /workspaces/{id}` com `members[]`, `RequireWriteRole`, `domain.Error`), fase
  **10** (página `/workspace`, `useRole`/`useCurrentWorkspace`). Todas **já construídas e
  deployadas** — sem espera.
- **PARALELIZÁVEL (worktree): SIM.** Migração própria (`0017`, número fixo — sem colisão com 0018/
  0019/0020 das outras worktrees), pacote backend novo (`internal/invitations`), rotas novas, UI
  num bloco novo da página. Toca arquivos compartilhados **de forma cirúrgica e aditiva**:
  `auth/oauth.go` (campo novo no `Config`; `NewStore` interno intocado), `auth/handlers.go` (1
  chamada a `ResolveInvites` no `HandleCallback` — contenção baixa, nenhuma outra fase da M2 mexe
  no login), `api/middleware_role.go` (só ADD `RequireOwner`), `api/auth_routes.go` (1 `if`),
  `Workspace.tsx` (página que **nenhuma** outra fase da M2 toca). **`workspace/store.go` fica
  INTOCADO** (assinatura do `NewStore` preservada — sem mexer nos 13 call sites). **Não** toca `tasks_handlers.go`
  (contenção 18/19), `App.tsx` (contenção 18/23) nem `realtime/hub.go` (21). Collab seeda membros
  por SQL (helpers `seedWorkspaceMember` já existem) — não precisa desta fase pra testar as outras.
- **Ordem:** é a **fundação** (M2 §9) — sem dependência de outra fase da M2. Pode ser a 1ª a buildar.

## 8. Armadilhas conhecidas (vetadas)

- **NÃO** remover/rebaixar o último owner sem o re-check sob `advisory_xact_lock(workspaceID)`
  (CA-18/19) — duas operações concorrentes podiam ver `owners=2` e ambas tirar o owner → 0 owners.
- **NÃO** esquecer o `WHERE workspace_id` em nenhuma query (CA-22) — vazamento entre workspaces;
  cada query de convite/membro filtra pelo `{workspaceId}` da rota, **e** a de revoke/PATCH/DELETE
  casa `id/userId AND workspace_id` (recurso de outro ws → 404, não 403, não vaza).
- **NÃO** comparar email com case/espaço cru — sempre `lower(trim)` dos dois lados (CA-05/24);
  o índice parcial usa `lower(email)`, então a checagem de já-pending tem que usar `lower` também.
- **NÃO** vazar o `token` em resposta/listagem (RNF-04).
- **NÃO** ligar a resolução no `workspace.NewStore` do routing (`api.WithAuth`) — o login real usa o
  store construído DENTRO de `auth.NewService` (`auth/oauth.go:50`), instância diferente → ficaria
  no-op. A resolução vai no **callback de auth** (`auth.Config.ResolveInvites` chamado pelo
  `HandleCallback`), com o `NewStore` intocado (D-01/D-05).
- **NÃO** deixar a resolução de convite no login **quebrar o login** (CA-27) — resolução em
  transação **própria pós-commit**, erro logado e engolido (D-05). Mutação que **remove a chamada a
  `ResolveInvites` do `HandleCallback`** deve matar um teste do callback real (RNF-05) — teste de
  store/service isolado não basta.
- **NÃO** aplicar `RequireOwner` ao subrouter **inteiro** do workspace — só ao grupo de
  invitations/members (senão member perde escrita de task/projeto — regressão da fase 02).
- **NÃO** materializar 2ª membership pra quem já é membro na resolução do login (CA-26) —
  `ON CONFLICT (workspace_id,user_id) DO NOTHING`, e mesmo assim marcar o convite accepted.
- **NÃO** aceitar `role='owner'` em `POST /invitations` nem `POST /members` (CA-03/CA-10) — owner só
  por `PATCH` (promoção), nunca por convite/add direto.
- **NÃO** mudar o JSX de leitura da página `/workspace` (CA-29/34) — só **adicionar** o bloco de
  gestão sob `isOwner`; member/viewer veem exatamente a tela da fase 10.
- **NÃO** criar ciclo de import (`invitations` ↔ `workspace`): `invitations` é folha (não importa
  `workspace`); o login chama `invitations` por **função injetada** (D-01/D-05).
