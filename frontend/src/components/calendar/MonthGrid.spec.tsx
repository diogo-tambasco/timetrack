// MonthGrid.spec — fase 11 T-08 (CA-18): 42 células segunda→domingo, hoje
// destacado, dias fora do mês atenuados, chips + "+N mais", clique na célula
// → view Dia, clique no chip → popover.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import MonthGrid from './MonthGrid';
import type { CalendarEvent } from '../../lib/api';

const at = (y: number, mo: number, d: number, h = 0, mi = 0) =>
  new Date(y, mo - 1, d, h, mi).toISOString();

const NOW = new Date(2026, 6, 15, 12, 0); // 15/07/2026

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});
afterEach(() => vi.useRealTimers());

const ev = (over: Partial<CalendarEvent>): CalendarEvent => ({
  id: 'e',
  summary: 'E',
  start: at(2026, 7, 15, 9, 0),
  end: at(2026, 7, 15, 10, 0),
  all_day: false,
  html_link: '',
  ...over,
});

describe('MonthGrid (fase 11)', () => {
  it('renderiza 42 células e o mês do refDate', () => {
    render(<MonthGrid refDate={NOW} events={[]} onPickDay={() => {}} />);
    expect(screen.getAllByTestId('month-cell')).toHaveLength(42);
  });

  it('hoje destacado e dias fora do mês atenuados (CA-18)', () => {
    render(<MonthGrid refDate={NOW} events={[]} onPickDay={() => {}} />);
    expect(screen.getByTestId('month-today')).toHaveTextContent('15');
    // a 1ª célula é 29/06 (segunda anterior ao 1º de julho) → atenuada
    const first = screen.getAllByTestId('month-cell')[0];
    expect(Number(first.style.opacity)).toBeLessThan(1);
  });

  it('clicar a célula vai pra view Dia (CA-18 UX)', () => {
    const onPick = vi.fn();
    render(<MonthGrid refDate={NOW} events={[]} onPickDay={onPick} />);
    fireEvent.click(screen.getAllByTestId('month-cell')[10]);
    expect(onPick).toHaveBeenCalledTimes(1);
    expect(onPick.mock.calls[0][0]).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it('chip de evento abre popover sem disparar a navegação da célula', () => {
    const onPick = vi.fn();
    render(<MonthGrid refDate={NOW} events={[ev({ summary: 'Reunião' })]} onPickDay={onPick} />);
    fireEvent.click(screen.getByTestId('cal-chip'));
    expect(screen.getByTestId('event-popover')).toBeInTheDocument();
    expect(onPick).not.toHaveBeenCalled();
  });

  it('fechar o popover pelo backdrop NÃO navega pra view Dia (CA-21, achado MEDIUM)', () => {
    const onPick = vi.fn();
    render(<MonthGrid refDate={NOW} events={[ev({ summary: 'Reunião' })]} onPickDay={onPick} />);
    fireEvent.click(screen.getByTestId('cal-chip'));
    fireEvent.click(screen.getByTestId('event-popover-backdrop'));
    expect(screen.queryByTestId('event-popover')).not.toBeInTheDocument(); // fechou
    expect(onPick).not.toHaveBeenCalled(); // mas NÃO navegou
  });

  it('"+N mais" quando os eventos estouram o limite da célula', () => {
    const many = Array.from({ length: 6 }, (_, i) => ev({ id: `m${i}`, summary: `E${i}`, start: at(2026, 7, 15, 9 + i, 0) }));
    render(<MonthGrid refDate={NOW} events={many} onPickDay={() => {}} />);
    const overflow = screen.getByTestId('month-overflow');
    expect(overflow).toHaveTextContent('+3 mais'); // 6 - 3 visíveis
  });

  it('all-day aparece na célula do dia coberto', () => {
    render(
      <MonthGrid
        refDate={NOW}
        events={[ev({ id: 'h', summary: 'Feriado', all_day: true, start: at(2026, 7, 9, 0, 0), end: at(2026, 7, 10, 0, 0) })]}
        onPickDay={() => {}}
      />,
    );
    // a célula do dia 9 contém o chip Feriado
    const cells = screen.getAllByTestId('month-cell');
    const cell9 = cells.find((c) => within(c).queryByText('9'));
    expect(cell9 && within(cell9).getByTestId('cal-chip')).toHaveTextContent('Feriado');
  });
});
