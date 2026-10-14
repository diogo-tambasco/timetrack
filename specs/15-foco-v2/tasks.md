# tasks.md — Fase 15 · Foco + Pomodoro 2.0

> TDD: red → green → refactor. **Commit por tarefa** (`TDD.md §1`). `make check-full` é o gate
> final (T-12). Cada T-XX lista os CAs cobertos. Backend primeiro (settings + ciclo), depois o
> som puro, FocoPicker, FocoBreak+Focus, auto-start no store, Ajustes, E2E, verificação, gate.
>
> **Blocos (regra ≤10 tarefas / contexto zerado entre blocos):**
> - **Bloco 1 — backend (T-01→T-04):** migração, Settings com merge, ciclo no service, handler/rota.
>   Ao fim: `make test-back` verde (Vitest/Playwright só no bloco que os toca).
> - **Bloco 2 — frontend (T-05→T-11):** som, api/types, FocoPicker, FocoBreak+Focus, auto-start,
>   BreakOverlay, Ajustes. Ao fim: Vitest verde.
> - **Bloco 3 — E2E + gate (T-12→T-13):** E2E, verificação adversarial, `make check-full` (deploy
>   é ÚNICO no fim da M1, depois da 16 — NÃO por fase). `make check-full` é gate só no bloco final (T-13).

---

## Bloco 1 — backend (Pomodoro 2.0 + ciclo)

### T-01 — Migração `0015_pomodoro_cycles` (CA-01/03 backstop, RF-01/02, RNF-01)

- `backend/internal/db/migrations/0015_pomodoro_cycles.sql` (plan §2): `ALTER pomodoro_settings`
  +5 colunas com defaults (`4/15/true/true/false`) e CHECKs (`cycles 1–12`, `long_break 1–120`);
  `ALTER timer_states` +`focus_count int NOT NULL DEFAULT 0` **+`break_is_long boolean NOT NULL
  DEFAULT false`** (kind da pausa PERSISTIDO — CA-06.1, furo confirmado: não recomputar).
- Teste (estende `migrations_*_test.go` ou novo `migrations_pomodoro_cycles_test.go`): colunas
  existem com os defaults certos; CHECKs barram `cycles=0/13`, `long_break=0/121`; `focus_count`
  default 0; `break_is_long` default false. **Os defaults de auto-start = `true`/`true`**
  (preservam a fase 03 — D-05/CA-01; pinar o valor).
- INDEX.md: registrar 0015 dona da fase 15.

### T-02 — `timer.Settings` +5 campos + merge no PUT (CA-01/02/02.1/03/04)

- `internal/timer/settings.go`: `Settings` ganha `CyclesBeforeLong/LongBreakMinutes/
  AutoStartBreak/AutoStartFocus/SoundEnabled`; `GetSettings` lê os 7 (defaults se sem linha,
  espelhando a migração — `25/5/4/15/true/true/false`); `SaveSettings` valida `long_break 1–120`
  e `cycles 1–12` (além de foco/pausa) ANTES do SQL → `domain.Validation`; upsert com as 7 cols.
- **Merge (CA-02.1):** struct de entrada com ponteiros no caminho do handler (`pomodoroPatch`)
  OU `SaveSettings` que recebe ponteiros — decisão do plan §3.1: handler decodifica ponteiros,
  preenche ausentes do `GetSettings` atual, chama `SaveSettings` com o objeto completo. (Ver T-04.)
- Novo `loadFullSettings(ctx,q,userID) (Settings,error)` (defaults se sem linha) pra Start/Pause
  usarem; `loadSettings` antigo vira wrapper fino ou some (plan §3.1).
- Teste `settings_test.go` (dbtest): defaults completos; upsert ida-e-volta dos 7 campos; cada
  limite novo (`cycles`, `long_break`) não persiste; isolamento por user (mutação removendo
  `WHERE user_id` morre).

### T-03 — Ciclo no `service.go`: contador + curta×longa + reset (CA-05/06/06.1/07/08/08.1)

