# MILESTONE 3 — "Núcleo de Organização" · PRD FECHADO

> Status: **PRD FECHADO (22/06/2026)** — decisões travadas via HITL + validadas no mockup funcional
> `mockups/projeto-workspace.html` (a **fonte visual da verdade** desta milestone).
> Próximo passo: escrever **specs por fase** (`specs/NN-*/`) ANTES de qualquer código (Regra Nº1).
> Herda `CLAUDE.md` (🧭 VISÃO ATUAL + perfil TDAH), `REQUISITOS.md`, `DESIGN.md`. PT-BR · HITL ·
> identificadores de código em inglês.

---

## 0. O que mudou em relação ao brainstorm (ler primeiro)

O brainstorm original (rodadas A–E) tratava o editor estilo Notion como **anti-NOA / fora**. Depois de
construir o mockup junto com o Diogo, a direção foi **deliberadamente expandida**: estamos **recriando o
Notion dentro do Timetrack**, com a camada de **tempo** por cima, no nosso próprio ambiente e com
liberdade total. Esta decisão é consciente e **supersede** a fronteira anterior de "editor simples".

**A frase que define a milestone (Diogo):** _"As tasks no Timetrack são quase só pra eu me organizar;
a documentação real eu escrevo e linko numa página do Notion. O que estamos fazendo é recriar o Notion
dentro da estrutura do Timetrack."_

→ Task no Timetrack = **índice executável + tempo gasto**. Doc longa = **link pro Notion**. Notas curtas
e captura = **nativas (.md no Postgres)**.

---

## 1. Por que esta milestone existe

O Timetrack é **forte no presente** (cronometrar o agora) e **vazio no planejamento**. A tela de Projetos
mostra cards mortos (`0h` hardcoded desde a fase 02). A organização real do Diogo (TDAH) vive no papel.

A M3 vira o tracker num **OS de organização pessoal** (segundo cérebro): adiciona a camada de
**planejamento** (projetos ricos · tasks Notion-like · notas .md · objetivos · calendário) + uma **porta de
captura < 2s** (Spotlight / Cmd+K), **sem tocar no cronômetro**. Diferencial que Notion/Todoist não têm:
**tudo já vem grudado com o tempo real gasto**.

**Lei de design (decide tudo):** cérebro **TDAH** — object permanence, captura < 2s, à prova de culpa,
revisão diária leve, dopamina imediata. Uso **pessoal primeiro** (dogfooding é a validação).

---

## 2. Os dois mundos (modelo central — intocado)

| Mundo | É | M3 |
|---|---|---|
| **TRACKING** | cronômetro = "o que faço AGORA" (label + projeto) | **intocado** |
| **PLANNING** | "o que preciso fazer / lembrar / anotar" (projetos · tasks · notas · objetivos) | **é o que se constrói** |

**O elo:** ao cronometrar, poder **puxar uma task planejada** → o tempo **gruda nela**. O label livre
rápido continua existindo. (O elo timer↔task é evolução natural; ver §9, fica no fim da fila.)

---

## 3. Decisões FECHADAS desta milestone (NÃO reabrir)

1. **Direção = Notion-dentro-do-Timetrack** (com liberdade total). Editor Notion-like é **dentro** agora.
2. **Propriedades de task = TIPADAS (Notion-full):** texto · número · seleção · multi-seleção · data ·
   pessoa · checkbox · URL. **Customizáveis por projeto** (cada projeto é quase uma "database" Notion).
3. **Status da task (4):** `Backlog · To do · In progress · Done`. (Hoje produção tem só todo/doing/done.)
4. **Prioridade:** `Sem prioridade · Baixa · Média · Alta`.
5. **Sub-tasks = filhas da task, no estilo "goals da task":** lista estruturada **dentro** da task
   (título + feito + ⭐ é-meta). A ⭐ marca a sub-task como **meta DAQUELA task** — **separado** dos
   **Goals/objetivos do projeto** (são dois níveis diferentes, não unificar).
