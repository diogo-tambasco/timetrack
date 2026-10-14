// TimerCard (fase 13, T-03) — o "Hero Timer" da Direção C do cockpit.jsx.
// REFLETE o timerStore (mesma fonte da TimerBar/Foco — CA-01/03): nunca abre
// requisição própria, muta só pelas ações do store. Três ramos: rodando
// (dígitos grandes + chip + Pausar/Parar), pausa (rótulo + countdown, sem o
// BreakOverlay completo — D-02) e ocioso (CTA, sem dígitos vivos — CA-14).
import { Icon } from '../icons';
import { useCurrentWorkspace } from '../../lib/auth';
import { fmtClock } from '../../lib/format';
import { useProjects } from '../../lib/queries/projects';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';

const heroDigits: React.CSSProperties = {
  fontSize: 46,
  fontWeight: 600,
  letterSpacing: '-.03em',
  color: 'var(--text-1)',
  fontVariantNumeric: 'tabular-nums',
  lineHeight: 1.05,
};

const labelStyle: React.CSSProperties = {
  fontSize: 11.5,
  fontWeight: 600,
  letterSpacing: '.04em',
  textTransform: 'uppercase',
  color: 'var(--accent-ink)',
};

export default function TimerCard({ onNewEntry }: { onNewEntry?: () => void }) {
  const state = useTimerStore((s) => s.state);
  if (state?.mode === 'break') return <BreakCard />;
  if (state) return <RunningCard onNewEntry={onNewEntry} />;
  return <IdleCard onNewEntry={onNewEntry} />;
}

// Rodando — "Rodando agora" + descrição/task + chip do projeto (archived:true,
// CA-12) + dígitos grandes (shown = countup?elapsed:remaining??elapsed, igual à
// TimerBar) + Pausar/Parar (ações do store).
function RunningCard({ onNewEntry }: { onNewEntry?: () => void }) {
  void onNewEntry;
  const ws = useCurrentWorkspace();
  const state = useTimerStore((s) => s.state);
  const pause = useTimerStore((s) => s.pause);
  const stop = useTimerStore((s) => s.stop);
  const { elapsed, remaining } = useTimerClock();
  const projectsQ = useProjects(ws?.id ?? '', { archived: true });

  if (!state?.entry) return null;
  const project = (projectsQ.data ?? []).find((p) => p.id === state.entry?.project_id);
  const shown = state.mode === 'countup' ? elapsed : (remaining ?? elapsed);

  return (
    <section data-testid="timer-card" className="tt-card" style={cardStyle}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
        <span style={labelStyle}>Rodando agora</span>
        {project && (
          <span className="tt-chip">
            <i className="tt-dot" style={{ background: project.color }} />
            {project.name}
          </span>
        )}
      </div>
      <div
        style={{
          fontSize: 16,
          fontWeight: 540,
          color: 'var(--text-1)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}
      >
        {state.entry.description || '(sem descrição)'}
      </div>
      <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 12 }}>
        <div className="mono" style={heroDigits}>
          {fmtClock(shown)}
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            className="tt-iconbtn"
            title="Pausar"
            aria-label="Pausar"
            style={{ background: 'var(--surface-3)', color: 'var(--text-1)' }}
            onClick={() => void pause()}
          >
            <Icon.pause size={15} />
          </button>
          <button
            className="tt-iconbtn tt-iconbtn--stop"
            title="Parar"
            aria-label="Parar"
            onClick={() => void stop()}
          >
            <Icon.stop size={15} />
          </button>
        </div>
      </div>
    </section>
  );
}

// Pausa — só REFLETE o estado (rótulo + countdown via useTimerClock); o
// BreakOverlay completo (presets/retomar/pular) segue na TimerBar (D-02/CA-13).
function BreakCard() {
  const { remaining } = useTimerClock();
  return (
    <section data-testid="timer-card" className="tt-card" style={cardStyle}>
      <span style={labelStyle}>Pausa</span>
      <div className="mono" style={heroDigits}>
        {fmtClock(remaining ?? 0)}
      </div>
      <p className="faint" style={{ fontSize: 12.5, margin: 0 }}>
        Em pausa — controle a pausa na barra do topo.
      </p>
    </section>
  );
}

// Ocioso — CTA pra começar, SEM dígitos vivos e SEM o formulário de start
// completo (esse é da TimerBar — CA-14/CA-03/D-03). Abre a "Nova entrada"
// (callback do dashboard) ou aponta pro topo.
function IdleCard({ onNewEntry }: { onNewEntry?: () => void }) {
  return (
    <section data-testid="timer-card" className="tt-card" style={cardStyle}>
      <span style={labelStyle}>Sem timer rodando</span>
      <div style={{ fontSize: 19, fontWeight: 600, color: 'var(--text-1)', letterSpacing: '-.01em' }}>
        Comece a cronometrar
      </div>
      <p className="faint" style={{ fontSize: 13, margin: 0 }}>
        Use a barra do topo pra iniciar, ou registre uma entrada manual.
      </p>
      {onNewEntry && (
        <button className="tt-btn tt-btn--accent" style={{ alignSelf: 'flex-start' }} onClick={onNewEntry}>
          <Icon.plus size={15} sw={2.2} />
          Comece a cronometrar
        </button>
      )}
    </section>
  );
}

const cardStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 12,
  padding: '20px 22px',
};