- `timer_states` ganha `focus_count` **e `break_is_long`** no `stateRow`/SELECTs/UPSERTs. `Start`
  zera `focus_count=0`, `break_is_long=false` (CA-07). `Stop` já zera (DELETE).
- `Pause` (plan §3.2): carrega `loadFullSettings`; se pausando `pomodoro_focus` → `focus_count+1`;
  escolhe **longa** (`focus_count % cycles == 0 && focus_count>0` → `break_is_long=true`) × **curta**
  (`false`); override manual (`breakMinutes` no payload) = sempre curta (`break_is_long=false`);
  **grava `focus_count` E `break_is_long`**; `State.BreakKind = break_is_long?"long":"short"`.
- `resumeFromBreak`: **lê `break_is_long` do row** — reset `focus_count=0` se era **longa**; curta
  preserva. **NÃO recomputar** por contador+cycles (furo confirmado: diverge do override). Mantém
  o renascer do foco cheio (CA-08 — não tocar o caminho do `cycle_test.go`).
- `stateOf`/`State`: no ramo break, `State.BreakKind = break_is_long?"long":"short"` **lendo a
  coluna** (sem carregar settings, sem recomputar).
- `State` ganha `BreakKind string json:"break_kind,omitempty"` (aditivo — RNF-07).
- Testes (espelham `cycle_test.go`): **CA-08.1** ciclo `cycles=2` → pausa1 curta, pausa2 longa,
  3º foco recomeça ciclo (próxima pausa curta), determinístico com clock fake; CA-05 countup
  pausado não incrementa; **CA-06 override manual num passo que daria longa = curta no pause E no
  GET seguinte** (`break_kind:"short"` nos dois — persistido, furo confirmado); CA-07 stop/start
  resetam (e resume de longa zera, lendo o flag). RED honesto: o teste de N+1 falha antes do código.

### T-04 — Handler `PUT` com merge + rota (CA-01/02/02.1/03/04)

- `internal/api/settings_pomodoro.go`: `handlePutPomodoro` passa a decodificar `pomodoroPatch`
  (ponteiros), buscar o `GetSettings` atual, aplicar só os campos presentes, chamar
  `SaveSettings` com o completo (CA-02.1). `handleGetPomodoro` intocado (já devolve o objeto).
- Teste `settings_pomodoro_test.go` (harness api): GET defaults 200 (7 campos); PUT completo 200
  + persiste; **PUT curto `{focus,break}` NÃO zera `cycles_before_long`** (CA-02.1); PUT inválido
  (`cycles=0`) 400 `validation_error`; sem sessão 401 nas duas.

## Bloco 2 — frontend

### T-05 — `lib/focusSound.ts` + `lib/api.ts`/types (CA-19/19.1; CA-01/06.1)

- `lib/focusSound.ts`: `playPhaseEndBeep()` — `AudioContext` + oscillator curto, try/catch
  silencioso (CA-19.1). `lib/api.ts`: `PomodoroSettings` +5 campos; `TimerState` +`break_kind?`.
- Testes: `focusSound.spec.ts` (stub `AudioContext` via defineProperty: toca 1×; sem suporte =
  no-op sem erro). `api`/types compilam; query de settings devolve os campos (mock).

### T-06 — `FocoPicker` (CA-09/10/11/12/13)

- `components/focus/FocoPicker.tsx` (plan D-01): título + input grande de descrição + grade de
  até 5 projetos recentes (cartões ≥80px, `aria-pressed`) + seção Continuar (entries fechadas de
  hoje, top 2–3) + botão "Começar a cronometrar". Recentes/continuar derivados das queries já em
  cache (`useProjects` + `useTodayEntries`); **nome do cliente do cartão via `useClients`**
  (`client_id→name`, cacheado/barato — furo confirmado; degrada pra cor+nome sem cliente em cache).
  Toda cor via token.
