# tasks.md — Fase 27 · Spotlight (Cmd+K)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-09).
> Cada T-XX lista os CAs cobertos. **FRONTEND + 1 ALLOWANCE MÍNIMA DE BACKEND — ZERO migração**
> (RNF-01): a fase adiciona **uma** rota (`PATCH /api/v1/timer` — edição de rótulo/projeto/tags da
> entry rodando, decisão HITL 23/06; §3.5 do plan); o `entries.Patch` fica **byte-idêntico** (segue
> dando `409` em entry rodando). Ordem: **allowance de backend (TDD Go) →** núcleo puro → store +
> `editCurrent` → ações → componentes → wiring → E2E → verificação → gate.
> **Pré-requisito de build:** fases 24 (popup de task) e 26 (popup de nota) já no app
> (build order 24 → 26 → 27 — plan §8). Se um abridor faltar, degradar (CA-17).

---

## Bloco único — allowance de backend + frontend (T-00 → T-09)

- **T-00 — Allowance de backend: `PATCH /api/v1/timer` (`EditCurrent`)** (CA-13/13.1/13.2/18) — §3.5 do plan
  - **Backend (TDD Go):**
    - `internal/timer/service.go`: método novo `EditCurrent(ctx, userID, EditCurrentInput) (Result, error)`
      — `EditCurrentInput{Description *string; Tags *[]string; ProjectIDSet bool; ProjectID *string}`
      (SEM campos de tempo — CA-13.1). Sob `lockUser` (advisory lock do timer); `loadStateRow(FOR
      UPDATE)`; `ErrNoRows` **ou** `mode == "break"` ⇒ `409 no_active_timer`; `entries.GetByID` da
      entry rodando; aplica desc/tags/project_id (tri-state); `entries.ValidateRefs(tx,
      e.WorkspaceID, e.ProjectID, e.TaskID)` (project de outro ws ⇒ `404 not_found`); `entries.Update`
      (timing **intocado** — não recalcula `duration`, não mexe em `started_at`/`stopped_at`);
      `tx.Commit`; `workState(...)` + **`s.onChange(userID, st, now)`**; devolve `Result` canônico.
    - `internal/api/timer_handlers.go`: `handleTimerEdit(svc)` — `PATCH`, decode do body
      `{description?, project_id? (tri-state via RawMessage/ponteiro), tags?}` (SEM `workspace_id`),
      `svc.EditCurrent`, `respond.JSON(200, Result)`; erros via `writeDomainError`.
    - `internal/api/timer_routes.go`: `tr.Patch("/", handleTimerEdit(svc))` no `mountTimer` (ao lado
      de `Get("/")`/`Post("/start")`).
  - **Testes Go (TDD red→green):**
    - `EditCurrent` edita description/project_id/tags da entry RODANDO → `200` + `Result.State.entry`
      reflete os campos novos; **`started_at`/`stopped_at`/`duration_seconds`/`mode`/`target_seconds`
      INTOCADOS** (asserta igualdade com o estado antes — CA-13/13.1).
    - **`OnChange` é disparado exatamente 1×** com o `State` novo: registrar um hook fake via
      `svc.SetOnChange(...)` (espelha os testes da fase 04 — `realtime_*_test.go`), provar 1 chamada
      com `state.entry.description` novo (CA-13 / broadcast da fase 04).
    - mudar `started_at`/`stopped_at`/`duration` de uma entry rodando **continua `409 entry_running`**
      pelo caminho manual: teste pinando que `entries.Patch` (ou `PATCH /workspaces/{ws}/entries/{id}`)
      **não regrediu** (entry rodando → `409`) — CA-13.1, byte-idêntico.
    - `project_id` de **outro workspace** (ou inexistente/uuid inválido) → `404 not_found` via
      `ValidateRefs` (CA-13.2/18); o timer **não** muda (rollback).
    - **sem timer rodando** (ocioso) → `409 no_active_timer`; **durante `break`** → `409
      no_active_timer` (não há entry de trabalho aberta).
    - isolamento: `EditCurrent` edita só o timer do `userID` da sessão (mutação que remove o filtro
      por usuário **quebra** o teste — espelha `TestIsolamentoTimerEntreUsuarios` da fase 03).
  - **Acceptance:** `make test-back` verde; o `entries.Patch` permanece byte-idêntico (diff só
    aditivo em `timer/service.go` + `api/timer_*`); mutação que faz a allowance tocar timing **quebra**
    o teste (CA-13.1).

