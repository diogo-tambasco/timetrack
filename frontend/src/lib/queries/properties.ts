// Hooks de propriedades tipadas (fase 25, T-08): toda mutação (def ou value)
// invalida o boardKey da fase 12 → a Table/popup refletem sem reload (CA-20/21).
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  clearPropertyValue,
  createProperty,
  deleteProperty,
  patchProperty,
  setPropertyValue,
  type CreatePropertyInput,
  type PatchPropertyInput,
} from "../api";
import { boardKey } from "./objectives";

export function useCreateProperty(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreatePropertyInput) =>
      createProperty(ws, projectId, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function usePatchProperty(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: PatchPropertyInput }) =>
      patchProperty(ws, projectId, id, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function useDeleteProperty(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteProperty(ws, projectId, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function useSetPropertyValue(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      taskId,
      defId,
      value,
    }: {
      taskId: string;
      defId: string;
      value: unknown;
    }) => setPropertyValue(ws, projectId, taskId, defId, value),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}

export function useClearPropertyValue(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ taskId, defId }: { taskId: string; defId: string }) =>
      clearPropertyValue(ws, projectId, taskId, defId),
    onSuccess: () => qc.invalidateQueries({ queryKey: boardKey(ws, projectId) }),
  });
}
