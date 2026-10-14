# plan.md — Fase 26 · Notas nativas (.md no Postgres) + slash "/"

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> assinaturas + contratos de rota + ordem de tarefas. **Estende a fase 24** (tab-shell +
> componente de popup central `CenterPanel` — coordenação A) e **adiciona** `tasks.CreateTx`
> (método aditivo sobre o `tasks.Create` final da fase 24 — coordenação D) pro slash `/task`. Herda
> o baseline do `SDD.md §7` e a cadeia de middlewares (`RequireAuth` → `RequireMember` →
> `RequireWriteRole`). Nada aqui é código — é o contrato.

---

## 1. Arquitetura geral

```
Backend (pacote novo internal/notes; reusa internal/tasks pro slash /task):
  migrations/0024_notes.sql                   ← tabelas notes + note_links (RNF-04; número FIXO 0024)
  internal/notes/service.go         (NOVO)    ← notes.Service: CRUD de nota (CA-01..06) + gate de visibilidade
  internal/notes/links.go           (NOVO)    ← CreateLink (slash /task,/note — atômico, CA-07..10) + DeleteLink + resolve (CA-12)
  internal/api/notes_handlers.go    (NOVO)    ← REST de notas + links (handlers finos)
  internal/api/auth_routes.go (existe)        ← + subárvore /workspaces/{ws}/notes (Route IRMÃO, como comments — D-02)

Frontend (a aba Notes + editor):
  lib/api.ts                                  ← Note/NoteInput/NoteLink + getNotes/getNote/createNote/patchNote/deleteNote/createLink/deleteLink
  lib/queries/notes.ts              (NOVO)    ← useNotes(projectId) + useNote + mutações (invalidam notesKey)
  lib/md.ts                         (NOVO)    ← parser markdown mínimo (espelha o md() do mockup, linhas 211-220) — D-06
  lib/slash.ts                      (NOVO)    ← lógica PURA do slash: detecta /token no cursor, filtra, aplica bloco — testável sem DOM
  components/notes/
     NotesTab.tsx                   (NOVO)    ← aba "Notes": tabela Note·Preview·Updated + "New note" (CA-16/22)
     NotePanel.tsx                  (NOVO)    ← editor sempre-editável dentro do componente CenterPanel da 24 (coord. A) (CA-17/18)
     NoteEditor.tsx                 (NOVO)    ← title + textarea md + chips de link + menu slash (CA-19/20/21)
     SlashMenu.tsx                  (NOVO)    ← menu Task·Note·To-do·Heading·Divider (CA-19)
  routes/cockpit/ProjectDetail.tsx (existe/24)← registra a aba "Notes" no tab-shell (S.tab==='notas' → NotesTab)
```

## 2. Migração — `0024_notes.sql` (dona: fase 26)

```sql
-- 0024_notes — fase 26 (plan §2): notas nativas .md (1 linha/nota) + links do slash.
-- workspace_id denormalizado (igual tasks/objectives, D-09 da fase 02) pro filtro
-- tenant trivial (CA-14). Nota é do PROJETO (project_id) ou SOLTA (project_id NULL,
-- governada por created_by — CA-11.1); pode atrelar a uma task (task_id). Markdown
-- puro em body_md. Sem soft-delete: nota é hard delete (CA-06); links caem em cascata.
CREATE TABLE notes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid REFERENCES projects(id) ON DELETE CASCADE,   -- NULL = nota solta
    task_id      uuid REFERENCES tasks(id)    ON DELETE SET NULL,  -- atrelada a uma task (opcional)
    title        text NOT NULL DEFAULT '',
    body_md      text NOT NULL DEFAULT '',
    created_by   uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
-- lista por projeto ordenada por updated_at desc (CA-03/RNF-07)
CREATE INDEX idx_notes_project_updated ON notes (project_id, updated_at DESC);
-- lista de notas soltas do usuário (CA-03.1): só as com project_id NULL
CREATE INDEX idx_notes_loose ON notes (created_by, updated_at DESC) WHERE project_id IS NULL;

CREATE TABLE note_links (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    note_id      uuid NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    ref_type     text NOT NULL CHECK (ref_type IN ('task', 'note')),
    ref_id       uuid NOT NULL,                       -- id da task OU da sub-nota (sem FK: ver D-08)
    position     integer NOT NULL DEFAULT 0,
    workspace_id uuid NOT NULL REFERENCES workspaces(id)   -- denormalizado (CA-14)
);
CREATE INDEX idx_note_links_note ON note_links (note_id, position);
-- evita link duplicado da mesma entidade na mesma nota
CREATE UNIQUE INDEX uq_note_links ON note_links (note_id, ref_type, ref_id);
```

