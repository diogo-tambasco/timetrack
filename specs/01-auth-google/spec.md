# spec.md — Fase 01 · Auth Google

> **Feature:** Login via Google OAuth + sessão por cookie + criação de user/workspace no 1º login
> + middleware de autenticação + isolamento por workspace.
> Fonte de verdade desta fase. Deriva de `REQUISITOS.md` (§3, §6, §7 fase 01) e segue `SDD.md §4`.
> Status: rascunho pra validação do Diogo.

---

## 1. Contexto e mapa pro problema central

Sem auth não existe nada: o Timetrack é multi-tenant desde o dia 1 (`REQUISITOS.md §3`) e o
Google OAuth é a porta de entrada que **também destrava o Google Calendar** (problema central nº 2
do `REQUISITOS.md §2`) — cada usuário conecta o próprio Google. Esta fase entrega:

- Login com Google (authorization code flow) — escopos mínimos `openid email profile`.
  O escopo do Calendar fica pra **fase 06**, mas o fluxo já nasce preparado pra adicionar
  escopos depois sem retrabalho.
- No primeiro login: criação do `user` + **workspace pessoal automático** com role `owner`.
- Sessão por **cookie httpOnly** (tabela `sessions`, expiração, logout).
- **Middleware** que protege todo `/api/v1/*` e valida **pertencimento ao workspace** em
  toda rota com `workspace_id` (Constitution nº 4 do `SDD.md §8`).
- Tela de **login minimalista** seguindo o design (`tokens.css`).
- Tarefa **operacional** (manual, do Diogo) pra criar a credencial OAuth no Google Cloud Console.

## 2. User stories

- **US-1** — Como **Diogo (usuário)**, quero **entrar no Timetrack com 1 clique usando minha
  conta Google**, pra **não gerenciar senha e já deixar o caminho pronto pro sync de Calendar**.
- **US-2** — Como **usuário novo**, quero **que meu workspace pessoal seja criado sozinho no
  primeiro login, comigo como owner**, pra **começar a usar sem nenhum setup**.
- **US-3** — Como **usuário logado**, quero **continuar logado entre visitas (sessão persistente)
  e poder sair quando quiser**, pra **usar o app no dia a dia sem fricção e com controle**.
- **US-4** — Como **dono dos dados**, quero **que ninguém acesse a API sem sessão válida e que
  nenhum dado vaze entre workspaces**, pra **confiar no app com dados de clientes reais**
  (Acme, Lauda, freelas).
- **US-5** — Como **futuro colaborador (Carlos/Kadu, pós-MVP)**, quero **que a base já preveja
  papéis owner/member/viewer**, pra **o multiplayer entrar depois sem migração dolorosa**.
- **US-6** — Como **operador do projeto (Diogo)**, quero **um passo a passo claro pra criar a
  credencial OAuth no Google Cloud Console**, pra **destravar o build autônomo sem o Claude
  depender de credencial que só eu posso criar**.

## 3. Critérios de aceite (EARS — cada um vira exatamente 1 teste)

### Fluxo OAuth

- **CA-01** *(event-driven)* — QUANDO o navegador acessa `GET /auth/google/login`, o sistema
  DEVE responder `302` para o endpoint de autorização do Google contendo `client_id`,
  `redirect_uri` configurado, `response_type=code`, escopos `openid email profile` e um
  parâmetro `state` aleatório (≥ 128 bits de entropia) gravado em cookie temporário
  (httpOnly, `Secure` quando `COOKIE_SECURE=true`, validade 10 min).
- **CA-02** *(event-driven)* — QUANDO `GET /auth/google/callback` recebe `code` e `state`
  válidos, o sistema DEVE trocar o `code` por tokens no endpoint de token (stubbado em teste),
  obter `google_id`, `email`, `name` e `avatar_url` do perfil, criar sessão e responder `302`
  para `/`.
- **CA-03** *(unwanted)* — SE o `state` recebido no callback não corresponder ao do cookie (ou o
  cookie estiver ausente/expirado), o sistema NÃO DEVE criar sessão nem user e DEVE responder
  `400` com `{"error":{"code":"invalid_state","message":"..."}}`.
