// notion.test.ts — fase 24 T-08 (CA-24): notionTitle deriva o título do slug.
import { describe, expect, it } from 'vitest';
import { notionTitle } from './notion';

describe('notionTitle', () => {
  it('remove o hash hex e troca - por espaço', () => {
    expect(notionTitle('https://www.notion.so/OKRs-Q3-2025-8f3a1c9b')).toBe('OKRs Q3 2025');
  });

  it('aceita hash hex MAIÚSCULO (regex case-insensitive — CA-24)', () => {
    // mutação pro lowercase-only (-[0-9a-f]{6,}) deixaria "Projeto 8F3A1C9B".
    expect(notionTitle('https://www.notion.so/Projeto-8F3A1C9B')).toBe('Projeto');
  });

  it('sem hash → slug inteiro', () => {
    expect(notionTitle('https://www.notion.so/Minha-Pagina')).toBe('Minha Pagina');
  });

  it('URL inválida/vazia → fallback', () => {
    expect(notionTitle('')).toBe('Página do Notion');
  });

  it('ignora query e hash do fragmento', () => {
    expect(notionTitle('https://www.notion.so/Doc-abc123?v=xyz#sec')).toBe('Doc');
  });
});
