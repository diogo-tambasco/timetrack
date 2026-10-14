// SpotlightButton (fase 27, CA-02/03): botão que abre o Spotlight — idêntico ao
// atalho. Visível desktop+mobile (o shell o renderiza no chrome). aria descreve
// a ação + atalho. tokens via .tt-btn (RNF-05).
import { useSpotlightStore } from '../../lib/spotlightStore';
import { Icon } from '../icons';

export default function SpotlightButton() {
  const openSpotlight = useSpotlightStore((s) => s.openSpotlight);
  return (
    <button
      type="button"
      className="tt-btn"
      aria-label="Abrir captura (Cmd+K)"
      data-testid="spotlight-button"
      onClick={() => openSpotlight()}
      style={{ display: 'inline-flex', alignItems: 'center', gap: 7 }}
    >
      <Icon.bolt size={15} sw={1.9} style={{ color: 'var(--accent)' }} />
      <span className="mono" style={{ fontSize: 11, color: 'var(--text-2)' }}>
        ⌘K
      </span>
    </button>
  );
}
