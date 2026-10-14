# tasks.md — Fase 26 · Notas nativas (.md no Postgres) + slash "/"

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-13).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → gate+CRUD → links atômicos →
> handlers/wiring), depois frontend (api/queries → md/slash puros → editor+slash+aba → registro
> no tab-shell), E2E e gate. **Estende a fase 24 (tab-shell + componente `CenterPanel` — coord. A) e
> ADICIONA `tasks.CreateTx` (aditivo sobre o `tasks.Create` final da 24 — coord. D) pro slash; NÃO
> reescreve `tasks.Create`** (RNF-05). A task do slash nasce com status `todo` (o default; coord. C —
> NÃO `backlog`). Migração `0024` FIXA (PRD §7) — NÃO ler o próximo livre.

---

## Bloco 1 — backend domínio (T-01 → T-06)

- **T-01 — Migração `0024_notes`** (CA backstop: 06/14, RNF-04)
  - `backend/internal/db/migrations/0024_notes.sql` (plan §2): tabela `notes`
    (`project_id` nullable `ON DELETE CASCADE`, `task_id` nullable `ON DELETE SET NULL`, `title`/
    `body_md` `NOT NULL DEFAULT ''`, `created_by`, `created_at`/`updated_at`) + índices
    `idx_notes_project_updated (project_id, updated_at DESC)` e parcial `idx_notes_loose
    (created_by, updated_at DESC) WHERE project_id IS NULL`; tabela `note_links` (`note_id` FK
    `ON DELETE CASCADE`, `ref_type CHECK IN ('task','note')`, `ref_id` uuid **sem FK** — D-08,
    `position`, `workspace_id` denormalizado) + `idx_note_links_note (note_id, position)` +
    `uq_note_links (note_id, ref_type, ref_id)`.
  - Conferir o diretório antes: o último é `0020_reminders` (M2); a M3 numera FIXO — 24=`0021`/`0022`,
    25=`0023`, **esta=`0024`** (PRD §7). NÃO "ler o próximo livre".
  - Teste (`migrations_*_test.go` ou novo, dbtest): as tabelas existem; defaults (`title=''`,
    `body_md=''`, `position=0`); `ref_type` CHECK barra valor fora de `{task,note}`; **`uq_note_links`
    barra link duplicado via INSERT DIRETO** (teste de **integridade de schema** — é o ÚNICO teste do
    `uq`, 26-H8: o slash sempre gera `ref_id` novo/uuid fresh, então a colisão é inalcançável pela
    API; NÃO há decisão de produto 409×idempotência a tomar — isso NÃO vira um CA de produto na T-05);
    **`ON DELETE CASCADE` da `note_id`**: apagar nota apaga seus links (não a entidade); **apagar
    projeto apaga as notas do projeto** (CASCADE) e **não** apaga nota solta; apagar task com nota
    atrelada zera `notes.task_id` (SET NULL).
  - INDEX.md: registrar 0024 dona da fase 26 (orquestrador).

- **T-02 — `notes.Service`: gate de visibilidade (cópia, D-03)** (CA-11/11.1/13/14)
  - `internal/notes/gate.go` (NOVO): `loadNoteGate(ctx, ws, userID, role, noteID, blockArchived)
    (gate, error)` — para nota de **projeto**: visibilidade `(visibility='shared' OR
    owner_user_id=$u)`→404; escrita (dono ∥ `role=="owner"` ∥ `project_members.access=="edit"`)→403;
    **projeto arquivado SÓ bloqueia quando `blockArchived==true` → `400 invalid_reference` "projeto
    arquivado é só leitura"** (26-H7/G — espelha `tasks/service.go:240`; NÃO existe código `archived`;
    NÃO é 403). Para nota **solta** (`project_id IS NULL`): `created_by==$u`→ok; senão 404 (CA-11.1).
    `uuid.Validate(noteID)` antes do SQL → 404 nunca 500. Cópia do JOIN de `comments/service.go`/
    `tasks/service.go` — **NÃO** importa `projects`/`tasks` pra ler; **NÃO** toca os pacotes da fase 02
    (ficam byte-idênticos).
  - **`blockArchived` POR OPERAÇÃO** (26-H7): `Create`/`Patch` chamam com `blockArchived=true`;
    **`Delete` chama com `blockArchived=false`** (apagar nota de projeto arquivado é PERMITIDO →
    `204`, espelha `tasks.Delete service.go:609` e `comments.Delete`). NÃO usar `true` no Delete.
  - Teste `gate_test.go` (dbtest): nota de projeto private de outro → 404; `access=view`/member-sem-
    access → 403 em escrita; **arquivado+`blockArchived=true` → `400 invalid_reference`** (NÃO 403);
    dono/owner-ws/`access=edit` → ok; **nota solta de outro usuário → 404** (CA-11.1); `noteID`
    malformado → 404 (não 500). Suítes da fase 02/12/19 **continuam verdes sem mudança**.

