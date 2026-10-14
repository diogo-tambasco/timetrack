// PresenceStrip (fase 21, CA-17/18) — a faixa de membros ATIVOS ao vivo do
// workspace atual: nome + projeto (se visível) + "rodando há Xmin" (do since,
// pelo relógio local — RNF-06). Lê o presenceStore (deltas do WebSocket) e
// SEMEIA o snapshot inicial via useQuery(['presence', wsId]) no mount e ao
// trocar de workspace (US-04/CA-18). Leitura pura — não dispara nada no
// servidor além do snapshot. Sem ninguém ativo → a faixa não aparece.
import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getWorkspacePresence } from '../lib/api';
import { usePresenceStore, activeMembers } from '../lib/presenceStore';

// minutesSince deriva "há Xmin" do since (ISO) pelo relógio local. Sem since →
// null (sem rótulo de tempo).
function labelSince(since: string | undefined, now: number): string | null {
  if (!since) return null;
  const startMs = Date.parse(since);
  if (Number.isNaN(startMs)) return null;
  const mins = Math.max(0, Math.floor((now - startMs) / 60000));
  if (mins < 1) return 'rodando agora';
  if (mins < 60) return `rodando há ${mins}min`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return m === 0 ? `rodando há ${h}h` : `rodando há ${h}h${m}min`;
}

export default function PresenceStrip({ wsId }: { wsId?: string }) {
  const byWorkspace = usePresenceStore((s) => s.byWorkspace);
  const seed = usePresenceStore((s) => s.seed);

  // snapshot inicial: semeia o store ao montar e ao trocar de ws (CA-18). O
  // delta ao vivo do WebSocket converge por cima (presenceStore.apply).
  const snapQ = useQuery({
    queryKey: ['presence', wsId],
    queryFn: () => getWorkspacePresence(wsId ?? ''),
    enabled: !!wsId,
  });

  useEffect(() => {
    if (wsId && snapQ.data) seed(wsId, snapQ.data);
  }, [wsId, snapQ.data, seed]);

  const members = activeMembers(byWorkspace, wsId);
  if (members.length === 0) return null; // ninguém ativo → faixa some (CA-17)

  const now = Date.now();
  return (
    <div
      className="tt-card"
      data-testid="presence-strip"
      style={{ padding: 'var(--pad)' }}
    >
      <p className="eyebrow">Ativos agora</p>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
        {members.map((m) => {
          const tempo = labelSince(m.since, now);
          return (
            <div
              key={m.user_id}
              data-testid={`presence-${m.user_id}`}
              style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}
            >
              <span
                className="tt-avatar tt-pulse"
                style={{ background: 'var(--surface-3)', color: 'var(--text-2)' }}
              >
                {m.name.trim().charAt(0).toUpperCase()}
              </span>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 'var(--fs-row)', fontWeight: 500 }}>{m.name}</div>
                <div className="faint" style={{ fontSize: 11.5 }}>
                  {m.project ? <span>{m.project}</span> : <span>sem projeto</span>}
                  {tempo ? <span> · {tempo}</span> : null}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
