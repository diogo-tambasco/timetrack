# spec.md — Fase 14 · Relatórios 2.0 (período flexível, filtros, objetivos, comparação, KPIs)

> Fonte de verdade da fase. **Estende a fase 07** (ângulos 1–3, `GET /workspaces/{id}/reports/week`)
> e **não regride a fase 06** (ângulo 4 Planejado×Real, `GET /workspaces/{id}/reports/planned`).
> Base de design: `Design/Timmingtrack.life/timetrack/reports.jsx` (versão EXPANDIDA já
> desenhada — KPI strip, foco por hora, rastreado×não-rastreado em donut, toggle Gráficos↔Tabela).
> **Consome objetivos da fase 12** (tabela `objectives`, `tasks.objective_id` — MILESTONE-1 §5)
> e a noção de entrega (tasks `done`).
> **Decisões travadas com o Diogo (MILESTONE-1 §7, 19/06/2026):**
> 1. **Período flexível:** dia / semana / mês / **intervalo custom** (hoje só semana).
> 2. **Filtros:** projeto, cliente e **objetivo**.
> 3. **Progresso de objetivos = planejado × o que SAIU medido em ENTREGA** (tasks concluídas /
>    objetivos concluídos), **não só horas**.
> 4. **Comparar períodos:** este período × anterior (semana × anterior, mês × anterior).
> 5. **KPIs + cards novos:** rastreado×não-rastreado (donut), foco por hora.
> 6. **Toggle Gráficos ↔ Tabela** (tabela densa on-demand).
> 7. **Export FORA** desta fase (o Diogo decide depois — ver §7).

---

## 1. Objetivo

Transformar `/relatorios` (hoje uma página semanal fixa com 3 ângulos + o ângulo 4 da 06) num
painel de insights **com período flexível, filtros e comparação**, que mede o **progresso de
objetivos por entrega** (não só horas), exibe uma **faixa de KPIs** e cards novos (rastreado×
não-rastreado, foco por hora), com **toggle Gráficos↔Tabela**.

A fase 07 é a fundação: `internal/reports` e os ângulos 1–3 são reusados e **generalizados** de
"semana" pra "período arbitrário" (`from`/`to` no fuso). O ângulo 4 da 06 e o `/reports/week`
existente **não podem quebrar** (não-regressão — RNF-05). O grosso é backend (generalizar a
janela, agregar por objetivo/entrega, somar disponível×rastreado, foco por hora, comparação) +
frontend (período flexível, filtros, KPIs, novos cards, tabela).

## 2. User stories

- **US-01** — Como usuário, quero ver relatórios de **um dia, uma semana, um mês ou um intervalo
  que eu escolho**, pra revisar o recorte que me interessa, não só "a semana".
- **US-02** — Como usuário, quero **filtrar por projeto, cliente e objetivo**, pra isolar quanto
  rendi num cliente ou num objetivo específico.
- **US-03** — Como usuário com objetivos por projeto, quero ver o **progresso de cada objetivo
  pela ENTREGA** (% de tasks concluídas + horas, se houver meta), pra saber se estou avançando de
  verdade, não só "queimando horas".
- **US-04** — Como usuário, quero **comparar este período com o anterior** (esta semana × passada,
  este mês × anterior), pra ver se melhorei ou piorei.
- **US-05** — Como usuário, quero uma **faixa de KPIs** (rastreado no período, foco profundo %,
  sessão média, não-rastreado) pra bater o olho no resumo do período.
- **US-06** — Como usuário, quero ver **quanto do meu tempo disponível eu de fato rastreei**
  (rastreado × não-rastreado) e **em que horas eu foco mais** (foco por hora), pra ajustar rotina.
- **US-07** — Como usuário, quero **alternar entre Gráficos e uma Tabela densa**, pra exportar
  com o olho / conferir números exatos quando precisar.

## 3. Critérios de aceite (EARS — cada CA vira exatamente 1 teste)

### 3.1 Invariantes (ubíquos) — herdados da 07, valem no período generalizado

- **CA-01** *(ubíquo)* O relatório DEVE agregar **apenas entries do próprio usuário** no
  workspace da rota (filtro duplo workspace+user — CA-01 da 07): entries de outro membro do MESMO
  workspace não entram em NENHUMA seção, em NENHUM período.
- **CA-02** *(ubíquo)* A rota atravessa a cadeia 01/02: sem sessão → `401`; workspace
  alheio/inexistente → `404` uniforme (não vaza existência); **viewer PODE ler** (GET passa no
  `RequireWriteRole` — CA-03 da 07).
