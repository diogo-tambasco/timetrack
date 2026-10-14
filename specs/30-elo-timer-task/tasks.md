# tasks.md — Fase 30 · Elo timer↔task

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-12).
> Cada T-XX lista os CAs cobertos. **Fase majoritariamente FRONTEND** — o backend só ganha testes
> de NÃO-REGRESSÃO (o contrato já suporta o elo; RNF-01/RNF-06). **SEM migração.**
> **Depende DURA da fase 24** (popup de task montado) — esta fase só fia o botão Start real.

---

## Bloco 1 — backend: pinar o contrato (sem código de produção novo) (T-01 → T-02)

- **T-01 — Pinar o start-por-task no backend** (CA-01/03/19, RNF-01)
  - **SEM mudar produção.** Só testes Go que provam o contrato existente.
  - Teste (`internal/timer/*_test.go`, harness `timertest`/`startStopSetup` da fase 03):
    - `TestStartComTaskGravaProjectETaskID` — `Start({WorkspaceID, ProjectID: p, TaskID: t})`
      grava a entry com `project_id=p` **e** `task_id=t` (CA-01/03); a entry resultante (e o
      `GET /timer`) refletem os dois.
    - `TestStartComTaskNaoMudaCiclo` — start com `task_id` é um `countup` normal: `mode='countup'`,
      sem efeito no ciclo Pomodoro (`focus_count=0`, `break_is_long=false` — espelha o reset do
      start) — pina CA-19 (não-regressão do ciclo).
  - **Acceptance:** os testes passam contra o código atual (nasce verde — RNF-01); se a verificação
    mutar `entries.Insert` removendo `TaskID`, o `TestStartComTaskGravaProjectETaskID` QUEBRA.

- **T-02 — Pinar a ref inválida do start (404 not_found, NÃO 400)** (CA-02, RNF-01)
  - **SEM mudar produção.** Teste Go que pina o erro HERDADO da fase 03 (anti-armadilha do plan).
  - Teste (`internal/timer/*_test.go` ou `internal/api/timer_*_test.go`):
    - `TestStartTaskDeOutroWorkspace404` — `task_id` de outro workspace → `domain.NotFound`
      (HTTP `404 not_found`), nenhuma entry criada.
    - `TestStartTaskMalformada404` — `task_id` uuid malformado → `404 not_found` (via
      `uuid.Validate` no `ValidateRefs`), nunca 500.
  - **Acceptance:** verde contra o código atual; pina que o elo não introduz vazamento nem 500 nem
    troca o código de erro (mutação que troque `domain.NotFound` por `domain.Validation` no
    `refs.go` QUEBRARIA — registra a intenção, não muda o código).

> **Fim do bloco 1:** `make test-back` 100% verde (timer/entries intactos). Nenhum frontend tocado.

## Bloco 2 — frontend: o elo (T-03 → T-09)

- **T-03 — `TaskTimeControl` — leitura de tempo + Start** (CA-10/10.1/11.1/15/20)
  - `frontend/src/components/board/TaskTimeControl.tsx` (NOVO): props `{ task, workspaceId,
    canWrite }` (§4.2 do plan). Lê `useTimerStore` (`start`, `state`) e `useTimerClock` (`elapsed`).
    Estado **idle nesta task** + `canWrite`: tempo `fmtDayTotal(task.tracked_seconds)` + botão
    **Start** (`.tt-btn--accent`, `Icon.play`) → `void start({ workspace_id: workspaceId,
    project_id: task.project_id, task_id: task.id, description: task.name, tags: [] })`. Start
    inerte se `!task.project_id` (CA-15).
  - Spec `TaskTimeControl.spec.tsx` (jsdom): clicar Start chama `start` com **exatamente**
    `{workspace_id, project_id: task.project_id, task_id: task.id, description: task.name, tags:[]}`
    (mock do store) — pina CA-10/10.1; tempo formatado = `fmtDayTotal(tracked_seconds)` quando não
    rodando (CA-11.1); Start inerte sem `project_id` (CA-15).
  - **Acceptance:** spec red (componente não existe) → green.

