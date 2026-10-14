# spec.md — Fase 30 · Elo timer↔task ("Start" na task cronometra puxando-a)

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Última fatia da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §9 ponto 2`).
> Liga o **elo** entre os dois mundos (`MILESTONE-3.md §2`): cronometrar **puxando** uma task
> planejada — o tempo **gruda** na task. Cobre **RF-10** ("Time mostra o tempo rastreado real +
> botão **Start** que cronometra a task").
>
> **Decisão-chave (ancorada no código real, verificada antes de escrever):** o backend **JÁ
> suporta o elo**. `timer.StartInput` (`backend/internal/timer/service.go:62`) já tem
> `TaskID *string` **e** `ProjectID *string` (ambos opcionais/nullable); `Start` chama
> `entries.ValidateRefs` (`backend/internal/entries/refs.go`) que confere cada ref contra o
> workspace e grava `time_entries.task_id`. O `/board` (fase 12) **já agrega** `tracked_seconds`
> por `task_id`. Logo esta fase é **majoritariamente FRONTEND** — fiar o botão **Start** do popup
> de task (fase 24) no `useTimerStore` existente, refletir o estado na TimerBar e no
> card/lista, **sem regredir** o TRACKING (RNF-01). **SEM migração** (a menos que a verificação
> adversarial ache um gap mínimo — então documenta, não implementa).
>
> **Decisões travadas (MILESTONE-3 §2/§3/§6):**
> 1. **TRACKING intocado** (RNF-01): o cronômetro/sync/Pomodoro/foco rodam idênticos; o elo só
>    ADICIONA o caminho "iniciar a partir da task" e a reflexão de estado.
> 2. **Captura < 2s** (RNF-02): iniciar a task pelo botão Start é **1 gesto** (sem escolher
>    projeto — o projeto vem da própria task).
> 3. **À prova de culpa** (RNF-03): nenhum estado vermelho acusatório; iniciar/parar é reversível.

---

## 1. Mapa pro problema central (MILESTONE-3 §2 + RF-10 + §9 ponto 2)

O Timetrack tem **dois mundos** (MILESTONE-3 §2): **TRACKING** (cronômetro = "o que faço agora")
e **PLANNING** (tasks/notas/objetivos). Hoje eles são desconexos no popup: no mockup
(`mockups/projeto-workspace.html:527`) a propriedade **Time** mostra o tempo rastreado, mas o
botão **Start** é **mock** (`onclick="alert('mock: would start tracking this task')"`).

Esta fase **fecha o elo**: clicar **Start** no popup de task chama o `startTimer` existente com o
`project_id` e o `task_id` **da própria task**; o tempo cronometrado **gruda** na task (o backend
grava `time_entries.task_id`, e o `/board` da fase 12 já o agrega em `tracked_seconds`). É a
**evolução natural** anunciada no PRD: o `start` ganha um ponto de entrada a partir da task quando
o popup já existe.

**Fronteira com a fase 24 (reconciliada — ver decisão de coordenação B):** a fase 24 (Tasks 2.0)
é dona do **popup central de task** (`TaskPanel.tsx`/`TaskPanelProps.tsx`), do **componente de popup
central** (introduzido pela fase 24 — side↔full), e renderiza o **bloco Time** INLINE dentro de
`TaskPanelProps.tsx` (`tracked_seconds` real do `/board` + botão **Start** mock/desabilitado, com um
elemento identificável — `data-testid="task-start"` — fase 24 CA-26). **Não existe** um slot
`TaskTimeControl` provido pela 24. A fase 30 **substitui o bloco Time inline** de `TaskPanelProps.tsx`
pelo componente `TaskTimeControl` (CA-20), **wirando o mesmo botão Start** que a 24 criou (troca a
ação mock → `useTimerStore.start` real) e adicionando o estado "rodando" + reflexão na TimerBar. Essa
substituição do bloco Time é a **única** edição sancionada da fase 30 em `TaskPanelProps.tsx`; o resto
do popup/layout/demais propriedades fica **intocado** (da fase 24/25).

## 2. User stories

- **US-01** — Como usuário, quero **clicar Start no popup de uma task** e que o cronômetro comece
  já apontando pra essa task (sem eu escolher projeto), pra cronometrar o que planejei em 1 gesto
  (captura < 2s — RNF-02).
- **US-02** — Como usuário, quero que o **tempo cronometrado gruda na task** (apareça no
  `tracked_seconds` da task no `/board` e nos relatórios), pra que planejamento e tempo real sejam
  a mesma coisa (o diferencial da M3 — MILESTONE-3 §1).
- **US-03** — Como usuário, quando o timer **já está rodando NESTA task**, quero que o popup mostre
  **"rodando"** com o tempo ao vivo e um botão **Parar** (em vez de Start), pra eu não iniciar duas
  vezes e poder parar de onde estou.
- **US-04** — Como usuário, quando o timer está rodando **em OUTRA task** (ou em label livre) e eu
  clico **Start** numa task diferente, quero que o cronômetro **troque** pra nova task (fecha a
  entry anterior, abre a nova) — comportamento idêntico ao "trocar de timer" que já existe (CA-05
  da fase 03), pra eu pular de tarefa sem parar manualmente.
- **US-05** — Como usuário, quero ver **na TimerBar global** que o timer está apontando pra uma
  task (chip da task), pra ter object permanence do que está sendo cronometrado agora
  (MILESTONE-3 lei TDAH).
- **US-06** — Como usuário, quero que o **timer sem task (label livre)** continue funcionando
  **idêntico** ao de hoje, pra eu não perder o fluxo rápido (RNF-01).
- **US-07** — Como viewer (sem escrita no projeto), quero que o botão Start **não** apareça/aja na
  task que não posso cronometrar, mas que **Parar** o timer global continue acessível (não prender
  o timer — padrão da fase 23).

## 3. Critérios de aceite (EARS)

> Identificadores de código em inglês; prosa em PT-BR. O backend é **byte-idêntico** salvo o que o
> CA-30/31 explicitamente sancionarem (provável: nada — só um teste de não-regressão). O elo é
> **frontend** sobre contratos provados.

### Backend — confirmação do contrato (sem mudança esperada)

- **CA-01** *(ubíquo — contrato existente)* — O `POST /api/v1/timer/start` DEVE aceitar um payload
  com `task_id` **e** `project_id` simultâneos e iniciar uma entry com **ambos** gravados em
  `time_entries` (`project_id` e `task_id` da entry = os do payload). Já é o comportamento de
  `timer.Service.Start` (`service.go:227` → `entries.Insert` com `ProjectID`/`TaskID`); este CA
  **pina** que iniciar pela task (front manda os dois — CA-20) grava o vínculo. Teste de
  não-regressão (Go); nasce verde (RNF-01).
- **CA-02** *(unwanted — ref de outro workspace)* — SE o `task_id` (ou `project_id`) do payload
  não pertencer ao workspace da rota, for inexistente ou for um uuid **malformado**, o
  `POST /timer/start` NÃO DEVE criar entry e DEVE retornar o erro canônico de referência inválida
  **idêntico ao de hoje** — `404 error.code="not_found"` (o `entries.ValidateRefs` →
  `domain.NotFound`; `service.go:266`). **Importante:** o comportamento herdado da fase 03 é
  `not_found` (404), **não** `invalid_reference` (400) — esta fase **não** muda isso (RNF-01); o
  CA só **pina** o caminho pra garantir que o elo não introduza vazamento nem 500. Teste Go de
  não-regressão.
- **CA-03** *(state-driven — project_id inferível da task)* — Como o front sempre envia o
  `project_id` **da própria task** junto do `task_id` (CA-20), o `project_id` da entry resultante
  DEVE ser o `project_id` da task. **`ValidateRefs` NÃO cruza** task↔project (cada ref é checada
  contra o workspace isoladamente — `refs.go:21`), então a consistência task/projeto vem da
  **origem** (o objeto da task), não de uma validação cruzada nova. Decisão registrada no plan
  (D-01): **não** adicionar validação cruzada nova (escopo mínimo, RNF-01). Teste Go pina que
  `start({project_id: task.project_id, task_id: task.id})` grava os dois coerentes.

### Frontend — botão Start do popup (elo)

> O popup de task (fase 24) renderiza o **bloco Time INLINE** em `TaskPanelProps.tsx`
> (`tracked_seconds` real + botão **Start** mock/desabilitado com `data-testid="task-start"` — fase 24
> CA-26). A fase 30 **substitui esse bloco Time inline** por um componente `TaskTimeControl`
> (CA-20), reaproveitando o mesmo botão Start: troca a **ação** (mock → `startTimer` real) e o
> **estado visual** (Start/Parar + tempo ao vivo). NÃO existe slot `TaskTimeControl` provido pela 24
> (decisão de coordenação B); a costura é a substituição do bloco Time dentro de `TaskPanelProps.tsx`.

- **CA-10** *(event-driven — Start a partir da task)* — QUANDO o usuário **com escrita no projeto**
  da task clica no botão **Start** (Iniciar) do controle de tempo do popup, o sistema DEVE chamar
  `useTimerStore.start({ workspace_id, project_id: task.project_id, task_id: task.id, description:
  task.name, tags: [] })` (mode default `countup`) — **reusando** o store global, **sem** criar um
  segundo `attachTimerSync` nem fetch próprio (padrão da fase 23). O botão dispara a ação
  fire-and-forget (`void start(...)`), nunca rejeita (CA-17/17.1 do timerStore).
- **CA-10.1** *(state-driven — descrição da entry)* — A `description` da entry iniciada pela task
  DEVE ser o **nome da task** (`task.name`), pra a entry ser legível na TodayList/relatórios sem o
  usuário digitar nada (captura < 2s — RNF-02). Decisão de produto registrada no plan (D-02).
- **CA-11** *(state-driven — popup quando o timer já é DESTA task)* — ENQUANTO o timer global
  estiver rodando e a entry rodando apontar pra **esta** task (`state.entry.task_id === task.id`),
  o controle de tempo do popup DEVE exibir o estado **rodando**: o tempo **ao vivo** (o
  `tracked_seconds` persistido da task **+** o elapsed da entry rodando, derivado de
  `useTimerClock`) e um botão **Parar** (Stop) no lugar do **Start**. Clicar **Parar** chama
  `useTimerStore.stop()` (`void stop()`).
- **CA-11.1** *(state-driven — tempo ao vivo da task)* — O tempo exibido no controle DEVE somar:
  (a) o `tracked_seconds` da task (do `/board`, fase 12 — tempo das entries **fechadas**) **mais**
  (b) o `elapsed` da entry rodando **quando** ela é desta task. Quando o timer **não** roda nesta
  task, mostra **só** (a) (formato `fmtDayTotal`, ex.: `2h12` — igual ao mockup `fmtH`). O tempo ao
  vivo NÃO persiste sozinho; ele converge ao `tracked_seconds` quando a entry fecha e o `/board`
  refaz a query (CA-13).
- **CA-12** *(event-driven — trocar de task/label pra esta task)* — QUANDO o timer global está
  rodando em **OUTRA** task ou em **label livre** (sem task) e o usuário clica **Start** nesta
  task, o sistema DEVE chamar `start(...)` com os refs desta task; o backend fecha a entry anterior
  e abre a nova na MESMA transação (CA-05 da fase 03, `closeRunning`), e o popup reflete **rodando
  nesta task** após o `apply`. Nenhuma entry fica órfã (garantia herdada do backend; o front só
  dispara o `start`).
- **CA-13** *(event-driven — o tempo gruda; refetch do board)* — QUANDO uma entry que apontava pra
  uma task **deste projeto** é **parada** (Stop) — pelo popup, pela TimerBar, por troca ou auto-corte
  — o `tracked_seconds` da task DEVE refletir o tempo somado **após** o board/queries serem
  invalidados. O observador da transição DEVE morar no **`ProjectDetail.tsx` (`ProjectDetailPage`)**
  — que permanece montado enquanto a página de projeto está aberta, **mesmo com o popup fechado** —, e
  NÃO no `TaskTimeControl` (que desmonta ao fechar o popup). *(O `ProjectBoard.tsx` citado no design
  original NÃO existe pós-fase-28 — o host always-mounted é o `ProjectDetailPage`; ver tasks.md.)*
  **Gatilho exato (reconciliado pela verificação do build — 30-BUILD-1):** invalidar
  `boardKey(ws, projectId)` na transição em que a **entry rodando muda de IDENTIDADE** (`entry.id`) **e**
  a entry **anterior** era deste projeto (`prevEntry.project_id === projectId`). Isso cobre os DOIS Stops
  que o CA-13 lista: (i) **parar** → ocioso/pausa (`state.entry === null`) **e** (ii) **trocar** de task/
  projeto (Start atômico que fecha a anterior — CA-05/CA-12), em que a entry deste projeto fechou sem
  passar por ocioso. A transição é observada via `state` do `useTimerStore`, que o anti-stale da fase 04
  só atualiza no `apply` da resposta do servidor — logo a invalidação ocorre **após** o fechamento da
  entry estar contabilizado no `/board`. NÃO invalidar quando a entry anterior era de **outro projeto**,
  nem ao **iniciar do ocioso** (a entry anterior não era deste projeto), nem a cada render (a invalidação
  dispara **uma vez** por transição — `useRef` guarda `(id, project_id)` anteriores; sem loop). Isso
  garante que parar/trocar pela TimerBar/auto-corte **com o popup já fechado** faz o `tracked_seconds`
  convergir **sem reload** (padrão de invalidação local da fase 12). Pina que o tempo "gruda" (US-02).
  *(O `TaskTimeControl` PODE manter um efeito local só pra UX enquanto montado, mas a convergência do
  board NÃO depende dele — depende do observador no `ProjectDetailPage`.)*
- **CA-14** *(state-driven — gating de viewer/sem-escrita)* — ENQUANTO o usuário **não** tiver
  escrita no projeto da task (`board.project.can_write === false` — viewer do ws, `access=view`,
  ou projeto arquivado), o controle de tempo do popup NÃO DEVE exibir o botão **Start** (não pode
  iniciar tempo numa task que não pode cronometrar). PORÉM, se o timer global **já** estiver
  rodando nesta task (caso de borda: o estado mudou em outro device), o botão **Parar** DEVE
  continuar visível — **não prender o timer** (padrão CA-11 da fase 23). O tempo (leitura) é sempre
  exibido.
- **CA-15** *(unwanted — task sem project_id)* — SE a task, por qualquer motivo, não tiver
  `project_id` resolvível (caso teórico — toda task tem projeto na fase 02), o controle NÃO DEVE
  chamar `start` sem `project_id` válido; o botão Start fica **inerte** (sem efeito) — nunca dispara
  uma requisição inválida. (Defesa; na prática `task.project_id` sempre existe.)

### Frontend — reflexão na TimerBar e no card/lista

- **CA-16** *(state-driven — chip da task na TimerBar)* — ENQUANTO o timer global estiver rodando
  com uma entry que tem `task_id`, a `TimerBar` (RunningBar) DEVE exibir um **chip da task** (nome
  da task) **além** do chip do projeto já existente, pra dar object permanence do que se cronometra
  agora (US-05). Quando a entry **não** tem `task_id` (label livre), a TimerBar fica **idêntica** à
  de hoje (só o chip do projeto, se houver — RNF-01). O nome da task é resolvido por uma query de
  tasks do projeto da entry (padrão do chip de projeto que já usa `useProjects({archived:true})` —
  CA-31 da fase 03); se o nome não resolver, **degrada** (só o chip do projeto), nunca quebra.
- **CA-17** *(optional — indicador "cronometrando agora" no card/lista)* — ONDE o board/lista de
  tasks (fase 24) exibe uma task que é a **entry rodando** (`state.entry.task_id === task.id`), o
  sistema PODE exibir um indicador discreto "cronometrando agora" (ex.: ponto pulsante / ícone de
  timer) no card/linha. **Opcional** (não bloqueia o gate); se implementado, deriva do
  `useTimerStore` global, sem fetch próprio, e some quando o timer para. À prova de culpa
  (RNF-03): é informativo, não acusatório.

### Não-regressão do TRACKING (RNF-01 como CA testável)

- **CA-18** *(unwanted — label livre intocado)* — O fluxo de iniciar timer **sem task** (label
  livre pela TimerBar/IdleBar, pelo `/remoto`, pelo Foco) DEVE permanecer **byte-idêntico** ao de
  hoje: mesmo payload, mesma UI, mesmo resultado. Pina que o elo NÃO mexeu no caminho sem task
  (teste de componente + o `cronometro.spec` E2E existente continua verde).
- **CA-19** *(unwanted — ciclo Pomodoro/sync não quebra)* — Iniciar um timer a partir de uma task
  (mode `countup`) NÃO DEVE alterar o ciclo Pomodoro nem o sync realtime: a entry iniciada pela
  task é um `countup` normal; pausar/retomar/auto-transição e o broadcast WebSocket (fase 04)
  seguem idênticos. Pina (teste do timerStore) que `start` com `task_id` não muda nada além do
  `task_id`/`project_id`/`description` da entry. A suíte existente do `timerStore`/`realtime`
  continua verde (regressão zero).

### Contrato do front (consumido pela fase 24)

- **CA-20** *(ubíquo — `TaskTimeControl` é o contrato)* — A fase 30 DEVE entregar um componente
  reusável `TaskTimeControl` (em `components/board/`) que recebe a task (com `tracked_seconds`),
  o `workspace_id` e o `can_write` do projeto, e renderiza **só** a leitura de tempo + o
  botão (Start/Parar) com o comportamento dos CA-10..15. A fase 30 **SUBSTITUI o bloco Time inline**
  de `TaskPanelProps.tsx` (entregue mock/desabilitado pela fase 24 — CA-26) por `<TaskTimeControl/>`,
  reaproveitando o **mesmo botão Start** (`data-testid="task-start"`) que a 24 criou — só troca a ação
  (mock → real) e o estado. Essa substituição do bloco Time é a **ÚNICA** edição sancionada da fase 30
  em `TaskPanelProps.tsx` (resto do popup intocado). NÃO existe slot provido pela 24 (decisão de
  coordenação B). O componente é **autossuficiente** (lê o `useTimerStore` global), não exige props de
  callback do timer.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-30.1 | Botão Start do popup cronometra a task (reusa o `startTimer` global) | CA-10, 10.1, 12, 15, 20 |
| RF-30.2 | O tempo gruda na task (grava `task_id`, reflete no `/board`) | CA-01, 03, 13 |
| RF-30.3 | Estado "rodando nesta task" no popup (Stop + tempo ao vivo) | CA-11, 11.1 |
| RF-30.4 | Reflexão na TimerBar (chip da task) + indicador opcional no card | CA-16, 17 |
| RF-30.5 | Gating de viewer/sem-escrita (Start gated, Stop nunca prende) | CA-14 |
| RF-30.6 | Não-regressão do TRACKING (label livre, Pomodoro, sync, refs) | CA-02, 18, 19 |

**Cobre o RF do PRD:** **RF-10** (MILESTONE-3 §5.2 — "Time mostra o tempo rastreado real + botão
**Start** que cronometra a task — elo timer↔task, §9") **e** o §2 (o elo entre os dois mundos).

## 5. Requisitos não-funcionais

- **RNF-01 (não regredir o TRACKING)** — cronômetro/sync/Pomodoro/foco **byte-idênticos** onde não
  for o elo. O backend do timer **não muda** (o contrato já suporta `task_id`); a única adição
  frontend é o botão real + a reflexão de estado, **reusando** o `useTimerStore`. Provado por
  CA-18/19 e pela suíte existente (timerStore/realtime/cronometro.spec) verde.
- **RNF-02 (captura < 2s)** — iniciar pela task é **1 gesto** (sem escolher projeto/data); a
  `description` nasce do nome da task (CA-10.1).
- **RNF-03 (à prova de culpa)** — nenhum estado vermelho acusatório; o indicador de
  "cronometrando" (CA-17) é informativo; parar/trocar é reversível.
- **RNF-04 (sem fetch novo no caminho do timer)** — o controle do popup e a TimerBar derivam do
  `useTimerStore` global (1 só `attachTimerSync`, já no boot); o nome da task na TimerBar sai de
  uma query de tasks já cacheada (React Query), sem request por render. **Sem** 2º socket (padrão
  fase 23).
- **RNF-05 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): o botão usa
  `.tt-btn--accent`/`.tt-iconbtn`, o chip da task usa `.tt-chip`/`.tt-dot`, o indicador opcional
  usa `.tt-pulse`/`--accent` — nada de hex literal. Inter na UI (a M3 já assume; a troca
  Geist→Inter é da fase 24).
- **RNF-06 (sem migração)** — nenhuma migração nesta fase; o schema já tem `time_entries.task_id`
  (fase 03) e o `/board` agrega por task (fase 12). Se a verificação adversarial achar um gap
  mínimo (ex.: garantir `project_id` preenchido ao iniciar por task), **documentar**, não
  implementar sem reabrir.
- **RNF-07 (mobile)** — o controle do popup e o chip da TimerBar reflowam < md (breakpoint 768 da
  M2); o `/remoto` (fase 23) já mostra o timer rodando — herda a reflexão sem mudança própria.
- **Segurança/multi-tenant** — toda chamada de start atrás de sessão+membership; refs validadas
  pelo `ValidateRefs` existente (CA-02). Gating de escrita por `board.project.can_write` (CA-14).
- **Gate de fase:** `make check-full` verde.

## 6. Fora de escopo (fronteiras explícitas)

- **Reescrever o popup/layout/demais propriedades da fase 24** — é da **fase 24**. A fase 30 SÓ
  substitui o **bloco Time inline** de `TaskPanelProps.tsx` pelo `TaskTimeControl` (CA-20), wirando o
  **mesmo botão Start** (`data-testid="task-start"`) que a 24 criou (mock → real) e adicionando a
  reflexão de estado. NÃO toca título/Status/Priority/Due/Notion/descrição/sidebar nem o componente
  de popup central da 24.
- **Mudar o backend do timer** (StartInput, ValidateRefs, ciclo, sync) — **proibido** (RNF-01). O
  contrato já basta; nenhuma migração.
- **Validação cruzada task↔project** nova no backend — fora (D-01): o front manda os dois da mesma
  task; `ValidateRefs` segue checando cada ref contra o workspace, sem cruzar (escopo mínimo).
- **Iniciar Pomodoro/countdown a partir da task** — o Start da task inicia **countup** (o modo
  rápido). Pomodoro/countdown têm UI própria (Foco — fase 15); fatia futura se o Diogo pedir.
- **Editar a entry/tempo da task manualmente pelo popup** — o `EntryModal` (fase 03) já edita
  entries; o popup de task não vira editor de entries nesta fase.
- **"Parar e continuar" / múltiplas sessões visíveis na task** — a task mostra o agregado
  (`tracked_seconds`) + o vivo; o histórico de sessões por task é relatório (fase 14), fora daqui.
- **Notificação / push ao iniciar pela task** — fora; o start é silencioso como hoje.
- **Elo a partir das Notas / do Spotlight** — o `/timer` do Spotlight (fase 27) já edita o timer
  atual; cronometrar *uma task específica* a partir do Spotlight é evolução futura (não bloqueia).
- **Drag/posição/propriedades da task** — das fases 24/25; intocado aqui.
