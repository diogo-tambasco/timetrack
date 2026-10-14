// lib/slash.ts — lógica PURA do menu slash (fase 26, D-09/D-11). Sem DOM:
// detecta `/token` no segmento da linha antes do cursor e aplica o bloco
// (markdown OU marcador de link) no lugar da `/…`. Testável por unidade.
// Espelha onNoteInput/applySlash do mockup (linhas 600-622).

export type SlashCmd = 'task' | 'note' | 'todo' | 'h1' | 'h2' | 'divider';

export type SlashItem = {
  cmd: SlashCmd;
  label: string;
  desc: string;
  // task/note chamam a API de link (D-11); o resto só insere texto (CA-20).
  kind: 'link-task' | 'link-note' | 'block';
};

export const SLASH_ITEMS: SlashItem[] = [
  { cmd: 'task', label: 'Task', desc: 'Cria uma task ligada a esta nota', kind: 'link-task' },
  { cmd: 'note', label: 'Note', desc: 'Cria uma sub-nota', kind: 'link-note' },
  { cmd: 'todo', label: 'To-do', desc: 'Item de checklist', kind: 'block' },
  { cmd: 'h1', label: 'Heading 1', desc: 'Título de seção', kind: 'block' },
  { cmd: 'h2', label: 'Heading 2', desc: 'Subtítulo', kind: 'block' },
  { cmd: 'divider', label: 'Divider', desc: 'Linha separadora', kind: 'block' },
];

// detectSlash devolve { query } quando há `/token` no segmento da linha ANTES do
// cursor (token = palavra, sem espaço), senão null. Ignora `/` no meio de uma
// URL ou de uma palavra (precisa de `/` seguido de \w* no fim do segmento).
export function detectSlash(value: string, caret: number): { query: string } | null {
  const ls = value.lastIndexOf('\n', caret - 1) + 1;
  const seg = value.slice(ls, caret);
  const m = seg.match(/\/(\w*)$/);
  if (!m) return null;
  // `/` precedido de não-espaço (ex.: "http://x") não é um comando — exige
  // início do segmento ou um espaço antes da barra.
  const slashIdx = seg.length - m[0].length;
  if (slashIdx > 0) {
    const prev = seg[slashIdx - 1];
    if (prev !== ' ' && prev !== '\t') return null;
  }
  return { query: m[1].toLowerCase() };
}

// filterItems devolve os itens cujo cmd/label casa o prefixo digitado.
export function filterItems(query: string): SlashItem[] {
  if (!query) return SLASH_ITEMS;
  const q = query.toLowerCase();
  return SLASH_ITEMS.filter(
    (s) => s.cmd.startsWith(q) || s.label.toLowerCase().startsWith(q),
  );
}

// blockText devolve o markdown a inserir pros comandos de BLOCO (CA-20).
function blockText(cmd: SlashCmd): string {
  switch (cmd) {
    case 'h1':
      return '# ';
    case 'h2':
      return '## ';
    case 'todo':
      return '- [ ] ';
    case 'divider':
      return '\n---\n';
    default:
      return '';
  }
}

// applyBlock substitui o `/…` (do início da barra até o cursor) pelo texto dado
// e devolve { value, caret } pós-inserção. `insert` é o markdown/marcador final
// (block via blockText, OU o marcador de link montado pelo caller — CA-21).
export function applyBlock(
  value: string,
  caret: number,
  insert: string,
): { value: string; caret: number } {
  const ls = value.lastIndexOf('\n', caret - 1) + 1;
  const seg = value.slice(ls, caret);
  const si = seg.lastIndexOf('/');
  const cut = si >= 0 ? ls + si : caret;
  const before = value.slice(0, cut);
  const after = value.slice(caret);
  return { value: before + insert + after, caret: (before + insert).length };
}

// applySlashBlock é o atalho pros comandos de BLOCO (CA-20): monta o markdown e
// chama applyBlock. Task/Note NÃO passam por aqui (o caller monta o marcador de
// link após a API — CA-21).
export function applySlashBlock(
  value: string,
  caret: number,
  cmd: SlashCmd,
): { value: string; caret: number } {
  return applyBlock(value, caret, blockText(cmd));
}

// linkMarker monta o marcador de texto pro chip de link (CA-21): task → checkbox
// com o nome; note → wikilink com o título.
export function linkMarker(kind: 'task' | 'note', name: string): string {
  return kind === 'task' ? `- [ ] ${name}` : `[[${name}]]`;
}
