# tasks.md — Fase 01 · Auth Google

> Tarefas atômicas, em ordem de execução. **1 commit por tarefa** (mensagem PT-BR).
> TDD obrigatório: toda tarefa de código tem teste que **falha antes** (red) e passa depois
> (green) — ver `TDD.md §1` (anti-cheat: confirmar o red). Google **sempre stubbado**.
> Estas tarefas descrevem trabalho futuro do build autônomo — nada é executado agora.
> **Exceção registrada à convenção "identificadores em inglês":** nomes de função de teste em
> PT-BR (`TestMigracoesAuth`, `TestPrimeiroLoginCriaWorkspaceOwner`…) são permitidos — padrão
> já adotado pelas 4 specs (00–03); o resto do código segue inglês.

---

## T-01 — [OPERACIONAL · Diogo, manual · ADIADA] Criar credencial OAuth no Google Cloud Console

> **ADIADA por decisão do Diogo (10/06):** NÃO bloqueia o build — todos os testes usam Google
> stubbado (TDD.md §3) e o dev roda com placeholders no `.env`. Bloqueia apenas o login REAL.
> Passo a passo centralizado em `PENDENCIAS-DIOGO.md` (P-01), pra fazer depois da
> implementação. **O builder autônomo pula esta tarefa sem perguntar.**

- **Objetivo:** existir um OAuth Client válido pro Timetrack (necessário pro uso real, não pro build).
- **Entradas:** conta Google do Diogo; domínio `timmingtrack.life`.
- **Passos (Diogo):** criar projeto "Timetrack" no Google Cloud Console → tela de consentimento
  (External, escopos `openid email profile`; usuário de teste: o próprio Diogo) → credencial
  **OAuth Client ID (Web)** com redirects autorizados:
  `https://timmingtrack.life/auth/google/callback` **e**
  `http://localhost:8080/auth/google/callback` (dev).
- **Entregável:** `GOOGLE_CLIENT_ID` e `GOOGLE_CLIENT_SECRET` anotados em `SECRETS.local.md`
  e no `.env` local (ambos gitignored). Placeholders no `deploy/.env.example`.
- **Acceptance check:** variáveis presentes no `.env` local; nada commitado. (Sem teste
  automatizado — tarefa operacional; os testes usam stub, não a credencial real.)
- **⚠️ Nota (risco pra fase 06):** app com tela de consentimento em status **Testing** tem
  refresh tokens que **expiram em ~7 dias**. Não afeta esta fase (a sessão do Timetrack é
  própria), mas quebraria silenciosamente o sync do Calendar. **Antes da fase 06:** publicar
  o app (status *In production*) ou aceitar e documentar a expiração de 7 dias.

## T-02 — Migrações de auth (users, workspaces, members, sessions, oauth_tokens)

- **Objetivo:** schema da fase no ar via runner de migrações do boot.
- **Entradas:** `plan.md §2` (SQL canônico); runner da fase 00 (dona da `0001_init.sql`).
- **Entregável:** `backend/internal/db/migrations/0002_users.sql`, `0003_workspaces.sql`,
  `0004_sessions.sql`, `0005_oauth_tokens.sql` (numeração fixa após a `0001_init.sql` da
  fase 00; **sem** repetir `CREATE EXTENSION pgcrypto`).
- **Acceptance check (TDD):** teste `TestMigracoesAuth` red→green — sobe Postgres de teste,
  roda migrações, verifica: 5 tabelas existem; `INSERT workspace_members ... role='member'`
  funciona e `role='admin'` **falha** (CA-16); `users.google_id` e `users.email` são UNIQUE;
  `oauth_tokens` tem PK composta `(user_id, provider)`.

## T-03 — Pacote `auth/crypto`: token de sessão, hash e AES-256-GCM

