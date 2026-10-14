// GoalRow (fase 29 T-08, CA-12/12.1/15): a linha da tabela Notion de Goals
// (mockup renderMetas 471–479). Colunas: ⭐ · Goal · Progress · Tasks · Hours ·
// Status · ações. A barra reflete o MODO (goalProgress) — by_hours mostra horas,
// não tasks. Chip Done (accent) / Open. À prova de culpa (RNF-03): sem vermelho,
// marco vencido aberto = Open discreto. Ações de escrita gated por canWrite (CA-15).
import type { BoardObjective } from '../../lib/api';
import { Icon } from '../icons';
import { fmtDayTotal } from '../../lib/format';
import { goalProgress } from './goalProgress';

export default function GoalRow({
  objective,
  color,
  canWrite,
  onEdit,
  onDelete,
  onToggleStatus,
}: {
  objective: BoardObjective;
  color: string;
  canWrite: boolean;
  onEdit: () => void;
  onDelete: () => void;
  // concluir/reabrir manual (CA-08): 1-clique no chip. Em by_date é a ÚNICA
  // forma de fechar (CA-07 — sem auto-close). Dopamina sem culpa (RNF-03).
  onToggleStatus: () => void;
}) {
  const pct = goalProgress(objective);
  const done = objective.status === 'done';
  const hasGoal = objective.goal_hours !== null && objective.goal_hours > 0;
  const hours = hasGoal
    ? `${fmtDayTotal(objective.tracked_seconds)}/${objective.goal_hours}h`
    : fmtDayTotal(objective.tracked_seconds);

  return (
    <tr data-testid="goal-row">
      <td style={{ width: 26, color: 'var(--accent)' }}>
        <Icon.star size={15} />
      </td>
      <td style={{ fontWeight: 540, whiteSpace: 'nowrap' }}>{objective.name}</td>
      <td style={{ minWidth: 180 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <div className="tt-goal" style={{ flex: 1 }} aria-label="progresso do goal">
            <i
              data-testid="goal-bar-fill"
              style={{ width: `${pct}%`, background: pct >= 100 ? 'var(--accent)' : color }}
            />
          </div>
          <span className="mono faint" style={{ fontSize: 11.5, width: 34, textAlign: 'right' }}>
            {pct}%
          </span>
        </div>
      </td>
      <td className="num faint" style={{ fontSize: 12 }}>
        {objective.done_count}/{objective.task_count}
      </td>
      <td className="num faint mono" style={{ fontSize: 12 }}>
        {hours}
      </td>
      <td>
        <StatusChip done={done} canWrite={canWrite} onToggle={onToggleStatus} />
      </td>
      <td className="num">
        {canWrite && (
          <div style={{ display: 'inline-flex', gap: 4 }}>
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
      </td>
    </tr>
  );
}

// chip de status: clicável (concluir/reabrir, CA-08) quando canWrite, senão
// leitura. Done = accent (dopamina); Open = neutro discreto (sem culpa, RNF-03).
function StatusChip({
  done,
  canWrite,
  onToggle,
}: {
  done: boolean;
  canWrite: boolean;
  onToggle: () => void;
}) {
  const style = done
    ? {
        color: 'var(--accent)',
        border: '1px solid var(--accent-line)',
        background: 'var(--accent-soft)',
      }
    : {
        color: 'var(--text-2)',
        border: '1px solid var(--border)',
        background: 'var(--surface-2)',
      };
  const base = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 5,
    fontSize: 11,
    fontWeight: 600,
    padding: '2px 9px',
    borderRadius: 999,
    whiteSpace: 'nowrap',
    ...style,
  } as const;
  const content = (
    <>
      {done && <Icon.check size={12} sw={2.4} />}
      {done ? 'Done' : 'Open'}
    </>
  );
  if (!canWrite) {
    return (
      <span className="chip" style={base}>
        {content}
      </span>
    );
  }
  return (
    <button
      type="button"
      className="chip"
      aria-label={done ? 'Reabrir goal' : 'Concluir goal'}
      onClick={onToggle}
      style={{ ...base, cursor: 'pointer' }}
    >
      {content}
    </button>
  );
}
