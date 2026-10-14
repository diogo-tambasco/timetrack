# plan.md — Fase 30 · Elo timer↔task

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + (não-)modelo de dados +
> contratos + ordem de tarefas. **Fase majoritariamente FRONTEND**: o backend do timer já suporta
> o elo (verificado no código). Herda o baseline do `SDD.md §7` e os contratos reais das fases 03
> (timer/entries), 04 (sync), 12 (board), 23 (reuso do store no `/remoto`), 24 (popup de task).
> Nada aqui é código — é o contrato. **SEM migração** (RNF-06).

---

## 1. Arquitetura geral

```
Backend (INTOCADO — só testes de não-regressão; RNF-01/RNF-06):
  internal/timer/service.go        (existe)  ← Start JÁ grava project_id+task_id (CA-01); ValidateRefs (CA-02/03)
  internal/entries/refs.go         (existe)  ← ValidateRefs (workspace por ref, sem cruzar task↔project — D-01)
  internal/objectives/board.go     (existe)  ← /board já agrega tracked_seconds por task_id (fase 12; CA-13)
  → NENHUM arquivo Go novo/alterado de produção. Só *_test.go pinando o contrato (CA-01/02/03/19).

Frontend (o elo):
  lib/timerStore.ts                (existe)  ← REUSADO: start({...,task_id,project_id}). NÃO alterar a lógica.
                                                (start já aceita task_id — TimerBar.IdleBar usa hoje.)
                                                (store zustand vive em frontend/src/lib/ — decisão H.)
  lib/queries/objectives.ts        (existe)  ← useBoard / boardKey p/ invalidação (CA-13)
                                                (NÃO existe lib/queries/board.ts — boardKey/useBoard moram aqui.)
  components/board/
     TaskTimeControl.tsx           (NOVO)    ← o contrato (CA-20): leitura de tempo + Start/Parar, lê o store
     TaskTimeControl.spec.tsx      (NOVO)
     TaskPanelProps.tsx            (ALTERA, só o bloco Time) ← substitui o bloco Time inline da fase 24
                                                pelo <TaskTimeControl/>, wirando o botão Start mock (CA-20/B)
  components/timer/
     TimerBar.tsx                  (ALTERA)  ← RunningBar ganha o chip da task (CA-16), aditivo
  components/board/  (opcional)
     <card/lista da fase 24>       (consome) ← indicador "cronometrando agora" (CA-17, opcional)
  components/board/ProjectBoard.tsx (ALTERA) ← observa transição rodando-neste-projeto→ocioso e invalida
                                                boardKey (sobrevive ao fechar o popup — CA-13/30-H5).
                                                JÁ importa boardKey + tem board.project.id (projectId).
  routes/cockpit/ProjectDetail.tsx (existe)  ← página de projeto: chama useBoard(wsId, projectId) e
                                                renderiza o ProjectBoard; permanece montada com o popup
                                                aberto/fechado (alternativa de hospedar o observador).
```

A fase 24 renderiza o **bloco Time INLINE** em `TaskPanelProps.tsx` (`tracked_seconds` real + botão
**Start** mock/desabilitado com `data-testid="task-start"` — fase 24 CA-26). **NÃO existe** slot
`TaskTimeControl` provido pela 24 (decisão de coordenação B). A fase 30 cria `TaskTimeControl.tsx` e o
**monta no lugar do bloco Time mock** dentro de `TaskPanelProps.tsx` (a 30 reaproveita o **mesmo botão
Start** que a 24 criou — troca a ação mock → real), entrega o chip da TimerBar e a invalidação do
board (no `ProjectBoard`, que sobrevive ao fechar o popup). **Nada de backend novo.** `TaskPanelProps.tsx`
entra na lista de arquivos da 30 como **ALTERA (só o bloco Time)** — a 30 é a única fase que toca esse
trecho depois da 24; a fase 25 (propriedades custom) mexe em linhas distintas, sem colisão.

## 2. (Não-)Migração — RNF-06

**Nenhuma migração nesta fase.** O schema já cobre o elo:

