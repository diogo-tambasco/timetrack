# plan.md — Fase 25 · Propriedades tipadas ("database por projeto")

> Como construir a `spec.md` desta pasta. Decisões de design (D-xx) + modelo de dados +
> assinaturas + ordem de tarefas. **Estende o `/board` da fase 12 e o popup/Table da fase 24** —
> não os reescreve. Herda o baseline do `SDD.md §7` e a cadeia de middlewares das fases 01/02/12
> (`RequireAuth` → `RequireMember` → `RequireWriteRole`) + o `loadProjectGate` (visibilidade→404,
> escrita→403, arquivado→400). Nada aqui é código — é o contrato. **Última fase do build da M3.**

---

## 1. Arquitetura geral

```
Backend (pacote novo internal/properties + 2 campos no /board):
  migrations/0023_task_properties.sql            ← project_property_defs + task_property_values (RNF-05)
  internal/properties/service.go      (NOVO)     ← Service: CRUD de defs + set/clear de value + validação por tipo (CA-01..13)
  internal/properties/gate.go         (NOVO)     ← loadProjectGate auto-contido (cópia do padrão da fase 12, D-03)
  internal/properties/validate.go     (NOVO)     ← validação por tipo (text/number/select/date/url/checkbox) — CA-09
  internal/objectives/board.go        (existe)   ← estende o Board: + property_defs (1 query) + property_values por task (1 query) — CA-16
  internal/api/properties_handlers.go (NOVO)     ← REST de defs + set/clear de value (handlers finos)
  internal/api/core_routes.go         (existe)   ← + subárvore /projects/{projectId}/properties e .../tasks/{id}/properties

Frontend (Table com colunas custom + popup da task):
  lib/api.ts                                     ← PropertyDef/PropertyValue types; Board.project.property_defs + Task.property_values;
                                                   listProperties/createProperty/patchProperty/deleteProperty/setValue/clearValue
  lib/queries/properties.ts           (NOVO)     ← useCreateProperty/usePatch/useDelete/useSetValue/useClearValue (invalidam boardKey)
  lib/propertyValue.ts                 (NOVO)     ← helpers puros: validateValue(type,v), normalizeValue, resolveOption(def,id) (testável jsdom)
  components/board/PropertyCell.tsx    (NOVO)     ← célula da Table por tipo (input/checkbox/dropdown), editável inline (CA-19/20)
  components/board/PropertyRow.tsx     (NOVO)     ← linha de propriedade custom no popup central (CA-21) — inspirado no mockup 528-531
  components/board/AddPropertyButton.tsx (NOVO)   ← "Add property" → cria def + abre p/ nomear/tipar (CA-21)
  <visão Table da fase 24>             (24, est.) ← colunas dinâmicas das defs (a fase 24 cria a visão Table; a 25 estende as colunas)
  <popup central da fase 24>           (24, est.) ← + bloco de propriedades custom + Add property (a fase 24 cria o popup central)
```

> **Dependência dura da fase 24** (ver §8): o **popup central** da task (o componente que a fase 24
> introduz com título + propriedades nativas + descrição .md + sidebar de sub-tasks) e a **visão
> Table** (a 3ª visão da aba Tasks) **são da fase 24**. Esta fase **estende** esses dois (adiciona o
> bloco de propriedades custom e as colunas dinâmicas). **NÃO** hardcodar nomes de arquivo da fase
> 24 (`TaskModal.tsx`/`TaskTable.tsx` PODEM não existir — o `TaskModal.tsx` atual é o formulário
> pequeno da fase 12, NÃO o popup central). Como a fase 25 é a **última do build da M3**, a T-11
> **confirma os nomes reais** do componente de popup central e da visão Table que a fase 24 deixou
> e injeta por **comportamento** (linha por def no popup; coluna por def na Table), não por arquivo
> fixo. Os números de linha do `mockup` são só inspiração visual, **não** contrato.

## 2. Migração — `0023_task_properties.sql` (dona: fase 25)

