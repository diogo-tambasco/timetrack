# plan.md — Fase 00 · Esqueleto

> Deriva da `spec.md` desta pasta. Herda o baseline do `SDD.md §7` (topologia, estrutura de
> pastas, persistência). Toda decisão aqui tem razão registrada (Constitution: leveza, isolamento,
> multi-tenant, spec é a verdade).

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Go stdlib `net/http` + `chi`** como único "framework" web | Decidido no CLAUDE.md. `chi` é router puro (zero deps transitivas pesadas), 100% compatível com `http.Handler` — leveza (RNF-01) |
| D-02 | **`pgx/v5`** (pool `pgxpool`) como driver Postgres, queries diretas, **sem ORM** | Driver padrão de mercado pra Go+Postgres, performático. ORM seria dependência pesada sem ganho num domínio pequeno (Constitution §8.6). `sqlc` pode entrar na fase 02 se o volume de queries justificar — decisão adiada de propósito |
| D-03 | **Runner de migração próprio** (≈100 linhas): `embed.FS` + ordenação por prefixo `NNNN` + tabela `schema_migrations` + transação por arquivo | Requisito é simples (só "up", sequencial, no boot). `golang-migrate`/`goose` trazem CLI, drivers múltiplos e features que não usamos. Código próprio é menor que a configuração da lib e 100% testável (CA-02/03/05) |
| D-04 | **Binário Go serve o frontend** (`dist/` via `http.FileServer` + fallback SPA) em produção; **Vite dev server com proxy** em dev | 1 container só de app na VPS (leveza, RNF-02). Fallback SPA: requisição sem extensão e fora de `/api`/`/healthz` → `index.html`. Em dev, hot-reload do Vite com `server.proxy` → `:8080` |
| D-05 | **Tailwind v4** com `@theme inline` mapeando as custom properties de `tokens.css` | `tokens.css` é a fonte de verdade (header do próprio arquivo manda mapear no Tailwind). v4 referencia `var(--*)` direto no tema, sem duplicar valores em JS — tema/densidade trocam em runtime só com `data-theme`/`data-density` |
| D-06 | **`vite-plugin-pwa`** (Workbox) pra manifest + service worker precache | Gerar SW na mão é fonte de bug; o plugin é dev-dependency (não pesa no runtime) e cobre CA-08/CA-14 com config declarativa |
| D-07 | **Zustand** pra estado de skin (tema) — único estado global da fase | Já previsto no baseline §7.2. O "skin engine" do protótipo (`app.jsx`, `window.TTskin`) vira um store Zustand tipado com persistência em localStorage (CA-11/12). 1.1kb, dentro da leveza |
| D-08 | **Fontes Geist + Geist Mono self-hosted** (woff2 em `frontend/src/assets/fonts/`, `@font-face` próprio) | CA-13. O `@import` do Google Fonts que existe no `tokens.css` do protótipo **não vai pro app** — licença das Geist (SIL OFL) permite self-host |
| D-09 | **Migração `0001_init.sql` só habilita `pgcrypto`** | Prova o pipeline de migração de ponta a ponta sem invadir o schema canônico (users/workspaces = fase 01). `gen_random_uuid()` fica disponível pras fases seguintes |
| D-10 | Porta interna do app: **8080**; compose dev publica `8080:8080`; compose prod **não publica porta nenhuma** (só rede interna do Caddy) | Isolamento da VPS (CA-16): quem expõe é o Caddy existente via vhost |
| D-11 | **npm** como gerenciador de pacotes JS | Zero tooling extra; lockfile padrão. (Trocar pra pnpm é decisão do Diogo — ver dúvidas) |
| D-12 | Postgres **17-alpine** nos dois composes | Versão estável atual, imagem pequena |
| D-13 | Testes: `go test` (+ `testify` p/ asserts), Postgres efêmero de teste via compose (`timetrack-db-test`, porta 5433, `tmpfs`); front: Vitest + Playwright | TDD.md §2/§3 — nunca tocar banco de produção; container de teste com tmpfs é rápido e descartável |

## 2. Modelo de dados / migrações desta fase

A fase 00 é dona de **infraestrutura** de migração, não de schema de domínio (schema canônico:
`users`/`workspaces`/`workspace_members`/`sessions` → fase 01; `clients`/`projects`/`tasks` →
fase 02; `time_entries`/`pomodoro_settings` → fase 03).

