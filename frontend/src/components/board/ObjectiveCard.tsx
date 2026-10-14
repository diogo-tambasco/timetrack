// ObjectiveCard (fase 12 T-09, CA-19/22): cabeçalho do objetivo (nome, badge
// "Concluído" quando status='done', progresso) + ações de escrita gated por
// canWrite (vem de board.project.can_write — CA-22, não a role do ws). As
// tasks do objetivo entram pelo slot `children` (board/lista — composto pelo
// ProjectBoard, T-11).
import type { ReactNode } from 'react';
import type { BoardObjective } from '../../lib/api';
import { Icon } from '../icons';
import ProgressBar from './ProgressBar';
import { goalProgress } from './goalProgress';

export default function ObjectiveCard({
  objective,
  color,
  canWrite,
  onEdit,
  onDelete,
  onNewTask,
  children,
}: {
  objective: BoardObjective;
  color: string;
  canWrite: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onNewTask: () => void;
  children?: ReactNode;
}) {
  const done = objective.status === 'done';
  return (
    <section
      className="tt-card"
      style={{
        padding: 18,
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
        background: 'var(--surface-1)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <h3
              style={{
                margin: 0,
                fontSize: 15.5,
                fontWeight: 600,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {objective.name}
            </h3>
            {done && (
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 5,
                  fontSize: 11,
                  fontWeight: 600,
                  padding: '2px 9px',
                  borderRadius: 999,
                  color: 'var(--accent)',
                  background: 'var(--accent-soft)',
                  whiteSpace: 'nowrap',
                }}
              >
                <Icon.check size={12} sw={2.4} />
                Concluído
              </span>
            )}
          </div>
          {objective.description && (
            <p className="faint" style={{ margin: '3px 0 0', fontSize: 12.5 }}>
              {objective.description}
            </p>
          )}
        </div>
        {canWrite && (
          <div style={{ display: 'flex', gap: 4, flex: '0 0 auto' }}>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={onNewTask}>
              <Icon.plus size={14} sw={2.2} />
              Task
            </button>
            <button
              type="button"
              aria-label="Editar objetivo"
              className="tt-btn tt-btn--ghost"
              onClick={onEdit}
            >
              Editar
            </button>
            <button
              type="button"
              aria-label="Excluir objetivo"
              className="tt-btn tt-btn--ghost"
              onClick={onDelete}
            >
              <Icon.trash size={14} />
            </button>
          </div>
        )}
      </div>

      <ProgressBar
        doneCount={objective.done_count}
        taskCount={objective.task_count}
        donePct={objective.done_pct}
        barPct={goalProgress(objective)}
        trackedSeconds={objective.tracked_seconds}
        goalHours={objective.goal_hours}
        color={color}
      />

      {children}
    </section>
  );
}
