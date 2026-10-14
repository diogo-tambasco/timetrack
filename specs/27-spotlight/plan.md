# plan.md — Fase 27 · Spotlight (Cmd+K)

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + componentes + assinaturas +
> ordem de tarefas. **FRONTEND + 1 ALLOWANCE MÍNIMA DE BACKEND — ZERO migração** (RNF-01): orquestra
> fluxos já construídos (24/26/03/12) e adiciona **uma única rota** `PATCH /api/v1/timer` (edição de
> rótulo/projeto/tags da entry rodando, decisão HITL 23/06 — §3.5/D-10). Nada aqui é código — é o
> contrato.

---

## 1. Arquitetura geral

```
Backend (allowance ÚNICA da decisão HITL 23/06 — §3.5):

  internal/timer/service.go        (EDITA)  ← método novo EditCurrent(ctx, userID, in):
      EditCurrentInput{Description, ProjectIDSet/ProjectID, Tags}   carrega a entry RODANDO sob o
      → loadStateRow FOR UPDATE (advisory lock)                     lock; edita SÓ desc/project/tags;
      → entries.ValidateRefs(workspaceID da entry, projectID)       ValidateRefs no ws da entry;
      → entries.Update (desc/project_id/tags; timing INTOCADO)      dispara s.onChange(State novo)
      → workState(...) + s.onChange(userID, st, now)                ⇒ broadcast da fase 04 propaga

  internal/api/timer_handlers.go   (EDITA)  ← handleTimerEdit (PATCH): decode → svc.EditCurrent → Result
  internal/api/timer_routes.go     (EDITA)  ← tr.Patch("/", handleTimerEdit(svc))  (rota por-usuário)

Frontend:

  lib/spotlight.ts                 (NOVO)  ← lógica PURA, testável sem DOM:
      SPOT (lista de comandos)              parseSpot(input) → { act|null, title }
      spotItems(input)                      (filtro por token — espelha mockup l.633-634)
      spotTitle(input)                      (título sem o comando — espelha mockup l.636)
      → SEM React, SEM fetch: só strings (o coração do red→green)

  lib/spotlightStore.ts            (NOVO)  ← zustand minúsculo: { open, query }
      open()/close()/setQuery()             (estado do popup; reaproveitável pelo atalho e botão)
      (decisão H: stores vivem em src/lib/ — junto de timerStore.ts/presenceStore.ts/noticeStore.ts;
       NÃO criar src/stores/)

  components/spotlight/
      Spotlight.tsx                (NOVO)  ← conteúdo do Spotlight DENTRO do componente de popup
                                            central da fase 24 (decisão A); input + lista
                                            (.spotitem) + chips de tecla; teclado
      SpotlightButton.tsx          (NOVO)  ← botão ⌘K no shell (desktop) + acesso mobile
      useSpotlightHotkey.ts        (NOVO)  ← registra Cmd/Ctrl+K no document (1×, cleanup)

  lib/spotlightActions.ts          (NOVO)  ← runSpot(act, ctx): chama os fluxos EXISTENTES
      /task  → createTask(ws, projectId, {name}) → abridor de task da 24 (foco no título) (fase 24)
      /note  → createNote(ws, {project_id, title}) → abridor de nota da 26                 (fase 26)
      /timer → useTimerStore.getState().start({mode:'countup', description, project_id})   (03)
      /goal  → createObjective(ws, projectId, {name})                          (fase 12)
      sem cmd, rodando → useTimerStore.getState().editCurrent({description, project_id})   (allowance 23/06 — 27-H1)
      sem cmd, ocioso  → start({mode:'countup', description, project_id})      (03)

  routes/AppShell.tsx              (EDITA)  ← monta <Spotlight/> + useSpotlightHotkey + <SpotlightButton/>
  components/nav/navItems.ts       (EDITA?) ← opcional: item "Captura"/⌘K no menu "Mais" mobile (D-06)
```

**Princípio (D-01):** o **núcleo é puro** (`lib/spotlight.ts` — parse/filtro/título sem DOM),
testado por Vitest unitário direto (igual `lib/calendar-layout.ts` da fase 11 e `lib/recent.ts` da
13). A **orquestração** (`spotlightActions.ts`) é uma casca fina que delega a fluxos existentes; os
testes dela mockam `api`/`timerStore`/`openTask`/`openNote` e afirmam **qual** foi chamado com
**quais** args. O componente `Spotlight.tsx` é testado em jsdom (teclado, filtro, foco).

