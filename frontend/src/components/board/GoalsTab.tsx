// GoalsTab (fase 29 T-10, CA-12/13/15): a aba Goals do workspace de projeto —
// visão LISTA Notion (mockup renderMetas) derivada do /board (sem query nova).
// Toolbar com texto informativo + New goal (gated por can_write). Linhas via
// GoalRow (barra por modo). New/Edit abrem o GoalModal; criar/editar/excluir
// invalidam o boardKey (hooks da fase 12) → reflete sem reload. À prova de culpa.
import { useState } from 'react';
import type { Board, BoardObjective } from '../../lib/api';
import { useDeleteObjective, usePatchObjective } from '../../lib/queries/objectives';
import { Icon } from '../icons';
import GoalRow from './GoalRow';
import GoalModal from './GoalModal';

export default function GoalsTab({ wsId, board }: { wsId: string; board: Board }) {
  const projectId = board.project.id;
  const canWrite = board.project.can_write;
  const del = useDeleteObjective(wsId, projectId);
  const patch = usePatchObjective(wsId, projectId);
  const [modal, setModal] = useState<{ open: boolean; objective: BoardObjective | null } | null>(null);

  const objectives = [...board.objectives].sort((a, b) => a.position - b.position);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: 10,
          flexWrap: 'wrap',
        }}
      >
        <p className="faint" style={{ margin: 0, fontSize: 12 }}>
          Progress by tasks, hours or date. Soon:{' '}
          <span className="mono" style={{ color: 'var(--text-1)' }}>
            /goal
          </span>{' '}
          command, cross-project.
        </p>
        {canWrite && (
          <button
            type="button"
            className="tt-btn tt-btn--accent"
            onClick={() => setModal({ open: true, objective: null })}
          >
            <Icon.plus size={15} sw={2.2} />
            New goal
          </button>
        )}
      </div>

      {objectives.length === 0 ? (
        <p className="faint" style={{ fontSize: 13 }}>
          No goals yet.
        </p>
      ) : (
        <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto' }}>
          <table className="tt-table" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th></th>
                <th>Goal</th>
                <th>Progress</th>
                <th className="num">Tasks</th>
                <th className="num">Hours</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {objectives.map((o) => (
                <GoalRow
                  key={o.id}
                  objective={o}
                  color={board.project.color}
                  canWrite={canWrite}
                  onEdit={() => setModal({ open: true, objective: o })}
                  onDelete={() => {
                    if (window.confirm(`Excluir o goal "${o.name}"?`)) del.mutate(o.id);
                  }}
                  onToggleStatus={() =>
                    patch.mutate({
                      id: o.id,
                      patch: { status: o.status === 'done' ? 'open' : 'done' },
                    })
                  }
                />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal?.open && (
        <GoalModal
          wsId={wsId}
          projectId={projectId}
          objective={modal.objective}
          onClose={() => setModal(null)}
        />
      )}
    </div>
  );
}
