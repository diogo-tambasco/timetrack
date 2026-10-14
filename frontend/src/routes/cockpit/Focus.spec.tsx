// Focus.spec — T-04/T-05 da fase 05: o Modo Foco real (CA-03..CA-13).
// Padrão das páginas da 02/03: mock de ../../lib/api + QueryClientProvider
// + useAuthStore.setState + estado semeado direto no useTimerStore.
// O CA-03 tem o teste de integridade CANÔNICO do D-03 (readFileSync do
// tokens.css — jsdom não carrega CSS, o assert via style só prova a
// referência; precedente palette.spec.ts).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import Focus from './Focus';
import { useAuthStore } from '../../lib/auth';
import { initialTimerState, useTimerStore } from '../../lib/timerStore';
import * as api from '../../lib/api';
import type { Project, TimeEntry, TimerState } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, listProjects: vi.fn(), listEntries: vi.fn(), listClients: vi.fn() };
});

const ws = 'w1';

// ARQUIVADO de propósito: o CA-06 exige o nome resolvido inclusive pra
// projeto arquivado (mesma regra archived:true do chip da TimerBar)
const projeto: Project = {
  id: 'p1',
  workspace_id: ws,
  client_id: null,
  name: 'Lauda Clínicas',
  color: '#b07cd6',
  weekly_hours_goal: null,
  visibility: 'shared',
  owner_user_id: 'u1',
  archived_at: '2026-06-11T10:00:00Z',
  created_at: '2026-06-01T10:00:00Z',
};

function entry(over: Partial<TimeEntry> = {}): TimeEntry {
  return {
    id: 'e1',
    workspace_id: ws,
    project_id: null,
    task_id: null,
    description: '',
    tags: [],
    started_at: new Date().toISOString(),
    stopped_at: null,
    duration_seconds: null,
    source: 'timer',
    ...over,
  };
}

function seedTimer(state: TimerState | null) {
  useTimerStore.setState({
    state,
    anchorMs: Date.now(),
    offsetMs: 0,
    tick: 0,
    firedKey: null,
    transitioning: false,
  });
}

function pausa(remaining: number, resumeMode: 'pomodoro_focus' | 'countup'): TimerState {
  return {
    mode: 'break',
    entry: null,
    elapsed_seconds: 300 - remaining,
    target_seconds: 300,
    remaining_seconds: remaining,
    resume: {
      entry_id: 'e1',
      mode: resumeMode,
      target_seconds: resumeMode === 'countup' ? null : 900,
    },
  };
}

// ações originais do store — specs que espionam restauram via beforeEach
const acoes = {
  start: useTimerStore.getState().start,
  stop: useTimerStore.getState().stop,
  pause: useTimerStore.getState().pause,
  resume: useTimerStore.getState().resume,
  skip: useTimerStore.getState().skip,
};

function renderFoco() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/foco']}>
        <Routes>
          <Route path="/foco" element={<Focus />} />
          <Route path="/hoje" element={<div>rota-hoje</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const flush = () => act(async () => {});

beforeEach(() => {
  vi.clearAllMocks();
  useTimerStore.setState({ ...initialTimerState, ...acoes });
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: ws, name: 'Workspace de Ana', role: 'owner' }],
    },
  });
  // mock FIEL a args (padrão TimerBar.spec, correção da verificação
  // adversarial do build): o fixture é ARQUIVADO — sem {archived:true} a
  // API real não o devolveria; o mock incondicional deixava a mutação
  // "remove o parâmetro" passar 113/113
  vi.mocked(api.listProjects).mockImplementation(async (_ws, opts) =>
    opts?.archived ? [projeto] : [],
  );
  vi.mocked(api.listEntries).mockResolvedValue({ entries: [], total_seconds: 0 });
  vi.mocked(api.listClients).mockResolvedValue([]);
});

