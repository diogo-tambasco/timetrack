# spec.md — Fase 22 · Mobile responsivo (base)

> Fonte de verdade da fase. Resolve a dor **P6** do `MILESTONE-2.md §2/§4` ("no celular o app é
> inutilizável"). O app é desktop-first denso e **zero responsivo** (confirmado nesta sessão:
> `grep -rE 'sm:|md:|lg:|xl:|@media' frontend/src` só acha as definições em `styles/`, **nenhum
> componente** usa breakpoint; nenhuma `@media`). A fase 22 é a **camada de layout/responsividade**
> que torna o app utilizável no toque a **375px** — sem migração, sem backend, **sem mudança de
> dados ou comportamento**.
>
> **Decisões travadas (HITL `MILESTONE-2.md §7.7 / §9`):**
> 1. **Mobile = híbrido**, dissecado em **22 (responsivo base, esta)** + **23 (controle remoto
>    dedicado)**. A 22 não cria telas novas — só adapta as existentes.
> 2. **Puro layout:** breakpoints Tailwind nos componentes que já existem; **comportamento, dados
>    e contratos idênticos** ao desktop (CA de não-regressão — §4).
> 3. **Cross-cutting, NÃO paralelizável** (toca quase todo o front) — buildar **depois** das fases
>    de backend (17–21) e **antes** da 23. A 23 depende desta base.
> 4. **Mantém a linguagem visual** (tokens do `DESIGN.md`, densidade). Responsividade **não
>    introduz cor** — `scripts/check-cores.sh` (tokens.css única fonte de cor) **segue valendo**.

---

## 1. Objetivo

Tornar o cockpit **utilizável no toque/celular** a partir de **375px** de largura (iPhone SE / base
de mercado), sem perder nada do desktop. Hoje:

- O **rail** é `width: 64px` fixo (`.tt-sidenav`, tokens.css:190) com hover-expand pra 232px — em
  touch não há hover; a nav é **inalcançável** com confiança e os rótulos nunca aparecem.
- Páginas usam **grades multi-coluna inline** que não colapsam (ex.: `Dashboard.tsx`
  `gridTemplateColumns: 'minmax(0, 1.6fr) minmax(0, 1fr)'`), **tabelas densas** (`.tt-table`,
  `ClientsTable`, `ProjectsTable`, `ReportTable`) que estouram a viewport, e modais/grids largos.
- A **TimerBar** global (`height: 66`, input + selects + botões em linha) **estoura** num celular.

A entrega é **aditiva no markup/estilo**: cada superfície ganha um comportamento mobile **abaixo de
um breakpoint canônico** e segue **idêntica** acima dele. Nenhuma query, rota, store, validação ou
fluxo muda — é **camada de apresentação**.

## 2. Definições precisas

- **Breakpoint canônico `md` = 768px.** Abaixo de 768px = **modo mobile** (nav mobile, 1 coluna,
  tabelas→cartões, TimerBar enxuta). De 768px pra cima = **layout desktop atual, byte-compatível**.
  Onde o Tailwind for usado, `md:` (≥768px) traz o layout desktop; o default (sem prefixo) é o
  mobile (mobile-first). O número 768 é **único e consistente** em toda a fase (CA-11). Onde
  precisar de um degrau extra (ex.: grid 3→2→1 col), usar `lg` = 1024px como segundo passo — mas o
  **divisor mobile↔desktop é sempre 768**.
- **"Utilizável a 375px"** = numa viewport de 375×667: (a) **zero overflow horizontal** do
  documento (`document.scrollWidth <= viewport.width`); (b) a **navegação é alcançável** sem hover
  (a nav mobile visível, todas as rotas-chave acionáveis por toque); (c) as **ações principais** de
  cada página são alcançáveis (criar/iniciar/abrir); (d) **touch targets ≥ 40px** nos controles
  primários de nav/ação (alvo confortável de toque — não bloqueia o build se um secundário ficar
  menor, mas os primários da nav e a TimerBar respeitam).
- **Modo mobile da nav:** o rail de 64px **some** (`display:none` abaixo de `md`) e dá lugar a uma
  **navegação mobile** — bottom bar fixa (decisão de implementação no plan, §3) com as rotas-chave +
  acesso ao resto. O `<main>` ganha respiro inferior pra não ficar sob a bar fixa.
