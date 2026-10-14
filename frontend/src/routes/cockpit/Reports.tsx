// Relatórios 2.0 (fase 14): período flexível (Dia/Semana/Mês/Custom), filtros
// (cliente/projeto/objetivo), KPIs + cards novos (rastreado×não-rastreado,
// foco por hora), objetivos por ENTREGA, toggle Gráficos↔Tabela e o ângulo 4
// (Planejado×real) só no preset Semana (D-07). O rótulo do período DERIVA do
// from/to da RESPOSTA (CA-08/27). Reusa ProjectBars/FocusStats/GoalsSection/
// PlannedVsReal da 07/06 sem fork.
import { useMemo, useState } from "react";
import { useCurrentWorkspace } from "../../lib/auth";
import { useReport } from "../../lib/queries/reports";
import { usePlannedReport } from "../../lib/queries/calendar";
import { useClients } from "../../lib/queries/clients";
import { useProjects } from "../../lib/queries/projects";
import { useObjectives } from "../../lib/queries/objectives";
import {
  addDays,
  addMonths,
  addWeeks,
  isoDay,
} from "../../lib/calendar-layout";
import type { ReportParams } from "../../lib/api";
import PeriodNav, { type PeriodMode } from "../../components/reports/PeriodNav";
import ReportFilters, {
  type ReportFilterState,
} from "../../components/reports/ReportFilters";
import ChartsTableToggle, {
  type ReportView,
} from "../../components/reports/ChartsTableToggle";
import KpiStrip from "../../components/reports/KpiStrip";
import TrackedDonut from "../../components/reports/TrackedDonut";
import FocusByHour from "../../components/reports/FocusByHour";
import ObjectivesSection from "../../components/reports/ObjectivesSection";
import ReportTable from "../../components/reports/ReportTable";
import ProjectBars from "../../components/reports/ProjectBars";
import FocusStats from "../../components/reports/FocusStats";
import GoalsSection from "../../components/reports/GoalsSection";
import PlannedVsReal from "../../components/reports/PlannedVsReal";

const emptyFocus = {
  focus_seconds: 0,
  fragment_seconds: 0,
  focus_count: 0,
  fragment_count: 0,
  longest_seconds: 0,
  switches: 0,
};
const emptyAvailability = {
  tracked_seconds: 0,
  expected_seconds: 0,
  untracked_seconds: 0,
  tracked_pct: 0,
  expected_basis: "",
};
const emptyKpis = {
  tracked_seconds: 0,
  deep_focus_pct: 0,
  avg_session_seconds: 0,
  untracked_seconds: 0,
};
const zeroHours = Array.from({ length: 24 }, (_, h) => ({
  hour: h,
  focus_seconds: 0,
}));

// passo da navegação por preset (Dia ±1d, Semana ±1sem, Mês ±1mês — CA-27).
function step(mode: PeriodMode, ref: Date, dir: number): Date {
  if (mode === "day") return addDays(ref, dir);
  if (mode === "month") return addMonths(ref, dir);
  return addWeeks(ref, dir); // week (custom não navega por passo)
}

function Card({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className="tt-card"
      style={{ padding: "var(--pad)", marginBottom: "var(--gap)" }}
    >
      <p className="eyebrow" style={{ marginTop: 0 }}>
        {title}
      </p>
      {children}
    </div>
  );
}

