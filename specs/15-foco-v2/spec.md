# spec.md — Fase 15 · Foco + Pomodoro 2.0

> Fonte de verdade da fase. Estende a fase 05 (Modo Foco): o `/foco` ganha uma **tela de
> entrada pensada pro toque** (FocoPicker), o modo descanso ganha o **visual desenhado**
> (FocoBreak) e o Pomodoro fica **mais personalizável** (ciclos → pausa longa, auto-start,
> alerta sonoro). O ciclo de timer (foco→pausa) ganha a noção de **pausa longa após N focos**.
> Depende de: fase 03 (`timer.Service`, `timerStore`, `pomodoro_settings`) + fase 04 (sync) +
> fase 05 (overlay fullscreen, `useWakeLock`, tokens `--focus-*`).
>
> **Decisões travadas (MILESTONE-1 §4 Fase 15 / §7; este recorte de spec):**
> 1. **FocoPicker** substitui o início ocioso atual do `/foco` (CA-09 da fase 05): campo grande
>    pra digitar + projetos recentes como cartões ≥80px + continuar entrada recente. Toque-first.
> 2. **FocoBreak** segue o design `foco.jsx::FocoBreak` (countdown grande, presets visíveis,
>    retomar/pular/ajustar) — alinhar o overlay de descanso do `/foco` a esse visual.
> 3. **Pomodoro 2.0:** além de `focus_minutes`/`break_minutes`, adiciona **`cycles_before_long`**
>    (focos até a pausa longa), **`long_break_minutes`**, **`auto_start_break`**,
>    **`auto_start_focus`** e **`sound_enabled`** (alerta nativo, sem áudio pesado).
> 4. **Migração `0015_pomodoro_cycles`** (dona desta fase): `ALTER` em `pomodoro_settings` com os
>    campos novos + CHECKs; **defaults que preservam o comportamento atual** da fase 03.
> 5. Wake Lock e o overlay fullscreen da fase 05 **seguem** (não regredir).

---

## 1. Objetivo

Fazer do Modo Foco uma superfície **completa de iPad/celular** — começar uma sessão sem
dropdown apertado, descansar num modo descanso bonito — e dar ao Pomodoro o **ciclo de verdade**
(foco curto ×N → pausa longa) com auto-start opcional e um alerta sonoro discreto no fim de cada
fase. O servidor segue dono do tempo (fase 03); esta fase só estende o que ele já decide.

## 2. User stories

- **US-01** — Como usuário no iPad, quero **começar uma sessão de foco** digitando o que faço e
  tocando num **projeto recente grande** (sem dropdown), pra entrar em foco em 1–2 toques.
- **US-02** — Como usuário, quero **continuar uma entrada recente** direto da tela de entrada do
  Foco, pra repetir trabalho recorrente sem redigitar.
- **US-03** — Como usuário, quero um **modo descanso bonito e calmo** (contagem grande, presets,
  retomar/pular/ajustar) igual ao que foi desenhado, pra a pausa não ser um retângulo seco.
- **US-04** — Como usuário, quero que depois de **N focos** a próxima pausa seja uma **pausa
  longa**, pra ciclar como um Pomodoro de verdade.
- **US-05** — Como usuário, quero ligar/desligar o **auto-start** (a pausa começa sozinha ao fim
  do foco; o foco recomeça sozinho ao fim da pausa), pra escolher entre fluxo automático e
  controle manual.
- **US-06** — Como usuário, quero **personalizar a duração da pausa longa, quantos focos até
  ela** e ligar um **alerta sonoro** no fim de cada fase, nos Ajustes, e que isso me siga entre
  dispositivos.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Backend — Pomodoro 2.0 (`pomodoro_settings` estendida)

