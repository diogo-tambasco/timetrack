# plan.md — Fase 22 · Mobile responsivo (base)

> Como construir a spec. Decisões de design (D-xx) + onde mexer no front + ordem de tarefas.
> **Frontend puro** (RNF-01): nenhum arquivo de `backend/**`, nenhuma migração, nenhum contrato de
> API. A fase adiciona um componente de nav mobile, breakpoints Tailwind/CSS nos componentes
> existentes e regras responsivas nos tokens — **mantendo o desktop byte-compatível** (CA-13).
> Reusa: `AppShell` (rail+TimerBar+Outlet), `SideNav` (itens canônicos), `tokens.css` (densidade/
> cor), `ViewToggle`, e os specs de página/E2E existentes (ajuste só de seletor — CA-18).

---

## 1. Arquitetura geral

```
Frontend (SÓ frontend/src/**):
  routes/AppShell.tsx (existe)       ← + MobileNav no shell; <main> ganha respiro inferior
                                       (padding-bottom abaixo de md, pra não ficar sob a bottom bar)
  components/SideNav.tsx (existe)    ← rail oculto abaixo de md (hidden md:flex / classe responsiva)
  components/nav/MobileNav.tsx (NOVO)← bottom bar fixa: rotas-chave + acesso a conta/Sair (CA-01/03/04)
  components/nav/MobileNav.spec.tsx (NOVO)
  components/timer/TimerBar.tsx (existe)  ← controles reflowam/encolhem abaixo de md (CA-05)
  components/timer/BreakOverlay.tsx (existe) ← cabe a 375px (CA-06)
  routes/cockpit/Dashboard.tsx (existe)   ← grade 2col → 1col abaixo de md (CA-07)
  routes/cockpit/Reports.tsx + components/reports/* (existe) ← ReportTable→cartões; grids 1col (CA-08)
  components/board/* (existe)              ← TaskBoardColumns reflow/scroll contido abaixo de md (CA-09)
  components/projects/* · clients/* (existe) ← tabelas→cartões; modais full-width abaixo de md (CA-09)
  routes/cockpit/{Settings,Workspace,Calendar}.tsx (existe) ← empilham 1col; calendário não vaza (CA-10)
  styles/tokens.css (existe)         ← (se preciso) helpers responsivos de layout via @media md;
                                       NENHUMA cor nova (CA-14); densidade reusa --pad/--gap/--row-h
  tailwind.config / @theme (existe)  ← confirmar que sm/md/lg estão disponíveis (md=768, lg=1024)
```

**Não toca:** `backend/**`, `sw.ts`, `vite.config.ts`, `playwright.config.ts` (já injeta `GUARD_TICK`;
o E2E mobile só seta viewport no próprio teste — D-07), `lib/*` de domínio (api/queries/stores),
`--focus-*` e o `/foco` (CA-16), `--p-*`/cores (CA-14).

## 2. Sem migração / sem backend (RNF-01)

- **`MILESTONE-2.md §4`:** "21/22/23 sem migração (presence é efêmero; mobile é frontend)". A fase 22
  **não** é dona de migração. `0017–0020` são das fases 17–20; a 22 não cria nem renumera nada.
- INDEX.md: registrar **22 = sem migração** (frontend responsivo). Nenhuma entrada de migração nova.
- Nenhuma rota/handler/store novo no backend. `go test ./...` **não muda** (RNF-05 / CA-12).

## 3. Decisões de design

### D-01 — Breakpoint canônico `md = 768px` (divisor mobile↔desktop único — CA-11)

- **< 768px = mobile** (default mobile-first, sem prefixo Tailwind): nav mobile visível, rail oculto,
  grids 1-col, tabelas→cartões, TimerBar enxuta.
- **≥ 768px = desktop atual** (prefixo `md:`): rail 64px hover-expand, grids multi-col, tabelas,
  TimerBar em linha — **byte-compatível** (CA-13).
- **`lg = 1024px`** permitido **só** como degrau extra de grid (ex.: 1→2→3 col), nunca pra mover o
  divisor mobile↔desktop. O número **768 é único** em toda a fase (CA-11). Confirmar na T-01 que o
  Tailwind v4 expõe `sm`(640)/`md`(768)/`lg`(1024) padrão (não há config custom hoje — usar os
  defaults). Onde Tailwind não alcançar um estilo (CSS custom em `tokens.css`), usar
  `@media (min-width: 768px)` com **o mesmo 768**.

