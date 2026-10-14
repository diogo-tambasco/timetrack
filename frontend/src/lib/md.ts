// lib/md.ts — parser markdown MÍNIMO (fase 26, D-06; espelha o md() do mockup,
// linhas 211-220). Sem dependência nova (RNF-01/anti-NOA): cobre títulos
// (#/##/###), negrito/itálico/code inline, links [t](u), listas, code-block,
// quebra de linha. Escapa <, >, & ANTES de inserir as tags (XSS). NÃO é editor
// bloco-a-bloco; o corpo é um textarea de markdown puro.
//
// Devolve uma string de HTML (renderizada via dangerouslySetInnerHTML no
// NoteEditor) — o escape garante que o texto do usuário nunca injeta tags.

function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function md(src: string): string {
  let h = escapeHTML(src || '');
  // code-block ``` ``` (antes do resto pra não processar markdown dentro).
  h = h.replace(/```([\s\S]*?)```/g, (_m, c) => `<pre>${String(c).trim()}</pre>`);
  // títulos.
  h = h
    .replace(/^### (.*)$/gm, '<h4>$1</h4>')
    .replace(/^## (.*)$/gm, '<h3>$1</h3>')
    .replace(/^# (.*)$/gm, '<h2>$1</h2>');
  // ênfase + code inline.
  h = h
    .replace(/\*\*(.+?)\*\*/g, '<b>$1</b>')
    .replace(/(^|[^*])\*(?!\*)(.+?)\*/g, '$1<i>$2</i>')
    .replace(/`(.+?)`/g, '<code>$1</code>');
  // links [texto](url) — target _blank + rel seguro.
  h = h.replace(
    /\[(.+?)\]\((.+?)\)/g,
    '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>',
  );
  // listas `- `.
  h = h.replace(/(?:^- .*(?:\n|$))+/gm, (b) =>
    '<ul>' +
    b
      .trim()
      .split('\n')
      .map((l) => `<li>${l.replace(/^- /, '')}</li>`)
      .join('') +
    '</ul>',
  );
  // quebras de linha.
  h = h.replace(/\n{2,}/g, '<br><br>').replace(/(?<!>)\n(?!<)/g, '<br>');
  return h;
}
