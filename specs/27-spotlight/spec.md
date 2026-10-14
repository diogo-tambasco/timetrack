# spec.md — Fase 27 · Spotlight (Cmd+K) — porta única de captura < 2s

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green).
> **Quarta fase da Milestone 3** ("Núcleo de Organização" — `MILESTONE-3.md §7`). Cobre
> **RF-18, RF-19, RF-20** (`MILESTONE-3.md §5.4`). É a **porta de captura < 2s** (RNF-02 — lei).
> **FRONTEND + 1 ALLOWANCE MÍNIMA DE BACKEND**, ZERO migração — consome o que já existe e
> adiciona **uma única rota nova** por-usuário (`PATCH /api/v1/timer`) pra editar SÓ
> descrição/projeto/tags da entry RODANDO (allowance da decisão HITL de 23/06 — ver §6 e 27-H1):
> - o **componente de popup central da fase 24** (o painel central com chrome side↔full,
>   acionado pelo abridor de task `openTask` / pelo estado de painel do shell — ver D-09),
> - o popup de **nota** da fase 26 (reusa o **mesmo componente de popup central da fase 24**;
>   abridor `openNote`),
> - o `timerStore` da fase 03 (`start`/`useTimerStore`) p/ **iniciar** o count-up,
> - o CRUD de **objetivos** da fase 12 (`createObjective`, rota aninhada já existe),
> - a **allowance nova** `PATCH /api/v1/timer` (rota por-usuário, roteada pelo `timer.Service`)
>   p/ editar a descrição/projeto/tags do **timer atual** (propaga pelo broadcast da fase 04).
> **Decisão de coordenação A (cross-fase):** o popup central é um **componente React** introduzido
> pela fase 24 (ex.: `CenterPanel`), com tokens.css. NÃO se referencia classes cruas do mockup
> (`.mscrim`/`.scard`/`.swrap`) como contrato — fases 26 e 27 **reusam o componente de popup central
> da 24**.
> **Decisões travadas (`MILESTONE-3.md §3.8 / §3.9 / §10`):** Spotlight vive no **mesmo popup
> central**; captura mínima (nome+projeto → Enter), enriquecer é opcional e depois.
> **⚠️ Edição do timer rodando (27-H1) — DECISÃO HITL 23/06: ALLOWANCE MÍNIMA DE BACKEND.** O
> backend `entries.Patch` **REJEITA** PATCH de entry rodando (`409 entry_running` —
> `backend/internal/entries/service.go:184-187`), e a TimerBar de hoje torna a descrição do timer
> rodando **read-only** justamente por isso (`TimerBar.tsx:156-157,176`). A 27 **deixa de ser
> 100% frontend**: adiciona **uma rota por-usuário** `PATCH /api/v1/timer` roteada pelo
> `timer.Service` que edita **SOMENTE** `description`/`project_id`/`tags` da entry RODANDO do
> usuário, **dentro do advisory lock do timer** (`pg_advisory_xact_lock(hashtext(user_id))`),
> validando `project_id` via `entries.ValidateRefs` e **disparando `OnChange`** (o broadcast da
> fase 04 propaga o rótulo novo a TimerBar/Foco/Remoto ao vivo). Os campos de **TEMPO**
> (`started_at`/`stopped_at`/`duration`/`mode`/`target`) seguem **RECUSADOS** numa entry rodando
> (não se relaxa isso — `entries.Patch` continua dando `409` pros caminhos de edição manual). Com
> timer rodando, o sem-comando do Spotlight passa a **editar** descrição+projeto do timer atual
> (CA-13); com o timer **ocioso**, sem-comando **inicia** o count-up (CA-14).
> **Fonte visual (referência, NÃO contrato):** `mockups/projeto-workspace.html` linhas 624–654
> (`SPOT`/`openSpotlight`/`spotItems`/`spotTitle`/`spotKey`/`spotlightPanel`/`runSpot`) + 676–679
> (listener de teclado: `Cmd/Ctrl+K` abre, `Esc` fecha) + 153 (botão `⌘K` no shell). As classes
> cruas do mockup (`.mscrim`/`.swrap`/`.scard`/`.spotitem`, l.104–107) são só inspiração — o
> **scrim/card vêm do componente de popup central da fase 24** (decisão A); a 27 só pode declarar
> classes próprias de conteúdo (ex.: `.spotitem`), com tokens.

