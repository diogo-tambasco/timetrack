# spec.md — Fase 25 · Propriedades tipadas ("database por projeto")

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Última fase do build da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`).
> Cobre **RF-05** (Table com colunas custom editáveis inline) e **RF-11** (adicionar/remover
> propriedades do projeto; novas tasks herdam o schema).
> **A peça mais cara da M3 (`MILESTONE-3.md §9.1`): risco de virar "Notion-completo".**
> **Decisões travadas (HITL + `MILESTONE-3.md §4.3 / §9.1`):**
> 1. Cada projeto é quase uma **"database" Notion**: tem um **schema de propriedades**
>    (defs) definido pelo dono; cada task carrega **valores** por def.
> 2. **Propriedades NATIVAS (Status · Priority · Due · Time · Notion) NÃO entram aqui** — são
>    colunas próprias das fases 24/0021. Esta fase é só o **custom** que o Diogo criar
>    (ex.: "Cliente", "Estimativa", "Energia").
> 3. **A v1 entrega SÓ os tipos `text · number · select · date · url · checkbox`.** `person` e
>    `multi_select` ficam para a fatia 25.1 (fora-de-escopo-da-v1; ver §6). O schema `options
>    jsonb` já comporta `select`/`multi_select`; só a UI/validação de `person`/`multi_select`
>    é adiada.

---

## 1. Mapa pro problema central (`MILESTONE-3.md §3.2 / §5.1 RF-05 / §5.2 RF-11`)

A fase 24 deu à task um **popup central** (título + propriedades nativas + descrição .md +
sidebar de sub-tasks) e a aba **Tasks** com 3 visões (Board · List · **Table**). Mas a Table
(`mockup` 412–426) só mostra as colunas **nativas** (status/priority/due/to-dos/time). O Diogo
quer **adicionar campos próprios por projeto** — é o que torna cada projeto uma "database"
(decisão §4.3). Esta fase adiciona:

- a **definição de schema** por projeto (`project_property_defs`: nome + tipo + ordem +
  `options` pra select),
- o **valor** por (task, def) (`task_property_values`: jsonb tipado),
- a **validação por tipo** no backend,
- a entrega dos valores **junto do `/board`** (sem N+1) pra render na Table,
- a **UI**: colunas dinâmicas na Table (editáveis inline) + linha de propriedade custom no
  popup da task com "Add property" (`mockup` 528–531, 571–574).

**Herança intocada:** as propriedades nativas (Status/Priority/Due/Time/Notion) seguem nas
colunas próprias das fases 24/0021; este pacote **não** as toca. Tasks, objetivos, board,
comentários, lembretes, presence — tudo da M1/M2 — fica funcional (RNF-05).

## 2. User stories

- **US-01** — Como dono/membro com escrita num projeto, quero **criar propriedades** para o
  projeto (nome + tipo: texto, número, seleção, data, URL, checkbox; e opções para seleção),
  pra modelar os campos que importam pra mim naquele projeto (uma "database").
- **US-02** — Como usuário, quero **renomear, reordenar e deletar** uma propriedade do projeto,
  pra ajustar o schema conforme o trabalho muda (deletar a def remove os valores das tasks).
- **US-03** — Como usuário, quero **preencher o valor** de cada propriedade por task (texto,
  número, data, URL, checkbox, ou escolher uma opção de seleção), e **limpar** o valor, pra
  registrar os dados da task.
- **US-04** — Como usuário, quero que **toda task do projeto herde o schema** de propriedades:
  abrir/criar uma task mostra todas as defs do projeto (vazias por padrão), pra não recriar o
  esquema a cada task.
- **US-05** — Como usuário, quero ver as propriedades custom como **colunas na visão Table**,
  **editáveis inline** por tipo (input/checkbox/dropdown), pra trabalhar a database direto na
  tabela.
- **US-06** — Como viewer do workspace (ou `access=view`, ou projeto arquivado), quero **só ler**
  o schema e os valores, sem criar/editar nada.

## 3. Critérios de aceite (EARS)

### Definições de propriedade (schema do projeto) — API

> Rotas aninhadas no projeto (mesma cadeia de visibilidade das fases 02/12):
> `/api/v1/workspaces/{workspaceId}/projects/{projectId}/properties[/{defId}]`.

- **CA-01** *(event-driven)* — QUANDO um usuário **com permissão de escrita no projeto** (dono
  do projeto, owner do workspace em projeto visível a ele, ou membro do projeto com
  `access=edit`) envia `POST .../projects/{projectId}/properties` com `name` válido (1–80
  caracteres após trim) e `type` válido (∈ `{text, number, select, date, url, checkbox}` na v1),
  o sistema DEVE criar a def com `position` = próximo no fim da lista do projeto, `options` =
  `[]` (salvo `select`, ver CA-03) e retornar `201` com o objeto `{id, project_id, workspace_id,
  name, type, options, position}`.
- **CA-01.1** *(unwanted — nome)* — SE `name` for vazio, só espaços ou tiver mais de 80
  caracteres, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400`
  `error.code="validation_error"`.
