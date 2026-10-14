# spec.md — Fase 21 · Presence ao vivo (broadcast por workspace)

> Fonte de verdade da fase. Resolve a dor **P5** do `MILESTONE-2.md §2/§4`: "não vejo o que o
> outro está fazendo agora". Entrega a **decisão arquitetural central da M2** — ver o **timer
> ativo dos membros do workspace AO VIVO**. Hoje o Hub (`realtime/hub.go`) é **estritamente
> per-usuário** (`users map[string]map[Sink]struct{}`); presence introduz o **eixo de workspace**
> SEM refatorar o core do hub.
>
> **Decisões travadas (HITL `MILESTONE-2.md §7.2`, e ancoradas no código real em 21/06):**
> 1. **Tempo real da M2 = SÓ presence** (broadcast por workspace do timer ativo). Cross-assign,
>    notas e status seguem propagando **no reload** (fora desta fase). Presence é o ÚNICO fan-out
>    de domínio por workspace.
> 2. **Fan-out ADITIVO, sem regredir o timer.** O `type:"timer"` per-usuário e o anti-stale do
>    `timerStore` **NÃO mudam** (CA de não-regressão forte). Presence é um envelope **novo**
>    `type:"presence"`, num caminho lateral — espelha como o `type:"notice"` (fase 16) foi
>    adicionado sem tocar o `type:"timer"`.
> 3. **Arquitetura escolhida (avaliada e travada — ver plan §1):** manter o Hub per-usuário e
>    adicionar um **`presence.Service`** que, no `OnChange` do user A, resolve os **membros dos
>    workspaces compartilhados de A** (query em `workspace_members`) e chama
>    `hub.Broadcast(memberUserID, presenceMsg)` pra cada membro — **NÃO** indexa o hub por
>    workspace (isso refatoraria o core e arriscaria o RNF da fase 04).
> 4. **Privacidade por visibilidade de projeto (fase 02/07):** B vê que A está ATIVO; o **nome do
>    projeto** só aparece se B **pode ver** o projeto (`shared`, ou B é o dono). Projeto privado
>    de A **não vaza o nome** — presence reporta `running:true` sem `project`.
> 5. **Sem migração** — presence é **estado efêmero** (deriva do `OnChange` e do hub em memória).

---

## 1. Objetivo

Quando A (membro de um workspace compartilhado) começa, pausa, retoma ou para um timer, os
**outros membros conectados** dos workspaces que A compartilha veem, ao vivo, **quem está rodando o
quê** — numa faixa de presence na página `/workspace` (e no dashboard). Pra isso:

1. **`presence.Service`** (pacote novo `internal/presence`): no `OnChange(A, state, at)` do
   `timer.Service`, resolve os membros dos workspaces compartilhados de A e empurra, pra cada
   membro conectado, um envelope **`type:"presence"`** pelo Hub (`hub.Broadcast(memberUserID, …)`).
   A resolução de visibilidade do projeto decide se o **nome do projeto** entra no payload.
2. **Sem refatorar o Hub:** o Hub continua per-usuário; presence usa o `Broadcast([]byte)` que já
   existe, só com um envelope `type` novo — exatamente como o `BroadcastNotice` (fase 16). O
   `BroadcastTimer`/`Sink`/`Broadcast`/`BroadcastNotice` **não mudam de assinatura**.
3. **Front:** `lib/realtime.ts` ganha `onPresence?` (ramos `type:"timer"`/`type:"notice"`
   INTOCADOS); um `presenceStore` (zustand) consolida o estado por workspace; uma **`PresenceStrip`**
   (faixa/seção) na página `/workspace` e no dashboard lista os membros ATIVOS ao vivo. Reconexão
   re-sincroniza (o re-sync do open chama `load()` do timer, e o presence converge pelos
   `OnChange` subsequentes + um snapshot inicial — ver §2).

Timer (03/04), corte (16), notice (16), GCal/relatórios/foco **seguem intactos**: presence
**adiciona** um caminho lateral, não muda contrato REST nem o ramo `type:"timer"` (RNF-04).

