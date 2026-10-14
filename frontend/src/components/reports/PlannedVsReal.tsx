// PlannedVsReal (fase 06, CA-20 — ângulo 4): por dia da semana, duas barras
// (planejado no GCal × realizado cronometrado), proporcionais ao maior valor.
// Não conectado → CTA de conexão (sem quebrar os ângulos 1–3). Barras CSS
// puras (tt-goal), realizado no accent.
import { fmtDayTotal } from '../../lib/format';
import { CALENDAR_CONNECT_PATH } from '../../lib/api';
import type { PlannedReport } from '../../lib/api';

const DOW = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];

function weekdayOf(dateISO: string): string {
  const [y, m, d] = dateISO.split('-').map(Number);
  const dt = new Date(y, m - 1, d);
  return DOW[(dt.getDay() + 6) % 7];
}

export default function PlannedVsReal({ report }: { report: PlannedReport | undefined }) {
  if (!report) {
    return (
      <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
        Carregando…
      </div>
    );
  }
  if (!report.connected) {
    return (
      <div style={{ display: 'grid', gap: 10 }}>
        <span className="muted" style={{ fontSize: 13 }}>
          Conecte o Google Calendar pra comparar o que você planejou × o que cronometrou.
        </span>
        <a
          href={CALENDAR_CONNECT_PATH}
          data-testid="planned-connect"
          className="tt-btn tt-btn--accent"
          style={{ width: 'fit-content', textDecoration: 'none' }}
        >
          Conectar Google Calendar
        </a>
      </div>
    );
  }

  const max = Math.max(...report.days.flatMap((d) => [d.planned_seconds, d.realized_seconds]), 1);
  const pct = (sec: number) => Math.max(0, Math.round((sec / max) * 1000) / 10);

  return (
    <div data-testid="planned-vs-real" style={{ display: 'grid', gap: 12 }}>
      {report.days.map((d) => (
        <div key={d.date} data-testid={`pvr-${d.date}`} style={{ display: 'grid', gap: 5 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
            <span style={{ fontSize: 'var(--fs-row)', fontWeight: 540, minWidth: 34 }}>
              {weekdayOf(d.date)}
            </span>
            <span style={{ flex: 1 }} />
            <span className="mono faint" style={{ fontSize: 12 }}>
              plan {fmtDayTotal(d.planned_seconds)}
            </span>
            <span className="mono" style={{ fontSize: 12, fontWeight: 540 }}>
              real {fmtDayTotal(d.realized_seconds)}
            </span>
          </div>
          <div className="tt-goal" style={{ height: 6 }}>
            <i style={{ width: `${pct(d.planned_seconds)}%`, background: 'var(--text-3)' }} />
          </div>
          <div className="tt-goal" style={{ height: 6 }}>
            <i style={{ width: `${pct(d.realized_seconds)}%`, background: 'var(--accent)' }} />
          </div>
        </div>
      ))}
      <div className="faint" style={{ display: 'flex', gap: 16, fontSize: 11.5, marginTop: 2 }}>
        <span>▮ planejado</span>
        <span style={{ color: 'var(--accent)' }}>▮ realizado</span>
      </div>
    </div>
  );
}
