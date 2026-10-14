// TimeGrid (fase 11, CA-09..17/24): grade hora×dia para as views Dia (1
// coluna) e Semana (7). Renderiza SEMPRE as 24h (CA-15) — a faixa
// hour_start..hour_end é só viewport/alvo de auto-scroll, não recorta. Faixa
// all-day fixa no topo (CA-13), linha do "agora" só na coluna de hoje
// (CA-16), auto-scroll no mount (CA-17). A densidade usa um token PRÓPRIO e
// ESCOPADO (--cal-row-h) — NUNCA o --row-h global (CA-24.1).
import { useEffect, useRef, type CSSProperties } from 'react';
import type { CalendarEvent } from '../../lib/api';
import { layoutTimed, allDayByDay, parseDay, isoDay } from '../../lib/calendar-layout';
import EventBlock from './EventBlock';
import EventChip from './EventChip';
import NowLine from './NowLine';

const DOW = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
const pad = (n: number) => String(n).padStart(2, '0');
const GUTTER = 56; // px da coluna de horas

function dayHeader(dayISO: string): { dow: string; day: number } {
  const d = parseDay(dayISO);
  return { dow: DOW[(d.getDay() + 6) % 7], day: d.getDate() };
}

export default function TimeGrid({
  days,
  events,
  hourStart = 6,
  hourEnd = 23,
  density = 'comfortable',
}: {
  days: string[];
  events: CalendarEvent[];
  hourStart?: number;
  hourEnd?: number;
  density?: 'comfortable' | 'compact';
}) {
  const rowH = density === 'compact' ? 40 : 56;
  // viewport visível sem rolar = a faixa configurada (CA-15/24); a grade tem
  // 24h e o resto fica alcançável rolando.
  const viewportH = Math.max(1, hourEnd - hourStart) * rowH;
  const scrollRef = useRef<HTMLDivElement>(null);

  const todayISO = isoDay(new Date());
  const todayInView = days.includes(todayISO);

  const positioned = layoutTimed(events, days);
  const allDay = allDayByDay(events, days);
  const hasAllDay = allDay.size > 0;

  // auto-scroll (CA-17): hoje → horário atual; senão → hour_start.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const now = new Date();
    const targetMin = todayInView ? now.getHours() * 60 + now.getMinutes() : hourStart * 60;
    el.scrollTop = Math.max(0, (targetMin / 60 - 0.5) * rowH);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days.join(','), density, hourStart]);

  const cols: CSSProperties = {
    display: 'grid',
    gridTemplateColumns: `${GUTTER}px repeat(${days.length}, minmax(0, 1fr))`,
  };
  const bodyVars = { '--cal-row-h': `${rowH}px` } as CSSProperties;

  return (
    <div data-testid="time-grid" className="tt-card" style={{ padding: 0, overflow: 'hidden' }}>
      {/* cabeçalho de dias */}
      <div style={{ ...cols, borderBottom: '1px solid var(--border)' }}>
        <div />
        {days.map((d) => {
          const h = dayHeader(d);
          const isToday = d === todayISO;
          return (
            <div
              key={d}
              style={{
                padding: '8px 6px',
                textAlign: 'center',
                borderLeft: '1px solid var(--border)',
                color: isToday ? 'var(--accent)' : 'var(--text-2)',
              }}
            >
              <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.04em' }}>{h.dow}</div>
              <div style={{ fontSize: 16, fontWeight: 600, color: isToday ? 'var(--accent)' : 'var(--text-1)' }}>
                {h.day}
              </div>
            </div>
          );
        })}
      </div>

      {/* faixa all-day (CA-13) */}
      {hasAllDay && (
        <div style={{ ...cols, borderBottom: '1px solid var(--border)', background: 'var(--surface-1)' }}>
          <div
            className="faint"
            style={{ fontSize: 10.5, textAlign: 'right', padding: '4px 6px', alignSelf: 'center' }}
          >
            dia todo
          </div>
          {days.map((d) => (
            <div key={d} data-testid="allday-cell" style={{ borderLeft: '1px solid var(--border)', padding: 3, display: 'grid', gap: 2 }}>
              {(allDay.get(d) ?? []).map((e) => (
                <EventChip key={e.id} event={e} />
              ))}
            </div>
          ))}
        </div>
      )}

      {/* grade de horas (24h, scroll) */}
      <div ref={scrollRef} data-testid="time-grid-scroll" style={{ overflowY: 'auto', maxHeight: `${viewportH}px` }}>
        <div style={{ ...cols, ...bodyVars, position: 'relative', height: 'calc(var(--cal-row-h) * 24)' }}>
          {/* coluna de horas */}
          <div style={{ position: 'relative' }}>
            {Array.from({ length: 24 }, (_, h) => (
              <div
                key={h}
                className="faint mono"
                style={{ position: 'absolute', top: `calc(var(--cal-row-h) * ${h})`, right: 6, fontSize: 10.5, transform: 'translateY(-50%)' }}
              >
                {h > 0 ? `${pad(h)}:00` : ''}
              </div>
            ))}
          </div>
          {/* colunas de dia */}
          {days.map((d) => {
            const blocks = positioned.filter((p) => p.dayISO === d);
            return (
              <div
                key={d}
                data-testid="day-column"
                style={{
                  position: 'relative',
                  borderLeft: '1px solid var(--border)',
                  backgroundImage: 'linear-gradient(var(--border) 1px, transparent 1px)',
                  backgroundSize: '100% var(--cal-row-h)',
                }}
              >
                {blocks.map((p) => (
                  <EventBlock key={`${p.event.id}-${p.dayISO}`} p={p} />
                ))}
                {d === todayISO && <NowLine />}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
