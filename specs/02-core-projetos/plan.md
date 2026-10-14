# plan.md — Fase 02 · Domínio core (clientes → projetos → tasks)

> Como implementar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7` (topologia, pastas,
> persistência, auth da fase 01). Nada aqui é código — é o contrato que a implementação segue.

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Rotas aninhadas por workspace**: `/api/v1/workspaces/{workspaceId}/clients...` | Torna o multi-tenant explícito na URL; o middleware de membership valida 1 vez e injeta `workspace_id` no contexto — impossível "esquecer" o filtro (CA-26). Prepara múltiplos workspaces por usuário sem mudar contrato. |
| D-02 | **Cadeia de middlewares**: `RequireAuth` + `RequireMember` (ambos **prontos da fase 01** — esta fase só os consome, não os reescreve) → **role gate novo** | Sessão e membership já existem (fase 01, T-09/T-12 de lá): `RequireAuth` devolve `401` sem sessão (CA-25) e `RequireMember` devolve `404` pra não-membro e injeta `workspace_id`+role no contexto (CA-26). O único middleware **novo** desta fase é o role gate, que barra `viewer` em métodos de escrita com `403` (CA-23) num ponto único. |
| D-03 | **Soft-delete via `archived_at`** pra clientes e projetos; **hard delete** pra tasks | Clientes/projetos serão referenciados por time entries (fase 03) e relatórios (fase 07) — apagar quebraria histórico. Task sem entry não tem histórico a preservar; task com entry futura terá `ON DELETE SET NULL` na fase 03 (schema canônico: `time_entries.task_id` nullable). |
| D-04 | **Visibilidade resolvida na query do repositório** (`visibility='shared' OR owner_user_id=$user`) e não em filtro pós-fetch — **sem caso especial pro owner do workspace**: privado de outro usuário é `404` pra ele também (CA-09, privacidade absoluta) | Um único lugar pra regra (CA-09/10/22); impossível um handler novo esquecer; barato no Postgres com índice. Recurso invisível e inexistente retornam o mesmo `404`. |
| D-05 | **Permissões de escrita em 2 níveis, sempre condicionadas à visibilidade (D-04)**: metadados de projeto = dono do projeto ou owner do workspace (este só em projeto visível a ele — em privado de outro, `404` antes de qualquer checagem de permissão); tasks = esses + membro do projeto com `access='edit'`; `access='view'` e membro do ws fora de `project_members` = `403` em escrita de task (CA-16.1) | Espelha DESIGN-additions ("Editar × Só vê" é sobre participar do projeto, não sobre administrá-lo). Mantém a base multiplayer útil sem inventar colaboração real (CA-14/16/16.1). |
| D-06 | **`PUT` substitutivo em `/projects/{id}/members`** (lista completa) em vez de POST/DELETE individuais | O modal `newproject.jsx` edita a lista inteira de uma vez (checkboxes + acesso); 1 endpoint, 1 transação, 0 estados intermediários (CA-11). |
| D-07 | **Validação na mão (stdlib)**: trim+len pra nomes, regex `^#[0-9a-fA-F]{6}$` pra cor, `time.Parse("2006-01-02")` pra due_date, switch pra enums | Leveza é requisito (REQUISITOS.md §6); uma lib de validação inteira pra 6 regras não se justifica. |
| D-08 | **`recurrence` é texto opaco (≤255)** nesta fase — sem motor de recorrência | Schema canônico define "texto/rrule"; interpretar recorrência é trabalho da fase 03+ (geração de tasks). Aqui só persiste/devolve (CA-20). |
| D-09 | **`tasks.workspace_id` denormalizado** (além do `project_id`) com checagem de consistência no service | Schema canônico exige; permite filtro tenant direto em `GET /tasks` sem join e mantém CA-26 trivial de garantir e testar. |
| D-10 | **Visibilidade obrigatória no `POST /projects`** (sem default no servidor) | Evita decidir agora o default global ("projetos novos são compartilhados" do `workspace.jsx` ficou fora da fase); a UI sempre envia (modal pré-seleciona como no protótipo). |
| D-11 | Front: **visão cartões default + tabela `.tt-table` sob toggle** nas DUAS páginas (Clientes e Projetos) | Regra canônica do DESIGN-additions ("tabelas só sob seleção do usuário"). O `projects.jsx` prototipa uma lista densa — ela vira a **visão tabela** da página; o cartão de projeto reaproveita seus elementos (régua de cor, `.tt-goal`). |
| D-12 | Front: estado servidor via **TanStack Query** + cliente HTTP fino próprio (fetch) | Baseline SDD §7.2 já prevê "query"; invalidação de query resolve CA-29 sem store manual. Zustand fica pra estado de timer (fase 03) — esta fase não precisa. |

