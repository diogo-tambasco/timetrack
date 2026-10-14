// Hooks de objetivos + board (fase 12, T-08): useBoard é a query central da
// página de projeto; as mutações de objetivo invalidam o boardKey → a árvore
// (incl. a conclusão automática do CA-06) reflete sem reload (CA-20/21).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createObjective,
  deleteObjective,
  getBoard,
  listObjectives,
  patchObjective,
  type ObjectiveInput,
  type ObjectivePatch,
} from '../api';

export const boardKey = (ws: string, projectId: string) => ['board', ws, projectId] as const;
export const objectivesKey = (ws: string, projectId: string) =>
  ['objectives', ws, projectId] as const;

export function useBoard(ws: string, projectId: string) {
  return useQuery({
    queryKey: boardKey(ws, projectId),
    queryFn: () => getBoard(ws, projectId),
  });
}

export function useObjectives(ws: string, projectId: string) {
  return useQuery({
    queryKey: objectivesKey(ws, projectId),
    queryFn: () => listObjectives(ws, projectId),
  });
}

export function useCreateObjective(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ObjectiveInput) => createObjective(ws, projectId, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function usePatchObjective(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: ObjectivePatch }) =>
      patchObjective(ws, projectId, id, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function useDeleteObjective(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteObjective(ws, projectId, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}
