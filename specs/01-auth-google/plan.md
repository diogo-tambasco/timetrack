# plan.md — Fase 01 · Auth Google

> Como implementar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7`
> (estrutura de pastas §7.2, persistência §7.4, auth §7.5). Nada aqui é código — é contrato.

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-1 | **OAuth com `golang.org/x/oauth2`** (lib oficial, ~zero deps) em vez de goth/authboss ou implementação manual do token exchange | Leveza é requisito (Constitution nº 6). A lib oficial cobre exatamente o authorization code flow + refresh, sem trazer framework. Endpoints **injetáveis** via `oauth2.Endpoint` → permite stub nos testes (`TDD.md §3`). |
| D-2 | **Sessão opaca em Postgres** (tabela `sessions`), não JWT | Revogação imediata no logout (CA-13), zero gestão de chave de assinatura, instância única de app → sem necessidade de stateless. Mais simples = mais leve. |
| D-3 | Token de sessão: **32 bytes `crypto/rand` em base64url** no cookie; banco guarda **SHA-256** | Vazamento do banco não vaza sessões (CA-08). Lookup por hash é O(1) com PK. |
| D-4 | **`state` anti-CSRF em cookie temporário** (httpOnly, 10 min), não em storage server-side | Evita tabela/TTL extra. O callback compara cookie × query param (CA-01, CA-03). |
| D-5 | **Tokens Google cifrados com AES-256-GCM** (stdlib `crypto/aes` + `crypto/cipher`), chave de 32 bytes em env `TT_TOKEN_KEY` (hex), nonce aleatório por registro prefixado no ciphertext | Exigência da fase ("cifrados em repouso", CA-14) sem dependência externa. A fase 06 reutiliza o mesmo pacote pra ler/refrescar. |
| D-6 | **Workspace no path**: rotas de domínio em `/api/v1/workspaces/{workspaceId}/...`, com middleware `RequireMember` que carrega a membership e injeta `workspace_id`+`role` no contexto | Torna o isolamento **estrutural**: handler nenhum roda sem membership validada (CA-15). Fases 02/03 só penduram sub-rotas. |
| D-7 | **`404` (não `403`) pra workspace alheio** | Não vazar a existência de workspaces de terceiros. |
| D-8 | **Escopos como parâmetro do builder da URL** (`BuildAuthURL(state string, extraScopes ...string)`) + `access_type=offline` + `include_granted_scopes=true` | Fase 06 adiciona `calendar` por **auth incremental**, sem mudar contrato (CA-07). `offline` **pede** o refresh token, mas o Google só o devolve no **1º consentimento** — logins repetidos vêm **sem** `refresh_token` na resposta. Por isso o upsert de `oauth_tokens` é não-destrutivo (COALESCE, CA-14.1). Se o refresh for perdido, a saída é forçar `prompt=consent` num novo login (só será necessário na fase 06). |
| D-9 | **Middleware `RequireAuth` desenhado pra 2 esquemas**: hoje só cookie; a interface deixa espaço pra um segundo autenticador (token `tt_` por header, fase 08) | A fase 08 não pode ser impedida pelo desenho de hoje (escopo entre fases). |
| D-10 | Nome do workspace pessoal: `"Workspace de <primeiro nome>"` (primeiro token do `name` do Google); **fallback `"Workspace pessoal"`** se o `name` vier vazio | Zero setup no 1º login (CA-04) sem nome quebrado (`"Workspace de "`); renomear vem com UI de workspace depois. |

## 2. Modelo de dados / migrações desta fase

Migrações em `backend/internal/db/migrations/` (formato `NNNN_nome.sql`, aplicadas no boot —
convenção canônica). A fase 00 é dona da **`0001_init.sql`** (habilita `pgcrypto` →
`gen_random_uuid()` já disponível); esta fase começa na **`0002`** e é **dona** de `users`,
`workspaces`, `workspace_members`, `sessions` e `oauth_tokens`. **Numeração fixa:**
`0002_users.sql` · `0003_workspaces.sql` · `0004_sessions.sql` · `0005_oauth_tokens.sql`.

> `oauth_tokens` é **extensão da fase 01 ao schema canônico compartilhado** (suporte ao
> requisito "tokens cifrados em repouso"). **Esta é a definição canônica** — a fase 06
> (consumidora) referencia o SQL abaixo e NÃO redefine a tabela.

```sql
-- 0002_users.sql  (pgcrypto já habilitado pela 0001_init.sql da fase 00 — não repetir)
CREATE TABLE users (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  google_id   text NOT NULL UNIQUE,
  email       text NOT NULL UNIQUE,
  name        text NOT NULL,
  avatar_url  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

```sql
-- 0003_workspaces.sql
CREATE TABLE workspaces (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE workspace_role AS ENUM ('owner','member','viewer');

CREATE TABLE workspace_members (
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role         workspace_role NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id)
);
```

```sql
-- 0004_sessions.sql
CREATE TABLE sessions (
  token_hash text PRIMARY KEY,                  -- sha256 hex do token do cookie
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
```

```sql
-- 0005_oauth_tokens.sql
CREATE TABLE oauth_tokens (
  user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider          text NOT NULL DEFAULT 'google',
  access_token_enc  bytea NOT NULL,             -- AES-256-GCM (nonce || ciphertext)
  refresh_token_enc bytea,
  token_expires_at  timestamptz,
  scopes            text[] NOT NULL,
  updated_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, provider)               -- permite outro provider no futuro
);
```

**1º login (CA-04) — uma transação:** `INSERT users` → `INSERT workspaces` →
`INSERT workspace_members (role='owner')` → upsert `oauth_tokens`. Login repetido (CA-05):
`UPDATE users SET name, avatar_url` + upsert `oauth_tokens` + `INSERT sessions`.

**Upsert não-destrutivo de `oauth_tokens` (CA-14.1):** `ON CONFLICT (user_id, provider) DO
UPDATE` com `refresh_token_enc = COALESCE(EXCLUDED.refresh_token_enc,
oauth_tokens.refresh_token_enc)` — login repetido sem `refresh_token` na resposta do Google
(o caso normal, ver D-8) **preserva** o refresh existente, que a fase 06 precisa.

**Conflito de email (CA-04, borda):** mesmo `email` com `google_id` diferente (conta Google
recriada) → re-vincular: `UPDATE users SET google_id = <novo>` no user existente, dentro da
mesma transação. Nunca deixar o `UNIQUE(email)` estourar em 500.

## 3. Contratos de API

Convenções: prefixo `/api/v1/`, erros sempre `{"error":{"code","message"}}`, auth por cookie
de sessão httpOnly. Rotas `/auth/*` ficam **fora** do prefixo `/api/v1` (são navegação de
browser, não API).

### 3.1 `GET /auth/google/login`

Inicia o fluxo. Sem payload.

- **302** → `https://accounts.google.com/o/oauth2/v2/auth?client_id=...&redirect_uri=<OAUTH_REDIRECT_URL>&response_type=code&scope=openid%20email%20profile&state=<rand>&access_type=offline&include_granted_scopes=true`
- `Set-Cookie: tt_oauth_state=<rand>; HttpOnly; SameSite=Lax; Max-Age=600; Path=/` (+ `Secure` se `COOKIE_SECURE=true` — mesmo critério do `tt_session`)

### 3.2 `GET /auth/google/callback?code=...&state=...`

- **302 → `/`** com `Set-Cookie: tt_session=<token>; HttpOnly; SameSite=Lax; Max-Age=2592000; Path=/` (+ `Secure` se `COOKIE_SECURE=true`) — sucesso (CA-02). State cookie é zerado.
- **400** — state inválido/ausente (CA-03):
  ```json
  {"error":{"code":"invalid_state","message":"state do OAuth não confere"}}
  ```
- **302 → `/login?error=google`** — Google retornou `error` (ex.: `access_denied`) ou a troca
  do code falhou (CA-06). Nenhum efeito no banco.

### 3.3 `GET /api/v1/me` *(autenticada)*

- **200**:
  ```json
  {
    "user": {
      "id": "5f1b1c0a-9f1e-4f9c-b2a1-7c2d6a0e4b11",
      "email": "voce@exemplo.com",
      "name": "Diogo Tambasco",
      "avatar_url": "https://lh3.googleusercontent.com/a/abc"
    },
    "workspaces": [
      {"id": "0c8e7a44-3a21-4d7e-9b6f-2e5d8c1a9f02", "name": "Workspace de Diogo", "role": "owner"}
    ]
  }
  ```
- **401** (sem/expirada — CA-11/CA-12):
  ```json
  {"error":{"code":"unauthorized","message":"sessão ausente ou expirada"}}
  ```

### 3.4 `POST /api/v1/auth/logout` *(autenticada)*

- **204**, com `Set-Cookie: tt_session=; Max-Age=0; ...` e sessão apagada do banco (CA-13).
- **401** se já não há sessão válida.

### 3.5 `GET /api/v1/workspaces/{workspaceId}` *(autenticada + membership)*

Rota mínima desta fase que materializa o isolamento (as fases 02/03 penduram sub-rotas aqui).

- **200** (membro):
  ```json
  {"workspace": {"id": "0c8e7a44-...", "name": "Workspace de Diogo", "role": "owner", "created_at": "2026-06-10T12:00:00Z"}}
  ```
- **404** (não-membro ou inexistente — CA-15):
  ```json
  {"error":{"code":"not_found","message":"workspace não encontrado"}}
  ```
- **401** sem sessão.

### 3.6 Variáveis de ambiente (em `deploy/.env.example`, valores reais fora do git)

```
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
OAUTH_REDIRECT_URL=http://localhost:8080/auth/google/callback   # prod: https://timmingtrack.life/auth/google/callback
TT_TOKEN_KEY=<64 hex chars = 32 bytes>                          # cifra AES-256-GCM
COOKIE_SECURE=false                                             # true em prod
```

## 4. Libs escolhidas

| Lado | Lib | Justificativa |
|---|---|---|
| Go | stdlib `net/http` + `chi` | Já decidido no `CLAUDE.md` (stack). |
| Go | `golang.org/x/oauth2` (+ endpoint Google) | Oficial, minúscula, testável por injeção de endpoint (D-1). |
| Go | `jackc/pgx/v5` | Driver Postgres padrão de fato, sem ORM (baseline §7.4 prevê "sqlc ou pgx direto" — começamos com pgx direto; sqlc só se as queries crescerem). |
| Go | stdlib `crypto/rand`, `crypto/sha256`, `crypto/aes`, `crypto/cipher` | Token de sessão, hash e AES-GCM sem dependência externa (D-3, D-5). |
| Go (test) | stdlib `testing` + `net/http/httptest` (+ `testify` se ajudar) | `TDD.md`. Google stubbado com `httptest.Server` + fixtures em `backend/testdata/google/*.json`. |
| Front | React + Vite + TS + Tailwind (já no scaffold da fase 00) | Stack travada. Sem lib de auth no front: o cookie httpOnly é invisível ao JS; o guard só consulta `GET /api/v1/me`. |
| Front (test) | Vitest (unit) + Playwright (E2E) | `TDD.md §2`. |

## 5. Componentes / telas do front

Referências visuais: `Design/Timmingtrack.life/timetrack/tokens.css` (fonte de verdade,
mapeado no Tailwind v4 via `@theme inline` — D-05 da fase 00) e `DESIGN-additions.md`. Dark-first,
accent default teal `#2dd4bf`, Geist + Geist Mono **self-hosted** (o `@import` do Google Fonts
do protótipo NÃO vai pra prod — convenção canônica).

| Peça | Onde | Descrição / tokens |
|---|---|---|
| `routes/Login.tsx` | `frontend/src/routes/` | Tela única centrada: fundo `--bg` (#0e0e10), card `.tt-card` discreto opcional, marca "Timetrack" (Geist, `--text-1`) + tagline curta (`--text-3`) + botão **"Entrar com Google"** no estilo `.tt-btn--accent` (`--accent` / `--accent-ink`) com o G logo. Único CTA. `?error=google` → linha de erro discreta (`--text-2`). É um `<a href="/auth/google/login">` — navegação plena, não fetch. |
| `lib/api.ts` | `frontend/src/lib/` | Cliente fetch com `credentials: 'include'`, base `/api/v1`, parse do envelope de erro `{error:{code,message}}`. `getMe()`, `logout()`. **Nasce nesta fase** (a fase 00 não tem rota de API); as fases 02/03 só o estendem. |
| `lib/auth.tsx` (guard) | `frontend/src/lib/` | `AuthProvider` consulta `GET /api/v1/me` no boot; `401` → redirect `/login` (CA-17a; a própria `/login` fica fora do guard, sem loop); `200` → guarda user+workspaces no store (zustand) e segue pro cockpit (rota `/` do shell da fase 00). |
| Item "Sair" | menu do avatar no rail (`.tt-sidenav`, `.tt-avatar` do tokens.css) | Chama `POST /api/v1/auth/logout` e manda pra `/login`. |

**Sem tela de Workspace/membros nesta fase** — `workspace.jsx` do design (papéis `.tt-role`,
`.tt-role--owner`, convites, presença) é **referência do modelo de dados** (roles
Owner/Membro/Viewer) e só vira UI quando o multiplayer entrar (pós-MVP).

## 6. Estrutura de código (dentro do baseline §7.2)

```
backend/
  cmd/server/main.go            # wiring: env, db, migrações no boot, rotas
  internal/
    auth/
      oauth.go                  # BuildAuthURL(state, extraScopes...), exchange, userinfo
      handlers.go               # /auth/google/login, /auth/google/callback, logout, GET /api/v1/me
      session.go                # criar/validar/apagar sessão; hash sha256
      crypto.go                 # token aleatório; AES-256-GCM seal/open
      middleware.go             # RequireAuth (cookie) — extensível p/ token tt_ (fase 08)
    workspace/
      store.go                  # queries users/workspaces/members (pgx)
      handlers.go               # GET /api/v1/workspaces/{id}
      middleware.go             # RequireMember (membership → contexto)
    db/migrations/0002..0005*.sql   # 0001_init.sql é da fase 00
  testdata/google/              # fixtures: token.json, userinfo.json, token_error.json
frontend/src/
  routes/Login.tsx
  lib/api.ts · lib/auth.tsx
```

## 7. Pontos de integração

- **Fase 00 (esqueleto):** usa o server, o runner de migrações no boot, `GET /healthz`
  (`200 {"status":"ok"}`) e o compose local. Esta fase só **adiciona** rotas e migrações.
- **Fase 02 (core):** pendura `/clients`, `/projects`, `/tasks` sob
  `/api/v1/workspaces/{workspaceId}/` reaproveitando `RequireAuth` + `RequireMember`; usa a
  `role` do contexto pra permissões.
- **Fase 03 (cronômetro):** idem; `user_id` do contexto identifica o dono do timer.
- **Fase 06 (Calendar):** chama `BuildAuthURL(state, "https://www.googleapis.com/auth/calendar")`
  (auth incremental, D-8) e lê/refresca tokens via `oauth_tokens` + `auth/crypto.go`.
- **Fase 08 (conector Claude):** registra um segundo autenticador no `RequireAuth`
  (header `Authorization: Bearer tt_...`) sem tocar nos handlers (D-9).
- **Fase 09 (deploy):** o redirect de produção `https://timmingtrack.life/auth/google/callback`
  já fica registrado na credencial (tarefa operacional T-01) — deploy não exige volta ao console.
