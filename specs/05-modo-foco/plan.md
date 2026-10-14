# plan.md — Fase 05 · Modo Foco

> Como implementar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7` e os contratos
> das fases 03/04: `timerStore` (estado = resposta do `GET /timer` + âncora), `useTimerClock`
> (derivação local), `attachTimerSync` (runtime único no AppShell), sync via WS (a 04 entrega
> o estado em todos os devices — o Modo Foco é SÓ mais um consumidor). **Fase 100% frontend.**

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Overlay fixed, não layout novo.** `/foco` continua DENTRO do `AppShell` (rota existente); o componente renderiza um `position: fixed; inset: 0` com z-index acima do rail/TimerBar. | O `attachTimerSync()` é chamado 1 única vez no AppShell (decisão da 03/04 — runtime único, teardown único). Tirar `/foco` do shell exigiria duplicar o runtime ou içá-lo — refatoração proibida pelo CA-02. O overlay cobre tudo e o shell continua montado por baixo, com o mesmo store. |
| D-02 | **Zero requisição nova de timer.** Dígitos = `useTimerClock()` (já re-deriva por tick); estado = `useTimerStore`. A rota só adiciona: `useTodayEntries(ws)` (mesma queryKey da página Hoje — cache compartilhado) e `useProjects(ws, {archived: true})` (cache idem, regra do chip da TimerBar). | CA-01/RNF-03. O Modo Foco é um *projection* do mesmo estado; qualquer caminho novo de dados seria superfície de bug e de teste à toa. |
| D-03 | **Tokens novos `--focus-bg`/`--focus-ink`/`--focus-dim` no `tokens.css`**, definidos no `:root` e **NÃO sobrescritos** em `[data-theme="light"]`. **Teste canônico do CA-03** (verificação adversarial das specs): jsdom não carrega o tokens.css (precedente documentado no `palette.spec.ts`), então o assert "via style" só prova a referência — a invariância de tema é provada por um **teste de integridade** que lê `styles/tokens.css` do disco (readFileSync) e assere `--focus-*` declarados no bloco `:root` e **AUSENTES** do bloco `[data-theme="light"]` (mesma razão do palette.spec: "divergiu, quebra"). | CA-03 (preto independente do tema) com contraprova real. Hex literal no componente reprovaria no `scripts/check-cores.sh` (gate da fase 00); token é o caminho sancionado (precedente: `--danger` na 03). |
| D-04 | **`useWakeLock()` em `lib/wakeLock.ts`**, hook isolado: monta → `navigator.wakeLock.request('screen')`; `visibilitychange` → re-adquire se visível **e não há sentinel ativo** (checar released/ref nula); desmonta → cleanup com **flag de cancelamento**: libera o sentinel ativo E, se houver request **pendente**, dá `release()` no sentinel **quando ele chegar** (release-on-arrival — sem isso, desmontar antes do request resolver vaza wake lock zumbi; o StrictMode do dev reproduz em TODO mount). Sem suporte/rejeição → no-op silencioso (uma tentativa por gatilho; sem retry em loop). | CA-14/15/16 testáveis com `navigator.wakeLock` stubado em Vitest (sentinel fake com `release()`, promise controlada pro caso da corrida), sem browser real. O hook fica reusável (um futuro modo apresentação herdaria). |
| D-05 | **CA-17 mora no `timerStore`:** as 5 ações manuais (`start/stop/pause/resume/skip`) passam por um wrapper `mutate(fn)` — sucesso → `apply(result)`; falha → `load()` imediato (e o `load` falhando também é engolido — CA-17.1). As ações **deixam de rejeitar** (fire-and-forget seguro). | Um ponto único corrige TimerBar + BreakOverlay + Modo Foco de uma vez. Os componentes já chamam `void start(...)` (rejeição era unhandled); nenhum teste das fases 03/04 assere rejeição das ações (verificado por grep antes desta spec) — mudança aditiva, CA-02 preservado. O `checkAutoTransition` continua com o próprio `.catch(() => load())`: vira redundante mas inofensivo (o caminho do 409 automático já era convergente; manter evita mexer em lógica provada). |
| D-06 | **Total do dia = `total_seconds` da query de hoje + elapsed vivo da ENTRY RODANDO** (copiar a aritmética do TodayList: `liveSeconds` da entry rodando vinda da QUERY — **nunca** o `useTimerClock().elapsed` cru, que em `mode='break'` é o elapsed da PAUSA e inflaria o total). Quando `state.entry?.id` muda (start/stop/troca), um `useEffect` invalida `entriesKey(ws)` — a entry recém-fechada entra no `total_seconds` do servidor. | CA-10 ("absorve a entry recém-fechada sem reload"; "em pausa o total não avança"). A invalidação por mudança de id é a mesma necessidade que a página Hoje já tem; fazer no Modo Foco mantém a fase autocontida (o TodayList tem o próprio fluxo de invalidação via mutações). |
| D-07 | **Estados da tela = função direta do store** (sem state machine própria): `state === null` → ocioso; `state.mode === 'break'` → descanso; senão → rodando. Dígitos: countup → `elapsed`; resto → `remaining ?? elapsed` (regra `shown` idêntica à RunningBar). | CA-04/05/07/09 sem lógica nova de timer — qualquer transição (inclusive vinda de outro device pela 04) é só re-render. |
| D-08 | **Sair = `navigate('/hoje')`**, botão ✕ ghost no canto superior direito. | CA-13. A rota desmonta → o cleanup do `useWakeLock` libera o sentinel. Sem histórico esperto (`navigate(-1)` poderia sair do app se /foco foi aberto por link direto no iPad). |

## 2. Modelo de dados / migrações

**Nenhuma.** Fase 100% frontend (CA-02).

## 3. Contratos

Nenhum contrato novo de API. Consumo (tudo já existente):

| Fonte | Uso no Modo Foco |
|---|---|
| `useTimerStore` / `useTimerClock` | estado + dígitos derivados (CA-04/05/07/09) |
| ações do store `pause/resume/skip/stop` | controles (CA-11/12) — com a convergência nova do CA-17 |
| `useTodayEntries(ws)` → `total_seconds` | rodapé (CA-10) |
| `useProjects(ws, {archived: true})` | nome do projeto (CA-06) |
| `navigator.wakeLock` | CA-14/15/16 |

### 3.1 Formato do rodapé

`fmtDayTotal(sec)` → `"5h12"` (horas sem zero à esquerda, minutos com 2 dígitos; `0h00`
pra zero). Renderizado como `hoje: 5h12` em `--focus-dim`.

## 4. Libs

**Nenhuma nova** (RNF-01). Wake Lock é API nativa; tipos: `navigator.wakeLock` já existe no
lib.dom do TS 5.x — se o `tsc` do projeto não conhecer, declarar o mínimo em
`src/types/wakelock.d.ts` (declaração local ≠ dependência).

## 5. Onde vive o quê

```
frontend/src/
  lib/
    timerStore.ts        wrapper mutate() nas 5 ações manuais (CA-17/17.1 — D-05)
    timerStore.spec.ts   casos novos: 409 manual → load imediato; load falhando → silêncio
    wakeLock.ts          useWakeLock() (D-04)
    wakeLock.spec.ts     request/release/re-acquire/sem-suporte/rejeição (CA-14/15/16)
    format.ts            fmtDayTotal (RF-03)
    format.spec.ts       (novo arquivo se não existir; casos 0h00/5h12/26h05)
  routes/cockpit/
    Focus.tsx            o Modo Foco real (substitui o StubPage)
    Focus.spec.tsx       render dos estados + controles + rodapé (CA-03..13 unit)
  styles/tokens.css      --focus-bg/--focus-ink/--focus-dim no :root (D-03)