### D-02 — Nav mobile = **bottom bar fixa** (não drawer)

- **Por quê bottom bar e não drawer:** zero estado novo (drawer exigiria abrir/fechar, foco-trap,
  overlay — mais superfície de regressão e de teste); alcance com o polegar; padrão do controle
  remoto (fase 23). A bottom bar é `position: fixed; bottom: 0` **abaixo de `md`** (escondida no
  desktop), com 4–5 itens (`/`, `/projetos`, `/relatorios`, `/foco`) + um item **"Mais"/menu** que
  cobre o resto (`/clientes`, `/workspace`, `/calendario`, `/ajustes`, **Sair**) — CA-04. Itens reusam
  os mesmos `to`/ícones/labels do `SideNav` (única fonte: extrair a lista pra um módulo
  `components/nav/navItems.ts` consumido pelo `SideNav` E pela `MobileNav`, pra não divergir).
- **Ativo:** mesma regra do `SideNav` — `pathname === to || pathname.startsWith(to + '/')`, com `/`
  casando só `=== '/'` (CA-03). `aria-current="true"` no item ativo (RNF-04).
- **Conta/Sair (CA-04):** o `AvatarMenu` vive no rail (oculto no mobile) — expor o `signOut` no item
  "Mais"/menu da bottom bar (reusa `useAuthStore.signOut → navigate('/login')`, **idêntico**, sem
  chamada nova).
- **Respiro do conteúdo:** o `<main>` do `AppShell` ganha `padding-bottom` abaixo de `md` (altura da
  bottom bar + safe-area) pra o último elemento não ficar sob a bar (e respeitar
  `env(safe-area-inset-bottom)` no iOS). Acima de `md` esse padding é 0 (sem bar).

### D-03 — Rail oculto abaixo de `md` (CA-02)

- `SideNav` (ou o wrapper no `AppShell`) recebe `hidden md:flex` (ou `.tt-sidenav { display:none }`
  + `@media(min-width:768px){ .tt-sidenav{ display:... } }`). **Acima de `md` o `.tt-sidenav` é
  exatamente o de hoje** (64px, hover→232px, overlay — tokens.css:190–203 intocado no conteúdo;
  só ganha o gate de exibição por largura). Nada do comportamento canônico do rail muda no desktop.

### D-04 — Grids inline → responsivas, **sem mudar o desktop**

- Várias páginas usam **estilo inline** (`style={{ display:'grid', gridTemplateColumns: ... }}`),
  que **não** responde a breakpoint. Estratégia: trocar o inline por **classe** (Tailwind
  `grid grid-cols-1 md:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]` no Dashboard, p.ex.) **OU** por uma
  classe utilitária no `tokens.css` com `@media`. O **resultado ≥768px é idêntico** ao inline atual
  (mesmas frações/colunas) — só abaixo de 768 vira 1-col. Páginas afetadas: Dashboard (2col→1),
  Reports (gráficos/grid→1), Workspace (membros→1), board (colunas→1/scroll), etc.
- **Regra de ouro:** preservar o desktop **pixel-equivalente**. Onde o inline tem `minmax(0,1.6fr)
  minmax(0,1fr)`, o `md:` reproduz exatamente isso. (A verificação adversarial do build deve comparar
  o render desktop antes/depois — CA-13.)

### D-05 — Tabela densa → cartões abaixo de `md`

- `.tt-table` e os componentes `ReportTable`/`ProjectsTable`/`ClientsTable` ganham um **modo cartão**
  abaixo de `md`: ou (a) renderização condicional por breakpoint (`useMediaQuery('(min-width:768px)')`
  → tabela; senão → lista de cartões label:valor), ou (b) CSS responsivo que reempilha as células
  (`display:block` + `::before` com o label) abaixo de 768. **Decisão por componente** (o relatório
  é o caso obrigatório — CA-08 → cartões; board pode usar **scroll horizontal contido** pras colunas,
  CA-09). O importante: **zero overflow no documento** e **nenhum dado removido** (label:valor mantém
  tudo — §7 da spec). Um pequeno hook `lib/useMediaQuery.ts` (NOVO, sem dep) cobre os casos que
  precisam de render condicional; CSS-only onde der.

### D-06 — TimerBar enxuta (CA-05/06)