6. **Doc longa = link do Notion** (só link + **preview bonito**: mark "N" + título derivado do slug +
   `notion.so`). **Sem** API/OAuth do Notion nesta milestone (nível 1, YAGNI).
7. **Notas = nativas (.md, 1 linha/nota no Postgres)**, editor **sempre editável** (sem botão Editar),
   com **slash `/`** criando **task** e **sub-nota** ligadas. Markdown simples (não bloco-a-bloco Notion).
8. **Spotlight (Cmd+K)** = porta única de captura no **mesmo popup central**. `/task` `/note` `/timer`
   `/goal`; `/task <nome>` cria já nomeada e **pula pro título** (edição fluida, Enter→corpo).
9. **Task abre em popup central**, com **sub-tasks numa sidebar à direita**, **expansível pra tela cheia**.
10. **Calendário por projeto = read-only** (deadlines de task/objetivo do projeto + overlay do GCal que já
    existe). **Sem write-back** (decisão herdada do CLAUDE.md).
11. **Fonte = Inter** (a do Notion) em toda a UI; mono só pra números/tempo.

**Escopo da milestone (tudo dentro):** ① Workspace de projeto (abas · board editável · lista · tabela) ·
② Notas .md + slash · ③ Spotlight Cmd+K · ④ Calendário por projeto + Goals com modos.

---

## 4. Modelo de dados (aterrado no schema atual)

> Convenções herdadas: `workspace_id` denormalizado pra filtro tenant (D-09); migrações numeradas a
> partir de **0021**; identificadores em inglês; soft-delete via `archived_at` onde já existe.

### 4.1 `tasks` (ALTER) — fase de Projetos
- `ALTER TYPE task_status ADD VALUE 'backlog' BEFORE 'todo'` → enum vira `backlog·todo·doing·done`.
  Rótulos na UI: `backlog→Backlog`, `todo→To do`, **`doing→In progress`**, `done→Done` (NÃO renomear o
  valor `doing` no banco — só o rótulo; evita migração destrutiva).
- `+ priority smallint NOT NULL DEFAULT 0` (0=sem · 1=baixa · 2=média · 3=alta).
- `+ notion_url text` (NULL = sem doc).
- `+ position integer NOT NULL DEFAULT 0` (ordenação manual / drag-and-drop no board e na lista).
- **Descrição (.md) reusa `tasks.notes`** (já existe, `text`) — é o corpo markdown sempre-editável. (O
  `recurrence` já existe; comentários/thread já existem da fase 19, ficam intocados.)

### 4.2 `task_subtasks` (NOVA tabela) — sub-tasks "goals da task"
```
task_subtasks(
  id uuid pk, task_id uuid fk→tasks ON DELETE CASCADE,
  title text, done boolean default false,
  is_goal boolean default false,          -- a ⭐: marca como meta da task
  position integer default 0, created_at timestamptz )
```
Progresso de metas da task = `done where is_goal` / `count is_goal`. **Não** referencia `objectives`.

### 4.3 Propriedades tipadas (NOVAS tabelas) — "database por projeto" ⚠️ peça mais cara
```
project_property_defs(
  id uuid pk, project_id uuid fk→projects ON DELETE CASCADE,
  name text, type text,                   -- text·number·select·multi_select·date·person·checkbox·url
  options jsonb default '[]',             -- p/ select/multi_select: [{id,label,color}]
  position integer default 0 )

task_property_values(
  task_id uuid fk→tasks ON DELETE CASCADE,
  def_id  uuid fk→project_property_defs ON DELETE CASCADE,
  value   jsonb,                          -- tipado conforme def.type
  primary key(task_id, def_id) )
```
- Status/Priority/Due/Time/Notion são propriedades **nativas** (colunas em §4.1), **não** entram aqui;
  custom é só pro que o Diogo criar (ex.: "Cliente", "Estimativa", "Energia").
