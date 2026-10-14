# Fase 31 — Recriação do design frontend (QA visual + polish)

> SDD (Regra Nº1) · Design segue a **Regra Nº2** (toda superfície passa pela skill `/frontend-design`).
> Fonte visual = mockups `mockups/projeto-workspace.html` + `mockups/projetos.html`.
> **Não toca lógica nem backend** — é a camada visual da M3 que ficou crua/divergente do mockup.

---

## 1. Contexto

A Milestone 3 (fases 24–30) entregou a **estrutura + a lógica** com gate verde e está em produção.
O **frontend, porém, divergiu do mockup**: componentes funcionais mas crus (chips sem estilo, slash
menu sem ícones, editor de notas em "caixa" em vez de documento, hierarquia tipográfica achatada,
timestamps omitidos). O QA de design (sessão 27) mapeou as divergências começando pelas **Notas**.

Esta fase **recria o design frontend tela por tela**, convergindo ao mockup e **elevando o polish**
com a `/frontend-design`, sem mexer em nada que já está testado e no ar funcionalmente.

## 2. Objetivo

- **CONVERGIR** cada superfície da M3 ao mockup (estrutura, espaçamento, hierarquia, estados).
- **ELEVAR** onde o mockup é cru/ambíguo, com decisões de design intencionais (`/frontend-design`).
- **PRESERVAR** 100% da lógica/comportamento: backend byte-idêntico, suítes Vitest/Playwright verdes.

## 3. Princípios (não-negociáveis)

| # | Princípio |
|---|-----------|
| P-01 | **Mockup = contrato visual.** Divergência implementado×mockup = bug de implementação a corrigir. |
| P-02 | **`tokens.css` = única fonte de cor** (gate `check-cores`). Nada de hex literal em componente. |
| P-03 | **Fonte = Inter** (o app trocou Geist→Inter na fase 24). O mockup usa Geist só por ser anterior — **seguimos o LAYOUT/escala do mockup, mas a família é Inter** (gate `check-fonts`). Ver D-01. |
| P-04 | **Perfil TDAH é lei de design:** object permanence (o que importa em 0 clique), à prova de culpa (nada vermelho acusatório; vazio = convite a agir, não erro), dopamina imediata. |
| P-05 | **`/frontend-design` por superfície** (Regra Nº2): personalidade na tipografia, **um** signature element por tela, restraint (uma ousadia por vez), quality floor (responsivo ≤768, foco visível, `prefers-reduced-motion`). |
| P-06 | **Zero regressão funcional** (RNF). Só markup/CSS/estilo de componente. Lógica, stores, queries, rotas, handlers e backend = intocados. |

## 4. Escopo — superfícies a recriar (em ORDEM)

> Cada superfície é uma **fatia própria** (mockup → `/frontend-design` → build → validação HITL no
> localhost → próxima). Ordem decidida pelo Diogo: **Notas primeiro** (caminho A).

1. **Notas** (caminho A) — lista Notion + painel/editor + slash menu + recent-notes no Overview.
2. **Detalhe do projeto** — header + tab-shell (Overview/Tasks/Notes/Goals/Calendar).
3. **Lista de projetos** — cards vivos + visão tabela.
4. **Board de tasks** — 4 colunas DnD + list + table + popup central (CenterPanel) + sub-tasks + propriedades + time control.
5. **Overview** — KPIs, This week, Upcoming, Recent notes.
6. **Goals** — lista + barras por modo.
7. **Calendar** — grade do projeto + overlay.
8. **Spotlight (Cmd+K)** — overlay de captura.
9. **Chrome global** — SideNav (rail hover-expand), TimerBar, MobileNav.

## 5. Critérios de aceite — GERAIS (valem por superfície)

- **CA-G1** Cada superfície converge ao mockup em estrutura, espaçamento (tokens `--pad/--gap/--r-*`), hierarquia tipográfica e estados (hover/vazio/selecionado).
- **CA-G2** `make check-full` **GATE VERDE**: `check-cores` + `check-fonts` + `check-pwa` + Vitest + Playwright (não-regressão funcional) + smoke.
- **CA-G3** Responsivo ≤768px (mobile da fase 22 preservado), foco de teclado visível, `prefers-reduced-motion` respeitado.
- **CA-G4** **Validação visual HITL**: o Diogo aprova a superfície no `localhost:5173` (espelho read-only) antes de fechar a fatia.
- **CA-G5** Nenhum arquivo de lógica/back alterado na fatia (diff = só `.tsx`/`.css`/tokens). Conferido no fim de cada fatia.

