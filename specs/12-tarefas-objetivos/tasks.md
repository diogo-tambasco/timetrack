# tasks.md — Fase 12 · Gerenciador de tarefas (objetivos → tasks)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-16).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → gate → objectives →
> board → objective_id em tasks → handlers), depois frontend (api/queries → componentes →
> rota → gating), E2E e gate. **Estende a fase 02 — não a reescreve** (RNF-05).

---

## Bloco 1 — backend domínio (T-01 → T-07)

- **T-01 — Migração `0013_objectives`** (CA-02/07/11 backstop, RNF-03)
  - `backend/internal/db/migrations/0013_objectives.sql` (plan §2): tabela `objectives`
    (+ enum `objective_status`, índice `(project_id, position)`, **`goal_hours numeric(6,2)`** com
    `CHECK (goal_hours IS NULL OR (goal_hours > 0 AND goal_hours <= 9999))`) +
    `ALTER TABLE tasks ADD COLUMN objective_id uuid REFERENCES objectives(id) ON DELETE SET NULL`
    + índice `(objective_id)` + `CREATE INDEX idx_entries_task ON time_entries (task_id) WHERE
    duration_seconds IS NOT NULL` (soma por task do board — RNF-04).
  - Conferir o diretório antes: `0012_calendar_settings` é o último → **`0013`** livre (fase 08
    pulada, sem contenção).
  - Teste (`migrations_*_test.go` ou novo): tabela existe, defaults (`status=open`,
    `description=''`, `position=0`); CHECK barra `goal_hours<=0` **e** `goal_hours>9999`;
    `goal_hours=9999` cabe (numeric(6,2) — pina o furo do `numeric(5,2)`); `ON DELETE SET NULL`
    zera `tasks.objective_id` ao apagar objetivo (não apaga a task); `idx_entries_task` existe.
  - INDEX.md: registrar 0013 dona da fase 12 (orquestrador).

- **T-02 — Gate de projeto AUTO-CONTIDO no `objectives` (D-03 alt. B — DECIDIDO; sem extração)**
  (RNF-05)
  - `internal/objectives/gate.go` (NOVO): `loadProjectGate(ctx, ws, userID, role, projectID,
    blockArchived) (gate, error)` — **cópia** da lógica de `tasks/service.go:110-138` (visibilidade
    `(visibility='shared' OR owner_user_id=$user)`→404; permissão dono ∥ `role=="owner"` ∥
    `project_members.access=="edit"`→403; arquivado quando `blockArchived`→400). Devolve também
    `canWrite`/`archived` pro `Board` montar `project.can_write` (CA-16). **NÃO** tocar
    `tasks.Service` nem `projects.Service` (ficam byte-idênticos — RNF-05 de fato); **NÃO** mudar
    `tasks.NewService`/`projects.NewService` (zero churn de construtor).
  - Teste `gate_test.go` (dbtest): private de outro → 404; `access=view`/member-sem-access → 403;
    arquivado+blockArchived → 400; dono/owner-ws/`access=edit` → ok; **paridade** com a rota de
    task (mesma entrada → mesmo erro). A suíte da fase 02 (tasks/projects) **continua verde sem
    NENHUMA mudança** (nada foi tocado). `canEditMeta` da fase 02 segue owner-only (não unificar
    com este gate — regras diferentes).

