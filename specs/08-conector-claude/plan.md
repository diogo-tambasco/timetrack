# plan.md — Fase 08 · Conector Claude

> Como implementar a `spec.md` desta pasta. Herda o baseline (`SDD.md §7.5`: tokens `tt_`)
> e os contratos: `authenticate` extensível da 01 (D-9 de lá), primitivas `NewSessionToken`/
> `HashToken` (crypto.go), envelope de erro canônico, harness `coretest_test.go`,
> `POST /timer/start` com validação de membership/role no payload (T-11.1 da 03).

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Token = `tt_` + base64url(32 bytes crypto/rand); banco guarda SHA-256 hex** — mesmas primitivas das sessões (`HashToken`), função nova `NewAPIToken() (token, hash, hint)`. | CA-01/02 com zero primitiva nova de crypto; lookup O(1) pelo índice único do hash, padrão provado da 01. |
| D-02 | **Ramo Bearer DENTRO de `auth.authenticate`** (o comentário da 01 já reservou o lugar): header presente e prefixado `Bearer tt_` → lookup em `api_tokens` por hash → `user_id` + update `last_used_at`; header presente e inválido → erro (SEM fallback pro cookie — CA-04); sem header → cookie como sempre. | Um único ponto de autenticação pra TODAS as rotas (CA-03/05) — handlers intocados. Sem fallback porque credencial explícita ambígua mascara revogação (token revogado "funcionando" pelo cookie do browser seria bug de segurança silencioso). |
| D-03 | **Método de auth no contexto:** `reqctx.WithAuthMethod(ctx, "session"\|"token")` (pacote-folha, extensão aditiva). Rotas de gestão usam middleware local `requireSessionAuth` → `403 forbidden` quando método = token. | CA-11 sem tocar o RequireAuth global; o contexto já carrega user_id — método é o mesmo padrão. |
| D-04 | **`last_used_at` atualizado na PRÓPRIA query de autenticação** (`UPDATE … SET last_used_at = now() WHERE token_hash = $1 RETURNING user_id`). | CA-06/07 numa query só: atômico, sem corrida, e a revogação (DELETE da linha) é imediata por construção — `RETURNING` vazio = 401. |
| D-05 | **Revogar = DELETE da linha** (hard delete). | CA-07/10 triviais; histórico de tokens revogados não tem valor no modelo single-user (registrado; um `revoked_at` soft seria estado morto). |
| D-06 | **Rotas de gestão por usuário** em `/api/v1/tokens` (fora do subrouter de workspace, padrão `mountTimer`), montadas por option nova `api.WithTokens(store)`. | Token é credencial do usuário, não recurso de workspace (INDEX.md: rotas por usuário no `/api/v1`). |
| D-07 | **Skill fina, API faz o trabalho:** a SKILL.md instrui o Claude a usar `curl` com o token de `~/.timetrack-token` contra `https://timmingtrack.life`, no fluxo exato do CA-12; pergunta projeto/nome via AskUserQuestion quando faltam nos args. Instalação global = cópia do arquivo do repo (tarefa do build), com `TIMETRACK_URL` documentado como override (dev/local). | CA-17/18. A inteligência fica no backend testado (CA-12); a skill é só roteiro — zero lógica duplicada. |
| D-08 | **Front:** `lib/queries/tokens.ts` (list/create/revoke com invalidação) + componente `settings/ApiTokens.tsx` plugado na página Ajustes (padrão do `PomodoroGroup` da 03). O claro vive SÓ no estado local do componente após o 201 (nunca em query cache persistente). | CA-14/15/16 com o padrão provado das mutações da 02; o claro fora do cache elimina re-exibição acidental. |

## 2. Modelo de dados / migrações

### `0012_api_tokens.sql` (dona: fase 08 — INDEX.md)

```sql
CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash   text NOT NULL UNIQUE,
    hint         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);
CREATE INDEX api_tokens_user_idx ON api_tokens (user_id);
```

## 3. Contratos

### 3.1 Autenticação (toda rota atrás do RequireAuth)

```
Authorization: Bearer tt_<43 chars>
```
- válido → request autenticada como o dono (CA-03); `last_used_at = now()` (D-04).
- header presente + token inválido/revogado/malformado → `401 {"error":{"code":"unauthorized",...}}` (CA-04).
- sem header → cookie de sessão (fase 01, intacto — CA-05).

### 3.2 Gestão (sessão por cookie OBRIGATÓRIA — CA-11; token → 403)

```
POST   /api/v1/tokens          {"name": "MacBook do Diogo"}
  201  {"id","name","token","hint","created_at"}        ← token claro SÓ aqui
  400  validation_error (name ausente/vazio/>100)

GET    /api/v1/tokens
  200  {"tokens":[{"id","name","hint","created_at","last_used_at"}]}   ← sem claro/hash

DELETE /api/v1/tokens/{id}
  204  · 404 uniforme (alheio/inexistente/malformado)
```

