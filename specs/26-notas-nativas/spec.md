# spec.md — Fase 26 · Notas nativas (.md no Postgres) + slash "/"

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Terceira fase da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`, build order
> 24 → **26** → 27 → 28 → 29 → 25). Cobre **RF-14, RF-15, RF-16, RF-17** do PRD.
> **Decisões travadas (MILESTONE-3.md §3):**
> 7. Notas = **nativas (.md, 1 linha/nota no Postgres)**, editor **sempre editável** (sem botão
>    Editar), com slash `/` criando **task** e **sub-nota** ligadas. **Markdown simples** (não
>    bloco-a-bloco Notion).
> 9.4 (riscos): **o slash `/` cria blocos SÓ nas Notas — NUNCA na descrição da task** (a descrição
>    da task é markdown simples, sem slash).

---

## 1. Mapa pro problema central (MILESTONE-3.md §1 / §3 dec. 7 / §5.3)

A frase do Diogo (PRD §0): _"a documentação real eu escrevo e linko numa página do Notion (...)
notas curtas e captura = nativas (.md no Postgres)."_ Hoje **não existe UI de notas** no Timetrack
(nem tabela). Esta fase adiciona a camada de **captura escrita rápida**: uma nota é **1 linha no
Postgres** (`title` + `body_md` em markdown puro), **sempre editável** (sem modo leitura/edição), com
um menu **slash `/`** que cria **task** e **sub-nota** ligadas (chips clicáveis no topo da nota).

Esta fase **estende a fase 24** (Tasks 2.0): a aba **"Notes"** entra no **tab-shell do projeto**
criado na 24, e o editor de nota reusa o **componente de popup central (`CenterPanel`)** introduzido
pela fase 24 (popup central expansível pra tela cheia, side↔full, via tokens.css) — **não** classes
cruas do mockup (`.mscrim`/`.scard`/`.swrap`) como contrato (decisão de coordenação A). **Não
reescreve** a 24 — só adiciona a aba e um novo conteúdo de painel (`note`/`spotlight` ficam pra 27).
O **backend de notas é independente** (pode ser construído em paralelo com a 24); o **front depende
da 24** (build order 24 → 26).

A criação de task pelo slash **reusa o endpoint de tasks que já existe** (fase 02/12/24,
`POST .../tasks`) — esta fase **não** reimplementa CRUD de task. Ela adiciona o CRUD de **notas** e a
tabela de **links** (`note_links`) que costura nota → task/sub-nota.

## 2. User stories

- **US-01** — Como dono/membro de um projeto, quero **criar uma nota** no projeto em 1 gesto (New
  note → editor já aberto), pra capturar uma ideia sem fricção (< 2s — RNF-02).
- **US-02** — Como usuário, quero que a nota seja **sempre editável** (sem botão Editar), com
  markdown simples (títulos, listas, negrito, código, links), pra escrever fluido.
- **US-03** — Como usuário, quero **listar as notas do projeto** (Nota · Prévia · Atualizada), pra
  reencontrar o que anotei.
- **US-04** — Como usuário, quero digitar **`/`** dentro da nota e abrir um menu (**Task · Note ·
  To-do · Heading · Divider**), pra inserir blocos sem sair do teclado.
- **US-05** — Como usuário, quero que **`/task`** crie uma **task real** no projeto e **`/note`** crie
  uma **sub-nota**, ambas viram **chip clicável** no topo da nota (link), pra costurar a captura ao
  resto do projeto.
- **US-06** — Como viewer do workspace, quero **só ler** as notas do que é compartilhado, sem criar/
  editar/apagar nem usar o slash.

## 3. Critérios de aceite (EARS)

> Rotas: notas de projeto são aninhadas no workspace (`/api/v1/workspaces/{workspaceId}/notes`,
> filtro `?project_id=`). A visibilidade da nota **herda do projeto** (nota com `project_id`) ou é
> **do criador** (nota solta, `project_id=null`) — ver CA-13/14. Erros canônicos do INDEX.md.

### Notas — CRUD (API)

- **CA-01** *(event-driven — criar nota de projeto)* — QUANDO um usuário **com permissão de escrita
  no projeto** (dono do projeto, owner do workspace em projeto visível a ele, ou membro do projeto
  com `access=edit`) envia `POST .../notes` com `project_id` válido e `title` ≤ 200 caracteres (após
  trim; vazio é **permitido** — vira "Untitled" no client, mas a coluna aceita `''`), o sistema DEVE
  criar a nota com `body_md=''` (default), `created_by` = usuário da sessão, `created_at=updated_at=
  now()`, e retornar `201` com o objeto (id, workspace_id, project_id, task_id, title, body_md,
  created_by, created_at, updated_at).
- **CA-01.1** *(unwanted — título grande)* — SE `title` tiver mais de 200 caracteres (após trim), o
  sistema NÃO DEVE criar/atualizar e DEVE retornar `400` `error.code="validation_error"`.
- **CA-01.2** *(event-driven — criar nota solta)* — QUANDO um usuário envia `POST .../notes` **sem**
  `project_id` (ou `project_id:null`), o sistema DEVE criar uma **nota solta** (`project_id=null`,
  `task_id=null`) **do criador** (`created_by` = sessão) e retornar `201`. Nota solta não passa pelo
  gate de projeto (CA-13).
- **CA-02** *(event-driven — atrelar a task, opcional)* — QUANDO o `POST .../notes` inclui `task_id`,
  o sistema DEVE validar que a task existe, é **visível** ao usuário (CA-09 da fase 02) e está no
  **mesmo workspace**; caso ok, grava `task_id` e herda o `project_id` da task. SE a `task_id` não
  existir / for de outro workspace / invisível / for uuid **malformado**, o sistema NÃO DEVE escrever
  e DEVE retornar `400` `error.code="invalid_reference"` (este é um **ref no payload**, idêntico ao
  precedente real de `objective_id`/`client_id`/`user_id`-membro da fase 02/12 —
  `backend/internal/tasks/objective_link.go:30`/`projects/service.go:116` retornam `invalid_reference`
  para ref-de-payload inexistente/de-outro-ws/malformada). Isso é coerente com a decisão de
  coordenação G: o `404 not_found` da G vale para refs de **rota/path** e visibilidade de projeto na
  listagem; o ref atrelado no corpo segue a convenção `invalid_reference`.
- **CA-02.1** *(unwanted — gate de escrita roda contra o project_id EFETIVO)* — Como `tasks.project_id`
  é **NOT NULL** (toda task tem projeto — D-08), um `POST .../notes` com `task_id` (sem `project_id` no
  payload) **sempre** herda o `project_id` da task e a nota NASCE como **nota de projeto**, NUNCA
  solta. O gate de **escrita** do projeto inferido DEVE rodar contra o `project_id` **EFETIVO** da
  nota (payload OU herdado da task), não contra a presença de `project_id` no payload: viewer do
  workspace, member com `access=view`, ou quem só enxerga o `shared` sem `access=edit` (mesmo que veja
  a task) → `403` `forbidden` e nada persiste (CA-11). Isto impede que um viewer crie nota no projeto
  burlando o gate via `task_id`.
- **CA-03** *(event-driven — listar por projeto)* — QUANDO um membro que **enxerga** o projeto envia
  `GET .../notes?project_id={id}`, o sistema DEVE retornar as notas daquele projeto **ordenadas por
  `updated_at` desc** (desempate `created_at, id`), cada uma com um campo `preview` (primeiros ≤ 140
  caracteres do `body_md` com markdown removido — derivado no backend, ver D-05) e os **links** já
  resolvidos (CA-12). Projeto invisível → `404` (herda visibilidade — CA-13).
- **CA-03.1** *(event-driven — listar soltas)* — QUANDO um usuário envia `GET .../notes` **sem**
  `project_id` (ou com `?scope=loose`), o sistema DEVE retornar **apenas as notas soltas dele**
  (`project_id IS NULL AND created_by = userID`), na mesma ordem. Nunca lista notas soltas de outro
  usuário (CA-14).
- **CA-04** *(event-driven — get de uma nota)* — QUANDO um usuário envia `GET .../notes/{noteId}` de
  uma nota visível a ele, o sistema DEVE retornar o objeto completo (incl. `body_md` e `links`). SE a
  nota não existir, for de outro workspace, invisível, ou `noteId` for **malformado**, DEVE retornar
  `404` `not_found` (mesma resposta — não vaza existência).
- **CA-05** *(event-driven — patch title/body)* — QUANDO um usuário com escrita envia `PATCH
  .../notes/{noteId}` com qualquer subconjunto de `{title, body_md}`, o sistema DEVE persistir os
  campos presentes, atualizar `updated_at = now()`, e retornar `200` com o objeto atualizado.
  `title`/`body_md` ausentes mantêm o valor (tri-state ausente×presente; `body_md` aceita `''`).
- **CA-05.1** *(unwanted — patch inválido)* — SE o `PATCH` mandar `title` com mais de 200 caracteres,
  o sistema NÃO DEVE escrever e DEVE retornar `400` `validation_error`. `body_md` **não** tem limite
  de tamanho aplicado (markdown longo é válido — D-06).
- **CA-06** *(event-driven — deletar)* — QUANDO um usuário com escrita envia `DELETE
  .../notes/{noteId}`, o sistema DEVE remover a nota (hard delete) e retornar `204`. Os `note_links`
  da nota DEVEM ser removidos em cascata (`ON DELETE CASCADE`). **Apagar a nota NÃO apaga as
  entidades ligadas** (a task/sub-nota referenciada sobrevive — só o link some). 2º `DELETE` do mesmo
  id → `404` (idempotente — não vaza).
- **CA-06.1** *(state-driven — DELETE não bloqueia em projeto arquivado)* — O gate de escrita usa
  `blockArchived` POR OPERAÇÃO, espelhando o precedente real: **Create=true** e **Patch=true**
  (escrever conteúdo num projeto arquivado → `400 invalid_reference "projeto arquivado é só leitura"`,
  como `tasks.Create`/`Patch` CA-15.2 — `tasks/service.go:240`); **Delete=false** (apagar uma nota de
  um projeto arquivado é **permitido** → `204`, NÃO `403`/`400`, espelhando `tasks.Delete`
  [`service.go:609`, `loadProjectGate(..., false)`] e `comments.Delete`). À-prova-de-culpa do TDAH
  (PRD): nada trava o apagar.

### Links (note_links) — costura do slash

- **CA-07** *(event-driven — slash /task cria task + link, atômico)* — QUANDO um usuário com escrita
  numa nota de projeto envia `POST .../notes/{noteId}/links` com `{kind:"task", name:"..."}`, o
  sistema DEVE, **numa única transação**: (a) criar uma task no `project_id` da nota com o **status
  DEFAULT do backend = `'todo'`** (a task é criada via `tasks.CreateTx` — método aditivo da T-04 que
  delega à mesma lógica de insert do `tasks.Create` final da fase 24; `tasks.Create` NÃO recebe
  status na criação, a coluna fica no DEFAULT `'todo'` do enum — decisões de coordenação C e D),
  `created_by`=sessão, (b) inserir um `note_links(note_id, ref_type='task', ref_id=task.id,
  position=fim, workspace_id)`, e retornar `201` com `{link:{ref_type,ref_id,position}, task:{...}}`.
  SE qualquer passo falhar, a transação inteira DEVE reverter (sem task órfã nem link órfão — D-04).
  > **Nota (coordenação C):** a task do slash NÃO nasce `'backlog'` — nasce `'todo'` (o DEFAULT da
  > coluna; a fase 24 só fez `ADD VALUE 'backlog' BEFORE 'todo'`, sem mudar o DEFAULT). O mockup
  > `applySlash` está desatualizado quanto a isso; o contrato real (CA-07) é `'todo'`. Quem quiser a
  > task no Backlog move depois (PATCH status), igual a qualquer task nova.
- **CA-08** *(event-driven — slash /note cria sub-nota + link, atômico)* — QUANDO o `POST
  .../notes/{noteId}/links` manda `{kind:"note", title:"..."}`, o sistema DEVE, numa única
  transação, criar uma **sub-nota** (mesmo `project_id` da nota-mãe; `task_id=null`;
  `created_by`=sessão) e inserir `note_links(note_id, ref_type='note', ref_id=subnote.id,
  position=fim, workspace_id)`, retornando `201` com `{link, note:{...}}`. Reverte tudo em falha (D-04).
- **CA-08.1** *(unwanted — kind inválido)* — SE o `kind` não for `task` nem `note`, o sistema DEVE
  retornar `400` `validation_error` sem criar nada.
- **CA-09** *(event-driven — remover link sem apagar a entidade)* — QUANDO um usuário com escrita
  envia `DELETE .../notes/{noteId}/links/{linkId}` (o `linkId` é o campo `id` retornado em cada item
  de `links` na leitura — CA-12), o sistema DEVE remover **apenas** a linha de `note_links` e retornar
  `204`; a task/sub-nota referenciada **permanece**. Link inexistente / `linkId` malformado / link de
  outra nota → `404`. **Forma única de rota** (alinha plan §4.2/§5, T-05/T-06 e a convenção dos irmãos
  `comments`/`reminders` — por `id` do recurso, não por `{ref_type}/{ref_id}`).
- **CA-10** *(state-driven — nota solta não cria task de projeto pelo slash)* — SE a nota for **solta**
  (`project_id IS NULL`) e o usuário enviar `POST .../notes/{noteId}/links` com `kind:"task"`, o
  sistema NÃO DEVE criar a task (não há projeto pra ancorá-la) e DEVE retornar `400`
  `validation_error` (`"nota solta não tem projeto para a task"`). `kind:"note"` numa nota solta DEVE
  criar a sub-nota **também solta** (`project_id=null`, `created_by`=sessão) e o link — permitido.

### Roles, auth e isolamento (herda fases 02/12/19)

- **CA-11** *(unwanted — viewer escreve)* — SE um usuário com role `viewer` no workspace, um membro
  do projeto com `access=view`, ou alguém que enxerga o `shared` mas sem `access=edit` enviar
  `POST`/`PATCH`/`DELETE` de nota ou de link de uma nota de projeto, o sistema NÃO DEVE alterar nada
  e DEVE retornar `403` `forbidden`. (Leitura segue permitida — CA-03/04.)
- **CA-11.1** *(state-driven — nota solta é do dono)* — A nota solta (`project_id IS NULL`) é
  governada **só** pelo `created_by`: o criador lê/edita/apaga; qualquer outro usuário do mesmo
  workspace recebe `404` (não vaza). Role do workspace **não** dá acesso a nota solta alheia.
- **CA-12** *(event-driven — links resolvidos na leitura)* — O `GET .../notes/{noteId}` e o
  `GET .../notes?project_id=` DEVEM devolver, por nota, um array `links` com cada link **resolvido**
  para exibir o chip: `{id, ref_type, ref_id, label, status?, position}` — `task` → `label`=nome da
  task + `status`=status atual; `note` → `label`=título da sub-nota. A visibilidade aplicada na
  resolução é **por tipo de alvo**:
  - **`ref_type='task'`** → visível pela regra de **task** (`project shared` OU `owner_user_id=$u`);
    task que ficou private de outro → **omitida** (não vaza o nome).
  - **`ref_type='note'`** → visível pela regra de **NOTA** (a mesma do gate da T-02), com os DOIS
    ramos: a sub-nota resolve se `(projeto shared OU project.owner_user_id=$u)` **OU** `(sub-nota com
    project_id IS NULL AND created_by=$u)`. Em particular, uma **sub-nota SOLTA do próprio criador**
    (criada por `/note` numa nota solta — CA-10) DEVE resolver e aparecer como chip; uma sub-nota de
    projeto invisível, ou solta de outro usuário, DEVE ser **omitida**.

  A resolução **NÃO** usa `INNER JOIN projects` cego para `ref_type='note'` (isso descartaria toda
  sub-nota com `project_id NULL`); usa `LEFT JOIN projects` + a cláusula `OR (n.project_id IS NULL AND
  n.created_by=$u)` (D-07). Um link cujo alvo sumiu (deletado) ou ficou invisível DEVE ser **omitido**
  do array (não vaza o nome — D-07).
- **CA-13** *(unwanted — projeto invisível)* — SE a rota referenciar um projeto **invisível** ao
  usuário (private de outro — CA-09 da fase 02), inexistente, de outro workspace, ou uuid malformado,
  a resposta segue os códigos canônicos REAIS (decisão de coordenação G; espelha as fases 02/12),
  **nunca** os dados do outro tenant:
  - `?project_id=` numa LISTAGEM (`GET .../notes?project_id=`) referenciando projeto invisível/
    inexistente → `404` `not_found` (ref de **rota**, igual ao gate de listagem das fases anteriores).
  - `project_id` no payload de `POST` referenciando projeto invisível/inexistente/de outro ws/uuid
    malformado → `400` `invalid_reference` (ref de **payload**, idêntico ao `client_id` de
    `projects/service.go:116` e ao `objective_id` de `objective_link.go:30`).
  - `task_id` no payload de `POST` inválido/invisível/de outro ws → `400` `invalid_reference` (CA-02).
  - Projeto **arquivado** no gate de **escrita** (Create/Patch) → `400` `invalid_reference`
    (`"projeto arquivado é só leitura"`, espelha `tasks/service.go:240` — NÃO existe código `archived`;
    DELETE NÃO bloqueia em arquivado — CA-06.1).
- **CA-14** *(ubíquo — isolamento multi-tenant; teste de vazamento OBRIGATÓRIO)* — Toda query DEVE
  filtrar por `workspace_id` da rota **e** pela visibilidade (projeto `shared`/dono p/ nota de
  projeto; `created_by` p/ nota solta). Um usuário que referencie (URL ou payload) uma nota/link de
  **outro** workspace, ou uma nota solta de **outro** usuário, DEVE receber `404` — nunca o conteúdo
  do outro tenant. Mutação que remova o `WHERE workspace_id` / a cláusula de visibilidade DEVE quebrar
  um teste.
- **CA-15** *(unwanted — sem sessão)* — SE a requisição chegar sem cookie de sessão válido, qualquer
  rota desta fase DEVE retornar `401` `error.code="unauthorized"`.

### UI — aba Notes (tab-shell da 24) + editor + slash

- **CA-16** *(event-driven — aba Notes na lista)* — A aba **"Notes"** do projeto (no tab-shell da
  fase 24) DEVE listar as notas do projeto numa tabela estilo Notion (colunas **Note · Preview ·
  Updated**, ícone `.md`), e um botão **"New note"** (gated por escrita — CA-22). Clicar numa linha
  abre o editor da nota (CA-17). Sem notas: mensagem "No notes yet." + o botão.
- **CA-17** *(event-driven — editor sempre editável)* — Ao abrir/criar uma nota, o sistema DEVE exibir
  o **editor dentro do componente de popup central (`CenterPanel`) introduzido pela fase 24**
  (decisão de coordenação A — popup central expansível side↔full via tokens.css; NÃO classes cruas do
  mockup `.mscrim`/`.scard`/`.swrap`): título grande editável + textarea de markdown **sempre editável**
  (sem botão Editar — RF-15/dec. 7), com placeholder indicando "press / for commands". Os **chips dos
  links** aparecem no topo (abaixo do título); clicar num chip abre a entidade (task → painel de task
  da 24; nota → editor da sub-nota, no mesmo `CenterPanel`).
- **CA-18** *(event-driven — salvar com debounce)* — ENQUANTO o usuário digita no título ou no corpo,
  o cliente DEVE persistir via `PATCH .../notes/{id}` com **debounce** (≥ 600 ms sem digitar) e
  refletir o novo `updated_at` na lista — **sem reload**. Formulário não chama a API com `title >
  200` (sinaliza erro local) — espelha CA-05.1.
- **CA-19** *(event-driven — slash abre menu)* — QUANDO o usuário digita **`/`** no início de um
  "token" no corpo da nota, o sistema DEVE abrir um menu **Task · Note · To-do · Heading 1 · Heading
  2 · Divider** filtrado pelo texto após a `/`; navegável por ↑/↓, confirmável por **Enter**, fechável
  por **Esc** ou clique fora. (Espelha o `SLASH`/`showSlash`/`noteEditorKey` do mockup, linhas 591–614.)
- **CA-20** *(event-driven — slash de bloco markdown)* — QUANDO o usuário escolhe **To-do / Heading 1
  / Heading 2 / Divider**, o cliente DEVE inserir o markdown correspondente (`- [ ] `, `# `, `## `,
  `\n---\n`) no lugar da `/…` e manter o foco no editor; **nenhuma chamada de API de link** é feita
  (só edição de texto, persistida pelo debounce do CA-18).
