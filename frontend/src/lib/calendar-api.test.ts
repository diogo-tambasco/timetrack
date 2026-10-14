import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  getCalendarStatus,
  getCalendarEvents,
  getPlannedReport,
  getCalendarSettings,
  putCalendarSettings,
} from './api';

afterEach(() => {
  vi.unstubAllGlobals();
});

// fase 06: shape das chamadas do client de calendar (querystring + envelope).
describe('lib/api calendar', () => {
  it('calendar_status', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ connected: true }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const st = await getCalendarStatus();
    expect(st.connected).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/calendar/status',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('calendar_events_querystring_e_desembrulho', async () => {
    const events = [
      { id: 'e1', summary: 'Reunião', start: '2026-06-15T09:00:00-03:00', end: '2026-06-15T10:00:00-03:00', all_day: false, html_link: 'x' },
    ];
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ events }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const out = await getCalendarEvents({ from: '2026-06-15', to: '2026-06-21', tz: 'America/Sao_Paulo' });
    expect(out).toHaveLength(1);
    expect(out[0].id).toBe('e1');
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('/api/v1/calendar/events?');
    expect(url).toContain('from=2026-06-15');
    expect(url).toContain('to=2026-06-21');
    expect(url).toContain('tz=America%2FSao_Paulo');
  });

  it('planned_report_no_workspace', async () => {
    const rep = {
      week_start: '2026-06-15',
      week_end: '2026-06-21',
      connected: true,
      days: [{ date: '2026-06-15', planned_seconds: 7200, realized_seconds: 3600 }],
    };
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(rep), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const out = await getPlannedReport('w1', { date: '2026-06-15', tz: 'UTC' });
    expect(out.connected).toBe(true);
    expect(out.days[0].planned_seconds).toBe(7200);
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('/api/v1/workspaces/w1/reports/planned?');
    expect(url).toContain('date=2026-06-15');
  });

  // fase 11: preferências do calendário (GET defaults + PUT corpo certo).
  it('calendar_settings_get', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({ default_view: 'week', hour_start: 6, hour_end: 23, density: 'comfortable' }),
        { status: 200 },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const s = await getCalendarSettings();
    expect(s).toEqual({ default_view: 'week', hour_start: 6, hour_end: 23, density: 'comfortable' });
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/settings/calendar',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('calendar_settings_put_envia_corpo', async () => {
    const body = { default_view: 'day' as const, hour_start: 8, hour_end: 20, density: 'compact' as const };
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const out = await putCalendarSettings(body);
    expect(out).toEqual(body);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/v1/settings/calendar');
    expect(init.method).toBe('PUT');
    expect(JSON.parse(init.body as string)).toEqual(body);
  });
});
