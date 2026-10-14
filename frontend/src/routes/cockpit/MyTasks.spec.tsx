// MyTasks.spec — fase 18 T-10 (CA-18/19 UI): "Minhas tasks" (assigned) e
// "Pedidas por mim" (requested) com contexto cross-ws; troca de aba refaz a
// query; estado vazio amigável.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import MyTasks from './MyTasks';
import * as api from '../../lib/api';
import type { AssignedTask } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, getAssignedTasks: vi.fn(), getRequestedTasks: vi.fn() };
});

function mk(over: Partial<AssignedTask> & { id: string }): AssignedTask {
  return {
    id: over.id,
    workspace_id: 'w1',
    project_id: 'p1',
    objective_id: null,
    name: over.name ?? 'Task',
    status: over.status ?? 'todo',
    due_date: null,
    recurrence: null,
    notes: '',
    created_by: 'u9',
    assignee: 'me',
    requested_by: null,
    created_at: '2026-06-04T10:00:00Z',
    project: over.project ?? { id: 'p1', name: 'App', color: '#4f9dde' },
    workspace: over.workspace ?? { id: 'w1', name: 'Meu WS' },
    client: over.client ?? null,
    ...over,
  };
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <MyTasks />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('MyTasks (CA-18/19)', () => {
  it('renderiza as tasks atribuídas com contexto cross-ws (projeto/workspace/cliente)', async () => {
    vi.mocked(api.getAssignedTasks).mockResolvedValue([
      mk({ id: 't1', name: 'Revisar deck', project: { id: 'p1', name: 'App', color: '#4f9dde' }, workspace: { id: 'w1', name: 'Meu WS' }, client: 'Acme' }),
    ]);
    vi.mocked(api.getRequestedTasks).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText('Revisar deck')).toBeTruthy();
    expect(screen.getByText('App')).toBeTruthy();
    expect(screen.getByText(/Meu WS/)).toBeTruthy();
    expect(screen.getByText(/Acme/)).toBeTruthy();
  });

  it('a aba "Pedidas por mim" troca a query', async () => {
    vi.mocked(api.getAssignedTasks).mockResolvedValue([mk({ id: 'a1', name: 'Minha task' })]);
    vi.mocked(api.getRequestedTasks).mockResolvedValue([mk({ id: 'r1', name: 'Pedi pra outro' })]);
    renderPage();
    expect(await screen.findByText('Minha task')).toBeTruthy();

    fireEvent.click(screen.getByRole('tab', { name: /pedidas por mim/i }));
    expect(await screen.findByText('Pedi pra outro')).toBeTruthy();
    await waitFor(() => expect(api.getRequestedTasks).toHaveBeenCalled());
  });

  it('estado vazio mostra placeholder amigável', async () => {
    vi.mocked(api.getAssignedTasks).mockResolvedValue([]);
    vi.mocked(api.getRequestedTasks).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText(/nenhuma task/i)).toBeTruthy();
  });
});
