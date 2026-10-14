// TimerCard.spec — fase 13 T-03 (CA-12/13/14): rodando mostra dígitos+projeto+
// Pausar/Parar (e clicar chama as ações do store); pausa mostra rótulo +
// countdown sem o BreakOverlay completo; ocioso mostra CTA e NÃO mostra dígitos
// vivos. Padrão de mock dos specs da TimerBar (store real + api mockada).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TimerCard from './TimerCard';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Project, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listProjects: vi.fn(), pauseTimer: vi.fn(), stopTimer: vi.fn() };
});

const ws = 'w1';
const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: null, name: 'Lauda', color: '#4f9dde',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-01T10:00:00Z',
  },
];

function rodando(): TimerState {
  return {
    mode: 'countup',
    entry: {
      id: 'e1', workspace_id: ws, project_id: 'p1', task_id: null,
      description: 'Revisão de laudas — lote 14', tags: [],
      started_at: '2026-06-10T14:00:00Z', stopped_at: null,
      duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 5070, target_seconds: null, remaining_seconds: null,
  };
}

function emPausa(): TimerState {
  return {
    mode: 'break', entry: null, elapsed_seconds: 0,
    target_seconds: 900, remaining_seconds: 900,
  };
}

function renderCard(onNewEntry?: () => void) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TimerCard onNewEntry={onNewEntry} />
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
      workspaces: [{ id: ws, name: 'WS', role: 'owner' }],
    },
  });
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.pauseTimer).mockResolvedValue({ state: emPausa(), server_time: '2026-06-10T14:00:00Z' });
  vi.mocked(api.stopTimer).mockResolvedValue({ state: null, server_time: '2026-06-10T14:00:00Z' });
});

describe('TimerCard', () => {
  it('rodando: rótulo, descrição, chip do projeto, dígitos grandes e Pausar/Parar', async () => {
    useTimerStore.setState({ state: rodando(), anchorMs: Date.now(), offsetMs: 0 });
    renderCard();

    expect(screen.getByText(/rodando agora/i)).toBeInTheDocument();
    expect(screen.getByText('Revisão de laudas — lote 14')).toBeInTheDocument();
    expect(screen.getByText('1:24:30')).toBeInTheDocument(); // 5070s

    const chip = await waitFor(() => {
      const el = document.querySelector('.tt-chip');
      expect(el).not.toBeNull();
      return el as HTMLElement;
    });
    expect(chip.textContent).toContain('Lauda');

    fireEvent.click(screen.getByRole('button', { name: /pausar/i }));
    await waitFor(() => expect(api.pauseTimer).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /parar/i }));
    await waitFor(() => expect(api.stopTimer).toHaveBeenCalled());
  });

  it('pausa: rótulo "Pausa" + countdown, sem presets do BreakOverlay', () => {
    useTimerStore.setState({ state: emPausa(), anchorMs: Date.now(), offsetMs: 0 });
    renderCard();

    expect(screen.getByText('Pausa')).toBeInTheDocument();
    expect(screen.getByText('0:15:00')).toBeInTheDocument(); // 900s restantes
    // NÃO replica os controles de pausa da TimerBar (D-02)
    expect(screen.queryByRole('button', { name: /retomar/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /pular/i })).toBeNull();
  });

  it('ocioso: CTA pra começar, sem dígitos vivos; clicar chama onNewEntry', () => {
    const onNew = vi.fn();
    renderCard(onNew);

    expect(screen.getByText(/sem timer rodando/i)).toBeInTheDocument();
    // ocioso não mostra dígitos vivos (CA-14): nenhum mono de tempo
    expect(document.querySelector('.mono')).toBeNull();
    expect(screen.queryByText(/rodando agora/i)).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /comece a cronometrar/i }));
    expect(onNew).toHaveBeenCalled();
  });
});
