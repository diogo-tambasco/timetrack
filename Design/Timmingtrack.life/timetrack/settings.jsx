/* Timetrack — Ajustes (completo). Aparência mexe no window.TTskin de verdade. */
const { Icon: SIcon, SideNav: SSide, Switch: SSwitch, Avatar: SAvatar } = window.TT;
function sskin() { return window.TTskin || { get: () => ({}), set: () => {} }; }

const S_ACCENTS = [
  { id: 'teal', hex: '#2dd4bf' }, { id: 'amber', hex: '#e0a13c' }, { id: 'peri', hex: '#7c9cf5' },
  { id: 'lime', hex: '#9ae66e' }, { id: 'violet', hex: '#b07cd6' }, { id: 'rose', hex: '#e0795e' },
];
const S_NAV = ['Aparência', 'Modo Foco', 'Pomodoro', 'Projetos', 'Conta', 'Integrações', 'Conector Claude', 'Notificações'];

function SGroup({ title, sub, children, full }) {
  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 20, gridColumn: full ? '1 / -1' : 'auto' }}>
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 14, fontWeight: 600 }}>{title}</div>
        {sub && <div className="faint" style={{ fontSize: 12, marginTop: 2 }}>{sub}</div>}
      </div>
      {children}
    </section>
  );
}
function Seg({ options, value, onChange }) {
  return (
    <div style={{ display: 'inline-flex', background: 'var(--surface-3)', borderRadius: 9, padding: 3, gap: 3 }}>
      {options.map(o => (
        <button key={o.v} onClick={() => onChange && onChange(o.v)} style={{
          border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540, padding: '6px 14px', borderRadius: 6,
          background: value === o.v ? 'var(--surface-1)' : 'transparent', color: value === o.v ? 'var(--text-1)' : 'var(--text-2)',
        }}>{o.l}</button>
      ))}
    </div>
  );
}
function Row({ label, hint, children }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
      <div><div style={{ fontSize: 13 }}>{label}</div>{hint && <div className="faint" style={{ fontSize: 11.5, marginTop: 2 }}>{hint}</div>}</div>
      {children}
    </div>
  );
}