```sql
-- 0023_task_properties — fase 25 (plan §2): "database por projeto".
-- project_property_defs = o SCHEMA de propriedades custom de um projeto.
-- task_property_values  = o VALOR de uma def numa task (jsonb tipado).
-- workspace_id denormalizado (D-09 da fase 02) pro filtro tenant trivial (CA-15).
-- O CHECK de type lista os 8 tipos (schema comporta person/multi_select), mas o
-- SERVICE da v1 restringe aos 6 (text/number/select/date/url/checkbox) — CA-02/§6.

CREATE TABLE project_property_defs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name         text NOT NULL,
    type         text NOT NULL CHECK (type IN
                   ('text','number','select','multi_select','date','person','checkbox','url')),
    options      jsonb NOT NULL DEFAULT '[]'::jsonb,   -- select/multi_select: [{id,label,color}]
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_prop_defs_project ON project_property_defs (project_id, position);

CREATE TABLE task_property_values (
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    def_id       uuid NOT NULL REFERENCES project_property_defs(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    value        jsonb NOT NULL,                        -- tipado conforme def.type (CA-09)
    PRIMARY KEY (task_id, def_id)
);
-- leitura do /board (CA-16/RNF-02): values do projeto = WHERE def_id = ANY($defs).
CREATE INDEX idx_prop_values_def ON task_property_values (def_id);
```

- Numeração: `0020_reminders` é o último BUILDADO; a M3 começa em `0021` (fase 24 = `0021`/`0022`),
  então a fase 25 é **`0023`** (FIXO do PRD §7 — NÃO ler o próximo livre do disco; sob build
  paralelo de worktrees a leitura colidiria). INDEX.md: registrar **0023 dona da fase 25**.
- `project_property_defs.project_id ON DELETE CASCADE`: apagar um projeto leva o schema junto.
- `task_property_values` com **duas** FKs cascade (`task_id`→tasks, `def_id`→defs): deletar a
  task **ou** a def remove o value (CA-06 — deletar def limpa os values; deletar task limpa os
  seus values). PK `(task_id, def_id)` = upsert natural (CA-08, D-05).
- `type` CHECK com os 8 valores = backstop; a restrição da v1 aos 6 é **no service** (CA-02) —
  evita migração destrutiva quando a fatia 25.1 ligar `person`/`multi_select` (só muda o service).
- `value jsonb NOT NULL`: a ausência de value = **ausência de linha** (não `NULL` numa linha) —
  zero linha por task até preencher (CA-17). Limpar = `DELETE` da linha (CA-10).

