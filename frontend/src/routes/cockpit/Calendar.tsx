// Calendário 2.0 (fase 11): grade de horários estilo Google Calendar. Mantém
// os 3 estados da 06 (loading/erro do status, CTA não-conectado, erro de
// eventos → reconectar — CA-26). Acrescenta: view (init = default_view das
// prefs, CA-23), refDate, navegação por período (CA-19), TimeGrid (Dia/Semana)
// e MonthGrid (Mês). Janela de fetch conforme a view (CA-05/06/07); eventos só
// quando conectado (CA-08). Reusa a camada de dados da 06 inteira.
import { useEffect, useState } from 'react';
import { CALENDAR_CONNECT_PATH, type CalendarSettings } from '../../lib/api';
import {
  useCalendarStatus,
  useCalendarEvents,
  useCalendarSettings,
} from '../../lib/queries/calendar';
import {
  type View,
  weekRange,
  dayRange,
  monthRange,
  weekDays,
  periodTitle,
  isoDay,
  parseDay,
  addDays,
  addWeeks,
  addMonths,
} from '../../lib/calendar-layout';
import CalendarNav from '../../components/calendar/CalendarNav';
import TimeGrid from '../../components/calendar/TimeGrid';
import MonthGrid from '../../components/calendar/MonthGrid';

const DEFAULTS: CalendarSettings = {
  default_view: 'week',
  hour_start: 6,
  hour_end: 23,
  density: 'comfortable',
};

function windowFor(view: View, refDate: Date): { from: string; to: string } {
  if (view === 'day') return dayRange(refDate);
  if (view === 'month') return monthRange(refDate);
  return weekRange(refDate);
}

export default function Calendar() {
  const statusQ = useCalendarStatus();
  const prefsQ = useCalendarSettings();
  const prefs = prefsQ.data ?? DEFAULTS;

  const [view, setView] = useState<View | undefined>(undefined);
  const [refDate, setRefDate] = useState<Date>(() => new Date());

  // view inicial = default_view das prefs (uma vez, quando as prefs assentam —
  // data OU erro). Trocar de view depois é de sessão, não muda a preferência.
  useEffect(() => {
    if (view === undefined && (prefsQ.data || prefsQ.isError)) {
      setView(prefs.default_view);
    }
  }, [prefsQ.data, prefsQ.isError, view, prefs.default_view]);

  const connected = statusQ.data?.connected ?? false;
  const effView: View = view ?? prefs.default_view;
  const { from, to } = windowFor(effView, refDate);
  const eventsQ = useCalendarEvents(from, to, connected);

  // ——— estados herdados da 06 (CA-26) ———
  if (statusQ.isLoading || view === undefined) {
    return (
      <section>
        <p className="eyebrow">Calendário</p>
        <div className="muted" style={{ padding: 14, fontSize: 13 }}>
          Carregando…
        </div>
      </section>
    );
  }

  if (statusQ.isError) {
    return (
      <section>
        <p className="eyebrow">Calendário</p>
        <div className="tt-card" data-testid="status-error" style={{ padding: 'var(--pad)', maxWidth: 460 }}>
          <span className="muted" style={{ fontSize: 13 }}>
            Não foi possível verificar a conexão com o Google Calendar. Recarregue a página.
          </span>
        </div>
      </section>
    );
  }

  if (!connected) {
    return (
      <section>
        <p className="eyebrow">Calendário</p>
        <div className="tt-card" style={{ padding: 'var(--pad)', display: 'grid', gap: 12, maxWidth: 460 }}>
          <strong style={{ fontSize: 'var(--fs-row)' }}>Conecte seu Google Calendar</strong>
          <span className="muted" style={{ fontSize: 13 }}>
            Veja os eventos da sua agenda principal aqui e compare planejado × real nos relatórios.
          </span>
          <a
            href={CALENDAR_CONNECT_PATH}
            data-testid="connect-calendar"
            className="tt-btn tt-btn--accent"
            style={{ width: 'fit-content', textDecoration: 'none' }}
          >
            Conectar Google Calendar
          </a>
        </div>
      </section>
    );
  }

  // ——— navegação por período (CA-19) ———
  const step = (dir: 1 | -1) => {
    setRefDate((d) =>
      effView === 'day' ? addDays(d, dir) : effView === 'week' ? addWeeks(d, dir) : addMonths(d, dir),
    );
  };

  const pickDay = (dayISO: string) => {
    setRefDate(parseDay(dayISO));
    setView('day');
  };

  return (
    <section>
      <p className="eyebrow">Calendário</p>
      <CalendarNav
        view={effView}
        onView={setView}
        title={periodTitle(effView, refDate)}
        onPrev={() => step(-1)}
        onNext={() => step(1)}
        onToday={() => setRefDate(new Date())}
      />

      {eventsQ.isError ? (
        <div
          className="tt-card"
          data-testid="events-error"
          style={{ padding: 'var(--pad)', display: 'grid', gap: 10, maxWidth: 460 }}
        >
          <span className="muted" style={{ fontSize: 13 }}>
            Não foi possível carregar sua agenda. Pode ser preciso reconectar o Google Calendar.
          </span>
          <a
            href={CALENDAR_CONNECT_PATH}
            data-testid="reconnect-calendar"
            className="tt-btn tt-btn--accent"
            style={{ width: 'fit-content', textDecoration: 'none' }}
          >
            Reconectar
          </a>
        </div>
      ) : effView === 'month' ? (
        <MonthGrid refDate={refDate} events={eventsQ.data ?? []} onPickDay={pickDay} />
      ) : (
        <TimeGrid
          days={effView === 'day' ? [isoDay(refDate)] : weekDays(refDate)}
          events={eventsQ.data ?? []}
          hourStart={prefs.hour_start}
          hourEnd={prefs.hour_end}
          density={prefs.density}
        />
      )}
    </section>
  );
}
