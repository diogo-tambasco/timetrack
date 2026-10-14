// projects-api.test.tsx — fase 28 T-07: o cliente de API bate nos paths do
// summary e do calendário do projeto; o patch de project invalida a query de
// summary + o board (CA-04/10/16).
import { describe, it, expect, vi, afterEach, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { getProjectsSummary, getProjectCalendar } from '../api';
import { boardKey } from './objectives';
import { projectsSummaryKey, usePatchProject } from './projects';

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

describe('lib/api summary + calendar (T-07)', () => {
  it('getProjectsSummary bate em /projects/summary com tz', async () => {
    const fetchMock = stubFetch(200, { projects: [{ id: 'p1' }] });
    const list = await getProjectsSummary('w1', { tz: 'America/Sao_Paulo' });
    expect(list).toHaveLength(1);
    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/api/v1/workspaces/w1/projects/summary');
    expect(url).toContain('tz=America%2FSao_Paulo');
  });

  it('getProjectsSummary inclui archived=true quando pedido', async () => {
    const fetchMock = stubFetch(200, { projects: [] });
    await getProjectsSummary('w1', { archived: true });
    expect(fetchMock.mock.calls[0][0]).toContain('archived=true');
  });

  it('getProjectCalendar bate em /projects/{id}/calendar com month', async () => {
    const fetchMock = stubFetch(200, { items: [], from: '2025-06-01', to: '2025-06-30' });
    const cal = await getProjectCalendar('w1', 'p1', { month: '2025-06', tz: 'UTC' });
    expect(cal.from).toBe('2025-06-01');
    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/api/v1/workspaces/w1/projects/p1/calendar');
    expect(url).toContain('month=2025-06');
  });
});

describe('usePatchProject invalida summary + board (T-07)', () => {
  function wrapperWith(qc: QueryClient) {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
  }

  it('invalida projectsSummaryKey e boardKey do projeto editado', async () => {
    stubFetch(200, { id: 'p1' });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, 'invalidateQueries');
    const { result } = renderHook(() => usePatchProject('w1'), { wrapper: wrapperWith(qc) });
    await result.current.mutateAsync({ id: 'p1', patch: { deadline: '2025-09-30' } });
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: projectsSummaryKey('w1') });
      expect(invalidate).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
    });
  });
});