- Numeração: o último no disco é `0020_reminders`; as fases da M3 vêm em ordem FIXA do PRD §7 — 24
  leva `0021_tasks_alter`+`0022_task_subtasks`, 25 leva `0023_task_properties`, **esta leva
  `0024_notes`**. **NÃO** "ler o próximo livre" (sob build paralelo todas leriam o mesmo número e
  colidiriam — aviso do INDEX.md §Migrações M2). INDEX.md: registrar **0024 dona da fase 26**.
- `notes.project_id ON DELETE CASCADE`: apagar um projeto leva as notas do projeto junto (coerente
  com `project_property_defs`/subtasks da 24/25). Nota solta (`project_id NULL`) não é afetada.
- `notes.task_id ON DELETE SET NULL`: apagar a task não apaga a nota que a atrela (a nota vira sem
  task; o `note_links` ref_type='task' some por D-08/limpeza, ou fica e some da resolução por CA-12).
- `note_links.ref_id` **sem FK** (D-08): aponta pra `tasks.id` OU `notes.id` (alvo polimórfico — não
  dá pra ter 1 FK pros dois). A integridade é mantida na **resolução** (CA-12: link cujo alvo sumiu/
  ficou invisível é **omitido**) e numa limpeza best-effort ao criar (sem trigger, sem dep nova).

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Pacote novo `internal/notes`**, NÃO estender `tasks`/`projects` | Notas têm ciclo de vida próprio (CRUD + links). Mantém os pacotes das fases anteriores intocados (RNF-05). O `notes.Service` tem **seu próprio** gate de visibilidade (cópia do JOIN `(p.visibility='shared' OR p.owner_user_id=$u)` — igual `comments.Service` da fase 19; NÃO importa `projects`/`tasks` pra ler). |
| D-02 | **Rotas = subárvore aninhada no workspace, montada como Route IRMÃO** (`/api/v1/workspaces/{ws}/notes[...]`) com `RequireMember`, gate fino no service — espelha **exatamente** o `comments` (fase 19, `auth_routes.go:65`) | O slash `/task` cria a task via `tasks.CreateTx` (aditivo; precisa de membership), mas o **gate de escrita por-nota** (viewer pode LER nota visível, só escreve com `access=edit`) é fino demais pro `RequireWriteRole` blanket. O Route irmão deixa `GET` passar com `RequireMember` e o `POST/PATCH/DELETE` decide no service (como o carve-out viewer-assignee da 18/19). `middleware_role.go` INTOCADO. |
| D-03 | **Gate de visibilidade replicado no `notes` (cópia, sem extração)** — o JOIN `projects p ... (p.visibility='shared' OR p.owner_user_id=$u)` + permissão (dono ∥ owner-ws ∥ `access=edit`) é **copiado** de `tasks/service.go`/`comments/service.go` | Mesma decisão da fase 12 (D-03 alt. B) e 19: ~25 linhas duplicadas > tocar os pacotes da fase 02 num build autônomo. Nota **solta** (`project_id NULL`) **não** passa pelo gate de projeto — é governada por `created_by` (CA-11.1). Um teste de paridade prova mesma resposta pra mesma entrada. |
| D-04 | **Slash `/task` e `/note` são ATÔMICOS** — `CreateLink` abre **uma transação** que (a) cria a task via `tasks` (SQL inline ou chamada que aceita `tx` — ver §5) **e** (b) insere o `note_link`; falha em qualquer ponto → ROLLBACK total | CA-07/08: a UI mostra um chip por link; uma task órfã (criada mas link não inserido) ou um link órfão (inserido mas task falhou) corromperia a nota. A atomicidade é o invariante. **Cuidado registrado** (PRD §2 da tarefa): NÃO criar a task fora da tx e depois postar o link em 2 requests — isso permite estado parcial. |
| D-05 | **`preview` derivado no BACKEND** (não no client) — na listagem, o backend devolve `preview` = `md_strip(body_md)[:140]` | RNF-07/CA-03: derivar no client exigiria mandar `body_md` inteiro de TODAS as notas na lista (payload pesado). O backend faz um strip simples (remove marcações markdown e colapsa espaço — espelha o `md(...).replace(/<[^>]+>/g,' ').slice(0,100)` do mockup linha 459, mas SEM renderizar HTML; é um strip de texto puro) e manda só o trecho. **GET de uma nota** (CA-04) devolve o `body_md` completo. |
| D-06 | **Markdown = parser próprio mínimo `lib/md.ts`** (≈ 30 linhas, espelha o `md()` do mockup, linhas 211-220), **sem dependência nova** | RNF-01 (leveza/anti-NOA). Cobre títulos (`#/##/###`), negrito/itálico/code inline, links, listas, code-block, quebra de linha — exatamente o que o mockup já valida. Uma lib (`marked`/`markdown-it`) traria features que **não queremos** (anti-NOA) e peso de bundle. O parser próprio é testável por unidade (entrada md → HTML esperado) e o `md_strip` do backend é o equivalente em Go (regex stdlib). **Justificativa de leveza:** zero dep, ~30 linhas, escopo travado no que o mockup mostra. |
| D-07 | **Resolução de links omite o que sumiu/ficou invisível, com visibilidade POR TIPO de alvo** (CA-12) — na leitura, cada `note_link` é resolvido por query batch com a visibilidade do user **aplicada conforme o `ref_type`**: `ref_type='task'` → regra de task (`project shared OR owner_user_id=$u`); `ref_type='note'` → regra de **NOTA** (a MESMA do gate T-02), `LEFT JOIN projects` + **`(projeto shared OR project.owner_user_id=$u) OR (sub-nota project_id IS NULL AND created_by=$u)`**. Alvo sem linha (deletado) ou invisível → **omitido** | Não vazar nome de task que virou private de outro (RNF-04 do PRD). **Crítico (26-H2):** para `ref_type='note'` NÃO usar `INNER JOIN projects` cego — isso descartaria toda **sub-nota SOLTA** (`project_id NULL`), fazendo o chip de uma sub-nota que o próprio usuário acabou de criar pelo `/note` numa nota solta (CA-10 permite!) sumir. A sub-nota solta do criador DEVE resolver. Mantém o link no banco (não apaga em leitura), só não o exibe. Resolução sem N+1: 1 query batch dos task-links + 1 dos note-links da página (D-05). |
| D-08 | **`note_links.ref_id` polimórfico SEM FK** (aponta task OU note) | Não dá pra ter uma FK que referencie duas tabelas. A integridade vem da resolução (D-07) + cascade da `note_id`. Cleanup best-effort: ao apagar a entidade alvo, o link fica órfão mas some da resolução (CA-12); um cron/limpeza é YAGNI. **Sobre `uq_note_links` (26-H8):** o slash `/task`/`/note` SEMPRE cria uma entidade NOVA (`ref_id` = uuid fresh via `gen_random_uuid()`) — não há rota nesta fase pra linkar uma entidade EXISTENTE (link-a-existente é fora de escopo, spec §6). Logo a colisão de `(note_id, ref_type, ref_id)` é **inalcançável pela API**; o `uq_note_links` é só **defesa-de-banco best-effort**, NÃO há decisão de produto (409 vs idempotência) a tomar. O único teste do `uq` é de **integridade de schema** (INSERT direto duplicado → erro do banco) na T-01 — NÃO um CA de produto na T-05. |
| D-09 | **Editor salva por DEBOUNCE** (`PATCH` após ≥600ms sem digitar) — não a cada tecla, não num botão Salvar | RNF-02 (sempre editável, sem botão) + não martelar o backend. Padrão "auto-save" comum; o `updated_at` da resposta atualiza a lista. O `lib/slash.ts` é PURO (detecção/aplicação de bloco sem DOM) pra ser testável; o `NoteEditor` só amarra ao textarea. |
| D-10 | **Gating viewer pela `board.project.can_write`** (contrato do `/board` da fase 12/24), não pela role do ws | Igual fase 12 (CA-22): um member com `access=view` num projeto `shared` é `can_write=false`. A aba Notes carrega o board (ou um `can_write` derivado dos dados do projeto) pra decidir "New note"/editor read-only. |
| D-11 | **Slash inserido como TEXTO markdown** (To-do/Heading/Divider) vs **link** (Task/Note) | Mockup linhas 615-622: To-do/Heading/Divider só inserem markdown (sem API). Task/Note chamam a API de link e inserem um marcador (`- [ ] nome` / `[[título]]`) + exibem o chip. Esta separação é o coração do CA-20 × CA-21. |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) · `invalid_reference` (400) — idênticos às fases anteriores.

