// Hooks de projetos (T-13, D-12): lista, detalhe (com members) e mutações
// com invalidação — criar/editar reflete na lista sem reload (CA-29).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  archiveProject,
  createProject,
  getProject,
  getProjectsSummary,
  getProjectCalendar,
  listProjects,
  patchProject,
  replaceProjectMembers,
  type ProjectInput,
  type ProjectMember,
} from '../api';
import { browserTZ } from '../format';
import { boardKey } from './objectives';

export const projectsKey = (ws: string) => ['projects', ws] as const;
export const projectKey = (ws: string, id: string) => ['projects', ws, 'detail', id] as const;
// fase 28: cards vivos + calendário do projeto.
export const projectsSummaryKey = (ws: string) => ['projects', ws, 'summary'] as const;
export const projectCalendarKey = (ws: string, id: string, month: string) =>
  ['projects', ws, 'calendar', id, month] as const;

export function useProjects(ws: string, opts?: { archived?: boolean }) {
  return useQuery({
    queryKey: [...projectsKey(ws), { archived: opts?.archived ?? false }],
    queryFn: () => listProjects(ws, opts),
  });
}

export function useProject(ws: string, id: string) {
  return useQuery({
    queryKey: projectKey(ws, id),
    queryFn: () => getProject(ws, id),
  });
}

export function useCreateProject(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ProjectInput) => createProject(ws, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectsKey(ws) }),
  });
}

export function usePatchProject(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: Partial<ProjectInput> }) =>
      patchProject(ws, id, patch),
    // fase 28: o card vivo e o board do projeto refletem o PATCH (ex.: deadline)
    // sem reload — invalida a lista, o summary e o board do projeto editado.
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: projectsKey(ws) });
      qc.invalidateQueries({ queryKey: projectsSummaryKey(ws) });
      qc.invalidateQueries({ queryKey: boardKey(ws, id) });
    },
  });
}

// fase 28: cards vivos da lista de Projetos (1 request — CA-04/16).
export function useProjectsSummary(ws: string, opts?: { archived?: boolean }) {
  return useQuery({
    queryKey: [...projectsSummaryKey(ws), { archived: opts?.archived ?? false }],
    queryFn: () => getProjectsSummary(ws, { tz: browserTZ(), archived: opts?.archived }),
  });
}

// fase 28: itens datados do projeto por mês (CA-10).
export function useProjectCalendar(ws: string, id: string, month: string) {
  return useQuery({
    queryKey: projectCalendarKey(ws, id, month),
    queryFn: () => getProjectCalendar(ws, id, { month, tz: browserTZ() }),
    enabled: ws !== '' && id !== '' && month !== '',
  });
}

export function useArchiveProject(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => archiveProject(ws, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectsKey(ws) }),
  });
}

export function useReplaceProjectMembers(ws: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, members }: { id: string; members: ProjectMember[] }) =>
      replaceProjectMembers(ws, id, members),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectsKey(ws) }),
  });
}
