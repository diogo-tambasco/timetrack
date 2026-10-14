import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
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
    createProject: vi.fn(),
    patchProject: vi.fn(),
    archiveProject: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
];

const projeto: Project = {
  id: 'p1', workspace_id: ws, client_id: 'c1', name: 'App mobile', color: '#4f9dde',
  weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u-ana',
  archived_at: null, created_at: '2026-06-03T10:00:00Z', deadline: null,
};

const workspace: WorkspaceDetail = {
  id: ws,
  name: 'Workspace de Ana',
  role: 'owner',
  created_at: '2026-06-01T10:00:00Z',
  members: [{ user_id: 'u-ana', name: 'Ana Owner', email: 'ana@example.com', role: 'owner' }],
};

function renderPagina() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Projects />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u-ana', email: 'ana@example.com', name: 'Ana Owner', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listClients).mockResolvedValue(clientes);
  vi.mocked(api.listProjects).mockResolvedValue([projeto]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
  vi.mocked(api.getProjectsSummary).mockResolvedValue([
    {
      id: 'p1', name: 'App mobile', color: '#4f9dde', client_id: 'c1', deadline: null,
      task_count: 0, done_count: 0, open_count: 0, tracked_seconds: 0, due_today_count: 0,
    },
  ]);
  vi.mocked(api.getWorkspace).mockResolvedValue(workspace);
  vi.mocked(api.getProject).mockResolvedValue({ ...projeto, members: [] });
});

// T-19 — integração do ProjectModal na página Projetos: "Novo projeto"
// abre o modal de criação; "Editar" no cartão abre o modo Editar
// pré-preenchido (CA-28.1 "a partir de um projeto existente").
describe('página Projetos — abertura do modal', () => {
  it('"Novo projeto" abre o modal de criação', async () => {
    renderPagina();
    await screen.findByText('App mobile');

    fireEvent.click(screen.getByRole('button', { name: /novo projeto/i }));

    expect(await screen.findByRole('dialog', { name: /novo projeto/i })).toBeInTheDocument();
  });

  it('"Editar" num projeto abre o modal pré-preenchido', async () => {
    renderPagina();
    await screen.findByText('App mobile');

    fireEvent.click(screen.getAllByRole('button', { name: /^editar$/i })[0]);

    expect(await screen.findByRole('dialog', { name: /editar projeto/i })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText(/nome do projeto/i)).toHaveValue('App mobile'),
    );
  });
});