---

## 1. Mapa pro problema central (MILESTONE-3 §1 + §5.4)

O Timetrack é forte no **presente** (cronometrar) e tem uma porta de captura espalhada: pra criar
uma task hoje o Diogo precisa navegar até um projeto, abrir o board, achar a coluna, clicar
"+nova". Pra um cérebro TDAH isso é > 2s e a ideia se perde (RNF-02 — "mais de 2 passos = ignorado",
`CLAUDE.md §perfil TDAH`). O Spotlight é a **porta ÚNICA**: um atalho global (`Cmd+K` / `Ctrl+K`)
abre um input central; o usuário digita um slash-command (`/task` `/note` `/timer` `/goal`) seguido
de um nome e dá `Enter` — a entidade nasce **já nomeada** e (quando faz sentido) o editor abre com
o título focado pra enriquecer **opcionalmente**. Sem comando, o `Enter` **inicia um count-up**
quando o timer está **ocioso** (reusa o fluxo da IdleBar); com o timer **rodando**, **edita a
descrição+projeto do timer atual** via a allowance de backend (27-H1/CA-13 — `PATCH /api/v1/timer`,
roteado pelo `timer.Service`, propaga pelo broadcast da fase 04). O backend ganha **uma allowance
mínima** (a rota acima, só rótulo/projeto/tags, sem tocar timing); o resto do Spotlight é uma
**casca de orquestração** sobre fluxos já construídos (24/26/03/12).

A fase **não** reimplementa nenhum domínio de planejamento: ela *roteia* pro abridor de task da 24,
o abridor de nota da 26, `timerStore.start` (03) e `createObjective` (12). O **único** acréscimo de
backend é a allowance `PATCH /api/v1/timer` — uma operação fina sobre o `timer.Service` existente
(não um domínio novo): edita só rótulo/projeto/tags da entry rodando, sob o lock do timer, e dispara
`OnChange` (broadcast da 04). Se uma dependência de UI ainda não estiver no app no momento do build,
o Spotlight degrada com clareza (CA-17) — nunca quebra.

## 2. User stories

- **US-01** — Como usuário (TDAH), quero abrir uma porta de captura de **qualquer lugar** com
  `Cmd+K`/`Ctrl+K`, pra jogar uma ideia em < 2s sem navegar.
- **US-02** — Como usuário, quero digitar `/task Comprar domínio` + `Enter` e ver a task **já
  nomeada** nascer no projeto de contexto **e** o editor (fase 24) abrir com o título focado, pra
  capturar mínimo agora e enriquecer (ou não) depois.
- **US-03** — Como usuário, quero `/note <nome>` análogo (cria a nota da fase 26 já titulada e
  abre o editor), `/timer` (começa a cronometrar) e `/goal <nome>` (cria um objetivo no projeto de
  contexto), tudo pela mesma porta.
- **US-04** — Como usuário com o timer parado/ocioso, quero digitar texto **sem comando** e dar
  `Enter` pra **começar um count-up** com aquela descrição (e o projeto de contexto), reusando o
  fluxo da IdleBar.
- **US-05** — Como usuário com um timer **rodando**, ao digitar texto **sem comando** e dar `Enter`,
  quero **editar a descrição (e, com projeto de contexto, o projeto) do timer atual** sem o
  cronômetro parar nem perder o tempo já corrido, pra corrigir/precisar o que estou cronometrando
  num gesto só. O Spotlight chama a allowance `PATCH /api/v1/timer` (decisão HITL 23/06 — só
  rótulo/projeto/tags, sem tocar timing); a mudança **propaga pela sincronização da fase 04** e o
  rótulo atualiza ao vivo em TimerBar/Foco/Remoto nos outros devices. (Os campos de **tempo** seguem
  recusados numa entry rodando — `started_at`/`stopped_at` continuam dando `409` pelos caminhos de
  edição manual; ver §6.)
