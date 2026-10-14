# plan.md — Fase 11 · Calendário 2.0

> Como construir a spec. Decisões de design (D-xx) + arquitetura + ordem de tarefas.
> Reusa a camada de dados da fase 06 inteira; adiciona só `/settings/calendar` no backend.

---

## 1. Arquitetura geral

```
Backend (mínimo, espelha pomodoro_settings):
  migrations/0012_calendar_settings.sql       ← tabela por-usuário + CHECKs
  internal/calendar/settings.go               ← calendar.Settings + GetSettings/SaveSettings
      (NOVO arquivo no pacote internal/calendar; o pacote já existe da fase 06)
  internal/api/settings_calendar.go           ← handlers GET|PUT finos
  internal/api/calendar_routes.go (existe)    ← + mountCalendarSettings no /api/v1 por-usuário

Frontend (o grosso):
  lib/api.ts                                  ← CalendarSettings + get/put
  lib/queries/calendar.ts (existe)            ← useCalendarSettings + useSaveCalendarSettings
  lib/calendar-layout.ts          (NOVO)      ← FUNÇÕES PURAS de layout (o coração, CA-09..14)
  lib/calendar-layout.test.ts     (NOVO)      ← testes das funções puras (red primeiro)
  components/calendar/
     TimeGrid.tsx                 (NOVO)      ← grade hora×dia (usada por Dia e Semana)
     MonthGrid.tsx                (NOVO)      ← grade de células do mês
     EventBlock.tsx               (NOVO)      ← bloco posicionado (accent) + abre popover
     EventPopover.tsx             (NOVO)      ← detalhe + "Abrir no Google"
     NowLine.tsx                  (NOVO)      ← linha do "agora" (só hoje)
     CalendarNav.tsx              (NOVO)      ← anterior/próximo/hoje + título + troca de view
     WeekCalendar.tsx (existe)               ← removido/aposentado (lista da 06) — ver D-09
  components/settings/CalendarGroup.tsx (NOVO)← grupo "Calendário" nos Ajustes
  routes/cockpit/Calendar.tsx (existe)        ← reescrito: mantém os 3 estados da 06, troca a
                                                grade pela nova (Dia/Semana/Mês + nav + prefs)
  routes/cockpit/Settings.tsx (existe)        ← + <CalendarGroup/>
```

## 2. Migração — `0012_calendar_settings.sql` (dona: fase 11)

```sql
-- 0012_calendar_settings — fase 11 (plan §2): preferências da UI de
-- calendário POR USUÁRIO (sincroniza entre dispositivos). Sem linha =
-- defaults aplicados na leitura (CA-01).
CREATE TABLE calendar_settings (
  user_id      uuid PRIMARY KEY REFERENCES users(id),
  default_view text NOT NULL DEFAULT 'week'        CHECK (default_view IN ('day','week','month')),
  hour_start   integer NOT NULL DEFAULT 6          CHECK (hour_start BETWEEN 0 AND 23),
  hour_end     integer NOT NULL DEFAULT 23         CHECK (hour_end BETWEEN 1 AND 24),
  density      text NOT NULL DEFAULT 'comfortable' CHECK (density IN ('comfortable','compact')),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (hour_end > hour_start)
);
```

- INDEX.md: registrar **0012 dona da fase 11** (a convenção atual lista 0009+ como fase 03;
  11 estende). Próximo número livre após `0011_timer_states`.

## 3. Backend — `internal/calendar/settings.go` + handlers

Espelho FIEL do `timer/settings.go` + `api/settings_pomodoro.go`:

```go
// calendar.Settings — contrato do GET|PUT /api/v1/settings/calendar.
type Settings struct {
    DefaultView string `json:"default_view"`
    HourStart   int    `json:"hour_start"`
    HourEnd     int    `json:"hour_end"`
    Density     string `json:"density"`
}
func (s *Service) GetSettings(ctx, userID) (Settings, error)   // defaults se sem linha
func (s *Service) SaveSettings(ctx, userID, in) (Settings, error) // valida → upsert
```

- **Validação (CA-03)** no `SaveSettings`, ANTES do SQL, devolvendo `domain.Validation(...)`:
  `default_view ∈ {day,week,month}`; `density ∈ {comfortable,compact}`;
  `0 ≤ hour_start ≤ 23`; `1 ≤ hour_end ≤ 24`; `hour_end > hour_start`.
- `GetSettings`: `SELECT ... WHERE user_id=$1`; `sql.ErrNoRows` → defaults (6/23/week/comfortable).
- **D-01 (RESOLVIDO):** o `calendar.Service` hoje guarda `store *workspace.Store`, **não** o
  pool — mas `NewService(db, ...)` JÁ recebe o `*pgxpool.Pool`. Opção escolhida = **A**: guardar
  `db *pgxpool.Pool` num campo novo do `Service` e pendurar `GetSettings`/`SaveSettings` nele
  (espelha o `timer.Service`, que faz timer + pomodoro_settings juntos). Sem serviço novo, sem
  mudar a assinatura de `NewService`. Os handlers recebem o `*calendar.Service` já injetado via
  `WithCalendar` (CA já montado nas rotas).
