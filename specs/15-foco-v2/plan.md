# plan.md — Fase 15 · Foco + Pomodoro 2.0

> Como construir a `spec.md` desta pasta. Reusa o `timer.Service`/`timerStore`/`useWakeLock`/
> tokens `--focus-*` das fases 03/05; estende `pomodoro_settings` (migração `0015`) e o ciclo do
> timer. **Backend mínimo (settings + contador de ciclo no `timer_states`); o grosso é frontend**
> (FocoPicker, FocoBreak, auto-start condicional, som). Herda o baseline do `SDD.md §7`.

---

## 1. Arquitetura geral

```
Backend (estende a fase 03):
  migrations/0015_pomodoro_cycles.sql      ← ALTER pomodoro_settings (+5 cols) + ALTER timer_states (+focus_count, +break_is_long)
  internal/timer/settings.go (existe)      ← Settings ganha os 5 campos; SaveSettings valida faixas novas;
                                              GET com merge (CA-02.1 — campos ausentes mantêm o persistido)
  internal/timer/service.go (existe)       ← Pause: incrementa focus_count, escolhe curta×long (CA-05/06);
                                              break_kind no State; reset em stop/start/resume-de-longa (CA-07/08.1)
  internal/api/settings_pomodoro.go (existe)← handlers intocados em forma (corpo cresce via timer.Settings)

Frontend (o grosso):
  lib/api.ts                               ← PomodoroSettings +5 campos; TimerState + break_kind
  lib/queries/settings.ts (existe)         ← intocado (a query já carrega o objeto inteiro)
  lib/timerStore.ts (existe)               ← checkAutoTransition lê as flags (auto-start) + som no fim de fase
  lib/focusSound.ts            (NOVO)      ← beep WebAudio (CA-19/19.1) — isolado/testável
  components/focus/
     FocoPicker.tsx            (NOVO)      ← tela de entrada (campo + recentes + continuar) — CA-09..13
     FocoBreak.tsx             (NOVO)      ← modo descanso desenhado — CA-14/14.1/15
  components/timer/BreakOverlay.tsx (existe)← rótulo "Pausa longa" quando break_kind==='long' (CA-16)
  routes/cockpit/Focus.tsx (existe)        ← ocioso → <FocoPicker/>; break → <FocoBreak/> (mantém trabalho/rodapé/wakelock)
  components/settings/Pomodoro.tsx (existe)← +ciclos/pausa-longa/3 toggles (CA-20/20.1)
frontend/e2e/
  cronometro.spec.ts (existe)              ← fluxo nº3 atualizado pro FocoPicker (CA-21) — arquivo serial
```

## 2. Migração — `0015_pomodoro_cycles.sql` (dona: fase 15)

```sql
-- 0015_pomodoro_cycles — fase 15 (plan §2): Pomodoro 2.0. Estende
-- pomodoro_settings com ciclos/pausa-longa/auto-start/som e timer_states
-- com o contador de focos do ciclo. Defaults PRESERVAM o comportamento da
-- fase 03 (ciclo automático foco↔pausa segue ligado — D-05).
ALTER TABLE pomodoro_settings
  ADD COLUMN cycles_before_long integer NOT NULL DEFAULT 4   CHECK (cycles_before_long BETWEEN 1 AND 12),
  ADD COLUMN long_break_minutes integer NOT NULL DEFAULT 15  CHECK (long_break_minutes BETWEEN 1 AND 120),
  ADD COLUMN auto_start_break   boolean NOT NULL DEFAULT true,
  ADD COLUMN auto_start_focus   boolean NOT NULL DEFAULT true,
  ADD COLUMN sound_enabled      boolean NOT NULL DEFAULT false;

ALTER TABLE timer_states
  ADD COLUMN focus_count   integer NOT NULL DEFAULT 0,    -- focos completos desde a última pausa longa/reset
  ADD COLUMN break_is_long boolean NOT NULL DEFAULT false; -- kind da pausa atual, PERSISTIDO (CA-06.1) — não recomputar
```

- Defaults de auto-start = **`true`** (D-05): a fase 03 já entrega e testa o ciclo automático
  (auto-pause CA-34, auto-resume CA-34.1); um default `false` REGREDIRIA isso. O usuário desliga
  nos Ajustes se quiser controle manual.
