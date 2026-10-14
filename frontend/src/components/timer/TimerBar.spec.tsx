// TimerBar.spec — T-15 (CA-31): rodando, a barra exibe descrição, chip do
// projeto na cor, tempo decorrido em mono e stop pulsante; ociosa, inicia
// um timer com o payload do contrato.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TimerBar from './TimerBar';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Project, Task, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listProjects: vi.fn(),
    listTasks: vi.fn(),
    startTimer: vi.fn(),
    stopTimer: vi.fn(),
  };
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
      description: 'Revisão de laudas — lote 14', tags: [],
      started_at: '2026-06-10T14:00:00Z', stopped_at: null,
      duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 5070,
    target_seconds: null,
    remaining_seconds: null,
  };
}

function mkTask(over: Partial<Task> = {}): Task {
  return {
    id: 't1', workspace_id: ws, project_id: 'p1', objective_id: null,
    name: 'Definir telas', status: 'todo', priority: 1, notion_url: null,
    position: 0, due_date: null, recurrence: null, notes: '', created_by: 'u1',
    assignee: null, requested_by: null, created_at: '2026-06-01T10:00:00Z', ...over,
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

describe('TimerBar', () => {
  // CA-31 — rodando: descrição + chip do projeto (.tt-chip/.tt-dot na cor)
  // + tempo decorrido mono h:mm:ss + stop com .tt-pulse.
  it('rodando', async () => {
    useTimerStore.setState({
      state: estadoRodando(),
      anchorMs: Date.now(),
      offsetMs: 0,
    });
    renderBar();

    expect(screen.getByDisplayValue('Revisão de laudas — lote 14')).toBeInTheDocument();
    expect(screen.getByText('1:24:30')).toBeInTheDocument(); // 5070s em mono

    const chip = await waitFor(() => {
      const el = document.querySelector('.tt-chip');
      expect(el).not.toBeNull();
      return el as HTMLElement;
    });
    expect(chip.textContent).toContain('Lauda');
    const dot = chip.querySelector('.tt-dot') as HTMLElement;
    expect(dot).not.toBeNull();
    expect(dot.style.background).toContain('rgb(79, 157, 222)'); // #4f9dde

    const stop = screen.getByRole('button', { name: /parar/i });
    expect(stop.classList.contains('tt-pulse')).toBe(true);
  });

  // F5 da verificação adversarial: o lookup do chip só via projetos ATIVOS
  // — arquivar o projeto do timer rodando (ou continuar entry de projeto
  // arquivado, permitido pelo backend) sumia com o chip exigido pelo CA-31.
  it('rodando com projeto ARQUIVADO ainda mostra o chip', async () => {
    const arquivado: Project = {
      ...projetos[0],
      id: 'p2',
      name: 'Antigo',
      color: '#e2b714',
      archived_at: '2026-06-09T00:00:00Z',
    };
    // o mock se comporta como o servidor: arquivados só vêm com archived=true
    vi.mocked(api.listProjects).mockImplementation(async (_ws, opts) =>
      opts?.archived ? [...projetos, arquivado] : projetos,
    );
    const estado = estadoRodando();
    estado.entry!.project_id = 'p2';
    useTimerStore.setState({ state: estado, anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    const chip = await waitFor(() => {
      const el = document.querySelector('.tt-chip');
      expect(el).not.toBeNull();
      return el as HTMLElement;
    });
    expect(chip.textContent).toContain('Antigo');
  });

  // fase 30 CA-16 — rodando com task_id resolvível: chip da task (nome) ALÉM do
  // chip de projeto (aditivo — não substitui).
  it('CA-16: rodando com task_id resolvível mostra o chip da task', async () => {
    vi.mocked(api.listTasks).mockResolvedValue([mkTask({ id: 't9', name: 'Definir telas' })]);
    const estado = estadoRodando();
    estado.entry!.task_id = 't9';
    useTimerStore.setState({ state: estado, anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    expect(await screen.findByText('Definir telas')).toBeInTheDocument();
    expect(screen.getByText('Lauda')).toBeInTheDocument(); // chip de projeto segue presente
    // a query de tasks foi filtrada pelo projeto da entry (cacheada — RNF-04)
    expect(api.listTasks).toHaveBeenCalledWith(ws, { project_id: 'p1' });
  });

  // fase 30 CA-18/RNF-04 — rodando SEM task_id: nenhum chip de task E nenhuma
  // query de tasks dispara (caminho byte-idêntico ao de hoje).
  it('CA-18: rodando sem task_id não mostra chip de task nem busca tasks', async () => {
    useTimerStore.setState({ state: estadoRodando(), anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    await waitFor(() => expect(document.querySelector('.tt-chip')).not.toBeNull());
    expect(document.querySelectorAll('.tt-chip')).toHaveLength(1); // só o chip de projeto
    expect(api.listTasks).not.toHaveBeenCalled(); // RNF-04: sem task → sem fetch
  });

  // fase 30 D-08 — task_id cujo nome NÃO resolve: degrada (sem chip de task),
  // sem quebrar a TimerBar compartilhada com o TRACKING.
  it('D-08: task_id sem nome resolvível → sem chip de task, sem crash', async () => {
    vi.mocked(api.listTasks).mockResolvedValue([]); // lista vazia → não acha a task
    const estado = estadoRodando();
    estado.entry!.task_id = 'tX';
    useTimerStore.setState({ state: estado, anchorMs: Date.now(), offsetMs: 0 });
    renderBar();

    expect(await screen.findByText('Lauda')).toBeInTheDocument(); // o chip de projeto carregou
    await waitFor(() => expect(api.listTasks).toHaveBeenCalled()); // a query da task rodou (há task_id)
    expect(document.querySelectorAll('.tt-chip')).toHaveLength(1); // mas não resolveu → só o chip de projeto
  });

  it('ociosa inicia timer com o payload do contrato', async () => {
    vi.mocked(api.startTimer).mockResolvedValue({
      state: estadoRodando(),
      server_time: '2026-06-10T14:00:00Z',
    });
    renderBar();

    fireEvent.change(screen.getByPlaceholderText(/no que você está trabalhando/i), {
      target: { value: 'Nova frente' },
    });
    await screen.findByRole('option', { name: 'Lauda' }); // projetos carregados
    fireEvent.change(screen.getByLabelText(/projeto/i), { target: { value: 'p1' } });
    fireEvent.click(screen.getByRole('button', { name: /iniciar/i }));

    await waitFor(() =>
      expect(api.startTimer).toHaveBeenCalledWith(
        expect.objectContaining({ workspace_id: ws, description: 'Nova frente', project_id: 'p1' }),
      ),
    );
    // o estado retornado pelo servidor vira a barra rodando
    expect(await screen.findByDisplayValue('Revisão de laudas — lote 14')).toBeInTheDocument();
  });
});
