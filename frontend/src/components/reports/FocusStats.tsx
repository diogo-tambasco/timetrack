// FocusStats (fase 07 — ângulo 2): tempo focado × fragmentado com %,
// maior bloco e trocas de contexto. % com guarda de divisão por zero
// (CA-20 — sem NaN em semana vazia).
import { fmtDayTotal } from '../../lib/format';
import type { ReportFocus } from '../../lib/api';

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div style={{ display: 'grid', gap: 3 }}>
      <span className="faint" style={{ fontSize: 'var(--fs-label)' }}>
        {label}
      </span>
      <span className="mono" style={{ fontSize: 19, fontWeight: 600 }}>
        {value}
      </span>
      {hint && (
        <span className="faint" style={{ fontSize: 11.5 }}>
          {hint}
        </span>
      )}
    </div>
  );
}

export default function FocusStats({ focus }: { focus: ReportFocus }) {
  const total = focus.focus_seconds + focus.fragment_seconds;
  if (total === 0) {
    return (
      <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
        Sem blocos nesta semana ainda.
      </div>
    );
  }
  const pct = Math.round((focus.focus_seconds / total) * 100);
  return (
    <div
      data-testid="focus-stats"
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))',
        gap: 'var(--gap)',
      }}
    >
      <Stat
        label="Focado (≥25min)"
        value={fmtDayTotal(focus.focus_seconds)}
        hint={`${focus.focus_count} ${focus.focus_count === 1 ? 'bloco' : 'blocos'} · ${pct}% do tempo`}
      />
      <Stat
        label="Fragmentado"
        value={fmtDayTotal(focus.fragment_seconds)}
        hint={`${focus.fragment_count} ${focus.fragment_count === 1 ? 'fragmento' : 'fragmentos'}`}
      />
      <Stat label="Maior bloco" value={fmtDayTotal(focus.longest_seconds)} />
      <Stat
        label="Trocas de contexto"
        value={String(focus.switches)}
        hint="entre entries do mesmo dia"
      />
    </div>
  );
}
