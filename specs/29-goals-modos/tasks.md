# tasks.md — Fase 29 · Goals (objetivos) com modos de conclusão

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-14).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → service → board → recálculo por modo
> → handlers), depois frontend (api/tipos → componentes → aba Goals → reforço de conclusão), E2E e
> gate. **Estende a fase 12 — não a reescreve** (RNF-05). A aba Goals pendura no tab-shell da fase 24.

---

## Bloco 1 — backend (T-01 → T-06)

- **T-01 — Migração `0026_objectives_modes`** (CA-01, RNF-03)
  - `backend/internal/db/migrations/0026_objectives_modes.sql` (plan §2): `ALTER TABLE objectives ADD
    COLUMN completion_mode text NOT NULL DEFAULT 'by_tasks' CHECK (completion_mode IN
    ('by_tasks','by_hours','by_date')), ADD COLUMN target_date date`.
  - Conferir o diretório antes: a M3 vai de `0021` (fase 24) a `0026` (fase 29) — **`0026`** é o número
    FIXO desta fase (não "ler o próximo livre" — worktrees colidiriam).
  - Teste (`migrations_*_test.go` ou novo, dbtest): colunas existem; `completion_mode` default
    `'by_tasks'` e `target_date` default `NULL` num objetivo criado sem esses campos (objetivos da fase
    12 permanecem válidos); CHECK barra `completion_mode='foo'` (insere → erro); `target_date` aceita
    `YYYY-MM-DD`. **SEM enum novo** (text+CHECK — sem a restrição do `ALTER TYPE ADD VALUE`).
  - INDEX.md: registrar `0026_objectives_modes` dona da fase 29 (orquestrador).
  - Commit: `fase 29 T-01: migração 0026_objectives_modes (completion_mode + target_date)`.

- **T-02 — `objectives.Service`: campos + validação** (CA-02/02.1/03/03.1/04, RNF-05)
  - `internal/objectives/service.go`: `Objective` ganha `CompletionMode string` + `TargetDate *string`;
    `CreateInput` ganha `CompletionMode string` (`""`→`by_tasks`, D-07) + `TargetDate *string`;
    `PatchInput` ganha `CompletionMode *string` + `TargetDateSet bool`/`TargetDate *string` (tri-state).
    `objectiveCols` inclui `completion_mode, to_char(target_date,'YYYY-MM-DD')` (scan p/ `*string`).
    `validCompletionMode` (switch nos 3; senão `domain.Validation` — CA-02.1) e `validTargetDate`
    (`time.Parse("2006-01-02", *d)`; senão `domain.Validation` — CA-03), ambos ANTES do SQL. Create
    aplica default `by_tasks` quando vazio. Patch aplica o subconjunto (modo ponteiro; target_date
    tri-state). **O `loadProjectGate`/`getInProject`/visibilidade da fase 12 ficam INTOCADOS.**
  - Teste `service_test.go` (dbtest, estende o da fase 12): create com `by_hours`+`goal_hours` persiste
    modo; create sem `completion_mode` → `by_tasks` (default); create/patch com `completion_mode='foo'`
    → 400 (não persiste, não 500); `target_date='2026-13-40'` → 400; `target_date='2026-07-15'` cria;
    PATCH `target_date:null` limpa (tri-state), ausente mantém; PATCH `completion_mode` troca sem zerar
    `goal_hours`/`target_date` (CA-04); **CA-04.1 (marco dormente, 29-H8):** PATCH que troca um `by_date`
    com `target_date` setado pra `by_tasks`/`by_hours` **preserva** o `target_date` no banco (não zera) —
    ele vira marco dormente (a renderização/gate `completion_mode='by_date'` é teste da fase 28, CA-11);
    **regressão fase 12: create sem campos novos = comportamento
    idêntico** (a suíte da fase 12 verde sem mudar asserção). Mutação removendo `validCompletionMode`
    deve QUEBRAR (CA-02.1).
  - Commit: `fase 29 T-02: completion_mode + target_date no objectives.Service (validação + tri-state)`.

