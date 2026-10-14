// useWakeLock (fase 05, RF-02 — plan D-04): mantém a tela acordada
// enquanto o componente está montado (CA-14), re-adquire quando a página
// volta a ficar visível e o sistema soltou o lock (CA-15) e degrada em
// silêncio sem suporte/rejeição (CA-16) — 1 tentativa por gatilho, nunca
// retry em loop. O cleanup usa flag de cancelamento com release-on-arrival:
// desmontar com o request pendente libera o sentinel QUANDO ele chegar
// (sem isso, todo mount do StrictMode vazaria um wake lock zumbi).
import { useEffect } from 'react';

type Sentinel = { released: boolean; release: () => Promise<void> };
type WakeLockAPI = { request: (type: 'screen') => Promise<Sentinel> };

export function useWakeLock(): void {
  useEffect(() => {
    const wakeLock = (navigator as Navigator & { wakeLock?: WakeLockAPI }).wakeLock;
    if (!wakeLock) return; // CA-16: sem suporte = no-op

    let cancelled = false;
    let pending = false;
    let sentinel: Sentinel | null = null;

    const acquire = () => {
      if (pending) return; // um request por vez: o segundo sobrescreveria (e vazaria) o primeiro
      pending = true;
      wakeLock.request('screen').then(
        (s) => {
          pending = false;
          if (cancelled) {
            void s.release().catch(() => undefined); // release-on-arrival (D-04)
            return;
          }
          sentinel = s;
        },
        () => {
          pending = false; // CA-16: rejeição é silêncio; próximo gatilho tenta de novo
        },
      );
    };

    const onVisible = () => {
      if (document.visibilityState !== 'visible') return;
      if (sentinel && !sentinel.released) return; // lock segue ativo
      acquire(); // CA-15
    };

    acquire();
    document.addEventListener('visibilitychange', onVisible);

    return () => {
      cancelled = true;
      document.removeEventListener('visibilitychange', onVisible);
      if (sentinel) void sentinel.release().catch(() => undefined);
      sentinel = null;
    };
  }, []);
}