export default function Reports() {
  const ws = useCurrentWorkspace();
  const wsId = ws?.id ?? "";

  const [mode, setMode] = useState<PeriodMode>("week");
  const [refDate, setRefDate] = useState<Date>(() => new Date());
  const [customFrom, setCustomFrom] = useState<string>(() =>
    isoDay(new Date()),
  );
  const [customTo, setCustomTo] = useState<string>(() => isoDay(new Date()));
  const [filters, setFilters] = useState<ReportFilterState>({
    projectId: "",
    clientId: "",
    objectiveId: "",
  });
  const [compare, setCompare] = useState(false);
  const [view, setView] = useState<ReportView>("charts");

  // params do useReport derivados do estado (CA-27/28).
  const params: ReportParams = useMemo(() => {
    const base: ReportParams = {
      project_id: filters.projectId || undefined,
      client_id: filters.clientId || undefined,
      objective_id: filters.objectiveId || undefined,
      compare: compare ? "previous" : undefined,
    };
    if (mode === "custom") {
      return { ...base, from: customFrom, to: customTo };
    }
    return { ...base, period: mode, date: isoDay(refDate) };
  }, [mode, refDate, customFrom, customTo, filters, compare]);

  const reportQ = useReport(wsId, params);
  const rep = reportQ.data;

  // ângulo 4 (Planejado×real) só na Semana (D-07/CA-33).
  const plannedQ = usePlannedReport(
    mode === "week" ? wsId : "",
    isoDay(refDate),
  );

  // dados pros selects de filtro
  const clientsQ = useClients(wsId);
  const projectsQ = useProjects(wsId);
  const objectivesQ = useObjectives(wsId, filters.projectId);

  return (
    <section>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--gap)",
          flexWrap: "wrap",
          gap: "var(--gap)",
        }}
      >
        <p className="eyebrow" style={{ margin: 0 }}>
          Relatórios
        </p>
        <ChartsTableToggle view={view} onView={setView} />
      </div>

      <PeriodNav
        mode={mode}
        onMode={(m) => {
          setMode(m);
          if (m !== "custom") setRefDate(new Date());
        }}
        from={rep?.from}
        to={rep?.to}
        onPrev={() => setRefDate((d) => step(mode, d, -1))}
        onNext={() => setRefDate((d) => step(mode, d, 1))}
        onToday={() => setRefDate(new Date())}
        customFrom={customFrom}
        customTo={customTo}
        onCustomFrom={setCustomFrom}
        onCustomTo={setCustomTo}
      />

      <div style={{ marginBottom: "var(--gap)" }}>
        <ReportFilters
          clients={clientsQ.data ?? []}
          projects={projectsQ.data ?? []}
          objectives={objectivesQ.data ?? []}
          value={filters}
          onChange={setFilters}
          compare={compare}
          onCompare={setCompare}
        />
      </div>

      <div style={{ marginBottom: "var(--gap)" }}>
        <KpiStrip
          kpis={rep?.kpis ?? emptyKpis}
          previous={compare ? rep?.previous : null}
        />
      </div>

      {view === "charts" ? (
        <>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
              gap: "var(--gap)",
              marginBottom: "var(--gap)",
            }}
          >
            <div className="tt-card" style={{ padding: "var(--pad)" }}>
              <p className="eyebrow" style={{ marginTop: 0 }}>
                Rastreado × não-rastreado
              </p>
              <TrackedDonut
                availability={rep?.availability ?? emptyAvailability}
              />
            </div>
            <div className="tt-card" style={{ padding: "var(--pad)" }}>
              <p className="eyebrow" style={{ marginTop: 0 }}>
                Foco por hora
              </p>
              <FocusByHour buckets={rep?.focus_by_hour ?? zeroHours} />
            </div>
          </div>

          <Card title="Tempo por cliente/projeto">
            <ProjectBars buckets={rep?.by_project ?? []} />
          </Card>

          <Card title="Objetivos (progresso por entrega)">
            <ObjectivesSection objectives={rep?.objectives ?? []} />
          </Card>

          <Card title="Foco vs. fragmentação">
            <FocusStats focus={rep?.focus ?? emptyFocus} />
          </Card>

          <Card title="Meta vs. realizado">
            <GoalsSection goals={rep?.goals ?? []} />
          </Card>

          {mode === "week" && (
            <Card title="Planejado × real">
              <PlannedVsReal report={plannedQ.data} />
            </Card>
          )}
        </>
      ) : (
        <Card title="Tabela">
          <ReportTable
            byProject={rep?.by_project ?? []}
            goals={rep?.goals ?? []}
            objectives={rep?.objectives ?? []}
          />
        </Card>
      )}
    </section>
  );
}
