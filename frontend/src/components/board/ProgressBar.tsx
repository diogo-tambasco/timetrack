// ProgressBar (fase 12 T-09, CA-19/23): progresso de um objetivo — barra .tt-goal
// + barra de horas (GoalBar) SÓ com goal_hours (regra do .tt-goal da fase 02).
// Accent ao atingir 100%. Fase 29 (CA-12.2): a barra principal aceita `barPct`
// (override por MODO — by_hours mostra horas, não tasks); sem barPct cai em
// donePct (comportamento da fase 12). A contagem "X/Y tasks" segue informativa.
import { GoalBar } from '../projects/ProjectChip';
import { fmtDayTotal } from '../../lib/format';

export default function ProgressBar({
  doneCount,
  taskCount,
  donePct,
  trackedSeconds,
  goalHours,
  color,
  barPct,
}: {
  doneCount: number;
  taskCount: number;
  donePct: number;
  trackedSeconds: number;
  goalHours: number | null;
  color: string;
  barPct?: number;
}) {
  // barra principal POR MODO (CA-12.2): barPct quando dado (goalProgress), senão
  // donePct (fase 12). by_hours fechado por horas com 0 tasks done → barra cheia.
  const pct = barPct ?? donePct;
  const hasGoal = goalHours !== null && goalHours > 0;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 9 }}>
      <div>
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            fontSize: 11.5,
            marginBottom: 5,
            whiteSpace: 'nowrap',
          }}
        >
          <span className="faint">
            {doneCount}/{taskCount} task{taskCount === 1 ? '' : 's'}
          </span>
          <span className="mono" style={{ fontWeight: 540 }}>
            {pct}%
          </span>
        </div>
        <div className="tt-goal" aria-label="progresso do objetivo">
          <i
            data-testid="progress-bar-fill"
            style={{
              width: `${pct}%`,
              background: pct >= 100 ? 'var(--accent)' : color,
            }}
          />
        </div>
      </div>

      {hasGoal && (
        <div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              fontSize: 11.5,
              marginBottom: 5,
              whiteSpace: 'nowrap',
            }}
          >
            <span className="mono faint">{fmtDayTotal(trackedSeconds)}</span>
            <span className="mono faint">meta {goalHours}h</span>
          </div>
          <GoalBar color={color} hours={trackedSeconds / 3600} goal={goalHours} />
        </div>
      )}
    </div>
  );
}
