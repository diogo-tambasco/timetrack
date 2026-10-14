# spec.md — Fase 24 · Tasks 2.0 (status backlog · prioridade · sub-tasks · popup central · DnD · Notion · Inter)

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Primeira fase da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`). A maior
> alavanca: transforma a task num **índice executável + tempo** (a frase do Diogo) e o
> `ProjectDetail` num **workspace com abas**. Cobre os RFs do PRD: **RF-03 · RF-04 · RF-06 ·
> RF-07 · RF-08 · RF-09 · RF-10 · RF-12 · RF-13** (e prepara o tab-shell que 26/28/29 estendem).
> **Decisões travadas (PRD M3 §3 — NÃO reabrir):**
> 1. **Status da task (4):** `backlog · todo · doing · done`. Rótulos na UI: `Backlog / To do /
>    In progress / Done` (**`doing→In progress`** — só o rótulo; NÃO renomear o valor no banco).
> 2. **Prioridade (4):** `0 Sem prioridade · 1 Baixa · 2 Média · 3 Alta` (`No priority/Low/Medium/High`).
> 3. **Sub-tasks = "goals da task"** (tabela própria `task_subtasks`): título + feito + ⭐ é-meta.
>    A ⭐ é **separada** dos Goals/objetivos do projeto (dois níveis; NÃO unificar).
> 4. **Doc longa = link do Notion** (só link + preview client-side; **sem API/OAuth do Notion**).
> 5. **Task abre em popup central** com sub-tasks na **sidebar à direita**, **expansível pra tela
>    cheia**. **Descrição `.md` SEMPRE editável** (reusa `tasks.notes`; Enter no título → foco no corpo).
> 6. **Fonte = Inter** (a do Notion) em toda a UI; mono só pra números/tempo (esta fase é a 1ª
>    UI-heavy da M3 → é dona da troca Geist→Inter — decisão 11 do PRD).

---

## 1. Mapa pro problema central (REQUISITOS.md §2 + MILESTONE-3 §1)

A produção tem hoje (fase 12) um `/board` por projeto com objetivos + tasks + tempo agregado, mas a
task é **pobre**: só `todo/doing/done`, sem prioridade, sem ordenação manual, sem sub-tasks, sem
link de doc, e a UI move status por botões `‹ ›` (sem drag). A organização real do Diogo (TDAH) vive
no Notion. Esta fase **estende** o que existe (não reescreve a fase 12): a task ganha **backlog**,
**priority**, **position**, **notion_url** e uma camada de **sub-tasks** ("goals da task"); a UI
ganha o **popup central editável** (RF-07/08/09/10/12/13), o **board com drag-and-drop nativo**
(RF-04) e três visões persistidas (RF-03 — Board/List/Table) com quick-add (RF-06).

Esta fase também é a **dona do tab-shell** do projeto (Overview · Tasks · Notes · Goals · Calendar):
nela só a aba **Tasks** fica rica e a **Overview** ganha um esqueleto mínimo; **Notes** (fase 26),
**Calendar** (fase 28) e **Goals** com modos (fase 29) entram como abas vazias/placeholder que as
fases seguintes preenchem. O tab-shell é o ponto de integração compartilhado (ver §6 / plan §3.D-13).

O que **NÃO** muda: o cronômetro/sync/Pomodoro/foco (RNF-01 do PRD — zero-diff). O botão **Start**
no popup é **mock/desabilitado** nesta fase (o elo timer↔task real é a fase 30 — PRD §9.2).

## 2. User stories

- **US-01** — Como dono/membro com escrita num projeto, quero classificar a task num de **4 status**
  (`Backlog/To do/In progress/Done`) e numa de **4 prioridades**, pra refletir o estado real e o que
  é urgente.
- **US-02** — Como usuário, quero **arrastar** um card entre colunas no board e ele mudar de status
  (e guardar a ordem), pra organizar sem clicar em menus (RF-04).
- **US-03** — Como usuário, quero **3 visões** das tasks do projeto (Board · List · Table) com a
  escolha persistida, e **criar uma task em 1 gesto** (quick-add por coluna/grupo) — RF-03/06.
- **US-04** — Como usuário, quero **abrir uma task num popup central** com título grande editável,
  propriedades nativas (Status · Priority · Due · Time · Notion) e **descrição markdown sempre
  editável**, e poder **expandir pra tela cheia** — RF-07/08/09/10.
- **US-05** — Como usuário, quero uma **lista de sub-tasks** dentro da task (adicionar com Enter,
  marcar feita, marcar ⭐ = meta da task), com o cabeçalho mostrando `feitas/total` e
  `metas-feitas/total` — RF-12.
- **US-06** — Como usuário, quero **colar um link do Notion** numa task e ver um **preview bonito**
  (mark "N" + título do slug + `notion.so` + abrir em nova aba + desvincular), e um **ícone Notion**
  no card/lista/tabela quando há link — RF-13.
- **US-07** — Como viewer (ou `access=view`, ou projeto arquivado), quero **só ler** tasks e
  sub-tasks do que é compartilhado, sem criar/editar/arrastar nada.
- **US-08** — Como usuário, quero a UI inteira em **Inter** (a fonte do Notion), com números/tempo
  em mono — sem regredir o tema/dark-mode.

## 3. Critérios de aceite (EARS)

### Status, prioridade, position, notion_url — API (estende a fase 12)

> Rotas e gate herdados da fase 02/12: `POST .../tasks` e `PATCH .../tasks/{taskId}` sob
> `RequireMember`+`RequireWriteRole`, gate fino `loadProjectGate` (dono ∥ owner-ws ∥ `access=edit`,
> não arquivado). O carve-out viewer-assignee (PATCH de task única, fase 18) **permanece** —
> ver CA-11.

- **CA-01** *(state-driven — enum estendido)* — O enum `task_status` DEVE passar a aceitar **4
  valores** `backlog · todo · doing · done` (`backlog` posicionado **antes** de `todo`). O `DEFAULT`
  da coluna `tasks.status` **permanece `todo`** (uma task criada sem status nasce `todo`, nunca
  `backlog` — RNF-05 / migração não-destrutiva, plan §2). `validStatus` no Go DEVE aceitar os 4 e
  rejeitar qualquer outro com `400 validation_error` (CA-08).
- **CA-02** *(event-driven — criar com priority/notion_url)* — QUANDO um `POST .../tasks` inclui
  `priority` (inteiro `0..3`), `status` (∈ os 4) e/ou `notion_url` (texto), o sistema DEVE persistir
  e devolver os campos na task. `priority` ausente nasce `0`; `status` ausente nasce `todo`;
  `notion_url` ausente/`null` = sem doc. `position` nasce **no fim** da coluna de status da task no
  projeto (= `max(position)+1` entre as tasks do mesmo `project_id` e `status`; ver plan §3.D-03).
- **CA-02.1** *(unwanted — priority fora da faixa)* — SE `priority` for `< 0` ou `> 3` num
  `POST`/`PATCH`, o sistema NÃO DEVE escrever e DEVE retornar `400` `validation_error` (validado no
  Go ANTES do SQL — D-07; o CHECK na coluna é backstop).
- **CA-03** *(event-driven — patch tri-state)* — QUANDO um `PATCH .../tasks/{id}` envia qualquer
  subconjunto de `{status, priority, notion_url, position}`, o sistema DEVE persistir só os campos
  presentes e devolver a task. `notion_url` é **tri-state** (ausente mantém, `null` limpa, string
  define — padrão `due_date` da fase 02). `priority`/`position`/`status` são presentes-ou-ausentes
  (não há "null" pra eles — `priority`/`position` são inteiros sempre-presentes-por-default).
- **CA-04** *(ubíquo — notion_url sem validação de formato)* — O `notion_url` é **texto livre
  opcional** (limite de tamanho `≤ 2048` caracteres → `400 validation_error` se exceder; sem
  validar URL/host — o preview é client-side, RF-13). String vazia após trim = trata como `null`
  (sem doc).
- **CA-05** *(state-driven — recálculo de objetivo intocado)* — A mudança de `status` de uma task
  (inclusive **para/de `backlog`**) DEVE continuar disparando o **recálculo de status do objetivo**
  da fase 12 (CA-06 da fase 12), na mesma transação. `backlog` conta como **não-done** (só `done`
  fecha o objetivo). Mudança de **só** `priority`/`notion_url`/`position` é **edição benigna** e
  **NÃO** dispara recálculo (preserva o D-04 da fase 12). Mover task entre status via DnD = `PATCH
  status` → recalcula (CA-06 da fase 12 herdado, não reimplementar).

### Sub-tasks ("goals da task") — API (migração `0022_task_subtasks`)

> **Rotas (decisão de coordenação F — supersede "Route IRMÃO"):** sub-tasks são montadas no
> **Core normal** dentro de `mountCore`, no bloco `/tasks` já existente:
> `/api/v1/workspaces/{workspaceId}/tasks/{taskId}/subtasks[/{subtaskId}]` sob a cadeia herdada
> `RequireMember` + **`RequireWriteRole`** (o blanket do subrouter `/workspaces/{workspaceId}` —
> `auth_routes.go:86`). O gate fino no `subtasks.Service` (`loadTaskGate` → **`can_write` do projeto
> da task via a mesma lógica do `loadProjectGate`**) **permanece a autoridade** (resolve visibilidade
> → 404 e escrita → 403). **SEM carve-out viewer-assignee pra sub-tasks** (CA-11). NÃO referencia
> `objectives` (D-09 do PRD — separado dos Goals). Ver plan §4.1 / T-07 (mount Core, não sibling).

- **CA-06** *(event-driven — criar sub-task)* — QUANDO um usuário com escrita na task envia `POST
  .../tasks/{taskId}/subtasks` com `title` válido (1–500 caracteres após trim), o sistema DEVE criar
  a sub-task com `done=false`, `is_goal=false`, `position` = próximo no fim da lista da task, e
  retornar `201` com `{id, task_id, title, done, is_goal, position, created_at}`. `workspace_id` é
  gravado denormalizado (D-09) = o da task; o cliente **não** o envia.
- **CA-06.1** *(unwanted — título inválido)* — SE o `title` for vazio, só espaços ou `> 500`
  caracteres, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400` `validation_error`.
