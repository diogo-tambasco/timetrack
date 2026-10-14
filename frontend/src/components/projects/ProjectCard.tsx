// Cartão de projeto VIVO (fase 28, CA-16): mata o `0h` hardcoded desde a fase
// 02. Mostra horas REAIS (fmtHours(tracked_seconds)), `X pendentes · Y no total`
// (open_count/task_count do summary), o cliente, o prazo (deadline, quando há) e
// um dot "tem algo hoje" quando due_today_count>0. Linka pro detalhe (board).
// Dados de UM GET .../projects/summary (sem N+1 — fonte: routes/cockpit/Projects).
import { Link } from 'react-router-dom';
import type { ProjectSummary } from '../../lib/api';
import { fmtHours, fmtDateBr } from '../../lib/format';
import { Icon } from '../icons';

export default function ProjectCard({
  summary,
  clientName,
  canWrite,
  onEdit,
  onArchive,
}: {
  summary: ProjectSummary;
  clientName: string | null;
  // CA-20/31: viewer não vê Editar/Arquivar
  canWrite: boolean;
  onEdit: () => void;
  onArchive: () => void;
}) {
  const { id, name, color, deadline, open_count, task_count, tracked_seconds, due_today_count } =
    summary;
  return (
    <div
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
        <span
          aria-hidden
          style={{ width: 4, height: 30, borderRadius: 3, background: color, flex: '0 0 auto', marginTop: 2 }}
        />
        <div style={{ flex: 1, minWidth: 0 }}>
          <Link
            to={`/projetos/${id}`}
            style={{
              display: 'block',
              fontSize: 15,
              fontWeight: 600,
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              color: 'var(--text-1)',
              textDecoration: 'none',
            }}
          >
            {name}
          </Link>
          <div className="faint" style={{ fontSize: 11.5, marginTop: 1 }}>
            {open_count} pendente{open_count === 1 ? '' : 's'} · {task_count} no total
          </div>
        </div>
        {/* "tem algo hoje" — dot no accent (à prova de culpa: não é vermelho) */}
        {due_today_count > 0 && (
          <span
            className="chip"
            data-testid="project-today"
            title={`${due_today_count} para hoje`}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 5, flex: '0 0 auto' }}
          >
            <span
              aria-hidden
              className="tt-dot"
              style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--accent)' }}
            />
            Hoje
          </span>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
        <span
          style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13, color: 'var(--text-2)' }}
        >
          <Icon.cliente size={14} style={{ color: 'var(--text-3)' }} />
          {clientName ?? '—'}
        </span>
        <span className="mono" style={{ fontSize: 12.5, fontWeight: 540, color: 'var(--text-2)' }}>
          {fmtHours(tracked_seconds)}
        </span>
        {deadline && (
          <span
            data-testid="project-deadline"
            className="chip"
            style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 11.5 }}
          >
            <Icon.cal size={12} />
            {fmtDateBr(deadline)}
          </span>
        )}
      </div>

      <div
        style={{
          borderTop: '1px solid var(--border)',
          paddingTop: 12,
          display: 'flex',
          justifyContent: 'space-between',
          gap: 4,
        }}
      >
        <Link to={`/projetos/${id}`} className="tt-btn tt-btn--ghost" style={{ textDecoration: 'none' }}>
          <Icon.list size={14} />
          Abrir
        </Link>
        {canWrite && (
          <div style={{ display: 'flex', gap: 4 }}>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={onEdit}>
              Editar
            </button>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={onArchive}>
              Arquivar
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
