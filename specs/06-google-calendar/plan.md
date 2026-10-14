# plan.md — Fase 06 · Google Calendar (leitura)

> Como construir a `spec.md`. Herda o baseline do `SDD.md §7` e o `specs/INDEX.md`.

---

## 1. Decisões de arquitetura (com razão)

- **D-01 — Leitura ao vivo, sem persistir eventos.** Nenhuma migração/tabela nova
  (RNF-03). A página `/calendario` e o ângulo 4 buscam do Google sob demanda. Cache local
  (tabela `calendar_events` + sync incremental) é a otimização natural depois — registrada
  como fora de escopo. Razão: leveza + escopo de 1 sessão + menor superfície de bug.
- **D-02 — Consentimento incremental, não no login.** O login base segue só
  `openid email profile` (o `oauth.go` já comenta isso). Calendar entra por um fluxo
  dedicado `GET /api/v1/calendar/connect` que usa um **método novo
  `BuildConsentURL(state, scopes...)`** (NÃO mexer na assinatura do `BuildAuthURL`, que tem
  testes pinando os params do login em `oauth_test.go`) adicionando `calendar.readonly` +
  `oauth2.SetAuthURLParam("prompt","consent")`. Razão: não forçar o escopo sensível em quem
  só quer logar; e `prompt=consent` garante o refresh token mesmo em re-consentimento (sem
  ele o Google pode não devolver refresh, e o CA-09 quebraria em ~1h).
- **D-02.1 — Merge de scopes no upsert (CA-03.2).** A `UpsertUserFromGoogle` (fase 01) hoje
  faz `scopes = EXCLUDED.scopes` (substituição total). Um login normal DEPOIS do connect
  derrubaria `calendar.readonly` (o campo `scope` da resposta de token reflete só os scopes
  do pedido atual). Correção mínima e necessária: trocar por **união**
  `scopes = (SELECT array(SELECT DISTINCT unnest(oauth_tokens.scopes || EXCLUDED.scopes)))`.
  É idempotente pro login normal (união de {base} com {base} = {base} → testes da 01
  intactos) e preserva calendar entre logins. Registrado como decisão de implementação da 06
  tocando código da 01 (escopo necessário). Revogação real no Google ainda é pega pelo
  `/events` (401 → `calendar_reconnect_required`).
- **D-03 — `internal/calendar` é o dono do token-source + cliente HTTP da Calendar API.**
  Carrega `oauth_tokens` (decifra com `auth.Open`), monta um `oauth2.TokenSource`
  auto-renovável, re-persiste o token renovado (CA-09). Razão: concentra a lógica de token
  fora do HTTP, testável com Google stubbado.
- **D-04 — Ângulo 4 é endpoint SEPARADO (`/reports/planned`), o `/reports/week` não muda.**
  Razão: o `week` tem orçamento de latência (RNF-02 da 07) e nenhuma chamada externa; o
  `planned` chama o Google (lento, pode falhar). Acoplar quebraria os ângulos 1–3 quando o
  GCal estivesse fora. O `reports.Service` ganha o cálculo via uma **interface consumidora**
  (`PlannedSource`) que o `calendar.Service` satisfaz — `reports` NÃO importa `calendar`
  (sem ciclo); o `api` (que importa os dois) faz o adaptador.
- **D-05 — Rotas de calendar são POR USUÁRIO** (como o timer, INDEX.md): montadas em
  `/api/v1` atrás do `RequireAuth`, fora do subrouter de workspace. O ângulo 4 é a exceção:
  fica no subrouter do workspace (precisa de membership pra ler as time entries).
- **D-06 — Escopo:** `https://www.googleapis.com/auth/calendar.readonly` (constante única,
  fácil de trocar). Suficiente pra ler eventos da `primary`.
- **D-07 — Erros são `*domain.Error` (não sentinela).** O `writeDomainError` existente só
  reconhece `*domain.Error`; qualquer outro vira 500. Então o `calendar.Service` devolve:
  não conectado → `domain.Conflict("calendar_not_connected", …)` (409); refresh revogado →
  `domain.Conflict("calendar_reconnect_required", …)` (409); falha upstream (5xx/JSON
  inválido/timeout) → `&domain.Error{Status:502, Code:"calendar_upstream", …}` (502);
  validação → `domain.Validation(…)` (400). Tudo flui pelo `writeDomainError` sem mapeamento
  novo. Log nunca inclui token/Authorization.

