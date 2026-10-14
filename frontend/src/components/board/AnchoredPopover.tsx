// AnchoredPopover (fase 31): popover ancorado a um elemento, renderizado via
// PORTAL no body — escapa do overflow:hidden do CenterPanel (que cliparia um
// popover inline). Mede o anchor 1× ao abrir e posiciona logo abaixo; backdrop
// fixo fecha no clique-fora. Usado pelo calendário de Due e pelo dropdown de
// Status. align='right' alinha a borda direita do popover ao anchor.
import { useLayoutEffect, useState, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';

export default function AnchoredPopover({
  anchorRef,
  onClose,
  children,
  align = 'left',
}: {
  anchorRef: RefObject<HTMLElement | null>;
  onClose: () => void;
  children: ReactNode;
  align?: 'left' | 'right';
}) {
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);

  useLayoutEffect(() => {
    const el = anchorRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setPos({ top: r.bottom + 6, left: align === 'right' ? r.right : r.left });
  }, [anchorRef, align]);

  if (pos == null || typeof document === 'undefined') return null;

  return createPortal(
    <>
      <div
        aria-hidden
        onClick={onClose}
        style={{ position: 'fixed', inset: 0, zIndex: 80 }}
      />
      <div
        style={{
          position: 'fixed',
          top: pos.top,
          left: pos.left,
          zIndex: 81,
          ...(align === 'right' ? { transform: 'translateX(-100%)' } : null),
        }}
      >
        {children}
      </div>
    </>,
    document.body,
  );
}
