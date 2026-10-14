// Remote (fase 23 T-07) — a rota /remoto: controle remoto dedicado mobile-first
// (DESIGN §3.3). Coluna ÚNICA, alvos de toque grandes — a tela enxuta POR
// ESCOLHA (D-01), não por viewport. Compõe RemoteControl (start/stop/trocar +
// timer ao vivo), RemotePresence (time ativo da 21, degrada) e RemoteShortcuts
// (Minhas tasks 18 + Foco 05). Tudo escopado ao workspace ATIVO (CA-11.1):
// useCurrentWorkspace() é a única fonte de ws.id; useRole(ws.id) decide canWrite
// (só o Iniciar é gateado — CA-11). Sem ws ativo (sessão sem dados) → vazio.
import { useCurrentWorkspace, useRole } from '../../lib/auth';
import RemoteControl from '../../components/remote/RemoteControl';
import RemotePresence from '../../components/remote/RemotePresence';
import RemoteShortcuts from '../../components/remote/RemoteShortcuts';

export default function Remote() {
  const ws = useCurrentWorkspace();
  const role = useRole(ws?.id);
  // start é a ÚNICA ação com 403 no backend (CA-11): owner/member podem iniciar,
  // viewer não. role null (sessão sem dados) → sem escrita.
  const canWrite = role === 'owner' || role === 'member';

  if (!ws) {
    return (
      <section data-testid="remote" className="tt-remote">
        <p className="faint">Sem workspace ativo.</p>
      </section>
    );
  }

  return (
    <section
      data-testid="remote"
      className="tt-remote"
      style={{
        display: 'grid',
        gap: 18,
        maxWidth: 440,
        margin: '0 auto',
        width: '100%',
        padding: 'var(--pad)',
      }}
    >
      <header style={{ display: 'grid', gap: 2 }}>
        <p className="eyebrow">Controle remoto</p>
        <h1 style={{ fontSize: 20, fontWeight: 600, letterSpacing: '-.02em' }}>{ws.name}</h1>
      </header>
      <div className="tt-card" style={{ padding: 'var(--pad)' }}>
        <RemoteControl ws={ws} canWrite={canWrite} />
      </div>
      <RemotePresence wsId={ws.id} />
      <RemoteShortcuts />
    </section>
  );
}
