# plan.md — Fase 13 · Dashboard (home nova)

> Como construir a spec. Decisões de design (D-xx) + arquitetura + ordem de tarefas.
> **Fase 100% frontend** — composição de queries existentes + o contrato de objetivos da
> fase 12. Sem backend, sem migração (RNF-03). Commit por tarefa; `make check-full` no fim.

---

## 1. Arquitetura geral

```
Roteamento:
  App.tsx (existe)                            ← / → <Dashboard/>; /hoje → <Navigate to="/" replace/>
  components/SideNav.tsx (existe)             ← item "Hoje" aponta pra "/" (D-05)

Frontend (o grosso — tudo novo é aditivo):
  routes/cockpit/Dashboard.tsx     (NOVO)     ← compõe a home (CA-04, 15, 10, 20)
  routes/cockpit/Today.tsx (existe)           ← APOSENTADO (removido; ninguém mais importa — D-06)
  components/dashboard/
     TimerCard.tsx                 (NOVO)     ← hero: rodando / pausa / ocioso (CA-12/13/14)
     RecentEntries.tsx             (NOVO)     ← dias ANTERIORES agrupados por dia (CA-06/07/09/11); hoje = TodayList reusado
     WeekGoals.tsx                 (NOVO)     ← metas de horas da semana (CA-16/18/19) — reusa GoalBar
     FocusShortcut.tsx             (NOVO)     ← atalho pro /foco (CA-20) — pode ser inline no card
  lib/recent.ts                    (NOVO)     ← groupByDay PURA (CA-07) + dayLabel ("Hoje"/"Ontem")
  lib/recent.test.ts               (NOVO)     ← testes da função pura (red primeiro)
  lib/queries/entries.ts (existe)             ← + useRecentEntries (aditivo — CA-06)
  (SEM WeekObjectives — progresso por-objetivo agregado fica pra fase futura, D-04/CA-17)

Reuso direto (NÃO recriar):
  components/timer/{TodayList,EntryModal}.tsx ← TodayList reusado p/ HOJE (CA-24.1); RecentEntries replica a linha p/ dias anteriores
  lib/timerStore (useTimerStore/useTimerClock), lib/queries/{projects,reports}, lib/format,
  lib/palette, lib/auth (useCurrentWorkspace/useAuthStore)
```

## 2. Sem backend — o que NÃO muda (RNF-03/05)

- Nenhuma migração, rota, handler ou payload. `make check-full` Go segue verde sem tocar nada.
- A janela de recentes usa o **mesmo** `GET /entries?from&to&tz` da fase 03 (o backend já
  resolve `[from 00:00, to+1d 00:00)` no fuso — `entries/service.go::ListDay`). 7 dias = 1 request.
- Metas = `GET /reports/week?date&tz` da fase 07 (já existe, `useWeekReport`).
- Objetivos = o que a **fase 12** expõe (endpoint/hook). Esta fase **consome**, não define.

## 3. Toques em arquivo compartilhado (aditivos — enumeração fechada para o CA-02)

1. **`App.tsx`** — `/` passa a renderizar `<Dashboard/>`; nova rota `/hoje` →
   `<Navigate to="/" replace/>`. (Antes `/` era `Navigate to /hoje`; inverte.)
2. **`SideNav.tsx`** — item "Hoje" muda `to: '/hoje'` → `to: '/'` (D-05). Mantém label/ícone.
3. **`lib/queries/entries.ts`** — adiciona `useRecentEntries` (hook novo; nenhum existente muda).
4. **`Today.tsx`** — removido (D-06); nenhum teste de fase anterior pina a página como tela
   própria (checar no T-08). `TodayList.tsx`/`EntryModal.tsx` ficam INTOCADOS (reusados).
5. **`e2e/helpers.ts`** (FURO CONFIRMADO — exceção mecânica) — `loginViaStub` afirma
   `expect(page).toHaveURL(/\/hoje$/)`; como o login agora cai em `/` (Dashboard), trocar pra
   `toHaveURL(/\/$/)` (ou assertar que o Dashboard renderizou). É a ÚNICA asserção de E2E que
   muda, e é mecânica (URL pós-login), não de comportamento (CA-02/23/24.1).
6. **`Focus.tsx`** (OPCIONAL) — o `✕` faz `navigate('/hoje')` (→ redireciona pra `/`, CA-05).
   Pode-se trocar pra `navigate('/')` direto pra evitar o bounce **se** o `Focus.spec` não pinar
   o destino `/hoje`; senão deixar como está (funciona via redirect). Decidir no T-08.

Tudo aditivo/redirecionamento; nenhuma **asserção de comportamento** das fases 00–12 muda
(CA-02/23). As entradas de HOJE no Dashboard saem do `TodayList` **reusado** (CA-24.1), então os
selectors que o `cronometro.spec` dirige por `goto('/hoje')` (→ `/`) seguem idênticos.

## 4. Objetivos da fase 12 — por que o dashboard NÃO mostra progresso por-objetivo (D-04)