- **T-03 — Board: campos novos + `progress_pct` por modo** (CA-10/10.1, RNF-05)
  - `internal/objectives/board.go`: `BoardObjective` ganha `CompletionMode string` + `TargetDate
    *string` + `ProgressPct int`. O SELECT de objetivos do board traz `completion_mode,
    to_char(target_date,'YYYY-MM-DD')`. Helper PURO `progressByMode(mode string, donePct int,
    trackedSeconds int64, goalHours *float64) int`: `by_tasks`→`donePct`; `by_hours`→`goalHours nil ?
    0 : clamp(round(tracked/(goalHours*3600)*100),0,100)`; `by_date`→`donePct`. Calculado em memória
    após montar `done_pct`/`tracked_seconds` (sem query nova). **Campos da fase 12 byte-idênticos.**
  - Teste `board_test.go` (estende fase 12): board traz `completion_mode`/`target_date` corretos;
    `progress_pct` `by_tasks`==`done_pct`; `by_hours` com `tracked=1.8h`/`goal=4h`→`45`; `by_hours`
    sem `goal_hours`→`0` (sem div/0); `by_hours` `tracked>meta`→clampado `100`; `by_date`==`done_pct`.
    **Regressão:** todos os campos da fase 12 inalterados (mutação que renomeia/remove `done_pct`
    quebra). `progressByMode` testada como função pura (tabela de casos).
  - Commit: `fase 29 T-03: completion_mode/target_date/progress_pct no /board (aditivo)`.