- **CA-02** *(unwanted — tipo)* — SE `type` estiver fora de `{text, number, select, date, url,
  checkbox}` (inclusive `person`/`multi_select`, **adiados na v1** — §6), o sistema NÃO DEVE
  criar/atualizar e DEVE retornar `400` `validation_error`. (O CHECK da migração lista os 8
  tipos como backstop, mas o **service** restringe aos 6 da v1 — `person`/`multi_select` viram
  `400` no service.)
- **CA-03** *(state-driven — options de select)* — QUANDO `type = "select"`, o campo `options`
  DEVE ser um array de `{id, label, color}` (cada `label` 1–40 chars após trim; `id` gerado pelo
  servidor se ausente; `color` opcional, token de paleta ou hex `#RRGGBB`). Para `type` ∈ `{text,
  number, date, url, checkbox}`, `options` enviado DEVE ser ignorado/normalizado para `[]`. SE
  `type="select"` e `options` for vazio ou tiver um `label` inválido/duplicado, o sistema DEVE
  retornar `400` `validation_error`.
- **CA-04** *(event-driven — listar)* — QUANDO um membro que **enxerga** o projeto (CA-09 da
  fase 02) envia `GET .../projects/{projectId}/properties`, o sistema DEVE retornar as defs do
  projeto **ordenadas por `position` asc** (desempate por `id`), `200 {"properties":[…]}`. O
  projeto invisível (private de outro) responde `404` (herda a visibilidade do projeto — CA-15).
- **CA-05** *(event-driven — renomear/reordenar)* — QUANDO um usuário com escrita envia `PATCH
  .../properties/{defId}` com qualquer subconjunto de `{name, position, options}`, o sistema DEVE
  persistir os campos presentes e retornar `200` com o objeto atualizado. `name`/`options` seguem
  as validações do CA-01.1/CA-03. **`type` é IMUTÁVEL** no PATCH (mudar o tipo invalidaria os
  `value`s já gravados — decisão D-04): se `type` vier no payload, é ignorado (não é erro). Editar
  `options` de um `select` **não apaga** values cujo `id` permanece; um value que aponta para um
  `option.id` removido é tratado como "limpo" na leitura (CA-13).
- **CA-06** *(event-driven — deletar)* — QUANDO um usuário com escrita envia `DELETE
  .../properties/{defId}`, o sistema DEVE remover a def (hard delete) e **todos os
  `task_property_values` daquela def** (via `ON DELETE CASCADE`), retornando `204`. Nenhuma task é
  apagada.
- **CA-07** *(unwanted — escrita por quem não pode)* — SE um membro do projeto com
  `access=view`, um viewer do workspace, ou um member/viewer que enxerga o `shared` mas não está
  em `project_members`, enviar `POST`/`PATCH`/`DELETE` em defs desse projeto, o sistema NÃO DEVE
  alterar nada e DEVE retornar `403` `forbidden`.

### Valores de propriedade (por task) — API

> Set/clear de value: `PUT/DELETE
> .../projects/{projectId}/tasks/{taskId}/properties/{defId}` (rota aninhada na task —
> ver plan §3/§4.3). Decisão D-05: PUT idempotente (upsert), DELETE limpa.

- **CA-08** *(event-driven — set)* — QUANDO um usuário com escrita no projeto envia `PUT
  .../tasks/{taskId}/properties/{defId}` com `{ "value": <v> }` **válido para o `type` da def**,
  o sistema DEVE fazer **upsert** do `task_property_values (task_id, def_id, workspace_id, value)`
  e retornar `200 {"def_id", "value"}`. A def DEVE pertencer ao **mesmo `project_id`** da task
  (ver CA-12); a task DEVE pertencer ao projeto/workspace da rota.
