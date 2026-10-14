// GoalModal.spec.tsx — fase 29 T-09 (CA-13): New/Edit goal com seletor de modo.
// Trocar modo mostra/esconde goal_hours (by_hours) e target_date (by_date).
// Validação client antes do POST/PATCH; inválido NÃO chama a API. Edit pré-
// preenchido + PATCH só com o diff.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import GoalModal from './GoalModal';
import * as api from '../../lib/api';
import { mkObjective } from './testFixtures';
import type { BoardObjective } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, createObjective: vi.fn(), patchObjective: vi.fn() };
});

function renderModal(objective: BoardObjective | null = null, onClose = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <GoalModal wsId="w1" projectId="p1" objective={objective} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createObjective).mockResolvedValue({} as never);
  vi.mocked(api.patchObjective).mockResolvedValue({} as never);
});

describe('GoalModal (CA-13)', () => {
  it('modo by_tasks (default): goal_hours e target_date escondidos', () => {
    renderModal();
    expect(screen.queryByLabelText(/meta de horas/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/data do marco/i)).not.toBeInTheDocument();
  });

  it('escolher Por horas mostra goal_hours; Por data mostra target_date', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: /por horas/i }));
    expect(screen.getByLabelText(/meta de horas/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/data do marco/i)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /por data/i }));
    expect(screen.getByLabelText(/data do marco/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/meta de horas/i)).not.toBeInTheDocument();
  });

  it('nome vazio NÃO chama a API e sinaliza erro', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: /criar/i }));
    expect(api.createObjective).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('by_hours com goal_hours fora de faixa NÃO chama a API', () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: 'X' } });
    fireEvent.click(screen.getByRole('button', { name: /por horas/i }));
    fireEvent.change(screen.getByLabelText(/meta de horas/i), { target: { value: '10000' } });
    fireEvent.click(screen.getByRole('button', { name: /criar/i }));
    expect(api.createObjective).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('by_date com data vazia NÃO chama a API', () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: 'Marco' } });
    fireEvent.click(screen.getByRole('button', { name: /por data/i }));
    fireEvent.click(screen.getByRole('button', { name: /criar/i }));
    expect(api.createObjective).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('by_hours válido chama createObjective com completion_mode + goal_hours', async () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: '  Bater 40h  ' } });
    fireEvent.click(screen.getByRole('button', { name: /por horas/i }));
    fireEvent.change(screen.getByLabelText(/meta de horas/i), { target: { value: '40' } });
    fireEvent.click(screen.getByRole('button', { name: /criar/i }));
    await waitFor(() =>
      expect(api.createObjective).toHaveBeenCalledWith('w1', 'p1', {
        name: 'Bater 40h',
        description: '',
        completion_mode: 'by_hours',
        goal_hours: 40,
      }),
    );
  });

  it('by_date válido chama createObjective com completion_mode + target_date', async () => {
    renderModal();
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: 'Demo cliente' } });
    fireEvent.click(screen.getByRole('button', { name: /por data/i }));
    fireEvent.change(screen.getByLabelText(/data do marco/i), { target: { value: '2026-07-15' } });
    fireEvent.click(screen.getByRole('button', { name: /criar/i }));
    await waitFor(() =>
      expect(api.createObjective).toHaveBeenCalledWith('w1', 'p1', {
        name: 'Demo cliente',
        description: '',
        completion_mode: 'by_date',
        target_date: '2026-07-15',
      }),
    );
  });

  it('Editar pré-preenchido + PATCH só com o diff (só o nome)', async () => {
    const obj = mkObjective({
      id: 'o9', name: 'Antigo', description: 'desc', completion_mode: 'by_date',
      target_date: '2026-07-15', goal_hours: null,
    });
    renderModal(obj);
    // pré-preenchido: nome do objetivo presente; modo by_date → data visível.
    expect((screen.getByLabelText(/nome do objetivo/i) as HTMLInputElement).value).toBe('Antigo');
    expect(screen.getByLabelText(/data do marco/i)).toBeInTheDocument();
    // troca só o nome → PATCH leva só { name }.
    fireEvent.change(screen.getByLabelText(/nome do objetivo/i), { target: { value: 'Novo nome' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));
    await waitFor(() =>
      expect(api.patchObjective).toHaveBeenCalledWith('w1', 'p1', 'o9', { name: 'Novo nome' }),
    );
  });
});