- `time_entries.task_id` (NULL ok) existe desde a fase 03 (`0009_time_entries`), com
  `ON DELETE SET NULL` (task deletada não apaga a entry — fase 03).
- `idx_entries_task ON time_entries (task_id) WHERE duration_seconds IS NOT NULL` existe desde a
  fase 12 (`0013_objectives`) — é o índice que a soma por task do `/board` usa (RNF-04 da fase 12).
- `tasks.project_id` (NOT NULL) existe desde a fase 02 — toda task tem projeto (base do CA-03/15).

> Se a ETAPA 2 (verificação adversarial) achar um gap **mínimo** (ex.: o `Start` aceitar `task_id`
> sem `project_id` e gravar entry com `project_id` nulo, deixando o `/board` do projeto sem
> contabilizar — embora a soma por task continue certa por D-06 da fase 12), a correção é **front**
> (sempre mandar `project_id` da task — CA-20) e/ou **documentar**, **NÃO** criar migração nem mudar
> o backend sem reabrir a decisão.

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Backend intocado; `ValidateRefs` NÃO cruza task↔project** (cada ref é checada contra o workspace — `refs.go:21`). O front manda `{project_id: task.project_id, task_id: task.id}` da MESMA task → consistência por origem, não por validação nova. | Escopo mínimo (RNF-01). Adicionar validação cruzada mudaria o backend do timer e exigiria reconciliar o caminho da TimerBar/`/remoto`/Foco que já mandam `task_id` solto. A task SEMPRE tem `project_id` (fase 02), e o front mandar os dois resolve a coerência sem risco de regressão. A verificação adversarial pina que start-por-task grava os dois coerentes (CA-03). |
| D-02 | **`description` da entry = `task.name`** ao iniciar pela task (CA-10.1) | Captura < 2s (RNF-02): a entry nasce legível na TodayList/relatórios sem digitar. O usuário pode editar depois (a entry rodando é readOnly na TimerBar, mas o `EntryModal` edita a fechada — fase 03). |
| D-03 | **`TaskTimeControl` é AUTOSSUFICIENTE** (lê `useTimerStore`/`useTimerClock`/`useBoard` direto), sem props de callback do timer; a fase 24 só passa `task`, `workspaceId`, `canWrite` | Padrão da fase 23 (`RemoteControl` reusa o store sem fetch próprio nem 2º `attachTimerSync`). Mantém o popup da fase 24 burro em relação ao timer; o elo fica encapsulado num componente testável isolado (jsdom). |
| D-04 | **Start inicia `countup`** (mode default), nunca pomodoro/countdown | É o modo rápido (a TimerBar/IdleBar também só faz countup — `TimerBar.tsx:78`). Pomodoro/countdown têm UI própria (Foco). Menos superfície, menos regressão. |
| D-05 | **Estado "rodando nesta task" derivado de `state.entry.task_id === task.id`** | Fonte única da verdade = o `useTimerStore` global. Sem flag local, sem corrida: o popup, a TimerBar e o card derivam todos do mesmo `state` (que o sync/anti-stale já mantém correto — fase 04). |
| D-06 | **Tempo ao vivo no controle = `tracked_seconds` (board) + `elapsed` da entry rodando QUANDO é desta task** (CA-11.1) | O `tracked_seconds` do `/board` só conta entries **fechadas** (D-06 da fase 12). A sessão em curso ainda não fechou → somar o `elapsed` derivado (`useTimerClock`) dá o número vivo. Ao parar, o board refaz a query e o vivo vira persistido (CA-13) — sem dupla contagem (a entry fechada entra no board, o `elapsed` some porque o timer ficou ocioso). |
| D-07 | **Invalidação do `boardKey` na transição-pra-ocioso, LOCAL no `ProjectBoard.tsx`** (CA-13/30-H5) — **NÃO** dentro do `TaskTimeControl` | O `TaskTimeControl` desmonta ao fechar o popup; um efeito nele não dispararia se o usuário parasse pela TimerBar/troca/auto-corte com o popup já fechado (corrida real). O observador mora no `ProjectBoard.tsx` (via `ProjectDetail`), que **sobrevive** enquanto a página de projeto está aberta. Padrão da fase 12 (mutações de task invalidam o board localmente). **Gatilho:** `queryClient.invalidateQueries({queryKey: boardKey(ws, projectId)})` **uma vez** quando o `state` do timer transiciona de "rodando numa entry com `entry.project_id === projectId`" → ocioso (`state===null`/`entry===null`), comparando o `project_id` anterior com o atual (sem loop de re-render). Como o `state` só atualiza no `apply` da resposta do servidor (anti-stale da fase 04), a invalidação ocorre **após** o stop confirmar e a entry estar contabilizada no `/board`. NÃO invalida se o running era de outro projeto. *(O `TaskTimeControl` pode ter efeito local só de UX enquanto montado, mas a convergência do board não depende dele.)* |
| D-08 | **Chip da task na TimerBar = ADITIVO** ao chip do projeto (RunningBar), resolvido por query de tasks já cacheada | Object permanence (CA-16) sem mudar o layout sem-task. Degrada (some o chip da task) se o nome não resolver — nunca quebra. Espelha o chip de projeto que já existe (`TimerBar.tsx:177`). |
| D-09 | **Gating: só o Start é gated por `canWrite`; o Parar nunca prende** (CA-14) | Idêntico ao CA-11 da fase 23 (controle remoto): esconder Parar prenderia um timer global que o viewer não consegue mais parar. Start gated porque iniciar tempo numa task que não pode cronometrar não faz sentido (o backend negaria a escrita de entry? não — o timer é por-usuário; mas a UX correta é não oferecer). |
| D-10 | **Indicador "cronometrando agora" (CA-17) é OPCIONAL** | Não bloqueia o gate; se a fase 24 já tiver um slot no card, plugar o `state.entry.task_id` é trivial. Se virar custo, fica fora sem furar a spec. |

