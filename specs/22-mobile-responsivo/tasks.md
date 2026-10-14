# tasks.md — Fase 22 · Mobile responsivo (base)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-12).
> Cada T-XX lista os CAs cobertos. **Frontend puro** (RNF-01): nenhum arquivo de `backend/**`,
> nenhuma migração. Onde o DOM mudar, ajustar **só** o seletor dos specs antigos, documentando
> (CA-18) — nenhuma asserção de comportamento enfraquecida.
> **Breakpoint canônico = `md` (768px)** em TODA a fase (CA-11).

---

## Fundação responsiva

- **T-01 — Breakpoints + `useMediaQuery` + extração de `navItems`** (CA-11, base p/ D-01/02/05)
  - Confirmar que o Tailwind v4 do projeto expõe `sm`(640)/`md`(768)/`lg`(1024) padrão (sem config
    custom hoje); fixar **768 como divisor mobile↔desktop** em toda a fase. `lib/useMediaQuery.ts`
    (NOVO, sem dep — `window.matchMedia` + listener, SSR-safe default). `components/nav/navItems.ts`
    (NOVO): a **única** lista de itens de nav (to/label/ícone), extraída do `SideNav` atual; `SideNav`
    passa a consumi-la **renderizando idêntico** (refactor sem mudança de comportamento desktop).
  - Acceptance: `useMediaQuery.spec.ts` (matchMedia mockado: true→match, false→não; reage a change);
    `SideNav` segue renderizando os mesmos itens/rotas (specs do shell/`tema`/nav existentes verdes);
    `navItems` é a fonte única (importado por SideNav). `check-cores` verde (sem cor nova).

## Navegação mobile (rail → bottom bar)

- **T-02 — `MobileNav` (bottom bar) + rail oculto + wiring no shell** (CA-01/02/03/04, RNF-04)
  - `components/nav/MobileNav.tsx` (NOVO): bottom bar **fixa** (`position:fixed; bottom:0`) visível
    **abaixo de md** (`md:hidden`), oculta no desktop; itens de `navItems.ts` (rotas-chave `/`,
    `/projetos`, `/relatorios`, `/foco` + "Mais"/menu com `/clientes`,`/workspace`,`/calendario`,
    `/ajustes`,**Sair**). Ativo pela regra do SideNav (`=== to || startsWith(to+'/')`, `/` só `==='/'`),
    `aria-current` no ativo; alvos ≥40px. `SideNav` ganha gate `hidden md:flex` (rail some < md;
    **conteúdo/hover-expand do rail intocado ≥ md** — CA-02/13). `AppShell` monta `<MobileNav>`;
    `<main>` ganha `padding-bottom` abaixo de md (altura da bar + `env(safe-area-inset-bottom)`).
    "Sair" reusa `useAuthStore.signOut → navigate('/login')` (idêntico — CA-04).
  - `MobileNav.spec.tsx` (NOVO, jsdom + MemoryRouter): itens levam às rotas certas; item ativo casa a
    rota (incl. `/` ≠ prefixo de `/projetos`); "Sair" chama `signOut` (sem chamada de API nova);
    a bar tem `role="navigation"`/`data-testid="mobile-nav"`. Specs do shell ajustados **só** no
    seletor onde a bottom bar entrou (documentar — CA-18).

## TimerBar / pausa

- **T-03 — TimerBar + BreakOverlay responsivos** (CA-05/06, CA-12)
  - CSS/markup responsivo abaixo de md: controles reflowam/encolhem (input `min-width:0`, selects
    compactos, esconder rótulo secundário do select de task), **nenhuma ação removida** (iniciar/
    parar/pausar/trocar projeto seguem); `overflow` contido (a barra nunca empurra a largura do
    documento). `BreakOverlay` cabe a 375px (countdown + retomar/pular/parar). **`timerStore` e
    comportamento intocados** (CA-12).
  - Acceptance: `TimerBar.spec`/`BreakOverlay.spec` existentes verdes (só seletor se mudou); um teste
    novo afirma que, com viewport mobile (matchMedia mock), todas as ações seguem no DOM e a barra
    não declara largura fixa que estoure (smoke de markup). Comportamento do timer **não** muda.

## Páginas-chave (grids 1-col · tabela→cartões)

