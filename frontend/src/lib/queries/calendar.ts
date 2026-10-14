// Hooks da fase 06 (Google Calendar leitura): status da conexão, eventos da
// semana e o ângulo 4 (Planejado × Real). Todo fetch vai com o fuso IANA do
// navegador (padrão CA-35.1 da 03 / CA-21 da 07).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getCalendarStatus,
  getCalendarEvents,
  getPlannedReport,
  getCalendarSettings,
  putCalendarSettings,
  type CalendarSettings,
} from '../api';
import { browserTZ } from '../format';

export function useCalendarStatus() {
  return useQuery({
    queryKey: ['calendar', 'status'],
    queryFn: getCalendarStatus,
  });
}

// Só busca eventos quando conectado (evita o 409 calendar_not_connected) e
// com a semana resolvida.
export function useCalendarEvents(from: string, to: string, enabled: boolean) {
  return useQuery({
    queryKey: ['calendar', 'events', from, to, browserTZ()],
    queryFn: () => getCalendarEvents({ from, to, tz: browserTZ() }),
    enabled: enabled && from !== '' && to !== '',
  });
}

export function usePlannedReport(ws: string, date: string) {
  return useQuery({
    queryKey: ['reports', ws, 'planned', date, browserTZ()],
    queryFn: () => getPlannedReport(ws, { date, tz: browserTZ() }),
    enabled: ws !== '',
  });
}

// fase 11: preferências da UI de calendário (default_view/horas/densidade) —
// por usuário, sincroniza entre dispositivos (CA-01/02/23/24).
export const calendarSettingsKey = ['settings', 'calendar'] as const;

export function useCalendarSettings() {
  return useQuery({ queryKey: calendarSettingsKey, queryFn: getCalendarSettings });
}

export function useSaveCalendarSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (s: CalendarSettings) => putCalendarSettings(s),
    onSuccess: (saved) => qc.setQueryData(calendarSettingsKey, saved),
  });
}
