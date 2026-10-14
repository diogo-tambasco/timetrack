// Fase 22 T-01: navItems é a fonte única (to/label/ícone) consumida por
// SideNav e MobileNav. Pina a lista canônica e a regra de active.
import { describe, expect, it } from 'vitest';
import { isActive, navItems, settingsItem } from './navItems';

describe('navItems (fonte única de navegação)', () => {
  it('expõe as 9 rotas canônicas do rail na ordem de hoje (fase 23: + /remoto)', () => {
    expect(navItems.map((i) => i.to)).toEqual([
      '/',
      '/calendario',
      '/projetos',
      '/minhas-tarefas',
      '/clientes',
      '/relatorios',
      '/workspace',
      '/foco',
      '/remoto',
    ]);
    expect(navItems.map((i) => i.label)).toEqual([
      'Hoje',
      'Calendário',
      'Projetos',
      'Minhas tasks',
      'Clientes',
      'Relatórios',
      'Workspace',
      'Foco',
      'Remoto',
    ]);
    // todo item tem um componente de ícone
    for (const it of navItems) expect(typeof it.I).toBe('function');
  });

  it('Ajustes fica separado dos itens principais', () => {
    expect(settingsItem.to).toBe('/ajustes');
    expect(settingsItem.label).toBe('Ajustes');
  });

  it('isActive: "/" casa só com === "/" (nunca prefixo de outra rota)', () => {
    expect(isActive('/', '/')).toBe(true);
    expect(isActive('/projetos', '/')).toBe(false);
    expect(isActive('/projetos/123', '/')).toBe(false);
  });

  it('isActive: rota normal casa com === e com prefixo (subrota)', () => {
    expect(isActive('/projetos', '/projetos')).toBe(true);
    expect(isActive('/projetos/abc', '/projetos')).toBe(true);
    expect(isActive('/clientes', '/projetos')).toBe(false);
  });
});