- **CA-01** — `GET /api/v1/settings/pomodoro` DEVE devolver os campos novos junto dos
  existentes; **sem linha persistida**, DEVE responder os defaults que **preservam o
  comportamento da fase 03**: `focus_minutes:25`, `break_minutes:5`, `cycles_before_long:4`,
  `long_break_minutes:15`, **`auto_start_break:true`, `auto_start_focus:true`**,
  `sound_enabled:false`. Sem sessão → **401** canônico. *(Furo confirmado: os defaults de
  auto-start são **`true`** — espelham a migração `0015` (`DEFAULT true`) e o D-05 do plan. Um
  default `false` REGREDIRIA o ciclo automático foco↔pausa já entregue e testado da fase 03
  (`timerStore` auto-pausa/auto-resume incondicional, `cycle_test.go`, CA-34/34.1). CA-01 manda;
  migração, `GetSettings` e o teste do T-04 batem nele.)*
- **CA-02** — `PUT /api/v1/settings/pomodoro` com corpo válido (todos os campos) DEVE fazer
  **upsert por usuário** (CA-04 da fase 11 / §8.4) e o `GET` seguinte DEVE retornar exatamente os
  valores salvos (200).
- **CA-02.1** (compat) — um `PUT` legado, só com `{focus_minutes, break_minutes}` (corpo da fase
  03), NÃO DEVE zerar os campos novos: ausência de um campo no JSON significa **manter o valor
  atual persistido** (ou o default, se nunca salvo). (O front da fase 03 que ainda mandasse o
  corpo curto não pode apagar os ciclos.)
- **CA-03** (validação — 400 `validation_error`, sem persistir): `focus_minutes` fora de `[15,90]`;
  `break_minutes` fora de `[1,60]`; `long_break_minutes` fora de `[1,120]`; `cycles_before_long`
  fora de `[1,12]`. Os `CHECK` da migração `0015` são o backstop. (Faixas: §4.)
- **CA-04** (isolamento) — leitura/escrita usam exclusivamente o `user_id` da sessão; a linha de
  outro usuário NUNCA é lida nem sobrescrita.

### 3.2 Backend — ciclo (foco → pausa curta ×N → pausa longa)

> O estado do ciclo (quantos focos completos desde a última pausa longa) é detalhe interno do
> `timer_states` (mesma natureza de `resume_*` da fase 03 — contrato público é o `TimerState`).

- **CA-05** (contador de focos) — QUANDO um **pause** fecha um foco de Pomodoro (`pomodoro_focus`),
  o sistema DEVE **incrementar** o contador de focos completos do ciclo; QUANDO um countup/countdown
  é pausado, o contador NÃO muda (só foco conta pro ciclo).
- **CA-06** (pausa longa após N) — QUANDO o pause de um `pomodoro_focus` faz o contador de focos
  atingir um **múltiplo de `cycles_before_long`**, a pausa iniciada DEVE ser a **pausa longa**
  (`remaining = long_break_minutes × 60`); caso contrário, a **pausa curta** (`break_minutes ×
  60`). O `break_minutes` explícito do payload (CA-08 da fase 03) **vence** essa regra (override
  manual só desta pausa).
- **CA-06.1** (rótulo no estado — PERSISTIDO, não recomputado) — o `TimerState` da pausa DEVE
  expor se é **longa** ou **curta** (campo novo aditivo, ex.: `break_kind: "short"|"long"`), pra a
  UI rotular sem recalcular. O kind é **persistido** no `timer_states` (coluna nova
  `break_is_long boolean`, gravada no `pause`) e **lido** no `GET`/`resume` — **NÃO recomputado**
  por `focus_count % cycles`. *(Furo confirmado: recomputar diverge no override manual que cai num
  múltiplo do ciclo — pause grava `short` (override sempre curta), mas o GET recomputaria `long`
  porque o `focus_count` virou múltiplo. Persistir o flag elimina a divergência; o reset do ciclo
  no resume (CA-07) lê o flag, não recomputa.)* Não altera nenhum campo existente do `State`.
- **CA-07** (reset do ciclo) — QUANDO uma **pausa longa** é encerrada (resume/skip/stop) OU o
  timer é **parado** (`stop` fora de pausa) OU um **novo start** começa do zero, o contador de
  focos do ciclo DEVE **reiniciar em 0**. (Pausa curta encerrada NÃO reseta — o ciclo continua.)