- **CA-21** *(event-driven — slash /task e /note criam entidade + chip)* — QUANDO o usuário escolhe
  **Task** ou **Note** no slash, o cliente DEVE chamar `POST .../notes/{id}/links` (CA-07/08); ao
  sucesso, inserir um marcador no markdown (ex.: `- [ ] <nome>` p/ task, `[[<título>]]` p/ nota) **e**
  exibir o **chip** novo no topo da nota, **sem reload**. SE a chamada falhar (ex.: nota solta + task
  — CA-10), a UI sinaliza e **não** insere o chip.
- **CA-22** *(state-driven — viewer só lê)* — ENQUANTO o usuário não puder escrever no projeto (viewer,
  `access=view`, ou projeto arquivado), a UI NÃO DEVE exibir "New note", e o editor DEVE abrir em modo
  **leitura** (sem slash, textarea `readOnly`). A UI deriva a permissão de **`board.project.can_write`**
  (contrato do `/board` da fase 12/24), **não** da role do workspace sozinha. (Backend já nega —
  CA-11.)

### E2E

- **CA-23** — Logado (stub da fase 01) num projeto `shared` com escrita: abre a aba **Notes**, cria
  uma nota (**New note** → editor abre), digita um título e um corpo markdown, **dá `/` e escolhe
  Task** → uma task aparece ligada (chip no topo) e some o menu; fecha o editor, **a nota aparece na
  lista com o preview** e a task criada existe na aba **Tasks**; reabre a nota e o corpo/chip
  persistem após reload. **O gating de `viewer`/`access=view` NÃO é asserido neste E2E** (o
  `googlestub` autentica só o owner — limitação herdada das fases 12/19/24); o **CA-22** é coberto por
  **teste de componente jsdom** (`can_write=false` mockado) na T-11.