- INDEX.md: registrar **0015 dona da fase 15** (após `0013_objectives` da fase 12 e `0014_report_settings` da fase 14).
  `pomodoro_settings` segue dona da fase 03 (migração 0010); a 0015 só ADD COLUMN. `timer_states`
  é da fase 03 (0011); a 0015 ADD COLUMN (aditivo, sem mudar o contrato público `TimerState`).

## 3. Backend — `timer.Settings` + ciclo no `service.go`

### 3.1 Settings (CA-01/02/02.1/03/04)

```go
type Settings struct {
    FocusMinutes     int  `json:"focus_minutes"`
    BreakMinutes     int  `json:"break_minutes"`
    CyclesBeforeLong int  `json:"cycles_before_long"`
    LongBreakMinutes int  `json:"long_break_minutes"`
    AutoStartBreak   bool `json:"auto_start_break"`
    AutoStartFocus   bool `json:"auto_start_focus"`
    SoundEnabled     bool `json:"sound_enabled"`
}
```

- `GetSettings`: `SELECT` os 7 campos; `ErrNoRows` → defaults da §2 (`25/5/4/15/true/true/false`).
- **CA-02.1 (merge):** o handler `PUT` decodifica num struct com **ponteiros** (`*int`/`*bool`)
  pra distinguir "ausente" de "zero", OU `SaveSettings` faz **`GET` corrente → aplica só os
  campos presentes → upsert**. Decisão: usar struct de entrada com ponteiros no handler
  (`pomodoroPatch`), preencher os ausentes a partir do `GetSettings` atual, e chamar
  `SaveSettings` com o `Settings` completo. Assim o serviço continua validando o objeto inteiro
  (CA-03) e o corpo curto da fase 03 não zera nada. *(Sem isso, o front da fase 03 — que ainda
  existe — apagaria os ciclos ao salvar.)*
- `SaveSettings` valida ANTES do SQL: `focus 15–90`, `break 1–60`, `long_break 1–120`,
  `cycles 1–12` → `domain.Validation`. Upsert `ON CONFLICT (user_id)` com todas as colunas.
- **`loadSettings` (interno, usado por Start/Pause):** passa a devolver também `cycles`,
  `longBreak`, e (pro front) nada — auto-start/som são lidos só pelo front. Manter a assinatura
  atual `(focus, brk int, err error)` quebra muitos call sites; **adicionar** `loadFullSettings`
  que devolve o `Settings` e deixar `loadSettings` como fino wrapper, OU estender a tupla.
  Decisão: novo `loadFullSettings(ctx,q,userID) (Settings,error)` (defaults se sem linha);
  `Start`/`Pause` passam a usá-lo. (`loadSettings` antigo pode sumir se nada mais o usa — checar.)

### 3.2 Ciclo (CA-05/06/06.1/07/08/08.1)

- `timer_states` ganha `focus_count` (lido/gravado no `stateRow`).
- **Start (CA-07):** o `INSERT ... ON CONFLICT` zera `focus_count = 0` (novo timer = ciclo do
  zero). Espelha o reset de `resume_*`.
- **Pause (CA-05/06/06.1):**
  1. carrega `Settings` (`loadFullSettings`).
  2. SE o modo que está sendo pausado é `pomodoro_focus` → `newCount = row.FocusCount + 1`
     (countup/countdown não incrementam — CA-05).
  3. SE `breakMinutes` veio no payload → pausa do override (CA-08 da 03), **`break_is_long =
     false`** (`break_kind = "short"` — override é sempre curta, manual), e o `focus_count` ainda
     incrementa se era foco. SENÃO → SE `newCount % cycles == 0` (e era foco) → **longa**
     (`long_break_minutes`), `break_is_long = true`; senão → **curta** (`break_minutes`),
     `break_is_long = false`.
  4. grava `focus_count = newCount` **E `break_is_long`** no `timer_states` (junto do break).
  5. `State.BreakKind = break_is_long ? "long" : "short"` (campo novo derivado da coluna, CA-06.1).
- **resumeFromBreak (CA-07):** lê **`break_is_long` do `timer_states`** — SE a pausa encerrada era
  **longa** → `focus_count = 0` no resume; pausa **curta** → preserva `focus_count` (o ciclo
  continua). NÃO recomputa por `focus_count % cycles` (evita a divergência do override — furo
  confirmado). O resume já lida com o renascer do foco cheio (CA-08 — `cycle_test.go` da fase 03,
  preservado).
