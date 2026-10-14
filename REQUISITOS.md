# REQUISITOS.md — Timetrack

> Levantado em sessão HITL (10/06/2026). Fonte de verdade do **o quê**. As specs por feature
> (EARS) derivam daqui. Inspiração: **Toggl 2.0** (`focus.toggl.com`) — mas mais interativo,
> personalizável e com IA.

---

## 1. Posicionamento

App pessoal de **cronometragem + planejamento de tempo**. Consolida 3 ferramentas que o Diogo usa
hoje em paralelo:

| Ferramenta hoje | Pra quê | Vira no Timetrack |
|---|---|---|
| **Toggl** | separar cliente/projeto, ter certeza do que fez | núcleo de projetos + cronometragem precisa |
| **Google Calendar** | agenda principal da rotina | **fonte da verdade** da agenda → sync 2-way |
| **Flip Clock (iPad)** | cronômetro grande sempre visível | **Modo Foco** fullscreen, 1ª classe |

Single-user agora; **multi-workspace/multi-user na arquitetura desde o dia 1** (vira produto
interno com colaboradores depois). Não vira produto público tão cedo → **minimalista, funcional,
sem lero-lero, bem personalizável**.

## 2. Problema central (o porquê)

1. Trackear tempo **rápido**, separando cliente/projeto.
2. Estar **sincronizado com o Google Calendar** (meu e dos colaboradores).
3. **Sync em tempo real entre dispositivos:** pausei o timer no celular/iPad/Mac → pausa em todos
   → **abre um Pomodoro configurável (5 a 15–20 min)** automaticamente.

## 3. Modelo de dados (base — multi-tenant desde já)

```
Workspace → Membros (users, com Role) → Clientes → Projetos → Tasks → Entradas de tempo (time entries)
```
- **Workspace:** unidade de isolamento. Single-user hoje; compartilhável depois.
- **Roles simples (já no banco):** Owner / Membro / Viewer. Suficiente pro futuro multiplayer.
- **Visibilidade de projeto (já no banco):**
  - **Privado** (só o dono vê) × **Compartilhado** (membros do workspace veem).
  - Em workspace compartilhado, dá pra **sempre ver o que o outro está fazendo** (timer ativo).
- **Projeto:** nome, **cor**, cliente (opcional), **meta de horas** (opcional), visibilidade.
- **Task:** nome, projeto, status, datas (opcional), recorrência (opcional), **notas**.
- **Time entry:** descrição, task/projeto, tags, início/fim, duração, fonte (timer/manual/calendar/claude).

## 4. Escopo do MVP (v1) — o que entra

### 4.1 Cronometragem (núcleo Toggl)
- Timer ao vivo (**count up**), **countdown**, entrada **manual**, continuar entrada anterior.
- **Pomodoro configurável** (duração de foco e de pausa ajustáveis).
- Cada entrada carrega projeto/task/tags.
- **Sync em tempo real entre todos os dispositivos** (Mac/iPad/celular): start/stop/pause refletem
  na hora em todos. **Ao pausar → inicia Pomodoro de pausa configurável (5/15/20 min).**

### 4.2 Projetos & planejamento
- CRUD de **clientes → projetos → tasks**. Cor por projeto. Meta de horas por projeto/semana.
- Tasks com data e recorrência opcionais, com **notas**.

### 4.3 Google Calendar (2-way)
- **GCal é a fonte da verdade.** Eventos do GCal aparecem como blocos planejados; o tempo
  cronometrado reflete de volta no GCal.
- Vale **pro Diogo e pros colaboradores** (cada um conecta seu Google).
- **Exige credencial OAuth no Google Cloud** → spec/task dedicada.

### 4.4 Modo Foco (iPad/PWA — substitui o Flip Clock)
- **Estilo flip clock:** dígitos gigantes, preto, nome do projeto, tela sempre acordada (Wake Lock).
- **Ao pausar:** abre automaticamente um **timer de Pomodoro** (que o Diogo configura, 5–20 min).
- Minimalista, zero distração.

### 4.5 Relatórios / insights (os 4 ângulos)
1. **Tempo por cliente/projeto** (Acme, Lauda, freela, Arkan, Primor).
2. **Foco vs. fragmentação** (blocos longos × pula-pula).
3. **Meta vs. realizado** (ex.: freela 10h/sem → alerta de desvio).
4. **Planejado vs. real** (o que foi pro calendário × o que aconteceu).

### 4.6 Conector Claude (dia 1)
- Skill **`/timetrack-task`** (ou `/timetrack-task new`): pergunta **qual projeto** e **nome da
  task**, cria e **já começa a cronometrar** (igual iniciar timer no Toggl).
- Tecnicamente: backend expõe **API autenticada por token** + um conector/skill que fala com ela.
- Motivo: o Diogo começa muita task de dentro do Claude — quer controlar o timer sem trocar de app.