## 4. Requisitos funcionais (resumo)

| RF (PRD §5.3) | Descrição | CAs |
|---|---|---|
| RF-14 | Notes lista as notas do projeto (Nota·Prévia·Atualizada) + criar (New note) | CA-03, 03.1, 16 |
| RF-15 | Editor sempre editável (markdown), popup central expansível (reusa o `CenterPanel` da 24 — coordenação A) | CA-05, 17, 18 |
| RF-16 | Slash `/` abre menu Task·Note·To-do·Heading·Divider | CA-19, 20 |
| RF-17 | `/task` cria task (status `todo` — coord. C) + link (chip), `/note` cria sub-nota + link; chips abrem a entidade | CA-07, 08, 09, 12, 21 |
| — (CRUD+isolamento) | criar/get/patch/delete nota; multi-tenant; roles | CA-01, 01.1, 01.2, 02, 02.1, 04, 06, 06.1, 10, 11, 11.1, 13, 14, 15, 22 |

## 5. Requisitos não-funcionais

- **RNF-01 (leveza / anti-NOA)** — zero dependência Go nova além do baseline (chi + pgx); validação
  na mão com stdlib. **Markdown simples**, NÃO editor bloco-a-bloco (PRD dec. 7 / §8): o render é um
  **parser markdown mínimo** (espelha o `md()` do mockup, linhas 211–220) escrito à mão ou **uma lib
  JS minúscula** — **escolha justificada no plan §6** (preferência: parser próprio ≈ 30 linhas, sem
  dep, igual ao mockup; lib só se a leveza compensar). **Nenhuma** lib de editor rico (ProseMirror/
  TipTap/Slate) — é anti-NOA explícito.