> **Reescrito após a verificação adversarial da spec (furo confirmado 2/2).** A fase 12 JÁ está
> construída e em produção (sessão 17, 20/06); o "consumir o que a 12 expuser com degradação"
> da versão anterior era código morto — a query existe, só tem o shape errado.

**Fato do código real (ancorado):**
- `useObjectives(ws, projectId)` / `listObjectives` → objetivo "pelado": `id, name, description,
  status, goal_hours, position, …` — **SEM** `done_pct`/`task_count`/`tracked_seconds`.
- O progresso (`done_pct`, etc.) vive só no **`BoardObjective`**, devolvido **exclusivamente**
  por `getBoard(ws, projectId)` (`GET /workspaces/{id}/projects/{projectId}/board`) — **por
  projeto**. O componente de barra (`board/ProgressBar.tsx`) exige exatamente
  `{doneCount, taskCount, donePct, trackedSeconds, goalHours, color}` — tudo board-only.
- **Não existe** `GET /workspaces/{id}/objectives` (com progresso, por workspace).

**Decisão (D-04):** o dashboard **não exibe progresso por-objetivo** nesta fase (CA-17). A seção
"da semana" mostra as **metas de horas por projeto** (`WeekGoals`, de `/reports/week` — CA-16),
que é dado real e independente da fase 12. Agregar objetivos por workspace exigiria **N requests**
(um `/board` por projeto — viola RNF-02) ou **endpoint novo** (viola RNF-03/CA-02), então fica
**registrado como fase futura** (`GET /workspaces/{id}/objectives` com progresso). O lugar do
progresso por objetivo continua sendo `/projetos/:id` (o board da fase 12); o dashboard pode
**linkar** pra lá, não recalcula nem reagrega nada. **Não há `WeekObjectives` nem fallback de
runtime** — a seção da semana é o `WeekGoals`, ponto.

## 5. Função pura — `lib/recent.ts` (o testável)

`groupByDay(entries, tz)` → `{ dayISO, label, items }[]` ordenado do dia mais recente pro mais
antigo:
- Dia local de cada entry = `started_at` no fuso `tz` (parede local — consistente com a 03/11;
  `new Date(iso)` + componentes locais, NÃO epoch).
- `dayLabel(dayISO, todayISO)` → "Hoje" / "Ontem" / data formatada pt-BR (ex.: "qua, 17 jun").
- A entry **rodando** (sem `stopped_at`) entra no dia de hoje (started_at de hoje) — destaque no
  componente, não na função.
- **Casos de teste obrigatórios:** entry de hoje → grupo "Hoje"; entry de ontem → "Ontem";
  entry de 5 dias atrás → grupo com label de data; ordem: hoje antes de ontem antes de antes;
  dois itens no mesmo dia ficam juntos; lista vazia → `[]` (CA-11); fuso: uma entry de
  domingo 23:30 em America/Sao_Paulo cai no dia local certo (parede local, tz-independente nos
  testes via round-trip `Date local → toISOString`, padrão da fase 11).

**Por que puro:** o agrupamento por dia (o coração do P3) testa sem render. `RecentEntries` só
consome os grupos e desenha as linhas (forma do `TodayList`).

## 6. Componentes

- **`Dashboard.tsx`** (`routes/cockpit/`): layout da Direção C — header (saudação + data +
  "Nova entrada"), `TimerCard` (hero), grid com `RecentEntries` (coluna maior) e a coluna de
  `WeekObjectives`/`WeekGoals` + `FocusShortcut`. `useCurrentWorkspace` pro `ws`; `EntryModal`
  controlado por estado local (igual ao `Today.tsx` atual).
- **`TimerCard.tsx`**: lê `useTimerStore`/`useTimerClock`. Três ramos:
  - rodando → "Rodando agora" + descrição + chip projeto (`archived:true`) + dígitos grandes
    (`shown` = countup?elapsed:remaining??elapsed) + Pausar/Parar (`pause`/`stop` do store).
  - `mode==='break'` → rótulo "Pausa" + contagem regressiva (`useTimerClock`) — sem replicar o
    BreakOverlay inteiro (D-02).
  - ocioso → CTA "Comece a cronometrar" + botão que foca a TimerBar do topo ou abre Nova
    entrada (D-03; **sem** formulário de start completo — CA-14/CA-03).
  Estilo: `.tt-card`, glow accent opcional, dígitos `mono` `tabular-nums` (espelha o `HeroTimer`
  do mockup), tokens (RNF-04).
- **HOJE = `TodayList` reusado (CA-24.1).** O Dashboard renderiza o componente `TodayList`
  existente direto pras entradas de hoje — selectors ("Nova entrada" no header, "Editar" por
  linha, entry rodando em accent) idênticos por construção, então o `cronometro.spec` passa pelo
  redirect `/hoje`→`/` sem editar asserção. O `TodayList` já cobre CA-08 (rodando em accent,
  duração viva) e CA-09 (continuar/editar/parar) pra hoje.