- **Stop (CA-07):** o `DELETE FROM timer_states` já zera tudo (linha some) → próximo start começa
  com `focus_count=0`, `break_is_long=false`. OK sem código extra.
- **State / stateOf (CA-06.1):** quando `mode=='break'`, `State.BreakKind = break_is_long ?
  "long" : "short"` **lendo a coluna persistida** (sem carregar settings, sem recomputar). O
  `stateRow` já é lido — só inclui a coluna nova.

> **Por que PERSISTIR `break_is_long` (e NÃO recomputar) — furo confirmado na verificação da
> spec:** recomputar o kind por `focus_count % cycles` no GET/resume DIVERGE do que o `pause`
> decidiu no caso do **override manual** que cai num múltiplo do ciclo: o pause grava `short`
> (override sempre curta), mas o `focus_count` virou múltiplo de `cycles`, então o GET recomputaria
> `long` — estado incoerente (State diz `short`, GET diz `long`, CA-06.1 viola). Persistir o flag
> no `timer_states` (gravado no pause, lido no GET/resume) elimina a divergência e é imune a o
> usuário mudar `cycles` no meio. Teste obrigatório: `cycles_before_long=2`, 2º foco + **override
> manual** → pause E GET seguinte ambos `short`; e o reset do ciclo (CA-07) lê o flag.

### 3.3 Hook OnChange / fase 04 (RNF-06)

`break_kind` e `focus_count` entram no `Result`/`State` que o `onChange` já serializa — o
broadcast da fase 04 carrega de graça. Nenhuma mudança no hub/socket.

## 4. Frontend

### D-01 — FocoPicker (CA-09..13)

- `components/focus/FocoPicker.tsx`: porte do `foco.jsx::FocoPicker` pros tokens/queries reais.
  Recebe `ws` e as ações. Estado local: `description` (input controlado), `selectedProjectId`.
- **Projetos recentes (CA-10):** derivar dos projetos + entries já em cache. `useProjects(ws)`
  (ativos) ordenados por uso recente: cruzar com `useTodayEntries(ws)` (project_id mais
  frequente/recente primeiro); fallback = ordem da listagem. Top 5. O **nome do cliente** no
  cartão vem de **`useClients(ws)`** (`client_id → name`) — query barata já usada pelas páginas de
  projetos, tipicamente cacheada (furo confirmado: `Project` só tem `client_id`, não `client_name`);
  o cartão **degrada pra cor+nome** se o cliente não estiver em cache (RNF-03 preservada — clients
  é leve e cacheado, não um request por-tick).
- **Começar (CA-11):** `onStart` → `timerStore.start({ workspace_id, description, project_id })`.
  Botão desabilitado enquanto `description` vazio **e** nenhum projeto (UX — start mínimo);
  o CA pina que o `start` sai com o contexto selecionado.
- **Continuar (CA-12):** os itens recentes (entries fechadas de hoje, top 2–3) chamam
  `useContinueEntry(ws).mutate(id)`. Seção some se não há entries fechadas.
- Cartões = `button` ≥80px (`minHeight: 92` como o mockup), `aria-pressed`; cores via token.

### D-02 — FocoBreak (CA-14/14.1/15)

- `components/focus/FocoBreak.tsx`: porte do `foco.jsx::FocoBreak`. Consome `useTimerStore`
  (state, resume/skip/stop) + `useTimerClock` (remaining). **Sem clock próprio** (RNF-03 — o
  `useCountdown` do mockup é descartado; usa o `remaining` derivado da âncora).
- Rótulo: `break_kind === 'long'` → "Pausa longa"; senão "Pausa Pomodoro"/"modo descanso". Pausa
  encerrada (`remaining<=0` && origem não-pomodoro) → "Pausa encerrada" + retomar/parar (CA-15,
  herda CA-08 da fase 05). "Ajustar" → `navigate('/ajustes')`.
- Presets (chips) visíveis como no mockup; o ativo = `Math.round(target/60)` casado com preset
  (mesma regra do BreakOverlay). Cor via `--focus-*`/`--accent*` (sem hex).