- **T-01 — Núcleo puro `lib/spotlight.ts`** (CA-05/06/07/08)
  - `lib/spotlight.ts`: `SPOT` (4 comandos: task/note/timer/goal — rótulo/desc/ícone, espelho do
    mockup l.625-630), `spotToken(input)` (1º token sem `/`, lowercase — l.633), `spotItems(input)`
    (filtro `act.startsWith(token) || label.toLowerCase().includes(token)` — l.633-634),
    `spotTitle(input)` (`replace(/^\/?\s*(task|note|timer|goal)\b/i,'').trim()` — l.636),
    `spotParse(input)` → `{act: SpotAct|null, title}` (act = comando do 1º token; null = sem
    comando). **Funções PURAS, sem React/DOM/fetch.**
  - Teste `spotlight.test.ts` (Vitest unit): `/ta` ⇒ `[task]`; `task` (sem barra) ⇒ `[task]`; `/x`
    ⇒ `[]` (CA-07); `/task Comprar domínio` ⇒ `spotTitle="Comprar domínio"`, `spotParse.act='task'`
    (CA-08); `Reunião amanhã` (sem comando) ⇒ `spotParse.act=null`, `title="Reunião amanhã"`
    (RF-19); `/timer` ⇒ `act='timer'`, `title=''`; `/goal Lançar v2` ⇒ `act='goal'`,
    `title="Lançar v2"`; `'  '` ⇒ `act=null`, `title=''`.
  - **Acceptance:** `vitest run lib/spotlight.test.ts` verde; nenhuma dependência de DOM.

- **T-02 — `spotlightStore` (zustand) + `editTimer`/`editCurrent`** (CA-01/04/13)
  - **`lib/spotlightStore.ts`** (decisão H/27-H4 — store em `src/lib/`, **NÃO** `src/stores/`):
    `{ open, query }` + `openSpotlight(initial?)` (seta open=true, query=initial??''), `close()`
    (open=false, query=''), `setQuery(q)`. Padrão `useTimerStore`/`presenceStore` (que já vivem em
    `src/lib/`).
  - **`lib/api.ts`:** helper novo `editTimer(input: EditTimerInput)` → `PATCH /timer` (tri-state em
    `project_id`), devolve `TimerResult` (mesmo shape de `startTimer`/`stopTimer`). **`lib/timerStore.ts`:**
    ação nova `editCurrent(input)` que passa pelo **mesmo `mutate`** das outras ações (fase 05 CA-17 —
    nunca rejeita; sucesso → `apply`; falha → `load`).
  - Teste `lib/spotlightStore.test.ts` (espelha `timerStore.spec.ts`/`presenceStore.spec.ts`, mesma
    pasta): `openSpotlight()` ⇒ open=true; `openSpotlight('foo')` ⇒ query='foo' (semente opcional);
    `close()` ⇒ open=false, query=''; `openSpotlight` com store já aberto mantém aberto (CA-04
    idempotente). **Nota (allowance 23/06):** semear `openSpotlight` com a descrição do timer rodando
    é UX **opcional** (plan §6) — não é obrigatório no teste do store.
  - Teste do `editCurrent` no `timerStore.spec.ts` (stub de fetch — espelha os testes de `start`/`stop`
    da fase 03): `editCurrent({description, project_id})` chama `PATCH /timer` com o body certo e o
    `apply` reflete o `State` da resposta; falha (rejeição do fetch) → `load()` chamado, **não rejeita**.
  - **Acceptance:** Vitest verde.