- **CA-08** (ciclo canônico — herda o cuidado da fase 03) — no ciclo automático do US-06 (foco
  roda até o fim → auto-pause → pausa zera → auto-resume), o resume de um `pomodoro_focus`
  completado DEVE **renascer com o alvo cheio** (`focus_minutes × 60`), nunca com alvo 0 (que
  degeneraria em foco-de-1s ↔ pausa-cheia). [Comportamento já provado no `cycle_test.go` da fase
  03 — esta fase **mantém** o invariante e adiciona o teste do ciclo de 4+1 sobre ele.]
- **CA-08.1** (ciclo completo de N+1) — com `cycles_before_long=2`, dois focos completos seguidos
  DEVEM produzir: pausa 1 = **curta**, pausa 2 = **longa**; após retomar da longa, o 3º foco
  começa um ciclo novo (a próxima pausa volta a ser **curta**). Pinado num teste de ciclo
  determinístico (clock fake), espelho do `TestCicloPomodoroCompletoRenasceComFocoCheio`.

### 3.3 Frontend — FocoPicker (tela de entrada do Foco)

- **CA-09** — QUANDO o `/foco` está **ocioso** (sem timer nem pausa — `state === null`), em vez
  do `0:00:00` esmaecido + "sem timer rodando" (CA-09 da fase 05), DEVE renderizar o
  **FocoPicker**: título ("No que você tá trabalhando?"), **campo grande de descrição**, grade
  de **projetos recentes** e seção **Continuar**, e um botão **"Começar a cronometrar"**.
  (A fase 05 dizia "ocioso não vira dashboard / sem start"; **esta fase revê essa fronteira**
  pra `/foco` — desvio sancionado, registrado em §7.)
- **CA-10** (projetos recentes) — o FocoPicker DEVE listar até **5 projetos recentes** como
  **cartões grandes** (alvo de toque **≥80px** de altura), cada um com cor + nome **+ cliente
  quando resolvível** do cache de clientes (`useClients` — query barata já usada pelas páginas de
  projetos; sem cliente resolvido, só cor + nome). *(Furo confirmado: o `Project` do `useProjects`
  só tem `client_id`, não `client_name` — o nome do cliente vem do cache `useClients`, que é
  cacheado/barato e não fere a RNF-03; o cartão degrada pra cor+nome se o cliente não estiver em
  cache.)* Tocar num cartão o **seleciona** (estado visual de selecionado). "Recentes" = projetos
  ativos ordenados por uso recente em entries (fallback: ordem da listagem de projetos quando não
  há histórico; §plan).
- **CA-11** (começar) — QUANDO o usuário toca **"Começar a cronometrar"** com uma descrição
  e/ou um projeto selecionado, o app DEVE chamar **`start`** do `timerStore` (countup, fase 03)
  com `description` + `project_id` selecionado; o estado vira "rodando" e o FocoPicker some (a
  tela de trabalho da fase 05 aparece). Sem descrição **e** sem projeto, o start é permitido
  (CA-20 da fase 03 — tudo opcional), mas o botão pode ficar desabilitado até haver pelo menos um
  dos dois (decisão de UX em §plan; o CA pina **que o start sai com o contexto escolhido**).
- **CA-12** (continuar) — QUANDO o usuário toca um item da seção **Continuar** (entrada recente),
  o app DEVE chamar **`continueEntry`** (fase 03) daquela entry — clona descrição/projeto/task e
  inicia o countup; reflete em todos os devices (fase 04). Lista vazia → a seção Continuar não
  aparece (sem placeholder vazio).
- **CA-13** (toque-first / acessível) — o campo de descrição é um `input`/`textarea` real
  focável; os cartões e itens são `button`s com `aria-pressed`/label; nada depende de hover
  (RNF — toque). Toda cor é token (`--accent*`, `--focus-*`, `--surface-*`) — sem hex literal.

### 3.4 Frontend — FocoBreak (modo descanso redesenhado)

- **CA-14** — ENQUANTO `state.mode === 'break'`, o `/foco` DEVE renderizar o **modo descanso no
  visual desenhado** (`foco.jsx::FocoBreak`): rótulo "modo descanso"/"Pausa Pomodoro", **dígitos
  gigantes** do restante, **presets visíveis** (chips), e controles **pular pausa / retomar /
  ajustar**. O "ajustar" abre os Ajustes do Pomodoro (`navigate('/ajustes')`) — sem editor
  inline novo nesta fase.