- **CA-07** *(event-driven — listar)* — QUANDO um membro que **enxerga** a task (a visibilidade
  herda do projeto da task — `(visibility='shared' OR owner_user_id=$user)`) envia `GET
  .../tasks/{taskId}/subtasks`, o sistema DEVE retornar as sub-tasks **ordenadas por `position` asc**
  (desempate `created_at, id`). Task invisível/inexistente → `404` (não vaza — CA-13).
- **CA-08** *(event-driven — patch sub-task)* — QUANDO um usuário com escrita envia `PATCH
  .../subtasks/{subtaskId}` com subconjunto de `{title, done, is_goal, position}`, o sistema DEVE
  persistir os campos presentes (tri-state por presença) e retornar `200`. `done`/`is_goal` são
  booleanos; `title` 1–500; `position` inteiro `≥ 0`.
- **CA-09** *(event-driven — deletar)* — QUANDO um usuário com escrita envia `DELETE
  .../subtasks/{subtaskId}`, o sistema DEVE remover a sub-task (hard delete) e retornar `204`.
  Deletar a task pai (CRUD da fase 02) DEVE remover as sub-tasks em cascata (`ON DELETE CASCADE` —
  plan §2).
- **CA-10** *(state-driven — progresso de metas da task)* — O agregado de progresso de metas de uma
  task = `count(*) FILTER (WHERE is_goal AND done)` / `count(*) FILTER (WHERE is_goal)`; o de
  sub-tasks gerais = `count(*) FILTER (WHERE done)` / `count(*)`. Esses números são derivados das
  linhas (o cliente conta; o `/board` pode embutir `subtask_count`/`subtask_done` — ver CA-14). NÃO
  há coluna agregada nova.

