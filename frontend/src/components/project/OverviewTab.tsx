// OverviewTab (fase 28 T-09, CA-17/20/21): a aba Overview rica do projeto.
// KPIs (Total time/This week/Open/Done — do /board + relatório da fase 14 p/
// "This week"); Upcoming (tasks com due futuro/hoje, status!=done, asc,
// clicáveis → openTask da fase 24); Progress por status + Goals (do board,
// reusa ProgressBar); Recent notes (fase 26, ADITIVO — omitida se vazia/ausente
// — CA-21). À prova de culpa (RNF-02): vazio = "No deadlines set." (não acusa).
import type { Board, BoardTask } from '../../lib/api';
import { fmtDayTotal } from '../../lib/format';
import { useReport } from '../../lib/queries/reports';
import { useNotes } from '../../lib/queries/notes';
import { dataBr } from '../board/dateBr';
import { STATUS_ORDER, STATUS_META } from '../board/status';
import ProgressBar from '../board/ProgressBar';
import { goalProgress } from '../board/goalProgress';
import { Icon } from '../icons';

function allTasks(board: Board): BoardTask[] {
  return [...board.objectives.flatMap((o) => o.tasks), ...board.general.tasks];
}

export default function OverviewTab({
  wsId,
  board,
  onOpenTask,
  now = new Date(),
}: {
  wsId: string;
  board: Board;
  onOpenTask: (taskId: string) => void;
  now?: Date;
}) {
  const projectId = board.project.id;
  const tasks = allTasks(board);
  const total = tasks.length;
  const done = tasks.filter((t) => t.status === 'done').length;
  const open = total - done;
  const donePct = total === 0 ? 0 : Math.round((done / total) * 100);
  const byStatus = (s: string) => tasks.filter((t) => t.status === s).length;

  // "This week" do relatório da fase 14, filtrado por projeto (D-11).
  const week = useReport(wsId, { period: 'week', project_id: projectId });
  const thisWeek = week.data ? fmtDayTotal(week.data.total_seconds) : '—';

  // Upcoming: due >= hoje (parede local), status != done, asc por data.
  const todayISO = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(
    now.getDate(),
  ).padStart(2, '0')}`;
  const upcoming = tasks
    .filter((t) => t.due_date != null && t.status !== 'done' && t.due_date >= todayISO)
    .sort((a, b) => (a.due_date! < b.due_date! ? -1 : a.due_date! > b.due_date! ? 1 : 0));

  // Recent notes (fase 26, aditivo): top 3 mais recentes. Omitida se vazia.
  const notes = useNotes(wsId, projectId).data ?? [];
  const recentNotes = [...notes]
    .sort((a, b) => (a.updated_at < b.updated_at ? 1 : -1))
    .slice(0, 3);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
          gap: 12,
        }}
      >
        <Kpi label="Total time" value={fmtDayTotal(board.project.tracked_seconds)} mono />
        <Kpi label="This week" value={thisWeek} mono />
        <Kpi label="Open" value={String(open)} />
        <Kpi label="Done" value={`${donePct}%`} sub={`${done}/${total} tasks`} mono />
      </div>

      <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>Upcoming</h3>
        {upcoming.length === 0 ? (
          <p className="faint" style={{ fontSize: 13 }}>
            No deadlines set.
          </p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {upcoming.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => onOpenTask(t.id)}
                className="tt-card"
                style={{
                  padding: '10px 14px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 10,
                  cursor: 'pointer',
                  textAlign: 'left',
                  background: 'var(--surface-1)',
                  border: '1px solid var(--border)',
                }}
              >
                <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                  <span
                    aria-hidden
                    style={{
                      width: 8,
                      height: 8,
                      borderRadius: '50%',
                      background: STATUS_META[t.status].color,
                      flex: '0 0 auto',
                    }}
                  />
                  <span style={{ fontSize: 13.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {t.name}
                  </span>
                </span>
                <span className="chip mono" style={{ fontSize: 11.5, flex: '0 0 auto', gap: 5 }}>
                  <Icon.cal size={12} />
                  {dataBr(t.due_date!)}
                </span>
              </button>
            ))}
          </div>
        )}
      </section>

      <section
        className="tt-card"
        style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}
      >
        <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>Progress</h3>
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
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
                <span style={{ fontSize: 13.5, fontWeight: 540, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {o.name}
                </span>
                {o.status === 'done' && (
                  <span
                    data-testid="goal-done-chip"
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 5,
                      flex: '0 0 auto',
                      fontSize: 11,
                      fontWeight: 600,
                      padding: '2px 9px',
                      borderRadius: 999,
                      color: 'var(--accent)',
                      border: '1px solid var(--accent-line)',
                      background: 'var(--accent-soft)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Icon.check size={12} sw={2.4} />
                    Done
                  </span>
                )}
              </div>
              {/* CA-12.2: a barra reflete o MODO (goalProgress), não done_pct —
                  by_hours fechado por horas mostra a barra cheia, não 0%. */}
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

      {recentNotes.length > 0 && (
        <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>Recent notes</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {recentNotes.map((n) => (
              <div
                key={n.id}
                className="tt-card"
                style={{ padding: '10px 14px', display: 'flex', alignItems: 'center', gap: 10 }}
              >
                <Icon.doc size={15} style={{ color: 'var(--text-3)', flex: '0 0 auto' }} />
                <span style={{ flex: 1, minWidth: 0, fontSize: 13.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {n.title || 'Untitled'}
                </span>
                <span className="faint mono" style={{ fontSize: 11.5, whiteSpace: 'nowrap', flex: '0 0 auto' }}>
                  {dataBr(n.updated_at.slice(0, 10))}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function Kpi({ label, value, sub, mono }: { label: string; value: string; sub?: string; mono?: boolean }) {
  return (
    <div className="tt-card" style={{ padding: 14 }}>
      <p className="eyebrow" style={{ margin: 0 }}>
        {label}
      </p>
      <p className={mono ? 'mono' : undefined} style={{ margin: '4px 0 0', fontSize: 20, fontWeight: 600 }}>
        {value}
      </p>
      {sub && (
        <p className="faint" style={{ margin: '2px 0 0', fontSize: 11.5 }}>
          {sub}
        </p>
      )}
    </div>
  );
}
