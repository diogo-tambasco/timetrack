/* Timetrack — Modo Foco (iPad). Work (count-up) + Break (Pomodoro). */
const { Icon: FIcon, useClock: fUseClock, fmt: fFmt, proj: fProj } = window.TT;

function GiantDigits({ value, size = 152, color = 'var(--text-1)', glow }) {
  // render char-by-char so colons don't jump; tabular mono
  return (
    <div className="mono" style={{
      fontSize: size, fontWeight: 600, lineHeight: 1, letterSpacing: '-.04em',
      color, fontVariantNumeric: 'tabular-nums', display: 'flex', justifyContent: 'center',
      textShadow: glow ? `0 0 60px ${glow}` : 'none', userSelect: 'none',
    }}>
      {value.split('').map((ch, i) => (
        <span key={i} style={{ display: 'inline-block', width: ch === ':' ? '0.36em' : '0.62em', textAlign: 'center' }}>{ch}</span>
      ))}
    </div>
  );
}

function useCountdown(startSec) {
  const [s, setS] = React.useState(startSec);
  React.useEffect(() => { const t = setInterval(() => setS(v => (v > 0 ? v - 1 : 0)), 1000); return () => clearInterval(t); }, [startSec]);
  return s;
}

function FocoShell({ children, tint }) {
  return (
    <div className="tt" style={{
      width: '100%', height: '100%', position: 'relative', overflow: 'hidden',
      background: tint || '#000', color: '#fff',
      display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    }}>{children}</div>
  );
}

function TopBar({ right }) {
  return (
    <div style={{ position: 'absolute', top: 0, left: 0, right: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '26px 30px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 9, fontSize: 13, color: 'rgba(255,255,255,.45)' }}>
        <span className="tt-dot" style={{ background: 'var(--accent)', width: 7, height: 7 }} />tela acordada
      </div>
      {right}
    </div>
  );
}

/* Work mode — giant count-up, project context, big controls */
function FocoWork() {
  const sec = fUseClock(5070, true);
  const e = window.TT_DATA.activeEntry;
  return (
    <FocoShell>
      <TopBar right={<span className="mono" style={{ fontSize: 13, color: 'rgba(255,255,255,.4)' }}>16:36</span>} />
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 30 }}>
        <GiantDigits value={fFmt(sec)} size={150} glow="rgba(45,212,191,.18)" />
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, color: 'rgba(255,255,255,.62)' }}>
          <span className="tt-dot" style={{ background: fProj(e.p).color, width: 11, height: 11 }} />
          <span style={{ fontSize: 22, fontWeight: 500, color: '#fff' }}>{e.task}</span>
          <span style={{ fontSize: 22 }}>·</span>
          <span style={{ fontSize: 22 }}>{fProj(e.p).name}</span>
        </div>
      </div>
      <div style={{ position: 'absolute', bottom: 56, display: 'flex', alignItems: 'center', gap: 22 }}>
        <button style={btnFoco('ghost')}><FIcon.arrow size={18} style={{ transform: 'rotate(180deg)' }} />trocar task</button>
        <button style={btnFoco('big')}><FIcon.pause size={26} /></button>
        <button style={btnFoco('ghost')}><FIcon.stop size={17} />parar</button>
      </div>
    </FocoShell>
  );
}

/* Break mode — pause → Pomodoro. Calmer "modo descanso" with accent breath */
function FocoBreak() {
  const left = useCountdown(272); // 4:32 left of a 5:00 break
  const presets = [5, 15, 20];
  return (
    <FocoShell tint="radial-gradient(120% 90% at 50% 30%, color-mix(in srgb, var(--accent) 16%, #07100e) 0%, #060807 70%)">
      <TopBar right={<span style={{ fontSize: 13, color: 'var(--accent)', fontWeight: 600, letterSpacing: '.1em', textTransform: 'uppercase' }}>modo descanso</span>} />
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 26 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, color: 'var(--accent)' }}>
          <FIcon.clock size={18} /><span style={{ fontSize: 16, fontWeight: 600, letterSpacing: '.14em', textTransform: 'uppercase' }}>Pausa Pomodoro</span>
        </div>
        <GiantDigits value={fFmt(left, false)} size={168} color="#fff" glow="rgba(45,212,191,.28)" />
        <div style={{ display: 'flex', gap: 10 }}>
          {presets.map((m, i) => (
            <span key={m} style={{
              fontSize: 15, fontWeight: 540, padding: '7px 18px', borderRadius: 999,
              border: '1px solid', borderColor: i === 0 ? 'var(--accent)' : 'rgba(255,255,255,.14)',
              background: i === 0 ? 'var(--accent-soft)' : 'transparent',
              color: i === 0 ? 'var(--accent)' : 'rgba(255,255,255,.6)',
            }}>{m} min</span>
          ))}
        </div>
      </div>
      <div style={{ position: 'absolute', bottom: 56, display: 'flex', alignItems: 'center', gap: 22 }}>
        <button style={btnFoco('ghost')}>pular pausa</button>
        <button style={{ ...btnFoco('big'), background: 'var(--accent)', color: 'var(--accent-ink)' }}><FIcon.play size={24} /></button>
        <button style={btnFoco('ghost')}><FIcon.cog size={16} />ajustar</button>
      </div>
    </FocoShell>
  );
}

