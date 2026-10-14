// autoClose.spec — fase 24 T-15 (CA-05/17, ponta a ponta no front; reconcilia o
// ProjectBoard.autoclose.spec da fase 12 — CA-28.1): mover a última task de um
// objetivo pra `done` fecha o objetivo (recálculo no backend, fase 12), e a aba
// Overview reflete a conclusão APÓS o refetch do board. Aqui simulamos os dois
// estados do /board (antes/depois) e asseguramos que a Overview mostra done%=100.
import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import ProjectOverview from './ProjectOverview';
import { mkBoard, mkTask, mkProject } from './testFixtures';
import type { Board, BoardObjective } from '../../lib/api';

function boardWith(objStatus: 'open' | 'done', taskStatus: 'doing' | 'done'): Board {
  const objective: BoardObjective = {
    id: 'o1', name: 'Lançar', description: '', status: objStatus, goal_hours: null,
    position: 0, task_count: 1, done_count: taskStatus === 'done' ? 1 : 0,
    done_pct: taskStatus === 'done' ? 100 : 0, tracked_seconds: 0,
    tasks: [mkTask({ id: 't1', name: 'Última', status: taskStatus, objective_id: 'o1' })],
  };
  return mkBoard({
    project: mkProject(),
    objectives: [objective],
    general: { task_count: 0, done_count: 0, tracked_seconds: 0, tasks: [] },
  });
}

// localiza o card do goal "Lançar" pra escopar a asserção do % (o KPI "Concluídas"
// também mostra %; o goal card é o que reflete o recálculo do objetivo).
function goalCard(): HTMLElement {
  const heading = screen.getByText('Lançar');
  // o ProgressBar do goal vive no mesmo card (div.tt-card) do nome do goal.
  return heading.closest('.tt-card') as HTMLElement;
}

describe('conclusão automática do objetivo na Overview (CA-05/17/28.1)', () => {
  it('última task doing → objetivo a 0% (aberto)', () => {
    render(<ProjectOverview board={boardWith('open', 'doing')} />);
    expect(screen.getByText('Goals')).toBeInTheDocument();
    expect(within(goalCard()).getByText('0%')).toBeInTheDocument();
    expect(within(goalCard()).getByText('0/1 task')).toBeInTheDocument();
  });

  it('última task done (refetch do board) → objetivo a 100% na Overview', () => {
    render(<ProjectOverview board={boardWith('done', 'done')} />);
    // a Overview reflete done_pct=100 vindo do board recalculado (fase 12).
    expect(within(goalCard()).getByText('100%')).toBeInTheDocument();
    expect(within(goalCard()).getByText('1/1 task')).toBeInTheDocument();
  });
});
