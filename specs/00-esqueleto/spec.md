# spec.md — Fase 00 · Esqueleto

> **Status:** rascunho pra validação do Diogo.
> **Fonte de verdade desta fase.** Deriva de `REQUISITOS.md §7` (fase 00), `SDD.md §7` (baseline),
> `TDD.md §5` (foco de teste da fase 00) e do design em
> `Design/Timmingtrack.life/timetrack/` (tokens.css, parts.jsx, app.jsx, DESIGN-additions.md).

---

## 1. Objetivo da fase

Scaffold completo do Timetrack **rodando LOCAL**: backend Go com healthcheck e infra de migrações
ligada no boot, Postgres via Docker Compose, frontend PWA com o **shell do cockpit** (rail de
navegação hover-expand + barra de timer global placeholder + tema dark/light com os tokens do
design), e a pasta `deploy/` com os artefatos de produção **prontos mas não aplicados** (deploy
real na VPS é a fase 09).

Esta fase não entrega feature de usuário final — entrega a **fundação verificável** sobre a qual
as fases 01–09 constroem sem retrabalho.

## 2. Mapa pro problema central (REQUISITOS.md §2)

A fase 00 não resolve nenhum dos 3 problemas diretamente, mas destrava todos:

| Problema central | O que a fase 00 prepara |
|---|---|
| 1. Trackear rápido por cliente/projeto | Shell do cockpit com a barra de timer global (herói visual) já no lugar; infra de migrações pra receber o schema das fases 01–03 |
| 2. Sync com Google Calendar | Estrutura `backend/internal/` e envelope de erro/rotas `/api/v1/` que a fase 06 reusa |
| 3. Sync em tempo real entre devices | PWA instalável (manifest + service worker) — pré-condição do iPad/celular como devices; servidor Go único que vai hospedar o hub WebSocket (fase 04) |

## 3. User stories

- **US-1** — Como **dono do projeto (Diogo)**, quero um backend Go mínimo com healthcheck e
  migrações automáticas no boot, pra que toda fase seguinte só precise adicionar arquivos SQL e
  handlers, sem mexer em infraestrutura.
- **US-2** — Como **desenvolvedor (Claude em build autônomo)**, quero subir todo o ambiente local
  com um comando (`make up`), pra rodar a suíte de testes e as próximas fases sem setup manual.
- **US-3** — Como **usuário (Diogo)**, quero abrir o app no navegador e ver o shell do cockpit
  (rail de navegação + barra de timer) com o visual do design aprovado (dark, teal, Geist), pra
  validar a direção visual antes das features existirem.
- **US-4** — Como **usuário no iPad/celular**, quero que o app seja um PWA instalável que carrega
  o shell mesmo offline, pra base do Modo Foco (fase 05) e do controle remoto já existir.
- **US-5** — Como **operador da VPS compartilhada**, quero que os artefatos de deploy
  (`deploy/`) já nasçam isolados (sem tomar 80/443, sem tocar no PrimaOS), pra que a fase 09 seja
  só aplicar — sem risco pros vizinhos.

## 4. Critérios de aceite (EARS — todo critério é verificado por pelo menos um teste; nenhum critério sem teste)

### Backend (Go)

- **CA-01** *(event-driven)* — QUANDO o servidor receber `GET /healthz`, o sistema DEVE responder
  HTTP `200` com corpo JSON `{"status":"ok"}` e header `Content-Type: application/json`.
- **CA-02** *(event-driven)* — QUANDO o processo do servidor iniciar, o sistema DEVE aplicar as
  migrações pendentes de `backend/internal/db/migrations/` em ordem crescente de prefixo `NNNN`
  **antes** de abrir o listener HTTP (nenhuma requisição é atendida com migração pendente).
- **CA-03** *(ubíquo)* — O sistema DEVE registrar cada migração aplicada na tabela
  `schema_migrations` e NUNCA reaplicar uma migração já registrada (dois boots seguidos deixam o
  banco em estado idêntico).
- **CA-04** *(unwanted)* — SE o Postgres estiver indisponível no boot, o sistema DEVE tentar
  reconectar com backoff por até **30 segundos** e, persistindo a falha, encerrar com exit code
  diferente de 0 e uma linha de log indicando o motivo (nunca ficar de pé sem banco).
- **CA-05** *(unwanted)* — SE uma migração falhar no meio da execução, o boot DEVE abortar com
  exit code diferente de 0 e a migração falhada NÃO DEVE constar em `schema_migrations`
  (cada migração roda em transação própria).
- **CA-06** *(unwanted)* — SE a variável de ambiente `DATABASE_URL` não estiver definida, o
  processo DEVE falhar no boot com mensagem de erro que cita a variável faltante.
- **CA-07** *(ubíquo)* — O sistema DEVE responder: (a) a qualquer rota inexistente sob `/api/v1/`
  com HTTP `404` e corpo `{"error":{"code":"not_found","message":"rota não encontrada"}}`;
  (b) a método não permitido em **rota existente** com HTTP `405` e corpo
  `{"error":{"code":"method_not_allowed","message":"método não permitido"}}` — exemplo canônico
  exercitável nesta fase: `DELETE /healthz` (a fase 00 não tem nenhuma rota sob `/api/v1/`, então
  o 405 sob o prefixo só vira exercitável na fase 01). Envelope de erro canônico de todas as fases.
