# tasks.md — Fase 16 · Anti-timer-esquecido

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-15).
> Cada T-XX lista os CAs cobertos. Backend primeiro (migração → corte → push → wiring), depois
> o frontend (notice/banner/push/SW), E2E, verificação adversarial e gate.
> **Blocos (regra ≤10 tarefas):** bloco 1 = T-01→T-08 (backend) · bloco 2 = T-09→T-15 (front +
> E2E + verificação + deploy). Gate `make check-full` só no fim (T-15).

---

## Bloco backend (corte + push)

- **T-01 — Migração `0016_timer_guard`** (CA-01, RNF-04 backstop)
  - Fixar o número lendo `backend/internal/db/migrations/` (próximo livre após as da M1; nome
    lógico `timer_guard`). `time_entries.auto_cut boolean NOT NULL DEFAULT false` + tabela
    `push_subscriptions` (PK `endpoint`, FK `user_id` cascade, `p256dh`/`auth`/`created_at`,
    índice por `user_id`) — plan §2.
  - Teste (estende `migrations_*_test.go` ou novo): colunas/tabela existem; `auto_cut` default
    false; índice presente; FK cascade.
  - INDEX.md: registrar 0016 (número final) dona da fase 16.

- **T-02 — `guard.Cutter.RunOnce` — corte de uma passada** (CA-02/03/04/05/06/07/08, RNF-02/03)
  - Pacote novo `internal/guard`; `Cutter` com `db`, `now` injetável (`WithClock`), `onTimer` e
    `onCut` injetáveis (plan §3). Seleção `stopped_at IS NULL AND started_at < now−2h`; corte por
    usuário na transação (advisory lock + `FOR UPDATE` re-check + `stopped_at=started_at+2h`,
    `duration_seconds=7200`, `auto_cut=true` + `DELETE timer_states`).
  - `cutter_test.go` (dbtest, RED primeiro): **casos obrigatórios** —
    - exatamente 2h NÃO corta; 2h+1s corta (limiar estrito — CA-02);
    - entry cortada tem `stopped_at == started_at+2h`, `duration_seconds==7200`, `auto_cut==true`
      (marco, não `now` — CA-03);
    - `timer_states` do user é removido após o corte (CA-04); entry rodando órfã (sem
      `timer_states`) ainda é fechada;
    - usuário em `break` (semear `timer_states` mode='break', sem entry rodando) NÃO é cortado;
      entry manual já fechada não entra (CA-05);
    - 2ª passada sobre o mesmo estado não corta de novo nem reemite (idempotência — CA-06);
    - **corrida:** parar o timer entre a seleção e o corte (simular: deletar/fechar a entry antes
      do RRunOnce sob lock) → o `FOR UPDATE` re-check aborta, nada é fechado em dobro (CA-07);
    - isolamento: 2 usuários, só o vencido é cortado; mutação removendo `WHERE user_id` morre
      (CA-08);
    - `onTimer`/`onCut` chamados **só** pras entries cortadas nesta passada (CA-09) — spy nos hooks.

- **T-03 — `guard.Worker` — ticker + goroutine** (CA-18, RNF-01)
  - `Worker.Run(ctx)`: `time.Ticker` (intervalo de `config.GuardTick`, default 60s); `<-ticker.C`
    → `cutter.RunOnce`; `<-ctx.Done()` → return (para no shutdown). Log de partida + por corte.
  - Teste: `Run` com ctx cancelável + intervalo curto (ms) chama `RunOnce` ≥1× e **retorna** ao
    cancelar o ctx (sem goroutine zumbi — usar canal/`sync` pra provar a parada).

- **T-04 — `push.Subscriptions` (repo)** (CA-13 persistência/isolamento)
  - `internal/push/subscriptions.go`: `Upsert` (`ON CONFLICT(endpoint)`), `ListByUser`, `Delete`.
    Sempre por `user_id` da sessão.
  - `subscriptions_test.go` (dbtest): upsert idempotente por endpoint (re-inscrever não duplica);
    troca de user no mesmo endpoint reatribui; `ListByUser` só vê as do user; `Delete` por
    endpoint; mutação removendo `WHERE user_id` no ListByUser morre.

