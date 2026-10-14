# spec.md — Fase 20 · Lembretes (due date + manual)

> Fonte de verdade da fase. Resolve a dor **P4** do `MILESTONE-2.md §2/§4`: "Esqueço prazos e não
> sou lembrado". Entrega **lembretes** de dois tipos — `due` (prazo da task chegando) e `manual`
> ("me lembra dessa task em X") — com entrega **in-app (notice per-usuário) + Web Push**, reusando
> a infra de notificação da fase 16 (`Hub.BroadcastNotice` + `push.Sender`). É a **2ª infra de
> background** do projeto: um worker próprio (`internal/reminders`) que **espelha** o
> `guard.Worker`/`guard.Cutter` da fase 16, mas **NÃO toca o pacote `guard`**.
>
> **Decisões travadas com o Diogo (HITL, `MILESTONE-2.md §7.5`):**
> 1. **Lembretes = `due_date` chegando + lembrete `manual`.** Os dois entregues por in-app + Web
>    Push. **Sem email / provider externo** (herdado da fase 16).
> 2. **Recorrência do lembrete FORA** — o lembrete em si não recorre (a task pode recorrer via
>    `recurrence`, mas isso não gera lembretes recorrentes nesta fase).
> 3. **Worker PRÓPRIO** (`internal/reminders`), espelhando o `guard.Worker` mas isolado — o pacote
>    `guard` da fase 16 fica **intocado** (CA de não-regressão).
> 4. **Responsável lembrado do `due`** = `assignee ?? created_by` (fase 18 liga o `assignee`; sem
>    assignee cai no criador). Migração `0020_reminders`.

---

## 1. Objetivo

Quando uma task tem prazo (`due_date`) chegando, ou quando o usuário marca "me lembra dessa task em
X", o sistema **avisa a pessoa certa na hora certa** — banner in-app se ela estiver com o app
aberto, e Web Push se ela tiver autorizado — pra prazos não passarem batido. Pra isso:

1. **Migração `0020_reminders`** — tabela `reminders` (quem é lembrado, qual task, quando,
   tipo `due|manual`, status `pending|sent|canceled`) com **índice parcial** por `(remind_at) WHERE
   status='pending'` pra a varredura do worker ser barata (RNF-01).
2. **Worker próprio** (`internal/reminders`, **não** o `guard`): um ticker leve (clock injetável,
   espelha o `guard.Worker`) varre `reminders` com `status='pending' AND remind_at <= now`, dispara
   o aviso (in-app + push) e marca `sent` — **idempotente** (uma 2ª passada não reenvia).
3. **Reminder `due`**: nasce **ao setar/mudar a `due_date`** de uma task (não a cada tick do
   worker — §2.2), pra o responsável (`assignee ?? created_by`). 1 reminder `due` por task+due_date
   (não duplica). Limpar/mudar a `due_date` cancela o `due` pendente antigo.
4. **Reminder `manual`**: `POST` cria ("me lembra em `<remind_at>`"); o usuário cancela (`DELETE` →
   `status=canceled`).

A fase é **aditiva**: o `guard`/timer/sync/foco e o contrato REST das fases anteriores seguem
intocados (RNF-05). Só liga **um** worker novo no `cmd/server/main.go` (paralelizável — `§9`).

## 2. Definições precisas

### 2.1 A varredura (worker)

A passada do worker (`reminders.Dispatcher.RunOnce`) seleciona **exatamente** os reminders
`status='pending' AND remind_at <= now` (limiar **inclusivo** — exatamente no horário JÁ dispara;
o `due` cria com horário-alvo, então atrasar 1s não é problema; ver CA-10). Pra cada um, sob
transação serializada (advisory lock por `user_id`, igual ao `guard`):

1. Re-confere `SELECT 1 FROM reminders WHERE id=$id AND status='pending' FOR UPDATE` — sumiu/já
   não-pending (cancelado ou já enviado por outra passada concorrente) → **rollback, nada feito**
   (idempotência/corrida — CA-08).
2. `UPDATE reminders SET status='sent' WHERE id=$id AND status='pending'` (o flip é a fonte da
   verdade da idempotência — CA-08/CA-12).
3. `COMMIT`.
4. **Pós-commit** (fora do lock): dispara o aviso — `Hub.BroadcastNotice(user_id, {...})` (in-app)
   + `push.Sender.SendToUser(user_id, ...)` (Web Push, best-effort). **Ordem é load-bearing:** o
   flip `pending→sent` **commita ANTES** do envio. Se o processo morrer entre o commit e o envio, o
   reminder fica `sent` sem ter avisado (perda silenciosa aceita: melhor não-avisar que avisar em
   loop — decisão D-04 do plan). 2 passadas concorrentes não reenviam (a 2ª não acha `pending`).

