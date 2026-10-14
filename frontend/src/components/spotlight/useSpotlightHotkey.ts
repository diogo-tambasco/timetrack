// useSpotlightHotkey (fase 27, CA-01/01.1): registra Cmd+K (macOS) / Ctrl+K
// (Win/Linux) UMA vez no shell. preventDefault (rouba o atalho nativo do
// navegador — D-04); responde APENAS a (meta||ctrl)&&'k' (CA-01.1: 'k' sozinho
// num input NÃO abre). Cleanup no unmount — sem listener vazado.
import { useEffect } from 'react';
import { useSpotlightStore } from '../../lib/spotlightStore';

export function useSpotlightHotkey(): void {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        useSpotlightStore.getState().openSpotlight();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);
}
