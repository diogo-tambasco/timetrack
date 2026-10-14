// Fase 22 T-01: hook de breakpoint caseiro (sem dependência nova). Resolve um
// media query via window.matchMedia e reage a mudanças (resize/orientação).
// SSR-safe: fora do browser (sem matchMedia) devolve false. Usado pelos
// componentes que precisam de render CONDICIONAL por largura (tabela→cartões —
// D-05); o resto da fase é CSS-only (classes Tailwind md:). Breakpoint canônico
// da fase = (min-width: 768px) (CA-11) — passe a string completa.
import { useEffect, useState } from 'react';

export function useMediaQuery(query: string): boolean {
  const get = () =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(query).matches
      : false;

  const [matches, setMatches] = useState<boolean>(get);

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const mql = window.matchMedia(query);
    const onChange = (e: MediaQueryListEvent) => setMatches(e.matches);
    // sincroniza no mount (o estado inicial pode ter sido SSR/false)
    setMatches(mql.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [query]);

  return matches;
}
