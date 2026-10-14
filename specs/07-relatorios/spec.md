# spec.md — Fase 07 · Relatórios (ângulos 1–3)

> Os insights do `REQUISITOS.md §4.5`, **recorte travado pelo Diogo (11/06)**: SÓ os ângulos
> **1 (tempo por cliente/projeto)**, **2 (foco vs. fragmentação)** e **3 (meta vs.
> realizado)**. O ângulo **4 (planejado vs. real) NÃO entra** — vai pra fase 06 COM os dados
> do Calendar, **sem placeholder aqui**. Período do relatório = **semana** (a meta de projeto
> é semanal — `weekly_hours_goal` do schema da 02 — então a semana resolve o ângulo 3 sem
> inventar conversão).
> Depende de: fase 02 (projects/clients, metas) + fase 03 (time_entries, recorte por tz).

---

## 1. Mapa pros insights (`REQUISITOS.md §4.5`)

| Ângulo | O que esta fase entrega |
|---|---|
| 1. Tempo por cliente/projeto | Total da semana por projeto (com cliente e cor), bucket "sem projeto", barras proporcionais |
| 2. Foco vs. fragmentação | Blocos longos (≥25min) × fragmentos (<25min): tempo e contagem de cada, maior bloco, trocas de contexto no dia |
| 3. Meta vs. realizado | Por projeto com meta semanal: meta × realizado na semana (reusa a GoalBar da 02) |
| 4. Planejado vs. real | **FORA** — fase 06 (Calendar). Nem placeholder. |

## 2. User stories

- **US-01** — Como usuário, quero ver **quanto dei pra cada cliente/projeto na semana**
  (Acme, Lauda, freela, Arkan, Primor), pra saber onde meu tempo foi de verdade.
- **US-02** — Como usuário, quero ver se trabalhei em **blocos longos ou no pula-pula**,
  pra ajustar minha rotina de foco.
- **US-03** — Como usuário com meta de horas (ex.: freela 10h/sem), quero ver **meta ×
  realizado** da semana e o desvio, pra corrigir o rumo antes do fim da semana.
- **US-04** — Como usuário, quero **navegar entre semanas** (esta, passada, retrasada…),
  pra revisar o histórico sem exportar nada.

## 3. Critérios de aceite (EARS) — cada CA vira exatamente 1 teste

### 3.1 Invariantes (ubíquos)

- **CA-01** *(ubíquo)* O relatório DEVE agregar **apenas entries do próprio usuário** no
  workspace da rota (filtro duplo workspace+user, CA-01/01.1 da fase 03): entries de outro
  membro do MESMO workspace não entram em NENHUMA seção.
- **CA-02** *(ubíquo)* A rota atravessa a cadeia da 01/02: sem sessão → `401`; workspace
  alheio/inexistente → `404` uniforme (não vaza existência).
- **CA-03** *(ubíquo)* **Viewer PODE ler** o relatório (GET passa no `RequireWriteRole` —
  CA-24 da fase 02): relatório é leitura do próprio tempo.
- **CA-04** *(ubíquo)* Entry **rodando** (sem `stopped_at`) fica FORA de todas as
  agregações — só entries fechadas contam (`duration_seconds` não-nulo).
- **CA-05** *(ubíquo)* Nenhum contrato existente muda (rota nova é aditiva); suítes das
  fases 00–05 seguem verdes sem edição de asserções.

### 3.2 Janela da semana (event-driven)

- **CA-06** QUANDO `GET .../reports/week` chega com `date` (YYYY-MM-DD) e `tz` (IANA), o
  sistema DEVE devolver a semana **segunda→domingo** que contém `date` no fuso `tz`
  (janela `[segunda 00:00, segunda+7d 00:00)` no fuso), com `week_start`/`week_end`
  (datas) na resposta. `date` ausente → hoje no fuso; `tz` ausente → UTC (defaults da 03).
- **CA-07** O recorte é por `started_at` **no fuso**: entry de domingo 23:30 em
  America/Sao_Paulo pertence à semana que termina; segunda 00:30 à seguinte (mesmo padrão
  do CA-14.1 da fase 03, provado com horário que cruza o dia em UTC).
- **CA-08** SE `date` ou `tz` forem malformados, o sistema DEVE responder
  `400 validation_error` (convenção do INDEX).

### 3.3 Ângulo 1 — tempo por cliente/projeto (state-driven)

- **CA-09** A seção `by_project` DEVE trazer o total de segundos por projeto na semana,
  ordenado por total DESC, com `name`/`color`/`client_id`/`client_name` resolvidos —
  e `total_seconds` do relatório = soma exata dos buckets.
- **CA-10** Entries **sem projeto** DEVEM virar o bucket `project_id: null` (name/color/
  client nulos) — tempo sem projeto não some do total.
- **CA-11** Projeto **arquivado** com tempo na semana DEVE aparecer com nome e cor (o
  arquivamento não quebra vínculo — CA-05 da fase 02).