### 4.1 Notas (CRUD)

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/notes?project_id={id}` | membro que enxerga o projeto (invisível → 404) |
| `GET` | `/notes` (sem project_id) | o usuário (só suas notas soltas — CA-03.1) |
| `GET` | `/notes/{noteId}` | quem enxerga a nota (de projeto: visível; solta: criador) |
| `POST` | `/notes` | escrita no projeto (CA-01); nota solta = qualquer membro (CA-01.2); viewer no projeto → 403 |
| `PATCH` | `/notes/{noteId}` | escrita na nota (projeto: edit; solta: criador) |
| `DELETE` | `/notes/{noteId}` | idem (hard delete + cascade dos links — CA-06) |

`POST .../notes`:

```json
// → request (project_id e task_id opcionais; sem project_id = nota solta — CA-01.2)
{ "project_id": "0d9e...", "title": "Decisões de produto", "task_id": null }

// ← 201
{ "id": "n1...", "workspace_id": "8c...", "project_id": "0d9e...", "task_id": null,
  "title": "Decisões de produto", "body_md": "", "created_by": "me0...",
  "created_at": "2026-06-22T14:00:00Z", "updated_at": "2026-06-22T14:00:00Z" }

// ← 400 título > 200 (CA-01.1)
{ "error": { "code": "validation_error", "message": "título deve ter no máximo 200 caracteres" } }

