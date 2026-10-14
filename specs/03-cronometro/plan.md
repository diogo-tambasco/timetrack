# plan.md — Fase 03 · Cronômetro

> Como implementar a `spec.md` desta pasta. Herda o baseline do `SDD.md §7`
> (Go `net/http` + chi · Postgres/pgx · React+Vite+TS+Tailwind PWA · estrutura de pastas §7.2).
> Convenções: rotas `/api/v1/`, sessão por cookie httpOnly, erros `{"error":{"code","message"}}`,
> migrações em `backend/internal/db/migrations/NNNN_nome.sql` aplicadas no boot.

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Servidor é o relógio.** `started_at`/`stopped_at` sempre `now()` do backend; `duration_seconds` calculado só no stop/edição. Cliente recebe `server_time` em toda resposta de estado e deriva o tick localmente (offset de skew). | iPad dormindo, aba suspensa e relógio errado do device não podem corromper o registro (CA-02, CA-21, RNF-02). |
| D-02 | **Pause = stop + clone no resume.** Pausar **encerra** a entry atual (segmento fechado, imutável) e o resume cria uma **nova** entry clonada. Nada de entry "congelada" com buracos. | Duração vira aritmética trivial (`stopped_at − started_at`), relatórios da fase 07 somam segmentos sem caso especial, e o histórico mostra a verdade (trabalhou 2×25min, não 50min corridos). É o modelo do Toggl. (CA-07, CA-09) |
| D-03 | **Tabela `timer_states` (1 linha por usuário)** guarda o estado de runtime (modo, alvo, pausa, o que retomar) separado de `time_entries`. **É detalhe interno desta fase** — não consta no contrato canônico entre as specs; o contrato público é o objeto `TimerState` do `GET /timer` (inclusive a semântica "durante a pausa NÃO há entry rodando", uma extensão sobre o canônico "1 entry com `stopped_at` nulo"). A fase 04 consome **só** `timer.Service`/`OnChange` e esse `TimerState` — **nunca lê a tabela direto**. | Estado de countdown/pausa não é entry (pausa não vira registro de tempo). Persistido → sobrevive a restart do server e a reload da página (CA-37). PK em `user_id` reforça "1 timer por usuário" no banco. Encapsular atrás do `TimerState` protege a fase 04 de conhecer schema interno. |
| D-04 | **Unique parcial em `time_entries (user_id) WHERE stopped_at IS NULL`.** | CA-03 exige a garantia no banco, não só na aplicação — 2 starts concorrentes não podem deixar 2 entries rodando. |
| D-05 | **Toda mutação de timer passa por `internal/timer.Service`**, que executa em transação e retorna sempre o mesmo `TimerState` canônico do `GET /timer`. | RNF-05: a fase 04 só pluga `OnChange(userID, state)` nesse serviço pro broadcast WebSocket — zero refatoração aqui. |
| D-06 | **Transições por fim de alvo são disparadas pelo cliente** (countdown a zero → `stop`; foco Pomodoro a zero → `pause`), e o servidor valida/recalcula com o próprio relógio. | Sem WebSocket (fase 04) o servidor não tem como empurrar o evento; um job no servidor seria complexidade morta que a fase 04 substituiria. O dado nunca corrompe: a duração final é sempre do relógio do servidor (CA-33, CA-34). |
| D-07 | **Rotas de timer são por usuário** (`/api/v1/timer/...`, workspace vai no payload do start — mas o handler **valida membership** do `workspace_id` via repositório da fase 02: não-membro → `404` (CA-29.1), `viewer` → `403` (CA-29.2)); **entries são aninhadas em workspace** (`/api/v1/workspaces/{workspaceId}/entries...`), seguindo o padrão **já fixado** pela fase 02 (D-01/D-02 de lá): cadeia sessão → membership → role gate, que valida 1 vez e injeta `workspace_id` no contexto. | "Máx 1 timer por usuário" é regra global do usuário, atravessa workspaces. Entry pertence a workspace → o middleware de membership torna impossível "esquecer" o filtro (CA-01/29.1) e o builder recebe **um único padrão de rota**, sem condicional. |
| D-08 | **`pomodoro_settings` por usuário** (não por workspace), com defaults aplicados na leitura quando não há linha. | A pausa é hábito da pessoa, vale em qualquer workspace/device (US-10, CA-19). |
| D-09 | **Enum `entry_source` já nasce com `'calendar'` e `'claude'`**, sem uso nesta fase. | Fases 06 e 08 só inserem com outro source — schema não bloqueia o roadmap (regra de escopo). |

