# M4 — Pesquisa & Decisões (rodar ANTES da spec)

> Objetivo deste doc: dar ao Diogo um mapa pra **decidir a stack e a arquitetura**
> do editor markdown próprio, antes de virar PRD/spec. É HITL: Claude pesquisa e
> propõe, **Diogo bate o martelo** nas decisões marcadas com 🔨. Tudo aqui é
> ponto de partida — confirmar/atualizar ao executar.

---

## 0. As 3 perguntas que decidem tudo

1. **🔨 Stack base:** construir sobre qual motor? (TipTap/ProseMirror vs BlockNote
   vs Lexical vs Milkdown vs do-zero). → §2 + §3
2. **🔨 Fonte da verdade do conteúdo:** markdown (como hoje) ou JSON do editor? É
   a decisão técnica nº1 e condiciona o banco. → §4
3. **🔨 Fronteira de isolamento:** pacote no monorepo (`packages/`) ou repo
   separado publicável? E qual o contrato de integração? → §5

Sem essas 3 fechadas, não escrever spec.

---

## 1. Critérios de avaliação (peso pro perfil do Diogo)

| Critério | Por quê importa aqui |
|----------|----------------------|
| **Markdown round-trip fiel** | a nota É markdown no Postgres (decisão M3); perder dado no ida-e-volta é inaceitável |
| **Isolável / headless** | a regra-mãe da M4: arrancar e reusar em qualquer projeto |
| **Slash commands + blocos ricos** | o que motiva sair da `<textarea>` |
| **Extensibilidade p/ integrações** | criar task/linkar nota/embed via adapter |
| **Curva + manutenção** | é projeto pessoal/hobby; não pode virar buraco de tinkering (anti-NOA) |
| **Licença / custo** | precisa ser MIT-like, sem licença paga (TipTap tem extensões pagas — ver §2) |
| **Tamanho do bundle** | PWA; peso importa |
| **Maturidade / comunidade** | sustentar sozinho a longo prazo |

---

## 2. Landscape (estado em jun/2026 — re-verificar ao executar)

