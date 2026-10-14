# spec.md — Fase 17 · Membros & convites (workspace multiplayer) [FUNDAÇÃO]

> Fonte de verdade da fase. Resolve a dor **P1** do `MILESTONE-2.md §2/§4` (fase 17): hoje só dá
> pra adicionar membro **por SQL** — não há porta de entrada de gente no workspace. Esta fase é a
> **fundação do multiplayer da M2**: API de convite + gestão de membros, com **gestão Owner-only**.
> É a 1ª fase a **escrever** em `workspace_members` por API (até hoje só o 1º login criava o owner).
>
> **Decisões travadas com o Diogo (HITL, `MILESTONE-2.md §7.1`):**
> 1. **Convite = os DOIS fluxos:** (A) **convite por email pendente** — Owner cria um convite que
>    fica `pending` e **resolve no 1º login daquele email** (reusa o re-vínculo da fase 01,
>    `workspace/store.go::UpsertUserFromGoogle`); (B) **buscar user existente** — Owner adiciona
>    direto, por email, quem **já tem conta** → vira membership na hora (sem aceite).
> 2. **Gestão é Owner-only:** convidar, revogar convite, remover membro, mudar role. Member e viewer
>    **NÃO** gerem o quadro (403). Leitura (ver membros) segue pra todos (herdado da fase 10).
> 3. **Invariante do último Owner:** nunca remover/rebaixar o ÚLTIMO owner do workspace (inclusive a
>    si mesmo) — o workspace sempre tem ≥1 owner.
> 4. **Esta fase SUPERA a fase 10:** a página `/workspace` era "100% leitura, sem convite/rename
>    (sem API no MVP)". A 17 **adiciona mutações** na página — **só pra Owner** (member/viewer
>    seguem vendo só leitura). Rename/criação de workspace continuam **fora** (M2 §3).

---

## 1. Objetivo

Dar ao Owner de um workspace as ferramentas pra **montar e gerir o time** sem tocar no banco:

1. **Convidar por email** quem ainda não tem conta — o convite fica pendente e a pessoa **entra
   sozinha no 1º login** (resolução automática pelo caminho de login que já existe).
2. **Adicionar direto** quem já tem conta (busca por email em `users`) — membership imediata.
3. **Gerir o quadro:** listar/revogar convites pendentes, remover membros, mudar role — tudo
   **Owner-only**, com a invariante de **nunca ficar sem owner**.
4. **Isolamento total:** convites e membros são sempre do **workspace da rota**; nada vaza entre
   workspaces (filtro `workspace_id` em toda query — mutação que o remove deve quebrar um teste).
5. **UI:** a página `/workspace` (fase 10) ganha um bloco de convites (form + lista de pendentes +
   revogar) e gestão de membros (remover, trocar role), **renderizado só pra Owner**.

O timer, tasks, relatórios, foco e o login normal seguem **intactos**: esta fase é **aditiva** —
uma tabela nova (`invitations`), um pacote novo (`internal/invitations`), rotas novas no subrouter
do workspace, e um **hook de resolução** no caminho de login (sem regredir o login atual). O hook
roda **pelo callback de auth** (`auth.Service.HandleCallback`), logo após `UpsertUserFromGoogle`
retornar — **não** por um store isolado: o `s.store` que o callback usa é construído **dentro** de
`auth.NewService` (`auth/oauth.go:50` → `workspace.NewStore(cfg.DB)`), instância diferente da
passada em `api.WithAuth(...)`, então ligar a resolução no `NewStore` do routing deixaria o login
real no-op (ver plan §D-01/D-05).

## 2. Definições precisas

- **Email normalizado.** O alvo de um convite e a busca de user existente normalizam o email com
  **`lower(trim(email))`**. O `users.email` já é `NOT NULL UNIQUE` (0002) — a comparação de convite
  ↔ user de login é por email normalizado (`lower(trim(...))` dos dois lados) pra casar variações de
  caixa/espaço.
