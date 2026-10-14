// md.test.ts — fase 26 T-08: o parser markdown mínimo (D-06) renderiza cada
// elemento e escapa <, >, & (XSS).
import { describe, it, expect } from 'vitest';
import { md } from './md';

describe('lib/md (T-08)', () => {
  it('renderiza títulos #/##/###', () => {
    expect(md('# Um')).toContain('<h2>Um</h2>');
    expect(md('## Dois')).toContain('<h3>Dois</h3>');
    expect(md('### Três')).toContain('<h4>Três</h4>');
  });

  it('renderiza negrito, itálico e code inline', () => {
    expect(md('**b**')).toContain('<b>b</b>');
    expect(md('um *it* dois')).toContain('<i>it</i>');
    expect(md('`code`')).toContain('<code>code</code>');
  });

  it('renderiza link [t](u) com target _blank e rel seguro', () => {
    const h = md('[texto](http://x.com)');
    expect(h).toContain('href="http://x.com"');
    expect(h).toContain('>texto</a>');
    expect(h).toContain('rel="noopener noreferrer"');
  });

  it('renderiza listas - como <ul><li>', () => {
    const h = md('- a\n- b');
    expect(h).toContain('<ul>');
    expect(h).toContain('<li>a</li>');
    expect(h).toContain('<li>b</li>');
  });

  it('renderiza code-block ``` como <pre>', () => {
    const h = md('```\nx = 1\n```');
    expect(h).toContain('<pre>');
    expect(h).toContain('x = 1');
  });

  it('escapa <, > e & (XSS) — uma tag injetada não vira HTML', () => {
    const h = md('<script>alert(1)</script> & <b>');
    expect(h).not.toContain('<script>');
    expect(h).toContain('&lt;script&gt;');
    expect(h).toContain('&amp;');
  });

  it('converte quebra de linha simples em <br>', () => {
    expect(md('a\nb')).toContain('<br>');
  });
});