### Roles, auth, isolamento — sub-tasks (herda fases 02/12/18)

- **CA-11** *(unwanted — escrita por quem não pode)* — SE um membro com `access=view`, um viewer do
  workspace, ou um usuário num projeto cuja task ele não enxerga, enviar `POST`/`PATCH`/`DELETE` de
  sub-task, o sistema NÃO DEVE alterar nada e DEVE retornar `403 forbidden` (sem visibilidade →
  `404`, CA-13). **O carve-out viewer-assignee NÃO se estende a sub-tasks nesta fase** (sub-task é
  escrita de planejamento, não status da própria task) — viewer-assignee de uma task **não** cria/
  edita sub-tasks dela (decisão registrada, plan §4.1 / decisão de coordenação F: mount Core sob
  `RequireWriteRole`, gate fino no service, sem bypass viewer). O gate de sub-task usa o `can_write`
  do projeto da task (`loadProjectGate`), idêntico ao gate do board (CA-16 da fase 12). **Nota:** a
  cadeia `RequireWriteRole` do Core já barra o viewer do workspace cedo (403); o `loadTaskGate`
  cobre os casos finos restantes (`access=view` no projeto, projeto arquivado, sem visibilidade →
  404), preservando a mesma matriz de respostas.
- **CA-12** *(unwanted — sessão)* — SE a requisição chegar sem cookie de sessão válido, qualquer
  rota desta fase DEVE retornar `401 unauthorized`.
- **CA-13** *(unwanted — vazamento; teste OBRIGATÓRIO)* — SE a rota referenciar uma task **invisível**
  ao usuário (private de outro), inexistente, em **outro workspace**, ou um `taskId`/`subtaskId`
  **malformado** (não-uuid), qualquer operação de sub-task DEVE retornar `404 not_found` (mesma
  resposta pros casos — não vazar existência). A query de carga da sub-task SEMPRE casa
  `(workspace_id da rota, task_id da rota, id)`; `uuid.Validate` roda ANTES do SQL pra nunca dar
  `500`. Pina que uma sub-task da task B alcançada por `PATCH .../tasks/A/subtasks/{id-de-B}` → `404`.
- **CA-14** *(state-driven — board estendido, opcional/aditivo)* — O `/board` da fase 12 PODE
  incluir, por `BoardTask`, `subtask_count` e `subtask_done` (e os de meta `goal_count`/`goal_done`)
  numa **query agregada única por projeto** (GROUP BY task_id, sem N+1 — RNF-04), e `priority`/
  `notion_url`/`position`/`status` (já no objeto Task). **NÃO é obrigatório o board listar as
  sub-tasks inteiras** (o popup busca via `GET .../subtasks`). Os campos novos da Task no `BoardTask`
  (priority/notion_url/position) DEVEM aparecer (são colunas de `tasks`). O board DEVE ordenar as
  tasks de cada coluna/bucket por `position` asc (desempate `created_at`).
