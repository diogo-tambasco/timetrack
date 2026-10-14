// Hooks de time entries (T-17): a lista do dia SEMPRE vai com o fuso IANA
// do navegador (CA-35.1 → recorte certo no servidor, CA-14.1); continuar
// aplica o TimerResult no timerStore (o timer começa na hora) e invalida o
// dia.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { continueEntry, createEntry, listEntries, patchEntry, type EntryInput } from '../api';
import { browserTZ } from '../format';
import { RECENT_DAYS, addDaysISO, localDayISO } from '../recent';
import { useTimerStore } from '../timerStore';

export const entriesKey = (ws: string) => ['entries', ws] as const;

export function useTodayEntries(ws: string) {
  return useQuery({
    queryKey: [...entriesKey(ws), 'today', browserTZ()],
    queryFn: () => listEntries(ws, { tz: browserTZ() }),
  });
}

// useRecentEntries (fase 13, CA-06) — janela dos últimos RECENT_DAYS dias
// (from = hoje−6, to = hoje, no fuso do navegador), reusando o filtro from/to
// da fase 03 (sem endpoint novo — RNF-03). Resolve a virada do dia (P3).
export function useRecentEntries(ws: string) {
  const tz = browserTZ();
  const today = localDayISO(new Date().toISOString(), tz);
  const from = addDaysISO(today, -(RECENT_DAYS - 1));
  return useQuery({
    queryKey: [...entriesKey(ws), 'recent', tz, from, today],
    queryFn: () => listEntries(ws, { from, to: today, tz }),
  });
}

export function useContinueEntry(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => continueEntry(ws, id),
    onSuccess: (result) => {
      useTimerStore.getState().apply(result);
      void qc.invalidateQueries({ queryKey: entriesKey(ws) });
    },
  });
}

export function useCreateEntry(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: EntryInput) => createEntry(ws, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: entriesKey(ws) }),
  });
}

export function usePatchEntry(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: Partial<EntryInput> }) =>
      patchEntry(ws, id, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: entriesKey(ws) }),
  });
}
