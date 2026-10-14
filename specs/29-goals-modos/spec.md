# spec.md — Fase 29 · Goals (objetivos) com modos de conclusão

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Sexta e última fase da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`).
> Cobre **RF-24, RF-25**.
> **Mockup (fonte visual da verdade):** `mockups/projeto-workspace.html` — `renderMetas`
> (linhas 464–481, visão lista Notion de Goals: Goal · Progresso · Tasks · Horas · Status),
> `objStats` (linha 265, progresso por tasks done) e o `goalRow` do Overview (linhas 344–348).
>
> **Decisões travadas (`MILESTONE-3.md §3 + §5.6 + §4.6`):**
> 1. Um objetivo (Goal) ganha **modo de conclusão**: por **tasks** (`by_tasks`, % done — o que já
>    existe), por **horas** (`by_hours`, `tracked_seconds` vs `goal_hours`) ou por **data**
>    (`by_date`, `target_date` = milestone/marco do roadmap).
> 2. Mantém o **auto-close a 100%** que já existe da fase 12 (`recalcObjectiveStatus`, dopamina
>    pronta) — **sem regredir o `by_tasks`**.
> 3. `target_date` (marco) alimenta o **calendário do projeto** (fase 28) e o "Upcoming"/Overview.
> 4. **À prova de culpa (RNF-03):** nada zera nem fica vermelho; o feedback de conclusão é positivo.

---

## 1. Mapa pro problema central (MILESTONE-3.md §5.6 + §1)

A fase 12 criou a camada `objectives` entre projeto e task: progresso = **% de tasks `done`** + meta de
horas (`goal_hours`) **opcional e separada do critério de conclusão** (a barra de horas é só
informativa hoje — o objetivo só auto-fecha a 100% das tasks). O Diogo, no dogfooding, planeja entregas
de três jeitos: "fechar quando todas as tasks estiverem feitas" (status), "fechar quando bater X horas"
(esforço cronometrado real) e "isto é um marco numa data" (deadline que vira item de calendário, fecho
manual). Hoje só o primeiro existe.

Esta fase **estende a fase 12** (não reescreve): adiciona `completion_mode` e `target_date` à tabela
`objectives` (migração `0026`), ensina o recálculo de status a respeitar o modo (sem quebrar o
`by_tasks` que produção já usa), expõe o modo/marco no contrato do `/board` (aditivo), e dá a aba
**Goals** rica no tab-shell da fase 24 (visão lista Notion) + reforça o sinal de dopamina ao fechar.
O `by_date` (marco) plugа no calendário da fase 28 via `target_date`.

A árvore `workspace › projeto › objetivo › task` (decisão fechada do CLAUDE.md) **não muda**: Goal
continua sendo o objetivo do projeto (camada de entrega), distinto das **sub-tasks "goals da task"** da
fase 24 (filhas da task, `task_subtasks` — **NÃO** unificar; são dois níveis diferentes, decisão §3.5
do PRD).

## 2. User stories

- **US-01** — Como dono/membro de um projeto, quero escolher **como um Goal conclui** (por tasks, por
  horas ou por data), pra cada entrega ter o critério que faz sentido, sem fingir que tudo é "% de
  tasks".
- **US-02** — Como usuário, quero que um Goal **por horas** mostre o progresso `horas realizadas /
  meta` e **conclua sozinho** quando eu bater a meta de horas, pra premiar o esforço cronometrado real
  (não só checkbox).
- **US-03** — Como usuário, quero marcar um Goal como **marco numa data** (`by_date`), pra ele virar um
  item no calendário do projeto e no "Upcoming", e fechá-lo **manualmente** quando o marco for batido.
- **US-04** — Como usuário, quero a aba **Goals** com uma visão lista Notion (Goal · Progresso · Tasks
  · Horas · Status) e um **New goal** que me deixa escolher o modo, pra gerir entregas num lugar só.
- **US-05** — Como usuário TDAH, quero **dopamina** ao fechar um Goal a 100% (feedback positivo, à
  prova de culpa) e **nada vermelho acusatório** quando estou atrasado, pra recomeçar sem vergonha.
- **US-06** — Como viewer do workspace, quero **só ler** Goals e seus modos do que é compartilhado, sem
  criar/editar nada.

## 3. Critérios de aceite (EARS)

### Modelo & migração

- **CA-01** *(ubíquo — migração `0026_objectives_modes`, RNF-03)* — A tabela `objectives` DEVE ganhar
  `completion_mode text NOT NULL DEFAULT 'by_tasks'` com `CHECK (completion_mode IN
  ('by_tasks','by_hours','by_date'))` e `target_date date` (nullable). Objetivos existentes (criados na
  fase 12) DEVEM continuar válidos com `completion_mode='by_tasks'` e `target_date=NULL` (default), sem
  qualquer mudança de comportamento (auto-close a 100% das tasks segue valendo — RNF-05).

### Goals — API (estende a fase 12; rotas aninhadas no projeto, inalteradas)

> `/api/v1/workspaces/{workspaceId}/projects/{projectId}/objectives[/{objectiveId}]`.
> A cadeia visibilidade→permissão→arquivado e o gate auto-contido da fase 12 ficam **intocados**.

- **CA-02** *(event-driven — criar com modo)* — QUANDO um usuário **com escrita no projeto** envia
  `POST .../objectives` com `completion_mode` ∈ `{by_tasks, by_hours, by_date}` (e os campos da fase
  12: nome válido, `goal_hours` opcional), o sistema DEVE criar o objetivo com o modo informado e
  retornar `201` com o objeto incluindo `completion_mode` e `target_date`. `completion_mode` **ausente**
  no payload = `'by_tasks'` (default — preserva o cliente da fase 12).
- **CA-02.1** *(unwanted — modo inválido)* — SE `completion_mode` for informado fora de
  `{by_tasks, by_hours, by_date}`, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400`
  `error.code="validation_error"` (validado no service ANTES do SQL — espelha `validStatus` da fase 12;
  nunca deixar o CHECK estourar pra `500`).
