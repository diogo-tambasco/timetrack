# tasks.md — Fase 25 · Propriedades tipadas ("database por projeto")

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-15).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → gate → validação → CRUD defs +
> set/clear value → board estendido → handlers/wiring), depois frontend (api/queries/helpers →
> célula/linha → integração na Table/popup da 24 → gating), E2E e gate. **Estende o `/board` da
> fase 12 e o popup/Table da fase 24 — não os reescreve** (RNF-06). **Última fase do build da M3.**

---

## Bloco 1 — backend domínio (T-01 → T-07)

- **T-01 — Migração `0023_task_properties`** (CA-06/08/17 backstop, RNF-05)
  - `backend/internal/db/migrations/0023_task_properties.sql` (plan §2): `project_property_defs`
    (`type` CHECK com os 8 tipos; `options jsonb default '[]'`; índice `(project_id, position)`) +
    `task_property_values` (PK `(task_id, def_id)`; 2 FKs `ON DELETE CASCADE`; índice `(def_id)`).
  - Número FIXO **`0023`** (PRD §7 — fase 24 é dona de 0021/0022; NÃO ler o próximo livre).
  - Teste (`migrations_*_test.go` ou novo): tabelas existem; `options` default `'[]'`; `position`
    default 0; **deletar def → CASCADE limpa os values daquela def, NÃO apaga a task** (CA-06);
    **deletar task → CASCADE limpa os values dela** (CA-08); `idx_prop_values_def` existe; CHECK
    barra `type='foo'` mas aceita `'select'`/`'multi_select'`/`'person'` (o service é quem barra os
    2 da fatia — CA-02). PK `(task_id, def_id)` impede 2ª linha pro mesmo par (upsert — D-05).
  - INDEX.md: registrar 0023 dona da fase 25 (orquestrador).

- **T-02 — Gate de projeto AUTO-CONTIDO no `properties`** (D-03, RNF-06)
  - `internal/properties/gate.go` (NOVO): `loadProjectGate(ctx, ws, userID, projectID)
    (projectGate, error)` — **cópia** do `internal/objectives/gate.go` da fase 12 (visibilidade
    `(visibility='shared' OR owner_user_id=$user)`→404; `canWrite` = dono ∥ `role=="owner"` ∥
    `project_members.access=="edit"`; `archived` exposto pro caller decidir 400). **NÃO** tocar
    `tasks`/`projects`/`objectives` (byte-idênticos — RNF-06); **NÃO** importar esses pacotes.
  - Teste `gate_test.go` (dbtest): private de outro → 404; `access=view`/member-sem-access →
    `canWrite=false`; dono/owner-ws/`access=edit` → `canWrite=true`; **paridade** com a rota de
    objetivo (mesma entrada → mesmo erro/decisão). Suíte das fases 02/12 **continua verde sem
    nenhuma mudança**.

- **T-03 — Validação por tipo** (CA-09)
  - `internal/properties/validate.go` (NOVO): `validateValue(typ string, options []Option, raw
    json.RawMessage) (json.RawMessage, error)` — switch por `typ` (plan §5): `text`/`url`=string
    ≤2000; `number`=float finito (NaN/Inf/string→erro); `date`=`time.Parse("2006-01-02")`;
    `checkbox`=bool; `select`=string ∈ `option.ID`; default→`domain.Validation`. Retorna o jsonb
    normalizado (canônico) ou erro.
  - Teste `validate_test.go` (unit Go, sem db): tabela por tipo — válidos passam, inválidos viram
    `validation_error` (`"abc"` em number; `"2026-13-40"` em date; `1` em checkbox; option.id
    inexistente em select; string >2000 em text; NaN em number); `person`/`multi_select` (default)
    → erro. **Mutação removendo o ramo `select` (aceitar qualquer string) deve QUEBRAR** (CA-09).