- **T-03 — `notes.Service`: CRUD de nota + validação** (CA-01/01.1/01.2/02/02.1/03/03.1/04/05/05.1/06/06.1)
  - `internal/notes/service.go`: `Note`, `NoteLink`, `CreateInput`/`PatchInput` (title/body tri-state
    — D-09/CA-05), `List`/`Get`/`Create`/`Patch`/`Delete`. Validação na mão: `title` trim ≤ 200 (CA-
    01.1/05.1), `body_md` sem limite (D-06). `Create`: **decidir o gate pelo `project_id` EFETIVO**
    (payload OU herdado do `task_id`), não pela presença de `project_id` no payload (26-H3/CA-02.1):
    (1) se `task_id` informado → valida visibilidade/mesmo-ws senão **`400 invalid_reference`**
    (CA-02; ref de PAYLOAD = `invalid_reference`, igual `objective_id`/`client_id` da fase 02/12) e
    **herda o `project_id` da task** (tasks.project_id é NOT NULL → nota vira de PROJETO, nunca solta);
    (2) com `project_id` efetivo não-nulo → roda o gate de **escrita** (`blockArchived=true`) contra
    ESSE id (CA-01/13; viewer/`access=view` que só vê a task → 403, CA-02.1/CA-11); (3) `project_id`
    efetivo nulo (nem payload nem task) → nota solta sem gate de projeto (CA-01.2). `List(projectID
    =="")` → notas soltas do user (CA-03.1); `List(projectID!="")` → notas do projeto visível, ordem
    `updated_at desc`, com `preview` derivado (D-05) — mas a **resolução de links** entra na T-05
    (aqui `links=[]` ou raw; ajustar na T-05). `Patch` carrega pelo gate com **`blockArchived=true`**;
    **`Delete` com `blockArchived=false`** (apagar nota de projeto arquivado é permitido — 26-H7/
    CA-06.1, espelha `tasks.Delete`/`comments.Delete`); `updated_at=now()` no patch; delete hard +
    cascade dos links (CA-06).
  - Teste `service_test.go` (dbtest): create de projeto/solta/atrelada-a-task; `title>200` → 400 não
    persiste (CA-01.1); `task_id` de outro ws/invisível/malformado → **400 `invalid_reference`** (CA-02);
    **viewer/`access=view` que ENXERGA uma task posta `POST` com `{task_id}` (sem `project_id`) → 403
    forbidden e nada persiste** (CA-02.1/26-H3 — **mutação que pule o gate no caminho task_id-inferido,
    ou que decida o gate por "tem project_id no payload?", DEVE quebrar este teste**); list por projeto
    ordenado por `updated_at desc` + `preview` correto (markdown strip, ≤140 — D-05); list soltas só do
    user (CA-03.1) — **mutação removendo `created_by=$u` do WHERE de soltas DEVE quebrar** (CA-14); get
    de nota invisível/malformada → 404 (CA-04); patch title+body atualiza `updated_at`; **Create/Patch
    em projeto ARQUIVADO → 400 `invalid_reference` ("projeto arquivado é só leitura")**; **DELETE de
    nota em projeto ARQUIVADO → 204 (NÃO 403/400)** (CA-06.1/26-H7, pinando a matriz blockArchived
    red→green); delete hard + 2º delete → 404 (CA-06); **isolamento por workspace** (mutação removendo
    `WHERE workspace_id` morre — CA-14).