- **"Tabela vira cartão":** abaixo de `md`, as **tabelas densas** (que dependem de N colunas lado a
  lado) renderizam como **lista de cartões empilhados** (1 por linha, label:valor) OU recebem scroll
  horizontal contido **sem estourar a página** (decisão por componente no plan; o relatório
  vira **cartões** — CA-07). Acima de `md`, a tabela é a de hoje, intocada.

## 3. User stories

- **US-01** — Como usuário no **celular**, quero **navegar entre as telas** (Dashboard, Projetos,
  Relatórios, Foco, etc.) sem depender de hover, pra usar o app no toque.
- **US-02** — Como usuário no celular, quero que **cada página caiba na tela** (sem rolar pro lado,
  sem texto cortado), pra ler e agir confortavelmente.
- **US-03** — Como usuário no celular, quero **iniciar/parar o timer e trocar o projeto** pela
  TimerBar sem ela estourar a largura, pra controlar o cronômetro no bolso.
- **US-04** — Como usuário, quero que **no desktop nada mude** — mesmos layouts, mesma densidade,
  mesmos dados — pra a melhoria do mobile não regredir o que já uso.
- **US-05** — Como usuário no celular, quero ver **relatórios e listas densas** em formato legível
  (cartões empilhados em vez de uma tabela espremida), pra não precisar de zoom.

## 4. Critérios de aceite (EARS — cada um vira teste)

> Convenção: testes de componente/página = **Vitest + Testing Library** (jsdom, com viewport/matchMedia
> mockados onde o layout depende do breakpoint); o **E2E mobile** = **Playwright** com viewport 375×667
> (CA-20). Onde o markup mudar (classes/elementos novos), os **specs antigos seguem verdes** ajustando
> **só** o seletor que a mudança de DOM exigir (documentado — CA-18).

### Navegação mobile (rail → nav mobile)

- **CA-01** (nav mobile aparece abaixo de `md`) — abaixo de **768px**, uma **navegação mobile**
  (bottom bar) DEVE estar **visível e acionável por toque**, expondo as rotas-chave (`/`, `/projetos`,
  `/relatorios`, `/foco`, `/ajustes` no mínimo) como links navegáveis. Cada item leva à rota certa
  (mesmos `to` do `SideNav` atual). Em **≥768px** a nav mobile NÃO aparece.
- **CA-02** (rail some no mobile) — abaixo de **768px**, o rail `.tt-sidenav` (64px, hover-expand)
  DEVE estar **oculto** (não ocupar espaço nem capturar toque). Em **≥768px** o rail aparece
  **idêntico ao de hoje** (64px, hover→232px overlay) — sem regressão do comportamento canônico.
- **CA-03** (rota ativa na nav mobile) — a nav mobile DEVE marcar o item **ativo** pela rota atual,
  com a **mesma regra** do `SideNav` (`pathname === to || pathname.startsWith(to + '/')`; o item `/`
  casa só com `pathname === '/'`, nunca como prefixo de `/projetos`). Trocar de rota atualiza o ativo.
- **CA-04** (acesso ao "Sair"/conta no mobile) — o **logout** (e o nome do usuário) DEVE permanecer
  alcançável no mobile (o `AvatarMenu` de hoje vive no rail oculto): exposto numa entrada da nav
  mobile (ex.: "Ajustes"/menu) ou na própria bottom bar. A ação `signOut → /login` é **idêntica**
  (nenhuma chamada nova).

### TimerBar adaptada

- **CA-05** (TimerBar não estoura) — abaixo de **768px**, a **TimerBar** global DEVE caber na
  largura (sem overflow horizontal): os controles (descrição, projeto, iniciar/parar/pausar)
  reflowam/encolhem (ex.: selects compactos, input com `min-width:0`, esconder rótulos secundários)
  **sem remover** nenhuma ação. Iniciar, parar, pausar e trocar de projeto seguem acionáveis no
  toque. O **estado e o comportamento do timer não mudam** (mesmo `timerStore`).