- **`RecentEntries.tsx`** (dias ANTERIORES): `useRecentEntries(ws)` → `groupByDay`; renderiza só
  os grupos com `dayISO < hoje` (pula "Hoje" — é do TodayList). Para cada grupo: cabeçalho de dia
  + linhas no formato do `TodayList` (dot da cor do projeto, descrição, projeto, horário, duração;
  continuar/editar — sem entry rodando aqui, ela é de hoje). Projetos com `{archived:true}`.
  Re-busca ao mudar `activeEntryID`. Vazio (nenhum dia anterior com entry) → mensagem amigável
  (CA-11). **D-07:** replicar a linha (NÃO tocar o `TodayList` — CA-24); registrar na T-05.
- **`WeekGoals.tsx`**: `useWeekReport(ws, hojeISO)` → `report.goals`; cada um numa `GoalBar`
  (reusa o componente da fase 02/07, fill cor do projeto, accent ≥100%, meta×realizado). Vazio
  (sem metas) → estado vazio amigável (CA-19). É a seção "da semana" canônica (CA-16/17/18).
- **`FocusShortcut.tsx`** (ou inline): `Link to="/foco"` proeminente, rótulo "Modo Foco",
  ícone `Icon.focus`. Não substitui o item da SideNav (CA-20).

## 7. Decisões (D-xx)

- **D-01:** janela de recentes = **7 dias** (`from = addDays(hoje, -6)`, `to = hoje`). Constante
  `RECENT_DAYS = 7` documentada. Cobre "ontem" e a virada do dia (P3) sem inchar o request.
- **D-02:** o `TimerCard` na pausa só **reflete** o estado (rótulo + countdown) — o BreakOverlay
  completo (retomar/pular/parar com presets) segue na TimerBar; o dashboard não duplica os
  controles de pausa pra não competir (CA-03/13).
- **D-03 (ocioso):** o CTA do card NÃO recria o formulário de start (descrição+projeto+task+play)
  — isso é da TimerBar global, sempre visível no topo. O CTA aponta pra ela (foco/scroll) ou
  abre "Nova entrada". Evita dois starts conflitantes (CA-14).
- **D-04:** consumo dos objetivos da fase 12 com degradação pra metas — ver §4. **A 13 não
  define endpoint de objetivos.**
- **D-05:** o item "Hoje" da SideNav aponta pra **`/`** (a home agora é o dashboard). Mantém
  label "Hoje" e ícone. (`/hoje` ainda redireciona pra `/` — CA-05 — pra bookmarks.) Registrar
  no INDEX se mudar o rótulo no futuro.
- **D-06:** `Today.tsx` é **removido** (a home é o `Dashboard`); checar no T-08 que nenhum spec
  importa `Today` (busca por `routes/cockpit/Today`). O `TodayList`/`EntryModal` ficam.
- **D-07:** reuso vs. duplicação da linha de entrada — decidido na T-05 (preferir reuso do
  `TodayList` por grupo; se a forma do `TodayList` não aceitar "entries de um dia X" sem tocá-lo,
  replicar a linha no `RecentEntries`). Em nenhum caso tocar o `TodayList` (CA-24).

## 8. Armadilhas conhecidas (vetadas)

- **NÃO** introduzir polling/requisição periódica nova (RNF-02/CA-01) — o timer vem do store
  (sync da 04 já empurra); recentes/metas/objetivos são query cacheada.
- **NÃO** duplicar o controle de start completo da TimerBar dentro do card (CA-14/D-03) — dois
  formulários de start = dois timers conflitantes.
- **NÃO** acumular tempo local na duração viva da entry rodando — derivar de
  `Date.now()+offsetMs−started_at` (forma do `TodayList`, RNF-02 da 05).
- **NÃO** agrupar recentes por dia via `localeCompare`/epoch — usar parede local
  (`new Date(iso)` + componentes locais), senão a virada do dia erra no fuso do Diogo (CA-07).
- **NÃO** deixar uma seção que falha (`/reports/week` ou objetivos) derrubar o dashboard inteiro
  (CA-21) — cada bloco trata seu `isError`/`isLoading`.
- **NÃO** acoplar no shape exato dos objetivos da fase 12 sem fallback (CA-18/D-04b).
- **NÃO** tocar `TodayList.tsx`/`EntryModal.tsx`/TimerBar/BreakOverlay/sync (CA-23/24) — só
  reusar. Os toques compartilhados são os 4 do §3.
- **NÃO** embutir o `CalendarGrid` da Direção C na home (fora de escopo §7) — mantém o boot leve.
- **NÃO** inventar token/hex novo — `tokens.css` + `palette.ts` (RNF-04).

## 9. Ordem de execução (ver tasks.md)

Função pura (recent) com TDD → hook → card → recentes → metas → objetivos+fallback → dashboard
+ roteamento/SideNav (aposenta Today) → E2E → gate. Commit por tarefa. `make check-full` no fim.
Cabe em **1 bloco** (≤10 tarefas).