- **T-03 — Ações `lib/spotlightActions.ts` (deps mockadas)** (CA-09/10/11/12/13/14/15/16/17/18)
  - `lib/spotlightActions.ts`: `runSpot(input, ctx)` (`ctx = {ws, projectId, canWrite}`) — plan §5.
    Roteia por `spotParse(input).act`: `task`→`createTask`+abridor de task da 24 (foco);
    `note`→`createNote`+abridor de nota da 26; `timer`→`useTimerStore.getState().start(
    {mode:'countup',...})`; `goal`→`createObjective`; **`act===null`+timer-rodando+texto →
    `useTimerStore.getState().editCurrent({description:input, project_id})`** (allowance §3.5 — edita
    o timer atual; fecha); `act===null`+timer-rodando+vazio → no-op (não apaga a descrição; mantém
    aberto); `act===null`+ocioso+texto→`start(countup)`; `act===null`+ocioso+vazio→no-op. Falta
    projeto/permissão p/ task|note|goal ⇒ **não chama API** (CA-15/16). try/catch gracioso (CA-17 —
    não rejeita).
  - Teste `spotlightActions.test.ts` (mocka `../lib/api` [`createTask`/`createNote`/`createObjective`],
    `../lib/timerStore` [`useTimerStore.getState().start`/`.editCurrent`/`.state`], e os **abridores
    da 24/26** — confirmar nomes reais em T-04; **`patchEntry` de `entries` NÃO é usado** — a edição do
    timer rodando é via `editCurrent`/allowance):
    - `/task X` + ctx{projectId, canWrite:true} → `createTask` chamado com `{project_id, name:'X'}`
      no `ws` certo (CA-18); abridor de task da 24 chamado com `(novoId, true)` (foco); store fechado
      (CA-09).
    - `/note X` análogo → `createNote` + abridor de nota da 26 (CA-10).
    - `/timer` (+título) → `start({mode:'countup', description:title, project_id})`; **`createTask`
      NÃO chamado** (CA-11).
    - `/goal X` + ctx ok → `createObjective(ws, projectId, {name:'X'})` (CA-12).
    - sem comando + **timer rodando** (state.entry mockado) + texto → **`editCurrent`** chamado com
      `{description:<texto>, project_id:<contexto ou ausente>}`; `start` **NÃO** chamado, `createTask`
      **NÃO** chamado; store fecha (CA-13 — allowance §3.5). + **sem projeto de contexto** → `editCurrent`
      chamado **sem** `project_id` (edita só a descrição). + **vazio** → **NENHUMA** chamada (não apaga
      a descrição), store segue aberto (no-op).
    - sem comando + **ocioso** + texto → `start(countup)` (CA-14); + **vazio** → nenhuma chamada,
      store segue aberto (no-op).
    - `/task X` **sem projectId** → `createTask` **NÃO** chamado, store segue aberto + flag de aviso
      (CA-15). `/task X` com `canWrite:false` → `createTask` **NÃO** chamado (CA-16).
    - `createTask` rejeita → `runSpot` **resolve** (não rejeita) e não quebra (CA-17). `editCurrent`
      rejeita → `runSpot` **resolve** (não rejeita — reusa o mutate do store, CA-17). Abridor ausente
      (undefined) → degrada (navega/fallback) sem throw (CA-17/D-09).
  - **Acceptance:** Vitest verde; mutação que remove o guard de `projectId`/`canWrite` (faz a API
    ser chamada sem contexto) **quebra** o teste (CA-15/16); mutação que troca `editCurrent` por
    `start` (ou vice-versa) no ramo sem-comando **quebra** o teste (rodando→`editCurrent`,
    ocioso→`start` — CA-13/14); mutação que faz o ramo vazio chamar `editCurrent` **quebra** (no-op).

- **T-04 — Confirmar contratos REAIS: componente de popup central da 24 + abridores + `api`/`timerStore`** (integração)
  - **Sem teste novo** — passo de verificação no build: confirmar no código (decisão A) o
    **componente de popup central da fase 24** (ex.: `CenterPanel`) e o **abridor** real que a 24
    expõe (handler do shell `openTask(id, focus)` **ou** o estado de painel `panel:{type,id}` do
    shell que o Spotlight também aciona — o contrato exato deve estar **fixado no plan da fase 24**,
    não inventado aqui), o abridor de nota da 26, `createTask`/`createNote`/`createObjective`
    (`api.ts`), `useTimerStore.getState().start`/`.editCurrent`/`.state?.entry` (`lib/timerStore.ts` —
    `editCurrent` é a ação nova da T-02 que consome a allowance da T-00), `useCurrentWorkspace`
    (fase 10), `useBoard` com/sem `enabled` (`lib/queries/objectives.ts` — 27-H3/CA-16.1). **NÃO** usar
    `patchEntry` de `entries` (mexe em timing e rejeita entry rodando — a porta é `editCurrent`/
    allowance §3.5). Ajustar `spotlightActions.ts` (T-03) pros nomes reais. Se um abridor não existir
    (build fora de ordem),
    implementar o **fallback** de degradação (D-09/CA-17). Registrar os contratos confirmados num
    comentário no topo de `spotlightActions.ts`.
  - **Acceptance:** `spotlightActions.test.ts` (T-03) continua verde contra os nomes reais; nenhum
    import quebrado.

