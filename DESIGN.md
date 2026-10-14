# DESIGN.md — Timetrack (brief pro claude.design)

> Documento pra alimentar o **claude.design** e gerar o protótipo visual. Princípio: gosto da
> *ideia* do Toggl, **não das cores**. Minimalista, funcional, **muito personalizável**, dark-first.
> Pesquisa de base: trackers minimalistas (Everhour, Super Productivity) — hierarquia por
> **espaçamento + tipografia + cor contida**; o **timer ativo** é o herói visual, o resto recua.

---

## 1. Princípios

1. **O timer é o herói.** Em qualquer tela, o que está rodando agora salta; histórico/analytics ficam acessíveis mas visualmente contidos.
2. **Minimalismo funcional** — sem enfeite, sem lero-lero. Densidade alta no cockpit, densidade zero no Modo Foco.
3. **Personalização de verdade** — o Diogo escolhe **accent**, tema (dark/light), densidade, e cor por projeto. O app é dele.
4. **Anti-Toggl no visual:** fugir do magenta/rosa e do vermelho do Toggl. Base neutra + accent à escolha.
5. **3 superfícies, 1 linguagem:** cockpit (Mac) denso · Modo Foco (iPad) zen · mobile (controle remoto) enxuto.

## 2. Direção visual

- **Tema:** **dark por padrão** + light. Fundo dark quase-preto neutro (sem azulado forte).
- **Accent:** **configurável pelo usuário** (default sugerido: um verde-azulado calmo / teal, ou âmbar — algo que NÃO seja o magenta do Toggl). Um seletor de accent nas settings.
- **Cor só com função:** accent = ação/ativo; cor por projeto = identidade do projeto; resto em tons de cinza. Nada de arco-íris gratuito.
- **Tipografia:** sans-serif geométrica limpa pra UI (ex.: **Geist** / Inter) + uma **mono** pros números do timer (ex.: **Geist Mono** / JetBrains Mono). Self-hosted.
- **Cantos** suaves, **sombra** discreta, **bordas** de 1px em cinza baixo contraste. Espaçamento generoso cria a hierarquia.
- **Densidade** alternável (confortável × compacta) no cockpit.

## 3. Telas-chave

### 3.1 Cockpit (desktop/Mac) — onde planeja e analisa
- **Topo:** barra de timer global (descrição + projeto/task + play/stop), sempre visível.
- **Esquerda:** navegação — Hoje · Calendário · Projetos · Relatórios · Foco.
- **Centro:** alterna **Calendário** (drag-and-drop, blocos, **planejado vs. real** embaixo do dia) e **Lista/Timesheet** (estilo o `m=timesheet&ly=split` do Toggl 2 — timesheet ao lado do calendário).
- **Projeto:** chip com a cor do projeto; meta de horas com barrinha de progresso.

### 3.2 Modo Foco (iPad/PWA) — substitui o Flip Clock 🦸
- **Fullscreen, fundo preto, dígitos GIGANTES** (mono), centralizados — estilo flip clock.
- Embaixo, discreto: **nome do projeto/task** atual. Tela **sempre acordada** (Wake Lock).
- **Botões grandes:** pausar / parar. **Ao pausar → vira um Pomodoro de pausa** (contagem regressiva grande, 5/15/20 min configurável), com um respiro visual diferente do modo trabalho (ex.: accent mais suave / "modo descanso").
- Toque discreto pra **trocar de task** sem sair do modo.
- Zero menus, zero distração. É a tela que fica ligada o dia todo de canto de olho.

### 3.3 Mobile — controle remoto
- Uma tela: **o que está rodando** + play/stop + trocar projeto. Lista curta de projetos recentes.

### 3.4 Relatórios — os 4 ângulos
- Cards/abas: **Tempo por cliente/projeto** · **Foco vs. fragmentação** · **Meta vs. realizado** · **Planejado vs. real**. Gráficos simples, leitura rápida, sem poluição.

## 4. Personalização (settings)
- Seletor de **accent** (paleta + custom hex).
- **Tema** (dark/light/sistema) e **densidade**.
- **Cor por projeto.**
- **Config do Pomodoro:** duração de foco e de pausa (presets 5/15/20 + custom).
- Aparência do Modo Foco (estilo dos dígitos, mostrar/ocultar contexto).

## 5. O que levar pro claude.design (prompt-base)

> "App de cronometragem de tempo pessoal, dark-first, minimalista e muito personalizável.
> Inspiração na *estrutura* do Toggl 2.0 (calendário drag-and-drop com planejado vs. real,
> timesheet split, board), mas **sem o rosa/magenta do Toggl** — accent configurável (default
> teal/âmbar calmo), base neutra quase-preta. Tipografia geométrica + mono pros números.
> Três superfícies: **cockpit denso** (desktop), **Modo Foco fullscreen estilo flip-clock** com
> dígitos gigantes que vira Pomodoro ao pausar (iPad), e **controle remoto enxuto** (mobile).
> Exportar CSS puro + componentes, pra reaproveitar no front React/Vite/Tailwind."

## 6. Saída esperada do claude.design

CSS/tokens + componentes (timer bar, flip-clock, calendário, cards de relatório) que o front
React+Vite+Tailwind reaproveite. Guardar export em `Design/` dentro de `Timetrack/`.