- **US-06** — Como usuário, quero navegar a lista de comandos por **teclado** (↑/↓ + `Enter`) e
  fechar com `Esc` ou clicando fora, pra operar tudo sem o mouse.
- **US-07** — Como usuário de **mobile** (sem teclado físico), quero um **botão** que abre o
  Spotlight, pra ter a mesma porta sem o atalho.

## 3. Critérios de aceite (EARS)

### Abertura, atalho e acesso

- **CA-01** *(event-driven — atalho global)* — QUANDO o usuário pressiona `Cmd+K` (macOS) ou
  `Ctrl+K` (Windows/Linux) em **qualquer rota autenticada** do shell, o sistema DEVE abrir o
  Spotlight (popup central em modo Spotlight) e **focar o input**, e DEVE chamar
  `preventDefault()` (não disparar o atalho nativo do navegador). O atalho é registrado **uma
  única vez** no shell (`AppShell`) e removido no unmount (sem listener vazado).
- **CA-01.1** *(unwanted — não sequestrar digitação)* — O handler do atalho responde **apenas** à
  combinação `(metaKey || ctrlKey) && key === 'k'`; qualquer outra tecla (inclusive `k` sozinho,
  num input de texto) NÃO DEVE abrir o Spotlight.
- **CA-02** *(event-driven — botão no shell)* — A UI DEVE exibir um **botão `Cmd+K`** acessível
  (label/aria "Abrir captura (Cmd+K)") que, ao clique, abre o Spotlight — idêntico ao atalho. O
  botão aparece no shell (desktop) e há um ponto de acesso no **mobile** (item/botão; ver CA-03).
- **CA-03** *(state-driven — mobile)* — ENQUANTO a viewport for `< 768px` (`md`, breakpoint único
  da M2 — `useMediaQuery`), o ponto de acesso ao Spotlight DEVE estar presente e clicável (botão
  no shell mobile e/ou item na `MobileNav` via `navItems.ts` — fonte única de nav). O `Cmd+K`
  segue funcionando se houver teclado.
- **CA-04** *(event-driven — fechar)* — QUANDO o Spotlight está aberto e o usuário pressiona `Esc`
  **ou** clica no scrim (fora do card), o sistema DEVE fechá-lo e devolver o foco ao documento. Um
  segundo `Cmd+K` com o Spotlight já aberto **mantém** o Spotlight aberto e o input focado
  (idempotente — não fecha nem empilha).
- **CA-05** *(state-driven — popup único)* — O Spotlight reusa o **componente de popup central da
  fase 24** (decisão de coordenação A — o `CenterPanel`/painel central da 24, com chrome side↔full,
  usando `tokens.css`; **não** classes cruas do mockup `.mscrim`/`.scard`/`.swrap` como contrato).
  ENQUANTO o Spotlight está aberto, NÃO DEVE haver dois popups centrais simultâneos: abrir o
  Spotlight sobre um popup de task/nota **substitui** o painel ativo (1 painel por vez). Ao
  **rotear** pra um editor (CA-09/CA-10/CA-12), o Spotlight é substituído pelo popup-alvo (não fica
  por baixo). O contrato real do abridor/estado de painel da 24 é confirmado no build (T-04) —
  ver D-09.

### Lista de comandos e filtragem

- **CA-06** *(state-driven — lista)* — ENQUANTO o Spotlight está aberto, a UI DEVE listar os
  comandos `/task` `/note` `/timer` `/goal` (rótulo + descrição + ícone — espelho do `SPOT` do
  mockup, linhas 625–630), com o **primeiro item selecionado** por padrão.