## 3. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Defs aninhadas no projeto** (`/projects/{projectId}/properties[/{defId}]`); **values aninhados na task** (`/projects/{projectId}/tasks/{taskId}/properties/{defId}`) | A URL torna a cadeia de visibilidade explícita (reusa o gate com o `projectId` da rota). Impossível "esquecer" o filtro projeto/workspace (CA-12/15). O value mora sob a task **e** o projeto → revalida os dois pertencimentos. |
| D-02 | **Pacote novo `internal/properties`**, NÃO estender `internal/tasks`/`projects` | `tasks.Service`/`projects.Service` ficam byte-idênticos (RNF-06); o ciclo de vida de def/value é próprio. Só o `internal/objectives/board.go` é tocado (estende a leitura) — e a estensão é aditiva (2 campos). |
| D-03 | **Gate de escrita REPLICADO no `properties`** (cópia do `objectives/gate.go` da fase 12) — `loadProjectGate(ctx, ws, userID, projectID)` retorna `{ownerUserID, archived, access}` + `canWrite(userID,role)`; o `ensureWrite` espelhado devolve `domain.InvalidReference("projeto arquivado é só leitura")` quando arquivado | A fase 12 já provou esse padrão (D-03 alt. B): cópia de ~25 linhas, sem extração, sem ciclo de import, `tasks`/`projects` intocados. O `properties` ganha o seu próprio gate; `visibility='shared' OR owner_user_id=$user` → 404; dono ∥ owner-ws ∥ `access=edit` → permite escrita; arquivado quando `blockArchived` → **400 `invalid_reference`** (código REAL da fase 12 — `domain.InvalidReference`; **NÃO** existe code `archived`). Teste de **paridade** (T-02) prova mesmo erro/code pra mesma entrada que a rota de objetivo. |
| D-04 | **`type` IMUTÁVEL após criar a def** (PATCH ignora `type`) | O `value jsonb` é tipado conforme `def.type`; mudar o tipo invalidaria todos os values já gravados (number→checkbox?). Imutável é o caminho seguro e simples (deletar+recriar pra trocar tipo). Sem coluna de "tipo antigo", sem migração de values. |
| D-05 | **Value = upsert idempotente** via `PUT`; clear = `DELETE` | A PK `(task_id, def_id)` é o ponto natural de upsert (`ON CONFLICT (task_id,def_id) DO UPDATE SET value=$`). PUT é idempotente (RFC) e bate com edição inline (a UI manda o valor inteiro, não um patch). DELETE limpa (204 idempotente — CA-10). |
| D-06 | **Schema+values entregues no `/board`** (2 queries fixas adicionais), NÃO endpoint separado por task | RNF-02: evita N+1 e um 2º round-trip. O `/board` já é o read agregado da fase 12; adicionar 1 query de defs (`WHERE project_id=$`) + 1 de values (`WHERE workspace_id=$ AND def_id = ANY($defs do projeto)`), agrupadas por `task_id` em memória, mantém **um** request. O `GET .../properties` (CA-04) existe pro fluxo de gestão de schema isolado, mas o render da Table/popup come do `/board`. |
| D-07 | **Validação por tipo na mão (stdlib + `encoding/json`)** em `validate.go`: switch por `def.type` | Leveza (RNF-01); 6 regras simples não pagam uma lib. `number`=`json.Number`/float finito; `date`=`time.Parse("2006-01-02")`; `checkbox`=bool; `select`=id ∈ `def.options`; `text`/`url`=string ≤2000. `value` chega como `json.RawMessage` → decode tipado por `type`. |
| D-08 | **Value de `select` guarda o `option.id`** (não o label) | Renomear um label de option não quebra os values (estável por id). A leitura resolve `id→{label,color}` via `def.options`; id órfão (option removido) → tratado como limpo (CA-13). |
| D-09 | **Restrição da v1 aos 6 tipos no SERVICE** (CA-02), CHECK da migração com os 8 | Permite ligar `person`/`multi_select` na fatia 25.1 mudando **só o service** (sem migração) — schema já comporta. A v1 fica enxuta (anti-Notion-creep). |
| D-10 | **Front: Table estende a fase 24** com colunas dinâmicas; **popup estende a fase 24** com bloco custom + Add property | Não reescreve a Table/popup (RNF-06); só injeta as colunas/linhas custom. Reusa `lib/palette.ts` pra cor de option (exceção sancionada da fase 02). |
| D-11 | **value órfão de option é resolvido pra "limpo" na LEITURA**, não apagado na escrita do PATCH de options | À prova de erro (RNF-04 da M3); o dado permanece (caso o option volte), mas o render não quebra (CA-13). Sem cascade de limpeza de values ao editar options (mais barato e reversível). |

## 4. Contratos de API

Prefixo: `/api/v1/workspaces/{workspaceId}`. Auth: cookie de sessão (fase 01). Erros sempre
`{"error":{"code","message"}}`: `unauthorized` (401) · `forbidden` (403) · `not_found` (404) ·
`validation_error` (400) · `invalid_reference` (400, gate de escrita em projeto arquivado, padrão
fase 12 — o `ensureWrite`/`loadProjectGate` devolve `domain.InvalidReference`; **não existe code
`archived`** no backend) — idênticos às fases 02/12.

### 4.1 Definições de propriedade (schema)

| Método | Rota | Quem pode |
|---|---|---|
| `GET`    | `/projects/{projectId}/properties` | membro que enxerga o projeto (invisível → 404) |
| `POST`   | `/projects/{projectId}/properties` | escrita no projeto (dono/owner-ws/membro edit — CA-01); viewer 403 |
| `PATCH`  | `/projects/{projectId}/properties/{defId}` | idem (type imutável — D-04) |
| `DELETE` | `/projects/{projectId}/properties/{defId}` | idem (hard delete, values cascade — CA-06) |

`POST .../properties`:

```json
// → request (text/number/date/url/checkbox: options ignorado)
{ "name": "Estimativa", "type": "number" }

// → request select (options obrigatório, label 1-40, id opcional, color opcional)
{ "name": "Cliente", "type": "select",
  "options": [ { "label": "Lauda", "color": "#4f9dde" }, { "label": "Arkan" } ] }

// ← 201
{ "id": "df1...", "project_id": "0d9e...", "workspace_id": "8c...",
  "name": "Cliente", "type": "select",
  "options": [ { "id": "op_a", "label": "Lauda", "color": "#4f9dde" },
               { "id": "op_b", "label": "Arkan", "color": null } ],
  "position": 2 }

// ← 400 tipo inválido / person|multi_select (CA-02)
{ "error": { "code": "validation_error", "message": "tipo de propriedade inválido" } }
// ← 400 select sem options (CA-03)         ← 403 viewer (CA-07)   ← 404 projeto invisível (CA-15)
```

`GET .../properties` → `200 {"properties":[ {…} ]}` (ordem por `position` — CA-04).
`PATCH .../properties/{defId}` aceita subconjunto de `{name, position, options}` (`type` ignorado —
D-04); `options` re-normaliza (gera id pra option nova, mantém id de option existente — CA-05).
`DELETE .../properties/{defId}` → `204` (values cascade — CA-06).

### 4.2 Valores de propriedade (por task)

| Método | Rota | Quem pode |
|---|---|---|
| `PUT`    | `/projects/{projectId}/tasks/{taskId}/properties/{defId}` | escrita no projeto (CA-08); viewer 403 (CA-11) |
| `DELETE` | `/projects/{projectId}/tasks/{taskId}/properties/{defId}` | idem (clear, 204 idempotente — CA-10) |

```json
// → PUT (value tipado conforme def.type)
{ "value": "op_a" }          // select: option.id existente
{ "value": 12.5 }            // number: número JSON finito
{ "value": "2026-07-01" }    // date: YYYY-MM-DD
{ "value": true }            // checkbox: bool
{ "value": "https://…" }     // url/text: string ≤2000

// ← 200
{ "def_id": "df1...", "value": "op_a" }

// ← 400 value não casa com o tipo (CA-09)
{ "error": { "code": "validation_error", "message": "valor inválido para o tipo da propriedade" } }
// ← 404 def de outro projeto / task de outro projeto / id malformado (CA-12)
// ← 403 viewer/access=view (CA-11)        ← 400 invalid_reference (projeto arquivado)
```

`DELETE .../tasks/{taskId}/properties/{defId}` → `204` (idempotente — CA-10).

### 4.3 `/board` — campos novos (extensão da fase 12, RNF-06)

`GET .../projects/{projectId}/board` ganha (sem mudar o que já entrega):

```json
{
  "project": {
    "id": "0d9e...", "...": "...", "can_write": true,
    "property_defs": [                                  // NOVO — o schema do projeto (CA-16)
      { "id": "df1...", "name": "Cliente", "type": "select",
        "options": [ {"id":"op_a","label":"Lauda","color":"#4f9dde"} ], "position": 0 }
    ]
  },
  "objectives": [
    { "...": "...", "tasks": [
      { "id":"77aa...", "...": "...", "tracked_seconds": 4200,
        "property_values": [ {"def_id":"df1...","value":"op_a"} ]   // NOVO — só defs c/ value (CA-16/17)
      } ] } ],
  "general": { "...": "...", "tasks": [ { "...": "...", "property_values": [] } ] }
}
```

- `project.property_defs`: as defs do projeto **uma vez** (CA-16). `task.property_values`: só as
  defs **com value** pra aquela task (CA-17 — task nova = `[]`; a UI deriva o resto do
  `property_defs`). Value de `select` cujo `option.id` sumiu de `def.options` → **omitido**
  (CA-13/D-11). O cliente resolve `select` via `def.options` (entregamos o id cru — D-08).
- **Sem N+1** (RNF-02/D-06): defs do projeto em 1 query; values em 1 query
  (`WHERE workspace_id=$ AND def_id = ANY($defs do projeto)`), `map[task_id][]value` em memória.

## 5. Backend — assinaturas

