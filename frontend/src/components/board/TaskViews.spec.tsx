// TaskViews.spec — fase 24 T-14 (CA-18/19/25): List agrupa por status + quick-add
// por grupo + status 1-clique cicla; Table cicla status/priority + abre popup na
// linha. Componentes puros (callbacks como spies).
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import TaskListView from './TaskListView';
import TaskTableView from './TaskTableView';
import { mkTask } from './testFixtures';

const tasks = [
  mkTask({ id: 'a', name: 'Alpha', status: 'todo', priority: 1 }),
  mkTask({ id: 'b', name: 'Bravo', status: 'doing', priority: 0 }),
];

describe('TaskListView (CA-16/18/25)', () => {
  it('agrupa por status com os 4 grupos (quick-add por grupo)', () => {
    render(<TaskListView tasks={tasks} canWrite onOpenTask={vi.fn()} onQuickAdd={vi.fn()} onCycleStatus={vi.fn()} />);
    // cada grupo tem um quick-add "Add em <status>" — prova os 4 grupos sem colidir
    // com os rótulos de chip de status das linhas.
    for (const label of ['Backlog', 'To do', 'In progress', 'Done']) {
      expect(screen.getByRole('button', { name: new RegExp(`add em ${label}`, 'i') })).toBeInTheDocument();
    }
  });

  it('status 1-clique cicla (CA-18)', () => {
    const onCycle = vi.fn();
    render(<TaskListView tasks={tasks} canWrite onOpenTask={vi.fn()} onQuickAdd={vi.fn()} onCycleStatus={onCycle} />);
    fireEvent.click(screen.getByLabelText('Status de Alpha'));
    expect(onCycle).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }), 'doing');
  });

  it('quick-add por grupo passa o status do grupo (CA-18)', () => {
    const onQuickAdd = vi.fn();
    render(<TaskListView tasks={tasks} canWrite onOpenTask={vi.fn()} onQuickAdd={onQuickAdd} onCycleStatus={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /add em done/i }));
    expect(onQuickAdd).toHaveBeenCalledWith('done');
  });

  it('clicar no nome abre o popup', () => {
    const onOpen = vi.fn();
    render(<TaskListView tasks={tasks} canWrite onOpenTask={onOpen} onQuickAdd={vi.fn()} onCycleStatus={vi.fn()} />);
    fireEvent.click(screen.getByText('Alpha'));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }));
  });

  it('can_write=false: sem quick-add, status congelado (CA-25)', () => {
    render(<TaskListView tasks={tasks} canWrite={false} onOpenTask={vi.fn()} onQuickAdd={vi.fn()} onCycleStatus={vi.fn()} />);
    expect(screen.queryByRole('button', { name: /add em/i })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Status de Alpha')).toBeDisabled();
  });
});

describe('TaskTableView (CA-19/25)', () => {
  it('mostra as colunas nativas status/priority/due/to-dos/time', () => {
    render(<TaskTableView tasks={tasks} canWrite onOpenTask={vi.fn()} onQuickAdd={vi.fn()} onCycleStatus={vi.fn()} onCyclePriority={vi.fn()} />);
    const table = screen.getByTestId('task-table');
    for (const h of ['Status', 'Priority', 'Due', 'To-dos', 'Time']) {
      expect(within(table).getByText(h)).toBeInTheDocument();
    }
  });

  it('clicar na linha abre o popup; status/priority ciclam', () => {
    const onOpen = vi.fn();
    const onStatus = vi.fn();
    const onPrio = vi.fn();
    render(<TaskTableView tasks={tasks} canWrite onOpenTask={onOpen} onQuickAdd={vi.fn()} onCycleStatus={onStatus} onCyclePriority={onPrio} />);
    fireEvent.click(screen.getByText('Bravo'));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'b' }));
    fireEvent.click(screen.getByLabelText('Status de Alpha'));
    expect(onStatus).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }), 'doing');
    fireEvent.click(screen.getByLabelText('Prioridade de Alpha'));
    expect(onPrio).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }), 2);
  });

  it('can_write=false: chips congelados, sem quick-add (CA-25)', () => {
    render(<TaskTableView tasks={tasks} canWrite={false} onOpenTask={vi.fn()} onQuickAdd={vi.fn()} onCycleStatus={vi.fn()} onCyclePriority={vi.fn()} />);
    expect(screen.getByLabelText('Status de Alpha')).toBeDisabled();
    expect(screen.queryByRole('button', { name: /add task/i })).not.toBeInTheDocument();
  });
});
