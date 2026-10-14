// NoteEditor.spec — fase 26 T-09 (CA-17/18/19/20/21/22): textarea sempre
// editável (sem botão Editar); PATCH com debounce ≥600ms; slash To-do insere
// markdown sem API; slash Task chama createLink e insere chip; falha de link
// sinaliza e não insere; viewer → readOnly sem slash.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import NoteEditor from './NoteEditor';
import type { Note } from '../../lib/api';

const { patchSpy, createLinkSpy } = vi.hoisted(() => ({
  patchSpy: vi.fn(),
  createLinkSpy: vi.fn(),
}));

vi.mock('../../lib/queries/notes', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/queries/notes')>();
  return {
    ...mod,
    usePatchNote: () => ({ mutate: patchSpy }),
    useCreateLink: () => ({ mutate: createLinkSpy }),
  };
});

const mkNote = (o: Partial<Note> = {}): Note => ({
  id: 'n1',
  workspace_id: 'w1',
  project_id: 'p1',
  task_id: null,
  title: 'Mãe',
  body_md: '',
  links: [],
  created_by: 'u1',
  created_at: '2026-06-04T10:00:00Z',
  updated_at: '2026-06-04T10:00:00Z',
  ...o,
});

function renderEditor(note = mkNote(), canWrite = true) {
  render(<NoteEditor wsId="w1" projectId="p1" note={note} canWrite={canWrite} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers();
});
afterEach(() => vi.useRealTimers());

describe('NoteEditor (CA-17/18)', () => {
  it('textarea é sempre editável (sem botão Editar)', () => {
    renderEditor();
    expect(screen.queryByText(/editar/i)).not.toBeInTheDocument();
    expect(screen.getByTestId('note-body')).not.toHaveAttribute('readonly');
  });

  it('PATCH dispara DEPOIS de 600ms e NÃO antes (debounce)', () => {
    renderEditor();
    const ta = screen.getByTestId('note-body');
    fireEvent.change(ta, { target: { value: 'novo corpo' } });
    act(() => vi.advanceTimersByTime(500));
    expect(patchSpy).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(200));
    expect(patchSpy).toHaveBeenCalledWith({ id: 'n1', input: { body_md: 'novo corpo' } });
  });
});

describe('NoteEditor slash (CA-19/20/21)', () => {
  it('/ + Enter em To-do insere "- [ ] " sem chamar API de link', () => {
    renderEditor();
    const ta = screen.getByTestId('note-body') as HTMLTextAreaElement;
    fireEvent.change(ta, { target: { value: '/todo' } });
    // o menu abre filtrado.
    expect(screen.getByTestId('slash-menu')).toBeInTheDocument();
    fireEvent.keyDown(ta, { key: 'Enter' });
    expect(createLinkSpy).not.toHaveBeenCalled();
    expect((screen.getByTestId('note-body') as HTMLTextAreaElement).value).toContain('- [ ] ');
  });

  it('/ + Task chama createLink({kind:"task"})', () => {
    renderEditor();
    const ta = screen.getByTestId('note-body') as HTMLTextAreaElement;
    fireEvent.change(ta, { target: { value: '/task' } });
    fireEvent.keyDown(ta, { key: 'Enter' });
    expect(createLinkSpy).toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'task' }),
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });

  it('falha de createLink (nota solta + task — CA-10) sinaliza e não insere chip', () => {
    createLinkSpy.mockImplementation((_input, opts) => opts.onError());
    renderEditor(mkNote({ project_id: null }));
    const ta = screen.getByTestId('note-body') as HTMLTextAreaElement;
    fireEvent.change(ta, { target: { value: '/task' } });
    fireEvent.keyDown(ta, { key: 'Enter' });
    expect(screen.getByTestId('note-link-error')).toBeInTheDocument();
  });

  it('Esc fecha o menu', () => {
    renderEditor();
    const ta = screen.getByTestId('note-body') as HTMLTextAreaElement;
    fireEvent.change(ta, { target: { value: '/h1' } });
    expect(screen.getByTestId('slash-menu')).toBeInTheDocument();
    fireEvent.keyDown(ta, { key: 'Escape' });
    expect(screen.queryByTestId('slash-menu')).not.toBeInTheDocument();
  });
});

describe('NoteEditor viewer (CA-22)', () => {
  it('readOnly quando can_write=false e slash NÃO abre', () => {
    renderEditor(mkNote(), false);
    const ta = screen.getByTestId('note-body');
    expect(ta).toHaveAttribute('readonly');
    // digitar / não abre o menu (readOnly: o onChange nem dispara, mas garantimos
    // que o componente não renderiza o menu sem can_write).
    fireEvent.change(ta, { target: { value: '/task' } });
    expect(screen.queryByTestId('slash-menu')).not.toBeInTheDocument();
  });
});

describe('NoteEditor flush blur/unmount (T-12 — fechar o painel persiste)', () => {
  it('flush no UNMOUNT persiste a última edição antes dos 600ms', () => {
    const { unmount } = render(
      <NoteEditor wsId="w1" projectId="p1" note={mkNote()} canWrite onOpenLink={undefined} />,
    );
    const ta = screen.getByTestId('note-body');
    fireEvent.change(ta, { target: { value: 'rascunho não salvo' } });
    // NÃO avança os 600ms do debounce.
    act(() => vi.advanceTimersByTime(100));
    expect(patchSpy).not.toHaveBeenCalled();
    // fecha o painel (desmonta o editor) antes do debounce → o flush persiste.
    act(() => unmount());
    expect(patchSpy).toHaveBeenCalledWith({ id: 'n1', input: { body_md: 'rascunho não salvo' } });
  });

  it('flush no BLUR do corpo persiste antes do debounce', () => {
    renderEditor();
    const ta = screen.getByTestId('note-body');
    fireEvent.change(ta, { target: { value: 'corpo em edição' } });
    act(() => vi.advanceTimersByTime(100));
    expect(patchSpy).not.toHaveBeenCalled();
    fireEvent.blur(ta);
    expect(patchSpy).toHaveBeenCalledWith({ id: 'n1', input: { body_md: 'corpo em edição' } });
  });

  it('flush no BLUR do título persiste antes do debounce', () => {
    renderEditor();
    const ti = screen.getByTestId('note-title');
    fireEvent.change(ti, { target: { value: 'Novo título' } });
    act(() => vi.advanceTimersByTime(100));
    expect(patchSpy).not.toHaveBeenCalled();
    fireEvent.blur(ti);
    expect(patchSpy).toHaveBeenCalledWith({ id: 'n1', input: { title: 'Novo título' } });
  });
});

describe('NoteEditor chips (CA-17)', () => {
  it('renderiza um chip por link com o label', () => {
    renderEditor(
      mkNote({
        links: [
          { id: 'l1', ref_type: 'task', ref_id: 't1', label: 'Publicar', status: 'todo', position: 0 },
          { id: 'l2', ref_type: 'note', ref_id: 'n2', label: 'Sub', position: 1 },
        ],
      }),
    );
    expect(screen.getByText(/Publicar/)).toBeInTheDocument();
    expect(screen.getByText(/Sub/)).toBeInTheDocument();
  });
});
