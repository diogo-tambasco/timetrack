# plan.md — Fase 23 · Controle remoto dedicado (mobile)

> Como construir a spec. Decisões (D-xx) + arquitetura + ordem de tarefas. **Frontend puro,
> aditivo, sem backend.** Reusa: `timerStore`/`useTimerClock` (03/04), `BreakOverlay` (03/05),
> `useProjects`/`useRecentEntries` (02/13), `onPresence`/store de presence (**21**), visão "Minhas
> tasks" (**18**), nav mobile (**22**), `useRole`/`useCurrentWorkspace` (01/02/10).

---

## 1. Arquitetura geral

```
Frontend (SÓ isto muda):
  src/App.tsx (existe)                       ← + <Route path="/remoto" element={<Remote/>} /> (ADITIVO)
  src/routes/cockpit/Remote.tsx     (NOVO)   ← a página /remoto: compõe os 3 blocos abaixo
     Remote.spec.tsx                (NOVO)    ← render por ramo (ocioso/rodando/pausa/viewer)
  src/components/remote/                (NOVO pasta)
     RemoteControl.tsx              (NOVO)    ← start/stop grande + descrição + projeto+recentes
     RemoteControl.spec.tsx         (NOVO)    ← consome timerStore (spy em start/stop), recentes
     RemotePresence.tsx             (NOVO)    ← lista presence do time (consome store da 21; degrada)
     RemotePresence.spec.tsx        (NOVO)    ← presença mockada; ausente/vazio → não renderiza
     RemoteShortcuts.tsx            (NOVO)    ← atalhos "Minhas tasks" (18) + "Modo Foco" (/foco)
     RemoteShortcuts.spec.tsx       (NOVO)    ← links certos; degrada sem 18
  src/lib/recentProjects.ts         (NOVO)    ← projetos recentes (puro): distinct project_id das
                                                 entries recentes, ordem do mais recente
     recentProjects.test.ts         (NOVO)    ← testes puros (dedup/ordem/sem projeto/vazio)
  src/components/nav/navItems.ts (da fase 22) ← + item { to:'/remoto', label:'Remoto', I:Icon.bolt }
                                                 (FONTE ÚNICA da 22, consumida por SideNav E nav
                                                 mobile; 1 edição cobre desktop+mobile — CA-02)
  src/components/icons.tsx (existe)           ← reusar um ícone existente (ex.: Icon.bolt/play); só
                                                 adicionar um glyph novo se nenhum servir
  e2e/remoto.spec.ts                (NOVO)    ← fluxo mobile 375px (CA-20) + presence best-effort (CA-21)

Backend: NADA. Nenhuma migração, nenhum pacote, nenhuma rota REST.
```

### D-01 — rota dedicada `/remoto` (NÃO "modo compacto" do Dashboard)

Duas opções foram pesadas:
- **(A) rota dedicada `/remoto`** — endereçável (link direto, atalho da nav mobile, futuro PWA
  `shortcut`), testável isolada, sem inflar o Dashboard com ramos `isMobile?`.
- **(B) Dashboard que vira compacto abaixo de um breakpoint** — acopla a tela enxuta ao Dashboard,
  mistura duas responsabilidades, e força media-query como lógica (frágil em teste).

**Escolhida (A).** A fase 22 já deixou o resto do app utilizável no toque; a 23 é a superfície
**enxuta por escolha do usuário**, não uma reação a viewport. `/remoto` é uma rota como as outras
(dentro de `RequireSession`→`AppShell`), então herda o guard de sessão, a `TimerBar` global e o
**único `attachTimerSync`** do shell (CA-13/RNF-03 — o remoto NÃO monta outro). A fase 22 já extraiu
os itens de nav pra a fonte única `components/nav/navItems.ts` (consumida por `SideNav` E pela nav
mobile); a 23 adiciona **UM** item `{ to:'/remoto', ... }` ali e o atalho aparece nos dois (CA-02).

### D-02 — composição da página `Remote.tsx`