## 4. Contratos

### 4.1 Backend — contrato EXISTENTE confirmado (sem mudança)

`POST /api/v1/timer/start` (por usuário; fase 03):

```jsonc
// → request (o que o front da fase 30 manda ao iniciar pela task)
{ "workspace_id": "8c…", "description": "Definir telas",
  "project_id": "0d9e…",        // = task.project_id
  "task_id": "77aa…",           // = task.id
  "tags": [] }                  // mode ausente → "countup" (default)

// ← 201  TimerResult { state: TimerState, server_time } — corpo IDÊNTICO ao GET /timer
//   state.entry.task_id === "77aa…"  e  state.entry.project_id === "0d9e…"

// ← 404 ref de outro workspace / inexistente / uuid malformado (CA-02 — HERDADO, não muda)
{ "error": { "code": "not_found", "message": "task não encontrada" } }
```

- **Importante (anti-armadilha):** o erro de ref inválida do timer é **`404 not_found`** (via
  `entries.ValidateRefs` → `domain.NotFound`), **NÃO** `400 invalid_reference`. A fase 30 **não**
  altera isso (RNF-01). O CA-02 é teste de não-regressão.
- `ValidateRefs` confere `project_id` e `task_id` **cada um** contra o workspace; **não** confere
  que a task pertence ao projeto (D-01). A coerência vem do front.
- `time_entries.task_id` da entry → o `/board` (fase 12) já o agrega em `tracked_seconds` por task
  (`GROUP BY task_id WHERE duration_seconds IS NOT NULL AND user_id=$ AND task_id = ANY(...)`).

### 4.2 Frontend — `TaskTimeControl` (o contrato da fase 30, consumido pela fase 24)

```ts
// components/board/TaskTimeControl.tsx
type TaskTimeControlProps = {
  task: { id: string; project_id: string; name: string; tracked_seconds: number };
  workspaceId: string;
  canWrite: boolean;   // = board.project.can_write (fase 12)
};
```

Comportamento (derivado do `useTimerStore` global — D-03/D-05/D-06):

