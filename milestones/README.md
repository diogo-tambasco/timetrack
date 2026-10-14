# milestones/ — uma pasta por milestone

> Convenção (decisão do Diogo, 24/06/2026): **cada milestone é uma PASTA**, não um
> arquivo solto. Dentro vivem o PRD e todos os documentos daquela milestone
> (briefing, pesquisa, decisões, etc.). Antes, cada milestone era um único
> `MILESTONE-N.md` na raiz — foram movidos pra cá.

## Índice

| # | Pasta | Tema | Status |
|---|-------|------|--------|
| 1 | [`1-core-solido/`](1-core-solido/) | Core Sólido (tarefas/objetivos · dashboard · relatórios 2.0 · foco 2.0 · anti-timer-esquecido) | ✅ no ar |
| 2 | [`2-multiplayer-mobile/`](2-multiplayer-mobile/) | Multiplayer + Mobile (membros/convites · cross-assign · comentários · lembretes · presence · responsivo · controle remoto) | ✅ no ar |
| 3 | [`3-nucleo-organizacao/`](3-nucleo-organizacao/) | Núcleo de Organização (Notion-dentro-do-Timetrack: props tipadas · sub-tasks · notas · spotlight · projeto vivo · goals · elo timer↔task) | ✅ no ar |
| 4 | [`4-editor-markdown/`](4-editor-markdown/) | Editor Markdown próprio, isolado e reaproveitável, que integra com tudo | 🟡 briefing/pesquisa |

## Convenção da pasta de cada milestone

- `MILESTONE-N.md` — o **PRD** (fonte da verdade do escopo). Para a M4, ainda em
  formação: começa por `BRIEFING.md` + `PESQUISA.md`.
- Documentos auxiliares conforme a milestone pede (briefing, pesquisa, decisões,
  mockups específicos…).
- As **specs por fase** continuam em `specs/NN-feature/` (Regra Nº1) — a pasta da
  milestone aponta pra elas; não duplica.

> ⚠️ Specs antigos (`specs/NN-*/`) e handoffs datados no `CLAUDE.md` podem citar o
> caminho antigo `MILESTONE-N.md` na raiz — são registros congelados; o arquivo é
> o mesmo, agora em `milestones/<n>-slug/MILESTONE-N.md`.
