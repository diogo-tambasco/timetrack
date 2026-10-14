# DESIGN-additions.md — Timetrack (sessão 10/06, parte 2)

> Decisões novas pra **mesclar** no `DESIGN.md` e `REQUISITOS.md` do repo.
> Geradas no claude.design junto do protótipo `Timetrack Design.html`.

## Navegação — sidebar abre no hover (CANÔNICO)
- A barra lateral é um **rail de 64px só com ícones**. Ao passar o mouse (**hover**), ela
  **expande pra 232px** mostrando os rótulos — como **overlay** (não empurra o conteúdo, não
  causa reflow). Sai do hover → recolhe.
- Vale em **todas as superfícies de cockpit** (Hoje, Calendário, Projetos, Clientes, Relatórios,
  Workspace, Foco, Ajustes). No iPad/Modo Foco não há rail (tela limpa).
- Implementação de referência: `.tt-sidenav` / `.tt-sidenav-inner` em `tokens.css`.

## Tabelas só sob seleção do usuário
- A visão padrão é sempre a **mais leve** (cartões / gráficos). A **tabela densa** aparece
  **apenas quando o usuário escolhe** no toggle *Cartões/Gráficos ↔ Tabela*.
- Aplicado em **Clientes** (Cartões ↔ Tabela) e **Relatórios** (Gráficos ↔ Tabela).
- Estilo de tabela: `.tt-table` (cabeçalho eyebrow, números em mono tabular, hover por linha).

## Workspace & papéis (multi-user — base do dia 1)
- Página **Workspace** dedicada: lista de **membros**, **presença ao vivo** (o que cada um está
  rastreando agora), **papéis** e **convites**.
- **Papéis:** `Owner` (controle total: cobrança, membros, exclusão), `Membro` (cria projetos/tasks,
  rastreia, vê o compartilhado), `Viewer` (só leitura do compartilhado).
- **Convite por email** + escolha de papel (Membro/Viewer). Cada convidado **conecta o próprio
  Google** ao entrar (destrava o Calendar dele).
- **Visibilidade padrão** configurável: "projetos novos são compartilhados" e "ver quem está
  rodando agora".

## Criação de projeto envolve membros específicos
- O modal **Novo projeto** tem: nome, cliente, cor, **visibilidade (Privado/Compartilhado)**,
  meta de horas/semana e **atribuição de membros**.
- Em projeto **compartilhado**, escolhe-se **quais membros** participam e o **acesso de cada um**
  (*Editar* × *Só vê*). Owner é fixo. Em **Privado**, a seção de membros fica desabilitada.

## Modo Foco — seletor de task no iPad (anti-Toggl)
- Tela de **entrada do Modo Foco** pensada pro toque: **campo grande pra só digitar** o que está
  fazendo + **projetos recentes** como cartões grandes (alvos ≥ 80px) + atalho **Continuar** uma
  entrada recente. Zero dropdown apertado.
- Mesmo começando a task pelo Claude (`/timetrack-task`) ou pelo Mac, dá pra **escolher/trocar
  rápido no iPad**.

## Relatórios — ângulos ampliados
- Além de cliente/projeto, foco-vs-fragmentação, meta-vs-realizado e planejado-vs-real:
  - **Foco por hora** (intensidade de deep-work ao longo do dia).
  - **Rastreado vs. não rastreado** (donut + proporção do dia) — deixa explícito o tempo fora do timer.
  - **KPIs** no topo: rastreado na semana, foco profundo %, sessão média, não rastreado hoje.

## Integração Google Calendar — visualização
- Tela dedicada: **contas conectadas** (cada membro conecta seu Google), **status de sync**,
  controles de **2-way** (GCal→Timetrack vira planejado · Timetrack→GCal devolve o rastreado ·
  ignorar eventos pessoais) e **mapeamento evento ↔ entrada** com estado (sincronizado / enviando /
  ignorado).
