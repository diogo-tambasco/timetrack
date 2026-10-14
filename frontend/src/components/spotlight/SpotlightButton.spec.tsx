// SpotlightButton.spec — T-06 (CA-02/03): clique abre o store; aria descreve a
// ação+atalho. O botão é agnóstico de largura; o chrome o posiciona (rail no
// desktop via SideNav, menu "Mais" na MobileNav < md — NÃO overlay fixed, que
// cobria os controles do topo). O caminho mobile é coberto em MobileNav.spec.
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import SpotlightButton from './SpotlightButton';
import { useSpotlightStore } from '../../lib/spotlightStore';

beforeEach(() => useSpotlightStore.setState({ open: false, query: '' }));
afterEach(cleanup);

describe('SpotlightButton', () => {
  it('clique abre o Spotlight (CA-02)', () => {
    render(<SpotlightButton />);
    fireEvent.click(screen.getByTestId('spotlight-button'));
    expect(useSpotlightStore.getState().open).toBe(true);
  });

  it('aria descreve a ação e o atalho (CA-02)', () => {
    render(<SpotlightButton />);
    expect(screen.getByLabelText('Abrir captura (Cmd+K)')).toBeTruthy();
  });
});
