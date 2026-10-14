// wakeLock.spec — T-02 da fase 05 (CA-14/15/16): navigator.wakeLock
// stubado (defineProperty), sentinel fake com release() espiável e promise
// controlada pro caso (f) da corrida do desmonte (release-on-arrival, D-04).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useWakeLock } from './wakeLock';

type FakeSentinel = {
  released: boolean;
  release: ReturnType<typeof vi.fn>;
};

function makeSentinel(): FakeSentinel {
  const s: FakeSentinel = {
    released: false,
    release: vi.fn(async () => {
      s.released = true;
    }),
  };
  return s;
}

let request: ReturnType<typeof vi.fn>;

beforeEach(() => {
  request = vi.fn(async () => makeSentinel());
  Object.defineProperty(navigator, 'wakeLock', {
    value: { request },
    configurable: true,
  });
});

afterEach(() => {
  Reflect.deleteProperty(navigator, 'wakeLock');
});

const flush = () => act(async () => {});

describe('useWakeLock', () => {
  // (a) CA-14 — montar requisita o wake lock de tela exatamente 1 vez
  it('montar chama request("screen") exatamente 1 vez', async () => {
    renderHook(() => useWakeLock());
    await flush();

    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith('screen');
  });

  // (b) CA-14 — desmontar libera o sentinel e não deixa listener vazado
  it('desmontar libera o sentinel e remove o listener de visibilidade', async () => {
    const sentinel = makeSentinel();
    request.mockResolvedValue(sentinel);

    const { unmount } = renderHook(() => useWakeLock());
    await flush();
    unmount();
    await flush();

    expect(sentinel.release).toHaveBeenCalledTimes(1);

    // listener removido: visibilitychange depois do desmonte não re-adquire
    document.dispatchEvent(new Event('visibilitychange'));
    await flush();
    expect(request).toHaveBeenCalledTimes(1);
  });

  // (c) CA-15 — sentinel liberado pelo sistema + página visível de novo →
  // re-adquire
  it('re-adquire no visibilitychange quando o sistema soltou o wake lock', async () => {
    const sentinel = makeSentinel();
    request.mockResolvedValueOnce(sentinel);

    renderHook(() => useWakeLock());
    await flush();
    expect(request).toHaveBeenCalledTimes(1);

    // o sistema liberou sozinho (tela bloqueada); a página volta a aparecer
    sentinel.released = true;
    document.dispatchEvent(new Event('visibilitychange'));
    await flush();

    expect(request).toHaveBeenCalledTimes(2);
  });

  // sentinel ATIVO + visibilitychange → NÃO re-adquire (1 sentinel por vez)
  it('não re-adquire se o sentinel segue ativo', async () => {
    renderHook(() => useWakeLock());
    await flush();

    document.dispatchEvent(new Event('visibilitychange'));
    await flush();

    expect(request).toHaveBeenCalledTimes(1);
  });

  // (d) CA-16 — API ausente: monta/desmonta sem erro e sem request
  it('navigator.wakeLock indefinido é no-op silencioso', async () => {
    Reflect.deleteProperty(navigator, 'wakeLock');

    const { unmount } = renderHook(() => useWakeLock());
    await flush();
    unmount();

    expect(request).not.toHaveBeenCalled();
  });

  // (e) CA-16 — request rejeitado (ex.: economia de bateria): sem erro e
  // sem retry em loop — 1 tentativa por gatilho
  it('request rejeitando é silêncio, sem retry em loop', async () => {
    request.mockRejectedValue(new Error('denied'));

    const { unmount } = renderHook(() => useWakeLock());
    await flush();
    await flush();

    expect(request).toHaveBeenCalledTimes(1); // só a tentativa do mount

    unmount(); // desmontar sem sentinel não explode
    await flush();
  });

  // (f) corrida do desmonte (D-04, verificação adversarial das specs):
  // desmontar com o request AINDA pendente → quando o sentinel chegar,
  // release() é chamado nele (release-on-arrival) — sem wake lock zumbi
  it('desmonte antes do request resolver libera o sentinel quando ele chega', async () => {
    let resolveRequest: (s: FakeSentinel) => void = () => {};
    request.mockImplementation(
      () =>
        new Promise<FakeSentinel>((resolve) => {
          resolveRequest = resolve;
        }),
    );

    const { unmount } = renderHook(() => useWakeLock());
    expect(request).toHaveBeenCalledTimes(1);

    unmount(); // request ainda pendente

    const sentinel = makeSentinel();
    resolveRequest(sentinel);
    await flush();

    expect(sentinel.release).toHaveBeenCalledTimes(1);
  });
});