## 2. Migração

**NENHUMA.** A fase não cria/altera tabela nem enum (RNF-01). Esta seção existe só pra firmar:
`backend/internal/db/migrations/` fica **inalterado** (o último é `0020_reminders`; a próxima
migração da M3 é da fase 24, `0021`, fora desta fase). Um teste/asserção do plan registra a
intenção, mas não há SQL.

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Núcleo puro `lib/spotlight.ts`** (parse/filtro/título), separado da UI e das ações | Testável sem DOM nem fetch (red→green limpo); espelha 1:1 `spotItems`/`spotTitle` do mockup (l.633-636). O resto (componente, ações) consome funções já provadas. |
| D-02 | **Estado do Spotlight num `spotlightStore` (zustand)** `{ open, query }` — NÃO um `panel` global novo | O store é alcançável pelo atalho (registrado no shell), pelo botão e pelos testes, sem prop-drilling. **Reusa o padrão `useTimerStore`/`presenceStore`.** O popup-central de task/nota (24/26) tem o **seu** estado; o Spotlight tem o dele — ao rotear pra um editor, o Spotlight fecha e o editor abre (CA-05). (Se 24/26 já expuserem um único "panel manager", o Spotlight pode entrar como mais um `type`; ver D-09 — o store é o fallback robusto que NÃO depende de como 24/26 nomearam o painel.) |
| D-03 | **Atalho global registrado UMA vez no `AppShell`** via `useSpotlightHotkey` (effect com cleanup) | Espelha o `document.addEventListener('keydown', …)` do mockup (l.676-679), mas em React idiomático (sem listener vazado — CA-01). O shell é o ponto único montado em toda rota autenticada. |
| D-04 | **`preventDefault()` no `Cmd/Ctrl+K`** | O atalho nativo de alguns navegadores (foco na barra de busca) seria roubado; o PRD/mockup querem a captura (CA-01). |
| D-05 | **Comandos = `/task` `/note` `/timer` `/goal` (4, fixos do PRD)**; parse pelo **primeiro token** sem barra | Espelha `SPOT`/`spotItems` (l.625-634). Mais comandos = fora (§6 da spec). |
| D-06 | **Acesso mobile = botão no shell + (opcional) item no menu "Mais" da `MobileNav`** via `navItems.ts` | A M2 fez `navItems.ts` fonte única; um botão de **ação** (abre o store, não navega) cabe melhor como botão no shell que como `NavItem` (que é `{to,label,I}` — uma ROTA). **Decisão:** botão dedicado no shell (visível desktop+mobile); se o menu "Mais" precisar do acesso, entra como um item especial que dispara `open()` (não um `<Link>`). Não forçar um `to` falso. |
| D-07 | **`/task`/`/note`/`/goal` exigem projeto de contexto** (`/projetos/:id`); sem contexto ⇒ dica à-prova-de-culpa (CA-15), **sem** Inbox default | YAGNI/anti-NOA: criar um projeto "Inbox" é decisão de produto não fechada; `/timer` e count-up cobrem a captura sem-projeto. Registrado como fronteira. |
| D-08 | **`/timer` e sem-comando(ocioso) reusam `useTimerStore.getState().start`** — NUNCA chamam a API de timer direto | Não duplicar lógica de timer (RNF-04); o store já tem o anti-stale, auto-transição e "ações nunca rejeitam" (fase 05 CA-17). O Spotlight só chama `start(...)`. (Sem-comando + timer rodando = `editCurrent(...)` — 27-H1/D-10; via store, não API direto.) |
| D-09 | **Reuso do popup central = o componente de popup central da fase 24 (decisão A)**, acionado pelo **abridor** que a 24 expõe (handler do shell ou estado de painel) | Decisão de coordenação A: o popup central é um **componente React** da 24 (ex.: `CenterPanel`, chrome side↔full, tokens.css). O Spotlight **não** renderiza o editor de task/nota — ele aciona o **mesmo estado de painel do shell** que a 24 usa. **A 24 deve EXPOR um abridor** (provável handler do shell `openTask(id, focus)`/`openNote(id)`, ou um estado de painel `panel:{type,id}` no shell que o Spotlight também aciona) — **esse contrato exato é fixado no plan da fase 24, não deixado 100% pro "confirmar no build" da 27**. **T-04 confirma o nome/assinatura reais.** **Degradação (CA-17):** se o abridor não existir no app (build fora de ordem), `runSpot` cria a entidade e navega pra `/projetos/{id}` (fallback) sem quebrar. |
| D-10 | **Sem-comando + timer rodando (CA-13) = EDITA o timer atual via allowance `PATCH /api/v1/timer`** (decisão HITL 23/06) — 27-H1 | O `entries.Patch` rejeita PATCH de entry rodando (`409 entry_running` — `service.go:184-187`) e a TimerBar deixa a descrição read-only pelo mesmo motivo (`TimerBar.tsx:156-157,176`). A decisão de 23/06 **superou** o no-op: a 27 ganha uma **allowance mínima de backend** — uma rota por-usuário que edita SÓ desc/project_id/tags da entry rodando, sob o advisory lock do timer, validando `project_id` via `ValidateRefs` e disparando `OnChange` (broadcast da 04 propaga o rótulo ao vivo). **Escolha (a) sobre (b) — ver §3.5.** Timing intocado (CA-13.1); o `entries.Patch` segue dando `409` (não se relaxa). |
| D-13.5 | **Contrato escolhido = (a) `PATCH /api/v1/timer` roteado pelo `timer.Service` — NÃO (b) relaxar `entries.Patch`** | §3.5: (a) é mais limpo e dispara o broadcast naturalmente; (b) espalharia o `OnChange` num service que hoje não o tem (`entries.Service` não conhece o hub) e teria de detectar "esta entry é a do timer". |
| D-11 | **Sem-comando + ocioso = count-up** (CA-14); input vazio + sem-comando = **no-op** | Espelha a IdleBar (RF-19 "sem timer rodando"); não criar entry vazia é higiene (não polui o histórico). |
| D-12 | **Scrim/card vêm do componente de popup central da 24 (decisão A)**; a 27 só declara classes **próprias de conteúdo** (ex.: `.spotitem`) no CSS global, **só com `var(--*)`** | Decisão A: NÃO referenciar/reusar `.mscrim`/`.scard`/`.swrap` do mockup como contrato — essas classes **não existem** no codebase (grep = 0), e o padrão de overlay vigente é self-contained no componente. O Spotlight é renderizado **dentro** do componente de popup central da 24 (que já traz scrim/card com tokens). RNF-05 (check-cores): nenhum hex literal; classes próprias do Spotlight (lista/itens) reusam `--surface-*`/`--border`/`--accent`. |
| D-13 | **Projeto de contexto = `matchPath('/projetos/:id', useLocation().pathname)?.params.id ?? null`** — NÃO `useParams()` (27-H2) | O Spotlight é montado no `AppShell` (layout PAI; `<Outlet/>` renderiza os filhos, e `/projetos/:id` é FILHO — `App.tsx`). Em React Router v6 os `params` só fluem **pai→filho**: `useParams()` no nível do shell devolve `{}`, então o projeto de contexto seria **sempre `null`** e todo `/task`/`/note`/`/goal` cairia no ramo "sem contexto" (CA-15) — nunca criaria nada, mesmo dentro de um projeto. Resolver pelo `pathname` (espelha `SideNav`/`MobileNav`, irmãos do `<Outlet/>` que leem `useLocation().pathname`). |