- **CA-14.1** (longa × curta) — o modo descanso DEVE indicar quando é **pausa longa** (rótulo
  distinto, ex.: "Pausa longa") usando o `break_kind` do estado (CA-06.1); pausa curta mantém o
  rótulo padrão. O respiro visual de accent suave segue o design (token, não regride o tema).
- **CA-15** (pausa encerrada) — SE a pausa zera e a origem **não** é ciclo pomodoro (resume de
  countup/countdown), o modo descanso DEVE exibir "pausa encerrada" e **esperar o usuário**
  (CA-08 da fase 05 preservado; o auto-resume segue só do ciclo pomodoro, no `timerStore`).
- **CA-16** (BreakOverlay compacto alinhado) — o `BreakOverlay` (modo descanso do cockpit, fora
  do `/foco`) DEVE refletir a noção de pausa longa: rótulo "Pausa longa" quando `break_kind ===
  'long'`. Nenhum comportamento de timer muda — só o rótulo. (Sem isso, o cockpit e o Foco
  divergiriam no nome da pausa.)

### 3.5 Frontend — auto-start (consome o backend; mora no `timerStore`)

- **CA-17** — QUANDO o foco do Pomodoro chega a zero no cliente E `auto_start_break` está
  **ligado**, o front DEVE acionar o auto-pause (CA-34 da fase 03) como hoje; QUANDO está
  **desligado**, o front NÃO DEVE auto-pausar — exibe o estado "foco encerrado, iniciar pausa?"
  e **espera o usuário** tocar (uma ação manual de `pause`). [Hoje o CA-34 sempre auto-pausa;
  esta fase torna isso condicional à preferência.]
- **CA-18** — QUANDO a **pausa** chega a zero E o contexto é `pomodoro_focus` E `auto_start_focus`
  está **ligado**, o front DEVE acionar **exatamente 1** `resume` (CA-34.1 da fase 03); quando
  **desligado**, NÃO retoma sozinho — exibe "pausa encerrada / retomar?" e espera o toque (o
  servidor segue reportando `mode='break'` com `remaining=0`, CA-22.1 da fase 03). A regra do
  countup/countdown (nunca auto-retoma — CA-34.2) é **independente** desta preferência.
- **CA-18.1** (sem regressão da auto-transição) — as auto-transições continuam disparando
  **no máximo 1 vez** por estado (a chave `firedKey` da fase 03 vale): ligar/desligar auto-start
  só **gata** se a ação dispara, nunca quantas vezes. A preferência é lida do mesmo cache do
  `pomodoro_settings` (sem request por tick — RNF).

### 3.6 Frontend — alerta sonoro

- **CA-19** — QUANDO uma fase **termina** (foco→fim ou pausa→fim) E `sound_enabled` está
  **ligado**, o app DEVE emitir um **alerta sonoro curto** (beep nativo via WebAudio, sem arquivo
  de áudio empacotado — RNF leveza); desligado, silêncio. O beep dispara **exatamente 1 vez** por
  transição (atrelado ao mesmo gate da auto-transição / mudança de estado, nunca por tick).
- **CA-19.1** (degrada sem suporte) — SE o WebAudio não existir ou o navegador bloquear áudio sem
  gesto do usuário, o alerta DEVE falhar **silenciosamente** (sem erro visível, sem loop) — o
  Foco funciona normalmente.
- **CA-19.2** (1 beep mesmo com auto-start OFF e re-syncs — furo confirmado) — a `soundedKey` que
  garante "1×/estado" DEVE ser **atrelada à instância do estado** (`stateKey`) e **NÃO ser
  resetada pelo `apply()`** enquanto o `stateKey` não muda. *(O `firedKey` é zerado pelo `apply()`
  a cada resposta; uma `soundedKey` "análoga ao firedKey" re-armaria a cada re-sync (60s) ou
  evento WS — com auto-start **OFF**, a fase fica parada em `remaining=0` e o beep tocaria de novo
  a cada re-sync. A `soundedKey` por instância de estado evita isso.)* QUANDO `auto_start` OFF e
  som ON, uma fase parada em `remaining=0` atravessando vários re-syncs DEVE tocar **exatamente
  uma vez**.

