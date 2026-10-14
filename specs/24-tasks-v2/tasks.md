# tasks.md — Fase 24 · Tasks 2.0

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-18).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migrações → tasks ALTER → subtasks → handlers
> → board), depois frontend (api/queries → tab-shell → board/DnD → popup → list/table → Inter →
> gating), E2E e gate. **Estende as fases 02/12 — não as reescreve** (RNF-05).

---

## Bloco 1 — backend domínio (T-01 → T-07)

- **T-01 — Migração `0021_tasks_v2`** (CA-01/02/02.1/03/04 backstop, RNF-05)
  - `backend/internal/db/migrations/0021_tasks_v2.sql` (plan §2.1): `ALTER TYPE task_status ADD VALUE
    'backlog' BEFORE 'todo'` + `ADD COLUMN priority smallint NOT NULL DEFAULT 0 CHECK (0..3)` +
    `ADD COLUMN notion_url text` + `ADD COLUMN position integer NOT NULL DEFAULT 0` +
    `CREATE INDEX idx_tasks_project_status_position ON tasks (project_id, status, position)`.
    **⚠️ NENHUM statement referencia `'backlog'`** (DEFAULT de status fica `todo`; sem WHERE/CHECK/
    UPDATE no valor novo — PG17/PRD §9.3).
  - Conferir o diretório antes: `0020_reminders` é o último → **`0021`** livre (08 pulada).
  - Teste (`migrations_*_test.go` ou novo): enum `task_status` tem os 4 valores na ordem
    `backlog,todo,doing,done`; `DEFAULT` da coluna `status` = `'todo'` (pina o furo do "backlog vira
    default"); `priority` default 0 + CHECK barra `-1` e `4`; `notion_url` nullable; `position`
    default 0; `idx_tasks_project_status_position` existe. **A migração aplica sem erro de tx**
    (prova que `ADD VALUE` rodou com o valor não-usado no mesmo arquivo).
  - INDEX.md: registrar 0021 dona da fase 24 (orquestrador).

- **T-02 — Migração `0022_task_subtasks`** (CA-06/09/10/14 backstop)
  - `backend/internal/db/migrations/0022_task_subtasks.sql` (plan §2.2): tabela `task_subtasks`
    (`id`/`task_id` FK→tasks ON DELETE CASCADE/`workspace_id` FK→workspaces/`title`/`done`/`is_goal`/
    `position`/`created_at`) + `idx_subtasks_task (task_id, position)` + `idx_subtasks_task_goal
    (task_id) WHERE is_goal`.
  - Teste: tabela/colunas/defaults (`done=false`, `is_goal=false`, `position=0`); **`ON DELETE
    CASCADE` apaga sub-tasks ao deletar a task pai** (não erra FK); índices existem.
  - INDEX.md: registrar 0022 dona da fase 24.

- **T-03 — `tasks.Service`: priority/notion_url/position + status backlog** (CA-01/02/02.1/03/04/05)
  - `internal/tasks/service.go`: `validStatus` ganha `case "backlog"`; `CreateInput` += `Priority
    *int`/`NotionURL *string`; `PatchInput` += `PrioritySet/Priority`, `NotionURLSet/NotionURL`
    (tri-state — D-09), `PositionSet/Position`. Validação ANTES do SQL (D-07): priority `0..3` senão
    `validation_error` (CA-02.1); notion_url trim → vazio vira `null`, `len > 2048` → `validation_error`
    (CA-04); position `≥ 0`. INSERT do Create grava `priority`/`notion_url`/`position` (position =
    `nextPosition(tx, project_id, status)` = `COALESCE(max(position),-1)+1` — D-03). UPDATE do Patch
    grava os presentes. **`recalcObjectiveStatus` e o `ReminderHook`/due hook INTOCADOS** — só
    `status`/`objective_id`/`due_date` os disparam (priority/notion_url/position entram nos campos
    benignos — CA-05/D-04).
  - Teste (`service_test.go`/novo, dbtest, harness existente): cria task com `status:backlog`/
    `priority:2`/`notion_url:"..."` → persiste e devolve; priority `-1`/`4` → 400 (CA-02.1, mutação
    que remove o guard morre); notion_url `> 2048` → 400, vazio → null (CA-04); PATCH `notion_url:null`
    limpa, ausente mantém (tri-state CA-03); **PATCH só `priority` NÃO recalcula objetivo** (objetivo
    manual-done a <100% + patch priority → continua done — pina D-04/CA-05; mutação que recalcula em
    benigno morre); **PATCH `status:backlog→done` recalcula** (auto-conclui o objetivo se 100% —
    CA-05); `nextPosition` cresce por `(project_id, status)` (2 creates no mesmo status → 0,1).
    **A suíte das fases 02/12/18/20 continua verde** (assinaturas estendidas, asserções intactas;
    os call sites de `Create`/`Patch` que mudaram de assinatura — se mudaram — atualizados
    mecanicamente sem alterar asserções).

- **T-04 — `subtasks.Service`: CRUD + gate** (CA-06/06.1/07/08/09/10/11/13)
  - `internal/subtasks/service.go` + `gate.go`: `Subtask`, `CreateInput`/`PatchInput`, `List`/`Create`/
    `Patch`/`Delete`. `loadTaskGate(ctx, ws, userID, role, taskID, blockArchived)` (D-08): `uuid.
    Validate(taskID)`→404; carrega task JOIN projects (+LEFT JOIN project_members) casando
    `t.workspace_id=$ws AND t.id=$taskID`; visibilidade `(p.visibility='shared' OR p.owner_user_id=$u)`
    → senão 404; `can_write = dono ∥ role=="owner" ∥ access=="edit"`; escrita em projeto arquivado →
    403. Validação na mão: title 1–500 (CA-06.1). Patch/Delete carregam a sub-task com `WHERE
    workspace_id=$ws AND task_id=$taskIdDaRota AND id=$subtaskId` + `uuid.Validate(subtaskId)` (CA-13).
    `position` no create = max+1 da task. **NÃO** importa `tasks`/`projects`/`objectives` (SQL direto —
    D-05).
  - Teste `service_test.go` (dbtest): create/list (ordem por position); title vazio/`>500` → 400
    (CA-06.1); patch done/is_goal/title/position; delete → 204 + linha some; **escrita por `access=
    view`/viewer → 403** (CA-11, mutação que afrouxa o gate morre); **task invisível (private de
    outro) → 404** (CA-13); `taskId`/`subtaskId` malformado → 404 (não 500); **sub-task da task B
    via rota da task A → 404** (mutação que remove `task_id=$rota` do WHERE morre — CA-13); **viewer-
    assignee da task NÃO cria sub-task → 403** (D-10/CA-11); **isolamento por workspace** (mutação
    removendo `WHERE workspace_id` morre — CA-13/RNF). Cascade já provado na T-02.

- **T-05 — Board: sub-task agregada + campos novos no BoardTask + client_name no BoardProject** (CA-14/14.1, RNF-04)
  - `internal/objectives/board.go` (estende): `BoardTask` ganha `priority`/`notion_url`/`position`
    (do SELECT de tasks — já existem como colunas) + `subtask_count`/`subtask_done`/`goal_count`/
    `goal_done` de uma **4ª query agregada** `SELECT task_id, count(*), count(*) FILTER (WHERE done),
    count(*) FILTER (WHERE is_goal), count(*) FILTER (WHERE is_goal AND done) FROM task_subtasks WHERE
    task_id = ANY($tasks) GROUP BY task_id` (zero N+1 — RNF-04). Tasks de cada coluna/bucket ordenadas
    por `position` asc (desempate `created_at`). **`BoardProject` ganha `ClientName *string`** (CA-14.1):
    o SELECT de metadados do projeto (`board.go:87` `SELECT p.id, p.name, p.color, … FROM projects p
    WHERE p.id = $3`) ganha `LEFT JOIN clients c ON c.id = p.client_id` e a coluna `c.name`; NULL
    quando o projeto não tem cliente. Aditivo (nenhum campo existente muda). `json:"client_name"`.
  - Teste: board devolve `priority`/`notion_url`/`position` por task; `subtask_count`/`done`/`goal_*`
    corretos (task sem sub-task → 0; task com 3 sub-tasks 1 feita 2 metas 1 meta-feita → 3/1/2/1);
    **tasks ordenadas por position** (2 tasks no mesmo status com position 1,0 → vem 0 antes — mutação
    que tira o ORDER BY position morre); **`board.project.client_name` = nome do cliente do projeto;
    `null` se projeto sem cliente** (mutação que omite o LEFT JOIN/coluna morre — CA-14.1); a contagem
    é **uma** query (sem N+1 — pode reusar o `TestBoardSob200ms`/seed da fase 12 estendido com
    sub-tasks). Suíte da fase 12 do board intacta (campos aditivos).

- **T-06 — Handlers de sub-tasks** (CA-06..13)
  - `internal/api/subtasks_handlers.go`: handlers finos (forma de handler espelha comments/reminders,
    mas o MOUNT é Core — decisão F/T-07) pra `GET/POST/PATCH/DELETE .../tasks/{taskId}/subtasks[/
    {subtaskId}]`. `priority`/`notion_url`/`position` adicionados aos handlers de task
    (`handleCreateTask`/`handlePatchTask` — notion_url tri-state via `json.RawMessage`, padrão fase 02).
  - Teste `subtasks_handlers_test.go` (harness `coretest`, que monta o Core sob RequireWriteRole): GET
    200 ordenado; POST 201 + persiste; PATCH/DELETE; 400 validation (title); **403** (viewer do ws
    barrado pelo RequireWriteRole; `access=view` no projeto barrado pelo gate fino — CA-11); **400
    invalid_reference** em projeto arquivado (decisão G); 404 task invisível; **404 sub-task de task B
    via rota de A** (CA-13); 401 sem sessão (table-driven nas rotas novas — CA-12); task POST/PATCH com
    priority/notion_url retorna os campos (CA-02/03).

- **T-07 — Wiring no `core_routes.go`** (CA-11/12/13) — **decisão de coordenação F: mount Core, NÃO sibling**
  - `mountCore` (`core_routes.go`): adicionar a sub-rota `/tasks/{taskId}/subtasks` (+ `/{subtaskId}`)
    **DENTRO do bloco `wr.Route("/tasks", …)` que já existe** — `r.Route("/{taskId}/subtasks", func(s
    chi.Router){ s.Get("/", …); s.Post("/", …); s.Patch("/{subtaskId}", …); s.Delete("/{subtaskId}",
    …) })`. Herda a cadeia do subrouter `/workspaces/{workspaceId}`: **`RequireMember` +
    `RequireWriteRole`** (blanket — `auth_routes.go:86`). Injetar `subtasks.Service` **no `Core`**
    (campo novo `Subtasks *subtasks.Service` em `Core` no `core_routes.go`); `cmd/server/main.go` liga
    `Subtasks: subtasks.NewService(pool)` dentro de `api.WithCore(api.Core{…})` (junto de Clients/
    Projects/Tasks/Objectives). **NÃO** criar Option/sibling separado em `auth_routes.go`, **NÃO**
    usar `WithSubtasks`. Razão (plan §4.1): sub-task não tem carve-out viewer-assignee (D-10/CA-11),
    então o `RequireWriteRole` do Core é coerente — o gate fino (`loadTaskGate`) cobre só os casos que
    o blanket não cobre (access=view → 403; arquivado → 400; sem visibilidade → 404). Confirmar que o
    chi NÃO colide com as rotas de comments/reminders já irmãs nem com `/tasks/{taskId}` da fase 02
    (smoke de routing primeiro).
  - Teste: rota viva (smoke de roteamento); POST de sub-task por **viewer do ws → 403** (barrado pelo
    `RequireWriteRole` do Core); POST por membro **`access=view` no projeto → 403** (barrado pelo gate
    fino do service — CA-11); POST em **projeto arquivado → 400 invalid_reference** (gate fino —
    decisão G); sem sessão → **401** (CA-12); task invisível/cross-task/uuid malformado → **404**
    (CA-13). **REMOVIDO desta T toda menção a "Route IRMÃO" / `WithSubtasks` / sibling em
    `auth_routes.go`** (decisão F).

> **Fim do bloco 1:** `make test-back` 100% verde (subtasks + tasks + objectives/board + fases
> 02/12/18/20 intactas); Vitest/Playwright não rodados (bloco sem frontend). Handoff "retomar na T-08".

## Bloco 2 — frontend (T-08 → T-15)

- **T-08 — `lib/api.ts` + `lib/queries/subtasks.ts` + `lib/notion.ts` + `useView` genérico** (CA-03/04/14/14.1/16/16.1/24)
  - `Task` ganha `priority`/`notion_url`/`position`; `BoardTask` ganha `subtask_count`/`subtask_done`/
    `goal_count`/`goal_done`; **`BoardProject` ganha `client_name: string | null`** (CA-14.1);
    `TaskInput` ganha os campos novos (notion_url tri-state). `Subtask` type
    + `listSubtasks`/`createSubtask`/`patchSubtask`/`deleteSubtask`. Hooks `useSubtasks(taskId)` +
    mutações (invalidam `subtasksKey(taskId)` E `boardKey`). `lib/notion.ts::notionTitle(url)` puro
    (último segmento, remove hash hex **case-insensitive `-[0-9a-fA-F]{6,}$`**, `-`→espaço — CA-24,
    espelha o mockup `projeto-workspace.html` linha 261). **Reescrever `useView` em
    `components/ui/ViewToggle.tsx`** pra genérico `useView<T extends string>(key, options: readonly
    T[], fallback: T): [T, (v:T)=>void]` com **mapeamento do legado** ao ler `tt.view.board`
    (`'cards'→'board'`, `'table'→'list'`; desconhecido → fallback) — CA-16/16.1. Os consumidores
    `Clients.tsx`/`Projects.tsx` passam a chamar `useView('tt.view.clients', ['cards','table'],
    'cards')` (e equivalente projects) — sem mudança de comportamento.
  - Teste `subtasks-api.test.ts` + `notion.test.ts` + `useView.test.tsx`: chamadas batem nos paths
    certos; hooks invalidam as queries certas; `notionTitle('.../OKRs-Q3-2025-8f3a1c9b')` → `"OKRs Q3
    2025"`; **`notionTitle('https://www.notion.so/Projeto-8F3A1C9B')` → `"Projeto"`** (hex MAIÚSCULO —
    pina o regex case-insensitive, mutação pro lowercase-only morre — CA-24); URL inválida → fallback
    `"Página do Notion"`; sem hash → slug inteiro. **`useView`:** `tt.view.board='cards'` lê `'board'`;
    `='table'` lê `'list'`; valores novos `'board'/'list'/'table'` round-trip; desconhecido → `'board'`
    (CA-16/16.1); a versão `('cards','table')` segue cards/table (Clients/Projects intactos).

- **T-09 — Inter: troca de fonte + tokens do mark Notion** (CA-27/14/RNF-03)
  - `frontend/public/fonts/Inter-*.woff2` (regular/medium/semibold) + `@font-face` no `tokens.css`/
    `index.css`; `--font-ui` (no `@theme`) → `'Inter', system-ui, sans-serif`; `--font-mono`
    **intocado**. Self-hosted (sem CDN — RNF-01); precache do SW pega os woff2 (offline.spec/check-pwa
    verdes). **Criar os dois tokens novos do mark "N" do Notion** em `tokens.css` — `--notion-mark-bg`
    e `--notion-mark-ink` — no bloco **dark E no light** (o mockup usa `#fff`/`#111`; valores
    equivalentes viram token). É o que mantém o `check-cores` verde sem editar o gate (a allowlist do
    `check-cores.sh` é regex ANCORADA por arquivo, não substring — não dá pra "adicionar
    NotionPreview.tsx"; a rota é token). O `NotionPreview` (T-13) usa `var(--notion-mark-bg)`/
    `var(--notion-mark-ink)`.
  - Teste: `tokens.css` declara `--font-ui` com `Inter` (asserção de string no arquivo, padrão da
    integridade de token das fases anteriores); **`tokens.css` declara `--notion-mark-bg` e
    `--notion-mark-ink` (asserção de string)**; `check-cores` verde (fonte/mark não tocam hex literal);
    `check-pwa`/`offline.spec` verdes. Tema dark/light intacto (snapshot/spec de tema existente passa).

- **T-10 — Tab-shell: `ProjectDetail` + `ProjectTabs` + `ProjectOverview`** (CA-15/14.1)
  - `ProjectTabs.tsx` (barra Overview/Tasks/Notes/Goals/Calendar — fonte de integração D-13);
    `ProjectDetail.tsx` (estende) vira o shell: header (**cor + nome + chip de cliente derivado de
    `board.project.client_name`** — CA-14.1; o chip só aparece quando `client_name != null`; NÃO usar
    2º fetch `useProjects`/`useClients`) + tabs + corpo por aba. **Tasks** renderiza `TasksTab`
    (T-11+); **Overview** renderiza `ProjectOverview` (esqueleto: KPIs total time/this week 0h/open/
    done% + andamento por status [reusa `ProgressBar` da 12] + goals do `/board`); **Notes/Goals/
    Calendar** = placeholder ("Em breve"). Aba ativa = state local (não persiste — CA-15).
  - Specs (jsdom, `/board` mockado): renderiza as 5 abas; clicar troca a aba; Overview mostra KPIs +
    status do board; Notes/Goals/Calendar mostram placeholder.

- **T-11 — Board com DnD nativo + quick-add + reconciliação dos specs da fase 12** (CA-17/18/25/28.1)
  - `TaskBoardColumns.tsx` (reescreve a da 12 sem regredir o contrato de uso) + `TaskBoardCard.tsx`:
    **4 colunas** backlog/todo/doing/done (rótulos do mockup linha 197), cada uma ordenada por
    `position`; card `draggable` + `onDragStart` (guarda dragId) + coluna `onDragOver`(preventDefault)/
    `onDrop` → **`dropStatus(dragId, targetStatus)` puro** → `PATCH {status, position:max+1}` + invalida
    `boardKey` (D-04/D-12). Card mostra nome + prioIcon + meta (due/todos `subtask_done/count`/⭐ `goal_*`/
    time `tracked_seconds`/ícone Notion se `notion_url`). Quick-add "Add" por coluna cria no status da
    coluna e abre o popup focado (CA-18). Gating: `can_write=false` → `draggable=false`, sem quick-add.
  - **Reconciliação OBRIGATÓRIA dos specs jsdom da fase 12/19 (desvio sancionado — CA-28.1):**
    reescrever `components/board/TaskBoardColumns.spec.tsx` (rótulos `A fazer/Fazendo/Feito`, botões
    `/mover.*direita/`, `grid-cols-3` aposentados → 4 colunas + `dropStatus` DnD) e `ProjectBoard.spec.tsx`;
    `ProjectBoard.autoclose.spec.tsx` vira spec do board novo (última task → `done` via drop/popup →
    objetivo concluído na Overview/cartão). **PRESERVAR `ProjectBoard.canComment.spec.tsx`**:
    `canCommentOn(canWrite, task, currentUserId)` (`ProjectBoard.tsx:25`) é **load-bearing pro carve-out
    viewer-assignee da fase 19** — manter a função pura e seu spec (re-hospedados no `TaskPanel`/`TasksTab`
    com o mesmo teste). O `onMoveTask`/`‹ ›` são aposentados em favor do `dropStatus` DnD.
  - Specs (jsdom): board renderiza 4 colunas com tasks nos status certos, ordenadas por position;
    `dropStatus` puro devolve a mutação certa (CA-17); quick-add chama create com o status da coluna
    (CA-18); `can_write=false` esconde quick-add e desabilita drag (CA-25, padrão `viewer-gating`).

- **T-12 — Popup central: `CenterPanel` + `TaskPanel` + `TaskPanelProps` + descrição + thread** (CA-20/21/22/26/28.1)
  - **`CenterPanel.tsx`** (NOVO — decisão de coordenação A): componente React reusável dono do scrim +
    card central + toggle expand/collapse (side↔full), usando tokens.css. **NÃO** referenciar classes
    cruas do mockup (`.mscrim`/`.scard`/`.swrap`) como contrato — são estilo interno. As fases **26
    (nota) e 27 (Spotlight) REUSAM este componente** — declarar isso (espelha D-13 do tab-shell).
  - `TaskPanel.tsx`: **usa `CenterPanel`** (toggle side↔full — CA-22, ícones expand/collapse),
    título grande (input; **Enter → foca o textarea de descrição** com cursor no fim — CA-21),
    `TaskPanelProps` (Status chip 1-clique cicla; Priority chip 1-clique cicla; Due date-picker; Time =
    `fmtH(tracked_seconds)` + **botão Start MOCK/desabilitado com `data-testid="task-start"`** — elemento
    estável; a fase 30 troca só a ação — decisão B; CA-26; Notion input↔preview), textarea de
    descrição **sempre editável** (PATCH notes no blur/debounce — CA-20). **A thread de comentários da
    fase 19 (`TaskThread`) aparece no popup, gateada por `canCommentOn(canWrite, task, currentUserId)`**
    (carve-out viewer-assignee da fase 19 NÃO pode ser perdido na migração do board — CA-28.1). Editar
    reflete sem reload (PATCH + invalida boardKey). Fechar via ✕ / scrim (modo side).
  - Specs (jsdom, board+task mockados): abrir task → popup com título/props/descrição/sidebar; Enter no
    título move foco pro textarea (CA-21); toggle expand alterna side↔full sem fechar (CA-22); **Start
    tem `data-testid="task-start"`, está disabled e não chama API/timer** (CA-26/B); editar status pelo
    chip dispara PATCH status (CA-20); **a thread aparece no popup e respeita `canCommentOn`** (espelha
    `ProjectBoard.canComment.spec` da fase 19 — CA-28.1); `can_write=false` → chips não-clicáveis, date
    read-only, Start disabled (CA-25).

- **T-13 — Sidebar de sub-tasks + preview do Notion** (CA-23/24/25)
  - `SubtaskSidebar.tsx`: `useSubtasks(taskId)`; input "add item" (Enter → createSubtask + limpa +
    mantém foco — CA-23); checkbox done (patchSubtask done); ⭐ is_goal (patchSubtask is_goal);
    cabeçalho `feitas/total` + (se metas) `metas-feitas/total ⭐`; vazio → "No sub-tasks yet."
    `NotionPreview.tsx` + integração no TaskPanelProps: com `notion_url` → card (**mark "N" usando
    `var(--notion-mark-bg)`/`var(--notion-mark-ink)` — tokens da T-09, ZERO hex literal, RNF-03** +
    `notionTitle` [regex case-insensitive `-[0-9a-fA-F]{6,}$`] + notion.so + abrir nova aba +
    desvincular → PATCH notion_url:null); sem → input "Cole o link…" → PATCH notion_url; ícone Notion
    no card/linha/célula quando há link (CA-24). Gating: `can_write=false` → sidebar/notion read-only,
    sem add/toggles (CA-25).
  - Specs (jsdom): Enter no add cria sub-task e limpa o input (CA-23); checkbox/⭐ disparam PATCH;
    cabeçalho conta feitas/metas; preview mostra `notionTitle`, abrir tem `target=_blank`, desvincular
    faz PATCH null (CA-24); `can_write=false` esconde add/toggles e o input de Notion (CA-25).
    `check-cores` segue verde (o mark "N" usa tokens, não hex literal — RNF-03).

- **T-14 — List + Table** (CA-16/16.1/18/19/25)
  - `TasksTab.tsx`: toolbar (segmented Board/List/Table persist `tt.view.board` via o `useView`
    genérico da T-08 — CA-16 + "New task" → create todo) + render da visão. `TaskListView.tsx` (grupos
    por status + quick-add por grupo; status 1-clique cicla — CA-16/18). `TaskTableView.tsx` (colunas
    status/priority/due/todos `subtask_done/count`/time; status/priority 1-clique ciclam; clicar linha
    abre popup; quick-add — CA-19). Gating `can_write=false` em ambas (CA-25).
  - Specs (jsdom): troca de visão persiste em `tt.view.board` e recarrega na visão salva (CA-16);
    **mapeamento do legado pinado no nível do `TasksTab`** — `localStorage['tt.view.board']='cards'`
    renderiza o **Board**; `='table'` renderiza a **List** (não a Table nativa); `='board'/'list'/
    'table'` renderizam Board/List/Table; valor desconhecido → Board (CA-16.1 — a mutação que ignora o
    mapeamento e cai direto na Table morre); List agrupa por status + quick-add por grupo (CA-18);
    Table mostra as colunas nativas + cicla status/prio (CA-19); `can_write=false` esconde quick-add e
    congela os ciclos (CA-25).

- **T-15 — Conclusão automática do objetivo na UI + ícone Notion nas visões** (CA-17/CA-05/28.1 ponta a ponta)
  - Garantir que mover a última task pra `done` no board (DnD ou popup) reflete o objetivo concluído
    após invalidação do board (CA-05/fase 12 chega no refetch — **agora verificado na aba Overview**,
    não mais no card do board simples da 12); ícone Notion presente em card/linha/célula quando
    `notion_url` (CA-24). **Fechar a reconciliação dos specs jsdom da fase 12/19 iniciada na T-11**
    (desvio sancionado — CA-28.1): garantir que `TaskBoardColumns.spec.tsx`/`ProjectBoard.spec.tsx`/
    `ProjectBoard.autoclose.spec.tsx` rodam VERDES no board novo e que `ProjectBoard.canComment.spec.tsx`
    (preservado) segue verde com `canCommentOn` re-hospedado no `TaskPanel`/`TasksTab`.
  - Specs (jsdom, board mockado): última task done → cartão/Overview de objetivo mostra concluído;
    volta pra doing → reabre; card com notion_url mostra ícone.

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright não
> rodado ainda (E2E é o bloco 3). Handoff "retomar na T-16".

## Bloco 3 — E2E + verificação + gate (T-16 → T-18)

- **T-16 — E2E `tasks-v2.spec.ts` + reconciliação dos E2E da fase 12/18/19** (CA-28/28.1)
  - Logado+escrita (stub fase 01) num projeto `shared`, em `/projetos/{id}` aba Tasks: (a) Board com 4
    colunas; (b) quick-add numa coluna (`backlog`) → task aparece nela; (c) abrir a task (popup) →
    editar título; (d) adicionar sub-task pela sidebar + marcar ⭐ + done; (e) colar link do Notion →
    preview; (f) expandir popup pra tela cheia e voltar; (g) fechar → Table → task com priority/due/
    time; (h) **reload persiste**. **SEM** segundo perfil viewer no E2E (stub = 1 user — CA-28); o DnD
    entre colunas é teste de componente jsdom (`dropStatus` puro — Playwright drag flaky); CA-25 =
    componente jsdom (`can_write=false`).
  - **Reconciliação OBRIGATÓRIA dos E2E Playwright (desvio sancionado — CA-28.1; o gate roda o
    Playwright completo):**
    - `frontend/e2e/tarefas.spec.ts` (fase 12): `getByRole('button',{name:/tabela/i})` do ViewToggle
      binário → segmented **Board/List/Table**; `getByLabel('Status de …').selectOption('done')` do
      `TaskRow` → **chip de status 1-clique** (ou drop no Board); `0/1 task` → contagem na coluna/popup;
      dialog `Nova task`/`Criar task` (TaskModal) → **popup central `TaskPanel`** com título focado; a
      **conclusão automática do objetivo** verificada na aba **Overview** (não no card do board). Asserção
      de **dado** preservada (objetivo conclui em 100%).
    - `frontend/e2e/comentarios.spec.ts` (fase 19): abrir a task pelo **`TaskPanel`** (não mais o
      dialog `Editar task`); comentar dentro do popup; criação via quick-add.
    - `frontend/e2e/cross-assign.spec.ts` (fase 18): idem — Responsável/Solicitante migram pro fluxo do
      popup/criação novo.
    - `frontend/e2e/mobile.spec.ts` (M2): confirmar que o board reshaped segue **verde a 375px** (9
      rotas, zero overflow horizontal — não regredir).
    - Marcar como **desvio sancionado** (a fase 24 é dona do board e substitui a UI da 12); garantir
      red→green dos E2E migrados ANTES do `make check-full` (T-18).
  - Reusar `loginViaStub`; nomes únicos (banco reusado, idempotente). Liberar a :8080 antes (regra do
    KICKOFF — `reuseExistingServer`).

- **T-17 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: migração `ALTER TYPE` na tx / `backlog` não-usado, isolamento
    multi-tenant/visibilidade de sub-task [CA-13], **gate de sub-task no mount Core sob RequireWriteRole
    — viewer-ws 403, access=view 403, arquivado 400 invalid_reference, sem visibilidade 404** [CA-11/13/
    D-10/decisão F/G], recálculo de objetivo NÃO disparar em benigno [CA-05/D-04], board sub-task sem
    N+1 + ordenação por position + **`client_name` via LEFT JOIN** [CA-14/14.1], DnD `dropStatus` puro /
    quick-add status correto, notion_url tri-state + **`notionTitle` com hex MAIÚSCULO** [CA-24],
    **`useView` genérico + mapeamento legado `cards→board`/`table→list`** [CA-16/16.1], **mark "N" do
    Notion via tokens, check-cores verde sem editar o gate** [RNF-03], **Start mock com
    `data-testid="task-start"`** [CA-26/B], **`canCommentOn`/thread preservados no `TaskPanel`** [CA-28.1],
    Inter sem regredir tema/check-cores/check-pwa, regressão fases 02/12/18/19/20 — incl. E2E
    reconciliados) + refutação dupla. Corrigir confirmados red→green. Registrar.

- **T-18 — Gate + (sem deploy por fase) + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose + check-cores +
    check-pwa). **NÃO deployar** (deploy da M3 é único, no fim — alinhar com o orquestrador; cada fase
    fecha com gate verde + handoff, sem deploy individual).
  - Handoff (CLAUDE.md "retomar..."), INDEX (24 construída, 0021/0022 donas, tab-shell pronto pras
    fases 26/28/29), KICKOFF da próxima fase (26 — Notas), commit, briefing pro Diogo.

