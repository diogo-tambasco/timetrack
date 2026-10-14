# tasks.md — Fase 23 · Controle remoto dedicado (mobile)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-11).
> **Frontend puro, aditivo, sem backend** (sem migração/rota REST/pacote Go). Cada T-XX lista os
> CAs cobertos. Consome: `timerStore`/`BreakOverlay` (03/05), `useProjects`/`useRecentEntries`
> (02/13), presence (21), minhas tasks (18), nav mobile (22). **Buildar por ÚLTIMO** (depende de
> 21/22/18 mergeadas). Fase pequena — sem blocos, cabe numa sessão.

---

- **T-01 — `lib/recentProjects.ts` (função pura) + testes** (CA-06/06.1)
  - `recentProjectIds(entries: TimeEntry[]): string[]` — `project_id` distintos das entries
    recentes, na **ordem do mais recente**, sem duplicar, ignorando `null`. Função PURA (sem React),
    espelha o estilo de `lib/recent.ts`.
  - `recentProjects.test.ts` (RED primeiro): dedup mantém a 1ª ocorrência (mais recente); `null`/
    sem projeto é ignorado; lista vazia → `[]`; ordem preservada.
  - **Acceptance:** `recentProjects.test.ts` verde.

- **T-02 — `RemoteControl` ocioso: descrição + projeto + recentes + Iniciar** (CA-04/06/06.1, RNF-04)
  - `components/remote/RemoteControl.tsx` ramo **ocioso** (`state==null`): campo de descrição grande
    (placeholder "No que você está trabalhando?"), seletor `useProjects(ws.id)` (`!archived_at`,
    "Sem projeto"), fila de **recentes** (`recentProjectIds(useRecentEntries(ws.id).data)` resolvida
    via `useProjects(ws.id,{archived:true})` p/ nome+cor — CA-06.1), botão **Iniciar** ≥56px que
    chama `void start({workspace_id:ws.id, description, project_id, task_id:null, tags:[]})` e limpa a
    descrição. Tocar um recente seleciona o projeto.
  - `RemoteControl.spec.tsx` (RED): mock de `useProjects`/`useRecentEntries`/`useTimerStore`; spy em
    `start` → tocar Iniciar chama com `project_id` selecionado; tocar chip recente muda o
    `project_id`; sem recentes/projetos → fila some, seletor "Sem projeto". (Mock FIEL a args, padrão
    `TimerBar.spec`.)
  - **Acceptance:** `RemoteControl.spec.tsx` (ramo ocioso) verde; spy de `start` recebe os args
    certos.

- **T-03 — `RemoteControl` rodando: timer ao vivo + Pausar/Parar** (CA-05/07/12/13, RNF-02)
  - Ramo **rodando**: descrição read-only + chip do projeto (`useProjects(ws.id,{archived:true})`) +
    dígitos GRANDES via `useTimerClock()` com `shown = mode==='countup'?elapsed:(remaining??elapsed)`
    (expressão idêntica à `TimerBar`); botões **Pausar** (`void pause()`) e **Parar** (`void stop()`)
    ≥56px. NENHUM `fetch`/`startTimer` direto.
  - Specs: tocar Parar chama `useTimerStore.getState().stop` (spy); dígitos refletem o store (não
    contador próprio); descrição read-only. Mutação trocando `void stop()` por fetch direto quebra.
  - **Acceptance:** `RemoteControl.spec.tsx` (ramo rodando) verde.

- **T-04 — `RemoteControl` pausa: reusa `BreakOverlay`** (CA-05.1/12)
  - Ramo `state.mode==='break'` → monta **`<BreakOverlay/>`** (mesmo componente da `TimerBar`) numa
    caixa grande. Não reescreve countdown/presets/retomar/pular/parar.
  - Spec: em `mode==='break'`, o `BreakOverlay` é renderizado (data-testid/role dele presente); o
    remoto não duplica os controles de pausa.
  - **Acceptance:** spec do ramo pausa verde; `BreakOverlay.spec` (fase 03/05) intocado e verde.

