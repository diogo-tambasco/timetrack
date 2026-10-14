// Store da presence ao vivo (fase 21, CA-16). Um zustand PURO (fora de React,
// como o noticeStore) consolida os membros ATIVOS por workspace: um payload
// running:true adiciona/atualiza o membro; running:false REMOVE o membro da
// faixa daquele workspace. A UI lê só o workspace ativo. O attachTimerSync
// alimenta via apply() (o que chega pelo WebSocket type:"presence"); o snapshot
// inicial semeia via seed() (o GET /workspaces/{id}/presence). Idempotente:
// remover um membro inexistente é no-op.
import { create } from 'zustand';
import type { Presence } from './realtime';

// ActiveMember é o que a faixa mostra de um membro ativo (sem o running, que é
// sempre true na lista consolidada; e sem o workspace_id, que é a chave do mapa).
export type ActiveMember = {
  user_id: string;
  name: string;
  project?: string;
  since?: string;
};

type PresenceStore = {
  /** membros ativos por workspace_id → (user_id → ActiveMember) */
  byWorkspace: Record<string, Record<string, ActiveMember>>;
  /** aplica um delta do WebSocket: running add/atualiza, !running remove */
  apply: (p: Presence) => void;
  /** semeia o snapshot de um workspace (substitui a lista daquele ws) */
  seed: (workspaceId: string, members: Presence[]) => void;
};

export const usePresenceStore = create<PresenceStore>((set) => ({
  byWorkspace: {},

  apply: (p) =>
    set((s) => {
      const ws = p.workspace_id;
      const current = s.byWorkspace[ws] ?? {};
      // running:false → remove (idempotente: ausente = no-op)
      if (!p.running) {
        if (!(p.user_id in current)) return s; // nada muda
        const next = { ...current };
        delete next[p.user_id];
        return { byWorkspace: { ...s.byWorkspace, [ws]: next } };
      }
      // running:true → add/atualiza
      const member: ActiveMember = {
        user_id: p.user_id,
        name: p.name,
        project: p.project,
        since: p.since,
      };
      return {
        byWorkspace: { ...s.byWorkspace, [ws]: { ...current, [p.user_id]: member } },
      };
    }),

  seed: (workspaceId, members) =>
    set((s) => {
      const map: Record<string, ActiveMember> = {};
      for (const m of members) {
        if (!m.running) continue; // o snapshot já vem só com running, defensivo
        map[m.user_id] = {
          user_id: m.user_id,
          name: m.name,
          project: m.project,
          since: m.since,
        };
      }
      return { byWorkspace: { ...s.byWorkspace, [workspaceId]: map } };
    }),
}));

// pushPresence é o ponto de entrada de fora-do-React (attachTimerSync), igual ao
// pushNotice do noticeStore.
export function pushPresence(p: Presence): void {
  usePresenceStore.getState().apply(p);
}

// activeMembers seleciona os membros ativos de um workspace (lista estável pra
// UI; vazio se ninguém ativo / workspace desconhecido).
export function activeMembers(
  byWorkspace: PresenceStore['byWorkspace'],
  workspaceId: string | undefined,
): ActiveMember[] {
  if (!workspaceId) return [];
  return Object.values(byWorkspace[workspaceId] ?? {});
}
