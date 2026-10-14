# plan.md — Fase 10 · Página Workspace

> Como implementar a `spec.md` desta pasta. Herda o baseline (`SDD.md §7`) e os contratos
> das fases 01/02: `GET /me` (`workspaces` ordenados por `created_at` — pessoal primeiro),
> `GET /workspaces/{id}` (embute `members`), `useWorkspaceMembers` (queries/workspace.ts),
> `useAuthStore`/`useCurrentWorkspace` (lib/auth.tsx). **Fase 100% frontend.**

---

## 1. Decisões de arquitetura (com razão)

| # | Decisão | Razão |
|---|---|---|
| D-01 | **Workspace ativo mora no `useAuthStore`** (`activeWorkspaceId: string \| null` + `setActiveWorkspace(id)`), inicializado de `localStorage['tt.ws.active']` na criação do store. `useCurrentWorkspace()` resolve `find(ativo) ?? workspaces[0]`. | O store de auth já é a fonte do `/me`; um store novo seria estado paralelo. O fallback `?? workspaces[0]` dá o CA-07 (id morto) e o RNF-02 (compat byte-idêntica sem id salvo) de graça. |
| D-02 | **Persistência síncrona no setter** (`localStorage.setItem` dentro de `setActiveWorkspace`), leitura síncrona no boot do store. Chave `tt.ws.active` (padrão `tt.view.*` do ViewToggle). | Sem efeito/registro extra; o reload restaura sozinho (CA-07). localStorage indisponível (Safari private) → try/catch silencioso, cai no default. |
| D-03 | **O switch invalida o cache inteiro do TanStack** (`queryClient.invalidateQueries()` sem filtro, no handler do seletor). | CA-08. As chaves já são por `ws.id` (busca nova de qualquer jeito); a invalidação total é barata (poucas queries vivas) e elimina QUALQUER resíduo do contexto anterior — mais simples que enumerar prefixos de chave, e à prova de fase futura que esquecer o ws.id na chave. |
| D-04 | **Seletor = `<select>` nativo** no cabeçalho da página, visível só com `workspaces.length > 1` (CA-05/06). | Caso raro hoje (single-user); um dropdown custom seria polish sem CA. O `<select>` herda o estilo dos selects da TimerBar. |
| D-05 | **Nome do workspace vem do `getWorkspace`** (mesma resposta que traz os members — 1 fonte, 1 query), não do `/me`. | O `/me` tem o nome, mas a página já busca o detalhe pros members; usar a MESMA resposta evita o caso "nome de uma fonte, membros de outra". |
| D-06 | **Roles PT-BR fixas** no componente: `owner`→Dono, `member`→Membro, `viewer`→Viewer; badge `.tt-role` (+`--owner`). | CA-04; tokens prontos no tokens.css desde a fase 00. |
| D-07 | **Timer intocado no switch.** Trocar workspace não mexe no timerStore (timer é POR USUÁRIO — fase 03). | Limitação do chip cross-ws registrada na spec §6; mexer aqui seria escopo de multiplayer (fora do MVP). |

## 2. Modelo de dados / migrações

**Nenhuma.** Fase 100% frontend (CA-02).

## 3. Contratos

Nenhum contrato novo. Consumo (tudo existente): `GET /me` (workspaces+roles) ·
`GET /workspaces/{id}` (nome + members) · `useWorkspaceMembers` · `workspaceKey`.

## 4. Libs

**Nenhuma nova** (RNF-01).

## 5. Onde vive o quê

```
frontend/src/
  lib/auth.tsx             activeWorkspaceId + setActiveWorkspace + useCurrentWorkspace
                           resolvendo o ativo (D-01/D-02) — mudança ADITIVA
  routes/cockpit/
    Workspace.tsx          a página real (substitui o StubPage)
    Workspace.spec.tsx     CA-03..09 unit (padrão das páginas da 02/03)
frontend/e2e/
  helpers.ts               seedSecondWorkspace (RF-03 — SQL idempotente, padrão seedWorkspaceMember)
  workspace.spec.ts        CA-10 — arquivo NOVO paralelo (não muta timer)
```

## 6. Pontos de integração e armadilhas conhecidas

- **Ordem do `/me` é load-bearing:** `ORDER BY w.created_at` (`workspace/store.go:158`)
  garante que o pessoal segue `workspaces[0]` mesmo depois que o E2E semear um 2º
  workspace no banco REUSADO — sem isso, o seed da 10 quebraria o `cronometro.spec`
  (projetos criados no pessoal). NÃO mudar a ordem do `/me`.
- **E2E paralelo + banco reusado:** `seedSecondWorkspace` idempotente (`ON CONFLICT` /
  `WHERE NOT EXISTS` por nome fixo); o teste TERMINA voltando o seletor pro workspace
  pessoal e limpando `tt.ws.active` (outros contextos nascem limpos, mas o PRÓPRIO
  contexto reusado num retry não pode herdar o ws errado).
- **`Workspace.spec.tsx` com >1 ws:** semeia `useAuthStore.setState` com 2 workspaces —
  nenhum mock novo além de `getWorkspace`.
- **localStorage no jsdom:** disponível; `localStorage.clear()` no beforeEach (padrão
  ViewToggle.spec). O store de auth é módulo-singleton: o spec reseta `activeWorkspaceId`
  explicitamente no setState (não confiar em import order).
- **Boot:** `RequireSession` já carrega o `/me`; nenhum gate novo. A página com `ws null`
  (me ainda carregando) renderiza vazio sem erro (padrão das outras páginas).
- **check-cores:** badges usam tokens existentes; nenhuma cor nova.
