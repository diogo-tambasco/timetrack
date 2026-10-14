# tasks.md — Fase 02 · Domínio core (clientes → projetos → tasks)

> Tarefas atômicas, em ordem de execução. **1 commit por tarefa** (mensagem PT-BR).
> Toda tarefa segue TDD (TDD.md): escrever o teste do critério, **confirmar RED**, implementar o
> mínimo, GREEN, refactor, commit. Postgres de teste = efêmero (nunca o de prod). Suíte inteira
> verde ao fim da fase é pré-requisito pra fase 03.

---

## Backend

### T-01 — Migrações do domínio core
- **Objetivo:** criar `0006_clients.sql`, `0007_projects.sql` (projects + project_members + enums) e `0008_tasks.sql` conforme `plan.md §2` — a fase 01 termina em `0005_oauth_tokens.sql` (conferir/ajustar a numeração real do diretório antes).
- **Entradas:** `plan.md §2`; runner de migração do boot (fase 00/01); Postgres efêmero.
- **Entregável:** 3 arquivos em `backend/internal/db/migrations/` aplicáveis no boot, idempotentes na sequência.
- **Acceptance check:** teste `TestMigrationsCoreSchema` (verifica existência de tabelas `clients`/`projects`/`project_members`/`tasks`, enums e colunas canônicas) **red→green**.

### T-02 — Role gate sobre o `RequireMember` existente (fase 01)
- **Objetivo:** **reusar** (NÃO reescrever) `RequireAuth` e `RequireMember` da fase 01 — que já resolvem `{workspaceId}`, devolvem `404` pra não-membro, `401` sem sessão e injetam `workspace_id`+role no contexto (T-09/T-12 da fase 01). O entregável **novo** é só o **role gate**: middleware que devolve `403` pra `viewer` em POST/PATCH/PUT/DELETE, encadeado após o `RequireMember` no subrouter desta fase (D-02).
- **Entradas:** `auth/middleware.go` e `workspace/middleware.go` da fase 01; tabela `workspace_members`; D-01/D-02 do plan.
- **Entregável:** role gate (ex.: `backend/internal/api/middleware_role.go`) + registro da cadeia no subrouter `/api/v1/workspaces/{workspaceId}` — sem duplicar código da fase 01.
- **Acceptance check:** testes `TestViewerEscritaProibida` (CA-23) e `TestViewerLeituraPermitida` (CA-24) **red→green** (httptest). CA-25 (`401` sem sessão) já é garantido pelo `RequireAuth` da fase 01 — cobrir aqui com um teste de **regressão/integração** nas rotas novas (`TestSemSessao401` pode nascer verde por reuso; documentar como verificação de wiring, não red→green).

### T-03 — Clients: service + validação
- **Objetivo:** regras de domínio de cliente (nome trim 1–120; arquivamento via `archived_at`).
- **Entradas:** T-01; D-03/D-07.
- **Entregável:** `backend/internal/clients/` (service + repo pgx).
- **Acceptance check:** teste `TestClienteNomeInvalido` (CA-02: vazio, só espaços, >120 → erro de validação, nada persiste) **red→green**.

### T-04 — Clients: handlers REST
- **Objetivo:** `GET/POST /clients`, `PATCH/DELETE /clients/{id}` conforme contrato `plan.md §3.1`.
- **Entradas:** T-02, T-03.
- **Entregável:** handlers em `backend/internal/api/` registrados no subrouter do workspace.
- **Acceptance check:** testes `TestCriarCliente201` (CA-01), `TestListarClientesSemArquivados` (CA-03, incl. `?archived=true`), `TestRenomearCliente` (CA-04) e `TestArquivarClienteMantemProjetos` (CA-05) **red→green**.

