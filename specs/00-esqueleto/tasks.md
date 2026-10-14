# tasks.md — Fase 00 · Esqueleto

> Tarefas atômicas em ordem de execução. **1 commit por tarefa** (mensagem PT-BR).
> TDD obrigatório: acceptance check = teste **red→green** (confirmar o RED antes do GREEN —
> TDD.md §1). Esta lista descreve trabalho futuro; nada é executado na fase de planejamento.

---

## T-01 — Estrutura do repo, .gitignore e .env.example

- **Objetivo:** repositório git inicializado com a estrutura do plan.md §8, `.gitignore` cobrindo
  segredos e `.env.example` com placeholders.
- **Entradas:** plan.md §4/§8; SDD.md §7.2.
- **Entregável:** `.gitignore` (`.env`, `.env.*`, `!.env.example`, `SECRETS.local.md`,
  `node_modules/`, `dist/`, binários), `.env.example`, árvore `backend/ frontend/ deploy/`.
- **Acceptance check (CA-17):** script `scripts/check-segredos.sh` (usa `git check-ignore` pra
  `.env`, `.env.local`, `SECRETS.local.md` e greps de placeholder no `.env.example`) —
  red (antes do .gitignore) → green.

## T-02 — Backend: módulo Go + config por env

- **Objetivo:** `go.mod` (go 1.26, deps: chi, pgx, testify) + pacote de config que lê
  `DATABASE_URL`/`ADDR` e falha com mensagem clara se `DATABASE_URL` faltar.
- **Entradas:** plan.md §4/§5.
- **Entregável:** `backend/go.mod`, `backend/internal/config/config.go`.
- **Acceptance check (CA-06):** `TestConfig_SemDatabaseURL_Erra` (erro cita "DATABASE_URL") e
  `TestConfig_Completa_Carrega` — red→green.

## T-03 — Backend: router chi, /healthz e envelope de erro

- **Objetivo:** router com `GET /healthz` (200 `{"status":"ok"}`) e 404/405 JSON canônicos sob
  `/api/v1/`.
- **Entradas:** contratos plan.md §3.1/§3.2.
- **Entregável:** `backend/internal/api/router.go`, `handlers.go`, `errors.go` (helper do
  envelope `{"error":{"code","message"}}` — reusado por todas as fases).
- **Acceptance check (CA-01, CA-07):** via `httptest`: `TestHealthz_200JSON`,
  `TestAPIv1_RotaInexistente_404Envelope`, `TestHealthz_MetodoErrado_405Envelope` — red→green.

## T-04 — Backend: conexão pgx com retry/backoff

- **Objetivo:** `db.Connect(ctx, dsn)` com retry/backoff até 30s; esgotado → erro (boot aborta
  exit ≠ 0).
- **Entradas:** plan.md D-13 (Postgres de teste 5433); CA-04.
- **Entregável:** `backend/internal/db/db.go`.
- **Acceptance check (CA-04):** `TestConnect_BancoIndisponivel_FalhaAposTimeout` (DSN de porta
  morta, timeout curto injetável) e `TestConnect_BancoDePe_Conecta` (db-test) — red→green.

## T-05 — Backend: runner de migrações

- **Objetivo:** runner próprio: `embed.FS`, ordem `NNNN`, `schema_migrations`, transação por
  arquivo, rejeita nome fora do padrão.
- **Entradas:** plan.md §2 (contrato do runner); Postgres de teste.
- **Entregável:** `backend/internal/db/migrate.go`.
- **Acceptance check (CA-02, CA-03, CA-05):** contra o db-test:
  `TestMigrate_AplicaEmOrdem` (FS fake com 0002 antes de 0001 no nome → aplica 0001 primeiro),
  `TestMigrate_Idempotente_NaoReaplica` (roda 2x → 1 registro por versão, sem erro),
  `TestMigrate_SQLInvalido_AbortaSemRegistrar` (migração quebrada → erro + ausente de
  `schema_migrations`), `TestMigrate_NomeForaDoPadrao_Erra` — red→green.