- **T-04 — `properties.Service`: CRUD de defs + set/clear de value** (CA-01/01.1/02/03/04/05/06/07/08/10/11/12/15)
  - `internal/properties/service.go` (NOVO): `PropertyDef`/`Option`/`PropertyValue`,
    `CreateDefInput`/`PatchDefInput` (sem `type` — D-04). `ListDefs`/`CreateDef`/`PatchDef`/
    `DeleteDef`/`SetValue`/`ClearValue`. Validação na mão (plan §5/§7): `name` 1–80; `type` ∈ 6 da
    v1 no CreateDef (`person`/`multi_select`→400 — CA-02); `options` de select (label 1–40, sem
    dup, gera `id` faltante — CA-03). `SetValue` chama `validateValue` (CA-09) e faz **upsert**
    (`ON CONFLICT (task_id,def_id) DO UPDATE SET value=$` — D-05). Toda operação passa pelo
    `loadProjectGate` interno (visibilidade→404, escrita→403, arquivado→400). **Patch/Delete de def
    + Set/Clear de value carregam com `WHERE workspace_id AND project_id AND id`** + `uuid.Validate`
    (CA-12). `position` auto no fim no CreateDef.
  - **CA-09.1 (value:null = clear):** o `SetValue` (ou o handler — ver T-06) detecta
    `string(raw) == "null"` ANTES de `validateValue` e delega pra `ClearValue` (DELETE da linha; a
    coluna `value` é NOT NULL). `validateValue` só roda pros values não-null.
  - Teste `service_test.go` (dbtest): create def por tipo (CA-01); `name` vazio/>80 → 400 (CA-01.1);
    `type` inválido + **`person`/`multi_select` → 400** (CA-02); select sem options / label dup →
    400 (CA-03); list por position (CA-04); PATCH name/position/options (gera id de option nova,
    mantém id existente; **`type` no payload é ignorado, não erra** — CA-05/D-04); **DELETE def →
    values somem (CASCADE), task fica** (CA-06); SetValue válido faz upsert (2º PUT sobrescreve —
    D-05); SetValue inválido por tipo → 400 (CA-09); **SetValue com `value:null` (qualquer tipo) →
    remove a linha, idempotente (sem value prévio também) — CA-09.1**; ClearValue idempotente → 204
    mesmo sem value (CA-10); escrita por view/viewer → 403 (CA-07/11);
    **escrita em projeto ARQUIVADO → 400 com `error.code == "invalid_reference"`** (código REAL da
    fase 12 — `domain.InvalidReference`; **NÃO** `archived` — CA-11; copiar o `ensureWrite`/
    `loadProjectGate` da fase 12); projeto invisível → 404 (CA-15);
    **def do projeto B alcançada por rota do projeto A → 404** (mutação que remove `project_id=$rota`
    do WHERE deve morrer — CA-12); **task do projeto B + def de A via rota de A → 404** (CA-12);
    `defId`/`taskId` malformado no path → 404 (não 500); **isolamento por workspace** (mutação
    removendo `WHERE workspace_id`/visibilidade morre — CA-15).

- **T-05 — Board estendido: `property_defs` + `property_values` por task** (CA-13/16/17/18, RNF-02)
  - `internal/objectives/board.go` (estende): no payload do `Board`, adicionar
    `project.property_defs` (1 query: defs do projeto por position) e `tasks[].property_values`
    (1 query: `WHERE workspace_id=$ AND def_id = ANY($defs do projeto)`, `map[task_id][]value`).
    Value de `select` cujo `option.id` sumiu de `def.options` → **omitido** (CA-13/D-11). Task nova
    = `property_values: []` (CA-17 — derivação no cliente, zero linha materializada). **Sem N+1**
    (D-06): 2 queries fixas a mais, montagem em memória O(n). **Decisão de import (plan §5):** SQL
    inline em `board.go` (default, sem importar `properties`) OU `PropertySource` injetado no boot
    (espelho do `PlannedSource` da fase 06) — escolher na T-05 e registrar; default = inline.
  - Teste `board_test.go` (estende): `/board` traz `property_defs` ordenado; task com value traz
    `property_values` com o par certo; task sem value → `[]` (CA-17); **value de select órfão de
    option → omitido** (CA-13 — mutação que envia o id órfão deve QUEBRAR); **value de OUTRO
    workspace/projeto invisível NÃO aparece** (mutação removendo `workspace_id`/`ANY($defs)` morre
    — CA-15); deletar def → `property_defs` não lista + values somem do board (CA-06/16).
  - **CA-18 (perf — exceção red→green, igual CA-18 da fase 12):** `backend/testdata/properties/
    board_props_seed.sql` (10 defs / 300 tasks / ~3.000 values, placeholders `{{WORKSPACE_ID}}`/
    `{{USER_ID}}`/`{{PROJECT_ID}}`, padrão `board_seed.sql`) + `TestBoardComPropsSob200ms` (melhor
    de 3 < 200ms; nasce verde, pina o N+1/RNF-02).