### T-05 — Projects: service + validações
- **Objetivo:** validar nome (trim 1–120), cor `#RRGGBB`, `weekly_hours_goal ∈ (0,168]`, `client_id` do mesmo workspace e **não arquivado**; criação com `owner_user_id` da sessão.
- **Entradas:** T-01; D-05/D-07.
- **Entregável:** `backend/internal/projects/` (service + repo).
- **Acceptance check:** testes `TestProjetoNomeInvalido` (CA-06.1: vazio, só espaços, >120 → 400), `TestProjetoCorEMetaInvalidas` (CA-07), `TestProjetoClienteDeOutroWorkspace` (CA-08) e `TestProjetoClienteArquivado400` (CA-08.1) **red→green**.

### T-06 — Projects: handlers create/patch/archive + permissão de metadados
- **Objetivo:** `POST/GET /projects`, `PATCH/DELETE /projects/{id}`; metadados editáveis só pelo dono do projeto ou owner do workspace.
- **Entradas:** T-02, T-05.
- **Entregável:** handlers REST conforme `plan.md §3.2`.
- **Acceptance check:** testes `TestCriarProjeto201` (CA-06), `TestPatchProjetoSoDonoOuOwner` (CA-14: dono 200; owner do ws 200 em projeto `shared`; member comum 403 em `shared`; **qualquer um que não seja o dono — inclusive owner do ws — recebe 404 em projeto `private` de outro**, conforme CA-09), `TestArquivarProjetoMantemTasks` (CA-15) e `TestDetalheProjetoArquivado200` (CA-15.1: GET detalhe 200 com `archived_at`; tasks dele seguem legíveis) **red→green**.

### T-07 — Projects: visibilidade privado × compartilhado nas queries
- **Objetivo:** repositório lista/detalha com `visibility='shared' OR owner_user_id=$user` (D-04); invisível = `404` idêntico a inexistente.
- **Entradas:** T-06.
- **Entregável:** queries de listagem/detalhe com regra de visibilidade num ponto único.
- **Acceptance check:** testes `TestProjetoPrivadoInvisivelParaOutros` (CA-09: fora da lista + detalhe/PATCH/DELETE/PUT members 404 — **inclusive pro owner do workspace**, privacidade absoluta) e `TestProjetoCompartilhadoVisivelPorTodos` (CA-10: owner/member/viewer veem) **red→green**.

### T-08 — Projects: membros (`PUT /projects/{id}/members` + `members` no POST)
- **Objetivo:** substituição transacional da lista de membros (dono implícito, fora da lista); persistir `members` enviados no `POST /projects` na mesma transação da criação; rejeitar não-membro do workspace e membros em projeto privado; remover `project_members` quando o projeto vira `private`.
- **Entradas:** T-06; D-06.
- **Entregável:** handler do PUT + ramo de members no POST + persistência em `project_members`.
- **Acceptance check:** testes `TestSubstituirMembrosDoProjeto` (CA-11), `TestCriarProjetoComMembros` (CA-11.1: POST shared com members → 201 com members persistidos e devolvidos), `TestMembroForaDoWorkspace400` (CA-12: nada persiste — no POST e no PUT), `TestPrivadoComMembros400` (CA-13: no POST, no PATCH→private e no PUT) e `TestPatchPrivateRemoveMembros` (CA-13.1: shared com membros → PATCH `visibility=private` sem campo members → `project_members` zerado na mesma transação) **red→green**.

### T-09 — Tasks: service + handlers create/list/get
- **Objetivo:** `POST /tasks` (status default `todo`, `created_by` da sessão, permissão de escrita D-05 — incluindo o `403` pra `access=view` e pra membro do ws fora de `project_members`), `GET /tasks` com filtros `project_id`/`status`, validações (nome trim 1–200, `due_date` ISO, `recurrence` ≤255, `notes`), bloqueio de escrita em projeto arquivado.
- **Entradas:** T-01, T-07, T-08.
- **Entregável:** `backend/internal/tasks/` + handlers conforme `plan.md §3.3`.
- **Acceptance check:** testes `TestCriarTaskDefaults` (CA-16), `TestEscritaTaskSemPermissao403` (CA-16.1: membro do projeto com `access=view` e member do ws fora de `project_members` → POST/PATCH/DELETE 403, nada persiste), `TestTaskNomeInvalido` (CA-16.2: vazio, só espaços, >200 → 400), `TestTaskProjetoInvisivel404` (CA-17: inexistente e privado de outro → resposta idêntica), `TestListarTasksComFiltros` (CA-19), `TestTaskCamposOpcionais` (CA-20: persiste os 3 + due_date inválida 400), `TestTaskRecurrenceLonga400` (CA-20.1: recurrence >255 → 400) e `TestTaskEmProjetoArquivado400` (CA-15.2: POST/PATCH em projeto arquivado → 400 `invalid_reference`; leitura segue 200) **red→green**.