## 2. Modelo de presence (definição precisa)

**Presence = "este membro está com um timer RODANDO?" + (se visível) o projeto.** É um derivado do
estado do timer, NÃO uma fonte nova de verdade:

- O `OnChange(userID, state, at)` do `timer.Service` dispara **após toda mutação commitada**
  (start/stop/pause/resume; e o corte da fase 16 chama o mesmo hook com `state=nil`). Anchor real:
  `service.go` chama `s.onChange(...)` em 5 pontos; `cmd/server` faz `tsvc.SetOnChange(hub.BroadcastTimer)`.
- **`running`** = há entry de trabalho rodando = `state != nil && state.Mode ∈ {countup, countdown,
  pomodoro}` (há `state.Entry`). Em **pausa Pomodoro** (`state.Mode == "break"`, sem entry rodando)
  e em **ocioso** (`state == nil`, ex.: stop/corte) → `running:false`. Pausa e corte **derrubam** a
  presence de A pra inativa (CA-08/09).
- **`project`** (nome) só entra se o observador B **pode ver** o projeto do timer de A: regra
  `visibleWhere` da fase 02 (`projects/service.go:215`): **`(visibility='shared' OR owner_user_id =
  B)`**. Como B nunca é dono de projeto de A num caminho normal, na prática: **`shared` → mostra o
  nome; `private` → omite o nome** (mas ainda reporta `running:true`). O timer **sem projeto**
  (count-up livre) reporta `running:true` sem `project` também.
- **Snapshot vs. delta:** o presence chega por **delta** (cada `OnChange` empurra o novo estado do
  membro que mudou). Pra B que ABRE a página depois de A já estar rodando, há um **snapshot
  inicial** (CA-15): ao conectar/abrir, B busca um `GET /api/v1/workspaces/{id}/presence` que lista
  os membros com timer rodando AGORA (mesma regra de visibilidade). Sem o snapshot, B só veria A ao
  vivo na **próxima** mudança de A.

### Payload `presence` (envelope `{"type":"presence","payload": PresencePayload}`)

```
PresencePayload = {
  user_id:      string,   // o membro cujo timer mudou (A)
  name:         string,   // nome de exibição de A
  workspace_id: string,   // o workspace ao qual ESTE broadcast se refere (B é membro dele)
  running:      boolean,  // true = timer de trabalho rodando; false = pausa/ocioso
  project?:     string,   // nome do projeto — SÓ se B pode ver (shared); ausente se privado/sem projeto
  since?:       string    // RFC3339 — started_at da entry rodando (pra "rodando há Xmin"); ausente se !running
}
```

> O payload é **por-(workspace, observador)**: o mesmo `OnChange` de A pode gerar dois payloads
> distintos pro mesmo evento se A participa de 2 workspaces compartilhados, e o `project` pode ser
> omitido pra um observador que não vê o projeto e incluído pra outro que vê. (Na prática, dentro de
> um workspace, todos os membros têm a mesma visibilidade de um projeto `shared`.)

## 3. User stories

- **US-01** — Como membro de um workspace, quero **ver na hora quando um colega começa a rodar um
  timer**, pra saber que ele está focado e no quê — sem precisar dar reload.
- **US-02** — Como membro, quero ver o presence **sumir quando o colega para ou pausa**, pra a
  faixa refletir só quem está realmente ativo agora.
- **US-03** — Como dono de um **projeto privado**, quero que os outros vejam que **estou ativo** mas
  **não** o nome do projeto privado, pra preservar a confidencialidade (mesma regra de visibilidade
  que já vale nos relatórios e no board).
- **US-04** — Como membro que **abre a página depois** do colega já estar rodando, quero ver o
  estado atual (snapshot) na hora, não só a próxima mudança dele.
- **US-05** — Como usuário num workspace **só meu** (sem colegas), quero que nada quebre e a faixa
  de presence simplesmente não apareça / mostre só eu (sem custo).