- **T-03 — `objectives.Service`: CRUD + validação** (CA-01/01.1/02/03/04/07/08/13/13.1)
  - `internal/objectives/service.go`: `Objective`, `CreateInput`/`PatchInput` (goal_hours
    tri-state — CA-04), `List`/`Create`/`Patch`/`Delete`. Validação na mão (D-07): nome 1–200,
    `goal_hours` rejeita `<=0` **e** `>9999` ANTES do SQL (CA-02 — nunca 500), `status ∈
    {open,done}`. Toda operação passa pelo `loadProjectGate` interno (visibilidade→404,
    escrita→403, arquivado→400). **Patch/Delete carregam o objetivo com `WHERE workspace_id AND
    project_id AND id`** + `uuid.Validate(objectiveId)` (CA-13.1). `position` auto no fim no Create.
  - Teste `service_test.go` (dbtest): create/list (ordem por position); validações não
    persistem; **`goal_hours=9999` cria (cabe), `goal_hours=9999.01`/`10000` → 400** (CA-02 borda);
    PATCH tri-state de goal_hours (null remove); delete hard + tasks viram Geral; escrita por
    view/viewer → 403; projeto invisível → 404; **objetivo do projeto B alcançado por rota do
    projeto A → 404** (mutação que remove `project_id=$rota` do WHERE deve morrer — CA-13.1);
    `objectiveId` malformado no path → 404 (não 500); **isolamento por workspace** (mutação
    removendo `WHERE workspace_id`/visibilidade morre — CA-15).

- **T-04 — Auto-status do objetivo + `objective_id` em tasks** (CA-05/06/06.1/09/09.1/11)
  - `tasks.Service`: `CreateInput`/`PatchInput` ganham `ObjectiveID *string` (+ `ObjectiveIDSet`
    tri-state — D-08); checagem `objective.project_id == task.project_id && mesmo workspace`
    senão `invalid_reference`; **`objective_id` malformado (não-uuid) no payload →
    `invalid_reference`** (`uuid.Validate` antes do SQL, padrão `checkClientRef` — CA-09.1).
    Recálculo na MESMA transação (D-09) **SÓ quando a mutação muda `status`/`objective_id`/cria/
    deleta** (edição benigna de `name`/`notes`/`due_date`/`recurrence` **NÃO** recalcula — D-04):
    helper SQL inline `recalcObjectiveStatus(tx, objectiveID)` — `n=count`, `d=done_count`; `n==0`
    no-op; `d==n && status='open'`→`done`; `d<n && status='done'`→`open`; senão no-op. Mover task
    A→B recalcula os dois. **Sem import de objectives** (plan §5).
  - Teste: marcar última task `done` → objetivo `done` (auto, CA-06); task volta `doing` →
    objetivo `open`; objetivo **sem task** NÃO auto-conclui nem é tocado (deletar a última task de
    um objetivo manual-done o mantém `done`, `n==0` no-op); `objective_id` de outro projeto/
    malformado → 400 `invalid_reference` (CA-09.1); PATCH `objective_id:null` desassocia (CA-09).
  - **Interação manual×auto (CA-06.1 — pina o furo da verificação):** (a) objetivo fechado
    MANUAL com `<100%` das tasks done + **edição benigna** de uma task (só `name`) → **continua
    `done`** (recálculo não dispara); (b) objetivo reaberto MANUAL (`status:open`) a 100% +
    edição benigna → **continua `open`**; (c) auto-done a 100% + task volta `doing` → reabre
    (transição real dispara). Mutação removendo o guard "só em transição" deve QUEBRAR (a)/(b).