// ← 400 task_id inválido/invisível no PAYLOAD (CA-02)   ← 400 project_id invisível/de-outro-ws no PAYLOAD (CA-13)
// ← 400 projeto arquivado no gate de escrita (CA-13: "projeto arquivado é só leitura")
// ← 403 viewer/access=view (CA-11)
{ "error": { "code": "invalid_reference", "message": "task_id não encontrada" } }
// nota: ref no PAYLOAD (task_id/project_id) → 400 invalid_reference (igual client_id/objective_id da
// fase 02/12); só o ?project_id= da LISTAGEM (rota) e a nota inexistente respondem 404 not_found (CA-13).
```

`GET .../notes?project_id=` → `200`:

```json
{ "notes": [
  { "id": "n1...", "project_id": "0d9e...", "task_id": null, "title": "Decisões de produto",
    "preview": "Decisões — Cobrança anual vira default. Onboarding em 3 passos.",
    "updated_at": "2026-06-20T10:00:00Z", "created_at": "...",
    "links": [ { "id":"l1","ref_type": "task", "ref_id": "77aa...", "label": "Publicar", "status": "todo", "position": 0 } ] }
] }
```
- `preview` = `md_strip(body_md)[:140]` (D-05); a lista **NÃO** devolve `body_md` inteiro (RNF-07).
- `links` resolvidos com `label`/`status` (CA-12); link cujo alvo sumiu/invisível é **omitido** (D-07).
- Ordem por `updated_at desc` (CA-03). Listas vazias = `[]` não-nil.

`GET .../notes/{noteId}` → `200` o objeto **completo** (com `body_md` e `links`). `404` se invisível/
malformado (CA-04). `PATCH .../notes/{id}` aceita `{title?, body_md?}` (tri-state ausente×presente;
`body_md:''` limpa, é válido — CA-05); `updated_at=now()`. `DELETE` → `204` (CA-06).

### 4.2 Links — slash (CA-07/08/09/10)

| Método | Rota | Efeito |
|---|---|---|
| `POST` | `/notes/{noteId}/links` | cria entidade + link, atômico (CA-07/08) |
| `DELETE` | `/notes/{noteId}/links/{linkId}` | remove só o link (entidade sobrevive — CA-09) |

`POST .../notes/{noteId}/links`:

```json
// → /task: cria task no project_id da nota + link  (CA-07)
{ "kind": "task", "name": "Publicar OKRs" }
// ← 201   (status = 'todo' — o DEFAULT do enum; o slash NÃO crava 'backlog' — coordenação C)
{ "link": { "id": "l1", "ref_type": "task", "ref_id": "77aa...", "position": 1 },
  "task": { "id": "77aa...", "project_id": "0d9e...", "name": "Publicar OKRs", "status": "todo", ... } }