## 2. Modelo de dados

**Nenhuma migração nova** (D-01/RNF-03). Reaproveita `oauth_tokens` (def. canônica em
`01-auth-google/plan.md §2`): leitura nova de `access_token_enc`, `refresh_token_enc`,
`token_expires_at`, `scopes`; escrita nova só do access renovado (CA-09).

Métodos novos no `workspace.Store` (dono do `oauth_tokens`, evita SQL de token espalhado):
- `LoadGoogleToken(ctx, userID) (StoredToken, bool, error)` — devolve os blobs cifrados +
  expiry + scopes; `false` quando não há linha.
- `SaveGoogleAccessToken(ctx, userID, accessEnc []byte, expiresAt time.Time) error` —
  `UPDATE oauth_tokens SET access_token_enc=$, token_expires_at=$, updated_at=now()` — **só
  access + expiry**; NÃO toca `refresh_token_enc` nem `scopes` (a resposta de refresh do
  Google omite ambos; re-derivar zeraria o `calendar.readonly`). CA-09 pina: pós-refresh,
  `scopes`/`refresh_token_enc` seguem byte-idênticos.

Mudança no upsert da fase 01 (D-02.1): `scopes = união(antigos, novos)` em vez de
substituição.

```go
type StoredToken struct {
    AccessTokenEnc  []byte
    RefreshTokenEnc []byte // nil se ausente
    ExpiresAt       time.Time
    Scopes          []string
}
```

## 3. Contratos de API

### 3.1 `GET /api/v1/calendar/connect` (CA-01/02)
Atrás do `RequireAuth`. Gera `state`, grava cookie `tt_oauth_state` (igual ao login) +
cookie `tt_oauth_return=/calendario`, e **302** pro Google:
`BuildAuthURL(state, "https://www.googleapis.com/auth/calendar.readonly")` com
`prompt=consent` adicionado. Sem sessão → 401 (pelo middleware).

`HandleCallback` (existente) ganha **uma** mudança aditiva: ao final, lê o cookie
`tt_oauth_return` (se presente e começa com `/`) e redireciona pra ele; default `/`
(comportamento do login intocado). O cookie é limpo junto com o `state`.

### 3.2 `GET /api/v1/calendar/status` (CA-04)
```json
{ "connected": true }
```
`connected` = existe token Google do usuário com `calendar.readonly` nos scopes.

### 3.3 `GET /api/v1/calendar/events?from=YYYY-MM-DD&to=YYYY-MM-DD&tz=IANA` (CA-05..CA-12)
```json
{ "events": [
  { "id": "abc", "summary": "Reunião", "start": "2026-06-15T09:00:00-03:00",
    "end": "2026-06-15T10:00:00-03:00", "all_day": false,
    "html_link": "https://calendar.google.com/..." }
] }
```
- Janela: `[from 00:00 loc, (to+1d) 00:00 loc)` (mesmo padrão `ParseInLocation` do
  `entries.ListDay`). `singleEvents=true&orderBy=startTime` na chamada ao Google (expande
  recorrências, ordena).
- Não conectado → 409 `calendar_not_connected` (CA-06). Refresh revogado (`invalid_grant`) →
  409 `calendar_reconnect_required` (CA-10). Validação → 400 (CA-11). Google 5xx/JSON
  inválido/timeout → 502 `calendar_upstream` (CA-10.1, NUNCA 500).
- Dia inteiro: `start.date`/`end.date` → `all_day:true`, `start` = 00:00 do dia local,
  `end` = 00:00 do dia local de `end.date` (a API do Google usa fim exclusivo). Cancelados
  (`status:"cancelled"`) omitidos (CA-08).

### 3.4 `GET /api/v1/workspaces/{id}/reports/planned?date&tz` (CA-13..CA-17)
```json
{ "week_start": "2026-06-15", "week_end": "2026-06-21", "connected": true,
  "days": [ { "date": "2026-06-15", "planned_seconds": 7200, "realized_seconds": 5400 }, ... ] }
```
- O handler reusa `reports.Service.weekWindow(date, loc)` (já privado, mesmo do `/reports/week`)
  → `[monday 00:00, monday+7d 00:00)`. **`week_end = monday + 6 dias`** (domingo INCLUSIVO,
  byte-idêntico ao `/reports/week` — NUNCA o limite exclusivo `monday+7d`).
