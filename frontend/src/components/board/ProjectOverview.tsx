// ProjectOverview (fase 24 T-10, CA-15): esqueleto da aba Overview — KPIs do
// /board (tempo total / this week=0h por ora / tasks abertas / % done) +
// andamento por status (backlog/todo/doing/done) + os goals do projeto (reusa
// ProgressBar da fase 12). SÓ dados do /board — nada inventado. A fase 28
// substitui por uma Overview rica.
import type { Board, BoardTask } from '../../lib/api';
import { fmtDayTotal } from '../../lib/format';
import { STATUS_ORDER, STATUS_META } from './status';
import ProgressBar from './ProgressBar';
import { goalProgress } from './goalProgress';

function allTasks(board: Board): BoardTask[] {
  return [...board.objectives.flatMap((o) => o.tasks), ...board.general.tasks];
}

export default function ProjectOverview({ board }: { board: Board }) {
  const tasks = allTasks(board);
  const total = tasks.length;
  const done = tasks.filter((t) => t.status === 'done').length;
  const open = total - done;
  const donePct = total === 0 ? 0 : Math.round((done / total) * 100);
  const byStatus = (s: string) => tasks.filter((t) => t.status === s).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
          gap: 12,
        }}
      >
        <Kpi label="Tempo total" value={fmtDayTotal(board.project.tracked_seconds)} mono />
        <Kpi label="Esta semana" value="0h" mono />
        <Kpi label="Abertas" value={String(open)} />
        <Kpi label="Concluídas" value={`${donePct}%`} mono />
      </div>

      <section
        className="tt-card"
        style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}
      >
        <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>Andamento por status</h3>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
          {STATUS_ORDER.map((s) => (
            <div key={s} style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
              <span
                aria-hidden
                style={{ width: 9, height: 9, borderRadius: '50%', background: STATUS_META[s].color }}
              />
              <span className="faint" style={{ fontSize: 12.5 }}>
                {STATUS_META[s].label}
              </span>
              <span className="mono" style={{ fontSize: 13, fontWeight: 600 }}>
                {byStatus(s)}
              </span>
            </div>
          ))}
        </div>
      </section>

      {board.objectives.length > 0 && (
        <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>Goals</h3>
          {board.objectives.map((o) => (
            <div
              key={o.id}
              className="tt-card"
              style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 8 }}
            >
              <span style={{ fontSize: 13.5, fontWeight: 540 }}>{o.name}</span>
              <ProgressBar
                doneCount={o.done_count}
                taskCount={o.task_count}
                donePct={o.done_pct}
                barPct={goalProgress(o)}
                trackedSeconds={o.tracked_seconds}
                goalHours={o.goal_hours}
                color={board.project.color}
              />
            </div>
          ))}
        </section>
      )}
    </div>
  );
}

function Kpi({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="tt-card" style={{ padding: 14 }}>
      <p className="eyebrow" style={{ margin: 0 }}>
        {label}
      </p>
      <p
        className={mono ? 'mono' : undefined}
        style={{ margin: '4px 0 0', fontSize: 20, fontWeight: 600 }}
      >
        {value}
      </p>
    </div>
  );
}