> **FURO 1 (HIGH):** os testes de T-04..T-07 são **Vitest+jsdom** → cobrem **só ESTRUTURA**
> (1-col / tabela→cartões / modal full-width). **"Zero overflow"/"não estoura"/"cabe" NÃO é
> assertado aqui** (jsdom não mede `scrollWidth`); é exclusivo do **E2E mobile (T-10/CA-19)**, que
> varre `/`, `/projetos`, `/projetos/:id`, `/clientes`, `/relatorios`, `/calendario`, `/foco`,
> `/workspace`, `/ajustes`.

- **T-04 — Dashboard 1-col abaixo de md** (CA-07, CA-12/13; overflow no T-10/CA-19)
  - Trocar a grade inline `gridTemplateColumns: 'minmax(0,1.6fr) minmax(0,1fr)'` por classe
    responsiva (`grid-cols-1 md:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]` ou util `@media`):
    **1 col < md**, **idêntico ≥ md**. `TimerCard` e cabeçalho (saudação + "Nova entrada") presentes.
    Conteúdo/ordem lógica iguais.
  - Acceptance: `Dashboard.spec` existente verde (comportamento intocado); teste novo Vitest
    (estrutura): render mobile (matchMedia mock) → uma coluna (sem a 2ª coluna lado a lado); render
    desktop → layout 2-col atual. (**Overflow a 375px da `/` = T-10/CA-19**, não aqui.)

- **T-05 — Relatórios 2.0: ReportTable → cartões + grids 1-col** (CA-08, CA-12; overflow no T-10/CA-19)
  - `ReportTable` ganha **modo cartão** abaixo de md (lista label:valor, **todos os dados**, 1 por
    linha) via `useMediaQuery`/CSS; `PeriodNav`/`KpiStrip`/filtros/`TrackedDonut`/`FocusByHour`/
    `ProjectBars` reflowam 1-col. Toggle Gráficos↔Tabela segue; números idênticos.
  - Acceptance: `Reports.spec` existente verde; teste novo Vitest (estrutura): viewport mobile →
    `ReportTable` renderiza como cartões (mesmos valores que a tabela desktop) e grids em 1-col;
    desktop → tabela atual. (**Overflow a 375px de `/relatorios` = T-10/CA-19**, não aqui.)

- **T-06 — Board + Projetos + Clientes** (CA-09, CA-12; overflow no T-10/CA-19)
  - `TaskBoardColumns`: abaixo de md, colunas reflowam pra 1 col **ou** scroll horizontal **contido**.
    `ProjectsTable`/`ClientsTable` → cartões (ou scroll contido) < md. `ViewToggle` cards↔tabela
    intocado. Modais (`ProjectModal`/`TaskModal`/`ObjectiveModal`/`EntryModal`) full-width < md
    (campos/botões inteiros).
  - Acceptance: specs de board/projetos/clientes existentes verdes (comportamento intocado); teste
    novo Vitest (estrutura): viewport mobile → board em 1-col ou container de scroll contido, tabela
    vira cartões, modal full-width. Desktop intocado. (**Overflow a 375px de `/projetos`,
    `/projetos/:id` e `/clientes` = T-10/CA-19**, não aqui.)

- **T-07 — Ajustes · Workspace · Calendário** (CA-10, CA-12; overflow no T-10/CA-19)
  - Grupos de Ajustes empilham 1-col < md; lista de membros do Workspace empilha; `/calendario`
    (`TimeGrid`/`MonthGrid`) rola **contido** no container próprio da grade (o shell não estoura) < md.
    Nenhuma preferência/dado de calendário muda.
  - Acceptance: specs de Settings/Workspace/Calendar existentes verdes; teste novo Vitest (estrutura):
    viewport mobile → grupos/membros em 1-col; calendário com o container da grade marcado pra scroll
    contido. (**Overflow a 375px de `/ajustes`, `/workspace` e `/calendario` = T-10/CA-19**, não aqui.)

## Não-regressão (cross-cutting)

- **T-08 — Foco + PWA + cor/densidade intactos** (CA-14/15/16/17, RNF-03/06)
  - Confirmar (sem editar) que `/foco`/`--focus-*` (fase 05) cabe a 375px e **não** mudou (CA-16);
    `sw.ts`/`vite.config`/PWA **não** tocados (CA-17 — `offline.spec`/check-pwa verdes); `check-cores`
    verde (CA-14 — zero hex novo); densidade comfortable/compact segue (CA-15).
  - Acceptance: `offline.spec` + check-pwa verdes; `check-cores.sh` OK; `Focus.spec` existente verde
    (intocado); `tema.spec`/densidade verdes. Um teste novo afirma `/foco` sem overflow a 375px (ou
    cobre no E2E mobile da T-10).