- Spec `FocoPicker.spec.tsx` (padrão das páginas: mock de `../../lib/api` + QueryClientProvider +
  `useAuthStore`/`useTimerStore`): renderiza campo+recentes+continuar (CA-09); 5 cartões ≥80px e
  selecionar marca `aria-pressed` (CA-10); **cliente resolvido do cache aparece, sem cliente só
  cor+nome** (CA-10); Começar chama `start` com description+project_id certo (CA-11); item
  Continuar chama `continueEntry` (CA-12); Continuar some sem entries (CA-12); sem hover
  dependency / cores token (CA-13).

### T-07 — `FocoBreak` + `Focus.tsx` (ocioso→Picker, break→FocoBreak) (CA-09/14/14.1/15)

- `components/focus/FocoBreak.tsx` (plan D-02): porte do mockup com `useTimerClock`/store (sem
  clock próprio — RNF-03); rótulo longa×curta via `break_kind` (CA-14.1); pausa encerrada (CA-15,
  herda CA-08 da fase 05); "ajustar" → `navigate('/ajustes')`; presets visíveis; cor token.
- `routes/cockpit/Focus.tsx`: ramo `!state` → `<FocoPicker/>` (CA-09); ramo break → `<FocoBreak/>`.
  Trabalho/rodapé `hoje:`/wake lock/✕ da fase 05 **intactos**.
- Specs: `FocoBreak.spec.tsx` (render do modo descanso; longa×curta; encerrada; ajustar navega;
  retomar/pular/parar chamam o store). **Reescrever** o `Focus.spec.tsx` da fase 05: o teste do
  ocioso (CA-09 da 05) vira "ocioso renderiza FocoPicker"; manter os testes de trabalho/rodapé/
  wake lock (desvio registrado no plan D-03 / spec §7).

### T-08 — auto-start condicional no `timerStore` + som no gate (CA-17/18/18.1/19)

- `lib/timerStore.ts` (plan D-04): `checkAutoTransition` consulta um getter de política injetado
  (`setAutoStartPolicy`), default `{break:true, focus:true, sound:false}` (preserva a fase 03);
  auto-pause só se `auto_start_break`; auto-resume só se `auto_start_focus`; beep
  (`playPhaseEndBeep`) no fim de fase só se `sound_enabled`, atrelado a `soundedKey` **por
  `stateKey`**. **CRÍTICO (CA-19.2, furo confirmado): a `soundedKey` NÃO é resetada pelo `apply()`**
  (ao contrário do `firedKey`) — só muda quando o `stateKey` muda; senão re-sync/WS re-armam e o
  beep repete com auto-start OFF. `attachTimerSync` injeta o getter lendo
  `queryClient.getQueryData(pomodoroKey)`.
- Testes `timerStore.spec.ts` (casos NOVOS, existentes intactos): foco zera + `break:false` → NÃO
  auto-pausa (CA-17); pausa zera pomodoro + `focus:false` → NÃO auto-retoma (CA-18); ambos true
  (default/legado) → comportamento da fase 03 segue (auto-pause/resume disparam 1× — CA-18.1);
  `sound:true` → beep 1×/transição (espiar o stub); `sound:false` → silêncio; **`sound:true` +
  auto-start OFF + VÁRIOS `apply()`/re-syncs no MESMO estado parado em 0 → beep exatamente 1×
  (CA-19.2)**. **Pinar:** store sem política setada (testes da fase 03) → default true →
  auto-transição segue verde (sem regressão).

### T-09 — `BreakOverlay` rótulo "Pausa longa" (CA-16)

- `components/timer/BreakOverlay.tsx`: rótulo "Pausa longa" quando `state.break_kind === 'long'`;
  curta mantém "Pausa Pomodoro". Nenhum comportamento de timer muda.
- Spec (estende `BreakOverlay.spec`): break_kind long → rótulo longo; sem break_kind / short →
  rótulo padrão. Casos existentes intactos.

### T-10 — Ajustes do Pomodoro 2.0 (CA-20/20.1)