### 3.7 Frontend — Ajustes do Pomodoro 2.0

- **CA-20** — a página **Ajustes** grupo "Pomodoro" DEVE ganhar os controles novos: **ciclos até
  a pausa longa** (1–12), **duração da pausa longa** (1–120 min), **toggle auto-start pausa**,
  **toggle auto-start foco**, **toggle alerta sonoro** — além de foco/pausa que já existem.
- **CA-20.1** (validação no client) — formulário inválido (ex.: pausa custom fora de 1–60, ciclos
  fora de 1–12, pausa longa fora de 1–120) NÃO chama a API e sinaliza o erro (`role=alert`); o
  PUT só sai com o corpo completo e válido (CA-02). Salvar com sucesso reflete na próxima pausa
  (CA-38 da fase 03 estendido).

### 3.8 E2E (Playwright — arquivo serial do timer)

- **CA-21** — Logado, com pausa curta 5min / longa 15min / `cycles_before_long=2` fixados via
  `PUT /settings/pomodoro` pela API: abre `/foco` ocioso → **FocoPicker** visível → digita
  descrição, toca um projeto recente, **Começar** → tela de trabalho (dígitos + contexto) →
  **Pausar** → modo descanso (FocoBreak: dígitos + presets + retomar) → retomar → parar →
  **FocoPicker de volta** → ✕ volta pro cockpit. (Auto-start desligado nesse fluxo pra ser
  determinístico; a regra do ciclo longa×curta é provada no backend, CA-08.1.)

## 4. Requisitos funcionais (faixas e defaults)

- **RF-01** `pomodoro_settings` ganha 5 colunas (migração `0015`), com defaults que **não mudam o
  comportamento da fase 03**:
  - `cycles_before_long integer NOT NULL DEFAULT 4 CHECK (1..12)`
  - `long_break_minutes integer NOT NULL DEFAULT 15 CHECK (1..120)`
  - `auto_start_break boolean NOT NULL DEFAULT true` (o CA-34 da fase 03 passa a respeitar a flag;
    o default **`true`** PRESERVA o auto-pause atual — ver D-05 do plan)
  - `auto_start_focus boolean NOT NULL DEFAULT true` (preserva o auto-resume do ciclo, CA-34.1)
  - `sound_enabled boolean NOT NULL DEFAULT false`
  > **Defaults canônicos (furo confirmado, reconciliado):** os defaults de auto-start são **`true`/
  > `true`** em TODA parte — CA-01, migração `0015` (plan §2), `GetSettings` e o teste do T-04.
  > Default `false` regrediria o ciclo automático foco↔pausa já entregue e testado na fase 03
  > (`cycle_test.go`, CA-34/34.1). O usuário desliga nos Ajustes se quiser controle manual.
- **RF-02** Ciclo: o `timer_states` ganha `focus_count integer NOT NULL DEFAULT 0` (focos
  completos desde a última pausa longa/reset). Incrementa no pause de foco (CA-05); a pausa longa
  dispara em `focus_count % cycles_before_long == 0` (CA-06); reset em pausa longa encerrada /
  stop / start (CA-07).
- **RF-03** FocoPicker: `description` (input grande) + seleção de projeto recente + continuar →
  `timerStore.start`/`continueEntry`. Sem modo countdown/pomodoro escolhível no picker (countup;
  o Pomodoro segue acionável pelas outras superfícies — recorte desta fase, §7).
- **RF-04** FocoBreak: o overlay de descanso do `/foco` adota o layout do mockup; usa só estado +
  ações do `timerStore` (`resume`/`skip`/`stop`) + `navigate('/ajustes')` no "ajustar".
