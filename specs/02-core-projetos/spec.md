# spec.md — Fase 02 · Domínio core (clientes → projetos → tasks)

> Fonte de verdade desta fase. Formato: SDD.md §4. Critérios EARS numerados (CA-NN) —
> **cada CA vira pelo menos um teste nomeado** (TDD.md; CAs compostos podem fechar com
> sub-asserts no mesmo teste, desde que a matriz CA→tarefa rastreie o red→green). Depende da fase 01 (auth Google, sessão,
> workspaces, workspace_members, roles). Entrega o CRUD completo de clientes, projetos e tasks
> via API REST + UI no cockpit, com visibilidade privado×compartilhado e roles aplicados.

---

## 1. Mapa pro problema central (REQUISITOS.md §2)

Esta fase ataca diretamente o **problema 1**: "trackear tempo rápido, **separando
cliente/projeto**". Sem clientes/projetos/tasks não há contra o quê cronometrar (fase 03), nem
ângulo "tempo por cliente/projeto" e "meta vs. realizado" nos relatórios (fase 07 — a **meta de
horas/semana** nasce aqui). A base multiplayer (visibilidade, membros de projeto, campos
`created_by`/`assignee`/`requested_by`) é modelada **agora** pra nunca exigir retrabalho de
schema (REQUISITOS.md §5), mas **sem nenhuma colaboração real no MVP**.

## 2. User stories

- **US-01** — Como **owner do workspace**, quero cadastrar e arquivar **clientes**, pra agrupar
  meus projetos por quem eu atendo (Acme, Lauda, freela, Arkan, Primor).
- **US-02** — Como **membro do workspace**, quero criar **projetos** com nome, **cor**, cliente
  opcional e **meta de horas/semana** opcional, pra organizar o tempo e medir desvio depois.
- **US-03** — Como **dono de um projeto**, quero marcá-lo **Privado** ou **Compartilhado**, pra
  controlar quem do workspace enxerga o que eu faço.
- **US-04** — Como **dono de um projeto compartilhado**, quero escolher **quais membros
  participam** e o **acesso de cada um** (*Editar* × *Só vê*), pra deixar a base multiplayer
  pronta sem ligar colaboração real ainda.
- **US-05** — Como **membro do workspace**, quero criar **tasks** num projeto com status, data
  opcional, recorrência opcional e **notas**, pra planejar o trabalho que vou cronometrar.
- **US-06** — Como **usuário do cockpit**, quero ver Clientes e Projetos em **cartões leves** por
  padrão e alternar pra **tabela densa** só quando eu escolher, pra escanear rápido sem poluição.
- **US-07** — Como **viewer do workspace**, quero apenas **ler** o que é compartilhado, sem
  conseguir criar ou editar nada, pra acompanhar sem risco de mexer.

## 3. Critérios de aceite (EARS)

### Clientes — API

- **CA-01** *(event-driven)* — QUANDO um owner ou member envia `POST .../clients` com nome
  válido (1–120 caracteres após trim), o sistema DEVE criar o cliente no workspace da rota e
  retornar `201` com o objeto criado (id, name, archived_at=null, created_at... conforme contrato).
- **CA-02** *(unwanted)* — SE o nome do cliente for vazio, só espaços ou tiver mais de 120
  caracteres, o sistema NÃO DEVE criar e DEVE retornar `400` com
  `error.code="validation_error"`.
- **CA-03** *(event-driven)* — QUANDO um membro envia `GET .../clients`, o sistema DEVE retornar
  apenas os clientes **não arquivados** do workspace da rota, e DEVE incluir os arquivados
  apenas quando `?archived=true`.
- **CA-04** *(event-driven)* — QUANDO um owner ou member envia `PATCH .../clients/{id}` com nome
  válido, o sistema DEVE renomear o cliente e retornar `200` com o objeto atualizado.
- **CA-05** *(event-driven)* — QUANDO um owner ou member envia `DELETE .../clients/{id}`, o
  sistema DEVE **arquivar** (preencher `archived_at`, nunca apagar a linha) e os projetos
  vinculados DEVEM manter seu `client_id` intacto.

### Projetos — API