- **T-04 — `TaskTimeControl` — estado rodando (Stop + tempo ao vivo)** (CA-11/11.1)
  - Estende `TaskTimeControl`: `const running = state?.entry?.task_id === task.id`. Rodando → tempo
    `fmtDayTotal(task.tracked_seconds + elapsed)` (vivo) + botão **Parar** (`Icon.stop`,
    `.tt-iconbtn--stop`) → `void stop()`. Não-rodando → só `task.tracked_seconds`.
  - Spec: store mockado com `state.entry.task_id === task.id` → renderiza **Parar** (não Start) e o
    tempo soma `tracked_seconds + elapsed` (CA-11/11.1); `state.entry.task_id !== task.id` → mostra
    Start e o tempo NÃO soma o elapsed (CA-11.1 — só desta task).
  - **Acceptance:** spec red→green; a mutação que somasse o elapsed de OUTRA task quebra o teste.

- **T-05 — `TaskTimeControl` — trocar de task** (CA-12)
  - Garantir (via o `start` existente) que clicar Start nesta task **enquanto** o timer roda em
    outra task/label dispara `start` com os refs desta task (o backend fecha a anterior — CA-05 da
    fase 03). É comportamento do store; o teste pina a UI.
  - Spec: store rodando em `entry.task_id = "OUTRA"` → o controle desta task mostra **Start** (não
    Parar, porque o running é de outra task — CA-11); clicar chama `start` com `task_id` desta task
    (CA-12). (O fechamento da anterior é do backend, coberto pela fase 03; aqui só a UI.)
  - **Acceptance:** spec red→green.

- **T-06 — `TaskTimeControl` — gating de viewer/sem-escrita** (CA-14, D-09)
  - `!canWrite` **e** `!running` → **sem** botão Start (só leitura). `!canWrite` **e** `running`
    (borda) → botão **Parar** visível (não prende — CA-14). `canWrite` → comportamento normal.
  - Spec (padrão `viewer-gating.spec` da fase 12): `canWrite=false` + não-rodando → sem
    Start/Parar, só o tempo; `canWrite=false` + rodando nesta task → **Parar** presente (CA-14).
  - **Acceptance:** spec red→green; a mutação que escondesse o Parar com `!canWrite && running`
    quebra.

- **T-07 — Chip da task na TimerBar** (CA-16/18, RNF-01)
  - `frontend/src/components/timer/TimerBar.tsx` (ALTERA, aditivo): no `RunningBar`, quando
    `state.entry.task_id` existir, resolver o nome via `useTasks(ws, { project_id:
    state.entry.project_id })` (cacheada) e renderizar um `<span className="tt-chip">{taskName}</span>`
    da task **após** o chip de projeto. **SEM ícone** — `Icon.board` **NÃO existe** no set de
    `icons.tsx` (chaves reais: `play`/`stop`/`check`/`list`/`table`/…); usá-lo daria erro de TS/
    `undefined` e quebraria a TimerBar compartilhada com o TRACKING (regressão geral — RNF-01). O chip
    espelha o chip de projeto (`tt-chip` só com texto — `TimerBar.tsx:177`). Sem `task_id` → **nada
    muda** (byte-idêntico). Nome não resolve → não renderiza o chip da task (degrada — D-08).
  - Spec `TimerBar.spec.tsx` (estende): entry rodando com `task_id` resolvível → chip da task
    visível (CA-16); entry sem `task_id` → TimerBar idêntica ao baseline (CA-18, sem o chip de
    task); entry com `task_id` cujo nome não resolve → sem chip de task, sem crash (D-08).
  - **Acceptance:** spec red→green; o baseline sem-task (`TimerBar.spec`/`TimerBar.responsive.spec`)
    continua verde (regressão zero).