- **CA-06** (BreakOverlay no mobile) — quando em pausa, o **BreakOverlay** (variante de pausa da
  TimerBar) DEVE caber em 375px (countdown + ações retomar/pular/parar acionáveis), sem overflow.

### Páginas-chave utilizáveis a 375px

> **Onde "overflow" é assertado (FURO 1 — HIGH):** "zero overflow horizontal" é **medível só com
> layout engine** — Vitest+jsdom **não** mede `scrollWidth` (sem motor de layout), então **NÃO**
> serve pra esse assert. Por isso a verificação de overflow a 375px de **CADA** página-chave é feita
> no **E2E mobile** (Playwright, viewport 375×667 — CA-19, que varre `/`, `/projetos`, `/projetos/:id`,
> `/clientes`, `/relatorios`, `/foco`, `/workspace`, `/calendario`, `/ajustes`). Os CAs CA-07..CA-10
> abaixo, testados em **Vitest+jsdom**, cobrem **só a ESTRUTURA** (grade multi-col → 1 coluna; tabela
> → cartões label:valor; modal full-width) — **nunca** "não estoura"/"cabe", que é responsabilidade
> do CA-19. Os dados/estados (loading/erro/vazio) são os de hoje.

- **CA-07** (Dashboard — estrutura 1-col) — abaixo de `md`, a home (`/`) DEVE renderizar a grade
  `minmax(0,1.6fr) minmax(0,1fr)` colapsada pra **1 coluna** (TodayList/Recentes acima, WeekGoals +
  atalho Foco abaixo); o conteúdo é o mesmo (mesmas seções, mesma ordem lógica), com o `TimerCard` e
  o cabeçalho (saudação + "Nova entrada") presentes. (Teste = Vitest+jsdom: estrutura 1-col abaixo de
  `md`, 2-col ≥ `md`. **Overflow a 375px da `/` é assertado no CA-19**, não aqui.)
- **CA-08** (Relatórios 2.0 — tabela vira cartões) — abaixo de `md`, `/relatorios` DEVE renderizar a
  **`ReportTable`** (colunas por projeto/objetivo) como **cartões empilhados** (1 por linha,
  label:valor, **todos os dados**) e os gráficos (`TrackedDonut`/`FocusByHour`/`ProjectBars`) +
  `PeriodNav`/`KpiStrip`/filtros reflowados pra **1 coluna**. O **toggle Gráficos↔Tabela** segue
  funcionando; os números são idênticos aos da tabela desktop. (Teste = Vitest+jsdom: tabela→cartões
  e grids 1-col abaixo de `md`. **Overflow a 375px de `/relatorios` é assertado no CA-19**, não aqui.)
- **CA-09** (Tarefas/board + Projetos — estrutura) — abaixo de `md`, `/projetos`, `/projetos/:id`
  (board) e `/clientes` DEVEM renderizar: as **colunas do board** (`TaskBoardColumns`, hoje lado a
  lado) reflowadas pra **1 coluna** **OU** num container de scroll horizontal contido; `ProjectsTable`/
  `ClientsTable` como **cartões** (ou container de scroll contido); os modais (`ProjectModal`/
  `TaskModal`/`ObjectiveModal`/`EntryModal`) **full-width** (sem cortar campos/botões). O `ViewToggle`
  cards↔tabela segue. (Teste = Vitest+jsdom: board 1-col/scroll-contido, tabela→cartões, modal
  full-width abaixo de `md`. **Overflow a 375px de `/projetos`, `/projetos/:id` e `/clientes` é
  assertado no CA-19**, não aqui.)
- **CA-10** (Ajustes · Workspace · Calendário — estrutura) — abaixo de `md`, `/ajustes`, `/workspace`
  e `/calendario` DEVEM renderizar: os grupos de Ajustes empilhados em **1-col**; a lista de membros
  do Workspace empilhada; o `/calendario` (grade `TimeGrid`/`MonthGrid` densa) com a faixa de horas
  rolando **dentro do próprio container** (scroll **contido**, não no documento — a grade já tem
  container próprio; a fase garante que o **shell** não vaza). Nenhuma preferência/dado de calendário
  muda. (Teste = Vitest+jsdom: grupos/membros em 1-col, scroll contido no container da grade.
  **Overflow a 375px de `/ajustes`, `/workspace` e `/calendario` é assertado no CA-19**, não aqui.)