## T-06 — Backend: 0001_init.sql + boot completo (migrate antes do listen)

- **Objetivo:** migração real `0001_init.sql` (pgcrypto) e `cmd/server/main.go` amarrando
  config → connect(retry) → migrate → router → listen, nesta ordem.
- **Entradas:** T-02..T-05; plan.md §2.2.
- **Entregável:** `backend/internal/db/migrations/0001_init.sql`, `backend/cmd/server/main.go`
  (com `Run(ctx, cfg) error` testável).
- **Acceptance check (CA-02, CA-04):** (a) teste de integração `TestBoot_MigraAntesDeServir` —
  sobe `Run` contra db-test zerado; quando `/healthz` responde 200, `schema_migrations` já contém
  `0001_init` e `pgcrypto` está instalada; (b) `TestRun_BancoIndisponivel_ErroComMotivo` — `Run`
  contra DSN morto (timeout curto injetado) retorna erro cujo texto cita o motivo, e o log
  capturado contém a linha de falha (o `main` converte esse erro em log + `os.Exit(1)`, fechando
  o exit ≠ 0 da CA-04) — red→green.

## T-07 — Backend: estáticos + fallback SPA

- **Objetivo:** servir `dist/` (embed, com override `STATIC_DIR`) com fallback SPA: rota
  não-arquivo fora de `/api` e `/healthz` → `index.html`.
- **Entradas:** plan.md D-04/§3.3.
- **Entregável:** `backend/internal/api/static.go`.
- **Acceptance check (CA-19):** `TestSPA_RotaDesconhecida_ServeIndex`,
  `TestSPA_Asset_ServeArquivo`, `TestSPA_NaoEngoleAPIv1` (404 JSON continua valendo) — red→green.

## T-08 — Dockerfile + compose dev (app + db + db-test)

- **Objetivo:** `Dockerfile` multi-stage (node → go → runtime) e `docker-compose.dev.yml`
  (db com healthcheck, app publicando 8080, db-test em profile `test`/tmpfs) + alvos `make up/down`.
- **Entradas:** plan.md §7.1.
- **Entregável:** `Dockerfile`, `docker-compose.dev.yml`, Makefile parcial.
- **Acceptance check (CA-15):** script `scripts/smoke-compose.sh`: `make up` → polla
  `http://localhost:8080/healthz` por até 60s esperando 200 → `make down`. Red (antes do
  compose/Dockerfile) → green.

## T-09 — Frontend: scaffold Vite+React+TS+Tailwind + tokens integrados

- **Objetivo:** projeto `frontend/` com Tailwind v4, `tokens.css` copiado do design (sem o
  `@import` do Google Fonts), `@theme inline` mapeando os tokens, classes `.tt-*` disponíveis.
- **Entradas:** `Design/Timmingtrack.life/timetrack/tokens.css`; plan.md §6.1.
- **Entregável:** `frontend/` buildável, `src/styles/tokens.css` + `index.css`.
- **Acceptance check (CA-11.1):** (a) `npm run build` exit 0 E o CSS gerado em `dist/assets/`
  contém o mapeamento dos tokens (grep por `--color-bg:var(--bg)` e
  `--color-accent:var(--accent)` no artefato — inspeção do build real, **não** jsdom, que não
  computa CSS do Tailwind); (b) script `scripts/check-cores.sh`: grep em `frontend/src/`
  (exceto `styles/tokens.css`) por hex literal `#[0-9a-fA-F]{3,8}` retorna vazio (allowlist
  documentada no próprio script) — red→green.

## T-10 — Frontend: fontes Geist self-hosted

- **Objetivo:** woff2 de Geist/Geist Mono em `src/assets/fonts/` + `@font-face`; nenhuma
  referência a Google Fonts no bundle.
- **Entradas:** plan.md D-08.
- **Entregável:** fontes + CSS `@font-face`.
- **Acceptance check (CA-13):** teste `scripts/check-fonts.test` (ou Vitest sobre o `dist/`):
  build → grep recursivo por `fonts.googleapis.com|fonts.gstatic.com` retorna vazio E existe
  `*.woff2` no `dist/assets/` — red (com o @import do protótipo) → green.

