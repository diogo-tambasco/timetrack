// useBoardTimerSync.spec — fase 30 T-08 (CA-13/30-H5): o observador da transição
// "rodando-neste-projeto → ocioso" invalida o boardKey UMA vez, mesmo com o popup
// FECHADO (o hook vive no shell, é independente do TaskTimeControl). NÃO invalida
// pra outro projeto, NÃO ao iniciar, NÃO a cada render.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useBoardTimerSync } from './useBoardTimerSync';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import { boardKey } from '../../lib/queries/objectives';
import type { TimerState } from '../../lib/api';

function runningOn(projectId: string, entryId = 'e1'): TimerState {
  return {
    mode: 'countup',
    entry: {
      id: entryId, workspace_id: 'w1', project_id: projectId, task_id: entryId,
      description: 'x', tags: [], started_at: '2026-06-23T14:00:00Z',
      stopped_at: null, duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 0, target_seconds: null, remaining_seconds: null,
  };
}

function setRunning(projectId: string | null, entryId = 'e1'): void {
  useTimerStore.setState({
    ...initialTimerState,
    state: projectId === null ? null : runningOn(projectId, entryId),
    anchorMs: Date.now(),
    offsetMs: 0,
  });
}

function wrapper(qc: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

beforeEach(() => {
  useTimerStore.setState({ ...initialTimerState });
});

describe('useBoardTimerSync', () => {
  // CA-13/30-H5 — parar o timer DESTE projeto (popup fechado) invalida o board.
  it('invalida o board ao parar timer deste projeto', () => {
    setRunning('p1');
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useBoardTimerSync('w1', 'p1'), { wrapper: wrapper(qc) });
    spy.mockClear(); // ignora o efeito de mount

    act(() => useTimerStore.setState({ ...initialTimerState, state: null })); // PARA → ocioso
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
  });

  // CA-13 — parar timer de OUTRO projeto NÃO invalida este board.
  it('NÃO invalida ao parar timer de outro projeto', () => {
    setRunning('pOUTRO');
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useBoardTimerSync('w1', 'p1'), { wrapper: wrapper(qc) });
    spy.mockClear();

    act(() => useTimerStore.setState({ ...initialTimerState, state: null }));
    expect(spy).not.toHaveBeenCalled();
  });

  // CA-12/13 (verificação adversarial) — TROCAR de task no MESMO projeto (Start
  // atômico que fecha a anterior, SEM passar por ocioso) invalida o board: a entry
  // de A fechou e seu tracked_seconds mudou. O gatilho observa a IDENTIDADE da entry.
  it('trocar de task no MESMO projeto (sem ir a ocioso) invalida o board', () => {
    setRunning('p1', 'eA'); // rodando na task A
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useBoardTimerSync('w1', 'p1'), { wrapper: wrapper(qc) });
    spy.mockClear();

    act(() => setRunning('p1', 'eB')); // Start na task B do MESMO projeto (entry troca)
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
  });

  // CA-12/13 — trocar pra OUTRO projeto invalida o board que A DEIXOU (A fechou lá).
  it('trocar pra outro projeto invalida o board do projeto que a entry deixou', () => {
    setRunning('p1', 'eA');
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useBoardTimerSync('w1', 'p1'), { wrapper: wrapper(qc) });
    spy.mockClear();

    act(() => setRunning('p2', 'eB')); // troca p1 → p2 (A fechou em p1)
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
  });

  // CA-13 — iniciar nesta task NÃO invalida (só ao parar); sem loop por render.
  it('iniciar não invalida e re-renders sem mudança não disparam (sem loop)', () => {
    setRunning(null); // idle
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    const { rerender } = renderHook(() => useBoardTimerSync('w1', 'p1'), { wrapper: wrapper(qc) });
    spy.mockClear();

    act(() => setRunning('p1')); // começa a rodar neste projeto
    expect(spy).not.toHaveBeenCalled(); // iniciar não invalida

    rerender();
    rerender();
    expect(spy).not.toHaveBeenCalled(); // sem loop de re-render

    act(() => useTimerStore.setState({ ...initialTimerState, state: null })); // PARA
    expect(spy).toHaveBeenCalledTimes(1); // invalida UMA vez
    expect(spy).toHaveBeenCalledWith({ queryKey: boardKey('w1', 'p1') });
  });
});
