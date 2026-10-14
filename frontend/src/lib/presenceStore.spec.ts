// presenceStore.spec — fase 21 T-08 (CA-16): o store consolida os membros
// ativos POR workspace. running:true add/atualiza; running:false remove;
// seed substitui a lista; troca de ws não mistura; remover inexistente é no-op.
import { describe, it, expect, beforeEach } from 'vitest';
import { usePresenceStore, activeMembers, pushPresence } from './presenceStore';
import type { Presence } from './realtime';

const p = (over: Partial<Presence>): Presence => ({
  user_id: 'u',
  name: 'Nome',
  workspace_id: 'ws-1',
  running: true,
  ...over,
});

beforeEach(() => {
  usePresenceStore.setState({ byWorkspace: {} });
});

describe('presenceStore', () => {
  it('running:true adiciona o membro ao workspace', () => {
    usePresenceStore.getState().apply(p({ user_id: 'a', name: 'Ana', project: 'Site', since: '2026-06-21T12:00:00Z' }));
    const list = activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1');
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({ user_id: 'a', name: 'Ana', project: 'Site', since: '2026-06-21T12:00:00Z' });
  });

  it('running:true atualiza o membro existente (projeto/since)', () => {
    const s = usePresenceStore.getState();
    s.apply(p({ user_id: 'a', project: 'A' }));
    s.apply(p({ user_id: 'a', project: 'B' }));
    const list = activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1');
    expect(list).toHaveLength(1);
    expect(list[0].project).toBe('B');
  });

  it('running:false remove o membro do workspace', () => {
    const s = usePresenceStore.getState();
    s.apply(p({ user_id: 'a' }));
    s.apply(p({ user_id: 'b' }));
    s.apply(p({ user_id: 'a', running: false }));
    const list = activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1');
    expect(list.map((m) => m.user_id)).toEqual(['b']);
  });

  it('remover um membro inexistente é no-op', () => {
    const before = usePresenceStore.getState().byWorkspace;
    usePresenceStore.getState().apply(p({ user_id: 'fantasma', running: false }));
    // estado não muda (mesma referência -> sem re-render)
    expect(usePresenceStore.getState().byWorkspace).toBe(before);
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1')).toHaveLength(0);
  });

  it('não mistura workspaces distintos', () => {
    const s = usePresenceStore.getState();
    s.apply(p({ user_id: 'a', workspace_id: 'ws-1' }));
    s.apply(p({ user_id: 'b', workspace_id: 'ws-2' }));
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1').map((m) => m.user_id)).toEqual(['a']);
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-2').map((m) => m.user_id)).toEqual(['b']);
  });

  it('seed substitui a lista do workspace pelo snapshot', () => {
    const s = usePresenceStore.getState();
    s.apply(p({ user_id: 'velho' }));
    s.seed('ws-1', [p({ user_id: 'x' }), p({ user_id: 'y' })]);
    const list = activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1');
    expect(list.map((m) => m.user_id).sort()).toEqual(['x', 'y']);
  });

  it('activeMembers de workspace desconhecido / undefined é vazio', () => {
    expect(activeMembers({}, undefined)).toEqual([]);
    expect(activeMembers({}, 'ws-x')).toEqual([]);
  });

  it('pushPresence delega pro apply', () => {
    pushPresence(p({ user_id: 'z' }));
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1').map((m) => m.user_id)).toEqual(['z']);
  });
});
