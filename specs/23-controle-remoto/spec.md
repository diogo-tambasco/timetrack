# spec.md — Fase 23 · Controle remoto dedicado (mobile)

> Fonte de verdade da fase. Resolve a dor **P7** do `MILESTONE-2.md §2/§4` (REQUISITOS §4.7 /
> DESIGN §3.3): no celular o Diogo quer **só o essencial** — **start/stop + trocar projeto + ver o
> que está rodando** (o próprio timer ao vivo + a presence do time da fase 21), numa superfície
> mobile-first otimizada pra toque rápido, com os atalhos do dia ("minhas tasks" da fase 18 e o
> Modo Foco) à mão.
> **Decisões travadas (HITL `MILESTONE-2.md §7.7`):**
> 1. **Mobile = híbrido:** fase 22 = base responsiva; fase 23 = **superfície dedicada enxuta**
>    (controle remoto). Esta fase é a 23.
> 2. **ZERO migração, ZERO backend novo.** A superfície **consome** o que já existe: `timerStore`
>    (timer da 03/04), `onPresence` (presence por workspace da 21), "Minhas tasks" (da 18). Não
>    reimplementa nada — **reusa**.
> 3. **Buildar por ÚLTIMO.** Dependências DURAS: **22** (base responsiva), **21** (presence),
>    **18** (minhas tasks). Não-paralelizável.
> 4. **Toque-first, zero densidade de cockpit.** Botões grandes, alvos generosos, uma coluna.

---

## 1. Objetivo

Entregar a **rota `/remoto`** — uma superfície dedicada mobile-first que cabe no bolso e faz o que o
Diogo precisa no celular sem abrir o cockpit denso:

1. **Start/stop** com botões grandes — consome **as mesmas ações do `timerStore`** que a `TimerBar`
   usa (`start`/`stop`/`pause`/`resume`), que **nunca rejeitam** (CA-17 herdado da fase 05): nenhuma
   lógica de timer é duplicada.
2. **Trocar projeto rápido** — seletor de projeto + **projetos recentes** (toque único) pra iniciar
   um timer já com o projeto certo.
3. **O que está rodando agora** — o **meu timer ao vivo** (dígitos derivados de `server_time+offset`,
   exatamente como o resto do app) + a **presence do time** (quem está ativo, vinda do `onPresence`
   da fase 21) — com **degradação graciosa** se a 21 não estiver montada/ligada.
4. **Atalhos** — "Minhas tasks" (visão da fase 18) e **Modo Foco** (rota `/foco` da fase 05) num
   toque.

A superfície é **só frontend, aditiva**: novas rotas REST = **zero**; o timer, a presence e as
tasks já têm contrato. As fases 03/04/05/18/21/22 seguem intocadas no contrato (RNF-05).

## 2. Definições precisas

- **Superfície remota** = a rota `/remoto` + seus componentes (`RemoteControl`, `RemotePresence`,
  `RemoteShortcuts`). Vive **dentro do shell autenticado** (`RequireSession` → `AppShell`), como as
  outras rotas de `App.tsx`; o `<Outlet/>` renderiza o conteúdo. **Decisão (D-01 no plan):** rota
  **dedicada `/remoto`**, NÃO um "modo compacto" do Dashboard condicionado a media-query — uma rota
  explícita é endereçável (link/atalho/PWA shortcut), testável isoladamente, e não infla o Dashboard
  com ramos `mobile?`. A fase 22 já tornou o resto responsivo; a 23 é a tela enxuta **por escolha**,
  não por viewport.
- **"Rodando ao vivo"** = o estado do `useTimerStore` (`state` + `useTimerClock`), a **mesma fonte**
  da `TimerBar`/`TimerCard`. O remoto **reflete**, nunca abre requisição própria de timer (CA-13).
