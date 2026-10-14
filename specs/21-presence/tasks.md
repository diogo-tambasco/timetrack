# tasks.md — Fase 21 · Presence ao vivo (broadcast por workspace)

> TDD: red → green → refactor. Commit por tarefa. `make check-full` é o gate final (T-13).
> Cada T-XX lista os CAs cobertos. Backend primeiro (hub aditivo → presence.Service → snapshot →
> wiring), depois o frontend (realtime onPresence → store → strip), E2E (limitado — CA-22),
> verificação adversarial e gate. **Sem migração.** **NÃO paralelizável** (toca o core de realtime
> e o wiring do OnChange em cmd/server — isolar e buildar sozinho; plan §7).

---

## Bloco backend (fan-out + snapshot)

- **T-01 — Confirmar SEM migração + registrar** (CA-01)
  - Verificar que `backend/internal/db/migrations/` segue inalterado (nenhum `0021_*`). Presence é
    efêmero (plan §2).
  - INDEX.md: registrar "21 = sem migração" na seção da M2.
  - Acceptance: `git status` em `migrations/` limpo; INDEX atualizado. (Tarefa de doc/guarda — sem
    teste de código; o estado de "sem migração" é o backstop dos CAs de backend abaixo.)

- **T-02 — `Hub.BroadcastPresence` (aditivo)** (CA-19, RNF-02/04)
  - `internal/realtime/hub.go`: **só adicionar** `BroadcastPresence(userID, payload any)` (envelope
    `type:"presence"`) — espelha `BroadcastNotice`. `Broadcast`/`BroadcastTimer`/`BroadcastNotice`/
    `Sink`/`Register`/`Unregister` **intocados** (plan §3).
  - Teste (`presence_test.go` no pacote realtime, reusa o `Sink` fake da fase 04): `BroadcastPresence`
    entrega `{"type":"presence","payload":…}` **só** às conexões do user; o consumidor de `timer`/
    `notice` não é afetado; `hub_test.go`/`notice_test.go` da 04/16 seguem verdes **sem editar**.
  - Acceptance: `go test ./internal/realtime/...` verde; envelope serializado conferido byte a byte.

- **T-03 — `presence.Service.OnTimer` — fan-out por workspace** (CA-02/02.1/03/04/06/07/08/09/10, RNF-01/03)
  - Pacote novo `internal/presence`; `Service` com `db`, `broadcast` (=`hub.BroadcastPresence`),
    `now` injetável. `OnTimer(userID, state, at)`: deriva `running`/`since`/`projectID`/
    `entryWorkspaceID` do state (plan §3); resolve co-membros via self-join (`m2.user_id <> A` —
    D-03), **escopada por `m1.workspace_id = entryWorkspaceID` quando `running:true`** e por TODAS as
    memberships de A quando `running:false` (plan §3 passo 2 / CA-02.1/CA-04); resolve nome do
    projeto sob `visibleWhere`; empurra `PresencePayload` pra cada (B, W); best-effort (erro
    logado/engolido — CA-10).
  - `service_test.go` (dbtest, RED primeiro): **casos obrigatórios** —
    - A roda timer em projeto `shared` no ws W com B membro → B recebe presence `running:true`,
      `project=<nome>`, `since=started_at` (CA-02/04/05 — CA-05 detalhado no T-04 com visibilidade);
    - **isolamento:** C (membro de outro ws, sem interseção com A) recebe **ZERO** presence;
      mutação removendo o filtro de membership (`m2.workspace_id = m1.workspace_id` / o `WHERE
      m1.user_id`) faz C receber e **mata** o teste (CA-03);
    - **escopo do workspace da entry (CA-02.1):** A membro de **W1 ∩ W2**, rodando um timer num
      projeto de **W1** → o(s) co-membro(s) **de W1** recebem `running:true`, mas o(s) co-membro(s)
      **só de W2** (que não compartilham W1 com A) recebem **NADA** (`running:true`); **mutação** que
      troca `m1.workspace_id = $entryWorkspaceID` por todas as memberships de A faz o membro de W2
      receber `running:true` e **mata** este teste. Conferir que o delta concorda com o snapshot
      (D-05/T-05): snapshot de W1 lista A; snapshot de W2 não lista A;
    - **A não recebe presence de si** (D-03): spy no broadcast não tem entrada pra `A` (CA-02);
    - `running:false` quando state é `mode='break'` (pausa — CA-08) e quando state é `nil`
      (stop/corte — CA-09); `since`/`project` ausentes quando `!running`; e o `running:false`
      derruba A em **todas** as memberships de A (idempotente — co-membros de W1 E de W2 recebem
      `running:false`, CA-04/CA-16);
    - timer **sem projeto** (`Entry.ProjectID==nil`) → `running:true` sem `project` (CA-07);
    - **best-effort (CA-10):** forçar erro de query (ex.: pool fechado / ws inexistente) → `OnTimer`
      **não entra em panic** e **não propaga** (retorna silencioso); spy confirma nenhum broadcast.
  - Acceptance: os casos acima verdes; as 4 mutações (membership, escopo-workspace-da-entry,
    self-exclusion, running-derivado) morrem.

