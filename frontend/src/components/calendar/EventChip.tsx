// EventChip (fase 11): chip de evento em fluxo (faixa all-day da TimeGrid e
// células do MonthGrid). Cor accent (CA-22); clique abre o EventPopover
// (CA-21). Diferente do EventBlock (que é absoluto na grade de horas).
import { useState } from 'react';
import type { CalendarEvent } from '../../lib/api';
import { fmtHourMinute } from '../../lib/format';
import EventPopover from './EventPopover';

export default function EventChip({ event }: { event: CalendarEvent }) {
  const [open, setOpen] = useState(false);
  const label = event.all_day
    ? event.summary || '(sem título)'
    : `${fmtHourMinute(event.start)} ${event.summary || '(sem título)'}`;

  return (
    <>
      <button
        type="button"
        data-testid="cal-chip"
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
        title={label}
        style={{
          display: 'block',
          width: '100%',
          textAlign: 'left',
          overflow: 'hidden',
          whiteSpace: 'nowrap',
          textOverflow: 'ellipsis',
          padding: '1px 6px',
          borderRadius: 5,
          border: 'none',
          borderLeft: '2px solid var(--accent)',
          background: 'var(--accent-soft)',
          color: 'var(--text-1)',
          fontFamily: 'var(--font-ui)',
          fontSize: 11.5,
          lineHeight: 1.5,
          cursor: 'pointer',
        }}
      >
        {label}
      </button>
      {open && <EventPopover event={event} onClose={() => setOpen(false)} />}
    </>
  );
}
