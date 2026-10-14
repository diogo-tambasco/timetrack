// Tabela densa de clientes (T-15): a visão sob toggle (D-11), porte da
// ClientsTable de clients.jsx restrito ao schema — Cliente, Projetos e
// ações; sem contato/valor-h/status (spec §6).
import type { Client, Project } from '../../lib/api';

export default function ClientsTable({
  clients,
  projectsOf,
  canWrite,
  onArchive,
}: {
  clients: Client[];
  projectsOf: (clientId: string) => Project[];
  // CA-31: viewer não vê a coluna de ações
  canWrite: boolean;
  onArchive: (clientId: string) => void;
}) {
  return (
    <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto' }}>
      <table className="tt-table">
        <thead>
          <tr>
            <th>Cliente</th>
            <th>Projetos</th>
            <th className="num">Nº</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {clients.map((c) => {
            const projetos = projectsOf(c.id);
            return (
              <tr key={c.id}>
                <td>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 9, fontWeight: 540 }}>
                    {c.name}
                  </span>
                </td>
                <td className="muted">{projetos.map((p) => p.name).join(', ') || '—'}</td>
                <td className="num">{projetos.length}</td>
                <td style={{ textAlign: 'right' }}>
                  {canWrite && (
                    <button
                      type="button"
                      className="tt-btn tt-btn--ghost"
                      onClick={() => onArchive(c.id)}
                    >
                      Arquivar
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