## 2. Modelo de dados / migrações

> `NNNN` = próximos números livres após as migrações das fases 00–02. Nomes em inglês,
> arquivos em `backend/internal/db/migrations/`.

### `NNNN_time_entries.sql`

```sql
CREATE TYPE entry_source AS ENUM ('timer', 'manual', 'calendar', 'claude');

CREATE TABLE time_entries (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     uuid NOT NULL REFERENCES workspaces(id),
  user_id          uuid NOT NULL REFERENCES users(id),
  project_id       uuid REFERENCES projects(id),
  task_id          uuid REFERENCES tasks(id) ON DELETE SET NULL,  -- task tem hard delete (fase 02 D-03/CA-21): entry preserva o tempo, perde o vínculo
  description      text NOT NULL DEFAULT '',
  tags             text[] NOT NULL DEFAULT '{}',
  started_at       timestamptz NOT NULL,
  stopped_at       timestamptz,
  duration_seconds integer,          -- calculado pelo servidor ao parar/editar
  source           entry_source NOT NULL,
  CONSTRAINT entry_times_valid CHECK (stopped_at IS NULL OR stopped_at > started_at)
);

CREATE INDEX idx_entries_ws_day ON time_entries (workspace_id, started_at DESC);
CREATE INDEX idx_entries_user   ON time_entries (user_id, started_at DESC);
-- CA-03: no máximo 1 entry rodando por usuário, garantido pelo banco
CREATE UNIQUE INDEX uq_entries_one_running
  ON time_entries (user_id) WHERE stopped_at IS NULL;
```

### `NNNN_pomodoro_settings.sql`

```sql
CREATE TABLE pomodoro_settings (
  user_id       uuid PRIMARY KEY REFERENCES users(id),
  focus_minutes integer NOT NULL DEFAULT 25 CHECK (focus_minutes BETWEEN 15 AND 90),
  break_minutes integer NOT NULL DEFAULT 5  CHECK (break_minutes BETWEEN 1 AND 60),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
```

### `NNNN_timer_states.sql`

```sql
CREATE TYPE timer_mode AS ENUM ('countup', 'countdown', 'pomodoro_focus', 'break');

CREATE TABLE timer_states (
  user_id                uuid PRIMARY KEY REFERENCES users(id),  -- 1 estado por usuário
  mode                   timer_mode NOT NULL,
  -- ON DELETE SET NULL = defesa em profundidade; a API bloqueia antes com 409 (CA-24.1/24.2)
  entry_id               uuid REFERENCES time_entries(id) ON DELETE SET NULL,  -- NULL quando mode='break'
  target_seconds         integer,            -- alvo (countdown/pomodoro_focus/break)
  started_at             timestamptz NOT NULL,
  -- contexto pra retomar depois da pausa:
  resume_entry_id        uuid REFERENCES time_entries(id) ON DELETE SET NULL,  -- entry a clonar no resume
  resume_mode            timer_mode,                        -- modo a restaurar
  resume_target_seconds  integer                            -- restante do alvo (CA-10)
);
```

**Ciclo de vida:** start → linha em `timer_states` apontando pra entry rodando · stop → linha
removida · pause → entry fechada, linha vira `mode='break'` com `resume_*` preenchidos ·
resume/skip → nova entry clonada, linha volta pro modo de trabalho.

**Integridade com o CRUD de entries (CA-24.1/24.2):** `PATCH`/`DELETE` na entry rodando →
`409 entry_running` (toda mutação do timer ativo passa pelo `timer.Service`); `DELETE` na entry
que é `resume_entry_id` de pausa ativa → `409 entry_in_use`. O `ON DELETE SET NULL` acima é só
rede de segurança contra caminhos fora da API — nunca o comportamento esperado.

## 3. Contratos de API

Todas as rotas exigem sessão (cookie httpOnly) → sem sessão: `401
{"error":{"code":"unauthorized","message":"sessão ausente ou expirada"}}` (CA-30).
Erros de validação: `400 {"error":{"code":"validation_error","message":"..."}}` — mesmo
status e slug da fase 02 (convenção única de validação nas 4 fases).

### 3.1 `GET /api/v1/timer` — estado canônico (CA-21/22/23)

