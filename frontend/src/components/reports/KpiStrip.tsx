// KpiStrip (fase 14, CA-24/30): faixa de KPIs (rastreado, foco profundo %,
// sessão média, não-rastreado). Com compare ligado, cada KPI mostra o delta vs.
// o período anterior, definido SEM div/0 (CA-30):
//   anterior > 0 → "±N%" (round((atual−anterior)/anterior×100), com sinal)
//   anterior = 0 e atual > 0 → "novo"
//   ambos 0 → "0%"
// Nunca NaN%/Infinity%. Só tokens (sem hex).
import { fmtDayTotal } from "../../lib/format";
import type { ReportKpis, ReportPrevious } from "../../lib/api";

// deltaLabel — a fórmula do CA-30 (furo confirmado na verificação da spec).
export function deltaLabel(current: number, previous: number): string {
  if (previous > 0) {
    const pct = Math.round(((current - previous) / previous) * 100);
    return `${pct >= 0 ? "+" : ""}${pct}%`;
  }
  if (current > 0) return "novo";
  return "0%";
}

type Kpi = { key: string; label: string; value: number; fmtPct?: boolean };

export default function KpiStrip({
  kpis,
  previous,
}: {
  kpis: ReportKpis;
  previous?: ReportPrevious | null;
}) {
  const items: Kpi[] = [
    { key: "tracked", label: "Rastreado", value: kpis.tracked_seconds },
    {
      key: "deep",
      label: "Foco profundo",
      value: kpis.deep_focus_pct,
      fmtPct: true,
    },
    { key: "avg", label: "Sessão média", value: kpis.avg_session_seconds },
    { key: "untracked", label: "Não-rastreado", value: kpis.untracked_seconds },
  ];

  const prevValue = (key: string): number | undefined => {
    if (!previous) return undefined;
    switch (key) {
      case "tracked":
        return previous.kpis.tracked_seconds;
      case "deep":
        return previous.kpis.deep_focus_pct;
      case "avg":
        return previous.kpis.avg_session_seconds;
      case "untracked":
        return previous.kpis.untracked_seconds;
      default:
        return undefined;
    }
  };

  return (
    <div
      data-testid="kpi-strip"
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
        gap: "var(--gap)",
      }}
    >
      {items.map((it) => {
        const prev = prevValue(it.key);
        const shown = it.fmtPct ? `${it.value}%` : fmtDayTotal(it.value);
        return (
          <div
            key={it.key}
            data-testid={`kpi-${it.key}`}
            className="tt-card"
            style={{ padding: "var(--pad)", display: "grid", gap: 4 }}
          >
            <span className="eyebrow" style={{ margin: 0 }}>
              {it.label}
            </span>
            <strong className="mono" style={{ fontSize: 22, lineHeight: 1 }}>
              {shown}
            </strong>
            {prev !== undefined && (
              <span
                data-testid={`kpi-delta-${it.key}`}
                style={{ fontSize: 12, color: "var(--text-3)" }}
              >
                {deltaLabel(it.value, prev)}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