- **Presence** (fase 21) = o conjunto de membros do workspace ativo com timer rodando AGORA,
  entregue pelo callback **`onPresence`** do socket (envelope `type:"presence"` — definido pela 21).
  O remoto **consome** o store/hook que a 21 expõe; se a 21 não estiver presente, a seção some
  (CA-08).
- **"Minhas tasks"** (fase 18) = a visão `assignee = eu`; o remoto **linka** pra ela (não a
  reimplementa) e, se houver contrato de contagem barato, mostra um badge (CA-09 — degrada sem ele).
- **Role / escrita** = a matriz de `MILESTONE-2.md §8`. No workspace ativo: **owner/member** podem
  start/stop/trocar projeto; **viewer** é **somente leitura** na superfície (CA-11), exceto o
  carve-out já coberto pelo backend (viewer-assignee age na task dele — fora do escopo desta tela,
  que é cronômetro pessoal). A `useRole(ws.id)` (fase 01/02) é a fonte.

## 3. User stories

- **US-01** — Como Diogo no celular, quero **uma tela enxuta** com um botão grande de start/stop,
  pra começar/parar o timer rápido sem caçar a barrinha do cockpit.
- **US-02** — Como Diogo no celular, quero **trocar de projeto num toque** (lista de recentes +
  seletor), pra cronometrar já no projeto certo sem digitar.
- **US-03** — Como Diogo no celular, quero **ver o que está rodando** — o meu timer correndo ao
  vivo e **quem do time está ativo agora** (presence) — pra ter o pulso do dia sem abrir o cockpit.
- **US-04** — Como Diogo no celular, quero **atalhos** pras "Minhas tasks" e pro **Modo Foco**, pra
  ir direto ao que importa.
- **US-05** — Como viewer de um workspace, quero a superfície remota **somente leitura** (vejo o que
  roda, não inicio/paro nem atribuo), respeitando minha permissão.

## 4. Critérios de aceite (EARS — cada um vira teste)

> **Sem migração, sem rota REST nova.** Todos os CAs são de **frontend** (Vitest jsdom + Playwright),
> exceto os de não-regressão (provam que o baseline backend/contrato não muda).

### Rota & shell (estrutura)

- **CA-01** (rota dedicada) — `App.tsx` DEVE registrar `<Route path="/remoto" element={<Remote/>} />`
  **dentro** do bloco `RequireSession`→`AppShell` (mesmo nível das demais rotas do cockpit). Sem
  sessão, `/remoto` cai em `/login` pelo `RequireSession` existente (CA herdado da 01) — a fase NÃO
  adiciona guard novo.
- **CA-02** (entrada na nav) — a fase 22 EXTRAIU os itens de nav pra a **fonte única**
  `components/nav/navItems.ts` (consumida por `SideNav` E pela nav mobile da 22). A 23 DEVE
  adicionar **UM** item `{ to:'/remoto', label:'Remoto', I: Icon.bolt }` nesse `navItems.ts`
  (ADITIVO; não remove os itens existentes) — com isso `/remoto` aparece automaticamente no
  `SideNav` desktop E na nav mobile, sem editar nenhum dos dois componentes nem manter listas
  separadas. O `active` casa `pathname === '/remoto'` (mesmo critério dos demais itens).
- **CA-03** (mobile-first, uma coluna) — a superfície DEVE renderizar em **uma coluna** com alvos de
  toque grandes (botões de start/stop ≥ 56px de altura), legível e usável em **viewport 375px**
  (E2E — CA-20). Não usa a grade 2-colunas do Dashboard (densidade de cockpit é proibida — §1.4).

### Start / stop / pausa (consome o `timerStore` — NÃO duplica lógica)

- **CA-04** (start) — QUANDO ocioso (`state == null`) e o usuário toca **"Iniciar"**, a superfície
  DEVE chamar `useTimerStore().start({ workspace_id: ws.id, description, project_id, task_id: null,
  tags: [] })` — a **mesma ação** que a `IdleBar` da `TimerBar` usa. `description` vem de um campo
  grande (placeholder "No que você está trabalhando?"); `project_id` do seletor/recentes (CA-06).
  Após o start, o campo de descrição é limpo (espelha a `IdleBar`).
