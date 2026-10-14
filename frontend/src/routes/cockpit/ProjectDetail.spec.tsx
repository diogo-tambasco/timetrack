// ProjectDetail.spec.tsx — fase 28 T-11 (CA-19): o tab-shell ganha Overview +
// Calendar; default = Overview; trocar de projeto reseta pra Overview.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Routes, Route, Link } from 'react-router-dom';
import ProjectDetail from './ProjectDetail';
import { mkBoard } from '../../components/board/testFixtures';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    getBoard: vi.fn(),
    listNotes: vi.fn(),
    getReport: vi.fn(),
    getProjectCalendar: vi.fn(),
    getCalendarStatus: vi.fn(),
    getCalendarEvents: vi.fn(),
    listSubtasks: vi.fn(),
    getWorkspace: vi.fn(),
  };
});

// 28-REG-1: mock do TaskPanel pra expor onChanged como botão clicável — prova que
// editar a task pelo shell invalida o calendar do projeto (object permanence).
vi.mock('../../components/board/TaskPanel', () => ({
  default: ({ onChanged }: { onChanged: () => void }) => (
    <button data-testid="mock-task-changed" onClick={() => onChanged()}>
      changed
    </button>
  ),
}));

const ws = 'w1';

function renderAt(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/projetos/:id" element={<ProjectDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'WS', role: 'owner' }],
    },
  });
  vi.mocked(api.getBoard).mockImplementation(async (_ws, pid) =>
    mkBoard({ project: { ...mkBoard().project, id: pid, name: `Projeto ${pid}` } }),
  );
  vi.mocked(api.listNotes).mockResolvedValue([]);
  vi.mocked(api.getReport).mockResolvedValue({
    from: '', to: '', total_seconds: 0, by_project: [], focus: {} as never, goals: [],
    objectives: [], availability: {} as never, focus_by_hour: [], kpis: {} as never, previous: null,
  });
  vi.mocked(api.getProjectCalendar).mockResolvedValue({ items: [], from: '', to: '' });
  vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: false });
  vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
  vi.mocked(api.getWorkspace).mockResolvedValue({
    id: ws, name: 'WS', role: 'owner', created_at: '2025-01-01T00:00:00Z', members: [],
  });
});
afterEach(() => vi.clearAllMocks());

describe('ProjectDetail tab-shell (T-11, CA-19)', () => {
  it('abre na aba Overview por default (5 abas presentes)', async () => {
    renderAt('/projetos/p1');
    await screen.findByRole('tab', { name: /overview/i });
    for (const t of ['Overview', 'Tasks', 'Notes', 'Goals', 'Calendar']) {
      expect(screen.getByRole('tab', { name: new RegExp(t, 'i') })).toBeInTheDocument();
    }
    expect(screen.getByRole('tab', { name: /overview/i })).toHaveAttribute('aria-selected', 'true');
    // Overview rica: KPI "Total time"
    expect(await screen.findByText('Total time')).toBeInTheDocument();
  });

  it('trocar pra Calendar mostra a grade do mês', async () => {
    renderAt('/projetos/p1');
    await screen.findByRole('tab', { name: /calendar/i });
    fireEvent.click(screen.getByRole('tab', { name: /calendar/i }));
    expect(await screen.findByTestId('project-month-grid')).toBeInTheDocument();
  });

  it('28-REG-1: editar a task pelo shell invalida o calendar do projeto (object permanence)', async () => {
    // semeia uma task datada no calendar (no MÊS CORRENTE, pra cair na grade que o
    // ProjectCalendarTab abre por default = hoje) pra abrir o TaskPanel pela aba.
    const now = new Date();
    const y = now.getFullYear();
    const m = String(now.getMonth() + 1).padStart(2, '0');
    const today = `${y}-${m}-${String(now.getDate()).padStart(2, '0')}`;
    vi.mocked(api.getProjectCalendar).mockResolvedValue({
      items: [{ kind: 'task', id: 'tdue', name: 'Fechar OKRs', date: today, status: 'doing' }],
      from: `${y}-${m}-01`,
      to: today,
    });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalSpy = vi.spyOn(qc, 'invalidateQueries');
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/projetos/p1']}>
          <Routes>
            <Route path="/projetos/:id" element={<ProjectDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByRole('tab', { name: /calendar/i });
    fireEvent.click(screen.getByRole('tab', { name: /calendar/i }));
    // abre a task pela grade → TaskPanel (mock) montado no shell
    fireEvent.click(await screen.findByText('Fechar OKRs'));
    invalSpy.mockClear();
    // dispara onChanged (= editar/salvar a task)
    fireEvent.click(screen.getByTestId('mock-task-changed'));
    // invalida o board E o calendar do projeto (prefixo sem month — CA, object permanence).
    expect(invalSpy).toHaveBeenCalledWith({ queryKey: ['projects', ws, 'calendar', 'p1'] });
  });

  it('trocar de projeto volta pra Overview (reset via key={id} — CA-19)', async () => {
    // navegação real entre projetos dentro do MESMO router: um <Link> pra
    // /projetos/p2 prova que o key={id} remonta o ProjectDetailPage e reseta a aba.
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/projetos/p1']}>
          <Link to="/projetos/p2">ir pro p2</Link>
          <Routes>
            <Route path="/projetos/:id" element={<ProjectDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByRole('tab', { name: /overview/i });
    // vai pra Calendar em p1
    fireEvent.click(screen.getByRole('tab', { name: /calendar/i }));
    expect(await screen.findByTestId('project-month-grid')).toBeInTheDocument();

    // navega pro p2 — o key={id} remonta → aba volta pra Overview
    fireEvent.click(screen.getByRole('link', { name: /ir pro p2/i }));
    await screen.findByText('Projeto p2');
    expect(screen.getByRole('tab', { name: /overview/i })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByTestId('project-month-grid')).not.toBeInTheDocument();
  });
});
