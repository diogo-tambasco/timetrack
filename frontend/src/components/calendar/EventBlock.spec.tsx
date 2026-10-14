// EventBlock.spec — fase 11 T-06 (CA-21/22): bloco accent posicionado abre o
// popover; o popover mostra "Abrir no Google" só com html_link e fecha no Esc.
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import EventBlock from './EventBlock';
import type { Positioned } from '../../lib/calendar-layout';
import type { CalendarEvent } from '../../lib/api';

const ev = (over: Partial<CalendarEvent> = {}): CalendarEvent => ({
  id: 'e1',
  summary: 'Reunião',
  start: '2026-06-15T10:00:00-03:00',
  end: '2026-06-15T11:00:00-03:00',
  all_day: false,
  html_link: 'https://calendar.google.com/e1',
  ...over,
});

const pos = (over: Partial<Positioned> = {}): Positioned => ({
  event: ev(over.event),
  dayISO: '2026-06-15',
  startMin: 600,
  endMin: 660,
  topPct: 600 / 1440,
  heightPct: 60 / 1440,
  colIndex: 0,
  colCount: 1,
  ...over,
});

describe('EventBlock (fase 11)', () => {
  it('renderiza bloco com título e posição absoluta (CA-09/22)', () => {
    render(<EventBlock p={pos({ colIndex: 1, colCount: 2 })} />);
    const block = screen.getByTestId('cal-event');
    expect(block).toHaveTextContent('Reunião');
    expect(block.style.top).toBe(`${(600 / 1440) * 100}%`);
    expect(block.style.height).toBe(`${(60 / 1440) * 100}%`);
    // colIndex 1 de 2 → left a 50%
    expect(block.style.left).toContain('50%');
    expect(block.style.background).toContain('--accent-soft');
  });

  it('clique abre o popover com "Abrir no Google" (CA-21)', () => {
    render(<EventBlock p={pos()} />);
    expect(screen.queryByTestId('event-popover')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('cal-event'));
    expect(screen.getByTestId('event-popover')).toBeInTheDocument();
    const link = screen.getByTestId('open-in-google');
    expect(link).toHaveAttribute('href', 'https://calendar.google.com/e1');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('evento sem html_link NÃO mostra o link (CA-21)', () => {
    render(<EventBlock p={pos({ event: ev({ html_link: '' }) })} />);
    fireEvent.click(screen.getByTestId('cal-event'));
    expect(screen.getByTestId('event-popover')).toBeInTheDocument();
    expect(screen.queryByTestId('open-in-google')).not.toBeInTheDocument();
  });

  it('Esc e clique fora fecham o popover (CA-21)', () => {
    render(<EventBlock p={pos()} />);
    fireEvent.click(screen.getByTestId('cal-event'));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('event-popover')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('cal-event'));
    fireEvent.click(screen.getByTestId('event-popover-backdrop'));
    expect(screen.queryByTestId('event-popover')).not.toBeInTheDocument();
  });

  it('all-day mostra "dia inteiro" no popover', () => {
    render(<EventBlock p={pos({ event: ev({ all_day: true, summary: 'Feriado' }) })} />);
    fireEvent.click(screen.getByTestId('cal-event'));
    expect(screen.getByTestId('event-popover')).toHaveTextContent('dia inteiro');
  });
});