- **T-05 — `RemotePresence` (consome a 21, semeia o snapshot, degrada)** (CA-08/08.1/08.2/14, RNF-06)
  - `components/remote/RemotePresence.tsx`: **dispara o snapshot** `getWorkspacePresence(ws.id)` via o
    mesmo `useQuery(['presence', wsId])` da fase 21 **no mount E ao trocar o `ws.id` ativo** (CA-08.2
    — `/remoto` pode ser ponto de entrada; sem isso o `presenceStore` fica vazio a frio) e lê o
    **store/hook de presence da 21** (do `ws.id` ativo) pra listar "fulano: rodando <projeto>" por
    membro ativo ≠ eu. **Degradação:** store ausente/`undefined`/lista vazia → **não renderiza** (sem
    erro, sem espaço morto). NÃO cria fonte alternativa de "quem roda".
  - `RemotePresence.spec.tsx` (RED): **store vazio no mount → o componente busca o snapshot
    (`getWorkspacePresence(ws.id)`/`useQuery(['presence',wsId])`) e semeia** (mutação que removesse o
    fetch deixaria `/remoto` a frio sem presence → quebra); presence mockado com 1 membro ativo →
    mostra a linha; lista vazia → nada; hook ausente (mock retorna `undefined`) → nada, sem throw.
  - **Acceptance:** `RemotePresence.spec.tsx` verde (snapshot no mount + ramo ausente/vazio).

- **T-06 — `RemoteShortcuts` (Minhas tasks da 18 + Modo Foco)** (CA-09/10, RNF-06)
  - `components/remote/RemoteShortcuts.tsx`: `<Link>` pra **"Minhas tasks"** (rota da fase 18; badge
    de contagem se a 18 expuser hook barato — senão só link; rota ausente → atalho omitido) +
    `<Link to="/foco">` **"Modo Foco"** (sempre existe — fase 05).
  - `RemoteShortcuts.spec.tsx`: link de Foco aponta `/foco`; link de Minhas tasks aponta a rota da
    18; sem hook de contagem → sem badge, sem throw.
  - **Acceptance:** `RemoteShortcuts.spec.tsx` verde.

- **T-07 — `Remote.tsx` (página) + rota `/remoto` no `App.tsx`** (CA-01/03/11.1)
  - `routes/cockpit/Remote.tsx`: `useCurrentWorkspace()` + `useRole(ws.id)` → `canWrite`; coluna
    única mobile-first (`data-testid="remote"`) compondo `RemoteControl`/`RemotePresence`/
    `RemoteShortcuts`. `App.tsx`: `<Route path="/remoto" element={<Remote/>} />` dentro do
    `RequireSession`→`AppShell` (ADITIVO; nenhuma rota existente muda).
  - `Remote.spec.tsx`: render compõe os 3 blocos; tudo escopado ao `ws.id` ativo (mock de
    `useCurrentWorkspace`).
  - **Acceptance:** `Remote.spec.tsx` verde; `App.tsx` compila e a rota resolve.

- **T-08 — Gating de viewer: só o INICIAR é gateado** (CA-11)
  - `canWrite = role==='owner'||role==='member'`. **Gateia SÓ o Iniciar** (campo descrição/seletor/
    recentes/botão Iniciar — start é a única ação com 403 no backend). **Parar/Pausar/Retomar/Pular
    do timer JÁ RODANDO aparecem SEMPRE que `state != null`**, inclusive viewer (per-usuário, sem
    403 — não gatear, senão prende o timer do viewer iniciado em outro ws).
  - Specs (padrão `viewer-gating.spec` da fase 02), 3 casos: (a) viewer **ocioso** (`state==null`) →
    **sem** Iniciar (campo/seletor/recentes/botão ocultos); (b) viewer **com timer rodando** (state
    de outro ws) → **vê** Parar/Pausar (e no ramo pausa vê retomar/pular do `BreakOverlay`); (c)
    member/owner → veem Iniciar e Parar/Pausar. Mutação que gateasse Parar/Pausar por role (some pro
    viewer) quebra o caso (b); mutação que removesse o gate do Iniciar quebra o caso (a).
  - **Acceptance:** spec de gating verde nos 3 casos.

- **T-09 — Item de nav "Remoto" na fonte única `navItems.ts` (da 22)** (CA-02)
  - `components/nav/navItems.ts` (a FONTE ÚNICA que a fase 22 extraiu, consumida por `SideNav` E pela
    nav mobile): + item `{ to:'/remoto', label:'Remoto', I: Icon.bolt }` (ADITIVO; reusa um glyph
    existente). **NÃO** editar `SideNav.tsx` nem a nav mobile direto — uma edição em `navItems.ts`
    cobre desktop+mobile. `active` casa `pathname==='/remoto'`.
  - Specs/E2E: o item aparece (no rail desktop E na nav mobile) e navega pra `/remoto`; itens
    existentes intactos.
  - **Acceptance:** nav leva a `/remoto` (desktop e mobile); suíte da nav (22) verde.