## 4. Critérios de aceite (EARS — cada um vira teste)

### Modelo / arquitetura (sem migração)

- **CA-01** (sem migração) — a fase 21 **NÃO** adiciona migração nem tabela: presence é efêmero,
  deriva do `OnChange` e do hub em memória. `backend/internal/db/migrations/` fica **inalterado**
  (última migração segue `0020_reminders`/`0016_timer_guard` conforme o estado do build; nenhum
  `0021_*`). INDEX.md registra "21 = sem migração".

### `presence.Service` — fan-out por workspace (backend)

- **CA-02** (fan-out pros membros — escopado ao workspace DA ENTRY quando `running:true`) — QUANDO o
  `OnChange(A, state, at)` dispara com `state != nil` (timer rodando), o `presence.Service` DEVE
  resolver **somente os co-membros do workspace DA ENTRY de A** (`state.Entry.WorkspaceID`) — a
  entry roda em **um único** workspace — e, pra cada outro membro DESSE workspace, empurrar um
  envelope `type:"presence"` via `hub.Broadcast(memberUserID, …)`. O membro **A** (o próprio)
  **não** recebe o broadcast de si (a barra de timer dele já reflete o estado pelo `type:"timer"`)
  — decisão D-03; o teste pina que A não recebe presence de A. (Não fazer fan-out pra TODOS os
  workspaces de A: isso vazaria `running:true` pra co-membros de workspaces onde A NÃO está
  trabalhando, e divergiria do snapshot, que filtra por `e.workspace_id` — ver CA-02.1/D-05.)
- **CA-03** (isolamento entre workspaces — não-membro nunca recebe) — um usuário C que **NÃO** é
  membro de **nenhum** workspace de A NUNCA recebe presence de A. (Constituição multi-tenant —
  mutação removendo o `WHERE` de membership / o filtro por `workspace_members` deve **matar** um
  teste: o fan-out passaria a alcançar todo mundo.) Dois workspaces disjuntos: A em W1, C em W2 sem
  interseção → C recebe **zero** presence.
- **CA-04** (`running` derivado correto) — o payload DEVE ter `running:true` quando A tem entry de
  trabalho rodando (`state != nil` e `state.Mode ∈ {countup,countdown,pomodoro}`), e **`running:false`
  quando A entra em pausa Pomodoro (`state.Mode=="break"`) ou fica ocioso (`state==nil`)**. O `since`
  só é presente quando `running:true` (= `state.Entry.StartedAt`); ausente caso contrário.
  - **`running:true`** → fan-out **só pros co-membros de `state.Entry.WorkspaceID`** (o workspace da
    entry — CA-02).
  - **`running:false`** → como não há entry (state em pausa-Pomodoro não tem entry rodando; em
    stop/corte/pausa-órfã `state==nil` e não há workspace no state), o fan-out DEVE derrubar A em
    **TODAS as memberships de A** (todos os co-membros de todos os workspaces de A). É idempotente:
    remover A da faixa de um workspace onde ele nunca esteve ativo é no-op (CA-16). Isso garante que
    um `running:true` que tenha ido pro workspace W seja sempre cancelado por um `running:false`
    posterior, sem precisar lembrar em qual W A estava.
- **CA-02.1** (não vazar entre workspaces de A — `running:true` só no workspace da entry) — DADO A
  membro de **W1 ∩ W2** (participa dos dois) e A rodando um timer num projeto de **W1**, QUANDO o
  presence é emitido, os co-membros de **W2** (que não compartilham W1 com A) **NÃO** recebem
  `running:true` — recebem **nada** (ou `running:false` num evento de derrubada). Só os co-membros de
  **W1** veem A `running:true`. (Constituição: o delta ao vivo e o snapshot — D-05, que filtra por
  `e.workspace_id` — DEVEM concordar byte a byte pro mesmo estado de A; um fan-out por todos os ws de
  A faria o delta dizer "A ativo em W2" enquanto o snapshot de W2 diz "ninguém". Mutação que troca o
  filtro `m1.workspace_id = $entryWorkspaceID` por todas as memberships de A deve **matar** o teste do
  T-03.)