function Settings() {
  const init = sskin().get();
  const [accent, setAccent] = React.useState(init.accentId || 'teal');
  const [theme, setTheme] = React.useState(init.theme || 'dark');
  const [density, setDensity] = React.useState(init.density || 'comfortable');
  const [focusMin, setFocusMin] = React.useState(50);
  const [breakMin, setBreakMin] = React.useState(5);
  const [digits, setDigits] = React.useState('mono');
  const me = window.TT_DATA.members[0];

  const pick = (id, hex) => { setAccent(id); sskin().set({ accentId: id, accent: hex }); };

  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <SSide active="cog" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', overflow: 'hidden' }}>
        {/* settings sub-nav */}
        <div style={{ width: 196, flex: '0 0 auto', borderRight: '1px solid var(--border)', padding: '22px 12px', background: 'var(--surface-1)' }}>
          <div className="eyebrow" style={{ padding: '0 10px 12px' }}>Ajustes</div>
          {S_NAV.map((s, i) => (
            <div key={s} className="tt-nav" aria-current={i === 0} style={{ fontSize: 13 }}>{s}</div>
          ))}
        </div>

        <div style={{ flex: 1, minWidth: 0, overflowY: 'auto', padding: 26 }}>
          <div style={{ marginBottom: 20 }}>
            <h1 style={{ margin: 0, fontSize: 24, fontWeight: 600, letterSpacing: '-.02em' }}>Aparência & conta</h1>
            <p className="faint" style={{ fontSize: 13, marginTop: 6 }}>O app é seu. Accent, tema e densidade valem pra todas as telas — em tempo real.</p>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18, alignItems: 'start' }}>
            <SGroup title="Accent" sub="Cor de ação e estado ativo. Nada de magenta.">
              <div style={{ display: 'flex', gap: 11, flexWrap: 'wrap' }}>
                {S_ACCENTS.map(a => (
                  <button key={a.id} onClick={() => pick(a.id, a.hex)} style={{ width: 36, height: 36, borderRadius: 10, cursor: 'pointer', background: a.hex, border: '2px solid', borderColor: accent === a.id ? 'var(--text-1)' : 'transparent', position: 'relative' }}>
                    {accent === a.id && <span style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', color: '#06231e', fontWeight: 700, fontSize: 15 }}>✓</span>}
                  </button>
                ))}
                <div style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '0 12px', borderRadius: 10, border: '1px dashed var(--border-2)', height: 36 }}><SIcon.plus size={13} style={{ color: 'var(--text-3)' }} /><span className="mono faint" style={{ fontSize: 11.5 }}>#hex</span></div>
              </div>
            </SGroup>

            <SGroup title="Tema & densidade">
              <div style={{ display: 'flex', flexDirection: 'column', gap: 15 }}>
                <Row label="Tema"><Seg value={theme} onChange={v => { setTheme(v); sskin().set({ theme: v }); }} options={[{ v: 'dark', l: 'Escuro' }, { v: 'light', l: 'Claro' }, { v: 'system', l: 'Sistema' }]} /></Row>
                <Row label="Densidade"><Seg value={density} onChange={v => { setDensity(v); sskin().set({ density: v }); }} options={[{ v: 'comfortable', l: 'Confortável' }, { v: 'compact', l: 'Compacta' }]} /></Row>
              </div>
            </SGroup>

            <SGroup title="Modo Foco" sub="Aparência da tela flip-clock no iPad.">
              <div style={{ display: 'flex', flexDirection: 'column', gap: 15 }}>
                <Row label="Estilo dos dígitos"><Seg value={digits} onChange={setDigits} options={[{ v: 'mono', l: 'Mono' }, { v: 'flip', l: 'Flip' }]} /></Row>
                <Row label="Mostrar projeto/task"><SSwitch on /></Row>
                <Row label="Manter tela acordada" hint="Wake Lock"><SSwitch on /></Row>
              </div>
            </SGroup>

            <SGroup title="Pomodoro" sub="Ao pausar, abre uma pausa com esta duração.">
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8, fontSize: 13 }}><span>Foco</span><span className="mono" style={{ fontWeight: 600 }}>{focusMin} min</span></div>
                  <input type="range" min="15" max="90" step="5" value={focusMin} onChange={e => setFocusMin(+e.target.value)} style={{ width: '100%', accentColor: 'var(--accent)' }} />
                </div>
                <div>
                  <div style={{ fontSize: 13, marginBottom: 8 }}>Pausa</div>
                  <div style={{ display: 'flex', gap: 8 }}>
                    {[5, 15, 20].map(m => (
                      <button key={m} onClick={() => setBreakMin(m)} style={{ flex: 1, padding: '8px 0', borderRadius: 9, cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540, border: '1px solid', borderColor: breakMin === m ? 'var(--accent)' : 'var(--border-2)', background: breakMin === m ? 'var(--accent-soft)' : 'transparent', color: breakMin === m ? 'var(--accent)' : 'var(--text-2)' }}>{m} min</button>
                    ))}
                  </div>
                </div>
              </div>
            </SGroup>

            <SGroup title="Conta">
              <div style={{ display: 'flex', alignItems: 'center', gap: 13 }}>
                <SAvatar m={me} size={44} />
                <div style={{ flex: 1 }}><div style={{ fontSize: 14, fontWeight: 600 }}>{me.name}</div><div className="faint" style={{ fontSize: 12 }}>{me.email}</div></div>
                <span className="tt-role tt-role--owner"><SIcon.shield size={12} />Owner</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 16, padding: '11px 13px', borderRadius: 'var(--r-md)', border: '1px solid var(--border)', background: 'var(--surface-2)' }}>
                <span style={{ width: 26, height: 26, borderRadius: 7, background: '#fff', display: 'grid', placeItems: 'center', fontWeight: 700, color: '#1a73e8', fontSize: 14 }}>G</span>
                <div style={{ flex: 1 }}><div style={{ fontSize: 12.5, fontWeight: 540 }}>Google conectado</div><div className="faint" style={{ fontSize: 11.5 }}>diogo@primorlabs.cloud</div></div>
                <button className="tt-btn tt-btn--ghost" style={{ padding: '6px 10px', fontSize: 12 }}>Gerenciar</button>
              </div>
            </SGroup>

            <SGroup title="Conector Claude" sub="Skill /timetrack-task — cria task e já começa a cronometrar.">
              <div style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '10px 12px', borderRadius: 'var(--r-md)', background: 'var(--surface-3)', marginBottom: 12 }}>
                <SIcon.bolt size={15} style={{ color: 'var(--accent)' }} />
                <code className="mono" style={{ fontSize: 12.5, flex: 1 }}>/timetrack-task new</code>
              </div>
              <div style={{ fontSize: 12, color: 'var(--text-2)', marginBottom: 7 }}>Token da API</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '9px 12px', borderRadius: 'var(--r-md)', border: '1px solid var(--border-2)', background: 'var(--surface-2)' }}>
                <code className="mono faint" style={{ fontSize: 12.5, flex: 1, letterSpacing: '.05em' }}>tt_live_••••••••••••3f9a</code>
                <button className="tt-btn tt-btn--ghost" style={{ padding: '5px 9px', fontSize: 12 }}>Copiar</button>
                <button className="tt-btn tt-btn--ghost" style={{ padding: '5px 9px', fontSize: 12 }}>Revogar</button>
              </div>
            </SGroup>

            <SGroup title="Notificações" full>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px 40px' }}>
                <Row label="Fim do Pomodoro"><SSwitch on /></Row>
                <Row label="Meta semanal atingida"><SSwitch on /></Row>
                <Row label="Timer rodando há +2h"><SSwitch on /></Row>
                <Row label="Resumo semanal por email"><SSwitch /></Row>
                <Row label="Membro começou a rastrear"><SSwitch /></Row>
                <Row label="Convite aceito"><SSwitch on /></Row>
              </div>
            </SGroup>
          </div>
        </div>
      </main>
    </div>
  );
}

window.TT_Settings = { Settings };