- ⚠️ **Risco:** é a peça de maior custo (UI por tipo + validação + render na tabela). Candidata a
  **fatiar**: v1 = `text·select·date·number·url·checkbox`; `person`/`multi_select` numa fatia seguinte.

### 4.4 `notes` (NOVA tabela) + `note_links` — fase de Notas
```
notes(
  id uuid pk, workspace_id uuid fk, project_id uuid fk NULL, task_id uuid fk NULL,
  title text, body_md text not null default '',
  created_by uuid fk, created_at timestamptz, updated_at timestamptz )

note_links(                                -- refs criadas pelo slash / (chips clicáveis)
  note_id uuid fk→notes ON DELETE CASCADE,
  ref_type text,                           -- 'task' | 'note'
  ref_id uuid, position integer )
```
1 linha por nota, markdown puro. Nota pode ser do **projeto** (project_id) ou **solta**; pode atrelar a
uma **task**. `/task`/`/note` no editor criam a entidade e gravam um `note_links`.

### 4.5 `projects` (ALTER) — card vivo + calendário
- `+ deadline date` (NULL = sem prazo). Alimenta card vivo, "tem algo hoje" e o calendário do projeto.

### 4.6 `objectives` → **Goals** (ALTER) — Goals com modos
- `+ completion_mode text NOT NULL DEFAULT 'by_tasks'` (`by_tasks` · `by_hours` · `by_date`).
- `+ target_date date` (usado quando `by_date` = **milestone**; objetivo com data = marco do roadmap).
- Mantém auto-close a 100% (dopamina pronta da fase 12).

### 4.7 Calendário por projeto — **sem tabela nova**
Derivado: `tasks.due_date` + `objectives.target_date` + `projects.deadline` do projeto, **+ overlay**
dos eventos do Google Calendar (leitura já existe, fase 06). 1 endpoint novo agrega os itens datados do
projeto por mês.

---

## 5. Requisitos funcionais (EARS) por área

### 5.1 Workspace do projeto (abas)
- **RF-01** Ao abrir um projeto, o sistema deve exibir abas **Overview · Tasks · Notes · Goals · Calendar**.
- **RF-02** Overview deve mostrar KPIs (tempo total · esta semana · pendentes · % done), **próximos
  prazos**, **andamento por status** e **goals**, e **notas recentes** — tudo do projeto.
- **RF-03** Tasks deve oferecer 3 visões persistidas (`tt.view.board`): **Board · List · Table**.
- **RF-04** No Board, o usuário deve poder **arrastar** um card entre colunas de status (drag-and-drop) →
  persiste `status` (e `position`). Sem lib pesada de DnD se evitável.
- **RF-05** A visão **Table** deve listar tasks com colunas status/priority/due/to-dos/time **+ as
  propriedades custom** do projeto, editáveis inline.
- **RF-06** Toda visão deve ter **quick-add** (criar task no status/coluna em 1 gesto).

### 5.2 Task (popup central + sidebar)
- **RF-07** Ao abrir/criar uma task, o sistema deve exibir um **popup central** com: título (grande,
  editável), **propriedades** (Status · Priority · Due · Time · Notion · custom), **descrição .md** e uma
  **sidebar à direita** com as **sub-tasks**.
- **RF-08** O popup deve ser **expansível pra tela cheia** e voltar ao tamanho central (toggle).
- **RF-09** A descrição deve ser **sempre editável** (sem botão Editar); **Enter no título** move o foco
  pro corpo (edição fluida).
- **RF-10** Status e Priority devem ser editáveis em 1 clique; Due por date-picker; Time mostra o tempo
  rastreado real + botão **Start** (cronometra a task — elo timer↔task, ver §9).
- **RF-11** O usuário deve poder **adicionar/remover propriedades** do projeto (tipadas) e preencher o
  valor por task; novas tasks do mesmo projeto herdam o **schema** de propriedades.