- **T-04 — `tasks.CreateTx` aditivo (reuso pro slash)** (RNF-05/26-H4/26-H1, base do CA-07; coord. C+D)
  - Decisão TRAVADA (plan §5, coordenação D): adicionar `tasks.CreateTx(ctx, tx pgx.Tx, ws, userID,
    role, projectID, name string) (Task, error)` — método **ADITIVO** construído SOBRE a assinatura
    final que a fase 24 deixar, que faz **SOMENTE** `validName` + `loadProjectGate(blockArchived=true)`
    + INSERT na tx fornecida com `created_by`=sessão e **status no DEFAULT do enum (`todo`)** — o
    INSERT NÃO seta status (a coluna fica no default; **NÃO crava `backlog`** — coord. C, 26-H1). **SEM**
    objective/assignee/due/recurrence/notes/tz, **SEM** `recalcObjectiveStatus`, **SEM** `reminder.
    OnDueDateSet`, **SEM** `AssignNotify` — nada disso se aplica ao slash (§4.3), então o `CreateTx`
    é enxuto e os invariantes pesados do `Create` NÃO correm risco de omissão (26-H4). **`tasks.Create`
    fica BYTE-IDÊNTICO** (ZERO refactor, ZERO delegação — RNF-05); `tasks` **não** importa `notes`.
    **Remover** a frase antiga "passa a delegar a ele"/"refactor transparente" E a alternativa "INSERT
    inline replicando a validação" (ambas descartadas).
  - Teste (`tasks` dbtest): `CreateTx` numa tx que dá **rollback** → a task **não** persiste (prova
    atomicidade — base do CA-07); `CreateTx` numa tx que **commita** → task com **`status='todo'`** (o
    default; **teste de MUTAÇÃO**: se o INSERT setar `'backlog'`/qualquer outro valor, o assert
    `status=='todo'` DEVE quebrar — 26-H1), nome trimado, project gate respeitado; nome inválido em
    `CreateTx` → erro de validação (sem persistir); projeto arquivado → `400 invalid_reference`.
    **Suíte existente de `tasks.Create` continua verde SEM mudança (prova zero-diff/RNF-05).**

- **T-05 — `notes.Service`: links atômicos (slash) + resolução** (CA-07/08/08.1/09/10/12)
  - `internal/notes/links.go`: `CreateLink(ctx, ws, userID, role, noteID, kind, name)` —
    **transação única** (D-04): carrega a nota pelo gate de escrita (`blockArchived=true`);
    `kind=="task"`: se `project_id` da nota é NULL → ROLLBACK + `Validation` (CA-10), senão
    `tasks.CreateTx(ctx, tx, ws, userID, role, project_id_da_nota, name)` (a task nasce com **status
    `todo`** — o default; coord. C, **não** `backlog`); `kind=="note"`: cria sub-nota (mesmo
    `project_id` da mãe, ou NULL se mãe solta — CA-10) via INSERT na tx; `kind` outro → ROLLBACK +
    `Validation` (CA-08.1); INSERT `note_links(note_id, ref_type=kind, ref_id=entidade.id,
    position=max+1, workspace_id=ws)`; COMMIT. Falha em qualquer passo → rollback total (sem órfão).
    `DeleteLink(ctx, ws, userID, role, noteID, linkID)` → remove só o `note_link` pelo **id do link**
    (CA-09; forma de rota ÚNICA por `{linkId}` — 26-H6); `linkID` malformado / inexistente / link de
    outra nota → 404. Adicionar a **resolução de links** na leitura (CA-12): `List`/`Get` resolvem
    cada link por batch-query com visibilidade do user **por tipo de alvo** (D-07/26-H2): `task` →
    `LEFT/INNER JOIN tasks` com `(project shared OR owner_user_id=$u)`, label=name + status; `note` →
    **`LEFT JOIN projects` + `(projeto shared OR owner_user_id=$u) OR (sub-nota project_id IS NULL AND
    created_by=$u)`** (NÃO `INNER JOIN projects` cego — isso omitiria sub-nota SOLTA do criador!),
    label=title; alvo sumiu/invisível → **omitido**.
  - Teste `links_test.go` (dbtest): `/task` cria task no projeto da nota + link (CA-07); **a task
    nasce com `status='todo'`** (assert do status — coord. C); **mutação que faz o INSERT do link
    falhar deixa a transação inteira em rollback (task NÃO persiste)** (atomicidade D-04); `/note`
    cria sub-nota + link (CA-08); `kind` inválido → 400 sem criar nada (CA-08.1); **`/task` em nota
    solta → 400 e nenhuma task criada** (CA-10); `/note` em nota solta → sub-nota também solta + link
    (CA-10); `DeleteLink` por `{linkId}` remove só o link, a task sobrevive (CA-09); link de outra
    nota/inexistente/malformado → 404; **resolução omite link cujo alvo (task) virou private de
    outro** (CA-12/D-07 — mutação removendo a cláusula de visibilidade da resolução DEVE quebrar);
    **`/note` numa nota solta → o chip da sub-nota SOLTA aparece RESOLVIDO no array `links` do GET da
    nota-mãe pro criador** (CA-10/CA-12/26-H2 — **mutação que troque a resolução de `ref_type='note'`
    por `INNER JOIN projects` cego, ou que remova a cláusula `(project_id IS NULL AND created_by=$u)`,
    DEVE quebrar este teste**); sub-nota de outro user / de projeto invisível → omitida.
  - **Sobre `uq_note_links` (26-H8):** NÃO testar "duplicar o mesmo link" aqui — o slash SEMPRE gera
    `ref_id` novo (uuid fresh), então a colisão é **inalcançável pela API**; não há decisão de produto
    (409×idempotência). O único teste do `uq` é o de integridade de schema (INSERT direto) na T-01.