- **CA-03** *(event-driven — target_date)* — QUANDO o payload (POST/PATCH) informa `target_date` no
  formato `YYYY-MM-DD`, o sistema DEVE persisti-lo. SE `target_date` for informado mas malformado (não
  casa `YYYY-MM-DD`), o sistema DEVE retornar `400` `validation_error` ANTES do SQL.
- **CA-03.1** *(state-driven — target_date tri-state no PATCH)* — `target_date` no `PATCH` é
  **tri-state** (igual `goal_hours`/`due_date` da fase 02/12): **ausente** mantém o valor; **`null`**
  limpa o marco; valor `YYYY-MM-DD` define. O handler usa `json.RawMessage` pra distinguir ausente×null
  (padrão `patchObjectiveBody.GoalHours` da fase 12). `completion_mode` no PATCH é ponteiro simples
  (ausente mantém; valor válido troca o modo).
- **CA-04** *(event-driven — editar modo)* — QUANDO um usuário com escrita envia `PATCH
  .../objectives/{id}` com `completion_mode` e/ou `target_date`, o sistema DEVE persistir os campos
  presentes e retornar `200` com o objeto atualizado. Trocar o modo de um objetivo **não** zera
  `goal_hours` nem `target_date` (só são reescritos se vierem no payload — preservação, RNF-03).
- **CA-04.1** *(state-driven — `target_date` em modo ≠ `by_date` é "marco dormente", 29-H8)* — Como
  CA-04 **não** zera `target_date` ao trocar de modo, um objetivo `by_tasks`/`by_hours` PODE carregar um
  `target_date` residual (de quando foi `by_date`). Esse `target_date` é um **marco dormente**:
  preservado no banco (CA-04) mas **NÃO** renderizado como item de calendário. A renderização de marco
  é **gateada por `completion_mode='by_date'`** no agregador da fase 28 (ver CA-11): só objetivos
  `by_date` com `target_date` viram marco. Um `by_tasks`/`by_hours` com `target_date` residual **não**
  aparece no calendário do projeto nem no "Upcoming". (Defesa-em-profundidade opcional na UI: o
  `GoalModal` PODE enviar `target_date:null` no diff ao mudar pra um modo ≠ `by_date` — mas o gate
  `completion_mode='by_date'` no agregador da 28 é a correção robusta e suficiente.)

