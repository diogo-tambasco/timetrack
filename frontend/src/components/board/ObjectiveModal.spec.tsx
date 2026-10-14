import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ObjectiveModal from './ObjectiveModal';
import * as api from '../../lib/api';
import type { BoardObjective } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, createObjective: vi.fn(), patchObjective: vi.fn() };
});

function renderModal(objective: BoardObjective | null = null) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ObjectiveModal wsId="w1" projectId="p1" objective={objective} onClose={vi.fn()} />
    </QueryClientProvider>,
  );
}

const obj: BoardObjective = {
  id: 'o1', name: 'Antigo', description: 'desc', status: 'open', goal_hours: 12, position: 0,
  task_count: 0, done_count: 0, done_pct: 0, tracked_seconds: 0, tasks: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createObjective).mockResolvedValue({} as never);
  vi.mocked(api.patchObjective).mockResolvedValue({} as never);
});

describe('ObjectiveModal (CA-21/02)', () => {
  it('nome vazio NÃO chama a API', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: /criar objetivo/i }));
    expect(api.createObjective).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('goal_hours fora de (0,9999] NÃO chama a API', () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: 'X' } });
    fireEvent.change(screen.getByLabelText(/meta de horas/i), { target: { value: '10000' } });
    fireEvent.click(screen.getByRole('button', { name: /criar objetivo/i }));
    expect(api.createObjective).not.toHaveBeenCalled();
  });

  it('válido chama createObjective com os args certos', async () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: '  Lançar  ' } });
    fireEvent.change(screen.getByLabelText(/meta de horas/i), { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: /criar objetivo/i }));
    await waitFor(() =>
      expect(api.createObjective).toHaveBeenCalledWith('w1', 'p1', {
        name: 'Lançar',
        description: '',
        goal_hours: 12,
      }),
    );
  });

  it('modo Editar com meta vazia → patchObjective com goal_hours:null (remove)', async () => {
    renderModal(obj);
    fireEvent.change(screen.getByLabelText(/meta de horas/i), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() =>
      expect(api.patchObjective).toHaveBeenCalledWith('w1', 'p1', 'o1', {
        name: 'Antigo',
        description: 'desc',
        goal_hours: null,
      }),
    );
  });
});
