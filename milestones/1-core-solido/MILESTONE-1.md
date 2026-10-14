# MILESTONE 1 — "Core Sólido" (PRD)

> Status: **RASCUNHO pra validação do Diogo** (sessão 19/06/2026). Segue a Regra Nº1:
> este PRD vira specs por fase (`specs/NN-*/`) antes de qualquer código.
> Herda `CLAUDE.md`, `REQUISITOS.md`, `DESIGN.md`. PT-BR, HITL.

---

## 1. Por que esta milestone existe

O Timetrack já é um clone de Toggl funcional em produção (`timmingtrack.life`). Mas três
coisas quebram a confiança no app **hoje, usando sozinho**:

1. **Os dados não dão pra confiar** — timer esquecido ligado polui a métrica; tasks existem no
   banco mas não têm interface; relatórios estão rasos.
2. **A virada do dia apaga o contexto** — sumiu o que eu estava fazendo ontem.
3. **Não dá pra planejar** — não tem objetivo por projeto, só meta de horas.

**M1 = tornar o app confiável e completo pra mim sozinho, ANTES de trazer gente (M2).**
Razão da ordem: não dá pra onboardar Cadu/namorada num app cujas métricas eu mesmo não confio,
e consertar a captura faz os relatórios ganharem vida sozinhos.

---

## 2. Problemas → soluções (rastreabilidade)

| # | Dor do Diogo | Solução M1 | Fase |
|---|---|---|---|
| P1 | "Projetos não mostram dados reais / quanto gastei em cada task" | Gerenciador de tarefas real + tempo agregado por task/objetivo/projeto | 12 |
| P2 | "Não consigo planejar objetivos, só vejo horas" | Camada **objetivos** (projeto → objetivo → tasks), progresso por % de tasks concluídas **+ meta de horas própria** | 12 |
| P3 | "Virou o dia, sumiram minhas tasks" | **Dashboard** (absorve o /hoje) com entradas/tasks recentes dos últimos dias | 13 |
| P4 | "Relatórios horríveis, quase sem dados, sem filtros" | **Relatórios 2.0**: período flexível, filtros, progresso de objetivos, comparar períodos, KPIs | 14 |
| P5 | "Modo Foco ruim, Pomodoro pouco personalizável" | **Foco 2.0**: seletor de task (FocoPicker), design de pausa (FocoBreak), Pomodoro mais configurável | 15 |
| P6 | "Esqueci o timer ligado → fodeu a métrica" | **Corte automático após 2h sem pausa** + aviso (in-app + web push) | 16 |

---

## 3. Escopo

### Dentro da M1
- Gerenciador de tarefas com UI completa (board/lista), objetivos por projeto, progresso.
- Dashboard novo (mantém a TimerBar global do topo).
- Relatórios 2.0 (filtros, períodos, comparação, objetivos, KPIs, tabela on-demand).
- Modo Foco 2.0 + Pomodoro mais personalizável.
- Anti-timer-esquecido (1ª infra de background do projeto + web push).

### Fora da M1 (vai pra M2 — "Multiplayer + Mobile")
- Workspaces funcionais multi-usuário (Cadu, namorada) — ambientes separados.
- Colaboração **completa**: criar tasks um pro outro (assignee/requested_by), **notas em tasks**,
  **lembretes**. (A base no banco já existe: `task.assignee`, `task.requested_by`.)
- **Redesign mobile** (celular/tablet = gestão pequena + relatórios curtos + Modo Foco).

### Future features (registro — não nesta milestone)
- **IA "coach"**: resumo semanal inteligente que analisa progresso, toma esporro e sugere ajustes
  (usa todos os dados: entries, objetivos, foco, planejado×real). Encaixa no item já previsto
  "Resumo semanal inteligente" do `REQUISITOS.md`.
- Integração Git · billable/faturamento · metas avançadas · write-back do Calendar (entries→GCal).

---

## 4. Fases da M1

> Numeração continua do roadmap atual (00–11 existem; 08 e fechamento da 09 ainda pendentes,
> tratados à parte). M1 = fases **12–16**.

### Fase 12 — Gerenciador de tarefas (objetivos → tasks) **[núcleo]**
- **Migração nova** (`0013_objectives`): tabela `objectives` por projeto (nome, descrição opcional,
  status, **meta de horas opcional**, ordem, created_by, timestamps). Task ganha FK opcional
  `objective_id`.
- Todo projeto tem um objetivo implícito/"Geral" pra tasks soltas não ficarem órfãs.
- **Progresso do objetivo = % de tasks `done` + realizado vs. meta de horas própria** (a meta de
  horas do objetivo é separada da meta de horas/semana do projeto, que continua existindo).
- Status conclui automaticamente em 100% das tasks; também dá pra fechar manualmente.
- UI: tela de projeto com objetivos e suas tasks; criar/editar/mover/concluir task; board por
  status (`todo`/`doing`/`done`) ou lista. Usa a API de tasks que já existe (sem UI hoje).
- Tempo agregado por task/objetivo/projeto exibido (lê `time_entries`).
- Entry passa a poder referenciar a task (já há `task_id` em `time_entries`).

