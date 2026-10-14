# spec.md — Fase 12 · Gerenciador de tarefas (projeto → objetivos → tasks)

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Primeira fase da Milestone 1** ("Core Sólido" — `MILESTONE-1.md §4`). Resolve P1 e P2:
> tasks ganham **UI de verdade**, projetos ganham uma camada de **objetivos**, e o tempo de
> `time_entries` vira **agregado por task/objetivo/projeto**.
> **Decisões travadas com o Diogo (HITL 19/06/2026 — `MILESTONE-1.md §7`):**
> 1. Hierarquia **projeto → objetivo → tasks**.
> 2. **Progresso do objetivo = % de tasks `done` + meta de horas própria** (`goal_hours`
>    opcional, separada da `weekly_hours_goal` do projeto).
> 3. Status do objetivo conclui **automático em 100% das tasks `done`** OU é fechado
>    **manualmente** via PATCH.
> 4. Task sem objetivo = bucket **"Geral"** do projeto (nunca órfã).

---

## 1. Mapa pro problema central (REQUISITOS.md §2 + MILESTONE-1 §2)

A fase 02 criou clientes/projetos/tasks no banco e a fase 03 criou `time_entries` com
`task_id` — mas **tasks nunca ganharam interface** e não há nada entre "projeto" e "task"
pra planejar. Esta fase **estende a fase 02** (não reescreve): adiciona a tabela `objectives`,
liga `tasks.objective_id` (opcional), constrói a UI de gestão de tasks (board por status +
lista) e expõe o **tempo agregado** lido de `time_entries`. É o núcleo da M1: sem ela, P3
(dashboard), P4 (relatórios 2.0) e o "planejado×real por entrega" não têm objeto pra mostrar.

A base multiplayer da fase 02 (`tasks.assignee`, `tasks.requested_by`, `objectives.created_by`)
continua **só no schema, sem UI** — colaboração real (cross-assign, notas, lembretes) é M2.

## 2. User stories

- **US-01** — Como dono/membro de um projeto, quero criar **objetivos** dentro do projeto
  (nome, descrição opcional, **meta de horas opcional**), pra agrupar tasks por entrega e
  planejar acima do nível de task.
- **US-02** — Como membro com escrita no projeto, quero **criar/editar/mover/concluir tasks**
  numa UI (board por status `todo/doing/done` e/ou lista), associando cada task a um objetivo
  (ou deixando no bucket **"Geral"**), pra trabalhar o dia a dia sem ir ao banco.
- **US-03** — Como usuário, quero ver o **progresso de cada objetivo** (% de tasks concluídas
  + horas realizadas vs. meta de horas), pra saber o quão perto estou de entregar.
- **US-04** — Como usuário, quero que um objetivo **conclua sozinho** quando todas as suas tasks
  ficam `done`, e também **fechar/reabrir manualmente**, pra refletir a realidade.
- **US-05** — Como usuário, quero ver **quanto tempo já gastei** em cada task, objetivo e no
  projeto inteiro (lido das minhas entries), pra confiar nos números (P1).
- **US-06** — Como viewer do workspace, quero **só ler** objetivos e tasks do que é
  compartilhado, sem criar/editar nada.

## 3. Critérios de aceite (EARS)

### Objetivos — API

> Rotas aninhadas no projeto (igual à cadeia de visibilidade da fase 02):
> `/api/v1/workspaces/{workspaceId}/projects/{projectId}/objectives`.

- **CA-01** *(event-driven)* — QUANDO um usuário **com permissão de escrita no projeto** (dono
  do projeto, owner do workspace em projeto visível a ele, ou membro do projeto com
  `access=edit`) envia `POST .../projects/{projectId}/objectives` com nome válido (1–200
  caracteres após trim), o sistema DEVE criar o objetivo com `status="open"`, `created_by` =
  usuário da sessão, `position` = próximo no fim da lista do projeto, e retornar `201` com o
  objeto (id, workspace_id, project_id, name, description, status, goal_hours, position,
  created_by, created_at).
