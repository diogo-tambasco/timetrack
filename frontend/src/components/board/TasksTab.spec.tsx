// TasksTab.spec — fase 24 T-14 (CA-16/16.1/18/19/25): troca de visão persiste em
// tt.view.board; mapeamento do legado (cards→Board, table→List, NÃO a Table
// nativa); List agrupa por status + quick-add; Table mostra colunas nativas +
// cicla status/prio; gating.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TasksTab from './TasksTab';
import { mkBoard, mkTask } from './testFixtures';

const { createSpy, patchSpy } = vi.hoisted(() => ({ createSpy: vi.fn(), patchSpy: vi.fn() }));

vi.mock('../../lib/queries/tasks', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/tasks')>();
  return {
    ...mod,
    useCreateTask: () => ({ mutate: createSpy }),
    usePatchTask: () => ({ mutate: patchSpy }),
  };
});
// TaskPanel não abre rede nesta suíte.
vi.mock('./TaskPanel', () => ({ default: () => <div data-testid="task-panel" /> }));

function renderTab(canWrite = true) {
  const board = mkBoard({
    project: { id: 'p1', name: 'App', color: '#000000', client_name: null, weekly_hours_goal: null, archived_at: null, tracked_seconds: 0, can_write: canWrite },
    general: {
      task_count: 2, done_count: 0, tracked_seconds: 0,
      tasks: [
        mkTask({ id: 'a', name: 'Alpha', status: 'todo', priority: 0 }),
        mkTask({ id: 'b', name: 'Bravo', status: 'doing', priority: 2 }),
      ],
    },
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <TasksTab wsId="w1" board={board} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

describe('TasksTab — visão e persistência (CA-16/16.1)', () => {
  it('default Board (4 colunas)', () => {
    renderTab();
    expect(screen.getByTestId('board-columns')).toBeInTheDocument();
  });

  it('trocar pra List persiste tt.view.board e renderiza a List', () => {
    renderTab();
    fireEvent.click(screen.getByRole('tab', { name: /list/i }));
    expect(screen.getByTestId('task-list')).toBeInTheDocument();
    expect(localStorage.getItem('tt.view.board')).toBe('list');
  });

  it("legado 'cards' renderiza o Board (CA-16.1)", () => {
    localStorage.setItem('tt.view.board', 'cards');
    renderTab();
    expect(screen.getByTestId('board-columns')).toBeInTheDocument();
  });

  it("legado 'table' renderiza a List (NÃO a Table nativa — CA-16.1)", () => {
    localStorage.setItem('tt.view.board', 'table');
    renderTab();
    expect(screen.getByTestId('task-list')).toBeInTheDocument();
    expect(screen.queryByTestId('task-table')).not.toBeInTheDocument();
  });

  it("valor novo 'board'/'list' faz round-trip", () => {
    localStorage.setItem('tt.view.board', 'list');
    renderTab();
    expect(screen.getByTestId('task-list')).toBeInTheDocument();
  });

  it("Table nativa PERSISTE no reload: clicar Table grava 'table-v2' e o remount reabre a Table, NÃO List (CA-16.1/RF-03)", () => {
    renderTab();
    fireEvent.click(screen.getByRole('tab', { name: /table/i }));
    expect(screen.getByTestId('task-table')).toBeInTheDocument();
    expect(localStorage.getItem('tt.view.board')).toBe('table-v2'); // token distinto do legado
    cleanup(); // simula reload (novo mount lendo o localStorage)
    renderTab();
    expect(screen.getByTestId('task-table')).toBeInTheDocument();
    expect(screen.queryByTestId('task-list')).not.toBeInTheDocument();
  });
});

describe('TasksTab — List/Table/quick-add/gating (CA-18/19/25)', () => {
  it('Table nativa mostra as colunas e cicla status/prio (CA-19)', () => {
    renderTab();
    fireEvent.click(screen.getByRole('tab', { name: /table/i }));
    expect(screen.getByTestId('task-table')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Status de Alpha'));
    expect(patchSpy).toHaveBeenCalledWith(
      { id: 'a', patch: { status: 'doing' } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
    fireEvent.click(screen.getByLabelText('Prioridade de Bravo'));
    expect(patchSpy).toHaveBeenCalledWith(
      { id: 'b', patch: { priority: 3 } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('New task cria no backlog (CA-18, fase 31: default = backlog)', () => {
    renderTab();
    fireEvent.click(screen.getByRole('button', { name: /new task/i }));
    expect(createSpy).toHaveBeenCalledWith(
      { project_id: 'p1', name: 'New task', status: 'backlog' },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('quick-add da coluna cria no status da coluna (CA-18)', () => {
    renderTab();
    fireEvent.click(screen.getByRole('button', { name: /add em backlog/i }));
    expect(createSpy).toHaveBeenCalledWith(
      { project_id: 'p1', name: 'New task', status: 'backlog' },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('can_write=false: sem New task, sem quick-add, chips congelados (CA-25)', () => {
    renderTab(false);
    expect(screen.queryByRole('button', { name: /new task/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /table/i }));
    expect(screen.getByLabelText('Status de Alpha')).toBeDisabled();
    expect(screen.queryByRole('button', { name: /add task/i })).not.toBeInTheDocument();
  });
});
