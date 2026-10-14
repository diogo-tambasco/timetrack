# spec.md — Fase 06 · Google Calendar (somente leitura, v1)

> Fonte de verdade da fase. Deriva do `REQUISITOS.md §4.3` e `§4.5 (ângulo 4)`.
> **Recorte travado com o Diogo (14/06/2026):** apenas **leitura** (GCal → Timetrack),
> **só a agenda principal (primary)**, **com o ângulo 4 dos relatórios**. Escrever de volta
> no Google (entries → eventos), multi-agenda, sync incremental/webhooks e cache local de
> eventos ficam **fora desta fase** (ver §7).

---

## 1. Objetivo

Trazer a agenda do Google pra dentro do Timetrack: o usuário conecta seu Google Calendar
(consentimento incremental com o escopo `calendar.readonly`), vê os eventos da agenda
principal na página **Calendário** (por semana) e ganha o 4º ângulo dos relatórios —
**Planejado × Real** (o que estava na agenda × o que foi cronometrado).

GCal é a fonte da verdade da agenda (REQUISITOS §1); nesta v1 o Timetrack **lê** essa fonte,
não a altera.

## 2. User stories

- **US-01** — Como usuário, quero **conectar meu Google Calendar** ao Timetrack, pra ver
  minha agenda dentro do app sem trocar de ferramenta.
- **US-02** — Como usuário, quero ver os **eventos da minha agenda principal** na página
  Calendário, navegando por semana, pra planejar meu tempo.
- **US-03** — Como usuário, quero a comparação **Planejado × Real** nos relatórios, pra ver
  o desvio entre o que agendei e o que de fato fiz.

## 3. Critérios de aceite (EARS — cada um vira teste)

### Conexão (OAuth incremental)

- **CA-01** — QUANDO o usuário autenticado aciona a conexão (`GET /api/v1/calendar/connect`),
  o sistema DEVE redirecionar (302) ao consentimento do Google pedindo o escopo
  `https://www.googleapis.com/auth/calendar.readonly`, com `access_type=offline`,
  `prompt=consent` (garante refresh token na re-consentimento) e `include_granted_scopes=true`,
  gravando o `state` anti-CSRF em cookie temporário.
- **CA-01.1** (fronteira do login) — O login normal (`GET /auth/google/login`) NÃO DEVE
  forçar `prompt=consent` nem pedir o escopo de calendar (não irritar quem só quer logar). O
  `prompt=consent` é exclusivo do `connect`. (Pinar: param presente no connect, ausente no
  login.)
- **CA-02** — O endpoint de conexão DEVE exigir sessão: sem sessão válida → **401** canônico,
  sem iniciar nenhum fluxo OAuth.
- **CA-03** — QUANDO o callback do Google retorna com o escopo de calendar concedido, o
  sistema DEVE persistir o token cifrado (AES-256-GCM, herdado da 01) com os **scopes
  concedidos** (incluindo `calendar.readonly`) e o refresh token; refresh ausente preserva o
  anterior (COALESCE — CA-14.1 da fase 01). O callback de conexão DEVE redirecionar de volta
  pra `/calendario`.
- **CA-03.1** (não-regressão do login) — O callback SEM cookie `tt_oauth_return` DEVE manter
  o redirect pra `/` (login da fase 01 intocado). COM `tt_oauth_return` começando por `/` →
  redireciona pra esse path; valor que NÃO começa por `/` (tentativa de open-redirect) → `/`.
- **CA-03.2** (scope não regride no re-login) — Conectar o calendar e DEPOIS fazer um login
  normal (sem o escopo de calendar) NÃO DEVE remover `calendar.readonly` dos scopes
  persistidos: o upsert de `oauth_tokens` faz **merge** (união) dos scopes, nunca substituição
  total. Status DEVE seguir `connected:true`. (Sem isso, todo login derrubaria a conexão.)
- **CA-04** (status) — `GET /api/v1/calendar/status` DEVE responder `{"connected":true}` SE e
  somente SE existir token Google do usuário cujos `scopes` incluam `calendar.readonly`;
  caso contrário `{"connected":false}`. Sem sessão → 401.

### Leitura de eventos (agenda principal)

