// viewer-gating-notes.spec — fase 26 T-11 (CA-22): cobre o ramo viewer que o
// E2E não alcança (googlestub 1-user — CA-23). can_write=false esconde "New
// note", abre o editor readOnly e o slash não funciona; can_write=true (control)
// mostra tudo. Padrão das fases 12/19.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import NotesTab from './NotesTab';
import NoteEditor from './NoteEditor';
import type { Board, Note } from '../../lib/api';

const { notesRef, noteRef } = vi.hoisted(() => ({
  notesRef: { current: [] as Note[] },
  noteRef: { current: null as Note | null },
}));

vi.mock('../../lib/queries/notes', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/notes')>();
  return {
    ...mod,
    useNotes: () => ({ data: notesRef.current }),
    useNote: () => ({ data: noteRef.current }),
    useCreateNote: () => ({ mutate: vi.fn() }),
    usePatchNote: () => ({ mutate: vi.fn() }),
    useCreateLink: () => ({ mutate: vi.fn() }),
    useDeleteLink: () => ({ mutate: vi.fn() }),
  };
});

const mkNote = (o: Partial<Note> = {}): Note => ({
  id: 'n1',
  workspace_id: 'w1',
  project_id: 'p1',
  task_id: null,
  title: 'Nota',
  body_md: '',
  links: [],
  created_by: 'u1',
  created_at: '2026-06-04T10:00:00Z',
  updated_at: '2026-06-04T10:00:00Z',
  ...o,
});

const mkBoard = (canWrite: boolean): Board => ({
  project: {
    id: 'p1',
    name: 'App',
    color: '#4f9dde',
    client_name: null,
    weekly_hours_goal: null,
    archived_at: null,
    tracked_seconds: 0,
    can_write: canWrite,
  },
  objectives: [],
  general: { id: null, tasks: [] } as never,
});

beforeEach(() => {
  vi.clearAllMocks();
  notesRef.current = [mkNote()];
  noteRef.current = mkNote();
});

describe('gating viewer das notas (CA-22)', () => {
  it('viewer (can_write=false): sem "New note"', () => {
    render(<NotesTab wsId="w1" board={mkBoard(false)} />);
    expect(screen.queryByTestId('new-note')).not.toBeInTheDocument();
  });

  it('member control (can_write=true): "New note" aparece', () => {
    render(<NotesTab wsId="w1" board={mkBoard(true)} />);
    expect(screen.getByTestId('new-note')).toBeInTheDocument();
  });

  it('editor de viewer abre readOnly e o slash NÃO abre', () => {
    render(<NoteEditor wsId="w1" projectId="p1" note={mkNote()} canWrite={false} />);
    const ta = screen.getByTestId('note-body');
    expect(ta).toHaveAttribute('readonly');
    fireEvent.change(ta, { target: { value: '/task' } });
    expect(screen.queryByTestId('slash-menu')).not.toBeInTheDocument();
    expect(screen.getByTestId('note-title')).toHaveAttribute('readonly');
  });

  it('editor de member (control) é editável e o slash abre', () => {
    render(<NoteEditor wsId="w1" projectId="p1" note={mkNote()} canWrite={true} />);
    const ta = screen.getByTestId('note-body');
    expect(ta).not.toHaveAttribute('readonly');
    fireEvent.change(ta, { target: { value: '/task' } });
    expect(screen.getByTestId('slash-menu')).toBeInTheDocument();
  });
});