- **T-04 — Recálculo por modo em `recalcObjectiveStatus`** (CA-05/06/06.1/06.2/07, RNF-05)
  - `internal/tasks/objective_link.go`: `recalcObjectiveStatus` ramifica por `completion_mode` (D-02).
    Mantém o `FOR UPDATE` da fase 12. Lê `completion_mode`/`goal_hours` + `n`/`d` (subquery sobre
    `tasks`) + `secs` (subquery sobre `time_entries ⋈ tasks do objetivo — FILTRANDO `e.user_id` (do user
    da mutação) — per-usuário, igual o board`, plan §7 decisão FIXA / coordenação E). Ramos: `by_tasks` →
    regra da fase 12 (byte-equivalente — CA-05); `by_hours` → `goal_hours` não-nulo & `secs>=goal_hours*3600`
    & open→done / `secs<...` & done→open / `goal_hours` nulo→no-op (CA-06/06.1); `by_date` → no-op sempre
    (CA-07). Idempotente; preserva fecho manual (CA-08). **CUIDADO com o fan-out do join** (plan §7): `n`/`d`
    em subquery sobre `tasks` (não no join de entries); `secs` em subquery separada `WHERE e.user_id =
    <user> AND e.task_id IN (tasks do objetivo)` — senão `count` infla com N entries por task.
  - Teste `objective_status_test.go` (estende fase 12): **(by_tasks, CA-05)** 100% tasks done & open →
    done; <100% & done → open; vazio → no-op — **idêntico à fase 12** (sem mudar asserção; mutação que
    quebra o ramo by_tasks falha a suíte da 12). **(by_hours, CA-06)** objetivo `by_hours` goal=2h, semear
    entry 7300s numa task dele + transição de task → done; tirar horas (<meta) & done → reabre;
    `by_hours` **sem `goal_hours`** → nunca fecha (CA-06.1). **(by_date, CA-07)** objetivo `by_date`, marcar
    todas as tasks done → **continua open** (no-op); só `PATCH status:done` fecha (CA-08). **Horas
    per-usuário (decisão E):** entry de OUTRO usuário numa task do objetivo `by_hours` **NÃO** conta pro
    auto-close (só as do user da sessão/mutação — MESMO bucket do board) — teste pina (mutação que
    REMOVE `WHERE e.user_id = $user` do SUM passa a contar a entry do outro usuário e **quebra**,
    provando o filtro per-usuário). **CA-06.2 (gatilho real, 29-H7):** o auto-close `by_hours` só roda em
    **mutação de TASK** (criar/deletar/mudar `status`/`objective_id`) — fechar uma entry/parar o timer
    NÃO recalcula no instante. Teste prova a convergência **via transição de task subsequente**: (a)
    objetivo `by_hours` goal=2h com 1 task done; semear entry que bate a meta; SEM mexer em task o
    `status` persiste `open` (gatilho não disparou); (b) criar/mover OUTRA task do objetivo (mutação que
    toca o objetivo) → o recálculo roda e o `status` vira `done`. (NÃO testar "carregar o board fecha o
    status" — o board é read-only e não converge o `status` persistido; ver 29-H7.)
  - Commit: `fase 29 T-04: recalcObjectiveStatus ramifica por completion_mode (by_tasks/by_hours/by_date)`.

- **T-04.1 — "Concluído by_hours" coerente nos Relatórios (consome `status` persistido)** (CA-06.3, 29-H7)
  - **Dependência cross-fase (fase 14):** como o `status` persistido de um `by_hours` pode ficar `open`
    com a meta de horas batida (CA-06.2 — convergência condicionada a mutação de task), o relatório de
    objetivos (fase 14, `internal/reports/objectives.go`) DEVE, pra objetivos `by_hours`, derivar
    "concluído" de `tracked_seconds (per-usuário) >= goal_hours × 3600` (espelha o critério por modo do
    board — CA-10.1), **não** só do `status` cru; expor um `progress_pct`/`is_complete` por modo coerente
    com o board. Front da fase 14 (`ObjectivesSection.tsx`, `ReportTable.tsx`) deriva `done` desse
    critério por modo.
  - Teste Go (`reports/objectives_test.go` ou novo, estende fase 14): goal `by_hours` com a meta de horas
    batida mas `status='open'` (sem mutação de task) aparece como **concluído** nos Relatórios; goal
    `by_tasks` segue por `status`/`done_pct` (sem regressão). Spec jsdom da fase 14: `ObjectivesSection`/
    `ReportTable` mostram o `by_hours` da meta-batida como concluído, **não** "Open".
  - **Escopo:** se a fase 14 ficar FORA desta rodada, registrar em §6 da spec a limitação (Relatórios
    podem mostrar by_hours como "Open" até a próxima mutação de task) — mas a correção limpa (derivar por
    modo) é a preferida (29-H7). Decisão registrada no plan.
  - Commit: `fase 29 T-04.1: Relatórios derivam "concluído by_hours" por modo (coerente com o board)`.

- **T-05 — Handlers: completion_mode/target_date (tri-state)** (CA-02/02.1/03/03.1/04)
  - `internal/api/objectives_handlers.go`: `handleCreateObjective` body ganha `CompletionMode string`
    + `TargetDate *string`. `patchObjectiveBody` ganha `CompletionMode *string` + `TargetDate
    json.RawMessage` (tri-state como o `GoalHours` da fase 12: presente×"null"×valor → `TargetDateSet`
    + `TargetDate`). Mapeia pros inputs do service.
  - Teste `objectives_handlers_test.go` (harness `coretest`, estende fase 12): POST com `by_hours`+
    goal → 201 + corpo com modo; POST `completion_mode='foo'` → 400; POST `target_date` malformado →
    400; PATCH `target_date:null` limpa (tri-state); PATCH ausente mantém; board 200 com `completion_mode`/
    `target_date`/`progress_pct` no corpo (CA-10). 403 viewer em escrita de modo; 404 projeto invisível;
    401 sem sessão (CA-09/09.1). Mutação que ignora `completion_mode` no body quebra.
  - Commit: `fase 29 T-05: handlers de objetivo com completion_mode/target_date (tri-state)`.

- **T-06 — Isolamento multi-tenant dos campos novos** (CA-09.2, RNF-04)
  - Teste (no `service_test.go`/`objectives_handlers_test.go`): objetivo `by_hours`/`by_date` de outro
    workspace/projeto invisível alcançado por rota errada → 404 (carga `WHERE workspace_id AND
    project_id AND id` da fase 12 intocada cobre); o `target_date`/`completion_mode` de outro tenant
    nunca vaza; o `tracked` do auto-close `by_hours` só soma entries de tasks **deste** objetivo
    (mutação removendo o `WHERE e.task_id IN (tasks do objetivo)` quebra — CA-09.2) **e só do `user_id`
    da sessão** (mutação removendo o `WHERE e.user_id` quebra — per-usuário, decisão E / 29-H1).
  - Commit: `fase 29 T-06: testes de isolamento multi-tenant dos campos de modo`.

> **Fim do bloco 1:** `make test-back` 100% verde (objectives + tasks + board; **fase 12 intacta** —
> nenhum teste da 12 muda asserção); Vitest/Playwright não rodados (bloco sem frontend). Handoff
> "retomar na T-07".

## Bloco 2 — frontend (T-07 → T-11)

- **T-07 — `lib/api.ts`: tipos com modo/marco** (CA-10/12)
  - `Objective` ganha `completion_mode: 'by_tasks'|'by_hours'|'by_date'` + `target_date: string|null`;
    `BoardObjective` ganha os mesmos + `progress_pct: number`; `ObjectiveInput` ganha `completion_mode?`
    + `target_date?`; `ObjectivePatch` ganha `completion_mode?` + `target_date?` (`null` limpa).
    `createObjective`/`patchObjective` repassam os campos. **NÃO** mexer em `lib/queries/objectives.ts`
    (hooks já invalidam `boardKey`).
  - Teste `objectives-api.test.ts` (estende fase 12): create/patch enviam `completion_mode`/`target_date`
    no corpo certo; tipos compilam.
  - Commit: `fase 29 T-07: tipos de completion_mode/target_date/progress_pct na api.ts`.

- **T-08 — `GoalRow` + `goalProgress` (função pura)** (CA-12/12.1, RNF-03)
  - `components/board/GoalRow.tsx`: linha da tabela Notion (mockup 473–479): ícone star (`--warn`),
    nome, barra `.tt-goal` (cor: `>=100%`→`--accent`, senão cor do projeto via `palette.ts`), `done/total`
    de tasks, horas (`fmtH(tracked)` + `/{goal_hours}h` quando há meta), chip **Done** (accent) / **Open**.
    Função PURA `goalProgress(obj: BoardObjective): number` (CA-12.1): `by_tasks`→`done_pct`; `by_hours`→
    `progress_pct` (ou derivado); `by_date`→`done_pct`. **Nunca vermelho** (RNF-03); marco vencido aberto
    = **Open** discreto.
  - Specs `GoalRow.spec.tsx` + `goalProgress.spec.ts`: `goalProgress` por modo (tabela: by_tasks usa
    done_pct; by_hours sem meta→0; by_hours com meta→%; by_date→done_pct, sem flag de "falho"); barra
    accent ≥100%; coluna Horas sem `/Xh` quando sem meta; chip Done/Open; **sem classe/cor de erro**
    mesmo com `target_date` no passado (CA-12.1/RNF-03).
  - Commit: `fase 29 T-08: GoalRow + goalProgress (barra por modo, à prova de culpa)`.

- **T-09 — `GoalModal` (New/Edit com seletor de modo)** (CA-13, RNF-02)
  - `components/board/GoalModal.tsx`: seletor de modo (by_tasks/by_hours/by_date) que mostra/esconde
    `goal_hours` (by_hours) e `target_date` (by_date); nome sempre. Validação client antes do POST/PATCH:
    nome não-vazio; `goal_hours` em `(0,9999]` se by_hours; `target_date` `YYYY-MM-DD` se by_date.
    Inválido → não chama API, sinaliza erro. Edit pré-preenche; PATCH só com o diff (padrão fase 12).
  - Specs `GoalModal.spec.tsx`: trocar modo mostra/esconde os campos; submit inválido (nome vazio /
    goal_hours fora de faixa em by_hours / target_date malformado em by_date) NÃO chama a API; submit
    válido chama com `completion_mode` certo; modo Editar pré-preenchido + PATCH só com o diff.
  - Commit: `fase 29 T-09: GoalModal (New/Edit com seletor de modo + validação client)`.

- **T-10 — `GoalsTab` + montar no tab-shell** (CA-12/13/15)
  - `routes/cockpit/tabs/GoalsTab.tsx`: consome `useBoard(ws, projectId)`; toolbar (texto informativo
    do mockup + **New goal** gated por `board.project.can_write` — CA-15); visão lista com `GoalRow`
    (mockup `renderMetas`); vazio → "No goals yet." (linha 470). Montar a aba no tab-shell da fase 24
    (se a 24 deixou placeholder `GoalsTab`, preencher; senão criar e linkar no shell). New/Edit abrem o
    `GoalModal`; criar/editar invalidam `boardKey` (hooks da fase 12) → reflete sem reload.
  - Specs `GoalsTab.spec.tsx` (jsdom, board mockado): renderiza linhas dos objetivos do board; vazio →
    "No goals yet."; `can_write=false` → New goal/editar/excluir escondidos (CA-15, padrão
    `viewer-gating.spec` da fase 12); New goal abre o modal.
  - Commit: `fase 29 T-10: aba Goals (lista Notion) montada no tab-shell + gating viewer`.

- **T-11 — Reforço de conclusão (dopamina) + barra por modo no card/Overview** (CA-14/12.2, RNF-03)
  - `components/board/ObjectiveCard.tsx` (existe, fase 12) + `GoalRow`: feedback visual positivo ao
    virar `done` (chip Done no accent + check/brilho curto via CSS transition, sem lib — D-10); badge
    discreto do modo (Tasks/Horas/Data). Reabrir → Open discreto, **sem nada vermelho/culpa** (RNF-03).
  - **Barra por modo no `ObjectiveCard` (CA-12.2, 29-H6):** a barra do card passa a derivar de
    `progress_pct`/`goalProgress` (por modo), **não** `done_pct`. **Overview da fase 28:** a lista de
    Goals da Overview (`OverviewTab`/`ProjectOverview`) consome `progress_pct` por-modo do board em vez
    de `done_pct` (troca trivial; `by_tasks` `progress_pct==done_pct` → zero regressão).
  - Specs (jsdom): objetivo `status:done` mostra chip Done no accent + estado de conclusão; `status:open`
    discreto; **nenhum elemento com cor/classe de erro** quando `by_date` vencido aberto (CA-14/RNF-03).
    **CA-12.2:** objetivo `by_hours` `status:done` com `done_count=0`/`task_count=3` e `tracked>=meta`
    (`progress_pct=100`) → barra **cheia** (não 0%) no `ObjectiveCard` E na Overview, chip Done coerente;
    **nenhuma** classe/cor de erro; **proibido** chip "Concluído" sobre barra 0%.
  - Commit: `fase 29 T-11: reforço do feedback de conclusão + barra por modo no card/Overview`.

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright não rodado
> ainda (E2E é o bloco 3). Handoff "retomar na T-12".

## Bloco 3 — E2E + verificação + gate (T-12 → T-14)

- **T-12 — E2E `goals.spec.ts`** (CA-16)
  - Logado+escrita (stub fase 01) num projeto `shared`: cria Goal **by_hours** com `goal_hours` → vê a
    linha na aba Goals com barra de horas; cria Goal **by_date** com `target_date` → vê **Open**; marca
    tasks do `by_date` done → continua **Open** (auto-close não fecha `by_date` — CA-07); **fecha o
    `by_date` manualmente** → **Done**; reload → modo/estado persiste. SEM segundo perfil viewer no E2E
    (stub autentica só o owner — CA-16): gating viewer = teste de componente jsdom (T-10); auto-close por
    horas (CA-06) = teste Go (T-04, precisa semear entries). Reusar `loginViaStub`; nomes únicos (banco
    reusado, idempotente). Se o `goals.spec` for o 2º+ a mutar o timer global, usar o mutex
    `e2e/timerLock.ts` (padrão fase 23) — mas Goals **não** mexe no timer, provavelmente dispensável.
  - Commit: `fase 29 T-12: E2E goals.spec (modos by_hours/by_date + fecho manual + persistência)`.

- **T-13 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: regressão do `by_tasks` / recálculo por modo / fan-out do join de
    horas / `by_hours` sem meta / `by_date` no-op / **horas per-usuário (filtro `e.user_id`, decisão E)
    coerentes entre auto-close e progress_pct** / isolamento multi-tenant dos campos novos /
    progress_pct clamp+div0 / tri-state target_date / gating viewer / **barra por-modo na Overview da
    fase 28** / **convergência do `status` by_hours via gatilho de task, não no fechamento de entry**)
    + refutação dupla. Corrigir confirmados red→green. Registrar na seção abaixo.
  - **Antes de rodar:** commitar/limpar a working tree (os agentes de mutação rodam na mesma tree; o
    `git checkout` de restauração clobbera mudança não-commitada — lição da fase 17).
  - Commit: `fase 29 T-13: verificação adversarial do build + correções`.