- **Objetivo:** primitivas de segurança puras (sem HTTP, sem DB).
- **Entradas:** D-3 e D-5 do `plan.md`.
- **Entregável:** `backend/internal/auth/crypto.go` — `NewSessionToken() (token, hash)`,
  `HashToken(token)`, `Seal(key, plain) []byte`, `Open(key, blob) (plain, error)`.
- **Acceptance check (TDD):** `TestCryptoSessao` red→green — token tem 32 bytes de entropia
  (base64url, 2 chamadas ≠), hash = SHA-256 hex (CA-08 parcial); `TestCryptoAESGCM` red→green —
  round-trip `Seal/Open` devolve o original, ciphertext ≠ plaintext, blob adulterado → erro
  (CA-14 parcial).

## T-04 — `BuildAuthURL` + handler `GET /auth/google/login`

- **Objetivo:** iniciar o fluxo OAuth com state anti-CSRF e escopos extensíveis.
- **Entradas:** D-1, D-4, D-8; env `GOOGLE_CLIENT_ID`, `OAUTH_REDIRECT_URL`.
- **Entregável:** `auth/oauth.go` (`BuildAuthURL(state, extraScopes...)`) +
  `auth/handlers.go` (handler de login).
- **Acceptance check (TDD):** `TestLoginRedireciona` red→green — `httptest`: resposta 302 com
  `client_id`, `redirect_uri`, `scope=openid email profile`, `state` ≥ 128 bits,
  `access_type=offline`, `include_granted_scopes=true`, e cookie `tt_oauth_state` httpOnly
  Max-Age 600 com `Secure` quando `COOKIE_SECURE=true` (**CA-01**). `TestBuildAuthURLEscopoExtra` red→green — escopo extra aparece na
  URL sem mudar assinatura (**CA-07**).

## T-05 — Stub do Google (fixtures) + cliente OAuth com endpoint injetável

- **Objetivo:** testar callback sem internet/Google real (`TDD.md §3`).
- **Entradas:** formato das respostas do token endpoint e do userinfo.
- **Entregável:** `backend/testdata/google/token.json`, `token_no_refresh.json` (resposta sem
  `refresh_token`, p/ CA-14.1), `userinfo.json`, `token_error.json` + helper de teste que sobe
  `httptest.Server` servindo as fixtures e injeta o endpoint no config OAuth.
- **Acceptance check (TDD):** `TestStubGoogle` red→green — o exchange contra o stub devolve
  access+refresh token e o userinfo devolve `google_id/email/name/avatar` das fixtures.
  (Infra dos testes de T-06..T-08.)

## T-06 — Callback: validação de state e erro do Google

- **Objetivo:** caminhos de falha do callback **antes** do caminho feliz.
- **Entradas:** T-04, T-05.
- **Entregável:** começo do handler `GET /auth/google/callback` em `auth/handlers.go`.
- **Acceptance check (TDD):** `TestCallbackStateInvalido` red→green — state divergente/ausente →
  400 `{"error":{"code":"invalid_state"}}`, zero linhas em `users`/`sessions` (**CA-03**);
  `TestCallbackErroGoogle` red→green — `?error=access_denied` → 302 `/login?error=google`,
  zero efeitos no banco (**CA-06**).

## T-07 — Callback caminho feliz: upsert de user + workspace pessoal no 1º login

- **Objetivo:** identidade e tenant criados transacionalmente.
- **Entradas:** T-02, T-05, T-06; `workspace/store.go`.
- **Entregável:** fluxo completo do callback chamando `workspace.Store` (transação:
  user → workspace `"Workspace de <primeiro nome>"` → member `owner`).
- **Acceptance check (TDD):** `TestPrimeiroLoginCriaWorkspaceOwner` red→green — callback com
  google_id novo cria 1 user + 1 workspace + 1 membership `owner`, tudo ou nada; subcasos:
  `name` vazio → workspace `"Workspace pessoal"`, e mesmo `email` com `google_id` novo →
  re-vincula sem criar user (plan §2) (**CA-04**);
  `TestLoginRepetidoNaoDuplica` red→green — 2º callback do mesmo google_id: contagens de
  `users`/`workspaces` inalteradas, `name`/`avatar_url` atualizados (**CA-05**).

