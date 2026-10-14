// Hooks de tasks (T-13, D-12): lista com filtros combináveis (CA-19) e
// mutações com invalidação — criar task aparece sem reload (CA-30.1).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createTask,
  deleteTask,
  listTasks,
  patchTask,
  type TaskFilters,
  type TaskInput,
  type TaskStatus,
} from '../api';

export const tasksKey = (ws: string) => ['tasks', ws] as const;

export function useTasks(ws: string, filters?: TaskFilters) {
  return useQuery({
    queryKey: [...tasksKey(ws), filters ?? {}],
    queryFn: () => listTasks(ws, filters),
  });
}

export function useCreateTask(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: TaskInput) => createTask(ws, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: tasksKey(ws) }),
  });
}

export function usePatchTask(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: Partial<TaskInput> & { status?: TaskStatus } }) =>
      patchTask(ws, id, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: tasksKey(ws) }),
  });
}

export function useDeleteTask(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteTask(ws, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: tasksKey(ws) }),
  });
}