### Conclusão por modo (estende `recalcObjectiveStatus` — CA-06 da fase 12)

> O recálculo automático da fase 12 (`objective_link.go::recalcObjectiveStatus`, na transação da
> mutação de task, disparado SÓ por transição de `status`/`objective_id`/criação/deleção — D-04 da 12)
> é o ponto onde o auto-close mora. Esta fase o **estende** pra olhar o modo, **sem regredir** o
> `by_tasks`. O `FOR UPDATE` da fase 12 (serializa o recálculo por objetivo) **permanece**.

- **CA-05** *(state-driven — `by_tasks` inalterado, RNF-05)* — ENQUANTO `completion_mode='by_tasks'`, o
  recálculo automático DEVE se comportar **exatamente** como a fase 12: objetivo com ≥1 task e 100%
  `done` & `status='open'` → `done` (auto-conclui); <100% & `status='done'` → `open` (auto-reabre);
  `task_count=0` → no-op; senão no-op (preserva override manual — CA-06.1 da fase 12). **Nenhum teste
  da fase 12 pode regredir.**
- **CA-06** *(event-driven — `by_hours` auto-conclui ao bater a meta)* — QUANDO uma mutação de task que
  já dispara o recálculo (D-04 da 12: criar/deletar/mudar `status`/`objective_id`) ocorre num objetivo
  com `completion_mode='by_hours'` e `goal_hours` definido (`> 0`): SE `tracked_seconds` do objetivo
  (soma das entries FECHADAS das suas tasks — **MESMO bucket PER-USUÁRIO do `/board`**, CA-16 da 12,
  que filtra `e.user_id` da sessão; decisão de coordenação cross-fase E) `>= goal_hours ×
  3600` **e** `status='open'`, o sistema DEVE marcar o objetivo `done`; SE `tracked_seconds < goal_hours
  × 3600` **e** `status='done'` (e o objetivo está em `by_hours`), o sistema DEVE **reabri-lo** (`open`).
  O critério de `by_hours` é **independente** do done-ness das tasks (um objetivo `by_hours` pode ter 0
  tasks `done` e ainda fechar por horas). **O bucket de horas do auto-close é o MESMO que a UI renderiza
  (per-usuário)** — não há divergência possível entre o `status` persistido e a barra/`progress_pct`
  (29-H1/29-H2). Somar horas de TODOS os membros (esforço total da entrega) é **futuro/multiplayer**,
  fora desta fase.
- **CA-06.1** *(unwanted — `by_hours` sem meta NÃO fecha)* — SE `completion_mode='by_hours'` mas
  `goal_hours` é `NULL` (sem meta), o recálculo de horas DEVE ser **no-op** (não há limiar pra cruzar;
  o objetivo só fecha manualmente — CA-08). Um `by_hours` sem `goal_hours` nunca auto-fecha nem
  auto-reabre por horas.
