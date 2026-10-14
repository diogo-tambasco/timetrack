// DatePicker (fase 31): mini-calendário de mês pra agendar a Due de uma task —
// "quando preciso fazer isso?". Reusa monthGridDays/inMonth/addMonths da camada
// de calendário (parede LOCAL, tz-correto). Clicar num dia define a data; Hoje e
// Limpar como atalhos. Presentacional — value 'YYYY-MM-DD'|null, onPick devolve.
import { useState } from 'react';
import { monthGridDays, inMonth, addMonths, isoDay, parseDay, periodTitle } from '../../lib/calendar-layout';
import { Icon } from '../icons';

const WEEKDAYS = ['S', 'T', 'Q', 'Q', 'S', 'S', 'D']; // seg→dom (semana na segunda)

export default function DatePicker({
  value,
  onPick,
}: {
  value: string | null;
  onPick: (iso: string | null) => void;
}) {
  const todayISO = isoDay(new Date());
  const [ref, setRef] = useState<Date>(() => (value ? parseDay(value) : new Date()));
  const days = monthGridDays(ref);

  return (
    <div
      data-testid="date-picker"
      style={{
        width: 252,
        background: 'var(--surface-2)',
        border: '1px solid var(--border-2)',
        borderRadius: 12,
        boxShadow: 'var(--shadow)',
        padding: 12,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
        <button
          type="button"
          aria-label="Mês anterior"
          className="tt-iconbtn"
          onClick={() => setRef((d) => addMonths(d, -1))}
          style={{ width: 26, height: 26, background: 'transparent', color: 'var(--text-3)' }}
        >
          <Icon.chevron size={15} style={{ transform: 'rotate(90deg)' }} />
        </button>
        <span style={{ fontSize: 13, fontWeight: 600, textTransform: 'capitalize' }}>
          {periodTitle('month', ref)}
        </span>
        <button
          type="button"
          aria-label="Próximo mês"
          className="tt-iconbtn"
          onClick={() => setRef((d) => addMonths(d, 1))}
          style={{ width: 26, height: 26, background: 'transparent', color: 'var(--text-3)' }}
        >
          <Icon.chevron size={15} style={{ transform: 'rotate(-90deg)' }} />
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', gap: 2 }}>
        {WEEKDAYS.map((w, i) => (
          <span key={i} className="faint" style={{ fontSize: 10.5, textAlign: 'center', padding: '2px 0' }}>
            {w}
          </span>
        ))}
        {days.map((iso) => {
          const day = parseDay(iso).getDate();
          const isSel = iso === value;
          const isToday = iso === todayISO;
          const dim = !inMonth(iso, ref);
          return (
            <button
              key={iso}
              type="button"
              aria-label={iso}
              onClick={() => onPick(iso)}
              style={{
                aspectRatio: '1 / 1',
                border: isToday && !isSel ? '1px solid var(--accent-line)' : '1px solid transparent',
                borderRadius: 7,
                cursor: 'pointer',
                fontSize: 12.5,
                fontFamily: 'var(--font-ui)',
                background: isSel ? 'var(--accent)' : 'transparent',
                color: isSel ? 'var(--accent-ink)' : dim ? 'var(--text-3)' : 'var(--text-1)',
                fontWeight: isSel || isToday ? 600 : 440,
              }}
            >
              {day}
            </button>
          );
        })}
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 10 }}>
        <button
          type="button"
          className="tt-btn tt-btn--ghost tt-btn--sm"
          onClick={() => onPick(todayISO)}
        >
          Hoje
        </button>
        {value != null && (
          <button
            type="button"
            className="tt-btn tt-btn--ghost tt-btn--sm"
            onClick={() => onPick(null)}
            style={{ color: 'var(--text-3)' }}
          >
            Limpar
          </button>
        )}
      </div>
    </div>
  );
}
