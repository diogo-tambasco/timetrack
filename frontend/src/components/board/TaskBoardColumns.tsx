// TaskBoardColumns (fase 24 T-11, CA-17/18/25): 4 colunas backlog/todo/doing/done
// (rótulos do mockup), cada uma ordenada por position. DnD HTML5 NATIVO (sem lib):
// card draggable + coluna onDragOver(preventDefault)/onDrop → dropStatus puro →
// onDrop(mutation). Quick-add "Add" por coluna cria no status da coluna (CA-18).
// Gating: can_write=false → sem drag, sem quick-add (CA-25).
import { useRef, useState } from 'react';
import type { BoardTask, TaskStatus } from '../../lib/api';
import { Icon } from '../icons';
import { STATUS_ORDER, STATUS_META } from './status';
import { dropStatus, tasksByStatus, type DropMutation } from './dnd';
import TaskBoardCard from './TaskBoardCard';

export default function TaskBoardColumns({
  tasks,
  canWrite,
  onOpenTask,
  onDrop,
  onQuickAdd,
}: {
  tasks: BoardTask[];
  canWrite: boolean;
  onOpenTask: (task: BoardTask) => void;
  onDrop: (m: DropMutation) => void;
  onQuickAdd: (status: TaskStatus) => void;
}) {
  // id arrastado (HTML5 nativo guarda no ref; o drop lê e chama dropStatus puro).
  const dragId = useRef<string | null>(null);

  return (
    <div
      data-testid="board-columns"
      className="grid grid-cols-1 md:grid-cols-4"
      style={{ gap: 10, alignItems: 'start' }}
    >
      {STATUS_ORDER.map((status) => {
        const colTasks = tasksByStatus(tasks, status);
        return (
          <Column
            key={status}
            status={status}
            count={colTasks.length}
            canWrite={canWrite}
            onQuickAdd={() => onQuickAdd(status)}
            onDropHere={() => {
              const id = dragId.current;
              dragId.current = null;
              if (id == null) return;
              const m = dropStatus(id, status, tasks);
              if (m) onDrop(m);
            }}
          >
            {colTasks.map((t) => (
              <TaskBoardCard
                key={t.id}
                task={t}
                canWrite={canWrite}
                onOpen={() => onOpenTask(t)}
                onDragStart={() => {
                  dragId.current = t.id;
                }}
              />
            ))}
            {colTasks.length === 0 && (
              <span className="faint" style={{ fontSize: 11.5, padding: '2px 4px' }}>
                —
              </span>
            )}
          </Column>
        );
      })}
    </div>
  );
}

function Column({
  status,
  count,
  canWrite,
  onQuickAdd,
  onDropHere,
  children,
}: {
  status: TaskStatus;
  count: number;
  canWrite: boolean;
  onQuickAdd: () => void;
  onDropHere: () => void;
  children: React.ReactNode;
}) {
  const [over, setOver] = useState(false);
  return (
    <div
      data-testid={`board-col-${status}`}
      onDragOver={(e) => {
        if (!canWrite) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        if (!canWrite) return;
        e.preventDefault();
        setOver(false);
        onDropHere();
      }}
      style={{
        background: over ? 'var(--surface-3)' : 'var(--surface-2)',
        borderRadius: 10,
        padding: 8,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        minHeight: 56,
        outline: over ? '2px dashed var(--accent-line)' : 'none',
      }}
    >
      <div
        className="eyebrow"
        style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '2px 4px' }}
      >
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <span aria-hidden style={{ width: 8, height: 8, borderRadius: '50%', background: STATUS_META[status].color }} />
          {STATUS_META[status].label}
        </span>
        <span className="mono">{count}</span>
      </div>
      {children}
      {canWrite && (
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label={`Add em ${STATUS_META[status].label}`}
          onClick={onQuickAdd}
          style={{ padding: '5px 8px', fontSize: 11.5, justifyContent: 'flex-start', gap: 5 }}
        >
          <Icon.plus size={13} sw={2.2} />
          Add
        </button>
      )}
    </div>
  );
}