- **T-09 — Varredura de não-regressão das suítes antigas** (CA-12/18, RNF-05)
  - Rodar **toda** a suíte Vitest + Playwright desktop; ajustar **só** seletores de DOM que a bottom
    bar/classes responsivas exigirem (documentar cada ajuste no commit — CA-18). **Nenhuma** asserção
    de comportamento/dado enfraquecida. `go test ./...` **inalterado** (RNF-01 — nem roda diff de
    backend; só confirmar verde).
  - Acceptance: Vitest 100% verde, Playwright desktop 100% verde, Go test verde (intocado). Lista dos
    seletores ajustados registrada no commit.

## E2E mobile + gate

- **T-10 — E2E `mobile.spec.ts`** (CA-19/20 — FONTE DE VERDADE do overflow a 375px, FURO 1)
  - Novo `e2e/mobile.spec.ts` com `test.use({ viewport: { width: 375, height: 667 } })` (D-07):
    Logado (stub, `loginViaStub`), varrer **TODAS** as rotas-chave, incluindo as mais densas/
    arriscadas: `/` → `/projetos` → **`/projetos/:id` (board)** → **`/clientes`** → `/relatorios` →
    **`/calendario`** → `/foco` → **`/workspace`** → `/ajustes`. Em CADA rota afirmar **zero overflow**
    (`document.documentElement.scrollWidth <= document.documentElement.clientWidth`) e — nas rotas em
    que o shell monta — **nav mobile presente** (`getByTestId('mobile-nav')`); o `/foco` é overlay
    fullscreen (CA-16): só o overflow se aplica lá. `/projetos/:id` é alcançado abrindo um projeto a
    partir de `/projetos` (semear/criar um projeto no setup, reusando os helpers de
    `tarefas.spec`/`core-projetos.spec`). Ação principal alcançável (abrir "Nova entrada" no Dashboard
    OU iniciar timer pela TimerBar) sem estourar (CA-20). **Controle desktop:** um `describe` com
    viewport 1280×800 afirma **nav mobile ausente** e **rail presente** (CA-13/20). Reusa helpers; não
    muta o timer global (parte parável). Liberar :8080/:4173 antes do run.
  - Acceptance: `mobile.spec.ts` verde com a varredura de overflow cobrindo as 9 rotas (incl.
    `/calendario`, `/projetos/:id` board, `/clientes`, `/workspace`); o controle desktop confirma a
    inversão (rail sim, bottom bar não). **Este é o único teste que afirma "zero overflow a 375px"**
    (Vitest/jsdom só cobre estrutura).

- **T-11 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: **regressão desktop** ≥768px — o `md:` reproduz o layout inline
    atual? · **overflow** a 375px em cada rota-chave (as 9 do CA-19, incl. `/calendario`,
    `/projetos/:id` board, `/clientes`, `/workspace`) · **breakpoint único 768** em todos os
    componentes · **cor** — nenhum hex novo, check-cores · **comportamento/dados idênticos** —
    nenhuma query/store/validação tocada · **PWA/Foco intactos** · **acessibilidade de toque** — nav
    por teclado, `aria-current`, alvos ≥40px) + refutação dupla. Corrigir confirmados. Registrar no
    fim deste arquivo.

- **T-12 — Gate + handoff** (RNF-05)
  - `make check-full` **VERDE** (Go test inalterado + Vitest + Playwright + builds + smoke compose).
    **Sem deploy nesta fase** (decisão do roadmap M2: a 22 vem antes da 23; deploy conforme a rotina
    do KICKOFF quando o conjunto fechar — não deployar mobile sem o controle remoto da 23, salvo
    decisão do Diogo). INDEX (22 construída, **sem migração**, frontend responsivo) + handoff
    (CLAUDE.md "retomar...") + commit + briefing pro Diogo.
  - Acceptance: gate verde; INDEX/handoff atualizados; nenhuma mudança em `backend/**`/migrações.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

**1 furo confirmado 2/2 aplicado** (ancorado no código real, pré-código — Regra Nº1). Os refutados/
descartados na verificação NÃO entram (só os confirmados 2/2 viram correção de spec/plan/tasks).

