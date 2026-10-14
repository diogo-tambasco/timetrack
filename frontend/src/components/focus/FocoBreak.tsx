// FocoBreak (fase 15, CA-14/14.1/15) — o modo descanso REDESENHADO do
// /foco (referência: foco.jsx::FocoBreak): rótulo "Pausa Pomodoro" (ou
// "Pausa longa" quando break_kind==='long'), dígitos gigantes do restante,
// presets visíveis (chips) e controles retomar/pular/ajustar/parar. SEM
// clock próprio (RNF-03): o restante vem de useTimerClock (âncora do
// servidor). "ajustar" abre os Ajustes (navigate('/ajustes')). Toda cor é
// token --focus-*/--accent*. Mantém os testids/labels da fase 05 (focus-
// digits + nomes dos botões) pra não regredir os specs do Modo Foco.
import { useNavigate } from 'react-router-dom';
import { fmtClock } from '../../lib/format';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';

const PRESETS = [5, 15, 20];

const digitsStyle: React.CSSProperties = {
  fontSize: 'clamp(72px, 16vw, 200px)',
  fontWeight: 600,
  letterSpacing: '-.03em',
  lineHeight: 1,
  fontVariantNumeric: 'tabular-nums',
};

const labelStyle: React.CSSProperties = {
  fontSize: 13,
  fontWeight: 600,
  letterSpacing: '.14em',
  textTransform: 'uppercase',
  color: 'var(--accent)',
};

const ghostBtn: React.CSSProperties = {
  padding: '9px 20px',
  borderRadius: 999,
  border: '1px solid var(--focus-dim)',
  background: 'transparent',
  color: 'var(--focus-dim)',
  fontFamily: 'var(--font-ui)',
  fontSize: 13,
  fontWeight: 500,
  cursor: 'pointer',
};

export default function FocoBreak() {
  const navigate = useNavigate();
  const state = useTimerStore((s) => s.state);
  const resume = useTimerStore((s) => s.resume);
  const skip = useTimerStore((s) => s.skip);
  const stop = useTimerStore((s) => s.stop);
  const { remaining } = useTimerClock();

  if (!state || state.mode !== 'break') return null;
  const ended = (remaining ?? 0) <= 0;
  const isLong = state.break_kind === 'long';
  const activePreset = state.target_seconds ? Math.round(state.target_seconds / 60) : null;

  // rótulo: encerrada vence; senão longa×curta (CA-14.1)
  const label = ended ? 'Pausa encerrada' : isLong ? 'Pausa longa' : 'Pausa Pomodoro';

  return (
    <div data-testid="focus-break" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 22 }}>
      <span style={labelStyle}>{label}</span>

      <div className="mono" data-testid="focus-digits" style={digitsStyle}>
        {fmtClock(remaining ?? 0, false)}
      </div>

      {/* presets visíveis (chips) — o ativo casa com target/60 */}
      <div style={{ display: 'flex', gap: 10 }}>
        {PRESETS.map((m) => {
          const on = m === activePreset;
          return (
            <span
              key={m}
              style={{
                fontSize: 14,
                fontWeight: 540,
                padding: '6px 16px',
                borderRadius: 999,
                border: '1px solid',
                borderColor: on ? 'var(--accent)' : 'var(--focus-dim)',
                background: on ? 'var(--accent-soft)' : 'transparent',
                color: on ? 'var(--accent)' : 'var(--focus-dim)',
              }}
            >
              {m} min
            </span>
          );
        })}
      </div>

      <div style={{ display: 'flex', gap: 12, marginTop: 8 }}>
        <button style={ghostBtn} onClick={() => void resume()}>
          Retomar
        </button>
        {!ended && (
          <button style={ghostBtn} onClick={() => void skip()}>
            pular pausa
          </button>
        )}
        <button style={ghostBtn} onClick={() => navigate('/ajustes')}>
          ajustar
        </button>
        <button style={ghostBtn} onClick={() => void stop()}>
          Parar
        </button>
      </div>
    </div>
  );
}
