import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, type RenderResult } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import Clients from './Clients';
import Projects from './Projects';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';
import type { Client, Project, WorkspaceDetail } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listClients: vi.fn(),
    listProjects: vi.fn(),
    listTasks: vi.fn(),
    getProjectsSummary: vi.fn(),
    getWorkspace: vi.fn(),
    getProject: vi.fn(),
    createClient: vi.fn(),
    renameClient: vi.fn(),
    archiveClient: vi.fn(),
    createProject: vi.fn(),
    patchProject: vi.fn(),
    archiveProject: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
];

const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: 'c1', name: 'App mobile', color: '#4f9dde',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u-outro',
    archived_at: null, created_at: '2026-06-03T10:00:00Z', deadline: null,
  },
];

const workspace: WorkspaceDetail = {
  id: ws,
  name: 'Workspace',
  role: 'viewer',
  created_at: '2026-06-01T10:00:00Z',
  members: [{ user_id: 'u-outro', name: 'Outro', email: 'outro@example.com', role: 'owner' }],
};

function setRole(role: 'viewer' | 'member') {
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u-eu', email: 'eu@example.com', name: 'Eu', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace', role }],
    },
  });
}

function renderPagina(Page: () => React.JSX.Element | null): RenderResult {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Page />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const botoesEscrita = [/novo cliente/i, /novo projeto/i, /renomear/i, /arquivar/i, /^editar$/i];

function expectSemBotoesDeEscrita() {
  for (const nome of botoesEscrita) {
    expect(screen.queryAllByRole('button', { name: nome })).toHaveLength(0);
  }
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  vi.mocked(api.listClients).mockResolvedValue(clientes);
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.listTasks).mockResolvedValue([]);
  vi.mocked(api.getProjectsSummary).mockResolvedValue([
    {
      id: 'p1', name: 'App mobile', color: '#4f9dde', client_id: 'c1', deadline: null,
      task_count: 0, done_count: 0, open_count: 0, tracked_seconds: 0, due_today_count: 0,
    },
  ]);
  vi.mocked(api.getWorkspace).mockResolvedValue(workspace);
  vi.mocked(api.getProject).mockResolvedValue({ ...projetos[0], members: [] });
});

// T-20 / CA-31 — ENQUANTO o usuário logado for viewer, a UI NÃO exibe os
// botões de escrita (Novo cliente, Novo projeto, renomear/editar/arquivar)
// nas páginas da fase; member segue vendo tudo.
describe('gating de viewer (CA-31)', () => {
  it('viewer não vê nenhum botão de escrita na página Clientes (cards e tabela)', async () => {
    setRole('viewer');
    renderPagina(Clients);
    await screen.findByText('Acme');

    expectSemBotoesDeEscrita();

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));
    expectSemBotoesDeEscrita();
  });

  it('viewer não vê nenhum botão de escrita na página Projetos (cards e tabela)', async () => {
    setRole('viewer');
    renderPagina(Projects);
    await screen.findByText('App mobile');

    expectSemBotoesDeEscrita();

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));
    expectSemBotoesDeEscrita();
  });

  it('member vê os botões de escrita nas duas páginas', async () => {
    setRole('member');
    const { unmount } = renderPagina(Clients);
    await screen.findByText('Acme');
    expect(screen.getByRole('button', { name: /novo cliente/i })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /renomear/i }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('button', { name: /arquivar/i }).length).toBeGreaterThan(0);
    unmount();

    renderPagina(Projects);
    await screen.findByText('App mobile');
    expect(screen.getByRole('button', { name: /novo projeto/i })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /^editar$/i }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('button', { name: /arquivar/i }).length).toBeGreaterThan(0);
  });
});
