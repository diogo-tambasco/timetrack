// MonthGrid (fase 11, CA-18): grade de 6 semanas (42 células) segunda→domingo
// cobrindo o mês do refDate. Cada célula: número do dia + chips de evento
// (all-day primeiro, depois timed por horário), com "+N mais" quando estoura.
// Hoje destacado; dias fora do mês atenuados. Clicar a célula → view Dia;
// clicar o chip → popover (EventChip).
import type { CalendarEvent } from '../../lib/api';
import { monthGridDays, inMonth, allDayByDay, isoDay, parseDay } from '../../lib/calendar-layout';
import EventChip from './EventChip';

const DOW = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
const MAX_CHIPS = 3;

// localDay de um ISO com horário (pro bucket dos timed por dia de início).
function localDay(iso: string): string {
  return isoDay(new Date(iso));
}

export default function MonthGrid({
  refDate,
  events,
  onPickDay,
}: {
  refDate: Date;
  events: CalendarEvent[];
  onPickDay: (dayISO: string) => void;
}) {
  const days = monthGridDays(refDate);
  const todayISO = isoDay(new Date());
  const allDay = allDayByDay(events, days);

  // timed agrupados pelo dia local de início
  const timedByDay = new Map<string, CalendarEvent[]>();
  for (const e of events) {
    if (e.all_day) continue;
    const k = localDay(e.start);
    const list = timedByDay.get(k) ?? [];
    list.push(e);
    timedByDay.set(k, list);
  }

  return (
    <div data-testid="month-grid" className="tt-card" style={{ padding: 0, overflow: 'hidden' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: '1px solid var(--border)' }}>
        {DOW.map((d) => (
          <div key={d} className="faint" style={{ textAlign: 'center', padding: '6px 0', fontSize: 11, textTransform: 'uppercase', letterSpacing: '.04em' }}>
            {d}
          </div>
        ))}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', gridAutoRows: 'minmax(92px, 1fr)' }}>
        {days.map((d) => {
          const isToday = d === todayISO;
          const outside = !inMonth(d, refDate);
          const timed = (timedByDay.get(d) ?? []).slice().sort((a, b) => a.start.localeCompare(b.start));
          const list = [...(allDay.get(d) ?? []), ...timed];
          const shown = list.slice(0, MAX_CHIPS);
          const overflow = list.length - shown.length;
          return (
            <div
              key={d}
              data-testid="month-cell"
              onClick={() => onPickDay(d)}
              style={{
                borderLeft: '1px solid var(--border)',
                borderTop: '1px solid var(--border)',
                padding: 4,
                display: 'grid',
                gridTemplateRows: 'auto 1fr',
                gap: 3,
                cursor: 'pointer',
                background: outside ? 'var(--surface-1)' : 'transparent',
                opacity: outside ? 0.5 : 1,
              }}
            >
              <div
                data-testid={isToday ? 'month-today' : undefined}
                style={{
                  fontSize: 12,
                  fontWeight: isToday ? 700 : 500,
                  textAlign: 'right',
                  color: isToday ? 'var(--accent-ink)' : 'var(--text-2)',
                  background: isToday ? 'var(--accent)' : 'transparent',
                  borderRadius: 6,
                  width: isToday ? 22 : 'auto',
                  height: isToday ? 22 : 'auto',
                  lineHeight: isToday ? '22px' : undefined,
                  justifySelf: 'end',
                  padding: isToday ? 0 : '0 2px',
                }}
              >
                {parseDay(d).getDate()}
              </div>
              <div style={{ display: 'grid', gap: 2, alignContent: 'start' }}>
                {shown.map((e) => (
                  <EventChip key={e.id} event={e} />
                ))}
                {overflow > 0 && (
                  <button
                    type="button"
                    data-testid="month-overflow"
                    onClick={(ev) => {
                      ev.stopPropagation();
                      onPickDay(d);
                    }}
                    className="faint"
                    style={{ background: 'transparent', border: 'none', textAlign: 'left', fontSize: 11, cursor: 'pointer', padding: '0 6px', fontFamily: 'var(--font-ui)' }}
                  >
                    +{overflow} mais
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