- **CA-05** (stop / pausa) — QUANDO rodando (`state.mode` de trabalho), os botões **Parar** e
  **Pausar** DEVEM chamar `useTimerStore().stop()` e `.pause()` respectivamente — as mesmas ações da
  `RunningBar`. NENHUMA chamada REST direta de timer é feita pela superfície (toda mutação passa pelo
  store — mutação removendo o `void stop()`/`void start()` e pondo `fetch` direto deve quebrar
  teste).
- **CA-05.1** (pausa → BreakOverlay) — QUANDO `state.mode === 'break'`, a superfície DEVE mostrar um
  **controle de pausa** equivalente ao `BreakOverlay` (retomar/pular/parar) **reusando o componente
  `BreakOverlay`** (não reescreve countdown/presets). O remoto monta o mesmo `BreakOverlay` da
  `TimerBar` — comportamento idêntico (CA-12).
- **CA-06** (trocar projeto rápido) — A superfície DEVE oferecer:
  (a) um **seletor de projeto** (`useProjects(ws.id)` filtrando `!archived_at`, igual à `IdleBar`),
  e (b) uma fila de **projetos recentes** (derivada do mesmo dado de `useRecentEntries(ws.id)` que o
  Dashboard usa — os `project_id` distintos das entries recentes, na ordem do mais recente, sem
  duplicar). Tocar um projeto recente o **seleciona** (vira o `project_id` do próximo start). Sem
  entries recentes / sem projetos → a fila some, o seletor mostra "Sem projeto" (degrada).
- **CA-06.1** (recentes só com nome/cor) — cada chip de projeto recente DEVE resolver **nome + cor**
  via `useProjects(ws.id, { archived: true })` (inclui arquivado — o mesmo motivo do chip da
  `RunningBar`/`TimerCard`: a entry recente pode apontar pra projeto arquivado e o chip não pode
  sumir). `project_id` sem projeto resolvível é omitido da fila (não quebra).

### Timer ao vivo (deriva do store/offset — igual ao resto)

- **CA-07** (dígitos ao vivo) — QUANDO rodando, a superfície DEVE exibir os dígitos do tempo
  derivados de **`useTimerClock()`** (`elapsed`/`remaining` de `server_time+offset`), com
  `shown = state.mode === 'countup' ? elapsed : (remaining ?? elapsed)` — **exatamente** a regra da
  `TimerBar`/`TimerCard`. NÃO acumula tempo localmente como verdade (RNF-02 herdado). Mostra também
  a descrição (read-only — editar entry rodando é 409) e o chip do projeto rodando (archived:true).
- **CA-08** (presence do time, degradável) — QUANDO a fase 21 está montada e o `onPresence` entrega
  membros ativos do workspace, a superfície DEVE listar **"fulano: rodando <projeto>"** por membro
  ativo (≠ eu), ao vivo. **Degradação (DURA):** se o store/hook de presence da 21 **não existe** ou
  retorna **lista vazia/`undefined`**, a seção de presence **não renderiza** (nenhum erro, nenhum
  espaço morto) — a superfície segue funcional só com o meu timer. (A 23 não cria o canal de
  presence; consome o que a 21 expõe; ausência = oculto.)