- **CA-05** — QUANDO o usuário conectado chama
  `GET /api/v1/calendar/events?from&to&tz`, o sistema DEVE buscar os eventos da agenda
  **`primary`** do Google na janela **[from 00:00, (to+1d) 00:00)** no fuso `tz`, e responder
  a lista mapeada (`id`, `summary`, `start`, `end`, `all_day`, `html_link`), ordenada por
  início ascendente.
- **CA-06** — SE o usuário NÃO conectou o calendar (sem token ou sem o scope `calendar.readonly`),
  `GET .../events` DEVE responder **409 `calendar_not_connected`** SEM chamar o Google.
- **CA-07** (dia inteiro) — eventos de dia inteiro (campo `start.date`/`end.date`) DEVEM ser
  mapeados com `all_day:true` e os horários cobrindo o(s) dia(s); eventos com horário
  (`start.dateTime`) → `all_day:false`.
- **CA-08** (cancelados) — eventos com `status:"cancelled"` DEVEM ser omitidos da resposta.
- **CA-09** (refresh transparente) — SE o access token do Google estiver expirado
  (`token_expires_at` no passado) E houver refresh token válido, o sistema DEVE renová-lo
  automaticamente, **re-cifrar e persistir SÓ o novo access token + expiry** (via
  `SaveGoogleAccessToken`, NUNCA o upsert do callback — refresh e scopes ficam intocados), e
  completar a leitura sem erro pro usuário. **Teste falsificável:** semear token com expiry
  no passado + fixture de refresh com access **DIFERENTE**; após `Events()`, decifrar
  `access_token_enc` do banco e comparar com o novo (não basta "a leitura não falhou");
  afirmar que `scopes`/`refresh_token_enc` seguem byte-idênticos.
- **CA-10** (reconexão) — SE a renovação falhar (refresh inválido/revogado → `invalid_grant`),
  `GET .../events` DEVE responder **409 `calendar_reconnect_required`** (pedindo reconexão),
  NUNCA 500.
- **CA-10.1** (upstream) — SE o Google responder erro NÃO-autenticação (5xx, JSON inválido,
  timeout), `GET .../events` DEVE responder **502 `calendar_upstream`** com o envelope
  canônico, NUNCA 500, e sem logar token/Authorization.
- **CA-11** (validação) — `from`/`to` fora de `YYYY-MM-DD`, ou `tz` não-IANA → **400
  `validation_error`** (sem chamar o Google).
- **CA-12** (isolamento por usuário) — a leitura DEVE usar exclusivamente o token do usuário
  da sessão; o token de outro usuário NUNCA é usado. (Constituição §8.4 estendida ao GCal.)

> **Erros tipados:** o `calendar.Service` devolve `*domain.Error` (a 409 via
> `domain.Conflict("calendar_not_connected"|"calendar_reconnect_required", …)`, a 502 via
> `&domain.Error{Status:502, Code:"calendar_upstream"}`) — fluem pelo `writeDomainError`
> existente sem mapeamento novo. **Nunca** sentinela cru (`errors.New`), que viraria 500.

### Ângulo 4 — Planejado × Real

- **CA-13** — `GET /api/v1/workspaces/{id}/reports/planned?date&tz` DEVE resolver a **mesma
  semana** (segunda→domingo no fuso) do `/reports/week` (`week_start`/`week_end` byte-idênticos:
  `week_end = segunda + 6 dias`, domingo INCLUSIVO) e responder, por cada um dos **7 dias**:
  - `planned_seconds` — soma das durações dos **eventos COM HORÁRIO** (`all_day:false`) cujo
    **início** cai naquele dia local. Evento inteiro atribuído ao dia do `start` (SEM clip —
    simetria com o `realized` e com os switches da fase 07).
  - `realized_seconds` — soma das durações das **time entries FECHADAS** do usuário no
    workspace cujo `started_at` cai naquele dia local (entrada inteira pelo `started_at`, SEM
    clip — padrão do `focusMetrics` da 07).
- **CA-13.1** (all-day fora do planejado) — eventos `all_day:true` (feriados, férias) NÃO
  entram em `planned_seconds` (não representam tempo planejado de trabalho; 1 all-day = 86400s
  destruiria a comparação). Eles continuam visíveis na página `/calendario` (CA-19), só não
  contam no ângulo 4.
