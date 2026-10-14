# plan.md — Fase 21 · Presence ao vivo (broadcast por workspace)

> Como construir a spec. Decisões de arquitetura (D-xx) + contratos + ordem de tarefas.
> Reusa o **Hub** (`realtime/hub.go`, fase 04), o hook **`OnChange`** do `timer.Service` (03/04),
> a regra de **visibilidade de projeto** (`projects/service.go::visibleWhere`, fase 02), a página
> **`/workspace`** (fase 10) e o **`presenceStore`/`realtime.ts`** padrão do front. **Sem migração.**

---

## 1. Arquitetura — a decisão central (avaliada e travada)

**Problema:** o Hub é per-usuário (`users map[string]map[Sink]struct{}`). Presence precisa que a
mudança de timer de A alcance os **outros membros dos workspaces de A**.

**Duas opções avaliadas:**

- **Opção A (REFATORAR o hub):** indexar o hub também por workspace
  (`workspaces map[wsID]map[Sink]struct{}`) e dar um `BroadcastWorkspace(wsID, msg)`. **Descartada:**
  mexe no core de realtime (fase 04 em produção, com sync de timer crítico), exige rastrear a
  membership de cada conexão no `Register`/`Unregister`, e arrisca o RNF-04 (não-regressão da 04). É
  o caminho com maior superfície de bug no componente mais sensível.
- **Opção B (resolver fora do hub — ESCOLHIDA, D-01):** manter o Hub per-usuário **intocado** e
  adicionar um **`presence.Service`** que, no `OnChange(A, …)`, **resolve** os membros dos
  workspaces compartilhados de A (query em `workspace_members`) e chama
  `hub.Broadcast(memberUserID, presenceMsg)` pra **cada** membro — per-usuário, exatamente a API que
  o hub já expõe. O fan-out por workspace vira uma **resolução de banco**, não uma estrutura nova no
  hub. Espelha como o `BroadcastNotice` (fase 16) reusou `Broadcast([]byte)`.

**Por que B é a certa aqui:** o hub não muda (RNF-02/04); o eixo de workspace vive no
`presence.Service` (testável isolado com pool real + `Sink` fake); a visibilidade do projeto se
resolve junto, na mesma query, no lugar certo (perto do banco). O custo é uma query indexada por
mudança de timer — desprezível na escala do app (RNF-01).

```
Backend:
  internal/presence/                 (NOVO pacote)
     service.go                              ← OnTimer(userID, state, at): resolve membros + visibilidade
                                               + hub.BroadcastPresence(memberID, payload); Snapshot(ws, viewer)
     service_test.go                         ← TDD: fan-out, isolamento, visibilidade priv/shared, running/idle
  internal/realtime/hub.go (existe)          ← + BroadcastPresence (envelope type:"presence") — ADITIVO
     presence_test.go                        ← entrega só às conns do user; intocado o resto
  internal/api/presence_routes.go    (NOVO)  ← GET /workspaces/{id}/presence (snapshot, aninhado no ws)
     presence_routes_test.go                 ← 200 lista própria / 404 não-membro / 401 sem sessão / isolamento
  internal/api/auth_routes.go (existe)       ← monta o /presence no subrouter do workspace (RequireMember)
  cmd/server/main.go (existe)                ← compõe OnChange: BroadcastTimer + presence.OnTimer (D-04)

Frontend:
  lib/realtime.ts (existe)                   ← + onPresence (trata type:"presence"; timer/notice INTOCADOS)
  lib/presenceStore.ts            (NOVO)     ← zustand: membros ativos por workspace (running:true add, false remove)
  lib/api.ts (existe)                        ← + getWorkspacePresence(wsId)
  components/PresenceStrip.tsx     (NOVO)    ← faixa de membros ativos (nome + projeto + "há Xmin")
  routes/cockpit/Workspace.tsx (existe)      ← monta a PresenceStrip (aditivo)
  routes/cockpit/Dashboard.tsx (existe)      ← monta a PresenceStrip (aditivo, opcional)
  lib/timerStore.ts (existe)                 ← attachTimerSync passa onPresence ao connectTimerSocket
```

## 2. Sem migração

Presence é **efêmero** (deriva do `OnChange` + hub em memória + snapshot de leitura). **Nenhuma
tabela, nenhum `0021_*`.** A T-01 verifica que `backend/internal/db/migrations/` segue inalterado e
registra "21 = sem migração" no INDEX. (Migrações donas: 0017→17, 0018→18, 0019→19, 0020→20; 21
**sem**.)