- `components/settings/Pomodoro.tsx` (estende): + input ciclos (1–12), pausa longa (1–120), 3
  `Switch` (auto-start pausa/foco, alerta sonoro). Validação client (CA-20.1) → `role=alert`, não
  chama API; válido → `useSavePomodoro` com corpo COMPLETO (7 campos).
- Spec (estende `Pomodoro.spec` ou novo): form inválido (`cycles=0`, pausa longa=0) não chama PUT
  + erro; válido chama PUT com os 7 campos certos; toggles refletem o estado salvo.

### T-11 — (reservado / folga do bloco 2)

- Buffer pra ajustes de specs reescritos (Focus.spec da 05) e regressões do Vitest. Se vazia, o
  bloco 2 fecha em T-10. (Mantida pra a regra de ≤10 não estourar se o porte do mockup render
  exigir um spec extra de cor/token.)

## Bloco 3 — E2E + verificação + gate

### T-12 — E2E `cronometro.spec.ts` (FocoPicker) + verificação adversarial (CA-21)

- E2E (arquivo serial do timer — plan §6 da fase 05): fixa pausa curta 5 / longa 15 /
  `cycles_before_long=2` / **auto-start OFF** via `PUT /settings/pomodoro` (determinismo); abre
  `/foco` ocioso → FocoPicker → digita descrição + toca projeto recente → Começar → trabalho
  (dígitos+contexto) → Pausar → FocoBreak (dígitos+presets+retomar) → retomar → parar →
  FocoPicker de volta → ✕ volta pro cockpit. Tudo escopado em `getByTestId('focus')`.
  Atualizar o fluxo nº3 da fase 05 que abria pelo ocioso antigo (desvio — spec §7).
- **Verificação adversarial do build** (workflow ultracode; dimensões: ciclo/contador/reset,
  curta×longa×override, auto-start condicional sem regressão, som 1×/sem-suporte, merge do PUT
  legado, regressão das fases 03/04/05/11) + refutação dupla. Corrigir confirmados. Registrar.

### T-13 — Gate + handoff (SEM deploy — único no fim da M1)

- **CA-01:** revisão de imports do `Focus.tsx`/FocoPicker/FocoBreak — só store/queries/wakelock/
  navegação/`focusSound`/componentes locais; zero fetch/WS/interval próprio (lição CA-01 da 05).
- **RNF-07 (sem regressão):** `git diff` da fase — campos de `Settings`/`TimerState` só
  aditivos; suítes das fases 03/04/05/11 verdes sem editar asserções (salvo o `Focus.spec` da 05
  reescrito e o `cronometro.spec` atualizado — desvios sancionados §7; registrar no commit).