- **T-05 — `Spotlight.tsx` (conteúdo no popup central da 24 + teclado)** (CA-01/04/05/06/07/08/16.1)
  - `components/spotlight/Spotlight.tsx`: lê `useSpotlightStore`; `open=false` ⇒ `null`. Renderiza
    o conteúdo **dentro do componente de popup central da fase 24** (decisão A — scrim/card vêm de
    lá; **NÃO** classes cruas `.swrap`/`.scard`/`.mscrim` do mockup): input `#spotInput` (autofocus
    ao abrir — CA-01), lista `.spotitem` (classe própria da 27, CSS global só `var(--*)` — RNF-05)
    de `spotItems(query)` (1º selecionado — CA-06), chip `esc` no header, chip `↵` no item
    selecionado, realce do `spotTitle(query)` no 1º item p/ `/task`/`/note` (l.646). Teclado:
    `Enter`→`runSpot(query, ctx)` no item selecionado; ↑/↓ movem seleção; `Esc` fecha; clicar no
    scrim do popup central fecha (CA-04). Seleção volta a 0 a cada `setQuery` (CA-07).
  - **`ctx` (montagem do contexto):** `ws` de `useCurrentWorkspace`; **projeto de contexto =
    `matchPath('/projetos/:id', useLocation().pathname)?.params.id ?? null`** (27-H2/D-13 — **NÃO**
    `useParams()`, o Spotlight está no `AppShell`/pai); **`can_write` via `useBoard` GATEADO**
    (`enabled:!!projectId && open`, ou filho condicional renderizado só com `open && projectId` —
    27-H3/CA-16.1) — **nunca** disparar `GET /projects//board` fora de `/projetos/:id` nem fechado.
  - Specs `Spotlight.spec.tsx` (jsdom, mocka `runSpot`/`useCurrentWorkspace`/deps; envolve em
    `MemoryRouter`): aberto ⇒ input focado (CA-01); digitar `/ta` ⇒ só item `/task` (CA-07); `Esc`
    ⇒ `close()` (CA-04); clique no scrim ⇒ `close()` (CA-04); `Enter` ⇒ `runSpot` com a query atual
    e ctx (CA-09 via runSpot); ↑/↓ trocam a seleção (CA-06); título realçado aparece p/ `/task X`
    (CA-08); `open=false` ⇒ nada renderizado (CA-05).
  - **Teste obrigatório do projeto de contexto (27-H2):** montar o Spotlight **FORA da rota `:id`**
    (num `MemoryRouter` com `initialEntries=['/projetos/<id>']` e o Spotlight num elemento de
    **layout** acima do segmento `:id`, espelhando o `AppShell`) e provar que `runSpot` recebe
    `ctx.projectId === '<id>'` resolvido do **pathname** — mata a mutação que troca
    `matchPath`/`useLocation` por `useParams()` (que devolveria `null`).
  - **Teste obrigatório do gate de `useBoard` (27-H3/CA-16.1):** com o Spotlight **fechado** e/ou
    em rota fora de `/projetos/:id`, **nenhum** `getBoard`/`GET /projects//board` é disparado
    (mockar/spy na chamada do board) — mata a mutação que remove o `enabled`/guard.
  - **Acceptance:** Vitest verde.