- **T-06 — Handlers + wiring (Route IRMÃO)** (CA-01..15)
  - `internal/api/notes_handlers.go`: handlers finos (espelho de `comments_handlers.go`) pra
    `GET/POST /notes`, `GET/PATCH/DELETE /notes/{noteId}`, `POST /notes/{noteId}/links`,
    `DELETE /notes/{noteId}/links/{linkId}`. `auth_routes.go`: subárvore **Route irmão**
    `api.Route("/workspaces/{workspaceId}/notes", ...)` com só `RequireMember` (SEM
    `RequireWriteRole` blanket — o gate fino é no service, D-02; `middleware_role.go` INTOCADO);
    `cfg.notes != nil` guard como o `cfg.comments`. Option `api.WithNotes(api.Notes{Svc:...})`;
    `cmd/server` liga `notes.NewService(pool, tasksSvc)`.
  - Teste `notes_handlers_test.go` (harness `coretest`): GET lista 200 + ordem; POST 201 + persiste;
    nota solta 201 (CA-01.2); PATCH/DELETE; 400 validation/invalid_reference; **403 viewer em escrita
    de nota de projeto** (CA-11); **403 viewer/`access=view` que ENXERGA a task posta `POST /notes`
    com `{task_id}` sem `project_id`** (CA-02.1/26-H3 — o gate roda contra o project_id herdado);
    **leitura por viewer OK** (CA-03/04); 404 projeto/nota invisível na LISTAGEM/rota (CA-13);
    **404 nota solta de outro user** (CA-11.1); POST link `/task` 201 + task aparece **com status
    `todo`** (coord. C); `/task` em nota solta → 400 (CA-10); `DELETE /links/{linkId}` 204 (forma de
    rota ÚNICA — 26-H6); `401` table-driven em TODAS as rotas novas sem sessão (CA-15); smoke de
    routing (chi não colide com `/tasks/{id}/comments` — testar a coexistência das duas subárvores
    irmãs).

> **Fim do bloco 1:** `make test-back` 100% verde (notes + tasks/projects/comments intactos);
> Vitest/Playwright não rodados (bloco sem frontend). Handoff "retomar na T-07".

## Bloco 2 — frontend (T-07 → T-11)

- **T-07 — `lib/api.ts` + `lib/queries/notes.ts`** (CA-03/04/05/07/08/09/16)
  - `Note`/`NoteLink`/`CreateInput`/`PatchInput`/`LinkResult` types; `getNotes(ws, projectId?)`/
    `getNote(ws, id)`/`createNote`/`patchNote`/`deleteNote`/`createLink`/`deleteLink`. Hooks
    `useNotes(ws, projectId)`/`useNote(ws, id)` + mutações (invalidam `notesKey(ws, projectId)`).
    Mutação de **link `/task`** invalida **também** `boardKey(ws, projectId)` via `onSuccess` local
    (a task aparece na aba Tasks — CA-23/RNF-05; NÃO mexer em `lib/queries/tasks.ts`).
  - Teste `notes-api.test.ts` (vitest): chama os paths certos (incl. `?project_id=`); hooks invalidam
    a query certa; link `/task` invalida notes E board.