- **T-04 — Visibilidade do projeto no presence (priv/shared)** (CA-05/06, RNF-03)
  - No `OnTimer` (e reusado no snapshot): nome do projeto só entra se o **viewer B** pode ver
    (`visibility='shared' OR owner_user_id=B`). `private` de A → omite `project`; `shared` → inclui.
  - `service_test.go`: A roda em projeto **`private`** (só A dono) com B membro → B recebe
    `running:true` **SEM `project`** (CA-06); A roda em **`shared`** → B recebe **com** `project`
    (CA-05). **Mutação** que ignora a visibilidade (sempre inclui o nome) **mata** o teste do
    privado.
  - Acceptance: ambos verdes; mutação do vazamento de privado morre. (Ancorar no `visibleWhere` real
    de `projects/service.go:215`.)

- **T-05 — `GET /workspaces/{id}/presence` (snapshot REST)** (CA-11/12/13/14, RNF-03)
  - `internal/presence/service.go::Snapshot(ws, viewer)` (query do plan §3, D-05: só `running:true`,
    isolada por `e.workspace_id=$ws` + join de membership; visibilidade aplicada no service).
    `internal/api/presence_routes.go` (handler fino, aninhado no subrouter do workspace atrás do
    `RequireMember`): `200 {"members":[…]}` (cada item = PresencePayload); `WithPresence(svc)`
    Option; montar em `auth_routes.go::mountAuth` (onde core/reports penduram).
  - `presence_routes_test.go` (harness api da fase 02 — `coretest`/`newCoreRouter` ou equivalente):
    - **200** lista os membros do ws com timer rodando (semear 2 membros, 1 rodando shared, 1
      rodando private de outro membro → 1º com `project`, 2º sem); membro ocioso/em pausa **não**
      aparece (D-05);
    - **404** pra não-membro do ws / ws inexistente / id malformado (CA-12 — reusa `RequireMember`);
    - **401** sem sessão (CA-13);
    - **isolamento (CA-14):** entry rodando de um membro em OUTRO workspace dele **não** aparece no
      snapshot de W; mutação removendo `WHERE e.workspace_id` mata o teste.
  - Acceptance: tabela 200/404/401/isolamento verde; mutação do tenant morre.

- **T-06 — Wiring composto no boot (OnChange + corte da 16)** (CA-02/09/20, RNF-04)
  - `cmd/server/main.go`: construir `presence.NewService(pool, hub.BroadcastPresence)`; extrair o
    **hook composto** `onTimer := func(u, st, at){ hub.BroadcastTimer(u,st,at); presenceSvc.OnTimer
    (u,st,at) }` e usá-lo **tanto** em `tsvc.SetOnChange(onTimer)` **quanto** em
    `guard.WithOnTimer(onTimer)` (pro corte da 16 também derrubar a presence — D-04). Montar
    `api.WithPresence(presenceSvc)` no router.
  - Acceptance: o boot liga sem quebrar (exceção ao red estrutural documentada — espelha o
    `tsvc.SetOnChange` da fase 04/16); um teste leve confirma que uma mutação de timer dispara
    `BroadcastTimer` **e** `presence.OnTimer` (spy nos dois), e que o `guard.WithOnTimer` aponta pro
    hook composto. Suítes das fases 03/04/16 seguem verdes (CA-20).