---

## Blocos (regra ≤ contexto / handoff entre blocos)

- **Bloco 1 — backend domínio:** T-01 → T-07 (migrações, tasks ALTER, subtasks.Service+gate, board
  estendido, handlers, wiring). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-08 → T-15 (api/queries/notion, Inter, tab-shell, board/DnD/quick-add,
  popup+sidebar+notion, list/table, auto-conclusão). Suíte: Vitest + `make test-back` (regressão).
- **Bloco 3 — E2E + fechamento:** T-16 → T-18 (E2E, verificação adversarial, gate `make check-full`
  + handoff; **sem deploy individual**).

`make check-full` é gate **só no bloco 3 (T-18)**; os blocos 1 e 2 fecham com a suíte do que existe
verde + handoff + commit (regra de blocos do `CLAUDE.md`; sob "uma paulada", os blocos viram
checkpoints sequenciais no mesmo fluxo).

---

## Furos da verificação adversarial (ETAPA 2 — aplicados)

Todos os furos confirmados 2/2 da verificação adversarial da spec (`/tmp/m3verify/24-tasks-v2.json`)
foram aplicados em spec.md / plan.md / tasks.md antes do build:

- **24-H1 (MEDIUM, contrato/cross-fase) — `useView` só suporta 2 valores; reusar `tt.view.board` com
  3 valores quebra a persistência do legado.** O helper real (`components/ui/ViewToggle.tsx`) é
  `'cards'|'table'` com fallback `cards`. Resolvido: **CA-16 reescrito** (helper genérico
  `useView<T>(key, options, fallback)`, 3 valores `'board'|'list'|'table'`, default `'board'`) +
  **CA-16.1 NOVO** (mapeamento do legado `'cards'→'board'`, `'table'→'list'`; desconhecido → `'board'`;
  teste obrigatório). plan **D-11 atualizado**; **T-08** reescreve `useView` + teste `useView.test.tsx`;
  **T-14** pina o mapeamento no nível do `TasksTab`. Consumidores `Clients`/`Projects` passam a chamar
  a versão genérica sem mudança de comportamento.

