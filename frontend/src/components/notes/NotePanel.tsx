// NotePanel (fase 26 T-10, CA-17, coord. A): embrulha o NoteEditor no
// componente CenterPanel da fase 24 (popup central side↔full via tokens). Busca
// a nota COMPLETA (body_md + links) por useNote; abre o chip da entidade via
// onOpenLink (delegado ao caller — task → painel de task da 24; note → este
// mesmo painel na sub-nota). Sem reload (a query da nota invalida pós-PATCH/link).
//
// fase 31 (vibecode): mesmo acabamento do popup de task — página (fillHeight,
// largura confortável de leitura) + menu "…" de excluir no header. Sem sidebar:
// a nota é uma página de pensamento, o corpo é o herói (decisão HITL "página pura").
import { useState } from 'react';
import type { NoteLink } from '../../lib/api';
import { useNote, useDeleteNote } from '../../lib/queries/notes';
import CenterPanel from '../board/CenterPanel';
import { Icon } from '../icons';
import NoteEditor from './NoteEditor';

export default function NotePanel({
  wsId,
  projectId,
  noteId,
  canWrite,
  focusTitle,
  onClose,
  onOpenLink,
}: {
  wsId: string;
  projectId: string;
  noteId: string;
  canWrite: boolean;
  focusTitle?: boolean;
  onClose: () => void;
  onOpenLink?: (link: NoteLink) => void;
}) {
  const { data: note } = useNote(wsId, noteId);
  return (
    <CenterPanel
      ariaLabel={note?.title || 'Note'}
      onClose={onClose}
      width={960}
      fillHeight
      headerExtra={
        note && canWrite ? (
          <NoteMenu wsId={wsId} projectId={projectId} noteId={noteId} onDeleted={onClose} />
        ) : null
      }
    >
      {note ? (
        <NoteEditor
          key={note.id}
          wsId={wsId}
          projectId={projectId}
          note={note}
          canWrite={canWrite}
          focusTitle={focusTitle}
          onOpenLink={onOpenLink}
        />
      ) : (
        <div style={{ padding: 24 }} />
      )}
    </CenterPanel>
  );
}

// NoteMenu (fase 31): menu "…" no header do popup com "Excluir nota" — espelha o
// TaskMenu. Confirma antes (ação destrutiva, sem undo). Ao apagar: invalida a
// lista (hook) e fecha o popup → some da aba Notes sem reload.
function NoteMenu({
  wsId,
  projectId,
  noteId,
  onDeleted,
}: {
  wsId: string;
  projectId: string;
  noteId: string;
  onDeleted: () => void;
}) {
  const [open, setOpen] = useState(false);
  const deleteNote = useDeleteNote(wsId, projectId);

  const remove = () => {
    setOpen(false);
    if (typeof window !== 'undefined' && !window.confirm('Excluir esta nota? Não dá pra desfazer.')) return;
    deleteNote.mutate(noteId, { onSuccess: onDeleted });
  };

  return (
    <div style={{ position: 'relative' }}>
      <button
        type="button"
        aria-label="Mais ações"
        data-testid="note-menu"
        className="tt-iconbtn"
        onClick={() => setOpen((o) => !o)}
        style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
      >
        <Icon.dots size={16} />
      </button>
      {open && (
        <>
          <div aria-hidden onClick={() => setOpen(false)} style={{ position: 'fixed', inset: 0, zIndex: 70 }} />
          <div
            role="menu"
            style={{
              position: 'absolute',
              top: '100%',
              left: 0,
              marginTop: 4,
              zIndex: 71,
              minWidth: 160,
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 9,
              boxShadow: 'var(--shadow)',
              padding: 5,
            }}
          >
            <button
              type="button"
              role="menuitem"
              data-testid="note-delete"
              onClick={remove}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 9,
                width: '100%',
                padding: '8px 10px',
                borderRadius: 6,
                border: 'none',
                background: 'transparent',
                color: 'var(--danger)',
                font: 'inherit',
                fontSize: 13,
                cursor: 'pointer',
                textAlign: 'left',
              }}
            >
              <Icon.trash size={15} />
              Excluir nota
            </button>
          </div>
        </>
      )}
    </div>
  );
}