- **CA-03** *(ubíquo)* Entry **rodando** (sem `stopped_at`) fica FORA de todas as agregações de
  tempo — só entries fechadas contam.
- **CA-04** *(ubíquo, NÃO-REGRESSÃO)* Nenhum contrato existente quebra: `GET
  .../reports/week?date&tz` (07) e `GET .../reports/planned?date&tz` (06) **continuam respondendo
  o mesmo shape/semântica** (suítes das fases 06/07 seguem verdes sem edição de asserções). A
  generalização é **aditiva** (ver D-01: alias ou rota nova).

### 3.2 Período flexível (event-driven)

- **CA-05** QUANDO `GET .../reports?from=YYYY-MM-DD&to=YYYY-MM-DD&tz=IANA` chega, o sistema DEVE
  agregar a janela **`[from 00:00, to+1d 00:00)` no fuso `tz`** (intervalo INCLUSIVO nas datas:
  `from`=`to` é **um único dia**; recorte por `started_at` no fuso, padrão `ParseInLocation` da 03/07).
- **CA-06** QUANDO o cliente pede um **preset** (`period=day|week|month`) com uma data de
  referência, a janela DEVE ser resolvida no servidor: `day` = o dia; `week` = segunda→domingo que
  contém a data (idêntico ao `/reports/week` — CA-06 da 07, segunda BR/ISO); `month` = 1º→último
  dia do mês. `from`/`to` explícitos têm precedência sobre `period` (custom).
- **CA-07** SE `from`/`to`/`tz`/`period` forem malformados, OU `to < from`, o sistema DEVE
  responder `400 validation_error` (convenção do INDEX).
- **CA-08** A resposta DEVE ecoar `from`/`to` (datas resolvidas) — fonte única do rótulo de
  período no cliente (mesmo princípio do `week_start/week_end` da 07 — CA-17 da 07).
- **CA-09** `period=week` sem filtro DEVE produzir os ângulos 1–3 **byte-equivalentes** ao
  `/reports/week` da 07 pra a mesma semana (o novo endpoint é a 07 generalizada — prova de
  não-regressão semântica).

### 3.3 Filtros (state-driven)

- **CA-10** QUANDO `project_id` é passado, TODA agregação de tempo (por projeto, foco, objetivos,
  rastreado, foco/hora) DEVE considerar **apenas entries daquele projeto**; `client_id` filtra
  pelas entries cujos projetos pertencem ao cliente; `objective_id` filtra pelas entries cujas
  tasks pertencem àquele objetivo (`time_entries.task_id → tasks.objective_id` — fase 12).
- **CA-11** Os filtros são **combináveis** (projeto E cliente E objetivo) e **opcionais**
  (ausentes = sem filtro). `project_id`/`client_id`/`objective_id` malformados (não-uuid) →
  `400 validation_error`; id válido inexistente/invisível → resultado **vazio** (não 404 — não
  vaza existência de recurso alheio; o filtro respeita a visibilidade do projeto da 02).
- **CA-12** O filtro por objetivo DEVE respeitar a **visibilidade do projeto dono** do objetivo
  (objetivo de projeto privado de OUTRO usuário não filtra nada visível — herda `visibleWhere`
  da 02).

### 3.4 Ângulos 1–3 generalizados (state-driven) — herdam a 07

- **CA-13** A seção `by_project` DEVE trazer o total por projeto no PERÍODO (com
  `name/color/client_id/client_name` denormalizados, bucket `project_id:null` pro sem-projeto,
  ordenado total DESC, soma ≡ `total_seconds`) — idêntica à 07, só que no período/filtros.
- **CA-14** A seção `focus` (foco vs. fragmentação — limiar 1500s INCLUSIVO, `longest_seconds`,
  `switches` por dia local) DEVE valer no período (a 07 já calcula por dia; o período só amplia a
  janela). Projeto arquivado com tempo aparece nomeado (CA-11 da 07).
- **CA-15** A seção `goals` (meta de **horas** semanal/de período por projeto) DEVE seguir
  existindo, com `visibleWhere`, incluindo 0 realizado e meta fracionária exata (CA-15/16 da 07).
  *(O progresso por ENTREGA é a seção `objectives` nova — CA-19.)*

### 3.5 Progresso de objetivos por ENTREGA (state-driven) — o pedido central

