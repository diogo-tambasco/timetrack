# tasks.md — Fase 13 · Dashboard (home nova)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-10).
> Cada T-XX lista os CAs cobertos. Frontend puro primeiro (agrupamento por dia), depois
> hook, componentes, dashboard+roteamento, E2E e gate.
> **Bloco único** (≤10 tarefas — cabe numa sessão; a regra de blocos não precisa acionar).

---

## Bloco núcleo (frontend puro + hook)

- **T-01 — `lib/recent.ts` + testes** (CA-07/11, RNF-02)
  - Funções puras (plan §5): `groupByDay(entries, tz)` → grupos `{dayISO,label,items}` do mais
    recente pro mais antigo (dia local por parede, NÃO epoch); `dayLabel(dayISO, todayISO)` →
    "Hoje"/"Ontem"/data pt-BR; constante `RECENT_DAYS = 7` (D-01).
  - `lib/recent.test.ts` (RED primeiro): casos obrigatórios do plan §5 — hoje/"Hoje",
    ontem/"Ontem", 5 dias atrás/data, ordem decrescente, dois itens no mesmo dia juntos, vazio
    → `[]`, e o caso de fuso tz-independente (domingo 23:30 SP cai no dia local certo;
    round-trip `Date local → toISOString`, padrão da fase 11).

- **T-02 — `useRecentEntries` em `lib/queries/entries.ts`** (CA-06)
  - Hook aditivo: `listEntries(ws, {from: addDays(hoje,-6), to: hoje, tz: browserTZ()})`; key
    `['entries', ws, 'recent', tz]`. Re-busca ao mudar a entry ativa (consumidor cuida, igual
    `TodayList`). Nenhum hook existente muda (CA-02).
  - Teste: `recent-entries.test.ts` — chama o path certo (from = hoje−6, to = hoje, tz). RED→green.

## Bloco componentes

- **T-03 — `TimerCard`** (CA-12/13/14, CA-01/03)
  - Hero da Direção C: ramo rodando (rótulo "Rodando agora" + descrição + chip projeto
    `archived:true` + dígitos grandes `shown` via `useTimerClock` + Pausar/Parar do store), ramo
    pausa (`mode==='break'` → rótulo + countdown, sem BreakOverlay completo — D-02), ramo ocioso
    (CTA pra começar, sem formulário de start completo — D-03).
  - Spec `TimerCard.spec.tsx` (jsdom): rodando mostra dígitos+projeto+Pausar/Parar; pausa mostra
    countdown e rótulo; ocioso mostra CTA e NÃO mostra dígitos vivos; Pausar/Parar chamam as
    ações do store (mock do `useTimerStore`, padrão dos specs da TimerBar).

- **T-04 — `WeekGoals`** (CA-16, CA-19, RNF-04)
  - `useWeekReport(ws, hojeISO)` → `report.goals` em `GoalBar` (reusa o componente; fill cor do
    projeto, accent ≥100%). Sem metas → estado vazio.
  - Spec: renderiza uma GoalBar por meta com meta×realizado; vazio → mensagem; cor via token.

- **T-05 — `RecentEntries` (dias anteriores) + reuso do `TodayList` p/ hoje** (CA-06/07/09/11, CA-24.1)
  - `useRecentEntries` → `groupByDay`; renderiza só os grupos com `dayISO < hoje` (pula "Hoje" —
    é do `TodayList` reusado, CA-24.1). Por grupo: cabeçalho de dia + linhas no formato do
    `TodayList` (dot cor projeto, descrição, projeto, horário, duração; continuar/editar — sem
    entry rodando aqui). Replicar a linha (NÃO tocar `TodayList` — CA-24); registrar no commit.
    Re-busca ao mudar `activeEntryID`. Vazio → mensagem.
  - Spec `RecentEntries.spec.tsx`: agrupa por dia (mock de entries de ontem + 3 dias atrás → 2
    grupos com labels certos, "Hoje" ausente — CA-07/24.1); botões continuar/editar presentes
    (CA-09); vazio → mensagem (CA-11). Mock de `useRecentEntries`/`useProjects`/store (padrão
    `TodayList.spec`). *(CA-08 — entry rodando em accent — é do `TodayList` reusado, coberto pelo
    `TodayList.spec` existente; não re-testar aqui.)*