- **T-10 — E2E `remoto.spec.ts` (mobile 375px)** (CA-20, CA-21 best-effort)
  - **SERIAL**, `ensureIdle` no início (POST `/timer/stop`, 409 ok — timer global, padrão
    `cronometro.spec`). Contexto **viewport 375×667**: `loginViaStub` → `/remoto` → coluna única
    (Iniciar visível) → criar projeto via API → selecionar na fila/seletor → **Iniciar** → ver timer
    rodando ao vivo (dígitos avançam, chip/descrição certos) → **Parar** → ocioso.
  - **CA-21 (presence) best-effort:** se semeável (2º membro via `seedWorkspaceMember` + entry
    rodando + `timer_states` via SQL, padrão `timer-guard.spec`) **e** a presence da 21 entregar
    pelo socket vivo → afirma "fulano: rodando ...". **Senão, registrar a limitação** no cabeçalho do
    spec (como o Web Push da 16 / sync passivo da 04) — presence coberta por unit (T-05).
  - Liberar a :8080 antes (achado da sessão 17). Reusar `loginViaStub`/`seedWorkspaceMember`.
  - **Acceptance:** `remoto.spec.ts` verde (fluxo CA-20; CA-21 verde se exercitável, senão limitação
    registrada).

- **T-11 — Verificação adversarial do build + gate + handoff**
  - Workflow ultracode (dimensões: **reuso vs duplicação do timer** [CA-12/13: spy prova que start/
    stop são do store, sem fetch próprio] · **degradação 21/18 ausentes** [CA-08/09/RNF-06] ·
    **gating viewer** [CA-11] · **isolamento por workspace ativo** [CA-11.1: só `ws.id`] ·
    **não-regressão** [Dashboard/TimerBar/BreakOverlay/nav verdes] · **mobile 375px** [E2E]) +
    refutação dupla. Corrigir confirmados red→green. Registrar abaixo.
  - `make check-full` VERDE (Go test + Vitest + Playwright + builds + smoke compose; o backend não
    muda, mas o gate roda inteiro). check-cores/check-pwa verdes (RNF-07).
  - Handoff (CLAUDE.md "retomar..."), INDEX (23 construída, sem migração, frontend), commit. **Deploy
    = único no fim da M2** (não por fase — espelha a M1).

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

**3 furos confirmados 2/2 aplicados** (ancorados no código real + nas specs irmãs 21/22). Os
findings refutados/descartados NÃO entram neste registro.

- **[MEDIUM] Item de nav no lugar errado (CA-02/T-09/plan §1/§5):** a fase 22 EXTRAIU os itens pra a
  fonte única `components/nav/navItems.ts` (consumida por `SideNav` E pela nav mobile); a lista não
  vive mais em `SideNav.tsx` nem numa lista mobile separada. **Correção:** adicionar UM item
  `{ to:'/remoto', label:'Remoto', I: Icon.bolt }` em `navItems.ts` (uma edição cobre desktop+mobile),
  `active = pathname==='/remoto'`. Refletido em CA-02, T-09 e plan §1 (diagrama) / §5 (integração).
- **[HIGH] Gating de viewer prendia o timer (CA-11 reescrito + T-08):** o timer é GLOBAL/por-usuário;
  no backend só `POST /timer/start` checa role (403 viewer) — stop/pause/resume/skip NÃO checam.
  Esconder Parar/Pausar do viewer travaria um timer que o backend pararia sem 403 (iniciado em outro
  ws). **Correção:** gatear SÓ o Iniciar (start) pelo `useRole` do ws ativo; Parar/Pausar/Retomar/
  Pular do timer JÁ RODANDO ficam SEMPRE disponíveis quando `state != null` (espelha a TimerBar).
  Casos testáveis (a) viewer ocioso sem Iniciar / (b) viewer com timer rodando ainda vê Parar-Pausar
  / (c) member-owner com tudo. Refletido em CA-11, T-08 e plan D-02 / §7.
- **[MEDIUM] Presence nunca semeada com `/remoto` como ponto de entrada (CA-08.2 novo + T-05 + plan
  §3):** o snapshot inicial (`GET /workspaces/{id}/presence`, CA-18 da 21) é buscado no mount do
  `PresenceStrip` (Workspace/Dashboard), NÃO em `/remoto`; abrir `/remoto` a frio (deep-link/PWA
  shortcut) ou trocar o ws ativo deixaria o `presenceStore` vazio até chegar um delta. **Correção:**
  o `RemotePresence` DEVE disparar `getWorkspacePresence(ws.id)` (mesmo `useQuery(['presence', wsId])`
  da 21) no mount E ao trocar o `ws.id` ativo — não lê o store passivamente. Refletido em CA-08.2
  (novo), T-05 (caso: store vazio no mount → busca e semeia) e plan §3 (Presence).

## Registro — BUILD + verificação adversarial do BUILD (sessão 20, 22/06)