- **T-08 — `lib/md.ts` (parser markdown) + `lib/slash.ts` (lógica pura)** (CA-19/20/RNF-01)
  - `lib/md.ts`: parser mínimo (D-06, espelha `md()` do mockup linhas 211-220): `#/##/###`, `**b**`/
    `*i*`/`` `code` ``, links `[t](u)`, listas `- `, code-block ```` ``` ````, quebra de linha;
    **sem dependência nova**. `lib/slash.ts` (puro): `detectSlash(value, caret)` → `{query}` quando há
    `/token` no segmento da linha antes do cursor, senão `null`; `applyBlock(value, caret, cmd)` →
    `{value, caret}` inserindo `- [ ] `/`# `/`## `/`\n---\n` (To-do/H1/H2/Divider — CA-20) ou o
    marcador de link (`- [ ] <nome>` p/ task, `[[<título>]]` p/ note — CA-21) no lugar da `/…`.
  - Specs (vitest): `md()` renderiza cada elemento (títulos, b/i/code, link, lista, code-block) +
    escapa `<`/`>`/`&`; `detectSlash` acha `/ta` no cursor e ignora `/` no meio de uma URL/sem
    cursor adjacente; `applyBlock` insere o markdown certo e devolve o caret pós-inserção.

- **T-09 — `SlashMenu` + `NoteEditor`** (CA-17/18/19/20/21/22)
  - `SlashMenu` (Task·Note·To-do·Heading 1·Heading 2·Divider; ↑/↓ navega, Enter confirma, Esc/clique-
    fora fecha — CA-19; classes `.slashmenu`/`.slashitem` via tokens). `NoteEditor` (title grande +
    textarea `.noteedit` **sempre editável**; chips de link no topo que abrem a entidade — CA-17;
    debounce PATCH ≥600ms — CA-18/D-09; digitar `/` abre o `SlashMenu` via `lib/slash.ts`; To-do/
    Heading/Divider só inserem texto — CA-20; Task/Note chamam `createLink` e inserem marcador + chip
    — CA-21; viewer → `readOnly` sem slash — CA-22).
  - Specs (vitest, fake timers): textarea sempre editável (sem botão Editar); PATCH dispara **após**
    600ms e **não** antes (debounce); `/` + Enter em "To-do" insere `- [ ] ` sem chamar API (CA-20);
    `/` + "Task" chama `createLink({kind:"task"})` e insere chip (CA-21); falha de `createLink` (nota
    solta) sinaliza e não insere chip (CA-10/21); editor `readOnly` quando `can_write=false`, slash
    não abre (CA-22).

- **T-10 — `NotePanel` + aba "Notes" no tab-shell** (CA-16/17/23)
  - `NotePanel` (embrulha `NoteEditor` no componente `CenterPanel` da fase 24 — coord. A; chip "Note ·
    .md"; expande/colapsa pelos controles da 24). `NotesTab` (tabela `Note·Preview·Updated` via
    `.ntable`; "New note" gated por `can_write` cria a nota e abre o painel — CA-16; vazio = "No notes
    yet."). Registrar a aba no tab-shell do `ProjectDetail.tsx` (`S.tab==='notas'` → `NotesTab`) —
    **sem reescrever** o tab-shell da 24 (1 hunk aditivo).
  - Specs (vitest, jsdom): `NotesTab` lista notas (preview + updated) e esconde "New note" quando
    `can_write=false` (CA-22); clicar numa linha abre o `NotePanel`; "New note" cria e abre o editor
    (CA-16/17); painel usa o componente `CenterPanel` (coord. A; chip "Note · .md").

- **T-11 — Gating viewer (componente)** (CA-22)
  - Spec dedicado `viewer-gating-notes.spec.tsx` (padrão das fases 12/19): `can_write=false` esconde
    "New note", abre editor `readOnly`, slash não funciona; `can_write=true` (member control) mostra
    tudo. Cobre o ramo viewer que o E2E não alcança (stub 1-user — CA-23).

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright
> não rodado ainda (E2E é o bloco 3). Handoff "retomar na T-12".

## Bloco 3 — E2E + verificação + gate (T-12 → T-13)

- **T-12 — E2E `notas.spec.ts` + verificação adversarial do build** (CA-23 + correções)
  - E2E (`loginViaStub`, projeto `shared` com escrita): abre a aba **Notes**; **New note** abre o
    editor; digita título + corpo md; **`/` + Task** → task ligada (chip no topo, menu some); fecha o
    editor → a nota aparece na lista com preview; vai pra aba **Tasks** → a task criada existe; reabre
    a nota → corpo/chip persistem após **reload**. Nomes únicos (banco reusado, idempotente). **SEM**
    perfil viewer no E2E (stub 1-user — o gating é a T-11). **Liberar a :8080** antes (server alheio).
  - Verificação adversarial do build: workflow ultracode (dimensões: atomicidade do slash /task,
    isolamento multi-tenant/visibilidade [nota solta × projeto × workspace alheio], resolução de
    links que omite invisível, debounce/PATCH, gating viewer, regressão fases 02/12/19/24) +
    refutação dupla. Corrigir confirmados red→green. Registrar (seção abaixo).

- **T-13 — Gate + deploy(opcional) + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose + check-cores).
  - **Deploy é ÚNICO no fim da M3** (não por fase — regra do CLAUDE.md): esta fase **não deploya
    sozinha**; só deixa o gate verde. (Se for a última da paulada, segue a rotina rsync + `docker
    compose build && up -d` + smoke honesto; NÃO tocar Caddy/PrimaOS.)
  - Handoff (CLAUDE.md "retomar..."), INDEX (26 construída, 0024 dona), commit, briefing pro Diogo.

