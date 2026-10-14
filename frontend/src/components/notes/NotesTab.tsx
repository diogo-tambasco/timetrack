// NotesTab (fase 26 T-10, CA-16/22): a aba Notes do projeto. Tabela estilo
// Notion (Note · Preview · Updated, ícone .md) + "New note" gated por
// board.project.can_write. Clicar numa linha abre o NotePanel; "New note" cria a
// nota e abre o editor já aberto (RNF-02 < 2s). Vazio = "No notes yet.".
import { useState } from 'react';
import type { Board, NoteLink } from '../../lib/api';
import { useNotes, useCreateNote } from '../../lib/queries/notes';
import { dataBr } from '../board/dateBr';
import NotePanel from './NotePanel';
import { Icon } from '../icons';

export default function NotesTab({ wsId, board }: { wsId: string; board: Board }) {
  const projectId = board.project.id;
  const canWrite = board.project.can_write;
  const { data: notes } = useNotes(wsId, projectId);
  const createNote = useCreateNote(wsId, projectId);
  // o painel aberto: por noteId. Ao clicar num chip de sub-nota, troca o noteId
  // (abre a sub-nota no mesmo painel — CA-17).
  const [openId, setOpenId] = useState<string | null>(null);
  // foca/seleciona o título ao abrir SÓ quando a nota acabou de nascer ("New
  // note"/sub-nota); abrir nota existente (clique na linha) NÃO mexe no foco.
  const [focusTitle, setFocusTitle] = useState(false);

  const openExisting = (id: string) => {
    setFocusTitle(false);
    setOpenId(id);
  };

  const openLink = (link: NoteLink) => {
    if (link.ref_type === 'note') openExisting(link.ref_id);
    // ref_type === 'task' → abrir o painel de task da 24 fica pro caller do tab-
    // shell; aqui só tratamos a sub-nota (a 24 é dona do TaskPanel).
  };

  const newNote = () => {
    createNote.mutate(
      { project_id: projectId, title: '' },
      {
        onSuccess: (n) => {
          setFocusTitle(true);
          setOpenId(n.id);
        },
      },
    );
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        <p className="faint" style={{ margin: 0, fontSize: 12.5 }}>
          Markdown notes — stored as <span className="mono" style={{ color: 'var(--text-1)' }}>.md</span> in Postgres
        </p>
        {canWrite && (
          <button type="button" className="tt-btn tt-btn--accent" data-testid="new-note" onClick={newNote}>
            <Icon.plus size={15} sw={2.2} />
            New note
          </button>
        )}
      </div>

      {!notes || notes.length === 0 ? (
        <p className="faint" data-testid="notes-empty" style={{ fontSize: 13.5, textAlign: 'center', padding: 24 }}>
          No notes yet.
        </p>
      ) : (
        <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto' }}>
          <table className="tt-table" data-testid="notes-table">
            <thead>
              <tr>
                <th style={{ width: 28 }} />
                <th>Note</th>
                <th>Preview</th>
                <th>Updated</th>
              </tr>
            </thead>
            <tbody>
              {notes.map((n) => (
                <tr
                  key={n.id}
                  data-testid={`note-row-${n.id}`}
                  onClick={() => openExisting(n.id)}
                  style={{ cursor: 'pointer' }}
                >
                  <td style={{ width: 28, color: 'var(--text-3)' }}>
                    <Icon.doc size={15} />
                  </td>
                  <td style={{ fontWeight: 540, whiteSpace: 'nowrap' }}>
                    {n.title || 'Untitled'}{' '}
                    <span className="tt-chip" style={{ fontSize: 9.5, marginLeft: 4 }}>
                      .md
                    </span>
                  </td>
                  <td className="faint" style={{ maxWidth: 380, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {n.preview || ''}
                  </td>
                  <td className="faint" style={{ whiteSpace: 'nowrap' }}>
                    {dataBr(n.updated_at.slice(0, 10))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {openId && (
        <NotePanel
          wsId={wsId}
          projectId={projectId}
          noteId={openId}
          canWrite={canWrite}
          focusTitle={focusTitle}
          onClose={() => setOpenId(null)}
          onOpenLink={openLink}
        />
      )}
    </div>
  );
}
