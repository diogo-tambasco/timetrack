# tasks.md — Fase 08 · Conector Claude

> Tarefas atômicas, TDD red→green→refactor, **commit por tarefa** (`TDD.md §1`).
> Backend = `go test` (dbtest + harness coretest) · Front = Vitest · E2E = Playwright.

---

## T-01 — Migração `0012_api_tokens`

- **Objetivo:** tabela do plan §2 (PK uuid, FK users CASCADE, name 1–100, token_hash
  UNIQUE, hint, created_at, last_used_at NULL, índice por user).
- **RED:** `migrations_tokens_test.go` (padrão `migrations_timer_test.go`): tabela existe
  com defaults; UNIQUE de token_hash; CHECK do name; DELETE do user cascateia.
- **Acceptance:** `go test ./internal/db` verde.

## T-02 — Primitivas e store do token (internal/auth)

- **Objetivo:** `apitoken.go` — `NewAPIToken() (token, hash, hint)` (D-01) + `TokenStore`:
  `Create(ctx, userID, name)`, `List(ctx, userID)`, `Revoke(ctx, userID, id)`,
  `Consume(ctx, hash) (userID, ok)` (UPDATE…RETURNING — D-04).
- **RED:** `apitoken_test.go`: CA-02 formato `tt_`+43 base64url e hint 7+…+4; CA-01 o
  hash gravado = SHA-256 do claro e o claro NUNCA está no banco; CA-06 `Consume` atualiza
  `last_used_at`; CA-07 `Revoke` → `Consume` seguinte falha; List só do usuário; Revoke de
  id alheio/inexistente → not_found.
- **Acceptance:** `go test ./internal/auth` verde.

## T-03 — Ramo Bearer no authenticate + método no contexto

- **Objetivo:** `middleware.go` ganha o ramo do D-02; `reqctx.WithAuthMethod`/
  `AuthMethodFrom` (D-03); `RequireAuth` injeta o método.
- **RED:** `middleware_token_test.go` table-driven contra um router real de teste:
  Bearer válido → 200 com user do token (CA-03); Bearer inválido → 401; Bearer revogado →
  401; Bearer malformado (`tt_` errado, vazio) → 401; **Bearer inválido + cookie VÁLIDO
  junto → 401** (CA-04, sem fallback); sem header + cookie → 200 (CA-05); sem nada → 401;
  esquema não-Bearer (`Basic x`) + cookie válido → 200 (registrado no plan §6).
- **Acceptance:** suíte auth verde; suítes existentes da 01 intactas.

## T-04 — Rotas de gestão `/api/v1/tokens`

- **Objetivo:** `tokens_routes.go` (option `WithTokens`, mount por usuário,
  `requireSessionAuth` — D-03/D-06) + `tokens_handlers.go` (§3.2) + wiring no `cmd/server`.
- **RED:** `tokens_handlers_test.go` (harness coretest): CA-08 (201 com claro+hint; 400
  name vazio/ausente/101 chars); CA-09 (lista própria sem claro/hash; user B não vê os do
  A); CA-10 (204 + some; 404 alheio/inexistente/malformado); CA-11 (POST/GET/DELETE com
  Bearer → 403; com cookie → funciona).
- **Acceptance:** `make test-back` verde.

## T-05 — Integração: fluxo do conector

- **Objetivo:** provar o caminho exato da skill, só com Bearer (CA-12/13).
- **RED:** `connector_flow_test.go`: cria token via rota (com cookie), depois SÓ Bearer:
  `GET /me` → `GET projects` → `POST tasks` → `POST /timer/start` (description = nome,
  project+task) → `GET /timer` rodando com `task_id` certo e `source: "timer"`; CA-13:
  start em ws alheio → 404; como viewer → 403 (token de um viewer semeado).
- **Acceptance:** teste verde (red confirmado removendo só o wiring novo, se aplicável —
  o fluxo usa rotas existentes; o red natural é o T-03/T-04 ainda não commitados? NÃO:
  T-05 roda DEPOIS deles — red estrutural impossível pro caminho REST já provado.
  **Exceção documentada** (precedente T-05 da 04): a validade vem de 2 mutações — desligar
  o ramo Bearer e desligar o `Consume` — ambas DEVEM derrubar o teste; registrar no
  cabeçalho.)

## T-06 — Front: seção "API / Conector" nos Ajustes

- **Objetivo:** `lib/api.ts` + `lib/queries/tokens.ts` (D-08) + `ApiTokens.tsx` na página
  Ajustes (lista, gerar com nome, claro 1 vez com copiar+aviso, revogar).
- **RED:** `ApiTokens.spec.tsx` (padrão Pomodoro.spec): CA-14 (lista renderiza name/hint/
  datas; seção presente nos Ajustes); CA-15 (após criar, o claro do 201 mockado aparece;
  após "fechar", só hint — e o claro NÃO está no DOM); CA-16 (revogar chama DELETE e a
  lista atualiza sem reload — invalidação mockada).
- **Acceptance:** Vitest verde (specs existentes de Settings intactos — RNF-03).

## T-07 — Skill `/timetrack-task` (repo + instalação global)

- **Objetivo:** `skill/timetrack-task/SKILL.md` (frontmatter name/description + roteiro
  EXATO do plan §3.3, com AskUserQuestion pra projeto/nome, tratamento de 401/404/403 e a
  regra do token local `~/.timetrack-token` + `TIMETRACK_URL` override) e cópia instalada
  em `~/.claude/skills/timetrack-task/SKILL.md`.
- **Acceptance (CA-17/18):** arquivo no repo e cópia global **byte-idênticos** (diff
  vazio); roteiro cita exatamente as rotas do CA-12 (review contra o
  `connector_flow_test.go`); nenhuma menção a commitar token. *(Sem teste automatizado —
  skill é markdown; a prova executável do caminho é o T-05. Exceção registrada.)*

## T-08 — E2E + gate + INDEX

- **Objetivo:** `frontend/e2e/ajustes-tokens.spec.ts` (arquivo paralelo — não muta timer):
  CA-19 completo (gerar → claro visível 1 vez → request context NOVO sem cookies com o
  Bearer → `/me` 200 → reload → só hint → revogar → some + mesmo Bearer agora 401).
  Depois `make check-full` (GATE) e `specs/INDEX.md` (08 construída).
- **Acceptance:** GATE VERDE; INDEX atualizado.
