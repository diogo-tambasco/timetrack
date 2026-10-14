// RemoteControl.spec — fase 23 T-02 (ocioso): descrição + seletor + recentes +
// Iniciar. O start é a ação do timerStore (spy), com os args do contrato; tocar
// um chip recente muda o project_id; sem recentes/projetos a fila some. Mock
// FIEL a args (padrão TimerBar.spec): listProjects responde por opts.archived.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import RemoteControl from './RemoteControl';
import { useTimerStore, initialTimerState } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Project, TimeEntry, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listProjects: vi.fn(), listEntries: vi.fn() };
});

const ws = { id: 'w1', name: 'WS de Ana' };

function proj(id: string, name: string, color = '#4f9dde', archived = false): Project {
  return {
    id, workspace_id: ws.id, client_id: null, name, color,
    weekly_hours_goal: null, visibility: 'shared', owner_user_id: 'u1',
    archived_at: archived ? '2026-06-09T00:00:00Z' : null, created_at: '2026-06-01T10:00:00Z',
  };
}

function entry(project_id: string | null, started_at: string): TimeEntry {
  return {
    id: `e-${project_id ?? 'x'}-${started_at}`, workspace_id: ws.id, project_id,
    task_id: null, description: '', tags: [], started_at, stopped_at: null,
    duration_seconds: null, source: 'timer',
  };
}

// useRecentEntries devolve o envelope EntriesList ({ entries, total_seconds })
function entriesList(entries: TimeEntry[]) {
  return { entries, total_seconds: 0 };
}

function rodando(mode: TimerState['mode'] = 'countup', project_id: string | null = 'p1'): TimerState {
  return {
    mode,
    entry: {
      id: 'e1', workspace_id: ws.id, project_id, task_id: null,
      description: 'Revisão lote 14', tags: [],
      started_at: '2026-06-10T14:00:00Z', stopped_at: null,
      duration_seconds: null, source: 'timer',
    },
    elapsed_seconds: 5070,
    target_seconds: mode === 'countdown' ? 6000 : null,
    remaining_seconds: mode === 'countdown' ? 930 : null,
  };
}

function renderControl(canWrite = true) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<RemoteControl ws={ws} canWrite={canWrite} />, { wrapper });
}

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState });
  vi.mocked(api.listProjects).mockResolvedValue([]);
  vi.mocked(api.listEntries).mockResolvedValue(entriesList([]));
});

describe('RemoteControl — ocioso (CA-04/06/06.1)', () => {
  it('Iniciar chama start com workspace_id + project_id selecionado, e limpa a descrição', async () => {
    vi.mocked(api.listProjects).mockResolvedValue([proj('p1', 'Lauda')]);
    const start = vi.fn(async () => {});
    useTimerStore.setState({ start });
    renderControl();

    await screen.findByRole('option', { name: 'Lauda' });
    const desc = screen.getByTestId('remote-description') as HTMLInputElement;
    fireEvent.change(desc, { target: { value: 'Nova frente' } });
    fireEvent.change(screen.getByLabelText(/projeto/i), { target: { value: 'p1' } });
    fireEvent.click(screen.getByRole('button', { name: /iniciar/i }));

    expect(start).toHaveBeenCalledWith(
      expect.objectContaining({
        workspace_id: 'w1',
        description: 'Nova frente',
        project_id: 'p1',
        task_id: null,
        tags: [],
      }),
    );
    expect(desc.value).toBe(''); // descrição limpa após o start (espelha IdleBar)
  });

  it('sem projeto selecionado → start com project_id null', async () => {
    const start = vi.fn(async () => {});
    useTimerStore.setState({ start });
    renderControl();

    fireEvent.click(screen.getByRole('button', { name: /iniciar/i }));
    expect(start).toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: 'w1', project_id: null }),
    );
  });

  it('tocar um chip recente seleciona o project_id do próximo start', async () => {
    vi.mocked(api.listProjects).mockImplementation(async (_ws, opts) =>
      opts?.archived ? [proj('p1', 'Lauda'), proj('p2', 'Antigo', '#e2b714', true)] : [proj('p1', 'Lauda')],
    );
    vi.mocked(api.listEntries).mockResolvedValue(entriesList([
      entry('p2', '2026-06-22T10:00:00Z'),
      entry('p1', '2026-06-22T09:00:00Z'),
    ]));
    const start = vi.fn(async () => {});
    useTimerStore.setState({ start });
    renderControl();

    // chip do projeto arquivado p2 ainda aparece (CA-06.1)
    const chipP2 = await screen.findByTestId('remote-recent-p2');
    expect(chipP2).toHaveTextContent('Antigo');
    fireEvent.click(chipP2);

    fireEvent.click(screen.getByRole('button', { name: /iniciar/i }));
    expect(start).toHaveBeenCalledWith(expect.objectContaining({ project_id: 'p2' }));
  });

  it('sem entries recentes / sem projetos → a fila de recentes some, seletor "Sem projeto"', async () => {
    renderControl();
    await waitFor(() => expect(screen.getByRole('button', { name: /iniciar/i })).toBeInTheDocument());
    expect(screen.queryByTestId('remote-recents')).toBeNull();
    expect(screen.getByRole('option', { name: 'Sem projeto' })).toBeInTheDocument();
  });

  it('ids recentes sem projeto resolvível são omitidos da fila (não quebram)', async () => {
    vi.mocked(api.listProjects).mockResolvedValue([proj('p1', 'Lauda')]);
    vi.mocked(api.listEntries).mockResolvedValue(entriesList([
      entry('p-fantasma', '2026-06-22T10:00:00Z'), // sem projeto resolvível
      entry('p1', '2026-06-22T09:00:00Z'),
    ]));
    renderControl();
    await screen.findByTestId('remote-recent-p1');
    expect(screen.queryByTestId('remote-recent-p-fantasma')).toBeNull();
  });
});