- **CA-16** A seção `objectives` DEVE listar os objetivos (fase 12) dos projetos **visíveis** do
  workspace que tiveram **atividade no período OU têm meta** — cada um com:
  `project_id/project_name/project_color`, `objective_id/objective_name`, `status` (`open|done`),
  `tasks_total`/`tasks_done` (contagem de tasks do objetivo), `progress_pct` (= `round(tasks_done /
  tasks_total × 100)`, **inteiro 0–100**, 0 quando `tasks_total = 0`), `realized_seconds` (tempo de
  entries cujas tasks pertencem ao objetivo, **no período, do usuário**) e, se o objetivo tem
  `goal_hours`, `goal_seconds` (`round(goal_hours × 3600)`). *(Furo confirmado na verificação da
  spec: a fórmula original `tasks_done/tasks_total` omitia o ×100 — espelha o `donePct` da fase 12,
  `board.go`.)*
- **CA-16.1** (filtro restringe a LISTA de objetivos — furo confirmado) QUANDO um filtro
  `project_id`/`client_id`/`objective_id` está ativo, a lista de objetivos DEVE ser **restrita aos
  projetos/objetivo que casam o filtro**, sobrepondo o ramo "OU têm meta" do CA-16: um objetivo de
  OUTRO projeto que apenas "tem meta" **não** aparece sob `project_id=X` (nem sob `objective_id=Y`
  que não seja ele, nem sob `client_id` cujos projetos não o contenham). *(Sem isto, `goal_hours`
  ser independente de projeto fazia objetivos de projetos não-filtrados vazarem na seção.)*
- **CA-17** O **progresso primário do objetivo é por ENTREGA** (`progress_pct` = % de tasks
  `done`), não horas (decisão do Diogo): `tasks_done`/`tasks_total` são contados sobre **todas**
  as tasks do objetivo (não só as com entry no período — a entrega é cumulativa); `realized_seconds`
  é o tempo **do período** (a janela vale pro tempo, não pra contagem de entrega).
- **CA-18** Objetivo com `status='done'` DEVE vir marcado como concluído mesmo se `progress_pct <
  100%` (fechamento manual permitido — fase 12); e um objetivo com 100% de tasks `done` mas
  `status='open'` na base aparece com `progress_pct=100` e `status='open'` (o relatório **reporta**
  o estado, não conclui — a conclusão automática é responsabilidade da 12).
- **CA-19** Tasks **sem objetivo** (bucket "Geral" implícito do projeto — fase 12) **não** geram
  um item `objectives` (não há objetivo); seu tempo segue contando em `by_project`. (Sem isto, o
  "Geral" virava objetivo-fantasma.)

### 3.6 Rastreado × não-rastreado (state-driven)

- **CA-20** A resposta DEVE trazer `availability`: `tracked_seconds` (= soma das entries fechadas
  do período/filtro = `total_seconds`), `expected_seconds` (o **tempo disponível esperado** do
  período — ver CA-21), `untracked_seconds` (`max(expected − tracked, 0)`) e `tracked_pct`
  (`round(tracked / expected × 100)`, **0 quando `expected = 0`**, capado em 100).
- **CA-21** (suposição explícita do "esperado") O `expected_seconds` DEVE ser derivado de uma
  **jornada útil diária configurável** (`work_hours_per_day`, default **8h**) × **nº de dias do
  período**, com a opção de **só dias úteis** (`workdays_only`, default **true** → conta seg–sex
  do período). A suposição é registrada na resposta (`expected_basis`) e a config vem das
  preferências de relatório (CA-29). *(Sem uma fonte de "disponível", "não-rastreado" não tem
  denominador — esta é a fonte escolhida; alternativas como janela do GCal ficam fora, §7.)*

### 3.7 Foco por hora (state-driven)

- **CA-22** A resposta DEVE trazer `focus_by_hour`: para cada **hora local do dia (0–23)**, a
  soma de `focus_seconds` (só entries de **foco**, ≥1500s — limiar da 07) que caem naquela hora,
  agregada sobre o período. A entry é atribuída à(s) hora(s) **do seu `started_at` local** (versão
  v1: bucket pela hora de início — documentado; fatiar por hora real fica como refino futuro, §7).
- **CA-23** `focus_by_hour` DEVE ter sempre **24 buckets** (0–23, zeros inclusos) — o cliente
  desenha as barras sem lidar com horas ausentes.

### 3.8 KPIs (state-driven)

- **CA-24** A resposta DEVE trazer `kpis`: `tracked_seconds` (período), `deep_focus_pct` (=
  `round(focus_seconds / tracked_seconds × 100)`, 0 quando tracked=0), `avg_session_seconds`
  (`tracked_seconds / nº de entries fechadas`, 0 quando não há entries) e `untracked_seconds`
  (= `availability.untracked_seconds`). Derivam das seções já calculadas — sem query nova.