- **CA-06.2** *(state-driven — `by_hours` e o gatilho de recálculo)* — O recálculo de horas (CA-06)
  roda nos **mesmos** pontos do recálculo da fase 12 (transição de task que toca o objetivo: criar/
  deletar/mudar `status`/`objective_id`). Como o tempo agregado muda por **fechamento de entry**
  (cronômetro), e o fechamento de entry **não** é uma mutação de task, o auto-close por horas
  **não** dispara ao parar o timer — ele só é recalculado no **próximo** evento que toca o objetivo
  (uma mutação de TASK). **Convergência condicionada a uma mutação de task futura:** se um goal
  `by_hours` bate a meta de horas e **NENHUMA** task dele sofre mutação depois (criar/deletar/mudar
  `status`/`objective_id`), o `status` **persistido** pode **NUNCA** virar `done` por conta própria.
  Carregar o `/board`/Goals **NÃO** converge o `status` persistido (o board é read-only). Esta latência
  condicionada é **aceita nesta fase** (decisão registrada no plan **D-04**; um recálculo amarrado ao
  fechamento de entry é o elo timer↔task, §9 do PRD, fora desta fase). **À prova de mentira na UI
  (CA-12/CA-12.1):** a aba Goals/Overview deriva "atingiu a meta" do `progress_pct`/`tracked_seconds`
  **cru** do `/board` (per-usuário — CA-06), então a barra mostra 100% e o feedback de conclusão
  mesmo quando o `status` persistido ainda está `open`. **Consumidores do `status` persistido** (ex.:
  Relatórios da fase 14) DEVEM derivar "concluído by_hours" do mesmo critério por modo (horas vs meta),
  não só do `status` cru — ver CA-06.3 — pra um goal by_hours com a meta batida não aparecer "Open" em
  lugar nenhum.
- **CA-06.3** *(state-driven — "concluído by_hours" coerente nos consumidores do `status` persistido,
  29-H7)* — Como o `status` persistido de um `by_hours` pode ficar `open` mesmo com a meta de horas
  batida (CA-06.2, latência condicionada a mutação de task), os consumidores que hoje lêem o `status`
  cru DEVEM derivar "concluído" **por modo**, pra um goal by_hours com a meta batida não aparecer
  inconsistente como "Open":
  1. **Relatórios (fase 14, `reports/objectives.go` / `ObjectivesSection.tsx` / `ReportTable.tsx`):**
     pra objetivos `by_hours`, "concluído" DEVE ser derivado de `tracked_seconds (per-usuário) >=
     goal_hours × 3600` (não só do `status`). A fase 14 expõe `progress_pct`/um critério por modo
     coerente com o board (CA-10.1) e o front deriva `done` desse critério por modo. **Teste Go pina:**
     um goal `by_hours` com a meta batida mas `status='open'` (sem mutação de task) aparece como
     concluído nos Relatórios. (Edição que toca a fase 14 — registrada como dependência cross-fase.)
  2. **Aba Goals / Overview (esta fase + fase 28):** já derivam o progresso cru por modo (CA-12.1 /
     CA-12.2), então o item mostra a barra cheia + feedback de conclusão (CA-14) mesmo com `status`
     persistido `open`. **Nenhum lugar mostra um by_hours com a meta batida como "atrasado/falho"**
     (RNF-03). A barra (cru, per-usuário) e o eventual chip `Done` (do `status`, quando uma mutação de
     task já o fechou) são **coerentes**: barra 100% ⇔ meta batida; chip Done só aparece quando o
     `status` é `done` — nunca chip Done sobre barra <100% (CA-06 fechou a meta per-usuário, a barra
     usa o mesmo total).
  > Alternativa de escopo (se a fase 14 ficar FORA desta rodada): registrar EXPLICITAMENTE em §6 que os
  > Relatórios podem mostrar um goal by_hours como "Open" até a próxima mutação de task — mas isso
  > deixa a incoerência viva (um goal com a meta batida aparecendo "Open" nos Relatórios), então a
  > correção limpa (derivar por modo nos consumidores do `status`) é a preferida.

- **CA-07** *(state-driven — `by_date` fecha SÓ manual)* — ENQUANTO `completion_mode='by_date'`, o
  recálculo automático DEVE ser **no-op** (nem auto-conclui nem auto-reabre): um marco fecha/reabre
  **somente** por `PATCH status` manual (CA-08). O `target_date` é um item de calendário (CA-11), não um
  gatilho de auto-close. Marcar tasks `done` num objetivo `by_date` **NÃO** o fecha.