- **T-05 — Board: read agregado único** (CA-16/16.1/17/18, RNF-04)
  - `internal/objectives/board.go`: `Board(ctx, ws, userID, projectID)` — 3 queries fixas (D-05):
    objetivos do projeto; tasks do projeto com `objective_id`; **soma por task** =
    `SUM(duration_seconds) WHERE user_id=$ AND workspace_id=$ AND task_id = ANY($tasks do projeto)
    AND duration_seconds IS NOT NULL GROUP BY task_id` (**SEM** filtrar `project_id` da entry —
    D-06); o `project.tracked_seconds` é uma soma **separada** `WHERE project_id=projeto AND
    user_id=$` (bucket independente — CA-17). Monta árvore: objetivos com `task_count/done_count/
    done_pct/tracked_seconds/tasks[]` + bucket `general` + `project.tracked_seconds` +
    **`project.can_write`** (via `loadProjectGate` — CA-16). Visibilidade pelo gate interno (404).
  - Teste: árvore correta (tasks no objetivo certo, Geral com as sem objetivo); `tracked_seconds`
    por task = soma das entries; **entry com `task_id` do projeto mas `project_id`=NULL conta na
    task/objetivo mas NÃO no projeto** (D-06 — buckets independentes, sem invariante `≥`); objetivo
    = soma das tasks; `done_pct` round + 0 com `task_count=0` (CA-16.1); **entry de OUTRO user não
    soma — mutação removendo `user_id` do SUM deve QUEBRAR** (CA-15); task deletada não infla
    (CA-07/17); deletar objetivo → board não lista o objetivo + tasks aparecem em `general` com
    `tracked_seconds` preservado (CA-07/11); `can_write` `false` pra `access=view`/arquivado,
    `true` pra dono/edit (CA-16/22).
  - **CA-18 (perf — exceção red→green, igual CA-32 da fase 02):** `backend/testdata/objectives/
    board_seed.sql` (30 obj / 300 tasks / 1.000 entries, placeholders `{{WORKSPACE_ID}}`/
    `{{USER_ID}}`, padrão `seed500.sql`) + `TestBoardSob200ms` (melhor de 3 < 200ms; nasce verde,
    pina o N+1/RNF-04).

- **T-06 — Handlers de objetivos + board** (CA-01..08/13/13.1/14/16)
  - `internal/api/objectives_handlers.go`: handlers finos (espelho dos de tasks/calendar) pra
    `GET/POST/PATCH/DELETE .../projects/{projectId}/objectives[/{id}]` e `GET .../projects/{projectId}/board`.
    `objective_id` adicionado aos handlers de task (`handleCreateTask`/`handlePatchTask` — tri-state
    via `json.RawMessage` no patch, padrão da fase 02).
  - Teste `objectives_handlers_test.go` (harness `coretest`): GET defaults 200; POST 201 +
    persiste; PATCH/DELETE; 400 validation/invalid_reference; 403 viewer; 404 projeto invisível;
    **404 objetivo de projeto B via rota de A** (CA-13.1); 401 sem sessão (table-driven nas rotas
    novas); board 200 com a árvore **e `project.can_write` no corpo** (CA-16).

- **T-07 — Wiring no `core_routes.go`** (CA-13/14/15)
  - `mountCore`: subárvore `/projects/{projectId}/objectives` (+ `/{objectiveId}`) e
    `/projects/{projectId}/board` no subrouter do workspace (cadeia RequireAuth→RequireMember→
    RequireWriteRole já aplicada). Injetar `objectives.Service` no `Core` (campo novo, `WithCore`
    estendido ou option própria). `cmd/server` liga o service.
  - Teste: rota viva (smoke de roteamento no `router_test`/`coretest`); escrita de objetivo por
    viewer barrada pelo role gate (403) ANTES do service (CA-12).

> **Fim do bloco 1:** `make test-back` 100% verde (objectives + tasks + projects intactos);
> Vitest/Playwright não rodados (bloco sem frontend). Handoff "retomar na T-08".

## Bloco 2 — frontend (T-08 → T-13)

- **T-08 — `lib/api.ts` + `lib/queries/objectives.ts`** (CA-09/16/19/20/21)
  - `Objective`/`ObjectiveInput`/`Board`/`BoardObjective` types; `Board.project` inclui
    **`can_write:boolean`** (CA-16/22); `objective_id` + `tracked_seconds` em `Task`;
    `getBoard`/`listObjectives`/`createObjective`/`patchObjective`/`deleteObjective`; `objective_id`
    em `TaskInput`/`TaskFilters`. Hooks `useBoard`, `useCreateObjective`/`usePatch…`/`useDelete…`
    (invalidam `boardKey`). Mutações de **task** no board: invalidação do `boardKey` via
    `onSuccess` **local** no `ProjectBoard` (T-11) — **NÃO** estender `lib/queries/tasks.ts` da
    fase 02 (aditivo/local, sem regressão — plan §6).
  - Teste `objectives-api.test.ts`: chama os paths certos; hooks invalidam a query certa.

