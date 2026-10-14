# plan.md — Fase 20 · Lembretes (due date + manual)

> Como construir a spec. Decisões de design (D-xx) + arquitetura + contratos + ordem de tarefas.
> **Espelha** o `guard.Worker`/`guard.Cutter` da fase 16 (worker próprio, clock injetável, advisory
> lock por usuário) e **reusa** o `Hub.BroadcastNotice` + `push.Sender` da 16 pra entregar o aviso.
> **NÃO toca o pacote `guard`.** Adiciona: migração `0020`, pacote `internal/reminders`, criação do
> reminder `due` na escrita de task (efeito aditivo no `tasks.Service`), rotas de reminder, UI.

---

## 1. Arquitetura geral

```
Backend:
  db/migrations/0020_reminders.sql           ← tabela reminders + idx parcial pending + uq due-pending
  internal/reminders/              (NOVO pacote, espelha guard — NÃO importa/edita guard)
     store.go                                ← repo: Create, CancelDuePending, ListByUser, Cancel, Get
     store_test.go                           ← TDD (CRUD, isolamento, unique due-pending)
     dispatcher.go                           ← RunOnce (clock injetável): varre pending vencidos, flip
                                               pending→sent sob lock, hooks onNotice/onPush pós-commit
     dispatcher_test.go                      ← TDD (limiar, flip, idempotência, lock/corrida, isolamento)
     worker.go                               ← ticker + goroutine (Run(ctx) → para no shutdown) — espelha guard.Worker
     worker_test.go                          ← TDD (tica/despacha/para no ctx cancel)
  internal/tasks/service.go (existe)         ← Create/Patch passam a criar/cancelar o reminder due
                                               (efeito aditivo na MESMA tx; recebe um ReminderHook injetável)
  internal/api/reminder_routes.go  (NOVO)    ← POST /workspaces/{id}/tasks/{taskId}/reminders (manual)
                                               + DELETE /workspaces/{id}/tasks/{taskId}/reminders/{id}
                                               + GET /api/v1/reminders (os meus, cross-ws — por usuário)
     reminder_routes_test.go                 ← 201/400/401/403/404; cancela só dono; lista isolada
  internal/config/config.go (existe)         ← + REMINDER_TICK (env; espelha GUARD_TICK)
  cmd/server/main.go (existe)                ← liga reminders.Dispatcher + Worker + WithReminders;
                                               injeta o ReminderHook no tasks.Service

Frontend:
  lib/noticeStore.ts (existe)                ← pushNotice reconhece kind:"reminder" além de timer_auto_cut
  components/NoticeBanner.tsx (existe)       ← texto do banner por kind (corte × lembrete)
  lib/api.ts (existe)                        ← + listReminders / createReminder / cancelReminder
  lib/queries/reminders.ts        (NOVO)     ← TanStack Query (lista + mutações invalidam)
  components/ReminderButton.tsx   (NOVO)     ← ação "lembrar" no detalhe/modal de task (picker de quando)
  routes/.../Reminders (lista)    (NOVO)     ← seção "Meus lembretes" (pendentes + cancelar) — no /ajustes
                                               OU bloco no dashboard (decisão de UX na T)
  e2e/reminders.spec.ts           (NOVO)     ← cria manual (clock curto) → banner; cancela → some
  e2e/helpers.ts (existe)                    ← + seedReminder (SQL, padrão seedForgottenTimer)
```

## 2. Migração — `0020_reminders.sql` (dona: fase 20)

> **Número:** `0017→17, 0018→18, 0019→19` são da M2 (fases anteriores). A T-01 fixa o número lendo
> `backend/internal/db/migrations/` (próximo livre). Nome lógico `reminders`. Conteúdo (assumindo
> 0020):