- 7 dias (segunda→domingo), sempre presentes mesmo com zeros, derivados do mesmo `monday`.
- O adaptador busca eventos via `PlannedSource.Events(ctx, userID, from, to)` com
  `from=monday, to=monday+7d` (a janela inteira da semana). Evento fora da janela não entra
  (CA-15).
- `planned_seconds` = eventos `all_day:false` agregados pelo **dia local do `start`** (CA-13;
  all-day excluído — CA-13.1). `realized_seconds` = entries fechadas agregadas pelo **dia
  local do `started_at`** (`RealizedByDay`, SQL com `started_at AT TIME ZONE` ou bucket em Go
  igual ao `focusMetrics`).
- `connected:false` → `planned_seconds:0` em todos, `realized_seconds` preenchido (CA-14),
  sem chamar o Google.
- Interface consumidora em `reports` (sem importar `calendar`):
  `PlannedSource interface { Connected(ctx, userID) (bool, error); Events(ctx, userID, from, to time.Time) ([]PlannedEvent, error) }`
  onde `PlannedEvent{Start, End time.Time, AllDay bool}`. O adaptador (no pacote `api`) mapeia
  `calendar.Event` → `reports.PlannedEvent`.

### 3.5 Chamada ao Google (cliente)
`GET {GOOGLE_CALENDAR_API_URL}/calendars/primary/events` com query
`timeMin`(RFC3339)`&timeMax&singleEvents=true&orderBy=startTime&maxResults=2500`, header
`Authorization: Bearer <access token vivo>`. Base URL default
`https://www.googleapis.com/calendar/v3`. Sem paginação na v1 (maxResults cobre a semana de
um humano com folga; registrado).

## 4. Pacotes / libs

- **Nada de dep nova.** Reusa `golang.org/x/oauth2` (já presente) pro token-source/refresh e
  `net/http` pra chamada. JSON com `encoding/json`.
- Novos arquivos backend:
  - `internal/calendar/service.go` — `Service` (token-source, refresh+persist, `Status`,
    `Events`), tipos `Event`, erros `ErrNotConnected`/`ErrReconnect`.
  - `internal/calendar/client.go` — chamada HTTP da Calendar API (base URL injetável) +
    mapeamento do JSON do Google → `Event` (all-day, cancelados).
  - `internal/calendar/calendarstub/calendarstub.go` — Google Calendar de mentira (fixtures
    `backend/testdata/calendar/`), análogo ao `googlestub`.
  - `cmd/calendarstub/main.go` — serve o stub numa porta fixa pro E2E.
  - `internal/api/calendar_routes.go` — `WithCalendar(svc)` + mount de status/events/connect
    + `handlePlannedReport`.
  - testes: `internal/calendar/*_test.go`, `internal/api/calendar_routes_test.go`.
- `internal/reports/service.go` — `RealizedByDay(...)` + `PlannedReport(...)` + interface
  `PlannedSource`; `internal/reports/planned_test.go`.
- `internal/config/config.go` — env `GOOGLE_CALENDAR_API_URL`.
- `internal/workspace/store.go` — `LoadGoogleToken` / `SaveGoogleAccessToken`.
- `internal/auth/oauth.go` — `BuildAuthURL` aceita opção de `prompt=consent` (ou método novo
  `BuildConsentURL`); `handlers.go` lê o cookie de retorno.

- Frontend:
  - `lib/api.ts` — `getCalendarStatus`, `getCalendarEvents`, `getPlannedReport`, const
    `CALENDAR_CONNECT_PATH = '/api/v1/calendar/connect'` (navegação full-page).
  - `lib/queries/calendar.ts` — `useCalendarStatus`, `useCalendarEvents`, `usePlannedReport`.
  - `routes/cockpit/Calendar.tsx` — página real (substitui o StubPage).
  - `components/calendar/WeekCalendar.tsx` (+ subcomponentes) e
    `components/reports/PlannedVsReal.tsx`.
  - `e2e/calendario.spec.ts` + webServer do `calendarstub` no `playwright.config.ts`.