- **T-05 — `push.Sender` — envio VAPID best-effort + limpeza** (CA-15/16, RNF-04/07)
  - Dep nova sancionada `github.com/SherClockHolmes/webpush-go` (plan §4/D-04) no `go.mod`.
    `Sender.SendToUser`: monta payload, assina VAPID, POSTa em cada sub; 404/410 → `subs.Delete`;
    outros erros logam e seguem; `vapidPub==""` → no-op.
  - `sender_test.go`: servidor HTTP fake como push service — sucesso (200) não remove; **410
    remove a sub** (CA-16); 1 sub falhando não impede a outra (best-effort — CA-15); sem chaves →
    nenhum request (no-op — CA-16/RNF-04). (VAPID keys de teste geradas no harness.)

- **T-06 — Endpoints `POST /push/subscribe` + `GET /push/key`** (CA-13/14)
  - `internal/api/push_routes.go` (handlers finos por-usuário, espelho do settings/pomodoro):
    subscribe decodifica `{endpoint, keys:{p256dh,auth}}`, valida (400 `validation_error` se
    faltar campo), `Upsert` pelo `reqctx.UserID`; key responde `{"key": vapidPublic}` (vazio se
    desabilitado). `WithPush(...)` Option no router; montar no `/api/v1` atrás do `RequireAuth`.
  - `push_routes_test.go` (harness api): subscribe válido 200 + persiste; inválido 400; sem sessão
    401 nas duas; `GET /push/key` devolve a pública.

- **T-07 — `Hub.BroadcastNotice` (aditivo)** (CA-10, RNF-05)
  - `internal/realtime/hub.go`: **só adicionar** `BroadcastNotice(userID, payload)` (envelope
    `type:"notice"`) — plan §5. `BroadcastTimer`/`Sink`/`Broadcast` intocados.
  - Teste no pacote realtime: `BroadcastNotice` entrega `{"type":"notice","payload":...}` só às
    conexões do user (reusa o harness/`Sink` fake da fase 04); o consumidor de `timer` não é
    afetado.

- **T-08 — Config VAPID/GUARD_TICK + wiring no boot** (CA-09/15/18, RNF-01/04/05)
  - `config.go`: `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`/`VAPID_SUBJECT`/`GUARD_TICK` (vazios/0 →
    push desligado / tick 60s; **não** obrigatórios pro boot). `cmd/server/main.go`: construir
    `push.Sender`, `guard.Cutter` (`onTimer=hub.BroadcastTimer`, `onCut` = `BroadcastNotice` +
    `go sender.SendToUser`), `guard.Worker`; `go worker.Run(ctx)` com o `ctx` do `Run`; montar
    `WithPush` no router.
  - Teste: `config_test.go` (defaults/parse das env novas); wiring exercitado por um teste de boot
    leve ou pelo harness existente (o worker liga sem quebrar o boot; exceção ao red estrutural
    documentada — espelha o `tsvc.SetOnChange` da fase 04).

## Bloco frontend + E2E + gate

- **T-09 — `realtime.ts` `onNotice`** (CA-11/18)
  - `TimerSocketOptions` ganha `onNotice?`; `onmessage` trata `type:"notice"` **depois** do
    `type:"timer"` (ramo timer e anti-stale INTOCADOS); desconhecido segue ignorado.
  - `realtime.spec.ts` estendido: `type:"notice"` chama `onNotice` com o payload; `type:"timer"`
    segue chamando `onResult`; `type` desconhecido/JSON inválido não chama nem derruba (regressão
    da fase 04 preservada).

