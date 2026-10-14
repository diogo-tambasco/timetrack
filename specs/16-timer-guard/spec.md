# spec.md — Fase 16 · Anti-timer-esquecido (corte automático + aviso)

> Fonte de verdade da fase. Resolve a dor **P6** do `MILESTONE-1.md §2/§4`: timer esquecido
> ligado polui a métrica. Entrega a **1ª infra de background do projeto** (um worker leve que
> varre timers rodando) + **corte automático em 2h sem pausa** + **aviso in-app (WebSocket da
> fase 04) e Web Push (PWA)**.
> **Decisões travadas com o Diogo (HITL, `MILESTONE-1.md §7`):**
> 1. **Corte automático** quando um timer roda **> 2h SEM pausa** — fecha a entry **no marco das
>    2h** (`stopped_at = started_at + 2h`), NÃO no instante do corte, pra **não poluir a métrica
>    com horas-fantasma**.
> 2. **Aviso SEM email/provider externo:** banner **in-app** (pelo WebSocket que já existe) +
>    **Web Push** (o PWA já tem service worker). Email = evolução futura.
> 3. **Chaves VAPID geradas por nós**, em env/config — **NÃO é dependência externa do Diogo**
>    (não vai pro `PENDENCIAS-DIOGO.md` bloqueando; é só config de deploy).

---

## 1. Objetivo

Quando o Diogo esquece um timer ligado (foi dormir, fechou o notebook, virou o dia), o sistema
**fecha a entry sozinho no limite de 2h** e **avisa** — pra a métrica nunca registrar uma sessão
de 14h fantasma. Pra isso:

1. **Worker de background** (ticker ~1 min, goroutine no boot do `cmd/server`) que varre as
   entries rodando e aplica o corte. É a **primeira infra de background** do Timetrack: tem que
   ser **leve** (VPS compartilhada — RNF-01) e **segura** (idempotente, serializada pelo mesmo
   advisory lock por usuário do `timer.Service` — RNF-02).
2. **Corte** = fechar a entry com `stopped_at = started_at + 2h`, marcá-la `auto_cut = true`, e
   limpar o estado do timer (`timer_states`) — o usuário fica ocioso.
3. **Aviso** do corte por dois canais aditivos: um evento **in-app** pelo Hub WebSocket da fase
   04 (banner no app) e um **Web Push** pros navegadores inscritos.

GCal/relatórios/sync/foco seguem intactos: o worker **adiciona** comportamento, não muda o
contrato REST da fase 03/04 (RNF-05).

## 2. A janela de 2h (definição precisa)

A janela mede **tempo contínuo rodando desde o último start OU resume**, NÃO o tempo de parede
total da sessão:

- A entry rodando nasce com `started_at` no `timer.Service` (start, resume e continue criam uma
  **entry nova** — fase 03). Logo `started_at` da entry rodando É o instante do último
  start/resume.
- **Pausar reseta a janela:** ao pausar, a entry de trabalho é fechada e o usuário entra em
  `mode='break'` (não há entry rodando) — o worker não corta nada. Ao retomar, nasce uma entry
  **nova** com `started_at` no instante do resume → a contagem recomeça do zero.
- Portanto a regra de corte é puramente sobre a entry rodando: **`now − started_at > 2h`**
  (limiar `CUT_AFTER = 2h`, estrito; exatamente 2h ainda NÃO corta — CA-02).

`stopped_at` do corte = `started_at + 2h` (não `now`): a métrica registra **2h exatas**, nunca o
tempo-fantasma entre o marco e a passada do ticker.

## 3. User stories

- **US-01** — Como usuário, quero que um timer esquecido **ligado seja cortado em 2h
  automaticamente**, pra meus relatórios não acusarem uma sessão de 10h que eu não trabalhei.
- **US-02** — Como usuário, quero que o corte registre **2h exatas** (não o tempo até o sistema
  perceber), pra a métrica refletir o limite e não o atraso do servidor.
- **US-03** — Como usuário, quero **ser avisado quando um corte acontece** — um **banner no app**
  se eu estiver com ele aberto, e uma **notificação push** se eu tiver autorizado — pra eu saber
  que o timer foi fechado e por quê.