### 4.7 Superfícies
- **Desktop/Web (MacBook) — cockpit:** projetos, calendário, relatórios, cronometragem completa.
- **iPad/PWA — Modo Foco:** o cronômetro grande + visão leve do dia.
- **Celular — controle remoto:** start/stop, trocar projeto, ver o que roda.

### 4.8 IA (leve, dia 1)
- A criação de task por linguagem natural já é coberta em boa parte pelo conector Claude.
- Resumo semanal inteligente (usa os 4 insights) — logo em seguida.

## 5. Fora do MVP (mas a base no banco já contempla)

- **Multiplayer real → MILESTONE 2 "Multiplayer + Mobile"** (planejado 19/06): workspaces
  multi-usuário funcionais (Cadu, namorada — ambientes separados) + colaboração **completa**
  (criar tasks um pro outro via `assignee`/`requested_by` + **notas em tasks** + **lembretes**)
  + **redesign mobile** (celular/tablet = gestão pequena + relatórios curtos + Modo Foco).
- **IA "coach" (future feature):** resumo semanal inteligente que analisa progresso, "toma
  esporro" e sugere ajustes (usa todos os dados: entries, objetivos, foco, planejado×real).
  Evolução do "resumo semanal inteligente" (§4).
- **Integração Git** (futuro, sem prazo).
- Metas avançadas, billable/relatórios de time · write-back do Calendar (entries→GCal).

## 6. Requisitos não-funcionais

- **Leve** (Diogo pediu explicitamente) — sem stack pesada, baixo consumo na VPS compartilhada.
- **Isolamento:** containers próprios, atrás do Caddy existente; nunca derrubar PrimaOS/`brain`/`jarvis`.
- **Tempo real confiável:** o sync entre dispositivos é requisito central, não enfeite.
- **PWA:** funciona como web app (principal) e instalável no iPad/celular; offline-tolerante no timer.
- **Personalização:** o Diogo quer customizar bastante o próprio app (ver `DESIGN.md` — accent, tema, densidade).
- **Segurança:** OAuth Google; tokens da API com escopo; segredos fora do git; HTTPS via Caddy.

## 7. Mapa de fases (provisório — vira specs/NN-*)

| Fase | Feature | Entrega |
|---|---|---|
| 00 | Esqueleto | scaffold Go + Postgres + PWA + Docker + artefatos de deploy prontos em `deploy/` (sem aplicar na VPS — deploy real = fase 09) |
| 01 | Auth Google | login OAuth + sessão + Workspace/Owner |
| 02 | Domínio core | clientes/projetos/tasks + visibilidade + roles (base multiplayer) |
| 03 | Cronômetro | count up/countdown/Pomodoro + time entries + manual |
| 04 | Sync tempo real | WebSocket: start/stop/pause em todos os devices + Pomodoro no pause |
| 05 | Modo Foco | flip-clock fullscreen iPad + Wake Lock |
| 06 | Google Calendar | sync 2-way (eventos ↔ entries) |
| 07 | Relatórios | os 4 ângulos de insight |
| 08 | Conector Claude | API token + skill `/timetrack-task` |
| 09 | Release | deploy timmingtrack.life atrás do Caddy + polish |

**Milestone 1 "Core Sólido"** (PRD em `MILESTONE-1.md`; specs em `specs/12..16`, escritas 19/06):

| Fase | Feature | Entrega |
|---|---|---|
| 12 | Tarefas + objetivos | gerenciador de tarefas (UI nova) · objetivos por projeto · progresso (% tasks + meta de horas) · tempo agregado |
| 13 | Dashboard | home única que absorve o /hoje · card do timer · recentes de vários dias · objetivos da semana |
| 14 | Relatórios 2.0 | período flexível · filtros · progresso de objetivos · comparar períodos · KPIs · tabela |
| 15 | Foco 2.0 | FocoPicker (toque) · FocoBreak · Pomodoro com ciclos/pausa longa/auto-start/som |
| 16 | Anti-timer-esquecido | corte automático em 2h sem pausa + aviso in-app + Web Push (1ª infra de background) |

> Ordem e granularidade a confirmar antes de escrever as specs. Cada fase: spec → plan → tests → code.

## 8. Decisões travadas (sessão 10/06)

- Nome: **Timetrack** (`timmingtrack.life`). (Era "Nuvi/Nuve"; domínio perdido → trocado.)
- Stack: **Go + Postgres + WebSocket** (backend) / **React+Vite+TS+Tailwind PWA** (frontend).
- **GCal = fonte da verdade** (sync 2-way).
- **Modo Foco = flip-clock; pause → Pomodoro configurável.**
- **Sync em tempo real entre dispositivos** é requisito do MVP.
- **Multi-tenant no schema desde o dia 1**; multiplayer real fora do MVP.
- Conector Claude **`/timetrack-task`** no MVP.
- Deploy isolado **atrás do Caddy existente** na VPS compartilhada.
- Pendente: criar **OAuth no Google Cloud** (spec dedicada); confirmar design no claude.design.