```
Remote = useCurrentWorkspace() → ws; useRole(ws.id) → role; canWrite = role==='owner'||role==='member'
  <section data-testid="remote" coluna única, mobile-first>
    <RemoteControl ws={ws} canWrite={canWrite} />     // start/stop/trocar projeto + timer ao vivo
    <RemotePresence ws={ws} />                          // presence do time (degrada — CA-08)
    <RemoteShortcuts />                                 // Minhas tasks (18) + Foco (/foco)
  </section>
```

- `RemoteControl` decide o ramo pelo `useTimerStore(s => s.state)` (igual à `TimerBar`):
  - `state == null` → **IdleRemote**: campo descrição grande + seletor projeto + **recentes** + botão
    **Iniciar** (≥56px). O **Iniciar é gateado por `canWrite`** (CA-11): viewer vê "sem timer", sem o
    controle de start (start é a ÚNICA ação com 403 no backend).
  - `state.mode === 'break'` → monta **`<BreakOverlay/>`** (reuso direto — CA-05.1/CA-12) — **sempre**
    quando há pausa, **inclusive viewer** (retomar/pular/parar são per-usuário, sem 403 — CA-11).
  - rodando → **RunningRemote**: descrição read-only + chip + **`useTimerClock`** dígitos GRANDES +
    **Pausar/Parar** (≥56px). Os botões Parar/Pausar aparecem **SEMPRE que `state != null`**,
    **inclusive viewer** (o timer é per-usuário e o backend não dá 403 em stop/pause; sem isso o
    viewer ficaria com o timer preso ao trocar pro ws onde é viewer — CA-11).

### D-03 — reuso, não duplicação (CA-12/13)

- start/stop/pause/resume = **`useTimerStore().{start,stop,pause,resume}`**, chamadas com
  `void ação()` (RNF-04 — nunca rejeitam). **Zero** `fetch`/`startTimer` direto.
- dígitos = **`useTimerClock()`** com `shown = mode==='countup'?elapsed:(remaining??elapsed)` —
  copiar a expressão **exata** da `TimerBar.RunningBar`/`TimerCard.RunningCard` (não reinventar).
- pausa = **`BreakOverlay`** (o mesmo componente da `TimerBar`) — countdown/presets/retomar/pular/
  parar já estão lá; o remoto só o **posiciona** numa caixa grande.
- O remoto **NÃO** chama `attachTimerSync` (já há um no `AppShell`); só lê os stores.

## 2. Modelo de dados / migração

**Nenhum.** Sem migração (a fase 23 é frontend; `MILESTONE-2 §4/§9` fixa: 21/22/23 sem migração).
INDEX.md: registrar **23 sem migração** (frontend).

## 3. Contratos consumidos (ancorados no código real / specs das deps)

> A 23 **não define** contrato novo — consome. Citações conferidas no código (21/06) e nas specs
> 18/21/22 (irmãs nesta pasta).

### Timer (fases 03/04 — verificado em `lib/timerStore.ts`, `components/timer/TimerBar.tsx`)
- `useTimerStore`: `state: TimerState | null`, ações `start(input)`, `stop()`, `pause(min?)`,
  `resume()`, `skip()` — **todas `Promise<void>` que nunca rejeitam** (`mutate` engole, CA-17 da 05).
- `start` input = `{ workspace_id, description, project_id: string|null, task_id: string|null,
  tags: string[] }` (assinatura de `StartTimerInput`, usada pela `IdleBar`).
- `useTimerClock(): { elapsed: number; remaining: number|null }` — derivado de `server_time+offset`.
- `state.mode ∈ { 'countup','countdown','pomodoro','break' }`; `state.entry.{id,description,
  project_id}`; pausa = `mode==='break'` (sem `entry`). **Não ler `timer_states` direto** (INDEX).
- `BreakOverlay` (`components/timer/BreakOverlay.tsx`) — componente de pausa completo, reusável.

### Projetos / recentes (fases 02/13 — verificado em `lib/queries/projects.ts`, `lib/queries/entries.ts`)
- `useProjects(ws.id, opts?: { archived?: boolean })` → `Project[]` (`{ id, name, color,
  archived_at, ... }`); a `IdleBar` filtra `!archived_at` pra iniciar; o chip rodando usa
  `{ archived: true }` (resolve nome/cor de arquivado — CA-06.1/CA-07).