## 3.5 Allowance de backend — `PATCH /api/v1/timer` (decisão HITL 23/06) — 27-H1

> **Esta é a ÚNICA superfície de backend que a 27 toca.** ZERO migração; um método novo no
> `timer.Service` + um handler + uma linha de rota. Ancorado no código real abaixo.

### Por que (a) e não (b) — justificativa

Duas opções foram pesadas para destravar a edição do timer rodando:

- **(a) ESCOLHIDA — endpoint por-usuário `PATCH /api/v1/timer` roteado pelo `timer.Service`.** A
  rota nova entra no `mountTimer` (`timer_routes.go:25-32`), ao lado de `start/stop/pause/resume`;
  o handler (`timer_handlers.go`) chama `svc.EditCurrent(...)` e devolve o **mesmo `Result`
  canônico** das outras mutações (RNF-05 da fase 03 — `timer/service.go:50-56`). O `timer.Service`
  **já tem** `SetOnChange`/`onChange(userID, *State, at)` (`service.go:100-106`) plugado ao hub WS
  no boot — então emitir o broadcast da fase 04 é **uma chamada `s.onChange(...)`** (como todas as
  outras mutações). A edição roda no **advisory lock do timer** que o service já usa em toda mutação
  (`lockUser` → `pg_advisory_xact_lock(hashtext(userID))`, `service.go:129-132`).
