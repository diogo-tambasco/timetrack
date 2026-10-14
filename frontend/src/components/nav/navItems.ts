// Fase 22 T-01: ÚNICA fonte dos itens de navegação (to/label/ícone), extraída
// do SideNav. Consumida pelo SideNav (rail desktop) E pela MobileNav (bottom
// bar < md) — uma só lista pra os dois não divergirem (CA-01/03 espelham o
// SideNav). Refactor sem mudança de comportamento desktop: o SideNav renderiza
// idêntico ao de antes.
import type { ComponentType } from 'react';
import { Icon } from '../icons';

interface IconComponentProps {
  size?: number;
  sw?: number;
  className?: string;
}

export interface NavItem {
  to: string;
  label: string;
  I: ComponentType<IconComponentProps>;
}

// Os 8 itens "principais" do rail, na MESMA ordem de hoje. O active de '/' casa
// só com pathname==='/' (`to + '/'` = '//', nunca prefixo de outra rota).
export const navItems: NavItem[] = [
  // fase 13: a home (/) é o Dashboard que absorveu o /hoje (D-05).
  { to: '/', label: 'Hoje', I: Icon.today },
  { to: '/calendario', label: 'Calendário', I: Icon.cal },
  { to: '/projetos', label: 'Projetos', I: Icon.folder },
  // fase 18: visões cross-workspace "Minhas tasks"/"Pedidas por mim"
  { to: '/minhas-tarefas', label: 'Minhas tasks', I: Icon.list },
  { to: '/clientes', label: 'Clientes', I: Icon.briefcase },
  { to: '/relatorios', label: 'Relatórios', I: Icon.chart },
  { to: '/workspace', label: 'Workspace', I: Icon.users },
  { to: '/foco', label: 'Foco', I: Icon.focus },
  // fase 23: controle remoto dedicado (mobile-first). ADITIVO — aparece no rail
  // desktop E na nav mobile (cai no menu "Mais", fora do PRIMARY da MobileNav).
  { to: '/remoto', label: 'Remoto', I: Icon.bolt },
];

// Ajustes fica separado (o rail o renderiza após o spacer flex; a MobileNav o
// agrupa no menu "Mais").
export const settingsItem: NavItem = { to: '/ajustes', label: 'Ajustes', I: Icon.cog };

// active: mesma regra do SideNav em toda a fase (CA-03). '/' casa só com
// pathname==='/' (nunca como prefixo de /projetos etc).
export function isActive(pathname: string, to: string): boolean {
  return pathname === to || pathname.startsWith(to + '/');
}