### T-10 — Tasks: patch/delete + visibilidade herdada
- **Objetivo:** `PATCH /tasks/{id}` (enum de status), `DELETE` hard; tasks de projeto privado de outro usuário invisíveis em lista e detalhe.
- **Entradas:** T-09.
- **Entregável:** handlers restantes de tasks.
- **Acceptance check:** testes `TestPatchStatusTask` (CA-18: `doing` 200, valor inválido 400), `TestDeletarTaskHard` (CA-21) e `TestTaskHerdaVisibilidadeDoProjeto` (CA-22) **red→green**.

### T-11 — Teste de vazamento multi-tenant (crítico)
- **Objetivo:** garantir CA-26 ponta a ponta: 2 workspaces seedados; usuário do A referenciando ids reais do B (URL e payload) recebe `404` em clientes, projetos, membros de projeto e tasks; rota de workspace alheio → `404`.
- **Entradas:** T-04, T-08, T-10.
- **Entregável:** suíte `TestIsolamentoEntreWorkspaces` (tabela de casos cobrindo todos os recursos da fase) + correções que ela apontar.
- **Acceptance check:** `TestIsolamentoEntreWorkspaces` (CA-26) **red→green** (red = pelo menos um caso da tabela deve falhar se qualquer filtro for removido — validar com mutação manual antes do commit).

### T-12 — Smoke de performance das listagens (verificação informativa)
- **Objetivo:** seed de 500 registros na proporção fixa **50 clientes / 150 projetos / 300 tasks** num workspace; medir `GET /clients`, `GET /projects`, `GET /tasks` — cada um 3 vezes, vale a **melhor execução** — < 200 ms no Postgres efêmero local.
- **Entradas:** T-11 verde.
- **Entregável:** teste de integração com seed reutilizável em `backend/testdata/`.
- **Acceptance check:** teste `TestListagensSob200ms` (CA-32) **verde** — este é o único teste da fase **fora do ciclo red→green** (exceção documentada na spec: smoke de regressão tende a nascer verde e depende da máquina; não aplicar o anti-cheat do TDD.md aqui). Se falhar, a correção esperada é índice na migração; re-rodar.

## Frontend

### T-13 — Lib de API + hooks de query + `useRole`
- **Objetivo:** **estender** o cliente fetch tipado da fase 01 (`lib/api.ts`, que já parseia `{"error":{code,message}}`) com as chamadas desta fase, hooks TanStack Query pra clients/projects/tasks, hook `useRole()` a partir do `GET /me` (fase 01).
- **Entradas:** contratos `plan.md §3`; `lib/api.ts` da fase 01; baseline front fase 00.
- **Entregável:** extensão de `frontend/src/lib/api.ts` + `lib/queries/{clients,projects,tasks}.ts` (novos).
- **Acceptance check:** teste Vitest `api.spec.ts` (erro JSON vira exceção tipada com `code`; sucesso tipa o payload) **red→green**.

### T-14 — `ViewToggle` com persistência
- **Objetivo:** segmented control Cartões ↔ Tabela (visual de `clients.jsx`), estado persistido em `localStorage` por chave (`tt.view.clients` / `tt.view.projects`), default `cards`.
- **Entradas:** `tokens.css` (`--surface-3`/`--surface-1`).
- **Entregável:** `components/ui/ViewToggle.tsx`.
- **Acceptance check:** teste Vitest `ViewToggle.spec.tsx` (default cards; clique alterna; remonta lendo localStorage) **red→green**.

