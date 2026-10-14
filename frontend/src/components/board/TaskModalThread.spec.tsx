// TaskModalThread.spec — fase 19 T-09: a thread monta SÓ no modo Editar (a
// task existe); no modo criar não aparece. Editar Notas NÃO cria comentário
// (CA-18.1 — notes e thread são camadas separadas).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TaskModal from './TaskModal';
import * as api from '../../lib/api';
import { useAuthStore } from '../../lib/auth';
import type { BoardTask, Me } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    createTask: vi.fn(),
    patchTask: vi.fn(),
    getWorkspace: vi.fn(),
    listComments: vi.fn(),
    createComment: vi.fn(),
  };
});

const existing: BoardTask = {
  id: 't1', workspace_id: 'w1', project_id: 'p1', objective_id: null, name: 'Antiga',
  status: 'doing', due_date: null, recurrence: null, notes: 'nota inicial', created_by: 'u1',
  assignee: null, requested_by: null, created_at: '2026-06-04T10:00:00Z', tracked_seconds: 0,
};

const members = [{ user_id: 'u1', name: 'Ana', email: 'a@b.c', role: 'owner' as const }];

function setMe() {
  const me: Me = {
    user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
    workspaces: [{ id: 'w1', name: 'WS', role: 'owner' }],
  };
  useAuthStore.setState({ status: 'authenticated', me, activeWorkspaceId: 'w1' });
}

function renderModal(task: BoardTask | null) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TaskModal wsId="w1" projectId="p1" objectives={[]} task={task} canComment onClose={vi.fn()} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createTask).mockResolvedValue({} as never);
  vi.mocked(api.patchTask).mockResolvedValue({} as never);
  vi.mocked(api.createComment).mockResolvedValue({} as never);
  vi.mocked(api.listComments).mockResolvedValue([] as never);
  vi.mocked(api.getWorkspace).mockResolvedValue({
    id: 'w1', name: 'WS', role: 'owner', created_at: '', members,
  } as never);
  setMe();
});

describe('TaskModal monta a thread (T-09)', () => {
  it('modo Editar (task existe) mostra a seção de Comentários', async () => {
    renderModal(existing);
    await waitFor(() => expect(screen.getByLabelText('Comentários')).toBeInTheDocument());
    expect(api.listComments).toHaveBeenCalledWith('w1', 't1');
  });

  it('modo criar (sem task) NÃO mostra a thread', () => {
    renderModal(null);
    expect(screen.queryByLabelText('Comentários')).not.toBeInTheDocument();
    expect(api.listComments).not.toHaveBeenCalled();
  });

  it('editar Notas e salvar NÃO cria comentário (CA-18.1)', async () => {
    renderModal(existing);
    await waitFor(() => expect(screen.getByLabelText('Comentários')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/notas da task/i), { target: { value: 'nota nova' } });
    fireEvent.click(screen.getByRole('button', { name: /^salvar$/i }));

    await waitFor(() =>
      expect(api.patchTask).toHaveBeenCalledWith(
        'w1',
        't1',
        expect.objectContaining({ notes: 'nota nova' }),
      ),
    );
    expect(api.createComment).not.toHaveBeenCalled();
  });
});
