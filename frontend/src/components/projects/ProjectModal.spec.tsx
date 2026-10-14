import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ProjectModal from './ProjectModal';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';
import type { Client, WorkspaceDetail } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listClients: vi.fn(),
    getWorkspace: vi.fn(),
    getProject: vi.fn(),
    createProject: vi.fn(),
    patchProject: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
];

const workspace: WorkspaceDetail = {
  id: ws,
  name: 'Workspace de Ana',
  role: 'owner',
  created_at: '2026-06-01T10:00:00Z',
  members: [
    { user_id: 'u-ana', name: 'Ana Owner', email: 'ana@example.com', role: 'owner' },
    { user_id: 'u-bruno', name: 'Bruno Member', email: 'bruno@example.com', role: 'member' },
    { user_id: 'u-clara', name: 'Clara Viewer', email: 'clara@example.com', role: 'viewer' },
  ],
};

function renderModal(onClose = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <ProjectModal wsId={ws} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
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
});

// T-18 / CA-28 — modal Novo projeto: Privado desabilita a seção de
// membros; Compartilhado permite incluir membro e escolher o acesso
// (Editar × Só vê); Owner fixo e não removível (newproject.jsx).
describe('ProjectModal — Novo projeto (CA-28)', () => {
  it('selecionar Privado desabilita a seção de membros (sem interação)', async () => {
    renderModal();
    await screen.findByLabelText(/incluir bruno member/i);

    fireEvent.click(screen.getByRole('button', { name: /^privado$/i }));

    expect(screen.getByLabelText(/incluir bruno member/i)).toBeDisabled();
    expect(screen.getByLabelText(/incluir clara viewer/i)).toBeDisabled();
    // sem membro interagível, nenhum toggle de acesso responde
    expect(screen.queryByRole('button', { name: /só vê/i })).toBeNull();
    expect(screen.getByText(/disponível em projetos compartilhados/i)).toBeInTheDocument();
  });

  it('Compartilhado permite incluir membro e escolher o acesso de cada um', async () => {
    vi.mocked(api.createProject).mockResolvedValue({
      id: 'p9', workspace_id: ws, client_id: null, name: 'Site novo', color: '#4f9dde',
      weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u-ana',
      archived_at: null, created_at: '2026-06-10T10:00:00Z',
      members: [{ user_id: 'u-bruno', access: 'view' }],
    });
    const onClose = renderModal();
    await screen.findByLabelText(/incluir bruno member/i);

    // default do protótipo é Compartilhado — a seção responde direto
    fireEvent.click(screen.getByLabelText(/incluir bruno member/i));
    fireEvent.click(screen.getByRole('button', { name: /só vê/i }));

    fireEvent.change(screen.getByLabelText(/nome do projeto/i), {
      target: { value: 'Site novo' },
    });
    fireEvent.click(screen.getByRole('button', { name: /criar projeto/i }));

    await waitFor(() =>
      expect(api.createProject).toHaveBeenCalledWith(
        ws,
        expect.objectContaining({
          name: 'Site novo',
          visibility: 'shared',
          members: [{ user_id: 'u-bruno', access: 'view' }],
        }),
      ),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('Owner é fixo e não removível', async () => {
    renderModal();
    await screen.findByLabelText(/incluir bruno member/i);

    // a linha da owner traz o selo e o checkbox dela não interage
    expect(screen.getByText('Owner')).toBeInTheDocument();
    expect(screen.getByText('Owner').closest('.tt-role')).not.toBeNull();
    expect(screen.getByLabelText(/incluir ana owner/i)).toBeDisabled();
  });
});
