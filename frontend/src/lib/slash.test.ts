// slash.test.ts — fase 26 T-08: a lógica PURA do slash (D-09/D-11). detectSlash
// acha /token no cursor e ignora / no meio de URL; applyBlock insere o markdown
// e devolve o caret pós-inserção.
import { describe, it, expect } from 'vitest';
import {
  detectSlash,
  filterItems,
  applySlashBlock,
  applyBlock,
  linkMarker,
} from './slash';

describe('detectSlash (T-08)', () => {
  it('acha /ta no cursor', () => {
    const v = 'um texto /ta';
    expect(detectSlash(v, v.length)).toEqual({ query: 'ta' });
  });

  it('acha / no início de uma linha', () => {
    expect(detectSlash('/', 1)).toEqual({ query: '' });
  });

  it('ignora / no meio de uma URL (sem espaço antes)', () => {
    const v = 'veja http://x';
    expect(detectSlash(v, v.length)).toBeNull();
  });

  it('null quando o cursor não está logo após o /token', () => {
    const v = '/task já confirmado';
    expect(detectSlash(v, v.length)).toBeNull(); // tem espaço depois do token
  });

  it('só considera o segmento da LINHA do cursor', () => {
    const v = 'linha 1\n/no';
    expect(detectSlash(v, v.length)).toEqual({ query: 'no' });
  });
});

describe('filterItems (T-08)', () => {
  it('sem query devolve todos os 6', () => {
    expect(filterItems('')).toHaveLength(6);
  });
  it('filtra por prefixo de cmd/label', () => {
    const r = filterItems('h');
    expect(r.map((i) => i.cmd)).toEqual(['h1', 'h2']);
  });
});

describe('applySlashBlock / applyBlock (T-08)', () => {
  it('To-do insere "- [ ] " no lugar da /', () => {
    const v = 'antes /to';
    const out = applySlashBlock(v, v.length, 'todo');
    expect(out.value).toBe('antes - [ ] ');
    expect(out.caret).toBe(out.value.length);
  });

  it('Heading 1 insere "# "', () => {
    const v = '/h1';
    expect(applySlashBlock(v, v.length, 'h1').value).toBe('# ');
  });

  it('Heading 2 insere "## "', () => {
    const v = '/h2';
    expect(applySlashBlock(v, v.length, 'h2').value).toBe('## ');
  });

  it('Divider insere "\\n---\\n"', () => {
    const v = '/div';
    expect(applySlashBlock(v, v.length, 'divider').value).toBe('\n---\n');
  });

  it('preserva o texto APÓS o cursor', () => {
    const v = 'a /t';
    const after = ' resto';
    const full = v + after;
    const out = applyBlock(full, v.length, '- [ ] X');
    expect(out.value).toBe('a - [ ] X resto');
    expect(out.caret).toBe('a - [ ] X'.length);
  });

  it('linkMarker: task → checkbox; note → wikilink', () => {
    expect(linkMarker('task', 'Publicar')).toBe('- [ ] Publicar');
    expect(linkMarker('note', 'Sub')).toBe('[[Sub]]');
  });
});
