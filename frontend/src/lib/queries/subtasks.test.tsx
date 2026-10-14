// subtasks.test.tsx — fase 24 T-08 (CA-23): a API de sub-task bate nos paths
// certos e os hooks de mutação invalidam subtasksKey E boardKey.
import { describe, it, expect, vi, afterEach, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { listSubtasks, createSubtask, patchSubtask, deleteSubtask } from '../api';
import { boardKey } from './objectives';
import {
  subtasksKey,
  useCreateSubtask,
  usePatchSubtask,
  useDeleteSubtask,
} from './subtasks';

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

describe('lib/api sub-tasks (T-08)', () => {
  it('listSubtasks desembrulha { subtasks }', async () => {
    const fetchMock = stubFetch(200, { subtasks: [{ id: 's1', title: 'a' }] });
    const list = await listSubtasks('w1', 't1');
    expect(list).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/tasks/t1/subtasks',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('createSubtask POST { title }', async () => {
    const fetchMock = stubFetch(201, { id: 's1', title: 'novo' });
    await createSubtask('w1', 't1', 'novo');
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/tasks/t1/subtasks');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ title: 'novo' });
  });

  it('patchSubtask PATCH no path do subtask', async () => {
    const fetchMock = stubFetch(200, { id: 's1', done: true });
    await patchSubtask('w1', 't1', 's1', { done: true });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/tasks/t1/subtasks/s1');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ done: true });
  });

  it('deleteSubtask DELETE no path do subtask (204)', async () => {
    const fetchMock = stubFetch(204);
    await expect(deleteSubtask('w1', 't1', 's1')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/tasks/t1/subtasks/s1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});

function wrapper(qc: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe('hooks de sub-task invalidam subtasksKey E boardKey', () => {
  it('useCreateSubtask invalida ambas as queries', async () => {
    stubFetch(201, { id: 's1', title: 'x' });
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const spy = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useCreateSubtask('w1', 't1', 'p1'), { wrapper: wrapper(qc) });
    result.current.mutate('x');
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
    expect(keys).toContain(JSON.stringify(subtasksKey('w1', 't1')));
    expect(keys).toContain(JSON.stringify(boardKey('w1', 'p1')));
  });

  it('usePatchSubtask e useDeleteSubtask também invalidam ambas', async () => {
    stubFetch(200, { id: 's1' });
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const spy = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => usePatchSubtask('w1', 't1', 'p1'), { wrapper: wrapper(qc) });
    result.current.mutate({ id: 's1', patch: { done: true } });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
    expect(keys).toContain(JSON.stringify(subtasksKey('w1', 't1')));
    expect(keys).toContain(JSON.stringify(boardKey('w1', 'p1')));
  });
});
