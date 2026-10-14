// TaskRow (fase 12 T-10, CA-19/20/23): linha de task na visão LISTA — status
// (select que dispara PATCH status quando canWrite, senão pill), nome, data,
// tempo rastreado (tracked_seconds formatado) e Editar. Sem DnD (D-12).
import type { BoardTask, TaskStatus } from '../../lib/api';
import { fmtDayTotal } from '../../lib/format';

const statusLabel: Record<TaskStatus, string> = {
  backlog: 'Backlog',
  todo: 'A fazer',
  doing: 'Fazendo',
  done: 'Feito',
};

// data ISO YYYY-MM-DD → DD/MM/YYYY, sem depender de fuso/locale
const dataBr = (iso: string) => iso.split('-').reverse().join('/');

export default function TaskRow({
  task,
  canWrite,
  onEdit,
  onMove,
}: {
  task: BoardTask;
  canWrite: boolean;
  onEdit: (task: BoardTask) => void;
  onMove: (id: string, status: TaskStatus) => void;
}) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        fontSize: 13,
        minWidth: 0,
        padding: '7px 0',
        borderTop: '1px solid var(--border)',
      }}
    >
      {canWrite ? (
        <select
          aria-label={`Status de ${task.name}`}
          value={task.status}
          onChange={(e) => onMove(task.id, e.target.value as TaskStatus)}
          style={{
            background: 'var(--surface-2)',
            border: '1px solid var(--border-2)',
            borderRadius: 999,
            color: 'var(--text-2)',
            font: 'inherit',
            fontSize: 11,
            fontWeight: 540,
            padding: '2px 6px',
          }}
        >
          {(['todo', 'doing', 'done'] as TaskStatus[]).map((s) => (
            <option key={s} value={s}>
              {statusLabel[s]}
            </option>
          ))}
        </select>
      ) : (
        <span
          style={{
            fontSize: 11,
            fontWeight: 540,
            padding: '2px 8px',
            borderRadius: 999,
            border: '1px solid var(--border)',
            color: task.status === 'done' ? 'var(--text-3)' : 'var(--text-2)',
            whiteSpace: 'nowrap',
          }}
        >
          {statusLabel[task.status]}
        </span>
      )}

      <span
        style={{
          flex: 1,
          minWidth: 0,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          textDecoration: task.status === 'done' ? 'line-through' : 'none',
          color: task.status === 'done' ? 'var(--text-3)' : 'var(--text-1)',
        }}
      >
        {task.name}
      </span>

      {task.due_date && (
        <span className="mono faint" style={{ fontSize: 11.5, whiteSpace: 'nowrap' }}>
          {dataBr(task.due_date)}
        </span>
      )}
      <span className="mono faint" style={{ fontSize: 11.5, whiteSpace: 'nowrap' }}>
        {fmtDayTotal(task.tracked_seconds)}
      </span>
      {canWrite && (
        <button
          type="button"
          aria-label={`Editar ${task.name}`}
          className="tt-btn tt-btn--ghost"
          onClick={() => onEdit(task)}
        >
          Editar
        </button>
      )}
    </div>
  );
}