### 3.9 Comparação de períodos (event-driven)

- **CA-25** QUANDO `compare=previous` é passado, a resposta DEVE incluir um bloco `previous` com
  os **mesmos KPIs e `total_seconds`** do **período imediatamente anterior de mesma duração**
  (semana → semana anterior; mês → mês anterior com clamp do dia — padrão `addMonths`; custom →
  janela de mesmo tamanho terminando no dia antes de `from`), respeitando os mesmos filtros.
- **CA-26** SEM `compare`, o bloco `previous` é **ausente/null** (uma só janela calculada — sem
  custo). A comparação é **opt-in** (decisão: o backend devolve os dois num request quando pedido
  — D-04).

### 3.10 Frontend (state/event-driven)

- **CA-27** A página `/relatorios` DEVE ter um **seletor de período** (Dia/Semana/Mês/Custom com
  date-range) e navegação anterior/próximo coerente com o preset (Dia ±1d, Semana ±1sem, Mês
  ±1mês; Custom desliga a navegação por passo); o **rótulo do período** DERIVA do `from`/`to` da
  **resposta** (CA-08), nunca de cálculo local divergente.
- **CA-28** A página DEVE ter **filtros** (selects de projeto, cliente e objetivo, "todos" =
  sem filtro) que refazem a busca; e um **toggle Gráficos↔Tabela** (CA-31). Toda chamada vai com o
  `tz` IANA do navegador (`browserTZ` — CA-21 da 07).
- **CA-29** A página de **Ajustes** DEVE ganhar campos pra `work_hours_per_day` (1–24) e
  `workdays_only` (CA-21), persistidos por usuário (espelha o padrão `calendar_settings` da 11);
  formulário inválido NÃO chama a API e sinaliza erro.
- **CA-30** A faixa de **KPIs** (CA-24) e os cards novos (rastreado×não-rastreado donut — CA-20;
  foco por hora — CA-22) DEVEM renderizar com tokens (sem hex literal — RNF-02); quando
  `compare=previous` está ligado, cada KPI mostra o **delta vs. anterior**, definido (furo
  confirmado — sem fórmula a versão anterior dividia por zero): **`delta_pct = round((atual −
  anterior) / anterior × 100)` quando `anterior > 0`** (ex.: "+12%", com sinal); **`anterior = 0`
  e `atual > 0` → rótulo "novo"** (sem %); **ambos 0 → "0%"**. Nunca renderiza `NaN%`/`Infinity%`.
- **CA-31** O **toggle Tabela** DEVE renderizar uma **tabela densa** on-demand (`tt-table` do
  design), uma linha por projeto, **SÓ com colunas que o contrato fornece por projeto** (furo
  confirmado — o mockup `reports.jsx` FABRICAVA não-rastreado/foco%/sessões por projeto, que NÃO
  existem por projeto no contrato; `availability`/`kpis`/`focus` são agregados de **workspace**,
  não por projeto): **projeto, cliente, rastreado (`by_project.total_seconds`), meta
  (`goals.goal_seconds` do projeto, "—" sem meta), objetivos concluídos/total** (derivado da seção
  `objectives` agrupada por `project_id`). NÃO inventar colunas por-projeto fora do contrato. A
  tabela é só leitura, sem export (§7). Os KPIs agregados (foco %, sessões, não-rastreado) já vivem
  na faixa de KPIs / cards (CA-24/20), não na linha por projeto.
- **CA-32** Período/filtro **sem dados** → todas as seções exibem estado vazio amigável, **sem
  erro e sem NaN** (KPIs zerados, donut 0%, barras vazias, "nenhum objetivo").
- **CA-33** (NÃO-REGRESSÃO no front) O **ângulo 4 (Planejado × Real)** da 06 segue na página,
  reusando `usePlannedReport`/`PlannedVsReal` **sem alteração de contrato** (acompanha o período
  quando o período é uma semana; em Dia/Mês/Custom segue a regra documentada no plan — D-07).

### 3.11 E2E (Playwright — fluxo de relatórios 2.0)

- **CA-34** Logado, num projeto ÚNICO novo com 1 objetivo (2 tasks, 1 `done`) e meta, semeando
  entries fechadas via API (uma ≥25min ligada à task, uma <25min): `/relatorios` no preset
  **Semana** mostra o projeto no ângulo 1 com o total certo; **filtra por aquele projeto** e os
  números se mantêm escopados; a seção **objetivos** mostra o objetivo com **1/2 tasks (50%)** e o
  realizado; troca pro preset **Mês** e o período/rótulo mudam; **liga comparar** — como o período
  anterior do projeto novo é vazio (`anterior = 0`), o KPI mostra o rótulo **"novo"** (NÃO um
  delta numérico — CA-30; o E2E afirma "novo", determinístico e sem NaN); abre a **Tabela** e vê a
  linha do projeto (com rastreado e objetivos concluídos/total). (Escopo por projeto único =
  determinístico com banco reusado, padrão do `relatorios.spec` da 07.)

