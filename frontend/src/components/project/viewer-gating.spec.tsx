// viewer-gating.spec.tsx — fase 28 T-12 (CA-20/22): com board.project.can_write=
// false, Overview/Calendar não exibem ações de ESCRITA (criar task pelo Upcoming,
// editar deadline). Com can_write=true, exibem. O E2E não loga como viewer
// (limitação herdada) — este teste de componente é a cobertura do CA-20. Controle
// member×viewer num arquivo (padrão das fases 02/12).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import OverviewTab from './OverviewTab';
import ProjectCalendarTab from './ProjectCalendarTab';
import type { Board } from '../../lib/api';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listNotes: vi.fn().mockResolvedValue([]),
    getReport: vi.fn(),
    getProjectCalendar: vi.fn(),
    getCalendarStatus: vi.fn(),
    getCalendarEvents: vi.fn(),
  };
});

function mkBoard(canWrite: boolean): Board {
  return {
    project: {
      id: 'p1', name: 'Acme', color: '#4f9dde', client_name: null, weekly_hours_goal: null,
      archived_at: null, deadline: '2025-09-30', tracked_seconds: 0, can_write: canWrite,
    },
    objectives: [],
    general: { task_count: 0, done_count: 0, tracked_seconds: 0, tasks: [] },
  };
}

function renderOverview(canWrite: boolean) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <OverviewTab wsId="w1" board={mkBoard(canWrite)} onOpenTask={vi.fn()} now={new Date('2025-06-25')} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function renderCalendar(canWrite: boolean) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <ProjectCalendarTab wsId="w1" board={mkBoard(canWrite)} onOpenTask={vi.fn()} initialRef={new Date('2025-06-15')} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const escrita = [/new task/i, /nova task/i, /criar/i, /editar deadline/i, /editar prazo/i];

beforeEach(() => {
  vi.mocked(api.getReport).mockResolvedValue({
    from: '', to: '', total_seconds: 0, by_project: [], focus: {} as never, goals: [],
    objectives: [], availability: {} as never, focus_by_hour: [], kpis: {} as never, previous: null,
  });
  vi.mocked(api.getProjectCalendar).mockResolvedValue({ items: [], from: '', to: '' });
  vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: false });
  vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
});
afterEach(() => vi.clearAllMocks());

describe('gating viewer no Overview/Calendar (T-12, CA-20)', () => {
  it('viewer: Overview sem botão de escrita', async () => {
    renderOverview(false);
    await screen.findByText('Total time');
    for (const n of escrita) {
      expect(screen.queryAllByRole('button', { name: n })).toHaveLength(0);
    }
  });

  it('viewer: Calendar sem botão de escrita (só navegação read-only)', async () => {
    renderCalendar(false);
    await screen.findByTestId('project-month-grid');
    for (const n of escrita) {
      expect(screen.queryAllByRole('button', { name: n })).toHaveLength(0);
    }
  });

  it('member (can_write=true): Overview/Calendar renderizam sem erro', async () => {
    renderOverview(true);
    expect(await screen.findByText('Total time')).toBeInTheDocument();
    renderCalendar(true);
    expect(await screen.findByTestId('project-month-grid')).toBeInTheDocument();
  });
});