- **CA-09** *(unwanted — value inválido por tipo)* — SE o `value` não casar com o `type` da def,
  o sistema NÃO DEVE escrever e DEVE retornar `400` `validation_error`. Regras por tipo (value
  **não-null**; `value:null` é tratado à parte — CA-09.1):
  - `text` → string (≤ 2000 chars). String vazia `""` é um value VÁLIDO gravável (limpar usa DELETE
    — CA-10 — ou `value:null` — CA-09.1).
  - `number` → número JSON (finito, não NaN); string numérica é **rejeitada** (`400`).
  - `date` → string `YYYY-MM-DD` válida (calendário real; `2026-13-40` → `400`).
  - `url` → string ≤ 2000 chars (não valida esquema rígido). String vazia `""` é gravável; limpar
    usa DELETE (CA-10) ou `value:null` (CA-09.1).
  - `checkbox` → boolean (`true`/`false`); qualquer outra coisa → `400`.
  - `select` → string que seja um **`option.id` existente** em `def.options`; id inexistente →
    `400` `validation_error`.
- **CA-09.1** *(event-driven — `value:null` = clear, qualquer tipo)* — QUANDO o `PUT` de value chega
  com `{ "value": null }` para **QUALQUER** `type`, o sistema DEVE tratar como **clear**: remover a
  linha de value (se houver) e retornar `204`, idêntico ao `DELETE` (CA-10) — **idempotente** (`204`
  mesmo sem value prévio). Razão: a coluna `value jsonb` é **NOT NULL** (plan §2) e não comporta
  `null` materializado; em vez de gravar `'null'::jsonb` ou dar `400`, o `null` no PUT delega pro
  clear (DELETE da linha), unificando a semântica de limpeza pra UI inline da Table/popup. O handler
  detecta o null por `string(raw) == "null"` (padrão real de `tasks_handlers.go:172`) **antes** de
  chamar `validateValue` — `validateValue` só roda pros values **não-null**.
- **CA-10** *(event-driven — clear)* — QUANDO um usuário com escrita envia `DELETE
  .../tasks/{taskId}/properties/{defId}`, o sistema DEVE remover a linha de value (se houver) e
  retornar `204` (idempotente — `204` mesmo sem value prévio). `PUT` com `{ "value": null }` tem o
  **mesmo efeito** (CA-09.1) — ambos delegam ao mesmo clear (a coluna `value` é NOT NULL).
- **CA-11** *(unwanted — escrita de value por quem não pode)* — SE um usuário sem escrita no
  projeto (viewer/`access=view`/não-membro do `shared`) enviar `PUT`/`DELETE` de value, o sistema
  DEVE retornar `403` `forbidden` e não alterar nada. SE o projeto estiver **arquivado**, o gate de
  escrita DEVE retornar `400` `error.code="invalid_reference"` (padrão real da fase 12 —
  `ensureWrite`/`loadProjectGate` devolvem `domain.InvalidReference("projeto arquivado é só
  leitura")`; **NÃO** existe code `archived` no backend; ver plan §3 D-03). O teste DEVE assertar
  exatamente `error.code === "invalid_reference"` (não `archived`).
- **CA-12** *(unwanted — def↔task↔projeto)* — SE o `{defId}` da rota não pertencer ao
  **`projectId` da rota** (def de **outro projeto**, mesmo no MESMO workspace), ou se a
  `{taskId}` não pertencer ao `projectId`/workspace da rota, ou se qualquer id for **malformado**
  (não-uuid), o `PUT`/`DELETE` de value (e o `PATCH`/`DELETE` de def) DEVE retornar `404`
  `not_found` (mesma resposta — não vazar existência). `uuid.Validate` roda antes do SQL pra
  nunca dar `500`. A carga da def SEMPRE casa `(workspace_id, project_id da rota, id)`.

### Roles, auth e isolamento — API (herda fases 02/12)

- **CA-13** *(state-driven — value órfão de option)* — Quando uma `select` tem um value cujo
  `option.id` **não existe mais** em `def.options` (option removido via CA-05), a leitura
  (`/board`, ver CA-16) DEVE tratar esse value como **ausente** (não enviar o option resolvido,
  ou enviar `null`) — nunca quebrar o render. O dado fica no banco mas é apresentado como limpo
  (à prova de erro, RNF-04 da M3).
- **CA-14** *(unwanted — sem sessão)* — SE a requisição chegar sem cookie de sessão válido,
  qualquer rota desta fase DEVE retornar `401` `error.code="unauthorized"`.