## 6. Critérios de aceite — NOTAS (caminho A, detalhado pelo QA da sessão 27)

> Ref. mockup: `renderNotas`, `notePanel`, `SLASH`/`onNoteInput`, card "Recent notes".

- **CA-N1 (bug)** Chips de refs com estilo real — `.chip` **não existe** no CSS (só `.tt-chip`); `NoteEditor.tsx:200` usa `className="chip"` → chip cru. Corrigir (usar `.tt-chip` ou definir `.chip` em tokens).
- **CA-N2** **Slash menu com ícones** por comando (Task/Note/To-do/H1/H2/Divider) + elevação do mockup (fundo `--surface-2`, selecionado `--surface-3`, largura ~290, padding 6).
- **CA-N3** **Recent notes** (Overview) com **timestamp** ("ontem"/data) + ícone de documento (não `list`) + link "Notes →".
- **CA-N4** **Lista de notas**: ícone doc na linha + chip `.md` estilizado + toolbar com a linha de contexto ("Markdown notes — stored as `.md` in Postgres").
- **CA-N5** **Editor como documento, não formulário**: textarea **transparente, sem borda, flex** (preenche o painel) — hoje tem borda + `--surface-2` + altura fixa `rows=12`.
- **CA-N6** **Hierarquia tipográfica**: título da nota 30px / weight 700 / `letter-spacing -.02em`; corpo 15px / line-height 1.7 (hoje 24px/600 e 14px/1.6).
- **CA-N7** Chrome da nota com a dica *"type `/` for commands"*.
- **CA-N8** Comportamento intocado: autosave/debounce, slash cria task/sub-nota reais, links, navegação por teclado — **suíte da fase 26 segue verde** (CA-G5).

## 7. Não-objetivos (FORA desta fase)

- Mudar comportamento, dados, contratos de API, rotas ou backend.
- Redesenhar do zero / inventar identidade nova (o mockup já é a direção aprovada; `/frontend-design`
  **eleva**, não substitui).
- Reintroduzir Geist (D-01) ou tirar a M3 do ar (a recriação roda no espelho local; deploy é só no fim, HITL).
- Telas fora da M3 (Foco 2.0, Relatórios 2.0, etc. — fases próprias se precisarem).

## 8. Decisões

- **D-01 — Fonte = Inter, não Geist.** O mockup é referência de **layout/escala/espaçamento**, não de
  família tipográfica. O app padronizou Inter na fase 24 e o gate `check-fonts` valida isso. `/frontend-design`
  trabalha a **escala e os pesos** dentro do Inter (display vs corpo vs mono/dados).
- **D-02 — TDD de design = estrutura + não-regressão + HITL visual.** Os testes automatizáveis cobrem
  presença de classe/elemento, snapshots de markup, `check-cores`/`check-fonts`, e a não-regressão
  funcional (Vitest/Playwright). O julgamento **visual** final é HITL (Diogo no localhost) — registrado
  como CA-G4, não como assert de pixel.
- **D-03 — Uma superfície por fatia, validada antes da próxima.** Evita big-bang visual; cada fatia
  fecha com gate verde + aprovação do Diogo.
- **D-04 — Modo de trabalho = VIBECODE (decisão do Diogo, sessão 27).** A fase 31 **não** segue TDD
  red→green por ajuste (Regra Nº1 relaxada SÓ aqui, por ser design): recria-se fielmente o mockup no
  código, o Diogo ajusta no `localhost:5173` (HMR) e itera no visual. Esta `spec.md` é **âncora**
  (escopo/princípios/CAs como norte), não checklist rígido. **Guardrail mantido:** o gate
  (`check-cores`/`check-fonts`/Vitest/Playwright/smoke) roda no fim de cada superfície pra garantir
  **não-regressão funcional** (CA-G2/G5).

## 9. Plano e tarefas

`plan.md` (decomposição por superfície + abordagem de teste) e `tasks.md` (T-01… por fatia, começando
pelas Notas) **a escrever na sequência**, após o Diogo validar os mockups e o escopo acima.