- **RF-12** Sub-tasks: adicionar (Enter), marcar feito, e marcar **⭐ = meta da task**; o cabeçalho mostra
  `feitas/total` e `metas feitas/total`.
- **RF-13 (Notion)** O usuário deve poder colar um **link do Notion**; o sistema exibe um **preview**
  (mark "N" + título derivado do slug + `notion.so`) com "abrir em nova aba" e desvincular. Tasks com
  link mostram um **ícone Notion** no card/lista/tabela.

### 5.3 Notas (.md + slash)
- **RF-14** Notes deve listar as notas do projeto (visão lista Notion: Nota · Prévia · Atualizada) e
  permitir **criar** (New note).
- **RF-15** Ao abrir uma nota, o editor deve estar **sempre editável** (markdown), em popup central
  expansível.
- **RF-16** Digitar **`/`** deve abrir um menu de blocos: **Task · Note · To-do · Heading · Divider**.
- **RF-17** `/task` deve **criar uma task** no projeto e gravar um **link** (chip clicável no topo da nota);
  `/note` cria **sub-nota** ligada. Os chips abrem a entidade.

### 5.4 Spotlight (Cmd+K)
- **RF-18** **Cmd+K** (e botão no shell + mobile) deve abrir o **popup central** em modo Spotlight: input +
  comandos `/task` `/note` `/timer` `/goal`.
- **RF-19** Sem comando, edita a descrição+projeto do **timer atual**: **ocioso** → inicia um count-up com
  o texto digitado; **rodando** → edita a descrição/projeto da entry rodando.
  **✅ Decisão (23/06):** como `entries.Patch` recusa entry rodando (`409 entry_running`), a fase 27 ganha
  uma **allowance mínima de backend** — atualizar **descrição/projeto/tags** numa entry RODANDO
  (`started_at`/`stopped_at`/`duration`/`mode` **INTOCADOS**; propaga pelo broadcast da fase 04). Mudar os
  **horários** de uma entry rodando segue recusado. A fase 27 deixa de ser 100% frontend (+1 ajuste no
  caminho de edição do timer).
- **RF-20** `/task <nome>` + Enter deve **criar a task já nomeada** e abrir o editor com o título focado
  (captura < 2s; enriquecer é opcional).

### 5.5 Calendário do projeto
- **RF-21** Calendar deve renderizar um mês com os **itens datados do projeto** (task `due_date`,
  objetivo `target_date`, `projects.deadline`) nos dias certos, navegável por mês.
- **RF-22** Deve sobrepor (read-only) os **eventos do Google Calendar** já sincronizados (fase 06).
- **RF-23** Clicar num item datado deve abrir a task/objetivo correspondente.

### 5.6 Goals (objetivos) com modos
- **RF-24** Um objetivo deve ter **modo de conclusão**: por **tasks** (% done), por **horas** (`goal_hours`)
  ou por **data** (`target_date` = milestone).
- **RF-25** Goals deve ter visão **lista** (Goal · Progresso · Tasks · Horas · Status) e fechar
  automaticamente a 100% (dopamina).

---

## 6. Não-funcionais
- **RNF-01** Não regredir o **TRACKING** (cronômetro/sync/Pomodoro/foco) — zero-diff onde não for o elo.
- **RNF-02** **Captura < 2s** é lei: nenhum fluxo de criação pode exigir mais que nome (+projeto).
- **RNF-03** **À prova de culpa**: nada zera, nada fica vermelho acusatório; atrasado = "reagenda num
  toque".
- **RNF-04** Multi-tenant: toda query nova filtra por `workspace_id`/projeto (nada vaza entre workspaces).
- **RNF-05** Listagens do board/projeto < 200ms (padrão das fases anteriores).
- **RNF-06** Mobile responsivo (a M2 já estabeleceu o breakpoint 768).
- **RNF-07** Segue o gate `make check-full` verde + verificação adversarial (spec E build) por fase.

---

