// GoalsTab.spec — fase 29 T-10 (CA-12/13/15): a aba Goals (lista Notion) do
// /board. Renderiza GoalRow por objetivo; vazio → "No goals yet."; New goal/
// editar/excluir gated por board.project.can_write (CA-15); New goal abre o modal.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import GoalsTab from './GoalsTab';
import { mkBoard, mkObjective, mkProject } from './testFixtures';
import type { Board } from '../../lib/api';

const { delSpy, patchSpy } = vi.hoisted(() => ({ delSpy: vi.fn(), patchSpy: vi.fn() }));

vi.mock('../../lib/queries/objectives', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/objectives')>();
  return {
    ...mod,
    useDeleteObjective: () => ({ mutate: delSpy }),
    usePatchObjective: () => ({ mutate: patchSpy }),
  };
});
// isola a aba: o modal vira stub (não abre rede nem valida aqui).
vi.mock('./GoalModal', () => ({ default: () => <div data-testid="goal-modal" /> }));

function renderTab(board: Board) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <GoalsTab wsId="w1" board={board} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  cleanup();
});

describe('GoalsTab (CA-12/13/15)', () => {
  it('renderiza uma linha por objetivo do board', () => {
    renderTab(
      mkBoard({
        objectives: [
          mkObjective({ id: 'o1', name: 'Roadmap Q3', task_count: 4, done_count: 2, done_pct: 50, progress_pct: 50 }),
          mkObjective({ id: 'o2', name: 'Bater 40h', completion_mode: 'by_hours', goal_hours: 40, progress_pct: 30 }),
        ],
      }),
    );
    expect(screen.getByText('Roadmap Q3')).toBeInTheDocument();
    expect(screen.getByText('Bater 40h')).toBeInTheDocument();
    expect(screen.getAllByTestId('goal-row')).toHaveLength(2);
  });

  it('sem objetivos mostra "No goals yet."', () => {
    renderTab(mkBoard({ objectives: [] }));
    expect(screen.getByText(/No goals yet\./i)).toBeInTheDocument();
  });

  it('can_write=true: New goal presente e abre o modal', () => {
    renderTab(mkBoard({ objectives: [] }));
    const btn = screen.getByRole('button', { name: /new goal/i });
    expect(btn).toBeInTheDocument();
    expect(screen.queryByTestId('goal-modal')).not.toBeInTheDocument();
    fireEvent.click(btn);
    expect(screen.getByTestId('goal-modal')).toBeInTheDocument();
  });

  it('can_write=false: New goal escondido (CA-15)', () => {
    renderTab(mkBoard({ project: mkProject({ can_write: false }), objectives: [mkObjective({ name: 'G' })] }));
    expect(screen.queryByRole('button', { name: /new goal/i })).not.toBeInTheDocument();
    // a linha ainda aparece (leitura), mas sem ações de escrita (gating do GoalRow).
    expect(screen.getByText('G')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /editar objetivo/i })).not.toBeInTheDocument();
  });

  it('editar abre o modal (can_write)', () => {
    renderTab(mkBoard({ objectives: [mkObjective({ id: 'o1', name: 'Goal X' })] }));
    fireEvent.click(screen.getByRole('button', { name: /editar objetivo/i }));
    expect(screen.getByTestId('goal-modal')).toBeInTheDocument();
  });

  it('CA-08: concluir um goal open dispara PATCH status:done', () => {
    renderTab(mkBoard({ objectives: [mkObjective({ id: 'o1', name: 'Marco', status: 'open' })] }));
    fireEvent.click(screen.getByRole('button', { name: /concluir goal/i }));
    expect(patchSpy).toHaveBeenCalledWith({ id: 'o1', patch: { status: 'done' } });
  });

  it('CA-08: reabrir um goal done dispara PATCH status:open', () => {
    renderTab(mkBoard({ objectives: [mkObjective({ id: 'o2', name: 'Feito', status: 'done', done_pct: 100, progress_pct: 100 })] }));
    fireEvent.click(screen.getByRole('button', { name: /reabrir goal/i }));
    expect(patchSpy).toHaveBeenCalledWith({ id: 'o2', patch: { status: 'open' } });
  });
});
