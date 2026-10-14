import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TaskModal from './TaskModal';
import * as api from '../../lib/api';
import type { BoardTask } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, createTask: vi.fn(), patchTask: vi.fn(), getWorkspace: vi.fn() };
});

const objectives = [{ id: 'o1', name: 'Objetivo A' }];

// membros do workspace pros selects de Responsável/Solicitante (fase 18 T-09)
const members = [
  { user_id: 'u1', name: 'Ana Owner', email: 'a@b.c', role: 'owner' as const },
  { user_id: 'u2', name: 'Beto Membro', email: 'b@b.c', role: 'member' as const },
];

const existing: BoardTask = {
  id: 't1', workspace_id: 'w1', project_id: 'p1', objective_id: 'o1', name: 'Antiga', status: 'doing',
  due_date: null, recurrence: null, notes: '', created_by: 'u1', assignee: null, requested_by: null,
  created_at: '2026-06-04T10:00:00Z', tracked_seconds: 0,
};

function renderModal(task: BoardTask | null = null, defaultObjectiveId: string | null = null) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TaskModal
        wsId="w1"
        projectId="p1"
        objectives={objectives}
        task={task}
        defaultObjectiveId={defaultObjectiveId}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createTask).mockResolvedValue({} as never);
  vi.mocked(api.patchTask).mockResolvedValue({} as never);
  vi.mocked(api.getWorkspace).mockResolvedValue({
    id: 'w1', name: 'WS', role: 'owner', created_at: '', members,
  } as never);
});

describe('TaskModal (CA-21)', () => {
  it('criar com objetivo selecionado → createTask com objective_id', async () => {
    renderModal(null, 'o1');
    fireEvent.change(screen.getByLabelText(/nome da task/i), { target: { value: 'Definir telas' } });
    fireEvent.click(screen.getByRole('button', { name: /criar task/i }));
    await waitFor(() =>
      expect(api.createTask).toHaveBeenCalledWith('w1', expect.objectContaining({
        project_id: 'p1',
        name: 'Definir telas',
        objective_id: 'o1',
      })),
    );
  });

  it('criar com "Geral" → objective_id null', async () => {
    renderModal(null, 'o1');
    fireEvent.change(screen.getByLabelText(/nome da task/i), { target: { value: 'Solta' } });
    fireEvent.change(screen.getByLabelText('Objetivo'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: /criar task/i }));
    await waitFor(() =>
      expect(api.createTask).toHaveBeenCalledWith('w1', expect.objectContaining({ objective_id: null })),
    );
  });

  it('editar → patchTask com status e objective_id', async () => {
    renderModal(existing);
    fireEvent.change(screen.getByLabelText(/status da task/i), { target: { value: 'done' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() =>
      expect(api.patchTask).toHaveBeenCalledWith('w1', 't1', expect.objectContaining({
        name: 'Antiga',
        status: 'done',
        objective_id: 'o1',
      })),
    );
  });

  it('nome vazio NÃO chama a API', () => {
    renderModal(null);
    fireEvent.click(screen.getByRole('button', { name: /criar task/i }));
    expect(api.createTask).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  // fase 18 T-09 (CA-02/03 UI): selects de Responsável (assignee) e Solicitante
  // (requested_by) populados pelos membros do workspace; "—" = null.
  it('selects de Responsável/Solicitante listam os membros do workspace', async () => {
    renderModal(null);
    // a opção do membro aparece quando o GET /workspaces/{id} resolve
    await screen.findAllByRole('option', { name: 'Beto Membro' });
    expect(screen.getByLabelText('Responsável')).toBeTruthy();
    expect(screen.getByLabelText('Solicitante')).toBeTruthy();
  });

  it('criar com Responsável escolhido → createTask com assignee', async () => {
    renderModal(null);
    await screen.findAllByRole('option', { name: 'Beto Membro' }); // membros carregados
    fireEvent.change(screen.getByLabelText(/nome da task/i), { target: { value: 'Delegar' } });
    fireEvent.change(screen.getByLabelText('Responsável'), { target: { value: 'u2' } });
    fireEvent.click(screen.getByRole('button', { name: /criar task/i }));
    await waitFor(() =>
      expect(api.createTask).toHaveBeenCalledWith('w1', expect.objectContaining({ assignee: 'u2' })),
    );
  });

  it('criar com Responsável "—" → assignee null; requested_by não enviado fica null', async () => {
    renderModal(null);
    await screen.findAllByRole('option', { name: 'Beto Membro' });
    fireEvent.change(screen.getByLabelText(/nome da task/i), { target: { value: 'Sem dono' } });
    fireEvent.click(screen.getByRole('button', { name: /criar task/i }));
    await waitFor(() =>
      expect(api.createTask).toHaveBeenCalledWith('w1', expect.objectContaining({
        assignee: null,
        requested_by: null,
      })),
    );
  });

  it('editar → patchTask inclui assignee/requested_by escolhidos', async () => {
    renderModal(existing);
    await screen.findAllByRole('option', { name: 'Beto Membro' });
    fireEvent.change(screen.getByLabelText('Responsável'), { target: { value: 'u2' } });
    fireEvent.change(screen.getByLabelText('Solicitante'), { target: { value: 'u1' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() =>
      expect(api.patchTask).toHaveBeenCalledWith('w1', 't1', expect.objectContaining({
        assignee: 'u2',
        requested_by: 'u1',
      })),
    );
  });
});