// → /note: cria sub-nota (mesmo projeto da mãe) + link  (CA-08)
{ "kind": "note", "title": "Sub-detalhes" }
// ← 201
{ "link": { "id": "l2", "ref_type": "note", "ref_id": "n9...", "position": 2 },
  "note": { "id": "n9...", "project_id": "0d9e...", "title": "Sub-detalhes", "body_md": "", ... } }

// ← 400 kind inválido (CA-08.1)          ← 400 nota solta + kind:task (CA-10)
{ "error": { "code": "validation_error", "message": "nota solta não tem projeto para a task" } }
// ← 403 viewer (CA-11)                   ← 404 nota invisível (CA-13/14)
```
- **Atômico** (D-04): a task/sub-nota + o link nascem na MESMA transação; falha → rollback total.
- `status` da task = o **DEFAULT do enum (`todo`)** — o slash NÃO crava `backlog` (coordenação C); a
  task é criada via `tasks.CreateTx`, que não passa status (a coluna fica no default `todo`).
  `position` do link = `max(position)+1`.
- `DELETE .../links/{linkId}` → `204`; a task/sub-nota **permanece** (CA-09). Link inexistente /
  `linkId` malformado / link de outra nota → `404` (forma de rota ÚNICA — 26-H6; sem a variante
  `{ref_type}/{ref_id}`).

### 4.3 `task_id` na criação de task pelo slash

A task criada pelo `/task` nasce **sem** objetivo (`objective_id=null` → bucket "Geral" da fase 12),
**sem** assignee/due/recurrence/notes, com **status `todo`** (o DEFAULT do enum — coordenação C; o
slash NÃO crava `backlog`) e **herda o `project_id` da nota** (a nota é de projeto — CA-07; nota
solta + task → 400, CA-10). É criada via **`tasks.CreateTx`** (método ADITIVO da T-04 — coordenação
D; NÃO refatora `tasks.Create`, NÃO duplica a validação de nome/visibilidade — §5).

### 4.4 `task_id` no POST de NOTA (atrelar nota a task) — gate efetivo (26-H3)

Como `tasks.project_id` é **NOT NULL** (toda task tem projeto — D-08), um `POST .../notes` com
`task_id` (sem `project_id` no payload) **sempre** herda o `project_id` da task e a nota NASCE como
nota de **projeto** (nunca solta). O serviço DEVE: (1) resolver o `task_id` (visibilidade + mesmo ws;
inválido → 400 `invalid_reference`, CA-02) e herdar o `project_id` **antes** de decidir o gate;
(2) rodar o **gate de ESCRITA** contra o `project_id` **EFETIVO** (payload OU herdado da task), NÃO
contra a presença de `project_id` no payload. Assim um viewer/`access=view` que só enxerga a task
recebe `403 forbidden` e nada persiste (CA-02.1/CA-11) — não dá pra burlar o gate via `task_id`.

## 5. Backend — assinaturas

```go
// internal/notes
type Note struct {
    ID, WorkspaceID string
    ProjectID *string `json:"project_id"`   // null = solta
    TaskID    *string `json:"task_id"`      // null = não atrelada
    Title, BodyMD string
    Preview   string     `json:"preview,omitempty"`  // só na LISTA (D-05)
    Links     []NoteLink `json:"links"`               // resolvidos (CA-12)
    CreatedBy string
    CreatedAt, UpdatedAt time.Time
}
type NoteLink struct {
    ID, RefType, RefID string
    Label  string `json:"label"`            // nome da task / título da nota (CA-12)
    Status string `json:"status,omitempty"` // só p/ ref_type=task
    Position int
}
type CreateInput struct { ProjectID, TaskID *string; Title string }
type PatchInput struct { TitleSet bool; Title *string; BodySet bool; BodyMD *string }