- **CA-14.1** *(state-driven — cliente no header, extensão aditiva do `/board`)* — O `BoardProject`
  do `/board` (fase 12) DEVE ganhar **`client_name`** (`string | null`), resolvido por **1 LEFT JOIN
  `clients`** no SELECT de metadados do projeto (`board.go`: `LEFT JOIN clients c ON c.id =
  p.client_id` → `c.name`); `null` quando o projeto não tem cliente. É a **única** fonte do nome do
  cliente exibido no header do tab-shell (CA-15) — sem 2º fetch (`useProjects`/`useClients`). O
  campo é **aditivo** (não muda nenhum campo existente do `BoardProject`); o `BoardProject` do TS
  (`lib/api.ts`) espelha `client_name: string | null`. Teste no board: `board.project.client_name` =
  nome do cliente do projeto; `null` se projeto sem cliente (mutação que omite o LEFT JOIN/coluna
  morre).

### UI — workspace de projeto com abas + aba Tasks rica

- **CA-15** *(state-driven — tab-shell)* — A rota `/projetos/{id}` (`ProjectDetail`) DEVE renderizar
  um cabeçalho do projeto (**cor + nome + cliente**, onde o cliente vem de `board.project.client_name`
  — CA-14.1; chip de cliente só aparece quando `client_name != null`) e uma barra de abas
  **Overview · Tasks · Notes ·
  Goals · Calendar**. Nesta fase: **Tasks** é a aba rica; **Overview** mostra um esqueleto (KPIs
  básicos do `/board` + andamento por status + goals — sem inventar dados); **Notes/Goals/Calendar**
  são **placeholders** ("em breve" / vazio) que as fases 26/29/28 substituem. A aba ativa **NÃO**
  precisa persistir entre reloads nesta fase (decisão; default = Tasks ou Overview — ver plan §6).
- **CA-16** *(event-driven — 3 visões + persistência)* — A aba Tasks DEVE oferecer **Board · List ·
  Table** num segmented control; a escolha DEVE persistir em `localStorage` na chave
  **`tt.view.board`** (mesma chave/padrão da fase 12 — D-11 da 12, decisão travada RF-03). Recarregar
  mantém a visão. **O helper `useView` (em `components/ui/ViewToggle.tsx`) DEVE ser reescrito de
  binário (`'cards'|'table'`, default `cards`) pra genérico de N valores** — assinatura
  `useView<T extends string>(key, options: readonly T[], fallback: T): [T, (v: T) => void]` — pra
  suportar as **3 strings** desta fase: `'board' | 'list' | 'table'` (default `'board'`). Os
  consumidores existentes (`Clients`/`Projects` com `'cards'|'table'`) passam a chamar a versão
  genérica com `['cards','table'],'cards'`, **sem mudança de comportamento**.
- **CA-16.1** *(state-driven — migração do valor legado, teste OBRIGATÓRIO)* — A chave `tt.view.board`
  já pode conter os valores **legados da fase 12** (`'cards'` = board de colunas; `'table'` = a LISTA
  `TaskRow`). Ao ler `tt.view.board` na aba Tasks nova, o helper DEVE **mapear o legado** pra não
  regredir a preferência: `'cards'` → **`'board'`** (board de colunas) e `'table'` → **`'list'`** (a
  visão de lista, que herda semanticamente a LISTA `TaskRow` da fase 12). **A Table NATIVA é gravada
  sob um token de persistência PRÓPRIO (`'table-v2'`)** — não o `'table'` cru, que é o valor LEGADO da
  fase 12 → assim a Table nativa **persiste de verdade no reload** (RF-03) sem colidir com o legado. O
  helper decodifica o token próprio **ANTES** do mapeamento legado (precedência), e `change('table')`
  grava `'table-v2'`. Qualquer valor desconhecido → default `'board'`. Teste OBRIGATÓRIO:
  `tt.view.board='cards'` → renderiza Board; `='table'` (token LEGADO cru) → renderiza **List** (não
  Table); `change('table')` grava `'table-v2'` e o **reload reabre a Table** (round-trip da Table
  nativa — `escreve→lê→renderiza Table`, NÃO List); `'board'/'list'` fazem round-trip direto; valor
  desconhecido → `'board'`. (Mata DUAS mutações: a que ignora o legado e cai direto em Table com o
  `'table'` cru; E a que dropa o token próprio e faz a Table nativa regredir pra List no reload.)
  > **Histórico (24-H1 → reaberto 23/06, HITL):** a v1 da CA-16.1 mandava `'table'` SEMPRE → List
  > (legado tem precedência total), o que tornava a Table nativa **impersistível** — contradição
  > interna (a própria CA pedia round-trip de `'table'`) que feria o RF-03/object-permanence. Reaberta
  > com o Diogo e resolvida pelo **token distinto `'table-v2'`** (decisão da sessão 23).