## Bloco frontend + E2E + gate

- **T-07 — `realtime.ts` `onPresence`** (CA-15/20)
  - `TimerSocketOptions` ganha `onPresence?: (p: Presence) => void`; `onmessage` trata
    `type:"presence"` **depois** dos ramos `type:"timer"` e `type:"notice"` (ambos INTOCADOS);
    `type` desconhecido/JSON inválido segue ignorado sem derrubar. Tipo `Presence` exportado
    (`{user_id,name,workspace_id,running,project?,since?}`).
  - `realtime.spec.ts` estendido: `type:"presence"` chama `onPresence` com o payload; `type:"timer"`
    segue chamando `onResult` (anti-stale intocado); `type:"notice"` segue chamando `onNotice`;
    desconhecido/inválido não chama nem derruba (regressão das fases 04/16 preservada).
  - Acceptance: spec verde; os ramos timer/notice byte-idênticos no diff (só adição).

- **T-08 — `presenceStore` + `api.ts` snapshot** (CA-16/18)
  - `lib/presenceStore.ts` (zustand): estado = membros ativos **por workspace**; `apply(p)` →
    `running:true` add/atualiza, `running:false` remove (CA-16); `seed(wsId, members)` pro snapshot;
    seletor `forWorkspace(wsId)`. `lib/api.ts`: `getWorkspacePresence(wsId)` → `{members}`.
  - `presenceStore.spec.ts` (vitest): apply add/remove por running; seed sobrescreve; troca de ws
    não mistura; remoção de membro inexistente é no-op.
  - Acceptance: spec verde.

- **T-09 — `PresenceStrip` + montagem na `/workspace` e dashboard** (CA-17/21)
  - `components/PresenceStrip.tsx`: lê o `presenceStore` do **workspace ativo**; lista membros
    ativos (avatar/nome + projeto se presente + "rodando há Xmin" do `since`, derivado do relógio
    local — RNF-06); sem ninguém ativo → faixa some / vazio discreto. Não dispara nada no servidor.
    Montar na `Workspace.tsx` (aditivo, abaixo do header) e na `Dashboard.tsx` (aditivo).
  - Specs (jsdom): faixa lista membros ativos do ws; membro sem `project` não mostra projeto;
    `running:false` some da faixa; faixa ausente sem ninguém ativo; **`Workspace.spec.tsx` segue
    verde** (nome/membros/seletor preservados — CA-21).
  - Acceptance: specs verdes; Workspace.spec da fase 10 intocado nos asserts.

- **T-10 — Wiring no `attachTimerSync` + snapshot no mount/reconexão** (CA-16/18)
  - `lib/timerStore.ts::attachTimerSync`: passa `onPresence: (p) => presenceStore.apply(p)` ao
    `connectTimerSocket` (sem tocar `onResult`/anti-stale). A `PresenceStrip` (ou um hook) busca o
    snapshot `getWorkspacePresence(wsId)` no **mount** e **re-busca** ao reconectar (onOpen) e ao
    trocar de workspace ativo — semeia o store (CA-18). (Padrão: `useQuery(['presence',wsId])` +
    invalidate no onOpen; ou um `seedPresence` chamado no onOpen do socket.)
  - Acceptance: teste do wiring (mock do socket) confirma `onPresence` ligado + snapshot semeando o
    store no mount; `attachTimerSync` segue devolvendo o teardown único (CA-19 da 04 preservado).