- **Convite (`invitation`).** Uma linha em `invitations` com `status pending|accepted|revoked`. Um
  convite **`pending`** materializa a membership quando o email dono dele loga (fluxo A). Um convite
  **`accepted`** já virou membership; **`revoked`** foi cancelado pelo Owner e **nunca** materializa.
- **Convite válido pra resolução.** Um convite resolve no login SE `status='pending'` **E**
  (`expires_at IS NULL` **OU** `expires_at > now()`). Pending expirado **não** materializa (fica
  inerte; a UI pode mostrá-lo como expirado).
- **"Já é membro".** O email/user já tem linha em `workspace_members` do workspace. Convidar um
  email que já é membro → erro claro (não cria convite duplicado nem 2ª membership).
- **Último owner.** Owner é membership com `role='owner'`. A contagem de owners do workspace é
  `count(*) WHERE role='owner'`. **Remover** o único owner, ou **rebaixar** (PATCH role para
  member/viewer) o único owner, é **proibido** (409 — invariante).
- **Quem gere = Owner.** "Gestão" = `POST/DELETE /invitations`, `POST/DELETE/PATCH /members`. Todas
  exigem que **o solicitante seja `owner` do workspace da rota** (não basta não-viewer — member
  também recebe 403). É um gate **mais estrito** que o `RequireWriteRole` da fase 02 (que só barra
  viewer); por isso esta fase introduz um gate Owner-only escopado a essas rotas (ver plan §3.2).

## 3. User stories

- **US-01** — Como Owner, quero **convidar alguém por email** que ainda não usa o app, pra que ela
  entre no meu workspace **automaticamente no 1º login** — sem eu precisar mexer no banco.
- **US-02** — Como Owner, quero **adicionar direto** alguém que **já tem conta**, buscando por
  email, pra ela virar membro **na hora**, sem passo de aceite.
- **US-03** — Como Owner, quero **ver os convites pendentes** e **revogar** um que mandei por
  engano, pra controlar quem ainda pode entrar.
- **US-04** — Como Owner, quero **remover um membro** e **mudar a role** de alguém (member↔viewer,
  promover a owner), pra ajustar o time — sem nunca correr o risco de **ficar sem owner**.
- **US-05** — Como Member/Viewer, quero **ver** os membros do workspace (já vejo), mas **não** ter
  os controles de gestão na minha tela — porque não é meu papel (e a API me barra com 403 se eu
  tentar pela mão).
- **US-06** — Como convidado, quero que, **no meu 1º login**, qualquer convite pendente pro meu
  email vire membership automaticamente — sem clicar em link, sem fricção (reusa o re-vínculo).

## 4. Critérios de aceite (EARS — cada um vira 1 teste)

### Migração & modelo de dados

- **CA-01** (migração) — a fase 17 É dona da migração **`0017_invitations`** (próximo número livre
  após `0016_timer_guard`). Cria a tabela `invitations` com: `id uuid pk`, `workspace_id uuid NOT
  NULL REFERENCES workspaces(id) ON DELETE CASCADE`, `email text NOT NULL` (normalizado
  `lower/trim` pelo service), `role workspace_role NOT NULL` (reusa o enum `workspace_role` da
  0003), `token text NOT NULL UNIQUE`, `status text NOT NULL DEFAULT 'pending'` com **CHECK
  `status IN ('pending','accepted','revoked')`**, `invited_by uuid NOT NULL REFERENCES users(id)`,
  `created_at timestamptz NOT NULL DEFAULT now()`, `expires_at timestamptz NULL`. Os defaults são o
  backstop dos CAs abaixo.
- **CA-02** (unique parcial — 1 pending por email/workspace) — a migração cria um **índice único
  parcial** `ux_invitations_pending (workspace_id, lower(email)) WHERE status='pending'`: **no
  máximo 1 convite pending** por (workspace, email). Convidar o mesmo email 2× sem revogar/aceitar
  → o 2º bate no índice (o service trata como conflito/idempotente — CA-08, **não** 500).