- **CA-07** *(event-driven — filtro)* — QUANDO o usuário digita, a lista DEVE filtrar pelo
  **primeiro token** do input sem a barra inicial: `/ta` ⇒ só `/task`; `task` (sem barra) ⇒
  `/task`; `/x` ⇒ lista vazia com aviso "Nenhum comando". O filtro casa `act.startsWith(token)`
  **ou** `label` contém o token (espelha `spotItems`, linha 633–634). A seleção volta pro índice 0
  a cada filtragem.
- **CA-08** *(state-driven — título derivado)* — O **título** capturado é o input **sem** o
  comando: `/task Comprar domínio` ⇒ `"Comprar domínio"`; `/note Reunião` ⇒ `"Reunião"`; a regra
  de extração é `input.replace(/^\/?\s*(task|note|timer|goal)\b/i, '').trim()` (espelha
  `spotTitle`, linha 636). O título derivado DEVE aparecer realçado no item selecionado quando o
  comando é `/task` ou `/note` e há título (espelho do realce do mockup, linha 646).

### Execução dos comandos

> **Projeto de contexto** (`spotPid`, linha 631): se a rota atual é `/projetos/:id`, o projeto de
> contexto é esse `id`; senão, **não há projeto de contexto** (decisão desta spec — ver CA-15).
> **⚠️ Resolução do projeto de contexto (27-H2):** o Spotlight é montado no **`AppShell`** (layout
> PAI de toda rota autenticada — `<Outlet/>` renderiza os filhos, e `/projetos/:id` é FILHO do
> shell, `App.tsx`). Em React Router v6 os `params` só fluem **pai→filho**, então `useParams()`
> chamado no nível do shell (acima do segmento `:id`) devolve `{}`. Logo o projeto de contexto DEVE
> ser resolvido a partir do **`pathname`**: `matchPath('/projetos/:id', useLocation().pathname)?.
> params.id ?? null` — **NÃO** `useParams()` (que retornaria sempre `null` e jogaria todo
> `/task`/`/note`/`/goal` no ramo "sem contexto" do CA-15). Espelha o padrão já usado por
> `SideNav`/`MobileNav` (irmãos do `<Outlet/>`, que leem `useLocation().pathname`).
> **Workspace ativo** = `useCurrentWorkspace()` (fase 10), nunca hardcoded.

- **CA-09** *(event-driven — /task)* — QUANDO o usuário executa `/task <nome>` (Enter no item
  selecionado **ou** clique), **havendo projeto de contexto** e permissão de escrita, o sistema
  DEVE criar a task via o POST de task já existente (fase 02/24:
  `POST /api/v1/workspaces/{ws}/projects/{projectId}/tasks` — ou o helper `createTask` da `api.ts`)
  com `name = <nome>` (ou um nome default "Nova task" se vazio) e o `project_id` do contexto, e ao
  sucesso DEVE **fechar o Spotlight e abrir o popup de task da fase 24** (`openTask`) **com o
  título focado** (`focus=true`) — captura < 2s, enriquecer opcional. NÃO cria rota nova.
- **CA-10** *(event-driven — /note)* — QUANDO o usuário executa `/note <nome>` havendo projeto de
  contexto, o sistema DEVE criar a nota via o endpoint da fase 26
  (`POST /api/v1/workspaces/{ws}/notes` com `project_id` do contexto e `title = <nome>`) e ao
  sucesso DEVE **fechar o Spotlight e abrir o popup de nota da fase 26** (`openNote`) pra editar.
- **CA-11** *(event-driven — /timer)* — QUANDO o usuário executa `/timer`, o sistema DEVE começar a
  cronometrar **reusando o `timerStore.start`** da fase 03 — `mode:"countup"`, `description = <título
  digitado>` (se houver; senão vazio), `project_id =` projeto de contexto (ou null) — e fechar o
  Spotlight. **NÃO** duplica lógica de timer (chama o store, não a API direto).
