// format.spec — T-03 da fase 05: fmtDayTotal (rodapé do Modo Foco, CA-10).
// Horas sem zero à esquerda e sem teto de 24 (dia com >24h cronometradas é
// possível com entries manuais); minutos com 2 dígitos; segundos truncados.
import { describe, it, expect } from 'vitest';
import { fmtDayTotal } from './format';

describe('fmtDayTotal', () => {
  it('zero vira "0h00"', () => {
    expect(fmtDayTotal(0)).toBe('0h00');
  });

  it('5h12 sem zero à esquerda nas horas', () => {
    expect(fmtDayTotal(5 * 3600 + 12 * 60)).toBe('5h12');
  });

  it('horas sem teto de 24: "26h05"', () => {
    expect(fmtDayTotal(26 * 3600 + 5 * 60)).toBe('26h05');
  });

  it('segundos soltos são truncados (5h12m30s → "5h12")', () => {
    expect(fmtDayTotal(5 * 3600 + 12 * 60 + 30)).toBe('5h12');
  });

  it('negativo clampa em "0h00" (defesa)', () => {
    expect(fmtDayTotal(-7)).toBe('0h00');
  });
});
