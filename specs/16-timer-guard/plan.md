# plan.md — Fase 16 · Anti-timer-esquecido

> Como construir a spec. Decisões de design (D-xx) + arquitetura + ordem de tarefas.
> Reusa o `timer.Service` (advisory lock + `onChange`), o Hub WebSocket (fase 04), e o
> service worker do PWA (fase 00). Adiciona: migração `0016`, worker de background, endpoints
> de push, SW customizado.

---

## 1. Arquitetura geral

```
Backend:
  db/migrations/0016_timer_guard.sql        ← time_entries.auto_cut + tabela push_subscriptions
  internal/guard/                  (NOVO pacote)
     cutter.go                              ← lógica do corte: 1 passada (clock injetável)
     cutter_test.go                         ← TDD do corte (limiar, marco, idempotência, lock)
     worker.go                              ← ticker + goroutine (Start(ctx) → para no shutdown)
  internal/push/                   (NOVO pacote)
     subscriptions.go                       ← repo: Upsert/ListByUser/Delete (push_subscriptions)
     sender.go                              ← envia Web Push (VAPID) best-effort + limpa 404/410
     sender_test.go / subscriptions_test.go ← TDD (subscribe, isolamento, limpeza de morto)
  internal/api/push_routes.go      (NOVO)   ← POST /push/subscribe + GET /push/key (por-usuário)
     push_routes_test.go                    ← subscribe 200/400/401; key
  internal/realtime/hub.go (existe)         ← + BroadcastNotice (envelope type:"notice") — aditivo
  internal/config/config.go (existe)        ← + VAPID_* + GUARD_TICK (env)
  cmd/server/main.go (existe)               ← liga o guard.Worker + push sender + wiring das rotas

Frontend:
  lib/realtime.ts (existe)                  ← + onNotice (trata type:"notice"; type:"timer" intacto)
  lib/push.ts                     (NOVO)    ← subscribe do browser (Notification.requestPermission +
                                              PushManager.subscribe + POST /push/subscribe)
  lib/api.ts (existe)                       ← + postPushSubscribe / getPushKey
  components/NoticeBanner.tsx     (NOVO)    ← banner dispensável no shell, alimentado pelo onNotice
  components/AppShell.tsx (existe)          ← monta o NoticeBanner + liga onNotice no attachTimerSync
  lib/timerStore.ts (existe)                ← attachTimerSync passa onNotice ao connectTimerSocket
  src/sw.ts                       (NOVO)    ← service worker customizado (precacheManifest + push +
                                              notificationclick) — injectManifest
  vite.config.ts (existe)                   ← VitePWA strategy 'injectManifest' (srcDir/filename)
```

## 2. Migração — `0016_timer_guard.sql` (dona: fase 16)

> **Número:** as fases anteriores da M1 são donas de `0013` (12, objetivos), `0014` (14, report_settings) e `0015` (15, pomodoro_cycles). A T-01 fixa o
> número lendo `backend/internal/db/migrations/` (próximo livre). O nome lógico é `timer_guard`;
> o número segue a sequência real presente. Abaixo o conteúdo (assumindo 0016):

```sql
-- 0016_timer_guard — fase 16 (plan §2): marca de corte automático na entry
-- + subscriptions de Web Push por usuário. Sem cache de eventos novo.

ALTER TABLE time_entries
  ADD COLUMN auto_cut boolean NOT NULL DEFAULT false;

CREATE TABLE push_subscriptions (
  endpoint    text PRIMARY KEY,            -- a URL do push service é única por inscrição
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  p256dh      text NOT NULL,
  auth        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_push_subs_user ON push_subscriptions (user_id);
```

- PK por `endpoint` (CA-13 upsert por endpoint: re-inscrever o mesmo navegador substitui as
  chaves). `ON DELETE CASCADE` no user limpa subs órfãs se um user for removido.
- INDEX.md: registrar **0016 dona da fase 16** (número final confirmado na T-01).

## 3. Backend — worker de corte (`internal/guard`)

### D-01 — pacote novo `guard`, NÃO dentro de `timer`

O corte **reusa** as primitivas do timer (advisory lock, fechar entry, limpar `timer_states`,
`onChange`) mas é uma preocupação diferente (background). Pra não inchar o `timer.Service` nem
arriscar o RNF-05, o corte vive num **pacote novo `guard`** que recebe o `*pgxpool.Pool` e o
`onChange` (o mesmo `hub.BroadcastTimer`) por injeção. O SQL do corte espelha o `Stop` do timer.