- **T-14 — Gate + handoff** (NÃO deployar — deploy é único no fim da M3)
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). Liberar a :8080 de dev
    server alheio antes do E2E (lição sessão 17). Reiniciar o db-test antes do gate se houver flake de
    acúmulo.
  - Handoff (CLAUDE.md "retomar..."), INDEX (29 construída, `0026` dona), memória, commit, briefing.
    **NÃO deployar** (a M3 deploya tudo de uma vez no fim — padrão M1/M2).
  - Commit: `fase 29 handoff: gate verde + docs (CLAUDE/INDEX/memória)`.

---

## Blocos (regra de organização — gate só no bloco 3)

- **Bloco 1 — backend:** T-01 → T-06 (migração, service campos/validação, board+progress_pct, recálculo
  por modo, handlers, isolamento). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-07 → T-11 (api/tipos, GoalRow/goalProgress, GoalModal, GoalsTab+shell,
  reforço de conclusão). Suíte: Vitest + `make test-back` (regressão).
- **Bloco 3 — E2E + fechamento:** T-12 → T-14 (E2E, verificação adversarial, gate `make check-full` +
  handoff; SEM deploy).

`make check-full` é gate **só no bloco 3 (T-14)**; os blocos 1 e 2 fecham com a suíte do que existe
verde + handoff + commit.

