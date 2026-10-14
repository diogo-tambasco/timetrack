// Skin engine do Timetrack (referência: TTskin de app.jsx do protótipo).
// Fase 00 só cobre o tema dark/light (CA-11/12); accent e densidade ficam
// nos tokens com default — a UI de personalização é fase de polish (09).
import { useEffect } from 'react';
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Theme = 'dark' | 'light';

interface SkinState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

export const useSkinStore = create<SkinState>()(
  persist(
    (set) => ({
      theme: 'dark', // CA-11: dark é o padrão na primeira visita
      setTheme: (theme) => set({ theme }),
    }),
    { name: 'tt-skin' },
  ),
);

/** Aplica o tema do store no <html data-theme="..."> (chamado no root). */
export function useApplySkin() {
  const theme = useSkinStore((s) => s.theme);
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);
}