### `guard.Cutter` — uma passada (testável, clock injetável)

```go
type Cutter struct {
    db        *pgxpool.Pool
    now       func() time.Time             // injetável (RNF-03) — default time.Now
    onTimer   func(userID string, st *timer.State, at time.Time)  // = hub.BroadcastTimer
    onCut     func(c CutEvent)             // pós-commit: aviso in-app(notice) + push (D-05)
}
const cutAfter = 2 * time.Hour

// RunOnce varre as entries rodando vencidas e corta cada uma. Devolve os
// cortes aplicados (pra teste). Best-effort por usuário: erro num não
// derruba os outros.
func (c *Cutter) RunOnce(ctx context.Context) ([]CutEvent, error)
```

- **Seleção (CA-02):** `SELECT id, user_id, started_at FROM time_entries WHERE stopped_at IS NULL
  AND started_at < $1` onde `$1 = now − 2h` (`<` estrito → exatamente 2h não corta). Fora de
  qualquer transação (leitura barata, RNF-01); a verdade é reconferida sob o lock.
- **Corte por usuário (CA-03/04/07/08), na transação:**
  1. `BEGIN` → `pg_advisory_xact_lock(hashtext(user_id))` (MESMO lock do `timer.Service` — CA-07).
  2. Re-`SELECT ... WHERE id=$entry AND stopped_at IS NULL FOR UPDATE` — sumiu/já parada (o
     usuário parou no intervalo) → **rollback, nada feito** (idempotência/corrida — CA-06/07).
  3. `UPDATE time_entries SET stopped_at = started_at + interval '2 hours',
     duration_seconds = 7200, auto_cut = true WHERE id=$entry` (marco, não `now` — CA-03).
  4. `DELETE FROM timer_states WHERE user_id=$user` (fica ocioso — CA-04; sem linha = no-op,
     cobre a entry órfã).
  5. `COMMIT`.
  6. **Pós-commit:** `onTimer(user, nil, now)` → o Hub emite `type:"timer"` com estado nulo (o
     app converge a barra pra ociosa, igual a um Stop remoto — reusa a fase 04); e `onCut(evt)`
     dispara o **aviso** (notice in-app + push — D-05).
- **CA-09:** `onTimer`/`onCut` só rodam pra entries que **commitaram** o corte (o passo 2 que
  aborta não chama nada).
- **Por que `onTimer` nil E `onCut`:** o `type:"timer"` (state nil) faz a UI ficar ociosa
  automaticamente (reuso puro da 04); o `type:"notice"` é a **explicação humana** (banner/push).
  Os dois são complementares (CA-12).

### `guard.Worker` — o ticker (CA-18, RNF-01)

```go
func (w *Worker) Run(ctx context.Context)  // for { select { <-ticker.C: cutter.RunOnce; <-ctx.Done(): return } }
```

- Tick default **60s** (`config.GuardTick`, env `GUARD_TICK`, 0/inválido → 60s). 1 goroutine.
- No boot: `go worker.Run(ctx)` (mesmo `ctx` do `Run` do servidor) — o `ctx.Done()` do shutdown
  para o worker junto (sem goroutine zumbi). Logar `slog` na partida ("guard worker iniciado",
  intervalo) e por corte ("timer auto-cortado", user/entry).
- **Não roda RunOnce no boot imediatamente** (espera o 1º tick) pra não competir com a migração
  recém-aplicada / pico de boot — decisão D-02 (cosmético; pode rodar imediato se preferir, mas o
  1º tick em 60s é suficiente e mais simples).

### D-03 — clock do worker × clock do timer

Em produção ambos usam `time.Now`. Nos testes o `Cutter` recebe um `WithClock` próprio: pra
provar o limiar/marco, o teste insere uma entry com `started_at = base − (2h+1s)` e roda `RunOnce`
com `now = base` (ou avança o clock). O `dbtest` (fase 01) dá o pool real; helpers de inserir
entry rodando vêm do harness do `timer`/`entries`.

## 4. Backend — Web Push (`internal/push`)

### D-04 — dependência sancionada

**`github.com/SherClockHolmes/webpush-go`** (Web Push API + VAPID; mantida, sem CGO; puxa
`golang-jwt/jwt` e `golang.org/x/crypto`). É a 1 dependência Go nova (RNF-07). Registrar no
`go.mod`. Alternativa avaliada (`Appboy/webpush-go`, fork) descartada — o upstream original basta.