## 4. Requisitos funcionais (resumo)

- **RF-01** Generalizar `internal/reports`: `Window(period|from|to, tz)` + `Report(ws, user,
  window, filters, compare)` reusando `byProject`/`focusMetrics`/`goals` da 07.
- **RF-02** Rota nova **`GET /api/v1/workspaces/{workspaceId}/reports?period|from&to&tz&project_id
  &client_id&objective_id&compare`** no subrouter do workspace (cadeia sessão→membership; viewer
  lê). `/reports/week` mantido como **alias/compat** (D-01).
- **RF-03** Agregação por **objetivo/entrega** (lê `objectives` + `tasks` + `time_entries.task_id`
  da fase 12), `availability` (rastreado×esperado), `focus_by_hour`, `kpis`, `previous`.
- **RF-04** Preferências de relatório por usuário (`work_hours_per_day`, `workdays_only`) —
  migração nova `0014_report_settings` (espelha `calendar_settings` da 11).
- **RF-05** Página `/relatorios` 2.0 (período, filtros, KPIs, donut, foco/hora, objetivos, tabela)
  + grupo "Relatórios" nos Ajustes.

## 5. Requisitos não-funcionais

- **RNF-01 Leveza:** **zero dependência nova** (Go e JS) — donut em `conic-gradient`, barras/
  tabela em CSS puro com tokens (como o `reports.jsx`); sem lib de gráfico.
- **RNF-02 Tokens:** `tokens.css` é a única fonte de cor (gate `check-cores`); cor de projeto é
  dado de domínio (exceção já sancionada na 02/07). KPIs/donut/foco-hora só com `--accent`,
  `--surface-*`, `--text-*`.
- **RNF-03 Perf:** o relatório de um período responde em **< 200ms** com 1.000+ entries (herda o
  RNF-02 da 07; **mês com filtros + objetivos + comparação** medido — a comparação dobra a janela,
  então o teste cobre `period=month&compare=previous`).
- **RNF-04 Migração:** a fase 14 é dona de **`0014_report_settings`** (após `0013_objectives` da
  fase 12; INDEX atualizado). Nenhuma tabela de agregados (relatório é derivado — como a 07).
- **RNF-05 Não-regressão (06/07):** `/reports/week` e `/reports/planned` e o ângulo 4 dos
  relatórios permanecem **intocados em contrato** (CA-04/09/33). A fase só ADICIONA `/reports`,
  `/settings/reports` e a UI.

## 6. Pré-condições (dependências internas)

- **Fase 12 (objetivos)** DEVE estar construída: tabela `objectives` (`status`, `goal_hours`,
  `project_id`, `workspace_id`) + `tasks.objective_id`. As seções `objectives` (CA-16..19) e o
  filtro por objetivo (CA-10..12) dependem dela. (Ordem da M1: 12 → 13 → 14 — MILESTONE-1 §7.)
- Fase 02 (projects/clients, visibilidade, metas) + fase 03 (time_entries, `task_id`, tz) — base
  da 07, já em produção.
- Nada externo novo (Google stubbado nos testes — herda a 06/11). Build não bloqueia.

## 7. Fora de escopo (fronteiras explícitas)

- **Export (CSV/PDF/imagem)** — o botão "Exportar" do mockup é **ignorado** nesta fase; o Diogo
  decide depois de usar o dashboard (decisão MILESTONE-1 §7). Nenhum botão/endpoint de export.
- **IA "coach"** (resumo semanal inteligente que analisa e sugere) — **future feature**
  (MILESTONE-1 §5), não nesta fase.
- **Relatório de equipe** (tempo de outros membros) — multiplayer = M2 (fora do MVP).
- **Foco por hora fatiado por hora real** (uma entry longa distribuída pelas horas que cobre) — v1
  faz bucket pela hora de início (CA-22); refino futuro.
- **"Esperado/disponível" derivado do Google Calendar** (janela livre real) — v1 usa jornada útil
  configurável (CA-21); integrar GCal como fonte de disponível é fase futura.
- **Billable/faturamento, metas avançadas** — `REQUISITOS.md`, fora da M1.
- **Trazer planejado×real visual pra dentro da grade do calendário** — segue na 11/06, não aqui.
