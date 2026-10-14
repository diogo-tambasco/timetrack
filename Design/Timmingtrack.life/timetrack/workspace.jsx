/* Timetrack — Workspace: membros, papéis, convites */
const { Icon: WIcon, SideNav: WSide, Avatar: WAvatar, Switch: WSwitch, Dot: WDot, proj: wProj } = window.TT;
const WD = () => window.TT_DATA;

function WPanel({ title, sub, right, children, pad = 0 }) {
  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', borderBottom: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
          <span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span>
          {sub && <span className="faint" style={{ fontSize: 12 }}>{sub}</span>}
        </div>
        {right}
      </div>
      <div style={{ padding: pad }}>{children}</div>
    </section>
  );
}

function RoleBadge({ role }) {
  const owner = role === 'Owner';
  return (
    <span className={'tt-role' + (owner ? ' tt-role--owner' : '')}>
      {owner && <WIcon.shield size={12} />}{role}{!owner && <WIcon.chevron size={12} style={{ opacity: .6 }} />}
    </span>
  );
}

function MemberRow({ m }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 13, padding: '0 18px', height: 64, borderBottom: '1px solid var(--border)' }}>
      <WAvatar m={m} size={36} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 14, fontWeight: 540 }}>{m.name}</div>
        <div className="faint" style={{ fontSize: 12 }}>{m.email}</div>
      </div>
      <div style={{ width: 220, minWidth: 0 }}>
        {m.running ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="tt-pulse" style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--accent)', flex: '0 0 auto' }} />
            <span style={{ fontSize: 12.5, color: 'var(--text-1)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.running}</span>
          </div>
        ) : (
          <span className="faint" style={{ fontSize: 12.5 }}>ativo {m.active}</span>
        )}
      </div>
      <RoleBadge role={m.role} />
      <button className="tt-iconbtn" style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}><WIcon.dots size={16} /></button>
    </div>
  );
}

function Workspace() {
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <WSide active="ws" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 26, gap: 20, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow">Workspace</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 4 }}>
              <h1 style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>Primor Labs</h1>
              <button className="tt-btn tt-btn--ghost" style={{ padding: '5px 8px' }}><WIcon.chevron size={16} /></button>
            </div>
            <p className="faint" style={{ fontSize: 13, marginTop: 6 }}>4 membros · 1 convite pendente · plano interno</p>
          </div>
          <button className="tt-btn tt-btn--accent"><WIcon.mail size={15} />Convidar pessoas</button>
        </div>

        <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: '1.7fr 1fr', gap: 18, alignItems: 'start' }}>
          {/* left: members + invites */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18, minHeight: 0 }}>
            <WPanel title="Membros" sub={WD().members.length + ' pessoas'} right={<button className="tt-btn tt-btn--ghost" style={{ padding: '5px 9px' }}><WIcon.search size={15} /></button>}>
              {WD().members.map(m => <MemberRow key={m.id} m={m} />)}
            </WPanel>

            <WPanel title="Convites pendentes" sub={WD().invites.length + ''} pad={0}>
              {WD().invites.map((inv, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 13, padding: '0 18px', height: 58 }}>
                  <span style={{ width: 36, height: 36, borderRadius: '50%', border: '1.5px dashed var(--border-2)', display: 'grid', placeItems: 'center', color: 'var(--text-3)' }}><WIcon.mail size={16} /></span>
                  <div style={{ flex: 1 }}><div style={{ fontSize: 13.5, fontWeight: 540 }}>{inv.email}</div><div className="faint" style={{ fontSize: 12 }}>enviado {inv.sent}</div></div>
                  <span className="tt-role">{inv.role}</span>
                  <button className="tt-btn tt-btn--ghost" style={{ padding: '6px 10px', fontSize: 12.5 }}>Reenviar</button>
                  <button className="tt-iconbtn" style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}><WIcon.x size={15} /></button>
                </div>
              ))}
            </WPanel>
          </div>

          {/* right: invite + roles + visibility */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 18 }}>
              <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 4 }}>Convidar pessoas</div>
              <div className="faint" style={{ fontSize: 12, marginBottom: 14 }}>Cada um conecta o próprio Google ao entrar.</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '10px 12px', borderRadius: 'var(--r-md)', border: '1px solid var(--border-2)', background: 'var(--surface-2)', marginBottom: 12 }}>
                <WIcon.mail size={15} style={{ color: 'var(--text-3)' }} />
                <span style={{ flex: 1, fontSize: 13, color: 'var(--text-3)' }}>email@empresa.com</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 9, padding: 2, gap: 2, flex: 1 }}>
                  {['Membro', 'Viewer'].map((r, i) => (
                    <span key={r} style={{ flex: 1, textAlign: 'center', fontSize: 12.5, fontWeight: 540, padding: '7px 0', borderRadius: 6, cursor: 'pointer', background: i === 0 ? 'var(--surface-1)' : 'transparent', color: i === 0 ? 'var(--text-1)' : 'var(--text-2)' }}>{r}</span>
                  ))}
                </div>
                <button className="tt-btn tt-btn--accent">Enviar convite</button>
              </div>
            </section>

            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 18 }}>
              <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 14 }}>Papéis & permissões</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 13 }}>
                {WD().roles.map(r => (
                  <div key={r.id} style={{ display: 'flex', gap: 11 }}>
                    <WIcon.shield size={16} style={{ color: r.id === 'Owner' ? 'var(--accent)' : 'var(--text-3)', flex: '0 0 auto', marginTop: 1 }} />
                    <div><div style={{ fontSize: 13, fontWeight: 600 }}>{r.id}</div><div className="faint" style={{ fontSize: 12, marginTop: 2, lineHeight: 1.4 }}>{r.desc}</div></div>
                  </div>
                ))}
              </div>
            </section>

            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 18 }}>
              <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 14 }}>Visibilidade padrão</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
                  <span style={{ fontSize: 13 }}>Projetos novos são compartilhados</span><WSwitch on />
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
                  <span style={{ fontSize: 13 }}>Ver quem está rodando agora</span><WSwitch on />
                </div>
              </div>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}

window.TT_Workspace = { Workspace };
