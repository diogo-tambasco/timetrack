import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ReactElement } from 'react';
import Clients from './Clients';
import Projects from './Projects';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';
import type { Client, Project, ProjectSummary, Task } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listClients: vi.fn(),
    listProjects: vi.fn(),
    listTasks: vi.fn(),
    getProjectsSummary: vi.fn(),
  };
});

const ws = 'w1';

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
];

const clienteArquivado: Client = {
  id: 'c-arq', workspace_id: ws, name: 'Primor', archived_at: '2026-06-08T10:00:00Z',
  created_at: '2026-06-01T10:00:00Z',
};

const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: 'c1', name: 'App mobile', color: '#4f9dde',
    weekly_hours_goal: 18, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-03T10:00:00Z', deadline: null,
  },
];

// fase 28: cards vivos vêm do summary (CA-04/16).
const resumo: ProjectSummary[] = [
  {
    id: 'p1', name: 'App mobile', color: '#4f9dde', client_id: 'c1', deadline: null,
    task_count: 2, done_count: 0, open_count: 2, tracked_seconds: 0, due_today_count: 0,
  },
];

const tarefas: Task[] = [
  {
    id: 't1', workspace_id: ws, project_id: 'p1', name: 'Onboarding', status: 'todo',
    due_date: null, recurrence: null, notes: '', created_by: 'u1',
    assignee: null, requested_by: null, created_at: '2026-06-04T10:00:00Z',
  },
  {
    id: 't2', workspace_id: ws, project_id: 'p1', name: 'Fluxo de pagamento', status: 'doing',
    due_date: null, recurrence: null, notes: '', created_by: 'u1',
    assignee: null, requested_by: null, created_at: '2026-06-05T10:00:00Z',
  },
];

function renderCom(ui: ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
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
  // o backend devolve TODOS os clientes com ?archived=true (CA-03) — o
  // mock respeita o contrato pra exercitar a resolução de nomes da página
  vi.mocked(api.listClients).mockImplementation(async (_ws, opts) =>
    opts?.archived ? [...clientes, clienteArquivado] : clientes,
  );
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.listTasks).mockResolvedValue(tarefas);
  vi.mocked(api.getProjectsSummary).mockResolvedValue(resumo);
});

// T-16 — teste canônico do CA-27: Clientes E Projetos abrem na visão
// cartões; a .tt-table aparece só após o toggle; a escolha persiste POR
// PÁGINA entre visitas (localStorage).
describe('CA-27 — cartões default, tabela sob toggle', () => {
  it('Clientes abre em cartões; tabela só após toggle; escolha persiste ao remontar', async () => {
    const primeira = renderCom(<Clients />);
    await screen.findByText('Acme');
    expect(document.querySelector('table.tt-table')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));
    expect(document.querySelector('table.tt-table')).not.toBeNull();

    primeira.unmount();
    renderCom(<Clients />);
    await screen.findByText('Acme');
    expect(document.querySelector('table.tt-table')).not.toBeNull();
  });

  it('Projetos abre em cartões com os dados do summary; tabela só após toggle; persiste', async () => {
    const primeira = renderCom(<Projects />);
    await screen.findByText('App mobile');
    // card VIVO (fase 28): cliente + pendentes/total do summary
    expect(screen.getByText('Acme')).toBeInTheDocument();
    expect(screen.getByText(/2 pendentes · 2 no total/)).toBeInTheDocument();
    expect(document.querySelector('table.tt-table')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /tabela/i }));
    const tabela = document.querySelector('table.tt-table');
    expect(tabela).not.toBeNull();
    // colunas adaptadas ao schema (spec §6): "Esta semana" placeholder 0h,
    // "Total" — e Status derivado de archived_at (sem "Pausado")
    expect(screen.getByText('Esta semana')).toBeInTheDocument();
    expect(screen.getByText('0h')).toBeInTheDocument();
    expect(screen.getByText('Total')).toBeInTheDocument();
    expect(screen.queryByText('Pausado')).toBeNull();

    primeira.unmount();
    renderCom(<Projects />);
    await screen.findByText('App mobile');
    expect(document.querySelector('table.tt-table')).not.toBeNull();
    expect(localStorage.getItem('tt.view.projects')).toBe('table');
  });

  it('projeto de cliente arquivado mostra o nome do cliente, não "—" (CA-05)', async () => {
    // arquivar cliente mantém o client_id do projeto (CA-05) — a página
    // precisa resolver o nome contra a lista completa, não a default
    vi.mocked(api.listProjects).mockResolvedValue([
      ...projetos,
      {
        id: 'p2', workspace_id: ws, client_id: 'c-arq', name: 'Site institucional',
        color: '#22c55e', weekly_hours_goal: null, visibility: 'shared',
        owner_user_id: 'u1', archived_at: null, created_at: '2026-06-04T10:00:00Z', deadline: null,
      },
    ]);
    vi.mocked(api.getProjectsSummary).mockResolvedValue([
      ...resumo,
      {
        id: 'p2', name: 'Site institucional', color: '#22c55e', client_id: 'c-arq', deadline: null,
        task_count: 0, done_count: 0, open_count: 0, tracked_seconds: 0, due_today_count: 0,
      },
    ]);

    renderCom(<Projects />);
    await screen.findByText('Site institucional');

    expect(screen.getByText('Primor')).toBeInTheDocument();
  });

  it('a preferência é por página: tabela em Projetos não afeta Clientes', async () => {
    localStorage.setItem('tt.view.projects', 'table');

    renderCom(<Clients />);
    await screen.findByText('Acme');
    expect(document.querySelector('table.tt-table')).toBeNull();
  });
});
