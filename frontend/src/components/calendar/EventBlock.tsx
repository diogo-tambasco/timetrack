// EventBlock (fase 11, CA-09/10/12/21/22): bloco de evento posicionado na
// grade de horas. Recebe um `Positioned` (top/height em fração do dia,
// colIndex/colCount pra largura) e se posiciona ABSOLUTO dentro da coluna do
// dia (que tem altura = 24×row e position:relative). Cor = accent do app
// (CA-22, sem cor por evento). Clique abre o EventPopover (CA-21).
import { useState } from 'react';
import type { Positioned } from '../../lib/calendar-layout';
import { fmtHourMinute } from '../../lib/format';
import EventPopover from './EventPopover';

export default function EventBlock({ p }: { p: Positioned }) {
  const [open, setOpen] = useState(false);
  const e = p.event;
  const gap = 2; // px entre colunas sobrepostas

  return (
    <>
      <button
        type="button"
        data-testid="cal-event"
        onClick={() => setOpen(true)}
        title={e.summary || '(sem título)'}
        style={{
          position: 'absolute',
          top: `${p.topPct * 100}%`,
          height: `${p.heightPct * 100}%`,
          left: `calc(${(p.colIndex / p.colCount) * 100}% + ${gap}px)`,
          width: `calc(${(1 / p.colCount) * 100}% - ${gap * 2}px)`,
          overflow: 'hidden',
          textAlign: 'left',
          padding: '2px 6px',
          borderRadius: 6,
          border: 'none',
          borderLeft: '2px solid var(--accent)',
          background: 'var(--accent-soft)',
          color: 'var(--text-1)',
          fontFamily: 'var(--font-ui)',
          fontSize: 12,
          lineHeight: 1.25,
          cursor: 'pointer',
        }}
      >
        <span className="mono" style={{ color: 'var(--accent)', fontSize: 10.5 }}>
          {fmtHourMinute(e.start)}
        </span>{' '}
        <span style={{ fontWeight: 540 }}>{e.summary || '(sem título)'}</span>
      </button>
      {open && <EventPopover event={e} onClose={() => setOpen(false)} />}
    </>
  );
}