- **CA-08** *(event-driven — fecho/reabertura manual em qualquer modo)* — QUANDO um `PATCH
  .../objectives/{id}` envia `status: "done"` ou `"open"`, o sistema DEVE fechar/reabrir o objetivo
  **em qualquer modo** (CA-05 da fase 12 preservado), independentemente do progresso por modo. O fecho
  manual persiste contra edições benignas de task (CA-06.1 da fase 12) — o recálculo por modo só o
  sobrescreve quando uma transição cruza o limiar do modo (CA-06 pra `by_hours`; nunca pra `by_date`).

### Roles, auth e isolamento (herda fase 12/02 — sem mudança de comportamento)

- **CA-09** *(unwanted)* — SE um `viewer` do workspace, um `access=view` no projeto, ou um membro que
  enxerga o projeto `shared` mas sem escrita enviar `POST`/`PATCH` de modo/marco, o sistema NÃO DEVE
  alterar nada e DEVE retornar `403` `forbidden` (gate da fase 12 intocado).
- **CA-09.1** *(unwanted)* — SE a rota referenciar projeto invisível/inexistente, ou o `{objectiveId}`
  não pertencer ao `{projectId}` da rota (objetivo de outro projeto, mesmo workspace), ou for uuid
  malformado, qualquer operação DEVE retornar `404` `not_found` (CA-13/13.1 da fase 12, query de carga
  inalterada — casa `(workspace_id, project_id, id)`). Sem cookie de sessão → `401` `unauthorized`.
- **CA-09.2** *(ubíquo — multi-tenant, RNF-04)* — Toda query nova DEVE filtrar pelo `workspace_id` da
  rota e respeitar a visibilidade do projeto; nenhum modo/marco/agregado de horas de outro workspace ou
  projeto invisível DEVE vazar.

### `/board` — contrato aditivo (RNF-05)

- **CA-10** *(state-driven — board expõe modo e marco)* — No payload de `GET
  .../projects/{projectId}/board`, cada objetivo (`BoardObjective`) DEVE incluir, **aditivamente**,
  `completion_mode` e `target_date` (`null` quando ausente), além dos campos da fase 12
  (`task_count`, `done_count`, `done_pct`, `tracked_seconds`, `goal_hours`). Nenhum campo existente
  muda de nome/tipo (RNF-05 — o `ProjectBoard` da fase 12 continua funcionando).
- **CA-10.1** *(optional — progresso coerente com o modo)* — O board PODE incluir, por objetivo, um
  campo derivado `progress_pct` coerente com o modo: `by_tasks` → `done_pct`; `by_hours` →
  `round(tracked_seconds / (goal_hours×3600) × 100)` clampado em `[0,100]` (`0` quando `goal_hours`
  nulo); `by_date` → `done_pct` (informativo; marco não tem barra de progresso própria). Se incluído,
  DEVE ser clampado em `[0,100]` e nunca dividir por zero. Se NÃO incluído, o cliente deriva o mesmo a
  partir dos campos crus (CA-12) — a presença do campo é uma comodidade, não obrigatória (decisão no
  plan **D-05**).

### Calendário (elo com a fase 28)

- **CA-11** *(state-driven — target_date no calendário do projeto)* — Um objetivo `by_date` com
  `target_date` definido DEVE ser exibível como **item datado** no calendário do projeto (fase 28) e no
  "Upcoming" do Overview, no dia de `target_date`. Esta fase **garante o dado** (`target_date` no
  contrato do board e do objetivo); a fase 28 consome (o endpoint de calendário do projeto agrega
  `tasks.due_date` + `objectives.target_date` + `projects.deadline`). **O agregador da fase 28 DEVE
  filtrar `completion_mode='by_date'`** (`WHERE project_id=$ AND completion_mode='by_date' AND
  target_date BETWEEN $from AND $to`, mantendo o guard `s.hasObjectiveTargetDate` da CA-15.1 da 28): só
  objetivos `by_date` viram marco — um `by_tasks`/`by_hours` com `target_date` residual (marco dormente,
  CA-04.1) **NÃO** aparece (29-H8). **CA-11 é asserido aqui só pela presença e correção de
  `target_date`/`completion_mode` no contrato** (a renderização e o gate por `completion_mode='by_date'`
  no calendário são teste da fase 28 — dependência cross-fase registrada no plan §8).