- **RNF-02 (captura < 2s)** — criar nota = 1 gesto (New note abre o editor já criado); criar task/
  sub-nota pelo slash = 1 gesto (escolher no menu). Nenhum fluxo exige mais que digitar o nome.
- **RNF-03 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): a tabela de notas,
  o editor, os chips e o menu slash usam tokens/classes existentes (`.tt-panel`, `.chip`, `.eyebrow`,
  `--surface-*`, `--border`, `--accent`, `--text-*`) e as classes `.ntable`/`.noteedit`/`.slashmenu`/
  `.slashitem` do mockup (que viram CSS via tokens). Nenhum hex literal em `.tsx`.
- **RNF-04 (migração)** — a fase 26 **é dona da migração `0024_notes`** (número FIXO do PRD §7 —
  **NÃO** "ler o próximo livre"; a 24 leva `0021`+`0022`, a 25 `0023`, esta `0024`). Cria `notes` +
  `note_links`. Nenhuma outra tabela nova.
- **RNF-05 (não regredir)** — TRACKING (cronômetro/sync/foco) **zero-diff** (RNF-01 do PRD). A criação
  de task pelo slash **adiciona** `tasks.CreateTx` (método ADITIVO que aceita uma `pgx.Tx` e delega à
  MESMA lógica de insert do `tasks.Create` final da fase 24 — decisão de coordenação D); **NÃO**
  refatora/reescreve `tasks.Create` (que fica byte-idêntico ao da 24) nem duplica a regra de nome/
  visibilidade num INSERT inline. Topologia `notes → tasks` (uma direção, sem ciclo). As fases
  02/12/19/24 permanecem funcionais e a suíte de `tasks.Create` continua verde sem mudança; esta só
  ADICIONA o pacote `internal/notes`, o `tasks.CreateTx` aditivo, a subárvore `/notes` e a aba "Notes".
