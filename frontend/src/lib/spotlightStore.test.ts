// spotlightStore.test.ts — T-02 (CA-01/04): o store { open, query }.
import { describe, it, expect, beforeEach } from 'vitest';
import { useSpotlightStore } from './spotlightStore';

beforeEach(() => {
  useSpotlightStore.setState({ open: false, query: '' });
});

describe('useSpotlightStore', () => {
  it('openSpotlight() abre com query vazia', () => {
    useSpotlightStore.getState().openSpotlight();
    expect(useSpotlightStore.getState().open).toBe(true);
    expect(useSpotlightStore.getState().query).toBe('');
  });

  it('openSpotlight(initial) semeia a query', () => {
    useSpotlightStore.getState().openSpotlight('foco no projeto');
    expect(useSpotlightStore.getState().query).toBe('foco no projeto');
  });

  it('close() fecha e zera a query', () => {
    useSpotlightStore.getState().openSpotlight('algo');
    useSpotlightStore.getState().close();
    expect(useSpotlightStore.getState().open).toBe(false);
    expect(useSpotlightStore.getState().query).toBe('');
  });

  it('setQuery atualiza só a query', () => {
    useSpotlightStore.getState().openSpotlight();
    useSpotlightStore.getState().setQuery('/task X');
    expect(useSpotlightStore.getState().query).toBe('/task X');
    expect(useSpotlightStore.getState().open).toBe(true);
  });

  it('openSpotlight com o store já aberto mantém aberto (CA-04 idempotente)', () => {
    useSpotlightStore.getState().openSpotlight('um');
    useSpotlightStore.getState().openSpotlight();
    expect(useSpotlightStore.getState().open).toBe(true);
  });
});