---

## Blocos (regra de organização — o build da M3 é WORKFLOW-FIRST numa paulada; blocos só guiam o handoff)

- **Bloco 1 — backend domínio:** T-01 → T-06 (migração, gate, CRUD, `tasks.CreateTx`, links
  atômicos, handlers+wiring Route irmão). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-07 → T-11 (api/queries, md+slash puros, editor+slash, painel+aba,
  gating). Suíte: Vitest + `make test-back` (regressão).
- **Bloco 3 — E2E + fechamento:** T-12 → T-13 (E2E, verificação adversarial, gate `make
  check-full` + handoff).

`make check-full` é gate **no bloco 3 (T-13)**; os blocos 1 e 2 fecham com a suíte do que existe
verde + handoff + commit.

---

## Furos da verificação adversarial (ETAPA 2 — aplicados)

Verificação adversarial das specs (workflow ultracode, ancorado no código real). **7 furos
confirmados 2/2**, todos aplicados em spec/plan/tasks ANTES do build. Registro:

- **26-H1 (HIGH · contrato/consistência) — status da task do slash.** A spec/plan/mockup divergiam:
  CA-07 + exemplo do plan + `applySlash` do mockup diziam `status:'backlog'`, mas `tasks.Create`/
  `CreateTx` NÃO setam status (a coluna fica no DEFAULT `'todo'` do enum; a fase 24 só fez `ADD VALUE
  'backlog' BEFORE 'todo'`, sem mudar o DEFAULT). **Resolvido** pela decisão de coordenação **C**: a
  task do slash nasce `'todo'` (o default), NÃO `backlog`. Corrigidos CA-07 (spec) com nota explícita,
  plan §4.2/§4.3 (exemplo `status:"todo"` + texto), e T-04/T-05/T-06 (assert `status=='todo'` +
  **teste de mutação** que mata o caso "INSERT setar outro valor"). Mockup marcado como desatualizado.
- **26-H2 (MEDIUM · multi-tenant/segurança) — resolução de link de SUB-NOTA SOLTA.** A resolução
  (CA-12/D-07) descrevia `INNER JOIN projects`, que descartaria toda sub-nota com `project_id NULL`
  → o chip de uma sub-nota solta criada pelo `/note` (CA-10 permite!) sumiria pro próprio criador.
  **Resolvido:** CA-12 (spec) agora especifica visibilidade **por tipo de alvo**; para `ref_type=
  'note'` usa `LEFT JOIN projects` + `OR (project_id IS NULL AND created_by=$u)`. D-07 (plan) e T-05
  (tasks) ganharam o teste do chip da sub-nota solta resolvendo + a **mutação** (trocar por `INNER
  JOIN` cego / remover a cláusula `created_by=$u` DEVE quebrar).