## T-11 — Frontend: skin store (dark default, light persistente)

- **Objetivo:** `useSkinStore` (Zustand + persist em `tt-skin`) aplicando `data-theme` no
  `<html>`; dark é o default na primeira visita.
- **Entradas:** engine `TTskin` de `app.jsx` (referência); plan.md §6.2.
- **Entregável:** `src/lib/skin.ts` + hook de aplicação no root.
- **Acceptance check (CA-11, CA-12):** Vitest: `skin_padraoDark` (sem localStorage →
  `data-theme="dark"`), `skin_lightPersiste` (set light → localStorage atualizado; novo mount lê
  light) — red→green.

## T-12 — Frontend: AppShell + SideNav (rail hover-expand) + páginas stub

- **Objetivo:** shell com rail canônico (64px → 232px overlay no hover, itens Hoje/Calendário/
  Projetos/Clientes/Relatórios/Workspace/Foco/Ajustes) + rotas stub (react-router). **Sem o
  toggle de tema em Ajustes** — ele nasce na T-15 (red legítimo do E2E de tema).
- **Entradas:** `SideNav` de `parts.jsx`, `.tt-sidenav` de tokens.css, DESIGN-additions.md.
- **Entregável:** `src/routes/AppShell.tsx`, `src/components/SideNav.tsx`, páginas stub.
- **Acceptance check (CA-09, CA-10):** Playwright `shell.spec.ts`: rail visível com 8 itens;
  largura do aside = 64px; hover → inner com bounding box 232px E o conteúdo principal não muda
  de `x` (sem reflow); mouse sai → volta a 64px — red→green.

## T-13 — Frontend: TimerBar placeholder

- **Objetivo:** barra de timer global no topo do shell: input desabilitado, `0:00:00` mono,
  botão play desabilitado. Sem lógica (fase 03).
- **Entradas:** `TimerBar` de `parts.jsx`; plan.md §6.2.
- **Entregável:** `src/components/timer/TimerBar.tsx` montado no `AppShell` (mesmo arquivo que
  a fase 03 evolui).
- **Acceptance check (CA-09.1):** Playwright `timerbar.spec.ts`: barra visível em todas as rotas,
  texto `0:00:00`, controles com `disabled` — red→green.

## T-14 — Frontend: PWA (manifest + service worker precache)

- **Objetivo:** `vite-plugin-pwa` configurado (manifest do plan.md §6.3, ícones 192/512,
  precache do app shell).
- **Entradas:** plan.md §6.3/D-06.
- **Entregável:** config no `vite.config.ts` + ícones em `public/`.
- **Acceptance check (CA-08, CA-14):** (a) teste de build: `dist/manifest.webmanifest` existe e
  contém `"name":"Timetrack"`, `"display":"standalone"`, `"theme_color":"#0e0e10"`, ícones 192
  e 512; (b) Playwright `offline.spec.ts`: visita online → `context.setOffline(true)` → reload →
  rail e TimerBar renderizam. **Plano B documentado** (o `setOffline` do Playwright nem sempre
  alcança requests originadas do service worker): derrubar o server de verdade (ou `page.route`
  com abort em tudo) e recarregar, assertando o shell — nunca dar o critério por verde sem provar
  o offline real — red→green.

## T-15 — Toggle de tema em Ajustes + E2E

- **Objetivo:** criar o toggle dark/light na página Ajustes (`Switch` de parts.jsx) ligado ao
  `useSkinStore` e fechar o fluxo real do usuário: Ajustes → alternar pra light → persiste após
  reload. O toggle NÃO existe até aqui (T-12 entrega Ajustes sem ele) — o RED do E2E é legítimo.
- **Entradas:** T-11, T-12.
- **Entregável:** toggle em `src/routes/cockpit/Settings.tsx` (página Ajustes) ligado ao
  `useSkinStore`.