---

## Furos da verificação adversarial

> Verificação adversarial das specs (ETAPA 1 da M3). 5 furos confirmados 2/2 — todos aplicados nas
> specs ANTES do build. Decisão de coordenação cross-fase E (by_hours = bucket PER-USUÁRIO do /board)
> aplicada como base das correções de horas.

- **29-H1 (HIGH, CONSISTÊNCIA CROSS-FASE)** — Contradição interna: CA-06 dizia "mesmo bucket do /board"
  (per-usuário, `board.go:106 WHERE e.user_id`), mas o plan §7 e a T-04 fixavam o auto-close em
  **all-users** ("NÃO filtra user_id"). Incompatíveis: status persistido (all-users) divergiria do
  progresso renderizado (per-user). **Resolvido (decisão E):** adotado o bucket **PER-USUÁRIO** em tudo.
  plan §7: "Decisão FIXA" all-users → **per-usuário (filtra `e.user_id` do user da mutação, igual o
  board)**; all-users registrado como futuro/multiplayer. plan §5 (SQL do recálculo) ganhou
  `AND e.user_id = $2`. tasks.md T-04: subquery `secs` agora **filtra `e.user_id`**; teste invertido
  ("entry de outro usuário NÃO conta; mutação que REMOVE `WHERE e.user_id` quebra"). spec.md CA-06
  explicitou o bucket per-usuário + decisão E.