- **CA-17** *(event-driven — board com DnD nativo)* — No **Board**, as tasks DEVEM aparecer em **4
  colunas** `backlog/todo/doing/done` (rótulos `Backlog/To do/In progress/Done`), cada coluna
  ordenada por `position`. **Arrastar** um card pra outra coluna (drag-and-drop **HTML5 nativo** —
  `draggable` + `ondragover`/`ondrop`, **sem lib de DnD** — RF-04/RNF-01) DEVE chamar `PATCH
  .../tasks/{id}` com o novo `status` e refletir **sem recarregar** (invalida o `boardKey`). Ao
  arrastar a última task de um objetivo pra `done`, o cartão do objetivo reflete conclusão automática
  (CA-05 / fase 12) após a invalidação.
- **CA-18** *(event-driven — quick-add por coluna/grupo)* — Cada coluna do board (e cada grupo de
  status na List) DEVE ter um **quick-add** ("Add") que cria uma task **já no status daquela coluna**
  em 1 gesto (RF-06): clicar cria a task (nome default editável) e abre o popup com o título focado
  (captura — RNF-02). O botão "New task" cria no `todo`.
- **CA-19** *(state-driven — Table nativa)* — A visão **Table** DEVE listar as tasks com colunas
  **nativas** `status · priority · due · to-dos (sub-tasks feitas/total) · time` (RF-05; as
  propriedades **custom** são da fase 25 — fora desta fase). Clicar no status/priority na célula
  cicla o valor (1 clique → `PATCH`); clicar na linha abre o popup. To-dos = `subtask_done/
  subtask_count` (do board ou de uma contagem leve).
- **CA-20** *(event-driven — popup central da task)* — Clicar/criar uma task DEVE abrir um **popup
  central** (overlay com scrim) contendo: **título grande editável** (input), uma seção de
  **propriedades** (Status [chip 1-clique → cicla], Priority [chip 1-clique → cicla], Due
  [date-picker], **Time** [`tracked_seconds` formatado real do `/board` + botão **Start**
  desabilitado/mock nesta fase — CA-26], Notion [campo/preview]), a **descrição `.md`** (textarea
  sempre editável), e uma **sidebar à direita** com as sub-tasks. Editar título/status/priority/due/
  descrição/notion reflete **sem reload** (otimista ou via invalidação — plan §6).
- **CA-21** *(event-driven — Enter no título → corpo)* — ENQUANTO o foco está no input de **título**
  da task no popup, pressionar **Enter** DEVE mover o foco pro **textarea de descrição** (e posicionar
  o cursor no fim) — edição fluida (RF-09). O título salva no blur/debounce (PATCH name); a descrição
  salva no blur/debounce (PATCH notes). Formulário/edição inválida (ex.: nome vazio) NÃO chama a API
  e sinaliza (não cria task sem nome — usa o default "New task" até o usuário digitar).
- **CA-22** *(event-driven — expandir pra tela cheia)* — O popup DEVE ter um toggle **expandir/voltar**
  (ícones `expand`/`collapse`): em modo **central** mostra scrim + card médio; em modo **tela cheia**
  ocupa a viewport (RF-08). O toggle alterna entre os dois sem fechar o popup nem perder o estado de
  edição. Fechar (✕ ou clique no scrim em modo central) fecha o popup.
- **CA-23** *(event-driven — sub-tasks na sidebar)* — Na sidebar do popup: digitar e dar **Enter** no
  input "add item" DEVE criar uma sub-task (`POST .../subtasks`) e limpar o input (mantendo o foco);
  clicar no checkbox marca/desmarca `done` (`PATCH done`); clicar na ⭐ marca/desmarca `is_goal`
  (`PATCH is_goal`). O cabeçalho mostra `feitas/total` e, se houver metas, `metas-feitas/total ⭐`.
  Tudo reflete sem reload (RF-12). Sem sub-tasks → "No sub-tasks yet."
- **CA-24** *(event-driven — preview do Notion)* — QUANDO há `notion_url` na task: o popup DEVE
  mostrar um **card de preview** (mark "N" + **título derivado do slug** + `notion.so` + abrir em nova
  aba [`target="_blank"`] + botão **desvincular** que faz `PATCH {notion_url:null}`); e o card/linha/
  célula da task na aba Tasks DEVE mostrar um **ícone Notion**. QUANDO **não** há link: o campo Notion
  é um input "Cole o link da página do Notion…" que ao confirmar faz `PATCH {notion_url}`. O **título
  do slug** é derivado client-side: pega o último segmento do path, remove o hash hex final
  **case-insensitive** (`-[0-9a-fA-F]{6,}$` — IDs de página do Notion podem vir em maiúsculas/mistas
  dependendo da URL colada), troca `-` por espaço (igual `notionTitle()` do mockup,
  `projeto-workspace.html` linha 261). Teste obrigatório com hex MAIÚSCULO:
  `notionTitle('https://www.notion.so/Projeto-8F3A1C9B')` → `"Projeto"` (impede regressão pro regex
  lowercase-only).