- **T-11 — E2E `presence.spec.ts` (limitado — 1 user)** (CA-22)
  - **Limitação registrada (plan §7, CA-22):** o stub de OAuth serve **1 usuário**
    (`diogo.fixture@example.com` — `cmd/googlestub`/`testdata/google/userinfo.json`). **Não** dá pra
    logar 2 users reais → o cross-user A→B é coberto pelo **teste de integração Go** (T-03/T-04/T-05).
    O E2E faz o que dá: logado como o user único, **abre `/workspace`**, confirma que a
    `PresenceStrip` monta e o snapshot `GET /workspaces/{id}/presence` responde **200** sem erro
    (lista própria, possivelmente vazia); inicia um timer em projeto compartilhado e confirma que a
    página **não quebra** (a própria presence de A não aparece pra A — D-03, então a faixa pode ficar
    vazia: o assert é "monta sem erro + snapshot 200", não "vê a si mesmo"). Reusar `loginViaStub` e,
    se viável, semear um 2º membro por SQL (`seedWorkspaceMember`) só pra a lista de membros — mas
    **sem** sessão dele (não dá pra ele rodar timer via UI). Liberar a :8080 antes do E2E.
  - Acceptance: o spec passa (monta + snapshot 200 + sem crash); o comentário do arquivo registra a
    limitação e aponta os testes Go como a cobertura real do A→B.

- **T-12 — Verificação adversarial do build + correções**
  - Workflow ultracode (dimensões: **fan-out/isolamento entre workspaces** [não-membro recebe?
    `running:true` vaza pra co-membro de OUTRO ws de A? delta concorda com snapshot — CA-02.1] ·
    **visibilidade priv/shared** [vaza nome de privado?] · **running derivado** [break/stop/corte →
    false? `running:false` derruba A em todas as memberships?] · **não-regressão timer/notice**
    [ramo type:"timer"/anti-stale/notice intactos?] · **wiring composto** [corte da 16 derruba
    presence? OnChange não regride o sync?] · **snapshot tenant** [vaza entry de outro ws? concorda
    com o delta?] · best-effort [erro de presence derruba timer?]) + refutação dupla. Corrigir
    confirmados red→green. Registrar no fim deste arquivo.

- **T-13 — Gate + handoff**
  - `make check-full` VERDE (Go + Vitest + Playwright + builds + smoke compose). **Antes do E2E,
    liberar a :8080** de qualquer dev server alheio (`reuseExistingServer:true` no playwright.config
    — pega o server errado).
  - Handoff (CLAUDE.md "retomar…"), INDEX (21 construída, sem migração), commit, briefing pro Diogo.
  - **Deploy:** segue a política da M2 (deploy único no fim da milestone, OU conforme o KICKOFF da
    sessão) — **NÃO** tocar no Caddy/PrimaOS/portas. Presence não tem migração nem env nova (RNF-01),
    o deploy é só o app recriado.

---

## Verificação adversarial da spec (ETAPA 3) — sessão 19 (21/06)

> Furos confirmados **2/2** numa verificação adversarial ancorada no código real, aplicados ANTES do
> build (Regra Nº1). **Total: 1 furo confirmado 2/2 aplicado.** Refutados/descartados NÃO entram
> aqui (só os confirmados viram CA/caso de teste).

- **[HIGH] Fan-out de `running:true` vazava presence de A pros co-membros de TODOS os workspaces de
  A** — a entry roda em UM workspace só (`state.Entry.WorkspaceID`), mas a self-join original
  filtrava só por `m1.user_id = $A` → co-membros de workspaces onde A NÃO estava trabalhando
  recebiam `running:true`, e o delta ao vivo **divergia** do snapshot (D-05, que filtra por
  `e.workspace_id`). **Correção:** quando `running:true`, o fan-out passa a escopar pelo workspace
  da entry (`m1.workspace_id = $entryWorkspaceID`); quando `running:false` (state sem workspace —
  stop/corte/pausa-órfã), derruba A em TODAS as memberships de A (idempotente — CA-16). Delta e
  snapshot DEVEM concordar byte a byte pro mesmo estado.
  - **spec:** CA-02 reescrito (escopo ao workspace da entry); CA-04 estendido (fan-out de
    `running:true` só no ws da entry; `running:false` derruba em todas as memberships); **novo
    CA-02.1** (A em W1∩W2, timer em W1 → co-membros só de W2 não recebem `running:true`; delta×snapshot
    concordam).
  - **plan:** §3 passo 1 deriva `entryWorkspaceID`; passo 2 bifurca a self-join (filtro
    `m1.workspace_id = $entryWorkspaceID` pra `running:true` × todas as memberships pra
    `running:false`); nova armadilha vetada (§8).
  - **tasks:** T-03 ganha o caso de A em 2 workspaces (W1 recebe `running:true`, W2 não) + a
    **mutação** que remove o filtro de workspace-da-entry (mata o teste) + o caso do `running:false`
    derrubando em todas as memberships; CA list de T-03 atualizada (CA-02.1); dimensão de
    verificação do T-12 estendida (CA-02.1 + delta×snapshot).