- **T-06 — Handlers de propriedades + values** (CA-01..12/14/16)
  - `internal/api/properties_handlers.go` (NOVO): handlers finos (espelho dos de objetivos)
    pra `GET/POST/PATCH/DELETE .../projects/{projectId}/properties[/{defId}]` e `PUT/DELETE
    .../projects/{projectId}/tasks/{taskId}/properties/{defId}`. `value` chega como
    `json.RawMessage` no PUT; o handler (ou o service) trata `string(raw) == "null"` como clear
    (CA-09.1, padrão `tasks_handlers.go:172`) → 204.
  - Teste `properties_handlers_test.go` (harness `coretest`): GET defaults 200; POST 201 +
    persiste; PATCH/DELETE def; PUT value 200 + persiste; **PUT `{value:null}` → 204 e remove a
    linha (idempotente — CA-09.1)**; DELETE value 204; 400 validation
    (name/type/options/value); 403 viewer; **escrita em projeto arquivado → 400 `invalid_reference`
    no corpo (CA-11; NÃO `archived`)**; 404 projeto invisível / def de B via rota de A / id
    malformado (CA-12); 401 sem sessão (table-driven nas rotas novas — CA-14); board 200 com
    `property_defs` + `property_values` no corpo (CA-16).

- **T-07 — Wiring no `core_routes.go` + boot** (CA-12/14/15)
  - `mountCore`: subárvore `/projects/{projectId}/properties` (+ `/{defId}`) e
    `/projects/{projectId}/tasks/{taskId}/properties/{defId}` no subrouter do workspace (cadeia
    RequireAuth→RequireMember→RequireWriteRole já aplicada — viewer barrado no role gate ANTES do
    service pra escrita). Injetar `properties.Service` no `Core` (campo novo). `cmd/server` liga o
    service; se a T-05 escolheu `PropertySource`, injetar no `objectives` no boot.
  - Teste: rota viva (smoke de roteamento `coretest`/`router_test`); escrita de def por viewer
    barrada (403) — CA-07/CA-14.

> **Fim do bloco 1:** `make test-back` 100% verde (properties + board estendido; tasks/projects/
> objectives intocados salvo a extensão aditiva do board); Vitest/Playwright não rodados. Handoff
> "retomar na T-08".

## Bloco 2 — frontend (T-08 → T-12)

- **T-08 — `lib/api.ts` + `lib/queries/properties.ts` + `lib/propertyValue.ts`** (CA-08/16/19/20/21)
  - `lib/api.ts`: tipos `PropertyDef`/`Option`/`PropertyValue`; `Board.project.property_defs`;
    `Task.property_values`; `listProperties`/`createProperty`/`patchProperty`/`deleteProperty`/
    `setValue`/`clearValue` (paths certos — plan §4). `lib/queries/properties.ts`: hooks
    `useCreateProperty`/`usePatchProperty`/`useDeleteProperty`/`useSetValue`/`useClearValue`
    (invalidam `boardKey`). `lib/propertyValue.ts` (puro): `validateValue(type,v)` (espelho do
    backend), `resolveOption(def,id)`, `emptyValue(type)`.
  - Teste `properties-api.test.ts` + `propertyValue.test.ts`: chamam os paths certos; hooks
    invalidam a query certa; `validateValue` casa com o backend (tabela por tipo); `resolveOption`
    de id órfão → `null` (CA-13). **Limpar um value pela UI** = `clearValue` (DELETE) **ou**
    `setValue` com `value:null` (CA-09.1) — ambos resultam em 204/linha removida; o `propertyValue`
    pode expor um `emptyValue(type)` que a célula usa pra decidir clear vs set.

