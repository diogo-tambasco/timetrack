// Fase 22 T-02: navegação mobile = bottom bar FIXA, visível só abaixo de md
// (md:hidden), oculta no desktop. Itens de navItems.ts (fonte única — não
// diverge do rail). Rotas-chave na barra (/, /projetos, /relatorios, /foco) +
// botão "Mais" que abre um menu cobrindo o resto (/clientes, /workspace,
// /calendario, /minhas-tarefas, /ajustes) + "Sair" (CA-04). Ativo pela regra do
// SideNav (isActive); aria-current no ativo (RNF-04). Alvos ≥40px. Toda cor =
// token (CA-14). NENHUM estado de timer/dado tocado (CA-12).
import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../lib/auth';
import { useSpotlightStore } from '../../lib/spotlightStore';
import { Icon } from '../icons';
import { isActive, navItems, settingsItem, type NavItem } from './navItems';

// rotas-chave na barra (decisão D-02). O resto vai pro menu "Mais".
const PRIMARY = ['/', '/projetos', '/relatorios', '/foco'];

function barItem(item: NavItem, pathname: string) {
  const active = isActive(pathname, item.to);
  const { I } = item;
  return (
    <Link
      key={item.to}
      to={item.to}
      className="tt-mobnav-item"
      aria-current={active ? 'true' : undefined}
    >
      <I className="tt-mobnav-icon" />
      <span className="tt-mobnav-label">{item.label}</span>
    </Link>
  );
}

export default function MobileNav() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const signOut = useAuthStore((s) => s.signOut);
  const openSpotlight = useSpotlightStore((s) => s.openSpotlight);
  const [moreOpen, setMoreOpen] = useState(false);

  const primary = PRIMARY.map((to) => navItems.find((i) => i.to === to)).filter(
    (i): i is NavItem => Boolean(i),
  );
  // tudo que não está na barra principal cai no menu "Mais" (+ Ajustes).
  const secondary = [...navItems.filter((i) => !PRIMARY.includes(i.to)), settingsItem];
  const moreActive = secondary.some((i) => isActive(pathname, i.to));

  return (
    <>
      {moreOpen && (
        <>
          {/* backdrop pra fechar ao tocar fora */}
          <button
            type="button"
            aria-label="Fechar menu"
            className="tt-mobnav-backdrop md:hidden"
            onClick={() => setMoreOpen(false)}
          />
          <div className="tt-mobnav-menu tt-card md:hidden">
            {/* fase 27: captura rápida (⌘K) — acesso mobile pelo menu "Mais" (D-06) */}
            <button
              type="button"
              className="tt-mobnav-menuitem"
              aria-label="Abrir captura (Cmd+K)"
              onClick={() => {
                setMoreOpen(false);
                openSpotlight();
              }}
            >
              <Icon.bolt size={18} className="tt-mobnav-icon" />
              <span>Capturar (⌘K)</span>
            </button>
            {secondary.map((item) => {
              const { I } = item;
              const active = isActive(pathname, item.to);
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className="tt-mobnav-menuitem"
                  aria-current={active ? 'true' : undefined}
                  onClick={() => setMoreOpen(false)}
                >
                  <I size={18} className="tt-mobnav-icon" />
                  <span>{item.label}</span>
                </Link>
              );
            })}
            <button
              type="button"
              className="tt-mobnav-menuitem"
              onClick={async () => {
                setMoreOpen(false);
                await signOut();
                navigate('/login');
              }}
            >
              <Icon.arrow size={18} className="tt-mobnav-icon" />
              <span>Sair</span>
            </button>
          </div>
        </>
      )}

      {/* a própria <nav> é a bottom bar fixa (tem tamanho real de layout — o
          testid resolve num elemento visível). md:hidden + .tt-mobnav gate. */}
      <nav className="tt-mobnav md:hidden" data-testid="mobile-nav" aria-label="Navegação">
        {primary.map((item) => barItem(item, pathname))}
        <button
          type="button"
          className="tt-mobnav-item"
          aria-current={moreActive ? 'true' : undefined}
          aria-expanded={moreOpen}
          onClick={() => setMoreOpen((v) => !v)}
        >
          <Icon.dots className="tt-mobnav-icon" />
          <span className="tt-mobnav-label">Mais</span>
        </button>
      </nav>
    </>
  );
}
