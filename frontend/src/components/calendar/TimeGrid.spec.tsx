// TimeGrid.spec — fase 11 T-07 (CA-09..17/24): a grade posiciona eventos,
// mostra all-day na faixa, a linha do "agora" só na coluna de hoje e faz
// auto-scroll no mount. "Hoje" é fixado com fake timers pra determinismo.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import TimeGrid from './TimeGrid';
import { weekDays } from '../../lib/calendar-layout';
import type { CalendarEvent } from '../../lib/api';

// at — ISO de parede local (round-trip tz-independente).
const at = (y: number, mo: number, d: number, h = 0, mi = 0) =>
  new Date(y, mo - 1, d, h, mi).toISOString();

// quarta 17/06/2026 14:30 local — fixa o "hoje" pros testes.
const NOW = new Date(2026, 5, 17, 14, 30);

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});
afterEach(() => vi.useRealTimers());

const timed: CalendarEvent = {
  id: 't1',
  summary: 'Reunião',
  start: at(2026, 6, 17, 10, 0),
  end: at(2026, 6, 17, 11, 0),
  all_day: false,
  html_link: '',
};
const allday: CalendarEvent = {
  id: 'a1',
  summary: 'Feriado',
  start: at(2026, 6, 18, 0, 0),
  end: at(2026, 6, 19, 0, 0),
  all_day: true,
  html_link: '',
};

describe('TimeGrid (fase 11)', () => {
  it('posiciona evento com horário na coluna do dia (CA-09)', () => {
    render(<TimeGrid days={weekDays(NOW)} events={[timed]} />);
    const block = screen.getByTestId('cal-event');
    expect(block).toHaveTextContent('Reunião');
    expect(block.style.top).toBe(`${(600 / 1440) * 100}%`);
  });

  it('all-day vai pra faixa do topo, NÃO pra grade de horas (CA-13)', () => {
    render(<TimeGrid days={weekDays(NOW)} events={[allday]} />);
    const chip = screen.getByTestId('cal-chip');
    expect(chip).toHaveTextContent('Feriado');
    // não virou um EventBlock na grade
    expect(screen.queryByTestId('cal-event')).not.toBeInTheDocument();
  });

  it('linha do "agora" aparece SÓ quando hoje está na view (CA-16)', () => {
    const { unmount } = render(<TimeGrid days={weekDays(NOW)} events={[]} />);
    expect(screen.getByTestId('now-line')).toBeInTheDocument();
    unmount();
    // semana sem hoje (uma semana antes) → sem linha
    render(<TimeGrid days={weekDays(new Date(2026, 5, 10))} events={[]} />);
    expect(screen.queryByTestId('now-line')).not.toBeInTheDocument();
  });

  it('a linha do agora fica na coluna de HOJE, na posição do horário local', () => {
    render(<TimeGrid days={weekDays(NOW)} events={[]} />);
    const line = screen.getByTestId('now-line');
    // 14:30 → 870/1440
    expect(line.style.top).toBe(`${(870 / 1440) * 100}%`);
    // dentro da coluna de hoje (quarta = índice 2 da semana seg→dom)
    const cols = screen.getAllByTestId('day-column');
    expect(within(cols[2]).getByTestId('now-line')).toBe(line);
  });

  it('auto-scroll no mount: hoje na view → rola pro horário atual (CA-17)', () => {
    render(<TimeGrid days={weekDays(NOW)} events={[]} hourStart={6} density="comfortable" />);
    const scroll = screen.getByTestId('time-grid-scroll');
    // 14:30 → (14.5 - 0.5) * 56 = 784
    expect(scroll.scrollTop).toBe(784);
  });

  it('auto-scroll sem hoje → rola pro hour_start (CA-17)', () => {
    render(<TimeGrid days={weekDays(new Date(2026, 5, 10))} events={[]} hourStart={8} density="comfortable" />);
    const scroll = screen.getByTestId('time-grid-scroll');
    // hour_start 8 → (8 - 0.5) * 56 = 420
    expect(scroll.scrollTop).toBe(420);
  });

  it('densidade compact usa --cal-row-h escopado, não o --row-h global (CA-24.1)', () => {
    render(<TimeGrid days={['2026-06-17']} events={[]} density="compact" />);
    const body = screen.getByTestId('day-column').parentElement!;
    expect(body.style.getPropertyValue('--cal-row-h')).toBe('40px');
    // nunca toca o token global
    expect(document.documentElement.style.getPropertyValue('--row-h')).toBe('');
    expect(document.documentElement.getAttribute('data-density')).toBeNull();
  });

  it('view Dia = 1 coluna; Semana = 7 (CA-20)', () => {
    const { unmount } = render(<TimeGrid days={['2026-06-17']} events={[]} />);
    expect(screen.getAllByTestId('day-column')).toHaveLength(1);
    unmount();
    render(<TimeGrid days={weekDays(NOW)} events={[]} />);
    expect(screen.getAllByTestId('day-column')).toHaveLength(7);
  });
});