- **(b) REJEITADA — relaxar `entries.Patch` pra aceitar não-timing numa entry rodando + disparar
  `OnChange` quando a entry for a do timer.** Problemas: (1) o `entries.Service` **não conhece** o
  hub/`OnChange` (só o `timer.Service` tem o hook), então (b) teria de **injetar** o broadcast num
  segundo service e **detectar** "esta entry é a do timer" (re-ler `timer_states` dentro do
  `entries.Patch`) — espalha o conhecimento do timer; (2) afrouxar o `409 entry_running` do
  `entries.Patch` abre risco de relaxar timing por engano (o mesmo método trata `started_at`/
  `stopped_at`); (3) duplicaria a regra "rodando × parado" em dois lugares. **(a) mantém o
  `entries.Patch` byte-idêntico** (segue dando `409` — CA-13.1) e concentra a edição in-flight onde
  o timer já vive.

### Contrato (a) — assinaturas REAIS

```go
// internal/timer/service.go (método NOVO no Service existente)

// EditCurrentInput edita SÓ rótulo/projeto/tags da entry RODANDO (allowance
// 23/06). NÃO há campos de tempo: started_at/stopped_at/duration/mode/target
// são imutáveis numa entry em andamento (CA-13.1). ProjectIDSet distingue
// "ausente" de "null" (null remove o vínculo), como entries.PatchInput.
type EditCurrentInput struct {
    Description  *string  // nil = não mexe
    Tags         *[]string
    ProjectIDSet bool
    ProjectID    *string
}

// EditCurrent edita a entry RODANDO do usuário da sessão (CA-13). Sob o
// advisory lock do timer; carrega a entry de trabalho (mode != 'break') com
// FOR UPDATE; valida project_id novo via entries.ValidateRefs no WORKSPACE
// da entry; UPDATE só de description/project_id/tags (timing intocado);
// dispara s.onChange(State novo) → broadcast da fase 04. Sem timer de
// trabalho rodando → 409 no_active_timer (durante 'break' também: não há
// entry de trabalho aberta). project de outro ws/inexistente → 404 not_found.
func (s *Service) EditCurrent(ctx context.Context, userID string, in EditCurrentInput) (Result, error)
```

Esboço de implementação (espelha `Start`/`Stop` — `service.go:227-352`):
1. `now := s.now().UTC()`; `tx := s.db.Begin`; `defer Rollback`; `lockUser(tx, userID)`.
2. `row := loadStateRow(tx, userID, true)` (`FOR UPDATE`). `ErrNoRows` ⇒ `409 no_active_timer`. Se
   `row.Mode == "break"` ⇒ `409 no_active_timer` (não há entry de trabalho rodando — só pausa).
3. `e := entries.GetByID(tx, *row.EntryID)` (a entry rodando carrega `WorkspaceID` —
   `entries.Entry.WorkspaceID`, `repo.go:33`).
4. aplica `in` (Description/Tags/ProjectID com `ProjectIDSet`); `entries.ValidateRefs(tx,
   e.WorkspaceID, e.ProjectID, e.TaskID)` (`refs.go:21` — project de outro ws ⇒ `404`). Não
   recalcula `duration` (entry rodando tem `stopped_at = nil`; **não** se mexe nele).
5. `entries.Update(tx, e)` (`repo.go:133` — UPDATE com filtro duplo workspace+user; mantém
   `started_at`/`stopped_at`/`duration` como estão).
6. `tx.Commit`; monta `st := workState(apiModeOf(row.Mode), &e, row.TargetSeconds, row.StartedAt,
   now)` (`service.go:170` — elapsed/remaining derivados do relógio, **nada do tempo muda**);
   `s.onChange(userID, st, now)`; `return Result{State: st, ServerTime: now}, nil`.

### Handler + rota (api)

```go
// internal/api/timer_handlers.go (NOVO handler)
func handleTimerEdit(svc *timer.Service) http.HandlerFunc // PATCH: decodeJSON → svc.EditCurrent → respond.JSON(200, Result)
//   por-usuário (user da sessão); body {description?, project_id? (tri-state), tags?}; SEM workspace_id no body
//   (o ws é o da entry rodando, resolvido no service). writeDomainError mapeia 404/409 canônicos.

// internal/api/timer_routes.go (mountTimer — +1 linha)
tr.Patch("/", handleTimerEdit(svc))   // ao lado de Get("/") / Post("/start") etc.
```