- **[HIGH] "Zero overflow a 375px" era assertado onde NÃO é testável + faltava cobertura das rotas
  mais arriscadas.** A acceptance dos CAs de página (CA-07..CA-10) era Vitest+jsdom, que **não tem
  layout engine** → `scrollWidth` não-mensurável (o assert "não estoura"/"cabe" seria vazio/falso-
  verde). E o único teste de overflow real (E2E Playwright, CA-19) cobria só `/`→`/projetos`→
  `/relatorios`→`/foco`→`/ajustes`, deixando de fora exatamente as rotas densas: `/calendario`
  (TimeGrid/MonthGrid), `/projetos/:id` (board de colunas), `/clientes` e `/workspace`.
  **Correção:** (a) **spec** — CA-07..CA-10 reescritos pra cobrir **só ESTRUTURA** em Vitest/jsdom
  (1-col / tabela→cartões / modal full-width), com nota explícita de que "zero overflow a 375px" é
  responsabilidade do CA-19; nota nova no cabeçalho da seção "Páginas-chave"; **CA-19** estendido pra
  varrer **9 rotas** (incl. `/calendario`, `/projetos/:id`, `/clientes`, `/workspace`) e declarado a
  **única fonte de verdade** do "zero overflow a 375px". (b) **plan** — D-07 reescrito: E2E é a única
  fonte do overflow (jsdom não mede `scrollWidth`), lista as 9 rotas e como alcançar `/projetos/:id`.
  (c) **tasks** — T-10 estendido pra varrer as 9 rotas; T-04..T-07 reformulados (Vitest = estrutura,
  overflow → T-10/CA-19) com nota de seção; dimensão de overflow do T-11 atualizada pras 9 rotas.

## Registro — BUILD (T-01→T-10) — agente-executor

**Suítes finais:** Vitest **400/400** (70 arquivos; +8 vs baseline 392 — novos:
`useMediaQuery.spec` 4, `navItems.spec` 4, `MobileNav.spec` 7, `TimerBar.responsive.spec`
4, +1 estrutural em cada Dashboard/Reports/board/Workspace) · Playwright **mobile.spec.ts
4/4** + todas as suítes desktop TOCADAS verdes (shell, tema/densidade, dashboard,
relatorios v1/v2, workspace, calendario, tarefas, core-projetos, novo-projeto, timerbar,
offline) · `npm run build` verde (Tailwind emite `md:hidden`, `md:grid-cols-3`, grid
arbitrário 1.6fr, e os `@media 767.98px`) · `check-cores` verde (zero cor nova — CA-14) ·
**`go test` INTOCADO** (0 arquivos de `backend/**` no diff — RNF-01/CA-12). Tree limpa.

**Commits:** T-01 useMediaQuery+navItems+SideNav · T-02 MobileNav+rail oculto+shell ·
T-03 TimerBar/BreakOverlay responsivos · T-04 Dashboard 1-col · T-05 ReportTable→cartões ·
T-06 board 1-col · T-07 Ajustes/Workspace/Calendário · T-08 nav=barra real + cabeçalhos/
grids sem overflow (mobile.spec.ts entrou aqui).

**Desvios/decisões registrados:**
- **MobileNav = a própria `<nav>` é a barra fixa** (não um wrapper): um wrapper `display:block`
  com só filhos `position:fixed` tinha tamanho ZERO → `getByTestId` resolvia "hidden" no
  Playwright. A `<nav>.tt-mobnav` ganhou o `position:fixed`; o menu "Mais"/backdrop viraram
  irmãos. Breakpoint único 768: `.tt-mobnav{display:none}` default + `@media(max-width:767.98px)
  {display:flex}` + `md:hidden` (todos alinhados em 768).
- **Overflow a 375px (achado no E2E, não no jsdom — FURO 1):** o cabeçalho flex `space-between`
  sem wrap de Projetos/Clientes/Dashboard estourava (título nowrap + toggle + botão). Fix:
  `flex-wrap:wrap`+gap no header; grids de cartões → `minmax(min(100%,Npx),1fr)` (encolhe < Npx,
  idêntico ≥ Npx). Validado: `/projetos` foi de scrollW 517→375.
- **ReportTable table↔cartões via `useMediaQuery`** (não CSS-only): cartões preservam TODOS os
  dados (label:valor). `Reports.spec` ganhou stub de `matchMedia` (jsdom não tem) default=desktop
  + `afterEach` unstub (CA-18, ajuste de AMBIENTE, nenhuma asserção de dado enfraquecida).
- **Tabelas Projetos/Clientes e Calendário JÁ eram compatíveis** (scroll contido em `tt-panel
  overflow:auto` / `tt-card overflow:hidden` + `minmax(0,1fr)`); modais já têm `maxWidth:
  calc(100vw - 32px)`. Sem mudança nesses (CA-09/10).