```go
// internal/properties
type PropertyDef struct {
    ID, ProjectID, WorkspaceID, Name, Type string
    Options  []Option `json:"options"`
    Position int      `json:"position"`
}
type Option struct {
    ID, Label string
    Color *string `json:"color"`
}
type PropertyValue struct {
    DefID string          `json:"def_id"`
    Value json.RawMessage `json:"value"`   // tipado conforme def.Type
}
type CreateDefInput struct { Name, Type string; Options []Option }
type PatchDefInput struct {                  // type NÃO entra (imutável — D-04)
    Name *string
    PositionSet bool; Position *int
    OptionsSet bool; Options []Option
}

func NewService(db *pgxpool.Pool) *Service                                  // gate interno (D-03)
func (s *Service) ListDefs(ctx, ws, userID, projectID string) ([]PropertyDef, error)              // CA-04/15
func (s *Service) CreateDef(ctx, ws, userID, role, projectID string, in CreateDefInput) (PropertyDef, error) // CA-01/02/03/07
func (s *Service) PatchDef(ctx, ws, userID, role, projectID, defID string, in PatchDefInput) (PropertyDef, error) // CA-05/07/12
func (s *Service) DeleteDef(ctx, ws, userID, role, projectID, defID string) error                 // CA-06/07/12
func (s *Service) SetValue(ctx, ws, userID, role, projectID, taskID, defID string, raw json.RawMessage) error // CA-08/09/09.1/11/12
//   CA-09.1: ANTES de validateValue, se string(raw)=="null" (padrão tasks_handlers.go:172),
//            delega pra ClearValue (DELETE da linha — a coluna value é NOT NULL). O 204 do
//            handler reflete o clear. validateValue só roda pros values NÃO-null.
func (s *Service) ClearValue(ctx, ws, userID, role, projectID, taskID, defID string) error        // CA-10/09.1/11/12

// internal/properties/validate.go
func validateValue(typ string, options []Option, raw json.RawMessage) (json.RawMessage, error) // CA-09
//   switch typ:
//     "text","url": string, len<=2000 (decode pra string)
//     "number":     json.Number / float64 finito (NaN/Inf -> erro); string -> erro
//     "date":       string que time.Parse("2006-01-02") aceita
//     "checkbox":   bool
//     "select":     string == algum option.ID em options (senão erro)
//     default:      erro (validation) — person/multi_select caem aqui na v1 (CA-02 já barra no CreateDef)

// internal/properties/gate.go  (cópia do objectives/gate.go — D-03; NÃO importa projects/tasks)
//   loadProjectGate(ctx, ws, userID, projectID) (projectGate, error): visibilidade->404
//   canWrite(userID, role) bool: dono ∥ role=="owner" ∥ access=="edit"
//   CA-12 (pertencimento): PatchDef/DeleteDef/SetValue/ClearValue carregam a def com
//     WHERE workspace_id=$ws AND project_id=$projectIdDaRota AND id=$defId (uuid.Validate antes -> 404, nunca 500);
//     SetValue/ClearValue carregam a task com WHERE workspace_id=$ws AND project_id=$projectIdDaRota AND id=$taskId.
```

- **Validação (CA-01.1/02/03/09)** no Create/Patch/SetValue ANTES do SQL → `domain.Validation(...)`.
  `name` 1–80; `type` ∈ 6 da v1 (CreateDef); `options` de select (label 1–40, sem dup, gera id);
  `value` via `validateValue`.
- **Topologia de import (sem ciclo)**: `properties` tem o seu próprio `loadProjectGate` (não
  importa `projects`/`tasks`/`objectives`). O `objectives.Board` é estendido **dentro do pacote
  objectives** com 2 queries (defs + values) — `objectives` **não** importa `properties` (faz o SQL
  inline, como já faz o tracked_seconds; ou recebe os dados via um pequeno `PropertySource` injetado
  no boot, espelhando o `PlannedSource` da fase 06 — **decisão na T-05**, default = SQL inline pra
  não criar dependência). Os pacotes da fase 02/24 ficam intocados (RNF-06).

## 6. Frontend — telas e componentes

Tokens de `tokens.css`. Reusa `.tt-panel`, `.chip`, `lib/palette.ts` (cor de option), formatação do
board (fase 12/24). As classes de layout `.ntable`/`.prop`/`.prop-k`/`.prop-v` do `mockup`
**não existem no CSS real hoje** — quem as porta pro CSS real (`styles/index.css`/`tokens.css`) é a
**fase 24** (dona da Table/popup que as usam); a 25 só consome (RNF-03; ver §8). Se a 24 não portar,
a T-11 da 25 assume o porte + teste de presença. A fase 25 descreve a UI por **comportamento**, não
por classe crua do mockup.