- **CA-25** *(state-driven — gating viewer)* — ENQUANTO o usuário não pode escrever no projeto
  (`board.project.can_write === false` — CA-16 da fase 12: viewer do ws, `access=view`, ou projeto
  arquivado), a aba Tasks NÃO DEVE permitir ações de escrita: o **drag** fica desabilitado, os
  quick-add/New task somem, os chips de status/priority no popup/table **não** ciclam, o date-picker
  fica read-only, o input de sub-task e os toggles de checkbox/⭐ somem, e o campo de Notion vira
  read-only (preview-only). A UI deriva isso de **`board.project.can_write`**, NÃO da role do ws
  sozinha (padrão CA-22 da fase 12). (Backend já nega — CA-11.)
- **CA-26** *(state-driven — Start mock IDENTIFICÁVEL)* — O botão **Start** no campo Time do popup
  DEVE existir como um **elemento estável e identificável** — `data-testid="task-start"` (decisão de
  coordenação B) — mas estar **desabilitado/não-funcional** nesta fase (tooltip/aria indicando
  "em breve" — o elo timer↔task é a fase 30, PRD §9.2). Clicar nele NÃO inicia timer nem chama a API.
  A **fase 30** apenas TROCA a ação desse mesmo elemento (`onClick → timerStore.start`) e adiciona o
  chip "rodando" — **não há "slot/componente provido pela 24"**; é o **mesmo botão** que a 24 cria. O
  **valor** de Time exibido é o `tracked_seconds` real da task (do `/board`), formatado (`2h12`).

### Fonte Inter

- **CA-27** *(ubíquo — Inter em toda a UI)* — A variável `--font-ui` (em `tokens.css` / `@theme`)
  DEVE passar a apontar pra **Inter** (self-hosted woff2, peso ≥ regular+medium+semibold), aplicada
  em toda a UI; `--font-mono` (números/tempo) **permanece** inalterada. O tema (dark/light) e o gate
  `check-cores` DEVEM continuar verdes (a troca de fonte não toca em cor). Os arquivos da fonte ficam
  em `frontend/public/fonts/` e são precacheados/servidos sem CDN externo (RNF-01 — leveza/offline).

### E2E

- **CA-28** — Logado (stub da fase 01) num projeto `shared` com escrita, na rota `/projetos/{id}`,
  aba Tasks: o usuário **(a)** abre o **Board** (4 colunas), **(b)** cria uma task pelo quick-add de
  uma coluna (ex.: `backlog`) e ela aparece naquela coluna, **(c)** abre a task (popup central),
  **edita o título**, **(d)** adiciona uma **sub-task** pela sidebar e a marca como ⭐ meta e como
  feita, **(e)** cola um **link do Notion** e vê o preview, **(f)** **expande** o popup pra tela cheia
  e volta, **(g)** fecha, troca pra **Table** e a task aparece com priority/due/time, **(h)**
  recarrega e o estado persiste. **O gating de `viewer`/`access=view` NÃO é asserido neste E2E** (o
  `googlestub` da fase 01 autentica um único usuário/owner — limitação registrada desde a fase 12,
  CA-24). **O DnD entre colunas é asserido por teste de componente jsdom** (Playwright HTML5 drag é
  flaky; o E2E cobre o quick-add e a mudança de status pelo popup); a função de drop é pura/testável.
  **CA-25** é coberto por **teste de componente jsdom** (`can_write=false` mockado), padrão
  `viewer-gating.spec` da fase 12.
