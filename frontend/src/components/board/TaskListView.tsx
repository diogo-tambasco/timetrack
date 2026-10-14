// TaskListView (fase 24 T-14, CA-16/18): visão LISTA — grupos por status com
// quick-add por grupo; status 1-clique cicla. (Esqueleto na T-11; preenchido na
// T-14.)
import type { BoardTask, TaskStatus } from '../../lib/api';
import { STATUS_ORDER, STATUS_META, nextStatus } from './status';
import { tasksByStatus } from './dnd';
import { Icon } from '../icons';
import { fmtDayTotal } from '../../lib/format';

export default function TaskListView({
  tasks,
  canWrite,
  onOpenTask,
  onQuickAdd,
  onCycleStatus,
}: {
  tasks: BoardTask[];
  canWrite: boolean;
  onOpenTask: (t: BoardTask) => void;
  onQuickAdd: (status: TaskStatus) => void;
  onCycleStatus: (t: BoardTask, next: TaskStatus) => void;
}) {
  return (
    <div data-testid="task-list" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {STATUS_ORDER.map((status) => {
        const group = tasksByStatus(tasks, status);
        return (
          <div key={status} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <div
              className="eyebrow"
              style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '2px 0' }}
            >
              <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', background: STATUS_META[status].color }} />
              {STATUS_META[status].label}
              <span className="mono faint">{group.length}</span>
            </div>
            {group.map((t) => (
              <div
                key={t.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '7px 4px',
                  borderBottom: '1px solid var(--border)',
                }}
              >
                <button
                  type="button"
                  aria-label={`Status de ${t.name}`}
                  disabled={!canWrite}
                  onClick={() => canWrite && onCycleStatus(t, nextStatus(t.status))}
                  className="chip"
                  style={{ cursor: canWrite ? 'pointer' : 'default', color: STATUS_META[t.status].color }}
                >
                  {STATUS_META[t.status].label}
                </button>
                <button
                  type="button"
                  onClick={() => onOpenTask(t)}
                  style={{
                    flex: 1,
                    textAlign: 'left',
                    border: 'none',
                    background: 'transparent',
                    cursor: 'pointer',
                    fontFamily: 'var(--font-ui)',
                    fontSize: 13,
                    color: 'var(--text-1)',
                  }}
                >
                  {t.name}
                  {t.notion_url != null && (
                    <span data-testid="notion-icon" style={{ display: 'inline-flex', marginLeft: 6 }}>
                      <Icon.link size={12} style={{ color: 'var(--text-3)' }} />
                    </span>
                  )}
                </button>
                <span className="mono faint" style={{ fontSize: 11.5 }}>
                  {fmtDayTotal(t.tracked_seconds)}
                </span>
              </div>
            ))}
            {canWrite && (
              <button
                type="button"
                className="tt-btn tt-btn--ghost"
                aria-label={`Add em ${STATUS_META[status].label}`}
                onClick={() => onQuickAdd(status)}
                style={{ padding: '4px 6px', fontSize: 11.5, justifyContent: 'flex-start', gap: 5 }}
              >
                <Icon.plus size={13} sw={2.2} />
                Add
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