- **24-H2 (MEDIUM, contrato) — header pede "cliente" mas o `/board` (`BoardProject`) não tem
  client_id/client_name.** Resolvido com a opção (a) ADITIVA (registrada): **CA-14.1 NOVO** —
  `BoardProject` ganha `client_name` via **1 LEFT JOIN `clients`** no SELECT de metadados de
  `objectives/board.go`; `null` se sem cliente. **CA-15** referencia `board.project.client_name` como
  fonte única (chip só com `!= null`). plan **§1/§4.3/§6** + **T-05** (board + teste) + **T-08**
  (api.ts `client_name`) + **T-10** (header usa o campo, sem 2º fetch).

- **24-H3 (HIGH, cross-fase/regressão) — T-07 dizia "Route IRMÃO sob RequireMember" E "injetar no Core/
  mountCore", locais incompatíveis.** Resolvido via **decisão de coordenação F (mount Core)**: sub-tasks
  são montadas DENTRO do bloco `/tasks` do `mountCore` (sob `RequireMember`+`RequireWriteRole`), com
  `subtasks.Service` injetado **no `Core`** (campo novo) — NÃO sibling/`WithSubtasks` em `auth_routes.go`.
  Coerente porque sub-task NÃO tem carve-out viewer-assignee (gate = can_write puro), então o blanket
  `RequireWriteRole` não atrapalha; o `loadTaskGate` cobre os casos finos. spec **§sub-tasks/CA-11**,
  plan **§4.1 reescrito** + §1, **T-07 reescrito** (toda menção a "Route irmão"/`WithSubtasks`
  removida), **T-06** alinhado.

