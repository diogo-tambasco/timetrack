// Hooks da config de Pomodoro (T-18): GET com defaults do servidor
// (CA-19) e PUT validado (CA-18/28) — a config é por usuário, não por
// workspace (D-08).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getPomodoroSettings, putPomodoroSettings, type PomodoroSettings } from '../api';

export const pomodoroKey = ['settings', 'pomodoro'] as const;

export function usePomodoroSettings() {
  return useQuery({ queryKey: pomodoroKey, queryFn: getPomodoroSettings });
}

export function useSavePomodoro() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (s: PomodoroSettings) => putPomodoroSettings(s),
    onSuccess: (saved) => qc.setQueryData(pomodoroKey, saved),
  });
}