- **CA-06** *(event-driven)* — QUANDO um owner ou member envia `POST .../projects` válido (nome
  1–120 caracteres após trim, cor, visibilidade), o sistema DEVE criar o projeto com
  `owner_user_id` = usuário da sessão e retornar `201` com o objeto criado.
- **CA-06.1** *(unwanted)* — SE o nome do projeto for vazio, só espaços ou tiver mais de 120
  caracteres, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400` com
  `error.code="validation_error"`.
- **CA-07** *(unwanted)* — SE `color` não casar com o formato `#RRGGBB` (regex
  `^#[0-9a-fA-F]{6}$`) ou `weekly_hours_goal` estiver fora do intervalo `(0, 168]`, o sistema
  NÃO DEVE criar/atualizar e DEVE retornar `400` com `error.code="validation_error"`.
- **CA-08** *(unwanted)* — SE `client_id` informado não existir **no mesmo workspace**, o sistema
  NÃO DEVE criar/atualizar e DEVE retornar `400` com `error.code="invalid_reference"`.
- **CA-08.1** *(unwanted)* — SE `client_id` informado em `POST`/`PATCH` de projeto referenciar um
  cliente **arquivado**, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400` com
  `error.code="invalid_reference"`. Projetos já vinculados a um cliente que foi arquivado
  permanecem intactos e legíveis (CA-05).
- **CA-09** *(optional/state)* — ONDE o projeto for `private`, somente o `owner_user_id` DEVE
  enxergá-lo — **sem exceção, inclusive o owner do workspace**: ele não aparece no
  `GET .../projects` de nenhum outro membro e `GET .../projects/{id}` (assim como qualquer
  `PATCH`/`DELETE`/`PUT .../members`) por qualquer outro usuário DEVE retornar `404` (não vazar
  a existência). A privacidade prevalece sobre os poderes administrativos do owner do workspace.
- **CA-10** *(optional/state)* — ONDE o projeto for `shared`, todos os membros do workspace
  (owner, member e viewer) DEVEM vê-lo na listagem e no detalhe.
- **CA-11** *(event-driven)* — QUANDO o dono do projeto (ou owner do workspace, em projeto
  visível a ele — CA-09) envia `PUT .../projects/{id}/members` com lista `[{user_id, access}]`
  (`access` ∈ `edit|view`), o sistema DEVE substituir os membros do projeto por essa lista (o
  dono do projeto é implícito e não entra na lista) e retornar `200` com a lista persistida.
- **CA-11.1** *(event-driven)* — QUANDO o `POST .../projects` de um projeto `shared` incluir o
  campo opcional `members` (mesmo formato do CA-11), o sistema DEVE persistir os membros em
  `project_members` na mesma transação da criação e devolvê-los no corpo do `201`.
- **CA-12** *(unwanted)* — SE algum `user_id` da lista de membros do projeto (no `POST .../projects`
  ou no `PUT .../members`) não for membro do workspace, o sistema NÃO DEVE persistir nada da
  operação e DEVE retornar `400` com `error.code="invalid_reference"`.
- **CA-13** *(unwanted)* — SE o projeto for (ou estiver sendo tornado) `private` e a requisição
  (`POST`, `PATCH` ou `PUT .../members`) contiver lista de membros não vazia, o sistema DEVE
  retornar `400` com `error.code="validation_error"` (privado não tem membros — a UI desabilita,
  a API valida).
- **CA-13.1** *(event-driven)* — QUANDO um `PATCH .../projects/{id}` muda `visibility` para
  `private` (sem campo `members` ou com lista vazia), o sistema DEVE remover todas as linhas de
  `project_members` do projeto na mesma transação (privado nunca mantém membros órfãos).
- **CA-14** *(event-driven)* — QUANDO o dono do projeto ou o owner do workspace envia
  `PATCH .../projects/{id}` alterando nome/cor/cliente/meta/visibilidade **num projeto visível
  ao requisitante** (CA-09), o sistema DEVE persistir e retornar `200`; QUANDO outro member
  tenta num projeto `shared`, DEVE retornar `403` (`error.code="forbidden"`) — membro com
  `access=edit` edita **tasks**, não metadados. Em projeto `private` de outro usuário, a
  resposta é sempre `404` (CA-09), inclusive pro owner do workspace.
- **CA-15** *(event-driven)* — QUANDO o dono do projeto ou o owner do workspace (projeto visível
  a ele — CA-09) envia `DELETE .../projects/{id}`, o sistema DEVE **arquivar** o projeto
  (`archived_at`) sem apagar suas tasks, e o projeto sai da listagem padrão (`?archived=true`
  o inclui).
- **CA-15.1** *(optional/state)* — ONDE o projeto estiver arquivado, `GET .../projects/{id}` por
  quem o enxerga (CA-09/10) DEVE retornar `200` com `archived_at` preenchido — só as listagens
  o omitem por default (CA-15); as tasks existentes do projeto continuam legíveis em
  `GET .../tasks` (lista e detalhe).
- **CA-15.2** *(unwanted)* — SE um `POST .../tasks` ou `PATCH .../tasks/{id}` referenciar/pertencer
  a um projeto **arquivado**, o sistema NÃO DEVE escrever e DEVE retornar `400` com
  `error.code="invalid_reference"` (projeto arquivado é só leitura).

### Tasks — API

- **CA-16** *(event-driven)* — QUANDO um usuário com permissão de escrita no projeto (dono,
  owner do workspace — só em projeto visível a ele, CA-09 —, ou membro do projeto com
  `access=edit`) envia `POST .../tasks` com nome válido (1–200 caracteres após trim) e
  `project_id` visível, o sistema DEVE criar a task com `status="todo"` por default e
  `created_by` = usuário da sessão, retornando `201`.
- **CA-16.1** *(unwanted)* — SE um usuário **sem** permissão de escrita no projeto — membro do
  projeto com `access=view`, ou member/viewer do workspace que enxerga o projeto `shared` mas
  não está em `project_members` — enviar `POST`/`PATCH`/`DELETE` em tasks desse projeto, o
  sistema NÃO DEVE alterar nada e DEVE retornar `403` com `error.code="forbidden"`
  (*Editar* × *Só vê* tem efeito real).
- **CA-16.2** *(unwanted)* — SE o nome da task for vazio, só espaços ou tiver mais de 200
  caracteres, o sistema NÃO DEVE criar/atualizar e DEVE retornar `400` com
  `error.code="validation_error"`.
- **CA-17** *(unwanted)* — SE `project_id` não existir no workspace **ou** for um projeto privado
  de outro usuário, o sistema DEVE retornar `404` com `error.code="not_found"` (mesma resposta
  pros dois casos — não vazar existência).
- **CA-18** *(event-driven)* — QUANDO um usuário com escrita envia `PATCH .../tasks/{id}` com
  `status` ∈ `todo|doing|done`, o sistema DEVE atualizar; SE o valor estiver fora do enum, DEVE
  retornar `400` com `error.code="validation_error"`.
- **CA-19** *(event-driven)* — QUANDO um membro envia `GET .../tasks?project_id={id}&status={s}`,
  o sistema DEVE retornar apenas as tasks que casam com **ambos** os filtros informados (filtros
  são opcionais e combináveis).
- **CA-20** *(event-driven)* — QUANDO uma task é criada/editada com `due_date` (data ISO 8601
  `YYYY-MM-DD`), `recurrence` (texto livre/RRULE, ≤255 chars) e `notes` (texto), o sistema DEVE
  persistir e devolver os três campos; SE `due_date` não for uma data ISO válida, DEVE retornar
  `400` com `error.code="validation_error"`.
- **CA-20.1** *(unwanted)* — SE `recurrence` tiver mais de 255 caracteres, o sistema NÃO DEVE
  criar/atualizar e DEVE retornar `400` com `error.code="validation_error"`.
- **CA-21** *(event-driven)* — QUANDO um usuário com escrita envia `DELETE .../tasks/{id}`, o
  sistema DEVE remover a task definitivamente (hard delete — task não tem `archived_at`).
- **CA-22** *(optional/state)* — ONDE a task pertencer a um projeto `private` de outro usuário,
  ela NÃO DEVE aparecer no `GET .../tasks` desse membro e `GET .../tasks/{id}` DEVE retornar
  `404` (tasks herdam a visibilidade do projeto).

### Roles, auth e isolamento — API

- **CA-23** *(unwanted)* — SE um usuário com role `viewer` no workspace enviar qualquer
  requisição de escrita (`POST`/`PATCH`/`PUT`/`DELETE`) em clientes, projetos ou tasks, o
  sistema NÃO DEVE alterar nada e DEVE retornar `403` com `error.code="forbidden"`.
- **CA-24** *(state-driven)* — ENQUANTO autenticado como `viewer`, o usuário DEVE conseguir
  `GET` (lista e detalhe, `200`) de clientes, projetos compartilhados e suas tasks.
- **CA-25** *(unwanted)* — SE a requisição chegar sem cookie de sessão válido, qualquer rota
  desta fase DEVE retornar `401` com `error.code="unauthorized"`.
- **CA-26** *(ubíquo — teste de vazamento obrigatório)* — O sistema DEVE sempre filtrar clientes,
  projetos, membros de projeto e tasks pelo `workspace_id` da rota: um usuário autenticado que
  referencie (na URL ou no payload) um recurso existente de **outro** workspace DEVE receber
  `404` (rota de workspace que ele não integra DEVE retornar `404` igualmente).

### UI — cockpit (páginas Clientes e Projetos)

- **CA-27** *(state-driven)* — As páginas **Clientes** e **Projetos** DEVEM abrir na visão
  **cartões** por padrão; a tabela densa (`.tt-table`) DEVE aparecer apenas após o usuário
  acionar o toggle *Cartões ↔ Tabela*, e a escolha DEVE persistir por página entre visitas no
  mesmo navegador (mecanismo é decisão do plan).
- **CA-28** *(event-driven)* — QUANDO, no modal **Novo projeto**, o usuário seleciona
  **Privado**, a seção "Membros do projeto" DEVE ficar desabilitada (sem interação); QUANDO
  seleciona **Compartilhado**, DEVE permitir incluir membros e escolher o acesso de cada um
  (*Editar* × *Só vê*), com o Owner fixo e não removível — conforme `newproject.jsx`.
- **CA-28.1** *(event-driven)* — QUANDO o usuário abre o modal em modo **Editar** a partir de um
  projeto existente, os campos DEVEM vir preenchidos com os valores atuais (nome, cliente, cor,
  meta, visibilidade, membros) e, ao confirmar, as alterações DEVEM aparecer na lista sem
  recarregar a página.
- **CA-29** *(event-driven)* — QUANDO o usuário confirma o modal Novo projeto com dados válidos,
  o projeto DEVE aparecer na lista da página Projetos **sem recarregar a página** e o modal
  DEVE fechar.
- **CA-30** *(state-driven)* — O chip/cartão de projeto DEVE exibir a **cor do projeto**
  (`.tt-dot`/régua de cor) e, ONDE houver `weekly_hours_goal`, uma **barrinha de progresso**
  (`.tt-goal`) com largura = `min(100, horas_semana/meta×100)%` e preenchimento na **cor do
  projeto**, trocando pra cor de **accent** quando o progresso atingir 100% (regra do `ProjRow`
  em `projects.jsx`); sem meta definida, a barrinha NÃO DEVE ser renderizada. (Nesta fase,
  horas_semana = 0 — dados reais chegam na fase 03.)
- **CA-30.1** *(event-driven)* — O detalhe/cartão expandido do projeto DEVE listar as **tasks**
  do projeto (nome, status, data, notas) e, pra quem tem permissão de escrita (CA-16/16.1),
  permitir **criar task** com nome, data opcional e notas; QUANDO a task é criada, ela DEVE
  aparecer na lista sem recarregar a página.
- **CA-31** *(state-driven)* — ENQUANTO o usuário logado for `viewer`, a UI NÃO DEVE exibir os
  botões de escrita ("Novo cliente", "Novo projeto", editar/arquivar) nas páginas desta fase.

### Não-funcional

- **CA-32** *(ubíquo — verificação informativa, fora da regra red→green)* — Com 500 registros no
  workspace (proporção fixa: **50 clientes, 150 projetos, 300 tasks**), cada endpoint de
  listagem desta fase (`GET /clients`, `GET /projects`, `GET /tasks`) DEVE responder em
  **< 200 ms** no ambiente local de teste (Postgres efêmero), tomando a **melhor de 3
  execuções**. Este critério é um smoke de regressão de performance — por nascer verde e
  depender da máquina, ele NÃO segue o ciclo red→green do TDD.md (exceção documentada); se
  falhar, a correção esperada é índice na migração.

## 4. Requisitos funcionais (resumo)

| RF | Descrição | CAs |
|---|---|---|
| RF-01 | CRUD de clientes (criar, listar, renomear, arquivar) | CA-01..05 |
| RF-02 | CRUD de projetos (nome, cor, cliente opcional, meta h/sem opcional, visibilidade; arquivado é só leitura) | CA-06, 06.1, 07, 08, 08.1, 14, 15, 15.1, 15.2 |
| RF-03 | Visibilidade privado × compartilhado aplicada em lista e detalhe (privacidade absoluta, inclusive vs. owner do ws) | CA-09, 10, 22 |
| RF-04 | Membros do projeto com acesso edit/view (base multiplayer, sem colaboração real; view = 403 em escrita de task) | CA-11, 11.1, 12, 13, 13.1, 16.1 |
| RF-05 | CRUD de tasks (status, due_date, recorrência, notas; created_by/assignee/requested_by no schema) | CA-16, 16.2, 17..20, 20.1, 21, 22 |
| RF-06 | Roles aplicados: viewer só lê; escrita só owner/member | CA-23, 24, 31 |
| RF-07 | Auth por sessão + isolamento total por workspace | CA-25, 26 |
| RF-08 | Páginas Clientes e Projetos: cartões default, tabela sob toggle, modal Novo/Editar projeto, chips, tasks no detalhe | CA-27, 28, 28.1, 29, 30, 30.1 |

## 5. Requisitos não-funcionais

- **Leveza:** zero dependência Go nova além do baseline (chi + pgx); validação na mão com stdlib.
- **Performance:** CA-32 (smoke informativo: listagens < 200 ms com 500 registros, local —
  fora do ciclo red→green).
- **Segurança:** toda rota atrás do middleware de sessão da fase 01; respostas de recurso
  inexistente e recurso invisível são **idênticas** (`404`) pra não vazar existência; nenhum
  segredo logado.
- **Multi-tenant:** CA-26 é inegociável (SDD.md §8.4 — vazamento = bug crítico).
- **PT-BR na UI**, tokens visuais de `tokens.css` (dark-first, accent configurável).

## 6. Fora de escopo desta fase (fronteiras explícitas)

- **Cronômetro e time entries** → fase 03 (a barrinha de meta mostra 0h até lá; "Esta semana" /
  "Total" dos protótipos só ganham dados reais na 03/07).
- **Sync em tempo real entre devices (WebSocket)** → fase 04.
- **Modo Foco / flip-clock** → fase 05.
- **Google Calendar** → fase 06.
- **Relatórios (4 ângulos)** → fase 07 (a meta h/sem criada aqui alimenta "meta vs. realizado" lá).
- **API token `tt_` / conector Claude `/timetrack-task`** → fase 08 (nada aqui pode impedir —
  os handlers de criação de task ficam reutilizáveis pela camada de service).
- **Multiplayer real** (criar/pedir tasks entre colaboradores, notas cruzadas, UI de
  assignee/requested_by) → **fora do MVP**; só os campos no schema e o modal de membros.
- **Página Workspace** (convites, presença ao vivo — `workspace.jsx`) → fase 01 cobre o mínimo
  de membros/roles; convites e presença ficam fora desta fase.
- **Campos extras do protótipo de Clientes** (contato, valor/h, status Ativo/Interno/Pausado em
  `clients.jsx`) → fora do schema canônico do MVP; cartão de cliente usa nome + nº de projetos.
- **Campos extras do protótipo de Projetos** (`projects.jsx`): o status **"Pausado"** fica fora
  (não existe no schema — Status na UI = **Ativo/Arquivado**, derivado de `archived_at`) e as
  colunas **"Esta semana"/"Total"** não têm dado até a fase 03 — renderizar `0h`/`—` como
  placeholder, sem inventar campo no schema.
- **Configuração "projetos novos são compartilhados"** (workspace.jsx) → fora; visibilidade é
  sempre explícita no payload.