- **CA-03** (role do convite) — a migração/CHECK + o service garantem que `invitations.role` só é
  `member` ou `viewer`. **`owner` não pode ser convidado** (o Owner não se convida; promover a owner
  é via PATCH de membro, CA-19). Convite com `role='owner'` no payload → **400 `validation_error`**
  (rejeitado no service, antes do SQL).

### Backend — criar convite (`POST /workspaces/{id}/invitations`)

- **CA-04** (criar convite pending) — `POST /workspaces/{id}/invitations` com `{email, role}`
  (Owner) cria um convite `status='pending'` com `email` **normalizado** (`lower(trim)`), `role`
  ∈ {member,viewer}, `token` opaco único (gerado server-side, ≥128 bits de entropia), `invited_by`
  = user da sessão, `expires_at` = `now() + DEFAULT_INVITE_TTL` (ex.: 14 dias — fixado no plan).
  Responde **201** com `{id, email, role, status:"pending", expires_at, created_at}` (NUNCA o
  `token` no corpo — RNF-04).
- **CA-05** (email normalizado na criação) — convidar `"  Cadu@Example.COM "` grava
  `email='cadu@example.com'`. Dois convites pra `cadu@example.com` e `CADU@example.com` no mesmo
  workspace colidem no índice parcial (CA-02) — são o **mesmo** alvo.
- **CA-06** (payload inválido) — `POST /invitations` sem `email`, com email malformado (sem `@`),
  ou sem `role` (ou role fora de {member,viewer}) → **400 `validation_error`** (validado antes do
  SQL). O **auto-convite** (Owner convidando o próprio email) **não** é um caso de 400: o owner já é
  membro, então cai na regra de já-membro → **409 `already_member`** (CA-07). O service não recebe o
  email do solicitante (só `invited_by`); a checagem de já-membro cobre o auto-convite sem regra
  separada (ver plan §3 "Auto-convite"). O **400** fica restrito a email malformado/ausente e role
  fora de {member,viewer}.
- **CA-07** (já-membro → erro claro) — convidar um email que **já é membership** do workspace →
  **409 `already_member`** (ou `validation_error` com mensagem clara — fixado no plan), sem criar
  convite. (Não duplica gente.)
- **CA-08** (idempotência do pending duplicado) — `POST /invitations` pro mesmo email enquanto já
  existe um pending → **não cria um 2º**; responde de forma determinística (**200** retornando o
  pending existente OU **409 `invite_pending`** — fixado no plan, mas **nunca 500** e **nunca** 2
  linhas pending). O índice parcial (CA-02) é o backstop.
- **CA-09** (re-convite após revogar) — se o único convite pra um email está `revoked`/`accepted`
  (não pending), um novo `POST /invitations` cria um **novo** pending (o índice parcial só barra
  pending↔pending). Cobre "revoguei por engano, reconvido".

### Backend — adicionar membro existente (`POST /workspaces/{id}/members`)

- **CA-10** (add existente por email) — `POST /workspaces/{id}/members` com `{email, role}` (Owner),
  quando o email **já existe em `users`** (normalizado), cria a membership **na hora** com a role
  pedida e responde **201** com `{user_id, name, email, role}`. Sem passo de aceite (fluxo B).
  `role` ∈ {member,viewer} no payload deste endpoint (promover a owner é PATCH; ver CA-19/decisão
  no plan §3.4).
- **CA-11** (add de quem não tem conta → cai no convite) — `POST /members` com email **inexistente**
  em `users` → **NÃO** cria membership (não há `user_id`). Resposta: **404 com code
  `user_not_found`** (code custom — NÃO o `not_found` genérico; construído por literal
  `&domain.Error{...}`, pois `domain.NotFound(msg)` hardcoda `Code:"not_found"` — ver plan §3) com
  mensagem orientando a usar o convite por email (decisão registrada no plan §3.4 — alternativa
  "cria convite pending automaticamente" foi avaliada e **descartada** pra manter os dois fluxos
  explícitos e o endpoint previsível).
