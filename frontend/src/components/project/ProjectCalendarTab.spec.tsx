// ProjectCalendarTab.spec.tsx — fase 28 T-10 (CA-13/18/21): grade do mês com os
// itens do projeto na cor do projeto + overlay GCal; o overlay usa a janela da
// GRADE de 42 dias (monthRange — 28-H4), não o mês estrito. Clicar item de task
// → openTask. Degrada sem conexão GCal. kind:objective ausente não quebra.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import ProjectCalendarTab from './ProjectCalendarTab';
import type { Board, ProjectCalendar, CalendarEvent, CalendarStatus } from '../../lib/api';
import { monthRange } from '../../lib/calendar-layout';

const mockUseProjectCalendar = vi.fn();
const mockUseCalendarEvents = vi.fn();
const mockUseCalendarStatus = vi.fn();

vi.mock('../../lib/queries/projects', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/projects')>();
  return { ...mod, useProjectCalendar: (...a: unknown[]) => mockUseProjectCalendar(...a) };
});
vi.mock('../../lib/queries/calendar', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/calendar')>();
  return {
    ...mod,
    useCalendarEvents: (...a: unknown[]) => mockUseCalendarEvents(...a),
    useCalendarStatus: (...a: unknown[]) => mockUseCalendarStatus(...a),
  };
});

const board: Board = {
  project: {
    id: 'p1', name: 'Acme', color: '#4f9dde', client_name: null, weekly_hours_goal: null,
    archived_at: null, deadline: null, tracked_seconds: 0, can_write: true,
  },
  objectives: [],
  general: { task_count: 0, done_count: 0, tracked_seconds: 0, tasks: [] },
};

const REF = new Date('2025-06-15T12:00:00');

function setCalendar(cal: Partial<ProjectCalendar>) {
  mockUseProjectCalendar.mockReturnValue({
    data: { items: [], from: '2025-06-01', to: '2025-06-30', ...cal },
  });
}
function setEvents(events: CalendarEvent[]) {
  mockUseCalendarEvents.mockReturnValue({ data: events });
}
function setStatus(s: CalendarStatus) {
  mockUseCalendarStatus.mockReturnValue({ data: s });
}

function renderTab(onOpenTask = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <ProjectCalendarTab wsId="w1" board={board} onOpenTask={onOpenTask} initialRef={REF} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return onOpenTask;
}

beforeEach(() => {
  setCalendar({});
  setEvents([]);
  setStatus({ connected: true });
});
afterEach(() => vi.clearAllMocks());

describe('ProjectCalendarTab (T-10)', () => {
  it('renderiza a grade do mês (42 células)', () => {
    renderTab();
    expect(screen.getByTestId('project-month-grid')).toBeInTheDocument();
    expect(screen.getAllByTestId('cal-cell')).toHaveLength(42);
  });

  it('mostra o item de task no dia certo na cor do projeto', () => {
    setCalendar({
      items: [{ kind: 'task', id: 'tdue', name: 'Fechar OKRs', date: '2025-06-25', status: 'doing' }],
    });
    renderTab();
    const chip = screen.getByText('Fechar OKRs');
    expect(chip).toBeInTheDocument();
    // cor do projeto no border-left
    const styled = chip.closest('[data-testid="cal-item"]') as HTMLElement;
    expect(styled.style.borderLeftColor).not.toBe('');
  });

  it('clicar item de task chama openTask', () => {
    setCalendar({
      items: [{ kind: 'task', id: 'tdue', name: 'Fechar OKRs', date: '2025-06-25', status: 'doing' }],
    });
    const onOpen = renderTab();
    fireEvent.click(screen.getByText('Fechar OKRs'));
    expect(onOpen).toHaveBeenCalledWith('tdue');
  });

  it('28-H4: o overlay GCal é buscado com a janela da GRADE (monthRange), não o mês estrito', () => {
    renderTab();
    const { from, to } = monthRange(REF);
    // useCalendarEvents(from, to, enabled) — primeiros 2 args são a janela da grade
    expect(mockUseCalendarEvents).toHaveBeenCalledWith(from, to, true);
    // a grade de junho/2025 começa em 26/05 (segunda) — fora do mês estrito
    expect(from).toBe('2025-05-26');
    expect(to).toBe('2025-07-06');
  });

  it('evento do GCal sobreposto aparece', () => {
    setEvents([
      {
        id: 'ev1', summary: 'Reunião', start: '2025-06-10T14:00:00Z', end: '2025-06-10T15:00:00Z',
        all_day: false, html_link: 'https://cal/ev1',
      },
    ]);
    renderTab();
    expect(screen.getByText('Reunião')).toBeInTheDocument();
  });

  it('sem conexão GCal: só itens do projeto, sem overlay, sem erro (CA-13)', () => {
    setStatus({ connected: false });
    setEvents([]);
    setCalendar({
      items: [{ kind: 'project_deadline', id: 'p1', name: 'Acme', date: '2025-06-30', status: null }],
    });
    renderTab();
    // useCalendarEvents chamado com enabled=false
    const lastCall = mockUseCalendarEvents.mock.calls.at(-1);
    expect(lastCall?.[2]).toBe(false);
    expect(screen.getByText('Acme')).toBeInTheDocument();
  });

  it('navegar mês muda a janela do projeto E do overlay', () => {
    renderTab();
    mockUseProjectCalendar.mockClear();
    mockUseCalendarEvents.mockClear();
    fireEvent.click(screen.getByRole('button', { name: /próximo/i }));
    // novo mês: julho/2025
    expect(mockUseProjectCalendar).toHaveBeenCalledWith('w1', 'p1', '2025-07');
    const julRef = new Date('2025-07-15T12:00:00');
    const { from } = monthRange(julRef);
    expect(mockUseCalendarEvents).toHaveBeenCalledWith(from, expect.any(String), true);
  });

  it('kind:objective ausente não quebra (CA-21)', () => {
    setCalendar({ items: [] });
    expect(() => renderTab()).not.toThrow();
    expect(screen.getByTestId('project-month-grid')).toBeInTheDocument();
  });
});
