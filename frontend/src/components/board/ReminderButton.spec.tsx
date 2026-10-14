// ReminderButton.spec — fase 20 T-09 (CA-16): "Lembrar" abre o picker; um preset
// (ou input válido) chama createReminder com o remind_at certo; input vazio não
// chama a API. Mocka ../../lib/api.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReminderButton from './ReminderButton';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, createReminder: vi.fn() };
});

function renderBtn(dueDate: string | null = null) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <ReminderButton wsId="ws-1" taskId="task-1" dueDate={dueDate} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createReminder).mockResolvedValue({
    id: 'r-1',
    user_id: 'u-1',
    task_id: 'task-1',
    workspace_id: 'ws-1',
    remind_at: '2026-06-25T09:00:00Z',
    kind: 'manual',
    status: 'pending',
  });
});

describe('ReminderButton (fase 20)', () => {
  it('preset "Em 1h" chama createReminder com um remind_at ~1h no futuro', async () => {
    renderBtn();
    fireEvent.click(screen.getByRole('button', { name: 'Lembrar' }));
    fireEvent.click(screen.getByRole('button', { name: 'Em 1h' }));

    await waitFor(() => expect(api.createReminder).toHaveBeenCalledTimes(1));
    const [ws, task, at] = vi.mocked(api.createReminder).mock.calls[0];
    expect(ws).toBe('ws-1');
    expect(task).toBe('task-1');
    const delta = new Date(at).getTime() - Date.now();
    expect(delta).toBeGreaterThan(50 * 60 * 1000); // > 50min
    expect(delta).toBeLessThan(70 * 60 * 1000); // < 70min
  });

  it('preset "No prazo" só aparece com due_date e usa 09:00 do dia', async () => {
    renderBtn('2026-06-25');
    fireEvent.click(screen.getByRole('button', { name: 'Lembrar' }));
    fireEvent.click(screen.getByRole('button', { name: 'No prazo' }));

    await waitFor(() => expect(api.createReminder).toHaveBeenCalledTimes(1));
    const at = new Date(vi.mocked(api.createReminder).mock.calls[0][2]);
    expect(at.getFullYear()).toBe(2026);
    expect(at.getMonth()).toBe(5); // junho (0-based)
    expect(at.getDate()).toBe(25);
    expect(at.getHours()).toBe(9); // 09:00 local
  });

  it('sem due_date, o preset "No prazo" não aparece', () => {
    renderBtn(null);
    fireEvent.click(screen.getByRole('button', { name: 'Lembrar' }));
    expect(screen.queryByRole('button', { name: 'No prazo' })).toBeNull();
  });

  it('input vazio não chama a API e mostra erro', async () => {
    renderBtn();
    fireEvent.click(screen.getByRole('button', { name: 'Lembrar' }));
    // limpa o input de data/hora
    fireEvent.change(screen.getByLabelText('Quando lembrar'), { target: { value: '' } });
    // o botão "Lembrar" do picker (o segundo)
    const lembrarBtns = screen.getAllByRole('button', { name: 'Lembrar' });
    fireEvent.click(lembrarBtns[lembrarBtns.length - 1]);

    expect(api.createReminder).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/data\/hora válida/i);
  });
});