- **CA-04** *(event-driven)* — QUANDO um `google_id` desconhecido completa o login pela primeira
  vez, o sistema DEVE criar, **numa única transação**: o registro em `users`, um workspace
  pessoal nomeado `"Workspace de <primeiro nome>"` em `workspaces` (fallback
  `"Workspace pessoal"` se o `name` do Google vier vazio — D-10 do plan) e o vínculo em
  `workspace_members` com `role = 'owner'`. Caso de borda: mesmo `email` com `google_id`
  diferente (conta Google recriada) DEVE re-vincular o `google_id` ao user existente
  (plan §2), nunca falhar com 500.
- **CA-05** *(event-driven)* — QUANDO um `google_id` já existente completa o login, o sistema
  NÃO DEVE criar novo user nem novo workspace; DEVE atualizar `name`/`avatar_url` se mudaram e
  criar apenas uma nova sessão (contagem de linhas em `users`/`workspaces` permanece igual).
- **CA-06** *(unwanted)* — SE o Google retornar erro no callback (ex.: `?error=access_denied`),
  o sistema NÃO DEVE criar user nem sessão e DEVE responder `302` para `/login?error=google`.
- **CA-07** *(optional)* — ONDE escopos adicionais forem configurados (ex.: Calendar na fase 06),
  a função de montagem da URL de autorização DEVE incluí-los junto dos escopos base e enviar
  `include_granted_scopes=true`, **sem mudança de assinatura/contrato** (teste: chamar o builder
  com escopo extra e verificar a URL gerada).

### Sessão e cookie

- **CA-08** *(ubíquo)* — O sistema DEVE persistir em `sessions` apenas o **hash SHA-256** do
  token de sessão; o token em claro nunca DEVE aparecer no banco (teste: após login,
  nenhuma linha de `sessions.token_hash` é igual ao valor do cookie). *Nunca logar
  token/segredo é RNF (§5, `SDD.md §7.5`) — não é critério testável aqui.*
- **CA-09** *(ubíquo)* — O cookie de sessão DEVE ser `HttpOnly`, `SameSite=Lax`, `Path=/`,
  `Secure` quando `COOKIE_SECURE=true` (produção) e ter `Max-Age` de 30 dias, igual ao
  `sessions.expires_at` gravado.
- **CA-10** *(state-driven)* — ENQUANTO existir sessão válida (não expirada), `GET /api/v1/me`
  DEVE responder `200` com o user autenticado e a lista de seus workspaces com a respectiva
  `role`.
- **CA-11** *(unwanted)* — SE a requisição a qualquer rota `/api/v1/*` não trouxer cookie de
  sessão, ou trouxer token inexistente, o sistema DEVE responder `401`
  `{"error":{"code":"unauthorized","message":"..."}}` sem executar o handler.
- **CA-12** *(unwanted)* — SE a sessão estiver expirada (`expires_at < now()`), o middleware
  DEVE responder `401`, **apagar a linha** de `sessions` e enviar `Set-Cookie` zerando o cookie
  (`Max-Age=0`).
- **CA-13** *(event-driven)* — QUANDO o usuário chama `POST /api/v1/auth/logout`, o sistema DEVE
  apagar a sessão do banco, expirar o cookie e responder `204`; uma chamada seguinte a
  `GET /api/v1/me` com o mesmo cookie DEVE responder `401`.

### Segurança de tokens e isolamento

- **CA-14** *(ubíquo)* — O sistema DEVE cifrar os tokens OAuth do Google (access e refresh) com
  **AES-256-GCM** antes de gravar em `oauth_tokens`; o valor em claro nunca DEVE aparecer no
  banco (teste: o bytea gravado difere do token e o round-trip decifra de volta o original).
- **CA-14.1** *(unwanted)* — SE um login repetido vier **sem** `refresh_token` na resposta do
  Google (comportamento padrão: o Google só devolve refresh no 1º consentimento), o upsert em
  `oauth_tokens` NÃO DEVE sobrescrever nem apagar o `refresh_token_enc` existente (teste:
  2º login com fixture de token sem `refresh_token` → `refresh_token_enc` original permanece
  no banco).
- **CA-15** *(unwanted)* — SE um usuário autenticado acessar uma rota com `workspace_id` de um
  workspace do qual **não é membro**, o sistema DEVE responder `404`
  `{"error":{"code":"not_found","message":"..."}}` sem retornar nenhum dado (teste de
  isolamento: user B tenta ler o workspace de user A).