### 2.2 O reminder `due` (regra de criação)

- **Onde nasce:** **ao setar ou mudar `tasks.due_date`** (no `Create`/`Patch` de task — fase 02),
  NÃO numa varredura de tasks pelo worker. Razão (D-02 do plan): criar no momento da escrita evita
  uma 2ª varredura periódica (sobre `tasks`) que duplicaria a do worker e exigiria dedup por
  task+data a cada tick; com criação na escrita, o índice parcial + o `unique` por
  `(task_id, kind='due')` garantem **1 reminder `due` pendente por task** trivialmente.
- **`remind_at` do `due`:** **09:00 do dia da `due_date`, no fuso passado pela requisição** (`tz`
  IANA, igual ao `entries.ListDay`/calendar — §2.3). Ex.: `due_date=2026-06-25`, `tz=America/
  Sao_Paulo` → `remind_at = 2026-06-25T09:00:00-03:00` (= `12:00Z`). Sem `tz` → UTC (default
  herdado das outras rotas).
- **Responsável (quem é lembrado):** `assignee ?? created_by` da task **no instante da criação do
  reminder** (a fase 18 liga o `assignee`; antes da 18 é sempre `created_by`). Se o assignee mudar
  depois, o reminder `due` existente NÃO é reatribuído nesta fase (fora de escopo — fase futura;
  registrado).
- **Gatilho = a `due_date` MUDOU DE VALOR** (não "foi setada"): o hook compara o `due_date` **antigo
  da task** (antes da escrita) com o **novo** e só age quando o valor diverge. Setar/re-escrever a
  **MESMA** `due_date` (valor idêntico ao que já estava) é **NO-OP no reminder** — NÃO cancela o
  `due` pendente, NÃO recria nada (preserva inclusive um `due` já `sent` — não re-arma um aviso já
  dado). Razão (CA-12.1): um `Patch` benigno que reescreve a mesma data X, com o `due` antigo já
  `sent`, NÃO pode criar um novo `due` pending com `remind_at` no passado — isso re-disparia na
  próxima `RunOnce` um aviso já entregue.
- **Idempotência (não duplica):** quando a `due_date` muda de valor, o sistema **cancela** qualquer
  reminder `due` **pendente** pré-existente da task e cria **um** novo (upsert lógico). Constraint
  física: **índice unique parcial** `(task_id) WHERE kind='due' AND status='pending'` — no máximo 1
  `due` pendente por task a qualquer instante.
- **Limpar a `due_date`** (`due_date → null`, vindo de um valor não-nulo = mudança de valor):
  cancela o `due` pendente da task; NÃO cria novo.
- **Mudar a `due_date`** (data nova ≠ antiga): cancela o `due` pendente antigo, cria um novo pro
  novo horário/responsável.
- **`due_date` no passado** (data já vencida ao setar): o reminder nasce `pending` com `remind_at`
  no passado → a próxima passada do worker dispara imediatamente (avisa "tem prazo vencido"). É o
  comportamento desejado (não silencia prazo já passado).
- **Não recorre:** mesmo que a task tenha `recurrence`, o reminder `due` é **um só** pra aquela
  `due_date`. A `recurrence` é texto opaco nesta fase (não materializa próximas datas).

### 2.3 Fuso (tz)

Não há coluna de tz por usuário no banco (`users` não tem — verificado). O fuso vem da
**requisição** (query `tz` IANA), exatamente como `entries.ListDay` e as rotas de calendar/reports.
A criação do reminder `due` (no `Create`/`Patch` de task) usa o `tz` **da requisição que setou a
`due_date`**. O worker NÃO precisa de tz: ele compara `remind_at` (timestamptz absoluto) com `now`.

## 3. User stories

- **US-01** — Como usuário, quando eu defino um **prazo** numa task, quero ser **lembrado no dia**
  (de manhã), pra eu não deixar o prazo passar batido.
- **US-02** — Como usuário, quero **marcar manualmente** "me lembra dessa task em 2h / amanhã às
  9h", pra eu não confiar só na memória.
- **US-03** — Como usuário, quero **cancelar** um lembrete que criei (ou que não preciso mais), pra
  não receber aviso à toa.
