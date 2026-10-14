// RemoteShortcuts (fase 23 T-06, CA-09/10) — os atalhos do controle remoto:
// "Minhas tasks" (visão da fase 18, rota /minhas-tarefas) e "Modo Foco" (rota
// /foco da fase 05). A 23 só LINKA — não reimplementa a lista nem a query
// cross-workspace (a 18 é dona). Sem hook de contagem barato na 18 → só o link,
// sem badge (degrada — CA-09). /foco sempre existe (CA-10).
import { Link } from 'react-router-dom';
import { Icon } from '../icons';

const shortcutStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  minHeight: 52,
  padding: '0 16px',
  borderRadius: 'var(--r-md)',
  border: '1px solid var(--border)',
  background: 'var(--surface-2)',
  color: 'var(--text-1)',
  textDecoration: 'none',
  fontFamily: 'var(--font-ui)',
  fontSize: 15,
  fontWeight: 500,
};

export default function RemoteShortcuts() {
  return (
    <div data-testid="remote-shortcuts" style={{ display: 'grid', gap: 10 }}>
      <Link to="/minhas-tarefas" style={shortcutStyle}>
        <Icon.list size={18} />
        Minhas tasks
      </Link>
      <Link to="/foco" style={shortcutStyle}>
        <Icon.focus size={18} />
        Modo Foco
      </Link>
    </div>
  );
}
