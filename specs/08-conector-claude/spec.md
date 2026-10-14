# spec.md — Fase 08 · Conector Claude (API token + skill /timetrack-task)

> O Diogo começa muita task de dentro do Claude e quer controlar o timer sem trocar de app
> (`REQUISITOS.md §4.6`): o backend ganha **autenticação por token de API** (`tt_…`, hash em
> repouso como as sessões) e uma **skill global `/timetrack-task`** que pergunta projeto +
> nome da task, **cria a task e JÁ INICIA o timer**. Gestão dos tokens nos Ajustes
> (gerar/listar/revogar).
> Depende de: fase 01 (RequireAuth — o `authenticate` já foi desenhado extensível pra
> "fase 08 pluga o token tt_ aqui"), fase 02 (projects/tasks), fase 03 (`POST /timer/start`).
>
> **Decisões do Diogo (11/06, travadas no KICKOFF):** tabela nova `0012+`, hash em repouso ·
> seção "API / Conector" nos Ajustes (gerar, listar, revogar) · skill instalada **GLOBAL**
> (`~/.claude/skills/timetrack-task`) lendo o token de **arquivo local**
> (`~/.timetrack-token`, nunca commitado) · fluxo: pergunta projeto + nome da task, cria e
> **já começa a cronometrar** (`POST /timer/start`).

---

## 1. Mapa pro problema central

| Critério (`REQUISITOS.md`) | O que esta fase entrega |
|---|---|
| §4.6 conector Claude dia 1 | API autenticada por token + skill `/timetrack-task` criando task e ligando o timer |
| §6 segurança (tokens com escopo, segredos fora do git) | hash SHA-256 em repouso, claro exibido 1 única vez, gestão só por sessão de browser, token local em `~/.timetrack-token` fora de qualquer repo |
| TDD.md §4 fluxo nº6 | `/timetrack-task` → escolhe projeto+nome → task criada **e timer rodando** (via API token) |

## 2. User stories

- **US-01** — Como Diogo dentro do Claude Code, quero rodar `/timetrack-task`, escolher o
  projeto e dar o nome da task, e **ver o timer já rodando** no Timetrack — sem abrir o app.
- **US-02** — Como dono da conta, quero **gerar/listar/revogar** tokens nos Ajustes, com o
  claro exibido **uma única vez**, pra controlar quem/o quê acessa minha API.
- **US-03** — Como dono da conta, quero que um **token vazado não consiga criar outros
  tokens** nem listar os existentes — gestão é só pelo browser logado.
- **US-04** — Como usuário da API, quero erros canônicos (401 claro) quando o token for
  inválido ou revogado, pra skill orientar certo ("gere um novo nos Ajustes").

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Token e autenticação (ubíquos / event-driven)

- **CA-01** *(ubíquo)* O token em claro NUNCA fica em repouso: o banco guarda **só o
  SHA-256** (mesmo padrão das sessões da 01); o claro aparece **exclusivamente** no corpo
  do `201` do POST de criação. Nenhuma outra resposta (lista, detalhe) contém token ou hash.
- **CA-02** O token gerado DEVE ter o formato **`tt_` + 43 chars base64url** (32 bytes de
  `crypto/rand` — prefixo do baseline `SDD.md §7.5`), com `hint` de exibição
  (`tt_abcd…wxyz`: 7 primeiros + 4 últimos).
- **CA-03** QUANDO uma requisição chega com `Authorization: Bearer <token válido>`, o
  sistema DEVE autenticá-la como o **dono do token** — toda rota atrás do `RequireAuth`
  funciona por token (timer, workspaces, entries…), sem cookie.
- **CA-04** SE o header `Authorization: Bearer` está presente mas o token é
  inválido/revogado/malformado, o sistema DEVE responder `401` canônico **sem fallback pro
  cookie** (credencial explícita presente = ela decide).
- **CA-05** Sem credencial nenhuma, o comportamento da fase 01 segue intacto (`401`;
  cookie de sessão continua funcionando como sempre) — regressão zero.
- **CA-06** QUANDO um token autentica uma requisição, `last_used_at` DEVE ser atualizado
  (a lista dos Ajustes mostra "usado por último em…").
- **CA-07** A revogação tem efeito **imediato**: token revogado responde `401` na
  requisição seguinte (sem cache de autenticação).

### 3.2 Gestão de tokens (event-driven)

- **CA-08** `POST /api/v1/tokens {name}` → `201 {id, name, token, hint, created_at}`;
  `name` ausente/vazio/>100 chars → `400 validation_error`.
- **CA-09** `GET /api/v1/tokens` → lista **só do próprio usuário** (`id, name, hint,
  created_at, last_used_at`) — nunca o claro nem o hash; tokens de outro usuário nunca
  aparecem (isolamento).
- **CA-10** `DELETE /api/v1/tokens/{id}` → `204` e o token some da lista; id de outro
  usuário, inexistente ou malformado → `404` uniforme (não vaza existência).
