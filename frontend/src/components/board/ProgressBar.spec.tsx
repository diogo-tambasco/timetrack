import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import ProgressBar from './ProgressBar';

// CA-23: barra de horas só com goal_hours; accent ao atingir 100%.
describe('ProgressBar (CA-23)', () => {
  it('sem meta de horas: renderiza só a barra de tasks (1 .tt-goal)', () => {
    const { container } = render(
      <ProgressBar doneCount={1} taskCount={2} donePct={50} trackedSeconds={3600} goalHours={null} color="#4f9dde" />,
    );
    expect(container.querySelectorAll('.tt-goal')).toHaveLength(1);
  });

  it('com meta de horas: renderiza tasks + horas (2 .tt-goal) e a meta', () => {
    const { container, getByText } = render(
      <ProgressBar doneCount={1} taskCount={2} donePct={50} trackedSeconds={3600} goalHours={12} color="#4f9dde" />,
    );
    expect(container.querySelectorAll('.tt-goal')).toHaveLength(2);
    expect(getByText('meta 12h')).toBeInTheDocument();
  });

  it('100% das tasks: a barra de tasks usa o accent', () => {
    const { container } = render(
      <ProgressBar doneCount={3} taskCount={3} donePct={100} trackedSeconds={0} goalHours={null} color="#4f9dde" />,
    );
    const fill = container.querySelector('.tt-goal i') as HTMLElement;
    expect(fill.style.background).toContain('var(--accent)');
    expect(fill.style.width).toBe('100%');
  });
});