- `const running = state?.entry?.task_id === task.id`
- **leitura de tempo** = `fmtDayTotal(task.tracked_seconds + (running ? elapsed : 0))`
  (`elapsed` do `useTimerClock`; `fmtDayTotal` da fase 05 — ex.: `2h12`).
- **running**: mostra o tempo vivo + botão **Parar** (`void stop()`).
- **idle nesta task** (não-running) **E** `canWrite`: mostra o tempo + botão **Start**
  (`void start({ workspace_id, project_id: task.project_id, task_id: task.id,
  description: task.name, tags: [] })`).
- **!canWrite** e **!running**: só a leitura (sem botão — CA-14).
- **!canWrite** e **running** (borda): mostra **Parar** (não prende — CA-14).
- `task.project_id` ausente/falsy: Start inerte (CA-15).
- **NÃO** é o `TaskTimeControl` que invalida o `boardKey` (30-H5): o `TaskTimeControl` desmonta ao
  fechar o popup, então um efeito nele perderia o stop pela TimerBar/troca/auto-corte com o popup
  fechado. A invalidação do `boardKey(ws, projectId)` na transição rodando-nesta-task → ocioso mora
  no **`ProjectBoard.tsx`** (§4.4) — que sobrevive enquanto a página de projeto está aberta. O
  `TaskTimeControl` PODE manter um efeito local só de UX enquanto montado, mas a convergência do board
  não depende dele.

### 4.4 Frontend — observador de invalidação do board (`components/board/ProjectBoard.tsx`) — CA-13/30-H5

O `components/board/ProjectBoard.tsx` (renderizado por `routes/cockpit/ProjectDetail.tsx`, permanece
montado enquanto a página de projeto está aberta, **mesmo com o popup fechado** — o popup é overlay de
state local) observa o `state` do `useTimerStore` e invalida o board quando o tempo "gruda". O
`ProjectBoard` **já** importa `boardKey` (de `lib/queries/objectives.ts`) e tem `board.project.id`
(`projectId`):

```ts
// dentro do components/board/ProjectBoard.tsx (tem projectId = board.project.id e o queryClient)
const prevProj = useRef<string | null | undefined>(undefined);
const entryProj = useTimerStore((s) => s.state?.entry?.project_id ?? null);
useEffect(() => {
  // transição: estava rodando numa entry DESTE projeto → agora ocioso (entryProj === null)
  if (prevProj.current === projectId && entryProj === null) {
    qc.invalidateQueries({ queryKey: boardKey(workspaceId, projectId) });
  }
  prevProj.current = entryProj;
}, [entryProj, projectId, workspaceId, qc]);
```

- **Gatilho exato:** invalida **uma vez** quando o `project_id` da entry rodando passa de `projectId`
  (este projeto) → `null` (ocioso). Como o `state` só atualiza no `apply` da resposta do servidor
  (anti-stale da fase 04), a invalidação ocorre **após** o stop confirmar e a entry estar contabilizada
  no `/board` — sem corrida. **NÃO** invalida se o running era de **outro** projeto, **NÃO** dispara a
  cada render (`useRef` guarda o valor anterior — sem loop). Cobre stop pela TimerBar/troca/auto-corte
  com o popup já fechado (30-H5). `boardKey`/`useBoard` vêm de `lib/queries/objectives.ts` (não existe
  `lib/queries/board.ts`).

### 4.3 Frontend — chip da task na TimerBar (RunningBar)

`RunningBar` (`TimerBar.tsx:159`) ganha, **após** o chip de projeto, um chip de task quando
`state.entry.task_id` existir:

```tsx
// resolve o nome via tasks do projeto da entry (query já cacheada; degrada se não achar)
// SEM Icon.board (não existe no set de icons.tsx — chaves reais: play/stop/check/list/…);
// espelha o chip de projeto da TimerBar (tt-chip só com o nome, sem ícone — TimerBar.tsx:177).
{taskName && (
  <span className="tt-chip">{taskName}</span>
)}
```