- **CA-19** *(event-driven)* — QUANDO o servidor receber `GET` de rota **sem extensão de arquivo**
  fora de `/api` e `/healthz` (ex.: `/ajustes`), o sistema DEVE responder o `index.html` do
  `dist/` com HTTP `200` (fallback SPA); arquivos existentes (`/assets/*`,
  `/manifest.webmanifest`, `/sw.js`) DEVEM ser servidos como estáticos, e o 404 JSON de `/api/v1/`
  (CA-07) NÃO DEVE ser engolido pelo fallback.

### Frontend (PWA)

- **CA-08** *(event-driven)* — QUANDO `npm run build` for executado em `frontend/`, o build DEVE
  concluir com exit code 0 e produzir em `dist/` um `manifest.webmanifest` com `name` "Timetrack",
  `display` "standalone", `theme_color` `#0e0e10` e ícones de 192px e 512px, além do service
  worker gerado.
- **CA-09** *(event-driven)* — QUANDO o app carregar no navegador, o shell do cockpit DEVE exibir
  o rail de navegação à esquerda com os itens Hoje, Calendário, Projetos, Clientes, Relatórios,
  Workspace, Foco e Ajustes (só ícones em repouso).
- **CA-09.1** *(event-driven)* — QUANDO o app carregar no navegador, o shell DEVE exibir a barra
  de timer global no topo com o placeholder `0:00:00` em fonte mono e controles desabilitados
  (timer real é fase 03), presente em todas as rotas.
  *(SUPERADO em 10/06 pela fase 03 construída — a sunset clause acima venceu: a barra é funcional
  desde a T-15 da 03. O que sobrevive do CA (barra global visível em todas as rotas + dígitos
  mono) segue assertado em `frontend/e2e/timerbar.spec.ts`; o comportamento funcional vive em
  `cronometro.spec.ts` + specs Vitest do TimerBar.)*
- **CA-10** *(state-driven)* — ENQUANTO o ponteiro estiver sobre o rail, ele DEVE expandir pra
  **232px** como **overlay** exibindo os rótulos, SEM alterar a largura ocupada no layout
  (conteúdo principal não sofre reflow — largura reservada permanece **64px**); ao sair do hover,
  DEVE recolher pra 64px.
- **CA-11** *(ubíquo)* — O tema dark DEVE ser o padrão na primeira visita (sem localStorage):
  background computado `#0e0e10` (`--bg`) e accent `#2dd4bf` (`--accent`).
- **CA-11.1** *(ubíquo)* — Os tokens de `tokens.css` DEVEM ser a única fonte das cores de
  superfície/texto: o código do front (`frontend/src/`, **exceto** `styles/tokens.css`) NÃO DEVE
  conter cor hex literal (`#xxx`/`#xxxxxx`) — verificado por script de lint (grep com allowlist
  documentada no próprio script).
- **CA-12** *(optional)* — ONDE o usuário tiver selecionado o tema light, o documento DEVE receber
  `data-theme="light"` (tokens light aplicados) e a escolha DEVE persistir entre reloads
  (localStorage).
- **CA-13** *(ubíquo)* — O bundle de produção (`dist/`) NÃO DEVE conter nenhuma referência a
  `fonts.googleapis.com` ou `fonts.gstatic.com`; Geist e Geist Mono DEVEM ser servidas como
  arquivos `woff2` do próprio app (self-hosted).
- **CA-14** *(state-driven)* — ENQUANTO o dispositivo estiver offline (após pelo menos uma visita
  online), o shell do app DEVE carregar a partir do cache do service worker (app shell precache) —
  a página renderiza o rail e a barra de timer sem rede.

### Infra local & artefatos de deploy

- **CA-15** *(event-driven)* — QUANDO `make up` (Docker Compose local) for executado numa máquina
  com Docker, os containers `timetrack-db` (Postgres) e `timetrack-app` (Go) DEVEM subir e
  `GET http://localhost:8080/healthz` DEVE responder `200` em até **60 segundos**.
- **CA-16** *(ubíquo)* — O artefato `deploy/docker-compose.yml` NÃO DEVE publicar as portas 80 ou
  443 nem declarar um serviço Caddy próprio; DEVE conectar o app a uma rede Docker **externa** (a
  do Caddy existente) e `deploy/Caddyfile.snippet` DEVE conter apenas o vhost `timmingtrack.life`
  apontando pro container do app (artefatos validados por teste de inspeção — **sem aplicar na VPS**).
- **CA-17** *(unwanted)* — SE um segredo real aparecer em arquivo versionável, é bug: o
  `.gitignore` DEVE cobrir `.env`, `.env.*` (exceto `.env.example`) e `SECRETS.local.md`, e o
  `.env.example` DEVE conter todas as variáveis necessárias **só com placeholders**.
