// RemotePresence.spec — fase 23 T-05 (CA-08/08.1/08.2/14): a presence do time
// no remoto. CONSOME o presenceStore da 21 e SEMEIA o snapshot no mount/troca de
// ws (sem isso /remoto a frio fica sem presence — CA-08.2). Lista membros ≠ eu;
// store vazio/ausente → não renderiza. Mock de getWorkspacePresence (padrão
// PresenceStrip.spec).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import RemotePresence from './RemotePresence';
import { usePresenceStore } from '../../lib/presenceStore';
import { useAuthStore } from '../../lib/auth';
import * as api from '../../lib/api';

vi.mock('../../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/api')>();
  return { ...mod, getWorkspacePresence: vi.fn() };
});

function renderPresence(wsId?: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<RemotePresence wsId={wsId} />, { wrapper });
}

beforeEach(() => {
  vi.clearAllMocks();
  usePresenceStore.setState({ byWorkspace: {} });
  vi.mocked(api.getWorkspacePresence).mockResolvedValue([]);
  // eu = u1 (pra filtrar o próprio user da lista)
  useAuthStore.setState({
    status: 'authenticated',
    me: {
      user: { id: 'u1', email: 'a@b.c', name: 'Ana', avatar_url: null },
      workspaces: [{ id: 'ws-1', name: 'WS', role: 'owner' }],
    },
  });
});

describe('RemotePresence (consome a 21, semeia o snapshot, degrada)', () => {
  // CA-08.2 — o teste RED estrutural: store VAZIO no mount → o componente busca
  // o snapshot e semeia (mutação que removesse o fetch deixaria /remoto a frio
  // sem presence).
  it('store vazio no mount → busca o snapshot e semeia (CA-08.2)', async () => {
    vi.mocked(api.getWorkspacePresence).mockResolvedValue([
      { user_id: 'c', name: 'Cadu', workspace_id: 'ws-1', running: true, project: 'Cliente X' },
    ]);
    renderPresence('ws-1');

    await waitFor(() => expect(screen.getByTestId('remote-presence-c')).toBeInTheDocument());
    expect(api.getWorkspacePresence).toHaveBeenCalledWith('ws-1');
    expect(screen.getByText('Cadu')).toBeInTheDocument();
    expect(screen.getByText(/rodando Cliente X/)).toBeInTheDocument();
  });

  it('1 membro ativo no store → mostra a linha (≠ eu)', () => {
    usePresenceStore.setState({
      byWorkspace: { 'ws-1': { b: { user_id: 'b', name: 'Bia', project: 'Site' } } },
    });
    renderPresence('ws-1');
    expect(screen.getByTestId('remote-presence')).toBeInTheDocument();
    expect(screen.getByText('Bia')).toBeInTheDocument();
    expect(screen.getByText(/rodando Site/)).toBeInTheDocument();
  });

  it('o próprio user (eu = u1) NÃO aparece na lista', () => {
    usePresenceStore.setState({
      byWorkspace: {
        'ws-1': {
          u1: { user_id: 'u1', name: 'Ana', project: 'Mine' },
          b: { user_id: 'b', name: 'Bia', project: 'Site' },
        },
      },
    });
    renderPresence('ws-1');
    expect(screen.getByText('Bia')).toBeInTheDocument();
    expect(screen.queryByTestId('remote-presence-u1')).toBeNull();
  });

  it('membro ativo sem projeto → "rodando sem projeto"', () => {
    usePresenceStore.setState({
      byWorkspace: { 'ws-1': { b: { user_id: 'b', name: 'Bia' } } },
    });
    renderPresence('ws-1');
    expect(screen.getByText(/rodando sem projeto/)).toBeInTheDocument();
  });

  it('lista vazia (ninguém ativo além de mim) → não renderiza (CA-08)', () => {
    usePresenceStore.setState({
      byWorkspace: { 'ws-1': { u1: { user_id: 'u1', name: 'Ana' } } },
    });
    renderPresence('ws-1');
    expect(screen.queryByTestId('remote-presence')).toBeNull();
  });

  it('store ausente para o ws (undefined) → não renderiza, sem throw', () => {
    renderPresence('ws-1');
    expect(screen.queryByTestId('remote-presence')).toBeNull();
  });

  it('sem wsId → não busca snapshot nem renderiza', () => {
    renderPresence(undefined);
    expect(api.getWorkspacePresence).not.toHaveBeenCalled();
    expect(screen.queryByTestId('remote-presence')).toBeNull();
  });

  it('não mistura presence de outro workspace (CA-11.1)', () => {
    usePresenceStore.setState({
      byWorkspace: {
        'ws-1': { b: { user_id: 'b', name: 'Bia' } },
        'ws-2': { z: { user_id: 'z', name: 'Zé' } },
      },
    });
    renderPresence('ws-1');
    expect(screen.getByText('Bia')).toBeInTheDocument();
    expect(screen.queryByText('Zé')).toBeNull();
  });
});
