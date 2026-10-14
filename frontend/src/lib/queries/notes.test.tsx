// notes.test.tsx — fase 26 T-07: o cliente bate nos paths certos das notas
// (incl. ?project_id=) e os hooks de mutação invalidam notesKey; o link /task
// invalida TAMBÉM o boardKey (a task aparece em Tasks — CA-23).
import { describe, it, expect, vi, afterEach, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { listNotes, createNote, patchNote, deleteNote, createLink, deleteLink } from '../api';
import { boardKey } from './objectives';
import {
  notesKey,
  useCreateNote,
  usePatchNote,
  useDeleteNote,
  useCreateLink,
  useDeleteLink,
} from './notes';

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

describe('lib/api notas (T-07)', () => {
  it('listNotes desembrulha { notes } e passa ?project_id=', async () => {
    const fetchMock = stubFetch(200, { notes: [{ id: 'n1', title: 'A' }] });
    const list = await listNotes('w1', 'p1');
    expect(list).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/notes?project_id=p1',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('listNotes sem projectId vai pro path de soltas (sem query)', async () => {
    const fetchMock = stubFetch(200, { notes: [] });
    await listNotes('w1');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/notes',
      expect.anything(),
    );
  });

  it('createNote POSTa o input no path de notas', async () => {
    const fetchMock = stubFetch(201, { id: 'n1' });
    await createNote('w1', { project_id: 'p1', title: 'T' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/notes');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ project_id: 'p1', title: 'T' });
  });

  it('patchNote PATCHa por id', async () => {
    const fetchMock = stubFetch(200, { id: 'n1', body_md: 'x' });
    await patchNote('w1', 'n1', { body_md: 'x' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/notes/n1');
    expect(init.method).toBe('PATCH');
  });

  it('deleteNote DELETa por id (204)', async () => {
    const fetchMock = stubFetch(204);
    await expect(deleteNote('w1', 'n1')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/notes/n1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });

  it('createLink POSTa em /notes/{id}/links', async () => {
    const fetchMock = stubFetch(201, { link: { id: 'l1' }, task: { id: 't1' } });
    await createLink('w1', 'n1', { kind: 'task', name: 'X' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/workspaces/w1/notes/n1/links');
    expect(JSON.parse(init.body)).toEqual({ kind: 'task', name: 'X' });
  });

  it('deleteLink DELETa por {linkId}', async () => {
    const fetchMock = stubFetch(204);
    await deleteLink('w1', 'n1', 'l1');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/notes/n1/links/l1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});

describe('hooks de nota invalidam notesKey (T-07)', () => {
  function wrapperWith(qc: QueryClient) {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  it.each([
    ['create', () => useCreateNote('w1', 'p1'), () => ({ title: 'x' })],
    ['patch', () => usePatchNote('w1', 'p1'), () => ({ id: 'n1', input: { title: 'x' } })],
    ['delete', () => useDeleteNote('w1', 'p1'), () => 'n1'],
  ] as const)('%s invalida notesKey(w1,p1)', async (_n, useHook, makeVars) => {
    stubFetch(200, { id: 'n1' });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useHook(), { wrapper: wrapperWith(qc) });
    await result.current.mutateAsync(makeVars() as never);
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: notesKey('w1', 'p1') }),
    );
  });

  it('link /task invalida notesKey E boardKey (CA-23)', async () => {
    stubFetch(201, { link: { id: 'l1' }, task: { id: 't1', project_id: 'p1' } });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useCreateLink('w1', 'p1', 'n1'), {
      wrapper: wrapperWith(qc),
    });
    await result.current.mutateAsync({ kind: 'task', name: 'X' });
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: notesKey('w1', 'p1') });
      expect(invalidate).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
    });
  });

  it('link /note NÃO invalida boardKey (só a nota)', async () => {
    stubFetch(201, { link: { id: 'l2' }, note: { id: 'n2' } });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useCreateLink('w1', 'p1', 'n1'), {
      wrapper: wrapperWith(qc),
    });
    await result.current.mutateAsync({ kind: 'note', title: 'sub' });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: notesKey('w1', 'p1') }),
    );
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
  });

  it('deleteLink invalida notesKey', async () => {
    stubFetch(204);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => useDeleteLink('w1', 'p1', 'n1'), {
      wrapper: wrapperWith(qc),
    });
    await result.current.mutateAsync('l1');
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: notesKey('w1', 'p1') }),
    );
  });
});
