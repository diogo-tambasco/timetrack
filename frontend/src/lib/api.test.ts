import { describe, it, expect, vi, afterEach } from 'vitest';
import { getMe, logout, ApiError } from './api';

afterEach(() => {
  vi.unstubAllGlobals();
});

// T-13: cliente de API com credentials include e envelope de erro
// {"error":{code,message}} virando exceção tipada.
describe('lib/api', () => {
  it('api_getMeComCredenciais', async () => {
    const me = {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: 'w1', name: 'Workspace de Ana', role: 'owner' }],
    };
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(me), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const result = await getMe();

    expect(result.user.email).toBe('a@b.c');
    expect(result.workspaces[0].role).toBe('owner');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/me',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('api_erroViraExcecaoTipada', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ error: { code: 'unauthorized', message: 'sessão ausente ou expirada' } }),
            { status: 401 },
          ),
      ),
    );

    const err = await getMe().then(
      () => null,
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    expect((err as ApiError).code).toBe('unauthorized');
  });

  it('api_logout204SemCorpo', async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(logout()).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({ method: 'POST', credentials: 'include' }),
    );
  });
});