### `push.Subscriptions` (repo) + `push.Sender`

```go
type Subscription struct { Endpoint, P256dh, Auth string }

func (r *Subscriptions) Upsert(ctx, userID string, s Subscription) error  // ON CONFLICT(endpoint)
func (r *Subscriptions) ListByUser(ctx, userID string) ([]Subscription, error)
func (r *Subscriptions) Delete(ctx, endpoint string) error

type Sender struct { subs *Subscriptions; vapidPub, vapidPriv, subject string }
// SendToUser monta o payload, assina com VAPID e POSTa em cada sub do user.
// best-effort (CA-15): erro numa não para as outras. 404/410 → subs.Delete (CA-16).
// vapidPub=="" → no-op (push desligado — CA-16/RNF-04).
func (s *Sender) SendToUser(ctx, userID string, payload PushPayload)
```

- **Validação do subscribe (CA-13):** `endpoint != ""`, `p256dh != ""`, `auth != ""`, senão
  `domain.Validation(...)` → 400 `validation_error` (convenção INDEX).
- **Isolamento (CA-13/CA-8 análogo):** `Upsert`/`ListByUser` sempre filtram/gravam pelo `user_id`
  da sessão. Upsert por `endpoint` reatribui o `user_id` se o mesmo navegador trocar de conta
  (correto: o endpoint é do device atual).
- **Limpeza (CA-16):** o `webpush.SendNotification` devolve `*http.Response`; status 404/410 →
  `Delete(endpoint)`. Outros erros: logar e seguir.
- O `Sender` é chamado pelo `onCut` do `Cutter` (pós-commit, fora do lock) — push lento nunca
  segura o corte nem o ticker (rodar o envio em goroutine própria ou direto, mas SEM bloquear a
  varredura; D-05).

### D-05 — `onCut`: fan-out do aviso

```go
// no cmd/server: cutter.onCut = func(evt CutEvent) {
//   hub.BroadcastNotice(evt.UserID, noticePayload(evt))   // CA-10/12 (in-app)
//   go sender.SendToUser(ctx, evt.UserID, pushPayload(evt)) // CA-15 (push, best-effort)
// }
```

### Config nova (`config.go`)

```
VAPID_PUBLIC_KEY  string   // pública (vai pro cliente via /push/key — CA-14)
VAPID_PRIVATE_KEY string   // NUNCA sai do servidor (RNF-04)
VAPID_SUBJECT     string   // "mailto:..." exigido pelo protocolo
GUARD_TICK        duration // default 60s
```

Sem `VAPID_PUBLIC_KEY` → `/push/key` responde vazio/desabilitado e o Sender é no-op (CA-16). Não
é obrigatório pro boot (RNF-04) — diferente do `DATABASE_URL`.

## 5. Backend — Hub: `BroadcastNotice` (aditivo — RNF-05)

```go
// hub.go (fase 04, INTOCADO no resto): só adiciona este método.
func (h *Hub) BroadcastNotice(userID string, payload any) {
    evt, err := json.Marshal(struct{ Type string `json:"type"`; Payload any `json:"payload"` }{
        Type: "notice", Payload: payload})
    if err != nil { return }
    h.Broadcast(userID, evt)
}
```

- O `BroadcastTimer` e a `Sink`/`Broadcast` existentes **não mudam** (CA-18). É o mesmo
  `Broadcast([]byte)` por usuário, só com outro envelope `type`.

## 6. Frontend — `realtime.ts` (estender), banner, push, SW

### `realtime.ts` — `onNotice` (CA-11)

- `TimerSocketOptions` ganha `onNotice?: (n: Notice) => void`. No `onmessage`: depois do ramo
  `type:"timer"`, **adicionar** `if (env.type === 'notice') { opts.onNotice?.(env.payload); return; }`.
  O ramo `type:"timer"` e o `return` pra desconhecido seguem **idênticos** (CA-11/CA-18). Renomear
  o type interno do envelope pra `{ type?: string; payload?: unknown }` e narrow por ramo.
- `attachTimerSync` passa `onNotice: (n) => pushNoticeToBanner(n)` (um store/emitter simples do
  NoticeBanner). NÃO toca no `onResult`/anti-stale.

### `NoticeBanner.tsx` (CA-12)

- Pequeno store (zustand ou um event simples) que guarda o `notice` atual; o banner no `AppShell`
  renderiza "Seu timer foi pausado automaticamente após 2h" + botão ✕ (limpa o store). Vive no
  shell → aparece em qualquer rota. Acessível (`role="status"`/`alert`, dispensável por teclado).

