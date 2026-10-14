// NowLine (fase 11, CA-16): linha do "agora" na coluna de hoje. Posição =
// minuto-de-parede local / 1440. Atualiza a cada 30s (≤1min de defasagem). A
// TimeGrid só monta esta linha na coluna de HOJE — então ela "não aparece" em
// semanas/dias sem hoje por construção.
import { useEffect, useState } from 'react';

export default function NowLine() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 30000);
    return () => clearInterval(id);
  }, []);

  const pct = (now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60) / 1440;

  return (
    <div
      data-testid="now-line"
      aria-hidden
      style={{
        position: 'absolute',
        top: `${pct * 100}%`,
        left: 0,
        right: 0,
        height: 0,
        borderTop: '2px solid var(--danger)',
        zIndex: 3,
        pointerEvents: 'none',
      }}
    >
      <span
        style={{
          position: 'absolute',
          left: -4,
          top: -4,
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: 'var(--danger)',
        }}
      />
    </div>
  );
}
