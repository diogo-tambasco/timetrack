// Tabela densa de projetos (T-16, D-11): a lista do projects.jsx vira a
// visão tabela, com colunas ADAPTADAS ao schema (spec §6): "Esta semana"
// = 0h e "Total" = — até a fase 03; Status = Ativo/Arquivado via
// archived_at (sem o "Pausado" do protótipo).
import { Link } from 'react-router-dom';
import type { Client, Project } from '../../lib/api';
import StatusPill from './StatusPill';

export default function ProjectsTable({
  projects,
  clientOf,
  taskCountOf,
  canWrite,
  onEdit,
  onArchive,
}: {
  projects: Project[];
  clientOf: (p: Project) => Client | null;
  taskCountOf: (projectId: string) => number;
  // CA-31: viewer não vê a coluna de ações
  canWrite: boolean;
  onEdit: (p: Project) => void;
  onArchive: (projectId: string) => void;
}) {
  return (
    <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto' }}>
      <table className="tt-table">
        <thead>
          <tr>
            <th>Projeto</th>
            <th>Cliente</th>
            <th className="num">Esta semana</th>
            <th className="num">Total</th>
            <th>Status</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {projects.map((p) => (
            <tr key={p.id}>
              <td>
                <Link
                  to={`/projetos/${p.id}`}
                  style={{ color: 'inherit', textDecoration: 'none' }}
                >
                  <span style={{ display: 'flex', alignItems: 'center', gap: 9, fontWeight: 540 }}>
                    <span
                      aria-hidden
                      style={{ width: 4, height: 22, borderRadius: 3, background: p.color }}
                    />
                    <span>
                      {p.name}
                      <span className="faint" style={{ fontSize: 11.5, display: 'block' }}>
                        {taskCountOf(p.id)} task{taskCountOf(p.id) === 1 ? '' : 's'}
                      </span>
                    </span>
                  </span>
                </Link>
              </td>
              <td className="muted">{clientOf(p)?.name ?? '—'}</td>
              <td className="num">0h</td>
              <td className="num muted">—</td>
              <td>
                <StatusPill archived={p.archived_at !== null} />
              </td>
              <td style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                {canWrite && (
                  <>
                    <button
                      type="button"
                      className="tt-btn tt-btn--ghost"
                      onClick={() => onEdit(p)}
                    >
                      Editar
                    </button>
                    <button
                      type="button"
                      className="tt-btn tt-btn--ghost"
                      onClick={() => onArchive(p.id)}
                    >
                      Arquivar
                    </button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