- **29-H2 (HIGH, CONTRATO/CORRETUDE)** — Corolário do 29-H1: com all-users, um by_hours podia mostrar
  chip "Done" sobre barra <100% (viewer só lançou parte das horas), contradizendo CA-06.2 ("UI nunca
  mente"). **Resolvido junto (Opção A do JSON):** per-usuário em auto-close E `progress_pct` (board) →
  status e barra usam o MESMO total; barra 100% ⇔ meta batida (CA-06 + CA-06.3 fecharam o invariante
  "chip Done só sobre barra cheia"). spec.md CA-06 reforçada; CA-06.3 nova garante coerência também nos
  consumidores do status persistido.
- **29-H6 (MEDIUM, REGRESSÃO/RNF)** — A barra por-modo entrava só na GoalRow nova; o `ObjectiveCard` da
  fase 12 (board↔lista) e a lista de Goals da Overview (fase 28) seguiam por `done_pct` (tasks), então
  um by_hours fechado por horas (0 tasks done) apareceria "Concluído" sobre barra 0% — incoerente e
  acusatório (fere RNF-03). **Resolvido (correção real, não fora-de-escopo):** **CA-12.2 nova** —
  `ObjectiveCard` E a Overview derivam a barra por MODO (`progress_pct`/`goalProgress`), não `done_pct`
  (troca trivial; no by_tasks `progress_pct==done_pct` → zero regressão). plan §6 (linhas de
  ObjectiveCard/Overview) + D-11 novo; tasks.md T-11 estendida com a barra por modo no card/Overview +
  teste jsdom (by_hours done, done_count=0, tracked>=meta → barra cheia, sem chip Done sobre 0%).
- **29-H7 (MEDIUM, TESTABILIDADE/TDD)** — CA-06.2 chamava de "latência aceita" e dizia que o status
  "converge quando o /board é carregado" — falso: o board é read-only, não converge o `status`
  persistido; e se NENHUMA task sofrer mutação após a meta ser batida, o status fica `open` pra sempre
  (não é latência, é "nunca"). **Resolvido:** CA-06.2 reescrita pra a verdade (convergência condicionada
  a mutação de task futura; carregar o board NÃO converge). **CA-06.3 nova** + **T-04.1 nova** fecham o
  gap dos consumidores do `status` persistido: a fase 14 (Relatórios) deriva "concluído by_hours" por
  modo (tracked>=meta), não só do status cru — teste Go pina (goal by_hours da meta-batida com
  status='open' aparece concluído nos Relatórios). T-04 ajustada: o teste prova a convergência via
  **transição de task subsequente** (não "carregar o board"). plan D-04 sharpened.
- **29-H8 (LOW, CONTRATO/CORRETUDE)** — CA-04 não zera `target_date` ao trocar de modo → um
  `by_tasks`/`by_hours` pode ter `target_date` residual; sem gate, ele apareceria como marco no
  calendário do projeto (fase 28 agrega `objectives.target_date`). **Resolvido:** **CA-04.1 nova**
  registra o "marco dormente" (preservado no banco, NÃO renderizado); **CA-11 atualizada** exige que o
  agregador da fase 28 filtre `completion_mode='by_date'` (`WHERE … AND completion_mode='by_date' AND
  target_date BETWEEN …`). plan §8 (dependência fase 28) documenta o gate; tasks.md T-02 ganhou
  asserção de que o `target_date` residual persiste (a renderização/gate é teste da 28, CA-11).

### Decisões de coordenação cross-fase aplicadas

- **E (by_hours = bucket PER-USUÁRIO do /board):** base de 29-H1/29-H2. Auto-close (recalc) e
  `progress_pct` filtram `e.user_id` da sessão — coerentes entre si e com o board da fase 12
  (`board.go:90/106`). Soma all-users (esforço total entre membros) = futuro/multiplayer (§9 PRD),
  registrado em §6 da spec e no plan §7.
- **Cross-fase 14 (CA-06.3 / T-04.1):** os Relatórios (consumidores do `status` persistido) derivam
  "concluído by_hours" por modo — edição que toca a fase 14 (`reports/objectives.go`,
  `ObjectivesSection.tsx`, `ReportTable.tsx`), registrada como dependência.
- **Cross-fase 28 (CA-12.2 / CA-11 / CA-04.1):** a Overview (lista de Goals) deriva a barra por modo; o
  agregador de calendário gateia marcos por `completion_mode='by_date'`. Registradas no plan §8 pra a
  spec da 28 referenciar `0026` como dona de `target_date`/`completion_mode`.
- **G (erros canônicos):** as correções não introduzem códigos de erro novos — seguem
  `validation_error`/`forbidden`/`not_found`/`unauthorized` do envelope real (CA-02.1/03/09/09.1).