- **26-H3 (MEDIUM · contrato/segurança) — gate de escrita no caminho `task_id`-inferido.** CA-02 não
  dizia QUAL gate roda quando a nota herda o `project_id` via `task_id`: um viewer que vê a task
  poderia criar nota no projeto burlando CA-11. **Resolvido:** novo **CA-02.1** (spec) crava que a
  nota nasce de PROJETO (tasks.project_id é NOT NULL) e o gate de escrita roda contra o `project_id`
  EFETIVO (payload OU herdado); plan §4.4 + comentário do gate; T-03 decide o gate pelo project_id
  efetivo + teste "viewer com {task_id} → 403"; T-06 ganhou o assert handler-level. Mutação que pule
  o gate nesse ramo DEVE quebrar.
- **26-H4 (MEDIUM · regressão/RNF) — refatorar `tasks.Create` era arriscado.** A T-04 propunha
  refatorar `tasks.Create` (que a 24 reescreve pesado: status/priority/notion_url + objective/
  assignee/reminder hook) pra delegar a um `CreateTx`, com risco de omitir invariantes (reminder due/
  recalc de objetivo). **Resolvido** pela decisão de coordenação **D**: `tasks.CreateTx` é ADITIVO
  (validName + loadProjectGate + INSERT na tx, SEM objective/assignee/due/recalc/reminder/notify) e
  `tasks.Create` fica BYTE-IDÊNTICO (zero refactor; alternativa "INSERT inline" descartada). RNF-05
  (spec), §5 (plan) e T-04 (tasks) reescritos; teste prova zero-diff da suíte de `tasks.Create`.
- **26-H6 (MEDIUM · contrato) — rota do DELETE de link duplicada.** CA-09 citava duas formas
  (`{ref_type}/{ref_id}` OU `{linkId}`) com cross-reference morto. **Resolvido:** CA-09 (spec) e
  plan §4.2/§5 + T-05/T-06 (tasks) fixam UMA forma — `DELETE .../notes/{noteId}/links/{linkId}` (por
  id do link, igual aos irmãos comments/reminders); variante `{ref_type}/{ref_id}` removida.
- **26-H7 (LOW · contrato/migração) — DELETE de nota em projeto arquivado dava 403.** O gate de
  DELETE divergia do precedente real (`tasks.Delete` usa `loadProjectGate(...,false)` — arquivado NÃO
  bloqueia; à-prova-de-culpa TDAH). **Resolvido:** novo **CA-06.1** (spec) crava a matriz
  `blockArchived` por operação — Create=true, Patch=true (→ `400 invalid_reference` "projeto
  arquivado é só leitura", NÃO existe código `archived` — coord. G), **Delete=false** (→ 204). Gate
  comment do plan §5 + T-02/T-03 (tasks) com teste DELETE-em-arquivado → 204.
- **26-H8 (LOW · testabilidade/contrato) — `uq_note_links` inalcançável pela API.** A T-05 mandava
  testar "duplicar o mesmo link (409 vs idempotência)", mas o slash SEMPRE gera `ref_id` novo (uuid
  fresh) → a colisão é inalcançável pela API (não há link-a-existente nesta fase). **Resolvido:**
  T-05 (tasks) remove o CA de produto; D-08 (plan) documenta que o `uq` é defesa-de-banco best-effort;
  o único teste do `uq` é de integridade de schema (INSERT direto) na T-01.

### Decisões cross-fase aplicadas (coordenação)

- **A (popup central = componente `CenterPanel` da fase 24):** spec §1/CA-17/RNF-06/§6, plan §1/§6/
  D-02/§7/§8, tasks header/T-10 trocam toda referência a `panelChrome` (e às classes cruas do mockup
  `.mscrim`/`.scard`/`.swrap`) pelo componente `CenterPanel` introduzido pela 24, via tokens.css.
- **C (slash `/task` cria status DEFAULT `todo`):** aplicada junto com 26-H1 (acima).
- **D (NÃO refatorar `tasks.Create`; usar `tasks.CreateTx` aditivo sobre o `Create` final da 24):**
  aplicada junto com 26-H4 (acima); topologia `notes → tasks` mantida (sem ciclo).
- **G (erros canônicos reais):** projeto arquivado no gate de escrita → `400 invalid_reference` (não
  existe `archived`); ref no PAYLOAD (task_id/project_id) → `400 invalid_reference` (igual `client_id`/
  `objective_id` reais); ref de ROTA/listagem invisível/inexistente e nota inexistente → `404
  not_found`. CA-02/CA-13/CA-06.1 (spec) e plan §4/§5 + tasks alinhados aos códigos reais.