- **CA-12** (add de quem já é membro) — `POST /members` com email que já é membership → **409
  `already_member`**, sem 2ª linha (a PK `(workspace_id, user_id)` é o backstop; o service trata
  antes pra dar a mensagem certa, não um 500 de unique_violation).

### Backend — listar / revogar convite

- **CA-13** (listar pendentes) — `GET /workspaces/{id}/invitations` (Owner) responde **só os
  convites do workspace da rota**, ordenados por `created_at` (recentes primeiro), com `{id, email,
  role, status, expires_at, created_at}` (sem `token`). Por padrão lista os `pending` (e
  opcionalmente os expirados como tal); aceitos/revogados podem ser omitidos (fixado no plan). NUNCA
  inclui convite de outro workspace (CA-22).
- **CA-14** (revogar) — `DELETE /workspaces/{id}/invitations/{inviteId}` (Owner) marca o convite
  **`status='revoked'`** (soft — preserva auditoria) e responde **204**. Convite já `accepted` →
  **409 `invite_not_pending`** (não dá pra revogar quem já entrou; remover é via DELETE de membro).
  Convite já `revoked` → **204 idempotente** (revogar de novo é no-op; nunca 500).
- **CA-15** (revogar convite de outro workspace = 404) — `DELETE
  /workspaces/{A}/invitations/{inviteDoWorkspaceB}` → **404 `not_found`** (o convite existe, mas não
  no workspace da rota — não vaza existência; a query filtra `id=$1 AND workspace_id=$2`).

### Backend — remover membro / mudar role

- **CA-16** (remover membro) — `DELETE /workspaces/{id}/members/{userId}` (Owner) remove a linha de
  `workspace_members` e responde **204**. (FK `ON DELETE CASCADE` já cuida de relações; tasks
  `assignee`/`requested_by` apontando pro removido **não** são alteradas nesta fase — fora de
  escopo; ver §7.) Remover quem não é membro do workspace → **404 `not_found`**.
- **CA-17** (mudar role) — `PATCH /workspaces/{id}/members/{userId}` com `{role}` ∈
  {owner,member,viewer} (Owner) atualiza a role e responde **200** com `{user_id, role}`. Role fora
  do enum → **400 `validation_error`**. Promover a `owner` é permitido aqui (é o caminho de criar um
  2º owner).
- **CA-18** (invariante — não remover o último owner) — `DELETE /members/{ownerUserId}` quando esse
  é o **único** owner do workspace → **409 `last_owner`**, sem remover. Com ≥2 owners, remover um
  deles é OK. (Owner removendo a si mesmo também cai nesta regra se for o último.)
- **CA-19** (invariante — não rebaixar o último owner) — `PATCH /members/{ownerUserId}` mudando a
  role do **único** owner pra member/viewer → **409 `last_owner`**, sem alterar. Rebaixar um owner
  quando há ≥2 owners é OK. Promover alguém a owner ANTES de rebaixar o atual contorna a regra
  (correto — passa a haver 2 owners no meio do caminho).
- **CA-20** (member/membro inexistente em PATCH) — `PATCH /members/{userId}` de um `userId` que não
  é membro do workspace → **404 `not_found`** (não vaza; espelha CA-16).

### Permissões (matriz `MILESTONE-2.md §8` — Owner-only)

- **CA-21** (gestão é Owner-only — 403) — **member** e **viewer** que chamam **qualquer** rota de
  gestão (`POST/DELETE /invitations`, `POST/DELETE/PATCH /members`) recebem **403 `forbidden`**,
  sem efeito no banco. (A leitura `GET /invitations` também é Owner-only — convites são gestão; um
  member não precisa ver convites pendentes. Fixado no plan; member/viewer → 403 no GET também.)
- **CA-21.1** (não-membro → 404 antes do 403) — um usuário **sem membership** no workspace da rota,
  ao chamar qualquer rota da fase, recebe **404** (do `RequireMember` da fase 01, que roda antes do
  gate de owner — não vaza existência). A ordem é sessão(401) → membership(404) → owner(403).