```sql
-- 0020_reminders — fase 20: lembretes (due/manual). user_id = quem é lembrado.
CREATE TABLE reminders (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,                              -- denorm (filtro tenant, igual tasks)
  remind_at    timestamptz NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('due','manual')),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','canceled')),
  created_at   timestamptz NOT NULL DEFAULT now()
);
-- varredura barata do worker (RNF-01): só a fatia pending
CREATE INDEX idx_reminders_pending ON reminders (remind_at) WHERE status = 'pending';
-- no máximo 1 reminder due pendente por task (backstop da idempotência §2.2 — CA-03/12)
CREATE UNIQUE INDEX uq_reminders_due_pending ON reminders (task_id)
  WHERE kind = 'due' AND status = 'pending';
-- listar os meus (CA-20) — por user_id; remind_at pra ordenar
CREATE INDEX idx_reminders_user ON reminders (user_id, remind_at);
```

- **`workspace_id` denorm** sem FK explícita formal não é exigido pelo PRD; manter FK pra
  `workspaces(id)` é coerente com `tasks`. (Decisão T-01: seguir o padrão de `tasks` — FK a
  `workspaces`.) O filtro tenant nas rotas usa `task_id`+visibilidade (a verdade), o `workspace_id`
  é conveniência/consistência.
- INDEX.md: registrar **0020 dona da fase 20**.

## 3. Backend — pacote `internal/reminders`

### D-01 — pacote novo, espelha `guard`, NÃO importa `guard`

O despacho de reminders é background, como o corte da fase 16 — mas é **outra preocupação**
(prazos × timer esquecido). Pra não inchar/arriscar o `guard` (CA-26/RNF-05), o despacho vive num
pacote NOVO `reminders` que **copia o formato** do `guard` (Cutter→Dispatcher, Worker→Worker,
`WithClock`, advisory lock, `FOR UPDATE` re-check) mas é independente. **Não há `import
".../guard"`.** O `Dispatcher` recebe `*pgxpool.Pool`, o `onNotice` (= `hub.BroadcastNotice`) e o
`onPush` (= `pushSender.SendToUser`) por injeção.

### `reminders.Store` (repo)

```go
type Reminder struct {
    ID, UserID, TaskID, WorkspaceID string
    RemindAt   time.Time
    Kind       string  // "due" | "manual"
    Status     string  // "pending" | "sent" | "canceled"
    TaskName   string  // join (só na ListByUser — CA-20)
}

// Create insere um reminder pending. Usado pela rota manual e pela criação do due.
func (s *Store) Create(ctx, tx, r Reminder) (Reminder, error)
// CancelDuePending marca canceled o reminder due PENDENTE da task (idempotente; 0..1 linhas).
func (s *Store) CancelDuePending(ctx, tx, taskID string) error
// ListByUser devolve os reminders do user (cross-ws), filtrando por status (default pending). CA-20/21.
func (s *Store) ListByUser(ctx, userID, status string) ([]Reminder, error)
// Cancel marca canceled SE for do user e pending (CA-18/19). Devolve found (pra 204 × 404).
func (s *Store) Cancel(ctx, userID, reminderID string) (found bool, err error)
```

- `Create`/`CancelDuePending` recebem um `tx` (pgx.Tx) pra rodarem na MESMA transação do
  `Create`/`Patch` de task (CA-28). As rotas que criam fora de uma tx de task usam `s.db` direto
  (helper que abre tx própria, ou aceita `pgxConn` — escolher na T-04).
- `ListByUser` faz `JOIN tasks t ON t.id = r.task_id` pra trazer `t.name AS task_name` (CA-20).
  **Sempre** `WHERE r.user_id = $userID` (CA-21).
- `Cancel`: `UPDATE reminders SET status='canceled' WHERE id=$id AND user_id=$user AND
  status='pending' RETURNING id`. Zero linhas + o reminder existe de outro user → `found` via um
  `EXISTS` separado? **Não** — simplificar: `found` = "existe alguma linha com esse id E user_id"
  (independe de status, pra cancelar já-cancelado ser 204 idempotente — CA-18). Reminder de outro
  user / inexistente → `found=false` → 404 (CA-19). (Detalhe na T-04.)

