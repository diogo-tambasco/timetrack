// goalProgress.spec.ts — fase 29 T-08 (CA-12.1): a barra de progresso por MODO.
// by_tasks/by_date → done_pct; by_hours → progress_pct (horas vs meta, do board).
// À prova de culpa: sempre [0,100], nunca "falho" mesmo com marco vencido.
import { describe, it, expect } from 'vitest';
import { goalProgress } from './goalProgress';
import type { BoardObjective } from '../../lib/api';

function obj(over: Partial<BoardObjective>): BoardObjective {
  return {
    id: 'o', name: 'G', description: '', status: 'open', goal_hours: null,
    position: 0, task_count: 0, done_count: 0, done_pct: 0, tracked_seconds: 0,
    completion_mode: 'by_tasks', target_date: null, progress_pct: 0, tasks: [],
    ...over,
  };
}

describe('goalProgress (CA-12.1)', () => {
  it('by_tasks usa done_pct', () => {
    expect(goalProgress(obj({ completion_mode: 'by_tasks', done_pct: 50, progress_pct: 50 }))).toBe(50);
  });

  it('by_hours com meta usa progress_pct (não done_pct)', () => {
    // 0 tasks done, mas horas batendo 45% da meta → 45 (não 0).
    expect(
      goalProgress(obj({ completion_mode: 'by_hours', done_pct: 0, progress_pct: 45 })),
    ).toBe(45);
  });

  it('by_hours sem meta → 0 (progress_pct=0, sem div/0)', () => {
    expect(
      goalProgress(obj({ completion_mode: 'by_hours', goal_hours: null, done_pct: 80, progress_pct: 0 })),
    ).toBe(0);
  });

  it('by_hours acima da meta clampa em 100', () => {
    expect(
      goalProgress(obj({ completion_mode: 'by_hours', done_pct: 0, progress_pct: 100 })),
    ).toBe(100);
  });

  it('by_date usa done_pct informativo (sem flag de falho mesmo com data passada)', () => {
    expect(
      goalProgress(
        obj({ completion_mode: 'by_date', done_pct: 30, progress_pct: 30, target_date: '2020-01-01' }),
      ),
    ).toBe(30);
  });

  it('clampa valores fora de faixa em [0,100]', () => {
    expect(goalProgress(obj({ completion_mode: 'by_hours', progress_pct: 150 }))).toBe(100);
    expect(goalProgress(obj({ completion_mode: 'by_tasks', done_pct: -5 }))).toBe(0);
  });
});
