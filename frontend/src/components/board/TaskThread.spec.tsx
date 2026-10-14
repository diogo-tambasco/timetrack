import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TaskThread from './TaskThread';
import * as api from '../../lib/api';
import { useAuthStore } from '../../lib/auth';
import type { Comment, Me } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listComments: vi.fn(),
    createComment: vi.fn(),
    deleteComment: vi.fn(),
  };
});

const c = (over: Partial<Comment>): Comment => ({
  id: 'c1',
  task_id: 't1',
  workspace_id: 'w1',
  author_id: 'u1',
  author_name: 'Ana Owner',
  body: 'comentário',
  created_at: '2026-06-12T14:03:00Z',
  ...over,
});

// monta o store de auth com o user atual e a role no ws w1.
function setMe(userId: string, role: 'owner' | 'member' | 'viewer') {
  const me: Me = {
    user: { id: userId, email: 'x@b.c', name: 'X', avatar_url: null },
    workspaces: [{ id: 'w1', name: 'WS', role }],
  };
  useAuthStore.setState({ status: 'authenticated', me, activeWorkspaceId: 'w1' });
}

function renderThread(canComment = true) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TaskThread wsId="w1" taskId="t1" canComment={canComment} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createComment).mockResolvedValue(c({}) as never);
  vi.mocked(api.deleteComment).mockResolvedValue(undefined as never);
});

describe('TaskThread — lista + envio (CA-06/19.1)', () => {
  it('renderiza comentários ASC com autor, timestamp e corpo', async () => {
    vi.mocked(api.listComments).mockResolvedValue([
      c({ id: 'c1', body: 'primeiro', author_name: 'Ana' }),
      c({ id: 'c2', body: 'segundo', author_name: 'Beto', author_id: 'u2' }),
    ] as never);
    setMe('u1', 'member');
    renderThread(true);

    await waitFor(() => expect(screen.getByText('primeiro')).toBeInTheDocument());
    const itens = screen.getAllByTestId('comment');
    expect(itens).toHaveLength(2);
    // ordem ASC: o backend entrega ordenado, a UI preserva.
    expect(itens[0]).toHaveTextContent('primeiro');
    expect(itens[1]).toHaveTextContent('segundo');
    expect(screen.getByText('Ana')).toBeInTheDocument();
    expect(screen.getByText('Beto')).toBeInTheDocument();
  });

  it('enviar chama createComment com o corpo trimado', async () => {
    vi.mocked(api.listComments).mockResolvedValue([] as never);
    setMe('u1', 'member');
    renderThread(true);
    await waitFor(() => expect(screen.getByText(/nenhum comentário/i)).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/novo comentário/i), { target: { value: '  oi  ' } });
    fireEvent.click(screen.getByRole('button', { name: /comentar/i }));
    await waitFor(() => expect(api.createComment).toHaveBeenCalledWith('w1', 't1', 'oi'));
  });

  it('body vazio desabilita o botão Comentar', async () => {
    vi.mocked(api.listComments).mockResolvedValue([] as never);
    setMe('u1', 'member');
    renderThread(true);
    await waitFor(() => expect(screen.getByText(/nenhum comentário/i)).toBeInTheDocument());

    expect(screen.getByRole('button', { name: /comentar/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/novo comentário/i), { target: { value: '   ' } });
    expect(screen.getByRole('button', { name: /comentar/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/novo comentário/i), { target: { value: 'algo' } });
    expect(screen.getByRole('button', { name: /comentar/i })).toBeEnabled();
  });
});

describe('TaskThread — gating do ✕ apagar (CA-08/09 gating UI)', () => {
  it('✕ aparece só no PRÓPRIO (autor) pra um member', async () => {
    vi.mocked(api.listComments).mockResolvedValue([
      c({ id: 'c1', author_id: 'u1', body: 'meu' }),
      c({ id: 'c2', author_id: 'u2', body: 'do outro' }),
    ] as never);
    setMe('u1', 'member');
    renderThread(true);

    await waitFor(() => expect(screen.getByText('meu')).toBeInTheDocument());
    // só 1 botão de apagar (o do próprio comentário).
    expect(screen.getAllByRole('button', { name: /apagar comentário/i })).toHaveLength(1);
  });

  it('owner vê ✕ em TODOS os comentários', async () => {
    vi.mocked(api.listComments).mockResolvedValue([
      c({ id: 'c1', author_id: 'u2', body: 'do outro' }),
      c({ id: 'c2', author_id: 'u3', body: 'de mais um' }),
    ] as never);
    setMe('u1', 'owner');
    renderThread(true);

    await waitFor(() => expect(screen.getByText('do outro')).toBeInTheDocument());
    expect(screen.getAllByRole('button', { name: /apagar comentário/i })).toHaveLength(2);
  });

  it('clicar no ✕ chama deleteComment', async () => {
    vi.mocked(api.listComments).mockResolvedValue([c({ id: 'c1', author_id: 'u1' })] as never);
    setMe('u1', 'member');
    renderThread(true);
    await waitFor(() => expect(screen.getByRole('button', { name: /apagar comentário/i })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /apagar comentário/i }));
    await waitFor(() => expect(api.deleteComment).toHaveBeenCalledWith('w1', 't1', 'c1'));
  });
});

describe('TaskThread — gating viewer-assignee por can_comment (CA-19.1 nível 1)', () => {
  it('can_comment=true (viewer-assignee): input/botão DISPONÍVEL', async () => {
    vi.mocked(api.listComments).mockResolvedValue([] as never);
    setMe('uv', 'viewer'); // viewer; o ProjectBoard liga can_comment via assignee===currentUser
    renderThread(true);
    await waitFor(() => expect(screen.getByText(/nenhum comentário/i)).toBeInTheDocument());
    expect(screen.getByLabelText(/novo comentário/i)).toBeInTheDocument();
  });

  it('can_comment=false (não-assignee, can_write=false): input AUSENTE', async () => {
    vi.mocked(api.listComments).mockResolvedValue([] as never);
    setMe('uv', 'viewer');
    renderThread(false);
    await waitFor(() => expect(screen.getByText(/nenhum comentário/i)).toBeInTheDocument());
    expect(screen.queryByLabelText(/novo comentário/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /comentar/i })).not.toBeInTheDocument();
  });
});
