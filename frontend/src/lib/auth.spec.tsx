// auth.spec — T-01 da fase 10: workspace ativo como estado (CA-06/07 +
// RNF-02 compat). O caso do BOOT usa vi.resetModules + import dinâmico —
// o store é singleton de módulo e só um módulo FRESCO exercita a leitura
// do localStorage na criação (sem isso o caso seria tautológico).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useAuthStore, useCurrentWorkspace } from './auth';
import type { Me } from './api';

const me: Me = {
  user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
  workspaces: [
    { id: 'w1', name: 'Workspace de Ana', role: 'owner' },
    { id: 'w2', name: 'Time Primor', role: 'member' },
  ],
};

beforeEach(() => {
  localStorage.clear();
  useAuthStore.setState({ status: 'authenticated', me, activeWorkspaceId: null });
});

describe('workspace ativo (fase 10)', () => {
  // (a) RNF-02 — sem id salvo o comportamento é o de SEMPRE: workspaces[0]
  it('sem id salvo devolve workspaces[0]', () => {
    const { result } = renderHook(() => useCurrentWorkspace());
    expect(result.current?.id).toBe('w1');
  });

  // (b) CA-06 — setActiveWorkspace muda o devolvido e persiste
  it('setActiveWorkspace troca o ativo e grava no localStorage', () => {
    const { result } = renderHook(() => useCurrentWorkspace());

    act(() => {
      useAuthStore.getState().setActiveWorkspace('w2');
    });

    expect(result.current?.id).toBe('w2');
    expect(localStorage.getItem('tt.ws.active')).toBe('w2');
  });

  // (c) CA-07 — boot com id salvo: módulo fresco lê o localStorage na
  // criação do store
  it('boot com id salvo resolve o salvo', async () => {
    localStorage.setItem('tt.ws.active', 'w2');
    vi.resetModules();
    const fresh = await import('./auth');

    expect(fresh.useAuthStore.getState().activeWorkspaceId).toBe('w2');
  });

  // (d) CA-07 — id salvo que não existe mais no /me → workspaces[0], sem erro
  it('id morto cai em workspaces[0] sem erro', () => {
    useAuthStore.setState({ activeWorkspaceId: 'ws-removido' });

    const { result } = renderHook(() => useCurrentWorkspace());
    expect(result.current?.id).toBe('w1');
  });
});
