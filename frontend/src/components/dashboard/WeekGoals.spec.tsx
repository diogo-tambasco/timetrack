// WeekGoals.spec — fase 13 T-04 (CA-16/19/21): renderiza uma GoalBar por meta
// (meta×realizado); vazio → mensagem; erro → mensagem resiliente (não derruba).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import WeekGoals from './WeekGoals';
import { useAuthStore } from '../../lib/auth';
import * as reports from '../../lib/queries/reports';
import type { WeekReport } from '../../lib/api';

vi.mock('../../lib/queries/reports', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/reports')>();
  return { ...mod, useWeekReport: vi.fn() };
});

function report(goals: WeekReport['goals']): WeekReport {
  return {
    week_start: '2026-06-15', week_end: '2026-06-21', total_seconds: 0,
    by_project: [], focus: { deep_sessions: 0, switches: 0 } as never, goals,
  };
}

function mockQuery(over: Partial<ReturnType<typeof reports.useWeekReport>>) {
  vi.mocked(reports.useWeekReport).mockReturnValue({
    data: undefined, isLoading: false, isError: false, ...over,
  } as never);
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: 'w1', name: 'WS', role: 'owner' }],
    },
  });
});

describe('WeekGoals', () => {
  it('renderiza uma GoalBar por meta com meta×realizado (CA-16)', () => {
    mockQuery({
      data: report([
        { project_id: 'p1', name: 'Lauda', color: '#4f9dde', goal_seconds: 36000, realized_seconds: 18000 },
      ]),
    });
    render(<WeekGoals />);
    expect(screen.getByText('Lauda')).toBeInTheDocument();
    expect(screen.getByText('5h00')).toBeInTheDocument(); // realizado 18000s
    expect(screen.getByText(/10h00/)).toBeInTheDocument(); // meta 36000s
    expect(document.querySelector('.tt-goal')).not.toBeNull();
  });

  it('sem metas → mensagem vazia (CA-19)', () => {
    mockQuery({ data: report([]) });
    render(<WeekGoals />);
    expect(screen.getByText(/nenhum projeto com meta/i)).toBeInTheDocument();
  });

  it('erro → mensagem resiliente, sem throw (CA-21)', () => {
    mockQuery({ isError: true });
    render(<WeekGoals />);
    expect(screen.getByText(/não foi possível carregar as metas/i)).toBeInTheDocument();
  });
});