- **T-06 — Seção "da semana" = `WeekGoals` (metas de horas)** (CA-16/17/18/19)
  - `WeekGoals` (do T-04) É a seção canônica da semana. **Não há `WeekObjectives`** (CA-17:
    progresso por-objetivo agregado por workspace não existe no contrato da fase 12 — `useObjectives`
    é pelado, progresso só no `/board` por projeto; agregar = N requests/RNF-02 ou backend
    novo/RNF-03). Registrar a decisão (deferimento pra fase futura) no commit. Opcional: um link
    "ver objetivos" → `/projetos/:id` (board da fase 12), sem reagregar nada.
  - Sem spec novo (coberto pelo `WeekGoals.spec` do T-04: metas presentes; vazio → mensagem CA-19).

## Bloco dashboard + roteamento

- **T-07 — `Dashboard.tsx`** (CA-04/10/15/20/21)
  - Layout Direção C: header (saudação do `me` + data de hoje + "Nova entrada" → `EntryModal`),
    `TimerCard`, grid com `RecentEntries` + coluna `WeekObjectives`/`FocusShortcut`. Estados de
    carga neutros por seção (CA-21). `FocusShortcut` = `Link to="/foco"` proeminente (CA-20).
  - Spec `Dashboard.spec.tsx`: monta as seções (saudação com nome, card, recentes, metas/
    objetivos, atalho foco com `href="/foco"`); "Nova entrada" abre o `EntryModal`; uma seção em
    erro não derruba as outras (CA-21).

- **T-08 — Roteamento `/` → Dashboard, `/hoje` → redirect, SideNav, `helpers.ts`, aposentar `Today`**
  (CA-04/05/22/23/24/24.1, CA-02)
  - `App.tsx`: `/` → `<Dashboard/>`; `/hoje` → `<Navigate to="/" replace/>`. `SideNav.tsx`:
    item "Hoje" `to:'/'` (D-05). Remover `Today.tsx` (D-06) — `grep routes/cockpit/Today` deve
    ficar vazio (nenhum spec importa).
  - **`e2e/helpers.ts` (exceção mecânica sancionada — plan §3 item 5):** trocar a asserção
    pós-login `toHaveURL(/\/hoje$/)` → `toHaveURL(/\/$/)`. É a ÚNICA asserção de E2E que muda.
  - **`Focus.tsx` (opcional — plan §3 item 6):** se `Focus.spec` não pinar o destino, trocar o
    `✕` pra `navigate('/')`; senão deixar (vai pra `/hoje`→`/`). Registrar a escolha no commit.
  - Confirmar via `git diff` que nenhuma **asserção de comportamento** das fases 00–12 muda
    (CA-02/23) e que TimerBar/BreakOverlay/Focus/sync + `cronometro.spec` (via TodayList reusado,
    CA-24.1) seguem verdes.
  - Spec/checagem: `/` renderiza Dashboard; `/hoje` redireciona; SideNav "Hoje" leva ao
    dashboard com `aria-current`.

## Bloco E2E + gate

- **T-09 — E2E `dashboard.spec.ts`** (CA-25)
  - Logado: timer rodando → `/` mostra o card com dígitos vivos + projeto; semear via API uma
    entrada **fechada de ontem** → aparece em "Recentes" sob "Ontem" (prova P3); continuar uma
    recente reflete no card/TimerBar; clicar **Modo Foco** navega pra `/foco`; item "Hoje" da
    SideNav volta pro dashboard. Reusar `e2e/helpers.ts` (loginViaStub, seed via API); banco
    reusado → nomes únicos/seed idempotente.

- **T-10 — Verificação adversarial + gate + handoff**
  - Verificação adversarial do build (workflow: dimensões — roteamento/não-regressão da
    TimerBar/sync, agrupamento por dia/fuso, degradação dos objetivos da 12, boot leve/queries,
    reuso×duplicação da linha) + refutação. Corrigir confirmados. Registrar.
  - `make check-full` VERDE (Go intacto + Vitest + Playwright + builds + smoke compose).
  - **SEM deploy** (decisão do Diogo 20/06 — KICKOFF M1: o deploy é ÚNICO, no fim das 4 fases).
    Esta fase fecha no gate verde + handoff e segue DIRETO pra fase 14.
  - Handoff (CLAUDE.md "retomar..."), INDEX (13 construída), memória, commit, segue pra 14.

---

## Dependências entre fases (registro)

