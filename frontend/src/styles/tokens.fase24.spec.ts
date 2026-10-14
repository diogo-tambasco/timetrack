// tokens.fase24.spec.ts — fase 24 T-09 (CA-27/14/RNF-03): integridade dos tokens
// da fase 24 por leitura do arquivo (padrão das fases anteriores). --font-ui →
// Inter; --font-mono intocado (Geist Mono); os dois tokens do mark "N" do Notion
// declarados em dark E light (sem hex literal fora do tokens.css).
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

// no jsdom import.meta.url não é file: — cwd da suíte é frontend/ (espelha palette.spec)
const tokens = readFileSync(path.resolve(process.cwd(), 'src/styles/tokens.css'), 'utf8');
const fonts = readFileSync(path.resolve(process.cwd(), 'src/styles/fonts.css'), 'utf8');

describe('fase 24 — fonte Inter (CA-27)', () => {
  it('--font-ui aponta pra Inter', () => {
    expect(tokens).toMatch(/--font-ui:\s*'Inter'/);
  });

  it('--font-mono permanece Geist Mono (intocado)', () => {
    expect(tokens).toMatch(/--font-mono:\s*'Geist Mono'/);
  });

  it('fonts.css declara o @font-face de Inter self-hosted (sem CDN)', () => {
    expect(fonts).toMatch(/font-family:\s*'Inter'/);
    expect(fonts).toMatch(/InterVariable\.woff2/);
    expect(fonts).not.toMatch(/fonts\.(googleapis|gstatic)\.com/);
  });

  it('mantém o @font-face de Geist Mono (mono self-hosted)', () => {
    expect(fonts).toMatch(/font-family:\s*'Geist Mono'/);
  });
});

describe('fase 24 — tokens do mark Notion (RNF-03/CA-14)', () => {
  it('declara --notion-mark-bg e --notion-mark-ink', () => {
    expect(tokens).toContain('--notion-mark-bg');
    expect(tokens).toContain('--notion-mark-ink');
  });

  it('declara os dois tokens no bloco light também (2 ocorrências cada)', () => {
    expect(tokens.match(/--notion-mark-bg/g)?.length).toBeGreaterThanOrEqual(2);
    expect(tokens.match(/--notion-mark-ink/g)?.length).toBeGreaterThanOrEqual(2);
  });
});