describe('RemoteControl — rodando (CA-05/07/12/13)', () => {
  it('Parar chama useTimerStore.getState().stop (spy) — não fetch direto', () => {
    const stop = vi.fn(async () => {});
    useTimerStore.setState({ state: rodando(), anchorMs: Date.now(), offsetMs: 0, stop });
    renderControl();

    fireEvent.click(screen.getByRole('button', { name: /parar/i }));
    expect(stop).toHaveBeenCalledTimes(1);
  });

  it('Pausar chama a ação pause do store', () => {
    const pause = vi.fn(async () => {});
    useTimerStore.setState({ state: rodando(), anchorMs: Date.now(), offsetMs: 0, pause });
    renderControl();

    fireEvent.click(screen.getByRole('button', { name: /pausar/i }));
    expect(pause).toHaveBeenCalledTimes(1);
  });

  it('os dígitos refletem o store (countup elapsed), não um contador próprio', () => {
    useTimerStore.setState({ state: rodando('countup'), anchorMs: Date.now(), offsetMs: 0 });
    renderControl();
    // 5070s = 1:24:30 (elapsed do store, sem delta porque anchor = agora)
    expect(screen.getByTestId('remote-digits')).toHaveTextContent('1:24:30');
  });

  it('countdown mostra o RESTANTE (remaining), expressão `shown` da TimerBar', () => {
    useTimerStore.setState({ state: rodando('countdown'), anchorMs: Date.now(), offsetMs: 0 });
    renderControl();
    // remaining_seconds 930 = 0:15:30
    expect(screen.getByTestId('remote-digits')).toHaveTextContent('0:15:30');
  });

  it('descrição read-only + chip do projeto na cor (archived:true)', async () => {
    vi.mocked(api.listProjects).mockImplementation(async (_ws, opts) =>
      opts?.archived ? [proj('p1', 'Lauda', '#4f9dde')] : [],
    );
    useTimerStore.setState({ state: rodando('countup', 'p1'), anchorMs: Date.now(), offsetMs: 0 });
    renderControl();

    const desc = screen.getByTestId('remote-description') as HTMLInputElement;
    expect(desc.value).toBe('Revisão lote 14');
    expect(desc.readOnly).toBe(true);

    const chip = await waitFor(() => {
      const el = document.querySelector('.tt-chip');
      expect(el).not.toBeNull();
      return el as HTMLElement;
    });
    expect(chip.textContent).toContain('Lauda');
    const dot = chip.querySelector('.tt-dot') as HTMLElement;
    expect(dot.style.background).toContain('rgb(79, 157, 222)'); // #4f9dde
  });
});

function pausa(): TimerState {
  return {
    mode: 'break',
    entry: null,
    elapsed_seconds: 0,
    target_seconds: 300,
    remaining_seconds: 280,
    break_kind: 'short',
    resume: { entry_id: 'e1', mode: 'pomodoro_focus', target_seconds: 1500 },
  };
}

describe('RemoteControl — pausa reusa BreakOverlay (CA-05.1/12)', () => {
  it('mode==="break" → monta o BreakOverlay (data-testid dele presente)', () => {
    useTimerStore.setState({ state: pausa(), anchorMs: Date.now(), offsetMs: 0 });
    renderControl();
    // o componente da pausa é o BreakOverlay reusado, não controles duplicados
    expect(screen.getByTestId('break-overlay')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retomar/i })).toBeInTheDocument();
  });
});

describe('RemoteControl — gating de viewer (CA-11): só o INICIAR é gateado', () => {
  // (a) viewer OCIOSO → não vê o controle de Iniciar (campo/seletor/recentes/botão)
  it('(a) viewer ocioso (state==null) → SEM Iniciar/campo/seletor/recentes', () => {
    vi.mocked(api.listProjects).mockResolvedValue([proj('p1', 'Lauda')]);
    vi.mocked(api.listEntries).mockResolvedValue(entriesList([entry('p1', '2026-06-22T10:00:00Z')]));
    renderControl(false); // canWrite=false (viewer)

    expect(screen.queryByRole('button', { name: /iniciar/i })).toBeNull();
    expect(screen.queryByTestId('remote-description')).toBeNull();
    expect(screen.queryByLabelText(/projeto/i)).toBeNull();
    expect(screen.queryByTestId('remote-recents')).toBeNull();
  });

  // (b) viewer COM timer rodando (iniciado em outro ws) → vê Parar/Pausar
  // (per-usuário, sem 403 — esconder prenderia o timer)
  it('(b) viewer com timer rodando → VÊ Parar e Pausar', () => {
    useTimerStore.setState({ state: rodando(), anchorMs: Date.now(), offsetMs: 0 });
    renderControl(false); // viewer
    expect(screen.getByRole('button', { name: /parar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pausar/i })).toBeInTheDocument();
  });

  // (b') viewer na PAUSA → vê retomar/pular do BreakOverlay (per-usuário)
  it("(b') viewer na pausa → vê o BreakOverlay (retomar/pular/parar)", () => {
    useTimerStore.setState({ state: pausa(), anchorMs: Date.now(), offsetMs: 0 });
    renderControl(false); // viewer
    expect(screen.getByTestId('break-overlay')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retomar/i })).toBeInTheDocument();
  });

  // (c) owner/member ocioso → vê Iniciar
  it('(c) member/owner ocioso → VÊ Iniciar', async () => {
    renderControl(true); // canWrite
    expect(await screen.findByRole('button', { name: /iniciar/i })).toBeInTheDocument();
    expect(screen.getByTestId('remote-description')).toBeInTheDocument();
  });
});