- **T-10 — `NoticeBanner` + wiring no shell** (CA-12)
  - Store/emitter simples + `NoticeBanner.tsx` (mensagem do corte, ✕ dispensa, `role="status"`,
    vive no `AppShell`); `attachTimerSync` passa `onNotice` ao `connectTimerSocket` alimentando o
    store. Não toca no `onResult`.
  - Specs (jsdom): `notice` de `timer_auto_cut` mostra o banner; ✕ fecha (sem chamada de API);
    `notice` desconhecido não quebra; banner ausente sem notice.

- **T-11 — `push.ts` + `api.ts` + botão "Ativar notificações"** (CA-13/14)
  - `getPushKey`/`postPushSubscribe` em `api.ts`; `lib/push.ts` (`enablePush`: permission →
    `pushManager.subscribe(applicationServerKey)` → POST); grupo/botão nos Ajustes (gesto do
    usuário, NÃO no load). Degradação sem suporte/negado.
  - `push.spec.ts` (jsdom, mock de `Notification`/`navigator.serviceWorker`/`PushManager`):
    permission concedida → subscribe → POST com endpoint/keys certos; negada → não chama POST;
    sem `PushManager` → no-op sem throw.

- **T-12 — SW customizado (`sw.ts`) + `vite.config.ts` injectManifest** (CA-17, RNF-06)
  - `VitePWA` → `strategies:'injectManifest'`, `srcDir:'src'`, `filename:'sw.ts'`; `sw.ts` com
    `precacheAndRoute(self.__WB_MANIFEST)` + reaplicar `navigateFallback`/denylist
    `/api|/auth|/healthz` (não regredir offline/login — fase 00/01) + handlers `push`
    (`showNotification`) e `notificationclick` (focar/abrir `/`).
  - Validação: specs do PWA da fase 00 seguem verdes; o build do SW emite (`make build-front`); o
    smoke do E2E (app instalável, shell offline) não regride. (Handlers `push`/`click` validados
    pelo build + revisão; o disparo real do SO não é headless — registrado.)

- **T-13 — E2E `timer-guard.spec.ts`** (CA-19)
  - Logado (stub): semear uma entry rodando com `started_at` no passado (> 2h) via SQL helper
    (`e2e/helpers.ts`, padrão `seedWorkspaceMember`/`resetCalendarSettings`) + `timer_states`
    correspondente; com o app aberto e o worker tickando (intervalo curto por env no webServer do
    E2E, ou gatilho), ver o **banner de corte** aparecer; `GET /timer` ocioso; a entry no
    `/hoje`/relatório fechada com **2h** e `auto_cut`. Web Push real NÃO exercitado (unit no back).
  - Reusar `loginViaStub`; intervalo do worker curto só no ambiente E2E.

- **T-14 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: limiar/marco do corte · concorrência/lock/idempotência ·
    isolamento entre usuários · push best-effort/limpeza 404-410/segredos · regressão fase 03/04 ·
    PWA/SW offline) + refutação dupla. Corrigir confirmados. Registrar no fim deste arquivo.

- **T-15 — Gate + deploy + smoke + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose).
  - Gerar o par VAPID e pôr no `.env` de produção (config nossa — `PENDENCIAS-DIOGO.md` como
    config de deploy, NÃO dependência externa). Deploy pela rotina do KICKOFF (rsync + `docker
    compose build && up -d` em `/opt/timetrack`; **NÃO** tocar no Caddy do PrimaOS). Smoke honesto
    (`/healthz` 200; `/api/v1/push/key` responde; boot loga "guard worker iniciado"; vizinhos
    `primaos-*` intactos).
  - Handoff (CLAUDE.md "retomar..."), INDEX (16 construída), commit, briefing pro Diogo.

---

## Registro — verificação adversarial da SPEC (a preencher na sessão de verificação, pré-código)

> Espaço pros findings da verificação adversarial da spec (Regra Nº1: rodar ANTES do build).
> Confirmados viram CA/casos de teste novos AQUI antes de codar.

## Registro — BUILD + verificação adversarial do BUILD (a preencher)

> Suítes finais, findings do `verify-build`, correções aplicadas.