- **T-09 — `ProgressBar` + `ObjectiveCard` + `ObjectiveModal`** (CA-19/21/22/23)
  - `ProgressBar` (% de tasks done sempre + barra de horas via `.tt-goal` só com `goal_hours` —
    CA-23, regra do `.tt-goal` da fase 02). `ObjectiveCard` (nome, progresso, ações gated por
    **`board.project.can_write`** — CA-22, não a role do ws). `ObjectiveModal` (nome/descrição/
    goal_hours; validação `0 < goal_hours <= 9999` no client antes do POST/PATCH — CA-21/02).
  - Specs: barra de horas ausente sem meta / presente com meta / accent ≥100%; modal inválido
    (nome vazio, goal_hours fora de (0,9999]) não chama API; ações escondidas quando
    `can_write=false` (CA-22, padrão `viewer-gating.spec` da fase 02).

- **T-10 — `TaskRow` + `TaskModal` + `TaskBoardColumns`** (CA-20/21/22)
  - `TaskRow` (nome, status, data, `tracked_seconds` formatado, editar). `TaskModal` (nome,
    status, data, notas, **select de objetivo** com "Geral" — criar/editar). `TaskBoardColumns`
    (3 colunas todo/doing/done; mover = `PATCH status`, sem DnD — D-12).
  - Specs: criar/editar task chama a API com `objective_id` certo (incl. null→Geral); mover de
    coluna dispara PATCH status; gating por `board.project.can_write` (CA-22).

- **T-11 — `ProjectBoard` + rota `ProjectDetail`** (CA-19/20/22)
  - `ProjectBoard` orquestra objetivos (cada um `ObjectiveCard` + tasks) + bucket "Geral", modo
    board↔lista (toggle persistido `tt.view.board` — D-11). Mutações de task (mover/criar/editar)
    invalidam `boardKey` via **`onSuccess` local** (não mexe em `lib/queries/tasks.ts` — plan §6).
    Gating por **`board.project.can_write`** (CA-22). `routes/cockpit/ProjectDetail.tsx`
    (`/projetos/{id}`) consome `useBoard`; header com nome/cor/tempo total. Registrar a rota em
    `App.tsx` (`<Route path="/projetos/:id">`).
  - Specs: board renderiza objetivos + Geral; toggle troca board/lista; tempo total no header;
    mover task no board dispara `PATCH status` e reflete (CA-20).

- **T-12 — Linkar Projetos → detalhe + aposentar `ProjectTasks` inline** (CA-19, RNF-05)
  - Cartão/linha de projeto em `Projects.tsx` vira link pra `/projetos/{id}` (sem reescrever a
    página — D-10). Aposentar o `ProjectTasks.tsx` da fase 02 (lista inline no cartão) se o board
    o substitui; reescrever/remover seu spec — desvio sancionado registrado (a fase 02 é dona do
    CRUD de task, não da UI de gestão; a fidelidade segue assertada no board).
  - Specs: clicar no cartão navega pro detalhe; `Projects.spec`/`ProjectTasks.spec` ajustados.

- **T-13 — Conclusão automática na UI** (CA-20/CA-06 ponta a ponta)
  - Garantir que mover a última task pra `done` reflete o objetivo concluído após invalidação do
    board (CA-06 chega no refetch); badge/estado "concluído" no `ObjectiveCard`; reabrir ao
    quebrar 100%.
  - Spec (jsdom, board mockado): última task done → cartão mostra concluído; volta pra doing →
    reabre.

> **Fim do bloco 2:** Vitest 100% verde; `make test-back` verde (regressão zero). Playwright
> não rodado ainda (E2E é o bloco 3). Handoff "retomar na T-14".