### UI — aba Goals (tab-shell da fase 24) + Overview (fase 28)

- **CA-12** *(state-driven — visão lista Goals)* — A aba **Goals** DEVE renderizar uma **lista** (tabela
  Notion: colunas **Goal · Progresso · Tasks · Horas · Status**) com os objetivos do projeto, derivada
  do `/board` (CA-10). Por linha: nome; barra de progresso `.tt-goal` (% coerente com o modo — CA-10.1
  ou derivado no client); `done/total` de tasks; horas (`tracked` formatado, + `/{goal_hours}h` quando
  há meta); chip de status (**Done** no accent quando concluído, **Open** caso contrário). A escolha de
  visão segue persistida (padrão `tt.view.*` da fase 12; a aba Goals reusa a chave do tab-shell). Mockup:
  `renderMetas` (linhas 471–479).
- **CA-12.1** *(state-driven — barra por modo, à prova de culpa, RNF-03)* — A barra de progresso da
  linha DEVE refletir o modo: `by_tasks` → % de tasks `done`; `by_hours` → % de horas vs meta (`0` sem
  meta, sem barra "vermelha"); `by_date` → progresso de tasks como referência informativa, **sem** marcar
  o item como "falho" mesmo se a data passou (atrasado nunca é vermelho acusatório — RNF-03; um marco
  vencido aberto aparece como **Open**, não como erro). A barra usa `var(--accent)` quando `>=100%` e a
  cor do projeto abaixo disso (regra do mockup, linha 476). Sem meta de horas num `by_hours`, a coluna
  Horas mostra só o `tracked` (sem `/Xh`) e a barra fica em 0.
- **CA-12.2** *(state-driven — barra por modo na Overview da fase 28 E no `ObjectiveCard` do board, 29-H6)*
  — A lista de Goals da aba **Overview** (fase 28, `ProjectOverview`/`renderOverview`) **e** o
  `ObjectiveCard` da fase 12 (usado no board↔lista do `ProjectBoard`) DEVEM refletir o **modo de
  conclusão** de cada objetivo, usando a MESMA função pura `goalProgress(obj)` da CA-12.1 (ou o
  `progress_pct` do board, que é por modo — CA-10.1), **não** o `done_pct` de tasks: `by_tasks` →
  `done_pct`; `by_hours` → % de horas vs meta (`0` sem meta, sem barra "vermelha"); `by_date` →
  `done_pct` informativo. A barra usa `var(--accent)` quando `>=100%` e a cor do projeto abaixo. **À
  prova de culpa (RNF-03):** nenhum elemento de erro/vermelho mesmo com `target_date` no passado; e um
  Goal `done` aparece **coerente** com sua barra por-modo — **proibido** um chip/check de conclusão
  sobre barra a 0% num `by_hours` que fechou por horas (CA-06 permite 0 tasks `done`; sem esta regra o
  ObjectiveCard mostraria barra de tasks a 0% sob um chip "Concluído"). Como o `progress_pct` por-modo já
  está no contrato do `/board` (CA-10.1, D-05, com o propósito declarado de servir Overview E aba
  Goals), a mudança é **trivial e sem regressão no `by_tasks`** (cujo `progress_pct == done_pct`): trocar
  a fonte da barra de Goals na Overview e no `ObjectiveCard` de `done_pct` para `progress_pct`/`goalProgress`.