### Frontend — helper no timerStore + api.ts

```ts
// lib/api.ts (helper NOVO)
export type EditTimerInput = { description?: string; project_id?: string | null; tags?: string[] };
export const editTimer = (input: EditTimerInput) =>
  request<TimerResult>("/timer", { method: "PATCH", ... });   // mesmo TimerResult de start/stop

// lib/timerStore.ts (ação NOVA, converge pelo MESMO mutate das outras — fase 05 CA-17)
editCurrent: (input) => mutate(() => editTimer(input)),
```

A ação `editCurrent` passa pelo `mutate` existente (`timerStore.ts:114-120`): sucesso → `apply(result)`
(o `apply`/anti-stale ordena WS × REST igual a qualquer mutação — `timerStore.ts:125-138`); falha →
`load()` imediato; **nunca rejeita** (o Spotlight chama `void editCurrent(...)`). A TimerBar reflete o
rótulo novo pelo `state.entry.description` que o `apply` atualiza (o broadcast da 04 propaga aos
outros devices via `onResult` → `apply`). A `RunningBar` segue **read-only** no input local (a edição
vem pelo Spotlight) — não regride a UX da fase 03.

## 4. Contratos consumidos (já existem — só a allowance de §3.5 é nova)

> Esta fase define **uma** rota nova (a allowance de §3.5). Esta seção lista os contratos
> **existentes** que o Spotlight chama, pra fixar a integração e o teste.

### 4.1 Criação de task (fase 02/24)
`createTask(ws, { project_id, name })` → `POST /api/v1/workspaces/{ws}/projects/{projectId}/tasks`
(ou o helper de task da `api.ts`). Resposta = `Task` (com `id`). O Spotlight usa `id` pra
`openTask(id, true)` (foco no título — fase 24). **Sem novo payload.**

### 4.2 Criação de nota (fase 26)
`createNote(ws, { project_id, title })` → `POST /api/v1/workspaces/{ws}/notes` (rota da fase 26).
Resposta = `Note` (com `id`) → `openNote(id)`. **Contrato pertence à fase 26**; o Spotlight só o
consome (se ausente no build, degrada — CA-17/D-09).

### 4.3 Timer (fase 03 + allowance §3.5)
```ts
useTimerStore.getState().start({ mode: 'countup', description, project_id });  // /timer e count-up (CA-11/14)
useTimerStore.getState().editCurrent({ description, project_id });             // sem-cmd + rodando (CA-13 — allowance §3.5)
useTimerStore.getState().state?.entry;                                          // p/ detectar rodando vs ocioso (CA-13/14)
```
`StartTimerInput` (api.ts l.418): `{ workspace_id, description?, project_id?, task_id?, mode? }`.
**`editCurrent` é a allowance da decisão HITL 23/06 (27-H1):** edita SÓ desc/project_id/tags da entry
rodando via `PATCH /api/v1/timer` (§3.5), roteado pelo `timer.Service` ⇒ dispara `OnChange` ⇒ broadcast
da fase 04. **`patchEntry` (de `entries`) continua NÃO sendo usado** — ele rejeita entry rodando
(`409 entry_running`, `service.go:184-187`) e mexe em timing; a allowance é a porta certa.
`useTimerStore.getState().state?.entry` é lido pra **decidir** o ramo (rodando → `editCurrent`;
ocioso → count-up).

### 4.4 Objetivo (fase 12)
`createObjective(ws, projectId, { name })` →
`POST /api/v1/workspaces/{ws}/projects/{projectId}/objectives`. Resposta = `Objective`. **Sem novo
payload** (a fase 29 estende com `completion_mode`/`target_date`; o `/goal` usa o default).

### 4.5 Workspace ativo + projeto de contexto + permissão
- `useCurrentWorkspace()` (fase 10) → `ws.id` (CA-18).
- **Projeto de contexto = `matchPath('/projetos/:id', useLocation().pathname)?.params.id ?? null`**
  (CA-15 / D-13 / 27-H2). **NÃO usar `useParams()`** — o Spotlight é montado no `AppShell` (layout
  PAI de toda rota autenticada) e em React Router os params só fluem pai→filho; `useParams()` acima
  do segmento `:id` devolve `{}`. Espelha o padrão de `SideNav`/`MobileNav` (`useLocation().pathname`
  + `isActive`), que são irmãos do `<Outlet/>` no mesmo nível.