- **CA-12** *(event-driven — /goal)* — QUANDO o usuário executa `/goal <nome>` havendo projeto de
  contexto e permissão de escrita, o sistema DEVE criar o objetivo via o CRUD **já existente** da
  fase 12 (`POST /api/v1/workspaces/{ws}/projects/{projectId}/objectives` — `createObjective`) com
  `name = <nome>` e fechar o Spotlight. SE a fase 29 (Goals com modos) já estiver no app, o
  objetivo nasce no `completion_mode` default (`by_tasks`) — **sem** rota nova; SE não, usa o POST
  da 12 como está. Ao sucesso, abre o detalhe/board do projeto (`/projetos/{id}`) **ou** apenas
  fecha com um aviso de sucesso (decisão do plan — sem editor de objetivo dedicado obrigatório).

### Comportamento sem comando (RF-19)

- **CA-13** *(event-driven — sem comando, timer rodando ⇒ edita o timer atual via allowance)* —
  ENQUANTO há um timer **rodando** (`useTimerStore.state` com entry aberta), QUANDO o usuário digita
  texto **sem** comando (não começa com `/task|/note|/timer|/goal`) e dá `Enter`, o sistema DEVE
  editar a entry rodando chamando a allowance **`PATCH /api/v1/timer`** (decisão HITL 23/06)
  passando `description = <texto>` e — **havendo projeto de contexto** (`/projetos/:id`) — o
  `project_id` desse contexto (sem projeto de contexto, edita só a descrição). O sistema **NÃO DEVE**
  parar/alterar o cronômetro nem mexer em horários (RNF-04): a allowance toca **somente**
  description/project_id/tags. Ao sucesso, o `timer.Service` dispara `OnChange` ⇒ o broadcast da fase
  04 propaga o `State` novo (rótulo/projeto atualizados) aos outros devices; o Spotlight fecha. O
  Spotlight chama o helper de allowance do `timerStore` (não a API direto) — reusa o "ações nunca
  rejeitam" (fase 05 CA-17). **Texto vazio + timer rodando ⇒ no-op** (não apaga a descrição
  silenciosamente; mantém o Spotlight aberto). (O caso ocioso é o **count-up** — CA-14.)
- **CA-13.1** *(unwanted — allowance recusa edição de tempo)* — A allowance `PATCH /api/v1/timer`
  DEVE aceitar **apenas** `description`/`project_id`/`tags`; **NÃO DEVE** alterar
  `started_at`/`stopped_at`/`duration_seconds`/`mode`/`target_seconds` de uma entry rodando. Os
  caminhos de edição manual de horário (`entries.Patch`, `PATCH /workspaces/{ws}/entries/{id}`)
  seguem **recusando** entry rodando com `409 entry_running`
  (`backend/internal/entries/service.go:184-187`) — esse comportamento **não se relaxa** (RNF-01).
- **CA-13.2** *(unwanted — allowance valida o `project_id` e o dono)* — A allowance DEVE validar o
  `project_id` novo via `entries.ValidateRefs` contra o **workspace da entry rodando** (project de
  outro workspace/inexistente ⇒ `404 not_found` canônico, sem vazar existência — CA-29 herdado); e
  DEVE editar **apenas o timer do próprio usuário** (rota por-usuário, user da sessão; `not_found` se
  o usuário não tem timer rodando). A allowance roda **dentro do advisory lock do timer**
  (`pg_advisory_xact_lock(hashtext(user_id))`) — o status "rodando" não muda no meio da edição.
- **CA-14** *(state-driven — sem comando, ocioso)* — ENQUANTO **não** há timer rodando, SE o
  usuário digita texto sem comando e dá `Enter`, o sistema DEVE **começar um count-up** reusando
  `timerStore.start({mode:"countup", description:<texto>, project_id:<contexto ou null>})` (fluxo
  da IdleBar — RF-19 caso "sem timer rodando") e fechar o Spotlight. Input **vazio** + sem comando
  + Enter ⇒ **no-op** (não cria entry vazia silenciosa; mantém o Spotlight aberto).

### Permissão, contexto ausente e degradação