### Consistência do breakpoint

- **CA-11** (breakpoint único) — **todo** o responsivo da fase usa **um divisor mobile↔desktop
  canônico = 768px** (`md`): a nav mobile aparece, o rail some, grids colapsam e tabelas viram
  cartões **no mesmo limiar**. Um segundo degrau (`lg`=1024) é permitido só pra refinar grids
  multi-passo, nunca pra mover o limiar mobile↔desktop. (Teste: o ponto onde a nav mobile aparece e
  o rail some é o mesmo; nenhum componente da fase usa um breakpoint mobile↔desktop diferente de
  768.)

### Não-regressão (cross-cutting — o coração do risco)

- **CA-12** (comportamento e dados idênticos) — a fase **NÃO** altera nenhuma query, mutação, rota,
  store, validação, contrato de API ou fluxo de interação. As mesmas ações produzem os mesmos
  efeitos; a única diferença é **layout por largura de viewport**. (Teste: as suítes de
  comportamento existentes — criar projeto, iniciar timer, editar entry, filtrar relatório, gerir
  membros — seguem **verdes sem mudar asserção de comportamento**; só seletores de DOM se a marcação
  mudou.)
- **CA-13** (desktop byte-compatível ≥768px) — em **≥768px**, todas as páginas renderizam o
  **layout de hoje** (rail 64px hover-expand, grids multi-coluna, tabelas, TimerBar em linha) sem
  diferença visual/estrutural perceptível. A camada mobile só atua **abaixo** do breakpoint. (Teste:
  numa viewport desktop, o rail está presente e a nav mobile ausente — o inverso do CA-01/02.)
- **CA-14** (tokens/cor intactos) — a responsividade **não introduz nenhuma cor nova**: `scripts/
  check-cores.sh` segue **verde** (zero hex literal fora de `tokens.css`/`palette.ts`/specs). Novas
  regras de layout usam tokens existentes (`--pad`, `--gap`, `--surface-*`, etc.); nenhum hex novo.
- **CA-15** (densidade preservada) — a linguagem visual do `DESIGN.md` se mantém: o mobile reusa os
  tokens de densidade (`--pad`/`--gap`/`--row-h`); o toggle de densidade comfortable/compact segue
  funcionando; o mobile **não** sobrescreve as cores nem as fontes (Geist).
- **CA-16** (Modo Foco intacto) — o **Modo Foco** (`/foco`, overlay fullscreen preto, fase 05) já é
  fullscreen e mobile-amigável; a fase 22 **NÃO o altera** (confirmar que segue ok a 375px: dígitos
  gigantes, ✕, modo descanso — sem overflow). Nenhuma classe/token de `--focus-*` muda.
- **CA-17** (PWA/offline intacto) — o service worker, o precache, o `navigateFallback` e a denylist
  (`/api|/auth|/healthz`) **não mudam** (a fase é só CSS/markup do app, não toca `sw.ts`/
  `vite.config.ts`). `offline.spec` e o check-pwa seguem verdes.
- **CA-18** (suítes antigas verdes com ajuste mínimo de seletor) — onde a marcação mudar (ex.:
  bottom bar nova, classes responsivas, wrapper de cartão), os specs Vitest/Playwright existentes
  são ajustados **só** no seletor afetado, **documentando** cada ajuste; **nenhuma** asserção de
  comportamento/dado é enfraquecida.

### E2E mobile (Playwright)

- **CA-19** (rotas-chave a 375px sem overflow + nav presente) — Logado (stub), numa viewport
  **375×667**, ao percorrer **TODAS** as rotas-chave — incluindo as mais densas/arriscadas:
  `/` → `/projetos` → **`/projetos/:id` (board)** → **`/clientes`** → `/relatorios` →
  **`/calendario`** → `/foco` → **`/workspace`** → `/ajustes`: em **cada** rota o **documento não
  tem overflow horizontal** (`document.documentElement.scrollWidth <= document.documentElement.clientWidth`)
  e a **nav mobile está presente** (nas rotas onde o shell a monta — o `/foco` é overlay fullscreen,
  CA-16). A navegação pela **nav mobile** leva às rotas certas (URL muda); `/projetos/:id` é alcançado
  abrindo um projeto a partir de `/projetos`. **Este CA é a fonte de verdade do "zero overflow a
  375px"** de toda página-chave (os CA-07..CA-10 cobrem só estrutura — Vitest/jsdom não mede
  overflow). (Teste = `e2e/mobile.spec.ts`, Playwright 375×667.)