## T-08 — Sessão persistida + cookie httpOnly + tokens Google cifrados no banco

- **Objetivo:** fechar o callback: sessão válida no browser, tokens em repouso cifrados.
- **Entradas:** T-03, T-07; env `TT_TOKEN_KEY`, `COOKIE_SECURE`.
- **Entregável:** `auth/session.go` (criar sessão 30 dias, gravar hash) + escrita de
  `oauth_tokens` com `Seal` (upsert não-destrutivo, plan §2) + `Set-Cookie` no callback.
- **Acceptance check (TDD):** `TestCallbackSucesso` red→green — callback com `code`+`state`
  válidos contra o stub → `302` com `Location: /`, `Set-Cookie` de `tt_session` presente,
  exatamente 1 linha nova em `sessions`, cookie `tt_oauth_state` zerado (**CA-02**);
  `TestSessaoSoGuardaHash` red→green — após callback, valor do
  cookie ∉ `sessions.token_hash` e `HashToken(cookie)` ∈ tabela (**CA-08**);
  `TestCookieDeSessao` red→green — atributos HttpOnly, SameSite=Lax, Path=/, Max-Age=2592000,
  Secure quando `COOKIE_SECURE=true`, e `expires_at` ≈ now+30d (**CA-09**);
  `TestTokensGoogleCifrados` red→green — `access_token_enc` ≠ token das fixtures e `Open`
  devolve o original (**CA-14**);
  `TestLoginRepetidoPreservaRefreshToken` red→green — 2º callback com fixture de token **sem**
  `refresh_token` → `refresh_token_enc` gravado no 1º login permanece intacto (**CA-14.1**).

## T-09 — Middleware `RequireAuth` em `/api/v1/*`

- **Objetivo:** nenhuma rota de API roda sem sessão válida.
- **Entradas:** T-08; D-9 (desenho extensível p/ token `tt_` na fase 08).
- **Entregável:** `auth/middleware.go` montado no router sobre todo `/api/v1/*`.
- **Acceptance check (TDD):** `TestApiNegaSemSessao` red→green — sem cookie ou com token
  inexistente → 401 `{"error":{"code":"unauthorized"}}`, handler não executa (**CA-11**);
  `TestSessaoExpirada` red→green — sessão com `expires_at` no passado → 401, linha apagada de
  `sessions`, `Set-Cookie` com Max-Age=0 (**CA-12**).

## T-10 — `GET /api/v1/me`

- **Objetivo:** o front descobre quem está logado e seus workspaces.
- **Entradas:** T-09; query de memberships (T-07).
- **Entregável:** handler `me` em `auth/handlers.go` (árvore do `plan.md §6`) conforme
  contrato do `plan.md §3.3`.
- **Acceptance check (TDD):** `TestMeComSessaoValida` red→green — sessão válida → 200 com
  `user{id,email,name,avatar_url}` + `workspaces[{id,name,role:"owner"}]` (**CA-10**).

## T-11 — `POST /api/v1/auth/logout`

- **Objetivo:** revogação imediata da sessão.
- **Entradas:** T-09.
- **Entregável:** handler de logout conforme `plan.md §3.4`.
- **Acceptance check (TDD):** `TestLogout` red→green — logout → 204 + cookie zerado + linha
  removida de `sessions`; `GET /api/v1/me` seguinte com o mesmo cookie → 401 (**CA-13**).

## T-12 — `RequireMember` + `GET /api/v1/workspaces/{workspaceId}` (isolamento)

- **Objetivo:** isolamento multi-tenant estrutural (caminho crítico do `TDD.md §6`).
- **Entradas:** T-09, T-10; D-6, D-7.
- **Entregável:** `workspace/middleware.go` (membership → contexto) +
  `workspace/handlers.go` (rota da §3.5 do plan).