- **CA-11** As rotas de gestão (`/api/v1/tokens*`) DEVEM exigir **sessão por cookie**:
  requisição autenticada por TOKEN nelas → `403` (um token vazado não cria, lista nem
  revoga tokens).

### 3.3 Fluxo do conector (integração — o caminho exato da skill)

- **CA-12** Fluxo completo **só com Bearer** (zero cookie): `GET /me` → workspaces →
  `GET /workspaces/{id}/projects` → `POST /workspaces/{id}/tasks` (projeto escolhido +
  nome) → `POST /timer/start` (description = nome da task, `project_id` + `task_id`) →
  `GET /timer` mostra o timer **rodando com a task** — task criada E cronômetro ligado
  (TDD.md §4 nº6).
- **CA-13** A cadeia de permissão da 03 vale igual por token: start com `workspace_id` de
  workspace alheio → `404`; com role viewer → `403` (token não é bypass).

### 3.4 Ajustes — seção "API / Conector" (front)

- **CA-14** A página Ajustes DEVE ganhar a seção **"API / Conector"**: lista dos tokens
  (name, hint, criado em, usado por último) + botão **Gerar token** (com campo nome) +
  **Revogar** por linha.
- **CA-15** QUANDO um token é gerado, o claro aparece **uma única vez** (caixa com botão
  copiar + aviso "não será exibido de novo"); fechado o aviso (ou recarregada a página), a
  lista exibe **só o hint**.
- **CA-16** QUANDO o usuário revoga um token, a linha some da lista **sem reload**
  (invalidação da query, padrão das mutações da 02).

### 3.5 Skill `/timetrack-task` (artefato)

- **CA-17** O repo ganha `skill/timetrack-task/SKILL.md` (fonte versionada) e a skill é
  instalada **global** em `~/.claude/skills/timetrack-task/` (cópia byte-idêntica). A skill
  lê o token de `~/.timetrack-token` (arquivo local, fora de qualquer repo) e **nunca**
  instrui commitar/echoar o token; sem o arquivo, instrui gerar nos Ajustes
  (`https://timmingtrack.life/ajustes`) e salvar com `chmod 600`.
- **CA-18** O fluxo documentado na skill DEVE ser exatamente o do CA-12 (mesmas rotas,
  mesma ordem), perguntando **projeto** (lista dos ativos) e **nome da task** quando não
  vierem nos args, e terminando com a confirmação "task criada + timer rodando". *(Prova:
  o CA-12 testa o caminho de API; a skill em si é markdown — validada por review contra o
  CA-12 + smoke manual do Diogo no fim da sessão.)*

### 3.6 E2E (Playwright)

- **CA-19** E2E dos Ajustes: gerar token → o claro `tt_…` aparece (uma vez) → uma chamada
  `GET /api/v1/me` **com esse Bearer e SEM cookies** responde 200 → reload → só hint na
  lista → revogar → some da lista E a mesma chamada com o token passa a responder 401.

## 4. Requisitos funcionais (resumo)

- **RF-01** Migração `0012_api_tokens` (dona fixa desta fase, INDEX.md §migrações).
- **RF-02** Ramo Bearer no `authenticate` da fase 01 (o desenho D-9 da 01 já previu);
  método de autenticação exposto no contexto da requisição (pro CA-11).
- **RF-03** Rotas `/api/v1/tokens` (POST/GET/DELETE) por usuário, fora do subrouter de
  workspace (padrão settings/timer da 03).
- **RF-04** Seção "API / Conector" nos Ajustes (gerar/listar/revogar).
- **RF-05** Skill `/timetrack-task` versionada no repo + instalada global.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** zero dependência nova (Go e JS) — token usa `crypto/rand` + SHA-256
  já existentes.
- **RNF-02 Segurança:** hash em repouso (CA-01) · claro 1 vez (CA-15) · gestão
  session-only (CA-11) · token sem expiração mas revogável a qualquer momento (decisão
  registrada: é credencial de máquina pessoal; expiração viraria atrito sem ameaça nova
  no modelo single-user — re-avaliar quando houver colaborador) · nunca logar token
  (constituição §7; sem log de header Authorization em lugar nenhum).
- **RNF-03 Compat:** nenhum contrato existente muda; suítes 00–07 verdes sem edição de
  asserções.

## 6. Fora de escopo (fronteiras explícitas)

- **Escopo por workspace no token** (baseline §7.5 menciona) — v1 é single-user: o token
  autentica o usuário e a permissão por workspace continua sendo a membership/role normal
  (CA-13). Coluna de escopo futura é aditiva; registrado pra não re-litigar.
- **Expiração/rotação automática de token** — RNF-02 registra a decisão.
- **Rate limiting / auditoria de uso além do `last_used_at`** — futuro.
- **Skill com mais comandos** (parar timer, relatório, listar o dia) — a v1 é o fluxo nº6;
  candidatos pra iteração seguinte.
- **MCP server** — a skill via API token resolve o MVP; MCP é outra superfície, futuro.
