# tasks.md — Fase 20 · Lembretes (due date + manual)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-14).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → store → dispatcher → worker → hook
> do `due` → rotas → wiring), depois o frontend (banner por kind → api/queries → botão + lista),
> E2E, verificação adversarial e gate. **NÃO toca o pacote `guard`** (CA-26).

---

## Bloco backend (modelo + worker + due + rotas)

- **T-01 — Migração `0020_reminders`** (CA-01/02/03)
  - Objetivo: criar a tabela `reminders` + os índices.
  - Entregável: `backend/internal/db/migrations/0020_reminders.sql` (número fixado lendo a pasta —
    próximo livre após `0019`; nome lógico `reminders`). Tabela `reminders` (plan §2) + `idx_
    reminders_pending` (parcial `WHERE status='pending'`) + `uq_reminders_due_pending` (unique
    parcial `WHERE kind='due' AND status='pending'`) + `idx_reminders_user`. FKs `ON DELETE
    CASCADE` a `users`/`tasks`. CHECKs de `kind`/`status`.
  - Acceptance: `migrations_reminders_test.go` (dbtest) — tabela/colunas/defaults existem; `status`
    default `pending`; índice parcial pending presente; **2 inserts `due` pendentes da mesma task →
    o 2º viola o unique** (CA-03); FK cascade (apagar task limpa reminders); CHECK rejeita `kind`/
    `status` fora do enum.
  - INDEX.md: registrar 0020 (número final) dona da fase 20.

- **T-02 — `reminders.Store` (repo)** (CA-05/12/13/18/19/20/21)
  - Objetivo: persistência dos reminders, isolada por user, na tx de tasks ou própria.
  - Entregável: `internal/reminders/store.go` — `Create(ctx, tx, Reminder)`,
    `CancelDuePending(ctx, tx, taskID)` (marca canceled o `due` pendente da task; 0..1),
    `ListByUser(ctx, userID, status)` (join `tasks.name`, `WHERE user_id`, ordenado por remind_at),
    `Cancel(ctx, userID, reminderID) (found, err)` (soft-cancel só do user; found p/ 204×404).
  - Acceptance: `store_test.go` (dbtest, RED primeiro) — Create persiste pending; CancelDuePending
    marca só o `due` pendente (não toca sent/manual); ListByUser **só vê os do user** e traz
    `task_name` (mutação removendo `WHERE user_id` morre — CA-21); Cancel de outro user → `found=
    false` (CA-19); Cancel idempotente em já-canceled (CA-18).

- **T-03 — `reminders.Dispatcher.RunOnce` — despacho de uma passada** (CA-04/05/06/07/08/09/10, RNF-02/03)
  - Objetivo: a varredura/despacho, espelhando `guard.Cutter` (lock + FOR UPDATE + flip), clock
    injetável.
  - Entregável: `internal/reminders/dispatcher.go` — `Dispatcher` com `db`, `now` (`WithClock`),
    `onNotice`/`onPush` (`WithOnNotice`/`WithOnPush`). Seleção `status='pending' AND remind_at <=
    now` (inclusivo); despacho por reminder na tx (advisory lock `hashtext(user_id)` + `FOR UPDATE`
    re-check + flip `pending→sent` + carrega `task_id`/`title`); pós-commit chama `onNotice`/
    `onPush` só pros despachados.
  - Acceptance: `dispatcher_test.go` (dbtest, RED primeiro) — **casos obrigatórios:**
    - `remind_at == now` despacha; `remind_at` no futuro NÃO; sent/canceled nunca entram (CA-04);
    - despachado vira `status='sent'`, `remind_at`/`kind`/`task_id` intactos (CA-05);
    - `onNotice`/`onPush` chamados **só** pros despachados nesta passada; passada sem vencido = zero
      (spy nos hooks — CA-06);
    - payload do notice = `{"kind":"reminder","task_id","title"}` com o nome da task (CA-07);
    - **idempotência:** 2× `RunOnce` → exatamente 1 notice/push pro reminder (CA-08);
    - **corrida:** cancelar o reminder entre a seleção e o despacho (simular: UPDATE status=canceled
      antes do RunOnce sob lock) → o `FOR UPDATE`/`status='pending'` aborta, zero aviso (CA-08);
    - isolamento: 2 usuários com reminders vencidos → cada notice/push vai pro **dono** certo;
      mutação removendo `WHERE user_id`/o user do payload morre (CA-09);
    - best-effort do push: o `onPush` recebe (userID, payload) corretos; falha do envio não impede
      o flip nem o notice (o `onPush` é só o wiring — o best-effort real é do `Sender`, CA-10).