- **US-04** — Como usuário, quero o lembrete chegar por **banner no app** (se aberto) **e por push**
  (se autorizei), igual ao aviso de corte da fase 16, pra ele me alcançar mesmo de longe.
- **US-05** — Como usuário, quero ver a **lista dos meus lembretes pendentes** (cross-workspace),
  pra saber o que vem e cancelar o que não quero.

## 4. Critérios de aceite (EARS — cada um vira teste)

### Migração & modelo de dados

- **CA-01** (migração) — a fase 20 É dona da migração **`0020_reminders`** (próximo número livre
  após `0016_timer_guard`; `0017/0018/0019` são das fases 17/18/19 da M2 — a T-01 fixa o número
  lendo `backend/internal/db/migrations/`). Cria a tabela `reminders` com: `id uuid PK default
  gen_random_uuid()`; `user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE` (quem é
  lembrado); `task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE`; `workspace_id uuid NOT
  NULL` (denormalizado, filtro tenant — espelha `tasks`/`task_comments`); `remind_at timestamptz
  NOT NULL`; `kind text NOT NULL CHECK (kind IN ('due','manual'))`; `status text NOT NULL DEFAULT
  'pending' CHECK (status IN ('pending','sent','canceled'))`; `created_at timestamptz NOT NULL
  DEFAULT now()`.
- **CA-02** (índice parcial de varredura) — a migração cria o **índice parcial**
  `idx_reminders_pending ON reminders (remind_at) WHERE status='pending'` — a varredura do worker
  (`WHERE status='pending' AND remind_at <= now`) usa só essa fatia (RNF-01). (Um teste assere o
  índice presente e parcial.)
- **CA-03** (unique parcial do `due`) — a migração cria o **índice unique parcial**
  `uq_reminders_due_pending ON reminders (task_id) WHERE kind='due' AND status='pending'` — no
  máximo **1** reminder `due` pendente por task (backstop da idempotência §2.2). Inserir um 2º
  `due` pendente pra a mesma task viola o unique. (Teste: 2 inserts `due` pendentes da mesma task →
  o 2º falha.) `ON DELETE CASCADE` nas FKs: apagar a task/o user limpa os reminders órfãos.

### Worker de despacho (backend — clock injetável, espelha o `guard`)

- **CA-04** (seleção) — uma passada de `reminders.Dispatcher.RunOnce(ctx)` DEVE selecionar
  **exatamente** os reminders `status='pending' AND remind_at <= now` (limiar **inclusivo** —
  `remind_at == now` dispara). Um reminder com `remind_at` no futuro NÃO é selecionado; um `sent`
  ou `canceled` NUNCA entra na varredura (filtro `status='pending'`).
- **CA-05** (flip `pending→sent`) — ao despachar um reminder vencido, o worker DEVE marcá-lo
  **`status='sent'`** na transação (não apaga a linha — fica o histórico). `remind_at`, `kind`,
  `task_id`, `user_id` ficam **intactos**.
- **CA-06** (aviso só no despacho real) — o worker SÓ DEVE emitir aviso (notice + push) pros
  reminders **efetivamente despachados nesta passada** (transação commitada). Passada sem nenhum
  reminder vencido = **zero** notice, **zero** push. (Spy nos hooks `onNotice`/`onPush`.)
- **CA-07** (payload do aviso) — o aviso do reminder DEVE identificar **kind="reminder"** e levar
  o `task_id` e o `title` (nome da task) — `Hub.BroadcastNotice(user_id, {"kind":"reminder",
  "task_id":...,"title":...})`. (Distingue do `kind:"timer_auto_cut"` da fase 16; o front roteia
  por `kind`.)
- **CA-08** (idempotência + concorrência) — duas passadas seguidas sobre o mesmo estado DEVEM
  produzir o **mesmo resultado**: a 2ª passada não acha mais o reminder `pending` (já `sent`) e
  **não reenvia**. O despacho roda sob `pg_advisory_xact_lock(hashtext(user_id))` + re-`SELECT ...
  WHERE id=$id AND status='pending' FOR UPDATE` — se duas passadas concorrentes pegarem o mesmo
  reminder, **só uma** o marca `sent` e dispara o aviso (a outra acha `pending` já consumido →
  aborta). (Teste: 2× `RunOnce` → exatamente 1 notice/push pro reminder.)
- **CA-09** (isolamento) — o worker DEVE operar e despachar exclusivamente pelo `user_id` dono do
  reminder; NUNCA emite o aviso de um reminder de A pro user B. (Constituição §8.4 — uma mutação
  removendo o `WHERE user_id`/o `user_id` do payload do notice deve matar um teste; 2 usuários, só
  o vencido de cada um dispara pro alvo certo.)
