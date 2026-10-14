// SubtaskSidebar.spec — fase 24 T-13 (CA-23/25): add (Enter→cria+limpa+foco),
// checkbox done, ⭐ is_goal, cabeçalho feitas/total + metas, vazio, gating.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import SubtaskSidebar from './SubtaskSidebar';
import type { Subtask } from '../../lib/api';

const { createSpy, patchSpy, delSpy, subsRef } = vi.hoisted(() => ({
  createSpy: vi.fn(),
  patchSpy: vi.fn(),
  delSpy: vi.fn(),
  subsRef: { current: [] as Subtask[] },
}));

vi.mock('../../lib/queries/subtasks', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/subtasks')>();
  return {
    ...mod,
    useSubtasks: () => ({ data: subsRef.current }),
    useCreateSubtask: () => ({ mutate: createSpy }),
    usePatchSubtask: () => ({ mutate: patchSpy }),
    useDeleteSubtask: () => ({ mutate: delSpy }),
  };
});

const mkSub = (o: Partial<Subtask>): Subtask => ({
  id: 's1', task_id: 't1', title: 'Item', done: false, is_goal: false,
  position: 0, created_at: '2026-06-04T10:00:00Z', ...o,
});

function renderSidebar(subs: Subtask[], canWrite = true) {
  subsRef.current = subs;
  render(<SubtaskSidebar wsId="w1" taskId="t1" projectId="p1" canWrite={canWrite} />);
}

beforeEach(() => vi.clearAllMocks());

describe('SubtaskSidebar / Task-goals (CA-23/25, fase 31)', () => {
  it('cabeçalho "Task-goals"; vazio → "No goals yet."', () => {
    renderSidebar([]);
    expect(screen.getByText('Task-goals')).toBeInTheDocument();
    expect(screen.getByText(/no goals yet/i)).toBeInTheDocument();
  });

  it('Enter no add cria o task-goal (CA-23)', () => {
    renderSidebar([]);
    const input = screen.getByLabelText('Adicionar task-goal');
    fireEvent.change(input, { target: { value: 'Revisar' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(createSpy).toHaveBeenCalledWith('Revisar', expect.objectContaining({ onSuccess: expect.any(Function) }));
  });

  it('add vazio não cria', () => {
    renderSidebar([]);
    const input = screen.getByLabelText('Adicionar task-goal');
    fireEvent.change(input, { target: { value: '  ' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(createSpy).not.toHaveBeenCalled();
  });

  it('checkbox dispara PATCH done', () => {
    renderSidebar([mkSub({ id: 's1', title: 'Item', done: false })]);
    fireEvent.click(screen.getByLabelText('Concluir Item'));
    expect(patchSpy).toHaveBeenCalledWith({ id: 's1', patch: { done: true } });
  });

  it('fase 31: NÃO há mais a estrela is_goal por item', () => {
    renderSidebar([mkSub({ id: 's1', title: 'Item' })]);
    expect(screen.queryByLabelText('Marcar meta Item')).not.toBeInTheDocument();
  });

  it('cabeçalho conta feitas/total', () => {
    renderSidebar([
      mkSub({ id: 'a', done: true }),
      mkSub({ id: 'b', done: false }),
      mkSub({ id: 'c', done: true }),
    ]);
    expect(screen.getByText('2/3')).toBeInTheDocument();
  });

  it('can_write=false esconde add e os toggles (CA-25)', () => {
    renderSidebar([mkSub({ id: 's1', title: 'Item' })], false);
    expect(screen.queryByLabelText('Adicionar task-goal')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Concluir Item')).toBeDisabled();
  });
});