### T-15 — Página Clientes (cartões + tabela)
- **Objetivo:** `routes/cockpit/Clients.tsx` com `ClientCard` (sem campos fora do schema) e `ClientsTable` (`.tt-table`), header + botão "Novo cliente" (`.tt-btn--accent`), criação/renome/arquivamento ligados aos hooks.
- **Entradas:** T-13, T-14; `clients.jsx`; `tokens.css`.
- **Entregável:** página + `components/clients/{ClientCard,ClientsTable}.tsx`.
- **Acceptance check:** teste Vitest `Clients.spec.tsx` (renderiza cartões a partir de dados mockados; tabela só após toggle; criar cliente chama o hook certo) **red→green**.

### T-16 — Página Projetos (cartões + tabela) + teste canônico do toggle
- **Objetivo:** `routes/cockpit/Projects.tsx` com `ProjectCard` (régua de cor, nº de tasks, meta — elementos do `ProjRow` em cartão, D-11) e `ProjectsTable` (colunas do `projects.jsx` **adaptadas ao schema** — spec §6: "Esta semana"/"Total" como `0h`/`—` até a fase 03; Status = Ativo/Arquivado derivado de `archived_at`, sem o "Pausado" do protótipo); aqui fecha o critério CA-27 nas **duas** páginas.
- **Entradas:** T-13, T-14, T-15; `projects.jsx`.
- **Entregável:** página + `components/projects/{ProjectCard,ProjectsTable}.tsx`.
- **Acceptance check:** teste `cards-default.spec` (CA-27: Clientes **e** Projetos abrem em cartões; `.tt-table` só após toggle; escolha persiste por página após remontar) **red→green**.

### T-17 — `ProjectChip` com cor + barrinha de meta
- **Objetivo:** chip `.tt-chip`+`.tt-dot` com a cor do projeto; `.tt-goal` com largura `min(100, hours/goal×100)%` e fill na **cor do projeto** (troca pra `var(--accent)` quando ≥100% — regra do `ProjRow`, CA-30) recebendo `hours` por prop (página passa `0` até a fase 03); sem meta → sem barrinha.
- **Entradas:** `tokens.css`; DESIGN.md §3.1; `projects.jsx` (`ProjRow`).
- **Entregável:** `components/projects/ProjectChip.tsx` (usado em ClientCard e ProjectCard).
- **Acceptance check:** teste Vitest `ProjectChip.spec.tsx` (CA-30: 9h/18h → 50% com fill na cor do projeto; 20h/18h → 100% com fill accent; `goal=null` → sem `.tt-goal`) **red→green**.

### T-17.1 — UI de tasks no detalhe do projeto
- **Objetivo:** lista de tasks (nome, status, data, notas) dentro do detalhe/cartão expandido do projeto + criação de task inline (nome, data opcional, notas), refletida na lista sem recarregar a página; ações de escrita ocultas pra quem não tem permissão (CA-16.1/31).
- **Entradas:** T-13, T-16; hooks de `lib/queries/tasks.ts`; backend T-09/T-10.
- **Entregável:** componente de lista/criação de tasks em `components/projects/` integrado ao `ProjectCard`/detalhe.
- **Acceptance check:** teste Vitest `ProjectTasks.spec.tsx` (CA-30.1: renderiza tasks mockadas com status/data/notas; submeter criação chama o hook certo e a task aparece sem reload) **red→green**.

### T-18 — Modal Novo/Editar projeto
- **Objetivo:** `ProjectModal.tsx` fiel ao `newproject.jsx`: nome, cliente (select), paleta de cores `--p-*`, toggle Privado/Compartilhado, meta h/sem, lista de membros (checkbox + Editar/Só vê, Owner fixo `.tt-role--owner`); Privado desabilita a seção de membros; **modo Editar** abre pré-preenchido com os valores atuais do projeto e o submit aplica as alterações sem reload (CA-28.1).
- **Entradas:** T-13; `newproject.jsx`; `tokens.css`.
- **Entregável:** `components/projects/ProjectModal.tsx`.
- **Acceptance check:** testes Vitest `ProjectModal.spec.tsx` (CA-28: Privado → seção membros sem interação; Compartilhado → escolhe membro+acesso; Owner não removível) e `ProjectModalEdit.spec.tsx` (CA-28.1: abre com campos preenchidos; submit chama a mutation de PATCH e a lista reflete sem reload) **red→green**.

