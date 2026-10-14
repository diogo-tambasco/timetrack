import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskRow from './TaskRow';
import type { BoardTask } from '../../lib/api';

const task: BoardTask = {
  id: 't1', workspace_id: 'w1', project_id: 'p1', objective_id: 'o1', name: 'Definir telas',
  status: 'doing', due_date: '2026-06-15', recurrence: null, notes: '', created_by: 'u1',
  assignee: null, requested_by: null, created_at: '2026-06-04T10:00:00Z', tracked_seconds: 5 * 3600 + 12 * 60,
};

describe('TaskRow (CA-20/23)', () => {
  it('canWrite: select de status dispara onMove com o novo status', () => {
    const onMove = vi.fn();
    render(<TaskRow task={task} canWrite onEdit={vi.fn()} onMove={onMove} />);
    fireEvent.change(screen.getByLabelText(/status de definir telas/i), { target: { value: 'done' } });
    expect(onMove).toHaveBeenCalledWith('t1', 'done');
  });

  it('mostra o tempo rastreado formatado e a data', () => {
    render(<TaskRow task={task} canWrite onEdit={vi.fn()} onMove={vi.fn()} />);
    expect(screen.getByText('5h12')).toBeInTheDocument();
    expect(screen.getByText('15/06/2026')).toBeInTheDocument();
  });

  it('canWrite=false: sem select e sem Editar (CA-22)', () => {
    render(<TaskRow task={task} canWrite={false} onEdit={vi.fn()} onMove={vi.fn()} />);
    expect(screen.queryByLabelText(/status de/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /editar/i })).not.toBeInTheDocument();
  });
});
