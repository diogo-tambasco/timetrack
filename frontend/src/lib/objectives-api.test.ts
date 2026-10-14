// fase 29 T-07 (CA-02/03/04): a api de objetivos passa completion_mode/target_date
// no corpo de POST/PATCH. createObjective/patchObjective repassam o input/patch
// inteiro como JSON — o teste fixa o contrato (e os tipos compilam: completion_mode
// é CompletionMode, target_date é string|null tri-state no patch).
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createObjective, patchObjective } from './api';

afterEach(() => vi.unstubAllGlobals());

function stubFetch() {
  const fetchMock = vi.fn(
    async () =>
      new Response(
        JSON.stringify({
          id: 'o1', workspace_id: 'w1', project_id: 'p1', name: 'G', description: '',
          status: 'open', goal_hours: null, completion_mode: 'by_tasks', target_date: null,
          position: 0, created_by: 'u1', created_at: '2026-06-22T00:00:00Z',
        }),
        { status: 200 },
      ),
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function bodyOf(fetchMock: ReturnType<typeof vi.fn>): Record<string, unknown> {
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  return JSON.parse(init.body as string);
}

describe('objectives api — completion_mode/target_date (fase 29 T-07)', () => {
  it('createObjective envia completion_mode + target_date no corpo', async () => {
    const fetchMock = stubFetch();
    await createObjective('w1', 'p1', {
      name: 'Demo pro cliente',
      completion_mode: 'by_date',
      target_date: '2026-07-15',
    });
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/projects/p1/objectives',
      expect.objectContaining({ method: 'POST' }),
    );
    const body = bodyOf(fetchMock);
    expect(body.completion_mode).toBe('by_date');
    expect(body.target_date).toBe('2026-07-15');
  });

  it('createObjective by_hours envia goal_hours + completion_mode', async () => {
    const fetchMock = stubFetch();
    await createObjective('w1', 'p1', {
      name: 'Bater 40h no onboarding',
      completion_mode: 'by_hours',
      goal_hours: 40,
    });
    const body = bodyOf(fetchMock);
    expect(body.completion_mode).toBe('by_hours');
    expect(body.goal_hours).toBe(40);
  });

  it('patchObjective troca o modo + limpa o marco (target_date:null tri-state)', async () => {
    const fetchMock = stubFetch();
    await patchObjective('w1', 'p1', 'o1', { completion_mode: 'by_tasks', target_date: null });
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/w1/projects/p1/objectives/o1',
      expect.objectContaining({ method: 'PATCH' }),
    );
    const body = bodyOf(fetchMock);
    expect(body.completion_mode).toBe('by_tasks');
    expect(body.target_date).toBeNull();
  });
});