## 3. Backend — `presence.Service`

### D-02 — pacote novo `presence`, NÃO dentro de `realtime` nem `timer`

O fan-out reusa o Hub (transporte) e o `OnChange` (gatilho), mas é uma **preocupação de domínio**
(resolver membership + visibilidade de projeto) — pertence a um pacote próprio que importa
`realtime` (pro `BroadcastPresence`) e recebe o `*pgxpool.Pool`. **Não** incha o `timer.Service`
nem o `realtime.Hub`. (Espelha o `guard`/`push` da fase 16.)

```go
type Service struct {
    db        *pgxpool.Pool
    broadcast func(memberUserID string, payload any)  // = hub.BroadcastPresence
    now       func() time.Time                        // injetável (testes) — default time.Now
}

// OnTimer é plugado no OnChange do timer (D-04). userID = A; state = novo estado (nil=ocioso).
// Pós-commit, best-effort: erro de query é LOGADO e ENGOLIDO (CA-10/RNF-01).
func (s *Service) OnTimer(userID string, state *timer.State, at time.Time)

// Snapshot lista os membros do workspace com timer rodando AGORA, sob a regra de
// visibilidade do projeto pro viewer (CA-11/14). Só running:true (D-05).
func (s *Service) Snapshot(ctx context.Context, workspaceID, viewerID string) ([]PresencePayload, error)
```

### `OnTimer` — o fan-out (CA-02/03/04/05/06/07/08/09/10)

1. **Deriva `running`/`since`/projectID/entryWorkspaceID** do `state` (sem tocar o banco):
   - `state == nil` → `running:false` (ocioso/stop/corte — CA-09); **sem workspace no state**.
   - `state.Mode == "break"` → `running:false` (pausa Pomodoro — CA-08).
   - `state.Mode ∈ {countup,countdown,pomodoro}` e `state.Entry != nil` → `running:true`,
     `since = state.Entry.StartedAt`, `projectID = state.Entry.ProjectID` (pode ser nil — CA-07),
     **`entryWorkspaceID = state.Entry.WorkspaceID`** (a entry roda em UM workspace só).
2. **Resolve os co-membros que devem receber o presence** (CA-02/02.1/03), numa query — e o escopo
   depende de `running`:
   - **`running:true`** → fan-out **só pro workspace DA ENTRY** (`entryWorkspaceID`): a self-join
     filtra `m1.workspace_id = $entryWorkspaceID` (não todas as memberships de A). Isso impede o vazamento
     de `running:true` pros co-membros de outros workspaces de A (CA-02.1) e mantém o delta byte-igual
     ao snapshot daquele ws (D-05):
     ```sql
     SELECT m2.user_id
       FROM workspace_members m1
       JOIN workspace_members m2 ON m2.workspace_id = m1.workspace_id
      WHERE m1.user_id = $A
        AND m1.workspace_id = $entryWorkspaceID   -- SÓ o workspace da entry (CA-02.1)
        AND m2.user_id <> $A                       -- A não recebe presence de si (D-03)
     ```
     Cada linha = (membro B do workspace da entry) que deve receber presence `running:true` de A.
   - **`running:false`** → derruba A em **TODAS as memberships de A** (não há workspace no state em
     stop/corte; e na pausa-Pomodoro o que importa é cancelar o `running:true` anterior onde quer que
     tenha ido). Idempotente — remover A de uma faixa onde ele não estava é no-op (CA-04/CA-16):
     ```sql
     SELECT m2.user_id, m2.workspace_id
       FROM workspace_members m1
       JOIN workspace_members m2 ON m2.workspace_id = m1.workspace_id
      WHERE m1.user_id = $A AND m2.user_id <> $A   -- todas as memberships de A; A não recebe de si
     ```
     Cada (B, W) recebe `running:false` com `workspace_id=W` (o store remove A da faixa de W).