### 3.4 Ângulo 2 — foco vs. fragmentação (state-driven)

- **CA-12** Bloco de **foco** = entry fechada com duração **≥ 25min** (1500s, limiar
  inclusivo); o resto é **fragmento**. A seção `focus` DEVE trazer
  `focus_seconds`/`fragment_seconds`/`focus_count`/`fragment_count` corretos (caso de
  borda: 1500s exatos conta como foco).
- **CA-13** `longest_seconds` = duração da maior entry da semana (0 em semana vazia).
- **CA-14** `switches` = nº de **trocas de projeto entre entries consecutivas do MESMO dia
  local** (ordenadas por `started_at`): projeto→outro projeto conta; projeto→sem-projeto
  conta; a primeira entry de cada dia NÃO conta (não há troca entre dias).

### 3.5 Ângulo 3 — meta vs. realizado (state-driven)

- **CA-15** A seção `goals` DEVE listar os projetos do workspace com `weekly_hours_goal`
  não-nulo, **não-arquivados** e **visíveis pro usuário** (mesma regra `visibleWhere` da
  02: shared OU dono — projeto privado de OUTRO usuário com meta NÃO aparece), cada um com
  `goal_seconds` e `realized_seconds` (só a semana, só o usuário) — **incluindo projetos
  com 0 realizado** (é exatamente o desvio que o usuário precisa ver).
- **CA-16** `goal_seconds` = `weekly_hours_goal × 3600` com meta fracionária exata
  (12.25h → 44100s — o schema da 02 aceita fração, CA-07 da 02).

### 3.6 Frontend (state/event-driven)

- **CA-17** A página `/relatorios` DEVE exibir as 3 seções e a **navegação de semana**
  (‹ anterior / próxima ›, rótulo derivado de `week_start`/`week_end` **da resposta** —
  fonte única é o servidor); navegar refaz a busca com o `date` da semana alvo.
- **CA-18** O ângulo 1 DEVE renderizar barras proporcionais ao tempo na **cor do projeto**
  (bucket sem projeto em cor neutra de token).
- **CA-19** O ângulo 3 DEVE reusar a **GoalBar** da fase 02 (fill na cor do projeto,
  accent quando ≥100% — CA-30 da 02), com meta e realizado formatados.
- **CA-20** Semana sem entries → as 3 seções exibem estado vazio amigável, sem erro e sem
  NaN.
- **CA-21** Toda chamada do relatório DEVE ir com o fuso IANA do navegador (`browserTZ` —
  mesmo padrão do CA-35.1 da fase 03).

### 3.7 E2E (Playwright — fluxo nº5 do TDD.md §4, recorte 3 ângulos)

- **CA-22** Num projeto ÚNICO novo com meta: semeia 2 entries fechadas de hoje via API
  (uma ≥25min, uma <25min) → `/relatorios` mostra o projeto no ângulo 1 com o total certo
  (escopo por projeto = determinístico mesmo com outros testes rodando), a barra de meta
  no ângulo 3 com o realizado, e a seção de foco presente com números renderizados.

## 4. Requisitos funcionais (resumo)

- **RF-01** Pacote `internal/reports` (agregações testáveis com o Postgres de teste).
- **RF-02** Rota nova `GET /api/v1/workspaces/{workspaceId}/reports/week?date&tz` no
  subrouter do workspace (cadeia sessão→membership; leitura liberada pra viewer).
- **RF-03** Página `/relatorios` real (substitui o StubPage) com as 3 seções.
- **RF-04** Formatação de durações reusa `fmtDayTotal` (fase 05) — "10h30".

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** **zero dependência nova** (Go e JS) — sem lib de gráfico; barras em
  CSS puro com tokens.
- **RNF-02 Perf:** relatório da semana responde em **< 200ms** com 1.000+ entries no
  workspace (mesmo padrão do RNF da fase 02; asserção de teste com tolerância de harness).
- **RNF-03 Ângulo 4 inexistente:** nenhuma menção, aba, card ou placeholder de
  "planejado vs. real" — entra INTEIRO na fase 06.
- **RNF-04 Sem estado novo:** relatório é derivado 100% de `time_entries`/`projects`/
  `clients` existentes — nenhuma migração, nenhuma tabela de agregados.

## 6. Fora de escopo (fronteiras explícitas)

- **Ângulo 4 (planejado vs. real)** → fase 06, com os dados do Calendar (decisão travada).
- **Períodos custom / visão mensal / comparação entre semanas** — v1 é semanal navegável.
- **Relatório de equipe** (tempo de outros membros) — multiplayer fora do MVP (§8.8).
- **Export (CSV/PDF), e-mail semanal, resumo IA** — `REQUISITOS.md §4.8` "logo em
  seguida", não nesta fase.
- **Limiar de foco configurável** — 25min fixo na v1 (constante documentada no plan).
- **Cache/materialização de agregados** — RNF-02 prova que não precisa.