- **CA-22** (isolamento multi-tenant) — TODA query (criar/listar/revogar convite, add/remover/PATCH
  membro) filtra por `workspace_id` do `{workspaceId}` da rota. Owner do workspace A **nunca** vê,
  cria ou mexe em convite/membro do workspace B. **Mutação que remove o `WHERE workspace_id` deve
  matar um teste** (Constituição §8.4). Tabela-driven com 2 workspaces e ids cruzados.
- **CA-23** (sem sessão → 401) — qualquer rota da fase sem cookie de sessão → **401** canônico
  (`{"error":{"code":"unauthorized",...}}`), do `RequireAuth` da fase 01.

### Resolução do convite no 1º login (hook do callback — NÃO é rota)

- **CA-24** (materializa no 1º login) — QUANDO um usuário loga e existe ≥1 convite **válido pra
  resolução** (`pending` e não expirado — §2) com `lower(email)` = `lower(email do user)`, o sistema
  **cria a `workspace_members`** desse user em cada workspace convidado, com a **role do convite**, e
  marca o convite **`status='accepted'`**. A resolução é disparada **pelo callback de auth real**
  (`auth.Service.HandleCallback`), **logo após `UpsertUserFromGoogle` retornar** (o callback já tem
  o `email` do `userinfo` e o `userID`), via o hook `auth.Config.ResolveInvites` (default no-op se
  nil; o `cmd/server` injeta `invSvc.ResolveForEmail`). Roda **após o commit do login**, em
  transação própria sob o advisory lock por google_id (ver plan §3.5/D-05) — **não** por um store
  isolado do routing. O login **não falha** se a resolução der erro (best-effort, logado) — o login
  é prioritário. **RNF-05:** uma mutação que **remove a chamada a `ResolveInvites` do
  `HandleCallback`** deve matar um teste que prova a resolução PELO callback real (não só um teste de
  store isolado).
- **CA-25** (convite expirado/revogado não materializa) — convites `revoked` ou `accepted`, ou
  `pending` com `expires_at <= now()`, **NÃO** geram membership no login (filtro da CA-24). Um
  pending expirado segue inerte.
- **CA-26** (já-membro no login não duplica) — se o user que loga **já é membro** de um workspace
  que também tem um convite pending pro email dele (caso de borda: convidado e adicionado por
  outro caminho antes de logar), a resolução **não** cria 2ª membership (`INSERT ... ON CONFLICT
  (workspace_id,user_id) DO NOTHING`) e **mesmo assim marca o convite accepted** (consumido).
- **CA-27** (não regride o 1º login normal) — um login de email **sem** convites pendentes segue
  **idêntico** ao de hoje: cria user + workspace pessoal + membership owner (CA-04/05 da fase 01),
  e a sessão. `workspace.NewStore` fica **byte-idêntico** (assinatura intocada — a resolução NÃO
  passa por ele; ver plan §D-01); as suítes de `callback_*_test.go`/`store.go` da fase 01 seguem
  **verdes** (a mudança é no `auth.Config.ResolveInvites`, default no-op, chamado pelo
  `HandleCallback` após o `UpsertUserFromGoogle` — aditiva).
- **CA-28** (re-vínculo por email + convite) — o caminho de **re-vínculo** da fase 01 (google_id
  novo, email conhecido → re-vincula, dentro de `UpsertUserFromGoogle`) também resolve convites: o
  `HandleCallback` chama `ResolveInvites` com o mesmo `email`/`userID` após o upsert/re-vínculo
  retornar — um user antigo que recriou a conta Google e tinha convite pendente entra no workspace
  convidado ao re-logar.

### UI — página `/workspace` (estende a fase 10; supera "sem convite")

- **CA-29** (controles SÓ pra Owner) — na página `/workspace`, os controles de gestão (form de
  convite, lista de pendentes com "revogar", botões "remover"/seletor de role por membro) são
  renderizados **somente quando `useRole(ws.id) === 'owner'`**. Member e viewer veem a página de
  leitura **idêntica à fase 10** (nome + membros + roles), **sem nenhum** controle de gestão.