- **24-H4 (HIGH, regressão/RNF) — E2E `tarefas.spec.ts` (e comentarios/cross-assign/mobile) dependem
  da UI antiga (ViewToggle binário, TaskRow-select, TaskModal) que a 24 substitui; sem reconciliar, o
  gate quebra.** Resolvido: **CA-28.1 NOVO** (lista nominal dos E2E + o que muda, asserções de dado
  preservadas, desvio sancionado) e **T-16 reescrito** com a reconciliação OBRIGATÓRIA de
  `tarefas.spec.ts`/`comentarios.spec.ts`/`cross-assign.spec.ts` + confirmação do `mobile.spec.ts` a
  375px, com red→green antes do gate.

- **24-H5 (MEDIUM, tokens/RNF check-cores) — o mark "N" do Notion usa `#fff`/`#111` inline; a allowlist
  do `check-cores.sh` é regex ANCORADA por arquivo exato (não substring), então buildar com hex inline
  FALHA o gate e "adicionar à allowlist" exigiria editar o script.** Resolvido: **RNF-03 corrigido**
  (descrição certa do gate) + **rota dos tokens FIXADA como obrigatória** (não "OU"): `--notion-mark-bg`
  e `--notion-mark-ink` criados em `tokens.css` (dark+light), `NotionPreview` usa `var(...)`. plan **§7
  reescrito** (sem opção de editar o gate), **T-09** cria os tokens + asserta string, **T-13** usa os
  tokens.

