// CalendarNav (fase 11, CA-19/20): ‹ Hoje › + título do período + segmented
// control Dia/Semana/Mês. Aposenta o WeekNav da 06 nesta página. A troca de
// view é de SESSÃO (não muda a preferência salva — CA-23).
import type { View } from '../../lib/calendar-layout';

const VIEWS: { key: View; label: string }[] = [
  { key: 'day', label: 'Dia' },
  { key: 'week', label: 'Semana' },
  { key: 'month', label: 'Mês' },
];

export default function CalendarNav({
  view,
  onView,
  title,
  onPrev,
  onNext,
  onToday,
}: {
  view: View;
  onView: (v: View) => void;
  title: string;
  onPrev: () => void;
  onNext: () => void;
  onToday: () => void;
}) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 'var(--gap)',
        marginBottom: 'var(--gap)',
        flexWrap: 'wrap',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button type="button" className="tt-btn tt-btn--ghost" aria-label="Anterior" onClick={onPrev}>
          ‹
        </button>
        <button type="button" className="tt-btn" onClick={onToday}>
          Hoje
        </button>
        <button type="button" className="tt-btn tt-btn--ghost" aria-label="Próximo" onClick={onNext}>
          ›
        </button>
        <strong data-testid="cal-title" style={{ fontSize: 'var(--fs-row)', marginLeft: 6 }}>
          {title}
        </strong>
      </div>

      <div role="group" aria-label="Visualização" style={{ display: 'flex', gap: 4 }}>
        {VIEWS.map((v) => {
          const on = v.key === view;
          return (
            <button
              key={v.key}
              type="button"
              aria-pressed={on}
              onClick={() => onView(v.key)}
              style={{
                padding: '6px 12px',
                borderRadius: 8,
                cursor: 'pointer',
                fontFamily: 'var(--font-ui)',
                fontSize: 12.5,
                fontWeight: 540,
                border: '1px solid',
                borderColor: on ? 'var(--accent)' : 'var(--border-2)',
                background: on ? 'var(--accent-soft)' : 'transparent',
                color: on ? 'var(--accent)' : 'var(--text-2)',
              }}
            >
              {v.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