- `useRecentEntries(ws.id)` → entries dos últimos `RECENT_DAYS` (7) dias (`lib/queries/entries.ts`)
  — **a mesma fonte** dos "Recentes" do Dashboard. Os `project_id` distintos delas, na ordem do mais
  recente, viram a fila de **projetos recentes** (`lib/recentProjects.ts`, função PURA, testável).
- `useCurrentWorkspace()` (fase 10) → workspace ativo; `useRole(ws.id)` (fase 01/02) → role.

### Presence (fase 21 — ancorar na spec irmã `specs/21-presence/`)
- A 21 estende o socket com `type:"presence"` e (provável) um callback `onPresence` no
  `TimerSocketOptions` + um **store de presence** por workspace (espelho do `noticeStore` da 16:
  store puro, alimentado pelo `attachTimerSync`). A 23 **importa esse store/hook** e lê a lista de
  membros ativos do `ws.id` ativo. **Contrato exato = o que a 21 publicar**; o plan da 23 ancora em
  "store/hook de presence exposto pela 21, lista de `{ user_id, name, project?, ... }` por (o campo
  do payload `presence` é **`project`**, NÃO `project_name` — a 21 é dona do nome do campo no envelope)
  workspace". **Se a 21 ainda não estiver na árvore no momento do build da 23**, a 23 importa o
  módulo previsto e os specs mockam-no; a degradação (CA-08) cobre o caso vazio/ausente.
- **Snapshot inicial no mount / na troca de ws (CA-08.2):** o `presenceStore` só é semeado pelo
  snapshot (`GET /workspaces/{id}/presence`, CA-18 da 21) NO MOUNT do componente que o pede — e na
  21 isso só acontece no `PresenceStrip` do Workspace/Dashboard. Como `/remoto` pode ser o ponto de
  entrada (deep-link/PWA shortcut), o `RemotePresence` **DEVE disparar o snapshot ele mesmo** no
  mount e ao trocar o `ws.id` ativo, usando o **mesmo `useQuery(['presence', wsId])` da 21**
  (`getWorkspacePresence(ws.id)`) — não basta ler o store passivamente, senão `/remoto` a frio fica
  sem presence até chegar um delta. Isso é leitura (GET), não cria canal nem viola a fronteira CA-14.
- **A 23 NÃO toca `realtime.ts`/`timerStore.ts`/`hub`** — presence é responsabilidade da 21 (CA-14).

### Minhas tasks (fase 18 — ancorar na spec irmã `specs/18-cross-assign/`)
- A 18 cria a visão **"Minhas tasks"** (assignee = eu) com **rota própria** (`/minhas-tarefas`) +
  rotas REST (`GET /api/v1/tasks?assignee=me` ou equivalente — o contrato exato é da 18). A 23 só
  **linka** pra essa rota (`<Link>`), e mostra um badge **se** a 18 expuser uma contagem barata
  (hook/`useMyTasksCount`). Sem badge / sem a rota → o atalho some ou fica só link (CA-09).

### Nav mobile (fase 22 — ancorar na spec irmã `specs/22-mobile-responsivo/`)
- A 22 transforma o rail de 64px em **nav mobile** (bottom bar/drawer abaixo de um breakpoint). A 23
  **adiciona um item "Remoto"** nessa nav (e no `SideNav` desktop) — aditivo, não remove itens.

## 4. Libs / deps

- **Zero dependência nova** (RNF-07). Tudo é React + zustand (já no projeto) + os stores/hooks
  existentes. Sem CSS framework novo (Tailwind v4 já está); media-queries/breakpoints via classes
  Tailwind (`sm:`/`md:`) que a 22 introduziu.

## 5. Pontos de integração com o baseline

- **`App.tsx`** — 1 rota nova `/remoto` (ADITIVO; nenhuma rota existente muda — CA-01/15).
- **`components/nav/navItems.ts`** (fonte única da 22) — 1 item `{ to:'/remoto', label:'Remoto',
  I: Icon.bolt }` (ADITIVO; o `active` casa `pathname==='/remoto'`). NÃO editar `SideNav.tsx` nem a
  nav mobile direto — ambos consomem o `navItems.ts` (CA-02).
