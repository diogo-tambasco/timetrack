// Hooks dos lembretes (fase 20, plan §6). useReminders lista os MEUS pendentes
// (cross-workspace, rota por-usuário); as mutações invalidam a key no onSuccess
// (lembrete novo/cancelado aparece/some sem reload). Espelha lib/queries/
// comments.ts. NÃO toca os hooks de task/comentário (RNF-05).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  cancelReminder,
  createReminder,
  listReminders,
  type ReminderStatus,
} from '../api';

export const remindersKey = (status: ReminderStatus = 'pending') =>
  ['reminders', status] as const;

export function useReminders(status: ReminderStatus = 'pending') {
  return useQuery({
    queryKey: remindersKey(status),
    queryFn: () => listReminders(status),
  });
}

export function useCreateReminder(ws: string, task: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (remindAt: string) => createReminder(ws, task, remindAt),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ['reminders'] as const }),
  });
}

export function useCancelReminder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { ws: string; task: string; reminderId: string }) =>
      cancelReminder(v.ws, v.task, v.reminderId),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ['reminders'] as const }),
  });
}