## 5. Pontos de integração com o baseline

- `routerCfg` ganha `calendar *calendar.Service`; `WithCalendar` seta. `mountAuth` monta
  `calendar/{status,events,connect}` no `/api/v1` (D-05) e `reports/planned` no subrouter do
  workspace quando `cfg.calendar != nil && cfg.reports != nil`.
- `cmd/server/main.go` constrói `calendar.NewService(...)` com a `oauth2.Config` (clientID,
  secret, endpoint — mesmos do `authSvc`), `cfg.TokenKey`, `pool`, e
  `cfg.GoogleCalendarAPIURL`. Wire via `api.WithCalendar(calSvc)`.
- `auth.Open`/`auth.Seal` reusados pra decifrar/cifrar (calendar importa auth — sem ciclo:
  auth não importa calendar).
- E2E: `playwright.config.ts` sobe o `calendarstub` (porta 9101) e injeta
  `GOOGLE_CALENDAR_API_URL` no backend; o `googlestub` ganha o caso "scope com calendar →
  token com calendar.readonly" (fixture `token_calendar.json`, code `code-calendar`).

## 6. Armadilhas conhecidas (vetadas)

1. **Sem `prompt=consent` no connect** → re-consentimento não devolve refresh token → o
   access expira em ~1h e o calendar morre sem reconexão. SEMPRE `prompt=consent` no connect
   (via `BuildConsentURL`), e NUNCA no login (CA-01.1).
2. **Re-login apaga o `calendar.readonly`** (HIGH) — o upsert da 01 substitui scopes. SEM o
   merge (D-02.1), qualquer login normal pós-connect zera a conexão. CA-03.2 PINA: conecta →
   re-loga sem calendar → status segue `connected:true`.
3. **Re-persist do refresh** — usar `SaveGoogleAccessToken` (só access+expiry), NUNCA o
   upsert completo do callback no caminho de refresh (zeraria scopes/refresh). CA-09 PINA
   (decifra do banco, compara o access novo, afirma scopes/refresh intactos).
4. **Erro do calendar ≠ 500.** Devolver `*domain.Error` (409/400/502), NUNCA sentinela cru
   nem pânico — `writeDomainError` só entende `*domain.Error`. Não logar token/Authorization.
5. **`all_day` no ângulo 4** — 1 evento all-day = 86400s infla `planned_seconds` e a barra
   mente. all-day FORA do `planned_seconds` (CA-13.1); só aparece na grade do `/calendario`.
6. **`week_end` do `/reports/planned`** = `monday+6d` (domingo inclusivo), NUNCA o `to`
   exclusivo `monday+7d` do `weekWindow` — senão diverge do `/reports/week` (CA-13).
7. **`reports` NÃO importa `calendar`** (interface consumidora) — manter pra evitar ciclo e
   manter o `week` puro. `LoadGoogleToken`/`SaveGoogleAccessToken` vivem em `workspace.Store`
   (folha), não dentro de `calendar`.
8. **`changeOrigin`/proxy do Vite** já resolvido na fase 04 — o `/api/v1/calendar/*` passa
   pelo mesmo proxy; não criar proxy novo.
9. **Refresh nos testes:** o refresh do `oauth2.TokenSource` bate na `Endpoint.TokenURL`. O
   `googlestub` NÃO trata `grant_type=refresh_token` — então os testes UNITÁRIOS do
   `calendar` (CA-09/CA-10) usam um **token endpoint próprio do harness** (httptest) que
   serve um access NOVO (token rotacionado) ou `400 invalid_grant`. O `googlestub` só ganha o
   ramo `scope com calendar → code-calendar → token_calendar.json` (pro E2E, onde NÃO há
   refresh: token válido ~1h).

## 7. Blocos da sessão

Sessão única (Diogo pediu build + deploy na mesma sessão; 9 tarefas, ≤10 — cabe num bloco):
- **T-01→T-04** backend (calendar pkg + stub + rotas + ângulo 4).
- **T-05→T-07** frontend (api/queries + /calendario + ângulo 4 nos relatórios).
- **T-08** E2E.
- **T-09** gate `make check-full` + INDEX + handoff.
- **T-10** deploy em produção (rotina da 09) + smoke honesto.
