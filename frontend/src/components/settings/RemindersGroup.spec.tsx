// RemindersGroup.spec — fase 20 T-09 (CA-18/20): lista os pendentes do
// listReminders mockado; cancelar chama cancelReminder e a lista invalida.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import RemindersGroup from './RemindersGroup';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listReminders: vi.fn(), cancelReminder: vi.fn() };
});

function renderGrupo() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <RemindersGroup />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listReminders).mockResolvedValue([
    {
      id: 'r-1',
      user_id: 'u-1',
      task_id: 't-1',
      workspace_id: 'ws-1',
      remind_at: '2026-06-25T12:00:00Z',
      kind: 'manual',
      status: 'pending',
      task_name: 'Entregar relatório',
    },
  ]);
  vi.mocked(api.cancelReminder).mockResolvedValue(undefined as unknown as void);
});

describe('RemindersGroup (fase 20)', () => {
  it('lista os lembretes pendentes com o nome da task', async () => {
    renderGrupo();
    expect(await screen.findByText('Entregar relatório')).toBeTruthy();
    expect(screen.getByTestId('reminder-item')).toBeTruthy();
  });

  it('cancelar chama cancelReminder com ws/task/id', async () => {
    renderGrupo();
    await screen.findByText('Entregar relatório');
    fireEvent.click(screen.getByRole('button', { name: /cancelar lembrete de Entregar relatório/i }));

    await waitFor(() => expect(api.cancelReminder).toHaveBeenCalledTimes(1));
    expect(api.cancelReminder).toHaveBeenCalledWith('ws-1', 't-1', 'r-1');
  });

  it('lista vazia mostra o estado vazio', async () => {
    vi.mocked(api.listReminders).mockResolvedValue([]);
    renderGrupo();
    expect(await screen.findByTestId('reminders-empty')).toBeTruthy();
  });
});