- O `Focus.tsx` renderiza `<FocoBreak/>` no ramo `isBreak` (substitui o bloco inline atual).

### D-03 — Focus.tsx (ocioso → FocoPicker)

- O ramo `!state` (hoje `0:00:00`+"sem timer rodando") vira `<FocoPicker ws=.../>`. O ramo
  `isRunning` (trabalho) e o rodapé `hoje:`/wake lock da fase 05 **ficam intactos** (CA preservados
  da 05). O ✕ sair segue.
- **Atenção (desvio):** o `Focus.spec.tsx` da fase 05 que assere o ocioso antigo (CA-09 da 05) é
  reescrito pro FocoPicker (spec §7). Manter os testes de trabalho/rodapé/wake lock.

### D-04 — auto-start condicional + som (CA-17/18/18.1/19/19.1)

- **Onde:** o `checkAutoTransition` do `timerStore` decide as auto-transições. Hoje:
  `pomodoro`→`pause`, `break`(resume.mode pomodoro_focus)→`resume`. **A fase 15 condiciona:**
  - foco zera → auto-pause **só se `auto_start_break`** (CA-17);
  - pausa pomodoro zera → auto-resume **só se `auto_start_focus`** (CA-18).
  - As flags vêm do `pomodoro_settings` cacheado. **Problema:** o `timerStore` é um store puro
    (zustand fora de React) e não lê hooks. **Resolução:** o `attachTimerSync` (boot) injeta um
    getter de settings no store (`setAutoStartPolicy(() => settings)`) lido do
    `queryClient.getQueryData(pomodoroKey)`; o `checkAutoTransition` consulta esse getter. Default
    (sem settings carregadas) = `true/true` (preserva o ciclo atual — sem flicker). **Sem request
    por tick** (lê cache). Espelha como o store já recebe o socket no `attachTimerSync`.
  - `firedKey` segue garantindo "no máximo 1" (CA-18.1): a condição da flag só decide SE gata,
    nunca reabre a chave.
- **Som (CA-19/19.1/19.2):** `lib/focusSound.ts` exporta `playPhaseEndBeep()` — cria
  `AudioContext`, um `OscillatorNode` curto (~150ms), try/catch silencioso (CA-19.1). Disparado no
  `checkAutoTransition` no instante em que uma fase termina (remaining chega a 0 e o `firedKey`
  vai gatar), **só se `sound_enabled`**. Para o caso "auto-start desligado" (não há transição), o
  beep ainda deve tocar no fim da fase: disparar pelo cruzamento `remaining > 0 → <= 0` detectado
  no tick, guardado por uma **`soundedKey` atrelada ao `stateKey`** (instância do estado).
  **CRÍTICO (CA-19.2, furo confirmado):** a `soundedKey` **NÃO** é resetada pelo `apply()` (ao
  contrário do `firedKey`, que `apply()` zera a cada resposta) — ela só muda quando o `stateKey`
  muda. Senão, com auto-start OFF a fase fica parada em `remaining=0` e cada re-sync (60s)/evento
  WS re-armaria a key → beep repetido. **Testável:** `focusSound.spec.ts` stuba `AudioContext`
  (defineProperty) — toca 1×, silencia sem suporte; o `timerStore.spec` pina **1 beep mesmo com
  vários `apply()`/re-syncs no MESMO estado parado em 0** (CA-19.2).

### D-05 — defaults de auto-start = true (preservar a fase 03)

Já em §2. Razão: o ciclo automático foco↔pausa é comportamento entregue e testado (CA-34/34.1 da
03, `cycle_test.go`). Default `false` regrediria sem o usuário pedir. O usuário escolhe manual nos
Ajustes (CA-20). Registrado pra não re-litigar.

### D-06 — Ajustes (CA-20/20.1)

- `components/settings/Pomodoro.tsx` (estende o existente): além de foco (slider) e pausa
  (presets+custom), adiciona: input **ciclos** (1–12), input/slider **pausa longa** (1–120), e 3
  `Switch` (auto-start pausa / auto-start foco / alerta sonoro — reusar `components/Switch`).
- Validação no client (CA-20.1): pausa custom 1–60, ciclos 1–12, pausa longa 1–120 → erro
  `role=alert`, **não chama API**. Válido → `useSavePomodoro` com o corpo **completo** (7 campos).