- **T-04 — `reminders.Worker` — ticker + goroutine** (CA-29, RNF-01)
  - Objetivo: o ticker de background, espelhando `guard.Worker`.
  - Entregável: `internal/reminders/worker.go` — `Worker.Run(ctx)` (`time.Ticker` de
    `config.ReminderTick`, default 60s; `<-ticker.C` → `RunOnce`; `<-ctx.Done()` → return). Log de
    partida + por despacho. NÃO roda no boot imediato.
  - Acceptance: `worker_test.go` (espelha `guard/worker_test.go`) — `Run` com intervalo de ms
    despacha o reminder vencido (RunOnce roda ≥1×) e **retorna** ao cancelar o ctx (sem zumbi);
    intervalo `0` → default sem explodir.

- **T-05 — Helper `dueRemindAt` + criação do `due` no `tasks.Service`** (CA-11/12/12.1/13/14/15/28)
  - Objetivo: o reminder `due` nasce/cancela na escrita da `due_date`, na MESMA tx, tz-correto,
    com gatilho de **mudança de valor** (não presença do campo — CA-12.1).
  - Entregável: `internal/reminders/due.go` — `dueRemindAt(dueDate, tz string) (time.Time, error)`
    (`time.Date(y,m,d,9,0,0,0,loc)`, tz vazio→UTC, tz inválido→erro). `ReminderHook` interface +
    impl `reminders.Hook` (`OnDueDateSet(ctx, tx, taskID, userID, dueDate, tz)`: cancela due
    pendente + cria novo se `dueDate != nil`; resolve `user_id = assignee ?? created_by` lendo a
    task na tx). `tasks.Service.SetReminderHook(h)` (default no-op — RNF-05); `Create`/`Patch`
    chamam o hook **SOMENTE quando a `due_date` MUDOU DE VALOR** (`oldDueDate != newDueDate`; no
    `Create`, `due_date != nil`) — **NÃO** por `DueDateSet==true`. Re-setar a MESMA data = NO-OP
    (hook nem roda; preserva o `due` `sent`). Incluir o **ramo de edição benigna do Patch** (tornar
    "due_date mudou de valor" condição de `needsTx`).
  - Acceptance (RED primeiro):
    - `due_test.go`: `dueRemindAt("2026-06-25","America/Sao_Paulo")` = `2026-06-25T09:00-03:00`;
      tz vazio → UTC; tz inválido → erro (CA-11 base).
    - `tasks` (harness existente + hook fake/real): `Create` com due_date → 1 reminder `due`
      pending pro created_by com remind_at certo (CA-11); `Patch` muda due_date pra **outra** data →
      cancela o antigo, cria 1 novo, nunca 2 pendentes (CA-12); **`Patch` reescreve a MESMA
      due_date com o `due` antigo já `sent` → NENHUM reminder novo, o `sent` permanece e nenhuma
      `RunOnce` re-dispara (CA-12.1)**; `Patch` due_date→null → cancela, nenhum novo (CA-13);
      assignee setado (SQL) → due pro assignee (CA-14); due_date passado → pending no passado,
      despachado na 1ª `RunOnce` (CA-15); **as suítes de tasks da 02/12 seguem verdes** sem due_date
      (CA-28).

