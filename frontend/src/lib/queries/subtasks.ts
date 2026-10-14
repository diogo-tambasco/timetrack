// Hooks de sub-tasks (fase 24, CA-23): useSubtasks(taskId) + mutações que
// invalidam subtasksKey(taskId) E o boardKey (pro cabeçalho/contagem do board
// refletir sem reload). O board fica num arquivo separado (objectives.ts).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createSubtask,
  deleteSubtask,
  listSubtasks,
  patchSubtask,
  type SubtaskInput,
} from '../api';
import { boardKey } from './objectives';

export const subtasksKey = (ws: string, taskId: string) =>
  ['subtasks', ws, taskId] as const;

export function useSubtasks(ws: string, taskId: string) {
  return useQuery({
    queryKey: subtasksKey(ws, taskId),
    queryFn: () => listSubtasks(ws, taskId),
  });
}

// invalida a lista da task E o board do projeto (contagens no card).
function useSubtaskInvalidate(ws: string, taskId: string, projectId: string) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: subtasksKey(ws, taskId) });
    qc.invalidateQueries({ queryKey: boardKey(ws, projectId) });
  };
}

export function useCreateSubtask(ws: string, taskId: string, projectId: string) {
  const invalidate = useSubtaskInvalidate(ws, taskId, projectId);
  return useMutation({
    mutationFn: (title: string) => createSubtask(ws, taskId, title),
    onSuccess: invalidate,
  });
}

export function usePatchSubtask(ws: string, taskId: string, projectId: string) {
  const invalidate = useSubtaskInvalidate(ws, taskId, projectId);
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: SubtaskInput }) =>
      patchSubtask(ws, taskId, id, patch),
    onSuccess: invalidate,
  });
}

export function useDeleteSubtask(ws: string, taskId: string, projectId: string) {
  const invalidate = useSubtaskInvalidate(ws, taskId, projectId);
  return useMutation({
    mutationFn: (id: string) => deleteSubtask(ws, taskId, id),
    onSuccess: invalidate,
  });
}
