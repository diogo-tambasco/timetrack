// Hooks de clientes (T-13, D-12): TanStack Query sobre lib/api.ts —
// mutações invalidam a query da lista (CA-29 sem store manual).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { archiveClient, createClient, listClients, renameClient } from '../api';

export const clientsKey = (ws: string) => ['clients', ws] as const;

export function useClients(ws: string, opts?: { archived?: boolean }) {
  return useQuery({
    queryKey: [...clientsKey(ws), { archived: opts?.archived ?? false }],
    queryFn: () => listClients(ws, opts),
  });
}

export function useCreateClient(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string }) => createClient(ws, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: clientsKey(ws) }),
  });
}

export function useRenameClient(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => renameClient(ws, id, { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: clientsKey(ws) }),
  });
}

export function useArchiveClient(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => archiveClient(ws, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: clientsKey(ws) }),
  });
}
