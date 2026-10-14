// FocoPicker.spec — fase 15 T-06 (CA-09/10/11/12/13): a tela de entrada do
// Foco renderiza campo + recentes + continuar; cartões ≥80px com
// aria-pressed; cliente do cache aparece (degrada pra cor+nome sem cliente);
// Começar chama start com o contexto certo; Continuar chama continueEntry.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import FocoPicker from './FocoPicker';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Client, EntriesList, Project, TimerResult } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listProjects: vi.fn(),
    listClients: vi.fn(),
    listEntries: vi.fn(),
    startTimer: vi.fn(),
    continueEntry: vi.fn(),
    getTimer: vi.fn(),
  };
});

const ws = 'w1';

const projetos: Project[] = [
  {
    id: 'p1', workspace_id: ws, client_id: 'c1', name: 'Lauda', color: '#4f9dde',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-01T10:00:00Z',
  },
  {
    id: 'p2', workspace_id: ws, client_id: null, name: 'Interno', color: '#e0a24f',
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: null, created_at: '2026-06-01T10:00:00Z',
  },
];

const clientes: Client[] = [
  { id: 'c1', workspace_id: ws, name: 'Cliente X', archived_at: null, created_at: '2026-06-01T10:00:00Z' },
];

function entriesList(): EntriesList {
  return {
    total_seconds: 3600,
    entries: [
      {
        id: 'e-fechada', workspace_id: ws, project_id: 'p1', task_id: null,
        description: 'Revisão de laudas', tags: [], started_at: '2026-06-10T13:00:00Z',
        stopped_at: '2026-06-10T13:30:00Z', duration_seconds: 1800, source: 'timer',
      },
    ],
  };
}

const okResult: TimerResult = { state: null, server_time: '2026-06-10T14:00:00Z' };

function renderPicker() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <FocoPicker workspaceId={ws} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  vi.mocked(api.listProjects).mockResolvedValue(projetos);
  vi.mocked(api.listClients).mockResolvedValue(clientes);
  vi.mocked(api.listEntries).mockResolvedValue(entriesList());
  vi.mocked(api.startTimer).mockResolvedValue(okResult);
  vi.mocked(api.continueEntry).mockResolvedValue(okResult);
});

// CA-09 — renderiza título, campo de descrição, recentes e continuar.
it('renderiza campo, projetos recentes e continuar (CA-09)', async () => {
  renderPicker();
  expect(screen.getByText('No que você tá trabalhando?')).toBeInTheDocument();
  expect(screen.getByTestId('focus-picker-description')).toBeInTheDocument();
  await waitFor(() => expect(screen.getAllByTestId('focus-picker-project').length).toBeGreaterThan(0));
  await waitFor(() => expect(screen.getByTestId('focus-picker-continue')).toBeInTheDocument());
});

// CA-10 — cartões grandes (≥80px) com aria-pressed; selecionar marca pressed.
it('cartões de projeto têm alvo ≥80px e marcam aria-pressed ao selecionar (CA-10)', async () => {
  renderPicker();
  const cards = await screen.findAllByTestId('focus-picker-project');
  expect(cards).toHaveLength(2);
  for (const c of cards) {
    expect(Number.parseInt((c as HTMLElement).style.minHeight, 10)).toBeGreaterThanOrEqual(80);
    expect(c).toHaveAttribute('aria-pressed', 'false');
  }
  fireEvent.click(cards[0]);
  expect(cards[0]).toHaveAttribute('aria-pressed', 'true');
});

// CA-10 — nome do cliente do cache aparece; sem cliente, só cor+nome.
it('mostra o cliente do cache no cartão; sem cliente, só cor+nome (CA-10)', async () => {
  renderPicker();
  await screen.findAllByTestId('focus-picker-project');
  // p1 tem client_id c1 → "Cliente X" aparece
  await waitFor(() => expect(screen.getByText('Cliente X')).toBeInTheDocument());
  // p2 sem cliente: o nome do projeto aparece, mas não há linha de cliente extra
  expect(screen.getByText('Interno')).toBeInTheDocument();
  const clientLines = screen.getAllByTestId('focus-picker-client');
  expect(clientLines).toHaveLength(1); // só p1
});

// CA-11 — Começar chama start com description + project_id selecionado.
it('Começar chama start com a descrição e o projeto selecionado (CA-11)', async () => {
  renderPicker();
  const cards = await screen.findAllByTestId('focus-picker-project');
  fireEvent.change(screen.getByTestId('focus-picker-description'), {
    target: { value: 'Nova sessão' },
  });
  fireEvent.click(cards[0]); // p1
  fireEvent.click(screen.getByTestId('focus-picker-start'));
  await waitFor(() => expect(api.startTimer).toHaveBeenCalledTimes(1));
  expect(api.startTimer).toHaveBeenCalledWith(
    expect.objectContaining({ workspace_id: ws, description: 'Nova sessão', project_id: 'p1' }),
  );
});

// CA-11 — Começar desabilitado sem descrição E sem projeto.
it('Começar fica desabilitado sem descrição e sem projeto (CA-11)', async () => {
  renderPicker();
  await screen.findAllByTestId('focus-picker-project');
  const startBtn = screen.getByTestId('focus-picker-start');
  expect(startBtn).toBeDisabled();
  fireEvent.change(screen.getByTestId('focus-picker-description'), { target: { value: 'x' } });
  expect(startBtn).not.toBeDisabled();
});

// CA-12 — Continuar chama continueEntry daquela entry.
it('Continuar chama continueEntry da entrada (CA-12)', async () => {
  renderPicker();
  const item = await screen.findByTestId('focus-picker-continue');
  fireEvent.click(item);
  await waitFor(() => expect(api.continueEntry).toHaveBeenCalledWith(ws, 'e-fechada'));
});

// CA-12 — sem entries fechadas, a seção Continuar não aparece.
it('sem entradas fechadas, a seção Continuar não aparece (CA-12)', async () => {
  vi.mocked(api.listEntries).mockResolvedValue({ total_seconds: 0, entries: [] });
  renderPicker();
  await screen.findAllByTestId('focus-picker-project');
  expect(screen.queryByTestId('focus-picker-continue')).not.toBeInTheDocument();
  expect(screen.queryByText('Continuar')).not.toBeInTheDocument();
});