- **US-04** — Como usuário, quero **autorizar notificações** (Web Push) uma vez e o aviso me
  alcançar mesmo com o app fechado/em background, pra não depender de estar olhando a tela.

## 4. Critérios de aceite (EARS — cada um vira teste)

### Migração & modelo de dados

- **CA-01** (migração) — a fase 16 É dona da migração **`0016_timer_guard`** (próximo número
  livre após `0015`*): adiciona `time_entries.auto_cut boolean NOT NULL DEFAULT false` e cria a
  tabela `push_subscriptions` (`user_id`, `endpoint`, `p256dh`, `auth`, `created_at`; PK por
  `endpoint`, com índice por `user_id`). Os defaults/nullability são o backstop dos CAs abaixo.
  Entries antigas viram `auto_cut=false` sem reprocessamento.
  *(\*o número exato — `0013` (12), `0014` (14), `0015` (15) são das fases anteriores da M1 — é
  fixado na T-01 lendo as migrações presentes; ver plan §2 e tasks T-01.)*

### Worker de corte (backend — clock injetável, como o `timer.Service`)

- **CA-02** (limiar estrito) — uma passada do worker DEVE cortar **exatamente** as entries
  rodando (`stopped_at IS NULL`) com `now − started_at > 2h`. Uma entry com **exatamente 2h** ou
  menos NÃO é cortada (limiar estrito); uma com 2h+1s É cortada.
- **CA-03** (marco, não agora) — ao cortar, a entry DEVE ser fechada com **`stopped_at =
  started_at + 2h`** e `duration_seconds = 7200` (calculado a partir do marco, não de `now`), e
  marcada **`auto_cut = true`**. (Sem isto, a métrica somaria as horas-fantasma — viola US-02.)
- **CA-04** (limpa o estado) — ao cortar a entry rodando de um usuário, o worker DEVE **remover a
  linha de `timer_states`** desse usuário (fica ocioso), igual ao que o `Stop` faz — o `GET
  /api/v1/timer` passa a responder estado nulo. Uma entry rodando SEM linha de `timer_states`
  correspondente (corrupção fora da API) ainda é fechada (a entry é a fonte do "rodando").
- **CA-05** (pausa não corta) — um usuário em **`mode='break'`** (pausa Pomodoro) NUNCA é cortado:
  não há entry rodando, então não há o que cortar. (A janela reseta no pause — §2.) Manual entries
  (`stopped_at` já preenchido) também nunca entram na varredura.
- **CA-06** (idempotência) — duas passadas seguidas do worker sobre o mesmo estado DEVEM produzir
  o **mesmo resultado**: a 2ª passada não encontra mais a entry rodando (já fechada) e não corta
  nada de novo, não reemite aviso. (O filtro `stopped_at IS NULL` garante.)
- **CA-07** (concorrência — advisory lock) — o corte de um usuário DEVE rodar na **mesma
  transação serializada por `pg_advisory_xact_lock(hashtext(user_id))`** que o `timer.Service`
  usa, pra **nunca correr** com um stop/pause/resume simultâneo do usuário. Se o usuário parar o
  timer no exato intervalo entre a seleção e o corte, o `stopped_at IS NULL FOR UPDATE` dentro do
  lock garante que o worker **não fecha duas vezes** nem ressuscita uma entry já parada.
- **CA-08** (isolamento) — o corte DEVE operar exclusivamente sobre a entry e o `timer_states` do
  **dono daquela entry**; NUNCA toca entry/estado de outro usuário. (Constituição §8.4 — mutação
  removendo o `WHERE user_id` deve matar um teste.)
- **CA-09** (aviso só no corte real) — o worker SÓ DEVE emitir aviso (in-app + push) **quando uma
  entry foi efetivamente cortada nesta passada** (transação commitada). Passada sem corte = zero
  aviso, zero push, zero log de corte.

### Aviso in-app (WebSocket da fase 04)

