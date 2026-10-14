// Hooks do relatório semanal (fase 07) e do relatório de período (fase 14).
// A key sempre inclui o fuso IANA do navegador (CA-21); o useReport inclui
// período/filtros/compare. useReportSettings/useSaveReportSettings espelham as
// settings de calendar (fase 11).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getReport,
  getReportSettings,
  getWeekReport,
  putReportSettings,
  type ReportParams,
  type ReportSettings,
} from "../api";
import { browserTZ } from "../format";

export const reportsKey = (ws: string) => ["reports", ws] as const;

export function useWeekReport(ws: string, date: string) {
  return useQuery({
    queryKey: [...reportsKey(ws), "week", date, browserTZ()],
    queryFn: () => getWeekReport(ws, { date, tz: browserTZ() }),
    enabled: ws !== "",
  });
}

// useReport — relatório de período (fase 14). A key inclui TODOS os params +
// tz; toda chamada vai com o tz do navegador (CA-21/28).
export function useReport(ws: string, params: ReportParams) {
  const full: ReportParams = { ...params, tz: browserTZ() };
  return useQuery({
    queryKey: [...reportsKey(ws), "period", full],
    queryFn: () => getReport(ws, full),
    enabled: ws !== "",
  });
}

export const reportSettingsKey = ["report-settings"] as const;

export function useReportSettings() {
  return useQuery({
    queryKey: reportSettingsKey,
    queryFn: getReportSettings,
  });
}

export function useSaveReportSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (s: ReportSettings) => putReportSettings(s),
    onSuccess: (saved) => {
      qc.setQueryData(reportSettingsKey, saved);
      // o expected_seconds depende das settings → invalida os relatórios
      qc.invalidateQueries({ queryKey: ["reports"] });
    },
  });
}