- **CA-15** *(unwanted — sem projeto de contexto)* — SE o usuário executa `/task`, `/note` ou
  `/goal` **sem projeto de contexto** (rota não é `/projetos/:id`), o sistema NÃO DEVE criar a
  entidade às cegas e DEVE sinalizar a ausência de contexto de forma à-prova-de-culpa (RNF-03):
  exibe um aviso curto "Abra um projeto pra criar aqui" (ou desabilita esses comandos com a dica),
  mantendo `/timer` e o modo-sem-comando funcionais (não exigem projeto). **Decisão registrada
  (plan §3):** nesta fase **não** se inventa um projeto "Inbox" default — `/timer` e count-up
  cobrem a captura sem-projeto; `/task`/`/note`/`/goal` exigem projeto de contexto explícito.
- **CA-16** *(unwanted — sem permissão de escrita)* — SE o usuário não tem escrita no projeto de
  contexto (`can_write=false` — viewer/`access=view`/arquivado, vindo do `/board` da fase 12/24),
  o sistema NÃO DEVE tentar criar task/nota/goal (que o backend negaria com 403 — CA-08/12 da fase
  12) e DEVE sinalizar isso sem mensagem acusatória. O `/timer` e o modo-sem-comando (que mexem no
  **timer do usuário**, não no projeto) seguem disponíveis.
- **CA-16.1** *(unwanted — não disparar `/board` fora de contexto — 27-H3)* — A resolução de
  `can_write` consome o `/board` (fase 12, `useBoard`), mas `useBoard` **não tem guard `enabled`**
  (`frontend/src/lib/queries/objectives.ts:19-24` — a `queryFn` dispara sempre). Como o Spotlight é
  montado no `AppShell` (todo render do shell, toda rota), chamar `useBoard(ws, projectId)` com
  `projectId` vazio/ausente dispararia `GET /workspaces/{ws}/projects//board` **malformado** em toda
  navegação fora de projeto e com o Spotlight fechado. O sistema NÃO DEVE disparar essa request fora
  de `/projetos/:id`. **Implementação (escolher uma, fixar):** (a) estender `useBoard` com opção
  `enabled` e chamá-lo `useBoard(ws, projectId ?? '', { enabled: !!projectId && open })` — `useQuery`
  não dispara a `queryFn` com `enabled=false`; **ou** (b) isolar a leitura do `/board` num
  **componente-filho** do Spotlight renderizado só quando `open && projectId` (componente
  condicional, **não** hook condicional — respeita Rules of Hooks), elevando o `can_write` resolvido
  via callback/prop. Em ambos: **nenhum** `GET /projects//board` em rota fora de `/projetos/:id` nem
  com o Spotlight fechado (pinado em teste — T-08).
- **CA-17** *(unwanted — dependência ausente / falha de API)* — SE o popup-alvo (task da 24 ou
  nota da 26) ainda **não** existir no app, ou a chamada de criação falhar, o sistema NÃO DEVE
  travar o Spotlight num estado quebrado: a ação falha graciosamente (mantém o Spotlight aberto
  com um aviso curto **OU** fecha sem efeito), sem promessa não cumprida e sem exceção não tratada
  na UI. (Reusa o padrão "ações do timerStore nunca rejeitam" — fase 05 CA-17 — pro ramo `/timer`.)

### Multi-tenant e isolamento (herdado)

- **CA-18** *(ubíquo — workspace ativo)* — Toda criação (task/nota/goal) e o `/timer` usam o
  **workspace ativo** (`useCurrentWorkspace()`, fase 10) e o **projeto de contexto** da rota
  atual — nunca um workspace/projeto de outro tenant. O backend já filtra por `workspace_id` +
  visibilidade (fases 02/12/26); o Spotlight só passa os ids corretos. (Sem rota nova ⇒ sem
  superfície de vazamento nova; o teste pina que os paths chamados carregam o `ws`/`projectId`
  certos.)

### E2E

