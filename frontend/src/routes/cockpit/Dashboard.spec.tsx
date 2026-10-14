// Dashboard.spec — fase 13 T-07 (CA-04/10/15/20/21): compõe saudação + card +
// hoje + recentes + metas + atalho foco; "Nova entrada" abre o EntryModal; uma
// seção em erro (metas) NÃO derruba as outras (CA-21). Mocka a camada de dados
// (hooks de query) e stuba o EntryModal — o foco é a COMPOSIÇÃO.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ReactNode } from 'react';
import Dashboard from './Dashboard';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as entriesQ from '../../lib/queries/entries';
import * as projectsQ from '../../lib/queries/projects';
import * as reportsQ from '../../lib/queries/reports';

vi.mock('../../lib/queries/entries', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/entries')>();
  return { ...mod, useTodayEntries: vi.fn(), useRecentEntries: vi.fn(), useContinueEntry: vi.fn() };
});
vi.mock('../../lib/queries/projects', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/projects')>();
  return { ...mod, useProjects: vi.fn() };
});
vi.mock('../../lib/queries/reports', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/reports')>();
  return { ...mod, useWeekReport: vi.fn() };
});
vi.mock('../../components/timer/EntryModal', () => ({
  default: ({ onClose }: { onClose: () => void }) => (
    <div role="dialog" aria-label="Nova entrada" data-testid="entry-modal-stub">
      <button onClick={onClose}>fechar</button>
    </div>
  ),
}));

function renderDash() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return render(<Dashboard />, { wrapper });
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Diogo Tambasco', avatar_url: null },
      workspaces: [{ id: 'w1', name: 'WS', role: 'owner' }],
    },
  });
  vi.mocked(entriesQ.useTodayEntries).mockReturnValue({ data: { entries: [], total_seconds: 0 } } as never);
  vi.mocked(entriesQ.useRecentEntries).mockReturnValue({ data: { entries: [], total_seconds: 0 } } as never);
  vi.mocked(entriesQ.useContinueEntry).mockReturnValue({ mutate: vi.fn() } as never);
  vi.mocked(projectsQ.useProjects).mockReturnValue({ data: [] } as never);
  vi.mocked(reportsQ.useWeekReport).mockReturnValue({ data: { goals: [] }, isLoading: false, isError: false } as never);
});

describe('Dashboard', () => {
  it('compõe saudação com nome, card, recentes, metas e atalho pro Foco (CA-15/20)', () => {
    renderDash();
    expect(screen.getByText('Bom trabalho, Diogo.')).toBeInTheDocument();
    expect(screen.getByTestId('timer-card')).toBeInTheDocument();
    expect(screen.getByText('Recentes')).toBeInTheDocument();
    expect(screen.getByText('Metas da semana')).toBeInTheDocument();
    const foco = screen.getByRole('link', { name: /modo foco/i });
    expect(foco).toHaveAttribute('href', '/foco');
  });

  it('"Nova entrada" abre o EntryModal (CA-10)', () => {
    renderDash();
    expect(screen.queryByTestId('entry-modal-stub')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /nova entrada/i }));
    expect(screen.getByTestId('entry-modal-stub')).toBeInTheDocument();
  });

  it('uma seção em erro (metas) NÃO derruba o card nem as recentes (CA-21)', () => {
    vi.mocked(reportsQ.useWeekReport).mockReturnValue({ data: undefined, isLoading: false, isError: true } as never);
    renderDash();
    expect(screen.getByText(/não foi possível carregar as metas/i)).toBeInTheDocument();
    // resto da home segue de pé
    expect(screen.getByTestId('timer-card')).toBeInTheDocument();
    expect(screen.getByText('Recentes')).toBeInTheDocument();
  });

  // fase 22 T-04 (CA-07/13): estrutura responsiva — 1 coluna por padrão (mobile),
  // 2 colunas (1.6fr/1fr, idêntico ao desktop de hoje) a partir de md. Vitest/jsdom
  // não mede overflow/scrollWidth — só a estrutura (overflow a 375px = T-10/CA-19).
  it('grid 1-col por padrão (< md) e 2-col (1.6fr/1fr) ≥ md', () => {
    renderDash();
    const grid = screen.getByTestId('dashboard-grid');
    // default mobile-first = 1 coluna
    expect(grid).toHaveClass('grid-cols-1');
    // ≥ md reproduz o layout 2-col de hoje (1.6fr / 1fr)
    expect(grid).toHaveClass('md:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]');
    // cabeçalho + card presentes (conteúdo idêntico)
    expect(screen.getByText('Bom trabalho, Diogo.')).toBeInTheDocument();
    expect(screen.getByTestId('timer-card')).toBeInTheDocument();
  });
});