3. **Resolve o nome do projeto sob visibilidade** (CA-05/06/07): se `projectID != nil`, fazer
   **uma** query do nome do projeto **com a regra `visibleWhere`** parametrizada pelo **viewer B**:
   ```sql
   SELECT name FROM projects
    WHERE id = $projectID AND workspace_id = $W AND (visibility='shared' OR owner_user_id = $B)
   ```
   - achou → `payload.project = name` (CA-05).
   - não achou (privado de A, ou projeto de outro ws) → **omite** `project` (CA-06).
   > Otimização permitida (D-06): como, dentro de UM workspace, todos os membros enxergam um
   > `shared` igual e um `private` igual (B nunca é dono do projeto de A), o nome pode ser resolvido
   > **uma vez por (workspace, projeto)** — `shared` → nome pra todos; `private`/sem-projeto → omite
   > pra todos. Isso evita N queries de projeto por evento. O **resultado observável** (CA-05/06)
   > é idêntico; a query parametrizada por B acima é a forma de referência (mais simples de pinar
   > no teste). Implementação pode usar a forma agregada desde que os CAs passem.
4. **Empurra** pra cada (B, W): `payload := PresencePayload{UserID:A, Name:nameA, WorkspaceID:W,
   Running:running, Project:proj?, Since:since?}` → `s.broadcast(B, envelope(payload))`. O
   `BroadcastPresence` do hub embrulha em `{"type":"presence","payload":…}`.
5. **best-effort (CA-10/RNF-01):** todo o `OnTimer` roda dentro de um `defer recover()`/erro
   engolido — uma falha de query **nunca** propaga (o `OnChange` é chamado dentro do
   `timer.Service`, mas o hook já é pós-commit e o `timer.Service` não checa o retorno). Log via
   `slog`.

> **`nameA`** (nome de exibição de A): resolver junto (join `users u ON u.id = $A`) ou numa query
> leve; cacheável, mas a versão simples (1 query) basta (RNF-01).

### D-03 — A não recebe presence de si

O filtro `m2.user_id <> $A` exclui A do fan-out: a barra de timer de A já reflete o estado dele pelo
`type:"timer"` (fase 04). Mandar presence de A pra A seria redundante e poderia confundir a faixa
(A não deve aparecer "rodando" na própria lista de presence dos OUTROS). Pinado em teste.

### D-05 — snapshot só traz `running:true`

`GET /workspaces/{id}/presence` lista **apenas** os membros com timer rodando agora (não os
ociosos/em pausa) — a faixa mostra quem está ativo. Query:
```sql
SELECT e.user_id, u.name, e.started_at, e.project_id,
       p.name AS project_name, p.visibility, p.owner_user_id
  FROM time_entries e
  JOIN workspace_members wm ON wm.workspace_id = $ws AND wm.user_id = e.user_id  -- membro de $ws
  JOIN users u ON u.id = e.user_id
  LEFT JOIN projects p ON p.id = e.project_id
 WHERE e.workspace_id = $ws AND e.stopped_at IS NULL
```
- Filtro **`e.workspace_id = $ws`** + join de membership = isolamento de tenant (CA-14). A entry
  rodando de um membro num OUTRO workspace dele não aparece aqui (ela tem outro `workspace_id`).
- **Visibilidade aplicada no service** (não no SQL, pra ficar idêntico ao `OnTimer`): se
  `visibility=='private'` e `owner_user_id != viewerID` → omite `project_name`. `shared` → mantém.
  Sem projeto → sem `project`. `since = started_at`; `running = true` (todos da lista).
  > **Atenção (D-07):** uma entry rodando ≠ "trabalho rodando" se o usuário está em pausa Pomodoro
  > (no `mode='break'` NÃO há entry rodando — fase 03). Logo `stopped_at IS NULL` já exclui pausas.
  > Mas o snapshot vê a entry direto (não o `timer_states`), então é consistente com o `OnTimer`
  > (que deriva `running:false` no break porque o state não tem entry rodando).

### `Hub.BroadcastPresence` (aditivo — CA-19, RNF-02/04)

```go
// hub.go (fase 04/16, INTOCADO no resto): só adiciona este método (espelha BroadcastNotice).
func (h *Hub) BroadcastPresence(userID string, payload any) {
    evt, err := json.Marshal(struct {
        Type    string `json:"type"`
        Payload any    `json:"payload"`
    }{Type: "presence", Payload: payload})
    if err != nil { return }
    h.Broadcast(userID, evt)
}
```
- O `Broadcast`/`BroadcastTimer`/`BroadcastNotice`/`Sink` **não mudam** (CA-19). Mesmo
  `Broadcast([]byte)` por usuário, só com `type:"presence"`.

## 4. Contratos de API

### Snapshot — `GET /api/v1/workspaces/{workspaceId}/presence` (CA-11/12/13/14)

