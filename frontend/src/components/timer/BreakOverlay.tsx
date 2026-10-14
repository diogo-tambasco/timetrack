// BreakOverlay (T-16) — o "modo descanso" compacto do cockpit (CA-32):
// contagem regressiva da pausa, presets visíveis e retomar/pular/parar.
// Referência de COMPORTAMENTO: FocoBreak de foco.jsx — o fullscreen é a
// fase 05. Pausa zerada de countup/countdown vira "pausa encerrada"
// (CA-34.2): quem retoma é o usuário; o auto-resume é só do ciclo pomodoro
// e mora no timerStore (CA-34.1).
import { Icon } from '../icons';
import { fmtClock } from '../../lib/format';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';

const PRESETS = [5, 15, 20];

const ghostBtn: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: 7,
  padding: '7px 14px',
  borderRadius: 999,
  border: '1px solid var(--border-2)',
  background: 'transparent',
  cursor: 'pointer',
  color: 'var(--text-2)',
  fontFamily: 'var(--font-ui)',
  fontSize: 12.5,
  fontWeight: 500,
};

export default function BreakOverlay() {
  const state = useTimerStore((s) => s.state);
  const resume = useTimerStore((s) => s.resume);
  const skip = useTimerStore((s) => s.skip);
  const stop = useTimerStore((s) => s.stop);
  const { remaining } = useTimerClock();

  if (!state || state.mode !== 'break') return null;
  const ended = (remaining ?? 0) <= 0;
  const isLong = state.break_kind === 'long';
  const activePreset = state.target_seconds ? Math.round(state.target_seconds / 60) : null;
  // rótulo: encerrada vence; senão longa×curta (CA-16, espelha o FocoBreak)
  const label = ended ? 'Pausa encerrada' : isLong ? 'Pausa longa' : 'Pausa Pomodoro';

  return (
    <div
      data-testid="break-overlay"
      className="tt-breakbar"
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--gap)',
        width: '100%',
        borderRadius: 'var(--r-md)',
        background: 'var(--accent-soft)',
        padding: '8px 14px',
      }}
    >
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8, color: 'var(--accent)' }}>
        <Icon.clock size={15} />
        <span style={{ fontSize: 12, fontWeight: 600, letterSpacing: '.12em', textTransform: 'uppercase' }}>
          {label}
        </span>
      </span>
      <div
        className="mono"
        style={{
          fontSize: 22,
          fontWeight: 600,
          letterSpacing: '-.02em',
          color: 'var(--text-1)',
          fontVariantNumeric: 'tabular-nums',
          minWidth: 72,
        }}
      >
        {fmtClock(remaining ?? 0, false)}
      </div>
      <div style={{ display: 'flex', gap: 6 }}>
        {PRESETS.map((m) => {
          const on = m === activePreset;
          return (
            <span
              key={m}
              style={{
                fontSize: 11.5,
                fontWeight: 540,
                padding: '4px 11px',
                borderRadius: 999,
                border: '1px solid',
                borderColor: on ? 'var(--accent)' : 'var(--border-2)',
                background: on ? 'var(--accent-soft)' : 'transparent',
                color: on ? 'var(--accent)' : 'var(--text-3)',
              }}
            >
              {m} min
            </span>
          );
        })}
      </div>
      <span style={{ flex: 1 }} />
      {!ended && (
        <button style={ghostBtn} onClick={() => void skip()}>
          pular pausa
        </button>
      )}
      <button
        className="tt-iconbtn tt-iconbtn--go"
        title="Retomar"
        aria-label="Retomar"
        onClick={() => void resume()}
      >
        <Icon.play size={15} />
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
  );
}
