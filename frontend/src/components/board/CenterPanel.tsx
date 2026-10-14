// CenterPanel (fase 24 T-12, decisão de coordenação A): popup central REUSÁVEL —
// scrim + card central + toggle expand/collapse (side↔full). É o componente que as
// fases 26 (nota) e 27 (Spotlight) REUSAM (NÃO depender das classes cruas
// .mscrim/.scard/.swrap do mockup — estilo interno daqui). Tudo via tokens.css.
//
// modo 'side' = scrim + card médio centralizado (clique no scrim fecha); modo
// 'full' = ocupa a viewport (sem fechar pelo scrim). O toggle alterna sem
// desmontar (preserva o estado de edição do filho — CA-22). < md ocupa a tela
// (RNF-06).
import { useState, type ReactNode } from 'react';
import { Icon } from '../icons';

export default function CenterPanel({
  ariaLabel,
  onClose,
  children,
  width = 760,
  headerExtra,
  headerActions,
  fillHeight = false,
}: {
  ariaLabel: string;
  onClose: () => void;
  children: ReactNode;
  // largura do card no modo 'side' (Spotlight=760 default; TaskPanel maior — fase 31)
  width?: number;
  // ações à ESQUERDA do header (ex.: menu "…" de excluir task — fase 31)
  headerExtra?: ReactNode;
  // ações à DIREITA do header, antes do expand/fechar (ex.: play discreto — fase 31)
  headerActions?: ReactNode;
  // card ocupa quase toda a altura (feel de PÁGINA Notion, não popup curto — fase 31)
  fillHeight?: boolean;
}) {
  const [full, setFull] = useState(false);

  return (
    <div
      data-testid="center-panel"
      style={{ position: 'fixed', inset: 0, zIndex: 60, display: 'grid', placeItems: 'center' }}
    >
      <div
        aria-hidden
        data-testid="center-scrim"
        onClick={() => {
          if (!full) onClose(); // scrim só fecha no modo central (CA-22)
        }}
        style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(2px)' }}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
        data-testid="center-card"
        data-mode={full ? 'full' : 'side'}
        className="tt-card"
        style={{
          position: 'relative',
          background: 'var(--surface-1)',
          boxShadow: 'var(--shadow)',
          display: 'flex',
          flexDirection: 'column',
          ...(full
            ? { width: '100vw', height: '100vh', maxWidth: '100vw', maxHeight: '100vh', borderRadius: 0 }
            : {
                width,
                maxWidth: 'calc(100vw - 32px)',
                ...(fillHeight
                  ? { height: 'calc(100vh - 24px)' }
                  : { maxHeight: 'calc(100vh - 24px)' }),
              }),
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 4,
            padding: '10px 12px',
            borderBottom: '1px solid var(--border)',
            flex: '0 0 auto',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>{headerExtra}</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          {headerActions}
          <button
            type="button"
            aria-label={full ? 'Reduzir' : 'Expandir'}
            data-testid="center-toggle"
            className="tt-iconbtn"
            onClick={() => setFull((f) => !f)}
            style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
          >
            {full ? <Icon.collapse size={16} /> : <Icon.expand size={16} />}
          </button>
          <button
            type="button"
            aria-label="Fechar"
            data-testid="center-close"
            className="tt-iconbtn"
            onClick={onClose}
            style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.x size={16} />
          </button>
          </div>
        </div>
        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>{children}</div>
      </div>
    </div>
  );
}
