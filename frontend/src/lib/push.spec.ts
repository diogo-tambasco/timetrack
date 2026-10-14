// push.spec — fase 16 T-11 (CA-13/14): enablePush pede permissão → subscribe
// → POST com endpoint/keys certos; negada → sem POST; sem PushManager → no-op
// sem throw. Mocks de Notification/serviceWorker/PushManager + fetch.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { enablePush, pushSupported } from './push';

// VAPID public key de TESTE em base64url (não é real — só exercita o decode).
const TEST_VAPID = 'BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1Xbjhe7gO2bGm0CRrlV3pXk';

function mockFetchOk() {
  return vi.fn(async () =>
    new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'content-type': 'application/json' } }),
  );
}

beforeEach(() => {
  vi.unstubAllGlobals();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('pushSupported', () => {
  it('false quando faltam PushManager/Notification', () => {
    vi.stubGlobal('navigator', {}); // sem serviceWorker
    expect(pushSupported()).toBe(false);
  });
});

describe('enablePush', () => {
  function setupSupported(opts: {
    permission: NotificationPermission;
    subEndpoint?: string;
    keyResponse?: { key: string };
  }) {
    const subscribe = vi.fn(async () => ({
      endpoint: opts.subEndpoint ?? 'https://push.example/sub-abc',
      toJSON: () => ({
        endpoint: opts.subEndpoint ?? 'https://push.example/sub-abc',
        keys: { p256dh: 'p256dh-xyz', auth: 'auth-xyz' },
      }),
    }));
    vi.stubGlobal('navigator', {
      serviceWorker: { ready: Promise.resolve({ pushManager: { subscribe } }) },
    });
    vi.stubGlobal('window', { PushManager: function () {}, Notification: function () {} });
    vi.stubGlobal('Notification', {
      requestPermission: vi.fn(async () => opts.permission),
    });

    const keyBody = opts.keyResponse ?? { key: TEST_VAPID };
    const fetchMock = vi.fn(async (url: string) => {
      if (String(url).includes('/push/key')) {
        return new Response(JSON.stringify(keyBody), { status: 200, headers: { 'content-type': 'application/json' } });
      }
      return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'content-type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    vi.stubGlobal('atob', (s: string) => Buffer.from(s, 'base64').toString('binary'));
    return { subscribe, fetchMock };
  }

  it('permissão concedida → subscribe → POST com endpoint/keys', async () => {
    const { subscribe, fetchMock } = setupSupported({ permission: 'granted' });

    const res = await enablePush();
    expect(res).toBe('subscribed');
    expect(subscribe).toHaveBeenCalledOnce();

    // achar a chamada de POST /push/subscribe
    const post = fetchMock.mock.calls.find(([url]) => String(url).includes('/push/subscribe'));
    expect(post).toBeTruthy();
    const body = JSON.parse((post![1] as RequestInit).body as string);
    expect(body.endpoint).toBe('https://push.example/sub-abc');
    expect(body.keys).toEqual({ p256dh: 'p256dh-xyz', auth: 'auth-xyz' });
  });

  it('permissão negada → não assina nem POSTa', async () => {
    const { subscribe, fetchMock } = setupSupported({ permission: 'denied' });

    const res = await enablePush();
    expect(res).toBe('denied');
    expect(subscribe).not.toHaveBeenCalled();
    const post = fetchMock.mock.calls.find(([url]) => String(url).includes('/push/subscribe'));
    expect(post).toBeUndefined();
  });

  it('servidor sem VAPID (key vazia) → no-key, sem subscribe', async () => {
    const { subscribe } = setupSupported({ permission: 'granted', keyResponse: { key: '' } });
    const res = await enablePush();
    expect(res).toBe('no-key');
    expect(subscribe).not.toHaveBeenCalled();
  });

  it('sem PushManager → unsupported, sem throw e sem fetch', async () => {
    vi.stubGlobal('navigator', {}); // sem serviceWorker
    const fetchMock = mockFetchOk();
    vi.stubGlobal('fetch', fetchMock);

    const res = await enablePush();
    expect(res).toBe('unsupported');
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
