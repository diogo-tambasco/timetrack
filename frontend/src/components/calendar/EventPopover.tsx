// EventPopover (fase 11, CA-21): detalhe de um evento num card flutuante —
// título, faixa de horário (ou "dia inteiro") e link "Abrir no Google" (só
// com html_link; target=_blank, rel=noopener). Fecha no Esc ou clique fora.
import { useEffect, useRef } from 'react';
import type { CalendarEvent } from '../../lib/api';
import { fmtHourMinute } from '../../lib/format';

export default function EventPopover({
  event,
  onClose,
}: {
  event: CalendarEvent;
  onClose: () => void;
}) {
  const cardRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  const horario = event.all_day
    ? 'dia inteiro'
    : `${fmtHourMinute(event.start)} – ${fmtHourMinute(event.end)}`;

  return (
    <div
      data-testid="event-popover-backdrop"
      onClick={(e) => {
        // CA-21: dispensar o popover (clique fora) NÃO pode borbulhar pro
        // container — na view Mês a month-cell tem onClick que navegaria pro
        // Dia (achado MEDIUM da verificação do build). O popover é IRMÃO do
        // botão do chip (não filho), então o stopPropagation do botão não
        // cobre o backdrop — precisa stopar aqui.
        e.stopPropagation();
        onClose();
      }}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 50,
        display: 'grid',
        placeItems: 'center',
        background: 'rgba(0,0,0,0.32)',
      }}
    >
      <div
        ref={cardRef}
        data-testid="event-popover"
        role="dialog"
        aria-label="Detalhe do evento"
        onClick={(e) => e.stopPropagation()}
        className="tt-card"
        style={{
          padding: 'var(--pad)',
          display: 'grid',
          gap: 8,
          minWidth: 260,
          maxWidth: 360,
          background: 'var(--surface-2)',
          border: '1px solid var(--border-2)',
        }}
      >
        <strong style={{ fontSize: 'var(--fs-row)' }}>{event.summary || '(sem título)'}</strong>
        <span className="muted mono" style={{ fontSize: 12.5 }}>
          {horario}
        </span>
        {event.html_link && (
          <a
            href={event.html_link}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="open-in-google"
            className="tt-btn tt-btn--accent"
            style={{ width: 'fit-content', textDecoration: 'none', marginTop: 4 }}
          >
            Abrir no Google
          </a>
        )}
      </div>
    </div>
  );
}
