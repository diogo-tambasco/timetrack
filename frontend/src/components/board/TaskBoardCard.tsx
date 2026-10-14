// TaskBoardCard (fase 24, CA-17/24): card arrastável do board. Mostra nome +
// ícone de prioridade + meta (due · to-dos subtask_done/count · ⭐ goal_done/count
// · tempo rastreado · ícone Notion se notion_url). draggable só quando canWrite.
// Clicar abre o popup (onOpen).
import type { BoardTask } from '../../lib/api';
import { Icon } from '../icons';
import { fmtDayTotal } from '../../lib/format';
import { priorityColor } from './status';
import { dataBr } from './dateBr';

export default function TaskBoardCard({
  task,
  canWrite,
  onOpen,
  onDragStart,
}: {
  task: BoardTask;
  canWrite: boolean;
  onOpen: () => void;
  onDragStart: () => void;
}) {
  return (
    <div
      data-testid={`task-card-${task.id}`}
      className="tt-card"
      draggable={canWrite}
      onDragStart={(e) => {
        e.dataTransfer.effectAllowed = 'move';
        onDragStart();
      }}
      onClick={onOpen}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onOpen();
        }
      }}
      style={{
        background: 'var(--surface-1)',
        padding: 10,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        cursor: canWrite ? 'grab' : 'pointer',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 7 }}>
        {task.priority > 0 && <PrioIcon p={task.priority} />}
        <span style={{ fontSize: 12.5, fontWeight: 500, wordBreak: 'break-word', flex: 1 }}>
          {task.name}
        </span>
        {task.notion_url != null && (
          <span data-testid="notion-icon" style={{ display: 'inline-flex', flex: '0 0 auto' }}>
            <Icon.link size={13} style={{ color: 'var(--text-3)' }} />
          </span>
        )}
      </div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 9,
          flexWrap: 'wrap',
          fontSize: 11,
          color: 'var(--text-3)',
        }}
      >
        {task.due_date != null && <span>{dataBr(task.due_date)}</span>}
        {task.subtask_count > 0 && (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
            <Icon.check size={11} />
            {task.subtask_done}/{task.subtask_count}
          </span>
        )}
        {task.goal_count > 0 && (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
            <Icon.star size={11} />
            {task.goal_done}/{task.goal_count}
          </span>
        )}
        <span className="mono" style={{ marginLeft: 'auto' }}>
          {fmtDayTotal(task.tracked_seconds)}
        </span>
      </div>
    </div>
  );
}

// barras de prioridade (mockup): 3 barras de altura crescente, acesas até p.
function PrioIcon({ p }: { p: number }) {
  const c = priorityColor(p);
  return (
    <svg viewBox="0 0 24 24" width={13} height={13} style={{ color: c, flex: '0 0 auto', marginTop: 1 }}>
      {[
        [4, 6, p >= 1],
        [10.3, 10, p >= 2],
        [16.6, 14, p >= 3],
      ].map(([x, h, on], i) => (
        <rect
          key={i}
          x={x as number}
          y={20 - (h as number)}
          width={3.4}
          height={h as number}
          rx={1}
          fill="currentColor"
          opacity={on ? 1 : 0.28}
        />
      ))}
    </svg>
  );
}