### T-19 — Fluxo criar projeto sem reload (E2E)
- **Objetivo:** ligar o submit do modal ao `POST /projects` com invalidação da query de projetos.
- **Entradas:** T-16, T-18; backend da fase rodando local.
- **Entregável:** integração modal→mutation→invalidate em `Projects.tsx`.
- **Acceptance check:** teste Playwright `novo-projeto.spec.ts` (CA-29: preenche modal → projeto aparece na lista sem reload → modal fecha) **red→green**.

### T-20 — Gating de viewer na UI
- **Objetivo:** com `useRole()==='viewer'`, ocultar "Novo cliente", "Novo projeto" e ações de editar/arquivar nas páginas da fase.
- **Entradas:** T-13, T-15, T-16.
- **Entregável:** condicionais de role nas duas páginas e cartões.
- **Acceptance check:** teste Vitest `viewer-gating.spec.tsx` (CA-31: role viewer mockada → zero botões de escrita; role member → botões presentes) **red→green**.

### T-21 — E2E do fluxo central + suíte verde da fase
- **Objetivo:** smoke E2E do problema central nº1 (parcial, sem timer): login stub → cria cliente → cria projeto compartilhado com membro `view` → cria task com data e nota → tudo listado certo; rodar a suíte completa (Go + Vitest + Playwright).
- **Entradas:** todas as anteriores.
- **Entregável:** `e2e/core-projetos.spec.ts` + suíte inteira verde (pré-requisito da fase 03).
- **Acceptance check:** `core-projetos.spec.ts` **red→green** e `go test ./... && vitest run && playwright test` 100% verdes.

---

## Matriz CA → tarefa (cobertura obrigatória)

| CA | Tarefa | CA | Tarefa | CA | Tarefa | CA | Tarefa |
|---|---|---|---|---|---|---|---|
| CA-01 | T-04 | CA-11 | T-08 | CA-16.1 | T-09 | CA-25 | T-02 (regressão) |
| CA-02 | T-03 | CA-11.1 | T-08 | CA-16.2 | T-09 | CA-26 | T-11 |
| CA-03 | T-04 | CA-12 | T-08 | CA-17 | T-09 | CA-27 | T-16 |
| CA-04 | T-04 | CA-13 | T-08 | CA-18 | T-10 | CA-28 | T-18 |
| CA-05 | T-04 | CA-13.1 | T-08 | CA-19 | T-09 | CA-28.1 | T-18 |
| CA-06 | T-06 | CA-14 | T-06 | CA-20 | T-09 | CA-29 | T-19 |
| CA-06.1 | T-05 | CA-15 | T-06 | CA-20.1 | T-09 | CA-30 | T-17 |
| CA-07 | T-05 | CA-15.1 | T-06 | CA-21 | T-10 | CA-30.1 | T-17.1 |
| CA-08 | T-05 | CA-15.2 | T-09 | CA-22 | T-10 | CA-31 | T-20 |
| CA-08.1 | T-05 | CA-16 | T-09 | CA-23 | T-02 | CA-32 | T-12 (smoke) |
| CA-09 | T-07 | CA-10 | T-07 | CA-24 | T-02 | — | — |

Tarefas de suporte sem CA próprio: T-01 (migrações), T-13 (lib api), T-14 (toggle), T-15
(página Clientes — o CA-27 fecha em T-16), T-21 (E2E + suíte verde). Exceções ao red→green
estrito: CA-25 (reuso do `RequireAuth` da fase 01 — teste de regressão/wiring) e CA-32 (smoke
informativo de performance) — todo o resto segue TDD.md sem desvio.
