// Rail de navegação canônico (DESIGN-additions.md): 64px só ícones,
// hover → inner 232px como overlay (CSS .tt-sidenav de tokens.css).
import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { Icon } from './icons';
import { useAuthStore } from '../lib/auth';
// fase 22 T-01: itens de nav extraídos pra navItems.ts (fonte única, compartilhada
// com a MobileNav). O rail segue renderizando idêntico.
import { navItems, settingsItem, isActive, type NavItem as NavItemType } from './nav/navItems';
// fase 27: o botão de captura (⌘K) vive no chrome de nav (NÃO overlay fixed — evita
// cobrir os controles do topo). No rail aqui; na MobileNav, no menu "Mais".
import SpotlightButton from './spotlight/SpotlightButton';

function NavItem({ to, label, I }: NavItemType) {
  const { pathname } = useLocation();
  const active = isActive(pathname, to);
  return (
    <Link to={to} className="tt-nav" aria-current={active ? 'true' : undefined} style={{ textDecoration: 'none' }}>
      <I className="tt-navicon" />
      <span className="tt-navlabel">{label}</span>
    </Link>
  );
}

export default function SideNav() {
  return (
    <aside className="tt-sidenav">
      <div className="tt-sidenav-inner">
        <div style={{ display: 'flex', alignItems: 'center', gap: 11, padding: '6px 8px 16px' }}>
          <span
            style={{
              width: 24,
              height: 24,
              borderRadius: 7,
              background: 'var(--accent)',
              display: 'grid',
              placeItems: 'center',
              flex: '0 0 auto',
            }}
          >
            <Icon.clock size={15} style={{ color: 'var(--accent-ink)' }} sw={2.2} />
          </span>
          <span className="tt-brandname" style={{ fontWeight: 600, fontSize: 15, letterSpacing: '-.02em' }}>
            Timetrack
          </span>
        </div>
        {/* fase 27: captura rápida (⌘K) no topo do rail */}
        <div style={{ padding: '0 8px 12px' }}>
          <SpotlightButton />
        </div>
        {navItems.map((item) => (
          <NavItem key={item.to} {...item} />
        ))}
        <div style={{ flex: 1 }} />
        <NavItem {...settingsItem} />
        <AvatarMenu />
      </div>
    </aside>
  );
}

// Menu do avatar (fase 01 T-14): mostra quem está logado e o "Sair"
// (POST /api/v1/auth/logout → /login). Não usa .tt-nav de propósito —
// o rail canônico mantém 8 itens de navegação.
function AvatarMenu() {
  const me = useAuthStore((s) => s.me);
  const signOut = useAuthStore((s) => s.signOut);
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);

  if (!me) return null;
  const initial = (me.user.name || me.user.email).trim().charAt(0).toUpperCase();

  return (
    <div style={{ position: 'relative', marginTop: 6 }}>
      <button
        type="button"
        aria-label="Conta"
        onClick={() => setOpen((v) => !v)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 11,
          width: '100%',
          padding: '5px 6px',
          background: 'transparent',
          border: 0,
          borderRadius: 10,
          cursor: 'pointer',
          color: 'var(--text-2)',
        }}
      >
        {me.user.avatar_url ? (
          <img className="tt-avatar" src={me.user.avatar_url} alt="" />
        ) : (
          <span className="tt-avatar" style={{ background: 'var(--accent)' }}>
            {initial}
          </span>
        )}
        <span className="tt-navlabel" style={{ fontSize: 13 }}>
          {me.user.name || me.user.email}
        </span>
      </button>

      {open && (
        <div
          className="tt-card"
          style={{ position: 'absolute', bottom: '100%', left: 4, marginBottom: 6, padding: 6, minWidth: 130 }}
        >
          <button
            type="button"
            className="tt-btn tt-btn--ghost"
            style={{ width: '100%', justifyContent: 'flex-start' }}
            onClick={async () => {
              await signOut();
              navigate('/login');
            }}
          >
            Sair
          </button>
        </div>
      )}
    </div>
  );
}
