// ReportFilters (fase 14, CA-28): selects de projeto, cliente e objetivo
// ("todos" = sem filtro) + um toggle "Comparar com o período anterior". Os
// objetivos só ficam disponíveis quando um projeto está selecionado (a API de
// objetivos é por-projeto). Mudar qualquer um refaz a busca (o pai re-renderiza
// o useReport com os filtros novos).
import type { Client, Objective, Project } from "../../lib/api";

const selectStyle = {
  background: "var(--surface-2)",
  border: "1px solid var(--border-2)",
  borderRadius: "var(--r-md)",
  padding: "6px 9px",
  fontSize: 12.5,
  color: "var(--text-1)",
  fontFamily: "var(--font-ui)",
  outline: "none",
} as const;

export type ReportFilterState = {
  projectId: string;
  clientId: string;
  objectiveId: string;
};

export default function ReportFilters({
  projects,
  clients,
  objectives,
  value,
  onChange,
  compare,
  onCompare,
}: {
  projects: Project[];
  clients: Client[];
  objectives: Objective[];
  value: ReportFilterState;
  onChange: (next: ReportFilterState) => void;
  compare: boolean;
  onCompare: (on: boolean) => void;
}) {
  return (
    <div
      data-testid="report-filters"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        flexWrap: "wrap",
      }}
    >
      <select
        aria-label="Filtrar por cliente"
        style={selectStyle}
        value={value.clientId}
        onChange={(e) => onChange({ ...value, clientId: e.target.value })}
      >
        <option value="">Todos os clientes</option>
        {clients.map((c) => (
          <option key={c.id} value={c.id}>
            {c.name}
          </option>
        ))}
      </select>

      <select
        aria-label="Filtrar por projeto"
        style={selectStyle}
        value={value.projectId}
        onChange={(e) =>
          // trocar de projeto zera o objetivo (objetivos são por projeto)
          onChange({ ...value, projectId: e.target.value, objectiveId: "" })
        }
      >
        <option value="">Todos os projetos</option>
        {projects.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>

      <select
        aria-label="Filtrar por objetivo"
        style={selectStyle}
        value={value.objectiveId}
        disabled={value.projectId === ""}
        onChange={(e) => onChange({ ...value, objectiveId: e.target.value })}
      >
        <option value="">
          {value.projectId === "" ? "Escolha um projeto" : "Todos os objetivos"}
        </option>
        {objectives.map((o) => (
          <option key={o.id} value={o.id}>
            {o.name}
          </option>
        ))}
      </select>

      <label
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          fontSize: 12.5,
          color: "var(--text-2)",
          cursor: "pointer",
        }}
      >
        <input
          type="checkbox"
          checked={compare}
          onChange={(e) => onCompare(e.target.checked)}
        />
        Comparar
      </label>
    </div>
  );
}