- **24-H6 (LOW, contrato) — CA-24 usava regex lowercase-only `-[0-9a-f]{6,}$`, mas o mockup (fonte da
  verdade) usa case-insensitive `-[0-9a-fA-F]{6,}$`; hex maiúsculo vazaria o hash no título.** Resolvido:
  **CA-24** + plan §4.2/§7 + **T-08** trocados pra `-[0-9a-fA-F]{6,}$`, com **teste obrigatório de hex
  MAIÚSCULO** (`notionTitle('https://www.notion.so/Projeto-8F3A1C9B')` → `"Projeto"`).

- **24-H11 (MEDIUM, regressão/RNF) — reescrever `TaskBoardColumns.tsx` quebra os specs jsdom da fase
  12/19; `canCommentOn` (fase 19) é load-bearing e não pode ser perdido.** Resolvido: **CA-28.1**
  lista os specs a reescrever (`TaskBoardColumns.spec.tsx`, `ProjectBoard.spec.tsx`,
  `ProjectBoard.autoclose.spec.tsx`) e PRESERVA `ProjectBoard.canComment.spec.tsx`; **T-11** faz a
  reconciliação + preservação de `canCommentOn`; **T-12** garante que o `TaskPanel` carrega o
  `TaskThread`/comentários da fase 19 gateado por `canCommentOn` (sub-asserção pinada); **T-15** fecha a
  reconciliação verde.

