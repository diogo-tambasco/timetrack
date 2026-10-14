// ChartsTableToggle (fase 14, CA-28/31): alterna Gráficos ↔ Tabela. Estado de
// sessão (o pai decide o que renderizar). Segmented control com tokens.
export type ReportView = "charts" | "table";

const OPTS: { key: ReportView; label: string }[] = [
  { key: "charts", label: "Gráficos" },
  { key: "table", label: "Tabela" },
];

export default function ChartsTableToggle({
  view,
  onView,
}: {
  view: ReportView;
  onView: (v: ReportView) => void;
}) {
  return (
    <div
      role="group"
      aria-label="Visualização do relatório"
      style={{ display: "flex", gap: 4 }}
    >
      {OPTS.map((o) => {
        const on = o.key === view;
        return (
          <button
            key={o.key}
            type="button"
            aria-pressed={on}
            onClick={() => onView(o.key)}
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
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