- `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose; derrubar `make up`/
  dev servers na :8080 antes do Playwright).
- **SEM deploy** (decisão do Diogo 20/06 — o deploy é ÚNICO no fim das 4 fases da M1, depois da 16).
- Handoff (CLAUDE.md, INDEX 15 construída/0015 dona, memória), commit, segue DIRETO pra fase 16.

---

## Notas de TDD por CA (rastreabilidade)

| CA | Teste | Onde |
|---|---|---|
| CA-01/02/02.1/03/04 | settings GET/PUT/merge/validação/isolamento | Go `settings_test.go` + `settings_pomodoro_test.go` (T-02/T-04) |
| CA-05/06/06.1/07/08/08.1 | ciclo: contador, curta×longa, reset, foco cheio, N+1 | Go `cycle_test.go`/`service` (T-03) |
| CA-09/10/11/12/13 | FocoPicker render/recentes/começar/continuar/toque | Vitest `FocoPicker.spec` (T-06) |
| CA-14/14.1/15 | FocoBreak design/longa×curta/encerrada | Vitest `FocoBreak.spec` (T-07) |
| CA-16 | BreakOverlay "Pausa longa" | Vitest `BreakOverlay.spec` (T-09) |
| CA-17/18/18.1 | auto-start condicional + 1×/estado | Vitest `timerStore.spec` (T-08) |
| CA-19/19.1 | beep 1× / degrada sem suporte | Vitest `focusSound.spec` + `timerStore.spec` (T-05/T-08) |
| CA-20/20.1 | Ajustes 2.0 + validação client | Vitest `Pomodoro.spec` (T-10) |
| CA-21 | fluxo nº3 com FocoPicker | Playwright `cronometro.spec` (T-12) |

## Registro — furos confirmados na verificação adversarial da SPEC (PASSO 0, sessão 18, 20/06)

Workflow ultracode `verify-m1-specs` (176 agentes, 6 dim × refutação dupla, ancorado no código
real). Fase 15: **7 confirmados 2/2 (3 eram o mesmo par auto-start, 2 o mesmo par cliente),
aplicados ANTES de codar — 4 issues únicas:**

1. **(contract-drift + internal-consistency + migration, CA-01)** auto-start defaults `false/false`
   no CA-01 contradiziam a migração (`true`) e regrediriam o ciclo automático da fase 03
   (`timerStore` auto-pausa/resume incondicional, `cycle_test.go`, CA-34/34.1). → **CA-01 + RF-01
   §4: `true/true/false` em TODA parte** (migração já estava `true` — o furo era a spec).
2. **(contract-drift + completeness, CA-10)** o cartão de projeto pede `cliente` mas `useProjects`
   só tem `client_id`. → **CA-10 + D-01: nome do cliente via `useClients` (cache barato), degrada
   pra cor+nome.**
3. **(completeness, CA-06/06.1)** `break_kind` por recompute (`focus_count % cycles`) DIVERGE do
   override manual que cai num múltiplo (pause grava `short`, GET recomputaria `long`). → **CA-06.1
   + plan §3.2 + 0015: PERSISTIR `break_is_long` no `timer_states`** (gravado no pause, lido no
   GET/resume; não recomputar). A armadilha que vetava a coluna foi REVERTIDA.
4. **(completeness, CA-19.2 NOVO)** o beep re-dispararia no re-sync porque a `soundedKey` "análoga
   ao firedKey" seria resetada pelo `apply()` (com auto-start OFF a fase fica em `remaining=0`). →
   **CA-19.2 + D-04: `soundedKey` atrelada ao `stateKey`, NUNCA resetada pelo `apply()`** (1 beep
   mesmo com vários re-syncs no mesmo estado parado).

## Registro — build (executor) + verificação adversarial do BUILD (T-13/T-14, sessão 18, 20/06)

**Build delegado a 1 agente-executor** (instruções reforçadas pós-fase-14: rodar o gate COMPLETO,
sem arquivos scratch, mutação-testar os guards): T-01→T-12, 12 commits TDD, **`make check-full`
GATE VERDE rodado pelo próprio executor**, `git status` limpo. Os 4 furos honrados com teste
load-bearing (mutação confirmada): defaults `true/true/false`; `break_is_long` persistido (override
no múltiplo = curta no pause E no GET); `useClients` no FocoPicker; `soundedKey` por `stateKey` não
resetada por `apply()`. `cycle_test.go` da 03 intocado. Desvios sancionados: `Focus.spec` (ocioso→
FocoPicker), `cronometro.spec` (fluxo nº3 via FocoPicker), `Pomodoro.spec` (PUT 7 campos).

**Verificação adversarial do BUILD** (workflow ultracode, 9 agentes, 7 dim × refutação dupla):
**1 confirmado (LOW), 0 refutados.** Os testes de auto-start nunca exercitavam `break`/`focus`
INDEPENDENTEMENTE — uma mutação que TROCA `break↔focus` entre o auto-pause e o auto-resume
sobrevivia (código correto, gap de teste). → +2 casos cruzados (`break:false/focus:true`→sem pause;
`break:true/focus:false`→sem resume); red→green por mutação (swap do gate → 1 teste falha).

`make check-full` **GATE VERDE** — Go 100% · Vitest 289/289 · Playwright 25/25. **NÃO deployada**
(deploy único no fim da M1).
