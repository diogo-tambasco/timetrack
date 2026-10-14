// Hooks do detalhe do workspace (fase 10/02) + convites/gestão de membros
// (fase 17). A lista de membros alimenta o ProjectModal (CA-28) e a página
// /workspace; os convites vêm da rota nova GET /invitations.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addMember,
  changeRole,
  createInvitation,
  getWorkspace,
  listInvitations,
  removeMember,
  revokeInvitation,
  type InviteRole,
  type WorkspaceRole,
} from "../api";

export const workspaceKey = (ws: string) => ["workspace", ws] as const;
export const invitationsKey = (ws: string) => ["invitations", ws] as const;

export function useWorkspaceMembers(ws: string) {
  return useQuery({
    queryKey: workspaceKey(ws),
    queryFn: () => getWorkspace(ws),
    select: (w) => w.members,
  });
}

// fase 17: lista de convites pendentes — só habilitada pro owner (CA-21/13).
export function useInvitations(ws: string, opts?: { enabled?: boolean }) {
  return useQuery({
    queryKey: invitationsKey(ws),
    queryFn: () => listInvitations(ws),
    enabled: opts?.enabled ?? true,
  });
}

export function useCreateInvitation(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { email: string; role: InviteRole }) =>
      createInvitation(ws, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: invitationsKey(ws) }),
  });
}

export function useRevokeInvitation(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => revokeInvitation(ws, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: invitationsKey(ws) }),
  });
}

// add de quem já tem conta → vira membership na hora (CA-10): invalida a lista
// de membros (workspaceKey).
export function useAddMember(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { email: string; role: InviteRole }) =>
      addMember(ws, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: workspaceKey(ws) }),
  });
}

export function useRemoveMember(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => removeMember(ws, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: workspaceKey(ws) }),
  });
}

export function useChangeRole(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { userId: string; role: WorkspaceRole }) =>
      changeRole(ws, vars.userId, vars.role),
    onSuccess: () => qc.invalidateQueries({ queryKey: workspaceKey(ws) }),
  });
}