> **Ícone — anti-armadilha (30-H1):** o conjunto `icons.tsx` **não** tem a chave `board`. Usar
> `Icon.board` daria `undefined`/erro de TS e quebraria a `TimerBar` (compartilhada com o TRACKING →
> regressão geral, viola RNF-01). O chip da task **não usa ícone** (espelha o chip de projeto, que é
> `tt-chip` só com texto). Se um ícone vier a ser desejado no futuro, usar uma chave que EXISTE
> (`Icon.check` ou `Icon.list`) — nunca `Icon.board`. O CA-16 asserta o **nome** da task no chip, não
> o ícone, então nenhum CA muda por causa disso.

- Sem `task_id` → **byte-idêntico** ao de hoje (CA-18/RNF-01).
- Resolução do nome: `useTasks(ws, { project_id: state.entry.project_id })` (cacheada) → find por
  id; se não resolver, **não renderiza** o chip da task (degrada — D-08).

## 5. Frontend — telas e componentes

Tokens de `tokens.css`. Reusa `useTimerStore`/`useTimerClock` (fase 03/15), `fmtDayTotal`/`fmtClock`
(fase 05/03), `useBoard`/`boardKey` (fase 12, em `lib/queries/objectives.ts`), `.tt-btn--accent`/
`.tt-iconbtn`/`.tt-chip`/`.tt-dot`/`.tt-pulse`, `Icon.play`/`Icon.stop` (chaves reais de `icons.tsx`;
**NÃO** existe `Icon.board` — o chip da task é `tt-chip` só com o nome, sem ícone — 30-H1).

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Controle de tempo da task | `components/board/TaskTimeControl.tsx` (NOVO) | o contrato CA-20; lê o store; Start/Parar + leitura viva (CA-10..15) |
| Bloco Time do popup | `components/board/TaskPanelProps.tsx` (ALTERA, **só o bloco Time**) | substitui o bloco Time inline da fase 24 pelo `<TaskTimeControl/>`, wirando o botão Start mock `data-testid="task-start"` (CA-20/B); resto do popup intocado |
| Chip da task | `components/timer/TimerBar.tsx` (ALTERA, aditivo) | RunningBar +chip da task (CA-16, `tt-chip` só com nome — sem `Icon.board`); sem task = idêntico |
| Indicador "cronometrando" | card/lista da fase 24 (OPCIONAL) | `.tt-pulse`/ponto accent quando `state.entry.task_id===task.id` (CA-17) |
| Invalidação do board | `components/board/ProjectBoard.tsx` (ALTERA) | observador da transição rodando-neste-projeto→ocioso que **sobrevive ao fechar o popup** (renderizado por `routes/cockpit/ProjectDetail.tsx`); invalida `boardKey(ws, projectId)` (CA-13/30-H5) — **NÃO** só dentro do `TaskTimeControl`. Já importa `boardKey` + tem `projectId` |

- **Reuso, não duplicação (CA-10/RNF-04):** `TaskTimeControl` chama `useTimerStore((s)=>s.start)` /
  `s.stop` e `useTimerClock()` — **zero** fetch próprio, **zero** 2º `attachTimerSync` (padrão fase
  23). O `attachTimerSync` segue ligado uma vez só no boot do shell.
- **Gating (CA-14):** o componente recebe `canWrite` (da fase 24, que já tem o `board.project.
  can_write`). Não re-deriva role.

## 6. Armadilhas conhecidas (vetadas)

- **NÃO** mudar o backend do timer/entries/board (RNF-01/RNF-06) — o contrato já suporta o elo;
  só testes de não-regressão (CA-01/02/03/19).
- **NÃO** esperar `400 invalid_reference` do start — o erro de ref é **`404 not_found`** (herdado
  da fase 03; `ValidateRefs`→`domain.NotFound`). A fase 30 não muda (CA-02).
- **NÃO** criar um 2º `attachTimerSync`/socket no popup ou no controle (RNF-04) — reusar o store
  global (lição da fase 23).
