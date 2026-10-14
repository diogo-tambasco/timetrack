// OverviewTab.spec.tsx — fase 28 T-09 (CA-17/20/21): KPIs do board · Upcoming
// clicável (openTask) · Progress por status + Goals · Recent notes (fase 26
// aditivo). Gating por can_write (CA-20).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ReactNode } from 'react';
import OverviewTab from './OverviewTab';
import type { Board, BoardTask, Note } from '../../lib/api';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listNotes: vi.fn(), getReport: vi.fn() };
});

const TODAY = '2025-06-25';
function mkTask(over: Partial<BoardTask>): BoardTask {
  return {
    id: 't', workspace_id: 'w1', project_id: 'p1', objective_id: null, name: 'Task',
    status: 'todo', priority: 0, notion_url: null, position: 0, due_date: null,
    recurrence: null, notes: '', created_by: 'u1', assignee: null, requested_by: null,
    created_at: '2025-06-01T00:00:00Z', tracked_seconds: 0, subtask_count: 0,
    subtask_done: 0, goal_count: 0, goal_done: 0, ...over,
  };
}

function mkBoard(canWrite = true): Board {
  return {
    project: {
      id: 'p1', name: 'Acme', color: '#4f9dde', client_name: null,
      weekly_hours_goal: null, archived_at: null, deadline: '2025-09-30',
      tracked_seconds: 511200, can_write: canWrite,
    },
    objectives: [
      {
        id: 'o1', name: 'Roadmap Q3', description: '', status: 'open', goal_hours: null,
        position: 0, task_count: 2, done_count: 1, done_pct: 50, tracked_seconds: 0,
        completion_mode: 'by_tasks', target_date: null, progress_pct: 50,
        tasks: [
          mkTask({ id: 'tdue', name: 'Fechar OKRs', status: 'doing', due_date: '2025-06-30' }),
          mkTask({ id: 'tdone', name: 'Sprint', status: 'done', due_date: '2025-06-20' }),
        ],
      },
    ],
    general: {
      task_count: 1, done_count: 0, tracked_seconds: 0,
      tasks: [mkTask({ id: 'tpast', name: 'Antiga', status: 'todo', due_date: '2025-05-01' })],
    },
  };
}

function renderTabC(board: Board, onOpenTask = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <OverviewTab wsId="w1" board={board} onOpenTask={onOpenTask} now={new Date(`${TODAY}T12:00:00`)} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function renderTab(board: Board, onOpenTask = vi.fn()) {
  renderTabC(board, onOpenTask);
  return onOpenTask;
}

beforeEach(() => {
  vi.mocked(api.listNotes).mockResolvedValue([]);
  vi.mocked(api.getReport).mockResolvedValue({
    from: '', to: '', total_seconds: 7200, by_project: [], focus: {} as never,
    goals: [], objectives: [], availability: {} as never, focus_by_hour: [],
    kpis: {} as never, previous: null,
  });
});
afterEach(() => vi.clearAllMocks());

describe('OverviewTab (T-09)', () => {
  it('KPIs com os números do board', async () => {
    renderTab(mkBoard());
    // Total time = 511200s = 142h
    expect(await screen.findByText('142h00')).toBeInTheDocument();
    // Done = 33% (1/3) — 3 tasks, 1 done
    expect(screen.getByText('33%')).toBeInTheDocument();
  });

  it('Upcoming lista as tasks de prazo futuras/hoje, clicáveis (chama openTask)', async () => {
    const onOpen = renderTab(mkBoard());
    const item = await screen.findByText('Fechar OKRs');
    fireEvent.click(item);
    expect(onOpen).toHaveBeenCalledWith('tdue');
  });

  it('Upcoming exclui done e prazos passados', () => {
    renderTab(mkBoard());
    // task done com due no passado não entra
    expect(screen.queryByText('Sprint')).not.toBeInTheDocument();
    // task todo com due no passado (antes de hoje) não entra
    expect(screen.queryByText('Antiga')).not.toBeInTheDocument();
  });

  it('Upcoming vazio mostra a mensagem à prova de culpa', async () => {
    const board = mkBoard();
    board.objectives[0].tasks = [];
    board.general.tasks = [];
    renderTab(board);
    expect(await screen.findByText(/No deadlines set\./i)).toBeInTheDocument();
  });

  it('Progress por status presente', () => {
    renderTab(mkBoard());
    expect(screen.getByText('Progress')).toBeInTheDocument();
    expect(screen.getByText('In progress')).toBeInTheDocument();
    expect(screen.getByText('Backlog')).toBeInTheDocument();
  });

  it('Recent notes presente quando há notas (fase 26)', async () => {
    const notas: Note[] = [
      {
        id: 'n1', workspace_id: 'w1', project_id: 'p1', task_id: null, title: 'Minha nota',
        body_md: '', preview: 'corpo', links: [], created_by: 'u1',
        created_at: '2025-06-20T00:00:00Z', updated_at: '2025-06-24T00:00:00Z',
      },
    ];
    vi.mocked(api.listNotes).mockResolvedValue(notas);
    renderTab(mkBoard());
    expect(await screen.findByText('Minha nota')).toBeInTheDocument();
  });

  it('Recent notes omitida quando o hook devolve vazio (CA-21)', async () => {
    vi.mocked(api.listNotes).mockResolvedValue([]);
    renderTab(mkBoard());
    await screen.findByText('Fechar OKRs');
    expect(screen.queryByText(/Recent notes/i)).not.toBeInTheDocument();
  });

  it('viewer (can_write=false): Upcoming não vira ação de escrita', async () => {
    renderTab(mkBoard(false));
    // o item ainda aparece e é clicável (abrir é leitura), mas não há "criar task"
    expect(await screen.findByText('Fechar OKRs')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new task|nova task/i })).not.toBeInTheDocument();
  });

  it('CA-12.2: Goal by_hours fechado por horas → barra cheia + chip Done (não 0%)', () => {
    const board = mkBoard();
    board.objectives = [
      {
        id: 'oh', name: 'Bater 40h', description: '', status: 'done', goal_hours: 4,
        position: 0, task_count: 3, done_count: 0, done_pct: 0, tracked_seconds: 14400,
        completion_mode: 'by_hours', target_date: null, progress_pct: 100, tasks: [],
      },
    ];
    const { container } = renderTabC(board);
    // a barra do goal reflete o MODO (100%), não done_pct (0%).
    const fills = screen.getAllByTestId('progress-bar-fill');
    expect(fills.some((f) => f.style.width === '100%')).toBe(true);
    expect(screen.getByTestId('goal-done-chip')).toBeInTheDocument();
    expect(container.innerHTML).not.toContain('var(--danger)');
  });

  it('CA-12.2/RNF-03: Goal by_date vencido aberto → Open discreto, sem cor de erro', () => {
    const board = mkBoard();
    board.objectives = [
      {
        id: 'od', name: 'Demo cliente', description: '', status: 'open', goal_hours: null,
        position: 0, task_count: 0, done_count: 0, done_pct: 0, tracked_seconds: 0,
        completion_mode: 'by_date', target_date: '2020-01-01', progress_pct: 0, tasks: [],
      },
    ];
    const { container } = renderTabC(board);
    expect(screen.getByText('Demo cliente')).toBeInTheDocument();
    expect(screen.queryByTestId('goal-done-chip')).not.toBeInTheDocument();
    expect(container.innerHTML).not.toContain('var(--danger)');
  });
});