`200` rodando (countup):
```json
{
  "state": {
    "mode": "countup",
    "entry": {
      "id": "9f1c…", "workspace_id": "a2b3…", "description": "Revisão de laudas — lote 14",
      "project_id": "c4d5…", "task_id": null, "tags": ["deep-work"],
      "started_at": "2026-06-10T14:00:00Z", "source": "timer"
    },
    "elapsed_seconds": 5070,
    "target_seconds": null,
    "remaining_seconds": null
  },
  "server_time": "2026-06-10T15:24:30Z"
}
```
`200` em pausa Pomodoro:
```json
{
  "state": {
    "mode": "break",
    "entry": null,
    "elapsed_seconds": 28,
    "target_seconds": 300,
    "remaining_seconds": 272,
    "resume": { "entry_id": "9f1c…", "mode": "pomodoro_focus", "target_seconds": 900 }
  },
  "server_time": "2026-06-10T15:25:00Z"
}
```
`200` ocioso: `{"state": null, "server_time": "2026-06-10T15:26:00Z"}`

Pausa **expirada** sem ação do usuário → segue `mode='break'` com `remaining_seconds: 0`
(clampado, nunca negativo — CA-22.1), até resume/skip/stop ou o auto-resume do front (CA-34.1).

### 3.2 `POST /api/v1/timer/start` (CA-04/05/12/13/20/27/29)

```json
{
  "workspace_id": "a2b3…",
  "description": "Revisão de laudas — lote 14",
  "project_id": "c4d5…",
  "task_id": null,
  "tags": ["deep-work"],
  "mode": "countdown",
  "target_seconds": 1200
}
```
- `mode`: `countup` (default) | `countdown` (exige `target_seconds` > 0, senão `400`) |
  `pomodoro` (servidor define `target_seconds = focus_minutes × 60`; estado fica `pomodoro_focus`).
- **Mapeamento explícito de modo:** `mode` da API `"pomodoro"` ↔ enum `timer_mode` do banco
  `'pomodoro_focus'` (e vice-versa na serialização do `TimerState`). O handler traduz nos dois
  sentidos — nunca vazar `pomodoro_focus` na API nem gravar `pomodoro` no banco.
- `workspace_id` de workspace que o usuário **não integra** → `404 not_found`, sem expor dados
  (CA-29.1, validação de membership reusando o repositório da fase 02). Role `viewer` nesse
  workspace → `403 forbidden` (CA-29.2).
- `user_id` não existe no payload: a entry nasce sempre em nome do usuário da sessão (CA-03.1).
- Timer já rodando → servidor **para o atual e inicia o novo** na mesma transação (CA-05).
- `project_id`/`task_id` de outro workspace → `404 {"error":{"code":"not_found",...}}` (CA-29).
- Retorna `201` com o mesmo corpo do `GET /timer`.

### 3.3 `POST /api/v1/timer/stop` (CA-06)

Sem body. `200`:
```json
{
  "stopped_entry": { "id": "9f1c…", "started_at": "2026-06-10T14:00:00Z",
                     "stopped_at": "2026-06-10T15:24:30Z", "duration_seconds": 5070, "...": "…" },
  "state": null,
  "server_time": "2026-06-10T15:24:30Z"
}
```
Sem timer **nem pausa** ativa → `409 {"error":{"code":"no_active_timer",...}}`.
Durante uma pausa, `stop` encerra a pausa e deixa ocioso — sem retomar, sem criar entry (CA-06.1).
Stop após o alvo vencido (overrun) grava a duração **real**, sem clamp (CA-13.1).

### 3.4 `POST /api/v1/timer/pause` (CA-07/08/25) — **comportamento central**

Body opcional: `{"break_minutes": 15}` (preset 5/15/20 ou custom 1–60; ausente → config
persistida do usuário; fora da faixa → `400` **e o timer de trabalho segue rodando intacto,
nada é fechado** — CA-25.1). `200`: corpo do `GET /timer` com `state.mode="break"`,
`remaining_seconds = break_minutes × 60` e `stopped_entry` incluída.
Sem timer rodando → `409 no_active_timer` (CA-25).

### 3.5 `POST /api/v1/timer/resume` · `POST /api/v1/timer/break/skip` (CA-09/10/11/26)

Sem body. Encerram a pausa (skip = imediatamente, sem esperar zerar) e iniciam **nova** entry
clonando `description`/`project_id`/`task_id`/`tags` da entry referida em `resume_entry_id`,
restaurando `resume_mode` e `resume_target_seconds` (restante do alvo — CA-10).
`200` com corpo do `GET /timer`. Sem pausa ativa → `409 {"error":{"code":"no_active_break",...}}`.

### 3.6 Entries — aninhadas em workspace (padrão da fase 02)

