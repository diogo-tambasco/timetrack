// Página Clientes (T-15): cartões por default, tabela densa sob toggle
// persistido (CA-27 — chave tt.view.clients), criação/renome/arquivamento
// ligados aos hooks de lib/queries/clients (CA-01/04/05 na borda da UI).
import { useState, type FormEvent } from 'react';
import { Icon } from '../../components/icons';
import ViewToggle, { useView } from '../../components/ui/ViewToggle';
import ClientCard from '../../components/clients/ClientCard';
import ClientsTable from '../../components/clients/ClientsTable';
import {
  useArchiveClient,
  useClients,
  useCreateClient,
  useRenameClient,
} from '../../lib/queries/clients';
import { useProjects } from '../../lib/queries/projects';
import { useCurrentWorkspace, useRole } from '../../lib/auth';

export default function Clients() {
  const ws = useCurrentWorkspace();
  if (!ws) return null;
  return <ClientsPage wsId={ws.id} />;
}

function ClientsPage({ wsId }: { wsId: string }) {
  const [view, setView] = useView('tt.view.clients', ['cards', 'table'] as const, 'cards');
  const { data: clients = [] } = useClients(wsId);
  const { data: projects = [] } = useProjects(wsId);
  const criar = useCreateClient(wsId);
  const renomear = useRenameClient(wsId);
  const arquivar = useArchiveClient(wsId);
  // CA-31: viewer não vê ação de escrita nenhuma (o backend já nega — T-02)
  const role = useRole(wsId);
  const canWrite = role === 'owner' || role === 'member';

  const [criando, setCriando] = useState(false);
  const [nome, setNome] = useState('');

  const projectsOf = (clientId: string) => projects.filter((p) => p.client_id === clientId);

  const submitNovo = (e: FormEvent) => {
    e.preventDefault();
    const name = nome.trim();
    if (!name) return;
    criar.mutate(
      { name },
      {
        onSuccess: () => {
          setCriando(false);
          setNome('');
        },
      },
    );
  };

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 20, height: '100%' }}>
      {/* fase 22: cabeçalho quebra linha < md (toggle + Novo cliente caem) */}
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: 12,
        }}
      >
        <div>
          <p className="eyebrow" style={{ whiteSpace: 'nowrap', margin: 0 }}>
            {clients.length} cliente{clients.length === 1 ? '' : 's'}
          </p>
          <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>
            Clientes
          </h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <ViewToggle view={view} onChange={setView} />
          {canWrite && (
            <button
              type="button"
              className="tt-btn tt-btn--accent"
              onClick={() => setCriando(true)}
            >
              <Icon.plus size={15} sw={2.2} />
              Novo cliente
            </button>
          )}
        </div>
      </div>

      {criando && (
        <form
          onSubmit={submitNovo}
          className="tt-card"
          style={{ padding: 14, display: 'flex', gap: 10, alignItems: 'center' }}
        >
          <input
            aria-label="Nome do cliente"
            value={nome}
            onChange={(e) => setNome(e.target.value)}
            placeholder="Nome do cliente"
            autoFocus
            style={{
              flex: 1,
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 8,
              color: 'var(--text-1)',
              font: 'inherit',
              padding: '8px 10px',
            }}
          />
          <button type="submit" className="tt-btn tt-btn--accent">
            Criar
          </button>
          <button type="button" className="tt-btn tt-btn--ghost" onClick={() => setCriando(false)}>
            Cancelar
          </button>
        </form>
      )}

      {view === 'cards' ? (
        <div
          style={{
            display: 'grid',
            // min(100%, 280px): a coluna encolhe pra caber em telas < 280px (CA-13)
            gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 280px), 1fr))',
            gridAutoRows: 'min-content',
            gap: 16,
            alignContent: 'start',
          }}
        >
          {clients.map((c) => (
            <ClientCard
              key={c.id}
              client={c}
              projects={projectsOf(c.id)}
              canWrite={canWrite}
              onRename={(name) => renomear.mutateAsync({ id: c.id, name })}
              onArchive={() => arquivar.mutate(c.id)}
            />
          ))}
          {clients.length === 0 && (
            <span className="faint">Nenhum cliente ainda — crie o primeiro.</span>
          )}
        </div>
      ) : (
        <ClientsTable
          clients={clients}
          projectsOf={projectsOf}
          canWrite={canWrite}
          onArchive={(id) => arquivar.mutate(id)}
        />
      )}
    </section>
  );
}
