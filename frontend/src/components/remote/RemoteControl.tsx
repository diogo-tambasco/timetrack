// RemoteControl (fase 23 T-02/03/04) — o coração do controle remoto: start/stop
// + trocar projeto rápido + timer ao vivo. CONSOME o timerStore (CA-12/13): as
// ações são start/stop/pause do store (nunca rejeitam — RNF-04, `void ação()`),
// os dígitos derivam de useTimerClock (a MESMA expressão `shown` da TimerBar) e
// a pausa REUSA o BreakOverlay. Zero fetch/getTimer/startTimer direto, zero
// segundo attachTimerSync. Mobile-first: coluna única, botões ≥56px.
import { useState } from 'react';
import { Icon } from '../icons';
import { fmtClock } from '../../lib/format';
import { useProjects } from '../../lib/queries/projects';
import { useRecentEntries } from '../../lib/queries/entries';
import { recentProjectIds } from '../../lib/recentProjects';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';
import BreakOverlay from '../timer/BreakOverlay';

// botão de ação principal (Iniciar/Parar/Pausar) — ≥56px (CA-03/04/05). Cor =
// token sempre (RNF-07).
const actionBtn: React.CSSProperties = {
  minHeight: 56,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 8,
  borderRadius: 'var(--r-md)',
  border: 'none',
  cursor: 'pointer',
  fontFamily: 'var(--font-ui)',
  fontSize: 16,
  fontWeight: 600,
  padding: '0 18px',
  width: '100%',
};

export default function RemoteControl({
  ws,
  canWrite,
}: {
  ws: { id: string; name: string };
  canWrite: boolean;
}) {
  const state = useTimerStore((s) => s.state);

  if (state?.mode === 'break') {
    // pausa: REUSA o BreakOverlay (CA-05.1/12) — retomar/pular/parar são
    // per-usuário (sem 403), então aparecem SEMPRE, inclusive viewer (CA-11).
    return (
      <div data-testid="remote-control" style={{ width: '100%' }}>
        <BreakOverlay />
      </div>
    );
  }
  if (state) return <RunningRemote ws={ws} />;
  // ocioso: o Iniciar (única ação com 403 no backend) é gateado por canWrite
  // (CA-11). Viewer ocioso vê só o aviso de "sem timer".
  return canWrite ? <IdleRemote ws={ws} /> : <ViewerIdle />;
}

// IdleRemote (CA-04/06/06.1) — descrição grande + seletor de projeto + fila de
// recentes (toque único) + Iniciar. Espelha a IdleBar da TimerBar.
function IdleRemote({ ws }: { ws: { id: string } }) {
  const [description, setDescription] = useState('');
  const [projectID, setProjectID] = useState('');
  const start = useTimerStore((s) => s.start);

  const projectsActiveQ = useProjects(ws.id);
  const projectsAllQ = useProjects(ws.id, { archived: true });
  const recentQ = useRecentEntries(ws.id);

  const activeProjects = (projectsActiveQ.data ?? []).filter((p) => !p.archived_at);
  // recentes: ids distintos das entries recentes (CA-06), resolvidos a nome+cor
  // via projetos COM arquivados (CA-06.1 — a entry recente pode apontar pra
  // arquivado e o chip não some). Id sem projeto resolvível é omitido.
  const allProjects = projectsAllQ.data ?? [];
  const recentChips = recentProjectIds(recentQ.data?.entries)
    .map((id) => allProjects.find((p) => p.id === id))
    .filter((p): p is NonNullable<typeof p> => Boolean(p));

  const onStart = () => {
    void start({
      workspace_id: ws.id,
      description,
      project_id: projectID || null,
      task_id: null,
      tags: [],
    });
    setDescription('');
  };

  return (
    <div data-testid="remote-control" style={{ display: 'grid', gap: 14, width: '100%' }}>
      <input
        data-testid="remote-description"
        placeholder="No que você está trabalhando?"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && onStart()}
        style={{
          width: '100%',
          minHeight: 52,
          background: 'var(--surface-2)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--r-md)',
          padding: '0 16px',
          color: 'var(--text-1)',
          fontFamily: 'var(--font-ui)',
          fontSize: 16,
          outline: 'none',
        }}
      />
      <select
        aria-label="Projeto"
        value={projectID}
        onChange={(e) => setProjectID(e.target.value)}
        style={{
          width: '100%',
          minHeight: 48,
          background: 'var(--surface-2)',
          color: 'var(--text-1)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--r-md)',
          padding: '0 14px',
          fontFamily: 'var(--font-ui)',
          fontSize: 15,
        }}
      >
        <option value="">Sem projeto</option>
        {activeProjects.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>

      {recentChips.length > 0 && (
        <div data-testid="remote-recents" style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          {recentChips.map((p) => {
            const on = p.id === projectID;
            return (
              <button
                key={p.id}
                type="button"
                data-testid={`remote-recent-${p.id}`}
                aria-pressed={on}
                onClick={() => setProjectID(p.id)}
                className="tt-chip"
                style={{
                  cursor: 'pointer',
                  borderColor: on ? 'var(--accent)' : 'var(--border-2)',
                  background: on ? 'var(--accent-soft)' : 'transparent',
                }}
              >
                <i className="tt-dot" style={{ background: p.color }} />
                {p.name}
              </button>
            );
          })}
        </div>
      )}

      <button
        type="button"
        onClick={onStart}
        className="tt-iconbtn--go"
        title="Iniciar"
        aria-label="Iniciar"
        style={{ ...actionBtn, background: 'var(--accent)', color: 'var(--accent-ink)' }}
      >
        <Icon.play size={18} />
        Iniciar
      </button>
    </div>
  );
}