- **CA-14** — SE o usuário não conectou o calendar, `/reports/planned` DEVE responder
  `"connected":false` com `realized_seconds` por dia preenchido e `planned_seconds:0`
  (degrada graciosamente, sem erro — não chama o Google).
- **CA-15** (janela exata) — os eventos somados DEVEM vir da janela
  `[segunda 00:00, segunda+7d 00:00)` no fuso (a MESMA `weekWindow` do `/reports/week`):
  evento fora da semana NÃO entra; evento de domingo entra.
- **CA-16** — `/reports/planned` DEVE seguir a cadeia `RequireAuth → RequireMember`: workspace
  alheio/inexistente → **404**; sem sessão → **401**.
- **CA-17** (defaults) — `date` vazio → semana de hoje no fuso; `tz` vazio → UTC (mesma regra
  do `/reports/week`, CA-06 da fase 07).

### Frontend

- **CA-18** — A página `/calendario` DEVE mostrar o botão **"Conectar Google Calendar"**
  QUANDO `status.connected=false`, e a **grade semanal de eventos** QUANDO `connected=true`.
- **CA-19** — A página `/calendario` DEVE navegar por **semana** (anterior/próxima) e listar
  os eventos de cada dia com horário e título; eventos de dia inteiro marcados como tal.
- **CA-20** — A página **Relatórios** DEVE ganhar a seção **"Planejado × real"** com barras
  por dia (planejado × realizado). Quando não conectado, a seção mostra um CTA de conexão —
  **sem quebrar** os ângulos 1–3.

### E2E

- **CA-21** — Logado (stub), o usuário **conecta o calendar** (fluxo OAuth stubbado que
  concede o escopo de calendar) e vê os **eventos da fixture** na página `/calendario`; a
  seção **Planejado × real** aparece nos relatórios. (Google stubbado — TDD.md §3.)

## 4. Requisitos não-funcionais

- **RNF-01** — Em teste, NUNCA bater no Google real: cliente da Calendar API com **base URL
  injetável** por env (`GOOGLE_CALENDAR_API_URL`), stub com fixtures (TDD.md §3) — incluindo
  o caso de token expirado (CA-09).
- **RNF-02** (segurança) — tokens do Google permanecem **cifrados em repouso**; o access
  token renovado é **re-cifrado antes de persistir**; token/segredo NUNCA logado.
- **RNF-03** (leveza) — leitura **ao vivo** nesta v1: **nenhuma tabela/migração nova**, nenhum
  cache local de eventos, nenhum write-back. (Constituição §6.)
- **RNF-04** — O `/reports/week` (ângulos 1–3) permanece **intocado** e sem chamada ao Google
  no seu caminho (o ângulo 4 é endpoint separado — preserva o orçamento de latência da 07).

## 5. Pré-condições externas (Diogo — `PENDENCIAS-DIOGO.md`)

- **P-02** (feito pelo Diogo, 14/06): Google Calendar API habilitada + app OAuth publicado.
  Pendência derivada a confirmar: o **escopo `calendar.readonly` precisa estar listado na
  OAuth consent screen** do projeto, senão o `connect` real falha. App em *Testing* expira
  refresh tokens em ~7 dias (CA-09 quebraria semanalmente) → **publicar In production**.
  Nada disso bloqueia o build/testes (Google stubbado).

## 6. Mapa pro problema central (REQUISITOS)

- §4.3 (Google Calendar) — entregue **parcialmente**: a metade de leitura (GCal → app).
  O write-back (a outra metade do "2-way") fica registrado como fora desta fase.
- §4.5 ângulo 4 (Planejado × real) — entregue.

## 7. Fora de escopo (fronteiras explícitas)

- **Write-back** (time entries → eventos no GCal) — a 2ª metade do sync 2-way. Fase futura.
- **Multi-agenda** (ler além da `primary`) e seletor de agendas.
- **Sync incremental / webhooks / push notifications** do Google; **cache local** de eventos
  em tabela. (v1 lê ao vivo.)
- **Edição/criação** de eventos pela UI; cores por evento; convidados/RSVP.
- Calendário dos **colaboradores** num workspace compartilhado (cada um conecta o seu; cruzar
  agendas alheias é multiplayer — fora do MVP).