- **CA-01.1** *(unwanted)* — SE o nome do objetivo for vazio, só espaços ou tiver mais de 200
  caracteres, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400` com
  `error.code="validation_error"`.
- **CA-02** *(unwanted)* — SE `goal_hours` for informado e estiver fora do intervalo `(0, 9999]`
  (coluna `numeric(6,2)` — ver plan §2; o teto `9999` é validado no service **ANTES** do SQL pra
  garantir `400`, nunca `500` por overflow de `numeric`), o sistema NÃO DEVE criar/atualizar e
  DEVE retornar `400` com `error.code="validation_error"`. `goal_hours` ausente/`null` = **sem
  meta** (válido).
- **CA-03** *(event-driven)* — QUANDO um membro que **enxerga** o projeto (CA-09 da fase 02)
  envia `GET .../projects/{projectId}/objectives`, o sistema DEVE retornar os objetivos do
  projeto **ordenados por `position` asc** (desempate por `created_at, id`). O projeto invisível
  (private de outro) responde `404` (herda a visibilidade do projeto — CA-13).
- **CA-04** *(event-driven)* — QUANDO um usuário com escrita envia `PATCH
  .../objectives/{objectiveId}` com qualquer subconjunto de `{name, description, goal_hours,
  status, position}`, o sistema DEVE persistir os campos presentes e retornar `200` com o
  objeto atualizado. `goal_hours: null` **remove** a meta (tri-state: ausente mantém, null
  limpa).
- **CA-05** *(event-driven — conclusão manual)* — QUANDO um `PATCH .../objectives/{id}` envia
  `status: "done"`, o sistema DEVE marcar o objetivo como concluído mesmo que nem todas as
  tasks estejam `done`; `status: "open"` **reabre**. SE `status` estiver fora de `{open,done}`,
  DEVE retornar `400` `validation_error`.
- **CA-06** *(event-driven — conclusão/reabertura automática)* — O recálculo automático de
  status de um objetivo roda **na mesma transação** (D-09) **SOMENTE** quando uma mutação de task
  muda a composição ou o done-ness do objetivo: task **criada** no objetivo, **deletada** dele,
  com **`status` alterado**, ou com **`objective_id` alterado** (movida pra dentro/fora). Uma
  edição de task que toca **só** `name`/`notes`/`due_date`/`recurrence` (sem mudar `status` nem
  `objective_id`) **NÃO** dispara recálculo. Quando o recálculo roda para um objetivo com **≥1
  task**: SE 100% das tasks estão `done` e o objetivo está `open`, o sistema DEVE marcá-lo `done`;
  SE deixou de ser 100% e o objetivo está `done`, o sistema DEVE **reabri-lo** (`open`). Um
  objetivo **sem nenhuma task** (`task_count=0`) **NÃO** é tocado pelo recálculo — nem
  auto-concluído nem auto-reaberto (fica no status atual; só o fecho manual o move — CA-05).
  Mover uma task de objetivo A→B recalcula **os dois** (D-09).
- **CA-06.1** *(state-driven — override manual persiste contra edição benigna)* — Como o
  recálculo só dispara em mudança de `status`/`objective_id`/criação/deleção de task (CA-06), um
  objetivo **fechado manualmente** (CA-05, `status:done`) com `<100%` das tasks `done`, ou
  **reaberto manualmente** (`status:open`) com `100%` `done`, **PERMANECE** no status manual
  quando uma task tem só `name`/`notes`/`due_date`/`recurrence` editados. O override manual só é
  sobrescrito quando uma mutação de task **cruza** o limiar do recálculo (CA-06) — ex.: marcar
  uma task `done` ou `doing` muda o `status` e dispara o recálculo. Decisão registrada no plan
  **D-04**: sem coluna nova — a persistência vem de o recálculo **NÃO** rodar em edição benigna
  (RNF-01).
- **CA-07** *(event-driven)* — QUANDO um usuário com escrita envia `DELETE
  .../objectives/{objectiveId}`, o sistema DEVE remover o objetivo (hard delete) e as tasks que
  o referenciavam DEVEM ter `objective_id` setado para `null` (`ON DELETE SET NULL` — viram
  "Geral", nunca apagadas e nunca órfãs de FK).
- **CA-08** *(unwanted — escrita por quem não pode)* — SE um membro do projeto com
  `access=view`, ou um member/viewer do workspace que enxerga o projeto `shared` mas não está em
  `project_members`, enviar `POST`/`PATCH`/`DELETE` em objetivos desse projeto, o sistema NÃO
  DEVE alterar nada e DEVE retornar `403` `forbidden`. Viewer do workspace → `403` em qualquer
  escrita (CA-12).

### Tasks — extensão da fase 02 (vínculo com objetivo)

- **CA-09** *(event-driven)* — QUANDO um `POST .../tasks` ou `PATCH .../tasks/{id}` inclui
  `objective_id`, o sistema DEVE persistir o vínculo e devolver `objective_id` no corpo da
  task. `objective_id` é **tri-state** no PATCH (ausente mantém, `null` desassocia → "Geral").
- **CA-09.1** *(unwanted)* — SE o `objective_id` informado não existir, não pertencer ao
  **mesmo `project_id`** da task, ou não estiver no mesmo workspace, o sistema NÃO DEVE
  escrever e DEVE retornar `400` `error.code="invalid_reference"` (objetivo de outro projeto
  nunca vincula). `objective_id` **malformado** (não-uuid) no payload de `POST`/`PATCH` segue o
  **mesmo** caminho → `400 invalid_reference` (`uuid.Validate` antes do SQL, padrão do `client_id`
  da fase 02 — `projects.checkClientRef`). O `400 validation_error` de `objective_id` malformado é
  **exclusivo do filtro** `GET ?objective_id=` (CA-10), espelhando o `project_id` do filtro da
  fase 02.
- **CA-10** *(event-driven — filtro)* — QUANDO um membro envia
  `GET .../tasks?objective_id={id}`, o sistema DEVE retornar apenas as tasks daquele objetivo
  (combinável com os filtros `project_id`/`status` da fase 02). `objective_id` malformado →
  `400` `validation_error` (extensão do tratamento de filtro da fase 02).
- **CA-11** *(state-driven — bucket Geral)* — Tasks com `objective_id = null` pertencem ao
  bucket **"Geral"** do seu projeto: aparecem nas listagens do projeto, são editáveis/movíveis,
  e nunca contam pro progresso de nenhum objetivo nomeado. Toda task de um projeto **sempre**
  tem projeto; o objetivo é opcional.

### Roles, auth e isolamento — API (herda fase 02)

- **CA-12** *(unwanted)* — SE um usuário com role `viewer` no workspace enviar qualquer escrita
  (`POST`/`PATCH`/`DELETE`) em objetivos, o sistema DEVE retornar `403` `forbidden` e não
  alterar nada.
- **CA-13** *(unwanted)* — SE a rota referenciar um projeto **invisível** ao usuário (private de
  outro — CA-09 da fase 02) ou inexistente no workspace, qualquer operação de objetivo DEVE
  retornar `404` `not_found` (mesma resposta pros dois casos — não vazar existência).
- **CA-13.1** *(unwanted — pertencimento do objetivo à rota; teste de vazamento OBRIGATÓRIO)* —
  SE o `{objectiveId}` da rota não pertencer ao `{projectId}` da rota (objetivo de **outro
  projeto**, mesmo no MESMO workspace), for um uuid **malformado**, ou não existir, qualquer
  `GET`/`PATCH`/`DELETE` de objetivo DEVE retornar `404` `not_found` (mesma resposta — não
  vazar). A query de carga do objetivo SEMPRE casa `(workspace_id, project_id da rota, id)`, e
  `uuid.Validate(objectiveId)` roda antes do SQL pra nunca dar `500`. Pina que um usuário com
  escrita no projeto A **NÃO** edita/deleta um objetivo do projeto B via
  `PATCH /projects/A/objectives/{id-de-B}` (o gate do projeto A passa, mas o objetivo não é de A).
- **CA-14** *(unwanted)* — SE a requisição chegar sem cookie de sessão válido, qualquer rota
  desta fase DEVE retornar `401` `error.code="unauthorized"`.
- **CA-15** *(ubíquo — teste de vazamento OBRIGATÓRIO)* — O sistema DEVE sempre filtrar
  objetivos (e o tempo agregado) pelo `workspace_id` da rota **e** pela visibilidade do projeto:
  um usuário que referencie (na URL ou no payload) um objetivo de **outro** workspace, ou um
  `objective_id` que pertence a outro projeto/workspace, DEVE receber `404`/`invalid_reference`
  — nunca os dados do outro tenant. Toda agregação de tempo soma **só as entries do
  `user_id` da sessão** no workspace da rota.

### Tempo agregado (lido de `time_entries`)

- **CA-16** *(event-driven — visão do projeto com agregados)* — QUANDO um membro que enxerga o
  projeto envia `GET .../projects/{projectId}/board`, o sistema DEVE retornar, **num único
  request** (sem N+1), o projeto + seus objetivos (com progresso) + as tasks de cada objetivo +
  o bucket "Geral", cada task com seus **segundos somados** (`tracked_seconds`), cada objetivo
  com `tracked_seconds` (soma das tasks do objetivo) e `done_count`/`task_count`, e o projeto com
  `tracked_seconds` total. O `tracked_seconds` por **task** soma as entries FECHADAS
  (`duration_seconds` não nulo, `user_id` da sessão, `workspace_id` da rota) cujo **`task_id`
  pertence a uma task deste projeto** — **SEM** filtrar por `project_id` da entry (uma entry com
  `task_id` da task mas `project_id` nulo/diverso **ainda** conta pra task; D-06). O
  `tracked_seconds` do **projeto** soma as entries do user/workspace com **`project_id` = este
  projeto** (incluindo as sem `task_id`) — bucket **independente** do das tasks (CA-17). O objeto
  `project` do board inclui ainda **`can_write`** (boolean): `true` sse o usuário da sessão é dono
  do projeto, owner do workspace, **ou** membro do projeto com `access=edit`, **e** o projeto não
  está arquivado — o front usa esse campo no gating do CA-22 (a role do workspace sozinha não
  distingue `access=view`/`edit` num projeto `shared`).
- **CA-16.1** *(state-driven)* — No payload do `/board`, o **progresso do objetivo** DEVE conter:
  `task_count`, `done_count`, `done_pct = round(done_count/task_count×100)` (0 quando
  `task_count=0`), `tracked_seconds`, e `goal_hours` (ou null). O cliente deriva a barra de
  horas (`tracked_seconds / (goal_hours×3600)`); a API entrega os números crus.
- **CA-17** *(state-driven)* — As entries SEM `task_id` (ou cujo `task_id` foi anulado por
  delete da task — `time_entries.task_id ON DELETE SET NULL` da fase 03) **não** entram em nenhum
  agregado de task/objetivo; o `tracked_seconds` do projeto considera entries com `project_id`
  igual ao projeto (incluindo as sem task) — decisão do plan (D-06) fixa qual bucket cada entry
  alimenta. (Pina que "task deletada não infla objetivo", CA-07.) Os dois buckets são
  **independentes**: como `project_id` e `task_id` de uma entry **não** são forçados a concordar
  (schema da fase 03), `Σ tasks` pode **exceder** o `tracked` do projeto (entry com `task_id` mas
  `project_id` nulo/diverso) — isso é **esperado**; **NÃO** existe invariante `projeto ≥ Σ`.
- **CA-18** *(perf — verificação informativa, fora do red→green)* — Com 30 objetivos, 300 tasks
  e 1.000 entries num projeto, `GET .../projects/{id}/board` DEVE responder em **< 200 ms** no
  ambiente local de teste (melhor de 3). Nasce verde; se falhar, a correção é índice/SQL — não
  segue o ciclo red→green (exceção documentada, igual CA-32 da fase 02).

### UI — gerenciador de tarefas (cockpit)

- **CA-19** *(event-driven)* — A página de **projeto** (rota `/projetos/{id}` ou detalhe
  dedicado) DEVE renderizar os objetivos do projeto, cada um com nome, **progresso (% de tasks
  done + barra de horas quando há `goal_hours`)** e suas tasks, mais o bucket **"Geral"** das
  tasks sem objetivo. A escolha **board (colunas por status) ↔ lista** DEVE persistir por
  página no navegador (padrão da fase 02 — `localStorage`).
- **CA-20** *(event-driven — board)* — No modo **board**, as tasks DEVEM aparecer em colunas
  `todo/doing/done`; mover uma task de coluna DEVE chamar `PATCH .../tasks/{id}` com o novo
  `status` e refletir **sem recarregar a página**; ao virar a última task de um objetivo pra
  `done`, o cartão do objetivo DEVE refletir conclusão automática (CA-06) após a invalidação.
- **CA-21** *(event-driven — criar/editar)* — QUANDO o usuário com escrita cria ou edita uma
  task pela UI (nome, status, data opcional, notas, **objetivo** via select com opção "Geral"),
  a alteração DEVE aparecer na lista/board **sem reload**; criar/editar objetivo (nome,
  descrição, meta de horas) idem. Formulário inválido NÃO chama a API e sinaliza o erro.
- **CA-22** *(state-driven — viewer)* — ENQUANTO o usuário logado for `viewer` (ou `access=view`
  no projeto, ou o projeto estiver arquivado), a UI NÃO DEVE exibir ações de escrita (novo
  objetivo, nova task, editar, mover, concluir, arquivar) — só leitura. A UI deriva a permissão de
  **`board.project.can_write`** (CA-16), **não** da role do workspace sozinha (um member com
  `access=view` num projeto `shared` é `can_write=false`). (Backend já nega — CA-08/12.)
- **CA-23** *(state-driven — tempo na UI)* — Cada task DEVE exibir seu tempo rastreado
  (`tracked_seconds` formatado, ex.: `2h12`), cada objetivo o seu total e a barra de meta de
  horas (quando há `goal_hours`), e o projeto o total — todos vindos do `/board` (CA-16). Sem
  meta, a barra de horas NÃO é renderizada (regra do `.tt-goal`, igual CA-30 da fase 02).

### E2E

- **CA-24** — Logado (stub da fase 01) num projeto `shared` com escrita: o usuário **cria um
  objetivo** com meta de horas, **cria 2 tasks** nele (uma vai pra "Geral"), **move uma pra
  done** no board e vê o progresso do objetivo subir, **fecha a última task** e vê o objetivo
  concluir automaticamente; recarrega a página e o estado persiste. **O gating de
  `viewer`/`access=view` NÃO é asserido neste E2E**: o `googlestub` da fase 01 autentica um
  **único** usuário (o owner) — não há como o Playwright logar como um segundo perfil viewer
  (`seedWorkspaceMember` insere a membership no banco, mas não cunha sessão pra esse user; a fase
  02 também nunca observou a UI logado como viewer no E2E). O **CA-22** é coberto por **teste de
  componente jsdom** (padrão `viewer-gating.spec.tsx` da fase 02, com `can_write`/role mockados)
  nas tarefas T-09/T-10/T-13.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-01 | CRUD de objetivos por projeto (nome, descrição, meta de horas, ordem) | CA-01, 01.1, 02, 03, 04, 07 |
| RF-02 | Status do objetivo: auto-done em 100% das tasks + fecho/reabre manual | CA-05, 06 |
| RF-03 | Task ganha `objective_id` opcional (vínculo, filtro, bucket Geral) | CA-09, 09.1, 10, 11 |
| RF-04 | Roles/visibilidade/isolamento herdados da fase 02 aplicados a objetivos | CA-08, 12, 13, 14, 15 |
| RF-05 | Tempo agregado por task/objetivo/projeto (read único `/board`, sem N+1) | CA-16, 16.1, 17, 18 |
| RF-06 | UI: board por status + lista, objetivos com progresso, bucket Geral, tempo, gating viewer | CA-19..23 |

## 5. Requisitos não-funcionais

- **RNF-01 (leveza)** — zero dependência Go nova além do baseline (chi + pgx); validação na mão
  com stdlib (D-07 da fase 02). Frontend reusa TanStack Query + componentes próprios sobre
  `tokens.css` — **nenhuma lib de board/DnD nova** (mover task = botão/select de status, não
  drag-and-drop nesta fase — ver §6).
- **RNF-02 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): colunas do
  board, cartões de objetivo, barra de meta de horas e chips usam tokens/classes existentes
  (`--accent`, `--surface-*`, `--border`, `.tt-card`, `.tt-goal`, `.tt-dot`). Cor nova só como
  token, se necessário. Cor do projeto continua sendo dado de domínio (paleta `lib/palette.ts`,
  exceção sancionada da fase 02).
- **RNF-03 (migração)** — a fase 12 **é dona da migração `0013_objectives`** (próximo número
  livre — `0012_calendar_settings` é o último; INDEX atualizado pelo orquestrador). Cria a
  tabela `objectives` + `ALTER TABLE tasks ADD objective_id`. Nenhuma outra tabela nova.
- **RNF-04 (perf das agregações)** — o `/board` é **um** request (CA-16/18): a soma de tempo
  por task sai de **uma** query (`SUM(duration_seconds) … WHERE user_id=$ AND workspace_id=$ AND
  task_id = ANY($tasks do projeto) GROUP BY task_id`); a soma do **projeto** de outra
  (`WHERE project_id=$ AND user_id=$ AND workspace_id=$`); os objetivos de outra, as tasks de
  outra — montadas em memória em O(n). Índices: `(project_id, position)` em objectives,
  `(objective_id)` em tasks, e `time_entries (task_id) WHERE duration_seconds IS NOT NULL`
  (criado na `0013`, é o índice que a soma por task usa — os índices da 0009 cobrem só
  `user_id`/`workspace_id`+`started_at`). Nada de loop de query por objetivo (N+1 proibido).
- **RNF-05** — a fase 02 (rotas `/clients`, `/projects`, `/tasks` existentes) e a 03/07
  (`time_entries`, relatórios) permanecem **funcionais**; esta fase só ADICIONA `objective_id`
  em tasks (coluna nullable, não quebra contrato existente — o campo só aparece a mais) e a
  subárvore `/projects/{id}/objectives` + `/projects/{id}/board`. As entries continuam
  referenciando `task_id` como já fazem.
- **Segurança/multi-tenant** — toda rota atrás do middleware de sessão+membership da fase 01/02;
  recurso inexistente e invisível respondem o **mesmo** `404`; CA-15 é inegociável (SDD §8.4).
- **PT-BR na UI**; identificadores de código em inglês (nomes de teste em PT-BR são exceção
  registrada).

## 6. Fora de escopo (fronteiras explícitas)

- **Colaboração real** (cross-assign: criar/pedir task pra outro membro; **notas em task como
  fluxo de colaboração**; lembretes) → **M2**. A base no schema (`tasks.assignee`,
  `tasks.requested_by`) segue só no banco, sem UI. (As `notes` de task da fase 02 continuam um
  campo de texto livre simples, não um sistema de comentários.)
- **Drag-and-drop** de tasks entre colunas/objetivos → fase futura. Mover task = ação explícita
  (botão/select de status, select de objetivo) — sem lib de DnD (RNF-01).
- **Recurrence real / motor de RRULE** — `tasks.recurrence` segue **texto opaco** (D-08 da fase
  02); nada de gerar tasks recorrentes nesta fase.
- **Dashboard / home** que consome objetivos e recentes → **fase 13**.
- **Relatórios 2.0** (período flexível, filtro por objetivo, planejado×real por entrega,
  comparar períodos, KPIs) → **fase 14**. Esta fase entrega os **dados** (progresso de
  objetivo, tempo agregado); a apresentação rica é da 14.
- **Modo Foco 2.0 / Pomodoro** → fase 15. **Anti-timer-esquecido** → fase 16.
- **Reordenar objetivos por drag** — `position` existe e é editável via `PATCH` (CA-04), mas a
  UI de reordenar fica simples (mover pra cima/baixo ou nem isso nesta fase — decisão do plan).
- **Arquivar objetivo** — objetivo tem só `open/done` + hard delete (CA-07); sem soft-delete
  (diferente de projeto/cliente — objetivo concluído fica `done`, não arquivado).
