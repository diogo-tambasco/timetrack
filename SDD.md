# SDD.md — Spec-Driven Development · Timetrack

> A *spec de cada feature* é a fonte de verdade; o código é artefato derivado e verificado contra
> ela. Toda feature passa por **spec → plan → tasks → implement** antes de virar código. Pareado
> com TDD (`TDD.md`). Convenção herdada do `prima_os` (GitHub Spec Kit / Kiro, padrão 2025–2026),
> adaptada à stack **Go + PWA**.

---

## 1. Por que SDD aqui

A construção é **autônoma** (Fable 5 coda sozinho, Diogo vê o fim de cada fase). Sem uma spec
verificável por feature, código autônomo diverge da intenção, inventa API e apodrece. A spec é o
contrato que **guia a geração e valida o resultado**.

## 2. Fluxo por feature (4 fases)

```
/specify → spec.md   (o quê + critérios de aceite EARS)   ← fonte de verdade
/plan    → plan.md   (como: arquitetura, dados, contratos de API)
/tasks   → tasks.md  (tarefas atômicas, cada uma com acceptance check)
/implement           (TDD: red → green → refactor por tarefa — ver TDD.md)
```

## 3. Onde vivem as specs

```
specs/
  00-esqueleto/      spec.md  plan.md  tasks.md
  01-auth-google/    ...
  02-core-projetos/  ...
  03-cronometro/     ...
  04-sync-realtime/  ...
  05-modo-foco/      ...
  06-google-calendar/...
  07-relatorios/     ...
  08-conector-claude/...
  09-release/        ...
```
Uma pasta por fase do mapa em `REQUISITOS.md §7`. A spec é escrita **antes** de codar a fase.

## 4. Formato do `spec.md`

- **User stories:** "Como <papel>, quero <capacidade>, pra <resultado>."
- **Critérios de aceite em EARS** (frases testáveis — cada uma vira um teste):
  - *Ubíquo:* "O sistema DEVE sempre isolar dados por workspace."
  - *Event-driven:* "QUANDO o usuário pausa o timer, o sistema DEVE pausar em todos os dispositivos e iniciar o Pomodoro de pausa."
  - *State-driven:* "ENQUANTO um timer estiver ativo, o sistema DEVE refletir o tempo decorrido em tempo real em todos os devices conectados."
  - *Unwanted:* "SE o token OAuth do Google expirar, o sistema NÃO DEVE perder entradas e DEVE pedir reconexão."
  - *Optional:* "ONDE o projeto for compartilhado, os membros do workspace DEVEM ver o timer ativo do dono."
- **Requisitos funcionais** + **não-funcionais** (leveza, tempo-real, PWA/offline, segurança).
- **Fora de escopo** (fronteiras explícitas — ex.: multiplayer real fora do MVP).
- **Mapa pros 4 insights / problema central** do `REQUISITOS.md`, quando aplicável.

## 5. Formato do `plan.md`

Decisões de arquitetura + razão · modelo de dados/migrações · **contratos de API** (rotas REST,
mensagens WebSocket, payloads JSON, retornos/erros) · libs Go/JS · pontos de integração com o
baseline (§7). Herda o baseline.

## 6. Formato do `tasks.md`

Tarefas **atômicas e independentes**, cada uma com: objetivo único · entradas · entregável ·
**acceptance check** (geralmente "teste X verde"). **Commit por tarefa.**

## 7. Baseline técnico compartilhado (todo `plan.md` herda)

### 7.1 Topologia (deploy)
```
internet → Caddy EXISTENTE (TLS ACME, já no ar :80/:443)
              ├── hub.primorlabs.cloud → PrimaOS   (NÃO TOCAR)
              └── timmingtrack.life       → timetrack-app (Go)
                                            └── timetrack-db (Postgres)
```
Timetrack = containers próprios (`timetrack-app`, `timetrack-db`) numa rede Docker própria.
**Adicionar vhost ao Caddy existente, nunca subir um segundo Caddy nas portas 80/443.**

### 7.2 Estrutura de pastas (provisória)
```
backend/        Go — net/http + chi
  cmd/server/   main
  internal/
    auth/       OAuth Google + sessão
    workspace/ clients/ projects/ tasks/ entries/   (domínio)
    timer/      lógica de cronômetro + pomodoro
    realtime/   hub WebSocket (broadcast por workspace/user)
    calendar/   sync Google Calendar
    api/        handlers REST + token API (conector Claude)
    db/         migrações + queries (sqlc ou pgx direto)
  testdata/     fixtures
frontend/       React + Vite + TS + Tailwind (PWA)
  src/
    routes/     cockpit (desktop) · foco (iPad) · mobile
    components/  timer, flip-clock, calendar, reports
    lib/        api client, ws client, store (zustand), query
deploy/         docker-compose.yml · Caddy snippet (vhost) · .env.example
skill/          conector Claude /timetrack-task
```

### 7.3 Tempo real (núcleo)
Hub WebSocket em Go: cada device conecta autenticado; eventos de timer (`start/stop/pause/resume`)
e Pomodoro são **broadcast por usuário** (e por workspace quando compartilhado). Cliente reconcilia
estado ao reconectar. **Pause → evento que inicia o Pomodoro em todos os devices.**

### 7.4 Persistência
Postgres. Migrações versionadas. Multi-tenant: **toda query filtra por `workspace_id`** (e
visibilidade do projeto). Segredos/tokens OAuth cifrados em repouso.

### 7.5 Auth & API
Google OAuth (login + Calendar). Sessão por cookie no PWA. **Tokens de API com prefixo `tt_`**
(escopo por workspace) pro conector Claude. Nunca logar token/segredo.

### 7.6 Deploy / install
`deploy/docker-compose.yml` (app + db) + snippet do vhost pro Caddy existente. Migrate no boot.
Alvo: VPS labs `<IP_DA_VPS>` (compartilhada — ver `SECRETS.local.md`). Vizinhos intocados.

## 8. Constitution (princípios duráveis — toda spec respeita)

1. **PT-BR**, direto, sem disclaimer.
2. **Isolamento da VPS:** nunca derrubar PrimaOS/`brain`/`jarvis`; nunca tomar 80/443.
3. **TDD obrigatório:** nenhuma tarefa de código sem teste que falhou antes (ver `TDD.md`).
4. **Multi-tenant sempre:** toda query por `workspace_id`; vazamento entre workspaces = bug crítico.
5. **Spec é a verdade:** divergência código×spec = bug no código (ou a spec muda primeiro).
6. **Leveza:** sem dependência pesada sem decisão explícita.
7. **Segredos** (`SECRETS.local.md`, `.env*`, tokens) **sempre gitignored**.
8. **Escopo MVP travado** (`REQUISITOS.md §4/§5`): multiplayer real e Git ficam fora.