- **CA-15** *(ubíquo — teste de vazamento OBRIGATÓRIO)* — O sistema DEVE sempre filtrar defs e
  values pelo `workspace_id` da rota **e** pela visibilidade do projeto (`p.visibility='shared'
  OR p.owner_user_id=$user`): um usuário que referencie (na URL) uma def/value de **outro**
  workspace, ou de um projeto invisível, DEVE receber `404` — nunca os dados do outro tenant.
  Toda query nova carrega `workspace_id` no WHERE (D-01).

### Tasks novas herdam o schema (RF-11)

- **CA-16** *(state-driven — schema herdado no /board e na task)* — O `/board`
  (`GET .../projects/{projectId}/board`) DEVE devolver, além do que já entrega (fase 12), o
  **schema de propriedades do projeto** e os **valores por task**. O contrato (ver plan §4.2):
  - `project.property_defs: [{id, name, type, options, position}]` (uma vez, o schema do projeto);
  - cada `Task` (em objetivos e em `general`) ganha `property_values: [{def_id, value}]` com **só
    as defs que têm value** para aquela task (defs sem value são derivadas no cliente a partir do
    `property_defs` — toda task "herda" o schema na UI). Para `select`, o backend pode resolver o
    `option` (label/color) ou entregar o `id` cru e o cliente resolve via `def.options` (decisão
    do plan §4.2). Value órfão de option → omitido/`null` (CA-13).
  - **Sem N+1** (RNF-02): **uma** query de defs do projeto + **uma** query de values
    (`WHERE workspace_id=$ AND def_id = ANY($defs do projeto)`), agrupadas por `task_id` em
    memória (D-06).
- **CA-17** *(state-driven — task nova sem value)* — Uma task recém-criada num projeto que tem
  defs DEVE aparecer no `/board` com `property_values: []` (nenhum value), mas a UI a renderiza
  com **todas** as defs do `property_defs` (vazias) — a herança do schema é por **derivação no
  cliente** (RF-11), não por materialização de linhas vazias no banco (zero linha por task até
  preencher).

### Performance

- **CA-18** *(perf — verificação informativa, fora do red→green)* — Com 10 defs, 300 tasks e
  até 3.000 values num projeto, `GET .../projects/{id}/board` DEVE continuar respondendo em
  **< 200 ms** no ambiente local de teste (melhor de 3). Nasce verde; se falhar, a correção é
  índice/SQL (não segue o ciclo red→green — exceção documentada, igual CA-18 da fase 12). O índice
  `(def_id)` + a query única de values por `ANY($defs)` garantem (RNF-02 / D-06).

### UI — Table com colunas custom + popup da task

- **CA-19** *(event-driven — Table)* — Na aba **Tasks → Table**, além das colunas nativas
  (status/priority/due/to-dos/time — fase 24), a tabela DEVE renderizar **uma coluna por def** do
  projeto (`property_defs` do `/board`), na ordem `position`. Cada célula renderiza o value da
  task **por tipo**: `text`/`number`/`url` = input texto; `date` = date-picker; `checkbox` =
  toggle; `select` = dropdown das `options` (com a cor da option). Tasks sem value mostram o campo
  vazio (CA-17).
- **CA-20** *(event-driven — edição inline)* — QUANDO o usuário **com escrita** edita uma célula
  custom na Table (digitar/escolher/marcar) e confirma (blur/Enter/change), a UI DEVE chamar `PUT
  .../tasks/{taskId}/properties/{defId}` (ou `DELETE` se esvaziou) e refletir **sem recarregar a
  página** (invalida o `boardKey`). Value inválido pro tipo é barrado no client antes do PUT
  quando trivial (number/date), mas o backend é a autoridade (CA-09).
