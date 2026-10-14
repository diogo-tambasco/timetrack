# spec.md — Fase 10 · Página Workspace

> O furo de planejamento fechado: nenhuma fase era dona da UI de `/workspace` (StubPage em
> produção). **Escopo mínimo travado pelo Diogo (12/06, KICKOFF):** nome do workspace ativo +
> lista de membros (nome/email/role, fonte `GET /workspaces/{id}` que JÁ embute `members`
> desde a fase 02) + **seletor de workspace ativo** quando o `/me` trouxer >1 (persistido em
> localStorage; hoje `useCurrentWorkspace` é `workspaces[0]` fixo — vira estado). **SEM
> convite** (não há API no MVP — membros entram por SQL, igual aos testes) e **SEM rename**
> (sem endpoint; candidato a CA futuro). Viewer vê tudo (página é 100% leitura).
> Depende de: fase 01 (`/me`, sessão) + fase 02 (`GET /workspaces/{id}` com members).

---

## 1. Mapa pro REQUISITOS

| Critério | O que esta fase entrega |
|---|---|
| Base multi-workspace desde o dia 1 (§1) | A escolha do workspace ativo vira ESTADO de verdade (persistido), não mais `workspaces[0]` fixo |
| Colaboradores Carlos/Kadu (§1) | A lista de membros+roles fica visível — o que o banco já sabia aparece na UI |
| Minimalista (§1) | Uma página de leitura: nome, membros, seletor. Nada além. |

## 2. User stories

- **US-01** — Como usuário, quero **ver quem está no meu workspace** (nome/email/role), pra
  saber o estado do time sem abrir o banco.
- **US-02** — Como usuário com mais de um workspace, quero **escolher o ativo** e que a
  escolha **persista entre sessões**, pra navegar o app inteiro no contexto certo.
- **US-03** — Como viewer, quero ver a página normalmente — é leitura.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* A página DEVE ser **100% leitura**: nenhuma mutação de API exposta
  (sem convite, sem rename, sem remoção de membro). *(Verificação: revisão de imports do
  `Workspace.tsx` no fechamento — só queries/auth/ícones; zero `useMutation`/POST/PATCH/
  DELETE.)*
- **CA-02** *(ubíquo)* **Zero mudança de backend.** Arquivos compartilhados tocados
  (enumeração fechada): `lib/auth.tsx` (workspace ativo vira estado — mudança ADITIVA:
  sem id salvo o comportamento é IDÊNTICO ao atual `workspaces[0]`),
  `routes/cockpit/Workspace.tsx` (substitui o StubPage — entregável da fase) e
  `frontend/e2e/helpers.ts` (helper novo de seed de 2º workspace — função NOVA). Suítes
  das fases 00–07 seguem verdes sem edição de asserções. *(Verificação: `git diff` no
  fechamento, precedente do CA-04 da 04.)*

### 3.2 Página (state-driven)

- **CA-03** ENQUANTO `/workspace` está aberta, a página DEVE exibir o **nome do workspace
  ativo** e a **lista de membros** com nome, email e role de cada um (fonte:
  `GET /workspaces/{id}` — `useWorkspaceMembers` existente + nome do detalhe).
- **CA-04** A role de cada membro DEVE aparecer como **badge** (`.tt-role`; dono com
  `.tt-role--owner` em destaque), com rótulos PT-BR: `owner`→Dono, `member`→Membro,
  `viewer`→Viewer. A ordem é a da API (dono primeiro, depois por nome — handler da 02).
- **CA-05** SE o `/me` traz **um único** workspace, NENHUM seletor DEVE aparecer (só o
  nome — o seletor é ruído pro caso single-workspace de hoje).

### 3.3 Workspace ativo (event-driven)

- **CA-06** SE o `/me` traz **mais de um** workspace, a página DEVE exibir o **seletor**;
  QUANDO o usuário troca, `useCurrentWorkspace()` DEVE passar a devolver o escolhido em
  TODO o app (TimerBar, páginas, queries) e a escolha DEVE ser persistida em
  `localStorage['tt.ws.active']`.
- **CA-07** QUANDO o app abre com um id persistido, o workspace ativo DEVE ser o salvo;
  SE o id salvo não existe mais no `/me` (removido do workspace), o app DEVE cair em
  `workspaces[0]` **sem erro** (e sem limpar a tela).
- **CA-08** QUANDO o workspace ativo muda, as queries de domínio DEVEM refletir o novo
  contexto: toda query é keyed por `ws.id` (chave nova → busca nova) E o switch invalida o
  cache (dados nunca ficam do workspace anterior na tela).
- **CA-09** Viewer DEVE ver a página igual aos demais (leitura — nenhum gating novo).

### 3.4 E2E (Playwright)

- **CA-10** Fluxo completo num arquivo NOVO **paralelo** (a página não muta timer):
  login → `/workspace` → nome do workspace pessoal visível + o próprio usuário listado
  como **Dono** → membro extra semeado por SQL aparece com a role certa → 2º workspace
  semeado por SQL + reload → seletor aparece → trocar → escolha persiste após reload
  (localStorage) → voltar pro pessoal (deixa o contexto limpo pros outros testes).

## 4. Requisitos funcionais (resumo)

- **RF-01** `useCurrentWorkspace()` em `lib/auth.tsx` passa a resolver:
  `me.workspaces.find(w => w.id === ativo) ?? workspaces[0]`; setter novo
  `setActiveWorkspace(id)` persiste em `localStorage['tt.ws.active']`.
- **RF-02** Página `Workspace.tsx` real: nome + membros + seletor condicional.
- **RF-03** Helper E2E `seedSecondWorkspace` (SQL direto no db-test, padrão
  `seedWorkspaceMember` — idempotente).

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** zero dependência nova; zero rota nova.
- **RNF-02 Compat:** sem id salvo, o comportamento é byte-idêntico ao atual
  (`workspaces[0]` — a ordem do `/me` é `ORDER BY w.created_at`, então o workspace
  PESSOAL segue primeiro pra todo mundo; fato verificado no `workspace/store.go:158`).

## 6. Fora de escopo (fronteiras explícitas)

- **Convite/remoção de membro** — não há API no MVP (membros por SQL, decisão travada).
- **Rename de workspace** — sem endpoint; candidato a CA futuro.
- **Criar workspace novo** — fora do MVP.
- **Timer cross-workspace:** o timer é POR USUÁRIO (fase 03); trocar workspace NÃO pausa
  nem esconde o timer rodando. Se a entry rodando é de OUTRO workspace, o chip de projeto
  da TimerBar pode não resolver o nome (a query de projetos é do ativo) — limitação
  conhecida e registrada; vira CA quando o multiplayer real entrar (fora do MVP, §5 do
  REQUISITOS).
