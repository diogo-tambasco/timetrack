// Pomodoro.spec — T-18 (fase 03) + T-10 (fase 15): o grupo Pomodoro nos
// Ajustes carrega a config salva (7 campos), valida no client sem chamar a
// API (CA-20.1) e salva via PUT com o corpo COMPLETO (7 campos, CA-02/20).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import PomodoroGroup from './Pomodoro';
import * as api from '../../lib/api';
import type { PomodoroSettings } from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    getPomodoroSettings: vi.fn(),
    putPomodoroSettings: vi.fn(),
  };
});

const saved: PomodoroSettings = {
  focus_minutes: 50,
  break_minutes: 15,
  cycles_before_long: 4,
  long_break_minutes: 15,
  auto_start_break: true,
  auto_start_focus: true,
  sound_enabled: false,
};

function renderGrupo() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <PomodoroGroup />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getPomodoroSettings).mockResolvedValue({ ...saved });
});

describe('PomodoroGroup', () => {
  it('carrega a config persistida (slider + preset ativo + ciclos/pausa-longa)', async () => {
    renderGrupo();

    expect(await screen.findByText('50 min')).toBeInTheDocument();
    expect(screen.getByLabelText(/^foco$/i)).toHaveValue('50');
    expect(screen.getByRole('button', { name: /^15 min$/i })).toHaveAttribute('aria-pressed', 'true');
    // fase 15: ciclos e pausa longa carregados
    expect(screen.getByLabelText(/ciclos até a pausa longa/i)).toHaveValue(4);
    expect(screen.getByLabelText(/^pausa longa/i)).toHaveValue(15);
  });

  it('custom fora de 1–60 NÃO chama a API e mostra o erro (CA-20.1)', async () => {
    renderGrupo();
    await screen.findByText('50 min');

    fireEvent.change(screen.getByLabelText(/pausa personalizada/i), { target: { value: '75' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.putPomodoroSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/entre 1 e 60/i);
  });

  it('ciclos fora de 1–12 NÃO chama a API e mostra o erro (CA-20.1)', async () => {
    renderGrupo();
    await screen.findByText('50 min');

    fireEvent.change(screen.getByLabelText(/ciclos até a pausa longa/i), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.putPomodoroSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/ciclos.*entre 1 e 12/i);
  });

  it('pausa longa fora de 1–120 NÃO chama a API e mostra o erro (CA-20.1)', async () => {
    renderGrupo();
    await screen.findByText('50 min');

    fireEvent.change(screen.getByLabelText(/^pausa longa/i), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.putPomodoroSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/pausa longa.*entre 1 e 120/i);
  });

  it('salvar envia PUT com os 7 campos do contrato (CA-02/20)', async () => {
    vi.mocked(api.putPomodoroSettings).mockResolvedValue({ ...saved });
    renderGrupo();
    await screen.findByText('50 min');

    fireEvent.change(screen.getByLabelText(/^foco$/i), { target: { value: '30' } });
    fireEvent.click(screen.getByRole('button', { name: /^20 min$/i }));
    fireEvent.change(screen.getByLabelText(/ciclos até a pausa longa/i), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText(/^pausa longa/i), { target: { value: '25' } });

    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.putPomodoroSettings).toHaveBeenCalledWith({
        focus_minutes: 30,
        break_minutes: 20,
        cycles_before_long: 3,
        long_break_minutes: 25,
        auto_start_break: true,
        auto_start_focus: true,
        sound_enabled: false,
      }),
    );
  });

  it('os toggles refletem o estado salvo e entram no payload (CA-20)', async () => {
    vi.mocked(api.getPomodoroSettings).mockResolvedValue({
      ...saved,
      auto_start_break: false,
      sound_enabled: true,
    });
    vi.mocked(api.putPomodoroSettings).mockResolvedValue({ ...saved });
    renderGrupo();
    await screen.findByText('50 min');

    const switches = screen.getAllByRole('switch');
    // ordem: auto-start pausa, auto-start foco, alerta sonoro
    expect(switches[0]).toHaveAttribute('aria-checked', 'false'); // auto_start_break salvo false
    expect(switches[1]).toHaveAttribute('aria-checked', 'true'); // auto_start_focus salvo true
    expect(switches[2]).toHaveAttribute('aria-checked', 'true'); // sound salvo true

    // liga o auto-start pausa e desliga o som, salva → payload reflete
    fireEvent.click(switches[0]);
    fireEvent.click(switches[2]);
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.putPomodoroSettings).toHaveBeenCalledWith(
        expect.objectContaining({ auto_start_break: true, auto_start_focus: true, sound_enabled: false }),
      ),
    );
  });

  it('custom válido vence o preset no payload', async () => {
    vi.mocked(api.putPomodoroSettings).mockResolvedValue({ ...saved });
    renderGrupo();
    await screen.findByText('50 min');

    fireEvent.change(screen.getByLabelText(/pausa personalizada/i), { target: { value: '7' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.putPomodoroSettings).toHaveBeenCalledWith(
        expect.objectContaining({ focus_minutes: 50, break_minutes: 7 }),
      ),
    );
  });
});