- `lib/api.ts`: `PomodoroSettings` ganha os 5 campos (todos obrigatórios no type — o GET sempre
  devolve completo; o PUT manda completo do form). `TimerState` ganha `break_kind?: 'short'|'long'`.

## 5. Onde vive o quê (resumo)

Ver §1. Arquivos novos: `0015_pomodoro_cycles.sql`, `lib/focusSound.ts(.spec)`,
`components/focus/FocoPicker.tsx(.spec)`, `components/focus/FocoBreak.tsx(.spec)`. Tocados
(aditivo): `timer/settings.go`, `timer/service.go`, `api/settings_pomodoro.go`, `lib/api.ts`,
`lib/timerStore.ts(.spec)`, `routes/cockpit/Focus.tsx(.spec — reescrito o ocioso)`,
`components/settings/Pomodoro.tsx(.spec)`, `components/timer/BreakOverlay.tsx(.spec)`,
`e2e/cronometro.spec.ts`, migração-test.

## 6. Armadilhas conhecidas (vetadas)

- **NÃO** mudar a assinatura/semântica de campos existentes de `Settings`/`TimerState` (RNF-07) —
  só ADD. O `cycle_test.go` da fase 03 (foco cheio no resume — CA-08) **continua valendo**:
  não tocar nesse caminho; o ciclo de N+1 (CA-08.1) é teste NOVO em cima dele.
- **NÃO** auto-pausar/auto-retomar **incondicionalmente** depois desta fase (regrediria o
  CA-17/18): o gate passa a respeitar as flags. Mas o **default true** preserva o comportamento
  atual — os testes da fase 03 (que não setam flag) continuam vendo auto-transição (o getter
  default devolve true). Pinar: teste da fase 03 sem settings → auto-pause segue disparando.
- **NÃO** ler settings via hook dentro do `timerStore` (não é componente) — injetar o getter no
  `attachTimerSync` (D-04), default true. **NÃO** fazer request por tick (lê cache).
- **NÃO** criar clock próprio no FocoBreak (o `useCountdown` do mockup) — usar `useTimerClock`
  (RNF-03), senão o restante diverge do servidor e o auto-resume desincroniza.
- **NÃO** empacotar arquivo de áudio nem dep de som (RNF-01) — só WebAudio oscillator; falhar
  silencioso sem suporte/gesto (CA-19.1). **NÃO** tocar o beep por tick — gate 1×/estado
  (`soundedKey`).
- **NÃO** apagar os campos novos num PUT legado (CA-02.1) — handler com ponteiros + merge do GET
  atual. Pinar: `PUT {focus,break}` curto não zera `cycles_before_long`.
- **PERSISTIR `break_is_long` no `timer_states`** (plan §3.2, furo confirmado) — gravar no `pause`,
  ler no GET/`resume`. **NÃO recomputar** por `focus_count % cycles` (diverge do override manual
  que cai num múltiplo). Pinar: override curta num passo que daria longa → pause E GET ambos `short`.
- **NÃO** deixar o `focus_count` vazar como métrica/contrato público — é runtime interno do
  `timer_states` (como `resume_*`); só o `break_kind` é exposto no `TimerState`.
- **NÃO** quebrar o FocoPicker em hover (toque-first — CA-13); alvos ≥80px.
- **check-cores:** toda cor nova só via token (`--focus-*` da 05, `--accent*`); o mockup usa
  `rgba(255,255,255,.x)` literais e `#000` — **converter** pra `--focus-ink`/`--focus-dim`/
  `--focus-bg` (ou um token novo escopado se faltar tom). O gate barra hex/rgba fora do tokens.css.
- **Controles duplicados sob o overlay** (lição da fase 05): o `/foco` cobre o shell com a
  TimerBar/BreakOverlay montados por baixo — o E2E escopa tudo em `getByTestId('focus')`; o
  FocoBreak/FocoPicker usam testids próprios, NUNCA `break-overlay`.

## 7. Ordem de execução (ver tasks.md)

Backend primeiro (migração → settings com merge → ciclo no service → handler/teste de rota),
depois o som puro, o FocoPicker, o FocoBreak + Focus.tsx, auto-start no store, Ajustes, E2E,
verificação adversarial, gate. Commit por tarefa. `make check-full` é o gate final.