## 7. Decomposição em fases (maior alavanca primeiro)

| Fase | Entrega | Migrações | Risco |
|---|---|---|---|
| **24 — Tasks 2.0** | status (backlog), priority, position, descrição editável, popup central + sidebar de sub-tasks, board com **drag-and-drop**, link **Notion** + preview | `0021` tasks ALTER + `0022_task_subtasks` | médio |
| **25 — Propriedades tipadas** | "database por projeto": defs + valores, edição por tipo, coluna na Table | `0023_task_properties` | **alto** (fatiável) |
| **26 — Notas nativas** | tabela `notes` + `note_links`, editor sempre-editável, **slash `/`** criando task/sub-nota | `0024_notes` | médio |
| **27 — Spotlight (Cmd+K)** | porta única de captura no popup central; `/task` `/note` `/timer` `/goal` | — (consome endpoints) | baixo |
| **28 — Projeto vivo + Calendário** | abas (Overview/Calendar), `projects.deadline`, calendário por projeto + overlay GCal, cards vivos | `0025_projects_deadline` + 1 endpoint | médio |
| **29 — Goals com modos** | `completion_mode` + `target_date`, visão lista, dopamina ao fechar | `0026_objectives_modes` | baixo |
| **30 — Elo timer↔task** | botão Start na task cronometra puxando-a (o tempo gruda); `timer.StartInput` **já aceita `task_id`** → quase só frontend | — (sem migração) | baixo |

> **Uma fase entrega UMA coisa e PARA.** **Build order TRAVADO (HITL, sessão 22): 24 → 26 → 27 → 28 → 29 →
> 25 → 30** (propriedades tipadas por último/fatiada pelo custo; elo timer↔task = fatia própria depois da 24).
> Migração é DONA por FASE (não por ordem de build). Specs das 7 fases ESCRITAS + verificadas (ver §12).

---

## 8. Fora de escopo (anti-NOA — fronteiras explícitas)
API/OAuth do Notion (só link agora) · editor bloco-a-bloco arbitrário (drag de blocos, embeds ricos) ·
roadmap **Now/Next/Later** entre projetos (vira backlog) · streak/gamificação · write-back no Google
Calendar · multiplayer/compartilhar projeto · finanças · IA de insights · hábitos · servidor caseiro/vault.

---

## 9. Pontos a calibrar / riscos (registrados)
1. **Propriedades tipadas (fase 25)** é o maior custo e a maior chance de virar "Notion-completo"
   anti-NOA. Régua: só os tipos que o Diogo realmente usa no dogfooding; começar enxuto e crescer por dor.
2. **Elo timer↔task** (cronometrar puxando a task → tempo gruda): desejado, mas **não bloqueia** as
   fases 24–29. Entra como extensão do `start` quando o popup de task já existir (provável fase 24 ou
   fatia própria). Hoje o botão **Start** no modal é mock.
3. **`ALTER TYPE … ADD VALUE`** não roda dentro de transação no Postgres antigo — a migração 0021 precisa
   tratar isso (committar o ADD VALUE antes de usá-lo). Verificar na spec da fase 24.
4. **Slash em notas vs. descrição de task**: nesta milestone o `/` cria blocos só nas **Notas**; o `/`
   dentro da **descrição da task** fica pra depois (a descrição da task é markdown simples).

---

## 10. Fonte visual da verdade
**`mockups/projeto-workspace.html`** — protótipo funcional (em memória) que validou: abas, board
editável, popup central + sidebar de sub-tasks, expandir tela cheia, propriedades, descrição fluida,
notas com slash, Spotlight Cmd+K, preview do Notion. **Buildar pro mockup.**

---

## 11. Próximo passo
~~Escrever as specs da fase 24~~ ✅ **FEITO (sessão 22, 22/06):** specs das **7 fases (24–30)** escritas,
reconciliadas e verificadas adversarialmente (ver §12). **Próximo passo agora = BUILD da M3** — prompt em
`KICKOFF.md ## Prompt M3 — build`. Regra Nº1 já cumprida na camada de spec; o build segue a ordem travada.