## Bloco 3 — E2E + verificação + gate (T-14 → T-16)

- **T-14 — E2E `tarefas.spec.ts`** (CA-24)
  - Logado+escrita (stub fase 01) num projeto `shared`: cria objetivo com meta de horas; cria 2
    tasks (uma vai pra Geral); move uma pra `done` no board → progresso sobe; fecha a última →
    objetivo conclui automático; **reload persiste**. **SEM** segundo perfil viewer no E2E (o
    stub autentica só o owner — CA-24): o gating de `viewer`/`access=view` é coberto por **teste
    de componente jsdom** (`viewer-gating` com `can_write=false` mockado) nas T-09/T-10/T-13.
  - Reusar `loginViaStub`; nomes únicos (banco reusado, idempotente).

- **T-15 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: isolamento multi-tenant/visibilidade, auto-status ×
    fecho manual / corridas, agregação de tempo / bucket Geral / entry sem task, tri-state de
    objective_id, regressão fase 02/03) + refutação dupla. Corrigir confirmados. Registrar.

- **T-16 — Gate + deploy + smoke + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose).
  - Deploy pela rotina do KICKOFF (rsync + `docker compose build && up -d` em `/opt/timetrack`;
    **NÃO** tocar no Caddy do PrimaOS). Smoke honesto (`/healthz` 200; `/api/v1/workspaces/{id}/
    projects/{id}/board` sem sessão → 401 canônico; bundle contém o board; vizinhos intactos).
  - Handoff (CLAUDE.md "retomar..."), INDEX (12 construída, 0013 dona), KICKOFF da fase 13,
    commit, briefing pro Diogo.

---

## Blocos (regra ≤10 tarefas / contexto zerado entre blocos)

- **Bloco 1 — backend domínio:** T-01 → T-07 (migração, gate auto-contido, objectives.Service,
  auto-status + objective_id, board, handlers, wiring). Suíte: `make test-back`.
- **Bloco 2 — frontend:** T-08 → T-13 (api/queries, componentes, board, rota detalhe, link,
  auto-conclusão na UI). Suíte: Vitest + `make test-back` (regressão).
- **Bloco 3 — E2E + fechamento:** T-14 → T-16 (E2E, verificação adversarial, gate `make
  check-full` + deploy + handoff).

`make check-full` é gate **só no bloco 3 (T-16)**; os blocos 1 e 2 fecham com a suíte do que
existe verde + handoff + commit, conforme a regra de blocos do `CLAUDE.md`.

---

## Verificação adversarial da SPEC (ETAPA 1 — Regra Nº1, FEITA 19/06/2026)

Workflow ultracode `verify-fase12-spec` (8 dimensões × refutação dupla A/B, **50 agentes**,
~4,1M tokens) contra a spec/plan/tasks da fase 12 ancorado no **código real** das fases 02/03.
**21 findings brutos → 7 confirmados (2/2) + 8 splits (1/2, adjudicados) + 6 refutados.**
**Todos os confirmados e os splits-reais foram corrigidos na spec/plan/tasks/MILESTONE ANTES do
build.** (3 refuters caíram por erro de API; adjudicados pela evidência da lente sobrevivente.)

**Corrigidos (CONFIRMADOS 2/2):**
1. **`goal_hours numeric(5,2)` × faixa `(0,9999]`** (HIGH, achado por 2 dimensões) — `numeric(5,2)`
   só vai a `999.99`; `goal_hours=9999` estouraria (`22003`) → `500` em vez de `400`. **Fix:**
   coluna → `numeric(6,2)` + `CHECK <= 9999` + validação de teto no service ANTES do SQL (CA-02,
   plan §2/§5, T-01/T-03, MILESTONE §5).
