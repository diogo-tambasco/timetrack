// TodayList.spec — T-17: lista "Hoje" com início, duração mono e continuar
// por linha; a rodando destacada em accent com stop (CA-35); a listagem
// SEMPRE envia o fuso IANA do navegador (CA-35.1, stub de fetch puro).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import TodayList from './TodayList';
import { useAuthStore } from '../../lib/auth';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { EntriesList, TimeEntry } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    listEntries: vi.fn(),
    listProjects: vi.fn(),
    continueEntry: vi.fn(),
    stopTimer: vi.fn(),
  };
});

const ws = 'w1';

function entry(over: Partial<TimeEntry>): TimeEntry {
  return {
    id: 'x', workspace_id: ws, project_id: null, task_id: null,
    description: 'algo', tags: [], started_at: '2026-06-10T09:00:00Z',
    stopped_at: '2026-06-10T09:45:00Z', duration_seconds: 2700, source: 'manual',
    ...over,
  };
}

function renderLista() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TodayList />
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
  vi.mocked(api.listProjects).mockResolvedValue([]);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('TodayList', () => {
  // CA-35 — 3 entries (1 rodando): início + duração mono + continuar nas
  // fechadas; a rodando em accent com stop e SEM continuar; total no topo.
  it('render', async () => {
    const rodandoHa1500s = new Date(Date.now() - 1_500_000).toISOString();
    const lista: EntriesList = {
      entries: [
        entry({ id: 'e3', description: 'rodando agora', started_at: rodandoHa1500s, stopped_at: null, duration_seconds: null, source: 'timer' }),
        entry({ id: 'e2', description: 'reunião', duration_seconds: 1800 }),
        entry({ id: 'e1', description: 'deep work', duration_seconds: 3600 }),
      ],
      total_seconds: 5400,
    };
    vi.mocked(api.listEntries).mockResolvedValue(lista);
    renderLista();

    expect(await screen.findByText('rodando agora')).toBeInTheDocument();
    expect(screen.getByText('reunião')).toBeInTheDocument();
    expect(screen.getByText('deep work')).toBeInTheDocument();

    // duração mono nas fechadas
    expect(screen.getByText('0:30:00')).toBeInTheDocument();
    expect(screen.getByText('1:00:00')).toBeInTheDocument();

    // continuar por linha SÓ nas fechadas; a rodando tem stop
    expect(screen.getAllByRole('button', { name: /continuar/i })).toHaveLength(2);
    const rodando = screen.getByTestId('entry-e3');
    expect(rodando.querySelector('button[aria-label="Parar"]')).not.toBeNull();
    expect(rodando.querySelector('button[aria-label="Continuar"]')).toBeNull();

    // destaque accent + duração viva derivada do started_at (~25min)
    expect(rodando.textContent).toMatch(/0:25:0\d/);
    const duracaoViva = rodando.querySelector('[data-live-duration]') as HTMLElement;
    expect(duracaoViva.style.color).toBe('var(--accent)');

    // total do dia no cabeçalho: 5400 fechados + 1500 vivos = 1:55:0x
    expect(screen.getByText(/total hoje/i)).toBeInTheDocument();
    expect(screen.getByText(/1:55:0\d/)).toBeInTheDocument();
  });

  it('continuar dispara o continue e aplica o timer retornado', async () => {
    vi.mocked(api.listEntries).mockResolvedValue({
      entries: [entry({ id: 'e1', description: 'deep work' })],
      total_seconds: 2700,
    });
    vi.mocked(api.continueEntry).mockResolvedValue({
      state: {
        mode: 'countup',
        entry: entry({ id: 'e9', description: 'deep work', stopped_at: null, duration_seconds: null, source: 'timer' }),
        elapsed_seconds: 0,
        target_seconds: null,
        remaining_seconds: null,
      },
      server_time: new Date().toISOString(),
    });
    renderLista();

    fireEvent.click(await screen.findByRole('button', { name: /continuar/i }));

    await waitFor(() => expect(api.continueEntry).toHaveBeenCalledWith(ws, 'e1'));
    expect(useTimerStore.getState().state?.entry?.id).toBe('e9');
  });

  // CA-35.1 — a listagem envia tz = Intl.DateTimeFormat().resolvedOptions()
  // .timeZone do navegador (provado com stub de fetch PURO, sem mock da lib).
  it('timezone', async () => {
    const { listEntries } = await vi.importActual<typeof import('../../lib/api')>('../../lib/api');
    const fetchMock = vi.fn(
      async () =>
        new Response(JSON.stringify({ entries: [], total_seconds: 0 }), { status: 200 }),
    );
    vi.stubGlobal('fetch', fetchMock);

    await listEntries(ws, { tz: Intl.DateTimeFormat().resolvedOptions().timeZone });

    const url = String(fetchMock.mock.calls[0][0]);
    const tz = new URL(url, 'http://local').searchParams.get('tz');
    expect(tz).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);

    // e o componente usa exatamente esse fuso na query do dia
    vi.unstubAllGlobals();
    vi.mocked(api.listEntries).mockResolvedValue({ entries: [], total_seconds: 0 });
    renderLista();
    await waitFor(() =>
      expect(api.listEntries).toHaveBeenCalledWith(
        ws,
        expect.objectContaining({ tz: Intl.DateTimeFormat().resolvedOptions().timeZone }),
      ),
    );
  });
});