- **CA-05** (visibilidade — projeto compartilhado mostra nome) — QUANDO A roda um timer num projeto
  **`shared`** do workspace W, o presence enviado aos membros de W DEVE incluir **`project: <nome>`**
  (resolvido por join `projects` sob a regra `visibleWhere`). O nome é o `projects.name` real, não o
  id.
- **CA-06** (privacidade — projeto privado NÃO vaza nome) — QUANDO A roda um timer num projeto
  **`private`** (do qual só A é dono), o presence aos OUTROS membros DEVE reportar **`running:true`
  SEM o campo `project`** (omitido). (Sem isto, presence vazaria o nome de um projeto que o board e
  os relatórios já escondem — viola US-03 e a regra `visibleWhere` da fase 02. Mutação que troca a
  visibilidade-check por incondicional deve **matar** o teste.)
- **CA-07** (timer sem projeto) — QUANDO A roda count-up **sem projeto** (`state.Entry.ProjectID ==
  nil`), o presence DEVE ser `running:true` **sem `project`** (não há o que mostrar; não é erro).
- **CA-08** (pausa derruba presence) — QUANDO A **pausa** (entra em `mode='break'`), o presence aos
  membros DEVE ser `running:false` (o colega não está rodando trabalho agora). Retomar volta a
  `running:true` com a nova entry/`since`.
- **CA-09** (stop/corte derrubam presence) — QUANDO A **para** o timer (ou o corte automático da
  fase 16 fecha a entry — `OnChange(A, nil, at)`), o presence aos membros DEVE ser `running:false`.
  (Reusa o MESMO hook: o corte já chama `onTimer(A, nil, now)` no `guard.Cutter` — fase 16; presence
  cai junto, de graça.)
- **CA-10** (resolução fora do hot path do timer não bloqueia a mutação) — a resolução de membros +
  visibilidade do presence roda **após** o commit da mutação (no `OnChange`, que já é pós-commit) e
  **não pode** fazer o `Start/Stop/Pause/Resume` do timer falhar nem esperar: um erro de query no
  presence é **logado e engolido** (best-effort), nunca propaga pro caminho do timer. (Espelha o
  push best-effort da fase 16.)

### Snapshot inicial (REST)

- **CA-11** (snapshot autenticado) — `GET /api/v1/workspaces/{id}/presence` autenticado DEVE
  devolver a lista dos membros do workspace **com timer rodando agora**, cada um como um
  `PresencePayload` (mesma regra de visibilidade do projeto — `shared` mostra nome, `private` omite).
  Membros ociosos/em pausa **não** entram na lista (ou entram com `running:false` — decidir na spec:
  **a lista só traz `running:true`**, D-05). A rota é **aninhada no workspace** (atravessa
  sessão→membership), 404 se o caller não é membro.
- **CA-12** (snapshot — 404 não-membro) — `GET /workspaces/{id}/presence` por um usuário que **não é
  membro** de `{id}` (ou `{id}` inexistente/malformado) responde **404** uniforme (não vaza
  existência) — reusa o `RequireMember` da fase 01/02.
- **CA-13** (snapshot — 401 sem sessão) — `GET /workspaces/{id}/presence` **sem sessão** responde
  **401** canônico (`unauthorized`), antes de tocar o banco.
- **CA-14** (snapshot isolado por tenant) — o snapshot do workspace W DEVE listar **somente** os
  membros de W com timer rodando — nunca um membro de outro workspace, nunca o timer de um membro
  resolvido pelo projeto de OUTRO workspace. (Mutação removendo `WHERE workspace_id` na query do
  snapshot deve matar um teste.)

### Front — `realtime.ts` + store + UI