- **RF-05** Auto-start + som: lidos do `usePomodoroSettings` (cache TanStack), aplicados no gate
  de auto-transição do `timerStore` (auto-pause/auto-resume condicionais) e num side-effect de
  som no fim de fase. Nenhum request por tick.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** **zero** dependência nova (JS e Go). Som = WebAudio nativo (oscillator
  curto), **sem** arquivo/biblioteca de áudio empacotada. Migração = `ALTER TABLE` simples.
- **RNF-02 Tokens:** toda cor via `tokens.css` (gate `check-cores`); o FocoPicker e o FocoBreak
  herdam os `--focus-*` da fase 05 (só no `:root`, sem tema — CA-03 da 05 preservado) + `--accent*`.
  Cor nova só como token, se necessário.
- **RNF-03 Clock 100% local:** os dígitos do FocoBreak/trabalho seguem derivados de
  `useTimerClock` (fase 03/05) — **sem** requisição por segundo. O FocoPicker só usa as queries
  já cacheadas (entries de hoje, projetos).
- **RNF-04 Sem novo paralelo de estado de timer:** auto-start e som moram no `timerStore` /
  num hook que consome o store — nenhum caminho novo de mutação de timer, nenhum interval novo
  (reusa o tick de 1s da fase 03).
- **RNF-05 PWA/iPad:** Wake Lock e overlay fullscreen da fase 05 seguem; o som precisa de gesto do
  usuário em alguns browsers — degrada sem erro (CA-19.1).
- **RNF-06 Backend pronto pra fase 04:** o `break_kind` e o ciclo entram no mesmo `TimerState`
  canônico — o broadcast da fase 04 carrega de graça (nenhuma refatoração; `onChange` intocado).
- **RNF-07 Sem regressão:** as suítes das fases 03/04/05/11 seguem verdes; os campos novos do
  `State`/`Settings` são **aditivos** (nenhum campo existente muda de tipo/semântica).

## 6. Mapa pro problema central (MILESTONE-1 §4 / REQUISITOS §4.4)

- Resolve **P5** do MILESTONE-1 ("Modo Foco ruim, Pomodoro pouco personalizável"): entrega o
  FocoPicker, o FocoBreak desenhado e o Pomodoro com ciclos/auto-start/som.
- Estende a fase 05 (Modo Foco) e a fase 03 (Pomodoro) sem reescrevê-las.

## 7. Fora de escopo (fronteiras explícitas)

- **Escolher modo countdown/pomodoro no FocoPicker** — o picker inicia countup; Pomodoro segue
  acionável pela TimerBar/Ajustes (manter o picker simples; candidato a evolução).
- **Trocar de task/projeto NO MEIO de uma sessão de foco** (o "trocar task" do mockup `FocoWork`)
  — segue fora (era fora na fase 05 §6); o FocoPicker só atua no **início** da sessão.
- **Biblioteca de sons / sons customizáveis / escolher o timbre** — fora: só um beep nativo
  liga/desliga (RNF-01). Som por arquivo empacotado é evolução.
- **Notificação push** no fim do Pomodoro — fica pra fase 16 (anti-timer-esquecido, web push).
- **Temas de cor do Foco / personalizar a aparência do Foco** (estilo dos dígitos) — fora (era
  fora na fase 05 §6).
- **Estatística de ciclos / histórico de Pomodoros** (quantos focos hoje etc.) — fora; o
  `focus_count` é estado de runtime, não métrica persistida.
- **Desvio sancionado da fase 05 (CA-09):** a fase 05 fixou "ocioso = `0:00:00` + texto, sem
  start, sem virar dashboard". Esta fase **substitui o ocioso pelo FocoPicker** (com start) — o
  Diogo pediu o picker no `/foco` (MILESTONE §4). O spec `Focus.spec.tsx` da fase 05 que assere o
  estado ocioso antigo (CA-09) será **reescrito** pro FocoPicker (a fidelidade dos outros estados
  — rodando/descanso/rodapé/wake lock — segue assertada). Registrado como desvio (espelha o D-09
  da fase 11: reescrever spec de fase anterior quando o layout muda, mantendo o contrato de
  dados/comportamento). O E2E da fase 05 (fluxo nº3 no `cronometro.spec`) que passava pelo ocioso
  é atualizado pra abrir via FocoPicker.