- **T-08 — Invalidação do board ao parar (o tempo gruda)** (CA-13/30-H5)
  - O observador da invalidação DEVE morar no **`components/board/ProjectBoard.tsx`** (renderizado por
    `routes/cockpit/ProjectDetail.tsx`), que permanece montado enquanto a página de projeto está aberta
    — **mesmo com o popup fechado** (o popup é overlay de state local). O `ProjectBoard` **já** importa
    `boardKey` (de `lib/queries/objectives.ts`) e tem `board.project.id` (`projectId`). **NÃO**
    pôr o efeito no `TaskTimeControl` (ele desmonta ao fechar o popup → perderia o stop pela TimerBar/
    troca/auto-corte com o popup fechado — corrida real do 30-H5). Quando o `state` do timer transiciona
    de "rodando numa entry com `entry.project_id === projectId`" → **ocioso** (`state===null`/
    `entry===null`), invalidar `boardKey(workspaceId, projectId)` (de `lib/queries/objectives.ts` —
    **NÃO** existe `lib/queries/board.ts`) pra o `tracked_seconds` convergir sem reload. **Gatilho
    exato:** `useRef` guarda o `project_id` da entry rodando anterior; invalida **uma vez** quando o
    anterior era `projectId` e o atual é `null` (sem loop de re-render). Como o `state` só atualiza no
    `apply` da resposta do servidor (anti-stale da fase 04), a invalidação ocorre **após** o stop
    confirmar no servidor (entry contabilizada no `/board`). Reusa
    `queryClient.invalidateQueries({queryKey: boardKey})`.
  - Spec (`ProjectBoard.spec` / efeito do observador): simular `state` rodando numa entry deste projeto
    → depois ocioso **com o popup já FECHADO** (o `TaskTimeControl` desmontado) → assertar
    `invalidateQueries` chamado com `boardKey(ws, projectId)` (CA-13/30-H5 — cobre o caso "stop pela
    TimerBar, popup fechado"). NÃO invalida quando o running era de **outro** projeto; NÃO dispara a
    cada render (transição única).
  - **Acceptance:** spec red→green; a mutação que mova o observador pra dentro do `TaskTimeControl`
    (desmontável) faria o caso "popup fechado + stop pela TimerBar" falhar.

- **T-08.1 — Costura: substituir o bloco Time inline da fase 24 pelo `TaskTimeControl`** (CA-20, decisão B)
  - `frontend/src/components/board/TaskPanelProps.tsx` (ALTERA, **só o bloco Time**): a fase 24
    renderiza o bloco Time INLINE (`tracked_seconds` real + botão **Start** mock/desabilitado com
    `data-testid="task-start"` — fase 24 CA-26). **NÃO existe** slot `TaskTimeControl` provido pela 24.
    A fase 30 **substitui esse bloco Time inline** por `<TaskTimeControl task={...} workspaceId={...}
    canWrite={board.project.can_write} />`, reaproveitando o **mesmo botão Start** (`data-testid=
    "task-start"`) — o `TaskTimeControl` passa a renderizar o Start real (CA-10) ou o Parar (CA-11)
    nesse elemento. **Única** edição sancionada da fase 30 em `TaskPanelProps.tsx`: título/Status/
    Priority/Due/Notion/descrição/sidebar ficam intocados (da 24/25; a 25 mexe em linhas distintas, sem
    colisão). O componente de popup central da fase 24 não é tocado.
  - Spec: o popup (fase 24) renderiza o `TaskTimeControl` no lugar do bloco Time mock; o botão
    `data-testid="task-start"` agora dispara `start` (não é mais inerte). Reusa o spec do popup da 24
    onde possível (sem reescrever o popup).
  - **Acceptance:** spec red→green (o `task-start` da 24 era mock; vira real). Se a 24 não tiver
    deixado o `data-testid="task-start"` estável, registrar no handoff e alinhar com a 24 (decisão B).

- **T-09 — (OPCIONAL) indicador "cronometrando agora" no card/lista** (CA-17, D-10)
  - SE a fase 24 expuser um slot no card/linha de task: indicador discreto (`.tt-pulse`/ponto
    accent) quando `useTimerStore.getState().state?.entry?.task_id === task.id`. Some quando o timer
    para. Sem fetch próprio.
  - Spec (se implementado): card com a task rodando → indicador presente; outra/none → ausente.
  - **Acceptance:** OPCIONAL — se virar custo, sai sem furar a spec (CA-17 é `optional`). Se feito,
    spec red→green.

> **Fim do bloco 2:** Vitest 100% verde (TaskTimeControl + TimerBar); `make test-back` verde
> (regressão zero). Playwright = bloco 3.

## Bloco 3 — E2E + verificação + gate (T-10 → T-12)

- **T-10 — E2E `elo-timer-task.spec.ts`** (CA-13/16/18)
  - Logado+escrita (stub fase 01) num projeto `shared`: abrir uma task no popup (fase 24); o
    controle de tempo (o `TaskTimeControl` no lugar do bloco Time mock) mostra o tracked atual +
    **Start** (`data-testid="task-start"`); clicar Start → a TimerBar global mostra o **chip da task**
    (CA-16) e o controle do popup vira **Parar** (CA-11); clicar **Parar** → o `tracked_seconds` da
    task **sobe** no board/popup após a invalidação (CA-13, o tempo grudou). **Caso 30-H5:** iniciar
    pela task, **FECHAR o popup**, parar pela **TimerBar global** → reabrir o popup/ver o board → o
    `tracked_seconds` **subiu** (o observador no `ProjectBoard` invalidou mesmo com o popup fechado).
    Controle: um timer **sem task** (label livre pela TimerBar) NÃO mostra chip de task (CA-18).
  - Reusar `loginViaStub`; nomes únicos (banco reusado, idempotente); **liberar a :8080** antes do
    E2E; **mutex do timer global** (`e2e/timerLock.ts` da fase 23) — este spec MUTA o timer global,
    então herda o `beforeAll/afterAll` do lock pra não brigar com `cronometro.spec`/`remoto.spec`
    sob `fullyParallel` (lição da fase 23).
  - **Acceptance:** E2E verde; red honesto (antes de fiar o botão real, o Start mock não move a
    TimerBar — documentar no cabeçalho do spec se o popup já existir da 24).

- **T-11 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: não-regressão do TRACKING [label livre/Pomodoro/sync/refs],
    estado rodando×tracked [dupla contagem / elapsed de outra task], gating viewer [Parar preso],
    chip da TimerBar [degradação / caminho sem-task], invalidação do board [tempo não gruda /
    invalida demais], coerência project_id/task_id da entry) + refutação dupla. Corrigir
    confirmados red→green. Registrar no §"Furos da verificação adversarial".