### 3.3 Fluxo da skill (o que a SKILL.md roteiriza — provado pelo CA-12)

```
1. token ← ~/.timetrack-token   (ausente → instruir Ajustes + chmod 600, parar)
2. GET  /api/v1/me                          → workspaces[0] (single-user)
3. GET  /api/v1/workspaces/{ws}/projects    → ativos; AskUserQuestion se não veio nos args
4. nome da task ← args ou pergunta
5. POST /api/v1/workspaces/{ws}/tasks       {project_id, name}
6. POST /api/v1/timer/start                 {workspace_id, description: name,
                                             project_id, task_id}
7. confirmar: "task criada + timer rodando" · 401 em qualquer passo → token
   inválido/revogado → instruir gerar novo nos Ajustes
```

## 4. Libs

**Nenhuma nova** (RNF-01).

## 5. Onde vive o quê

```
backend/internal/db/migrations/0012_api_tokens.sql   (§2)
backend/internal/db/migrations_tokens_test.go        teste da migração
backend/internal/auth/
  apitoken.go        NewAPIToken (D-01) · TokenStore: Create/List/Revoke/consume por hash (D-04/05)
  apitoken_test.go   CA-01 (hash em repouso) · CA-02 (formato/hint) · CA-06/07 (last_used/revogação)
  middleware.go      ramo Bearer no authenticate (D-02) + método no contexto (D-03)
  middleware_token_test.go  CA-03/04/05 table-driven (válido/ inválido/ revogado/ malformado/
                            header ausente/ cookie intacto)
backend/internal/reqctx/reqctx.go   WithAuthMethod/AuthMethodFrom (extensão aditiva)
backend/internal/api/
  tokens_routes.go     option WithTokens + mountTokens + requireSessionAuth (D-03/D-06)
  tokens_handlers.go   POST/GET/DELETE (§3.2)
  tokens_handlers_test.go  CA-08/09/10/11
  connector_flow_test.go   CA-12/13 — fluxo inteiro só com Bearer no harness coretest
backend/cmd/server/    wiring WithTokens
frontend/src/
  lib/api.ts                 tipos + listTokens/createToken/revokeToken
  lib/queries/tokens.ts      hooks com invalidação (D-08)
  components/settings/ApiTokens.tsx       seção "API / Conector" (CA-14/15/16)
  components/settings/ApiTokens.spec.tsx  Vitest dos 3 CAs
  routes/cockpit/Settings.tsx             monta a seção (edição aditiva)
frontend/e2e/ajustes-tokens.spec.ts       CA-19 (arquivo paralelo — não muta timer)
skill/timetrack-task/SKILL.md             fonte versionada (D-07)
~/.claude/skills/timetrack-task/SKILL.md  instalação global (cópia — tarefa do build)
```

## 6. Pontos de integração e armadilhas conhecidas

- **Ordem dos ramos no `authenticate`:** header `Authorization` presente com prefixo
  `Bearer ` decide SOZINHO (CA-04) — atenção pra não cair no cookie quando o Bearer falha.
  Header com outro esquema (`Basic …`) → trata como "sem header" (cookie), registrado:
  só `Bearer` é da API do Timetrack.
- **`requireSessionAuth` DEPOIS do `RequireAuth`** na cadeia das rotas de gestão: o 401 de
  não-autenticado continua canônico; o 403 é só pra token AUTENTICADO (CA-11).
- **E2E com Bearer sem cookies:** usar `playwright.request.newContext({ extraHTTPHeaders })`
  NOVO (o `page.request` carrega os cookies da sessão — falso positivo). A revogação no
  CA-19 reusa o MESMO request context pra provar o 401.
- **WS (`/api/v1/ws`) por token:** funciona de graça (RequireAuth) — sem CA próprio;
  inofensivo e coerente (um device futuro autenticado por token receberia eventos).
- **Settings.tsx:** edição aditiva (montar `<ApiTokens />` abaixo do PomodoroGroup) — os
  specs existentes de Settings/Pomodoro não mudam (RNF-03).
- **Skill global:** instalar = `mkdir -p ~/.claude/skills/timetrack-task && cp` do arquivo
  do repo. A skill NÃO vai pro git de `~/.claude` (lá não é repo); o token NUNCA aparece
  na skill — só o caminho `~/.timetrack-token`. O `.gitignore` do projeto já cobre
  `SECRETS.local.md`/`.env*`; nada novo a ignorar (o token mora fora do repo).
- **`cmd/googlestub` intocado** — o fluxo por token não passa pelo OAuth.
- **Nunca logar Authorization:** o projeto não tem logger de headers hoje — manter assim
  (constituição §7; checagem de review, sem teste automatizável razoável).
