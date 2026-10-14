// TaskBoardColumns.spec — fase 24 T-11 (CA-17/18/25; reconcilia o spec da fase 12,
// CA-28.1): 4 colunas backlog/todo/doing/done, tasks ordenadas por position,
// quick-add por coluna, gating por can_write. O DnD em si é teste do dropStatus
// puro (dnd.test.ts) — aqui só o render/quick-add/gating.
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import TaskBoardColumns from './TaskBoardColumns';
import { mkTask } from './testFixtures';

const tasks = [
  mkTask({ id: 'b1', name: 'Item backlog', status: 'backlog' }),
  mkTask({ id: 't1', name: 'Item todo', status: 'todo', position: 1 }),
  mkTask({ id: 't2', name: 'Item todo 2', status: 'todo', position: 0 }),
  mkTask({ id: 'd1', name: 'Item doing', status: 'doing' }),
  mkTask({ id: 'f1', name: 'Item done', status: 'done' }),
];

describe('TaskBoardColumns (CA-17/18/25)', () => {
  it('renderiza 4 colunas com rótulos do mockup', () => {
    render(<TaskBoardColumns tasks={tasks} canWrite onOpenTask={vi.fn()} onDrop={vi.fn()} onQuickAdd={vi.fn()} />);
    expect(screen.getByTestId('board-col-backlog')).toBeInTheDocument();
    expect(screen.getByTestId('board-col-todo')).toBeInTheDocument();
    expect(screen.getByTestId('board-col-doing')).toBeInTheDocument();
    expect(screen.getByTestId('board-col-done')).toBeInTheDocument();
    expect(screen.getByText('Backlog')).toBeInTheDocument();
    expect(screen.getByText('In progress')).toBeInTheDocument();
  });

  it('distribui as tasks nos status certos, ordenadas por position', () => {
    render(<TaskBoardColumns tasks={tasks} canWrite onOpenTask={vi.fn()} onDrop={vi.fn()} onQuickAdd={vi.fn()} />);
    const todoCol = screen.getByTestId('board-col-todo');
    // position 0 (Item todo 2) deve aparecer antes de position 1 (Item todo).
    const cards = within(todoCol).getAllByText(/Item todo/);
    expect(cards[0]).toHaveTextContent('Item todo 2');
  });

  it('clicar num card chama onOpenTask', () => {
    const onOpen = vi.fn();
    render(<TaskBoardColumns tasks={tasks} canWrite onOpenTask={onOpen} onDrop={vi.fn()} onQuickAdd={vi.fn()} />);
    fireEvent.click(screen.getByTestId('task-card-b1'));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'b1' }));
  });

  it('quick-add chama onQuickAdd com o status da coluna (CA-18)', () => {
    const onQuickAdd = vi.fn();
    render(<TaskBoardColumns tasks={tasks} canWrite onOpenTask={vi.fn()} onDrop={vi.fn()} onQuickAdd={onQuickAdd} />);
    fireEvent.click(screen.getByRole('button', { name: /add em backlog/i }));
    expect(onQuickAdd).toHaveBeenCalledWith('backlog');
  });

  it('can_write=false esconde quick-add e desabilita drag (CA-25)', () => {
    render(<TaskBoardColumns tasks={tasks} canWrite={false} onOpenTask={vi.fn()} onDrop={vi.fn()} onQuickAdd={vi.fn()} />);
    expect(screen.queryByRole('button', { name: /add em/i })).not.toBeInTheDocument();
    expect(screen.getByTestId('task-card-b1')).toHaveAttribute('draggable', 'false');
  });
});
