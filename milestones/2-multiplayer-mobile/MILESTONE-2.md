# MILESTONE 2 — "Multiplayer + Mobile" (PRD)

> Status: **specs escritas a partir deste PRD** (sessão 19, 21/06/2026). Segue a Regra Nº1:
> este PRD vira specs por fase (`specs/NN-*/`) antes de qualquer código.
> Herda `CLAUDE.md`, `REQUISITOS.md` (§5 "Fora do MVP"), `MILESTONE-1.md` (bloco "Fora da M1"),
> `DESIGN.md`. PT-BR, HITL. Identificadores de código em inglês.

---

## 1. Por que esta milestone existe

A M1 ("Core Sólido") tornou o app **confiável e completo pra o Diogo sozinho** (objetivos,
dashboard, relatórios 2.0, foco 2.0, anti-timer-esquecido). Agora o app está pronto pra **trazer
gente** — o que sempre foi a aposta da arquitetura ("multi-tenant desde o dia 1", `REQUISITOS §1`):

1. **O banco já prevê multiplayer, mas não há porta de entrada** — `workspaces`, `workspace_members`
   (roles owner/member/viewer), `tasks.assignee`/`tasks.requested_by` existem, mas **não há API de
   convite** e os campos de atribuição estão **dormentes** (lidos, nunca escritos). Hoje só dá pra
   adicionar membro por SQL.
2. **Não dá pra colaborar** — sem cross-assign de tasks, sem conversa por task, sem lembretes.
3. **O celular é segunda classe** — o app é desktop-first denso, **zero responsivo** (nenhum
   breakpoint `sm:`/`md:`, nenhum `@media`). O "controle remoto" do REQUISITOS §4.7 não existe.

**M2 = abrir o app pra colaboradores (Cadu, namorada) com colaboração completa, e fazer o celular
virar primeira classe.** Razão da ordem (M1 antes): não dá pra onboardar gente num app cujas
métricas o próprio dono não confiava.

---

## 2. Problemas → soluções (rastreabilidade)

| # | Dor | Solução M2 | Fase |
|---|---|---|---|
| P1 | "Só dá pra adicionar membro por SQL — não tenho como convidar o Cadu" | API de **convite** (por email pendente **+** buscar user existente), aceitar/remover/mudar role — **só Owner gere** | 17 |
| P2 | "Não consigo pedir/atribuir uma task pra outra pessoa" | **Cross-assign** (liga `assignee`/`requested_by`) + notifica o assignee + visões **"Minhas tasks"** / **"Pedidas por mim"** | 18 |
| P3 | "Não dá pra conversar sobre uma task" | **Thread de comentários** por task (tabela nova, autor + timestamp) | 19 |
| P4 | "Esqueço prazos e não sou lembrado" | **Lembretes**: `due_date` chegando **+** lembrete manual; in-app + Web Push | 20 |
| P5 | "Não vejo o que o outro está fazendo agora" | **Presence ao vivo**: broadcast por workspace do timer ativo dos membros (a decisão arquitetural central) | 21 |
| P6 | "No celular o app é inutilizável" | **Mobile responsivo**: breakpoints nos componentes existentes + nav mobile | 22 |
| P7 | "No celular eu só quero start/stop e trocar projeto" | **Controle remoto** dedicado (superfície mobile-first enxuta) | 23 |

---

## 3. Escopo

### Dentro da M2
- **Workspaces multi-usuário funcionais**: convidar (email-pendente + buscar existente), aceitar
  no 1º login, remover membro, mudar role — gestão **Owner-only**.
- **Colaboração completa**: cross-assign (assignee/requested_by) + notificação + visões dedicadas;
  thread de comentários por task; lembretes (due_date + manual) in-app + push.
- **Presence ao vivo**: ver o timer ativo dos membros do workspace em tempo real (broadcast por
  workspace — extensão aditiva do hub per-usuário da fase 04).
- **Mobile**: base responsiva (todos os componentes existentes utilizáveis no toque) + nav mobile
  + superfície dedicada "controle remoto" (start/stop, trocar projeto, ver o que roda).

### Fora da M2 (fronteiras explícitas)
- **Colaboração ao vivo COMPLETA** (task atribuída/nota nova aparecendo na hora via WebSocket): a
  M2 entrega **só presence ao vivo**; cross-assign/notas/status propagam **no reload** (decisão HITL
  — fan-out de domínio completo fica pra evolução futura). Notificações de cross-assign/lembrete
  chegam ao alvo via o canal `notice` **per-usuário** que já existe (não exige broadcast por
  workspace).