---

## 12. ETAPA 0 resolvida + verificação adversarial (sessão 22, 22/06)

**Resíduos da ETAPA 0 (HITL, AskUserQuestion) — TRAVADOS:**
1. **Build order = 24 → 26 → 27 → 28 → 29 → 25 → 30** (propriedades tipadas por último pelo custo/risco
   de "Notion-completo"; elo timer↔task = fatia própria depois da 24).
2. **Propriedades tipadas v1 (fase 25) = `text · select · date · number · url · checkbox`.** `person` e
   `multi_select` ficam na **fatia 25.1** (o schema `options jsonb` já comporta; só a UI/validação é adiada).
3. **Elo timer↔task = fase 30 (fatia própria), depois da 24.** Confirmado pelo código: `timer.StartInput` já
   tem `task_id` e `Start` valida via `ValidateRefs` + grava `time_entries.task_id` → a 30 é majoritariamente
   frontend (botão Start mock da 24 vira real) + não-regressão do TRACKING (RNF-01).

**Specs (ETAPA 1):** 7 fases (24–30) × `spec.md`+`plan.md`+`tasks.md` escritas em paralelo (1 agente/fase),
ancoradas no **código real** (tasks/projects/objectives/board/calendar/timer) e no mockup, + passe de
consistência (migrações 0021–0026 por fase, `board.go`/tab-shell aditivos, rotas aninhadas).

**Verificação adversarial (ETAPA 2 — workflow ultracode, 125 agentes, finder/fase → 2 refutadores céticos
por furo):** 59 furos brutos → **33 confirmados 2/2** (24:7 · 25:3 · 26:7 · 27:5 · 28:3 · 29:5 · 30:3) + 3
splits, **todos aplicados nas specs ANTES de fechar** (registro no fim de cada `tasks.md`). HIGH corrigidos:
- **27-H1:** `entries.Patch` rejeita entry RODANDO (`409`) → RF-19 rescopado (ver §5.4); edição do timer
  rodando = fatia futura.
- **30-H2:** alinhado o contrato 24↔30 (a 24 entrega o botão Start **mock** `data-testid=task-start`; a 30
  só troca a ação — sem slot `TaskTimeControl` fantasma).
- **26-H1:** slash `/task` nasce `todo` (o `tasks.Create` não recebe status; `backlog` é `PATCH` depois).
- **25-H1:** projeto arquivado no gate de escrita → `400 invalid_reference` (não o inventado `archived`).
- **24-H3:** sub-tasks no **mount Core** (`RequireWriteRole`+`loadProjectGate`), sem carve-out viewer.
- **29-H1/H2:** `by_hours` coerente no **bucket per-usuário do `/board`** (auto-close = progress_pct).

**Decisões de coordenação cross-fase (valem no build):** A popup central = componente `CenterPanel` (dono =
24; 26/27 reusam) · B botão Start mock `task-start` na 24, wirado pela 30 · C slash `/task` = `todo` · D
`tasks.CreateTx` aditivo (26 não refatora `Create`) · E `by_hours` per-usuário · F sub-tasks mount Core · G
códigos de erro canônicos reais · H stores em `src/lib/`, `ProjectCard` em `components/projects/`.

**Decisões fechadas (23/06, HITL):** (1) edição do timer RODANDO (27-H1) = **allowance mínima de backend**
(PATCH descrição/projeto/tags numa entry rodando; timing intocado; propaga pela fase 04) → a fase 27 deixa
de ser 100% frontend; (2) `person`/`multi_select` (fase 25) = **backlog, por dor** (entram quando faltarem
no uso real; o `options jsonb` já comporta); (3) `by_hours` = **bucket per-usuário do `/board`** agora
(all-users = multiplayer/futuro). Nenhuma bloqueia o build.