- A `barStyle` é `flex; gap; height:66; padding:0 var(--pad)`. Abaixo de `md`: permitir
  **wrap**/encolher (input já tem `min-width:0`), selects com largura compacta (`max-width` reduzido
  ou esconder o rótulo do select de task, mantendo o de projeto), botões mantidos (iniciar/parar/
  pausar — **nenhuma ação removida**, CA-05). Garantir `overflow` contido (a barra nunca empurra a
  largura do documento). O `BreakOverlay` (variante de pausa) idem: countdown + 3 ações cabem em
  375px (CA-06). **O `timerStore` e o comportamento não mudam** (CA-12).

### D-07 — E2E mobile = viewport no próprio teste (sem novo webServer)

- O `playwright.config.ts` **não muda** (já tem os 4 webServers). O spec novo
  `e2e/mobile.spec.ts` usa `test.use({ viewport: { width: 375, height: 667 } })` (ou
  `page.setViewportSize`) e, no caso de controle desktop (CA-20), um `test.describe` com viewport
  1280×800. Reusa `loginViaStub` (helpers.ts).
- **Overflow horizontal (FURO 1 — HIGH): o E2E é a ÚNICA fonte de verdade do "zero overflow" a
  375px** (Vitest+jsdom não mede `scrollWidth` — sem layout engine). Assertar
  `await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)`
  em **CADA** rota-chave, incluindo as mais densas/arriscadas: `/`, `/projetos`, **`/projetos/:id`
  (board de colunas)**, **`/clientes`**, `/relatorios`, **`/calendario` (TimeGrid/MonthGrid denso)**,
  `/foco`, **`/workspace`** e `/ajustes`. `/projetos/:id` é alcançado abrindo um projeto a partir de
  `/projetos` (semear/criar um projeto no setup, reusando os helpers do `tarefas.spec`/`core-projetos.spec`).
- **Nav mobile presente:** `getByTestId('mobile-nav')` (ou `getByRole('navigation')` escopado) nas
  rotas em que o shell a monta (o `/foco` é overlay fullscreen — CA-16 —, então a presença da bottom
  bar não se aplica lá; o overflow a 375px do `/foco` ainda é assertado). **Não** depende de
  `GUARD_TICK` nem de timer rodando (parte parável — não muta o timer global; só navega/abre modal).

## 4. Contratos / libs / deps

- **Zero dependência nova** (RNF — JS): só Tailwind (já no projeto) + um hook `useMediaQuery`
  caseiro (≈10 linhas, `window.matchMedia`) onde precisar de render condicional. Nenhuma lib de UI
  mobile, nenhum `react-responsive`.
- **Nenhum contrato de API novo** — a fase **não** toca rota/handler/payload. (Por isso esta seção
  não lista endpoints: é frontend puro — `MILESTONE-2.md §6`/RNF-01.)
- **Módulo extraído** `components/nav/navItems.ts`: a **única fonte** dos itens de nav (to/label/
  ícone), consumida por `SideNav` e `MobileNav` — evita divergência (CA-01/03 espelham o
  `SideNav`). É refactor sem mudança de comportamento desktop (o `SideNav` renderiza igual).
- **Hook** `lib/useMediaQuery.ts` (NOVO, sem dep): `useMediaQuery(query: string): boolean` via
  `matchMedia` + listener; SSR-safe default. Testável com `matchMedia` mockado no jsdom.

## 5. Pontos de integração com o baseline (o que é ADITIVO × intocado)

| Arquivo/contrato | Ação na fase 22 |
|---|---|
| `AppShell.tsx` (rail+TimerBar+Outlet) | **ADITIVO**: monta `<MobileNav>`; `<main>` ganha `pb` abaixo de md. Lógica do `attachTimerSync`/policy **intocada**. |
| `SideNav.tsx` + `.tt-sidenav` (tokens.css) | **ADITIVO**: gate de exibição por largura (oculto < md). Conteúdo/hover-expand do rail **intocado** (CA-02/13). Itens extraídos pra `navItems.ts` (mesma renderização). |
| `TimerBar`/`BreakOverlay` | **ADITIVO** (CSS responsivo): reflow abaixo de md. `timerStore`/ações **intocados** (CA-12). |
| Páginas cockpit (Dashboard/Reports/Projetos/Clientes/Workspace/Settings/Calendar) | **ADITIVO** (classes responsivas): grids 1-col, tabelas→cartões abaixo de md. Queries/mutations/fluxos **intocados** (CA-12). |
| `ViewToggle`/`useView` | **intocado** — segue funcionando (cards↔tabela) no mobile (CA-09). |
| `tokens.css` densidade/cor (`--pad`/`--gap`/`--row-h`/`--p-*`/`--accent`) | **intocado em cor** (CA-14); pode ganhar helpers de layout `@media` (sem cor). |
| `Focus.tsx` + `--focus-*` (fase 05) | **INTOCADO** (CA-16) — só confirma que cabe a 375px. |
| `sw.ts`/`vite.config.ts`/PWA | **INTOCADOS** (CA-17/RNF-06). |
| `backend/**` (timer/entries/reports/realtime/guard/push/auth/workspace) | **INTOCADO** (RNF-01) — `go test` não muda. |
| Specs Vitest/Playwright existentes | ajuste **só** de seletor onde o DOM mudou (CA-18), documentado; nenhuma asserção de comportamento enfraquecida. |