## 2. Modelo de dados — migrações desta fase

Diretório: `backend/internal/db/migrations/` (aplicadas no boot). A numeração `NNNN` continua a
sequência existente ao fim da fase 01: `0001_init.sql` (fase 00) + `0002_users.sql` …
`0005_oauth_tokens.sql` (fase 01) → esta fase começa em **`0006`** (**conferir o diretório real
antes de criar** e renumerar se preciso).

### `0006_clients.sql`

```sql
CREATE TABLE clients (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name         text NOT NULL,
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_clients_workspace ON clients (workspace_id) WHERE archived_at IS NULL;
```

### `0007_projects.sql` (projects + project_members)

```sql
CREATE TYPE project_visibility AS ENUM ('private', 'shared');
CREATE TYPE project_access     AS ENUM ('edit', 'view');

CREATE TABLE projects (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL REFERENCES workspaces(id),
    client_id         uuid REFERENCES clients(id),
    name              text NOT NULL,
    color             text NOT NULL,                      -- '#RRGGBB'
    weekly_hours_goal numeric(5,2),                       -- NULL = sem meta; (0,168]
    visibility        project_visibility NOT NULL,
    owner_user_id     uuid NOT NULL REFERENCES users(id),
    archived_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_projects_workspace ON projects (workspace_id) WHERE archived_at IS NULL;

CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id),
    access     project_access NOT NULL,
    PRIMARY KEY (project_id, user_id)
);
```

### `0008_tasks.sql`

```sql
CREATE TYPE task_status AS ENUM ('todo', 'doing', 'done');

CREATE TABLE tasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    project_id   uuid NOT NULL REFERENCES projects(id),
    name         text NOT NULL,
    status       task_status NOT NULL DEFAULT 'todo',
    due_date     date,
    recurrence   text,                                    -- texto livre / RRULE, opaco nesta fase
    notes        text NOT NULL DEFAULT '',
    created_by   uuid NOT NULL REFERENCES users(id),
    assignee     uuid REFERENCES users(id),               -- base multiplayer, sem UI no MVP
    requested_by uuid REFERENCES users(id),               -- base multiplayer, sem UI no MVP
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_tasks_workspace ON tasks (workspace_id);
CREATE INDEX idx_tasks_project   ON tasks (project_id, status);
```

Notas: dono do projeto **não** entra em `project_members` (é implícito — D-06/CA-11).
`weekly_hours_goal` em horas decimais (ex.: `10.5`). Fase 03 referenciará `projects.id` e
`tasks.id` a partir de `time_entries` — nada aqui bloqueia isso.

## 3. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão httpOnly (fase 01).
Erros sempre `{"error":{"code":"...","message":"..."}}` com códigos:
`unauthorized` (401) · `forbidden` (403) · `not_found` (404) · `validation_error` (400) ·
`invalid_reference` (400).

### 3.1 Clientes

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/clients?archived=true\|false` | qualquer membro |
| `POST` | `/clients` | owner, member |
| `PATCH` | `/clients/{clientId}` | owner, member |
| `DELETE` | `/clients/{clientId}` | owner, member (arquiva) |

`POST /clients` — request/response:

```json
// → POST /api/v1/workspaces/8c.../clients
{ "name": "Acme" }

// ← 201
{ "id": "f2b1...", "workspace_id": "8c...", "name": "Acme",
  "archived_at": null, "created_at": "2026-06-10T14:03:00Z" }

