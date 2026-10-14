// Hooks da thread de comentários (fase 19, plan §6). Espelha lib/queries/
// tasks.ts: lista por (ws, task) e mutações que invalidam a key no onSuccess
// (comentário novo/apagado aparece sem reload manual). NÃO toca os hooks de
// task (RNF-04).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createComment, deleteComment, listComments } from '../api';

export const commentsKey = (ws: string, task: string) =>
  ['comments', ws, task] as const;

export function useComments(ws: string, task: string, enabled = true) {
  return useQuery({
    queryKey: commentsKey(ws, task),
    queryFn: () => listComments(ws, task),
    enabled: enabled && !!ws && !!task,
  });
}

export function useCreateComment(ws: string, task: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => createComment(ws, task, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentsKey(ws, task) }),
  });
}

export function useDeleteComment(ws: string, task: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (commentId: string) => deleteComment(ws, task, commentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentsKey(ws, task) }),
  });
}