- **RNF-06 (mobile)** — a aba Notes e o editor seguem o breakpoint 768 estabelecido na M2 (fase 22):
  a tabela vira lista 1-col < md; o popup central ocupa a tela < md (reusa o reflow do `CenterPanel`
  da 24 — coordenação A).
- **RNF-07 (perf)** — `GET .../notes?project_id=` < 200 ms com ~200 notas (preview/links derivados sem
  N+1 — D-05/D-07: 1 query de notas + 1 query de links + resolução em memória). Índice
  `(project_id, updated_at desc)` em notas e `(note_id)` em note_links.
- **Segurança/multi-tenant** — toda rota atrás de sessão+membership; recurso inexistente e invisível
  respondem o **mesmo** `404`; CA-14 é inegociável (SDD §8.4).
- **PT-BR na UI**; identificadores de código em inglês (nomes de teste em PT-BR são exceção
  registrada). **Fonte Inter** já assumida (a troca Geist→Inter é da fase 24).

## 6. Fora de escopo (fronteiras explícitas)

- **Spotlight (Cmd+K)** e o painel `spotlight` → **fase 27** (esta fase entrega o painel de nota e o
  CRUD; a 27 reusa o `CenterPanel` da 24 + os endpoints de notas pra `/note` global). O `/note` do
  **Spotlight** (criar nota de fora do projeto) é da 27; aqui o slash `/note` é **dentro de uma nota**
  (cria sub-nota ligada).
