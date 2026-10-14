import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Clients from './Clients';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';
import type { Client, Project } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listClients: vi.fn(),
    listProjects: vi.fn(),
    createClient: vi.fn(),
    renameClient: vi.fn(),
    archiveClient: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
  { id: 'c2', workspace_id: ws, name: 'Lauda', archived_at: null, created_at: '2026-06-02T10:00:00Z' },
];

const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: 'c1', name: 'App mobile', color: '#4f9dde',
    weekly_hours_goal: 10, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-03T10:00:00Z',
  },
];

function renderPagina() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Clients />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listClients).mockResolvedValue(clientes);
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.createClient).mockResolvedValue({
    id: 'c3', workspace_id: ws, name: 'Arkan', archived_at: null, created_at: '2026-06-10T10:00:00Z',
  });
});

// T-15 — página Clientes: cartões por default a partir dos dados da API,
// tabela densa só após o toggle, criação ligada ao hook certo.
describe('página Clientes', () => {
  it('abre em cartões com os clientes da API (sem .tt-table)', async () => {
    renderPagina();

    expect(await screen.findByText('Acme')).toBeInTheDocument();
    expect(screen.getByText('Lauda')).toBeInTheDocument();
    // cartão traz o nº de projetos e o chip do projeto do cliente
    expect(screen.getByText('1 projeto')).toBeInTheDocument();
    expect(screen.getByText('App mobile')).toBeInTheDocument();
    // visão default é cartões — nenhuma tabela densa na página
    expect(document.querySelector('table.tt-table')).toBeNull();
    expect(document.querySelector('.tt-card')).not.toBeNull();
  });

  it('tabela densa aparece só após acionar o toggle', async () => {
    renderPagina();
    await screen.findByText('Acme');

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));

    expect(document.querySelector('table.tt-table')).not.toBeNull();
    expect(localStorage.getItem('tt.view.clients')).toBe('table');
  });

  it('criar cliente chama o hook certo com o workspace da sessão', async () => {
    renderPagina();
    await screen.findByText('Acme');

    fireEvent.click(screen.getByRole('button', { name: /novo cliente/i }));
    fireEvent.change(screen.getByLabelText(/nome do cliente/i), { target: { value: 'Arkan' } });
    fireEvent.click(screen.getByRole('button', { name: /^criar$/i }));

    await waitFor(() => expect(api.createClient).toHaveBeenCalledWith(ws, { name: 'Arkan' }));
  });

  it('renomear e arquivar ligados aos hooks', async () => {
    vi.mocked(api.renameClient).mockResolvedValue({ ...clientes[0], name: 'Acme Tech' });
    vi.mocked(api.archiveClient).mockResolvedValue(undefined);
    renderPagina();
    await screen.findByText('Acme');

    fireEvent.click(screen.getAllByRole('button', { name: /renomear/i })[0]);
    fireEvent.change(screen.getByLabelText(/novo nome/i), { target: { value: 'Acme Tech' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() => expect(api.renameClient).toHaveBeenCalledWith(ws, 'c1', { name: 'Acme Tech' }));

    fireEvent.click(screen.getAllByRole('button', { name: /arquivar/i })[0]);
    await waitFor(() => expect(api.archiveClient).toHaveBeenCalledWith(ws, 'c1'));
  });

  it('renome que falha mantém o editor aberto (fecha só no sucesso)', async () => {
    const { ApiError } = await vi.importActual<typeof import('../../lib/api')>('../../lib/api');
    vi.mocked(api.renameClient).mockRejectedValue(
      new ApiError(400, 'validation_error', 'nome deve ter entre 1 e 120 caracteres'),
    );
    renderPagina();
    await screen.findByText('Acme');

    fireEvent.click(screen.getAllByRole('button', { name: /renomear/i })[0]);
    fireEvent.change(screen.getByLabelText(/novo nome/i), { target: { value: 'Nome Rejeitado' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() => expect(api.renameClient).toHaveBeenCalled());

    // a mutação falhou — o editor NÃO pode fechar como se tivesse salvo
    expect(screen.getByLabelText(/novo nome/i)).toBeInTheDocument();
  });
});