- **CA-15** (cliente trata `presence`, ramos existentes intocados) — `realtime.ts` DEVE ganhar
  `onPresence?: (p: Presence) => void` e tratar `type:"presence"` **depois** dos ramos
  `type:"timer"` e `type:"notice"`. O ramo `type:"timer"` (e o anti-stale do `timerStore`) e o ramo
  `type:"notice"` seguem **idênticos** (CA-18 da fase 04 / CA-11 da fase 16). `type` desconhecido /
  JSON inválido segue **ignorado sem derrubar a conexão**.
- **CA-16** (store consolida por workspace) — o `presenceStore` DEVE manter o conjunto de membros
  ativos **por workspace**: um payload `running:true` adiciona/atualiza o membro; `running:false`
  **remove** o membro da lista. A UI lê só o workspace ativo.
- **CA-17** (faixa de presence na UI) — a página `/workspace` (e o dashboard) DEVE exibir uma
  **`PresenceStrip`** com os membros ativos ao vivo do workspace atual (nome + projeto se presente +
  "rodando há Xmin" a partir do `since`). Sem ninguém ativo → a faixa não aparece (ou mostra um
  vazio discreto). A faixa **não** dispara nada no servidor (leitura pura).
- **CA-18** (snapshot no mount/reconexão) — ao montar a `PresenceStrip` (e ao reconectar o socket),
  o front DEVE buscar o snapshot `GET /workspaces/{id}/presence` e semear o store — pra B ver A já
  rodando ao **abrir** a página (US-04), não só na próxima mudança de A. Trocar de workspace ativo
  re-busca o snapshot do novo.

### Não-regressão (fases 03/04/10/16)

- **CA-19** — o `Hub` (`hub.go`) ganha **só** o método novo `BroadcastPresence(userID, payload)`
  (envelope `type:"presence"`) — `Broadcast`/`BroadcastTimer`/`BroadcastNotice`/`Sink`/`Register`/
  `Unregister` **byte-compatíveis** (não muda assinatura nem comportamento). As suítes de
  `realtime/hub_test.go`/`notice_test.go` seguem **verdes sem editar asserções**.
- **CA-20** — o wiring do `OnChange` continua: `tsvc.SetOnChange(hub.BroadcastTimer)` segue ligado;
  o presence é **encadeado** ao lado (o boot compõe `BroadcastTimer` + `presence.Service.OnTimer`
  num hook composto, OU registra o presence como segundo observador — ver plan §5). O ramo
  `type:"timer"` e o `timerStore.apply()`/anti-stale do front **não mudam** — as suítes da fase 03/04
  (`timer_*`, `realtime.spec.ts`, `cronometro.spec.ts`) seguem verdes.
- **CA-21** — a página `/workspace` (fase 10) preserva tudo: nome do workspace, lista de membros com
  role, seletor de workspace ativo — a `PresenceStrip` é **aditiva** (novo bloco). `Workspace.spec.tsx`
  segue verde (a faixa não some os asserts existentes).

### E2E

- **CA-22** (E2E — limitação de 1 user registrada) — **O stub de OAuth do E2E serve UM único
  usuário** (`diogo.fixture@example.com` — `cmd/googlestub` + `testdata/google/userinfo.json`; ver
  `frontend/e2e/helpers.ts`). Logo **não dá** pra exercitar A→B com dois usuários reais via
  Playwright. O caminho A→B (presence cruzando usuários no mesmo workspace) é coberto por **teste de
  integração Go** no `presence.Service` + hub (2 userIDs distintos semeados por SQL, `Sink` fake da
  fase 04). O E2E faz o que dá: logado como o user único, **abre `/workspace`, confirma que a
  `PresenceStrip` monta e busca o snapshot sem erro** (200/lista própria), e que o socket trata
  `type:"presence"` sem quebrar (injetando um payload via um helper, se viável). **A limitação fica
  registrada no `tasks.md` e neste CA** — não é furo, é o teto do stub.

## 5. Requisitos não-funcionais