- **T-06 — `SpotlightButton` + `useSpotlightHotkey` + wiring no shell** (CA-01/01.1/02/03)
  - `components/spotlight/SpotlightButton.tsx` (`.tt-btn`, aria "Abrir captura (Cmd+K)", `onClick=
    openSpotlight()`). `components/spotlight/useSpotlightHotkey.ts` (effect: `keydown` →
    `(meta||ctrl)&&key==='k'` → `preventDefault()`+`openSpotlight()`; cleanup no unmount — CA-01;
    ignora `k` sozinho — CA-01.1). `routes/AppShell.tsx`: monta `<Spotlight/>`, chama
    `useSpotlightHotkey()`, renderiza `<SpotlightButton/>` (visível desktop+mobile — CA-02/03).
    Acesso mobile: botão presente < md (D-06; opcional item no menu "Mais" da MobileNav que dispara
    `open()`).
  - Specs: `useSpotlightHotkey.spec.tsx` (dispara `Cmd+K` no document ⇒ store.open=true; `k`
    sozinho ⇒ nada — CA-01.1; unmount remove o listener — CA-01). `SpotlightButton.spec.tsx`
    (clique ⇒ `open()` — CA-02). Spec de presença do botão < 768 (matchMedia mockado — CA-03).
  - **Acceptance:** Vitest verde; `make test-back` verde (o único Go novo é a allowance da T-00 —
    aditiva; `entries.Patch` byte-idêntico — RNF-01/CA-13.1).

- **T-07 — E2E `spotlight.spec.ts`** (CA-19)
  - `frontend/e2e/spotlight.spec.ts`: logado (stub fase 01), navega pra um projeto (`/projetos/:id`
    — cria projeto via helper se preciso). `Cmd+K` (`page.keyboard.press('Meta+k')` /
    `'Control+k'`) ⇒ Spotlight aberto + input focado. Digita `/task Comprar domínio` + `Enter` ⇒ a
    task nasce e o **popup central da fase 24** abre (modo task) com o título "Comprar domínio"
    (asserta o título no editor); fecha. `Cmd+K` de novo, estando **ocioso**, digita "Estudar Go" +
    `Enter` (sem comando) ⇒ a TimerBar mostra um count-up rodando com essa descrição (CA-14). **Com o
    timer rodando**, `Cmd+K`, digita "Estudar Go avançado" + `Enter` (sem comando) ⇒ a descrição na
    TimerBar atualiza **sem o cronômetro zerar** (allowance §3.5/CA-13 — o single-user do stub cobre
    esse ramo no E2E). Reabre `Cmd+K` vazio, `Esc` ⇒ fecha (CA-04).
  - **Limitação (igual fase 12 CA-24):** o stub autentica 1 usuário (owner) — o ramo
    viewer/`can_write=false` (CA-16) e "sem projeto de contexto" (CA-15) são cobertos por
    **componente jsdom** (T-03/T-05), não no E2E. (A edição do timer rodando — CA-13 — **É**
    E2E-ável, pois mexe no timer do próprio usuário.) Reusar `loginViaStub`; nomes únicos (banco
    reusado, idempotente).
  - **Acceptance:** `make test-e2e` (ou playwright do spec) verde; liberar a :8080 antes (regra do
    KICKOFF — `reuseExistingServer:true` pega server alheio).

- **T-08 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: **allowance de backend** [`EditCurrent` toca SÓ desc/project/tags
    — timing intocado, CA-13.1; `OnChange` disparado 1×, CA-13/04; `ValidateRefs` no ws da entry,
    CA-13.2; `409 no_active_timer`/`404 not_found`; advisory lock; `entries.Patch` byte-idêntico],
    **parse/roteamento** [comando vs sem-comando — o ramo timer não pode engolir `/x`], **timer
    rodando vs ocioso** [CA-13 = `editCurrent` (não `patchEntry`/`start`); CA-14 count-up; vazio =
    no-op], **projeto de contexto** [resolvido do `pathname` via `matchPath`, não `useParams()` —
    27-H2; provar que dentro de `/projetos/:id` o `ctx.projectId` não é null], **gate do `useBoard`**
    [nenhum `GET /projects//board` fora de `/projetos/:id` nem com Spotlight fechado — 27-H3/CA-16.1],
    **contexto/permissão ausentes** [CA-15/16 não chamam API], **degradação** [abridor ausente/API
    falha — CA-17 não rejeita/não quebra], **listener de teclado** [vazamento/`preventDefault`/`k`
    sozinho — CA-01/01.1], **isolamento** [ws/projectId/user corretos nos paths — CA-18; `EditCurrent`
    só o timer do próprio user], **regressão zero do TRACKING** [timing/sync/pomodoro intocados —
    RNF-04]) + refutação dupla A/B. Corrigir confirmados red→green. Registrar na seção abaixo.
  - **Acceptance:** confirmados corrigidos; mutações relevantes morrem.

