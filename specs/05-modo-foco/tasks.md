# tasks.md — Fase 05 · Modo Foco

> Tarefas atômicas, TDD red→green→refactor, **commit por tarefa** (`TDD.md §1`).
> Fase 100% frontend — suíte alvo: Vitest + Playwright (Go intocado, mas `make test-back`
> roda no gate pra provar o CA-02).

---

## T-01 — CA-17/17.1: mutação manual falhou → convergência imediata (timerStore)

- **Objetivo:** as 5 ações manuais do `timerStore` (`start/stop/pause/resume/skip`) passam
  pelo wrapper `mutate(fn)` (plan D-05): sucesso → `apply`; falha → `load()` imediato;
  falha do próprio `load` → silêncio.
- **Entradas:** `lib/timerStore.ts`, `lib/timerStore.spec.ts` (harness de stub de fetch puro
  da fase 03).
- **RED:** casos novos no `timerStore.spec.ts`: (a) `pause()` com servidor respondendo 409 →
  um `GET /timer` sai IMEDIATAMENTE e o estado aplicado é o da resposta do GET; (b) a ação
  não rejeita (await não lança); (c) 409 + GET falhando (rede) → nenhuma exceção vaza.
  Confirmar que (a) FALHA antes do código (hoje a rejeição é unhandled e nenhum GET sai).
- **Acceptance:** casos novos verdes; **todos os casos existentes do spec intactos e
  verdes** (CA-02).

## T-02 — CA-14/15/16: hook useWakeLock

- **Objetivo:** `lib/wakeLock.ts` com `useWakeLock()` (plan D-04).
- **RED:** `lib/wakeLock.spec.ts` com `navigator.wakeLock` stubado: (a) montar → `request('screen')`
  chamado 1x; (b) desmontar → `release()` do sentinel + sem listener vazado; (c) sentinel
  liberado pelo sistema + `visibilitychange` (visível) → novo `request`; (d) `navigator.wakeLock`
  indefinido → monta/desmonta sem erro e sem request; (e) `request` rejeitando → sem erro,
  sem retry em loop (1 tentativa por gatilho); (f) **corrida do desmonte** (D-04, verificação
  adversarial das specs): request devolve promise CONTROLADA → desmontar ANTES de resolver →
  resolver depois → `release()` é chamado no sentinel recém-chegado (release-on-arrival) e
  nenhum sentinel fica ativo após o desmonte — é a mutação "sem flag de cancelamento" que o
  StrictMode reproduz em todo mount.
- **Acceptance:** spec verde; hook sem dependência nova.

## T-03 — CA-10 (metade pura): fmtDayTotal

- **Objetivo:** `fmtDayTotal(sec)` em `lib/format.ts` → `"0h00"`, `"5h12"`, `"26h05"`
  (horas sem teto de 24 — dia com >24h cronometradas é possível com entries manuais).
- **RED:** casos em spec de format (criar `lib/format.spec.ts` se não existir).
- **Acceptance:** spec verde.

## T-04 — CA-03..09 + CA-11..13: página Focus (overlay fullscreen)

- **Objetivo:** `routes/cockpit/Focus.tsx` real (substitui StubPage): overlay
  `fixed inset-0` com tokens `--focus-*` (adicionados ao `tokens.css` nesta tarefa — D-03),
  estados ocioso/rodando/descanso (D-07), nome do projeto (regra do chip — CA-06), controles
  ghost (Pausar/Parar rodando; retomar/pular/parar no descanso — semântica do BreakOverlay),
  ✕ sair → `/hoje` (D-08).
- **RED:** `Focus.spec.tsx` (padrão das páginas da 02/03: mock de `../../lib/api` +
  QueryClientProvider + `useAuthStore.setState` + estado semeado no `useTimerStore`):
  um teste por CA — **CA-03 = o teste de integridade CANÔNICO do D-03** (readFileSync de
  `styles/tokens.css`: `--focus-bg`/`--focus-ink`/`--focus-dim` declarados no bloco `:root`
  e AUSENTES do bloco `[data-theme="light"]` — precedente `palette.spec.ts`; o assert via
  style do overlay é complementar, NÃO a prova — verificação adversarial das specs),
  countup mostra elapsed (CA-04), countdown mostra remaining (CA-05), projeto/descrição/nada (CA-06, 3
  casos), break vira descanso com retomar/pular/parar (CA-07), pausa zerada de countup =
  "pausa encerrada" sem auto-ação (CA-08), ocioso = 0:00:00 esmaecido + "sem timer rodando"
  e ZERO botão de start (CA-09), Pausar/Parar chamam as ações do store (CA-11), retomar/
  pular/parar idem (CA-12), ✕ navega pra /hoje (CA-13).
- **Acceptance:** Focus.spec verde; `check-cores.sh` passa (cores só via token).

## T-05 — CA-10 + CA-14..16 wiring: rodapé do dia + wake lock na rota

- **Objetivo:** Focus ganha o rodapé `hoje: 5h12` (D-06: `useTodayEntries` + elapsed vivo +
  invalidação quando `state?.entry?.id` muda) e o `useWakeLock()` montado na rota.
- **RED:** casos novos no `Focus.spec.tsx`: (a) rodapé soma `total_seconds` + elapsed da
  entry rodando no formato certo; (b) stop (mudança de `entry.id` → undefined) → a query de
  entries é invalidada (espionar `queryClient.invalidateQueries` ou assertar refetch pelo
  stub); (c) wake lock requisitado ao montar a rota e liberado ao desmontar (reusa o stub
  do T-02 — aqui é só o wiring).
- **Acceptance:** Focus.spec verde; nenhuma requisição nova por segundo (caso (a) com fake
  timers prova que tick NÃO refaz fetch — RNF-03).

## T-06 — CA-18: E2E fluxo nº3 + gate + INDEX

- **Objetivo:** teste novo no `cronometro.spec.ts` (arquivo serial — plan §6): fixa pausa
  de 5min via `PUT /settings/pomodoro` (API), inicia timer com projeto, abre `/foco`,
  assere dígitos gigantes + nome do projeto + rodapé `hoje:`, Pausar → countdown ≤5:00 e
  >4:40 + rótulo de pausa, retomar → dígitos de trabalho de volta, parar → ocioso, ✕ →
  volta pra `/hoje`.
- **Depois do E2E, ANTES do gate (verificações delegadas pela spec — sem elas CA-01/02
  ficam órfãos):**
  1. **CA-01:** revisão de imports de `routes/cockpit/Focus.tsx` — só
     `useTimerStore`/`useTimerClock`, ações do store, `useTodayEntries`, `useProjects`,
     `useWakeLock`, `fmtDayTotal`/formatadores, navegação e componentes locais; **zero**
     fetch/WebSocket/`setInterval` próprio. Registrar o resultado no commit.
  2. **CA-02:** `git diff` da fase × enumeração fechada do CA-02 — nenhum arquivo
     compartilhado fora da lista (`lib/timerStore.ts`, `styles/tokens.css`, `lib/format.ts`,
     `routes/cockpit/Focus.tsx`, `e2e/cronometro.spec.ts`), asserções das fases 00–04
     intactas (precedente do CA-04 da 04). Registrar no commit de fechamento.
- **Depois:** `make check-full` (GATE da fase — derrubar `make up` antes do Playwright) e
  `specs/INDEX.md` marcando a 05 construída.
- **Acceptance:** `make check-full` GATE VERDE; INDEX atualizado; passos 1–2 registrados.