- **T-12 — Gate + handoff** (deploy = decisão do orquestrador; M3 pode ter deploy único no fim)
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Antes do E2E,
    liberar a :8080** (dev server alheio) — lição do KICKOFF.
  - Smoke honesto se deployar (decisão do orquestrador): `/healthz` 200; o start com `task_id`
    continua 401 sem sessão; bundle contém o chip da task / `TaskTimeControl`; PrimaOS/Caddy
    intactos. **NÃO** tocar Caddy/PrimaOS/portas 80-443.
  - Handoff (CLAUDE.md "retomar..."), INDEX (30 construída, **sem migração**), memória, commit,
    briefing pro Diogo. Registrar que o backend ficou byte-idêntico (RNF-01) e o elo é frontend.

---

## Blocos (workflow-first; main loop sequencial)

- **Bloco 1 — backend (pinar contrato, zero produção nova):** T-01 → T-02. Suíte: `make test-back`.
- **Bloco 2 — frontend (o elo):** T-03 → T-08.1 → T-09. Suíte: Vitest + `make test-back` (regressão).
  (T-08.1 = costura: substituir o bloco Time inline da fase 24 pelo `TaskTimeControl`.)
- **Bloco 3 — E2E + fechamento:** T-10 → T-12 (E2E, verificação adversarial, gate `make
  check-full`).

`make check-full` é gate no T-12; os blocos 1 e 2 fecham com a suíte do que existe verde + commit.

---

## Matriz CA → tarefa (rastreio do red→green)