### `push.ts` (CA-13/14/17)

- `enablePush()`: `Notification.requestPermission()` → se concedido, `registration.pushManager
  .subscribe({ userVisibleOnly: true, applicationServerKey: urlBase64ToUint8Array(vapidPublicKey) })`
  → `POST /api/v1/push/subscribe` com `endpoint`/`keys`. A `vapidPublicKey` vem de `GET /push/key`.
- **Onde disparar:** um botão "Ativar notificações" nos Ajustes (grupo pequeno) — NÃO pedir
  permissão no load (boa prática + Safari/iOS exige gesto do usuário). Detalhe de UX na T.
- **Degradação:** sem suporte a `PushManager`/permissão negada → o app segue (o banner in-app
  cobre quem está com o app aberto).

### `sw.ts` + `vite.config.ts` (CA-17, RNF-06)

- Mudar `VitePWA({ strategies: 'injectManifest', srcDir: 'src', filename: 'sw.ts', ... })`. O
  `sw.ts` usa `precacheAndRoute(self.__WB_MANIFEST)` (mantém o precache), reaplica
  `navigateFallback`/denylist `/api|/auth|/healthz` (workbox routing), e adiciona:
  - `self.addEventListener('push', e => e.waitUntil(self.registration.showNotification(title, opts)))`
  - `self.addEventListener('notificationclick', e => { e.notification.close(); e.waitUntil(clients
    .openWindow('/') ou focar uma aba existente) })`
- **Armadilha (RNF-06):** `injectManifest` NÃO gera as rotas de fallback sozinho como o
  `generateSW` — recriar manualmente (workbox-routing + workbox-precaching) o que o `generateSW`
  dava, senão o offline/instalação regridem. Validar pelos specs do PWA da fase 00 + smoke do E2E.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** fechar a entry com `stopped_at = now` (CA-03) — usar **`started_at + 2h`**, senão a
  métrica ganha as horas-fantasma (a dor inteira da fase).
- **NÃO** cortar sem o advisory lock + `FOR UPDATE` re-check (CA-07) — corre com um Stop/resume
  simultâneo e fecha duas vezes / ressuscita entry parada.
- **NÃO** cortar `mode='break'` nem entries já fechadas (CA-05) — o filtro `stopped_at IS NULL` na
  entry é a única fonte do "rodando"; pausa não tem entry rodando.
- **NÃO** disparar push/notice numa passada que não cortou nada (CA-09) — só pós-commit do corte.
- **NÃO** deixar o push (rede lenta / N subscriptions) bloquear o ticker (RNF-01) — push é
  pós-commit, fora do lock, e não pode segurar a varredura.
- **NÃO** mudar o `BroadcastTimer`/`Sink`/`Broadcast` da fase 04 (CA-18/RNF-05) — só **adicionar**
  `BroadcastNotice`.
- **NÃO** mudar o ramo `type:"timer"` nem o anti-stale do `timerStore` ao adicionar `onNotice`
  (CA-11) — o `notice` é um caminho lateral; o estado do timer converge pelo `type:"timer"`.
- **NÃO** vazar a `VAPID_PRIVATE_KEY` pro cliente nem pro git (RNF-04) — só a pública sai
  (`/push/key`); a privada fica no `.env`/`SECRETS.local.md`.
- **NÃO** rodar a goroutine do worker sem amarrar ao `ctx` do servidor (CA-18) — goroutine zumbi
  no shutdown; usar o mesmo `ctx` do `Run`.
- **NÃO** quebrar o precache/offline ao migrar pro SW customizado (RNF-06) — reaplicar
  `navigateFallback` + denylist no `sw.ts`; validar com os specs do PWA da fase 00.
- **NÃO** persistir estado de timer/cache de `/api` no SW novo (herdado da fase 04 §6 — proibido).

## 8. Ordem de execução (ver tasks.md)

Backend primeiro (migração → cutter TDD → worker → push repo/sender → endpoints → hub notice →
wiring no boot), depois o frontend (realtime onNotice → banner → push.ts → SW + vite config →
ajustes), depois E2E, verificação adversarial e gate. Commit por tarefa. `make check-full` é o
gate final. Blocos de ≤10 tarefas (regra do CLAUDE.md): **bloco 1 = T-01→T-08 (backend)** ·
**bloco 2 = T-09→T-15 (frontend + E2E + verificação + gate)**.