- **RNF-01 (leveza do fan-out)** — a resolução do presence por `OnChange` é **uma query indexada**
  (`workspace_members` por `user_id`, PK/índice existente) + um join leve de `projects` pra o nome/
  visibilidade. Roda **pós-commit**, fora do lock do timer, e **best-effort** (CA-10): erro logado e
  engolido. Numa VPS com poucos usuários, custo desprezível. **Zero** dependência nova (Go ou JS) —
  reusa Hub, pgxpool, zustand.
- **RNF-02 (sem broadcast por workspace no hub)** — o Hub **não** indexa por workspace; o fan-out é
  resolvido no `presence.Service` e entregue via `hub.Broadcast(memberUserID, …)` per-usuário. Isto
  **preserva** o core do hub (fase 04) intacto (RNF-04) e evita reescrever `Register`/`Unregister`/
  `Broadcast`.
- **RNF-03 (privacidade)** — o nome do projeto NUNCA sai pra quem não pode ver o projeto
  (`visibleWhere` da fase 02). Presence expõe **só** `running` + (se visível) nome — não vaza
  descrição da entry, tags, nem duração detalhada.
- **RNF-04 (não-regressão fases 03/04/16)** — `timer.Service`, `realtime/hub.go` (resto), o cliente
  WS (`realtime.ts` ramos timer/notice), o `timerStore` e seu anti-stale, e o corte/notice da fase
  16 **intocados** no contrato (CA-19/20). Presence é **aditivo** (método novo no hub, envelope novo,
  callback novo no cliente, store novo, faixa nova).
- **RNF-05 (degradação)** — sem socket (offline/queda), a `PresenceStrip` apenas não atualiza ao
  vivo; o snapshot do mount ainda dá o estado no momento da abertura. Reconexão re-busca o snapshot.
  Nada quebra a página.
- **RNF-06 (clock/tz)** — `since` é o `started_at` da entry em **RFC3339 UTC**; a UI deriva
  "rodando há Xmin" do relógio local do navegador (mesmo padrão da TodayList). Sem `since`, sem
  rótulo de tempo.

## 6. Pré-condições externas

- Nenhuma. Presence não precisa de credencial, chave nem provider novo. Reusa o WebSocket/Hub
  (fase 04) que já está em produção e a regra de visibilidade da fase 02. Multi-usuário REAL exige
  membros no workspace — hoje semeados por SQL (fase 17 entrega a API de convite); no E2E,
  `seedWorkspaceMember`. **Presence não depende da fase 17 pra construir** (a colaboração já seeda
  membros por SQL, como hoje — `MILESTONE-2.md §9`).

## 7. Fora de escopo (fronteiras explícitas)

- **Colaboração ao vivo TOTAL** (task atribuída / nota nova / status aparecendo na hora via
  WebSocket) — fora; presence é o ÚNICO fan-out por workspace da M2. Cross-assign/notas/status
  propagam **no reload** (`MILESTONE-2.md §3` "Fora da M2"). Future feature.
- **Histórico de presence / quem rodou o quê quando** — presence é **instantâneo** (estado agora),
  não um log. Relatórios já cobrem o histórico.
- **Indexar o Hub por workspace** (reescrever `users` pra `map[workspaceID]…`) — explicitamente
  **evitado** (RNF-02): o `presence.Service` resolve o fan-out fora do hub.
- **Presence de presença de página / cursor / "está digitando"** (à la Figma/Google Docs) — fora;
  só o **timer ativo**.
- **Notificar (push/notice) quando um colega começa/para** — fora; presence é **só na faixa ao
  vivo** de quem está com a página aberta. O canal `notice`/push da fase 16 fica pros eventos
  direcionados (corte, cross-assign, lembrete), não pra presence.
- **Configurar visibilidade de presence** ("ficar invisível") — fora; quem é membro vê quem está
  ativo (com a regra de projeto). Toggle de invisibilidade é future.
- **Privacidade da DESCRIÇÃO/tags do timer** — presence nunca expõe descrição nem tags (só nome do
  projeto se `shared`); não há CA pra expor mais — é proibido por RNF-03.
