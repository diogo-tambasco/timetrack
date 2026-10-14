// RemotePresence (fase 23 T-05, CA-08/08.1/08.2/14) — a presence do time no
// controle remoto: lista "fulano: rodando <projeto>" por membro ATIVO (≠ eu),
// vinda do presenceStore da fase 21. CONSOME a 21, não cria canal: o snapshot
// inicial é semeado AQUI (useQuery(['presence', wsId]) + seed) no mount E ao
// trocar o ws ativo (CA-08.2 — /remoto pode ser ponto de entrada a frio), igual
// ao PresenceStrip. Os deltas ao vivo do WebSocket convergem por cima
// (presenceStore.apply, alimentado pelo único attachTimerSync do shell).
// Degradação DURA (CA-08/RNF-06): store ausente/vazio → não renderiza nada.
import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getWorkspacePresence } from '../../lib/api';
import { useAuthStore } from '../../lib/auth';
import { usePresenceStore, activeMembers } from '../../lib/presenceStore';

export default function RemotePresence({ wsId }: { wsId?: string }) {
  const byWorkspace = usePresenceStore((s) => s.byWorkspace);
  const seed = usePresenceStore((s) => s.seed);
  const myId = useAuthStore((s) => s.me?.user.id);

  // snapshot inicial: semeia o store ao montar e ao trocar de ws (CA-08.2) —
  // sem isso /remoto a frio (deep-link/PWA shortcut) fica sem presence até
  // chegar um delta. O mesmo useQuery(['presence', wsId]) da fase 21.
  const snapQ = useQuery({
    queryKey: ['presence', wsId],
    queryFn: () => getWorkspacePresence(wsId ?? ''),
    enabled: !!wsId,
  });

  useEffect(() => {
    if (wsId && snapQ.data) seed(wsId, snapQ.data);
  }, [wsId, snapQ.data, seed]);

  // só o ws ativo (CA-11.1) e só os outros (≠ eu — o meu timer já é o herói).
  const members = activeMembers(byWorkspace, wsId).filter((m) => m.user_id !== myId);
  if (members.length === 0) return null; // degrada: nada de presence → some (CA-08)

  return (
    <div className="tt-card" data-testid="remote-presence" style={{ padding: 'var(--pad)' }}>
      <p className="eyebrow">Time ativo agora</p>
      <div style={{ display: 'grid', gap: 10 }}>
        {members.map((m) => (
          <div
            key={m.user_id}
            data-testid={`remote-presence-${m.user_id}`}
            style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}
          >
            <span
              className="tt-avatar tt-pulse"
              style={{ background: 'var(--surface-3)', color: 'var(--text-2)' }}
            >
              {m.name.trim().charAt(0).toUpperCase()}
            </span>
            <div style={{ minWidth: 0, fontSize: 'var(--fs-row)' }}>
              <span style={{ fontWeight: 500 }}>{m.name}</span>
              <span className="faint">: rodando {m.project ? m.project : 'sem projeto'}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
