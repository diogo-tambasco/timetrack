/* Timetrack — Novo projeto (com atribuição de membros específicos) */
const { Icon: NpIcon, Avatar: NpAvatar } = window.TT;
const NpD = () => window.TT_DATA;
const NP_COLORS = ['var(--p-acme)', 'var(--p-lauda)', 'var(--p-arkan)', 'var(--p-primor)', 'var(--p-freela)', 'var(--accent)'];

function NpField({ label, children, hint }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
      <label style={{ fontSize: 12.5, fontWeight: 540, color: 'var(--text-2)' }}>{label}</label>
      {children}
      {hint && <span className="faint" style={{ fontSize: 11.5 }}>{hint}</span>}
    </div>
  );
}
const npInput = { background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-md)', padding: '11px 13px', fontSize: 14, color: 'var(--text-1)', fontFamily: 'var(--font-ui)', outline: 'none', width: '100%', boxSizing: 'border-box' };

function NewProject() {
  const [color, setColor] = React.useState('var(--p-acme)');
  const [vis, setVis] = React.useState('shared');
  const members = NpD().members;
  const [inc, setInc] = React.useState({ carlos: true, kadu: true });
  const [roleByM, setRole] = React.useState({ carlos: 'edit', kadu: 'view' });
  const toggle = id => setInc(s => ({ ...s, [id]: !s[id] }));

  return (
    <div className="tt" style={{ width: '100%', height: '100%', position: 'relative', background: 'var(--bg)', display: 'grid', placeItems: 'center', overflow: 'hidden' }}>
      {/* dim backdrop hint */}
      <div style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(2px)' }} />
      <div className="tt-card" style={{ position: 'relative', width: 640, maxHeight: '92%', background: 'var(--surface-1)', boxShadow: 'var(--shadow)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {/* header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '18px 22px', borderBottom: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
            <span style={{ width: 30, height: 30, borderRadius: 9, background: color, transition: 'background .2s' }} />
            <div style={{ fontSize: 17, fontWeight: 600 }}>Novo projeto</div>
          </div>
          <button className="tt-iconbtn" style={{ width: 32, height: 32, background: 'transparent', color: 'var(--text-3)' }}><NpIcon.x size={16} /></button>
        </div>

        {/* body */}
        <div style={{ padding: 22, display: 'flex', flexDirection: 'column', gap: 18, overflowY: 'auto' }}>
          <NpField label="Nome do projeto">
            <div style={{ ...npInput, color: 'var(--text-1)' }}>Acme — App mobile</div>
          </NpField>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 16 }}>
            <NpField label="Cliente">
              <div style={{ ...npInput, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>Acme<NpIcon.chevron size={15} style={{ color: 'var(--text-3)' }} /></div>
            </NpField>
            <NpField label="Cor">
              <div style={{ display: 'flex', gap: 8, paddingTop: 2 }}>
                {NP_COLORS.map(c => (
                  <button key={c} onClick={() => setColor(c)} style={{ width: 26, height: 26, borderRadius: 8, cursor: 'pointer', background: c, border: '2px solid', borderColor: color === c ? 'var(--text-1)' : 'transparent' }} />
                ))}
              </div>
            </NpField>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <NpField label="Visibilidade" hint={vis === 'shared' ? 'Membros do workspace podem ver e rastrear.' : 'Só você vê este projeto.'}>
              <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 9, padding: 2, gap: 2 }}>
                {[['private', 'Privado'], ['shared', 'Compartilhado']].map(([v, l]) => (
                  <button key={v} onClick={() => setVis(v)} style={{ flex: 1, border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540, padding: '8px 0', borderRadius: 6, background: vis === v ? 'var(--surface-1)' : 'transparent', color: vis === v ? 'var(--text-1)' : 'var(--text-2)' }}>{l}</button>
                ))}
              </div>
            </NpField>
            <NpField label="Meta de horas / semana">
              <div style={{ ...npInput, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}><span className="mono">18</span><span className="faint" style={{ fontSize: 12 }}>h/sem</span></div>
            </NpField>
          </div>

          {/* members assignment */}
          <NpField label="Membros do projeto" hint={vis === 'shared' ? 'Escolha quem participa e o que cada um pode fazer.' : 'Disponível em projetos compartilhados.'}>
            <div style={{ border: '1px solid var(--border)', borderRadius: 'var(--r-md)', overflow: 'hidden', opacity: vis === 'shared' ? 1 : .5, pointerEvents: vis === 'shared' ? 'auto' : 'none' }}>
              {members.map((m, i) => {
                const owner = m.role === 'Owner';
                const on = owner || inc[m.id];
                return (
                  <div key={m.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 13px', borderTop: i ? '1px solid var(--border)' : 'none', background: on ? 'transparent' : 'var(--surface-1)' }}>
                    <button onClick={() => !owner && toggle(m.id)} style={{
                      width: 20, height: 20, borderRadius: 6, flex: '0 0 auto', cursor: owner ? 'default' : 'pointer', display: 'grid', placeItems: 'center',
                      border: '1.5px solid', borderColor: on ? 'var(--accent)' : 'var(--border-2)', background: on ? 'var(--accent)' : 'transparent', color: 'var(--accent-ink)',
                    }}>{on && <NpIcon.check size={13} sw={2.4} />}</button>
                    <NpAvatar m={m} size={30} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 13.5, fontWeight: 540 }}>{m.name}</div>
                      <div className="faint" style={{ fontSize: 11.5 }}>{m.email}</div>
                    </div>
                    {owner ? (
                      <span className="tt-role tt-role--owner"><NpIcon.shield size={12} />Owner</span>
                    ) : on ? (
                      <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 8, padding: 2, gap: 2 }}>
                        {[['edit', 'Editar'], ['view', 'Só vê']].map(([v, l]) => (
                          <button key={v} onClick={() => setRole(s => ({ ...s, [m.id]: v }))} style={{ border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 11.5, fontWeight: 540, whiteSpace: 'nowrap', padding: '4px 10px', borderRadius: 6, background: roleByM[m.id] === v ? 'var(--surface-1)' : 'transparent', color: roleByM[m.id] === v ? 'var(--text-1)' : 'var(--text-3)' }}>{l}</button>
                        ))}
                      </div>
                    ) : (
                      <span className="faint" style={{ fontSize: 12 }}>não incluído</span>
                    )}
                  </div>
                );
              })}
            </div>
          </NpField>
        </div>

        {/* footer */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '16px 22px', borderTop: '1px solid var(--border)' }}>
          <span className="faint" style={{ fontSize: 12 }}>{vis === 'shared' ? `${1 + Object.values(inc).filter(Boolean).length} membros` : 'Privado'}</span>
          <div style={{ display: 'flex', gap: 10 }}>
            <button className="tt-btn tt-btn--ghost">Cancelar</button>
            <button className="tt-btn tt-btn--accent"><NpIcon.plus size={15} sw={2.2} />Criar projeto</button>
          </div>
        </div>
      </div>
    </div>
  );
}

window.TT_NewProject = { NewProject };