| CA | Tarefa(s) | Camada |
|---|---|---|
| CA-01 start grava project+task | T-01 | Go (não-regressão) |
| CA-02 ref inválida → 404 not_found | T-02 | Go (não-regressão) |
| CA-03 project_id inferível da task | T-01 | Go |
| CA-10 Start chama o store | T-03 | Vitest |
| CA-10.1 description = task.name | T-03 | Vitest |
| CA-11 rodando nesta task → Parar | T-04 | Vitest |
| CA-11.1 tempo vivo = tracked+elapsed | T-04 | Vitest |
| CA-12 trocar de task | T-05 | Vitest |
| CA-13 board invalida (o tempo gruda) | T-08, T-10 | Vitest + E2E |
| CA-14 gating viewer (Parar não prende) | T-06 | Vitest |
| CA-15 Start inerte sem project_id | T-03 | Vitest |
| CA-16 chip da task na TimerBar | T-07, T-10 | Vitest + E2E |
| CA-17 indicador "cronometrando" (opc.) | T-09 | Vitest (opcional) |
| CA-18 label livre intocado | T-07, T-10 | Vitest + E2E |
| CA-19 Pomodoro/sync não quebra | T-01 | Go + suíte existente |
| CA-20 `TaskTimeControl` é o contrato + costura no `TaskPanelProps.tsx` | T-03..T-08, **T-08.1** | Vitest |

---

## Furos da verificação adversarial

Verificação adversarial das specs (ETAPA 1) — 3 furos CONFIRMADOS 2/2, todos aplicados nas specs
ANTES do build:

- **30-H1 (MEDIUM · contrato/corretude)** — o plan mandava renderizar o chip da task com
  `<Icon.board size={12} />`, mas a chave `board` **NÃO existe** em `frontend/src/components/icons.tsx`
  (chaves reais: arrow/bolt/briefcase/cal/chart/check/chevron/cliente/clock/cog/dots/focus/folder/
  globe/grid/link/list/mail/pause/play/plus/search/shield/stop/table/today/trash/users/x). `Icon.board`
  daria `undefined`/erro de TS e quebraria a `TimerBar` (compartilhada com o TRACKING → regressão geral,
  RNF-01). **Resolvido:** o chip da task vira `<span className="tt-chip">{taskName}</span>` **sem
  ícone** (espelha o chip de projeto da `TimerBar.tsx:177`, que é `tt-chip` só com texto). Atualizados
  plan §4.3 (snippet + nota anti-armadilha), plan §5 (lista de tokens reusados — `Icon.play`/`Icon.stop`
  só; sem `Icon.board`) e tasks.md T-07. CA-16 não muda (asserta o nome da task, não o ícone).

- **30-H2 (HIGH · consistência cross-fase)** — a fase 30 assumia que a fase 24 montava
  `<TaskTimeControl/>` num "slot da propriedade Time", mas a fase 24 renderiza o **bloco Time INLINE**
  em `TaskPanelProps.tsx` (`tracked_seconds` real + botão **Start** mock — CA-26), **sem** slot nem
  referência a `TaskTimeControl`. O veto "NÃO reescrever a propriedade Time da fase 24" contradizia a
  costura real. **Resolvido (decisão de coordenação B):** a fase 30 **substitui o bloco Time inline**
  de `TaskPanelProps.tsx` pelo `TaskTimeControl`, reaproveitando o **mesmo botão Start**
  (`data-testid="task-start"` — fase 24) e trocando só a ação (mock → `useTimerStore.start` real) + o
  estado; é a ÚNICA edição sancionada da 30 em `TaskPanelProps.tsx` (resto do popup intocado). NÃO
  existe slot provido pela 24. Atualizados: spec §1 (fronteira), spec nota antes do CA-10, spec CA-20,
  spec §6; plan §1 (arquitetura + prosa), plan §5 (tabela ganhou `TaskPanelProps.tsx` ALTERA só o bloco
  Time), plan §6 (veto reescrito), plan §7 (dependências); tasks.md ganhou **T-08.1** (costura) +
  matriz CA-20.

