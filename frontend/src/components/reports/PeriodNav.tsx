// PeriodNav (fase 14, CA-27): seletor Dia/Semana/Mês/Custom + ‹ Hoje › e o
// rótulo do período DERIVADO do from/to da RESPOSTA (CA-08, nunca cálculo
// local). Navegação por passo coerente com o preset (Dia ±1d, Semana ±1sem,
// Mês ±1mês); Custom desliga a navegação por passo e mostra um date-range.
import type { ReportPeriod } from "../../lib/api";

export type PeriodMode = ReportPeriod | "custom";

// estilo dos inputs (espelha o inputStyle do ProjectModal — tokens, sem hex)
const inputStyle = {
  background: "var(--surface-2)",
  border: "1px solid var(--border-2)",
  borderRadius: "var(--r-md)",
  padding: "6px 9px",
  fontSize: 12.5,
  color: "var(--text-1)",
  fontFamily: "var(--font-ui)",
  outline: "none",
} as const;

const PRESETS: { key: PeriodMode; label: string }[] = [
  { key: "day", label: "Dia" },
  { key: "week", label: "Semana" },
  { key: "month", label: "Mês" },
  { key: "custom", label: "Custom" },
];

// '2026-06-08' → Date local (new Date(iso) parsearia como UTC e recuaria um dia)
function parseISODate(s: string): Date {
  const [y, m, d] = s.split("-").map(Number);
  return new Date(y, m - 1, d);
}
const fmtDM = (d: Date) =>
  `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`;

// rótulo do período a partir do from/to da resposta (CA-27/08).
export function periodLabel(from?: string, to?: string): string {
  if (!from || !to) return "—";
  const a = parseISODate(from);
  const b = parseISODate(to);
  if (from === to) return `${fmtDM(a)}/${a.getFullYear()}`;
  return `${fmtDM(a)} – ${fmtDM(b)}/${b.getFullYear()}`;
}

export default function PeriodNav({
  mode,
  onMode,
  from,
  to,
  onPrev,
  onNext,
  onToday,
  customFrom,
  customTo,
  onCustomFrom,
  onCustomTo,
}: {
  mode: PeriodMode;
  onMode: (m: PeriodMode) => void;
  from?: string;
  to?: string;
  onPrev: () => void;
  onNext: () => void;
  onToday: () => void;
  customFrom: string;
  customTo: string;
  onCustomFrom: (v: string) => void;
  onCustomTo: (v: string) => void;
}) {
  const stepDisabled = mode === "custom";
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: "var(--gap)",
        marginBottom: "var(--gap)",
        flexWrap: "wrap",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label="Período anterior"
          disabled={stepDisabled}
          onClick={onPrev}
        >
          ‹
        </button>
        <button
          type="button"
          className="tt-btn"
          disabled={stepDisabled}
          onClick={onToday}
        >
          Hoje
        </button>
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label="Próximo período"
          disabled={stepDisabled}
          onClick={onNext}
        >
          ›
        </button>
        <strong
          data-testid="period-label"
          className="mono"
          style={{ fontSize: 13, marginLeft: 6 }}
        >
          {periodLabel(from, to)}
        </strong>
      </div>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          flexWrap: "wrap",
        }}
      >
        {mode === "custom" && (
          <div
            style={{ display: "flex", alignItems: "center", gap: 6 }}
            data-testid="custom-range"
          >
            <input
              type="date"
              aria-label="De"
              style={inputStyle}
              value={customFrom}
              onChange={(e) => onCustomFrom(e.target.value)}
            />
            <span style={{ color: "var(--text-3)" }}>–</span>
            <input
              type="date"
              aria-label="Até"
              style={inputStyle}
              value={customTo}
              onChange={(e) => onCustomTo(e.target.value)}
            />
          </div>
        )}
        <div
          role="group"
          aria-label="Período"
          style={{ display: "flex", gap: 4 }}
        >
          {PRESETS.map((p) => {
            const on = p.key === mode;
            return (
              <button
                key={p.key}
                type="button"
                aria-pressed={on}
                onClick={() => onMode(p.key)}
                style={{
                  padding: "6px 12px",
                  borderRadius: 8,
                  cursor: "pointer",
                  fontFamily: "var(--font-ui)",
                  fontSize: 12.5,
                  fontWeight: 540,
                  border: "1px solid",
                  borderColor: on ? "var(--accent)" : "var(--border-2)",
                  background: on ? "var(--accent-soft)" : "transparent",
                  color: on ? "var(--accent)" : "var(--text-2)",
                }}
              >
                {p.label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