func NewService(db *pgxpool.Pool, tasks *tasks.Service) *Service  // recebe tasks pro slash /task (D-04)
func (s *Service) List(ctx, ws, userID, projectID string) ([]Note, error)            // CA-03 (projectID=="" → soltas, CA-03.1)
func (s *Service) Get(ctx, ws, userID, noteID string) (Note, error)                  // CA-04
func (s *Service) Create(ctx, ws, userID, role string, in CreateInput) (Note, error) // CA-01/01.2/02/11/13
func (s *Service) Patch(ctx, ws, userID, role, noteID string, in PatchInput) (Note, error) // CA-05/05.1/11/14
func (s *Service) Delete(ctx, ws, userID, role, noteID string) error                 // CA-06/11/14
func (s *Service) CreateLink(ctx, ws, userID, role, noteID, kind, name string) (LinkResult, error) // CA-07/08/08.1/10
func (s *Service) DeleteLink(ctx, ws, userID, role, noteID, linkID string) error     // CA-09

// gate interno (cópia, D-03): loadNoteGate(ctx, ws, userID, role, noteID, blockArchived) resolve,
// por uma query:
//   - nota de PROJETO: visível (visibility='shared' OR owner_user_id=$u)→404; escrita (dono ∥ owner-ws
//     ∥ access=edit)→403; projeto arquivado SÓ bloqueia quando blockArchived==true → 400
//     invalid_reference "projeto arquivado é só leitura" (espelha tasks/service.go:240 — NÃO existe
//     código 'archived'; 26-H7/G).
//   - nota SOLTA (project_id NULL): created_by==$u→ok; senão 404 (CA-11.1).
//   uuid.Validate(noteID) antes do SQL → 404 nunca 500.
//
// blockArchived POR OPERAÇÃO (26-H7, espelha o precedente real):
//   Create=true · Patch=true  → escrever conteúdo em projeto arquivado dá 400 invalid_reference
//   Delete=false              → apagar nota de projeto arquivado é PERMITIDO → 204 (tasks.Delete
//                               service.go:609 loadProjectGate(...,false); comments.Delete idem).
//
// Create com task_id (sem project_id no payload) — 26-H3/CA-02.1: resolve o task_id (visibilidade +
//   mesmo ws senão 400 invalid_reference), herda o project_id da task (tasks.project_id é NOT NULL →
//   nota vira de PROJETO, nunca solta), e roda o gate de ESCRITA contra esse project_id EFETIVO
//   (viewer/access=view → 403, mesmo enxergando a task). NÃO decidir o gate por "tem project_id no
//   payload?", e sim por "qual o project_id EFETIVO (payload OU herdado da task)?".
//
// CreateLink (D-04 — ATÔMICO): BEGIN;
//   carrega a nota pelo gate de ESCRITA (blockArchived=true); se kind=='task':
//     project_id da nota NULL → ROLLBACK + Validation (CA-10);
//     task := tasks.CreateTx(ctx, tx, ws, userID, role, project_id_da_nota, name) // aditivo (coord. D);
//             status = DEFAULT 'todo' (coord. C; CreateTx NÃO recebe status); SEM objective/assignee/due.
//   if kind=='note': cria sub-nota (mesmo project_id da mãe, ou NULL se mãe é solta — CA-10)
//   else: ROLLBACK + Validation (CA-08.1)
//   INSERT note_links(note_id, ref_type=kind, ref_id=entidade.id, position=max+1, workspace_id=ws);
//   COMMIT;  // falha em qualquer passo → rollback total (sem órfão)
```

- **Validação (CA-01.1/05.1)** no `Create`/`Patch` ANTES do SQL → `domain.Validation`. `title` trim +
  len ≤ 200; `body_md` sem limite (D-06).
- **Topologia de import:** `notes` **importa `tasks`** (pro slash `/task`, D-04) — uma direção, sem
  ciclo (`tasks` NÃO importa `notes`). A fase **adiciona** `tasks.CreateTx(ctx, tx pgx.Tx, ws,
  userID, role, projectID, name string) (Task, error)` — método **ADITIVO** (coordenação D) que
  aceita uma `pgx.Tx` já aberta e delega à **MESMA lógica de insert** do `tasks.Create` final da
  fase 24, fazendo SOMENTE `validName` + `loadProjectGate(blockArchived=true)` + INSERT na tx (status
  no DEFAULT `todo` — coord. C). **NÃO** refatora/reescreve `tasks.Create` (que fica byte-idêntico) e
  **NÃO** usa a alternativa "INSERT inline replicando a validação" (descartada — duplicaria a regra e
  divergiria). O `/task` do slash não precisa de objective/assignee/due/recurrence/notes/tz, nem de
  `recalcObjectiveStatus`/`reminder.OnDueDateSet`/`AssignNotify` — nada disso se aplica (§4.3), então
  o `CreateTx` é enxuto e os invariantes pesados do `Create` (que continuam só no `Create`) não
  correm risco de omissão (26-H4). Construído SOBRE a assinatura final que a 24 deixar.
- **Slash `/task` reusa o gate de escrita do projeto** (a task nasce no projeto da nota; o gate da
  nota já garantiu escrita no projeto). **NÃO** revalidar duplicado. (O `CreateTx` ainda roda o
  `loadProjectGate` por segurança, mas é o mesmo projeto/role já validado pela nota.)

## 6. Frontend — telas e componentes

Tokens de `tokens.css`. Reusa o **tab-shell** e o **componente de popup central `CenterPanel`** da
fase 24 (coordenação A — popup central expansível side↔full via tokens.css; NÃO classes cruas do
mockup `.mscrim`/`.scard`/`.swrap`). Markdown via `lib/md.ts` (parser próprio — D-06). Lógica do
slash em `lib/slash.ts` (puro). Fonte Inter (fase 24).

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Aba Notes | `components/notes/NotesTab.tsx` | tabela `Note·Preview·Updated` (classe `.ntable` do mockup) + "New note" gated por `can_write` (CA-16/22); vazio = "No notes yet." |
| Painel da nota | `components/notes/NotePanel.tsx` | dentro do componente `CenterPanel` da 24 (coord. A; chip "Note · .md"); compõe `NoteEditor`; expande/colapsa pela 24 |
| Editor | `components/notes/NoteEditor.tsx` | title grande + textarea md sempre editável (`.noteedit`); chips de link no topo; debounce PATCH (CA-17/18); viewer → readOnly (CA-22) |
| Menu slash | `components/notes/SlashMenu.tsx` | Task·Note·To-do·Heading 1·Heading 2·Divider; ↑/↓/Enter/Esc (CA-19); render via tokens (`.slashmenu`/`.slashitem`) |
| Parser md | `lib/md.ts` | espelha `md()` do mockup (linhas 211-220) — D-06 |
| Slash (puro) | `lib/slash.ts` | `detectSlash(value, caret)` → `{query}` \| null; `applyBlock(value, caret, cmd)` → `{value, caret}` (insere `- [ ] `/`# `/`## `/`\n---\n` ou marcador de link) — CA-20/21, testável sem DOM |
| API + hooks | `lib/api.ts` (estende), `lib/queries/notes.ts` (novo) | `useNotes(projectId)`/`useNote(id)` + mutações (invalidam `notesKey`); link mutations invalidam a nota + o board (a task nova aparece em Tasks — CA-23) |