- **CA-28.1** *(REGRESSÃO — reconciliação dos E2E/specs da fase 12/18/19, DESVIO SANCIONADO)* — A
  fase 24 **substitui** o board simples da fase 12 (4 colunas + DnD no lugar do `‹ ›`/3 colunas) e o
  `TaskModal` (popup `TaskPanel` no lugar do dialog). Os artefatos abaixo, que assertam a UI antiga,
  DEVEM ser reconciliados pro novo fluxo (asserções de **dado** preservadas; só os seletores/fluxo
  mudam) ANTES do `make check-full` (T-15/T-16):
  - **E2E Playwright:** `frontend/e2e/tarefas.spec.ts` (fase 12) — `getByRole('button',{name:/tabela/i})`
    do ViewToggle binário → segmented **Board/List/Table**; `getByLabel('Status de …').selectOption('done')`
    do `TaskRow` → **chip de status 1-clique** (ou drop no Board); contagens `0/1 task` → contagem na
    coluna/popup; dialog `Nova task`/`Criar task` (TaskModal) → **popup central `TaskPanel`** com
    título focado; a **conclusão automática do objetivo** passa a ser verificada na aba **Overview**
    (não mais no card do board). `frontend/e2e/comentarios.spec.ts` (fase 19) e
    `frontend/e2e/cross-assign.spec.ts` (fase 18) — abrir a task pelo **`TaskPanel`** (não mais o
    dialog `Editar task`); criação via quick-add/popup; Responsável/Solicitante migram pro fluxo do
    popup. `frontend/e2e/mobile.spec.ts` (M2) — confirmar que o board reshaped segue **verde a 375px**
    (9 rotas, zero overflow horizontal).
  - **Specs jsdom da fase 12/19:** `components/board/TaskBoardColumns.spec.tsx` (rótulos `A fazer/Fazendo/
    Feito`, botões `/mover.*direita/`, `grid-cols-3`) e `ProjectBoard.spec.tsx` / `ProjectBoard.autoclose.spec.tsx`
    são **reescritos** pro board novo (4 colunas + `dropStatus` DnD; a conclusão automática vira spec do
    board novo: última task → `done` via drop/popup → objetivo concluído). **`ProjectBoard.canComment.spec.tsx`
    é PRESERVADO**: `canCommentOn(canWrite, task, currentUserId)` (`ProjectBoard.tsx:25`) é **load-bearing**
    pro carve-out viewer-assignee da fase 19 — a função pura E seu spec **migram pro `TaskPanel`/`TasksTab`
    sem regressão** (o thread de comentários da fase 19 aparece no popup, gateado por `canCommentOn`).
  Isto é **desvio sancionado** (a fase 24 é dona do board) e está detalhado em plan §6 e tasks T-11/
  T-12/T-15/T-16.

## 4. Requisitos funcionais (resumo)

| RF (PRD M3) | Descrição | CAs |
|---|---|---|
| RF-03 | 3 visões persistidas Board/List/Table (`tt.view.board`) | CA-16, 19 |
| RF-04 | Board com drag-and-drop nativo → persiste status (+position) | CA-17 |
| RF-06 | Quick-add por coluna/grupo (criar em 1 gesto) | CA-18 |
| RF-07 | Popup central: título + propriedades + descrição .md + sidebar sub-tasks | CA-20, 23 |
| RF-08 | Popup expansível pra tela cheia e voltar | CA-22 |
| RF-09 | Descrição sempre editável; Enter no título → corpo | CA-20, 21 |
| RF-10 | Status/Priority 1-clique; Due date-picker; Time real + Start (mock) | CA-20, 26 |
| RF-12 | Sub-tasks: add/feito/⭐ meta; cabeçalho feitas/total + metas | CA-06..10, 23 |
| RF-13 | Notion: colar link → preview + abrir/desvincular; ícone na visão | CA-04, 24 |
| (infra) | Status backlog · priority · position · notion_url no modelo | CA-01..05, 14 |
| (decisão 11) | Fonte Inter em toda a UI; mono pra números | CA-27 |

## 5. Requisitos não-funcionais

- **RNF-01 (TRACKING intocado)** — o cronômetro/sync/Pomodoro/foco/timer.Service ficam
  **byte-idênticos** onde não for o elo; o **Start** do popup é mock (CA-26). Zero dependência Go
  nova (chi + pgx + uuid do baseline). Frontend: **sem lib de DnD** (HTML5 nativo — RF-04), **sem lib
  de markdown** (a descrição da task é textarea simples; o render é client-side trivial, como o
  `md()` do mockup — fora do escopo de teste rigoroso, só preview leve). Fonte Inter self-hosted (sem
  CDN).
- **RNF-02 (captura < 2s)** — quick-add e `New task` criam com só o nome (default editável) e abrem o
  popup focado no título — nenhum fluxo de criação exige mais que nome.
- **RNF-03 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): colunas do board,
  card de task, chips de status/priority usam tokens/classes existentes (`--accent`, `--surface-*`,
  `--border`, `--text-*`, `--warn`, `.tt-card`, `.tt-goal`, `.chip`). Cor de status já é token
  (`--text-3`/`--text-2`/`--accent`/`--p-primor` — mockup linha 197). Cor nova só como token.
  **O mark "N" do Notion** (fundo branco `#fff` + glifo escuro `#111` no mockup) **NÃO** entra como
  hex literal nem na allowlist do `scripts/check-cores.sh` (que usa **regex ANCORADA por arquivo
  exato** — `^frontend/src/lib/palette\.ts:`, `^frontend/src/styles/tokens\.css:` —, **não** substring
  de path; editar o gate seria mudança não-prescrita). Em vez disso, **dois tokens novos OBRIGATÓRIOS**
  em `tokens.css` (dark **e** light) — `--notion-mark-bg` e `--notion-mark-ink` — e o `NotionPreview`
  usa `var(--notion-mark-bg)` / `var(--notion-mark-ink)`. Zero hex literal fora do `tokens.css` →
  `check-cores` segue verde sem tocar o script.