- **`timerStore`/`useTimerClock`/`BreakOverlay`** — **consumidos**, não alterados (CA-12/13).
- **store de presence da 21** — **consumido** (CA-08/14); a 21 é dona.
- **rota/contagem de "Minhas tasks" da 18** — **linkada** (CA-09); a 18 é dona.
- **`icons.tsx`** — reusar um glyph existente (ex.: `Icon.bolt`, `Icon.play`); só adicionar se
  nenhum servir (mantém o check-cores/tokens).

## 6. DEPENDÊNCIAS da fase + paralelização (PRD §9)

- **Dependências DURAS:** **22** (base responsiva — a nav mobile e os breakpoints), **21**
  (presence — "ver o que roda do time"), **18** ("minhas tasks" no bolso). Também depende
  implicitamente da **03/04/05** (timer/socket/BreakOverlay) e **13** (recentes), todas já no
  baseline.
- **PARALELIZÁVEL (worktree)?** **NÃO.** Buildar **por ÚLTIMO** (`MILESTONE-2 §9`): toca `App.tsx`
  (contende com 18) e a nav mobile (da 22), e consome os stores/rotas que 21/18 acabam de criar —
  precisa delas mergeadas. Build isolado, depois de 17–22.
- **Degradação se 21/18 ausentes:** mesmo buildada por último, a tela **degrada** (presence/minhas-
  tasks ocultos) e **start/stop/trocar-projeto seguem funcionando** (são da 03) — RNF-06. Isto é um
  guard de robustez, não uma autorização pra buildar antes das deps.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** duplicar a lógica de timer (CA-12) — start/stop/pause/resume = ações do `timerStore`;
  dígitos = `useTimerClock`; pausa = `BreakOverlay`. Copiar a **expressão `shown`** da `TimerBar`,
  não reinventar countup×remaining.
- **NÃO** montar um segundo `attachTimerSync` / WebSocket no remoto (CA-13/RNF-03) — já há **um** no
  `AppShell`; o remoto só lê os stores. Dois sockets = dois sync, condição de corrida.
- **NÃO** criar fonte alternativa de "quem está rodando" (varrer entries de outros, etc.) — presence
  vem **só** do `onPresence` da 21, que já filtra visibilidade (CA-08.1). Inventar fonte vaza projeto
  privado entre workspaces.
- **NÃO** quebrar quando 21/18 estiverem ausentes/vazias (RNF-06/CA-08/CA-09) — `?.`/guards; seção
  some, a tela segue. Os specs DEVEM testar o ramo "store de presence ausente/vazio".
- **GATEAR só o INICIAR pra viewer** (CA-11) — start é a ÚNICA ação com 403 no backend; o backend
  recusaria, a tela esconde o controle (espelha o `viewer-gating.spec` da fase 02). **NÃO** gatear
  Parar/Pausar/Retomar — são per-usuário, sem 403; esconder esses controles prenderia o timer do
  viewer (que pode ter iniciado em outro ws). Parar/Pausar aparecem sempre que `state != null`.
- **NÃO** introduzir hex literal de cor (RNF-07/check-cores) — tokens; cor de projeto vem do dado
  (como nos chips existentes).
- **NÃO** alterar o `Dashboard`/`/` (CA-15) — `/remoto` é rota nova, não um modo do Dashboard.
- **NÃO** abrir requisição de timer própria (CA-13) — sem `getTimer`/`startTimer` direto no remoto.
- **E2E:** o timer é **GLOBAL por usuário** (1 user no stub, db-test reusado) — o `remoto.spec` é
  **SERIAL** com `ensureIdle` no início (POST `/timer/stop`, 409 ok), padrão do `cronometro.spec`.
  Liberar a **:8080** de qualquer dev server alheio antes do E2E (achado da sessão 17 — o
  `playwright.config` tem `reuseExistingServer:true`).

## 8. Ordem de execução (ver tasks.md)

Função pura de recentes → controle (start/stop/trocar, reuso do store) → timer ao vivo + pausa
(BreakOverlay) → presence (degradável) → atalhos → página + rota + nav → gating viewer → E2E mobile →
verificação adversarial → gate. Commit por tarefa. `make check-full` é o gate final. (Fase pequena,
sem blocos — cabe numa sessão.)
