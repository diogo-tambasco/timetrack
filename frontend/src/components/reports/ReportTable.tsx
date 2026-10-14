// ReportTable (fase 14, CA-31): tabela densa on-demand (tt-table do design),
// UMA linha por projeto, SÓ com colunas que o contrato fornece POR PROJETO:
// projeto, cliente, rastreado (by_project.total_seconds), meta (goals.
// goal_seconds, "—" sem meta) e objetivos concluídos/total (da seção objectives
// agrupada por project_id). NÃO fabrica não-rastreado/foco%/sessões por projeto
// (são agregados de workspace — vivem na faixa de KPIs / cards). Só leitura.
//
// Fase 22 (CA-08): abaixo de md a tabela vira CARTÕES empilhados (1 por linha,
// label:valor, TODOS os dados — nada removido), pra caber a 375px sem estourar.
// ≥ md = a tt-table de hoje, intocada. useMediaQuery decide o modo.
import { fmtDayTotal } from "../../lib/format";
import { useMediaQuery } from "../../lib/useMediaQuery";
import type {
  ReportGoal,
  ReportObjective,
  ReportProjectBucket,
} from "../../lib/api";

interface Row {
  key: string;
  name: string;
  client: string;
  tracked: string;
  goal: string;
  objectives: string;
}

export default function ReportTable({
  byProject,
  goals,
  objectives,
}: {
  byProject: ReportProjectBucket[];
  goals: ReportGoal[];
  objectives: ReportObjective[];
}) {
  const isDesktop = useMediaQuery("(min-width: 768px)");

  if (byProject.length === 0) {
    return (
      <div className="muted" style={{ padding: "14px 4px", fontSize: 13 }}>
        Nada cronometrado no período.
      </div>
    );
  }

  const goalByProject = new Map<string, number>();
  for (const g of goals) goalByProject.set(g.project_id, g.goal_seconds);

  // objetivos concluídos/total por project_id
  const objAgg = new Map<string, { done: number; total: number }>();
  for (const o of objectives) {
    const agg = objAgg.get(o.project_id) ?? { done: 0, total: 0 };
    agg.total += 1;
    if (o.status === "done") agg.done += 1;
    objAgg.set(o.project_id, agg);
  }

  // mesmas células pros dois modos (table e cartões) — dados idênticos (CA-08).
  const rows: Row[] = byProject.map((b) => {
    const key = b.project_id ?? "null";
    const goal = b.project_id ? goalByProject.get(b.project_id) : undefined;
    const agg = b.project_id ? objAgg.get(b.project_id) : undefined;
    return {
      key,
      name: b.name ?? "Sem projeto",
      client: b.client_name ?? "—",
      tracked: fmtDayTotal(b.total_seconds),
      goal: goal ? fmtDayTotal(goal) : "—",
      objectives: agg ? `${agg.done}/${agg.total}` : "—",
    };
  });

  // < md: cartões empilhados (label:valor — todos os dados; nada removido).
  if (!isDesktop) {
    return (
      <div
        data-testid="report-table"
        style={{ display: "flex", flexDirection: "column", gap: 8 }}
      >
        {rows.map((r) => (
          <div
            key={r.key}
            className="tt-card"
            data-testid={`row-${r.key}`}
            style={{ padding: "12px 14px", display: "flex", flexDirection: "column", gap: 6 }}
          >
            <div style={{ fontWeight: 600, fontSize: 14 }}>{r.name}</div>
            <CardField label="Cliente" value={r.client} />
            <CardField label="Rastreado" value={r.tracked} />
            <CardField label="Meta" value={r.goal} />
            <CardField label="Objetivos" value={r.objectives} />
          </div>
        ))}
      </div>
    );
  }

  // ≥ md: a tabela densa de hoje, intocada.
  return (
    <table className="tt-table" data-testid="report-table">
      <thead>
        <tr>
          <th>Projeto</th>
          <th>Cliente</th>
          <th className="num">Rastreado</th>
          <th className="num">Meta</th>
          <th className="num">Objetivos</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.key} data-testid={`row-${r.key}`}>
            <td>{r.name}</td>
            <td>{r.client}</td>
            <td className="num">{r.tracked}</td>
            <td className="num">{r.goal}</td>
            <td className="num">{r.objectives}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// célula label:valor do modo cartão (mobile). Mantém o label visível (todos os
// dados acessíveis — §7 da spec).
function CardField({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13 }}>
      <span className="faint">{label}</span>
      <span style={{ fontVariantNumeric: "tabular-nums" }}>{value}</span>
    </div>
  );
}
