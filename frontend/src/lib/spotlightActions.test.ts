// spotlightActions.test.ts — T-03/T-04 (CA-09..18): a casca de orquestração.
// Mocka ./api (createTask/createNote/createObjective) e espia o timerStore
// (start/editCurrent) — afirma QUAL fluxo foi chamado com QUAIS args.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { runSpot, type SpotCtx, type SpotDeps } from './spotlightActions';
import { useTimerStore, initialTimerState } from './timerStore';
import type { TimerState } from './api';
import * as api from './api';

vi.mock('./api', async (orig) => {
  const real = await orig<typeof import('./api')>();
  return {
    ...real,
    createTask: vi.fn(async () => ({ id: 't1' })),
    createNote: vi.fn(async () => ({ id: 'n1' })),
    createObjective: vi.fn(async () => ({ id: 'o1' })),
  };
});

const WS = 'w1';
const PID = 'p1';

function ctx(over: Partial<SpotCtx> = {}): SpotCtx {
  return { ws: WS, projectId: PID, canWrite: true, ...over };
}

let start: ReturnType<typeof vi.fn>;
let editCurrent: ReturnType<typeof vi.fn>;
let deps: SpotDeps & {
  close: ReturnType<typeof vi.fn>;
  navigate: ReturnType<typeof vi.fn>;
  notify: ReturnType<typeof vi.fn>;
};

function runningEntry(): TimerState {
  return {
    mode: 'countup',
    entry: {
      id: 'e1',
      workspace_id: WS,
      project_id: null,
      task_id: null,
      description: 'foco',
      tags: [],
      started_at: '2026-06-10T15:00:00Z',
      stopped_at: null,
      duration_seconds: null,
      source: 'timer',
    },
    elapsed_seconds: 0,
    target_seconds: null,
    remaining_seconds: null,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  start = vi.fn(async () => undefined);
  editCurrent = vi.fn(async () => undefined);
  useTimerStore.setState({ start, editCurrent });
  deps = {
    close: vi.fn(),
    navigate: vi.fn(),
    notify: vi.fn(),
  };
});

describe('runSpot — comandos com entidade', () => {
  it('/task X cria a task no ws/projeto certos e fecha (CA-09/18)', async () => {
    await runSpot('/task Comprar domínio', ctx(), deps);
    expect(api.createTask).toHaveBeenCalledWith(WS, { project_id: PID, name: 'Comprar domínio', status: 'backlog' });
    expect(deps.close).toHaveBeenCalled();
    // sem abridor global → fallback navega pro projeto (D-09/CA-17)
    expect(deps.navigate).toHaveBeenCalledWith(`/projetos/${PID}`);
    expect(start).not.toHaveBeenCalled();
  });

  it('/task X (com nome) abre o popup com o CALENDÁRIO de Due pra agendar (fase 31)', async () => {
    const openTask = vi.fn();
    await runSpot('/task X', ctx(), { ...deps, openTask });
    expect(openTask).toHaveBeenCalledWith('t1', { scheduleOnOpen: true }); // nome dado → agenda
    expect(deps.navigate).not.toHaveBeenCalled();
  });

  it('/task SEM nome abre o popup FOCADO no título pra digitar (fase 31)', async () => {
    const openTask = vi.fn();
    await runSpot('/task', ctx(), { ...deps, openTask });
    expect(openTask).toHaveBeenCalledWith('t1', { focusTitle: true }); // sem nome → foca título
    expect(deps.navigate).not.toHaveBeenCalled();
  });

  it('/note X cria a nota e abre/navega (CA-10)', async () => {
    await runSpot('/note Reunião', ctx(), deps);
    expect(api.createNote).toHaveBeenCalledWith(WS, { project_id: PID, title: 'Reunião' });
    expect(deps.close).toHaveBeenCalled();
  });

  it('/goal X cria o objetivo via createObjective (CA-12)', async () => {
    await runSpot('/goal Lançar v2', ctx(), deps);
    expect(api.createObjective).toHaveBeenCalledWith(WS, PID, { name: 'Lançar v2' });
    expect(deps.close).toHaveBeenCalled();
  });

  it('/task sem projeto de contexto → NÃO chama API, avisa, mantém aberto (CA-15)', async () => {
    await runSpot('/task X', ctx({ projectId: null }), deps);
    expect(api.createTask).not.toHaveBeenCalled();
    expect(deps.close).not.toHaveBeenCalled();
    expect(deps.notify).toHaveBeenCalled();
  });

  it('/task com canWrite=false → NÃO chama API (CA-16)', async () => {
    await runSpot('/task X', ctx({ canWrite: false }), deps);
    expect(api.createTask).not.toHaveBeenCalled();
    expect(deps.notify).toHaveBeenCalled();
  });

  it('createTask rejeita → runSpot resolve, não quebra (CA-17)', async () => {
    vi.mocked(api.createTask).mockRejectedValueOnce(new Error('boom'));
    await expect(runSpot('/task X', ctx(), deps)).resolves.toBeUndefined();
    expect(deps.notify).toHaveBeenCalled();
  });
});