### Decisões de coordenação cross-fase aplicadas

- **A (popup central = componente React):** introduzido `CenterPanel.tsx` (NOVO) como o popup central
  reusável (scrim + card + toggle side/full, tokens.css); `TaskPanel` USA `CenterPanel`; declarado que
  **26/27 REUSAM** esse componente. Removida a dependência de classes cruas `.mscrim`/`.scard`/`.swrap`
  do mockup como contrato (plan §1/§6, **T-12**).
- **B (botão Start mock identificável):** a fase 24 entrega o botão Start como mock/desabilitado mas com
  **`data-testid="task-start"`** (elemento estável); a fase 30 troca SÓ a ação (`onClick →
  timerStore.start`) do mesmo botão — não há "slot/componente provido pela 24" (CA-26, plan §1/§6/§7,
  **T-12**).
- **F (sub-tasks = mount Core):** ver 24-H3 acima.
- **G (erros canônicos reais):** escrita em projeto arquivado → **400 invalid_reference** (não código
  'archived'); ref de outro ws/inexistente/uuid malformado → **404 not_found**. Aplicado no resumo de
  códigos do `loadTaskGate` (plan §5) e nos testes de **T-06/T-07** (ancorado em `objectives/gate.go:67`
  e `tasks/service.go:239/251`, que já fazem `blockArchived → invalid_reference`).
