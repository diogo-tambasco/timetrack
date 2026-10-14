// EntryModal.spec — T-18: formulário inválido não chama a API; criar envia
// POST com o payload do contrato (§3.6); editar envia PATCH só com o diff.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import EntryModal from './EntryModal';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';
import type { TimeEntry } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listProjects: vi.fn(),
    listTasks: vi.fn(),
    createEntry: vi.fn(),
    patchEntry: vi.fn(),
  };
});

const ws = 'w1';

const salva: TimeEntry = {
  id: 'e1', workspace_id: ws, project_id: null, task_id: null,
  description: 'Call Arkan', tags: ['call'],
  started_at: new Date('2026-06-10T09:00').toISOString(),
  stopped_at: new Date('2026-06-10T09:45').toISOString(),
  duration_seconds: 2700, source: 'manual',
};

function renderModal(entry: TimeEntry | null = null, onClose = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <EntryModal wsId={ws} entry={entry} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listProjects).mockResolvedValue([]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
});

describe('EntryModal', () => {
  it('fim antes do início NÃO chama a API e mostra o erro', async () => {
    renderModal();

    fireEvent.change(screen.getByLabelText(/início/i), { target: { value: '2026-06-10T10:00' } });
    fireEvent.change(screen.getByLabelText(/fim/i), { target: { value: '2026-06-10T09:00' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.createEntry).not.toHaveBeenCalled();
    expect(api.patchEntry).not.toHaveBeenCalled();
    expect(await screen.findByText(/o fim deve ser depois do início/i)).toBeInTheDocument();
  });

  it('horários ausentes NÃO chamam a API', () => {
    renderModal();

    fireEvent.change(screen.getByLabelText(/descrição/i), { target: { value: 'sem horário' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.createEntry).not.toHaveBeenCalled();
  });

  it('criar envia POST com o payload do contrato', async () => {
    vi.mocked(api.createEntry).mockResolvedValue(salva);
    const onClose = renderModal();

    fireEvent.change(screen.getByLabelText(/descrição/i), { target: { value: 'Call Arkan' } });
    fireEvent.change(screen.getByLabelText(/tags/i), { target: { value: 'call, arkan' } });
    fireEvent.change(screen.getByLabelText(/início/i), { target: { value: '2026-06-10T09:00' } });
    fireEvent.change(screen.getByLabelText(/fim/i), { target: { value: '2026-06-10T09:45' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.createEntry).toHaveBeenCalledWith(ws, {
        description: 'Call Arkan',
        project_id: null,
        task_id: null,
        tags: ['call', 'arkan'],
        started_at: new Date('2026-06-10T09:00').toISOString(),
        stopped_at: new Date('2026-06-10T09:45').toISOString(),
      }),
    );
    expect(onClose).toHaveBeenCalled();
  });

  // F2 da verificação adversarial: entries do timer têm SEGUNDOS nos
  // horários (now() do servidor); o round-trip do datetime-local trunca ao
  // minuto e gerava diff FALSO — editar a descrição deslocava os horários
  // e encolhia a duração silenciosamente.
  it('editar só a descrição de entry com segundos não toca os horários', async () => {
    const doTimer: TimeEntry = {
      ...salva,
      started_at: '2026-06-10T14:00:37.000Z',
      stopped_at: '2026-06-10T14:25:50.000Z',
      duration_seconds: 1513,
      source: 'timer',
    };
    vi.mocked(api.patchEntry).mockResolvedValue({ ...doTimer, description: 'ajustada' });
    renderModal(doTimer);

    fireEvent.change(screen.getByLabelText(/descrição/i), { target: { value: 'ajustada' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.patchEntry).toHaveBeenCalledWith(ws, 'e1', { description: 'ajustada' }),
    );
  });

  it('editar abre pré-preenchido e envia PATCH só com o diff', async () => {
    vi.mocked(api.patchEntry).mockResolvedValue({ ...salva, duration_seconds: 5400 });
    renderModal(salva);

    expect(screen.getByLabelText(/descrição/i)).toHaveValue('Call Arkan');

    fireEvent.change(screen.getByLabelText(/fim/i), { target: { value: '2026-06-10T10:30' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.patchEntry).toHaveBeenCalledWith(ws, 'e1', {
        stopped_at: new Date('2026-06-10T10:30').toISOString(),
      }),
    );
  });
});
