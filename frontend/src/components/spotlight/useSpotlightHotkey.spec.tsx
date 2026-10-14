// useSpotlightHotkey.spec — T-06 (CA-01/01.1): Cmd+K abre; 'k' sozinho não;
// unmount remove o listener.
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, cleanup, act } from '@testing-library/react';
import { useSpotlightHotkey } from './useSpotlightHotkey';
import { useSpotlightStore } from '../../lib/spotlightStore';

function Host() {
  useSpotlightHotkey();
  return null;
}

beforeEach(() => useSpotlightStore.setState({ open: false, query: '' }));
afterEach(cleanup);

describe('useSpotlightHotkey', () => {
  it('Cmd+K abre o Spotlight (CA-01)', () => {
    render(<Host />);
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }));
    });
    expect(useSpotlightStore.getState().open).toBe(true);
  });

  it('Ctrl+K abre o Spotlight (CA-01)', () => {
    render(<Host />);
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }));
    });
    expect(useSpotlightStore.getState().open).toBe(true);
  });

  it("'k' sozinho NÃO abre (CA-01.1)", () => {
    render(<Host />);
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k' }));
    });
    expect(useSpotlightStore.getState().open).toBe(false);
  });

  it('Cmd+J (outra tecla) NÃO abre (CA-01.1)', () => {
    render(<Host />);
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', metaKey: true }));
    });
    expect(useSpotlightStore.getState().open).toBe(false);
  });

  it('unmount remove o listener (CA-01 — sem vazamento)', () => {
    const { unmount } = render(<Host />);
    unmount();
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }));
    });
    expect(useSpotlightStore.getState().open).toBe(false);
  });
});
