// notionIcon.spec — fase 24 T-15 (CA-24): ícone Notion presente no card/linha/
// célula quando há notion_url; ausente quando não há.
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import TaskBoardCard from './TaskBoardCard';
import TaskListView from './TaskListView';
import TaskTableView from './TaskTableView';
import { mkTask } from './testFixtures';

describe('ícone Notion nas visões (CA-24)', () => {
  it('card mostra o ícone só com notion_url', () => {
    const { rerender } = render(
      <TaskBoardCard task={mkTask({ notion_url: 'https://www.notion.so/X-abc123' })} canWrite onOpen={vi.fn()} onDragStart={vi.fn()} />,
    );
    expect(screen.getByTestId('notion-icon')).toBeInTheDocument();
    rerender(<TaskBoardCard task={mkTask({ notion_url: null })} canWrite onOpen={vi.fn()} onDragStart={vi.fn()} />);
    expect(screen.queryByTestId('notion-icon')).not.toBeInTheDocument();
  });

  it('list mostra o ícone só com notion_url', () => {
    render(
      <TaskListView
        tasks={[mkTask({ id: 'n', name: 'Com doc', status: 'todo', notion_url: 'https://www.notion.so/X-abc123' }), mkTask({ id: 'p', name: 'Sem doc', status: 'todo' })]}
        canWrite
        onOpenTask={vi.fn()}
        onQuickAdd={vi.fn()}
        onCycleStatus={vi.fn()}
      />,
    );
    expect(screen.getAllByTestId('notion-icon')).toHaveLength(1);
  });

  it('table mostra o ícone só com notion_url', () => {
    render(
      <TaskTableView
        tasks={[mkTask({ id: 'n', name: 'Com doc', notion_url: 'https://www.notion.so/X-abc123' }), mkTask({ id: 'p', name: 'Sem doc' })]}
        canWrite
        onOpenTask={vi.fn()}
        onQuickAdd={vi.fn()}
        onCycleStatus={vi.fn()}
        onCyclePriority={vi.fn()}
      />,
    );
    expect(screen.getAllByTestId('notion-icon')).toHaveLength(1);
  });
});