- **T-09 — Gate + (deploy no fim da milestone) + handoff**
  - `make check-full` **VERDE** (Go [allowance aditiva da T-00; `entries.Patch` byte-idêntico] +
    Vitest + Playwright + builds + check-cores + check-pwa + smoke compose). **Deploy é único, no fim
    da M3** (não por fase — regra do KICKOFF); esta fase **não** deploya sozinha.
  - Smoke local honesto: `Cmd+K` abre; `/task` cria + abre o editor da 24; `/timer` cronometra;
    sem-comando ocioso inicia o count-up; **sem-comando com timer rodando edita a descrição do timer
    ao vivo** (allowance §3.5). Bundle contém os comandos do Spotlight.
  - Handoff (CLAUDE.md "retomar..."), INDEX (27 construída; **sem migração**; rota nova
    `PATCH /api/v1/timer` registrada), KICKOFF da próxima fase (28), commit, briefing pro Diogo.

---

## Notas de blocos

A fase é pequena: **uma allowance mínima de backend** (T-00 — `PATCH /api/v1/timer`) + frontend
(núcleo puro + casca + componentes); **um bloco só** (T-00 → T-09), com a allowance de backend
PRIMEIRO (TDD Go), pra o `editCurrent`/Spotlight terem contra o que rodar. `make check-full` é gate
no T-09. Commit por tarefa (checkpoints recuperáveis). **Não há migração**; o único Go novo é a
allowance aditiva (T-00) — o `entries.Patch` e o resto do backend ficam byte-idênticos (prova do
RNF-01/CA-13.1).

---

## Furos da verificação adversarial

Verificação adversarial das specs (ETAPA 1) — 5 furos confirmados 2/2, todos aplicados em
spec/plan/tasks ANTES do build:

- **27-H1 (HIGH — contrato/corretude) — RESOLVIDO PELA DECISÃO HITL 23/06 (allowance de backend).**
  O furo original: editar a descrição+projeto do **timer RODANDO** via `patchEntry` (CA-13/US-05/
  RF-19) é **impossível** com `entries.Patch` — ele rejeita PATCH de entry rodando (`409
  entry_running`, `backend/internal/entries/service.go:184-187`) e a TimerBar deixa essa descrição
  **read-only** pelo mesmo motivo (`TimerBar.tsx:156-157,176`); a afirmação "= comportamento de hoje"
  era FALSA. **A verificação da spec (ETAPA 1) tinha aplicado a Opção A "no-op + backlog" (porque a
  fase era declarada frontend-puro).** A **decisão HITL de 23/06 SUPERSEDE isso**: editar o timer
  rodando NÃO é mais no-op/backlog — a 27 ganha uma **ALLOWANCE MÍNIMA DE BACKEND** (a fase deixa de
  ser 100% frontend). **Contrato escolhido = (a)** rota nova por-usuário **`PATCH /api/v1/timer`**
  roteada pelo `timer.Service` (método `EditCurrent`): edita **SÓ** description/project_id/tags da
  entry rodando, dentro do advisory lock do timer (`lockUser`, `service.go:129-132`), validando
  `project_id` via `entries.ValidateRefs` (`refs.go:21`) no workspace da entry, e **disparando
  `s.onChange`** (`service.go:100-106`) ⇒ o broadcast da fase 04 propaga o rótulo ao vivo a
  TimerBar/Foco/Remoto. **(b) — relaxar `entries.Patch` + disparar `OnChange` — foi REJEITADA** (o
  `entries.Service` não conhece o hub; afrouxar o `409` arrisca relaxar timing; duplica a regra
  rodando×parado — justificativa completa no plan §3.5). **Invariantes preservados:** os campos de
  TEMPO seguem recusados numa entry rodando (CA-13.1 — `entries.Patch` byte-idêntico, ainda `409`);
  ZERO migração; timing/sync/pomodoro intocados (RNF-04). Corrigidos: spec.md (cabeçalho, §1, US-05,
  CA-13 + CA-13.1 + CA-13.2 novos, tabela RF-19, RNF-01, RNF-04, CA-19, §6); plan.md (diagrama,
  §3.5 nova [contrato (a)/(b) + assinaturas reais], D-10/D-13.5, §4.3, §5, §6 pré-povoar, §7, §8, §9);
  tasks.md (**T-00 nova** [allowance Go TDD], T-02 `editCurrent`, T-03 `editCurrent` + mutações, T-04,
  T-06, T-07 [agora E2E-ável], T-08 dimensão, T-09, Notas de blocos).