- **CA-19** — Logado (stub da fase 01) **num projeto** (`/projetos/:id`): o usuário pressiona
  `Cmd+K`, o Spotlight abre e foca o input; digita `/task Comprar domínio`, dá `Enter` → a task
  nasce e o popup de task da fase 24 abre com o título "Comprar domínio" focado; fecha. Abre de
  novo (`Cmd+K`), digita texto sem comando estando **ocioso**, dá `Enter` → um count-up começa
  (TimerBar mostra rodando). **Com o timer rodando**, `Cmd+K`, digita texto novo sem comando, `Enter`
  → a descrição do timer atualiza na TimerBar **sem o cronômetro zerar** (allowance da decisão HITL,
  CA-13). `Esc` fecha o Spotlight quando reaberto vazio. **Limitação registrada (igual fase 12
  CA-24):** o `googlestub` autentica um único usuário (owner) — o ramo viewer/`can_write=false`
  (CA-16) e o ramo "sem projeto de contexto" (CA-15) são cobertos por **teste de componente jsdom**,
  não no E2E.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-18 | Cmd+K (+Ctrl+K, +botão shell, +mobile) abre o popup central em modo Spotlight: input + comandos `/task` `/note` `/timer` `/goal`; navegação teclado; Esc fecha | CA-01, 01.1, 02, 03, 04, 05, 06, 07, 08 |
| RF-19 | Sem comando: **ocioso ⇒ começa count-up**; **rodando ⇒ edita descrição+projeto do timer atual** via a allowance `PATCH /api/v1/timer` (decisão HITL 23/06 — só rótulo/projeto/tags, sem tocar timing; propaga pelo broadcast da fase 04) | CA-13, 13.1, 13.2, 14 |
| RF-20 | `/task <nome>` cria task já nomeada e abre o editor com título focado (captura < 2s); `/note`/`/timer`/`/goal` análogos; reusa endpoints existentes | CA-08, 09, 10, 11, 12, 15, 16, 17, 18 |

## 5. Requisitos não-funcionais

- **RNF-01 (allowance mínima de backend — lei desta fase)** — ZERO migração. A fase é
  **frontend + 1 allowance mínima de backend**: orquestra `createTask`/`createNote`/
  `createObjective`/`timerStore.start` que **já existem**, e adiciona **exatamente uma** rota nova
  por-usuário (`PATCH /api/v1/timer`) roteada pelo `timer.Service` p/ editar **só**
  description/project_id/tags da entry RODANDO (decisão HITL 23/06 — 27-H1). A allowance é cirúrgica:
  **não** cria tabela/enum, **não** toca timing/sync/pomodoro (RNF-04), reusa `entries.ValidateRefs`
  + o advisory lock do timer + o hook `OnChange` já existentes. Os caminhos de edição manual de
  horário (`entries.Patch`) ficam **byte-idênticos** (seguem dando `409` em entry rodando — CA-13.1).
  O acréscimo Go é mínimo e isolado (um método no `timer.Service` + um handler/rota); o resto do
  `make check-full` no lado Go fica inalterado.
- **RNF-02 (captura < 2s — lei do PRD)** — o caminho feliz `/task <nome>` + `Enter` exige **só o
  nome** (o projeto vem do contexto); nenhum passo extra obrigatório antes de a task existir
  (MILESTONE-3 §6 RNF-02). Enriquecer (status/prioridade/descrição) é opcional, no popup que abre
  depois.
- **RNF-03 (à prova de culpa)** — nenhum estado vermelho acusatório; ausência de contexto/permissão
  é uma **dica** ("Abra um projeto pra criar aqui"), não um erro (MILESTONE-3 §6 RNF-03).
- **RNF-04 (não regredir o TRACKING)** — `/timer` e o modo-sem-comando(ocioso) reusam o `timerStore`
  da fase 03 **sem** duplicar lógica; o sync/Pomodoro/foco ficam zero-diff (MILESTONE-3 §6 RNF-01). A
  allowance do modo-sem-comando(rodando) (CA-13) altera **só** description/project_id/tags da entry
  rodando — **nunca** `started_at`/`stopped_at`/`duration`/`mode`/`target` (o tempo já corrido e o
  ciclo Pomodoro ficam intactos), e roda no **mesmo advisory lock** do timer (não entrelaça com
  start/stop/pause/resume). O elapsed/remaining derivam do `State` reemitido pelo `OnChange` igual a
  qualquer mutação do timer (anti-stale do `apply()` intocado).