| Item | Arquivo (em `frontend/src/`) | Notas |
|---|---|---|
| Helpers puros | `lib/propertyValue.ts` | `validateValue(type,v)` (espelho do backend), `resolveOption(def,id)`, `emptyValue(type)`. Testável jsdom. |
| API + hooks | `lib/api.ts` (estende), `lib/queries/properties.ts` (novo) | tipos `PropertyDef`/`Option`/`PropertyValue`; `Board.project.property_defs`; `Task.property_values`; `listProperties`/`createProperty`/`patchProperty`/`deleteProperty`/`setValue`/`clearValue`; hooks invalidam `boardKey` |
| Célula da Table | `components/board/PropertyCell.tsx` | render por tipo: text/number/url=input; date=`<input type=date>`; checkbox=toggle; select=dropdown com cor da option (`palette`). Edita inline → `setValue`/`clearValue` (CA-19/20) |
| Linha no popup | `components/board/PropertyRow.tsx` | linha de propriedade no popup central (inspiração `mockup` 528–531): nome editável da def + editor de value por tipo + ✕ remove def (CA-21) |
| Add property | `components/board/AddPropertyButton.tsx` | `mockup` 553/571 (inspiração): cria def (`POST`, default `text`) + foca pra nomear/tipar (CA-21) |
| Visão Table (estende 24) | *componente real da visão Table da fase 24 — confirmar nome no build (T-11)* | injeta `property_defs` como colunas dinâmicas após as nativas (CA-19); gating por `board.project.can_write` (CA-22) |
| Popup central (estende 24) | *componente real do popup central da fase 24 — confirmar nome no build (T-11); NÃO é o `TaskModal.tsx` da fase 12* | injeta o bloco custom (PropertyRow×N + AddPropertyButton) abaixo das nativas (CA-21); gating por `can_write` (CA-22) |

- `useBoard` (fase 12) é a query central; as mutações de propriedade (defs e values) **invalidam
  `boardKey(ws, projectId)`** → reflexo sem reload (CA-20/21). Não mexer em `lib/queries/tasks.ts`
  (aditivo/local).
- Gating viewer (CA-22): a UI lê **`board.project.can_write`** (fase 12, computado no backend) —
  **não** `useRole(ws)` sozinho. Padrão de teste `viewer-gating.spec` das fases 02/12 (`can_write`
  mockado).
- Cor de option de `select` = dado de domínio via `lib/palette.ts` (exceção sancionada do
  `check-cores`; espelha cor de projeto da fase 02). Sem hex literal solto em `.tsx`.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** apagar tasks ao deletar def (CA-06) — é `ON DELETE CASCADE` só nos **values** daquela def.
- **NÃO** permitir mudar `type` de def existente (D-04) — `type` imutável no PATCH; deletar+recriar.
- **NÃO** gravar value sem validar pelo `type` (CA-09) — `validateValue` ANTES do SQL → `400`,
  nunca `500` por jsonb torto.
- **NÃO** aceitar `person`/`multi_select` na v1 (CA-02/D-09) — service rejeita; CHECK da migração
  os lista só pra fatia 25.1 (sem migração depois).
- **NÃO** guardar o LABEL de `select` no value (D-08) — guarda o `option.id`; resolve na leitura.
- **NÃO** quebrar o render com value órfão de option (CA-13/D-11) — leitura trata como limpo.
- **NÃO** fazer N+1 no `/board` (RNF-02/D-06) — defs em 1 query, values em 1 query (`def_id =
  ANY($defs)`), agrupados em memória.
- **NÃO** somar/ler values de OUTRO workspace ou projeto invisível (CA-15) — `WHERE workspace_id`
  + visibilidade do projeto em **toda** query nova.
- **NÃO** carregar def/value (Patch/Delete/SetValue) sem casar `project_id` da rota (CA-12) —
  `WHERE workspace_id AND project_id AND id`; def de projeto vizinho via rota errada → `404`;
  `uuid.Validate` antes (404, nunca 500).
- **NÃO** materializar uma linha de value vazia por task ao criar a def (CA-17) — herança do
  schema é por **derivação no cliente**; zero linha até o 1º PUT.