- **H (caminhos reais):** `ViewToggle`/`useView` em `components/ui/ViewToggle.tsx`; `ProjectCard` em
  `components/projects/`; stores zustand em `lib/`. Referências do plan/tasks alinhadas aos caminhos reais.

---

## T-17 — Verificação adversarial do BUILD (ETAPA 3 — aplicada, sessão 23/06)

Workflow ultracode, **11 dimensões × finder ancorado no diff real (`c73ee58..HEAD`) → 2 refutadores
céticos por achado** (25 agentes). **7 achados brutos → 3 confirmados 2/2 (todos MEDIUM, gaps de
cobertura que mascaravam mutação — código correto) + 2 split (1/2) + 2 refutados (2/2).** Todos os 3
confirmados corrigidos **red→green com mutação PROVADA e restaurada** (sem resíduo na working tree):

1. **(gate-mount-core, CA-11)** o ramo `access=view → 403` e `arquivado → 400` só era testado em
   **Create**; uma mutação trocando `requireWrite/blockArchived` em `Patch`/`Delete` sobrevivia (o
   middleware `RequireWriteRole` do mount Core só barra ws-viewer; contra um MEMBRO com `access=view` o
   `loadTaskGate` fino é a ÚNICA autoridade). **Fix:** `TestEscritaPatchDeleteSemPermissao403` +
   `TestProjetoArquivadoPatchDelete400` (`subtasks/service_test.go`). Mutação `122/160: true,true →
   false,false` → ambos FAIL → restaurado.
