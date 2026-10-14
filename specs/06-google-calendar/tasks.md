# tasks.md — Fase 06 · Google Calendar (leitura)

> TDD red→green→refactor, **commit por tarefa**, mensagem PT-BR. Cada tarefa tem
> acceptance check. Google SEMPRE stubbado (TDD.md §3). Bloco único (T-01→T-10).

---

## T-01 — `internal/calendar`: token-source + cliente + `Service` (backend core)
**Objetivo:** decifrar `oauth_tokens`, montar `oauth2.TokenSource` auto-renovável, ler
eventos da `primary`, re-persistir o access renovado.
**Entregáveis:**
- `workspace.Store.LoadGoogleToken` / `SaveGoogleAccessToken` (plan §2).
- `calendar/client.go`: chamada HTTP (base URL injetável) + mapeamento do JSON do Google →
  `Event` (all-day via `start.date`; timed via `start.dateTime`; cancelados omitidos).
- `calendar/service.go`: `Service{db, oauthCfg, tokenKey, apiBaseURL, now}`; `Status`,
  `Events`; erros como **`*domain.Error`** (`domain.Conflict("calendar_not_connected"|
  "calendar_reconnect_required",…)`; 502 via `&domain.Error{Status:502,Code:"calendar_upstream"}`).
- `workspace.Store`: `LoadGoogleToken`/`SaveGoogleAccessToken` (só access+expiry) + **merge de
  scopes** no `UpsertUserFromGoogle` (D-02.1).
- Harness de teste do `calendar` com **token endpoint próprio** (httptest) pro refresh
  (token rotacionado / `invalid_grant`) — o `googlestub` não trata refresh (armadilha §6.9).
**Acceptance:** `go test ./internal/calendar/... ./internal/workspace/...` verde cobrindo:
CA-05 (eventos mapeados, ordenados), CA-06 (não conectado → 409 `calendar_not_connected`),
CA-07 (all-day), CA-08 (cancelado omitido), **CA-09 (expiry no passado + access de refresh
DIFERENTE → decifra `access_token_enc` do banco e compara; `scopes`/`refresh_token_enc`
byte-idênticos)**, CA-10 (`invalid_grant` → 409 `calendar_reconnect_required`), **CA-10.1
(Google 5xx/JSON inválido → 502 `calendar_upstream`)**, CA-12 (usa só o token do usuário),
**CA-03.2 (Upsert chamado com calendar e depois sem calendar → scopes ainda contêm
`calendar.readonly`)**. **RED antes.**

## T-02 — Stubs do Google Calendar + extensão do `googlestub`
**Objetivo:** Google Calendar de mentira pros testes/E2E + caminho de consentimento de
calendar no `googlestub`.
**Entregáveis:**
- `internal/calendar/calendarstub/` + fixtures `backend/testdata/calendar/events.json`
  (timed + all-day + 1 cancelado) e um caminho de **erro upstream** (rota/flag que devolve
  500 — pro CA-10.1 no E2E/integração). `cmd/calendarstub/main.go` (porta fixa, E2E).
- `googlestub`: `handleAuthorize` detecta `scope` contendo `calendar` → redireciona com
  `code-calendar`; senão `code-ok`. `handleToken(code-calendar)` → `token_calendar.json`
  (scope inclui `calendar.readonly` + refresh). Fixture `backend/testdata/google/token_calendar.json`.
**Acceptance:** `go test ./internal/auth/... ./internal/calendar/...` verde. **Teste de
unidade do `googlestub` afirmando os DOIS ramos** (sem calendar → `code-ok`; com calendar →
`code-calendar`) pra o caminho não nascer verde por acidente. O `calendarstub` serve as
fixtures. Testes da fase 01 seguem verdes (mudança aditiva — `code-ok` intacto). **RED antes.**

## T-03 — Rotas de calendar + `connect` + config + wiring
**Objetivo:** expor status/events/connect e ligar tudo.
**Entregáveis:**
- `auth.oauth.go`: método novo `BuildConsentURL(state, scopes...)` (`prompt=consent`), SEM
  tocar `BuildAuthURL` (login intocado); `handlers.go` lê cookie `tt_oauth_return` (default
  `/`; só aceita valor começando por `/` — anti open-redirect; limpo junto com o state).
- `api/calendar_routes.go`: `WithCalendar` + `routerCfg.calendar` + mount de
  `GET /calendar/{status,events,connect}` no `/api/v1`. Handlers delegam ao `writeDomainError`
  (erros já são `*domain.Error`). `config`: env `GOOGLE_CALENDAR_API_URL` (vazio → Google
  real). `cmd/server`: constrói e liga o `calendar.Service` (oauth2.Config = clientID/secret/
  endpoint do cfg; tokenKey; pool; calendar API URL).
**Acceptance:** `go test ./internal/api/... ./internal/config/... ./internal/auth/...`
verde cobrindo: CA-01 (302 com `calendar.readonly`+`prompt=consent`+offline), **CA-01.1
(`/auth/google/login` SEM `prompt=consent`)**, CA-02 (401 sem sessão), CA-03 (callback
persiste calendar scope + redirect `/calendario` via cookie), **CA-03.1 (sem cookie → `/`;
`tt_oauth_return=http://evil` → `/`)**, CA-04 (status true/false), CA-05/06/10/10.1/11 na
camada HTTP (códigos certos do envelope, não só status). **RED antes.**

## T-04 — Ângulo 4: `reports.PlannedReport` + `/reports/planned`
**Objetivo:** Planejado × Real por dia, sem tocar no `/reports/week`.
**Entregáveis:**
- `reports.Service.RealizedByDay(...)` (duração por dia local do `started_at`) +
  `PlannedReport(date, tz, source)` reusando `weekWindow` (`week_end = monday+6d`); interface
  consumidora `PlannedSource{Connected, Events(from,to)}` com `PlannedEvent{Start,End,AllDay}`;
  agregação por dia local do `start`, **all-day excluído** do planned. `reports` NÃO importa
  `calendar`.
