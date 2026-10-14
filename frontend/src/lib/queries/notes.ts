// Hooks de notas nativas (fase 26, plan §6). Espelha lib/queries/comments.ts:
// lista por (ws, projectId) e mutações que invalidam notesKey no onSuccess (a
// nota nova/editada/apagada aparece sem reload). A mutação de link /task
// invalida TAMBÉM o boardKey (a task aparece na aba Tasks da 24 — CA-23) via
// onSuccess local; NÃO toca lib/queries/tasks.ts (RNF-05, padrão da fase 12).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createLink,
  createNote,
  deleteLink,
  deleteNote,
  getNote,
  listNotes,
  patchNote,
  type CreateNoteInput,
  type PatchNoteInput,
} from '../api';
import { boardKey } from './objectives';

// projectId pode ser '' (notas soltas) — entra na key pra separar os caches.
export const notesKey = (ws: string, projectId: string) =>
  ['notes', ws, projectId] as const;

export const noteKey = (ws: string, id: string) => ['note', ws, id] as const;

export function useNotes(ws: string, projectId: string, enabled = true) {
  return useQuery({
    queryKey: notesKey(ws, projectId),
    queryFn: () => listNotes(ws, projectId || undefined),
    enabled: enabled && !!ws,
  });
}

export function useNote(ws: string, id: string, enabled = true) {
  return useQuery({
    queryKey: noteKey(ws, id),
    queryFn: () => getNote(ws, id),
    enabled: enabled && !!ws && !!id,
  });
}

export function useCreateNote(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateNoteInput) => createNote(ws, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: notesKey(ws, projectId) }),
  });
}

export function usePatchNote(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; input: PatchNoteInput }) =>
      patchNote(ws, v.id, v.input),
    onSuccess: (note) => {
      qc.invalidateQueries({ queryKey: notesKey(ws, projectId) });
      qc.invalidateQueries({ queryKey: noteKey(ws, note.id) });
    },
  });
}

export function useDeleteNote(ws: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteNote(ws, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: notesKey(ws, projectId) }),
  });
}

// useCreateLink: invalida a nota-mãe E, se /task, o boardKey (a task aparece em
// Tasks — CA-23). projectId é o do projeto da nota (pro boardKey).
export function useCreateLink(ws: string, projectId: string, noteId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { kind: 'task' | 'note'; name?: string; title?: string }) =>
      createLink(ws, noteId, input),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: noteKey(ws, noteId) });
      qc.invalidateQueries({ queryKey: notesKey(ws, projectId) });
      if (res.task && projectId) {
        qc.invalidateQueries({ queryKey: boardKey(ws, projectId) });
      }
    },
  });
}

export function useDeleteLink(ws: string, projectId: string, noteId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (linkId: string) => deleteLink(ws, noteId, linkId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: noteKey(ws, noteId) });
      qc.invalidateQueries({ queryKey: notesKey(ws, projectId) });
    },
  });
}