- **CA-30** (form de convite) — Owner preenche email + escolhe role (member/viewer) + envia → chama
  `POST /invitations`; sucesso adiciona o convite à lista de pendentes (invalida a query) e limpa o
  form; erro (já-membro, inválido) mostra mensagem sem quebrar a tela. Email vazio/sem `@` não
  chama a API (validação client-side espelha o backend).
- **CA-31** (lista de pendentes + revogar) — a página mostra os convites pendentes (email + role +
  "expira em…") com botão **revogar** que chama `DELETE /invitations/{id}` e remove o item da lista
  (invalida). Lista vazia → estado vazio discreto ("Nenhum convite pendente").
- **CA-32** (gestão de membros) — por membro (exceto leitura), o Owner tem **trocar role** (seletor
  owner/member/viewer → `PATCH /members/{userId}`) e **remover** (→ `DELETE /members/{userId}`,
  confirmação). O **próprio Owner** e o **último owner** não expõem ação que viole a invariante (a UI
  desabilita/oculta "remover"/"rebaixar" do último owner; o backend é a fonte da verdade — CA-18/19).
- **CA-33** (add existente) — a UI tem o caminho de "adicionar quem já tem conta" (mesmo form ou um
  toggle "já tem conta") → `POST /members`; user inexistente → mensagem orientando o convite por
  email (espelha CA-11). (Pode ser o mesmo form: tenta `POST /members`; 404 → oferece "enviar
  convite por email" que chama `POST /invitations`. Fixado no plan §6.)

### Não-regressão

- **CA-34** — `workspace/store.go` (`NewStore`, `UpsertUserFromGoogle`, `UserWithWorkspaces`,
  `MemberRole`), `workspace/handlers.go` (`GET /workspaces/{id}` com `members[]`),
  `workspace/middleware.go` (`RequireMember`), `api/middleware_role.go` (`RequireWriteRole`) seguem
  **byte-compatíveis no contrato** — em particular `workspace.NewStore` **não muda de assinatura**
  (evita os 13 call sites, incl. `auth/oauth.go:50` e `calendar/service.go:51`). A única mudança no
  caminho de login é **aditiva no `auth.Config`/`auth.Service.HandleCallback`** (campo novo
  `ResolveInvites`, chamada nova após o upsert). A fase é **aditiva** (tabela nova, pacote novo,
  rotas novas, hook de resolução no callback). As suítes das fases 01/02/10 seguem verdes sem editar
  asserções (exceto a extensão do `auth.Config`, sancionada e documentada no plan/tasks). A página
  `/workspace` de leitura (member/viewer) é **idêntica** à da fase 10.

### E2E (Playwright)

- **CA-35** — Logado como Owner (stub `diogo.fixture@example.com`): na página `/workspace`, o Owner
  **convida um email novo** (`cadu@example.com`) → o convite aparece na lista de pendentes;
  **simula-se o 1º login desse email** (via segundo contexto com o googlestub configurado pro email
  convidado, **ou** seed que materializa o login + roda a resolução — fixado no plan/tasks) → o novo
  membro **aparece na lista de membros** com a role do convite, e o convite some dos pendentes.
  Owner **adiciona um existente** por email → membership imediata. Owner **muda a role** de um
  membro e **remove** outro. Um **non-owner** (member, via troca de workspace ativo ou contexto
  semeado) abre `/workspace` e **não vê** os controles de gestão (CA-29) — só a leitura.

## 5. Requisitos não-funcionais

- **RNF-01 (isolamento estrutural)** — toda query da fase filtra por `workspace_id`; o gate
  Owner-only é escopado às rotas de gestão; o `RequireMember` (404) roda antes do gate de owner
  (403). Vazamento entre workspaces = bug crítico (CA-22).
- **RNF-02 (resolução de convite no login é leve e best-effort)** — a varredura de convites no
  login é **1 query indexada** (`WHERE lower(email)=$1 AND status='pending' AND (expires_at IS NULL
  OR expires_at>now())`) + N inserts (N = convites pendentes do email, tipicamente 0–1). Roda na
  transação do login (ou logo após, sob o mesmo lock); erro num convite **não** derruba o login.
- **RNF-03 (token opaco e seguro)** — o `token` do convite é gerado server-side com ≥128 bits de
  entropia (reusa `auth.NewSessionToken`/`crypto/rand`), `UNIQUE`. **Nunca** vai pro corpo de
  resposta nem pra UI (RNF-04). (Nesta fase o token não é o mecanismo de aceite — o aceite é por
  login do email; o token fica pronto pra um futuro "link de convite" sem nova migração.)
- **RNF-04 (segredos)** — `invitations.token` nunca é exposto a nenhum cliente; nenhuma listagem o
  inclui. Sem segredo novo de deploy (esta fase não usa VAPID/OAuth extra).
- **RNF-05 (não-regressão das fases 01/02/10)** — o contrato do `GET /workspaces/{id}`, do login, do
  `RequireMember`/`RequireWriteRole`, do `workspace.NewStore` (assinatura intocada) e da página de
  leitura não muda (CA-34). A extensão do caminho de login (campo `auth.Config.ResolveInvites` +
  chamada no `HandleCallback`) é aditiva e provada por mutação: **remover a chamada a
  `ResolveInvites` do `HandleCallback`** deve matar um teste do callback real (não basta um teste de
  store/service isolado — o login de produção é `HandleCallback` → `s.store.UpsertUserFromGoogle`,
  com `s.store` construído dentro de `auth.NewService`).
- **RNF-06 (deps)** — **zero** dependência Go ou JS nova esperada (reusa `chi`, `pgx`, `crypto/rand`
  já presentes; UI reusa TanStack Query/`lib/api.ts`). Se o plan precisar de uma, sanciona com
  razão.
- **RNF-07 (performance)** — listar convites/membros e as mutações respondem **<200ms** num
  workspace típico (poucas dezenas de membros), como as listagens da fase 02 (índice por
  `workspace_id`).

## 6. Pré-condições externas

- **Nenhuma dependência de terceiro.** Não exige credencial OAuth nova, env nova obrigatória, nem
  ação no Google Cloud. O `DEFAULT_INVITE_TTL` (TTL do convite pending) pode ser env opcional
  (default fixo no código — ver plan §3.3); não bloqueia o build.
- O fluxo A (convite por email) só **materializa de fato** quando o convidado faz login Google real
  em produção; nos testes, o googlestub/seed simula esse login (como nas fases 01/02). Registrado em
  `PENDENCIAS-DIOGO.md` apenas como nota operacional (convidar o Cadu de verdade), **não** como
  bloqueador.

## 7. Fora de escopo (fronteiras explícitas)

- **Rename/criação/exclusão de workspace, sub-workspaces, billing** — fora da M2 (M2 §3).
- **Link de convite clicável** ("aceite por URL com token") — o token fica **pronto** na tabela,
  mas o aceite desta fase é por **login do email** (decisão HITL). Um endpoint `POST /invitations/
  accept?token=` é evolução futura sem nova migração.
- **Reatribuição de tasks ao remover um membro** — ao remover um membro, as tasks com
  `assignee`/`requested_by` apontando pra ele **não** são limpas/reatribuídas nesta fase (a coluna
  é FK simples, sem cascade de assignee; a fase 18 é quem liga assignee). Tratar isso é candidato a
  CA da fase 18 (ou um carve-out futuro).
- **Notificar o convidado** (email/push de "você foi convidado") — fora; o aviso in-app/push é da
  fase 18 (cross-assign) e usa o canal `notice` per-usuário, que **só** alcança quem já tem conta.
  Um convidado sem conta não tem canal — ele descobre ao logar.
- **Cross-assign, comentários, lembretes, presence, mobile** — fases 18–23, não aqui.
- **Histórico/auditoria de convites numa tela** — `status`/`created_at`/`invited_by` na tabela
  bastam; tela dedicada é futuro.
- **Cota/limite de membros por workspace** — sem limite nesta fase.
