// NotesTab.spec — fase 26 T-10 (CA-16/17/22): lista as notas (preview+updated),
// esconde "New note" sem can_write, clicar numa linha abre o NotePanel, "New
// note" cria e abre o editor, painel usa o CenterPanel da 24.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import NotesTab from './NotesTab';
import type { Board, Note } from '../../lib/api';

const { notesRef, createSpy, noteRef } = vi.hoisted(() => ({
  notesRef: { current: [] as Note[] },
  createSpy: vi.fn(),
  noteRef: { current: null as Note | null },
}));

vi.mock('../../lib/queries/notes', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/notes')>();
  return {
    ...mod,
    useNotes: () => ({ data: notesRef.current }),
    useNote: () => ({ data: noteRef.current }),
    useCreateNote: () => ({ mutate: createSpy }),
    useDeleteNote: () => ({ mutate: vi.fn() }),
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
  title: 'Decisões',
  body_md: 'corpo',
  preview: 'preview da nota',
  links: [],
  created_by: 'u1',
  created_at: '2026-06-04T10:00:00Z',
  updated_at: '2026-06-20T10:00:00Z',
  ...o,
});

const mkBoard = (canWrite = true): Board => ({
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
  notesRef.current = [];
  noteRef.current = null;
});

describe('NotesTab (CA-16)', () => {
  it('vazio → "No notes yet."', () => {
    render(<NotesTab wsId="w1" board={mkBoard()} />);
    expect(screen.getByTestId('notes-empty')).toBeInTheDocument();
  });

  it('lista notas com preview e updated', () => {
    notesRef.current = [mkNote()];
    render(<NotesTab wsId="w1" board={mkBoard()} />);
    expect(screen.getByText('Decisões')).toBeInTheDocument();
    expect(screen.getByText('preview da nota')).toBeInTheDocument();
    expect(screen.getByText('20/06/2026')).toBeInTheDocument();
  });

  it('esconde "New note" quando can_write=false (CA-22)', () => {
    render(<NotesTab wsId="w1" board={mkBoard(false)} />);
    expect(screen.queryByTestId('new-note')).not.toBeInTheDocument();
  });

  it('"New note" cria a nota e abre o painel (CA-16/17)', () => {
    const created = mkNote({ id: 'n9', title: '' });
    createSpy.mockImplementation((_input, opts) => {
      noteRef.current = created;
      opts.onSuccess(created);
    });
    render(<NotesTab wsId="w1" board={mkBoard()} />);
    fireEvent.click(screen.getByTestId('new-note'));
    expect(createSpy).toHaveBeenCalledWith(
      { project_id: 'p1', title: '' },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
    // painel abre via CenterPanel da 24 (coord. A).
    expect(screen.getByTestId('center-panel')).toBeInTheDocument();
  });

  it('clicar numa linha abre o NotePanel (CenterPanel da 24)', () => {
    const note = mkNote();
    notesRef.current = [note];
    noteRef.current = note;
    render(<NotesTab wsId="w1" board={mkBoard()} />);
    fireEvent.click(screen.getByTestId('note-row-n1'));
    expect(screen.getByTestId('center-panel')).toBeInTheDocument();
    expect(screen.getByTestId('center-card')).toBeInTheDocument();
  });
});