- **CA-10** (best-effort push) — ao despachar (CA-06), o `push.Sender.SendToUser` é chamado para o
  dono; uma subscription que falha NÃO impede as outras nem o notice in-app (que já saiu) nem o
  flip `sent` (que já commitou — push é pós-commit). Sem VAPID configurado → push é **no-op** e o
  notice in-app segue (reusa o `Sender` da fase 16, RNF-04). (Reusa o teste best-effort do `Sender`
  — esta fase prova o *wiring*: o `onPush` recebe `user_id`+payload corretos por reminder.)

### Reminder `due` (nasce na escrita da task — clock controlado)

- **CA-11** (due nasce ao setar due_date) — QUANDO uma task ganha `due_date` (no `Create` com
  due_date, ou no `Patch` que **muda** a due_date de valor), o sistema DEVE criar **um** reminder
  `kind='due'`, `status='pending'`, `user_id = assignee ?? created_by` da task, `remind_at = 09:00
  do dia da due_date no fuso da requisição` (§2.2). (Teste: cria task com `due_date` + `tz` → existe
  1 reminder `due` com o `remind_at` esperado pro `created_by`.)
- **CA-12** (1 due por task+due_date, não duplica) — `Patch` que **muda** a `due_date` para uma data
  **diferente** DEVE **cancelar** o reminder `due` pendente antigo da task e criar **um** novo;
  nunca acumula dois `due` pendentes (backstop CA-03). (Teste: 2 patches com datas diferentes → 1
  reminder `due` pendente, os anteriores `canceled`.)
- **CA-12.1** (re-setar a MESMA due_date = NO-OP, preserva o sent) — `Patch` que reescreve a
  `due_date` para o **MESMO valor** que a task já tinha DEVE ser **NO-OP no reminder**: NÃO cancela
  o `due` pendente, NÃO cria um novo. Em particular, se o `due` antigo dessa task já está `sent`, o
  `Patch` que reescreve a mesma data **NÃO** cria um novo `due` pending (que teria `remind_at` no
  passado e re-dispararia um aviso já dado na próxima `RunOnce`). O gatilho da criação é a `due_date`
  **mudar de valor** (antigo ≠ novo), não a presença do campo no payload. (Teste: task com `due`
  já `sent` → `Patch` reescrevendo a MESMA due_date → nenhum reminder novo, o `sent` permanece
  intacto e nenhuma `RunOnce` re-dispara.)
- **CA-13** (limpar due_date cancela o due) — `Patch` que **muda** a `due_date` de um valor não-nulo
  para `null` DEVE cancelar o reminder `due` **pendente** da task (vira `status='canceled'`) e NÃO
  criar novo. Um `due` já `sent` NÃO é tocado (histórico). Patch com `due_date → null` numa task que
  já estava sem due_date (null → null, sem mudança de valor) é NO-OP (CA-12.1). (Teste: task com due
  → patch null → o `due` pendente vira canceled, nenhum novo.)
- **CA-14** (responsável = assignee ?? created_by) — o `user_id` do reminder `due` é o `assignee`
  da task se houver, senão o `created_by`. (Teste com assignee setado [fase 18] → reminder pro
  assignee; sem assignee → pro created_by. Antes da fase 18 estar buildada, o caso do assignee usa
  um assignee semeado por SQL/serviço — registrado.)
- **CA-15** (due_date no passado dispara já) — setar uma `due_date` cujo `remind_at` (09:00 local)
  já passou cria o reminder `pending` com `remind_at` no passado; a próxima `RunOnce` o despacha
  imediatamente (CA-04). Não há supressão de prazo vencido.

### Reminder `manual` (API)

- **CA-16** (criar manual) — `POST /api/v1/workspaces/{workspaceId}/tasks/{taskId}/reminders` com
  `{"remind_at": "<RFC3339>"}` autenticado, numa task **visível** ao usuário (visibilidade herdada
  do projeto — §CA-22 da fase 02), DEVE criar um reminder `kind='manual'`, `status='pending'`,
  `user_id = usuário da sessão` (o lembrete é **pra si** — §8 da matriz), `task_id` da URL,
  `workspace_id` denorm, `remind_at` do corpo. Responde **201** com o reminder criado.
