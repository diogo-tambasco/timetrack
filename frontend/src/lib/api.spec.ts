import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  ApiError,
  listClients,
  createClient,
  archiveClient,
  createProject,
  listTasks,
  type Client,
  type Task,
} from './api';

afterEach(() => {
  vi.unstubAllGlobals();
});

const ws = 'w1';

// T-13 — extensão da lib de API da fase 01 com as chamadas da fase 02:
// sucesso tipa o payload (envelopes de lista desembrulhados) e o envelope
// de erro {"error":{code,message}} vira ApiError com code.
describe('lib/api — fase 02', () => {
  it('clients_listaTipadaComCredenciais', async () => {
    const clients: Client[] = [
      { id: 'c1', workspace_id: ws, name: 'Acme', archived_at: null, created_at: '2026-06-10T14:03:00Z' },
    ];
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ clients }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const result = await listClients(ws);

    expect(result).toHaveLength(1);
    expect(result[0].name).toBe('Acme');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/clients',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('clients_archivedTrueNaQuery', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ clients: [] }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    await listClients(ws, { archived: true });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/clients?archived=true',
      expect.anything(),
    );
  });

  it('clients_criarViaPostJSON', async () => {
    const criado: Client = {
      id: 'c2', workspace_id: ws, name: 'Lauda', archived_at: null, created_at: '2026-06-10T14:03:00Z',
    };
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(criado), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);

    const result = await createClient(ws, { name: 'Lauda' });

    expect(result.id).toBe('c2');
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('/api/v1/workspaces/w1/clients');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ name: 'Lauda' });
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json');
  });

  it('clients_arquivar204SemCorpo', async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(archiveClient(ws, 'c1')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/clients/c1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });

  it('projects_erroViraExcecaoTipada', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ error: { code: 'validation_error', message: 'color deve estar no formato #RRGGBB' } }),
            { status: 400 },
          ),
      ),
    );

    const err = await createProject(ws, { name: 'X', color: 'azul', visibility: 'shared' }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(400);
    expect((err as ApiError).code).toBe('validation_error');
  });

  it('tasks_filtrosCombinaveisNaQuery', async () => {
    const tasks: Task[] = [];
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ tasks }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    await listTasks(ws, { project_id: 'p1', status: 'doing' });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/tasks?project_id=p1&status=doing',
      expect.anything(),
    );
  });
});