### `reminders.Dispatcher` — uma passada (clock injetável, espelha `guard.Cutter`)

```go
type Dispatcher struct {
    db        *pgxpool.Pool
    now       func() time.Time   // injetável (RNF-03) — default time.Now
    onNotice  func(userID string, payload any)        // = hub.BroadcastNotice
    onPush    func(userID string, payload []byte)      // = pushSender.SendToUser (best-effort)
}
type Option func(*Dispatcher)  // WithClock, WithOnNotice, WithOnPush

// RunOnce varre os reminders pending vencidos (remind_at <= now) e despacha cada um na
// MESMA transação serializada por usuário. Devolve os despachados (pra teste). best-effort.
func (d *Dispatcher) RunOnce(ctx context.Context) ([]Dispatched, error)
```

- **Seleção (CA-04):** `SELECT id, user_id, task_id FROM reminders WHERE status='pending' AND
  remind_at <= $now ORDER BY user_id` (índice parcial). Limiar **inclusivo** (`<=`) — o `due` é
  criado no horário-alvo, não há motivo de estrito como no corte. Fora de transação (leitura
  barata, RNF-01); a verdade é reconferida sob o lock.
- **Despacho por reminder (CA-05/08/09), na transação** (espelha `guard.cutOne`):
  1. `BEGIN` → `pg_advisory_xact_lock(hashtext(user_id))` (CA-08 — corrida com cancel/outro tick).
  2. Re-`SELECT id, kind FROM reminders WHERE id=$id AND user_id=$user AND status='pending' FOR
     UPDATE` — sumiu/não-pending (cancelado ou já consumido por outra passada) → **rollback**, nada
     feito (CA-08). Também carrega o `task_id`/o `name` da task pro payload (join ou 2º select).
  3. `UPDATE reminders SET status='sent' WHERE id=$id AND status='pending'`.
  4. `COMMIT`.
  5. **Pós-commit (CA-06):** `onNotice(user, noticePayload)` + `onPush(user, pushPayload)`. Fora do
     lock e da tx — push/notice lento nunca segura a varredura (RNF-01). O `title` (nome da task) é
     carregado no passo 2 pra montar os payloads.
- **CA-09 isolamento:** o `user_id` do `onNotice`/`onPush` é **o dono do reminder**, não o "alvo da
  task". Mutação removendo o `WHERE user_id`/o `user_id` do payload mata o teste.

### `reminders.Worker` — o ticker (espelha `guard.Worker`)

```go
func (w *Worker) Run(ctx context.Context)  // for { select { <-ticker.C: dispatcher.RunOnce; <-ctx.Done(): return } }
```

- Tick default **60s** (`config.ReminderTick`, env `REMINDER_TICK`, 0/inválido → 60s). 1 goroutine.
  `go worker.Run(ctx)` no boot com o `ctx` do `Run`; `ctx.Done()` para o worker (CA-29). Loga
  partida ("reminder worker iniciado", intervalo) e por despacho ("lembrete despachado", user/task).
- **Não roda RunOnce no boot imediato** (espera o 1º tick — espelha o `guard`, D-02 da 16).

### D-02 — o reminder `due` nasce na ESCRITA da task (não numa varredura)

O `due` é criado/cancelado dentro do `tasks.Service.Create`/`Patch`, na **mesma transação** que
grava a `due_date` (CA-28). Razão: criar no momento da escrita (a) evita uma 2ª varredura periódica
sobre `tasks` (duplicaria o worker e exigiria dedup por task+data a cada tick); (b) o `unique`
parcial `uq_reminders_due_pending` garante 1 `due` pendente por task de graça; (c) a transação
única mantém a coerência (due_date e reminder nunca divergem).

**Como o `tasks.Service` cria o reminder sem depender do pacote api/wiring:** injeta-se um
**hook fino** no `tasks.Service` (espelha o `OnChange` do `timer.Service`):