- **CA-13** *(event-driven — New goal com modo)* — QUANDO o usuário com escrita aciona **New goal**, o
  sistema DEVE abrir um formulário onde escolhe o **modo** (`by_tasks`/`by_hours`/`by_date`) e preenche
  os campos relevantes (nome sempre; `goal_hours` quando `by_hours`; `target_date` quando `by_date`),
  e ao salvar chamar `POST .../objectives` com `completion_mode` (CA-02). Formulário inválido (nome
  vazio, `goal_hours` fora de `(0,9999]` em `by_hours`, `target_date` malformado em `by_date`) **NÃO**
  chama a API e sinaliza o erro. Editar um goal abre o mesmo formulário pré-preenchido (PATCH só com o
  diff — padrão da fase 12).
- **CA-14** *(event-driven — dopamina ao fechar, RNF-03)* — QUANDO um objetivo passa a `done` (auto por
  modo OU manual), a UI DEVE dar feedback positivo de conclusão (chip **Done** no accent + sinal visual
  reforçado, ex.: brilho/check animado curto no `ObjectiveCard`/linha) **sem nada vermelho**. Reabrir
  (`done→open`) volta ao estado **Open** discreto, sem mensagem de culpa. (O sinal de conclusão da fase
  12 já existe no `ObjectiveCard`; esta fase **reforça** o feedback visual.)
- **CA-15** *(state-driven — viewer)* — ENQUANTO o usuário for `viewer`/`access=view`/projeto arquivado
  (`board.project.can_write=false` — CA-16/22 da fase 12), a aba Goals NÃO DEVE exibir ações de escrita
  (New goal, editar, mudar modo, concluir/reabrir, excluir) — só leitura. A UI deriva de
  `board.project.can_write`, **não** da role do workspace sozinha.

### E2E

- **CA-16** — Logado (stub da fase 01) num projeto `shared` com escrita: o usuário **cria um Goal por
  horas** com `goal_hours`, vê a linha na aba Goals com barra de horas; **cria um Goal por data
  (`by_date`)** com `target_date` e o vê listado como **Open**; **fecha o Goal por data manualmente** e
  o vê **Done** (auto-close não fechou o `by_date` — CA-07); recarrega e o estado/modo persiste. O
  gating de `viewer`/`access=view` **NÃO** é asserido no E2E (o `googlestub` autentica um único usuário,
  o owner — limitação herdada da fase 12/24); **CA-15** é coberto por teste de componente jsdom
  (`can_write=false` mockado). O auto-close por horas (CA-06) é coberto por **teste Go** (precisa semear
  entries com `duration_seconds`, inviável no E2E sem cronômetro real).

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-24 | Goal ganha modo de conclusão: por tasks (%done), por horas (`goal_hours`), por data (`target_date`=marco) | CA-01, 02, 02.1, 03, 03.1, 04, 05, 06, 06.1, 06.2, 06.3, 07, 08, 10, 10.1 |
| RF-25 | Aba Goals (visão lista Notion) + fecho automático a 100% (dopamina à prova de culpa) | CA-12, 12.1, 12.2, 13, 14, 15, 16 |
| — | `target_date` alimenta o calendário do projeto (fase 28) | CA-11 |
| — | Roles/visibilidade/isolamento herdados (fase 12/02) aplicados aos campos novos | CA-09, 09.1, 09.2 |

## 5. Requisitos não-funcionais

- **RNF-01 (leveza)** — zero dependência Go nova (chi + pgx); validação na mão com stdlib (D-07 da
  fase 12). Frontend reusa TanStack Query + componentes sobre `tokens.css` — **nenhuma lib nova**.
- **RNF-02 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): chips de status, barra
  de progresso e o seletor de modo usam tokens/classes existentes (`--accent`, `--accent-soft`,
  `--accent-line`, `.tt-goal`, `.tt-card`, `--warn` pro ícone de star, `var(--p-*)`/`palette.ts` pra cor
  do projeto). Cor nova só como token, se necessário.
- **RNF-03 (à prova de culpa — lei TDAH, MILESTONE-3 §6)** — nada zera, nada fica vermelho acusatório.
  Marco vencido aberto = **Open** discreto, não erro. Conclusão = dopamina positiva. Sem barra vermelha.
- **RNF-04 (multi-tenant)** — toda query nova filtra por `workspace_id`/projeto e respeita visibilidade;
  CA-09.2 inegociável (SDD §8.4).
