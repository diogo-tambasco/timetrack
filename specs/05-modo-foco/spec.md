# spec.md — Fase 05 · Modo Foco

> O flip clock do iPad vira 1ª classe (`REQUISITOS.md §4.4`): a rota `/foco` é um
> **flip-clock fullscreen preto** com dígitos gigantes, nome do projeto, controles mínimos
> e tela sempre acordada (Wake Lock). Pausar → vira o Pomodoro de pausa em todos os devices
> (a fase 04 já entrega o sync; aqui é só mais uma superfície do MESMO `timerStore`).
> Depende de: fase 03 (`timerStore`, `useTimerClock`) + fase 04 (sync realtime).
>
> **Decisões do Diogo (11/06, travadas no KICKOFF — não re-perguntar):** flip-clock
> fullscreen preto · dígitos gigantes · nome do projeto · controles mínimos
> (pausar/parar discretos) · Wake Lock API · pause → Pomodoro (comportamento da 04 cobre) ·
> **total do dia discreto no rodapé** (ex.: "hoje: 5h12") — sem lista, sem virar dashboard ·
> consome o timerStore/sync da 04 · o candidato a CA herdado das sessões 7/8 (**409 de
> mutação manual engolido pela UI no modo degradado**) AGORA vira CA (CA-17).

---

## 1. Mapa pro problema central (`REQUISITOS.md §2` e §4.4)

| Critério | O que esta fase entrega |
|---|---|
| Substituir o Flip Clock do iPad | `/foco` fullscreen: dígitos gigantes mono em fundo preto, projeto embaixo, zero distração |
| Pausei → Pomodoro em todos os devices | O Modo Foco REAGE ao mesmo `timerStore` sincronizado: pausa local ou remota vira o modo descanso na tela cheia |
| Tela sempre acordada o dia todo | Wake Lock API (re-adquirido quando a página volta a ficar visível) |
| Visão leve do dia (§4.7 iPad) | Total do dia discreto no rodapé — só o número, sem lista |

## 2. User stories

- **US-01** — Como usuário com o iPad de canto de olho, quero **dígitos gigantes em fundo
  preto** com o nome do projeto, pra saber o que estou cronometrando sem chegar perto.
- **US-02** — Como usuário, quero que a tela do iPad **nunca durma** enquanto o Modo Foco
  está aberto, pra não perder o relógio no meio do dia.
- **US-03** — Como usuário, quero **pausar/parar de dentro do Modo Foco** com botões
  discretos — e quando pauso (em qualquer device), a tela cheia vira o **Pomodoro de
  pausa** com contagem regressiva gigante.
- **US-04** — Como usuário, quero ver **quanto já trabalhei hoje** num canto discreto,
  sem o Modo Foco virar um dashboard.
- **US-05** — Como usuário com o WS caído (modo degradado), quero que um clique perdedor
  (409) **convirja pra verdade na hora**, em vez de a tela ficar mentindo até o re-sync
  de 60s.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* O Modo Foco DEVE consumir estado **exclusivamente** do `timerStore`
  da fase 03/04 (`useTimerStore`/`useTimerClock`) e mutar **exclusivamente** pelas ações
  existentes do store (`pause`/`resume`/`skip`/`stop`) — nenhuma requisição periódica nova,
  nenhum caminho novo de mutação, nenhum estado de timer paralelo. *(Verificação: revisão
  de imports do `Focus.tsx` no T-06 — só store/queries sancionadas no plan §3, zero
  fetch/WS/interval próprio — + a evidência executável do caso "tick não refaz fetch" do
  T-05.)*
- **CA-02** *(ubíquo)* **Zero mudança de backend**: nenhuma rota, payload ou migração nova.
  As suítes das fases 00–04 seguem verdes **sem edição de asserções**. Toques em arquivo
  compartilhado sancionados (TODOS aditivos — enumeração fechada): `lib/timerStore.ts`
  (caminho de falha do CA-17; nenhum caso existente do spec muda), `styles/tokens.css`
  (tokens novos `--focus-*` do D-03), `lib/format.ts` (função NOVA `fmtDayTotal` —
  nenhuma função existente muda), `routes/cockpit/Focus.tsx` (substitui o StubPage —
  é o entregável da fase; nenhum teste das fases 00–04 pina o stub) e
  `e2e/cronometro.spec.ts` (teste NOVO no arquivo serial — nenhuma asserção existente
  muda). *(Verificação: passo explícito de `git diff` no T-06, precedente do CA-04 da 04.)*