### 2.1 Tabela de controle (criada pelo próprio runner, fora dos arquivos NNNN)

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    text PRIMARY KEY,        -- ex.: '0001_init'
  applied_at timestamptz NOT NULL DEFAULT now()
);
```

### 2.2 `backend/internal/db/migrations/0001_init.sql`

```sql
-- 0001_init: extensões base. Schema de domínio começa na fase 01.
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- gen_random_uuid() pras PKs uuid das fases 01+
```

### 2.3 Comportamento do runner (contrato — CA-02/03/05)

1. No boot, antes do listener: conecta (retry/backoff até 30s — CA-04), garante
   `schema_migrations`, lista os `.sql` embarcados, ordena por `NNNN`.
2. Pra cada versão ausente de `schema_migrations`: `BEGIN` → executa o SQL → `INSERT INTO
   schema_migrations` → `COMMIT`. Erro em qualquer ponto → `ROLLBACK` + boot aborta (exit ≠ 0).
3. Nome de arquivo fora do padrão `NNNN_nome.sql` → erro de boot (proteção contra typo).

## 3. Contratos de API

Base canônica de **todas** as fases (definida aqui, fase 00):

- Prefixo REST: `/api/v1/`. Healthcheck fora do prefixo: `/healthz`.
- Erros JSON sempre no envelope `{"error":{"code":"<slug>","message":"<humano>"}}`.
- Auth por cookie de sessão httpOnly (implementada na fase 01 — fase 00 não tem rota autenticada).

### 3.1 `GET /healthz` (CA-01)

Sem auth. Resposta `200`:

```json
{"status":"ok"}
```

`Content-Type: application/json`. Sem corpo de request. (Liveness simples; não verifica o banco —
se o processo está de pé, as migrações já rodaram, CA-02.)

### 3.2 Envelope de erro — 404/405 sob `/api/v1/` (CA-07)

`GET /api/v1/nao-existe` → `404`:

```json
{"error":{"code":"not_found","message":"rota não encontrada"}}
```

`DELETE /healthz` (método errado em rota existente — exemplo canônico da CA-07b) → `405`:

```json
{"error":{"code":"method_not_allowed","message":"método não permitido"}}
```

> Nota: a fase 00 não tem nenhuma rota sob `/api/v1/`, então o 405 sob o prefixo é impossível de
> exercitar aqui (tudo cai no 404). O 405 é exercitado em `/healthz`; quando a fase 01 criar as
> primeiras rotas sob `/api/v1/`, o mesmo envelope vale lá (CA-07).

### 3.3 Estáticos / SPA (D-04)

- `GET /` e `GET /<rota-spa>` → `index.html` do `dist/` embarcado/montado.
- `GET /assets/*`, `/manifest.webmanifest`, `/sw.js` → arquivo estático com cache headers padrão.
- Nenhuma outra rota de API nesta fase. WebSocket (`/ws`) só na fase 04; token `tt_` só na 08 —
  nada aqui conflita (prefixos livres).

## 4. Configuração (env)

| Variável | Obrigatória | Default | Uso |
|---|---|---|---|
| `DATABASE_URL` | sim (CA-06) | — | DSN Postgres (`postgres://user:pass@host:5432/timetrack`) |
| `ADDR` | não | `:8080` | endereço do listener HTTP |
| `STATIC_DIR` | não | embed | override do diretório de estáticos em dev |
| `TEST_DATABASE_URL` | só nos testes Go | `postgres://postgres:postgres@localhost:5433/timetrack_test` | DSN do Postgres efêmero (`timetrack-db-test`, D-13); exportada pelo `make test-back` |

`.env.example` (raiz, pro compose dev) e `deploy/.env.example` (prod) documentam tudo com
placeholders (CA-17).

## 5. Libs escolhidas (resumo)

**Go** (`go 1.26`): `github.com/go-chi/chi/v5` (router) · `github.com/jackc/pgx/v5` (driver) ·
`github.com/stretchr/testify` (test only). **Mais nada.**

**JS**: `react`, `react-dom`, `react-router-dom` (rotas do shell), `zustand` (skin store) ·
dev: `vite`, `typescript`, `tailwindcss` (v4), `@vitejs/plugin-react`, `vite-plugin-pwa`,
`vitest`, `@testing-library/react`, `playwright`.

## 6. Frontend — componentes e telas

Referências de design: `Design/Timmingtrack.life/timetrack/tokens.css` (tokens canônicos),
`parts.jsx` (primitivas `SideNav`, `TimerBar`, ícones), `app.jsx` (skin engine `TTskin`),
`DESIGN-additions.md` (rail hover-expand canônico).

### 6.1 Integração de tokens (CA-11, D-05)

- `frontend/src/styles/tokens.css` = cópia fiel do `tokens.css` do design **menos** o `@import`
  do Google Fonts (substituído por `@font-face` self-hosted — CA-13/D-08).
- `frontend/src/styles/index.css`: `@import "tailwindcss";` + `@theme inline` mapeando
  `--color-bg: var(--bg)`, `--color-surface-1: var(--surface-1)`, `--color-accent: var(--accent)`,
  `--font-ui`, `--font-mono`, raios (`--r-*`) etc. → classes `bg-bg`, `bg-surface-1`,
  `text-accent`, `font-mono` saem dos tokens.
- Classes componentizadas do protótipo (`.tt-sidenav`, `.tt-nav`, `.tt-btn`, `.tt-chip`,
  `.tt-card`...) entram como CSS plano junto dos tokens — já prontas, não reescrever em Tailwind.

### 6.2 Componentes desta fase

| Componente | Arquivo | Referência no design | Conteúdo na fase 00 |
|---|---|---|---|
| `AppShell` | `src/routes/AppShell.tsx` | layout dos cockpits em `app.jsx` | grid: `SideNav` à esquerda (64px fixos) + coluna com `TimerBar` no topo + `<Outlet/>` |
| `SideNav` | `src/components/SideNav.tsx` | `SideNav` de `parts.jsx` + `.tt-sidenav`/`.tt-sidenav-inner` | rail 64px, hover → inner 232px overlay (CSS já dado em tokens.css), itens Hoje/Calendário/Projetos/Clientes/Relatórios/Workspace/Foco + Ajustes embaixo, brand Timetrack. Links `react-router` com `aria-current` (CA-09/10) |
| `TimerBar` | `src/components/timer/TimerBar.tsx` (mesmo arquivo que a fase 03 evolui — sem duplicar) | `TimerBar` de `parts.jsx` | **placeholder**: input de descrição desabilitado, `0:00:00` em `.mono`, botão play `.tt-iconbtn` desabilitado. Zero lógica de timer (fase 03) |
| `useSkinStore` | `src/lib/skin.ts` | engine `TTskin` de `app.jsx` | Zustand + `persist` (localStorage `tt-skin`): `{theme: 'dark'\|'light'\|'system'}`; efeito aplica `data-theme` no `<html>` (CA-11/12). Accent/densidade ficam nos tokens com default — UI de troca é fase 09 |
| Páginas stub | `src/routes/cockpit/{Today,Calendar,Projects,Clients,Reports,Workspace,Focus,Settings}.tsx` (arquivos em inglês — convenção de identificadores; URLs/labels em PT-BR: `/hoje`, "Ajustes"…; **fases 02+ substituem os stubs nesses mesmos arquivos**) | — | título `.eyebrow` + card vazio `.tt-card`; `Settings` (página Ajustes) ganha o toggle dark/light (`Switch` de parts.jsx) pra exercitar CA-12 — **o toggle é criado só na T-15** (red legítimo do E2E de tema) |

### 6.3 PWA (CA-08, CA-14)

`vite-plugin-pwa`: `registerType: 'autoUpdate'`, manifest `{ name: "Timetrack",
short_name: "Timetrack", display: "standalone", background_color: "#0e0e10",
theme_color: "#0e0e10", icons: [192, 512] }` (ícones gerados do glifo de relógio do brand —
placeholder até ter logo). Workbox: precache de `index.html`, JS/CSS, fontes woff2.

## 7. Infra — composes, Caddy, Makefile

### 7.1 `docker-compose.dev.yml` (raiz — CA-15)

Serviços: `timetrack-db` (postgres:17-alpine, volume nomeado, healthcheck `pg_isready`),
`timetrack-app` (build do `Dockerfile` multi-stage: stage 1 node→`dist/`, stage 2 golang→binário,
stage final distroless/alpine; `depends_on: condition: service_healthy`; publica `8080:8080`),
`timetrack-db-test` (postgres:17-alpine, `tmpfs` em `/var/lib/postgresql/data`, porta 5433,
profile `test`).

### 7.2 `deploy/docker-compose.yml` (artefato — CA-16, **não aplicar**)

Igual ao dev exceto: **sem `ports`** no app; rede `caddy_net` declarada `external: true` (nome
exato confirmado na fase 09); `restart: unless-stopped`; env via `deploy/.env` (gitignored).
Nunca menciona PrimaOS/`brain`/`jarvis`; nunca declara serviço Caddy.

### 7.3 `deploy/Caddyfile.snippet` (artefato — CA-16)

```
# Acrescentar ao Caddyfile EXISTENTE da VPS (fase 09). NÃO subir outro Caddy.
timmingtrack.life {
    reverse_proxy timetrack-app:8080
}
```

### 7.4 `Makefile`

`make dev` (Go com `STATIC_DIR` + Vite paralelos) · `make up`/`down` (compose dev) ·
`make migrate` (boot do app já migra; alvo é atalho de inspeção) · `make test-back` (**sobe o
`timetrack-db-test` antes**: `docker compose --profile test up -d --wait timetrack-db-test`,
exporta `TEST_DATABASE_URL` e só então roda `go test ./...` — sem skip silencioso) ·
`make test-front` · `make test-e2e` (Playwright) · `make check` (CA-18: test-back + test-front +
test-e2e + build front + build back) · `make check-full` (**gate oficial da fase**: `check` +
`scripts/smoke-compose.sh`) · `make build`.

## 8. Estrutura de pastas resultante (= SDD.md §7.2, mais `scripts/` de verificação desta fase)

```
backend/
  cmd/server/main.go            # boot: env → db(retry) → migrate → router → listen
  internal/
    api/                        # router chi, /healthz, envelope de erro, static/SPA
    db/
      db.go                     # pool pgx + connect com retry (CA-04)
      migrate.go                # runner (D-03)
      migrations/0001_init.sql
  testdata/
frontend/
  src/
    routes/                     # AppShell + cockpit/ (páginas stub)
    components/                 # SideNav, timer/TimerBar
    lib/                        # skin.ts (api.ts nasce na fase 01, com teste)
    styles/                     # tokens.css, index.css (@theme)
    assets/fonts/               # Geist*.woff2
  e2e/                          # Playwright (shell, hover, tema, offline)
deploy/
  docker-compose.yml · Caddyfile.snippet · .env.example
scripts/
  check-segredos.sh · smoke-compose.sh · check-cores.sh   # verificação (T-01, T-08, T-09)
docker-compose.dev.yml · Dockerfile · Makefile · .env.example · .gitignore
```

(Pastas `internal/auth|workspace|clients|projects|tasks|entries|timer|realtime|calendar` e
`skill/` nascem nas fases donas — não criar vazias agora.)

## 9. Pontos de integração com as fases vizinhas

- **Fase 01 (auth):** usa o runner de migração (arquivos `0002+`), o envelope de erro, o pool pgx
  e monta handlers no mesmo router chi sob `/api/v1/`. Cookie de sessão httpOnly entra lá.
- **Fase 02/03:** novas migrações seguem `NNNN_nome.sql`; `lib/api.ts` (criado na fase 01) e o
  shell (rail + TimerBar) recebem as telas/lógica reais. A TimerBar placeholder
  (`components/timer/TimerBar.tsx`) vira o herói funcional na 03, no mesmo arquivo.
- **Fase 04:** o servidor único Go ganha a rota WS — nada no router desta fase conflita.
- **Fase 08:** middleware de auth por token `tt_` se encaixa sob `/api/v1/` sem mudar o envelope.
- **Fase 09:** pega `deploy/` pronto, confirma o nome real da rede do Caddy e aplica o snippet.
- **Multi-tenant:** nenhuma query de domínio existe nesta fase; a regra "toda query filtra por
  `workspace_id`" passa a ser exercitável (e testada) a partir da fase 01.