- **RNF-05 (não regredir — MILESTONE-3 §6 RNF-01/05)** — a fase 12 (`objectives.Service`, `/board`,
  recálculo `by_tasks`), a fase 14 (relatórios consomem objetivos — `ReportObjective`), a fase 28
  (calendário) e o TRACKING permanecem **funcionais**. Os campos `completion_mode`/`target_date` são
  **aditivos** (colunas nullable/com default, campos a mais no JSON). O recálculo `by_tasks` é
  **byte-equivalente** ao da fase 12 (teste de regressão obrigatório — CA-05). As listagens do
  `/board`/Goals seguem `< 200ms` (RNF-05 da M3; o board é o mesmo read único da fase 12, +2 colunas
  scaneadas — sem query nova).
- **RNF-06 (mobile)** — a aba Goals respeita o breakpoint `md` (768) da M2; a tabela Notion vira
  cartões `< md` (padrão `ReportTable`→cartões da fase 22).
- **Segurança/multi-tenant** — toda rota atrás de sessão+membership+role; recurso inexistente e
  invisível respondem o **mesmo** `404`.
- **PT-BR na UI** (os rótulos da aba Goals em inglês — Goal/Progress/Tasks/Hours/Status/New goal — são o
  padrão do mockup Notion-like, mantidos); identificadores de código em inglês (nomes de teste em PT-BR
  são exceção registrada).

## 6. Fora de escopo (fronteiras explícitas)

- **Recálculo de horas amarrado ao fechamento de entry** (auto-close por horas imediato ao parar o
  timer) → evolução do **elo timer↔task** (`MILESTONE-3.md §9.2`), fora desta fase. CA-06.2: o `status`
  persistido converge no próximo gatilho de **task** (latência condicionada a uma mutação de task
  futura — não converge ao carregar o board, que é read-only). A aba Goals/Overview deriva "atingiu a
  meta" do `tracked_seconds` cru (per-usuário) e os consumidores do `status` persistido (Relatórios,
  fase 14) derivam "concluído by_hours" por modo (CA-06.3) — assim **nenhuma UI mostra um by_hours com a
  meta batida como "Open/atrasado"**.
- **Soma de horas de TODOS os membros no `by_hours`** (esforço total da entrega entre co-membros) →
  **futuro/multiplayer** (`MILESTONE-3.md §9`). Nesta fase o auto-close E o `progress_pct` usam o bucket
  **per-usuário** do `/board` (decisão de coordenação cross-fase E) — coerentes entre si (29-H1/29-H2).
  Single-user-first (dogfooding solo, CLAUDE.md): o dono é o único membro relevante hoje.
- **`/goal` no Spotlight** (criar goal pela porta de captura) → é da **fase 27** (Spotlight). Esta fase
  entrega a entidade/modo e a aba; o slash-command consome. (O mockup mostra "Soon: `/goal` command,
  cross-project" como nota informativa — linha 468.)
- **Goals cross-project / roadmap Now/Next/Later** → backlog (anti-NOA, `MILESTONE-3.md §8`).
- **Renderização do `target_date` no calendário do projeto** → **fase 28** (esta fase só garante o dado
  no contrato — CA-11).
- **Reordenar Goals por drag** — `position` existe e é editável via PATCH (CA-04 da fase 12); a UI de
  reordenar segue simples (sem DnD nesta fase).
- **Notificação/lembrete de marco vencido** — `target_date` não dispara reminder nesta fase (a fase 20
  é dona de lembretes por `due_date` de **task**; estender pra objetivo é backlog). À prova de culpa: um
  marco vencido só aparece **Open**, sem alarme.
- **Sub-tasks "goals da task"** (`task_subtasks`, ⭐) → é da **fase 24**, nível diferente. NÃO unificar
  com Goals do projeto (decisão §3.5 do PRD).
- **Multi-tenant/compartilhar Goal** → fora (anti-NOA).