- **CA-18** *(event-driven)* — QUANDO `make check` for executado, o sistema DEVE rodar
  `go test ./...` (backend, com o Postgres de teste de pé), `vitest run` (frontend), a suíte E2E
  Playwright (`test-e2e`) e os **dois builds** (Vite no front e `go build` no back), retornando
  exit code 0 com tudo verde; e QUANDO `make check-full` for executado, DEVE rodar `check` + o
  smoke do compose (`scripts/smoke-compose.sh` — CA-15). **`make check-full` é o gate oficial de
  fim de fase** (TDD.md §6).

## 5. Requisitos funcionais

- **RF-01** — Servidor HTTP Go (stdlib `net/http` + router `chi`) com `GET /healthz` e envelope
  de erro JSON canônico pra `/api/v1/` (CA-01, CA-07).
- **RF-02** — Runner de migrações embutido no binário: lê `NNNN_nome.sql` embarcados, aplica em
  ordem, registra em `schema_migrations`, transação por arquivo, roda no boot (CA-02, CA-03, CA-05).
- **RF-03** — Configuração 100% por variáveis de ambiente (`DATABASE_URL`, `ADDR`), documentadas
  em `.env.example` (CA-06, CA-17).
- **RF-04** — Migração `0001_init.sql`: habilita a extensão `pgcrypto` (pra `gen_random_uuid()`
  das tabelas da fase 01+) — prova o pipeline sem invadir o schema das fases seguintes.
- **RF-05** — Frontend React + Vite + TS + Tailwind com `tokens.css` integrado como fonte de
  verdade visual e shell do cockpit: `SideNav` (rail hover-expand canônico) + `TimerBar`
  placeholder (CA-09, CA-09.1, CA-10, CA-11, CA-11.1, CA-12).
- **RF-06** — PWA: `manifest.webmanifest` + service worker com precache do app shell (CA-08, CA-14).
- **RF-07** — Em produção, o binário Go serve os estáticos do frontend (`dist/`) com fallback de
  SPA (rota não-arquivo e não-`/api` → `index.html`) (CA-19); em dev, Vite dev server com proxy
  pro Go.
- **RF-08** — `docker-compose.dev.yml` (raiz) pra ambiente local; `deploy/docker-compose.yml` +
  `deploy/Caddyfile.snippet` + `deploy/.env.example` como artefatos de produção (CA-15, CA-16).
- **RF-09** — `Makefile` com alvos: `dev` (back+front em watch), `up`/`down` (compose local),
  `test-back`/`test-front`/`test-e2e`, `check`, `check-full` (gate da fase), `build` (CA-18).

## 6. Requisitos não-funcionais

- **RNF-01 · Leveza** — Backend sem framework web: stdlib + `chi` + driver Postgres (`pgx`).
  Runner de migração próprio (≈100 linhas) em vez de lib externa. `go.mod` enxuto; dependência
  nova exige decisão explícita (Constitution §8.6).
- **RNF-02 · Isolamento da VPS** — Nada desta fase toca a VPS. Os artefatos de `deploy/` nascem
  já isolados: containers próprios, rede externa do Caddy, **nunca** portas 80/443, **nunca**
  referência ao compose do PrimaOS (CA-16).
- **RNF-03 · Segurança/segredos** — Segredos fora do git (CA-17); nenhum log imprime valor de
  variável sensível.
- **RNF-04 · PWA/offline** — App shell carrega offline (CA-14); fontes self-hosted eliminam
  dependência de terceiros em runtime (CA-13).
- **RNF-05 · Performance local** — Boot do servidor (com banco de pé e migrações já aplicadas)
  em menos de 2s; `/healthz` responde em menos de 100ms local.
- **RNF-06 · Testabilidade** — Toda peça nasce testável: handler via `httptest`, migrador contra
  Postgres efêmero de teste, front via Vitest + Playwright (TDD.md §3 — nunca tocar produção).

## 7. Fora de escopo (fases seguintes — não invadir)

- **Auth Google, sessões, workspaces, users** → fase 01 (dona das migrações de `users`,
  `workspaces`, `workspace_members`, `sessions`).
- **CRUD de clientes/projetos/tasks** → fase 02.
- **Timer funcional, time entries, Pomodoro** → fase 03 (a TimerBar desta fase é placeholder
  estático, sem lógica).
- **WebSocket / sync entre devices** → fase 04 (nenhum endpoint WS nesta fase).
- **Modo Foco / flip-clock / Wake Lock** → fase 05.
- **Google Calendar** → fase 06. **Relatórios** → fase 07.
- **API token `tt_` / conector Claude** → fase 08 (mas o envelope de erro e o prefixo `/api/v1/`
  desta fase não podem impedi-la — e não impedem).
- **Deploy real na VPS, DNS, vhost aplicado** → fase 09 (esta fase só produz os artefatos).
- **Multiplayer real** (criar/pedir tasks entre colaboradores) → fora do MVP inteiro.
- **Seletor de accent/densidade nas Settings** → fase de polish (09); os tokens já suportam
  (`data-density`, `--accent`), mas a UI de personalização não entra aqui — apenas o toggle
  dark/light pra provar o pipeline de tema (CA-12).