```go
// no tasks.Service:
type ReminderHook interface {
    // OnDueDateSet é chamado dentro da tx do Create/Patch SOMENTE quando a due_date MUDOU DE VALOR.
    // tx é a MESMA transação; o hook cancela o due pendente antigo e cria o novo (se dueDate != nil).
    // tz vem da requisição.
    OnDueDateSet(ctx context.Context, tx pgx.Tx, taskID, userID string, dueDate *string, tz string) error
}
func (s *Service) SetReminderHook(h ReminderHook)   // wiring no boot; default no-op (RNF-05)
```

- **Gatilho = MUDANÇA DE VALOR, não presença do campo (CA-12.1):** o `tasks.Service` chama o hook no
  `Create` quando `due_date != nil`, e no `Patch` **somente quando o valor da `due_date` divergir**
  do valor que a task já tinha (`oldDueDate != newDueDate`, comparando os dois `*string` por valor —
  null↔null e X↔X são NO-OP; X↔null, null↔X e X↔Y disparam). **NÃO** usar `DueDateSet==true` como
  gatilho: um `Patch` que reescreve a MESMA data X, com o `due` antigo já `sent`, criaria um novo
  `due` pending com `remind_at` no passado e re-dispararia um aviso já dado na próxima `RunOnce`
  (furo MEDIUM da verificação da spec). O service lê o `due_date` antigo da task (SELECT antes do
  UPDATE, ou compara com o valor carregado) e só invoca o hook na divergência. Re-setar a mesma data
  preserva inclusive o `due` `sent` (o hook nem roda).
- O `tasks.Service.Create`/`Patch` já abrem tx pro recálculo de objetivo (D-04 da 12) quando há
  transição. **Atenção (armadilha §7):** o ramo "edição benigna" do `Patch` (sem mudar
  status/objetivo) hoje faz um UPDATE **fora de tx**; se a `due_date` **mudou de valor** nesse ramo,
  é preciso **abrir tx** pra rodar o hook junto (ou rodar o hook depois do UPDATE numa tx própria —
  decidir mantendo atomicidade; a opção mais limpa é: "due_date mudou de valor" passa a ser uma
  condição de `needsTx` junto de status/objetivo). Se a due_date NÃO mudou de valor, o ramo benigno
  segue fora de tx (NO-OP no reminder — CA-12.1).
- O hook resolve o `user_id` (assignee ?? created_by) lendo a task DENTRO da tx (a task já está
  gravada com o assignee corrente). `tz` vem da requisição (a rota passa `tz` ao service — ver §4).
- **Onde computar o 09:00 local:** o hook recebe `dueDate` (`YYYY-MM-DD`) + `tz` (IANA) e monta
  `remind_at = time.Date(y, m, d, 9, 0, 0, 0, loc)` com `loc, _ = time.LoadLocation(tz)` (tz vazio
  → UTC; tz inválido já barrou na validação da rota com 400). Helper testável `dueRemindAt(dueDate,
  tz) (time.Time, error)` no pacote `reminders` (clock-independente, tz-correto — espelha o
  `calendarWindow`).

### D-03 — `due` sem assignee até a fase 18

Antes da fase 18 estar buildada, `assignee` é sempre null → o `due` vai pro `created_by`. A
**resolução** `assignee ?? created_by` é escrita já (CA-14), só não há quem setar assignee pela API
ainda. Os testes do caso "assignee" semeiam o `assignee` por SQL/serviço (registrado). Se a 18 for
buildada em paralelo (worktree), a contenção é só conceitual — esta fase lê `tasks.assignee` que já
existe na coluna (0008).

### D-04 — ordem flip × envio (idempotência)

O flip `pending→sent` **commita ANTES** do `onNotice`/`onPush` (§2.1). Consequência: crash entre
commit e envio = reminder `sent` sem aviso entregue (perda silenciosa). É a escolha **certa**:
o inverso (enviar antes de marcar) reenviaria em loop a cada tick até o flip — pior (spam). O push
é best-effort por natureza (a fase 16 já assume isso). Documentado pra não re-litigar.

## 4. Contratos de API

