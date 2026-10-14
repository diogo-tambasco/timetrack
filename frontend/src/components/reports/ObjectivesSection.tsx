// ObjectivesSection (fase 14, CA-16..19): progresso de objetivos por ENTREGA.
// Barra de % de tasks done (progress_pct) na cor do projeto; tasks_done/total;
// realizado (e meta, se houver). Objetivo done marcado com um selo. Cor de
// projeto é dado de domínio (exceção sancionada do check-cores, como ProjectBars).
import { fmtDayTotal } from "../../lib/format";
import type { ReportObjective } from "../../lib/api";

export default function ObjectivesSection({
  objectives,
}: {
  objectives: ReportObjective[];
}) {
  if (objectives.length === 0) {
    return (
      <div className="muted" style={{ padding: "14px 4px", fontSize: 13 }}>
        Nenhum objetivo com atividade ou meta no período.
      </div>
    );
  }
  return (
    <div data-testid="objectives-section" style={{ display: "grid", gap: 16 }}>
      {objectives.map((o) => {
        const done = o.status === "done";
        return (
          <div
            key={o.objective_id}
            data-testid={`objective-${o.objective_id}`}
            style={{ display: "grid", gap: 5 }}
          >
            <div
              style={{
                display: "flex",
                alignItems: "baseline",
                gap: 9,
                flexWrap: "wrap",
              }}
            >
              <span style={{ fontSize: "var(--fs-row)", fontWeight: 540 }}>
                {o.objective_name}
              </span>
              <span className="faint" style={{ fontSize: 11.5 }}>
                {o.project_name}
              </span>
              {done && (
                <span
                  data-testid={`objective-done-${o.objective_id}`}
                  style={{
                    fontSize: 11,
                    fontWeight: 600,
                    padding: "1px 8px",
                    borderRadius: 999,
                    background: "var(--accent-soft)",
                    color: "var(--accent)",
                  }}
                >
                  Concluído
                </span>
              )}
              <span style={{ flex: 1 }} />
              <span className="mono" style={{ fontSize: 12.5 }}>
                {o.tasks_done}/{o.tasks_total} tarefas ({o.progress_pct}%)
              </span>
            </div>
            <div className="tt-goal" style={{ height: 7 }}>
              <i
                data-testid={`objective-fill-${o.objective_id}`}
                style={{
                  width: `${o.progress_pct}%`,
                  background: done ? "var(--accent)" : o.project_color,
                }}
              />
            </div>
            <div
              className="mono"
              style={{ fontSize: 12, color: "var(--text-3)" }}
            >
              {fmtDayTotal(o.realized_seconds)} no período
              {o.goal_seconds ? (
                <span className="faint">
                  {" "}
                  · meta {fmtDayTotal(o.goal_seconds)}
                </span>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}