- **CA-03** *(ubíquo)* O Modo Foco DEVE ter **fundo preto independente do tema** (dark e
  light): os tokens `--focus-*` não variam com `[data-theme]`.

### 3.2 Render (state-driven)

- **CA-04** ENQUANTO um timer **countup** roda, `/foco` DEVE exibir os dígitos gigantes
  (mono, tabular) com o **elapsed** derivado localmente (mesma derivação `useTimerClock`
  da fase 03), avançando a cada tick — sem nenhuma requisição por segundo.
- **CA-05** ENQUANTO um timer **countdown/pomodoro** roda, os dígitos DEVEM exibir o
  **restante** (mesma regra `shown` da TimerBar: countup → elapsed; resto → remaining).
- **CA-06** ENQUANTO um timer roda, abaixo dos dígitos DEVE aparecer o **nome do projeto**
  (resolvido inclusive pra projeto arquivado — mesma regra `archived: true` do chip da
  TimerBar); SE a entry não tem projeto, aparece a **descrição**; sem ambos, nada (sem
  placeholder).
- **CA-07** ENQUANTO o estado é pausa (`mode='break'`), o Modo Foco DEVE virar o **modo
  descanso** sem sair da rota: contagem regressiva gigante da pausa + respiro visual
  distinto (accent suave + rótulo "pausa") + controles **retomar / pular / parar**.
  Vale igualmente pra pausa iniciada em OUTRO device (o estado chega pelo sync da 04).
- **CA-08** SE a pausa zera e a origem NÃO é ciclo pomodoro (resume de countup/countdown),
  o modo descanso DEVE exibir "pausa encerrada" e **esperar o usuário** — o auto-resume
  continua sendo SÓ do ciclo pomodoro e mora no `timerStore` (CA-34.1/34.2 da fase 03;
  o Modo Foco não adiciona transição própria).
- **CA-09** ENQUANTO não há timer nem pausa (ocioso), `/foco` DEVE exibir os dígitos
  `0:00:00` esmaecidos + o texto discreto "sem timer rodando" — **sem** controles de start
  (iniciar timer é das outras superfícies; o Modo Foco não vira dashboard).
- **CA-10** O rodapé DEVE exibir o **total do dia** discreto no formato `hoje: 5h12` =
  `total_seconds` das entries fechadas de HOJE (recorte do servidor, tz do navegador —
  mesma fonte da lista Hoje) **+ elapsed vivo da ENTRY RODANDO** (mesma aritmética do
  TodayList: a soma viva só existe quando há entry rodando — em **pausa** o total NÃO
  avança e NÃO inclui o elapsed do break, que não é tempo trabalhado); QUANDO o timer
  para, o total DEVE absorver a entry recém-fechada (sem reload). Nada além do total no
  rodapé.

### 3.3 Controles (event-driven)

- **CA-11** QUANDO o usuário toca **Pausar** (rodando) ou **Parar** no Modo Foco, a mutação
  DEVE sair pela ação correspondente do `timerStore` — e portanto refletir em todos os
  devices (fase 04). Os botões são discretos (ghost), não competem com os dígitos.
- **CA-12** QUANDO o usuário toca **retomar/pular/parar** no modo descanso, valem as ações
  `resume`/`skip`/`stop` do store (mesma semântica do BreakOverlay da fase 03).
- **CA-13** QUANDO o usuário toca o **sair** (✕ discreto), o app DEVE navegar de volta pro
  cockpit (`/hoje`) — e o teardown da rota libera o Wake Lock (CA-14).

### 3.4 Wake Lock (state-driven / unwanted)

- **CA-14** ENQUANTO `/foco` está montada, o app DEVE requisitar
  `navigator.wakeLock.request('screen')`; ao desmontar a rota, DEVE liberar (`release()`)
  o sentinel — sem wake lock zumbi fora do Modo Foco.
- **CA-15** SE o sistema liberar o wake lock sozinho (página escondida/tela bloqueada) e a
  página **voltar a ficar visível** com `/foco` ainda montada, o app DEVE re-adquirir o
  wake lock (listener de `visibilitychange`).
- **CA-16** SE a Wake Lock API não existir (`navigator.wakeLock` indefinido) ou a
  requisição for rejeitada (ex.: economia de bateria), o Modo Foco DEVE funcionar
  normalmente, sem erro visível e sem retry em loop.

### 3.5 Modo degradado (unwanted — CA herdado das sessões 7/8, agora promovido)