- **27-H2 (HIGH — contrato/corretude):** projeto de contexto via `useParams()` está **quebrado** — o
  Spotlight é montado no `AppShell` (layout PAI; `/projetos/:id` é FILHO via `<Outlet/>`, `App.tsx`),
  e em React Router v6 os params só fluem pai→filho ⇒ `useParams()` no shell devolve `{}` e TODO
  `/task`/`/note`/`/goal` cairia no ramo "sem contexto" (CA-15), nunca criando nada. **Resolvido:**
  projeto de contexto = `matchPath('/projetos/:id', useLocation().pathname)?.params.id ?? null`
  (espelha SideNav/MobileNav, irmãos do `<Outlet/>`). Nova decisão **D-13** no plan; teste obrigatório
  que monta o Spotlight FORA da rota `:id` e prova `ctx.projectId==='<id>'` (mata a mutação pra
  `useParams`). Corrigidos: spec.md (nota antes do CA-09, CA-15 ref); plan.md (§4.5, D-13);
  tasks.md (T-05 + teste, T-08 dimensão).

- **27-H3 (MEDIUM — regressão/RNF):** `can_write` via `useBoard` em toda rota — `useBoard` **não tem
  `enabled`** (`lib/queries/objectives.ts:19-24`), então chamá-lo do Spotlight no shell com
  `projectId` vazio dispararia `GET /workspaces/{ws}/projects//board` malformado em toda navegação e
  todo render do shell. **Resolvido:** novo **CA-16.1** — gatear a leitura do `/board` por contexto:
  (a) estender `useBoard` com `enabled` e chamar `enabled:!!projectId && open`, ou (b) componente-filho
  condicional (não hook condicional). Teste pinando "nenhum `GET /projects//board` fora de
  `/projetos/:id` nem fechado". Corrigidos: spec.md (CA-16.1); plan.md (§4.5, D-12 vizinho, §7);
  tasks.md (T-04, T-05 + teste, T-08 dimensão).

- **27-H4 (LOW — consistência cross-fase, decisão H):** o plan inventava `src/stores/spotlightStore.ts`
  — diretório que NÃO existe; todos os stores zustand vivem em `src/lib/` (`timerStore.ts`,
  `presenceStore.ts`, `noticeStore.ts`). **Resolvido:** trocado pra `lib/spotlightStore.ts` (e teste
  `lib/spotlightStore.test.ts`) em plan.md (diagrama, §5, §8) e tasks.md (T-02); armadilha "NÃO criar
  src/stores/" no §7.

- **27-H5 (MEDIUM — consistência cross-fase, decisão A):** D-12/§6 afirmavam reusar
  `.mscrim`/`.scard`/`.swrap` ("o `.mscrim` já pode existir de 24/26"), mas essas classes NÃO existem
  no codebase (grep = 0) e o padrão de modal vigente é overlay self-contained. **Resolvido (decisão de
  coordenação A):** o popup central é um **componente React da fase 24** (ex.: `CenterPanel`); 26 e 27
  **reusam** esse componente. A 27 só declara classes próprias de conteúdo (ex.: `.spotitem`) com
  tokens; o scrim/card vêm do componente da 24. D-09 reescrito: a 24 deve EXPOR o abridor/estado de
  painel (contrato fixado no plan da 24, não 100% no "confirmar no build" da 27). Corrigidos: spec.md
  (cabeçalho, fonte visual, CA-05, CA-17 ref, RNF-05, RNF-07); plan.md (diagrama, D-09, D-12, §6, §7,
  §8); tasks.md (T-04, T-05, T-07).

### Decisões de coordenação cross-fase aplicadas

- **A (popup central = componente da 24):** aplicada em 27-H5 acima — toda menção a "reusa
  `.mscrim`/`.scard`/`.swrap`" virou "reusa o componente de popup central da fase 24".
- **H (stores em `src/lib/`):** aplicada em 27-H4 — `spotlightStore` em `src/lib/`, não `src/stores/`.
- **B (botão Start mock da 24):** não se aplica a esta fase (a 27 não wira o botão Start de task — isso
  é da fase 30); a 27 apenas cria a task e abre o popup central da 24.
