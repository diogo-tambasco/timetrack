# spec.md — Fase 13 · Dashboard (home nova)

> Fonte de verdade da fase. Transforma `/` na **home de comando** do app: substitui o
> `/hoje` por um dashboard que junta saudação + card do timer rodando + entradas recentes
> dos **últimos N dias** (não só hoje) + objetivos/metas da semana + atalho pro Modo Foco.
> Base no mockup `Design/.../cockpit.jsx` **Direção C "Hero Timer"** (+ ideias da B "Mission
> Control": KPI strip). É a fase 13 da M1 (`MILESTONE-1.md §4`), resolve a dor **P3** ("virou
> o dia, sumiram minhas tasks").
>
> **Decisões travadas (MILESTONE-1 §7, HITL 19/06):**
> 1. **O dashboard absorve o `/hoje`** — uma home só; o conteúdo do dia (lista, total, nova
>    entrada) entra no dashboard. `/` passa a renderizar o dashboard; `/hoje` redireciona.
> 2. **Mantém a TimerBar global** do topo (NÃO duplicar controles de timer de forma
>    conflitante — o card do timer reage ao MESMO `timerStore`, como o Modo Foco da 05).
> 3. **Sem migração nova** — lê de endpoints existentes (`/entries`, `/reports/week`) + o
>    contrato de objetivos da fase 12.
> 4. **Recentes = últimos 7 dias** (janela default; ver CA-06), pra cobrir a virada do dia.
>
> Depende de: fase 03 (`timerStore`/`useTimerClock`/`TodayList`/`EntryModal`/`listEntries`),
> fase 04 (sync realtime — o card reflete mudanças de outro device), fase 05 (Modo Foco —
> destino do atalho; `fmtDayTotal`), fase 07 (`/reports/week` → metas), fase 12 (objetivos
> da semana — contrato de `MILESTONE-1 §5`).

---

## 1. Objetivo

Dar ao Diogo uma **home única** que responde "onde eu estava e pra onde vou" assim que abre o
app: o que está rodando AGORA (hero), o que rolou nos **últimos dias** (não some na virada do
dia — P3), e o progresso dos **objetivos/metas da semana**, com um atalho direto pro Modo Foco.
A barra de timer global do topo continua sendo o controle primário; o dashboard nasce **abaixo**
dela e nunca briga com ela.

Esta fase é **frontend** (composição de poucas queries já existentes + uma da fase 12),
**sem backend novo** (RNF-03).

## 2. User stories

- **US-01** — Como usuário, quero abrir o app e ver **uma saudação + o timer que está rodando**
  num card de destaque (ou um CTA pra começar quando ocioso), pra entrar no ritmo na hora.
- **US-02** — Como usuário, quero ver minhas **entradas/tasks recentes dos últimos dias**
  (não só de hoje), pra não perder o contexto quando o dia vira à meia-noite.
- **US-03** — Como usuário, quero ver o **progresso dos meus objetivos/metas da semana** no
  dashboard, pra saber se estou no rumo sem abrir os relatórios.
- **US-04** — Como usuário, quero um **atalho pro Modo Foco** direto da home, pra entrar no
  flip-clock fullscreen num toque.
- **US-05** — Como usuário, quero **criar uma entrada manual** e **continuar/editar** uma
  entrada recente sem sair da home (o que o `/hoje` já fazia), pra não perder o fluxo.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* O dashboard DEVE consumir estado de timer **exclusivamente** do
  `timerStore` (fase 03/04) e mutar **exclusivamente** pelas ações existentes do store
  (`pause`/`stop`) — nenhuma requisição periódica nova de timer, nenhum estado de timer
  paralelo. *(Verificação: revisão de imports do `Dashboard.tsx` no T-08 — só store/queries
  sancionadas no plan §3.)*
- **CA-02** *(ubíquo)* **Zero mudança de backend**: nenhuma rota, payload ou migração nova.
  As suítes das fases 00–12 seguem verdes **sem edição de asserções de comportamento** (exceto
  os toques aditivos enumerados no plan §3: roteamento `/`/`/hoje`, SideNav, a aposentadoria do
  `Today.tsx`, e **o ajuste da asserção de URL pós-login em `e2e/helpers.ts`** — de `/hoje$` pra
  `/$`, já que agora o login cai na home `/` (Dashboard). Ver CA-22/CA-24/CA-24.1 e plan §3).
  *(Furo confirmado na verificação adversarial da spec: `loginViaStub` afirma `toHaveURL(/\/hoje$/)`
  e é usado por TODO E2E logado — sem esse ajuste, CA-04 e CA-02 se contradizem.)*
- **CA-03** *(ubíquo)* O dashboard nasce **abaixo da TimerBar global** e NÃO a remove nem a
  duplica: a TimerBar do `AppShell` continua sendo o controle primário do timer e o sync da
  fase 04 segue funcionando inalterado.

### 3.2 Roteamento (a home absorve o /hoje)

- **CA-04** — `/` (autenticado) DEVE renderizar o **Dashboard** (não mais `Navigate to /hoje`).
- **CA-05** — `/hoje` DEVE **redirecionar pra `/`** (`Navigate replace`), pra não quebrar
  bookmarks/links antigos nem o item de navegação. O conteúdo do dia vive no dashboard.

### 3.3 Entradas recentes — últimos N dias (resolve P3)

- **CA-06** (janela) — a seção "Recentes" DEVE buscar entradas dos **últimos 7 dias**
  (`from = hoje−6`, `to = hoje`, no fuso do navegador) via `listEntries(ws, {from, to, tz})`
  — **sem endpoint novo** (reusa o filtro from/to da fase 03). A janela (7 dias) é constante
  documentada no plan; não é configurável nesta fase.
- **CA-07** (agrupado por dia) — as entradas recentes DEVEM ser **agrupadas por dia local**
  (do `started_at` no fuso do navegador), mais recente primeiro, com um cabeçalho de dia
  legível ("Hoje", "Ontem", ou a data). Cada item mostra descrição, projeto (cor+nome,
  inclusive arquivado — `archived:true`), horário e duração — reaproveitando a forma de linha
  do `TodayList`.
- **CA-08** (entry rodando) — a entry **rodando** (`stopped_at === null`) aparece em destaque
  (accent) com duração viva derivada de `started_at + offset` (mesma aritmética do `TodayList`,
  RNF-02 da 05 — nunca acumula tempo local); QUANDO o timer para, ela vira fechada **sem
  reload** (o dashboard re-busca ao mudar a entry ativa, como o `TodayList`).
- **CA-09** (continuar/editar/parar) — cada entrada recente fechada DEVE oferecer
  **continuar** (clona via `useContinueEntry`) e **editar** (abre `EntryModal`); a rodando
  oferece **parar** (`stop` do store) — exatamente as ações do `TodayList` hoje.
- **CA-10** (nova entrada) — o dashboard DEVE ter o botão **"Nova entrada"** que abre o
  `EntryModal` (mesma ação do `/hoje` atual).
- **CA-11** (vazio) — sem entradas nos últimos 7 dias, a seção DEVE exibir um estado vazio
  amigável (ex.: "Nada cronometrado nos últimos dias."), sem erro e sem NaN.

### 3.4 Card do timer (hero — Direção C)

- **CA-12** (rodando) — ENQUANTO um timer roda, o **card do timer** DEVE exibir: rótulo
  "Rodando agora", a descrição/task da entry, o **chip do projeto** (cor+nome, inclusive
  arquivado), os **dígitos grandes mono** com o tempo `shown` (mesma regra da TimerBar:
  countup → `elapsed`; countdown/pomodoro → `remaining ?? elapsed`, derivado localmente via
  `useTimerClock`) e controles discretos **Pausar / Parar** (ações do store — CA-01).
- **CA-13** (pausa) — ENQUANTO o estado é pausa (`mode==='break'`), o card DEVE indicar a
  **pausa** (rótulo + contagem regressiva do break via `useTimerClock`), sem virar o
  BreakOverlay completo (esse é da TimerBar) — o dashboard só **reflete** o estado de pausa.
- **CA-14** (ocioso) — ENQUANTO não há timer nem pausa, o card DEVE exibir um **CTA pra
  começar**: texto convidando + um caminho pra iniciar (foca a TimerBar do topo, ou abre a
  "Nova entrada") — **sem** duplicar o formulário completo de start da TimerBar (CA-03; não
  competir). O CTA NÃO mostra dígitos vivos (não há timer).
- **CA-15** (saudação) — o topo do dashboard DEVE mostrar a **saudação** ("Bom trabalho,
  Diogo." usando o primeiro nome do `me`, ou "Bom trabalho." sem nome) + a data de hoje,
  espelhando o cabeçalho da Direção C.

### 3.5 Objetivos / metas da semana

- **CA-16** (metas de horas) — o dashboard DEVE exibir as **metas da semana por projeto**
  (`weekly_hours_goal`) reusando `useWeekReport(ws, hojeISO)` (fase 07): cada projeto com meta
  numa **GoalBar** (a mesma da fase 02/07: fill na cor do projeto, accent quando ≥100%) com
  meta × realizado. Projetos sem meta não aparecem aqui.
- **CA-17** (progresso de objetivo = FORA do escopo da fase 13, decisão de produto) — o
  **progresso por objetivo** (projeto→objetivo, % de tasks `done` + horas × `goal_hours`) **NÃO
  é exibido no dashboard nesta fase**. Motivo (furo confirmado na verificação adversarial,
  ancorado no código real): a fase 12 só expõe progresso de objetivo no `BoardObjective`
  (`done_pct`/`task_count`/`done_count`/`tracked_seconds`), devolvido **exclusivamente** pelo
  `GET /board` **por projeto** (`getBoard(ws, projectId)`); o `useObjectives`/`listObjectives`
  devolve o objetivo "pelado" SEM progresso. Não existe endpoint de objetivos-com-progresso por
  **workspace**. Agregá-lo exigiria **N requests** (um `/board` por projeto — viola RNF-02) ou
  **backend novo** (viola RNF-03/CA-02). Portanto o "objetivos da semana" da home é entregue
  como **metas de horas por projeto** (CA-16) e o progresso por-objetivo agregado fica
  **registrado como fase futura** (precisa de um endpoint `GET /workspaces/{id}/objectives`
  com progresso). *(A página `/projetos/:id` (board da fase 12) segue sendo o lugar do progresso
  por objetivo — o dashboard só linka pra lá quando o usuário quer detalhe.)*
- **CA-18** (seção da semana = metas de horas) — a seção "da semana" do dashboard DEVE renderizar
  as **metas de horas por projeto** (CA-16) como conteúdo canônico — não há "degradação por erro
  de runtime" (não existe query de objetivos-de-workspace pra falhar). SE `/reports/week` falha,
  a seção mostra estado de erro/vazio resiliente sem derrubar o resto da home (CA-21).
- **CA-19** (semana vazia) — sem metas e sem objetivos, a seção DEVE exibir um estado vazio
  amigável, sem erro.

### 3.6 Atalho pro Modo Foco

- **CA-20** — o dashboard DEVE ter um **atalho visível pro Modo Foco** (`/foco`), via
  `Link`/`navigate`, com rótulo claro (ex.: "Modo Foco"). Não duplica o item da SideNav — é
  uma ação proeminente da home (DESIGN da Direção C: o foco é primeira classe).

### 3.7 Estados de carga/erro

- **CA-21** — ENQUANTO as queries do dashboard carregam, cada seção exibe um estado neutro
  (skeleton/placeholder), sem layout shift brusco e sem NaN; um erro numa seção (ex.:
  `/reports/week` falha) NÃO derruba as outras seções (cada bloco é resiliente).

### 3.8 Não-regressão (herdado — não quebrar)

- **CA-22** — o item de navegação **"Hoje"** da SideNav DEVE continuar funcionando: ou aponta
  pra `/` (recomendado, já que o dashboard é a home), ou pra `/hoje` que redireciona pra `/`
  (CA-05). Em ambos os casos clicar "Hoje" leva ao dashboard, e o `aria-current` reflete o
  dashboard quando ativo. (Registrar a escolha no plan.)
- **CA-23** — a TimerBar global, o BreakOverlay e o sync da fase 04 seguem **intocados**: os
  testes das fases 03/04/05 (TimerBar/BreakOverlay/Focus/sync) passam **sem edição de asserções
  de comportamento**. Exceção mecânica única e sancionada: o ajuste da asserção de URL pós-login
  em `e2e/helpers.ts` (CA-02). Se o `✕` do `Focus.tsx` (que hoje faz `navigate('/hoje')`)
  precisar de ajuste, ele pode apontar pra `/` **ou** seguir indo pra `/hoje` (que redireciona pra
  `/` — CA-05); em qualquer caso o `Focus.spec` segue verde (o destino final é a home). Registrar
  a escolha no T-08.
- **CA-24** — o `Today.tsx` (página antiga) é **aposentado/absorvido**: removido OU reduzido a
  um redirect; nenhum teste de fase anterior pina a página `/hoje` como tela própria (checar no
  T-08; o `TodayList`/`EntryModal` continuam vivos — são reusados pelo dashboard, não tocados).
- **CA-24.1** (o Dashboard herda o contrato de UI do `/hoje` — furo confirmado) — como `/hoje`
  agora redireciona pra `/` (CA-05) e o `e2e/cronometro.spec` faz `goto('/hoje')` e então dirige
  **"Nova entrada"**, **"Editar"** por linha e a estrutura do `TodayList`, o **Dashboard DEVE
  expor os MESMOS papéis/nomes acessíveis** que a antiga página `/hoje` expunha, pra que o
  `cronometro.spec` passe **através do redirect sem editar asserções**: (a) um botão `name:
  /nova entrada/i` que abre o `EntryModal` (CA-10); (b) as entradas de **hoje** com **"Editar"**
  por linha e a forma do `TodayList`. **Implementação preferida (D-07):** o Dashboard **reusa o
  componente `TodayList`** para as entradas de hoje (selectors idênticos por construção) e a
  seção "Recentes" (RecentEntries) cobre os **dias anteriores** (ontem..−6) agrupados por dia
  (resolve P3). Assim o `cronometro.spec` (que mexe em entradas de hoje) bate no `TodayList`
  reusado, inalterado.

### 3.9 E2E (Playwright)

- **CA-25** — Logado, num workspace com timer rodando: abrir `/` mostra o **card do timer**
  com os dígitos vivos e o projeto; semear (via API) uma entrada **fechada de ontem** → ela
  aparece na seção **Recentes** sob o cabeçalho "Ontem" (prova P3: dado de dia anterior
  visível); **continuar** uma entrada recente reflete no card/TimerBar; clicar o atalho **Modo
  Foco** navega pra `/foco`; o item **Hoje** da SideNav volta pro dashboard.

## 4. Requisitos funcionais (resumo)

- **RF-01** Rota `/` vira `<Dashboard/>`; `/hoje` redireciona pra `/` (CA-04/05).
- **RF-02** `routes/cockpit/Dashboard.tsx` (NOVO) compõe: `Greeting`, `TimerCard` (hero), o
  **`TodayList` reusado** (entradas de HOJE — CA-24.1), `RecentEntries` (dias anteriores
  agrupados por dia), `WeekGoals` (metas de horas da semana), `FocusShortcut`, e o botão
  "Nova entrada" + `EntryModal`.
- **RF-03** `useRecentEntries(ws)` (NOVO hook em `lib/queries/entries.ts`, aditivo) =
  `listEntries(ws, {from: hoje−6, to: hoje, tz})`; agrupamento por dia local é função pura
  testável (`lib/recent.ts`). `RecentEntries` renderiza só os grupos de **dias anteriores**
  (pula o grupo "Hoje", que é do `TodayList` reusado — CA-24.1), evitando duplicar hoje.
- **RF-04** A seção "da semana" = `WeekGoals` (metas de horas por projeto via `useWeekReport`,
  CA-16). **Não há `WeekObjectives`** (CA-17: progresso por-objetivo agregado por workspace não
  existe no contrato da fase 12 — fica pra fase futura).
- **RF-05** Reusa `TodayList`-style de linha, `EntryModal`, `useContinueEntry`,
  `useTimerStore`, `useTimerClock`, `useWeekReport`, `fmtClock`/`fmtDayTotal`/`fmtHourMinute`/
  `browserTZ` — sem recriar nada.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** **zero dependência nova** (JS e Go). Composição de queries existentes +
  CSS/tokens.
- **RNF-02 Boot leve:** o dashboard compõe de **poucas queries** (timer já no store via sync;
  recentes = 1 request de `/entries`; metas = 1 de `/reports/week`; objetivos = 1 da fase 12)
  — no máximo ~3 requests na home, todos cacheados pelo TanStack Query (chaves reusadas). Não
  introduzir polling. As seções carregam independentes (CA-21) — uma lenta não trava as outras.
- **RNF-03 Sem estado novo:** nenhuma migração, nenhuma tabela, nenhum endpoint. Tudo derivado
  de `/entries`, `/reports/week`, do timerStore e do contrato da fase 12.
- **RNF-04 Tokens:** **`tokens.css` é a única fonte de cor** (gate `check-cores`): hero card,
  dígitos, GoalBar e barras usam tokens/classes existentes (`--accent*`, `--surface-*`,
  `--border`, `--text-*`, `.tt-card`, `.tt-goal`). Fontes Geist. Cor de projeto é dado
  (`palette.ts`, já sancionado na fase 02). Sem hex literal novo.
- **RNF-05 PWA intacto:** sem mexer no service worker; `navigateFallbackDenylist` segue como
  está; o app instalado continua abrindo na home (`/`).

## 6. Mapa pro problema central (REQUISITOS / MILESTONE-1)

- **P3** (`MILESTONE-1 §2`): "virou o dia, sumiram minhas tasks" → entradas recentes dos
  últimos 7 dias (CA-06/07), não só hoje. **É o coração da fase.**
- DESIGN §3.1 (cockpit: home de comando) + `cockpit.jsx` Direção C (Hero Timer) → o card do
  timer + saudação + recentes + metas (CA-12..16).
- `MILESTONE-1 §4 Fase 13` + §5 (contrato de objetivos) → CA-17 (consome objetivos da 12).

## 7. Fora de escopo (fronteiras explícitas)

- **Calendário embutido no dashboard** — a grade de calendário vive em `/calendario` (fase 11);
  o mockup da Direção C tem um `CalendarGrid`, mas trazê-lo pra home é fase futura (mantém a
  home leve — RNF-02). O dashboard NÃO embute calendário.
- **Drag-and-drop** de entradas/tasks no dashboard — fase futura.
- **KPI strip completa da Direção B** (foco profundo, donut rastreado×não) — esses cards
  pertencem aos **Relatórios 2.0** (fase 14); o dashboard só mostra metas/objetivos da semana.
  Um KPI mínimo ("total da semana"/"total hoje") é permitido se vier de query já existente, mas
  os cards analíticos densos ficam na 14.
- **Redesign mobile** do dashboard (celular/tablet) — vai pra **M2** (`MILESTONE-1 §3`).
- **Iniciar timer com formulário completo dentro do card** — o start completo é da TimerBar
  (CA-14; não duplicar controles conflitantes — CA-03).
- **Configurar a janela de "recentes"** (quantos dias/itens) — fixa em 7 dias nesta fase
  (CA-06); configurável é candidato a fase futura.
- **Resumo IA / coach** — future feature (`MILESTONE-1 §3`).
