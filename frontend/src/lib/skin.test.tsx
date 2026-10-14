import { describe, it, expect, beforeEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useSkinStore, useApplySkin } from './skin';

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  act(() => {
    useSkinStore.setState({ theme: 'dark' });
  });
});

describe('skin store', () => {
  // CA-11: dark é o default na primeira visita (sem localStorage).
  it('skin_padraoDark', () => {
    renderHook(() => useApplySkin());

    expect(useSkinStore.getState().theme).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  // CA-12: light persiste em localStorage e sobrevive a novo mount.
  it('skin_lightPersiste', async () => {
    renderHook(() => useApplySkin());

    act(() => {
      useSkinStore.getState().setTheme('light');
    });

    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    const raw = localStorage.getItem('tt-skin');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw!).state.theme).toBe('light');

    // novo mount: zera o estado em memória e rehidrata do localStorage
    // (setState do persist regrava o storage — preserva o snapshot antes)
    const snapshot = localStorage.getItem('tt-skin')!;
    act(() => {
      useSkinStore.setState({ theme: 'dark' });
    });
    localStorage.setItem('tt-skin', snapshot);
    await act(async () => {
      await useSkinStore.persist.rehydrate();
    });
    renderHook(() => useApplySkin());

    expect(useSkinStore.getState().theme).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});