- **CA-20** (ação principal alcançável + desktop sem nav mobile) — Logado (stub) a 375×667, a **ação
  principal** de uma página-chave é alcançável por toque (ex.: abrir "Nova entrada" no Dashboard, ou
  iniciar um timer pela TimerBar) **sem** estourar a tela; e — controle — numa viewport **desktop**
  (ex.: 1280×800) a **nav mobile NÃO aparece** e o **rail está presente** (CA-13). Mesmo usuário,
  mesma sessão; só a largura muda o layout.

## 5. Requisitos não-funcionais

- **RNF-01 (puro layout, zero backend)** — **nenhuma** migração, rota, handler, store, query ou
  validação muda. A fase toca **só** `frontend/src/**` (CSS/tokens + markup/classes Tailwind +
  componente de nav mobile). `backend/**` fica **byte-idêntico**.
- **RNF-02 (mobile-first, sem regressão desktop)** — o desktop (≥768px) é o **layout atual**; a
  camada mobile atua **abaixo** do breakpoint. Onde Tailwind for usado, default = mobile, `md:` =
  desktop. Estilos inline existentes que quebram no mobile (grids fixas) migram pra classes/util
  responsivas **sem** mudar o resultado desktop.
- **RNF-03 (tokens = única fonte de cor)** — `check-cores.sh` verde; nenhuma cor hex nova; densidade
  via tokens (`--pad`/`--gap`/`--row-h`). A linguagem visual do `DESIGN.md` é preservada (CA-14/15).
- **RNF-04 (acessibilidade de toque)** — controles **primários** de nav e da TimerBar com alvo de
  toque **≥40px**; a nav mobile é navegável por teclado (links/botões reais, `aria-current` no item
  ativo). Sem `hover`-only pra ações essenciais no mobile.
- **RNF-05 (não-regressão das fases existentes)** — as suítes das fases 00–21 (Vitest + Playwright
  desktop) seguem **verdes**; ajustes só de seletor de DOM, documentados (CA-18). `make check-full`
  é o gate (Go test **inalterado** + Vitest + Playwright + builds).
- **RNF-06 (PWA intacto)** — instalável + precache/offline atuais preservados; `sw.ts`/`vite.config`
  intocados (CA-17).

## 6. Pré-condições externas

Nenhuma. A fase é frontend puro; não exige credencial, chave, migração nem ação de terceiro. Não há
entrada nova no `PENDENCIAS-DIOGO.md`.

## 7. Fora de escopo (fronteiras explícitas)

- **Telas mobile dedicadas** além do responsivo — o **controle remoto** (start/stop enxuto
  mobile-first) é a **fase 23**, não esta. A 22 só adapta as telas que já existem.
- **Board mobile-nativo / gestos de arrastar otimizados pra toque** — fora; o board reflowa/scrolla,
  mas não ganha UX de drag-and-drop nova pra touch (decisão `MILESTONE-2.md §3 "Fora da M2"`).
- **Mudança de informação por tamanho de tela** (esconder dados, resumir conteúdo diferente do
  desktop) — fora; o mobile mostra a **mesma** informação, só **reorganizada** (1 coluna, cartões).
  Onde uma coluna secundária de tabela for ocultada visualmente no cartão, o **dado** segue presente
  (no cartão como label:valor) — nada é removido do DOM acessível.
- **Novos breakpoints além de `md`(768)/`lg`(1024)** ou um design system mobile separado — fora; a
  fase reusa os tokens e a linguagem atuais.
- **Modo Foco** — já fullscreen; a fase só **confirma** que segue ok (CA-16), não redesenha.
- **PWA / service worker / offline** — intocados (CA-17/RNF-06).
- **Backend, migração, dados** — nada (RNF-01). 21/22/23 são **sem migração** (`MILESTONE-2.md §4`).
