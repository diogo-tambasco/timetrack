// recent-entries.test.tsx — fase 13 T-02: useRecentEntries bate no path certo
// (from = hoje−6, to = hoje, tz do navegador), reusando o filtro from/to da
// fase 03. "Hoje" sai do MESMO new Date() do hook (sem fake timers — eles
// travam o waitFor da TanStack Query).
import { afterEach, describe, expect, it, vi, type Mock } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { browserTZ } from '../format';
import { addDaysISO, localDayISO } from '../recent';
import { useRecentEntries } from './entries';

function stubFetch(body: unknown = { entries: [], total_seconds: 0 }): Mock {
  const fetchMock = vi.fn(
    async () =>
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function wrapperWith(qc: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('useRecentEntries (CA-06)', () => {
  it('busca os últimos 7 dias (from=hoje−6, to=hoje, tz) no path de entries', async () => {
    const fetchMock = stubFetch();
    const tz = browserTZ();
    const today = localDayISO(new Date().toISOString(), tz);
    const from = addDaysISO(today, -6);

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderHook(() => useRecentEntries('w1'), { wrapper: wrapperWith(qc) });

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const url = String(fetchMock.mock.calls[0][0]);
    expect(url).toContain('/api/v1/workspaces/w1/entries?');
    expect(url).toContain(`from=${from}`);
    expect(url).toContain(`to=${today}`);
    expect(url).toContain(`tz=${encodeURIComponent(tz)}`);
    // a janela é exatamente 6 dias entre from e to (7 dias inclusivos)
    expect(addDaysISO(from, 6)).toBe(today);
  });
});