Prefixo: **`/api/v1/workspaces/{workspaceId}/entries`**. Toda rota passa pela cadeia da fase 02:
sessão → **membership** (não-membro → `404` sem expor dados, CA-29.1) → **role gate** (`viewer`
em método de escrita → `403`, CA-29.2), que injeta `workspace_id` validado no contexto.

- **`GET /api/v1/workspaces/{workspaceId}/entries?from=2026-06-10&to=2026-06-10&tz=America/Sao_Paulo`**
  — `from`/`to` default = **hoje no fuso IANA de `tz`** (ausente → UTC; o front **sempre envia**
  `Intl.DateTimeFormat().resolvedOptions().timeZone` — CA-14.1/35.1). Sempre filtra por
  `workspace_id` (CA-01) **e** por `user_id` da sessão — a v1 lista só entries do próprio
  usuário (CA-01.1). `200 {"entries":[ … ], "total_seconds": 18540}` — ordenado por
  `started_at DESC`, entry rodando primeiro com `stopped_at: null`.
- **`POST /api/v1/workspaces/{workspaceId}/entries`** (manual — CA-14/24/29):
  ```json
  { "description": "Call Arkan", "project_id": "e6f7…",
    "task_id": null, "tags": [], "started_at": "2026-06-10T09:00:00Z",
    "stopped_at": "2026-06-10T09:45:00Z" }
  ```
  `201` com a entry (`source:"manual"`, `duration_seconds:2700` calculado no servidor,
  `user_id` = sessão — CA-03.1). `stopped_at ≤ started_at` → `400`. `duration_seconds` no
  payload é ignorado (CA-02).
- **`PATCH /api/v1/workspaces/{workspaceId}/entries/{id}`** (CA-15) — campos parciais
  (`description`, `project_id`, `task_id`, `tags`, `started_at`, `stopped_at`); mudou horário →
  recalcula duração. `200` com a entry. Entry de outro workspace, inexistente **ou de outro
  usuário** → `404` (CA-29.3). Entry rodando → `409 entry_running` (CA-24.1).
- **`DELETE /api/v1/workspaces/{workspaceId}/entries/{id}`** (CA-16) — `204`. Fora do
  workspace / de outro usuário → `404` (CA-29.3). Entry rodando → `409 entry_running`
  (CA-24.1); entry que é `resume_entry_id` de pausa ativa → `409 entry_in_use` (CA-24.2).
- **`POST /api/v1/workspaces/{workspaceId}/entries/{id}/continue`** (CA-17) — sem body; clona e
  inicia count up (`source:"timer"`), aplicando a regra do CA-05. Só entries próprias
  (CA-29.3). `201` com corpo do `GET /timer`.

### 3.7 `GET|PUT /api/v1/settings/pomodoro` (CA-18/19/28)

- `GET` → `200 {"focus_minutes": 25, "break_minutes": 5}` (defaults se nunca salvou — CA-19).
- `PUT {"focus_minutes": 50, "break_minutes": 15}` → `200` com os valores salvos.
  Fora dos limites (focus 15–90, break 1–60) → `400`, nada persiste (CA-28).

## 4. Libs (leveza é requisito)

**Go — nada novo além do baseline:** `chi` (router, já decidido), `jackc/pgx/v5` (driver +
pool; queries à mão, sem ORM), `google/uuid`. Cálculo de duração e countdown = `time` da
stdlib. Testes: `testing` + `net/http/httptest`; `testify` só se encurtar asserts.

**Front — nada novo além do baseline:** `zustand` (store do timer), `fetch` nativo no api
client (`lib/api.ts`), datas com `Date`/`Intl` nativos. Tick: 1 `setInterval` de 1s no store
derivando de `started_at + offset` (RNF-02/03). Testes: Vitest (+ fake timers pros CA-33/34) e
Playwright.

## 5. Backend — onde vive o quê (estrutura SDD.md §7.2)

```
backend/internal/
  entries/    repositório + regras de entry (CRUD manual, listagem por dia, clone)
  timer/      Service: Start/Stop/Pause/Resume/SkipBreak/State (transacional, retorna TimerState)
              hook OnChange(userID, TimerState) — no-op nesta fase; fase 04 registra o hub WS
  api/        handlers REST /timer, /workspaces/{id}/entries, /settings/pomodoro
              + middleware de sessão (fase 01) + membership/role gate (reuso da fase 02)
  db/         migrations/NNNN_{time_entries,pomodoro_settings,timer_states}.sql
```

