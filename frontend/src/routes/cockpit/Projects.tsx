// Página Projetos (T-16): cartões por default, tabela densa sob toggle
// persistido (CA-27 — chave tt.view.projects). O modal Novo/Editar projeto
// (T-18/T-19) abre daqui: criar e editar refletem na lista por invalidação
// da query, sem reload (CA-29).
import { useState } from 'react';
import { Icon } from '../../components/icons';
import ViewToggle, { useView } from '../../components/ui/ViewToggle';
import ProjectCard from '../../components/projects/ProjectCard';
import ProjectsTable from '../../components/projects/ProjectsTable';
import ProjectModal from '../../components/projects/ProjectModal';
import { useClients } from '../../lib/queries/clients';
import { useArchiveProject, useProjects, useProjectsSummary } from '../../lib/queries/projects';
import { useTasks } from '../../lib/queries/tasks';
import { useCurrentWorkspace, useRole } from '../../lib/auth';
import type { Project } from '../../lib/api';

export default function Projects() {
  const ws = useCurrentWorkspace();
  if (!ws) return null;
  return <ProjectsPage wsId={ws.id} />;
}

function ProjectsPage({ wsId }: { wsId: string }) {
  const [view, setView] = useView('tt.view.projects', ['cards', 'table'] as const, 'cards');
  // fase 28: cards VIVOS de UM request (CA-04/16) — horas reais, pendentes,
  // deadline, "tem algo hoje". A lista crua (useProjects) segue só pro modo
  // Tabela e pra alimentar o modal de edição (Project completo com members).
  const { data: summary = [] } = useProjectsSummary(wsId);
  const { data: projects = [] } = useProjects(wsId);
  // lista COMPLETA (com arquivados): cliente arquivado mantém o client_id
  // dos projetos (CA-05) e o nome precisa seguir resolvível aqui
  const { data: clients = [] } = useClients(wsId, { archived: true });
  const { data: tasks = [] } = useTasks(wsId);
  const arquivar = useArchiveProject(wsId);
  // null = fechado · { project: null } = criar · { project } = editar
  const [modal, setModal] = useState<{ project: Project | null } | null>(null);
  // CA-31: viewer não vê ação de escrita nenhuma (o backend já nega — T-02)
  const role = useRole(wsId);
  const canWrite = role === 'owner' || role === 'member';

  const clientNameOf = (clientId: string | null) =>
    clients.find((c) => c.id === clientId)?.name ?? null;
  const projectOf = (id: string) => projects.find((p) => p.id === id) ?? null;
  const clientOf = (p: Project) => clients.find((c) => c.id === p.client_id) ?? null;
  const taskCountOf = (projectId: string) =>
    tasks.filter((t) => t.project_id === projectId).length;
  const nClientes = new Set(summary.map((p) => p.client_id).filter(Boolean)).size;

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 20, height: '100%' }}>
      {/* fase 22: o cabeçalho quebra linha quando não cabe (< md) — os controles
          (toggle + Novo projeto) caem pra baixo sem estourar a largura. */}
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
            {summary.length} projeto{summary.length === 1 ? '' : 's'} · {nClientes} cliente
            {nClientes === 1 ? '' : 's'}
          </p>
          <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>
            Projetos
          </h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <ViewToggle view={view} onChange={setView} />
          {canWrite && (
            <button
              type="button"
              className="tt-btn tt-btn--accent"
              onClick={() => setModal({ project: null })}
            >
              <Icon.plus size={15} sw={2.2} />
              Novo projeto
            </button>
          )}
        </div>
      </div>

      {view === 'cards' ? (
        <div
          style={{
            display: 'grid',
            // min(100%, 300px): em telas < 300px a coluna encolhe pra caber
            // (sem estourar < md); ≥ 300px = 300px como antes (CA-13).
            gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 300px), 1fr))',
            gridAutoRows: 'min-content',
            gap: 16,
            alignContent: 'start',
          }}
        >
          {summary.map((s) => (
            <ProjectCard
              key={s.id}
              summary={s}
              clientName={clientNameOf(s.client_id)}
              canWrite={canWrite}
              onEdit={() => setModal({ project: projectOf(s.id) })}
              onArchive={() => arquivar.mutate(s.id)}
            />
          ))}
          {summary.length === 0 && (
            <span className="faint">Nenhum projeto ainda — crie o primeiro.</span>
          )}
        </div>
      ) : (
        <ProjectsTable
          projects={projects}
          clientOf={clientOf}
          taskCountOf={taskCountOf}
          canWrite={canWrite}
          onEdit={(p) => setModal({ project: p })}
          onArchive={(id) => arquivar.mutate(id)}
        />
      )}

      {modal && (
        <ProjectModal wsId={wsId} project={modal.project} onClose={() => setModal(null)} />
      )}
    </section>
  );
}