- **NÃO** acumular o tempo vivo como verdade (RNF-02 do timerStore) — `elapsed` é derivado de
  `useTimerClock`; some quando o timer fica ocioso; o persistido vem do `/board` após refetch
  (CA-11.1/13). **Sem dupla contagem.**
- **NÃO** somar o `elapsed` da entry rodando se ela **não** é desta task (CA-11.1) — só quando
  `state.entry.task_id === task.id`.
- **NÃO** esconder o **Parar** quando o timer roda nesta task e o user é viewer (CA-14/D-09) —
  prenderia o timer global (furo que a fase 23 já corrigiu).
- **NÃO** mandar `task_id` sem `project_id` ao iniciar pela task (D-01/CA-03) — sempre os dois da
  mesma task; a coerência vem da origem.
- **NÃO** quebrar a TimerBar quando o nome da task não resolve (CA-16/D-08) — degrada pro chip de
  projeto; o caminho sem-task fica byte-idêntico (CA-18).
- **NÃO** reescrever o popup/layout/demais propriedades da fase 24 — a 30 SÓ substitui o **bloco Time
  inline** de `TaskPanelProps.tsx` pelo `<TaskTimeControl/>`, reaproveitando o **mesmo botão Start**
  (`data-testid="task-start"`) que a 24 criou (§1; decisão de coordenação B). NÃO existe slot provido
  pela 24; a costura é a substituição desse bloco. Título/Status/Priority/Due/Notion/descrição/sidebar
  e o componente de popup central da 24 ficam intocados.
- **NÃO** iniciar pomodoro/countdown a partir da task (D-04/§6 da spec) — Start = countup.
- **NÃO** deixar o `cronometro.spec` (E2E da fase 03) regredir — liberar a :8080 antes do E2E
  (lição do KICKOFF); o elo não toca o fluxo sem-task.

## 7. Dependências & paralelização

- **Depende DURA da fase 24** (Tasks 2.0): o **popup central de task** (`TaskPanel.tsx`/
  `TaskPanelProps.tsx`), o **bloco Time inline** e o **botão Start (mock, `data-testid="task-start"`)**
  são da fase 24. A fase 30 **substitui o bloco Time inline** de `TaskPanelProps.tsx` pelo
  `TaskTimeControl` e **wira o mesmo botão Start** (mock → real) — NÃO existe slot provido pela 24
  (decisão de coordenação B). **Não dá pra construir antes da 24** (o bloco Time/o botão Start não
  existem). Por isso é a **última fatia da M3** (decisão HITL — §9 ponto 2 do PRD: "entra como
  extensão do `start` quando o popup de task já existir").
- **Depende (leitura) da fase 12** (`/board` com `tracked_seconds` por task) — já em produção;
  estável.
- **Sensível ao mundo TRACKING** (fase 03/04/15/16/23): mexe no `TimerBar` e dispara `start`/`stop`
  do store. **RNF-01 é o risco central** → TDD + verificação adversarial **isolados** nesta fase,
  com CA-18/19 pinando a não-regressão.
- **NÃO paralelizável com a fase 24** (dependência dura: precisa do popup montado). **NÃO**
  worktree-isolável em relação à 24. Em relação às demais (25/26/27/28/29) é independente, mas como
  toca `TimerBar.tsx` (compartilhado), se construída sob worktree precisa de integração no
  `TimerBar.tsx` (arquivo compartilhado com TRACKING). **Recomendação:** construir **depois** da 24
  (e idealmente por último na M3), no main loop sequencial, pelo contention em `TimerBar.tsx` +
  sensibilidade ao timer.
- **SEM migração** → zero contenção de número de migração com 24–29.

## 8. Ordem de execução (ver tasks.md)

Backend primeiro só pra **pinar o contrato** (testes de não-regressão CA-01/02/03/19 — sem código
de produção novo), depois o frontend: `TaskTimeControl` (CA-10..15/20) → chip da TimerBar (CA-16) →
invalidação do board (CA-13) → indicador opcional (CA-17) → E2E (CA-13/16/18) → verificação + gate.
Commit por tarefa. `make check-full` é o gate final.
