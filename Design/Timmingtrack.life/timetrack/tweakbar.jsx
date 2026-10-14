/* Timetrack — floating live-skin bar (re-skins every frame on the canvas) */
const SA = window.__TT_ACCENTS || [
  { id: 'teal', hex: '#2dd4bf' }, { id: 'amber', hex: '#e0a13c' },
  { id: 'peri', hex: '#7c9cf5' }, { id: 'lime', hex: '#9ae66e' },
  { id: 'violet', hex: '#b07cd6' }, { id: 'rose', hex: '#e0795e' },
];

function TweakBar() {
  const [st, setSt] = React.useState(() => window.TTskin.get());
  const [open, setOpen] = React.useState(true);
  React.useEffect(() => window.TTskin.subscribe(setSt), []);
  const set = p => window.TTskin.set(p);

  const SegBtn = ({ on, children, onClick }) => (
    <button onClick={onClick} style={{
      border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540,
      padding: '6px 12px', borderRadius: 7, background: on ? '#34343a' : 'transparent',
      color: on ? '#f3f3f5' : '#9a9aa2', transition: 'background .12s',
    }}>{children}</button>
  );

  return (
    <div style={{
      position: 'fixed', bottom: 20, left: '50%', transform: 'translateX(-50%)', zIndex: 1000,
      display: 'flex', alignItems: 'center', gap: 14, padding: open ? '10px 14px' : '8px 12px',
      background: 'rgba(20,20,23,.92)', backdropFilter: 'blur(16px)', borderRadius: 16,
      border: '1px solid rgba(255,255,255,.1)', boxShadow: '0 12px 40px rgba(0,0,0,.4)',
      fontFamily: "'Geist', system-ui, sans-serif",
    }}>
      <button onClick={() => setOpen(o => !o)} title="Tweaks" style={{
        display: 'flex', alignItems: 'center', gap: 8, border: 'none', background: 'transparent', cursor: 'pointer', color: '#f3f3f5', padding: 0,
      }}>
        <span style={{ width: 7, height: 7, borderRadius: '50%', background: st.accent }} />
        <span style={{ fontSize: 12.5, fontWeight: 600, letterSpacing: '.02em' }}>Tweaks</span>
      </button>
      {open && <>
        <span style={{ width: 1, height: 22, background: 'rgba(255,255,255,.12)' }} />
        {/* accent swatches */}
        <div style={{ display: 'flex', gap: 7 }}>
          {SA.map(a => (
            <button key={a.id} onClick={() => set({ accentId: a.id, accent: a.hex })} title={a.id} style={{
              width: 22, height: 22, borderRadius: 7, cursor: 'pointer', background: a.hex,
              border: '2px solid', borderColor: st.accentId === a.id ? '#fff' : 'transparent',
            }} />
          ))}
        </div>
        <span style={{ width: 1, height: 22, background: 'rgba(255,255,255,.12)' }} />
        {/* theme */}
        <div style={{ display: 'flex', background: '#28282d', borderRadius: 9, padding: 2 }}>
          <SegBtn on={st.theme === 'dark'} onClick={() => set({ theme: 'dark' })}>Escuro</SegBtn>
          <SegBtn on={st.theme === 'light'} onClick={() => set({ theme: 'light' })}>Claro</SegBtn>
        </div>
        {/* density */}
        <div style={{ display: 'flex', background: '#28282d', borderRadius: 9, padding: 2 }}>
          <SegBtn on={st.density === 'comfortable'} onClick={() => set({ density: 'comfortable' })}>Confortável</SegBtn>
          <SegBtn on={st.density === 'compact'} onClick={() => set({ density: 'compact' })}>Compacta</SegBtn>
        </div>
      </>}
    </div>
  );
}

window.TT_TweakBar = { TweakBar };