2. **(board-agregado, CA-10/14)** a fixture do `board_v2_test` tinha `subtask_done == goal_done == 1`,
   então uma mutação conflando `count(*) FILTER (WHERE done)` com `FILTER (WHERE is_goal AND done)`
   sobrevivia (em prod: task com sub-task GERAL feita mostraria `0/N` na Table). **Fix:** 4ª sub-task
   `done+geral` → `subtask_done=2 ≠ goal_done=1`. Mutação `board.go:172` → FAIL → restaurado.
3. **(centerpanel-thread, CA-28.1)** o wiring `TaskPanel → TaskThread` (gate por `canCommentOn`, NÃO
   `canWrite` — o carve-out viewer-assignee da fase 19 que a migração TaskModal→TaskPanel poderia ter
   perdido) não tinha assert; mutação `canComment = canWrite` sobrevivia. **Fix:** mock do `TaskThread`
   expõe `data-can-comment` + 3 casos em `TaskPanel.spec.tsx` (o do meio — viewer `canWrite=false` que É
   assignee → `true` — mata a mutação). Mutação `TaskPanel.tsx:115` → o caso do meio FAIL → restaurado.

**Split (1/2) → reaberto (HITL 23/06) e CORRIGIDO — a visão Table NÃO persistia no reload (`'table'`
colidia com o valor legado da fase 12; o legacy-map tinha precedência → revertia pra List):** veredito
inicial = NÃO-defeito-de-build (o código era FIEL à CA-16.1/D-11/24-H1, decisão FECHADA), MAS a
verificação expôs uma **contradição interna da própria spec** (a CA-16.1 também afirmava "a Table
nativa vive no valor 'table'" + "board/list/table fazem round-trip") com dor real: a Table — uma das 3
visões que o RF-03 exige persistir — sumia no reload, ferindo o "object permanence" da M3. **O Diogo
autorizou reabrir a 24-H1 (AskUserQuestion) e aplicar o fix.** **Fix (token de persistência distinto):**
o `useView` ganhou um param `persist` (encode); a Table nativa grava `'table-v2'` (decodificado ANTES
do legado), enquanto o `'table'` cru LEGADO da fase 12 segue → List. Resolve a contradição sem quebrar
o legado: RF-03 satisfeito E CA-16.1 (token legado → List) preservada. **CA-16.1 da spec reconciliada**
(histórico 24-H1 registrado inline). Testes: `useView.test` (round-trip da Table nativa via remount +
decode-precedência + legado cru ainda → List), `TasksTab.spec` (clicar Table → `'table-v2'` → remount
reabre Table não List), E2E `tasks-v2` (reload reabre a Table SEM re-clicar). **2 mutações provadas
red→green** (dropar `VIEW_PERSIST` do TasksTab → Table regride; dropar o decode no `resolveView` →
round-trip falha; ambas restauradas), e os testes de legado seguem verdes (fix aditivo).

**Refutados (2/2):** (a) LOW — comentário do `board.go` dizia "ORDER BY status, position" mas o SQL
ordena só por `position` → **corrigido** (comentário honesto: o front agrupa por status preservando a
ordem). (b) LOW — `TaskModal.tsx`/`TaskRow.tsx` viraram código morto mas foram mantidos com specs vivos
(enquanto `ProjectBoard.tsx` foi removido) → mantido (remover é risco, specs seguem verdes; limpeza =
candidata a fatia futura).

**Achado de gate (raiz do flake do E2E que travou o WIP T-17):** as falhas eram sempre
`ERR_CONNECTION_REFUSED` no preview (`:4173`), nunca asserção, e "andavam" entre specs. Causa real: o
`vite preview` liga só em **IPv6 `::1`**; os testes navegam pra `localhost` (resolve dual-stack no
macOS) → quando o Chromium tenta o ramo IPv4 (`127.0.0.1`) recusa. **NÃO era contenção** (a hipótese do
`workers:3→1`). **Fix:** alinhar tudo que o browser acessa a `127.0.0.1` (single-stack determinístico) —
`baseURL`, `url`+`--host 127.0.0.1` do preview, `OAUTH_REDIRECT_URL` (`playwright.config.ts`). Gate E2E
determinístico: 40/40 em 5 runs (3× árvore original + 2× árvore final). `workers:1` mantido (defensável
pro backend singleton; comentário corrige a causa-raiz). Também: o `cronometro.spec` (CA-21/FocoPicker)
ganhou seed com `started_at = agora-2s` (era `-60s`) → garante rank #1 no FocoPicker mesmo em db-test
acumulado (mata o flake pré-existente do FocoPicker registrado no CLAUDE.md).

`make check-full` **GATE VERDE** (Go all green · Vitest 512/512 · Playwright 40/40 determinístico ·
builds · check-segredos/cores/fonts/pwa · smoke-compose 200).