- `can_write` do projeto de contexto vem do `/board` (fase 12/24, `useBoard`) — usado em CA-16.
  **GATEAR (27-H3/CA-16.1):** `useBoard` **não tem `enabled`** hoje (`objectives.ts:19-24`, a
  `queryFn` dispara sempre). Pra não disparar `GET /projects//board` malformado em toda rota/render
  do shell, escolher uma das opções (fixar): **(a)** estender `useBoard(ws, projectId, { enabled })`
  e chamar `useBoard(ws, projectId ?? '', { enabled: !!projectId && open })`; **ou (b)** isolar a
  leitura do `/board` num **componente-filho** renderizado só quando `open && projectId` (componente
  condicional, não hook condicional). Em ambos: nenhum request fora de `/projetos/:id` nem com o
  Spotlight fechado.

## 5. Frontend — assinaturas (núcleo puro + ações)

```ts
// lib/spotlight.ts  (PURO — testado por Vitest sem DOM)
export type SpotAct = 'task' | 'note' | 'timer' | 'goal';
export interface SpotCommand { act: SpotAct; label: string; desc: string; icon: string; }
export const SPOT: SpotCommand[];                       // 4 comandos (espelho mockup l.625-630)

export function spotToken(input: string): string;       // 1º token sem '/' , lowercase (l.633)
export function spotItems(input: string): SpotCommand[];// filtro startsWith(act) | label.includes (l.633-634)
export function spotTitle(input: string): string;       // input sem o comando (l.636), trim
export function spotParse(input: string): { act: SpotAct | null; title: string };
//  act = comando explícito do 1º token; null = "sem comando" (RF-19). title = spotTitle(input).

// lib/spotlightStore.ts  (decisão H — store em src/lib/, NÃO src/stores/)
interface SpotlightStore { open: boolean; query: string;
  openSpotlight(initial?: string): void; close(): void; setQuery(q: string): void; }
export const useSpotlightStore: UseBoundStore<...>;

// lib/spotlightActions.ts  (casca — mocka deps no teste)
export interface SpotCtx { ws: string; projectId: string | null; canWrite: boolean; }
export async function runSpot(input: string, ctx: SpotCtx): Promise<void>;
//  - spotParse(input).act === 'task'  && projectId && canWrite → createTask → abridor de task da 24 (foco), close
//  - ... 'note'  → createNote → abridor de nota da 26, close
//  - ... 'timer' → timerStore.start({mode:'countup', description:title, project_id}), close
//  - ... 'goal'  && projectId && canWrite → createObjective(ws,projectId,{name:title}), close
//  - act === null && timerRunning && input!=='' → timerStore.editCurrent({description:input, project_id}), close (CA-13 — allowance §3.5)
//  - act === null && timerRunning && input===''  → no-op (não apaga a descrição; mantém aberto)
//  - act === null && idle && input!=='' → start({mode:'countup', description:input, project_id}), close
//  - act === null && idle && input===''  → no-op (mantém aberto)
//  - falta projeto/permissão p/ task|note|goal → NÃO chama API (CA-15/16): sinaliza, mantém aberto
//  - qualquer falha → catch gracioso (CA-17): não rejeita pro componente (igual fase 05 CA-17)
```

- **A ordem do parse é load-bearing:** primeiro `spotParse` decide `act`; só `null` cai no ramo
  RF-19 (timer). Um input que **começa** com `/task ` etc. NUNCA cai no ramo do timer.
- **`runSpot` engole erros** (try/catch → `console`/aviso), igual ao `mutate` do `timerStore` (fase
  05 CA-17), pra a UI nunca ver uma promise rejeitada (CA-17).

## 6. Frontend — componentes

Tokens de `tokens.css`. **Scrim/card = o componente de popup central da fase 24** (decisão A); a 27
declara só classes **próprias de conteúdo** (ex.: `.spotitem`), no CSS global, **só com `var(--*)`**.
Reusa `.chip`/`.tt-btn` existentes. **NÃO referenciar `.mscrim`/`.scard`/`.swrap` do mockup** (não
existem no codebase — grep = 0; decisão A/D-12).