> Convenção INDEX: envelope de erro `{"error":{"code","message"}}`; 400 `validation_error`; 404
> não-vaza; 403 sem permissão de escrita; 401 sem sessão. Rotas de domínio aninhadas no workspace;
> "os meus" (cross-ws) ficam por-usuário em `/api/v1/...`.

### `POST /api/v1/workspaces/{workspaceId}/tasks/{taskId}/reminders` (manual — CA-16/17/22/23/25)

- **Cadeia:** `RequireAuth → RequireMember`. **NÃO** passa pelo `RequireWriteRole` global (viewer
  precisa do carve-out por-task — CA-23); a permissão é checada no handler/service por-task.
- **Body:** `{"remind_at": "2026-06-25T09:00:00-03:00"}` (RFC3339).
- **Permissão (por-task, espelha o gate da fase 19):**
  - task **visível** ao usuário (visibilidade herdada do projeto — `tasks.Get` já faz; 404 se não
    vê — CA-25).
  - role `owner`/`member` → ok. role `viewer` → 403, **exceto** se `task.assignee == userID` (então
    ok — carve-out CA-23).
- **Sucesso:** **201** + `{id, task_id, kind:"manual", remind_at, status:"pending", ...}`.
- **Erros:** `remind_at` ausente/inválido → 400 `validation_error` (CA-17); task invisível/`taskId`
  malformado → 404 (CA-25); viewer fora do carve-out → 403 (CA-23); sem sessão → 401 (CA-24).

### `DELETE /api/v1/workspaces/{workspaceId}/tasks/{taskId}/reminders/{reminderId}` (cancela — CA-18/19/24)

- **Cadeia:** `RequireAuth → RequireMember`.
- **Regra:** `store.Cancel(userID, reminderId)` — só cancela se for **do user da sessão** (o
  reminder é pessoal). `found=true` → **204**; `found=false` (de outro user / inexistente) → **404**
  (não-vaza — CA-19). Sem sessão → 401 (CA-24). Cancelar já-cancelado/sent → 204 idempotente.
- *(O `taskId` na URL é por consistência/REST; a autoridade é `user_id`+`id`. O handler pode
  ignorar o `taskId` ou validá-lo por coerência — decisão T-04: validar que o reminder pertence à
  task da URL, senão 404, pra a URL não mentir.)*

### `GET /api/v1/reminders?status=pending` (os meus, cross-ws — CA-20/21/24)

- **Cadeia:** `RequireAuth` (por-usuário, FORA do subrouter de workspace — espelha `/api/v1/timer`).
- **Query:** `status` opcional (`pending` default; `sent`/`canceled` válidos; outro → 400).
- **Resposta:** `{"reminders":[{id, task_id, task_name, kind, remind_at, status}]}`, ordenada por
  `remind_at`, **só do user da sessão**, qualquer workspace (CA-20). Lista isolada por user (CA-21).
- Sem sessão → 401 (CA-24).

### Aviso (notice + push) — payloads (CA-07)

- **Notice in-app** (`Hub.BroadcastNotice(userID, payload)`):
  `{"kind":"reminder","task_id":"<uuid>","title":"<nome da task>"}`.
- **Web Push** (`push.Sender.SendToUser(ctx, userID, payload []byte)` — assinatura REAL da 16):
  `payload = json.Marshal(push.Payload{Title:"Lembrete", Body:"<nome da task>", Kind:"reminder"})`
  — a struct `push.Payload` da 16 vira **bytes** ANTES da chamada (a 18 chama do mesmo jeito; ver
  `MILESTONE-2.md §6`).

### Wiring no router

- `WithReminders(Reminders{Store, ...})` — Option nova. O subrouter de task ganha `POST
  /tasks/{taskId}/reminders` + `DELETE .../{reminderId}` (montados no `mountCore` OU num
  `mountReminders` próprio pendurado no subrouter de workspace, atrás de `RequireMember` SEM o
  `RequireWriteRole` — decidir mantendo o carve-out por-task). `GET /api/v1/reminders` no `/api/v1`
  por-usuário (espelha o mount do timer/push).