- `api`: `handlePlannedReport` (adaptador `calendar.Service` → `reports.PlannedSource`,
  mapeando `calendar.Event`→`PlannedEvent`) montado em `/workspaces/{id}/reports/planned`
  quando há reports + calendar (gate `cfg.reports != nil && cfg.calendar != nil`, reusando o
  `cfg` do closure de `mountAuth` como o `mountRealtime`).
**Acceptance:** `go test ./internal/reports/... ./internal/api/...` verde cobrindo: CA-13
(planned por dia do start + realized por dia do started_at; `week_end` domingo inclusivo
idêntico ao `/reports/week`), **CA-13.1 (evento all-day NÃO entra em planned)**, CA-14 (não
conectado → connected:false, realized preenchido, sem chamar Google), CA-15 (evento fora da
janela da semana não entra; evento de domingo entra), CA-16 (404 ws alheio / 401), CA-17
(defaults date/tz). **RED antes.**

## T-05 — Frontend: `lib/api.ts` + `lib/queries/calendar.ts`
**Objetivo:** cliente e hooks da fase.
**Entregáveis:** tipos `CalendarStatus`, `CalendarEvent`, `PlannedReport`; `getCalendarStatus`,
`getCalendarEvents(from,to,tz)`, `getPlannedReport(ws,date,tz)`, const `CALENDAR_CONNECT_PATH`;
hooks `useCalendarStatus`, `useCalendarEvents(from,to)`, `usePlannedReport(ws,date)`.
**Acceptance:** `vitest run` verde — teste do api client (shape das chamadas, querystring,
tz do navegador) à la `api.test.ts`/`reports.ts`. **RED antes.**

## T-06 — Página `/calendario` real
**Objetivo:** substituir o StubPage pela página de eventos.
**Entregáveis:** `routes/cockpit/Calendar.tsx`: se `!connected` → cartão com "Conectar Google
Calendar" (navegação full-page pro `CALENDAR_CONNECT_PATH`); se `connected` → `WeekNav`
(reuso da 07) + grade/lista semanal de eventos por dia (horário + título; all-day marcado),
estado vazio "sem eventos esta semana". `components/calendar/WeekCalendar.tsx`.
> Nota: `/calendar/events` NÃO devolve `week_start/week_end` (RNF-03, sem eco do servidor).
> A página deriva segunda→domingo no CLIENTE a partir de uma data-âncora (igual ao `Reports.tsx`
> `dateForOffset`) e passa `weekStart/weekEnd` (strings ISO) ao `WeekNav` genérico.
**Acceptance:** `vitest run` verde: CA-18 (alterna connect × grade pelo status), CA-19
(navega semana, renderiza eventos da fixture mockada, marca all-day). Padrão de teste de
página (mock de `../../lib/api` + `QueryClientProvider`). **RED antes.**

## T-07 — Ângulo 4 nos Relatórios
**Objetivo:** seção "Planejado × real".
**Entregáveis:** `components/reports/PlannedVsReal.tsx` (barras por dia, planejado ×
realizado); `Reports.tsx` ganha a 4ª seção usando `usePlannedReport`. Não conectado →
CTA de conexão na seção, ângulos 1–3 intactos.
**Acceptance:** `vitest run` verde: CA-20 (barras com dados mockados; CTA quando
connected:false; ângulos 1–3 seguem renderizando). **RED antes.**

## T-08 — E2E `calendario.spec.ts`
**Objetivo:** fluxo central conectado.
**Entregáveis:** `playwright.config.ts` sobe o `calendarstub` (porta 9101) e injeta
`GOOGLE_CALENDAR_API_URL` no backend; `e2e/calendario.spec.ts`: login (stub) → /calendario
mostra "Conectar" → conecta (OAuth stub concede calendar) → eventos da fixture aparecem →
Relatórios mostram a seção Planejado × real.
**Acceptance:** `make test-e2e` (ou `playwright test calendario`) verde — CA-21. Derrubar
`make up` antes (porta 8080). **RED honesto:** rodar o spec antes do wiring da página.

## T-09 — Gate + INDEX + handoff
**Objetivo:** fechar a fase localmente.
**Entregáveis:** `make check-full` GATE VERDE (Go + Vitest + Playwright + builds + smoke do
compose); `specs/INDEX.md` marca a 06 construída; handoff no `CLAUDE.md` + `KICKOFF.md` da
próxima; commit.
**Acceptance:** `make check-full` verde numa execução única.

## T-10 — Deploy em produção (rotina da 09)
**Objetivo:** colocar a fase 06 no ar em `timmingtrack.life`.
**Entregáveis:** rsync (excludes do plan 09 §4, `deploy/.env` protegido) → `ssh timetrack-vps`
→ `docker compose build && up -d` em `/opt/timetrack/deploy` (forçar `--no-cache
timetrack-app` se o front não atualizar). **NÃO mexer no Caddy do PrimaOS** (vhost já existe).
**Acceptance:** smoke honesto (KICKOFF §"SMOKE QUE VALE"): `index.html` referencia o bundle
novo + bundle contém strings da fase (`Conectar Google Calendar`, `calendar/events`,
`reports/planned`); `/api/v1/calendar/status` sem sessão → 401 canônico; `primaos-*` SEM
restart. Briefing pro Diogo: ele clica **Conectar Google Calendar** em prod (anônimo) e
confirma os eventos na tela (validação logada — não dá headless).
