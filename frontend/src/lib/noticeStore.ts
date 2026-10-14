// Store do aviso in-app (fase 16, CA-12). Um zustand mínimo guarda o notice
// mais recente; o NoticeBanner (no AppShell) renderiza enquanto houver um, e o
// ✕ limpa (sem chamada ao servidor). O attachTimerSync empurra os notices que
// chegam pelo WebSocket (type:"notice"). O store é PURO (fora de React) — o
// timerStore alimenta via pushNotice, igual ao padrão do auto-start policy.
import { create } from 'zustand';
import type { Notice } from './realtime';

type NoticeStore = {
  /** o aviso atual a exibir, ou null (banner ausente) */
  notice: Notice | null;
  /** registra um aviso novo (substitui o anterior) */
  show: (n: Notice) => void;
  /** dispensa o banner (✕) — não dispara nada no servidor */
  dismiss: () => void;
};

export const useNoticeStore = create<NoticeStore>((set) => ({
  notice: null,
  show: (n) => set({ notice: n }),
  dismiss: () => set({ notice: null }),
}));

// pushNotice é o ponto de entrada de fora-do-React (attachTimerSync). Só os
// avisos conhecidos viram banner; um kind desconhecido é ignorado (forward-
// compat: o canal notice está pronto pra outros avisos — spec §7).
export function pushNotice(n: Notice): void {
  // fase 18 (CA-17): task_assigned entra como banner, SEM tocar o ramo
  // timer_auto_cut. Fase 20 (CA-27): reminder entra do mesmo jeito. Kinds
  // desconhecidos seguem ignorados (forward-compat).
  if (
    n?.kind === 'timer_auto_cut' ||
    n?.kind === 'task_assigned' ||
    n?.kind === 'reminder'
  ) {
    useNoticeStore.getState().show(n);
  }
}
