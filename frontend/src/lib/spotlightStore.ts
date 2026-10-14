// Store do Spotlight (fase 27, D-02/H/27-H4): zustand minúsculo { open, query }
// — alcançável pelo atalho (registrado no shell), pelo botão e pelos testes,
// sem prop-drilling. Vive em src/lib/ junto de timerStore/presenceStore/
// noticeStore (decisão H — NÃO src/stores/). O popup-central de task/nota
// (24/26) tem o SEU estado; o Spotlight tem o dele.
import { create } from 'zustand';

type SpotlightStore = {
  /** o Spotlight está aberto? */
  open: boolean;
  /** o texto do input (comando + título) */
  query: string;
  /** abre (idempotente — CA-04); semente opcional pré-povoa o input */
  openSpotlight: (initial?: string) => void;
  /** fecha e zera o input */
  close: () => void;
  /** atualiza o input (seleção volta a 0 na UI — CA-07) */
  setQuery: (q: string) => void;
};

export const useSpotlightStore = create<SpotlightStore>((set) => ({
  open: false,
  query: '',
  openSpotlight: (initial) => set({ open: true, query: initial ?? '' }),
  close: () => set({ open: false, query: '' }),
  setQuery: (q) => set({ query: q }),
}));
