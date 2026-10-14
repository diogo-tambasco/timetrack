// CalendarGroup.spec — fase 11 T-10 (CA-02/03/25): carrega a config salva,
// valida hour_end > hour_start no client (sem chamar a API) e salva via PUT
// com o corpo do contrato.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CalendarGroup from './CalendarGroup';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    getCalendarSettings: vi.fn(),
    putCalendarSettings: vi.fn(),
  };
});

function renderGrupo() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <CalendarGroup />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getCalendarSettings).mockResolvedValue({
    default_view: 'week',
    hour_start: 6,
    hour_end: 23,
    density: 'comfortable',
  });
});

describe('CalendarGroup (fase 11)', () => {
  it('carrega a config persistida (view ativa + horas)', async () => {
    renderGrupo();
    expect(await screen.findByRole('button', { name: 'Semana' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText('Hora inicial')).toHaveValue(6);
    expect(screen.getByLabelText('Hora final')).toHaveValue(23);
  });

  it('hour_end <= hour_start NÃO chama a API e mostra erro (CA-03/25)', async () => {
    renderGrupo();
    await screen.findByRole('button', { name: 'Semana' });

    fireEvent.change(screen.getByLabelText('Hora inicial'), { target: { value: '20' } });
    fireEvent.change(screen.getByLabelText('Hora final'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    expect(api.putCalendarSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/maior que a inicial/i);
  });

  it('salvar válido envia PUT com o corpo do contrato (CA-02)', async () => {
    vi.mocked(api.putCalendarSettings).mockResolvedValue({
      default_view: 'day',
      hour_start: 8,
      hour_end: 20,
      density: 'compact',
    });
    renderGrupo();
    await screen.findByRole('button', { name: 'Semana' });

    fireEvent.click(screen.getByRole('button', { name: 'Dia' }));
    fireEvent.change(screen.getByLabelText('Hora inicial'), { target: { value: '8' } });
    fireEvent.change(screen.getByLabelText('Hora final'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: 'Compacta' }));
    fireEvent.click(screen.getByRole('button', { name: /salvar/i }));

    await waitFor(() =>
      expect(api.putCalendarSettings).toHaveBeenCalledWith({
        default_view: 'day',
        hour_start: 8,
        hour_end: 20,
        density: 'compact',
      }),
    );
  });
});