- **T-09 — `PropertyCell` (célula da Table por tipo)** (CA-19/20/22)
  - `components/board/PropertyCell.tsx`: render por `def.type` — `text`/`number`/`url`=input;
    `date`=`<input type="date">`; `checkbox`=toggle; `select`=dropdown das `options` (cor via
    `lib/palette.ts`). Editar inline (blur/Enter/change) → `setValue`/`clearValue` (CA-20); value
    inválido trivial (number/date) barrado no client (backend é a autoridade — CA-09). Gating por
    `canWrite` (read-only quando false — CA-22).
  - Spec `PropertyCell.spec.tsx`: cada tipo renderiza o controle certo; editar chama setValue com
    o value certo (incl. select id, checkbox bool, date ISO); esvaziar chama clearValue;
    `canWrite=false` → célula só-leitura (sem onChange). Value órfão de select → vazio (CA-13).

- **T-10 — `PropertyRow` + `AddPropertyButton` (popup da task)** (CA-21/22)
  - `components/board/PropertyRow.tsx` (espelha `mockup` 528–531): `prop-k` (nome da def, editável
    via `patchProperty`) + `prop-v` (reusa `PropertyCell` pro value) + ✕ (`deleteProperty` com
    confirmação — remove a def do projeto inteiro). `AddPropertyButton.tsx` (`mockup` 553/571):
    cria def (`createProperty` default `text`) + foca pra nomear/tipar. Gating por `canWrite`.
  - Specs `PropertyRow.spec.tsx` / `AddPropertyButton.spec.tsx`: nome editável chama patch;
    ✕ chama delete (confirmação); Add cria def `text` e foca; ações escondidas quando
    `canWrite=false` (CA-22, padrão `viewer-gating` da fase 12).

- **T-11 — Integração na Table e no popup central da fase 24** (CA-19/21/22, RNF-06)
  - **Passo 0 (confirmar artefatos REAIS da fase 24 — a fase 25 é a ÚLTIMA do build):** localizar no
    código já buildado da 24 (a) o componente da **visão Table** (a 3ª visão da aba Tasks) e (b) o
    componente do **popup central** da task (NÃO o `TaskModal.tsx` da fase 12, que é o form pequeno).
    Usar os **nomes reais** que a 24 deixou; **não** assumir `TaskTable.tsx`/`TaskModal.tsx`.
  - Na visão Table da fase 24: injetar `board.project.property_defs` como **colunas dinâmicas** após
    as nativas (status/priority/due/to-dos/time), cada célula = `PropertyCell` resolvendo o value da
    task (CA-19). No popup central da fase 24: injetar o bloco custom (`PropertyRow`×N do
    `property_defs` + value resolvido + `AddPropertyButton`) abaixo das propriedades nativas (CA-21).
    Gating por `can_write` nos dois. **NÃO** reescrever a Table/popup — só estender (RNF-06).
  - **Classes CSS `.ntable`/`.prop*` (RNF-03 / plan §8):** confirmar que a fase 24 portou
    `.ntable`/`.prop`/`.prop-k`/`.prop-v` pro CSS real (`styles/index.css`/`tokens.css`). Se NÃO
    portou (gap), a T-11 assume o porte. **Teste de presença** (asserção de string no
    `index.css`/`tokens.css` de `.ntable`/`.prop-k`/`.prop-v`, padrão da asserção `--font-ui` da
    fase 24) — o `check-cores` não detecta classe ausente.
  - Specs (jsdom, board mockado com 1 def select): Table mostra a coluna custom com o value;
    popup mostra a linha custom + Add property; `can_write=false` → tudo só-leitura.

