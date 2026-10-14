// Fase 22 T-02: MobileNav (bottom bar < md). Itens levam às rotas certas; ativo
// casa a rota (incl. '/' ≠ prefixo de /projetos); "Sair" chama signOut sem
// chamada de API nova; a bar tem role=navigation/data-testid=mobile-nav.
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MobileNav from './MobileNav';
import { useSpotlightStore } from '../../lib/spotlightStore';

// Mock do authStore: signOut espionável (sem rede); useAuthStore(selector).
const signOut = vi.fn(async () => {});
vi.mock('../../lib/auth', () => ({
  useAuthStore: (sel: (s: { signOut: typeof signOut }) => unknown) => sel({ signOut }),
}));

// Captura a navegação do react-router pra afirmar a rota do "Sair".
const navigate = vi.fn();
vi.mock('react-router-dom', async (orig) => {
  const mod = (await orig()) as typeof import('react-router-dom');
  return { ...mod, useNavigate: () => navigate };
});

afterEach(() => {
  signOut.mockClear();
  navigate.mockClear();
});

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <MobileNav />
    </MemoryRouter>,
  );
}

describe('MobileNav (bottom bar mobile)', () => {
  it('é uma navegação identificável (role + testid)', () => {
    renderAt('/');
    const nav = screen.getByTestId('mobile-nav');
    expect(nav).toBeInTheDocument();
    expect(nav.getAttribute('role') === 'navigation' || nav.tagName.toLowerCase() === 'nav').toBe(true);
  });

  it('expõe as rotas-chave como links navegáveis (/, /projetos, /relatorios, /foco)', () => {
    renderAt('/');
    const nav = screen.getByTestId('mobile-nav');
    for (const to of ['/', '/projetos', '/relatorios', '/foco']) {
      const link = within(nav)
        .getAllByRole('link')
        .find((a) => a.getAttribute('href') === to);
      expect(link, `link pra ${to}`).toBeTruthy();
    }
  });

  it('marca o item ativo pela rota (aria-current)', () => {
    renderAt('/projetos');
    const nav = screen.getByTestId('mobile-nav');
    const projetos = within(nav)
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/projetos');
    expect(projetos!.getAttribute('aria-current')).toBe('true');
  });

  it('a Home "/" NÃO fica ativa quando estamos em /projetos (não é prefixo)', () => {
    renderAt('/projetos');
    const nav = screen.getByTestId('mobile-nav');
    const home = within(nav)
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/');
    expect(home!.getAttribute('aria-current')).toBeNull();
  });

  it('a Home "/" fica ativa em "/"', () => {
    renderAt('/');
    const nav = screen.getByTestId('mobile-nav');
    const home = within(nav)
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/');
    expect(home!.getAttribute('aria-current')).toBe('true');
  });

  it('o menu "Mais" expõe as rotas secundárias + Ajustes (+ /remoto da fase 23)', () => {
    renderAt('/');
    fireEvent.click(screen.getByRole('button', { name: /mais/i }));
    // o menu cobre clientes/workspace/calendario/ajustes/minhas tasks + remoto
    for (const to of ['/clientes', '/workspace', '/calendario', '/ajustes', '/minhas-tarefas', '/remoto']) {
      const link = screen.getAllByRole('link').find((a) => a.getAttribute('href') === to);
      expect(link, `link pra ${to} no menu Mais`).toBeTruthy();
    }
  });

  it('o menu "Mais" tem "Capturar (⌘K)" que abre o Spotlight (CA-02/03, D-06)', () => {
    useSpotlightStore.setState({ open: false, query: '' });
    renderAt('/');
    fireEvent.click(screen.getByRole('button', { name: /mais/i }));
    fireEvent.click(screen.getByRole('button', { name: /abrir captura/i }));
    expect(useSpotlightStore.getState().open).toBe(true);
  });

  it('"Sair" chama signOut e navega pra /login (sem chamada de API nova)', async () => {
    renderAt('/');
    fireEvent.click(screen.getByRole('button', { name: /mais/i }));
    fireEvent.click(screen.getByRole('button', { name: /sair/i }));
    // signOut foi chamado; a navegação pra /login acontece no then
    expect(signOut).toHaveBeenCalledTimes(1);
    await Promise.resolve();
    await Promise.resolve();
    expect(navigate).toHaveBeenCalledWith('/login');
  });
});
