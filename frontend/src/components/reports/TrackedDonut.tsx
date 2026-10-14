// TrackedDonut (fase 14, CA-20/30): donut em conic-gradient (sem lib de
// gráfico — RNF-01) do % rastreado × não-rastreado. % no centro; legenda com
// rastreado/não-rastreado formatados. Só tokens (--accent / --surface-3 /
// --text-*), sem hex.
import { fmtDayTotal } from "../../lib/format";
import type { ReportAvailability } from "../../lib/api";

export default function TrackedDonut({
  availability,
}: {
  availability: ReportAvailability;
}) {
  const pct = availability.tracked_pct; // já capado 0..100, 0 se expected=0
  return (
    <div
      data-testid="tracked-donut"
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--gap)",
        flexWrap: "wrap",
      }}
    >
      <div
        data-testid="donut-ring"
        style={{
          width: 120,
          height: 120,
          borderRadius: "50%",
          background: `conic-gradient(var(--accent) ${pct}%, var(--surface-3) 0)`,
          display: "grid",
          placeItems: "center",
          position: "relative",
        }}
      >
        <div
          style={{
            width: 84,
            height: 84,
            borderRadius: "50%",
            background: "var(--surface-2)",
            display: "grid",
            placeItems: "center",
          }}
        >
          <strong className="mono" style={{ fontSize: 20 }}>
            {pct}%
          </strong>
        </div>
      </div>
      <div style={{ display: "grid", gap: 6, fontSize: 12.5 }}>
        <span style={{ color: "var(--text-2)" }}>
          <b className="mono">{fmtDayTotal(availability.tracked_seconds)}</b>{" "}
          rastreado
        </span>
        <span style={{ color: "var(--text-3)" }}>
          <b className="mono">{fmtDayTotal(availability.untracked_seconds)}</b>{" "}
          não-rastreado
        </span>
        <span className="faint" style={{ fontSize: 11.5 }}>
          {availability.expected_basis}
        </span>
      </div>
    </div>
  );
}
