// Calendar.spec — fase 11 T-09 (CA-05/06/07/08/19/20/23/26). Reescrito do
// layout de LISTA da 06 pra grade 2.0 (desvio D-09 registrado: a 06 é dona da
// CAMADA DE DADOS, não do layout — a fidelidade dos dados segue assertada, só
// muda a forma). "Hoje" fixado com fake timers (quarta 17/06/2026).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Calendar from './Calendar';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return {
    ...mod,
    getCalendarStatus: vi.fn(),
    getCalendarEvents: vi.fn(),
    getCalendarSettings: vi.fn(),
  };
});

const at = (y: number, mo: number, d: number, h = 0, mi = 0) =>
  new Date(y, mo - 1, d, h, mi).toISOString();

const NOW = new Date(2026, 5, 17, 9, 0); // quarta 17/06/2026

const PREFS = (over: Partial<api.CalendarSettings> = {}): api.CalendarSettings => ({
  default_view: 'week',
  hour_start: 6,
  hour_end: 23,
  density: 'comfortable',
  ...over,
});

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Calendar />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  // shouldAdvanceTime: real timers ainda avançam (React Query/findBy resolvem),
  // mas a base de "hoje" fica fixa em NOW.
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(NOW);
  vi.mocked(api.getCalendarSettings).mockResolvedValue(PREFS());
});
afterEach(() => vi.useRealTimers());

describe('Calendário 2.0 (fase 11)', () => {
  it('não conectado → CTA e NÃO busca eventos (CA-08/26)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: false });
    renderPage();
    expect(await screen.findByTestId('connect-calendar')).toHaveAttribute('href', api.CALENDAR_CONNECT_PATH);
    expect(api.getCalendarEvents).not.toHaveBeenCalled();
    expect(screen.queryByTestId('time-grid')).not.toBeInTheDocument();
  });

  it('conectado → abre na default_view das prefs e busca a semana (CA-05/23)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByTestId('time-grid')).toBeInTheDocument();
    expect(screen.getAllByTestId('day-column')).toHaveLength(7); // semana
    // janela = segunda→domingo da semana de hoje
    await waitFor(() => expect(api.getCalendarEvents).toHaveBeenCalled());
    expect(api.getCalendarEvents).toHaveBeenCalledWith(expect.objectContaining({ from: '2026-06-15', to: '2026-06-21' }));
  });

  it('default_view month → abre no MonthGrid (CA-07/23)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
    vi.mocked(api.getCalendarSettings).mockResolvedValue(PREFS({ default_view: 'month' }));
    renderPage();
    expect(await screen.findByTestId('month-grid')).toBeInTheDocument();
    expect(screen.queryByTestId('time-grid')).not.toBeInTheDocument();
  });

  it('eventos posicionados na grade (CA-09)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockResolvedValue([
      { id: 'e1', summary: 'Reunião', start: at(2026, 6, 17, 10, 0), end: at(2026, 6, 17, 11, 0), all_day: false, html_link: '' },
    ]);
    renderPage();
    expect(await screen.findByText('Reunião')).toBeInTheDocument();
    expect(screen.getByTestId('cal-event')).toBeInTheDocument();
  });

  it('troca de view muda a grade e a janela buscada (CA-06/19/20)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
    renderPage();
    await screen.findByTestId('time-grid');

    fireEvent.click(screen.getByRole('button', { name: 'Mês' }));
    expect(await screen.findByTestId('month-grid')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Dia' }));
    await screen.findByTestId('time-grid');
    expect(screen.getAllByTestId('day-column')).toHaveLength(1);
    // janela do dia = hoje
    await waitFor(() =>
      expect(api.getCalendarEvents).toHaveBeenCalledWith(expect.objectContaining({ from: '2026-06-17', to: '2026-06-17' })),
    );
  });

  it('nav muda o período (título) (CA-19)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockResolvedValue([]);
    renderPage();
    await screen.findByTestId('time-grid');
    const before = screen.getByTestId('cal-title').textContent;
    fireEvent.click(screen.getByRole('button', { name: 'Próximo' }));
    await waitFor(() => expect(screen.getByTestId('cal-title').textContent).not.toBe(before));
  });

  it('erro no status → estado de erro, NÃO o CTA (CA-26)', async () => {
    vi.mocked(api.getCalendarStatus).mockRejectedValue(new Error('500'));
    renderPage();
    expect(await screen.findByTestId('status-error')).toBeInTheDocument();
    expect(screen.queryByTestId('connect-calendar')).not.toBeInTheDocument();
  });

  it('erro nos eventos (conectado) → CTA de reconectar (CA-26)', async () => {
    vi.mocked(api.getCalendarStatus).mockResolvedValue({ connected: true });
    vi.mocked(api.getCalendarEvents).mockRejectedValue(new Error('409'));
    renderPage();
    expect(await screen.findByTestId('reconnect-calendar')).toBeInTheDocument();
  });
});