- **Recorrência de lembrete** (o lembrete em si não recorre; a task já pode recorrer via
  `recurrence`).
- **Email / provider externo de notificação** — segue in-app + Web Push (herdado da fase 16).
- **Telas mobile dedicadas além do controle remoto** (ex.: board mobile-nativo) — o responsivo
  cobre; superfície dedicada é só o controle remoto.
- **Rename/criação de workspace, billing, sub-workspaces** — fora.

### Future features (registro — não nesta milestone)
- **IA "coach"** (resumo semanal que toma esporro) — `REQUISITOS §5`.
- **Colaboração ao vivo total** (broadcast por workspace de toda mutação de domínio) — evolução
  natural da fase 21.
- **Conector Claude (fase 08)** — segue PULADA/na fila, **não** entra na M2.
- Integração Git · write-back do Calendar · metas avançadas/billable.

---

## 4. Fases da M2

> Numeração continua o roadmap (00–16 existem; **08 segue PULADA**). M2 = fases **17–23**.
> Migrações: **0017→17, 0018→18, 0019→19, 0020→20**; 21/22/23 **sem migração** (presence é
> efêmero; mobile é frontend). Ownership fixado pra evitar colisão no build paralelo com worktrees.

### Fase 17 — Membros & convites (workspace multiplayer) **[fundação]**
- **Migração `0017_invitations`**: tabela `invitations` (workspace_id, email normalizado, role
  convidado, token, status `pending`/`accepted`/`revoked`, invited_by, created_at, expires_at).
- **Convite por email (pendente):** Owner cria convite → fica `pending`; **resolve no 1º login**
  daquele email (reusa o re-vínculo da fase 01 / `UpsertUserFromGoogle`): ao logar, o sistema
  varre convites pendentes do email do user e materializa a membership com a role do convite.
- **Buscar user existente:** Owner pode adicionar diretamente quem já tem conta (busca por email no
  banco de `users`) — vira membership na hora (sem passo de aceite).
- **Gestão Owner-only:** convidar, **revogar convite**, **remover membro**, **mudar role**. Member
  e viewer não gerem o quadro (403). Não dá pra remover/rebaixar o último Owner (invariante).
- **UI:** estende a página `/workspace` (fase 10) — bloco de convites (form + lista de pendentes +
  revogar) e gestão de membros (remover, trocar role), **só pra Owner**.
- Reusa: re-vínculo por email (01), `GET /workspaces/{id}` com `members` (02), página `/workspace`
  (10), isolamento multi-tenant (filtro duplo).

### Fase 18 — Cross-assign & visões ("Minhas tasks" / "Pedidas por mim")
- **Liga `assignee`/`requested_by`** na API de tasks (hoje dormentes — lidos, nunca escritos):
  `POST`/`PATCH /tasks` passam a aceitar `assignee` e `requested_by` (uuids **validados contra a
  membership do workspace**; null limpa). Migração `0018` = índices em `assignee`/`requested_by`
  pra a query cross-workspace.
- **Permissão:** member atribui/pede; viewer **pode ser assignee** (carve-out — ver §8). Atribuir
  pra fora da membership = `400 validation_error`.
- **Notifica o assignee:** ao receber uma task (assignee setado por outra pessoa), o alvo recebe um
  aviso **in-app (notice per-usuário, fase 16) + Web Push**. Reusa o canal `notice` que **já**
  fan-out pro userID alvo — **sem** broadcast por workspace.
- **Visões dedicadas:** **"Minhas tasks"** (assignee = eu, em qualquer projeto/workspace que eu
  participe) e **"Pedidas por mim"** (requested_by = eu). Rotas + tela.
- **UI:** seletor de assignee/solicitante no modal/board de task (fase 12); as duas visões.

### Fase 19 — Comentários (thread) em tasks
- **Migração `0019_task_comments`**: tabela `task_comments` (id, task_id, workspace_id denorm,
  author_id, body, created_at). A coluna `tasks.notes` que já existe **permanece** como a
  descrição/nota da task; o thread é a camada nova de colaboração.
- **API:** `GET`/`POST`/`DELETE` comentários sob a task. Permissão: member comenta nas tasks
  visíveis; viewer comenta **só** nas tasks onde é assignee (carve-out §8); autor (ou Owner) apaga.
- **UI:** painel de thread no detalhe da task (autor + timestamp + texto), input de novo comentário.