- **CA-17** SE uma mutação **manual** de timer falha (ex.: `409` porque o estado mudou em
  outro device com o WS caído), o cliente DEVE re-sincronizar **imediatamente** via
  `GET /timer` — o estado visível converge pra verdade sem esperar o re-sync periódico de
  60s. A correção mora no `timerStore` (ações `start`/`stop`/`pause`/`resume`/`skip`),
  valendo pra TODAS as superfícies (TimerBar, BreakOverlay, Modo Foco) de uma vez.
- **CA-17.1** A falha da mutação manual NÃO DEVE produzir erro intrusivo (sem dialog, sem
  `role=alert`): a convergência silenciosa É o feedback (coerente com a decisão "sync
  invisível" da fase 04). SE o próprio re-sync falhar (offline), o silêncio se mantém —
  o re-sync de 60s/visibilitychange da fase 03 segue sendo a rede de segurança.

### 3.6 E2E (Playwright — fluxo nº3 do TDD.md §4)

- **CA-18** Fluxo nº3 completo num teste: timer rodando com projeto → abre `/foco` →
  **dígitos gigantes + nome do projeto** visíveis → **Pausar** dentro do foco → contagem
  regressiva da pausa **configurada** aparece (modo descanso) → retomar → parar → **sair**
  volta pro cockpit. O rodapé `hoje:` está visível durante o fluxo.

## 4. Requisitos funcionais (resumo)

- **RF-01** Rota `/foco` (já existe no router) vira o Modo Foco real: overlay fullscreen
  (cobre rail + TimerBar) com os estados rodando/descanso/ocioso.
- **RF-02** Hook `useWakeLock()` em `lib/wakeLock.ts` — isolado e testável com
  `navigator.wakeLock` stubado (request/release/re-acquire/sem-suporte).
- **RF-03** Total do dia: mesma fonte da lista Hoje (`listEntries` com tz do navegador →
  `total_seconds`) + elapsed vivo do store; formatador novo `fmtDayTotal` ("5h12").
- **RF-04** `timerStore`: mutações manuais ganham convergência em falha (CA-17) — único
  arquivo da 03/04 tocado; `apply()` segue sendo o único ponto de entrada de estado.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** **zero** dependência nova (JS e Go). Wake Lock e fullscreen são CSS +
  API nativa do browser.
- **RNF-02 Zero distração:** preto real, sem menus, sem navegação visível além do ✕; o
  rodapé é a única informação extra (decisão travada).
- **RNF-03 Sem requisição nova por segundo:** o relógio é 100% derivado localmente
  (fase 03); a única busca nova da rota é a lista de hoje (cacheada pelo TanStack Query,
  mesma chave da página Hoje) e a de projetos (cache idem).
- **RNF-04 PWA/iPad:** funciona em HTTPS (Wake Lock exige secure context — produção já é
  TLS); em browser sem suporte degrada sem erro (CA-16).

## 6. Fora de escopo (fronteiras explícitas)

- **Trocar de task/projeto dentro do Modo Foco** (DESIGN.md §3.2 menciona "toque discreto
  pra trocar de task") — fica pro polish (09) ou fase futura; decisão travada do Diogo
  limitou os controles a pausar/parar/retomar/pular/sair.
- **Iniciar timer de dentro do Modo Foco** — ocioso mostra `0:00:00` + texto; start é das
  outras superfícies (CA-09; "sem virar dashboard").
- **Personalização da aparência do Modo Foco** (estilo dos dígitos, ocultar contexto —
  DESIGN.md §4) — fase de polish.
- **Notificação push / som ao fim da pausa** — herdado como fora de escopo da 04.
- **Fullscreen API do browser** (`requestFullscreen`) — o "fullscreen" é o overlay cobrindo
  o viewport; no iPad o PWA instalado já não tem chrome. Sem API extra.
- **Persistência local do timer / cache de `/api` no SW** — segue PROIBIDO (aviso herdado:
  `offline.spec.ts` depende disso).
- **Indicador de conexão** — segue fora (decisão da 04).
- **`continue` (useContinueEntry) fica FORA do CA-17** — registrado pra não re-litigar:
  não passa pelo store e não tem 409 de corrida (`timer.Start` é troca atômica — CA-05 da
  03 — então no modo degradado o continue VENCE e o `onSuccess` já aplica o Result);
  a falha possível (404 de entry sumida) deixa stale a LISTA, não o timer — o `load()` do
  CA-17 não a corrigiria. Candidato a wrapper se a UI dele mudar em fase futura.