- **Aninhado no workspace** (convenção INDEX): atravessa sessão→membership pelo `RequireMember`
  existente (fase 01/02). Caller não-membro / ws inexistente → **404** uniforme (CA-12). Sem sessão
  → **401** canônico (CA-13).
- **200** →
  ```json
  { "members": [
      { "user_id":"…", "name":"Cadu", "workspace_id":"…", "running":true,
        "project":"Cliente X — Site", "since":"2026-06-21T14:03:00Z" },
      { "user_id":"…", "name":"Bia",  "workspace_id":"…", "running":true,
        "since":"2026-06-21T13:50:00Z" }
  ] }
  ```
  (segundo membro sem `project` = projeto privado ou sem projeto.) Lista vazia = `{"members":[]}`
  (ninguém ativo — CA-17 mostra a faixa vazia).
- O `viewerID` da regra de visibilidade = `reqctx.UserID` da sessão (o próprio caller).

### Mensagem WebSocket — envelope `presence` (push, CA-02/15)

```json
{ "type":"presence",
  "payload": { "user_id":"…", "name":"Cadu", "workspace_id":"…",
               "running":false } }
```
- Chega aos membros de cada workspace compartilhado de A quando o timer de A muda. `running:false`
  remove A da faixa daquele workspace (CA-16); `running:true` adiciona/atualiza (com `project?`/
  `since?`).

### Erros (convenção INDEX)

- Sem sessão → `401 {"error":{"code":"unauthorized",…}}`.
- Não-membro / ws inválido → `404 {"error":{"code":"not_found",…}}` (não vaza).
- (Presence não tem escrita → sem 403/validação de payload nesta fase.)

## 5. Pontos de integração com o baseline

- **`OnChange` composto (D-04):** hoje `cmd/server` faz `tsvc.SetOnChange(hub.BroadcastTimer)`.
  Presence precisa **também** observar. Compor sem regredir o timer:
  ```go
  presenceSvc := presence.NewService(pool, hub.BroadcastPresence)
  tsvc.SetOnChange(func(userID string, st *timer.State, at time.Time) {
      hub.BroadcastTimer(userID, st, at)        // fase 04 — INTOCADO (1º, garante o sync do timer)
      presenceSvc.OnTimer(userID, st, at)       // fase 21 — aditivo, best-effort, não pode falhar
  })
  ```
  O `BroadcastTimer` roda **primeiro** (o sync do dono não pode esperar o fan-out de presence). O
  `OnTimer` é best-effort (CA-10) — engole erro. **Importante:** o `guard.Cutter` da fase 16 usa
  `guard.WithOnTimer(hub.BroadcastTimer)` direto (não o hook do `timer.Service`); pra o corte também
  derrubar a presence (CA-09), o `WithOnTimer` do cutter passa a apontar pro **mesmo hook composto**
  (ou um wrapper que chama os dois). **Decisão:** extrair o hook composto pra uma var (`onTimer :=
  func(...){ BroadcastTimer; presence.OnTimer }`) e usá-la **tanto** no `SetOnChange` **quanto** no
  `guard.WithOnTimer` — assim o corte da 16 também emite presence `running:false`. (Espelha o
  fan-out do notice que já existe no cutter.)
- **`presence_routes.go`** monta `GET /presence` no **subrouter do workspace**
  (`auth_routes.go::mountAuth` / onde o core/reports penduram) — atrás de `RequireMember`. Reusa o
  `routerCfg`/Option pattern (`WithPresence(svc)`).
- **`realtime.ts`** ganha `onPresence?`; `attachTimerSync` passa `onPresence: (p) =>
  presenceStore.apply(p)`. Os ramos `type:"timer"`/`type:"notice"` e o `onResult`/anti-stale
  **não mudam** (CA-15/20).
- **`Workspace.tsx`/`Dashboard.tsx`** montam a `PresenceStrip` (aditivo). O snapshot busca por
  `useQuery(['presence', wsId])` no mount + invalida na reconexão/troca de ws.

## 6. Libs / deps

- **Zero dependência nova** (Go ou JS). Reusa: `realtime.Hub`, `pgxpool`, `chi`, `reqctx`,
  `domain`, `slog` (backend); `zustand`, `@tanstack/react-query`, WebSocket nativo (frontend).
  (RNF-01.)

## 7. Dependências da fase + paralelização