- **RNF-04 (perf / sem N+1)** — o `/board` continua **um** request; se embutir `subtask_count/done`
  (CA-14), é **uma** query agregada por projeto (`GROUP BY task_id`), com índice
  `(task_id)`/`(task_id, is_goal)` em `task_subtasks`. Listagens do board < 200ms com 30 obj/300
  tasks/sub-tasks (padrão CA-18 da fase 12 — exceção informativa fora do red→green).
- **RNF-05 (não regredir)** — a fase 02 (CRUD `/tasks`), a fase 12 (`/board`, objetivos), a 18
  (assignee/visões/carve-out), a 19 (comentários), a 20 (lembretes/due hook) permanecem
  **funcionais**. Esta fase só **ADICIONA** colunas a `tasks` (todas com DEFAULT, não quebram o
  contrato), uma tabela `task_subtasks`, e o subrouter `/tasks/{id}/subtasks`. O **DEFAULT de status
  continua `todo`** (CA-01) e nenhum statement da `0021` referencia `'backlog'` (PRD §9.3 — `ALTER
  TYPE … ADD VALUE` numa transação no PG17 só vale se o valor novo não for usado no mesmo arquivo).
  O `due hook` (fase 20) e o `recalcObjectiveStatus` (fase 12) seguem disparando exatamente como hoje
  (CA-05).
- **Segurança/multi-tenant (RNF do PRD §6.4)** — toda query nova de sub-task filtra por
  `(workspace_id, task_id)` e a visibilidade herda do projeto da task; recurso inexistente e
  invisível respondem o **mesmo** `404` (CA-13, inegociável — SDD §8.4).
- **PT-BR na UI** (rótulos de status/priority em inglês são **dados de domínio** alinhados ao mockup
  e ao Notion — `Backlog/To do/In progress/Done`, `No priority/Low/Medium/High`); identificadores de
  código em inglês (nomes de teste em PT-BR são exceção registrada).
- **RNF-06 (mobile)** — o popup central e o board respeitam o breakpoint 768 (`md`) da M2: < md o
  popup ocupa a tela (sem sidebar lado-a-lado; a sidebar vira seção abaixo) e o board rola
  horizontalmente; ≥ md = layout do mockup. Não regredir o `mobile.spec` (9 rotas, zero overflow).

## 6. Fora de escopo (fronteiras explícitas)

- **Propriedades tipadas custom** (text/number/select/multi-select/date/person/checkbox/url por
  projeto; `project_property_defs`/`task_property_values`) → **fase 25**. O popup desta fase mostra só
  as **propriedades nativas** (Status/Priority/Due/Time/Notion); o botão "Add property" do mockup
  (linha 553) e os `customRows` (528-531) **NÃO** são construídos aqui (a fase 25 os liga). A Table
  lista só as colunas nativas (CA-19).
- **Notas nativas (.md) + slash `/` + `note_links`** (aba **Notes**, editor, `/task`/`/note`) →
  **fase 26**. Nesta fase a aba Notes é placeholder. O slash `/` **não** existe na descrição da task
  (decisão PRD §9.4 — a descrição é markdown simples; slash só nas Notas, fase 26).
- **Spotlight (Cmd+K)** → **fase 27**. Nesta fase não há Cmd+K nem comando `/task` global.
- **Calendário por projeto** (`projects.deadline`, endpoint agregado, overlay GCal) e **cards vivos**
  na lista de projetos → **fase 28**. A aba Calendar é placeholder; a `Projects.tsx` (lista) **não**
  ganha card vivo aqui (segue como está; o link pro detalhe da fase 12 permanece).
- **Goals com modos** (`completion_mode`/`target_date`, milestone, visão lista de Goals) → **fase
  29**. A aba Goals é placeholder; o cartão de objetivo/progresso da fase 12 segue na Overview.
- **Elo timer↔task real** (Start cronometra a task de fato; tempo gruda na task) → **fase 30** (PRD
  §9.2). O Start é mock (CA-26). A `StartInput.TaskID` já existe no `timer.Service`, mas **não** é
  usada por esta fase.
- **Editor markdown rico / bloco-a-bloco** (toolbar, embeds, blocos arrastáveis) → anti-NOA, fora.
  A descrição da task é uma `<textarea>` simples sobre `tasks.notes`.
- **Reordenar sub-tasks por drag** — `position` existe e é editável por `PATCH` (CA-08), mas a UI de
  reordenar sub-task fica simples nesta fase (sem DnD na sidebar; criar adiciona no fim). O **DnD do
  board** (tasks entre colunas) é o único drag desta fase (RF-04).
- **API/OAuth do Notion** — só link + preview client-side (PRD §3.6). Sem fetch ao Notion, sem
  metadados além do slug.