> Build TDD T-01→T-10 (commit por tarefa, base `335c741`) · verificação adversarial · gate verde.

**Suítes finais:** Vitest **438/438** (75 arquivos; +38: recentProjects 7, RemoteControl 15, RemotePresence
8, RemoteShortcuts 3, Remote 5) · Playwright **+`remoto.spec` 2/2** (CA-20 fluxo mobile 375px + CA-02
nav) · check-cores verde · **go test INTOCADO** (frontend puro). `make check-full` **GATE VERDE**.

**Entregue (frontend puro, aditivo):** rota `/remoto` (`App.tsx` +1 Route) · `routes/cockpit/Remote.tsx`
(coluna única, `useCurrentWorkspace`+`useRole`→`canWrite`) · `components/remote/{RemoteControl,
RemotePresence,RemoteShortcuts}` · `lib/recentProjects.ts` (puro) · item `{to:'/remoto',label:'Remoto',
I:Icon.bolt}` na fonte única `navItems.ts` (cai no rail E no menu "Mais" da MobileNav — 1 edição cobre
os dois). **Reuso (CA-12/13):** start/stop/pause = ações do `useTimerStore` (`void ação()`), dígitos =
`useTimerClock()` com o `shown` IDÊNTICO à TimerBar, pausa = `<BreakOverlay/>` reusado, ZERO fetch
próprio, ZERO 2º `attachTimerSync`. **Gating viewer (CA-11):** só o Iniciar é gateado por `canWrite`;
Parar/Pausar/BreakOverlay aparecem SEMPRE que `state!=null` (inclusive viewer — não prende o timer).
**Presence (CA-08.2):** `RemotePresence` semeia o snapshot (`useQuery(['presence',wsId])`+`seed`) no
mount E ao trocar ws, filtra o próprio user, degrada (vazio→null). Specs antigas: só ajuste de seletor/
contagem (navItems 8→9, MobileNav menu "Mais", shell `.tt-nav` 9→10) — comportamento intacto.

**Verificação adversarial do build (T-11, orquestrador):** workflow ultracode, 7 dimensões × refutação
dupla (reuso-timer · gating-viewer · presence-correção · isolamento-ws · não-regressão · recent-pureza/
degradação · mobile-e2e/cor), ancorado no diff real `335c741..HEAD` → **0 findings** (build limpo;
todos os 7 finders retornaram lista vazia). Spot-check manual do orquestrador (com os próprios olhos)
nos 2 pontos mais sutis — CA-11 gating (RemoteControl: `break`→BreakOverlay sempre; `if(state)`→Running
sem checar canWrite; ocioso→`canWrite?Idle:ViewerIdle`) e CA-08.2 seed (RemotePresence: useQuery+seed no
mount/troca-ws, filtra self, degrada) — **ambos corretos**. CA-21 (presence de colega no E2E) registrado
como limitação (best-effort por design; presence coberta por unit). **Nenhuma correção de LÓGICA necessária.**

**Conserto de estabilidade do GATE (cross-arquivo, introduzido pela 23):** o `remoto.spec` é o
**segundo** spec a MUTAR o timer global (o `cronometro.spec` era o único). Sob `fullyParallel` os dois
rodavam concorrentes em workers distintos → o `ensureIdle`/`Parar` do `remoto.spec` apagava o timer que
o `cronometro.spec` (CA-37) acabara de iniciar pra testar o reload (1ª run do gate caiu aí). **Fix:**
mutex cross-arquivo `e2e/timerLock.ts` (`mkdir` atômico + rouba lock obsoleto de worker morto); os DOIS
specs adquirem no `beforeAll`/liberam no `afterAll`, serializando os dois arquivos entre si (resto da
suíte segue paralelo). Validado: `cronometro+remoto` com `--workers=2` (força o paralelismo) — CA-37
verde em todas as runs. (Precedente: a fase 21 já fez conserto de estabilidade cross-fase no
`cronometro.spec` — `focus-day-total pointer-events:none`.) **Nota:** o CA-21/fase-15 (FocoPicker)
segue um flake PRÉ-EXISTENTE raro sob carga (acumulação no db-test reusado); reiniciar o db-test (tmpfs)
antes do gate o evita — `check-full` GATE VERDE confirmado.

> Decisão herdada da spec (sessão 19, registrada): a fila de recentes resolve nome+cor via
> `useProjects(ws.id,{archived:true})` (CA-06.1 — entry recente pode apontar pra arquivado; id sem
> projeto resolvível é OMITIDO da fila). `useRecentEntries` devolve `{entries, total_seconds}` (não
> array) — o `RemoteControl` lê `.data?.entries` (pego pelo `tsc`, commit de fix dedicado no build).
