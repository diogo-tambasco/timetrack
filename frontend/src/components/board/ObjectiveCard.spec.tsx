import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import ObjectiveCard from './ObjectiveCard';
import type { BoardObjective } from '../../lib/api';

const baseObjective: BoardObjective = {
  id: 'o1',
  name: 'Lançar onboarding',
  description: '',
  status: 'open',
  goal_hours: null,
  position: 0,
  task_count: 2,
  done_count: 1,
  done_pct: 50,
  tracked_seconds: 3600,
  completion_mode: 'by_tasks',
  target_date: null,
  progress_pct: 50,
  tasks: [],
};

function renderCard(objective: BoardObjective, canWrite: boolean) {
  return render(
    <ObjectiveCard
      objective={objective}
      color="#4f9dde"
      canWrite={canWrite}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
      onNewTask={vi.fn()}
    />,
  );
}

describe('ObjectiveCard (CA-19/22)', () => {
  it('mostra nome e progresso', () => {
    renderCard(baseObjective, true);
    expect(screen.getByText('Lançar onboarding')).toBeInTheDocument();
    expect(screen.getByText('1/2 tasks')).toBeInTheDocument();
  });

  it('canWrite=true: mostra ações de escrita', () => {
    renderCard(baseObjective, true);
    expect(screen.getByRole('button', { name: /editar objetivo/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /excluir objetivo/i })).toBeInTheDocument();
  });

  it('canWrite=false: esconde TODAS as ações de escrita (CA-22)', () => {
    renderCard(baseObjective, false);
    expect(screen.queryByRole('button', { name: /editar objetivo/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /excluir objetivo/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /task/i })).not.toBeInTheDocument();
  });

  it('status done: mostra o badge "Concluído"', () => {
    renderCard({ ...baseObjective, status: 'done' }, true);
    expect(screen.getByText(/concluído/i)).toBeInTheDocument();
  });

  it('status open: NÃO mostra "Concluído"', () => {
    renderCard(baseObjective, true);
    expect(screen.queryByText(/concluído/i)).not.toBeInTheDocument();
  });

  it('CA-12.2: by_hours fechado por horas (0 tasks done) → barra cheia, não 0%', () => {
    // done_pct=0 (0 tasks done) mas progress_pct=100 (horas batidas) e status done.
    // A barra DEVE refletir o modo (100%), nunca um chip "Concluído" sobre 0%.
    renderCard(
      {
        ...baseObjective,
        completion_mode: 'by_hours',
        goal_hours: 4,
        task_count: 3,
        done_count: 0,
        done_pct: 0,
        tracked_seconds: 14400,
        progress_pct: 100,
        status: 'done',
      },
      true,
    );
    expect(screen.getByText(/concluído/i)).toBeInTheDocument();
    expect(screen.getByTestId('progress-bar-fill').style.width).toBe('100%');
  });

  it('CA-12.2/RNF-03: by_date vencido aberto não tem cor de erro', () => {
    const { container } = renderCard(
      { ...baseObjective, completion_mode: 'by_date', target_date: '2020-01-01', status: 'open' },
      true,
    );
    expect(container.innerHTML).not.toContain('var(--danger)');
  });
});