### Fase 13 — Dashboard (home nova)
- Base no `Design/.../cockpit.jsx` **Direção C "Hero Timer"** (+ ideias da B "Mission Control").
- **Mantém a TimerBar global** do topo (decisão do Diogo). Dashboard nasce abaixo.
- Conteúdo: saudação · card do timer rodando (ou CTA pra começar) · **entradas/tasks recentes
  dos últimos N dias** (resolve P3) · objetivos/metas da semana · atalho pro Foco.
- **Vira a rota inicial e absorve o `/hoje`** (uma home só; o conteúdo do dia entra no dashboard).

### Fase 14 — Relatórios 2.0
- Base no `Design/.../reports.jsx` (já desenhado expandido).
- **Período flexível**: dia / semana / mês / intervalo custom (hoje só semana).
- **Filtros**: por projeto, cliente e **objetivo**.
- **Progresso de objetivos**: planejado × o que saiu medido em **entrega** (tasks concluídas),
  não só horas — atende o pedido central do Diogo.
- **Comparar períodos**: esta semana × anterior, este mês × anterior.
- Faixa de **KPIs** + cards novos: **rastreado vs. não-rastreado** (donut), **foco por hora**.
- Toggle **Gráficos ↔ Tabela** (tabela densa on-demand). *Export: fora por ora — o Diogo decide
  depois de ver o dashboard no app.*

### Fase 15 — Foco + Pomodoro 2.0
- **FocoPicker** (`Design/.../foco.jsx`): campo grande pra digitar + projetos recentes como
  cartões grandes + continuar entrada recente. Seleção de task no toque (iPad/celular).
- **FocoBreak**: aplicar o design de "modo descanso" já desenhado (o que o Diogo pediu pra seguir).
- **Pomodoro mais personalizável**: além de foco/pausa (já existe), avaliar ciclos
  (foco→pausa curta ×N → pausa longa), auto-start on/off, e som/alerta. (Detalhe na spec.)

### Fase 16 — Anti-timer-esquecido
- **1ª infra de background do projeto**: um ticker leve (varre a cada ~1 min timers rodando).
- **Corte automático** quando um timer roda > 2h **sem pausa**: fecha a entry no marco das 2h
  (não polui a métrica com as horas fantasma), marca como auto-cortada.
- **Aviso**: banner in-app + **web push** (o PWA já tem service worker; sem provider externo).
  Email fica como evolução futura.

---

## 5. Modelo de dados novo (proposta)

```
objectives
  id            uuid pk
  workspace_id  uuid   (denormalizado p/ filtro tenant, igual tasks)
  project_id    uuid   not null
  name          text   (1–200)
  description   text   default ''
  status        enum   ('open' | 'done')   -- auto 'done' em 100% das tasks; ou fechado manual
  goal_hours    numeric(6,2) null           -- meta de horas opcional do objetivo; faixa (0,9999] validada no service (numeric(5,2) estouraria — corrigido na spec da 12)
  position      int    (ordem manual)
  created_by    uuid
  created_at    timestamptz

tasks  (alteração)
  + objective_id uuid null  -> FK objectives (task sem objetivo = bucket "Geral" do projeto)
```

Próximo número de migração livre: **0013**.
`time_entries.task_id` já existe → agregação de tempo por task/objetivo sai de query.

---

## 6. A resolver na spec de cada fase (detalhes finos, não bloqueiam)

- Forma exata da meta de horas do objetivo no relatório (barra própria × dentro do projeto).
- Layout final do dashboard (qual mix das Direções B/C do mockup).
- Parâmetros do Pomodoro 2.0 (ciclos, pausa longa, auto-start, som) — desenhar na spec da 15.
- Janela de "recentes" do dashboard (quantos dias / quantos itens).

---

## 7. Decisões travadas (HITL 19/06)

- M1 (polish solo) **antes** de M2 (multiplayer + mobile).
- Objetivos em **hierarquia projeto → objetivo → tasks**.
- **Progresso do objetivo = % de tasks concluídas + meta de horas própria**; status conclui
  automático em 100% das tasks (ou fecho manual).
- **Dashboard novo absorve o /hoje** (uma home só) e **mantém a TimerBar** global.
- Planejado×real é sobre **objetivos/entrega**, não (só) horas.
- **Ordem das fases: 12 → 13 → 14 → 15 → 16** (objetivos primeiro).
- Colaboração da M2 = **completa** (cross-assign + notas + lembretes).
- Timer esquecido: **corte automático em 2h sem pausa** + aviso **in-app + web push**.
- **Export de relatório: fora por ora** (o Diogo decide depois de usar o dashboard).
- IA coach = **future feature**.

---

## 8. Aproveitamento do `Design/` (já pronto, nunca construído)

| Mockup | Uso na M1 |
|---|---|
| `cockpit.jsx` Direção C (Hero Timer) | Fase 13 — dashboard |
| `cockpit.jsx` Direção B (KPI strip, planejado×real ledger) | Fase 13/14 — ideias |
| `reports.jsx` (KPIs, foco/hora, donut, tabela, export) | Fase 14 — relatórios 2.0 |
| `foco.jsx` FocoPicker + FocoBreak | Fase 15 — foco 2.0 |
| `DESIGN-additions.md` (sidebar hover, tabela on-demand) | convenções já parcialmente aplicadas |
