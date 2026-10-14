/* Timetrack — Integração Google Calendar (2-way) */
const { Icon: GIcon, SideNav: GSide, Switch: GSwitch, Dot: GDot, proj: gProj } = window.TT;
const GD = () => window.TT_DATA.gcal;

function GMark({ size = 30 }) {
  return (
    <span style={{ width: size, height: size, borderRadius: 8, background: '#fff', display: 'grid', placeItems: 'center', flex: '0 0 auto', boxShadow: '0 1px 2px rgba(0,0,0,.2)' }}>
      <span style={{ fontFamily: 'var(--font-ui)', fontWeight: 700, fontSize: size * .5, color: '#1a73e8' }}>G</span>
    </span>
  );
}

function GPanel({ title, sub, right, children }) {
  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', borderBottom: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}><span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span>{sub && <span className="faint" style={{ fontSize: 12 }}>{sub}</span>}</div>
        {right}
      </div>
      {children}
    </section>
  );
}

function MapState({ state }) {
  if (state === 'synced') return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 11.5, fontWeight: 540, color: 'var(--accent)' }}><GIcon.check size={13} sw={2.2} />sincronizado</span>;
  if (state === 'push') return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 11.5, fontWeight: 540, color: 'var(--text-2)' }}><GIcon.arrow size={13} />enviando p/ GCal</span>;
  return <span className="faint" style={{ fontSize: 11.5 }}>ignorado</span>;
}

function Gcal() {
  const d = GD();
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <GSide active="cal" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 26, gap: 20, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow">Integração</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 6 }}>
              <GMark size={34} />
              <h1 style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>Google Calendar</h1>
            </div>
            <p className="faint" style={{ fontSize: 13, marginTop: 8 }}>O calendário é a fonte da verdade. Sincronização 2-way · cada membro conecta seu Google.</p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
            <span className="faint" style={{ fontSize: 12.5 }}>Sincronizado {d.lastSync}</span>
            <button className="tt-btn tt-btn--accent"><GIcon.link size={15} />Sincronizar agora</button>
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: '1.5fr 1fr', gap: 18, alignItems: 'start' }}>
          {/* left */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18, minHeight: 0 }}>
            <GPanel title="Contas conectadas" sub={d.accounts.length + ''}>
              {d.accounts.map((a, i) => (
                <div key={a.id} style={{ display: 'flex', alignItems: 'center', gap: 13, padding: '13px 18px', borderTop: i ? '1px solid var(--border)' : 'none' }}>
                  <GMark size={32} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13.5, fontWeight: 540, display: 'flex', alignItems: 'center', gap: 8 }}>{a.email}{a.primary && <span className="tt-role tt-role--owner" style={{ padding: '1px 8px' }}>principal</span>}</div>
                    <div className="faint" style={{ fontSize: 12, marginTop: 1 }}>{a.events} eventos sincronizados hoje</div>
                  </div>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--accent)' }}><span className="tt-dot" style={{ width: 6, height: 6, background: 'var(--accent)' }} />ativo</span>
                  <GSwitch on />
                </div>
              ))}
              <div style={{ padding: '13px 18px', borderTop: '1px solid var(--border)' }}>
                <button className="tt-btn tt-btn--ghost"><GIcon.plus size={15} sw={2.2} />Conectar outra conta</button>
              </div>
            </GPanel>

            <GPanel title="Mapeamento de hoje" sub="evento ↔ entrada" right={<span className="faint" style={{ fontSize: 11.5 }}>{d.mappings.length} itens</span>}>
              {d.mappings.map((m, i) => (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '1fr 28px 1fr 130px', alignItems: 'center', gap: 12, padding: '12px 18px', borderTop: i ? '1px solid var(--border)' : 'none' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 9, minWidth: 0 }}>
                    <GMark size={20} />
                    <span style={{ fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', opacity: m.cal ? 1 : .4 }}>{m.cal ? m.event : '— sem evento —'}</span>
                  </div>
                  <GIcon.link size={15} style={{ color: 'var(--text-3)', justifySelf: 'center' }} />
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                    {m.p ? <><GDot id={m.p} size={8} /><span className="mono" style={{ fontSize: 13 }}>{m.entry || '—'}</span><span className="faint" style={{ fontSize: 12, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{gProj(m.p).name}</span></> : <span className="faint" style={{ fontSize: 12.5 }}>sem entrada</span>}
                  </div>
                  <div style={{ justifySelf: 'end' }}><MapState state={m.state} /></div>
                </div>
              ))}
            </GPanel>
          </div>

          {/* right */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 18 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 14 }}>
                <span style={{ width: 9, height: 9, borderRadius: '50%', background: 'var(--accent)' }} />
                <span style={{ fontSize: 14, fontWeight: 600 }}>Conectado</span>
              </div>
              <div style={{ display: 'flex', gap: 22 }}>
                <div><div className="faint" style={{ fontSize: 11.5 }}>Eventos hoje</div><div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 3 }}>22</div></div>
                <div><div className="faint" style={{ fontSize: 11.5 }}>Refletidos</div><div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 3, color: 'var(--accent)' }}>18</div></div>
                <div><div className="faint" style={{ fontSize: 11.5 }}>Conflitos</div><div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 3, color: 'var(--text-2)' }}>0</div></div>
              </div>
            </section>

            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 18 }}>
              <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 14 }}>Sincronização 2-way</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
                  <div><div style={{ fontSize: 13, fontWeight: 500 }}>GCal → Timetrack</div><div className="faint" style={{ fontSize: 11.5, marginTop: 2 }}>Eventos viram blocos planejados.</div></div><GSwitch on />
                </div>
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
                  <div><div style={{ fontSize: 13, fontWeight: 500 }}>Timetrack → GCal</div><div className="faint" style={{ fontSize: 11.5, marginTop: 2 }}>Tempo rastreado volta pro calendário.</div></div><GSwitch on />
                </div>
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
                  <div><div style={{ fontSize: 13, fontWeight: 500 }}>Ignorar eventos pessoais</div><div className="faint" style={{ fontSize: 11.5, marginTop: 2 }}>Almoço, pausas, etc.</div></div><GSwitch />
                </div>
              </div>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}

window.TT_Gcal = { Gcal };