- `useNotes(projectId)` é a query da lista; mutações de nota/link invalidam `notesKey(ws, projectId)`.
  A criação de **task** pelo slash invalida **também** `boardKey(ws, projectId)` (a task aparece na
  aba Tasks da 24 — CA-23) via `onSuccess` local (não mexe em `lib/queries/tasks.ts` — padrão fase 12).
- Gating viewer (CA-22): lê `board.project.can_write` (contrato da 12/24). Editor de viewer abre
  `readOnly`, sem slash. Padrão de teste `viewer-gating.spec` (com `can_write` mockado).
- O **debounce** (D-09) é client-side; o teste de componente usa fake timers pra provar o PATCH
  após 600ms e a ausência de PATCH antes.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** criar a task fora da transação e postar o link depois (D-04) — quebra a atomicidade do
  CA-07/08; um request, uma transação.
- **NÃO** permitir `/task` em nota solta (CA-10) — sem projeto não há onde ancorar a task → `400`.
- **NÃO** apagar a task/sub-nota ao remover o link (CA-09) — `DELETE /links/{id}` remove só o
  `note_link`; a entidade sobrevive.
- **NÃO** apagar os links ao deletar a entidade alvo via trigger/cascade cruzado (D-08) — o `ref_id`
  é polimórfico, sem FX; o link órfão some da **resolução** (CA-12), não do banco.