- **[Novel](https://github.com/steven-tey/novel)** (Steven Tey) — *referência do
  Diogo*. Notion-style headless **sobre TipTap**, com slash menu, bubble toolbar,
  image resizer e **AI autocomplete** (OpenAI + Vercel AI SDK). MIT, último release
  v1.0.2 (fev/2025). **Não é uma lib de base** — é um *kit montado* sobre TipTap.
  Bom como **mapa de UX e de extensões**, não necessariamente como dependência.
- **[TipTap](https://tiptap.dev)** — o motor headless mais popular, **sobre
  ProseMirror**. Equilíbrio entre poder e não-ser-opinativo. ⚠️ Tem extensões/
  serviços **pagos** (Pro, Collab Cloud) — o core é MIT, mas checar quais blocos
  que a gente quer são pagos.
- **[BlockNote](https://github.com/TypeCellOS/BlockNote)** — editor **bloco-a-bloco
  Notion-style** sobre ProseMirror+TipTap, **com UI pronta** (menus/toolbars
  customizáveis) e colab via Yjs. Menos código que TipTap puro; mais opinativo.
  Ótimo "atalho pro Notion-like", mas o modelo é JSON de blocos (ver §4).
- **[Lexical](https://lexical.dev)** (Meta) — framework rápido/acessível, mas
  "você constrói tudo" (toolbar, plugins, serialização). Mais trabalho; máximo
  controle.
- **[Milkdown](https://milkdown.dev)** — WYSIWYG **markdown-first** plugin-driven
  (sobre ProseMirror + remark). Interessante PORQUE markdown é cidadão de primeira
  classe → casa com a decisão §4 se a fonte for markdown.
- **[Plate](https://platejs.org)** — sobre Slate, amigável a shadcn/Tailwind.
  Alternativa se o ecossistema Slate agradar.

**Leitura rápida:** se a prioridade é **markdown como fonte**, Milkdown e
TipTap+extensão markdown brigam na frente. Se a prioridade é **Notion-like com
menos esforço** e topar JSON, BlockNote. Novel = referência de UX (e de "como
fizeram"), provavelmente não dependência direta.

---

## 3. Caminhos arquiteturais (3 opções pra decidir)

- **Opção A — sobre TipTap (estilo Novel, mas nosso).** Mais controle, monta as
  extensões que quisermos. Precisa resolver markdown↔doc (TipTap guarda
  ProseMirror JSON). Custo: nós cuidamos da serialização.
- **Opção B — BlockNote.** Mais rápido pro Notion-like; UI pronta. Custo: modelo
  de blocos (JSON) e menos controle fino; markdown é export/import, não fonte.
- **Opção C — Milkdown (markdown-first).** Casa com o banco atual sem migração de
  storage. Custo: ecossistema menor, mais montagem manual de UX.

> Recomendação inicial do Claude (a debater): **A (TipTap) OU C (Milkdown)**,
> decidido pela §4. Se a fonte continuar markdown → Milkdown pesa mais; se
> aceitarmos JSON com export markdown → TipTap. BlockNote fica como plano B
> "rápido" se o tempo apertar.

---

## 4. 🔨 Decisão nº1 — markdown vs JSON como fonte da verdade

Hoje: `notes.body_md` guarda **markdown** (1 linha/nota). A maioria dos editores
ricos (TipTap/ProseMirror/BlockNote/Lexical) opera em **JSON estruturado** e trata
markdown como serialização — e blocos ricos (toggle, callout, colunas) **não têm
markdown canônico**, então o round-trip pode ser lossy.

Três saídas:
- **(4a) Markdown continua a fonte.** Editor serializa de/para markdown a cada
  save. Limita blocos aos que têm markdown fiel (ou estende com sintaxe própria,
  ex.: `:::callout`). Banco intacto. ✅ menos migração, ✅ portável (markdown é
  universal), ⚠️ blocos ricos limitados.
- **(4b) JSON do editor vira a fonte.** Nova coluna `body_json` (mantendo
  `body_md` como espelho/derivado pra busca/preview). Libera todo bloco rico. ⚠️
  migração + acoplamento ao formato do editor (menos portável — fere o espírito
  "reaproveitável"?).
- **(4c) Híbrido.** Fonte = markdown estendido (markdown + blocos custom via
  diretivas remark). Portável E rico. ⚠️ mais complexo de implementar.

> A regra "isolável/reaproveitável" e "markdown cidadão de primeira classe" puxam
> pra **4a ou 4c**. Decidir com o Diogo qual ambição de blocos ricos vale a
> complexidade.

---

## 5. 🔨 Decisão nº2 — isolamento + contrato de integração

**Forma do pacote (escolher):**
- `packages/md-editor/` num monorepo (pnpm/npm workspaces) dentro do repo do
  Timetrack — fácil de evoluir junto, ainda isolável.
- **OU** repo separado `md-editor` publicado no npm (ou git submodule) — isolamento
  máximo, custo de versionar/publicar.

**Contrato de isolamento (não-negociável):**
- Núcleo **headless**, sem nenhum import de `timetrack/*`.
- Persistência por prop: `value`, `onChange(markdown)` (ou `onChange(doc)`),
  `onBlur`/debounce decididos pelo HOST.
- Integrações por **adapter injetado**, ex.:
  ```ts
  <MdEditor
    value={md}
    onChange={setMd}
    integrations={{
      onCreateTask?(title): Promise<{id, href}>,
      onLinkEntity?(kind, query): Promise<{id, label, href}>,
      renderEmbed?(url): ReactNode,        // preview de link (ex.: Notion)
      slashExtra?: SlashItem[],            // comandos do host
      onMention?(query): Promise<Item[]>,
    }}
  />
  ```
  O Timetrack pluga task/timer/nota; outro projeto pluga o que quiser. O editor
  NÃO sabe o que é uma task.
- **Reaproveita o que a M3 já tem:** o `lib/slash` e os chips de link (task/nota)
  viram a primeira implementação do adapter — mas movidos pra trás da interface.

---

## 6. Integrações-alvo ("integra com a porra toda") — listar e priorizar

Candidatas (priorizar v1 vs futuro com o Diogo):
- **Linkar/criar task** a partir do texto (já existe no slash atual).
- **Sub-nota / menção a nota** (já existe).
- **Embed/preview de link** (Notion, YouTube, etc. — a M3 já tinha preview de
  link do Notion).
- **Puxar timer** a partir de um item (elo timer↔task da fase 30).
- **Menção a projeto/objetivo.**
- **Slash → comandos do host** (criar goal, agendar, etc.).
- (Futuro) **IA**: autocomplete/resumo (estilo Novel) — provavelmente fora da v1.

---

## 7. Checklist de pesquisa pro Diogo (próxima sessão HITL)

- [ ] Abrir o **Novel** rodando (novel.sh / repo) e listar quais extensões/UX a
      gente quer copiar e quais sobram.
- [ ] Fazer um **spike de 1–2h** com TipTap e com Milkdown (e talvez BlockNote)
      carregando uma nota markdown REAL do mirror e testando round-trip → medir a
      perda de dado de cada um.
- [ ] Bater a **§4** (markdown vs JSON) — é o que destrava a stack.
- [ ] Bater a **§5** (monorepo vs repo separado + assinatura do adapter).
- [ ] Confirmar licenças (TipTap pago? BlockNote? quais blocos).
- [ ] Listar e priorizar as integrações da §6 (v1 vs futuro).
- [ ] Só então: escrever `MILESTONE-4.md` (PRD) → specs `specs/NN-*/` (Regra Nº1).

**Buscas úteis pra começar:** "tiptap markdown serialization lossless",
"milkdown vs tiptap markdown source of truth", "blocknote markdown export
fidelity", "prosemirror custom block markdown directive remark",
"novel editor extensions list".

---

## Fontes (jun/2026)

- Novel — https://github.com/steven-tey/novel · https://novel.sh
- TipTap — https://tiptap.dev/product/editor
- BlockNote — https://github.com/TypeCellOS/BlockNote
- Lexical — https://lexical.dev
- Milkdown — https://milkdown.dev
- Comparativos — Liveblocks "which rich text editor framework should you choose",
  Velt "best rich text editors 2026".