// CA-03 — teste de integridade CANÔNICO (D-03): os --focus-* moram no
// :root e NÃO são redeclarados em nenhum outro bloco (tema light incluso).
describe('tokens --focus-* (CA-03, integridade do tokens.css)', () => {
  it('declarados no :root e ausentes de qualquer bloco de tema', () => {
    // lido do disco — precedente do palette.spec (jsdom: import.meta.url
    // não é file:; cwd da suíte é frontend/)
    const css = readFileSync(path.resolve(process.cwd(), 'src/styles/tokens.css'), 'utf8');
    const rootBlock = css.slice(css.indexOf(':root'), css.indexOf('}', css.indexOf(':root')) + 1);
    const declsIn = (s: string, name: string) =>
      (s.match(new RegExp(`${name}\\s*:`, 'g')) ?? []).length;

    for (const token of ['--focus-bg', '--focus-ink', '--focus-dim']) {
      expect(declsIn(rootBlock, token), `${token} declarado no :root`).toBe(1);
      expect(declsIn(css, token), `${token} declarado SÓ no :root (tema não sobrescreve)`).toBe(1);
    }
  });
});

describe('Modo Foco — render dos estados', () => {
  // CA-03 (complementar) — o overlay referencia os tokens --focus-*
  it('overlay usa os tokens --focus-* (fundo preto independente do tema)', () => {
    seedTimer(null);
    renderFoco();
    const overlay = screen.getByTestId('focus');
    expect(overlay.style.background).toBe('var(--focus-bg)');
    expect(overlay.style.color).toBe('var(--focus-ink)');
  });

  // CA-04 — countup mostra o elapsed derivado localmente, avançando por tick
  it('countup mostra elapsed e avança com o tick (derivação local)', () => {
    seedTimer({
      mode: 'countup',
      entry: entry({ description: 'foco' }),
      elapsed_seconds: 61,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();
    expect(screen.getByTestId('focus-digits').textContent).toBe('0:01:01');

    // o "tick": 5s passam no relógio do servidor (offset) + bump de tick
    act(() => {
      useTimerStore.setState({ offsetMs: 5000, tick: 1 });
    });
    expect(screen.getByTestId('focus-digits').textContent).toBe('0:01:06');
  });

  // CA-05 — countdown/pomodoro mostram o restante (regra shown da TimerBar)
  it('countdown mostra o remaining, não o elapsed', () => {
    seedTimer({
      mode: 'countdown',
      entry: entry(),
      elapsed_seconds: 10,
      target_seconds: 60,
      remaining_seconds: 50,
    });
    renderFoco();
    expect(screen.getByTestId('focus-digits').textContent).toBe('0:00:50');
  });

  // CA-05 (braço pomodoro — correção da verificação adversarial do build):
  // foco pomodoro RODANDO também mostra o restante, nunca o elapsed
  it('pomodoro rodando mostra o remaining (braço pomodoro da regra shown)', () => {
    seedTimer({
      mode: 'pomodoro',
      entry: entry(),
      elapsed_seconds: 60,
      target_seconds: 1500,
      remaining_seconds: 1440,
    });
    renderFoco();
    expect(screen.getByTestId('focus-digits').textContent).toBe('0:24:00');
  });

  // CA-06 — projeto (inclusive arquivado) > descrição > nada
  it('mostra o nome do projeto (inclusive arquivado)', async () => {
    seedTimer({
      mode: 'countup',
      entry: entry({ project_id: 'p1', description: 'qualquer' }),
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();
    await waitFor(() =>
      expect(screen.getByTestId('focus-context').textContent).toBe('Lauda Clínicas'),
    );
  });

  it('sem projeto mostra a descrição', async () => {
    seedTimer({
      mode: 'countup',
      entry: entry({ description: 'Revisão de laudas' }),
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();
    await flush();
    expect(screen.getByTestId('focus-context').textContent).toBe('Revisão de laudas');
  });

  it('sem projeto e sem descrição não mostra contexto nenhum', async () => {
    seedTimer({
      mode: 'countup',
      entry: entry(),
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();
    await flush();
    expect(screen.queryByTestId('focus-context')).toBeNull();
  });

  // CA-07 — pausa vira modo descanso na própria rota: countdown + rótulo +
  // retomar/pular/parar (vale igualmente pra pausa vinda de outro device:
  // o estado chega pelo MESMO store)
  it('break vira modo descanso (FocoBreak) com countdown, rótulo e controles', () => {
    seedTimer(pausa(300, 'countup'));
    renderFoco();

    // FocoBreak (fase 15) rotula a pausa curta como "Pausa Pomodoro"
    expect(screen.getByText('Pausa Pomodoro')).toBeInTheDocument();
    expect(screen.getByTestId('focus-digits').textContent).toBe('05:00');
    expect(screen.getByRole('button', { name: 'Retomar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pular pausa/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Parar' })).toBeInTheDocument();
  });

  // CA-08 — pausa zerada de countup espera o usuário: rótulo "encerrada",
  // sem auto-ação do componente (auto-resume é SÓ do ciclo pomodoro e mora
  // no timerStore)
  it('pausa zerada de countup mostra "Pausa encerrada" sem auto-ação', () => {
    const resumeSpy = vi.fn();
    useTimerStore.setState({ resume: resumeSpy });
    seedTimer(pausa(0, 'countup'));
    renderFoco();

    expect(screen.getByText('Pausa encerrada')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pular pausa/i })).toBeNull();
    expect(resumeSpy).not.toHaveBeenCalled();
  });

  // CA-09 (fase 15 — desvio sancionado da fase 05, spec §7): ocioso = o
  // FocoPicker (tela de entrada com start), NÃO mais "0:00:00 + sem timer".
  // O contrato dos OUTROS estados (rodando/descanso/rodapé/wake lock) segue
  // assertado nos demais testes deste arquivo.
  it('ocioso renderiza o FocoPicker (não o 0:00:00 da fase 05)', async () => {
    seedTimer(null);
    renderFoco();

    await waitFor(() => expect(screen.getByTestId('focus-picker')).toBeInTheDocument());
    expect(screen.getByText('No que você tá trabalhando?')).toBeInTheDocument();
    expect(screen.getByTestId('focus-picker-start')).toBeInTheDocument();
    // o ocioso antigo (dígitos esmaecidos + texto) não existe mais
    expect(screen.queryByText('sem timer rodando')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Pausar' })).toBeNull();
  });
});

describe('Modo Foco — controles', () => {
  // CA-11 — Pausar/Parar saem pelas ações do store (e portanto pelo sync)
  it('Pausar e Parar chamam as ações do store', () => {
    const pauseSpy = vi.fn();
    const stopSpy = vi.fn();
    useTimerStore.setState({ pause: pauseSpy, stop: stopSpy });
    seedTimer({
      mode: 'countup',
      entry: entry({ description: 'foco' }),
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();

    fireEvent.click(screen.getByRole('button', { name: 'Pausar' }));
    expect(pauseSpy).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
    expect(stopSpy).toHaveBeenCalledTimes(1);
  });

  // CA-12 — retomar/pular/parar no descanso = resume/skip/stop do store
  it('Retomar, pular pausa e Parar chamam resume/skip/stop', () => {
    const resumeSpy = vi.fn();
    const skipSpy = vi.fn();
    const stopSpy = vi.fn();
    useTimerStore.setState({ resume: resumeSpy, skip: skipSpy, stop: stopSpy });
    seedTimer(pausa(300, 'countup'));
    renderFoco();

    fireEvent.click(screen.getByRole('button', { name: 'Retomar' }));
    expect(resumeSpy).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: /pular pausa/i }));
    expect(skipSpy).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
    expect(stopSpy).toHaveBeenCalledTimes(1);
  });

  // CA-13 — ✕ navega de volta pro cockpit (/hoje)
  it('o sair (✕) navega pra /hoje', () => {
    seedTimer(null);
    renderFoco();

    fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
    expect(screen.getByText('rota-hoje')).toBeInTheDocument();
  });
});

describe('Modo Foco — rodapé do dia + wake lock (T-05)', () => {
  // CA-10 — total fechado do servidor + elapsed vivo da ENTRY RODANDO
  // (aritmética do TodayList); RNF-03: tick NÃO refaz fetch
  it('rodapé soma total fechado + vivo da entry rodando, sem refetch por tick', async () => {
    const startedAt = new Date(Date.now() - 120_000).toISOString();
    const rodando = entry({ id: 'e-run', description: 'foco', started_at: startedAt });
    vi.mocked(api.listEntries).mockResolvedValue({ entries: [rodando], total_seconds: 3600 });
    seedTimer({
      mode: 'countup',
      entry: rodando,
      elapsed_seconds: 120,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();

    // 3600 fechado + 120 vivos = 1h02
    await waitFor(() =>
      expect(screen.getByTestId('focus-day-total').textContent).toBe('hoje: 1h02'),
    );

    const fetches = vi.mocked(api.listEntries).mock.calls.length;
    // o relógio anda 5min (offset) + tick: o vivo avança SEM busca nova
    act(() => {
      useTimerStore.setState({ offsetMs: 300_000, tick: 1 });
    });
    expect(screen.getByTestId('focus-day-total').textContent).toBe('hoje: 1h07');
    expect(vi.mocked(api.listEntries).mock.calls.length).toBe(fetches);
  });

  // CA-10 — em PAUSA o total não avança (sem entry rodando na query, o
  // elapsed do break fica fora — não é tempo trabalhado)
  it('em pausa o total não inclui o elapsed do break', async () => {
    vi.mocked(api.listEntries).mockResolvedValue({ entries: [], total_seconds: 1800 });
    seedTimer(pausa(120, 'countup'));
    renderFoco();

    await waitFor(() =>
      expect(screen.getByTestId('focus-day-total').textContent).toBe('hoje: 0h30'),
    );
    // o relógio da pausa anda; o total do dia NÃO
    act(() => {
      useTimerStore.setState({ offsetMs: 60_000, tick: 1 });
    });
    expect(screen.getByTestId('focus-day-total').textContent).toBe('hoje: 0h30');
  });

  // CA-10 — stop: a entry ativa muda → query do dia invalidada → o total
  // absorve a entry recém-fechada sem reload
  it('stop invalida a query do dia e absorve a entry fechada', async () => {
    const rodando = entry({ id: 'e-run', started_at: new Date().toISOString() });
    vi.mocked(api.listEntries).mockResolvedValue({ entries: [rodando], total_seconds: 0 });
    seedTimer({
      mode: 'countup',
      entry: rodando,
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
    });
    renderFoco();
    await waitFor(() => expect(screen.getByTestId('focus-day-total')).toBeInTheDocument());
    const before = vi.mocked(api.listEntries).mock.calls.length;

    // estado novo aplicado (stop): sem entry ativa; o servidor já fechou
    vi.mocked(api.listEntries).mockResolvedValue({ entries: [], total_seconds: 900 });
    act(() => {
      useTimerStore.setState({ state: null });
    });

    await waitFor(() =>
      expect(vi.mocked(api.listEntries).mock.calls.length).toBeGreaterThan(before),
    );
    await waitFor(() =>
      expect(screen.getByTestId('focus-day-total').textContent).toBe('hoje: 0h15'),
    );
  });

  // CA-14..16 (wiring) — wake lock na montagem da ROTA, release no teardown
  it('wake lock requisitado ao montar a rota e liberado ao desmontar', async () => {
    const sentinel = {
      released: false,
      release: vi.fn(async () => {
        sentinel.released = true;
      }),
    };
    const request = vi.fn(async () => sentinel);
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true });

    seedTimer(null);
    const { unmount } = renderFoco();
    await flush();
    expect(request).toHaveBeenCalledWith('screen');

    unmount();
    await flush();
    expect(sentinel.release).toHaveBeenCalled();
    Reflect.deleteProperty(navigator, 'wakeLock');
  });
});
