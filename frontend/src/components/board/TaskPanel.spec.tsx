// TaskPanel.spec — fase 24 T-12 (CA-20/21/22/26/25/28.1): o popup central da task.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TaskPanel from './TaskPanel';
import { mkBoard, mkTask } from './testFixtures';

const { patchSpy } = vi.hoisted(() => ({ patchSpy: vi.fn() }));

vi.mock('../../lib/queries/tasks', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/tasks')>();
  return { ...mod, usePatchTask: () => ({ mutate: patchSpy }) };
});
// sub-tasks: sem rede — useSubtasks devolve lista vazia, mutações são no-op.
vi.mock('../../lib/queries/subtasks', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/subtasks')>();
  return {
    ...mod,
    useSubtasks: () => ({ data: [] }),
    useCreateSubtask: () => ({ mutate: vi.fn() }),
    usePatchSubtask: () => ({ mutate: vi.fn() }),
    useDeleteSubtask: () => ({ mutate: vi.fn() }),
  };
});
// thread: sem rede; expõe o canComment recebido pra pinar o wiring (CA-28.1).
vi.mock('./TaskThread', () => ({
  default: ({ canComment }: { canComment: boolean }) => (
    <div data-testid="thread-stub" data-can-comment={String(canComment)} />
  ),
}));
// workspace multiplayer (2 membros) por padrão: a thread de comentários e as
// linhas Responsável/Solicitante só aparecem fora do solo (fase 31). O caso solo
// (thread oculta) tem teste próprio que sobrescreve este mock.
const membersMock = vi.hoisted(() => ({ current: [{ user_id: 'u1', name: 'Eu' }, { user_id: 'u2', name: 'Colega' }] }));
vi.mock('../../lib/queries/workspace', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/workspace')>();
  return { ...mod, useWorkspaceMembers: () => ({ data: membersMock.current }) };
});
// auth: currentUserId estável.
vi.mock('../../lib/auth', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/auth')>();
  return { ...mod, useAuthStore: (sel: (s: { me: { user: { id: string } } }) => unknown) => sel({ me: { user: { id: 'u1' } } }) };
});

function renderPanel(over: Parameters<typeof mkTask>[0] = {}, canWrite = true) {
  const task = mkTask({ id: 'tp', name: 'Minha task', status: 'todo', priority: 1, ...over });
  const board = mkBoard({ general: { task_count: 1, done_count: 0, tracked_seconds: 0, tasks: [task] } });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <TaskPanel
        wsId="w1"
        projectId="p1"
        taskId="tp"
        board={board}
        canWrite={canWrite}
        onClose={vi.fn()}
        onChanged={vi.fn()}
      />
    </QueryClientProvider>,
  );
  return task;
}

beforeEach(() => vi.clearAllMocks());

describe('TaskPanel (CA-20/21/22/26)', () => {
  it('abre com título, props, descrição e sidebar', () => {
    renderPanel();
    expect(screen.getByLabelText('Título da task')).toHaveValue('Minha task');
    expect(screen.getByLabelText('Status')).toBeInTheDocument();
    expect(screen.getByLabelText('Descrição da task')).toBeInTheDocument();
    expect(screen.getByTestId('subtask-sidebar')).toBeInTheDocument();
  });

  it('Enter no título move o foco pro textarea de descrição (CA-21)', () => {
    renderPanel();
    const title = screen.getByLabelText('Título da task');
    title.focus();
    fireEvent.keyDown(title, { key: 'Enter' });
    expect(screen.getByLabelText('Descrição da task')).toHaveFocus();
  });

  it('Status: dropdown minimalista → escolher um estado faz PATCH (CA-20, fase 31)', () => {
    renderPanel({ status: 'todo' });
    fireEvent.click(screen.getByLabelText('Status')); // abre o dropdown
    fireEvent.click(screen.getByRole('option', { name: 'In progress' }));
    expect(patchSpy).toHaveBeenCalledWith(
      { id: 'tp', patch: { status: 'doing' } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('toggle expand alterna side↔full sem fechar (CA-22)', () => {
    renderPanel();
    expect(screen.getByTestId('center-card')).toHaveAttribute('data-mode', 'side');
    fireEvent.click(screen.getByTestId('center-toggle'));
    expect(screen.getByTestId('center-card')).toHaveAttribute('data-mode', 'full');
    // ainda montado (não fechou)
    expect(screen.getByLabelText('Título da task')).toBeInTheDocument();
  });

  // fase 31: a linha Time (contador/elo) saiu do popup; Priority virou dropdown
  // minimalista (escolha direta, não cicla). Pinamos o PATCH do nível escolhido.
  it('Priority: dropdown → escolher um nível faz PATCH (fase 31)', () => {
    renderPanel({ priority: 0 });
    fireEvent.click(screen.getByLabelText('Priority'));
    fireEvent.click(screen.getByRole('option', { name: 'High' }));
    expect(patchSpy).toHaveBeenCalledWith(
      { id: 'tp', patch: { priority: 3 } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('nome vazio NÃO chama a API no blur (CA-21)', () => {
    renderPanel();
    const title = screen.getByLabelText('Título da task');
    fireEvent.change(title, { target: { value: '   ' } });
    fireEvent.blur(title);
    expect(patchSpy).not.toHaveBeenCalled();
  });

  it('can_write=false: chips não ciclam, sem Start, sem add de sub-task (CA-25/30 CA-14)', () => {
    renderPanel({}, false);
    fireEvent.click(screen.getByLabelText('Status'));
    expect(patchSpy).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Status')).toBeDisabled();
    // fase 30 CA-14: viewer (idle) não vê o Start — não pode cronometrar a task.
    expect(screen.queryByTestId('task-start')).toBeNull();
    expect(screen.queryByLabelText('Adicionar sub-task')).not.toBeInTheDocument();
  });

  // CA-28.1: a thread herda canCommentOn (NÃO canWrite) — o wiring TaskPanel→TaskThread
  // é o ponto que a migração TaskModal→TaskPanel poderia ter quebrado. Estes 3 casos
  // matam a mutação `canComment = canWrite` (o caso do meio é a prova).
  it('canWrite=true → a thread habilita comentar', () => {
    renderPanel({}, true);
    expect(screen.getByTestId('thread-stub')).toHaveAttribute('data-can-comment', 'true');
  });

  it('viewer (canWrite=false) que É assignee → thread AINDA habilita comentar (carve-out)', () => {
    renderPanel({ assignee: 'u1' }, false); // u1 = currentUserId mockado
    expect(screen.getByTestId('thread-stub')).toHaveAttribute('data-can-comment', 'true');
  });

  it('viewer (canWrite=false) NÃO-assignee → thread bloqueia comentar', () => {
    renderPanel({ assignee: 'outro' }, false);
    expect(screen.getByTestId('thread-stub')).toHaveAttribute('data-can-comment', 'false');
  });

  // fase 31: workspace solo (1 membro) NÃO mostra comentários — planejamento limpo.
  it('solo (1 membro) oculta a thread de comentários', () => {
    membersMock.current = [{ user_id: 'u1', name: 'Eu' }];
    try {
      renderPanel();
      expect(screen.queryByTestId('thread-stub')).toBeNull();
    } finally {
      membersMock.current = [{ user_id: 'u1', name: 'Eu' }, { user_id: 'u2', name: 'Colega' }];
    }
  });
});