## 6. Dependências da fase + paralelização (`MILESTONE-2.md §9`)

- **DEPENDE de:** **todo o UI existente** (cross-cutting) — adapta Dashboard (13), Relatórios 2.0
  (14), Foco (05/15), board/tarefas (12), Projetos/Clientes (02), Workspace (10), Calendário (11),
  TimerBar/BreakOverlay (03), AppShell/SideNav (00). Idealmente também as superfícies novas da M2
  (visões "Minhas tasks"/18, presence/21) **se já estiverem construídas** — mas a 22 ancora no que
  existir no momento do build; o que vier depois (23) já nasce responsivo.
- **PARALELIZÁVEL: NÃO** (`MILESTONE-2.md §9`, linha da fase 22) — é **cross-cutting no frontend**:
  toca quase todos os componentes/páginas, então **conflita** com qualquer outra worktree de front.
  **Ordem:** buildar **depois** das fases backend (17/18/19/20/21) e **antes da 23** (o controle
  remoto depende desta base responsiva). Não roda em worktree paralela.
- **Risco de contenção:** se 18 (assignee picker no modal/board) e 21 (faixa de presence no
  workspace/dashboard) já tiverem mexido nesses arquivos, a 22  adapta o estado **mais recente**
  deles — por isso vem **depois** no pipeline.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** mudar comportamento/dados (CA-12/RNF-01) — só layout. Se um teste de comportamento exigir
  mudar **asserção de comportamento** pra passar, é sinal de regressão: parar e revisar (não
  enfraquecer o teste). Só seletor de DOM pode mudar (CA-18).
- **NÃO** regredir o desktop (CA-13) — o `md:` deve reproduzir **exatamente** o layout inline atual
  (mesmas colunas/frações). Comparar o render ≥768px antes/depois.
- **NÃO** introduzir cor (CA-14) — `check-cores.sh` é gate. Layout usa tokens existentes; nada de hex.
- **NÃO** usar dois breakpoints diferentes pro divisor mobile↔desktop (CA-11) — **768 único**; `lg`
  só pra degrau de grid, jamais movendo o limiar.
- **NÃO** esconder/remover **informação** no mobile (§7 spec) — reorganiza (1col/cartão), nunca
  remove dado do DOM acessível (cartão = label:valor com tudo).
- **NÃO** deixar a bottom bar cobrir o conteúdo — `<main>` ganha `padding-bottom` abaixo de md
  (altura da bar + `env(safe-area-inset-bottom)`).
- **NÃO** tocar `sw.ts`/`vite.config`/PWA (CA-17), o `/foco`/`--focus-*` (CA-16), nem `backend/**`
  (RNF-01).
- **NÃO** divergir os itens de nav entre rail e bottom bar — **uma** fonte (`navItems.ts`).
- **NÃO** quebrar `make check-full` por server na :8080/:4173 alheio — liberar as portas antes do
  E2E (achado de sessões anteriores: `reuseExistingServer:true` pega server errado).

## 8. Ordem de execução (ver tasks.md)

Fundação responsiva (confirmar breakpoints + extrair `navItems` + `useMediaQuery`) → nav mobile
(MobileNav + rail oculto + shell) → TimerBar/BreakOverlay → páginas por grupo (Dashboard → Relatórios
tabela→cartões → board/Projetos/Clientes → Ajustes/Workspace/Calendário) → não-regressão (check-cores,
suítes antigas, foco/PWA) → E2E mobile → verificação adversarial → gate. Commit por tarefa.
`make check-full` é o gate final.