| Item | Arquivo (`frontend/src/`) | Notas |
|---|---|---|
| Conteúdo Spotlight | `components/spotlight/Spotlight.tsx` | renderiza **dentro do componente de popup central da 24** (decisão A): input (`#spotInput`, autofocus) + lista `.spotitem` (de `spotItems(query)`); chip `esc` no header, chip `↵` no item selecionado; realce do `spotTitle` no 1º item p/ `/task`,`/note` (l.646). Lê `useSpotlightStore`; null quando `open=false`. Scrim do popup central fecha (CA-04). |
| Teclado | dentro de `Spotlight.tsx` | `Enter` → `runSpot(query, ctx)` no item selecionado (l.637); ↑/↓ movem a seleção; `Esc` fecha (CA-04/06/07). Seleção volta a 0 a cada `setQuery` (CA-07). |
| Botão ⌘K | `components/spotlight/SpotlightButton.tsx` | `.tt-btn`; aria "Abrir captura (Cmd+K)"; `onClick=openSpotlight()`. Visível desktop+mobile (CA-02/03). |
| Hotkey | `components/spotlight/useSpotlightHotkey.ts` | effect no `AppShell`: `keydown` → `(meta||ctrl)&&k` → `preventDefault()`+`openSpotlight()`; cleanup no unmount (CA-01/01.1). |
| Contexto (projeto+can_write) | dentro de `Spotlight.tsx` (ou filho condicional — CA-16.1) | `ws` de `useCurrentWorkspace()`; projeto de contexto via `matchPath('/projetos/:id', useLocation().pathname).params.id` (D-13/27-H2); `can_write` via `useBoard` **gateado** por `enabled:!!projectId && open` (ou filho condicional — 27-H3/CA-16.1). |
| Wiring | `routes/AppShell.tsx` | monta `<Spotlight/>`, chama `useSpotlightHotkey()`, renderiza `<SpotlightButton/>` no chrome (canto, como o mockup l.153). |

- **Foco do input ao abrir** (CA-01): `useEffect` no `Spotlight` que foca `#spotInput` quando
  `open` vira `true` (espelha o `setTimeout(...i.focus())` do mockup l.632) — sem `setTimeout` se o
  ref já estiver montado; testar com `jsdom`+`waitFor`.
- **Pré-povoar com a descrição do timer (allowance 23/06):** como editar o timer rodando virou uma
  ação real (CA-13), abrir o Spotlight com o timer rodando **pode** semear a `query` com
  `state.entry.description` (UX: o usuário edita o rótulo atual em vez de redigitar) — **opcional**,
  decisão de UX do build; o teste do núcleo puro não depende disso. Texto vazio + Enter + rodando =
  no-op (CA-13 — não apaga a descrição). Se semear, o ramo "sem-comando" do parse deve seguir vendo
  `act === null` (a descrição não pode acidentalmente começar com `/task` etc. — caso de borda raro;
  na dúvida, NÃO semeia).

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** criar migração (RNF-01) — ZERO `.sql`/`migrations` nesta fase. A **única** superfície de
  backend permitida é a allowance de §3.5 (`EditCurrent` no `timer.Service` + handler + 1 rota
  `PATCH /api/v1/timer`); qualquer OUTRA rota/tabela/enum/service Go novo é bug de escopo.
- **NÃO** relaxar o `409 entry_running` do `entries.Patch` (CA-13.1) — a allowance é uma rota nova no
  `timer.Service` (opção (a), §3.5); `entries.Patch` fica **byte-idêntico** (segue recusando entry
  rodando). **NÃO** deixar a allowance tocar timing (`started_at`/`stopped_at`/`duration`/`mode`/
  `target`) — só description/project_id/tags.
- **NÃO** duplicar lógica de timer (D-08/RNF-04) — `/timer` e count-up chamam `useTimerStore.start`,
  o sem-comando rodando chama `useTimerStore.editCurrent` (allowance §3.5); nunca a api de timer
  direto. **NÃO** chamar `patchEntry` (de `entries`) — ele mexe em timing e rejeita entry rodando; a
  porta é `editCurrent` (27-H1).
- **NÃO** deixar o ramo sem-comando engolir um comando (D-05) — `spotParse` decide `act` ANTES; só
  `act===null` chega no ramo timer (CA-13/14). Input `/task ...` jamais inicia/edita timer.
- **NÃO** abrir dois popups centrais (CA-05) — abrir o Spotlight fecha/substitui o painel de
  task/nota ativo; rotear pro editor fecha o Spotlight ANTES de abrir o editor.
- **NÃO** vazar listener de teclado (CA-01) — `useSpotlightHotkey` faz cleanup; registrar 1× no
  shell, não por componente.