### Fase 20 — Lembretes (due date + manual)
- **Migração `0020_reminders`**: tabela `reminders` (id, user_id, task_id, workspace_id denorm,
  remind_at, kind `due`/`manual`, status `pending`/`sent`/`canceled`, created_at).
- **Worker próprio** (`internal/reminders`, **não** toca o pacote `guard` da 16): um ticker leve
  varre `reminders` com `remind_at <= now` e `status='pending'` → dispara aviso **in-app (notice
  per-usuário) + Web Push** (reusa `push.Sender`/`Hub.BroadcastNotice` da 16) e marca `sent`.
- **Due date:** ao chegar perto da `due_date` de uma task (regra da spec — ex.: no dia, 09:00
  local), gera um reminder `due` pro responsável (assignee ?? created_by). **Não recorre** (a task
  pode, via `recurrence`).
- **Manual:** o user marca "me lembra dessa task às X" → cria reminder `manual`; pode cancelar.
- **UI:** ação "lembrar" na task + lista/cancelamento dos próprios lembretes.

### Fase 21 — Presence ao vivo (broadcast por workspace) **[arquitetural]**
- **A decisão central da M2:** o hub da fase 04 é **estritamente per-usuário**
  (`users map[string]map[Sink]struct{}`). Presence exige que, quando o timer de um membro muda, os
  **outros membros do workspace** que estão olhando vejam ao vivo.
- **Fan-out aditivo, sem regredir o timer:** estende o hub com um **canal de presence por
  workspace** (envelope novo `type:"presence"`), alimentado pelo `OnChange` do `timer.Service` (já
  existe o hook): a mudança do timer do user A é resolvida pros workspaces compartilhados de A e
  empurrada como `presence` pras conexões dos membros B/C. O `type:"timer"` per-usuário e o
  anti-stale do `timerStore` **não mudam** (CA de não-regressão).
- **Visibilidade:** só expõe o timer ativo onde a visibilidade do projeto permite (projeto
  compartilhado; privado não vaza — regra da fase 02/07). Mostra "fulano: rodando <projeto>".
- **UI:** faixa/seção de presence na página `/workspace` (e/ou dashboard) com os membros ativos
  ao vivo.
- **Sem migração** (estado efêmero).

### Fase 22 — Mobile responsivo (base)
- **Base responsiva** nos componentes que já existem: dashboard, relatórios 2.0, tarefas/board,
  projetos, clientes, ajustes, workspace, calendário — todos **utilizáveis no toque** via
  breakpoints Tailwind (`sm:`/`md:`/`lg:`). Hoje há **zero** breakpoint.
- **Nav mobile:** o rail de 64px vira navegação mobile (bottom bar ou drawer) abaixo de um
  breakpoint; a TimerBar global se adapta. Foco já é fullscreen.
- **Densidade/layout:** grades multi-coluna colapsam pra 1 coluna; tabelas densas viram cartões/
  scroll no celular.
- **Sem migração** (frontend). **Depende de**: todo o UI existente (cross-cutting).

### Fase 23 — Controle remoto dedicado (mobile)
- **A superfície enxuta do celular** (REQUISITOS §4.7 / DESIGN §3.3): **start/stop + trocar
  projeto + ver o que está rodando** (inclui presence dos outros, da fase 21) numa tela mobile-
  first, otimizada pra toque rápido, com os atalhos do dia.
