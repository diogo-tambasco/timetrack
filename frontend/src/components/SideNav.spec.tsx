// SideNav.spec — fase 27 (CA-02): o botão de captura (⌘K) vive no rail desktop
// (a correção que tirou o overlay fixed). Mata a mutação que remove o
// SpotlightButton do SideNav: o E2E spotlight.spec usa SÓ o atalho Cmd+K (nunca
// clica o botão) e o SpotlightButton.spec só cobre o componente isolado — sem
// este teste o wiring NO rail ficaria descoberto (27-CW-1).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SideNav from './SideNav';
import { useSpotlightStore } from '../lib/spotlightStore';

// authStore: o AvatarMenu precisa de `me`/`signOut` pra renderizar (sem `me`
// retorna null, mas o rail — e o SpotlightButton — seguem montados).
vi.mock('../lib/auth', () => ({
  useAuthStore: (sel: (s: { me: unknown; signOut: () => Promise<void> }) => unknown) =>
    sel({
      me: { user: { name: 'Diogo Fixture', email: 'd@example.com', avatar_url: null } },
      signOut: vi.fn(async () => {}),
    }),
}));

afterEach(cleanup);

function renderRail() {
  return render(
    <MemoryRouter>
      <SideNav />
    </MemoryRouter>,
  );
}

describe('SideNav (rail desktop)', () => {
  it('renderiza o botão de captura ⌘K no rail (CA-02)', () => {
    renderRail();
    expect(screen.getByTestId('spotlight-button')).toBeInTheDocument();
  });

  it('clicar no botão de captura abre o Spotlight (CA-02)', () => {
    useSpotlightStore.setState({ open: false, query: '' });
    renderRail();
    fireEvent.click(screen.getByTestId('spotlight-button'));
    expect(useSpotlightStore.getState().open).toBe(true);
  });
});