- **T-12 — `make test-back` (regressão) + Vitest verde**
  - Garantir que a fase 24 (Table/popup) + fase 12 (board) seguem verdes com a extensão; nenhum
    spec da 24/12 quebrou. Ajustes mecânicos sancionados (se a 24 mudou nomes) registrados.

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright
> não rodado ainda (E2E é o bloco 3). Handoff "retomar na T-13".

## Bloco 3 — E2E + verificação + gate (T-13 → T-15)

- **T-13 — E2E `propriedades.spec.ts`** (CA-23)
  - Logado+escrita (stub fase 01) num projeto `shared`: abre uma task; **Add property** →
    propriedade `select` "Cliente" com 2 opções; **define o valor** (uma opção) na task; abre a aba
    **Table** e vê a coluna "Cliente" com o valor; **edita inline** na Table (troca a opção) →
    reflete; **reload persiste**; **deleta a def** → coluna some da Table + value some da task.
    **SEM** segundo perfil viewer no E2E (stub autentica só o owner — CA-23); o gating de
    `viewer`/`access=view` é coberto por teste de componente jsdom (`viewer-gating` com
    `can_write=false`) nas T-09/T-10/T-11. Reusar `loginViaStub`; nomes únicos (banco reusado,
    idempotente). Liberar a :8080 antes do E2E (`playwright.config` reuseExistingServer:true).

- **T-14 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: isolamento multi-tenant/visibilidade def×value×task; validação
    por tipo / select id órfão / checkbox-number-date bordas; pertencimento def↔projeto↔task
    [CA-12]; N+1 do board / perf; tri-state value vs clear / upsert idempotente; restrição da v1
    aos 6 tipos [person/multi_select]; regressão fase 12/24/02) + refutação dupla. Corrigir
    confirmados red→green. Registrar. **Commitar/limpar a tree ANTES** de rodar o workflow de
    mutação (o `git checkout` de restauração clobbera mudança não-commitada — lição da M2).

- **T-15 — Gate + deploy (ÚNICO da M3) + smoke + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). Antes do E2E/gate,
    liberar a :8080 + reiniciar o `db-test` (evita flake de FocoPicker/timer global).
  - **DEPLOY ÚNICO DA M3** (esta é a última fase do build): pela rotina do KICKOFF (rsync +
    `docker compose build && up -d` em `/opt/timetrack/deploy`; `deploy/.env` protegido por
    exclude; **NÃO** tocar no Caddy/PrimaOS/portas 80-443). Boot: `migrações aplicadas` (0021–0023)
    + `servindo`. Smoke honesto: `/healthz` 200; `.../properties` sem sessão → 401 canônico;
    bundle novo contém a coluna custom/`Add property`; `hub.primorlabs.cloud` intacto
    (RestartCount=0).
  - Handoff (CLAUDE.md "retomar..."), INDEX (25 construída, 0023 dona), memória M3, KICKOFF,
    commit, briefing pro Diogo. Validação LOGADA do Diogo fica pendente (OAuth real).

---

## Blocos (regra de leitura por bloco)

- **Bloco 1 — backend domínio:** T-01 → T-07 (migração, gate, validação, service, board estendido,
  handlers, wiring). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-08 → T-12 (api/queries/helpers, PropertyCell, PropertyRow+Add,
  integração na Table/popup da 24, regressão). Suíte: Vitest + `make test-back`.
- **Bloco 3 — E2E + fechamento:** T-13 → T-15 (E2E, verificação adversarial, gate `make
  check-full` + **deploy único da M3** + handoff).

`make check-full` é gate **só no bloco 3 (T-15)**; os blocos 1 e 2 fecham com a suíte do que
existe verde + handoff + commit.

---

## Furos da verificação adversarial

