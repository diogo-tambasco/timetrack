// objectives.test.tsx — fase 12 T-08: o cliente de API bate nos paths certos
// e os hooks de mutação invalidam o boardKey (CA-20/21 refletem sem reload).
import { describe, it, expect, vi, afterEach, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import {
  getBoard,
  createObjective,
  patchObjective,
  deleteObjective,
  listObjectives,
} from '../api';
import {
  boardKey,
  useCreateObjective,
  usePatchObjective,
  useDeleteObjective,
} from './objectives';

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

describe('lib/api objetivos (T-08)', () => {
  it('getBoard bate no path do board', async () => {
    const fetchMock = stubFetch(200, { project: {}, objectives: [], general: {} });
    await getBoard('w1', 'p1');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/projects/p1/board',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('listObjectives desembrulha { objectives }', async () => {
    stubFetch(200, { objectives: [{ id: 'o1' }] });
    const list = await listObjectives('w1', 'p1');
    expect(list).toHaveLength(1);
  });

  it('createObjective faz POST no path de objetivos', async () => {
    const fetchMock = stubFetch(201, { id: 'o1' });
    await createObjective('w1', 'p1', { name: 'Lançar', goal_hours: 12 });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/projects/p1/objectives');
    expect(init).toMatchObject({ method: 'POST' });
    expect(JSON.parse(init.body)).toEqual({ name: 'Lançar', goal_hours: 12 });
  });

  it('patchObjective faz PATCH e null remove a meta', async () => {
    const fetchMock = stubFetch(200, { id: 'o1' });
    await patchObjective('w1', 'p1', 'o1', { goal_hours: null });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/projects/p1/objectives/o1');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ goal_hours: null });
  });

  it('deleteObjective faz DELETE (204 sem corpo)', async () => {
    const fetchMock = stubFetch(204);
    await expect(deleteObjective('w1', 'p1', 'o1')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/projects/p1/objectives/o1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});

describe('hooks de objetivo invalidam o boardKey (T-08)', () => {
  function wrapperWith(qc: QueryClient) {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  it.each([
    ['create', () => useCreateObjective('w1', 'p1'), () => ({ name: 'x' })],
    ['patch', () => usePatchObjective('w1', 'p1'), () => ({ id: 'o1', patch: { name: 'y' } })],
    ['delete', () => useDeleteObjective('w1', 'p1'), () => 'o1'],
  ] as const)('%s invalida boardKey(w1,p1)', async (_name, useHook, makeVars) => {
    stubFetch(200, { id: 'o1' });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useHook(), { wrapper: wrapperWith(qc) });
    await result.current.mutateAsync(makeVars() as never);
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') }),
    );
  });
});