- **CA-16** *(ubíquo)* — O schema DEVE restringir `workspace_members.role` ao enum
  `('owner','member','viewer')`: inserir qualquer outro valor DEVE falhar no banco (base
  multi-user do dia 1, sem UI de convites nesta fase).

### Front (tela de login)

- **CA-17a** *(state-driven)* — ENQUANTO não houver sessão válida, qualquer rota do PWA
  **exceto `/login`** DEVE redirecionar para `/login`; após autenticar, o usuário DEVE cair
  no **cockpit** (definição: a rota `/` do shell criado na fase 00). Teste: E2E (T-14).
- **CA-17b** *(ubíquo)* — A tela `/login` DEVE exibir a marca e um único botão
  **"Entrar com Google"** (estilo `.tt-btn--accent`, fundo `--bg`, fonte Geist) que é um link
  pra `/auth/google/login`. Teste: Vitest (T-13).

## 4. Requisitos funcionais (resumo)

| # | Requisito | CAs |
|---|---|---|
| RF-1 | Login Google via authorization code flow (escopos `openid email profile`) | CA-01..03, CA-06 |
| RF-2 | Fluxo preparado pra escopos incrementais (Calendar na fase 06) | CA-07 |
| RF-3 | 1º login cria user + workspace pessoal + role owner (transacional) | CA-04, CA-05 |
| RF-4 | Sessão opaca em banco, cookie httpOnly, expiração 30 dias, logout | CA-08..13 |
| RF-5 | Middleware de auth em `/api/v1/*` + checagem de membership por workspace | CA-11, CA-12, CA-15 |
| RF-6 | Tokens OAuth cifrados em repouso (AES-256-GCM) + upsert não-destrutivo do refresh | CA-14, CA-14.1 |
| RF-7 | Roles owner/member/viewer no banco (sem UI de convites) | CA-16 |
| RF-8 | Tela de login minimalista + guard de rota no PWA | CA-17a, CA-17b |
| RF-9 | **Operacional:** credencial OAuth criada no Google Cloud Console pelo Diogo | T-01 (tasks.md) |

## 5. Requisitos não-funcionais

- **Leveza:** stdlib Go + `chi` + `golang.org/x/oauth2`; **nenhum framework de auth** (goth,
  authboss etc.). Sem JWT — sessão opaca em Postgres.
- **Segurança:** cookie httpOnly/SameSite=Lax/Secure; `state` anti-CSRF no OAuth; token de
  sessão com 256 bits de `crypto/rand`; só hash no banco; tokens Google cifrados (chave de
  32 bytes em env, fora do git); **nunca logar token/segredo** (`SDD.md §7.5`).
- **Multi-tenant:** toda query filtra por `workspace_id`; vazamento = bug crítico com teste
  obrigatório (CA-15).
- **Testabilidade:** endpoints do Google **sempre stubbados** em teste, com fixtures JSON
  (`TDD.md §3`) — nunca bater no Google real.
- **Segredos:** `GOOGLE_CLIENT_ID/SECRET` e chave de cifra só em `.env`/`SECRETS.local.md`
  (gitignored).

## 6. Fora de escopo (fica pras fases seguintes)

- **Escopo OAuth do Calendar + sync 2-way** → fase 06 (aqui só `openid email profile`).
- **UI de convites, membros, presença, papéis e visibilidade padrão** (página Workspace do
  design, `DESIGN-additions.md`) → pós-MVP / fase 02+ conforme necessidade. Nesta fase os roles
  existem **só no banco**.
- **Multiplayer real** (criar/pedir tasks entre colaboradores) → fora do MVP (`REQUISITOS.md §5`).
- **CRUD de clientes/projetos/tasks** → fase 02. **Timer/entries** → fase 03.
- **Sync WebSocket entre devices** → fase 04. **Modo Foco** → fase 05. **Relatórios** → fase 07.
- **Token de API `tt_` / conector Claude** → fase 08 (o desenho de auth desta fase não pode
  impedir: o middleware aceitará, no futuro, um segundo esquema de credencial via header).
- **Deploy real na VPS** → fase 09 (aqui tudo roda local; só a credencial OAuth já registra o
  redirect de produção).
- Refresh automático do access token do Google (só será exercitado na fase 06; aqui apenas
  armazenamos cifrado).