- **Depende de:** **04** (Hub + `realtime.ts` + `timerStore`), **03** (hook `OnChange` do
  `timer.Service` + `entries.Entry` com `project_id`/`started_at`), **10** (página `/workspace`,
  onde a faixa mora), **02** (regra `visibleWhere` de visibilidade de projeto). Todas **já
  construídas e em produção** — presence ancora nelas, não as muda.
- **NÃO depende da fase 17** (convites): membros podem ser semeados por SQL (como hoje e no E2E —
  `MILESTONE-2.md §9`). Presence funciona com qualquer membership existente.
- **PARALELIZÁVEL (worktree)? → NÃO.** Toca o **core de realtime** (`hub.go` aditivo, e
  principalmente o **wiring do `OnChange` em `cmd/server`** que o `BroadcastTimer`/`guard.WithOnTimer`
  compartilham) e a página `/workspace` (que a fase 17 também estende) + dashboard. **Isolar e
  buildar SOZINHO** (`MILESTONE-2.md §9`), depois das fases de migração (17–20) ou em janela
  própria, pra não contender no `cmd/server` e na `/workspace`. A ordem recomendada do PRD: 17/18/
  19/20 em paralelo → **21 isolada** → 22 → 23.

## 8. Armadilhas conhecidas (vetadas)

- **NÃO** indexar o Hub por workspace (Opção A — D-01/RNF-02): refatora o core da fase 04. O
  fan-out vive no `presence.Service`.
- **NÃO** mandar presence de A pra A (D-03): a barra de A já reflete pelo `type:"timer"`; o filtro
  `m2.user_id <> $A` é load-bearing.
- **NÃO** fazer fan-out de `running:true` pra TODOS os workspaces de A (CA-02.1/D-05): a entry roda
  em UM workspace (`state.Entry.WorkspaceID`); o `running:true` só vai pros co-membros DESSE ws
  (`m1.workspace_id = $entryWorkspaceID`). Senão vaza "A ativo" pra co-membros de outro ws de A E
  diverge do snapshot (que filtra `e.workspace_id`) — delta e snapshot têm de concordar byte a byte.
  O `running:false` (sem ws no state) é a exceção: derruba A em TODAS as memberships (idempotente).
- **NÃO** vazar o nome do projeto **privado** (CA-06/RNF-03): aplicar `visibleWhere` (shared OU dono)
  — o mesmo da fase 02. Mutação que ignora a visibilidade deve matar o teste.
- **NÃO** deixar o `OnTimer` falhar/bloquear o timer (CA-10/RNF-01): é pós-commit, best-effort, erro
  engolido. Uma query lenta/erro de presence **nunca** pode derrubar um `Stop`/`Start`.
- **NÃO** mudar o ramo `type:"timer"` nem o anti-stale do `timerStore` ao adicionar `onPresence`
  (CA-15/20): presence é um caminho lateral; o estado do timer converge pelo `type:"timer"`.
- **NÃO** esquecer de compor o `guard.WithOnTimer` (fase 16) com o presence (D-04): senão o **corte**
  da 16 não derruba a presence de A (a entry fechou, mas a faixa segue mostrando A ativo até a
  reconexão/snapshot). Usar o **mesmo hook composto** nos dois lugares.
- **NÃO** listar membros ociosos/em pausa no snapshot (D-05): só `running:true` (`stopped_at IS
  NULL`). Um membro em `break` não tem entry rodando → não aparece (consistente com `OnTimer`).
- **NÃO** esquecer o filtro `e.workspace_id = $ws` + join de membership no snapshot (CA-14): sem
  ele, vazaria a entry rodando de um membro em outro workspace.
- **NÃO** assumir que o E2E cobre A→B (CA-22): o stub tem **1 user**. O cross-user é teste de
  integração Go. Registrar a limitação, não fingir cobertura.

## 9. Ordem de execução (ver tasks.md)

Backend primeiro (verificar sem-migração → `BroadcastPresence` aditivo no hub → `presence.Service`
TDD [fan-out/isolamento/visibilidade/running] → snapshot REST → wiring composto no boot), depois o
frontend (`realtime.ts` onPresence → `presenceStore` → `api.ts` snapshot → `PresenceStrip` na
`/workspace`+dashboard → wiring no `attachTimerSync`), depois E2E (limitado — CA-22), verificação
adversarial e gate. Commit por tarefa. `make check-full` é o gate final.