- **T-06 — Rotas manual + cancelar + lista** (CA-16/17/18/19/20/21/22/23/24/25)
  - Objetivo: a API REST dos reminders.
  - Entregável: `internal/api/reminder_routes.go` — `POST /workspaces/{id}/tasks/{taskId}/reminders`
    (decodifica `remind_at`, valida RFC3339 → 400; gate por-task: `tasks.Get` p/ visibilidade →
    404; role owner/member ok, viewer 403 EXCETO assignee da task; `user_id=sessão`; 201) ·
    `DELETE /workspaces/{id}/tasks/{taskId}/reminders/{reminderId}` (`store.Cancel(userID,id)`;
    valida que o reminder é da task da URL; found→204, senão 404) · `GET /api/v1/reminders` (por
    usuário, `status` default pending; lista isolada). `WithReminders` Option; mount no subrouter de
    task (atrás de `RequireMember`, SEM `RequireWriteRole`) + `GET /reminders` no `/api/v1`.
  - Acceptance: `reminder_routes_test.go` (harness api) — POST válido 201 + persiste pending pro
    user da sessão; remind_at ausente/inválido 400 (CA-17); task invisível/`taskId` malformado 404
    (CA-25); viewer comum 403, viewer-assignee 201 (CA-23); owner/member 201 (CA-22); DELETE do dono
    204 + soft-cancel, de outro user 404 (CA-18/19); `GET /reminders` só os do user, cross-ws,
    ordenado (CA-20/21); **401 sem sessão nas três rotas** (CA-24).

- **T-07 — Config `REMINDER_TICK` + wiring no boot** (CA-29, RNF-01)
  - Objetivo: ligar o worker + as rotas + o hook do `due`.
  - Entregável: `config.go` (+`ReminderTick`, env `REMINDER_TICK`, 0/inválido→0→default 60s no
    Worker; espelha `loadGuardTick`). `cmd/server/main.go`: `reminders.NewStore(pool)`,
    `reminders.NewDispatcher(pool, WithOnNotice(hub.BroadcastNotice), WithOnPush(func(u,p){ go
    pushSender.SendToUser(ctx, u, p) }))`, `reminders.NewWorker(...)`, `go worker.Run(ctx)`;
    `tsvc`/`tasks` recebe `SetReminderHook(reminders.NewHook(store))`; `api.WithReminders(...)` no
    router.
  - Acceptance: `config_test.go` (defaults/parse de `REMINDER_TICK`); o boot liga o worker sem
    quebrar (exceção ao red estrutural documentada — espelha o wiring do guard da 16). Log
    "reminder worker iniciado".

## Bloco frontend + E2E + gate

- **T-08 — `noticeStore`/`NoticeBanner` por kind + api/queries** (CA-27, CA-20/16/18)
  - Objetivo: o banner reconhece o lembrete; o front fala com a API.
  - Entregável: `lib/noticeStore.ts` (`pushNotice` aceita `kind:'reminder'` além de
    `timer_auto_cut`); `components/NoticeBanner.tsx` (texto/ícone por kind — corte intocado,
    lembrete = "Lembrete: <title>"); `lib/api.ts` (+`listReminders`/`createReminder`/
    `cancelReminder`); `lib/queries/reminders.ts` (TanStack; mutações invalidam a lista).
  - Acceptance (Vitest jsdom): `pushNotice` com `kind:'reminder'` mostra o banner; `timer_auto_cut`
    segue mostrando o de corte (regressão da 16 preservada — CA-27); kind desconhecido ignorado;
    banner ausente sem notice. Mock de `../lib/api` pras queries.

- **T-09 — `ReminderButton` + lista "Meus lembretes"** (CA-16/18/30)
  - Objetivo: a UI de criar/cancelar/listar.
  - Entregável: `components/ReminderButton.tsx` (ação "Lembrar" no detalhe/modal de task — picker de
    quando: presets + input → `createReminder`); bloco "Meus lembretes" no `/ajustes` (grupo,
    espelha o de Notificações da 16): lista pendentes (`task_name`+`remind_at`) + cancelar.
  - Acceptance (Vitest jsdom): clicar Lembrar com um "quando" válido chama `createReminder` com o
    `remind_at` certo; form inválido não chama API; a lista renderiza os pendentes do
    `listReminders` mockado; cancelar chama `cancelReminder` e invalida a lista.