// ← 400 (CA-02)
{ "error": { "code": "validation_error", "message": "nome deve ter entre 1 e 120 caracteres" } }
```

`GET /clients` → `200 { "clients": [ {…}, {…} ] }` (sem arquivados por default — CA-03).
`PATCH` body `{ "name": "Acme Tech" }` → `200` com objeto. `DELETE` → `204` (CA-05).

### 3.2 Projetos

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/projects?archived=...` | membro (vê shared + os próprios private — CA-09/10) |
| `GET` | `/projects/{projectId}` | idem (invisível → 404) |
| `POST` | `/projects` | owner, member |
| `PATCH` | `/projects/{projectId}` | dono do projeto ou owner do workspace, só em projeto visível ao requisitante — privado de outro = 404 (CA-09/14) |
| `DELETE` | `/projects/{projectId}` | idem (arquiva — CA-15) |
| `PUT` | `/projects/{projectId}/members` | idem (CA-11) |

Envelopes de leitura: `GET /projects` → `200 { "projects": [ {…}, {…} ] }` (objetos **sem**
`members` na listagem); `GET /projects/{id}` → `200` com o objeto do projeto **incluindo**
`"members": [ { "user_id", "access" } ]` embutido (vazio em projeto privado). Projeto arquivado:
detalhe `200` com `archived_at` preenchido (CA-15.1); listagem só com `?archived=true`.

`POST /projects`:

```json
// → request (visibility obrigatório — D-10; client_id, weekly_hours_goal, members opcionais —
//   members só em shared (CA-11.1/13); persiste na mesma transação da criação)
{ "name": "Acme — App mobile", "color": "#4f9dde", "client_id": "f2b1...",
  "weekly_hours_goal": 18, "visibility": "shared",
  "members": [ { "user_id": "a11...", "access": "edit" },
               { "user_id": "b22...", "access": "view" } ] }

// ← 201
{ "id": "0d9e...", "workspace_id": "8c...", "client_id": "f2b1...",
  "name": "Acme — App mobile", "color": "#4f9dde", "weekly_hours_goal": 18,
  "visibility": "shared", "owner_user_id": "me0...", "archived_at": null,
  "created_at": "2026-06-10T14:05:00Z",
  "members": [ { "user_id": "a11...", "access": "edit" },
               { "user_id": "b22...", "access": "view" } ] }

// ← 400 cor inválida (CA-07)
{ "error": { "code": "validation_error", "message": "color deve estar no formato #RRGGBB" } }

// ← 400 cliente de outro workspace (CA-08)
{ "error": { "code": "invalid_reference", "message": "client_id não pertence a este workspace" } }

// ← 400 membros em projeto privado (CA-13)
{ "error": { "code": "validation_error", "message": "projeto privado não tem membros" } }
```

`PUT /projects/{id}/members`:

```json
// → substitui a lista inteira (dono é implícito, não entra)
{ "members": [ { "user_id": "a11...", "access": "view" } ] }
// ← 200
{ "members": [ { "user_id": "a11...", "access": "view" } ] }
// ← 400 user fora do workspace (CA-12)
{ "error": { "code": "invalid_reference", "message": "user_id não é membro do workspace" } }
```

`PATCH /projects/{id}` aceita qualquer subconjunto de
`{name, color, client_id, weekly_hours_goal, visibility}` (`client_id: null` e
`weekly_hours_goal: null` removem). `visibility: "private"` apaga as linhas de
`project_members` na mesma transação (CA-13.1). `client_id` de cliente arquivado → `400`
`invalid_reference` (CA-08.1). `DELETE` → `204`.

### 3.3 Tasks

| Método | Rota | Quem pode |
|---|---|---|
| `GET` | `/tasks?project_id=&status=` | membro (só tasks de projetos visíveis — CA-22) |
| `GET` | `/tasks/{taskId}` | idem |
| `POST` | `/tasks` | dono do projeto, owner do ws, ou membro do projeto com `access=edit` (CA-16) |
| `PATCH` | `/tasks/{taskId}` | idem |
| `DELETE` | `/tasks/{taskId}` | idem (hard delete — CA-21) |

Envelopes de leitura: `GET /tasks` → `200 { "tasks": [ {…}, {…} ] }`; `GET /tasks/{id}` → `200`
com o objeto da task (mesmo shape do `201` abaixo). Tasks de projeto **arquivado** continuam
legíveis (lista e detalhe — CA-15.1), mas escrita (`POST`/`PATCH`) referenciando projeto
arquivado → `400` `invalid_reference` (CA-15.2). Escrita por `access=view` ou por membro do ws
fora de `project_members` → `403` (CA-16.1).

