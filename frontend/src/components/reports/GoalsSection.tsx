// GoalsSection (fase 07 — ângulo 3, CA-19): meta vs. realizado da semana
// por projeto, REUSANDO a GoalBar da fase 02 (fill na cor do projeto,
// accent ao atingir 100% — CA-30 da 02) sem fork.
import { GoalBar } from '../projects/ProjectChip';
import { fmtDayTotal } from '../../lib/format';
import type { ReportGoal } from '../../lib/api';

export default function GoalsSection({ goals }: { goals: ReportGoal[] }) {
  if (goals.length === 0) {
    return (
      <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
        Nenhum projeto com meta semanal.
      </div>
    );
  }
  return (
    <div data-testid="goals-section" style={{ display: 'grid', gap: 14 }}>
      {goals.map((g) => (
        <div key={g.project_id} style={{ display: 'grid', gap: 5 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
            <span style={{ fontSize: 'var(--fs-row)', fontWeight: 540 }}>{g.name}</span>
            <span style={{ flex: 1 }} />
            <span className="mono" style={{ fontSize: 13 }}>
              <span style={{ fontWeight: 600 }}>{fmtDayTotal(g.realized_seconds)}</span>
              <span className="faint"> / {fmtDayTotal(g.goal_seconds)}</span>
            </span>
          </div>
          <GoalBar
            color={g.color}
            hours={g.realized_seconds / 3600}
            goal={g.goal_seconds / 3600}
          />
        </div>
      ))}
    </div>
  );
}
