// spotlight.test.ts — T-01 (CA-05/06/07/08): núcleo puro, sem DOM.
import { describe, it, expect } from 'vitest';
import { SPOT, spotToken, spotItems, spotTitle, spotParse } from './spotlight';

describe('spotlight — núcleo puro', () => {
  it('SPOT tem os 4 comandos do PRD', () => {
    expect(SPOT.map((s) => s.act)).toEqual(['task', 'note', 'timer', 'goal']);
  });

  it('spotToken: 1º token sem barra, lowercase', () => {
    expect(spotToken('/ta')).toBe('ta');
    expect(spotToken('/TASK Comprar')).toBe('task');
    expect(spotToken('Reunião amanhã')).toBe('reunião');
    expect(spotToken('  ')).toBe('');
    expect(spotToken('')).toBe('');
  });

  it('spotItems: filtra por prefixo do act ou substring do label (CA-07)', () => {
    expect(spotItems('/ta').map((s) => s.act)).toEqual(['task']);
    expect(spotItems('task').map((s) => s.act)).toEqual(['task']); // sem barra
    expect(spotItems('/x')).toEqual([]); // nenhum comando (CA-07)
    expect(spotItems('').map((s) => s.act)).toEqual(['task', 'note', 'timer', 'goal']); // vazio = todos
  });

  it('spotTitle: input sem o comando (CA-08)', () => {
    expect(spotTitle('/task Comprar domínio')).toBe('Comprar domínio');
    expect(spotTitle('/note Reunião')).toBe('Reunião');
    expect(spotTitle('/timer')).toBe('');
    expect(spotTitle('Reunião amanhã')).toBe('Reunião amanhã'); // sem comando = título inteiro
  });

  it('spotParse: act do 1º token; null = sem comando (RF-19)', () => {
    expect(spotParse('/task Comprar domínio')).toEqual({ act: 'task', title: 'Comprar domínio' });
    expect(spotParse('/timer')).toEqual({ act: 'timer', title: '' });
    expect(spotParse('/goal Lançar v2')).toEqual({ act: 'goal', title: 'Lançar v2' });
    expect(spotParse('Reunião amanhã')).toEqual({ act: null, title: 'Reunião amanhã' });
    expect(spotParse('  ')).toEqual({ act: null, title: '' });
    expect(spotParse('/x sem comando')).toEqual({ act: null, title: '/x sem comando' });
  });
});