```json
// → POST /api/v1/workspaces/8c.../tasks
{ "project_id": "0d9e...", "name": "Definir fluxo de onboarding",
  "due_date": "2026-06-15", "recurrence": "FREQ=WEEKLY;BYDAY=MO", "notes": "ver Figma" }

// ← 201 (status default, created_by da sessão — CA-16/20)
{ "id": "77aa...", "workspace_id": "8c...", "project_id": "0d9e...",
  "name": "Definir fluxo de onboarding", "status": "todo", "due_date": "2026-06-15",
  "recurrence": "FREQ=WEEKLY;BYDAY=MO", "notes": "ver Figma",
  "created_by": "me0...", "assignee": null, "requested_by": null,
  "created_at": "2026-06-10T14:08:00Z" }

// ← 404 projeto privado de outro / inexistente (CA-17 — resposta idêntica)
{ "error": { "code": "not_found", "message": "projeto não encontrado" } }

// → PATCH /tasks/77aa...   { "status": "doing" }        ← 200 (CA-18)
// → PATCH /tasks/77aa...   { "status": "feito" }        ← 400 validation_error (CA-18)
// → DELETE /tasks/77aa...                               ← 204 (CA-21)
```

## 4. Libs

| Camada | Lib | Justificativa |
|---|---|---|
| Router Go | `go-chi/chi/v5` | Já é baseline (SDD §7.2); rotas aninhadas + middleware por subrouter. |
| Postgres Go | `jackc/pgx/v5` (direto, sem ORM) | Baseline §7.4; queries explícitas deixam o filtro `workspace_id` auditável. Sem sqlc nesta fase (poucas queries; avaliar na 03). |
| UUID Go | `google/uuid` | Geração/parse de ids nos testes e handlers; mínima. |
| Testes Go | stdlib `testing` + `net/http/httptest` (+ `testify/require` se ajudar) | TDD.md; Postgres efêmero (container/schema de teste) pros testes de domínio. |
| Front | TanStack Query, `react-router` | D-12; rotas do cockpit. **Nenhuma lib de UI** — componentes próprios sobre `tokens.css`. |
| Testes front | Vitest (+ Testing Library) e Playwright | TDD.md §2. |

Validação, JSON e regex: **stdlib Go** (`encoding/json`, `regexp`, `time`) — D-07.

## 5. Frontend — telas e componentes

Tokens de `Design/Timmingtrack.life/timetrack/tokens.css` mapeados no Tailwind (fase 00). Fontes
Geist/Geist Mono **self-hosted** (o `@import` do Google Fonts do tokens.css é só do protótipo —
não vai pra prod). Sidebar canônica `.tt-sidenav` (rail 64px → overlay 232px no hover) já vem do
baseline; estas páginas só se registram nela.

