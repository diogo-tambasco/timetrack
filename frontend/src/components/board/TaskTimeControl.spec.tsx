// TaskTimeControl.spec — fase 30 T-03..T-06 (CA-10/10.1/11/11.1/12/14/15).
// Usa o STORE REAL (zustand) + mock só do api (startTimer/stopTimer) — espelha
// o padrão do TimerBar.spec. Clicar Start/Parar passa pelo mutate real do
// timerStore → assertamos o input EXATO entregue ao startTimer (a costura).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import TaskTimeControl from './TaskTimeControl';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { TimerState, TimeEntry } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, startTimer: vi.fn(), stopTimer: vi.fn() };
});

const TASK = { id: 't1', project_id: 'p1', name: 'Definir telas', tracked_seconds: 3600 };

function mkEntry(over: Partial<TimeEntry> = {}): TimeEntry {
  return {
    id: 'e1',
    workspace_id: 'w1',
    project_id: 'p1',
    task_id: 't1',
    description: 'Definir telas',
    tags: [],
    started_at: '2026-06-23T14:00:00Z',
    stopped_at: null,
    duration_seconds: null,
    source: 'timer',
    ...over,
  };
}

function mkState(over: Partial<TimerState> = {}): TimerState {
  return {
    mode: 'countup',
    entry: mkEntry(),
    elapsed_seconds: 0,
    target_seconds: null,
    remaining_seconds: null,
    ...over,
  };
}

// anchor 5s no FUTURO → deltaSec clampa em 0 → elapsed = state.elapsed_seconds
// EXATO (determinístico, sem flake de relógio de parede).
function setRunning(state: TimerState | null): void {
  useTimerStore.setState({ ...initialTimerState, state, anchorMs: Date.now() + 5000, offsetMs: 0 });
}

// clicar dispara o mutate ASSÍNCRONO do timerStore (start/stop → apply). Flusha
// dentro de act pra não vazar update fora do test (sem warning de act).
async function click(el: HTMLElement): Promise<void> {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  vi.mocked(api.startTimer).mockResolvedValue({ state: mkState(), server_time: new Date().toISOString() });
  vi.mocked(api.stopTimer).mockResolvedValue({ state: null, server_time: new Date().toISOString() });
});

// T-03 / CA-10/10.1 — idle nesta task + canWrite: Start chama o store com os
// refs EXATOS da task (project_id+task_id da MESMA task; description = task.name).
it('CA-10/10.1: clicar Start cronometra a task com os refs exatos', async () => {
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite />);
  const btn = screen.getByTestId('task-start');
  expect(btn).toHaveAccessibleName('Iniciar');
  await click(btn);
  expect(api.startTimer).toHaveBeenCalledTimes(1);
  expect(api.startTimer).toHaveBeenCalledWith({
    workspace_id: 'w1',
    project_id: 'p1',
    task_id: 't1',
    description: 'Definir telas',
    tags: [],
  });
});

// T-03 / CA-11.1 — idle (timer parado): mostra SÓ o tracked_seconds (sem elapsed).
it('CA-11.1: idle mostra o tracked_seconds formatado (sem elapsed)', () => {
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite />);
  expect(screen.getByText('1h00')).toBeInTheDocument(); // fmtDayTotal(3600)
});

// T-03 / CA-15 — task sem project_id: Start inerte (nunca dispara requisição).
it('CA-15: Start inerte sem project_id', async () => {
  render(<TaskTimeControl task={{ ...TASK, project_id: '' }} workspaceId="w1" canWrite />);
  await click(screen.getByTestId('task-start'));
  expect(api.startTimer).not.toHaveBeenCalled();
});

// T-04 / CA-11/11.1 — rodando NESTA task: botão Parar + tempo VIVO (tracked +
// elapsed). Clicar Parar chama stop.
it('CA-11/11.1: rodando nesta task → Parar + tempo vivo (tracked+elapsed)', async () => {
  setRunning(mkState({ elapsed_seconds: 120, entry: mkEntry({ task_id: 't1' }) }));
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite />);
  const btn = screen.getByTestId('task-start');
  expect(btn).toHaveAccessibleName('Parar');
  expect(screen.getByText('1h02')).toBeInTheDocument(); // fmtDayTotal(3600+120)
  await click(btn);
  expect(api.stopTimer).toHaveBeenCalledTimes(1);
  expect(api.startTimer).not.toHaveBeenCalled();
});

// T-04 / CA-11.1 — rodando em OUTRA task: NÃO soma o elapsed dela; mostra Start.
it('CA-11.1: rodando em OUTRA task não soma o elapsed (mostra só o tracked)', () => {
  setRunning(mkState({ elapsed_seconds: 120, entry: mkEntry({ task_id: 'OUTRA' }) }));
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite />);
  expect(screen.getByTestId('task-start')).toHaveAccessibleName('Iniciar'); // Start, não Parar
  expect(screen.getByText('1h00')).toBeInTheDocument(); // tracked puro, sem +120
});

// T-05 / CA-12 — trocar de task: timer roda em OUTRA task, clicar Start nesta
// dispara start com os refs DESTA task (o backend fecha a anterior — CA-05/03).
it('CA-12: Start nesta task enquanto roda outra dispara start desta task', async () => {
  setRunning(mkState({ elapsed_seconds: 90, entry: mkEntry({ task_id: 'OUTRA', project_id: 'pX' }) }));
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite />);
  await click(screen.getByTestId('task-start'));
  expect(api.startTimer).toHaveBeenCalledWith({
    workspace_id: 'w1',
    project_id: 'p1',
    task_id: 't1',
    description: 'Definir telas',
    tags: [],
  });
});

// T-06 / CA-14 — viewer (!canWrite) + idle: SEM botão (só leitura).
it('CA-14: viewer sem timer nesta task → só leitura, sem botão', () => {
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite={false} />);
  expect(screen.queryByTestId('task-start')).toBeNull();
  expect(screen.getByText('1h00')).toBeInTheDocument(); // o tempo é sempre exibido
});

// T-06 / CA-14/D-09 — viewer + rodando NESTA task (borda): Parar NÃO some
// (não prender o timer global).
it('CA-14: viewer rodando nesta task → Parar continua visível (não prende)', async () => {
  setRunning(mkState({ elapsed_seconds: 30, entry: mkEntry({ task_id: 't1' }) }));
  render(<TaskTimeControl task={TASK} workspaceId="w1" canWrite={false} />);
  const btn = screen.getByTestId('task-start');
  expect(btn).toHaveAccessibleName('Parar');
  await click(btn);
  expect(api.stopTimer).toHaveBeenCalledTimes(1);
});