- **CA-17** (validação do remind_at) — `remind_at` ausente / não-RFC3339 / vazio → **400
  `validation_error`**. Um `remind_at` no passado É aceito (CA-15 vale igual: dispara na próxima
  passada — o usuário pode querer "me lembra já"). (Não há limite superior nesta fase.)
- **CA-18** (cancelar manual) — `DELETE /api/v1/.../reminders/{reminderId}` do **dono** (o
  `user_id` do reminder == sessão) DEVE marcar `status='canceled'` (soft — não apaga) e responder
  **204**. Um reminder `canceled` ou já `sent` deletado de novo é idempotente (204; o
  `status='pending'` no UPDATE simplesmente não casa — o registro fica como está). Cancelar **NÃO**
  emite aviso nem toca a task.
- **CA-19** (cancelar só o dono — 404 não-vaza) — `DELETE` de um reminder de **outro usuário** (ou
  inexistente) responde **404** (`not_found`) — não vaza existência (a query filtra por
  `user_id = sessão`). Um member/owner do mesmo workspace **não** pode cancelar o reminder de outro
  (o reminder é pessoal — `user_id`).

### Visões dos MEUS lembretes (API)

- **CA-20** (listar os meus, cross-workspace) — `GET /api/v1/reminders` autenticado DEVE devolver
  **os reminders do usuário da sessão** (`user_id = sessão`), **cross-workspace** (qualquer
  workspace que ele participe), `status='pending'` por default (com `?status=` opcional pra ver
  `sent`/`canceled`), ordenados por `remind_at`. Cada item leva `task_id`, `kind`, `remind_at`,
  `status` e o **nome da task** (join). É **por usuário** (rota fora do subrouter de workspace —
  espelha o `/api/v1/timer`), porque "meus lembretes" cruza workspaces.
- **CA-21** (lista isolada por user) — a lista NUNCA inclui reminder de outro usuário, mesmo de uma
  task que ambos enxergam. (Mutação removendo `WHERE r.user_id = $session` deve matar um teste; 2
  usuários, cada `GET /reminders` só vê os seus.)

### Permissões (matriz `MILESTONE-2.md §8`)

- **CA-22** (criar lembrete pra si — member/owner) — owner e member podem criar reminder `manual`
  numa task visível ao workspace (200/201). O lembrete é **sempre pra si** (`user_id = sessão`);
  não há "lembrar outra pessoa" nesta fase.
- **CA-23** (viewer só na task dele) — um **viewer** recebe **403** ao criar reminder numa task
  comum (carve-out de escrita §8), **EXCETO** numa task onde ele é o **`assignee`** — aí pode criar
  o `manual` (agir no que pediram; carve-out viewer-assignee §8). (Teste: viewer + task qualquer →
  403; viewer + task onde é assignee → 201. Antes da fase 18, o assignee é semeado por SQL.)
- **CA-24** (sem sessão → 401) — `POST .../reminders`, `DELETE .../reminders/{id}` e `GET
  /api/v1/reminders` sem sessão respondem **401** canônico (`{"error":{"code":"unauthorized"}}`).
- **CA-25** (task invisível/alheia → 404) — `POST .../reminders` numa task que o usuário **não
  enxerga** (projeto private de outro, ou task de outro workspace) responde **404** (não-vaza —
  mesma cadeia de visibilidade da fase 02/19). `taskId` malformado → 404.

### Não-regressão

- **CA-26** (guard intocado) — o pacote `internal/guard` (Cutter/Worker da fase 16), o
  `time_entries.auto_cut`, o aviso de corte e suas suítes seguem **byte-compatíveis**: a fase 20
  **NÃO edita** nenhum arquivo de `guard`. O worker de reminders é um pacote separado. (As suítes
  da fase 16 rodam sem alteração de asserção.)
- **CA-27** (notice/push reusados, não alterados) — `Hub.BroadcastNotice` e `push.Sender` são
  **consumidos** sem mudança de assinatura (o reminder usa um `kind` novo no payload do notice e um
  `Payload` de push novo, mas a infra é a da fase 16). O ramo `type:"timer"` e o anti-stale do
  `timerStore` **não mudam**; o `realtime.ts::onNotice` já roteia qualquer `notice` pro
  `pushNotice` — esta fase só **estende** `pushNotice`/`noticeStore` pra reconhecer `kind:"reminder"`
  além de `timer_auto_cut`. As suítes das fases 04/16 seguem verdes.
- **CA-28** (tasks REST aditivo) — `tasks.Service.Create`/`Patch` ganham a criação/cancelamento do
  reminder `due` (CA-11/12/13) **sem mudar** o contrato/resposta das rotas de task; quem não passa
  `due_date` não dispara nada. As suítes da fase 02/12/18 seguem verdes (o `due` é um efeito
  colateral aditivo na MESMA transação do `Create`/`Patch`).
