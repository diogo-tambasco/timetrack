// Fase 22 T-03: smoke de markup responsivo da TimerBar/BreakOverlay. Vitest+jsdom
// NÃO mede layout/overflow (isso é o E2E mobile, T-10/CA-19); aqui só afirmamos
// que (a) a barra usa a classe responsiva .tt-timerbar (que reflowa < md), (b)
// nenhuma ação some no mobile (iniciar/parar/pausar/trocar projeto seguem no
// DOM), (c) os dígitos/selects deixaram de declarar largura fixa inline (saíram
// pra classe que encolhe). Comportamento do timer NÃO muda (CA-12).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TimerBar from './TimerBar';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Project, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listProjects: vi.fn(), listTasks: vi.fn(), startTimer: vi.fn(), stopTimer: vi.fn() };
});

const ws = 'w1';
const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: null, name: 'Lauda', color: '#4f9dde',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-01T10:00:00Z',
  },
];

function estadoRodando(): TimerState {
  return {
    mode: 'countup',
    entry: {
      id: 'e1', workspace_id: ws, project_id: 'p1', task_id: null,
      description: 'Revisão', tags: [], started_at: '2026-06-10T14:00:00Z',
      stopped_at: null, duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 100, target_seconds: null, remaining_seconds: null,
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
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.listTasks).mockResolvedValue([]);
});

describe('TimerBar responsiva (T-03)', () => {
  it('a barra usa a classe responsiva .tt-timerbar (reflowa < md)', () => {
    renderBar();
    expect(screen.getByTestId('timerbar')).toHaveClass('tt-timerbar');
  });

  it('ociosa: ação de iniciar e troca de projeto seguem no DOM', async () => {
    renderBar();
    expect(await screen.findByRole('button', { name: /iniciar/i })).toBeInTheDocument();
    expect(screen.getByLabelText('Projeto')).toBeInTheDocument();
    // o dígito não declara largura fixa inline (saiu pra classe que encolhe)
    const digits = screen.getByText('0:00:00');
    expect(digits).toHaveClass('tt-timer-digits');
    expect(digits.style.minWidth).toBe('');
  });

  it('rodando: pausar e parar seguem no DOM no mobile', () => {
    useTimerStore.setState({ ...initialTimerState, state: estadoRodando() });
    renderBar();
    expect(screen.getByRole('button', { name: /pausar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /parar/i })).toBeInTheDocument();
    expect(screen.getByTestId('timerbar')).toHaveClass('tt-timerbar');
  });

  it('pausa: BreakOverlay usa a classe .tt-breakbar e mantém as 3 ações', () => {
    // anchorMs=Date.now()/offset=0 → deltaSec=0 → remaining = remaining_seconds
    // (sem isso o clock derivaria remaining 0 → "Pausa encerrada", sem o pular).
    useTimerStore.setState({
      ...initialTimerState,
      anchorMs: Date.now(),
      offsetMs: 0,
      state: {
        mode: 'break', entry: null, elapsed_seconds: 0,
        target_seconds: 300, remaining_seconds: 120, break_kind: 'short',
        resume: { entry_id: 'e1', mode: 'pomodoro_focus', target_seconds: null },
      } as TimerState,
    });
    renderBar();
    expect(screen.getByTestId('break-overlay')).toHaveClass('tt-breakbar');
    expect(screen.getByRole('button', { name: /retomar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /parar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pular pausa/i })).toBeInTheDocument();
  });
});