- **Acceptance check (CA-11, CA-12 — E2E):** Playwright `tema.spec.ts`: primeira visita tem
  `html[data-theme="dark"]` e background `#0e0e10`; clica no toggle → `data-theme="light"`;
  reload → continua light — red (página sem toggle) → green.

## T-16 — Artefatos de deploy (compose prod + snippet Caddy)

- **Objetivo:** `deploy/docker-compose.yml` (sem ports 80/443, sem Caddy próprio, rede externa,
  restart policy), `deploy/Caddyfile.snippet` (só vhost `timmingtrack.life`), `deploy/.env.example`.
  **Não aplicar na VPS** (fase 09).
- **Entradas:** plan.md §7.2/§7.3; SDD.md §7.1.
- **Entregável:** os 3 arquivos em `deploy/`.
- **Acceptance check (CA-16):** teste de inspeção `deploy_test.go` (parse do YAML): nenhum
  serviço publica `80`/`443`, nenhum serviço de imagem caddy, rede marcada `external: true`,
  snippet contém `timmingtrack.life` e `reverse_proxy timetrack-app:8080` e nada além do vhost —
  red→green.

## T-17 — Makefile completo + `make check-full` (gate da fase)

- **Objetivo:** alvos `dev`, `up`, `down`, `test-back` (**sobe o `timetrack-db-test`** via
  `docker compose --profile test up -d --wait timetrack-db-test`, exporta `TEST_DATABASE_URL` e
  só então roda `go test ./...` — sem skip silencioso), `test-front`, `test-e2e`, `build`,
  `check` (test-back + test-front + test-e2e + build front + build back) e `check-full`
  (`check` + `scripts/smoke-compose.sh` — **gate oficial da fase**).
- **Entradas:** todas as anteriores; plan.md §7.4.
- **Entregável:** `Makefile` final.
- **Acceptance check (CA-18):** `make check-full` red (alvo inexistente) → green (roda tudo —
  incluindo Playwright e o smoke do compose — e sai 0). Suíte inteira verde = fase 00 concluída
  (TDD.md §6); chamar o Diogo pra revisão de fim de fase.

---

## Matriz de rastreabilidade CA → Tarefa

| CA | Critério (resumo) | Tarefa(s) |
|---|---|---|
| CA-01 | /healthz 200 JSON | T-03 |
| CA-02 | migra no boot antes do listener | T-05, T-06 |
| CA-03 | idempotência (schema_migrations) | T-05 |
| CA-04 | Postgres fora → retry 30s → exit ≠ 0 + log do motivo | T-04, T-06 |
| CA-05 | migração falha → aborta sem registrar | T-05 |
| CA-06 | DATABASE_URL ausente → erro claro | T-02 |
| CA-07 | 404/405 com envelope de erro | T-03 |
| CA-08 | build + manifest PWA válido | T-14 |
| CA-09 | shell: rail com 8 itens | T-12 |
| CA-09.1 | TimerBar placeholder em todas as rotas | T-13 |
| CA-10 | hover-expand 64→232 overlay sem reflow | T-12 |
| CA-11 | dark default (#0e0e10 / accent teal) | T-11 (unit), T-15 (E2E) |
| CA-11.1 | tokens = única fonte de cor (lint hex + mapeamento no build) | T-09 |
| CA-12 | light opcional persistente | T-11, T-15 |
| CA-13 | fontes self-hosted, sem Google Fonts | T-10 |
| CA-14 | app shell offline (SW precache) | T-14 |
| CA-15 | compose local sobe app+db, healthz ok | T-08 |
| CA-16 | deploy/ isolado (sem 80/443, sem Caddy próprio) | T-16 |
| CA-17 | segredos fora do git | T-01 |
| CA-18 | make check / check-full verdes (gate da fase) | T-17 |
| CA-19 | fallback SPA → index.html (rota sem extensão fora de /api e /healthz) | T-07 |

Cobertura: 21/21 critérios com ≥1 tarefa.