- **CA-10** (envelope `notice`) — ao cortar, o worker DEVE empurrar pelo Hub (fase 04), pras
  conexões `/ws` do dono, um evento com envelope **`{"type":"notice","payload":{...}}`** distinto
  do `type:"timer"`. O payload identifica o aviso de corte (ex.: `{"kind":"timer_auto_cut",
  "entry_id","stopped_at"}`). O Hub NÃO muda de contrato — é uma `Broadcast` de bytes a mais.
- **CA-11** (não quebra o consumidor de timer) — o cliente WS atual (`realtime.ts`) IGNORA hoje
  qualquer `type` que não seja `"timer"` (CA-18 da fase 04). Esta fase DEVE estender o cliente pra
  **também** tratar `type:"notice"` (callback novo `onNotice`), **sem** alterar o tratamento de
  `type:"timer"` nem o anti-stale do `timerStore`. Mensagem malformada/`type` desconhecido segue
  ignorada sem derrubar a conexão.
- **CA-12** (banner) — QUANDO um `notice` de `timer_auto_cut` chega, o app DEVE exibir um
  **banner dispensável** ("Seu timer foi pausado automaticamente após 2h") em qualquer rota
  (ele vive no shell, não numa página). Fechar o banner não dispara nada no servidor. O estado do
  timer já terá convergido pra ocioso pelo evento `type:"timer"` que o `Stop`/corte também emite
  (CA-04 → o corte chama o mesmo `onChange` do timer), então o banner é o **complemento** humano.

### Web Push (subscribe + envio)

- **CA-13** (subscribe) — `POST /api/v1/push/subscribe` autenticado DEVE **persistir a
  subscription** do navegador (`endpoint`, chaves `p256dh`/`auth`) sob o **`user_id` da sessão**,
  com **upsert por `endpoint`** (re-inscrever o mesmo navegador não duplica). Corpo inválido
  (faltando `endpoint`/chaves) → **400 `validation_error`**. Sem sessão → **401** canônico.
- **CA-14** (chave pública) — o front DEVE conseguir obter a **VAPID public key** pra montar a
  subscription. Ela é exposta por um endpoint leve (ex.: `GET /api/v1/push/key` → `{"key":...}`,
  ou embutida na config do app) — a **private key NUNCA** sai do servidor (RNF-04).