| Item | Arquivo novo (em `frontend/src/`) | Referência de design | Tokens/classes |
|---|---|---|---|
| Página Clientes | `routes/cockpit/Clients.tsx` (substitui o stub da fase 00, mesmo arquivo) | `clients.jsx` | header com `.eyebrow` + h1, toggle Cartões/Tabela, `.tt-btn--accent` "Novo cliente" |
| Cartão de cliente | `components/clients/ClientCard.tsx` | `clients.jsx` (`ClientCard`) — **sem** contato/valor-h/status (fora do schema) | `.tt-card`, `.tt-chip` + `.tt-dot` pros projetos do cliente |
| Tabela de clientes | `components/clients/ClientsTable.tsx` | `clients.jsx` (`ClientsTable`) | `.tt-table` (th eyebrow, `.num` mono tabular), dentro de `.tt-panel` |
| Página Projetos | `routes/cockpit/Projects.tsx` (substitui o stub da fase 00, mesmo arquivo) | `projects.jsx` + regra D-11 | mesmo padrão de header/toggle de `clients.jsx` |
| Cartão de projeto | `components/projects/ProjectCard.tsx` | elementos do `ProjRow` (régua de cor 4×30px, contagem de tasks, meta) em formato cartão + lista/criação de tasks no detalhe (CA-30.1) | `.tt-card`, `.tt-goal` (CA-30: fill = cor do projeto, accent ao atingir 100%), pill Ativo/Arquivado com `.tt-dot` (derivado de `archived_at` — "Pausado" do protótipo fica fora, spec §6) |
| Tabela de projetos | `components/projects/ProjectsTable.tsx` | colunas do `projects.jsx` adaptadas ao schema (spec §6): Projeto/Cliente/Esta semana (`0h`/`—` até a fase 03)/Total (`—`)/Status (Ativo/Arquivado via `archived_at`) | `.tt-table` |
| Modal Novo/Editar projeto | `components/projects/ProjectModal.tsx` | `newproject.jsx` (campos, paleta `--p-*`, toggle Privado/Compartilhado, lista de membros com checkbox + Editar/Só vê, Owner fixo `.tt-role--owner`) | `.tt-card` + `--shadow`, inputs `--surface-2`/`--border-2`, `.tt-btn--accent` |
| Chip de projeto + barrinha | `components/projects/ProjectChip.tsx` | DESIGN.md §3.1 ("chip com cor; meta com barrinha") | `.tt-chip`, `.tt-dot`, `.tt-goal` |
| Toggle de visão reutilizável | `components/ui/ViewToggle.tsx` | segmented control de `clients.jsx` | `--surface-3`/`--surface-1` |
| API client + hooks | `lib/api.ts` (**extensão** do client criado na fase 01 — não recriar), `lib/queries/{clients,projects,tasks}.ts` (novos) | — | — |

Persistência do toggle (CA-27): chave `tt.view.clients` / `tt.view.projects` no localStorage.
Gating de viewer (CA-31): role do usuário no workspace vem da sessão (fase 01 expõe
`GET /api/v1/me` com workspaces+role); hook `useRole()` esconde ações de escrita.
Tasks nesta fase aparecem **dentro do detalhe/cartão do projeto** (lista simples com status,
data, notas + criação inline — CA-30.1) — sem página própria; UI de assignee/requested_by
**não existe** (fora do MVP). Barrinha `.tt-goal`: fill na **cor do projeto**, trocando pra
`var(--accent)` quando ≥100% (regra do `ProjRow` — CA-30).

## 6. Estrutura tocada (SDD.md §7.2)

```
backend/internal/clients/     service + repo + validação (CA-01..05)
backend/internal/projects/    service + repo + visibilidade + members (CA-06..15)
backend/internal/tasks/       service + repo (CA-16..22)
backend/internal/api/         handlers REST + role gate novo (CA-23; RequireAuth/RequireMember reusados da fase 01 — CA-25/26)
backend/internal/db/migrations/0006..0008_*.sql
frontend/src/routes/cockpit/  Clients.tsx · Projects.tsx
frontend/src/components/      clients/ · projects/ · ui/ViewToggle.tsx
frontend/src/lib/             api.ts · queries/
```

## 7. Integração com fases vizinhas

- **← Fase 01 (depende):** middleware de sessão, `users`, `workspaces`, `workspace_members`
  (roles `owner/member/viewer`) e `GET /me`. Esta fase só **consome** — não cria migração de
  auth.
- **→ Fase 03 (cronômetro):** `time_entries` referenciará `project_id`/`task_id` daqui; o
  service de tasks fica reutilizável (criar task fora do handler HTTP) pro "continuar entrada"
  e pra fase 08. A barrinha `.tt-goal` passa a receber horas reais da semana (endpoint de
  agregação da 03/07) — nesta fase o componente recebe `hours` por prop e a página passa `0`.
- **→ Fase 07 (relatórios):** `weekly_hours_goal` alimenta "meta vs. realizado"; `client_id`
  alimenta "tempo por cliente/projeto".
- **→ Fase 08 (conector Claude):** criação de task+projeto via service desacoplado do cookie de
  sessão — o token `tt_` da 08 só pluga outro middleware de auth na frente dos mesmos handlers.
  Nada nesta fase pode amarrar regra de negócio ao cookie.
- **Fora daqui:** WebSocket (04), Modo Foco (05), GCal (06), deploy real (09).