## 6. Frontend — componentes e referência de design

Tokens de `Design/Timmingtrack.life/timetrack/tokens.css` são a fonte de verdade visual
(mapeados no Tailwind). Dark-first, accent default teal `#2dd4bf`, Geist + Geist Mono
**self-hosted** (o `@import` do Google Fonts do protótipo NÃO vai pra prod). Sidebar = rail
64px com expansão overlay 232px no hover (`.tt-sidenav`, canônico — DESIGN-additions.md).

| Componente | Arquivo | Referência de design |
|---|---|---|
| `TimerBar` (barra global, topo do cockpit, sempre visível) | `frontend/src/components/timer/TimerBar.tsx` (evolui o placeholder da fase 00 — mesmo arquivo, sem duplicar) | `cockpit.jsx` → `TimerBar` (header 66px em `--surface-1`): input de descrição transparente, `.tt-chip`+`.tt-dot` do projeto, dígitos `.mono` 22px `tabular-nums`, `.tt-iconbtn--go`/`--stop`; rodando → `.tt-pulse` no botão |
| `BreakOverlay` (estado de pausa no cockpit) | `frontend/src/components/timer/BreakOverlay.tsx` | `foco.jsx` → `FocoBreak` (referência de **comportamento**: countdown grande, chips de preset 5/15/20, "pular pausa", play pra retomar) em versão compacta — tint `--accent-soft`, dígitos `.mono`. O fullscreen é fase 05 |
| `TodayList` (lista "Hoje") | `frontend/src/components/timer/TodayList.tsx` | `cockpit.jsx`/`parts.jsx` → `Timesheet`: linha com `.tt-dot`, descrição, projeto, início e duração `.mono`, botão play (continuar)/stop; rodando em `--accent`; "Total hoje" no cabeçalho |
| `EntryModal` (nova entrada / edição manual) | `frontend/src/components/timer/EntryModal.tsx` | botão "Nova entrada" (`.tt-btn--accent` + `Icon.plus`) do `cockpit.jsx`; campos em `.tt-card`/`.tt-panel` |
| `PomodoroSettings` (grupo nos Ajustes) | `frontend/src/components/settings/Pomodoro.tsx` (seção montada na página `routes/cockpit/Settings.tsx` da fase 00) | `settings.jsx` → `SGroup "Pomodoro"`: slider de foco 15–90 (step 5) + presets de pausa 5/15/20 com borda/`--accent-soft` no ativo + campo custom. **Atenção: o campo custom NÃO existe no protótipo `settings.jsx` (que só tem os 3 presets) — é adição exigida pelo `DESIGN.md §4`; seguir o estilo dos inputs do protótipo** |
| `useTimerStore` (zustand) | `frontend/src/lib/timerStore.ts` | estado = resposta do `GET /timer` + `clockOffset`; ações chamam a API e gravam o estado retornado; dispara auto-stop/auto-pause no zero (CA-33/34) |

## 7. Pontos de integração (baseline e fases vizinhas)

- **Fase 01 (auth):** middleware de sessão reutilizado em todas as rotas; `user_id` do contexto.
- **Fase 02 (domínio):** FKs `project_id`/`task_id`; validação de pertencimento ao workspace
  reaproveita os repositórios de projects/tasks; entries usam as rotas aninhadas + cadeia de
  middlewares sessão → membership → role gate **já fixadas** pela 02 (D-07).
- **Fase 04 (sync):** consome `timer.Service.OnChange` + serializa o mesmo `TimerState` do
  `GET /timer` como payload dos eventos WS. Nenhuma rota desta fase muda (RNF-05). A fase 04
  **não lê `timer_states` direto** — a tabela é interna; o contrato é o `TimerState` (D-03),
  incluindo a semântica "pausa = sem entry rodando, estado vive no `TimerState`".
- **Fase 05 (Modo Foco):** consome os mesmos endpoints e o `useTimerStore`; `FocoBreak`/`FocoWork`
  são a versão fullscreen dos mesmos estados.
- **Fase 06 (Calendar):** insere entries com `source='calendar'` — enum já pronto (D-09).
- **Fase 07 (relatórios):** agrega `time_entries` fechadas; segmentos do D-02 somam direto.
- **Fase 08 (conector Claude):** start via token `tt_` com `source='claude'` — `timer.Service`
  já aceita o source como parâmetro; nada aqui bloqueia.
- **Deploy:** nada novo — migrações entram no migrate-no-boot da fase 00; rodar **local** até a 09.
