// comments.test.tsx — fase 19 T-07: o cliente de API bate nos paths certos da
// thread e os hooks de mutação invalidam o commentsKey (CA-06; aparecem sem
// reload).
import { describe, it, expect, vi, afterEach, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { listComments, createComment, deleteComment } from '../api';
import { commentsKey, useCreateComment, useDeleteComment } from './comments';

afterEach(() => vi.unstubAllGlobals());

function stubFetch(status = 200, body: unknown = {}): Mock {
  const fetchMock = vi.fn(
    async () =>
      new Response(status === 204 ? null : JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('lib/api comentários (T-07)', () => {
  it('listComments desembrulha { comments }', async () => {
    const fetchMock = stubFetch(200, { comments: [{ id: 'c1', body: 'oi' }] });
    const list = await listComments('w1', 't1');
    expect(list).toHaveLength(1);
    expect(list[0].body).toBe('oi');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/tasks/t1/comments',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('createComment faz POST com { body } no path da thread', async () => {
    const fetchMock = stubFetch(201, { id: 'c1', body: 'novo' });
    await createComment('w1', 't1', 'novo');
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/tasks/t1/comments');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ body: 'novo' });
  });

  it('deleteComment faz DELETE no path do comentário (204 sem corpo)', async () => {
    const fetchMock = stubFetch(204);
    await expect(deleteComment('w1', 't1', 'c1')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/tasks/t1/comments/c1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});

describe('hooks de comentário invalidam o commentsKey (T-07)', () => {
  function wrapperWith(qc: QueryClient) {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  it.each([
    ['create', () => useCreateComment('w1', 't1'), () => 'corpo'],
    ['delete', () => useDeleteComment('w1', 't1'), () => 'c1'],
  ] as const)('%s invalida commentsKey(w1,t1)', async (_name, useHook, makeVars) => {
    stubFetch(200, { id: 'c1' });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useHook(), { wrapper: wrapperWith(qc) });
    await result.current.mutateAsync(makeVars() as never);
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: commentsKey('w1', 't1') }),
    );
  });
});