Verificação adversarial das specs (ETAPA 1) — furos **confirmados 2/2**, todos aplicados em
`spec.md`/`plan.md`/`tasks.md` ANTES do build:

- **25-H1 (HIGH · contrato):** a spec inventava um code de erro `archived` que **não existe** no
  backend. O gate real da fase 12 (que a D-03/T-02 mandam COPIAR) devolve
  `domain.InvalidReference(...)` → **400 `invalid_reference`** pra projeto arquivado
  (`objectives/gate.go::ensureWrite` linha 78; `domain/errors.go::InvalidReference` linha 24).
  **Resolvido:** trocadas as 3 menções a `archived` por `invalid_reference` — spec.md CA-11
  (agora exige `error.code === "invalid_reference"`, não `archived`), plan.md §4 (lista de codes),
  §3 D-03 (descrição do gate) e §4.2 (comentário do JSON). Tarefas T-04 e T-06 ganharam asserção
  explícita de `invalid_reference` pra escrita em projeto arquivado. (Decisão cross-fase G.)

- **25-H3 (MEDIUM · contrato/migração):** semântica de `PUT { "value": null }` era inconsistente
  com a coluna `value jsonb NOT NULL` — CA-09 dizia "null permitido = limpa" só pro `text`, sem
  definir os outros tipos, e a coluna NOT NULL não comporta `null` materializado. **Resolvido
  (DELETE-para-limpar, uniforme):** removidas as frases contraditórias ("null permitido = limpa",
  "vazio = limpa") de CA-09; string vazia `""` é value válido gravável. Adicionado **CA-09.1**:
  `PUT {value:null}` (qualquer tipo) → trata como clear (remove a linha, 204 idempotente), idêntico
  ao DELETE (CA-10), detectando `string(raw) == "null"` ANTES de `validateValue` (padrão real de
  `tasks_handlers.go:172`). CA-10 cruza com CA-09.1. plan §5 ganhou o ramo null na assinatura de
  `SetValue`. T-04/T-06/T-08 ganharam testes de `value:null` → 204/clear (com e sem value prévio).

- **25-H4 (MEDIUM · regressão/consistência cross-fase):** a spec assumia componentes/classes que
  **não existem** e dependem 100% da fase 24 — (a) `TaskTable.tsx`/`TaskModal.tsx` como âncoras (o
  `TaskModal.tsx` atual é o form pequeno da fase 12, NÃO o popup central; não há visão Table); (b)
  classes `.ntable`/`.prop-k`/`.prop-v` (0 ocorrências em `tokens.css`, só no `<style>` do mockup).
  **Resolvido:** (1) RNF-03 deixou de listar `.ntable`/`.prop*` como existentes e fixou que a **fase
  24 é dona do porte** dessas classes pro CSS real (opção A), com fallback de porte na T-11 + teste
  de presença de string; (2) plan §1 (diagrama), §6 (tabela de componentes) e §8 pararam de
  hardcodar `TaskTable.tsx`/`TaskModal.tsx` — referem o **componente real da visão Table** e o
  **popup central** da fase 24 (confirmados no build), descritos por comportamento; (3) §8 elevou a
  dependência da 24 a **BLOQUEADOR** com a lista de pré-requisitos (popup central ≠ TaskModal da 12;
  visão Table; classes CSS); (4) T-11 ganhou o "Passo 0" de confirmar os nomes reais da 24 + o teste
  de presença das classes. (Decisões cross-fase A e B: popup central = componente React da fase 24
  reusado; botão Start do tipo Time é mock da 24 que a 30 wira — sem contrato de slot.)

**Decisões de coordenação cross-fase aplicadas:** G (códigos canônicos reais —
`invalid_reference` no lugar de `archived`); A (popup central = componente da fase 24, não classes
cruas do mockup); a fase 25 é a última do build da M3 → as referências a componentes da 24 são
**confirmadas no build** (T-11), nunca hardcodadas. Decisões D/E/F não tocam esta fase
(properties não refatora `tasks.Create`, não usa `by_hours` nem sub-tasks).
