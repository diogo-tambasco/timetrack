// TaskTableView (fase 24 T-14, CA-19): visão TABLE nativa — colunas status ·
// priority · due · to-dos (subtask_done/count) · time. status/priority 1-clique
// ciclam; clicar na linha abre o popup; quick-add. (Custom props = fase 25.)
import type { BoardTask, PropertyDef, TaskStatus } from '../../lib/api';
import { STATUS_META, nextStatus, PRIORITY_LABEL, priorityColor, nextPriority } from './status';
import { Icon } from '../icons';
import { fmtDayTotal } from '../../lib/format';
import { dataBr } from './dateBr';
import PropertyCell from './PropertyCell';

export default function TaskTableView({
  tasks,
  canWrite,
  onOpenTask,
  onQuickAdd,
  onCycleStatus,
  onCyclePriority,
  wsId,
  projectId,
  defs = [],
}: {
  tasks: BoardTask[];
  canWrite: boolean;
  onOpenTask: (t: BoardTask) => void;
  onQuickAdd: (status: TaskStatus) => void;
  onCycleStatus: (t: BoardTask, next: TaskStatus) => void;
  onCyclePriority: (t: BoardTask, next: number) => void;
  // fase 25 (CA-19): colunas custom dinâmicas após as nativas (a Table reusa a classe
  // real .tt-table da fase 24; o mockup chamava .ntable). [] = sem propriedades.
  wsId?: string;
  projectId?: string;
  defs?: PropertyDef[];
}) {
  const cols = [...defs].sort((a, b) => a.position - b.position);
  const ordered = [...tasks].sort(
    (a, b) => a.position - b.position || a.created_at.localeCompare(b.created_at),
  );
  return (
    <div data-testid="task-table" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <table className="tt-table" style={{ width: '100%' }}>
        <thead>
          <tr>
            <th>Task</th>
            <th>Status</th>
            <th>Priority</th>
            <th>Due</th>
            <th>To-dos</th>
            <th className="num">Time</th>
            {cols.map((d) => (
              <th key={d.id}>{d.name}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {ordered.map((t) => (
            <tr key={t.id}>
              <td>
                <button
                  type="button"
                  onClick={() => onOpenTask(t)}
                  style={{ border: 'none', background: 'transparent', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 13, color: 'var(--text-1)', textAlign: 'left' }}
                >
                  {t.name}
                  {t.notion_url != null && (
                    <span data-testid="notion-icon" style={{ display: 'inline-flex', marginLeft: 6 }}>
                      <Icon.link size={12} style={{ color: 'var(--text-3)' }} />
                    </span>
                  )}
                </button>
              </td>
              <td>
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
              </td>
              <td>
                <button
                  type="button"
                  aria-label={`Prioridade de ${t.name}`}
                  disabled={!canWrite}
                  onClick={() => canWrite && onCyclePriority(t, nextPriority(t.priority))}
                  className="chip"
                  style={{ cursor: canWrite ? 'pointer' : 'default', color: priorityColor(t.priority) }}
                >
                  {PRIORITY_LABEL[t.priority]}
                </button>
              </td>
              <td>{t.due_date != null ? dataBr(t.due_date) : '—'}</td>
              <td>
                {t.subtask_count > 0 ? `${t.subtask_done}/${t.subtask_count}` : '—'}
              </td>
              <td className="num mono">{fmtDayTotal(t.tracked_seconds)}</td>
              {cols.map((d) => (
                <td key={d.id}>
                  {wsId && projectId ? (
                    <PropertyCell wsId={wsId} projectId={projectId} def={d} task={t} canWrite={canWrite} />
                  ) : null}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {canWrite && (
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label="Add task"
          onClick={() => onQuickAdd('todo')}
          style={{ alignSelf: 'flex-start', padding: '5px 8px', fontSize: 11.5, gap: 5 }}
        >
          <Icon.plus size={13} sw={2.2} />
          Add
        </button>
      )}
    </div>
  );
}