## Registro — BUILD + verificação adversarial do BUILD (sessão 20, 22/06)

> Build TDD T-01→T-11 (commit por tarefa, até `d2dd4e0`) · verificação adversarial do build ·
> correções T-12 · gate verde · handoff T-13.

**Suítes finais:** Go all green · Vitest 377 · Playwright (presence.spec, limitado a 1 user — CA-22).
`make check-full` **GATE VERDE**.

**Arquitetura entregue (Opção B do plan §1):** hub per-usuário da fase 04 **INTOCADO**;
`presence.Service` (novo pacote) resolve co-membros+visibilidade pós-commit e chama
`hub.BroadcastPresence(memberID, payload)` por membro. `BroadcastPresence` aditivo ao hub.
`OnChange` **composto** (`onTimer := BroadcastTimer + presence.OnTimer`) plugado em AMBOS
`tsvc.SetOnChange` E `guard.WithOnTimer` (pro corte da 16 também derrubar presence). `running:true`
escopado ao `Entry.WorkspaceID`; `running:false` derruba A em todas as memberships (idempotente).
`GET /workspaces/{id}/presence` (snapshot por viewer). Front: `presenceStore` + `PresenceStrip` na
/workspace+dashboard + `realtime.ts onPresence` + `api.getWorkspacePresence` (snapshot no mount/
reconexão). **Sem migração** (T-01 confirmou `migrations/` inalterado).

**Verificação adversarial do build (dimensões × refutação dupla) — 3 confirmados, corrigidos
red→green (T-12):**

- **[A · HIGH · BUG REAL] Vazamento de nome de projeto PRIVATE pro não-dono.** `timer.Start` aceita
  QUALQUER projeto do ws sem checar dono/visibilidade → A pode rodar timer num projeto private de um
  CO-MEMBRO B. O cache `projByWS[workspaceID]` resolvia o nome **já-filtrado pelo 1º observador** e o
  reusava por workspace → um não-dono C recebia o nome que só B (dono) deveria ver; pior, o **delta
  ao vivo divergia do Snapshot** (que decide por linha). **Fix:** resolver `projInfo` RAW
  (name/visibility/owner) UMA vez no `entryWS` e decidir a visibilidade **POR OBSERVADOR** no passo 4
  do fan-out via `visibilityVisible` — exatamente o per-row do Snapshot (delta=snapshot). Teste:
  `TestOnTimerProjetoPrivadoDeOutroNaoVazaProNaoDono` (dono vê "SegredoDoB", não-dono recebe presence
  com `Project=nil`). Mutação (ignorar a visibilidade no passo 4) mata o teste.
- **[B · MEDIUM · BUG REAL] Snapshot incluía o PRÓPRIO viewer.** O delta exclui A via `m2.user_id<>A`,
  mas o `Snapshot` listava A na própria faixa → A ficaria **preso** na faixa após parar o timer (o
  `running:false` do delta nunca chega a A). **Fix:** `AND e.user_id <> $2` (viewerID) na query do
  Snapshot. Teste: `TestSnapshotExcluiOProprioViewer`. Mutação (tautologia no lugar do filtro) mata
  o teste.
- **[C · LOW · cobertura] recover do `OnTimer` sem teste.** `TestOnTimerRecuperaDePanico`: um panic no
  broadcast NÃO propaga pro caminho do timer (best-effort, CA-10). Mutação removendo o `recover` faz
  o teste entrar em pânico e falhar.

**Lição registrada (cross-fase):** os agentes de mutação da verificação rodam na MESMA working tree
e o `git checkout` de restauração CLOBBERA mudança não-commitada → commitar/limpar a tree ANTES de
rodar workflow de verificação por mutação (já aplicado: build estava commitado em `d2dd4e0` antes da
verificação; as correções da verificação foram validadas por mutação manual no main loop com a tree
das correções, restaurando byte-a-byte).