- **CA-21** *(event-driven — popup central da task: linha custom + Add property)* — No **popup
  central da task** (o componente de popup central que a **fase 24** introduz e que reúne título +
  propriedades nativas + descrição .md + sidebar de sub-tasks; a fase 25 confirma o nome real do
  componente no build — ver plan §8), abaixo das propriedades nativas, a UI DEVE renderizar **uma
  linha por def** do projeto com o nome da def e o editor de value por tipo (`mockup` 528–531
  referenciado só como inspiração visual, não como contrato de classe), e um botão **"Add
  property"** (`mockup` 553, 571). Clicar em "Add property" DEVE **criar uma def no projeto** (`POST
  .../properties` com nome temporário/vazio + tipo padrão `text`) e abrir o campo pra nomear/tipar;
  remover (✕) DEVE chamar `DELETE .../properties/{defId}` (remove a def do **projeto inteiro**, com
  confirmação — não só desta task). Editar o value DEVE chamar `PUT .../tasks/{taskId}/properties/
  {defId}` da task.
- **CA-22** *(state-driven — viewer)* — ENQUANTO o usuário logado for `viewer` (ou `access=view`
  no projeto, ou o projeto estiver arquivado), a UI NÃO DEVE exibir ações de escrita de
  propriedade (criar/renomear/reordenar/deletar def; editar value) — só leitura (campos
  desabilitados/sem botões). A UI deriva a permissão de **`board.project.can_write`** (fase 12,
  CA-16), **não** da role do workspace sozinha. (Backend já nega — CA-07/11.)

### E2E

- **CA-23** — Logado (stub da fase 01) num projeto `shared` com escrita: o usuário **abre uma
  task**, clica **"Add property"**, cria uma propriedade `select` "Cliente" com 2 opções, **define
  o valor** de uma opção numa task; abre a aba **Table** e vê a coluna "Cliente" com o valor;
  **edita o valor inline** na Table (troca a opção) e recarrega — o valor persiste; **deleta a
  def** e a coluna some da Table e o valor some da task. **O gating de `viewer`/`access=view` NÃO é
  asserido no E2E** (o `googlestub` autentica só o owner — limitação herdada das fases 12/18); o
  **CA-22** é coberto por **teste de componente jsdom** (`can_write=false` mockado) nas T-10/T-11.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-05 (M3) | Table lista tasks com colunas custom editáveis inline | CA-16, 17, 19, 20 |
| RF-11 (M3) | Adicionar/remover propriedades do projeto; novas tasks herdam o schema; valor por task | CA-01..06, 08, 10, 17, 21 |
| RF-A | Validação por tipo no backend (text/number/select/date/url/checkbox) | CA-02, 03, 09, 13 |
| RF-B | Roles/visibilidade/isolamento herdados (defs e values) | CA-07, 11, 12, 14, 15, 22 |
| RF-C | Entrega de schema+values junto do `/board`, sem N+1, < 200ms | CA-16, 18 |

## 5. Requisitos não-funcionais

- **RNF-01 (leveza / anti-Notion-creep)** — zero dependência Go nova além do baseline (chi +
  pgx + `google/uuid`); validação na mão com stdlib (D-07 da fase 02/12). **Só os 6 tipos da v1**
  (`person`/`multi_select` adiados — §6). Frontend reusa TanStack Query + componentes próprios
  sobre `tokens.css`. Nada de lib de "table/datagrid" nova; a Table é HTML + inputs nativos
  (`mockup` usa `<table class="ntable">` + `<input type="date">`/checkbox/dropdown).
- **RNF-02 (perf / sem N+1)** — o `/board` é **um** request (CA-16/18): defs do projeto em
  **uma** query; values em **uma** query (`WHERE workspace_id=$ AND def_id = ANY($defs)`),
  agrupados por `task_id` em memória em O(n). Índice `task_property_values (def_id)` (e a PK
  `(task_id, def_id)`) cobrem a leitura. **Nada de loop de query por task/def** (N+1 proibido).
- **RNF-03 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): colunas/células
  da Table, chips de option e linhas de propriedade usam tokens existentes (`--surface-*`,
  `--border`, `.tt-card`, `.chip`). Cor de **option de select** é dado de domínio (via
  `lib/palette.ts`, exceção sancionada da fase 02) — espelha a cor de projeto. Sem hex literal em
  `.tsx` fora dessa exceção.
  - **Classes de layout `.ntable`/`.prop`/`.prop-k`/`.prop-v`/`.prop-val`** (do `mockup`): **NÃO
    existem hoje** no CSS real (`tokens.css` tem 0 ocorrências delas; só estão no `<style>` do
    mockup). **Quem porta essas classes pro CSS real é a fase 24** (Tasks 2.0), que é dona da Table
    e do popup que as usam — a fase 25 **só consome** (opção A da §6/§8 do plan, decisão de
    coordenação cross-fase). **Pré-requisito DURO da fase 24** (ver plan §8): antes da T-11 da fase
    25, essas classes DEVEM estar no CSS real (`styles/index.css` ou `tokens.css`). Se a fase 24 não
    portar (gap), a T-11 da fase 25 assume o porte e a T-11 ganha um **teste de presença** (asserção
    de string em `index.css`/`tokens.css` das classes `.ntable`/`.prop-k`/`.prop-v`, padrão da
    asserção de presença de `--font-ui` da fase 24 — `check-cores` NÃO detecta classe ausente). A
    fase 25 **não referencia classes cruas do mockup como contrato**; descreve a UI por
    comportamento (coluna por def, linha de propriedade no popup) e confirma os nomes reais de
    classe/componente da fase 24 no build (T-11).
- **RNF-04 (fonte)** — assume **Inter** (a troca Geist→Inter é da fase 24, a primeira UI-heavy
  da M3 — `MILESTONE-3.md` decisão 11); esta fase não troca fonte, só consome.
- **RNF-05 (migração)** — a fase 25 **é dona da migração `0023_task_properties`** (número FIXO do
  PRD §7 — NÃO "ler o próximo livre"; a fase 24 é dona de `0021`/`0022`). Cria `project_property_defs`
  + `task_property_values`. Nenhuma outra tabela. **NÃO** toca `tasks`/`objectives`/`projects`
  (intocados — RNF-05 herdado).
- **RNF-06 (regressão zero)** — fases 02/12 (tasks/board/objetivos), 18–21 (multiplayer), 24
  (Tasks 2.0) permanecem **funcionais**: esta fase só ADICIONA `project_defs` + `property_values`
  no payload do `/board` (campos a mais; o board da 12/24 não muda de forma) e a subárvore
  `/properties` + `/tasks/{id}/properties`. **O `/board` ganha 2 campos novos, não muda os
  existentes.**
- **RNF-07 (mobile)** — a Table com muitas colunas custom DEVE rolar horizontalmente em < 768
  (`overflow:auto` no `.tt-panel`, padrão do `mockup` 423/454); o popup da task já é responsivo
  (fase 24).
- **Segurança/multi-tenant** — toda rota atrás do middleware sessão+membership; recurso
  inexistente e invisível respondem o **mesmo** `404`; CA-15 inegociável (SDD §8.4).
- **PT-BR na UI**; identificadores de código em inglês (nomes de teste em PT-BR são exceção
  registrada).

## 6. Fora de escopo (fronteiras explícitas)

- **Tipos `person` e `multi_select` (fatia 25.1 — fora-de-escopo-da-v1, `MILESTONE-3.md §9.1`):**
  a coluna `type` e o CHECK da migração **já listam** os 8 tipos (`text·number·select·multi_select·
  date·person·checkbox·url`) e `options jsonb` já comporta a estrutura de `multi_select` — mas o
  **service da v1 rejeita** (`400`, CA-02) criar def com `type ∈ {person, multi_select}`, e a UI
  da v1 **não** os oferece. Isso é deliberado: a v1 começa enxuta e cresce por dor (régua
  anti-Notion-creep). A fatia 25.1 só liga a UI/validação desses 2 tipos (schema já pronto).
- **Mudar o `type` de uma def existente** — `type` é IMUTÁVEL no PATCH (D-04): mudar o tipo
  invalidaria os `value`s já gravados (jsonb tipado). Para mudar de tipo, deletar e recriar a def.
- **Propriedades NATIVAS como custom** — Status/Priority/Due/Time/Notion são colunas próprias da
  fase 24/0021, **nunca** entram em `project_property_defs`.
- **Fórmulas / rollups / relações entre databases** (Notion-completo) — fora; régua anti-NOA.
- **Drag-and-drop pra reordenar colunas/defs** — `position` é editável via `PATCH` (CA-05); a UI
  de reordenar fica simples (mover ‹ › ou nem isso nesta fase — decisão do plan). Sem lib de DnD.
- **Filtrar/ordenar a Table por propriedade custom** — a Table só **exibe e edita** as colunas
  nesta fase; filtros/sort por propriedade = fase futura (YAGNI).
- **Slash `/` criando propriedade** — a captura por slash é só nas **Notas** (fase 26); a
  descrição da task é markdown simples (`MILESTONE-3.md §9.4`). Propriedade nasce só pelo botão
  "Add property" (CA-21).
- **Sub-tasks / Goals com modos / Spotlight / Calendário / Notas** — outras fases da M3 (24, 29,
  27, 28, 26).