- **T-10 — E2E `reminders.spec.ts`** (CA-30)
  - Objetivo: o fluxo ponta a ponta com o worker tickando.
  - Entregável: `frontend/e2e/reminders.spec.ts` + `e2e/helpers.ts::seedReminder` (SQL, padrão
    `seedForgottenTimer`: insere um reminder `manual` pending com `remind_at` no passado pra uma
    task do user do stub). Logado (stub), com o worker em intervalo curto (env de E2E no webServer)
    e o app aberto: ver o **banner de lembrete** (`notice-banner` com texto de reminder); depois,
    criar/abrir a lista "Meus lembretes", **cancelar** um pendente e confirmar que some. Web Push
    real NÃO exercitado (unit no back). Reusar `loginViaStub`; **liberar a :8080** antes do E2E.
  - Acceptance: o spec roda verde; red honesto (rodar antes do wiring de UI/worker — registrar
    qualquer exceção ao red estrutural no cabeçalho do spec, padrão da fase 16).

- **T-11 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: **limiar/seleção do despacho · idempotência/lock/corrida ·
    isolamento por user (notice/push/lista/cancel) · criação/cancelamento do `due` na escrita
    (incl. ramo benigno do Patch) — gatilho por MUDANÇA DE VALOR, não presença do campo; re-setar a
    mesma data não re-arma o `sent` (CA-12.1) + tz do 09:00 · best-effort push/wiring · permissões
    (viewer-assignee carve-out, 403/404/401) · não-regressão `guard`/tasks 02-12-18/notice-push
    16**) + refutação dupla. Corrigir confirmados red→green. Registrar no fim deste arquivo.

- **T-12 — Gate + handoff** (NÃO deploy isolado — ver nota)
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Antes do E2E/
    gate, liberar a :8080** de qualquer dev server alheio.
  - Handoff (CLAUDE.md "retomar…"), INDEX (20 construída), commit, briefing pro Diogo. **Deploy:**
    seguir a política da milestone (deploy único no fim da M2, ou conforme o KICKOFF da sessão de
    build). `REMINDER_TICK` é opcional (default 60s); nenhuma env nova obrigatória.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

Furos confirmados 2/2 aplicados na spec/plan/tasks ANTES de codar: **1**.

- **[MEDIUM] Re-setar a MESMA `due_date` re-armava um lembrete não-pedido.** O gatilho do hook do
  `due` era "due_date setada" (`DueDateSet==true`): um `Patch` benigno que reescrevesse a mesma data
  X, com o `due` antigo já `sent`, criava um novo `due` pending com `remind_at` no passado → a
  próxima `RunOnce` re-disparava um aviso já dado. **Decisão (travada):** o gatilho passa a ser
  **"due_date MUDOU DE VALOR"** (`oldDueDate != newDueDate`); re-setar a mesma data é NO-OP no
  reminder (não cancela o pendente, não recria, preserva o `sent`). Mudanças: §2.2 reescrita (gatilho
  por mudança de valor) + CA-11/CA-12/CA-13 ajustados pra falar em "mudou de valor" + **CA-12.1 novo**
  (Patch reescrevendo a MESMA due_date com `due` já `sent` → nenhum reminder novo, `sent` intacto,
  com teste) no spec; D-02/§9 do plan (interface `OnDueDateSet` só roda na divergência; armadilha do
  `DueDateSet` adicionada) e contrato do hook; **T-05** com a condição de gatilho corrigida + o caso
  de teste de CA-12.1, e a dimensão de verificação da T-11 estendida.

> Refutados/descartados não entram neste registro.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> PENDENTE — suítes finais, findings do `verify-build`, correções aplicadas.
