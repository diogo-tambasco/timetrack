// openTaskStore (fase 31): abridor GLOBAL do popup de task. O Spotlight (/task)
// vive no AppShell pai e não tem acesso ao estado local de TasksTab — então o
// fluxo "criar task → abrir o popup completo focado no título" (igual ao mockup
// projeto-workspace.html::openTask) passa por aqui. A página de projeto
// (ProjectDetail) é a dona do popup montado no shell e escuta este store.
//
// focusTitle = abrir já focado/selecionado no título (quando a task nasce sem
// nome via /task), pro Diogo digitar o nome direto — captura < 2s, à-prova-de-
// fricção.
import { create } from 'zustand';

// intent de abertura: title = foca/seleciona o título (criar /task SEM nome);
// schedule = abre o calendário de Due (criar /task COM nome → "quando fazer?").
export interface OpenTaskOpts {
  focusTitle?: boolean;
  scheduleOnOpen?: boolean;
}

interface OpenTaskState {
  taskId: string | null;
  focusTitle: boolean;
  scheduleOnOpen: boolean;
  open: (taskId: string, opts?: OpenTaskOpts) => void;
  close: () => void;
}

export const useOpenTaskStore = create<OpenTaskState>((set) => ({
  taskId: null,
  focusTitle: false,
  scheduleOnOpen: false,
  open: (taskId, opts = {}) =>
    set({ taskId, focusTitle: !!opts.focusTitle, scheduleOnOpen: !!opts.scheduleOnOpen }),
  close: () => set({ taskId: null, focusTitle: false, scheduleOnOpen: false }),
}));
