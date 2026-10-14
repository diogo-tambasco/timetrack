// useView.test.tsx — fase 24 T-08 (CA-16/16.1): useView genérico + mapeamento do
// legado (cards→board, table→list) na chave tt.view.board.
import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { useView } from './ViewToggle';

const KEY = 'tt.view.board';
const TASK_OPTS = ['board', 'list', 'table'] as const;
const LEGACY = { cards: 'board', table: 'list' } as const;
const PERSIST = { table: 'table-v2' } as const; // Table nativa grava token próprio (CA-16.1/RF-03)

afterEach(() => localStorage.clear());

describe('useView (aba Tasks, 3 valores)', () => {
  it('sem valor salvo → fallback board', () => {
    const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY));
    expect(result.current[0]).toBe('board');
  });

  it("legado 'cards' → 'board' (CA-16.1)", () => {
    localStorage.setItem(KEY, 'cards');
    const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY));
    expect(result.current[0]).toBe('board');
  });

  it("'table' LEGADO (token cru, fase 12) → 'list' mesmo com persist (CA-16.1)", () => {
    // o token cru 'table' NÃO é o token da Table nativa ('table-v2') → o legado vence.
    localStorage.setItem(KEY, 'table');
    const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST));
    expect(result.current[0]).toBe('list');
  });

  // CA-16.1/RF-03: a Table NATIVA persiste sob token próprio ('table-v2'), decodificado
  // ANTES do legado → reverte pra Table no reload (não pra List). board/list round-trip direto.
  it("Table nativa persiste no reload: change('table') grava 'table-v2' e relê 'table'", () => {
    const { result, rerender, unmount } = renderHook(() =>
      useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST),
    );
    act(() => result.current[1]('table'));
    expect(localStorage.getItem(KEY)).toBe('table-v2'); // token distinto, não colide com legado
    expect(result.current[0]).toBe('table');
    rerender();
    // simula reload: novo mount lendo o localStorage persistido.
    unmount();
    const { result: reloaded } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST));
    expect(reloaded.current[0]).toBe('table'); // NÃO regride pra List
  });

  it("token 'table-v2' salvo → 'table' (decode tem precedência sobre o legado)", () => {
    localStorage.setItem(KEY, 'table-v2');
    const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST));
    expect(result.current[0]).toBe('table');
  });

  it('valores novos board/list round-trip (escreve→lê)', () => {
    for (const v of ['board', 'list'] as const) {
      localStorage.setItem(KEY, v);
      const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST));
      expect(result.current[0]).toBe(v);
    }
  });

  it('valor desconhecido → fallback board', () => {
    localStorage.setItem(KEY, 'xyz');
    const { result } = renderHook(() => useView(KEY, TASK_OPTS, 'board', LEGACY, PERSIST));
    expect(result.current[0]).toBe('board');
  });
});

describe('useView (consumidores legados, 2 valores)', () => {
  it("cards/table seguem funcionando (Clients/Projects intactos)", () => {
    localStorage.setItem('tt.view.clients', 'table');
    const { result } = renderHook(() => useView('tt.view.clients', ['cards', 'table'] as const, 'cards'));
    expect(result.current[0]).toBe('table');
  });

  it('sem valor → fallback cards', () => {
    const { result } = renderHook(() => useView('tt.view.clients', ['cards', 'table'] as const, 'cards'));
    expect(result.current[0]).toBe('cards');
  });
});