- **RNF-05 (tokens)** — `tokens.css` é a única fonte de cor (gate `check-cores`): o **componente de
  popup central da fase 24** (chrome side↔full) já usa só tokens; o conteúdo do Spotlight (input,
  itens da lista, chips de tecla) usa tokens/classes existentes (`--accent`, `--surface-*`,
  `--border`, `.chip`, `.tt-btn`). Qualquer classe nova de item de lista do Spotlight (ex.:
  `.spotitem`) é **própria desta fase** (não "reusada"), declarada no CSS global só com `var(--*)`.
  Nenhum hex literal em `.tsx`. (Decisão A: NÃO referenciar `.mscrim`/`.scard`/`.swrap` do mockup —
  o scrim/card vêm do componente de popup central da 24.)
- **RNF-06 (fonte Inter)** — a M3 já assume Inter (a troca Geist→Inter é da fase 24); esta fase usa
  o tipográfico vigente, mono só pros chips de tecla (`esc`, `↵`).
- **RNF-07 (mobile)** — responsivo no breakpoint 768 da M2 (`useMediaQuery`); o componente de popup
  central da 24 (decisão A) já é responsivo — o conteúdo do Spotlight cabe no mobile (largura
  herdada do popup central, ~`min(760px, 95vw)`).
- **PT-BR na UI**; identificadores de código em inglês (nomes de teste em PT-BR são exceção
  registrada).

## 6. Fora de escopo (fronteiras explícitas)

- **Editar os campos de TEMPO de uma entry RODANDO** (`started_at`/`stopped_at`/`duration`/`mode`/
  `target`) → **fora** (CA-13.1). A allowance da decisão HITL de 23/06 cobre **somente**
  description/project_id/tags da entry rodando (via `PATCH /api/v1/timer`); ajustar horários de uma
  entry em andamento segue **recusado** pelo backend (`entries.Patch` → `409 entry_running` —
  `backend/internal/entries/service.go:184-187`). Mexer no tempo do timer ativo continua sendo via
  as rotas `/timer` (start/stop/pause/resume) — não há edição de timing in-flight.
- **Busca/navegação** (ir pra uma página, achar uma task/projeto existente pelo Spotlight) → o
  placeholder fala "Comando ou busca…" no mockup, mas **busca de entidades é fora desta fase**
  (só comandos de **criação**). Busca = candidata a fatia futura.
- **Slash dentro da descrição da task** — o `/` que cria blocos é **só nas Notas** (fase 26,
  decisão `MILESTONE-3 §9.4`); o Spotlight tem seus próprios comandos de topo, **não** é o editor
  de blocos.
- **Projeto "Inbox" default** pra `/task`/`/note`/`/goal` sem contexto → fora (CA-15: exige
  projeto de contexto; `/timer`/count-up cobrem o sem-projeto). Candidato a futuro.
- **Editor de objetivo dedicado** — `/goal` cria via o POST da 12 e fecha (ou abre o board); não
  há modal de objetivo focado nesta fase.
- **Siri / LLM / linguagem natural** ("amanhã às 15h", parse de data) → fora (anti-NOA;
  `CLAUDE.md` decisão B "Siri→LLM = depois").
- **Comandos novos além de `/task` `/note` `/timer` `/goal`** (ex.: `/client`, `/event`) → fora;
  só os quatro do PRD.
- **Backend além da allowance única** → fora. A fase adiciona **exatamente uma** rota nova
  (`PATCH /api/v1/timer` — edição de rótulo/projeto/tags da entry rodando, decisão HITL 23/06) e o
  método correspondente no `timer.Service`. **ZERO migração**, nenhuma outra rota/tabela/enum, nenhum
  outro service Go tocado (RNF-01).