- **Depende de**: 22 (base responsiva) + 21 (presence, pra "ver o que roda" do time) + 18 ("minhas
  tasks" no bolso). **Buildar por ÚLTIMO.**
- **Sem migração** (frontend).

---

## 5. Modelo de dados novo (proposta)

```
invitations                                   -- fase 17 (migração 0017)
  id            uuid pk
  workspace_id  uuid   not null  -> workspaces(id) on delete cascade
  email         text   not null  (normalizado lower/trim — alvo do convite)
  role          workspace_role not null  (member|viewer; owner não se convida)
  token         text   not null unique     (opaco; aceite/lookup)
  status        text   not null  ('pending'|'accepted'|'revoked')  default 'pending'
  invited_by    uuid   not null  -> users(id)
  created_at    timestamptz not null default now()
  expires_at    timestamptz null
  -- unique parcial: 1 convite pending por (workspace_id, email)

task_comments                                 -- fase 19 (migração 0019)
  id            uuid pk
  task_id       uuid   not null  -> tasks(id) on delete cascade
  workspace_id  uuid   not null  (denormalizado p/ filtro tenant, igual tasks)
  author_id     uuid   not null  -> users(id)
  body          text   not null  (1..N validado no service)
  created_at    timestamptz not null default now()
  -- índice por (task_id, created_at)

reminders                                     -- fase 20 (migração 0020)
  id            uuid pk
  user_id       uuid   not null  -> users(id) on delete cascade   (quem é lembrado)
  task_id       uuid   not null  -> tasks(id) on delete cascade
  workspace_id  uuid   not null  (denormalizado)
  remind_at     timestamptz not null
  kind          text   not null  ('due'|'manual')
  status        text   not null  ('pending'|'sent'|'canceled')  default 'pending'
  created_at    timestamptz not null default now()
  -- índice parcial por (remind_at) WHERE status='pending'  (varredura do worker)

tasks (sem ALTER de coluna — assignee/requested_by JÁ existem em 0008)
  + índices novos (migração 0018): idx em (assignee), idx em (requested_by)
```

Colunas herdadas confirmadas no código (não redefinir): `tasks.assignee uuid null`,
`tasks.requested_by uuid null` (FKs a `users`, **dormentes**), `tasks.notes text NOT NULL DEFAULT
''`, `tasks.due_date date null`, `tasks.recurrence text null`, status enum `todo|doing|done`.

**Próximo número de migração livre: `0017`.** Donas fixas: 0017 (17), 0018 (18), 0019 (19),
0020 (20).

---

## 6. Contratos herdados (ancoragem no código real — verificado nesta sessão)

> As specs **ancoram nestes fatos**, não reinventam. Citações conferidas no código em 21/06.

- **Re-vínculo por email** — `workspace/store.go::UpsertUserFromGoogle`: google_id novo + email
  conhecido → `UPDATE users SET google_id=… WHERE email=…`, em transação sob
  `pg_advisory_xact_lock(hashtext(google_id))`. **Fundação do convite por email** (fase 17): o
  lugar onde se resolve a membership pendente é nesse caminho de 1º login.
- **Membership** — `workspace_members` PK `(workspace_id, user_id)`, role enum `owner|member|
  viewer`, FKs `on delete cascade`. `GET /api/v1/workspaces/{id}` já devolve
  `{workspace:{id,name,role,created_at,members:[{user_id,name,email,role}]}}` (extensão aditiva da
  fase 02). `workspace.MemberRole(ws,user)` devolve a role ou `""`.
- **Tasks** — rotas `GET|POST /workspaces/{id}/tasks`, `GET|PATCH|DELETE /…/tasks/{taskId}`
  (`tasks_handlers.go`); `CreateInput`/`PatchInput` **NÃO** têm assignee/requested_by hoje (fase 18
  adiciona). Board da fase 12: `GET /workspaces/{id}/projects/{projectId}/board`.
- **Hub realtime** — `realtime/hub.go`: `Hub{ users map[string]map[Sink]struct{} }`,
  `Broadcast(userID, msg)`, `BroadcastTimer(userID, st, at)` → `{"type":"timer","payload":…}`,
  `BroadcastNotice(userID, payload)` → `{"type":"notice","payload":…}` (aditivo, fase 16).
  **Estritamente per-usuário** — presence (fase 21) adiciona o eixo de workspace; notificações
  (18/20) usam `BroadcastNotice(targetUserID, …)`, que já fan-out pro alvo **sem** workspace.
- **Background + Push** — `guard.Worker` (ticker 60s amarrado ao ctx do boot), `push.Sender`
  (VAPID best-effort; no-op sem chave; limpa 404/410), `push.Subscriptions` (upsert por endpoint),
  tabela `push_subscriptions`. Front: `lib/push.ts`, `lib/realtime.ts::onNotice` → `noticeStore` →
  `NoticeBanner` no `AppShell`. **Lembretes (20) reusam tudo** com um worker próprio.
  **Assinaturas REAIS fixadas (verificadas no código — 18 e 20 chamam com ESTA forma):**
  `push.Sender.SendToUser(ctx context.Context, userID string, payload []byte)` — o `payload` é
  `json.Marshal(push.Payload{Title,Body,Kind})` (a struct `push.Payload` vira **bytes** ANTES da
  chamada; o `Sender` é no-op sem VAPID, best-effort, limpa 404/410); `hub.BroadcastNotice(userID
  string, payload any)` serializa `{"type":"notice","payload":…}`.
- **Frontend shell** — `App.tsx` (rotas), `AppShell.tsx` = `SideNav`(64px) + `NoticeBanner` +
  `TimerBar` + `<Outlet/>`. **Zero responsivo** (nenhum `sm:`/`md:`/`@media`). Mobile (22/23)
  parte daqui.
- **Migrações** — última = `0016_timer_guard`; **0017 livre**.

---

## 7. Decisões travadas (HITL 21/06 — sessão 19)

1. **Convite = os dois**: email-pendente (resolve no 1º login via re-vínculo) **+** buscar user
   existente. **Gestão Owner-only** (convidar/revogar/remover/mudar role).
2. **Tempo real = só presence ao vivo** (broadcast por workspace); cross-assign/notas/status no
   **reload**. Notificações de cross-assign/lembrete chegam pelo `notice` **per-usuário** (não
   exige broadcast por workspace).
3. **Notas = thread de comentários** (tabela nova `task_comments`); `tasks.notes` vira a descrição.
4. **Cross-assign = notifica + visões** "Minhas tasks" / "Pedidas por mim".
5. **Lembretes = due_date + manual**; entrega in-app + Web Push; **recorrência fora**.
6. **Viewer pode receber task** (ser assignee) → carve-out de escrita **escopada** (ver §8).
7. **Mobile = híbrido**, dissecado em **22 (responsivo base)** + **23 (controle remoto dedicado)**.
8. **Decomposição = 7 fases (17–23)**; migrações 0017–0020; 21/22/23 sem migração.
9. **Fase 08 (conector) segue PULADA**; **colaboração ao vivo total** e **IA coach** = future.

---

## 8. Matriz de permissões (colaboração — owner/member/viewer)

> Base: hoje (fase 02) **viewer recebe 403 em qualquer escrita**. A M2 mantém isso, com **um**
> carve-out escopado (viewer-assignee). Owner ⊇ member em tudo de colaboração.

| Ação | Owner | Member | Viewer |
|---|---|---|---|
| Convidar / revogar convite / remover membro / mudar role | ✅ | ❌ 403 | ❌ 403 |
| Criar/editar/mover task, objetivo, projeto, cliente | ✅ | ✅ | ❌ 403 |
| **Ser assignee** de uma task (receber atribuição) | ✅ | ✅ | ✅ (carve-out) |
| Atribuir/pedir task a outro (setar assignee/requested_by) | ✅ | ✅ | ❌ 403 |
| **Mudar o status da task onde é assignee** | ✅ | ✅ | ✅ **só na task dele** |
| Comentar numa task | ✅ | ✅ (tasks visíveis) | ✅ **só na task onde é assignee** |
| Apagar comentário | ✅ (qualquer) | ✅ (próprio) | ✅ (próprio) |
| Criar lembrete pra si | ✅ | ✅ | ✅ (na task dele) |
| Ver presence / membros / relatórios compartilhados | ✅ | ✅ | ✅ (leitura) |

**Carve-out viewer-assignee (preciso):** um viewer continua 403 em toda escrita **exceto** numa
task onde ele é o `assignee` — aí ele pode **mudar o status** dela (agir no que pediram),
**comentar** nela e **criar lembrete pra si** dela. Nada além (não cria/edita outras tasks, não
atribui, não gere membros, não toca task onde não é assignee).

**Princípio único de implementação do carve-out (as fases 18/19/20 ancoram NISTO — não divergem):**
o gate fino — "viewer só age na task onde é assignee, e só nas ações acima" — é SEMPRE decidido no
**service/handler da rota, por-task**, NUNCA no `RequireWriteRole` blanket (que é um middleware de
subrouter, cego à task — `internal/api/middleware_role.go`: 403 viewer em todo POST/PATCH/PUT/DELETE
do subrouter do workspace). As rotas que o viewer-assignee precisa alcançar (`PATCH /tasks/{taskId}`
p/ status, `POST .../comments`, `POST .../reminders`) ficam **fora** do alcance-403 do
`RequireWriteRole` — seja por um **subrouter-irmão** (com `RequireMember`, sem `RequireWriteRole`),
seja por uma **exceção explícita e escopada** ao path no próprio `RequireWriteRole`; o service então
nega o que o carve-out não permite. As demais escritas (criar task, editar outros campos de task,
criar projeto/cliente/objetivo) seguem barradas pelo `RequireWriteRole` (viewer 403). A fase que
estabelecer o subrouter-irmão primeiro define o padrão; as outras reusam. Mutação que amplie o
carve-out (qualquer task, qualquer campo, qualquer ação além das três) DEVE quebrar teste.

---

## 9. Build paralelo (worktrees) — dependências e paralelização

> Requisito de design (não detalhe): cada `plan.md` declara DEPENDÊNCIAS e se é PARALELIZÁVEL.

| Fase | Migração | Pacote backend novo | Toca arquivos compartilhados? | Paralelizável (worktree)? |
|---|---|---|---|---|
| 17 | 0017 | `internal/invitations` | `workspace/handlers.go`, página `/workspace`, caminho de login (re-vínculo) | **Sim** (collab seeda membros por SQL, como hoje) |
| 18 | 0018 (índices) | — (estende `tasks`) | `tasks/service.go`, `tasks_handlers.go`, `App.tsx`, modal/board de task | **Sim**, com cuidado (contende com 19 em tasks_handlers; com 23 em App.tsx) |
| 19 | 0019 | `internal/comments` | rotas (subrouter de task), detalhe da task no front | **Muito** (pacote/rotas/UI quase só novos) |
| 20 | 0020 | `internal/reminders` (worker próprio) | `cmd/server/main.go` (liga o worker), Ajustes/UI de task | **Sim** (isolado; não toca `guard`) |
| 21 | — | estende `internal/realtime` | **`realtime/hub.go`** (core), wiring do timer, `/workspace`/dashboard | **Não** — toca o core de realtime; isolar e buildar sozinho |
| 22 | — | — (frontend) | **MUITOS** componentes (cross-cutting) | **Não** — cross-cutting no front |
| 23 | — | — (frontend) | App.tsx, nav, dashboard mobile | **Não** — depende de 22; buildar por último |

**Ordem recomendada do build:** **17 / 18 / 19 / 20** podem rodar em worktrees paralelas (migrações
e pacotes independentes; vigiar a contenção em `tasks_handlers.go` entre 18 e 19, e `App.tsx`).
**21** isolada (mexe no hub). **22** depois (responsivo cross-cutting), **23** por último (depende de
22/21/18). A consistência de migração (0017–0020 fixas) impede colisão entre worktrees.

**Regras de integração das worktrees (do passe de consistência — não-negociáveis no build):**
1. **Número de migração FIXO por fase** (0017/0018/0019/0020 do PRD), entregue à worktree — NÃO
   "ler o próximo livre do disco" (sob worktrees paralelas todas leem `0016` e colidiriam).
2. **Sequenciar o boot (`cmd/server/main.go`): 18 → 20 → 21.** A 21 reescreve o `OnChange` composto e
   re-aponta o `guard.WithOnTimer` (hoje `main.go:107`); tem que assentar EM CIMA do `OnAssign` (18)
   e do worker de reminders (20). Por isso a 21 entra depois das backend mergeadas.
3. **Arquivos compartilhados = 1 integrador (não merge cego):** `auth_routes.go`/`core_routes.go`
   (17/18/19/20/21 montam Options aditivas), `tasks/service.go` (18+20 estendem `Patch`),
   `TaskModal`/board da 12 (18 picker + 19 thread), `noticeStore.ts` (18+20 ramos de `kind`),
   `Workspace.tsx` (17 gestão + 21 presence + 22 responsivo), `App.tsx`/`SideNav`/`navItems`
   (18+22+23). Todos ADITIVOS; sequenciar/integrar à mão.
4. **Carve-out viewer-assignee** (§8 princípio único): a 1ª fase a criar o subrouter-irmão sem
   `RequireWriteRole` define o padrão; 18/19/20 NÃO podem duplo-cobrir o mesmo path nem afrouxar o
   gate blanket pras outras escritas.

---

## 10. Aproveitamento do `Design/` e do que já existe

| Recurso | Uso na M2 |
|---|---|
| Página `/workspace` (fase 10) | Fase 17 estende (convites + gestão de membros) |
| Modal/board de task (fase 12) | Fase 18 (assignee picker) + 19 (thread) penduram aqui |
| `notice`/`NoticeBanner`/`push.ts` (fase 16) | Fases 18 e 20 reusam pra notificar |
| `guard.Worker`/`push.Sender` (fase 16) | Fase 20 espelha (worker próprio de reminders) |
| Hook `OnChange` do `timer.Service` (03/04) | Fase 21 alimenta o presence |
| Tokens/densidade do `DESIGN.md` | Fases 22/23 (mobile mantém a linguagem; controle remoto = DESIGN §3.3) |