- **NÃO** usar `useParams()` pro projeto de contexto (27-H2/D-13) — o Spotlight é montado no
  `AppShell` (pai); usar `matchPath('/projetos/:id', useLocation().pathname).params.id`.
- **NÃO** chamar `useBoard` sem gate (27-H3/CA-16.1) — gatear por `enabled:!!projectId && open` (ou
  filho condicional); senão dispara `GET /projects//board` malformado em toda rota/render do shell.
- **NÃO** criar task/nota/goal sem projeto de contexto ou sem permissão (CA-15/16) — checar
  `projectId`/`canWrite` ANTES de chamar a API; sinalizar à-prova-de-culpa (RNF-03), não erro
  vermelho.
- **NÃO** inventar projeto "Inbox" default (D-07/§6) — fora de escopo.
- **NÃO** deixar `runSpot` rejeitar pro componente (CA-17) — try/catch gracioso (padrão fase 05).
- **NÃO** depender de COMO a 24 nomeou as classes do popup (decisão A/D-09) — reusar o **componente
  de popup central da 24** + seu abridor; se ausente no build, degradar pra navegação
  (`/projetos/{id}`) sem quebrar.
- **NÃO** criar entry vazia no sem-comando+ocioso com input vazio (D-11) — é no-op.
- **NÃO** usar hex literal nem referenciar `.mscrim`/`.scard`/`.swrap` do mockup (RNF-05/D-12/
  decisão A) — o scrim/card vêm do popup central da 24; classes próprias do Spotlight só com
  `var(--*)`.
- **NÃO** criar `src/stores/` (decisão H/27-H4) — o `spotlightStore` vive em `src/lib/`, junto de
  `timerStore.ts`/`presenceStore.ts`/`noticeStore.ts`.
- **NÃO** transformar o placeholder "Comando ou busca…" em busca de entidades (§6 spec) — só
  comandos de criação nesta fase.

## 8. Dependências & paralelização

- **Depende (build order 24 → 26 → 27):** consome o **componente de popup central da fase 24**
  (decisão A) + seu abridor de task (`createTask`), o **popup de nota** da fase 26 (reusa o popup
  central da 24; abridor de nota + `createNote`) e o `timerStore` (fase 03, já em produção) +
  `createObjective` (fase 12, já em produção). A ordem do PRD (`MILESTONE-3 §7`) é
  **24 → 26 → 27 → 28 → 29 → 25**; o Spotlight deve ser construído **depois** de 24 e 26 estarem no
  app.
- **Paralelização:** **SEM migração**, mas a 27 agora **toca backend** (allowance §3.5:
  `internal/timer/service.go` + `internal/api/timer_handlers.go` + `internal/api/timer_routes.go`).
  Os toques Go são **aditivos** (método/handler/rota novos; nada existente muda — `entries.Patch`
  byte-idêntico), mas colidem com qualquer fase que mexa nos mesmos arquivos do timer; sob worktrees
  concorrentes, o pacote `timer` é ponto de integração (1 integrador). No frontend, edita do baseline
  **`routes/AppShell.tsx`** (montar Spotlight+hotkey+botão) e, opcionalmente, `navItems.ts` — os
  mesmos arquivos que 22/23 já tocaram; mais `lib/api.ts` (`editTimer`) e `lib/timerStore.ts`
  (`editCurrent`, aditivos). Os arquivos **novos** (`lib/spotlight.ts`, `lib/spotlightStore.ts`,
  `components/spotlight/*`, `lib/spotlightActions.ts`) não colidem com nada. **Não-paralelizável com
  24/26** (dependência dura do componente de popup central e dos abridores da 24/26). **Possível
  dependência aditiva no `useBoard`** (27-H3/CA-16.1, opção (a)): se a 27 estender `useBoard` com
  `enabled`, isso toca `lib/queries/objectives.ts` (fase 12) — opção ADITIVA; a opção (b) evita esse
  toque.

## 9. Ordem de execução (ver tasks.md)

**Allowance de backend primeiro** (TDD Go: `EditCurrent` + handler/rota — §3.5) → núcleo puro
(`lib/spotlight.ts` + testes) → store + `editTimer`/`editCurrent` → ações (`spotlightActions.ts`,
deps mockadas) → componentes (Spotlight, botão, hotkey) → wiring no shell → E2E → verificação + gate.
Commit por tarefa. `make check-full` é o gate final (Go: a allowance aditiva passa, `entries.Patch`
byte-idêntico; Vitest + Playwright + builds + check-cores + smoke do compose).