2. **Recálculo absoluto destrói fecho manual** (HIGH) + **destrói reabertura manual** (MEDIUM) —
   o modelo só tem `status enum`, sem distinguir manual×auto; recompute "senão open" em TODA
   mutação reabria objetivo fechado manual a <100% ao renomear uma task. **Fix:** recálculo
   **disparado por transição** (só em criação/deleção/mudança de `status`/`objective_id`; edição
   benigna não recalcula) + `task_count==0` no-op — **sem coluna nova** (D-04, CA-06 reescrito +
   **CA-06.1 novo**, T-04 com testes da interação manual×auto).
3. **CA-16/D-06/T-05 ambíguos no escopo da soma por task** (HIGH) — `entry.project_id` vs
   `task_id ∈ projeto`; a 1ª leitura PERDE entries com `task_id` do projeto mas `project_id`
   nulo/diverso. **Fix:** soma por task = `task_id ∈ tasks do projeto` (sem filtrar `project_id`);
   projeto = por `project_id` (bucket independente) (CA-16/17, D-06, RNF-04, T-05).
4. **E2E viewer inexecutável** (HIGH) — o `googlestub` autentica 1 usuário só; não dá pra logar
   como viewer. **Fix:** gating viewer vira teste de componente jsdom; E2E só o fluxo do owner
   (CA-24, T-14).
5. **`objective_id` malformado no payload sem código** (MEDIUM) — precedentes divergentes na base
   (client_id→invalid_reference × refs→not_found). **Fix:** payload malformado → `invalid_reference`
   (padrão `checkClientRef`); `validation_error` exclusivo do filtro (CA-09.1, T-04).

**Corrigidos (SPLITS adjudicados REAL):**
6. **`/board` não expõe permissão por-projeto** → gating CA-22 não-implementável só com a role do
   ws. **Fix:** `project.can_write` no contrato do board, computado pelo gate no backend (CA-16/22,
   plan §4.2/§6, T-05/T-08/T-09/T-10/T-11).
7. **Vazamento cross-project de objetivo** — `PATCH /projects/A/objectives/{id-de-B}` editaria
   objetivo de projeto vizinho. **Fix:** **CA-13.1 novo** (carga `WHERE workspace_id AND project_id
   AND id`; `uuid.Validate` antes → 404 nunca 500), T-03/T-06 com mutation-test.
8. **CA-18 (perf board) órfão** — nenhuma tarefa cria seed/teste. **Fix:** T-05 dona
   (`board_seed.sql` 30/300/1000 + `TestBoardSob200ms`).
9. **Invariante D-06 `projeto ≥ Σ` FALSA** (`project_id`/`task_id` independentes). **Fix:** D-06/
   CA-17 reescritos (buckets independentes, sem invariante).
10. **D-03 "decidir na T-02" + churn de construtor** num build autônomo escolheria o caminho
    invasivo. **Fix:** pré-decidida a **alternativa B** — gate **auto-contido** no `objectives`
    (cópia ~25 linhas), `tasks`/`projects` byte-intocados (D-03/§5/§7, T-02).

**Endurecimentos baratos aplicados (de findings refutados, já que a spec estava sendo tocada):**
invalidação do board **imperativa** no `onSuccess` local (plan §6, T-08/T-11); mutation-test do
`user_id` no SUM (T-05); nota `project_id` imutável (plan §4.3); índice `idx_entries_task` +
redação do RNF-04 (plan §2, T-01).

**Refutados (0/2 — NÃO corrigidos, registrados pra não re-litigar):** mutações de task não
invalidarem o board (o plan já nomeava o caminho; firmado pra imperativo) · agregação por
`user_id` "não-pinada" (CA-15 já obriga o teste de vazamento) · entry de task deletada no tracked
do projeto (CA-17 já cobre) · objetivo deletado some do board (cadeia CA-07↔11↔16↔17 coerente) ·
`project_id` imutável no PATCH (verificação positiva; virou nota) · RNF-04 alegar índice
inexistente (leitura equivocada do texto; ainda assim adicionei o índice + ajustei a redação).
