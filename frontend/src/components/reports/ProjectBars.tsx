// ProjectBars (fase 07, CA-18 — ângulo 1): barras CSS puras proporcionais
// ao MAIOR bucket, fill na cor do projeto (dado de domínio — exceção
// sancionada do check-cores, mesma do ProjectChip); bucket sem projeto em
// token neutro. Durações via fmtDayTotal (RF-04).
import { fmtDayTotal } from '../../lib/format';
import type { ReportProjectBucket } from '../../lib/api';

export default function ProjectBars({ buckets }: { buckets: ReportProjectBucket[] }) {
  if (buckets.length === 0) {
    return (
      <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
        Nada cronometrado nesta semana.
      </div>
    );
  }
  const max = Math.max(...buckets.map((b) => b.total_seconds), 1);
  return (
    <div data-testid="project-bars" style={{ display: 'grid', gap: 12 }}>
      {buckets.map((b) => {
        const key = b.project_id ?? 'null';
        const pct = Math.max(2, Math.round((b.total_seconds / max) * 1000) / 10);
        return (
          <div key={key} data-testid={`bar-${key}`} style={{ display: 'grid', gap: 5 }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
              <span style={{ fontSize: 'var(--fs-row)', fontWeight: 540 }}>
                {b.name ?? 'Sem projeto'}
              </span>
              {b.client_name && (
                <span className="faint" style={{ fontSize: 11.5 }}>
                  {b.client_name}
                </span>
              )}
              <span style={{ flex: 1 }} />
              <span className="mono" style={{ fontSize: 13, fontWeight: 540 }}>
                {fmtDayTotal(b.total_seconds)}
              </span>
            </div>
            <div className="tt-goal" style={{ height: 7 }}>
              <i
                data-testid={`bar-fill-${key}`}
                style={{ width: `${pct}%`, background: b.color ?? 'var(--text-3)' }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