describe('runSpot — /timer', () => {
  it('/timer (com título) inicia count-up; NÃO chama createTask (CA-11)', async () => {
    await runSpot('/timer Estudar', ctx(), deps);
    expect(start).toHaveBeenCalledWith({ workspace_id: WS, mode: 'countup', description: 'Estudar', project_id: PID });
    expect(api.createTask).not.toHaveBeenCalled();
    expect(deps.close).toHaveBeenCalled();
  });
});

describe('runSpot — sem comando (RF-19)', () => {
  it('timer RODANDO + texto → editCurrent (NÃO start/createTask), fecha (CA-13)', async () => {
    useTimerStore.setState({ state: runningEntry(), start, editCurrent });
    await runSpot('Estudar Go avançado', ctx(), deps);
    expect(editCurrent).toHaveBeenCalledWith({ description: 'Estudar Go avançado', project_id: PID });
    expect(start).not.toHaveBeenCalled();
    expect(api.createTask).not.toHaveBeenCalled();
    expect(deps.close).toHaveBeenCalled();
  });

  it('timer RODANDO + texto SEM projeto de contexto → editCurrent só com descrição', async () => {
    useTimerStore.setState({ state: runningEntry(), start, editCurrent });
    await runSpot('só rótulo', ctx({ projectId: null }), deps);
    expect(editCurrent).toHaveBeenCalledWith({ description: 'só rótulo', project_id: undefined });
  });

  it('timer RODANDO + vazio → no-op (não apaga a descrição), mantém aberto (CA-13)', async () => {
    useTimerStore.setState({ state: runningEntry(), start, editCurrent });
    await runSpot('   ', ctx(), deps);
    expect(editCurrent).not.toHaveBeenCalled();
    expect(deps.close).not.toHaveBeenCalled();
  });

  it('OCIOSO + texto → start count-up (CA-14)', async () => {
    await runSpot('Estudar', ctx(), deps);
    expect(start).toHaveBeenCalledWith({ workspace_id: WS, mode: 'countup', description: 'Estudar', project_id: PID });
    expect(editCurrent).not.toHaveBeenCalled();
    expect(deps.close).toHaveBeenCalled();
  });

  it('OCIOSO + vazio → no-op, mantém aberto (CA-14)', async () => {
    await runSpot('   ', ctx(), deps);
    expect(start).not.toHaveBeenCalled();
    expect(deps.close).not.toHaveBeenCalled();
  });

  it('editCurrent rejeita → runSpot resolve (CA-17)', async () => {
    editCurrent.mockRejectedValueOnce(new Error('boom'));
    useTimerStore.setState({ state: runningEntry(), start, editCurrent });
    await expect(runSpot('texto', ctx(), deps)).resolves.toBeUndefined();
  });
});
