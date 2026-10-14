// openNoteStore (fase 31): abridor GLOBAL do popup de nota. Espelha o
// openTaskStore — o Spotlight (/note) vive no AppShell pai e não tem acesso ao
// estado local de NotesTab, então o fluxo "criar note → abrir o popup completo"
// (igual ao /task) passa por aqui. A página de projeto (ProjectDetail) é a dona
// do NotePanel montado no shell e escuta este store.
//
// focusTitle = abrir já focado/selecionado no título (nota recém-criada via /note
// ou "New note"), pro Diogo nomear direto — captura < 2s, à-prova-de-fricção.
import { create } from 'zustand';

interface OpenNoteState {
  noteId: string | null;
  focusTitle: boolean;
  open: (noteId: string, opts?: { focusTitle?: boolean }) => void;
  close: () => void;
}

export const useOpenNoteStore = create<OpenNoteState>((set) => ({
  noteId: null,
  focusTitle: false,
  open: (noteId, opts = {}) => set({ noteId, focusTitle: !!opts.focusTitle }),
  close: () => set({ noteId: null, focusTitle: false }),
}));