**Flake pré-existente (NÃO regressão, NÃO desta fase):** no full-suite `fullyParallel`, o
`cronometro.spec` falha de forma intermitente (test diferente a cada run) por contenção do
**timer GLOBAL** do único usuário do stub com outros specs que tocam timer (presence etc.) —
documentado no `CLAUDE.md`. **Em isolamento passa 10/10.** Reproduzido SEM o `mobile.spec.ts`
(some/aparece igual) → não é introduzido pela 22; o `mobile.spec` não inicia/mantém timer
(só abre modal/navega). O gate do orquestrador (T-12) trata cronometro com o cuidado de sempre.

## Registro — verificação adversarial do BUILD (T-11 — orquestrador, sessão 20, 22/06)

> Workflow ultracode, 7 dimensões × refutação dupla, ancorado no diff real `d8614ba..HEAD`.
> **6 findings brutos → 1 confirmado 2/2 (MEDIUM) → corrigido red→green. 5 refutados 2/2, 0 divididos.**

**[MEDIUM · CONFIRMADO 2/2 · BUG REAL] O `padding-bottom` responsivo do `<main>` era dead code → a
bottom bar fixa cobria ~40px do último elemento no mobile.** Em `AppShell.tsx` o
`<main className="tt-mainpad" style={{ padding: 'var(--pad)' }}>` usava o **shorthand `padding`
inline**, que pela cascata CSS (style inline vence author-normal sem `!important`, independente de
media-query) **derrotava** a regra `.tt-mainpad { padding-bottom: calc(--mobnav-h + safe-area + --pad) }`
do `@media (max-width:767.98px)` (tokens.css:318). Provado empiricamente: `getComputedStyle(main)
.paddingBottom` a 375px = **18px** (não os ~76px pretendidos); a bar fixa tem 58px+safe-area → o
respiro real era só 18px e a nav cobria o conteúdo. É **exatamente a armadilha vetada no plan §7**
("NÃO deixar a bottom bar cobrir o conteúdo") e o requisito normativo da spec.md:54 ("o `<main>`
ganha respiro inferior pra não ficar sob a bar fixa"). A suíte verde (400/400 + mobile.spec 4/4)
convivia com o bug porque o `mobile.spec` só aferia overflow **horizontal**, nunca o clearance
**vertical**. **Correção:** (a) `AppShell.tsx` — inline só `paddingTop/Right/Left` (tira o
`padding-bottom` do shorthand); (b) `tokens.css` — regra BASE `.tt-mainpad { padding-bottom:
var(--pad) }` (preserva o desktop 18px que saía do inline; sem ela ≥768 viraria 0, regressão), com o
`@media` sobrescrevendo abaixo de 768; (c) `mobile.spec.ts` — teste novo (computed `padding-bottom`
do `<main>` ≥ 58px a 375px — jsdom não computa @media+classe, tem de ser E2E) + asserção de
byte-compat no controle desktop (`0 < padding-bottom < 58`, não-zerado e não o clearance mobile).
**Mutação confirmada red→green:** com o shorthand de volta o teste falha (`18 < 58`); com o fix passa
(~76). mobile.spec 5/5 verde.

**Refutados 2/2 (registro — não acionados, disciplina 2/2):**
- `board-cols-minmax-vs-1fr` — board ≥768 usa `repeat(3,minmax(0,1fr))` onde o inline era
  `repeat(3,1fr)`: render desktop idêntico (o `minmax(0,...)` só evita blowout — é melhor, não
  regressão).
- `timerbar-running-break-overflow-untested` / `mobile-spec-board-relatorios-estado-vazio` — gaps de
  cobertura do E2E (TimerBar rodando / BreakOverlay / board com cards / ReportTable-cartões nunca
  passam pelo assert de overflow). **Não acionado de propósito:** exercitar o timer rodando no
  `mobile.spec` mutaria o timer global do único usuário do stub e introduziria a flake de contenção
  do `cronometro.spec` (lição registrada no CLAUDE.md) — o risco do teste supera o ganho.
- `report-charts-grid-sem-min-100pct` — `minmax(260px,1fr)` sem `min(100%,…)`: a 375px (≈343px úteis)
  o auto-fit dá 1 coluna de 260px, não estoura (260 < 343).
- `timerbar-iconbtn-below-40px-touch` — botões `.tt-iconbtn` 38px < 40px: tamanho **pré-existente**
  (não introduzido pela fase, que é layout puro); a spec CA-05/RNF-04 diz que alvo <40px "não bloqueia
  o build". Candidato a ajuste de DX numa fase futura.
