# MILESTONE 4 — Editor Markdown próprio (isolado + integrado)

> **Status:** briefing / pré-spec (24/06/2026). **Ainda NÃO é PRD.** Antes de spec
> (Regra Nº1), o Diogo + Claude rodam a **`PESQUISA.md`** desta pasta pra fechar as
> decisões de arquitetura e stack. Este arquivo fixa a VISÃO e os limites.

---

## A ideia em uma frase

Um **editor de markdown próprio**, estilo Notion (WYSIWYG + slash commands), que
**vive dentro do Timetrack** (substitui a `<textarea>` atual das Notas e da
descrição de task) **mas cujo código é ISOLADO** — um pacote autocontido,
sem dependência do resto do app, que o Diogo possa **arrancar e reusar em
qualquer outro projeto**.

Inspiração de partida: **[Novel](https://github.com/steven-tey/novel)** (Notion-style,
headless, TipTap + AI autocomplete, MIT). Não é pra clonar — é o ponto de
referência. A `PESQUISA.md` avalia Novel vs. alternativas antes de decidir.

---

## Por que fazer isso (o problema real)

Hoje a nota é uma `<textarea>` de markdown cru com um slash caseiro
(`NoteEditor.tsx`). Funciona, mas:
- escrever markdown "na mão" tem fricção (TDAH: fricção = abandono);
- o slash atual é mínimo (Task/Note/To-do/Heading) e não dá feedback visual;
- não dá pra ter blocos ricos (toggle, callout, código com highlight, embed de
  link, etc.) sem virar uma bagunça de regex.

A M3 já apostou conscientemente em **"Notion-dentro-do-Timetrack"**. O editor é a
peça que faltava pra essa aposta ficar de pé — e o Diogo quer um editor **dele**,
não um SaaS embutido, pra reusar nos outros projetos (Lauda, NOA, etc.).

---

## Princípios inegociáveis

1. **Isolamento total (a regra-mãe desta milestone).** O editor é um **pacote
   próprio** (ex.: `packages/md-editor/` no repo, ou repo separado publicável no
   npm). Regras:
   - **Zero import do resto do Timetrack** pra dentro do editor. A dependência é
     só na direção `Timetrack → editor`, nunca o contrário.
   - **Core headless + sem acoplamento a backend.** Persistência, upload,
     integrações entram por **interfaces/adapters** injetados de fora (props),
     não hardcoded.
   - **Framework-agnóstico no núcleo** se viável; a casca React é uma camada fina.
   - Teste de fogo: "dá pra usar esse editor num projeto vazio em 10 min, sem
     arrastar nada do Timetrack junto?" Se não, falhou o princípio.

2. **Markdown é cidadão de primeira classe.** O Timetrack guarda nota como
   **markdown** (`notes.body_md`, 1 linha/nota — decisão FECHADA da M3). O editor
   tem que **ler e escrever markdown sem perder dados** no round-trip. (A
   `PESQUISA.md` trata a tensão markdown-como-fonte × JSON-do-editor — é a decisão
   técnica nº1.)

3. **Integra "com a porra toda" — mas via adapter.** O editor sabe disparar
   ações ("criar task", "linkar nota", "puxar timer", "embed de link"), mas **não
   sabe o que são tasks/timer**. Ele recebe um contrato de integração
   (`onCreateTask`, `onLinkEntity`, `renderEmbed`, …) e o Timetrack pluga a
   implementação real. Outro projeto pluga outra.

4. **Perfil TDAH (a lei do produto).** Captura sem fricção, slash rápido, nada de
   modal pesado, à-prova-de-culpa. Mesmas regras do resto do app.

5. **Não reabrir decisões FECHADas da M3.** Notas nativas em Postgres (não
   Notion/Obsidian), sem vault, sem servidor caseiro. O editor é a CAMADA DE
   EDIÇÃO; o armazenamento segue como está.

---

## Escopo provável (a confirmar na spec)

**Dentro (candidato a v1):**
- Substituir o `NoteEditor` (Notas) e, possivelmente, a descrição de task pelo
  editor novo.
- Blocos essenciais: headings, listas, to-do, citação/callout, código com
  highlight, divisória, link com preview.
- Slash menu rico (o que já existe + os blocos novos) com feedback visual.
- Round-trip markdown fiel.
- Os links/menções que a M3 já tem (chip de task/sub-nota) portados pro editor.

**Fora (YAGNI / milestone futura):**
- IA autocomplete (Novel tem; pode ser fatia futura — anti-NOA por ora).
- Colaboração em tempo real no documento (Yjs/CRDT) — multiplayer de doc é grande,
  fica pra depois.
- Imagens/upload pesado, embeds de mídia ricos — avaliar na spec, provável v2.

---

## Como esta milestone se organiza (pasta)

- **`BRIEFING.md`** (este) — visão + princípios + limites.
- **`PESQUISA.md`** — o trabalho de pesquisa/decisão a fazer ANTES da spec:
  landscape de editores, matriz de avaliação, a decisão markdown×JSON, desenho do
  isolamento, e as perguntas abertas pro Diogo bater o martelo (HITL).
- Depois da pesquisa → `MILESTONE-4.md` (PRD) → specs em `specs/NN-*/` (Regra Nº1).

---

## Próximo passo imediato

Rodar a **`PESQUISA.md`** (HITL): Claude pesquisa e propõe, Diogo decide stack +
estratégia de markdown + fronteira de isolamento. **Só depois** vira PRD e specs.
Nada de codar editor antes disso (Regra Nº1).