- Handlers `settings_calendar.go`: `handleGetCalendarSettings`/`handlePutCalendarSettings`
  finos, `reqctx.UserID`, `decodeJSON`, `writeDomainError`, `respond.JSON` — idênticos em forma
  aos do pomodoro.

### 3.1 Wiring (rota por-usuário)

No `calendar_routes.go` (ou onde o `WithCalendar` monta), adicionar no `/api/v1` atrás do
`RequireAuth`, FORA do subrouter de workspace (igual `settings/pomodoro`):
```
r.Get("/settings/calendar", handleGetCalendarSettings(...))
r.Put("/settings/calendar", handlePutCalendarSettings(...))
```

## 4. Frontend — funções puras de layout (`lib/calendar-layout.ts`) — D-02

O coração testável. **Sem React, sem DOM** — só datas → números. Tudo em fuso local.

- `type Positioned = { event, dayISO, topPct, heightPct, colIndex, colCount }`
- **`splitByDay(event, days: string[]): Segment[]`** — corta um evento timed nos limites de dia
  local; cada segmento tem `dayISO`, `startMin` (0–1440), `endMin` (0–1440, clipado). Cobre
  cross-midnight e multi-dia (CA-11). **Descarta segmento com `endMin == startMin`** (duração de
  parede zero — fim exatamente em 00:00, CA-11.1): 10:00→00:00 vira 1 segmento 10:00–24:00, sem
  fantasma no dia+1. All-day **não** passa por aqui.
- **`packColumns(segments por dia): Positioned[]`** — algoritmo de varredura de intervalos:
  ordena por início, agrupa clusters que se sobrepõem, atribui `colIndex`/`colCount` mínimo
  (CA-12). Borda-a-borda (`end == start`) NÃO sobrepõe.
- **`topPct/heightPct`** — `top = startMin/1440`, `height = max((end-start)/1440, MIN)` (CA-09/10),
  em fração de 0..1 do dia (a grade renderiza 24h sempre; o viewport é só scroll, CA-15).
- **`allDayByDay(events, days)`** — agrupa all-day por dia coberto, intervalo **`[startDay,
  endDay)`** (end EXCLUSIVO — convenção GCal do `client.go`, CA-13.1): all-day `D→D+1` = 1
  coluna; `D→D+3` = 3 colunas. NUNCA inclusivo (off-by-one).
- **`monthGridDays(refDate): string[]`** — 6 semanas (42 dias) segunda→domingo cobrindo o mês
  do `refDate`, incluindo dias adjacentes (CA-18). `inMonth(dayISO, ref)` pra atenuar.
- **`addMonths(refDate, delta)`** — soma meses com **clamp do dia** em `min(dia,
  últimoDiaDoMêsAlvo)` (CA-19.1): `2026-01-31 +1 → 2026-02-28`, não março. `addDays`/`addWeeks`
  triviais pros outros passos.
- **`periodTitle(view, refDate)`** — título a partir do **`refDate`** (CA-19.2): Mês = mês/ano
  do refDate (independe da grade de 42 dias); Dia = data; Semana = faixa seg→dom.
- **`weekDays(refDate)`/`dayWindow(refDate)`** — dias da semana (seg→dom) e janela do dia.
- **Helpers de janela de fetch:** `weekRange`/`dayRange`/`monthRange` → `{from,to}` ISO pro
  `getCalendarEvents` (CA-05/06/07). Reusa a lógica `weekRange` que hoje vive no `Calendar.tsx`.

**Por que puro:** o TDD da grade vive aqui (red→green sem render). Os componentes só consomem
`Positioned[]`. **Casos de teste obrigatórios:**
- evento 10:00–11:00 → 1 linha (top/height esperados);
- 23:00→01:00 → 2 segmentos clipados (dia1 23:00–24:00, dia2 00:00–01:00);
- **10:00→00:00 (fim na meia-noite) → 1 único segmento 10:00–24:00, NENHUM no dia+1 (CA-11.1);**
- 3 eventos 10:00–11:00 sobrepostos → colCount 3, colIndex 0/1/2;
- 10:00–11:00 + 11:00–12:00 borda-a-borda → colCount 1 cada (CA-12);
- evento de 5min → height ≥ MIN (CA-10);
- **all-day `D→D+1` → 1 coluna (só D); all-day `D→D+3` → 3 colunas D/D+1/D+2, não 4 (CA-13.1);**
- mês de fevereiro → 42 dias começando na segunda;
- **`addMonths(2026-01-31,+1)` → fevereiro/2026 (não março); round-trip next→prev preserva o mês (CA-19.1);**
- **`periodTitle('month', grade de Julho começando 29/06)` → "Julho 2026" (CA-19.2);**
- DST (se aplicável) → posição por hora local de parede.

## 5. Frontend — componentes

- **`TimeGrid`** (Dia = 1 coluna, Semana = 7): coluna de horas à esquerda (0–24, rótulos),
  colunas de dia, faixa all-day fixa no topo, `NowLine` na coluna de hoje, `EventBlock`s
  posicionados por `topPct/heightPct/colIndex/colCount`. `density` controla a altura de linha
  (`--row-h`). Auto-scroll no mount (CA-17) via `ref` + `scrollTo`.