- **CA-08.2** (semear o snapshot no mount / na troca de ws) — o snapshot inicial de presence
  (`GET /workspaces/{id}/presence`, CA-18 da 21) é buscado **no mount do componente que o pede**; o
  `PresenceStrip` da 21 só vive no Workspace/Dashboard. Como `/remoto` pode ser **ponto de entrada**
  (deep-link / PWA shortcut), o `presenceStore` estaria **vazio** ao abrir `/remoto` a frio (e ao
  trocar o ws ativo — CA-11.1 — o re-fetch também não aconteceria). Portanto o **`RemotePresence`
  DEVE disparar o snapshot** `getWorkspacePresence(ws.id)` **no mount E ao trocar o `ws.id` ativo**
  — usando o **mesmo `useQuery(['presence', wsId])` da fase 21** (não lê o store passivamente). Caso
  testável: presence store vazio no mount → o `RemotePresence` busca o snapshot e semeia (mutação que
  removesse o fetch e só lesse o store deixaria `/remoto` a frio sem presence → deve quebrar teste).
- **CA-08.1** (presence respeita visibilidade) — a superfície NÃO inventa visibilidade: ela mostra
  **exatamente** o que o `onPresence` da 21 entregar (a 21 já filtra projeto privado — `MILESTONE-2
  §4 fase 21`). Um membro ativo num projeto privado não compartilhado **não chega** ao remoto porque
  não chega ao `onPresence` — a 23 não re-filtra nem re-expõe (mutação que adicionasse uma fonte
  alternativa de "quem está rodando" — ex.: varrer entries de outros — deve quebrar teste / é
  proibida).

### Atalhos

- **CA-09** (Minhas tasks) — A superfície DEVE ter um atalho **"Minhas tasks"** que navega pra a
  visão da fase 18 (rota `/minhas-tarefas`, que a 18 define — o plan ancora no contrato da 18).
  Se a 18 expuser uma contagem barata de tasks atribuídas a mim, mostra um badge; **senão, só o
  link** (degrada sem badge). Se a 18 **não estiver presente** (rota inexistente), o atalho é
  **omitido** (sem link morto) — análogo à degradação de presence (CA-08).
- **CA-10** (Modo Foco) — A superfície DEVE ter um atalho **"Modo Foco"** que navega pra `/foco`
  (rota da fase 05, que sempre existe no baseline) — `<Link to="/foco">`, igual ao card do
  Dashboard.

### Permissões (matriz §8)

- **CA-11** (viewer = só o INICIAR é gateado) — O timer é **por-usuário**; no backend **só
  `POST /timer/start` checa role** (403 viewer, contra o `workspace_id` do payload). `stop`/`pause`/
  `resume`/`skip` **NÃO checam role** (espelham a `TimerBar`, que não gateia parar/pausar). Logo:
  - **Iniciar (start)** é gateado pelo `useRole(ws.id)` do ws **ativo**: QUANDO `role === 'viewer'`,
    a superfície DEVE **ocultar** o controle de Iniciar (campo descrição/seletor/recentes/botão
    Iniciar — tudo que cria entry). O backend recusaria com 403; a tela não oferece a ação.
  - **Parar/Pausar/Retomar/Pular** do timer **JÁ RODANDO** ficam **SEMPRE disponíveis** quando
    `state != null` — inclusive pra viewer no ws ativo —, porque o timer é per-usuário e o backend
    não dá 403 nesses caminhos. Sem isso, um viewer que iniciou o timer em OUTRO workspace ficaria
    com o timer **preso** (não conseguiria parar/pausar) ao trocar pro ws onde é viewer.
  - **Casos testáveis:** (a) viewer no ws ativo **ocioso** (`state == null`) → **não vê** Iniciar
    (campo/seletor/recentes/botão ocultos); (b) viewer no ws ativo **com timer rodando** (iniciado em
    outro ws) → **vê** Parar/Pausar (e o ramo pausa vê retomar/pular do `BreakOverlay`); (c) owner/
    member → veem Iniciar e Parar/Pausar normalmente. (Espelha o gating `useRole` da fase 02 só pro
    Iniciar, e a `TimerBar`, que nunca gateia stop/pause por role.)
