// FocusByHour (fase 14, CA-22/23/30): 24 barras de intensidade de foco por
// hora local (sempre 24 buckets, vindos da resposta). Altura proporcional ao
// pico; CSS puro com tokens (--accent / --surface-3 / --text-3), sem hex.
import type { FocusHourBucket } from "../../lib/api";

export default function FocusByHour({
  buckets,
}: {
  buckets: FocusHourBucket[];
}) {
  const max = Math.max(...buckets.map((b) => b.focus_seconds), 1);
  return (
    <div
      data-testid="focus-by-hour"
      style={{ display: "flex", alignItems: "flex-end", gap: 3, height: 88 }}
    >
      {buckets.map((b) => {
        const h =
          b.focus_seconds === 0
            ? 2
            : Math.max(4, Math.round((b.focus_seconds / max) * 80));
        return (
          <div
            key={b.hour}
            data-testid={`hour-${b.hour}`}
            title={`${b.hour}h`}
            style={{
              flex: 1,
              height: h,
              borderRadius: 3,
              background:
                b.focus_seconds > 0 ? "var(--accent)" : "var(--surface-3)",
            }}
          />
        );
      })}
    </div>
  );
}