- **NÃO** criar ciclo de import `objectives ↔ properties` (§5) — board faz SQL inline (ou recebe
  `PropertySource` injetado no boot, sem import).
- **NÃO** reescrever/extrair `tasks.Service`/`projects.Service`/`objectives.Service` salvo a
  extensão aditiva do `Board` (RNF-06/D-02) — `properties` é auto-contido.
- **NÃO** introduzir lib de datagrid/DnD (RNF-01/§6) — Table = HTML + inputs nativos; reordenar
  def = `PATCH position` (sem drag nesta fase).
- **NÃO** deixar a Table estourar horizontal em mobile (RNF-07) — `overflow:auto` no `.tt-panel`.

## 8. Dependências & paralelização

- **Dependência DURA da fase 24 (BLOQUEADOR explícito)** (Tasks 2.0): o **popup central** da task e
  a **visão Table** são da fase 24; esta fase os **estende** (bloco de propriedades custom + colunas
  dinâmicas). Sem a fase 24 buildada, T-10/T-11 não têm onde injetar. → **NÃO paralelizável com a
  24** (contention de arquivo nos mesmos componentes).
  - **Pré-requisitos da fase 24 que DEVEM existir antes da T-10/T-11** (a 25 confirma os nomes reais
    no build — não hardcodar):
    1. **Componente de popup central da task** (o que reúne título + propriedades nativas +
       descrição .md + sidebar de sub-tasks). **NÃO é o `TaskModal.tsx` da fase 12** (esse é o
       formulário pequeno: nome/status/data/notas/objetivo/assignee). O nome real vem da fase 24.
    2. **Componente da visão Table** (a 3ª visão da aba Tasks — board/list/**table**). Hoje **não
       existe** Table no `ProjectBoard` (só board/list); a fase 24 a cria.
    3. **As classes de layout `.ntable`/`.prop`/`.prop-k`/`.prop-v`/`.prop-val`** portadas do
       `mockup` pro CSS real (`styles/index.css` ou `tokens.css`). **Hoje 0 ocorrências em
       `tokens.css`** — só estão no `<style>` do mockup.
  - **Propriedade das classes CSS (decisão cross-fase, opção A):** a **fase 24** é a dona do porte
    de `.ntable`/`.prop*` pro CSS real (ela cria a Table/popup que as usam); a fase 25 **só
    consome**. **Fallback (opção B):** se a fase 24 não portar (gap), a fase 25 assume o porte como
    parte da T-11 e orça isso no escopo. Em qualquer caso, a T-11 ganha um **teste de presença**
    (asserção de string em `index.css`/`tokens.css` das classes `.ntable`/`.prop-k`/`.prop-v` —
    padrão da asserção de presença de `--font-ui` da fase 24), porque o `check-cores` **não**
    detecta classe ausente.
- **Dependência de leitura da fase 12** (board): estende `internal/objectives/board.go` (2 campos).
  Backend-side, a 25 toca `board.go` — se outra fase da M3 (28, projeto vivo) também tocar
  `board.go`, integrar em sequência (não worktrees paralelas no mesmo arquivo).
- **Independente do resto da M3** (notas 26 / spotlight 27 / calendário 28 / goals 29): o pacote
  `internal/properties` e a migração `0023` são auto-contidos. A única colisão de arquivo é com a
  24 (front) e com quem tocar `board.go` (back).
- **Ordem do PRD §7:** 24 → 26 → 27 → 28 → 29 → **25** (esta por **último**, pelo custo). A
  migração `0023` é o número mais alto da M3 → sem contenção de migração (as outras fases pegam
  0021/0022/0024/0025/0026).
- **Migração FIXA `0023`** — não "ler o próximo livre" (sob worktrees colidiria).

## 9. Ordem de execução (ver tasks.md)

Backend primeiro (migração → gate auto-contido → validação por tipo → CRUD de defs + set/clear de
value → board estendido → handlers/wiring), depois o frontend (api/queries/helpers → PropertyCell
→ PropertyRow + AddProperty → integração na Table/popup da 24 → gating), depois E2E e gate. Commit
por tarefa. `make check-full` é o gate final. **Última fase do build da M3 → o deploy único da M3
sai depois desta** (rotina do KICKOFF; NÃO tocar Caddy/PrimaOS).