## 5. Frontend

### `noticeStore.ts` + `NoticeBanner.tsx` (CA-27)

- `pushNotice` hoje só reconhece `kind === 'timer_auto_cut'`. **Estender** pra também aceitar
  `kind === 'reminder'` (guarda o notice). **Não** remover o ramo do corte.
- `NoticeBanner` escolhe o texto por `kind`: `timer_auto_cut` → "Seu timer foi pausado…"
  (intocado); `reminder` → "Lembrete: <title>" (do payload). Ícone por kind (clock × bell).
- `realtime.ts::onNotice` **não muda** (já encaminha qualquer `notice` pro `pushNotice`).

### `api.ts` + `lib/queries/reminders.ts` (CA-16/18/20)

- `listReminders(status?)`, `createReminder(workspaceId, taskId, {remind_at})`,
  `cancelReminder(workspaceId, taskId, reminderId)`. TanStack Query: a lista invalida nas mutações.

### `ReminderButton.tsx` + lista (CA-16/18/30)

- Ação **"Lembrar"** no detalhe/modal de task (fase 12 board): abre um picker de "quando" (presets:
  "em 1h", "amanhã 9h", "no prazo" se houver due_date, ou input de data/hora) → `createReminder`.
- **Lista "Meus lembretes"**: bloco no `/ajustes` (grupo, espelha o de Notificações da 16) OU no
  dashboard — lista os pendentes (`task_name` + `remind_at`) com botão cancelar. Decisão de UX na
  T-08 (default: grupo nos Ajustes, mais barato e consistente). **Não** mexer no layout de
  TimerBar/SideNav (responsivo é fase 22).

## 6. Libs / deps

- **Zero deps novas** (RNF-06): `webpush-go` (16), zustand (notice store), TanStack Query,
  Push/Notification nativas — tudo já no projeto. O `reminders` usa só `pgx` + stdlib.

## 7. Pontos de integração com o baseline (e o que é ADITIVO vs intocado)

| Recurso baseline | Uso na fase 20 |
|---|---|
| `guard.Worker`/`guard.Cutter` (16) | **MODELO** copiado (não importado/editado) — INTOCADO (CA-26) |
| `Hub.BroadcastNotice` (16) | **consumido** sem mudar assinatura (kind novo no payload) — INTOCADO |
| `push.Sender.SendToUser` (16) | **consumido** best-effort (Payload kind="reminder") — INTOCADO |
| `realtime.ts::onNotice` + `noticeStore.pushNotice` (16) | `pushNotice` **estendido** (+kind reminder); `onNotice` intocado |
| `NoticeBanner` (16) | **estendido** (texto por kind) |
| `tasks.Service.Create`/`Patch` (02/12) | **estendido** (hook do `due` na tx) — contrato REST intocado (CA-28) |
| `tasks.Get` (visibilidade por projeto) | reusado pro gate de permissão do POST manual (404 não-vaza) |
| advisory lock `hashtext(user_id)` (03/16) | mesma família no `Dispatcher` (CA-08) |
| `config.go` (`GUARD_TICK`) | espelhado em `REMINDER_TICK` |
| `cmd/server/main.go` | liga 1 worker + WithReminders + injeta o ReminderHook no tasks.Service |
| `e2e/helpers.ts::seedForgottenTimer` | modelo pro `seedReminder` (SQL) |

## 8. Dependências da fase + paralelização (PRD §9)

- **Depende de:** **fase 16** (worker/push/notice infra — modelo + `Hub.BroadcastNotice`/
  `push.Sender` consumidos) · **fase 02** (`tasks` + `due_date` + visibilidade). **Fase 18**
  (assignee) é dependência **branda**: o `due` resolve `assignee ?? created_by` lendo a coluna
  `tasks.assignee` (já existe em 0008); sem a 18, o assignee é sempre null e cai no `created_by`
  (D-03). O carve-out viewer-assignee (CA-23) lê `task.assignee` — funciona com a coluna existente.