- **CA-29** (boot aditivo) — `cmd/server/main.go` liga **um** `reminders.Dispatcher` +
  `reminders.Worker` (`go worker.Run(ctx)`, mesmo `ctx` do boot, para no shutdown — espelha o
  guard) + monta as rotas (`WithReminders`). O boot segue subindo sem nenhuma env nova obrigatória.

### E2E

- **CA-30** — Logado (stub): o usuário cria um lembrete **manual** numa task com `remind_at` no
  passado (via UI ou semeado), e com o worker tickando em intervalo curto (env de E2E) + o app
  aberto, vê o **banner de lembrete** aparecer (`data-testid="notice-banner"` com o texto/kind de
  reminder). Em seguida, **cancela** um lembrete pendente na lista dos meus lembretes e ele some da
  lista. (O Web Push real NÃO é exercitado headless — tem teste de unidade no backend.)

## 5. Requisitos não-funcionais

- **RNF-01 (leveza do worker)** — a passada é **uma query indexada** (`WHERE status='pending' AND
  remind_at <= now`, sobre `idx_reminders_pending` parcial) + o despacho por reminder. Tick default
  **60s** (env `REMINDER_TICK`, 0/inválido → 60s — espelha `GUARD_TICK`). 1 goroutine, encerra no
  shutdown via `context`. Zero dependência pesada nova no caminho do worker (reusa `webpush-go` da
  16).
- **RNF-02 (segurança/concorrência)** — despacho na **mesma família de advisory lock por usuário**
  do `timer.Service`/`guard` (`pg_advisory_xact_lock(hashtext(user_id))`) + `FOR UPDATE` re-check
  (CA-08). Idempotente. O worker NUNCA reenvia um reminder já `sent`.
- **RNF-03 (clock injetável)** — o `Dispatcher` tem **clock injetável** (`WithClock`, igual ao
  `guard.Cutter`); os testes setam `now` pra provar o limiar (CA-04) e o despacho sem esperar
  relógio real. A criação do `due` (no service de tasks) recebe o `now`/`tz` da requisição
  (injetável no teste).
- **RNF-04 (degradação do push)** — sem VAPID, o push é no-op e o aviso in-app segue (reusa o
  `Sender` da 16, CA-10). O boot não exige nenhuma chave nova.
- **RNF-05 (não-regressão)** — fases 16 (`guard`/push/notice), 04 (WS/Hub), 02/12/18 (tasks)
  **intocadas no contrato** (CA-26/27/28). O `guard` NÃO é importado nem editado.
- **RNF-06 (zero deps novas)** — **nenhuma** dependência Go ou JS nova: `webpush-go` (16), zustand
  (notice store), Push/Notification nativas — tudo já existe.

## 6. Pré-condições externas

- **Nenhuma nova.** As chaves VAPID já são da fase 16 (config de deploy, `PENDENCIAS-DIOGO.md`); sem
  elas o push fica desligado e só o aviso in-app funciona (RNF-04). Os testes geram chaves no
  harness (reuso da fase 16) ou injetam o `onPush`.

## 7. Fora de escopo (fronteiras explícitas)

- **Recorrência do lembrete** — o lembrete não recorre; a task pode recorrer (`recurrence`), mas
  isso NÃO materializa próximos lembretes nesta fase (decisão `§7.5`).
- **Reatribuir o `due` quando o assignee muda** depois de criado — o reminder `due` guarda o
  responsável do **instante da criação** (CA-14); mudar o assignee depois não move o reminder
  existente (fase futura).
- **Email / provider externo** — segue in-app + Web Push (herdado da 16).
- **Lembrar OUTRA pessoa** ("me lembra o Cadu") — nesta fase o `manual` é sempre pra si
  (`user_id = sessão`). Lembrete cruzado = future.
- **Limite superior de `remind_at` / agendar muito longe** — sem limite nesta fase.
- **Snooze / editar `remind_at` de um reminder existente** — fora; o usuário cancela e cria outro.
- **Lembrete a partir de "tasks atrasadas" varrendo o banco** — o `due` nasce na escrita da
  `due_date` (§2.2), não numa varredura de tasks; varredura de atrasados é future.
- **Histórico/tela de lembretes enviados** além da lista filtrável por `?status=` — a tela mínima é
  a dos pendentes + cancelar.