- **CA-11.1** (sem vazamento de workspace) — a superfície opera **exclusivamente** sobre o
  **workspace ativo** (`useCurrentWorkspace()` — fase 10). Projetos/recentes/role/presence são todos
  do `ws.id` ativo. Trocar o workspace ativo (seletor da fase 10) re-resolve tudo. NUNCA mostra
  projeto/entry/presence de outro workspace (o filtro vem das queries existentes, todas por
  `ws.id` — herdado; mutação que removesse o `ws.id` de `useProjects`/`useRecentEntries` quebraria os
  testes dessas fases, e o remoto não introduz fonte sem `ws.id`).

### Não-regressão (reuso, não duplicação)

- **CA-12** (comportamento idêntico à TimerBar) — a superfície **NÃO duplica** a lógica de timer:
  start/stop/pause/resume são as ações do `timerStore`; os dígitos derivam de `useTimerClock`; a
  pausa reusa `BreakOverlay`. Um teste DEVE provar que tocar "Parar" no remoto chama exatamente
  `useTimerStore.getState().stop` (spy) e que os dígitos refletem o store (não um contador próprio).
  As suítes da `TimerBar`/`timerStore`/`BreakOverlay` (fases 03/04/05) seguem **verdes sem editar
  asserções**.
- **CA-13** (sem requisição de timer própria) — a superfície NÃO chama `getTimer`/`startTimer`/etc.
  diretamente nem monta um segundo `attachTimerSync` (já existe **um** no `AppShell`); ela só lê
  `useTimerStore`/`useTimerClock` e dispara as ações. (Dois `attachTimerSync` = dois sockets/intervals
  — proibido; mutação que chamasse `attachTimerSync` na montagem do remoto deve quebrar teste.)
- **CA-14** (presence aditivo na 21, não na 23) — a 23 NÃO altera `realtime.ts`/`timerStore.ts`/`hub`
  — ela **consome** o `onPresence`/store que a **fase 21** adicionou. Se a 21 introduziu um
  `presenceStore`/hook, a 23 importa e lê; não recria o canal. (Garante a fronteira: a 21 é dona do
  presence; a 23 é dona da TELA.) Os arquivos compartilhados que a 23 toca são **`App.tsx`** (1 rota
  nova) e a **nav mobile da 22** (1 item) — listados e afirmados aditivos.
- **CA-15** (Dashboard intocado) — o `Dashboard`/`/` da fase 13 **não muda** — a superfície remota é
  uma **rota nova**, não um modo do Dashboard. A `dashboard.spec`/`Dashboard.spec` seguem verdes.

### E2E (Playwright — mobile)

- **CA-20** (fluxo remoto em 375px) — Logado (stub), num contexto de **viewport 375×667**: abrir
  `/remoto` → a superfície renderiza em uma coluna (botão Iniciar grande visível) → **selecionar um
  projeto** (criado/semeado no teste) na fila/seletor → **Iniciar** → ver o **timer rodando ao vivo**
  (dígitos avançam, descrição/chip do projeto certo) → **Parar** → volta a ocioso. (Mesmo padrão do
  `cronometro.spec` — serial, `ensureIdle` no começo: o timer é GLOBAL por usuário; ver TDD.)
- **CA-21** (presence de um colega, se semeável) — se um **segundo membro com timer rodando** for
  semeável (membership via `seedWorkspaceMember` + entry rodando + `timer_states` via SQL, padrão do
  `timer-guard.spec`/`core-projetos.spec`) **e a presence da 21 entregar pelo socket**, o E2E afirma
  que o remoto mostra "fulano: rodando ...". **Se não for exercitável headless** (presence depende do
  broadcast por workspace vivo, que a 21 prova nos próprios testes), o teste **registra a limitação**
  (comentário no spec, como o Web Push da fase 16 / o sync passivo da fase 04) e cobre presence por
  **teste de unidade** (CA-08, jsdom, com o store de presence mockado). O fluxo do CA-20 é o E2E
  obrigatório; o CA-21 é best-effort.

## 5. Requisitos não-funcionais