// RunningRemote (CA-05/07/12/13) — descrição read-only + chip do projeto +
// dígitos GRANDES (useTimerClock, expressão `shown` idêntica à TimerBar) +
// Pausar/Parar. Aparecem SEMPRE que rodando, inclusive viewer (CA-11).
function RunningRemote({ ws }: { ws: { id: string } }) {
  const state = useTimerStore((s) => s.state);
  const stop = useTimerStore((s) => s.stop);
  const pause = useTimerStore((s) => s.pause);
  const { elapsed, remaining } = useTimerClock();
  // archived:true — a entry rodando pode apontar pra projeto arquivado e o chip
  // do CA-07 não pode sumir (mesmo motivo da RunningBar).
  const projectsQ = useProjects(ws.id, { archived: true });

  if (!state?.entry) return null;
  const project = (projectsQ.data ?? []).find((p) => p.id === state.entry?.project_id);
  const shown = state.mode === 'countup' ? elapsed : (remaining ?? elapsed);

  return (
    <div data-testid="remote-control" style={{ display: 'grid', gap: 14, width: '100%' }}>
      <input
        data-testid="remote-description"
        value={state.entry.description}
        readOnly
        style={{
          width: '100%',
          minHeight: 52,
          background: 'var(--surface-2)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--r-md)',
          padding: '0 16px',
          color: 'var(--text-1)',
          fontFamily: 'var(--font-ui)',
          fontSize: 16,
          outline: 'none',
        }}
      />
      {project && (
        <span className="tt-chip" style={{ alignSelf: 'flex-start' }}>
          <i className="tt-dot" style={{ background: project.color }} />
          {project.name}
        </span>
      )}
      <div
        className="mono"
        data-testid="remote-digits"
        style={{
          fontSize: 56,
          fontWeight: 600,
          letterSpacing: '-.02em',
          color: 'var(--text-1)',
          fontVariantNumeric: 'tabular-nums',
          textAlign: 'center',
          lineHeight: 1.1,
        }}
      >
        {fmtClock(shown)}
      </div>
      <div style={{ display: 'flex', gap: 12 }}>
        <button
          type="button"
          onClick={() => void pause()}
          title="Pausar"
          aria-label="Pausar"
          style={{ ...actionBtn, background: 'var(--surface-3)', color: 'var(--text-1)' }}
        >
          <Icon.pause size={18} />
          Pausar
        </button>
        <button
          type="button"
          onClick={() => void stop()}
          className="tt-pulse"
          title="Parar"
          aria-label="Parar"
          style={{ ...actionBtn, background: 'var(--danger)', color: 'var(--accent-ink)' }}
        >
          <Icon.stop size={18} />
          Parar
        </button>
      </div>
    </div>
  );
}

// ViewerIdle — viewer ocioso no ws ativo: o backend daria 403 no start, então a
// tela não oferece o controle de Iniciar (CA-11). Sem campo/seletor/recentes.
function ViewerIdle() {
  return (
    <div data-testid="remote-control" style={{ width: '100%' }}>
      <p className="faint" style={{ fontSize: 'var(--fs-row)' }}>
        Sem timer rodando.
      </p>
    </div>
  );
}
