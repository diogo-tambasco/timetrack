import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { PROJECT_PALETTE } from './palette';

// lido do disco (o pipeline de CSS do Vitest devolve string vazia no ?raw;
// no jsdom o import.meta.url não é file:) — cwd da suíte é frontend/
const tokens = readFileSync(path.resolve(process.cwd(), 'src/styles/tokens.css'), 'utf8');

// Integridade da exceção do check-cores.sh: a paleta de projetos espelha
// os tokens --p-* (+ --accent default) do tokens.css. Se um token mudar
// lá e ninguém atualizar a paleta, este teste quebra.
describe('paleta de projetos', () => {
  it('espelha os tokens --p-* e o --accent default do tokens.css', () => {
    const hexDe = (token: string) => {
      const m = tokens.match(new RegExp(`${token}:\\s*(#[0-9a-fA-F]{6})`));
      expect(m, `token ${token} não encontrado no tokens.css`).not.toBeNull();
      return m![1].toLowerCase();
    };

    expect(PROJECT_PALETTE).toEqual([
      hexDe('--p-acme'),
      hexDe('--p-lauda'),
      hexDe('--p-arkan'),
      hexDe('--p-primor'),
      hexDe('--p-freela'),
      hexDe('--accent'),
    ]);
  });
});