- **Slash na descrição da task** — explicitamente FORA (PRD §9.4): o `/` cria blocos **só nas Notas**.
  A descrição da task (`tasks.notes`, fase 24) segue markdown simples **sem** menu slash.
- **Editor bloco-a-bloco Notion** (drag de blocos, embeds, toggles ricos, colunas, tabelas no corpo)
  → anti-NOA (PRD §8). O corpo da nota é **um textarea de markdown puro**.
- **Backlinks bidirecionais / grafo de notas** — só o link **dirigido** nota → task/sub-nota
  (`note_links`). Nada de "quais notas apontam pra esta task" nesta fase.
- **Compartilhar nota / colaboração em tempo real na nota** (cursores, presença no editor) → fora
  (multiplayer de nota é futuro; o multi-tenant da fase só governa quem **vê**).
- **Versionamento / histórico de edição** da nota → fora (só `updated_at`).
- **Anexos / imagens / upload** no corpo → fora (markdown puro, sem storage de blob).
- **Renderização do markdown além do `md()` mínimo** (KaTeX, Mermaid, syntax-highlight rico) → fora
  (RNF-01).
- **`/goal` e `/timer`** dentro da nota → fora (o slash da nota é só Task·Note·To-do·Heading·Divider —
  mockup linhas 592–599; `/goal`/`/timer` vivem no **Spotlight**, fase 27).
