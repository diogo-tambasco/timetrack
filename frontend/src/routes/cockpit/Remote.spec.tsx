// Remote.spec — fase 23 T-07 (CA-01/03/11.1): a página /remoto compõe os 3
// blocos (RemoteControl/RemotePresence/RemoteShortcuts), tudo escopado ao ws
// ATIVO (useCurrentWorkspace). Mocka os 3 filhos (testados isoladamente) pra
// provar a composição + o ws.id passado adiante; mocka auth pro ws/role.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Remote from './Remote';

const useCurrentWorkspace = vi.fn();
const useRole = vi.fn();
vi.mock('../../lib/auth', () => ({
  useCurrentWorkspace: () => useCurrentWorkspace(),
  useRole: (id?: string) => useRole(id),
}));

// filhos mockados: capturam as props recebidas (ws/canWrite/wsId)
const controlProps: { ws?: { id: string }; canWrite?: boolean }[] = [];
const presenceProps: { wsId?: string }[] = [];
vi.mock('../../components/remote/RemoteControl', () => ({
  default: (p: { ws: { id: string }; canWrite: boolean }) => {
    controlProps.push(p);
    return <div data-testid="mock-control">{p.canWrite ? 'write' : 'readonly'}</div>;
  },
}));
vi.mock('../../components/remote/RemotePresence', () => ({
  default: (p: { wsId?: string }) => {
    presenceProps.push(p);
    return <div data-testid="mock-presence">{p.wsId}</div>;
  },
}));
vi.mock('../../components/remote/RemoteShortcuts', () => ({
  default: () => <div data-testid="mock-shortcuts" />,
}));

function renderRemote() {
  return render(
    <MemoryRouter>
      <Remote />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  controlProps.length = 0;
  presenceProps.length = 0;
  useCurrentWorkspace.mockReturnValue({ id: 'ws-ativo', name: 'WS Ativo' });
  useRole.mockReturnValue('owner');
});

describe('Remote (página /remoto)', () => {
  it('compõe os 3 blocos numa coluna única (data-testid="remote")', () => {
    renderRemote();
    expect(screen.getByTestId('remote')).toBeInTheDocument();
    expect(screen.getByTestId('mock-control')).toBeInTheDocument();
    expect(screen.getByTestId('mock-presence')).toBeInTheDocument();
    expect(screen.getByTestId('mock-shortcuts')).toBeInTheDocument();
  });

  it('passa o ws ATIVO adiante (CA-11.1): control.ws.id e presence.wsId = ws.id ativo', () => {
    renderRemote();
    expect(controlProps[0].ws?.id).toBe('ws-ativo');
    expect(presenceProps[0].wsId).toBe('ws-ativo');
    // role resolvido pelo ws.id ativo
    expect(useRole).toHaveBeenCalledWith('ws-ativo');
  });

  it('owner/member → canWrite true', () => {
    useRole.mockReturnValue('member');
    renderRemote();
    expect(controlProps[0].canWrite).toBe(true);
    expect(screen.getByTestId('mock-control')).toHaveTextContent('write');
  });

  it('viewer → canWrite false', () => {
    useRole.mockReturnValue('viewer');
    renderRemote();
    expect(controlProps[0].canWrite).toBe(false);
    expect(screen.getByTestId('mock-control')).toHaveTextContent('readonly');
  });

  it('sem workspace ativo → estado vazio, sem quebrar', () => {
    useCurrentWorkspace.mockReturnValue(null);
    renderRemote();
    expect(screen.getByTestId('remote')).toBeInTheDocument();
    expect(screen.queryByTestId('mock-control')).toBeNull();
  });
});
