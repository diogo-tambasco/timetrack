// BreakOverlay.spec — T-16: pausar pela UI troca pro modo descanso com
// contagem regressiva e retomar/pular (CA-32); pausa zerada de
// countup/countdown NÃO retoma sozinha e vira "pausa encerrada" (CA-34.2).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TimerBar from './TimerBar';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { TimerResult, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listProjects: vi.fn(),
    listTasks: vi.fn(),
    pauseTimer: vi.fn(),
    resumeTimer: vi.fn(),
    skipBreak: vi.fn(),
    stopTimer: vi.fn(),
  };
});

const ws = 'w1';

function rodando(): TimerState {
  return {
    mode: 'countup',
    entry: {
      id: 'e1', workspace_id: ws, project_id: null, task_id: null,
      description: 'foco', tags: [], started_at: '2026-06-10T14:00:00Z',
      stopped_at: null, duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 1500,
    target_seconds: null,
    remaining_seconds: null,
  };
}

function pausa(
  remaining: number,
  resumeMode: 'pomodoro_focus' | 'countup',
  breakKind?: 'short' | 'long',
): TimerState {
  return {
    mode: 'break',
    entry: null,
    elapsed_seconds: 900 - remaining,
    target_seconds: 900,
    remaining_seconds: remaining,
    resume: { entry_id: 'e1', mode: resumeMode, target_seconds: null },
    ...(breakKind ? { break_kind: breakKind } : {}),
  };
}

function renderBar() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TimerBar />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listProjects).mockResolvedValue([]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
});

describe('BreakOverlay', () => {
  // CA-32 — pausar pela UI: a barra troca pro modo descanso com a contagem
  // regressiva da pausa e as ações retomar / pular pausa.
  it('aposPause', async () => {
    // server_time criado NA CHAMADA (não no início do teste): ancorado ao
    // relógio real, >1s de setup sob carga fazia o derive mostrar 14:59 e o
    // findByText('15:00') estourar — flake latente pego na fase 05; edição
    // SÓ de setup, asserções intactas (precedente da exceção da fase 04)
    vi.mocked(api.pauseTimer).mockImplementation(
      async (): Promise<TimerResult> => ({
        state: pausa(900, 'countup'),
        server_time: new Date().toISOString(),
      }),
    );
    useTimerStore.setState({ state: rodando(), anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    fireEvent.click(screen.getByRole('button', { name: /pausar/i }));

    expect(await screen.findByText('15:00')).toBeInTheDocument(); // 900s regressivos
    expect(screen.getByText(/pausa pomodoro/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retomar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pular pausa/i })).toBeInTheDocument();
    expect(api.pauseTimer).toHaveBeenCalledTimes(1);
  });

  // CA-34.2 — pausa ZERADA com contexto countup: nenhum resume automático;
  // estado "pausa encerrada" com retomar/parar.
  it('pausaEncerrada', async () => {
    useTimerStore.setState({ state: pausa(0, 'countup'), anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    expect(screen.getByText(/pausa encerrada/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retomar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /parar/i })).toBeInTheDocument();
    expect(api.resumeTimer).not.toHaveBeenCalled();
    expect(api.skipBreak).not.toHaveBeenCalled();
  });

  // CA-16 (fase 15) — break_kind 'long' rotula "Pausa longa" (alinha o
  // cockpit ao FocoBreak). Nenhum comportamento de timer muda.
  it('pausa longa mostra "Pausa longa" (CA-16)', () => {
    useTimerStore.setState({
      state: pausa(900, 'pomodoro_focus', 'long'),
      anchorMs: Date.now(),
      offsetMs: 0,
    });
    renderBar();
    expect(screen.getByText(/pausa longa/i)).toBeInTheDocument();
    expect(screen.queryByText(/pausa pomodoro/i)).toBeNull();
  });

  // CA-16 — break_kind 'short' (ou ausente) mantém "Pausa Pomodoro".
  it('pausa curta mantém "Pausa Pomodoro" (CA-16)', () => {
    useTimerStore.setState({
      state: pausa(300, 'pomodoro_focus', 'short'),
      anchorMs: Date.now(),
      offsetMs: 0,
    });
    renderBar();
    expect(screen.getByText(/pausa pomodoro/i)).toBeInTheDocument();
    expect(screen.queryByText(/pausa longa/i)).toBeNull();
  });
});