- **`MonthGrid`**: 6×7 células (`monthGridDays`), número do dia, chips de evento (timed +
  all-day, ordenados por início), "+N mais" quando estoura; clicar a célula → vai pra view Dia
  daquele dia (UX); clicar chip → popover. Hoje destacado; fora-do-mês atenuado.
- **`EventBlock`**: bloco accent (`--accent-soft`/`--accent-line`/texto), título + horário,
  `onClick` abre `EventPopover`. Altura mínima respeitada.
- **`EventPopover`**: card flutuante (título, horário/"dia inteiro", "Abrir no Google" se
  `html_link`). Fecha em Esc/clique-fora. Acessível (role, foco).
- **`NowLine`**: linha + bolinha na posição `nowPct`, atualiza via `setInterval` (60s) /
  `requestAnimationFrame`; some quando o dia não é hoje.
- **`CalendarNav`**: `‹ Hoje ›` + título do período + segmented control Dia/Semana/Mês.
  Aposenta o `WeekNav` da 06 nesta página (ou o reusa pro modo Semana — D-09).

### D-09 — `WeekCalendar.tsx` (lista da 06)

A lista agrupada por dia da fase 06 é **substituída** pela grade. Remover o componente e seu
spec, OU mantê-lo só se algum teste da 06 depender — checar na T-08. As specs da 06
(`Calendar.spec.tsx`) que assertam `week-calendar`/`cal-event` na **lista** serão
**reescritas** pro novo layout (exceção sancionada ao "não tocar testes de fase anterior": a 06
é dona da CAMADA DE DADOS, não do layout; a fidelidade dos dados — eventos certos na semana —
continua assertada, só muda a forma). Registrar como desvio.

## 6. Frontend — `Calendar.tsx` (reescrito) e `Settings.tsx`

- `Calendar.tsx`: mantém os 3 estados da 06 (loading/error do status, CTA não-conectado, erro
  de eventos → reconectar) — **CA-26**. Acrescenta: estado `view` (init = `default_view` das
  prefs, CA-23) e `refDate`; `CalendarNav`; busca a janela conforme a view (CA-05/06/07);
  renderiza `TimeGrid` (Dia/Semana) ou `MonthGrid` (Mês). Prefs via `useCalendarSettings`
  (carrega antes de fixar a view inicial; enquanto carrega, neutro).
- `Settings.tsx`: `<CalendarGroup/>` — select de view, dois selects/inputs de hora
  (start 0–23, end 1–24, com validação `end>start` no client antes do PUT), toggle de densidade.

## 7. Armadilhas conhecidas (vetadas)

- **NÃO** posicionar por segundos absolutos (epoch) — usar hora **local de parede** (CA-14):
  imune a DST e a offsets. `new Date(iso)` já dá local.
- **NÃO** recortar a grade na faixa de horas (perderia eventos) — faixa é só viewport (CA-15).
- **NÃO** tratar evento all-day na grade de horas (vira bloco de 24h gigante) — faixa própria
  (CA-13). (Espelha a lição do CA-13.1 da 06.)
- **NÃO** usar `localeCompare` de ISO pra ordenar quando precisa de minuto-do-dia — converter
  pra Date local (o `WeekCalendar` da 06 usava `localeCompare`; aqui precisa de minutos).
- **NÃO** esquecer borda-a-borda ≠ sobreposição (CA-12) — senão eventos consecutivos viram
  meia largura sem motivo.
- **NÃO** adicionar dep de calendário (FullCalendar etc.) — RNF-01.
- **NÃO** mexer no `internal/calendar/client.go` (colorId fica descartado — CA-22) nem nas
  rotas `/calendar/{status,events,connect}` (RNF-05).
- **NÃO** tratar o `end` de all-day como inclusivo (CA-13.1): é EXCLUSIVO (convenção GCal do
  `client.go`) — iterar `[startDay, endDay]` inclusivo pinta uma coluna a mais.
- **NÃO** criar segmento de duração zero pra evento que termina em 00:00 (CA-11.1) — descartar
  antes da altura mínima, senão vira bloco-fantasma clicável no dia seguinte.
- **NÃO** incrementar mês com `new Date(y, m±1, d)` cru (CA-19.1) — pula fevereiro a partir de
  31/jan; usar `addMonths` com clamp do dia.
- **NÃO** derivar o título do período da janela de fetch (CA-19.2) — na view Mês a grade de 42
  dias atravessa 3 meses; o título vem do `refDate`.
- **NÃO** ligar a `density` do calendário ao token global `--row-h` nem ao `data-density` do
  documento (CA-24.1) — usar `--cal-row-h` escopado à grade, senão regride `/hoje` e `/workspace`.

## 8. Ordem de execução (ver tasks.md)

Backend primeiro (settings: migração → service → handlers → wiring → teste de rota), depois o
núcleo puro de layout (TDD pesado), depois componentes, página, ajustes, E2E, gate. Commit por
tarefa. `make check-full` é o gate final.
