# tasks.md — Fase 10 · Página Workspace

> Tarefas atômicas, TDD red→green→refactor, **commit por tarefa** (`TDD.md §1`).
> Fase 100% frontend — Vitest + Playwright; `make test-back` roda no gate pra provar o CA-02.

---

## T-01 — Workspace ativo como estado (lib/auth.tsx)

- **Objetivo:** `activeWorkspaceId` + `setActiveWorkspace` no `useAuthStore` (plan D-01/
  D-02): persiste em `localStorage['tt.ws.active']`, boot lê o salvo,
  `useCurrentWorkspace()` resolve `find(ativo) ?? workspaces[0]`.
- **RED:** casos novos em spec (criar `lib/auth.spec.tsx`): (a) sem id salvo →
  `useCurrentWorkspace` devolve `workspaces[0]` (compat RNF-02); (b) `setActiveWorkspace`
  muda o devolvido E grava no localStorage (CA-06 metade store); (c) boot com id salvo →
  devolve o salvo (CA-07); (d) id salvo que não existe no `/me` → `workspaces[0]` sem
  erro (CA-07).
- **Acceptance:** casos novos verdes; **suítes existentes intactas** (CA-02 — nenhum
  componente muda de comportamento sem id salvo).

## T-02 — Página Workspace real (CA-03/04/05/09)

- **Objetivo:** `Workspace.tsx` substitui o StubPage: nome do workspace (do
  `getWorkspace` — D-05), lista de membros (nome/email/badge de role — D-06), nada de
  escrita.
- **RED:** `Workspace.spec.tsx` (padrão das páginas: mock de `../../lib/api` +
  QueryClientProvider + `useAuthStore.setState`): CA-03 nome + membros renderizados;
  CA-04 badges com rótulos Dono/Membro/Viewer e `--owner` no dono; CA-05 com 1 workspace
  NENHUM seletor (queryByRole('combobox') null); CA-09 viewer renderiza igual (setState
  com role viewer — sem gating).
- **Acceptance:** spec verde; `check-cores.sh` passa.

## T-03 — Seletor de workspace ativo (CA-06/07/08)

- **Objetivo:** seletor condicional (>1 ws — D-04) ligado no `setActiveWorkspace` +
  `queryClient.invalidateQueries()` no switch (D-03).
- **RED:** casos novos no `Workspace.spec.tsx`: (a) 2 workspaces → combobox aparece com os
  2 nomes; (b) trocar → `useCurrentWorkspace` passa a devolver o outro (assert via
  componente-sonda que imprime o id ativo) + `localStorage['tt.ws.active']` gravado
  (CA-06); (c) trocar → `invalidateQueries` chamado (spy no queryClient) E o
  `getWorkspace` é re-buscado com o id NOVO (CA-08).
- **Acceptance:** spec verde.

## T-04 — E2E + verificações + gate + INDEX (CA-10)

- **Objetivo:** `frontend/e2e/workspace.spec.ts` (arquivo NOVO paralelo) + helper
  `seedSecondWorkspace` em `e2e/helpers.ts` (RF-03, idempotente): login → `/workspace` →
  nome + Dono visíveis → `seedWorkspaceMember` aparece com role → `seedSecondWorkspace` +
  reload → seletor → trocar → reload → persistiu → **volta pro pessoal e limpa** (plan §6).
- **Verificações delegadas (antes do gate):** CA-01 revisão de imports do `Workspace.tsx`
  (zero mutação); CA-02 `git diff` × enumeração (lib/auth.tsx, Workspace.tsx, helpers.ts).
- **Depois:** `make check-full` (GATE — derrubar `make up` antes do Playwright) e
  `specs/INDEX.md` marcando a 10 construída.
- **Acceptance:** GATE VERDE; INDEX atualizado; verificações registradas no commit.