- **30-H5 (MEDIUM · regressão/RNF)** — CA-13 prometia invalidar o `boardKey` por efeito **dentro do
  `TaskTimeControl`**, mas esse componente **desmonta ao fechar o popup**: parar pela TimerBar/troca/
  auto-corte com o popup já fechado não dispararia a invalidação (o board não convergiria sem reload —
  corrida real). Além disso o plan citava `lib/queries/board.ts`, que **não existe** (`boardKey`/
  `useBoard` moram em `lib/queries/objectives.ts`). **Resolvido:** o observador da transição
  "rodando-numa-entry-deste-projeto → ocioso" mora no **`ProjectBoard.tsx`** (via `ProjectDetail`), que
  sobrevive enquanto a página de projeto está aberta. Gatilho exato: invalidar `boardKey(ws, projectId)`
  **uma vez** quando o `project_id` da entry rodando passa de `projectId` → `null` (ocioso), com `useRef`
  guardando o valor anterior (sem loop de re-render); como o `state` só atualiza no `apply` da resposta
  do servidor (anti-stale da fase 04), a invalidação ocorre **após** o stop confirmar no servidor.
  Atualizados: spec CA-13, plan §3 D-07, plan §4.2 (nota) + nova §4.4 (snippet do observador no
  ProjectBoard), plan §1 (arquitetura cita `ProjectBoard.tsx` ALTERA + `lib/queries/objectives.ts`),
  plan §5 (tabela); tasks.md T-08 (observador no ProjectBoard, teste do caso "popup FECHADO + stop pela
  TimerBar") + T-10 (caso E2E 30-H5).

Verificação adversarial do BUILD (ETAPA 3 — workflow ultracode, 6 dimensões × dupla refutação, 14 agentes):
4 achados brutos → **0 confirmados 2/2**, MAS os 4 convergiram no MESMO gap real (adjudicado manualmente e
corrigido, decisão HITL de qualidade):

- **30-BUILD-1 (LOW · cobertura/coerência de spec)** — `useBoardTimerSync` só invalidava o board na transição
  **→ ocioso** (`entryProjectId === null`). **Trocar** o timer de uma task pra outra (Start atômico que fecha a
  anterior — CA-05/CA-12) **sem** passar por ocioso NÃO invalidava: a entry de A fechava e seu `tracked_seconds`
  mudava, mas o board ficava stale até ir a ocioso/reload. Os refutadores marcaram "refutado" porque o código
  era **fiel à letra** do "Gatilho exato" da spec (`state === null`) — mas a **prosa do CA-13 lista "por troca"**
  como Stop que deve refrescar, e US-02 ("o tempo gruda na task") quer a troca cobrindo. Tensão interna da spec.
  **Corrigido (red→green, mutação provada):** o observador passa a comparar a **IDENTIDADE da entry** (`entry.id`)
  — invalida quando a entry rodando muda de identidade E a anterior era deste projeto (cobre parar **e** trocar).
  Sem regressão em stop/pause (durante o break `state.entry` é `nil` — `service.go:532` — logo o id vai a `null`
  igual ao antigo). +2 testes (troca mesmo-projeto / troca-pra-outro-projeto). **CA-13/D-07 reconciliados abaixo.**

**Decisões de coordenação cross-fase aplicadas:** **B** (a 30 wira o botão Start MOCK que a 24 criou —
mesmo elemento `data-testid="task-start"`; NÃO assume slot `TaskTimeControl` provido pela 24);
**A** (o popup central é componente React da fase 24 — referenciado como `TaskPanel.tsx`/
`TaskPanelProps.tsx`, não classes cruas do mockup); **H** (stores zustand em `frontend/src/lib/` —
`timerStore.ts`; `boardKey`/`useBoard` em `lib/queries/objectives.ts`; `ProjectCard` em
`components/projects/`). **G** (códigos de erro canônicos): a fase 30 já estava correta — o erro de ref
inválida do start é `404 not_found` (herdado da fase 03), NÃO `400 invalid_reference`; nenhuma mudança
necessária.