- **Acceptance check (TDD):** `TestIsolamentoEntreWorkspaces` red→green — cria user A e user B
  (cada um com seu workspace via fluxo de login stubbado); B requisita o workspace de A → 404
  `{"error":{"code":"not_found"}}` sem corpo de dados; A requisita o próprio → 200 com
  `role:"owner"` (**CA-15**).

## T-13 — Front: tela `/login` (design) + cliente de API

- **Objetivo:** porta de entrada visual, fiel aos tokens.
- **Entradas:** `tokens.css` (`--bg`, `.tt-btn--accent`, Geist self-hosted), `plan.md §5`.
- **Entregável:** `frontend/src/routes/Login.tsx` + `frontend/src/lib/api.ts`
  (`credentials:'include'`, envelope de erro — **criação do client**; fases 02/03 estendem).
- **Acceptance check (TDD):** Vitest `login.test.tsx` red→green — renderiza marca + botão
  "Entrar com Google" como link pra `/auth/google/login` com classe/estilo accent; com
  `?error=google` exibe mensagem de erro (**CA-17b**).

## T-14 — Front: guard de autenticação + E2E do fluxo completo

- **Objetivo:** não autenticado nunca vê o app; autenticado nunca vê `/login`.
- **Entradas:** T-10, T-13; Playwright configurado (fase 00).
- **Entregável:** `frontend/src/lib/auth.tsx` (AuthProvider/guard) + E2E
  `e2e/auth.spec.ts` rodando contra o backend local com Google stubbado (endpoint injetado
  via env de teste).
- **Acceptance check (TDD):** Playwright `auth.spec.ts` red→green — (a) visitar `/` (e uma
  rota qualquer ≠ `/login`) sem sessão redireciona pra `/login`, e `/login` NÃO entra em loop;
  (b) completar o login stubbado cai no cockpit (rota `/` do shell da fase 00) e
  `GET /api/v1/me` retorna 200; (c) "Sair" volta pra `/login` e `/` redireciona de novo
  (**CA-17a**, reusa CA-11/CA-13 ponta a ponta).

---

## Matriz CA → tarefa (todo critério coberto)

| CA | Critério (resumo) | Tarefa(s) |
|---|---|---|
| CA-01 | login redireciona c/ state + escopos | T-04 |
| CA-02 | callback troca code, cria sessão e 302 → `/` | T-08 (`TestCallbackSucesso`) |
| CA-03 | state inválido → 400, sem efeitos | T-06 |
| CA-04 | 1º login cria user+workspace+owner (transação) | T-07 |
| CA-05 | login repetido não duplica | T-07 |
| CA-06 | erro do Google → /login?error=google | T-06 |
| CA-07 | escopos incrementais sem mudar contrato | T-04 |
| CA-08 | banco só guarda hash da sessão | T-03, T-08 |
| CA-09 | cookie httpOnly/SameSite/Secure/30d | T-08 |
| CA-10 | /me com sessão válida → user+workspaces | T-10 |
| CA-11 | API nega sem sessão (401) | T-09 |
| CA-12 | sessão expirada → 401 + limpeza | T-09 |
| CA-13 | logout revoga sessão | T-11 |
| CA-14 | tokens Google cifrados AES-256-GCM | T-03, T-08 |
| CA-14.1 | login repetido sem refresh não apaga `refresh_token_enc` | T-08 (`TestLoginRepetidoPreservaRefreshToken`) |
| CA-15 | workspace alheio → 404 (isolamento) | T-12 |
| CA-16 | enum de roles no banco | T-02 |
| CA-17a | guard redireciona (exceto `/login`) + cockpit pós-login | T-14 (E2E) |
| CA-17b | tela `/login` no design (marca + botão accent) | T-13 (Vitest) |
| RF-9 | credencial OAuth no console (operacional) | T-01 |
