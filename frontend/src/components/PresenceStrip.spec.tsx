// PresenceStrip.spec — fase 21 T-09 (CA-17/18): a faixa lista os membros ativos
// do ws ativo (nome + projeto se presente); membro sem project não mostra
// projeto; faixa ausente sem ninguém ativo; o snapshot semeia o store no mount.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import PresenceStrip from './PresenceStrip';
import { usePresenceStore } from '../lib/presenceStore';
import * as api from '../lib/api';

vi.mock('../lib/api', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../lib/api')>();
  return { ...mod, getWorkspacePresence: vi.fn() };
});

function renderStrip(wsId?: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<PresenceStrip wsId={wsId} />, { wrapper });
}

beforeEach(() => {
  vi.clearAllMocks();
  usePresenceStore.setState({ byWorkspace: {} });
  vi.mocked(api.getWorkspacePresence).mockResolvedValue([]);
});

describe('PresenceStrip', () => {
  it('sem ninguém ativo → a faixa não aparece (CA-17)', () => {
    renderStrip('ws-1');
    expect(screen.queryByTestId('presence-strip')).toBeNull();
  });

  it('lista os membros ativos do ws ativo, com projeto quando presente (CA-17)', () => {
    usePresenceStore.setState({
      byWorkspace: {
        'ws-1': {
          a: { user_id: 'a', name: 'Ana', project: 'Site', since: new Date().toISOString() },
          b: { user_id: 'b', name: 'Bia', since: new Date().toISOString() },
        },
      },
    });
    renderStrip('ws-1');

    expect(screen.getByTestId('presence-strip')).toBeInTheDocument();
    expect(screen.getByTestId('presence-a')).toBeInTheDocument();
    expect(screen.getByTestId('presence-b')).toBeInTheDocument();
    expect(screen.getByText('Ana')).toBeInTheDocument();
    expect(screen.getByText('Site')).toBeInTheDocument();
    // Bia sem projeto → mostra "sem projeto", não um nome de projeto
    expect(screen.getByText('sem projeto')).toBeInTheDocument();
  });

  it('não mistura membros de outro workspace', () => {
    usePresenceStore.setState({
      byWorkspace: {
        'ws-1': { a: { user_id: 'a', name: 'Ana' } },
        'ws-2': { z: { user_id: 'z', name: 'Zé' } },
      },
    });
    renderStrip('ws-1');
    expect(screen.getByText('Ana')).toBeInTheDocument();
    expect(screen.queryByText('Zé')).toBeNull();
  });

  it('o snapshot semeia o store no mount (CA-18)', async () => {
    vi.mocked(api.getWorkspacePresence).mockResolvedValue([
      { user_id: 'c', name: 'Cadu', workspace_id: 'ws-1', running: true, project: 'Cliente X', since: new Date().toISOString() },
    ]);
    renderStrip('ws-1');

    // estado inicial vazio → começa sem faixa; após o snapshot resolver, aparece
    await waitFor(() => expect(screen.getByTestId('presence-c')).toBeInTheDocument());
    expect(screen.getByText('Cadu')).toBeInTheDocument();
    expect(screen.getByText('Cliente X')).toBeInTheDocument();
    expect(api.getWorkspacePresence).toHaveBeenCalledWith('ws-1');
  });

  it('sem wsId não busca o snapshot', () => {
    renderStrip(undefined);
    expect(api.getWorkspacePresence).not.toHaveBeenCalled();
    expect(screen.queryByTestId('presence-strip')).toBeNull();
  });
});