- **CA-15** (envio no corte) — ao cortar (CA-09), o servidor DEVE enviar um **Web Push** assinado
  com VAPID a **todas as subscriptions do dono**, com um payload de notificação ("Timer pausado
  após 2h"). O envio é **best-effort**: uma subscription que falha NÃO impede as outras nem o
  corte (o corte já commitou antes; push é pós-commit).
- **CA-16** (limpeza de subscription morta) — QUANDO o envio retorna **404/410 (Gone)** pra uma
  subscription, o servidor DEVE **removê-la** da tabela (o navegador desinscreveu/expirou); outros
  códigos de erro NÃO removem (transitório). Sem chaves VAPID configuradas, o envio é **pulado**
  silenciosamente (o corte e o aviso in-app seguem — RNF-04/degradação).
- **CA-17** (SW handlers) — o **service worker do PWA** DEVE tratar o evento `push` (mostrar a
  notificação via `showNotification`) e `notificationclick` (focar/abrir o app). Isso exige o SW
  **customizado** (estratégia `injectManifest` do `vite-plugin-pwa`) — a migração da config do PWA
  DEVE preservar o precache do app shell, o `navigateFallback` e a denylist `/api|/auth|/healthz`
  atuais (não regredir o offline/login — CA da fase 00/01).

### Não-regressão

- **CA-18** — o boot atual (`cmd/server`), o `timer.Service` (start/stop/pause/resume/state), o
  sync da fase 04 e a UI existentes seguem **byte-compatíveis**: o worker é **aditivo** (goroutine
  ligada no boot, parada no shutdown via `context`), o endpoint de push é novo, o envelope
  `notice` não toca o `timer`. As suítes das fases 03/04 seguem verdes sem editar asserções.

### E2E

- **CA-19** — Logado (stub): com um timer rodando há > 2h (entry semeada com `started_at` no
  passado, ou clock do worker controlado no harness), o usuário com o app aberto vê o **banner de
  corte** aparecer; o `GET /timer` reporta ocioso; a entry no `/hoje`/relatório aparece **fechada
  com 2h** e marcada como auto-cortada. (O Web Push real não é exercitado em E2E headless — o push
  tem teste de unidade no backend; ver TDD.)

## 5. Requisitos não-funcionais

- **RNF-01 (leveza do worker)** — a passada do worker é **uma query indexada** (`SELECT ... WHERE
  stopped_at IS NULL AND now − started_at > 2h`, sobre `uq_entries_one_running` / `idx_entries_*`)
  + o corte por usuário. Tick default **60s** (configurável por env). Numa VPS com poucos
  usuários, custo desprezível; **zero** dependência pesada nova no caminho do worker. Roda em **1
  goroutine**, encerra no shutdown.
- **RNF-02 (segurança/concorrência)** — corte na **mesma transação + advisory lock por usuário**
  do `timer.Service` (CA-07); idempotente (CA-06). O worker NUNCA pode fechar uma entry que o
  usuário acabou de parar/retomar.
- **RNF-03 (clock injetável)** — o worker tem **clock injetável** (igual ao `timer.Service`,
  `WithClock`) — os testes avançam o relógio pra provar o limiar (CA-02) e o marco (CA-03) sem
  esperar 2h reais.
- **RNF-04 (segredos VAPID)** — as chaves VAPID ficam em **env/config** (`VAPID_PUBLIC_KEY`,
  `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`) — a **private NUNCA** vai pro git nem pro cliente; entram
  no `.env` de produção (e no `SECRETS.local.md`/`PENDENCIAS-DIOGO.md` como **config de deploy**,
  não como dependência externa de terceiros). Sem chaves → push desligado, resto funciona (CA-16).
- **RNF-05 (não-regressão)** — fases 03 (timer/entries) e 04 (WebSocket/Hub) **intocadas** no
  contrato (CA-18); o Hub ganha só o uso de `Broadcast` pro `notice` (sem mudar a assinatura).
- **RNF-06 (PWA intacto)** — migrar pra SW customizado (`injectManifest`) DEVE manter o app
  **instalável** e o **precache/offline** atuais; o E2E e os specs do PWA da fase 00 seguem verdes.
- **RNF-07 (deps sancionadas)** — **1 dependência Go nova**: `github.com/SherClockHolmes/webpush-go`
  (Web Push + VAPID; ver plan §4). **Zero** dependência JS nova (Push/Notification APIs nativas).

## 6. Pré-condições externas

- **Gerar o par de chaves VAPID** (uma vez) e colocá-las no `.env` de produção. **Feito por nós**
  (a lib gera; ou `openssl`/ferramenta), NÃO é tarefa de terceiro — registrado como config no
  `PENDENCIAS-DIOGO.md` (não bloqueia o build: testes geram chaves no harness; sem chaves o push
  só fica desligado). Build/testes usam o WebSocket/Hub stubbado e chaves de teste.

## 7. Fora de escopo (fronteiras explícitas)

- **Email / provider externo de notificação** (SendGrid, etc.) — evolução futura (decisão do
  Diogo); esta fase é in-app + Web Push, sem terceiros.
- **Limites de timer configuráveis pelo usuário** (escolher 1h/3h, ligar/desligar o corte) — o
  limiar é fixo em **2h** nesta fase; tornar configurável é fase futura (provável grupo nos
  Ajustes, espelhando o `CalendarGroup`/Pomodoro).
- **Notificações de QUALQUER outra coisa** além do corte (lembretes, tasks atrasadas, resumo
  semanal, pausa em outro device) — o envelope `notice` e o canal push ficam **prontos** pra isso,
  mas só o `timer_auto_cut` é emitido nesta fase.
- **Aviso ANTES do corte** ("seu timer vai ser cortado em 5 min") — fora; corta e avisa.
- **Persistência/cache offline do estado do timer** — segue PROIBIDA (herdado da fase 04 §6); a
  migração do SW pra `injectManifest` NÃO pode introduzir isso.
- **Histórico/auditoria de cortes** numa tela — o `auto_cut` na entry basta pra distinguir; tela
  dedicada é futuro.