- **PARALELIZÁVEL: SIM** (worktree). O pacote `reminders` é isolado; a migração `0020` é dona fixa
  (sem colisão com 0017–0019); o único toque em arquivo compartilhado é (a) ligar **um** worker +
  `WithReminders` no `cmd/server/main.go` e (b) o hook no `tasks.Service`/`tasks_handlers.go` —
  **contende com a fase 18** (que também mexe em `tasks/service.go`/`tasks_handlers.go`) e
  potencialmente com a 19 (subrouter de task). Mitigação: o hook é aditivo (1 chamada injetada nos
  ramos de due_date); coordenar o merge de `tasks/service.go` se 18 e 20 rodarem juntas. O front
  contende com a 16 em `noticeStore.ts`/`NoticeBanner.tsx` (estende, não reescreve).

## 9. Armadilhas conhecidas (vetadas)

- **NÃO** importar nem editar `internal/guard` (CA-26/RNF-05) — copiar o formato, pacote separado.
- **NÃO** marcar `sent` DEPOIS de enviar (D-04) — reenviaria em loop a cada tick; o flip commita
  ANTES do envio.
- **NÃO** despachar sem o advisory lock + `FOR UPDATE` re-check (CA-08) — duas passadas/um cancel
  concorrente reenviariam ou cancelariam-e-enviariam o mesmo reminder.
- **NÃO** criar o `due` numa varredura periódica de tasks (D-02) — nasce na escrita da `due_date`;
  o `unique` parcial garante 1 pendente por task.
- **NÃO** disparar o hook do `due` por `DueDateSet==true` (presença do campo) — o gatilho é
  **due_date MUDOU DE VALOR** (`oldDueDate != newDueDate`; CA-12.1/D-02). Re-setar a MESMA data com
  o `due` antigo já `sent` re-armaria um aviso já dado (novo `due` pending no passado → `RunOnce`
  re-dispara). Re-setar a mesma data = NO-OP no reminder (hook nem roda; preserva o `sent`).
- **NÃO** esquecer de rodar o hook do `due` no **ramo de edição benigna** do `tasks.Patch` (§4 da
  spec / D-02) **quando a due_date mudou de valor**: hoje esse ramo faz UPDATE fora de tx; se a
  `due_date` mudou de valor, precisa de tx pra o hook (tornar "due_date mudou de valor" parte da
  condição `needsTx`). Senão mudar SÓ a due_date (sem mexer status/objetivo) não geraria reminder —
  bug silencioso.
- **NÃO** vazar reminder de outro usuário na lista nem no cancel (CA-19/21) — `WHERE user_id =
  sessão` obrigatório; cancelar de outro = 404.
- **NÃO** aplicar o `RequireWriteRole` global no POST manual (CA-23) — barraria o viewer-assignee;
  a permissão é por-task no service.
- **NÃO** computar o 09:00 do `due` em UTC e depois "ajustar" — usar `time.Date(...,9,0,...,loc)`
  com a `loc` do tz da requisição (tz-correto, igual ao `calendarWindow`); tz inválido = 400 na
  rota antes de chegar no service.
- **NÃO** rodar a goroutine do worker sem amarrar ao `ctx` do servidor (CA-29) — goroutine zumbi.
- **NÃO** reatribuir o `due` ao mudar o assignee depois (fora de escopo) — o `due` guarda o
  responsável do instante da criação.
- **NÃO** deixar o push (rede/N subs) bloquear o ticker (RNF-01) — push é pós-commit, fora do lock.

## 10. Ordem de execução (ver tasks.md)

Backend primeiro (migração → store TDD → dispatcher TDD → worker → hook do `due` no tasks.Service →
rotas → config/wiring), depois o frontend (noticeStore/banner por kind → api/queries → ReminderButton
+ lista), depois E2E, verificação adversarial e gate. Commit por tarefa. `make check-full` é o gate
final.