- **NÃO** vazar nome de task que virou invisível (CA-12/D-07) — a resolução de links aplica a
  visibilidade do user e **omite** o alvo invisível.
- **NÃO** mandar `body_md` inteiro de todas as notas na listagem (RNF-07/D-05) — manda só `preview`
  derivado no backend; `body_md` completo só no `GET /notes/{id}`.
- **NÃO** governar nota solta pela role do workspace (CA-11.1) — nota solta é do `created_by`; outro
  user (mesmo owner do ws) recebe `404`.
- **NÃO** deixar o slash criar blocos na **descrição da task** (PRD §9.4) — o `/` é SÓ nas Notas; o
  `tasks.notes` (descrição, fase 24) é markdown simples sem menu.
- **NÃO** trazer lib de editor rico (ProseMirror/TipTap/Slate) nem lib de markdown pesada (RNF-01/
  PRD §8) — `lib/md.ts` próprio + textarea.
- **NÃO** rodar slash `/goal`/`/timer` dentro da nota (spec §6) — esses são do Spotlight (fase 27).
- **NÃO** vazar entre workspaces (CA-14) — `WHERE workspace_id` + visibilidade em TODA query; teste
  de mutação obrigatório.
- **NÃO** importar `notes` de dentro de `tasks` (§5) — a direção é `notes → tasks` (uma só).
- **NÃO** "ler o próximo número de migração livre" (RNF-04) — `0024_notes` é fixo do PRD §7.
- **NÃO** reescrever o tab-shell/`CenterPanel` da fase 24 (RNF-05) — só registrar a aba "Notes" e o
  conteúdo de painel `note` dentro do `CenterPanel` (coord. A).
- **NÃO** refatorar/reescrever `tasks.Create` (26-H4/coord. D) — o slash usa o `tasks.CreateTx`
  ADITIVO; `tasks.Create` fica byte-idêntico e sua suíte continua verde.
- **NÃO** cravar `status:'backlog'` na task do slash (26-H1/coord. C) — a task nasce no DEFAULT `todo`.

## 8. Dependências & paralelização

- **Depende de:** fase 24 (tab-shell do projeto + componente `CenterPanel` + a forma final de
  `tasks.Create` sobre a qual o `CreateTx` é construído). A 24 adicionou `'backlog'` ao enum mas
  **manteve o DEFAULT da coluna em `todo`** — o slash usa esse default (coord. C; NÃO crava backlog).
  Build order do PRD: **24 → 26**. O **backend de notas é independente** da 24 (não precisa de nada
  novo da 24 no backend — só do `tasks.CreateTx` ADITIVO que esta fase mesmo adiciona sobre o
  `tasks.Create` final). O **frontend depende da 24** (o editor reusa o `CenterPanel`; a aba entra no
  tab-shell).
- **Paralelização (worktrees):** o **backend** (T-01→T-06) é **worktree-isolável** (pacote novo
  `internal/notes` + migração `0024` + Route irmão em `auth_routes.go`). O **único arquivo
  compartilhado** com outras fases da M3 é `auth_routes.go` (1 bloco aditivo, como `comments`) e
  `App.tsx`/`ProjectDetail.tsx` (registrar a aba) — ponto de integração, 1 hunk. **NÃO** paralelizar
  o frontend com a 24 (depende do tab-shell dela). Migração `0024` FIXA evita colisão de número.
- **Reusa (não recria):** `comments.Service` (fase 19) como **modelo** do gate de visibilidade +
  Route irmão; `tasks.CreateTx` ADITIVO sobre o `tasks.Create` final (fase 02/12/24) pro slash;
  componente `CenterPanel` + tab-shell (fase 24); `board.project.can_write` (fase 12) pro gating;
  harness `coretest`/`e2e/helpers.ts`.
- **Não depende de:** fase 25 (propriedades tipadas) nem 27 (Spotlight) nem 28/29.

## 9. Ordem de execução (ver tasks.md)

Backend primeiro (migração → gate + CRUD → links atômicos → handlers/wiring), depois frontend
(api/queries → `md`/`slash` puros → editor + slash + aba → registro no tab-shell), depois E2E e gate.
Commit por tarefa. `make check-full` é o gate final.