- **RNF-01 (zero backend)** — nenhuma rota REST nova, nenhuma migração, nenhum pacote Go. A fase é
  **frontend puro** que consome contratos existentes (timer 03/04, presence 21, tasks 18). O `make
  check-full` roda os mesmos backends.
- **RNF-02 (relógio derivado)** — os dígitos vivos derivam de `server_time + offset` via
  `useTimerClock` (não acumulam localmente) — herdado do `timerStore`. O remoto não introduz tick
  próprio.
- **RNF-03 (um socket só)** — a superfície NÃO abre um segundo WebSocket / `attachTimerSync` (CA-13):
  o `AppShell` já mantém o único socket vivo; o remoto lê os stores alimentados por ele.
- **RNF-04 (ações nunca rejeitam)** — as ações do `timerStore` (start/stop/pause/resume) **nunca
  rejeitam** (CA-17 da fase 05 — `mutate` engole tudo, falha → `load()`); a superfície chama
  `void ação()` (fire-and-forget seguro), igual à `TimerBar`/`TimerCard`.
- **RNF-05 (não-regressão)** — fases 03 (timer), 04 (socket/hub), 05 (foco/BreakOverlay), 13
  (Dashboard), 18 (tasks), 21 (presence), 22 (responsivo) **intocadas no contrato**: a 23 é
  **aditiva** (1 rota, 1 item de nav, componentes novos). Suítes antigas verdes sem editar asserções.
- **RNF-06 (degradação graciosa)** — sem a fase 21 (presence ausente) **ou** sem a 18 (minhas tasks
  ausente), a superfície **degrada** ocultando essas seções — **start/stop/trocar-projeto seguem
  funcionando** (são da 03, que sempre existe). Isto é um requisito **explícito** (a 23 buildada por
  último num cenário onde 21/18 já existem; mas a tela não pode dar erro se um store estiver vazio).
- **RNF-07 (tokens/DESIGN)** — a superfície usa os tokens do `DESIGN.md`/`tokens.css` (sem hex
  literal — `check-cores` verde; cores de projeto vêm do dado, como nos chips existentes); mantém a
  linguagem visual (controle remoto = DESIGN §3.3). **Zero dependência JS nova.**

## 6. Pré-condições externas

- **Nenhuma.** Não há credencial, chave nem publicação envolvida (sem backend, sem push novo). A
  fase é frontend que consome o que já está no ar.

## 7. Fora de escopo (fronteiras explícitas)

- **Criar o canal de presence** — é da **fase 21**; a 23 só **consome** o `onPresence`/store que a 21
  expôs. Se a 21 não existir, presence fica oculto (CA-08), a 23 não o inventa.
- **A visão "Minhas tasks" / "Pedidas por mim"** em si — é da **fase 18**; a 23 só **linka** (e mostra
  badge se houver contrato barato). A 23 não reimplementa a lista nem a query cross-workspace.
- **Base responsiva do resto do app** (Dashboard/Relatórios/Board/etc.) — é da **fase 22**. A 23 é
  **só** a superfície dedicada `/remoto`.
- **Countdown/Pomodoro/seleção de modo** pela superfície — o remoto inicia **count up** (start
  simples, como a `IdleBar`); modos com alvo têm UI própria (Foco/Ajustes). A pausa Pomodoro que
  surge é controlada pelo `BreakOverlay` reusado (CA-05.1), mas a superfície não oferece "iniciar
  Pomodoro" (atalho pro `/foco` cobre isso).
- **Edição de entry / manual entry / relatórios** no remoto — a tela é **start/stop/trocar/ver**; o
  resto fica no cockpit (com a 22 responsiva).
- **Telas mobile-nativas além do remoto** (board mobile, etc.) — fora da M2 inteira (`§3` do PRD).
- **Push/notificação nova** — o canal `notice`/push da fase 16 já existe; a 23 não adiciona avisos.
