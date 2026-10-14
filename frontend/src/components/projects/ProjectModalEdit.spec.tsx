import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';
import ProjectModal from './ProjectModal';
import { useAuthStore } from '../../lib/auth';
import { useProjects } from '../../lib/queries/projects';
import * as api from '../../lib/api';
import type { Client, Project, WorkspaceDetail } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listClients: vi.fn(),
    listProjects: vi.fn(),
    getWorkspace: vi.fn(),
    getProject: vi.fn(),
    createProject: vi.fn(),
    patchProject: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
  { id: 'c2', workspace_id: ws, name: 'Lauda', archived_at: null, created_at: '2026-06-02T10:00:00Z' },
];

const workspace: WorkspaceDetail = {
  id: ws,
  name: 'Workspace de Ana',
  role: 'owner',
  created_at: '2026-06-01T10:00:00Z',
  members: [
    { user_id: 'u-ana', name: 'Ana Owner', email: 'ana@example.com', role: 'owner' },
    { user_id: 'u-bruno', name: 'Bruno Member', email: 'bruno@example.com', role: 'member' },
  ],
};

const projeto: Project = {
  id: 'p1', workspace_id: ws, client_id: 'c1', name: 'App mobile', color: '#b07cd6',
  weekly_hours_goal: 12, visibility: 'shared', owner_user_id: 'u-ana',
  archived_at: null, created_at: '2026-06-03T10:00:00Z',
};

// harness com uma lista viva de useProjects: prova que o submit do modo
// Editar reflete na lista por invalidação, sem recarregar a página
function ListaComModal() {
  const { data: projects = [] } = useProjects(ws);
  const [editando, setEditando] = useState<Project | null>(projeto);
  return (
    <>
      <ul aria-label="lista de projetos">
        {projects.map((p) => (
          <li key={p.id}>{p.name}</li>
        ))}
      </ul>
      {editando && (
        <ProjectModal wsId={ws} project={editando} onClose={() => setEditando(null)} />
      )}
    </>
  );
}

function renderHarness() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <ListaComModal />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u-ana', email: 'ana@example.com', name: 'Ana Owner', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listClients).mockResolvedValue(clientes);
  vi.mocked(api.getWorkspace).mockResolvedValue(workspace);
  vi.mocked(api.getProject).mockResolvedValue({
    ...projeto,
    members: [{ user_id: 'u-bruno', access: 'edit' }],
  });
});

// T-18 / CA-28.1 — modo Editar: campos pré-preenchidos com os valores
// atuais (nome, cliente, cor, meta, visibilidade, membros); confirmar
// aplica via PATCH e a lista reflete sem recarregar a página.
describe('ProjectModal — modo Editar (CA-28.1)', () => {
  it('abre com os campos preenchidos com os valores atuais do projeto', async () => {
    renderHarness();

    expect(await screen.findByLabelText(/nome do projeto/i)).toHaveValue('App mobile');
    // o select é controlado com c1; espera a lista de clientes materializar a option
    await waitFor(() => expect(screen.getByLabelText(/^cliente$/i)).toHaveValue('c1'));
    expect(screen.getByLabelText(/meta de horas/i)).toHaveValue(12);
    // CA-07: a API aceita qualquer meta em (0,168] com 2 casas (ex.: 12.25);
    // step de 0.5 deixava o form nativamente inválido e travava o Salvar
    expect(screen.getByLabelText(/meta de horas/i)).toHaveAttribute('step', 'any');
    expect(screen.getByLabelText(/meta de horas/i)).toHaveAttribute('min', '0.01');
    expect(screen.getByRole('button', { name: /compartilhado/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByLabelText(/cor #b07cd6/i)).toHaveAttribute('aria-pressed', 'true');
    // membros atuais do projeto: Bruno incluído com acesso Editar
    expect(screen.getByLabelText(/incluir bruno member/i)).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /^editar$/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('submit chama o PATCH só com o que mudou e a lista reflete sem reload', async () => {
    vi.mocked(api.listProjects)
      .mockResolvedValueOnce([projeto])
      .mockResolvedValue([{ ...projeto, name: 'App mobile v2' }]);
    vi.mocked(api.patchProject).mockResolvedValue({ ...projeto, name: 'App mobile v2' });
    renderHarness();

    expect(await screen.findByText('App mobile')).toBeInTheDocument();
    const nome = await screen.findByLabelText(/nome do projeto/i);
    fireEvent.change(nome, { target: { value: 'App mobile v2' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    // diff-based: só o nome mudou — nada de reenviar client_id/membros
    await waitFor(() =>
      expect(api.patchProject).toHaveBeenCalledWith(ws, 'p1', { name: 'App mobile v2' }),
    );

    // a lista viva reflete a alteração (invalidate → refetch), sem reload
    expect(await screen.findByText('App mobile v2')).toBeInTheDocument();
    // e o modal fechou
    await waitFor(() => expect(screen.queryByLabelText(/nome do projeto/i)).toBeNull());
  });
});