function btnFoco(kind) {
  if (kind === 'big') return {
    width: 78, height: 78, borderRadius: '50%', border: 'none', cursor: 'pointer',
    display: 'grid', placeItems: 'center', background: 'rgba(255,255,255,.1)', color: '#fff',
  };
  return {
    display: 'inline-flex', alignItems: 'center', gap: 9, padding: '12px 20px', borderRadius: 999,
    border: '1px solid rgba(255,255,255,.13)', background: 'transparent', cursor: 'pointer',
    color: 'rgba(255,255,255,.6)', fontSize: 15, fontWeight: 500, fontFamily: 'var(--font-ui)',
  };
}

/* Task picker — the iPad entry: just type + tap a recent project.
   The anti-Toggl: spacious, big touch targets, recents up front. */
function FocoPicker() {
  const D = window.TT_DATA;
  const [sel, setSel] = React.useState('la');
  const recent = D.projects.slice(0, 5);
  const cont = D.entries.slice(1, 3);
  const caret = { display: 'inline-block', width: 2, height: 30, background: 'var(--accent)', marginLeft: 2, verticalAlign: '-4px', animation: 'ttpulse 1.2s steps(1) infinite' };
  return (
    <div className="tt" style={{
      width: '100%', height: '100%', color: '#fff', position: 'relative',
      background: 'radial-gradient(130% 90% at 50% -10%, #14171a 0%, #08090a 68%)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '0 86px',
    }}>
      <TopBar right={<span className="mono" style={{ fontSize: 13, color: 'rgba(255,255,255,.4)' }}>16:36</span>} />
      <div style={{ width: '100%', maxWidth: 780, display: 'flex', flexDirection: 'column', gap: 30 }}>
        <div>
          <div className="eyebrow" style={{ color: 'var(--accent)', letterSpacing: '.14em' }}>Modo Foco · nova sessão</div>
          <h1 style={{ margin: '8px 0 0', fontSize: 30, fontWeight: 600, letterSpacing: '-.02em' }}>No que você tá trabalhando?</h1>
        </div>

        {/* big type field */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 16, padding: '20px 24px', borderRadius: 18,
          background: 'rgba(255,255,255,.05)', border: '1.5px solid var(--accent-line)',
        }}>
          <span className="tt-dot" style={{ background: fProj(sel).color, width: 12, height: 12 }} />
          <span style={{ fontSize: 26, fontWeight: 500, color: '#fff' }}>Revisão de laudas — lote 14</span>
          <span style={caret} />
          <span style={{ flex: 1 }} />
          <span style={{ fontSize: 16, color: 'rgba(255,255,255,.4)' }}>{fProj(sel).name}</span>
        </div>

        {/* recent projects — big tappable */}
        <div>
          <div style={{ fontSize: 14, color: 'rgba(255,255,255,.5)', marginBottom: 12 }}>Projetos recentes</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12 }}>
            {recent.map(p => {
              const on = sel === p.id;
              return (
                <button key={p.id} onClick={() => setSel(p.id)} style={{
                  display: 'flex', flexDirection: 'column', gap: 10, padding: '16px 16px', borderRadius: 16,
                  cursor: 'pointer', textAlign: 'left', minHeight: 92,
                  background: on ? 'rgba(255,255,255,.08)' : 'rgba(255,255,255,.04)',
                  border: '1.5px solid', borderColor: on ? p.color : 'rgba(255,255,255,.08)',
                }}>
                  <span style={{ width: 12, height: 12, borderRadius: '50%', background: p.color }} />
                  <div>
                    <div style={{ fontSize: 16, fontWeight: 600, color: '#fff' }}>{p.name}</div>
                    <div style={{ fontSize: 12.5, color: 'rgba(255,255,255,.45)', marginTop: 2 }}>{p.client}</div>
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* continue a recent entry */}
        <div>
          <div style={{ fontSize: 14, color: 'rgba(255,255,255,.5)', marginBottom: 12 }}>Continuar</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {cont.map((r, i) => (
              <button key={i} style={{
                display: 'flex', alignItems: 'center', gap: 14, padding: '14px 18px', borderRadius: 14, cursor: 'pointer',
                background: 'rgba(255,255,255,.04)', border: '1px solid rgba(255,255,255,.08)', color: '#fff', textAlign: 'left',
              }}>
                <span className="tt-dot" style={{ background: fProj(r.p).color, width: 9, height: 9 }} />
                <span style={{ flex: 1, fontSize: 16, fontWeight: 500 }}>{r.task}</span>
                <span className="mono" style={{ fontSize: 14, color: 'rgba(255,255,255,.5)' }}>{r.dur}</span>
                <FIcon.play size={18} style={{ color: 'var(--accent)' }} />
              </button>
            ))}
          </div>
        </div>

        {/* start */}
        <button style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 12, height: 64, borderRadius: 16,
          border: 'none', cursor: 'pointer', background: 'var(--accent)', color: 'var(--accent-ink)',
          fontFamily: 'var(--font-ui)', fontSize: 18, fontWeight: 600,
        }}>
          <FIcon.play size={20} />Começar a cronometrar
        </button>
      </div>
    </div>
  );
}

window.TT_Foco = { FocoWork, FocoBreak, FocoPicker };