frontend/e2e/
  cronometro.spec.ts     teste novo do fluxo nº3 (CA-18) — NO ARQUIVO SERIAL (regra da 03/04)
```

## 6. Pontos de integração e armadilhas conhecidas

- **E2E muta o timer ⇒ arquivo serial.** O teste do CA-18 entra no `cronometro.spec.ts`
  existente (regra do bloco 3 da fase 03: timer é global por usuário; arquivo paralelo
  novo brigaria pelo timer). `ensureIdle` no início, settings de pausa fixadas via
  `PUT /settings/pomodoro` pela API antes do fluxo (determinismo — o CA-38 da 03 deixa 15
  no banco reusado).
- **Controles DUPLICADOS sob o overlay (strict mode):** a TimerBar (e o BreakOverlay
  dentro dela) continuam montados por baixo do overlay (D-01) com os MESMOS botões
  ("Pausar"/"Parar"/"Retomar") e dígitos `.mono` — locator page-level no E2E estoura o
  strict mode ou assere o gêmeo errado. Regra: o overlay tem `data-testid="focus"`
  PRÓPRIO (**proibido** reusar `break-overlay` dentro do Focus) e TODA query do fluxo
  nº3 é escopada nele (`const foco = page.getByTestId('focus'); foco.getByRole(...)`).
- **Elapsed em pausa NÃO é tempo trabalhado:** `useTimerClock().elapsed` em
  `mode='break'` é o elapsed da PAUSA — o rodapé do CA-10 deriva a soma viva da ENTRY
  RODANDO da query (aritmética do TodayList), nunca do clock cru (D-06).
- **`offline.spec.ts:23`** (aviso herdado): esta fase NÃO introduz persistência local nem
  cache de `/api` — o assert `0:00:00` continua determinístico. O CA-17 não muda o `load()`
  (caminho do offline): mutação manual nem existe no fluxo daquele spec.
- **Wake Lock em teste:** Vitest stuba `navigator.wakeLock` (defineProperty); o sentinel
  fake expõe `release()` (vi.fn) e o teste de re-acquire dispara `visibilitychange` com
  `document.visibilityState` stubado. **Playwright NÃO assere wake lock** (flaky em
  headless): CA-14/15/16 são unit-only, decisão registrada.
- **check-cores:** qualquer cor nova SÓ via token (D-03). O script barra hex fora do
  tokens.css/palette.ts (exceções sancionadas na 02).
- **z-index do overlay:** acima do `.tt-sidenav` e da TimerBar — conferir os z usados no
  tokens.css e usar um patamar folgado (ex.: 80). O overlay cobre o shell inteiro; nada do
  shell precisa esconder (continua montado por baixo — D-01).
- **`useEffect` de invalidação (D-06):** depender de `state?.entry?.id` (string | undefined),
  NÃO do objeto state (re-render por tick não pode invalidar query — seria 1 invalidação/s).
- **Suítes da 03/04 intactas (CA-02):** o `timerStore.spec.ts` ganha casos NOVOS; os
  existentes não mudam (as ações continuam aplicando resultado em sucesso; só o caminho de
  falha — antes não-testado — ganha comportamento definido).