- **Fase 12 (objetivos)** é construída ANTES da 13 (ordem 12→13, `MILESTONE-1 §7`). A 13 **NÃO**
  consome progresso de objetivo (CA-17/D-04 reescritos pós-verificação): o contrato da 12 só
  expõe progresso no `/board` por projeto; a seção "da semana" da home = metas de horas
  (`WeekGoals`). O E2E (CA-25) não depende de objetivos.
- A 13 **aposenta** o `/hoje` como tela própria (absorve no dashboard — CA-04/05/24); o
  `TodayList`/`EntryModal` continuam vivos, reusados (o `TodayList` é reusado direto pras
  entradas de HOJE — CA-24.1).

## A verificar na verificação adversarial do BUILD (T-10 — candidatos a furo)

- Agrupamento por dia no fuso do Diogo na virada da meia-noite (CA-07) — a entry de ontem 23:55
  não pode escorregar pra "Hoje" nem sumir.
- Duração viva da entry rodando (no `TodayList` reusado) continuar andando sem acumular local.
- O CTA do card ocioso não criar um segundo caminho de start que conflite com a TimerBar (CA-14).
- `cronometro.spec` passar pelo redirect `/hoje`→`/` batendo no `TodayList` reusado (CA-24.1) —
  "Nova entrada"/"Editar" presentes e funcionais no Dashboard.
- `git diff` provar CA-02/23/24: nenhuma asserção de COMPORTAMENTO das fases anteriores muda (só
  a URL de `helpers.ts`); `Today.tsx` some sem órfão de import.

---

## Registro — furos confirmados na verificação adversarial da SPEC (PASSO 0, sessão 18, 20/06)

Workflow ultracode `verify-m1-specs` (176 agentes, 6 dimensões × refutação dupla, ancorado no
código real). Fase 13: **4 confirmados 2/2, todos aplicados ANTES de codar:**

1. **(contract-drift, CA-17)** Progresso de objetivo (`done_pct`/`task_count`/`tracked_seconds`)
   só existe no `BoardObjective` do `/board` **por projeto** — `useObjectives` devolve o objetivo
   pelado SEM progresso, e não há endpoint de objetivos-com-progresso por workspace. → **CA-17
   reescrito:** progresso por-objetivo é FORA do escopo da 13 (agregar = N requests/RNF-02 ou
   backend/RNF-03); a seção "da semana" = metas de horas (`WeekGoals`, CA-16). Fase futura:
   `GET /workspaces/{id}/objectives` com progresso.
2. **(internal-consistency, CA-02/23)** `e2e/helpers.ts:14` (`loginViaStub`) afirma
   `toHaveURL(/\/hoje$/)`; com `/`→Dashboard o login cai em `/`. → **plan §3 item 5 + CA-02/23:**
   trocar pra `toHaveURL(/\/$/)` (exceção mecânica única; usada por TODO E2E logado).
3. **(internal-consistency, CA-05/CA-24.1)** `Focus.tsx:123` (`✕`→`/hoje`) e `cronometro.spec`
   (`goto('/hoje')` dirigindo "Nova entrada"/"Editar"/`TodayList`) viram exercícios do Dashboard
   pelo redirect, sem contrato pinado. → **CA-24.1 novo:** Dashboard reusa o `TodayList` pras
   entradas de hoje (selectors idênticos) + `Focus.tsx` opcional pra `/` (plan §3 item 6).
4. **(design-fidelity, D-04)** A "degradação por erro de runtime" (CA-18 antigo) era código
   morto — a fase 12 está em produção e o hook existe (shape errado, não ausente). → **§4/CA-18
   reescritos:** sem `WeekObjectives`, sem fallback de runtime; a seção da semana É o `WeekGoals`.

**Fase 16 = 0 furos confirmados** (16/16 refutados) — spec limpa.

## Registro — verificação adversarial do BUILD (T-10, sessão 18, 20/06)

Workflow ultracode `verify-phase13-build` (5 dimensões — regressão, tz/agrupamento, timer-card,
boot/resiliência, fidelidade da spec — × refutação dupla, ancorado no código real): **0 furos
brutos, 0 confirmados.** Build limpo. Suítes finais: **Go 100% · Vitest 247/247 · Playwright
24/24** (novos: `dashboard.spec` + card de timer no `cronometro.spec`; `recent`/`recent-entries`/
`TimerCard`/`WeekGoals`/`RecentEntries`/`Dashboard` specs). `make check-full` **GATE VERDE**.
**NÃO deployada** — o deploy é único no fim da M1 (fase 16).
