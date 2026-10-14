/* Timetrack — Clientes (Toggl-style: cards + tabela on-demand) */
const { Icon: ClIcon, SideNav: ClSide, Dot: ClDot, proj: clProj } = window.TT;
const ClD = () => window.TT_DATA;

function statusPill(status) {
  const map = { Ativo: ['var(--accent)', 'var(--accent)'], Interno: ['var(--text-3)', 'var(--text-2)'], Pausado: ['var(--text-3)', 'var(--text-3)'] };
  const [dot, col] = map[status] || map.Ativo;
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11.5, fontWeight: 540, padding: '3px 10px', borderRadius: 999, border: '1px solid var(--border)', color: col }}>
      <i className="tt-dot" style={{ width: 6, height: 6, background: dot }} />{status}
    </span>
  );
}

function ClientCard({ c }) {
  return (
    <div className="tt-card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14, background: 'var(--surface-1)' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        <span style={{ width: 38, height: 38, borderRadius: 11, background: c.color, flex: '0 0 auto', display: 'grid', placeItems: 'center', color: 'var(--accent-ink)', fontWeight: 700, fontSize: 15 }}>{c.name[0]}</span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{c.name}</div>
          <div className="faint" style={{ fontSize: 12, marginTop: 1, display: 'flex', alignItems: 'center', gap: 5 }}><ClIcon.cliente size={12} />{c.contact}</div>
        </div>
        {statusPill(c.status)}
      </div>
      <div style={{ display: 'flex', gap: 16 }}>
        <div><div className="faint" style={{ fontSize: 11, whiteSpace: 'nowrap' }}>Semana</div><div className="mono" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>{c.week}h</div></div>
        <div><div className="faint" style={{ fontSize: 11, whiteSpace: 'nowrap' }}>Total</div><div className="mono" style={{ fontSize: 16, fontWeight: 600, marginTop: 2, color: 'var(--text-2)' }}>{c.total}h</div></div>
        <div><div className="faint" style={{ fontSize: 11, whiteSpace: 'nowrap' }}>Valor/h</div><div className="mono" style={{ fontSize: 16, fontWeight: 600, marginTop: 2, color: 'var(--text-2)' }}>{c.rate ? 'R$' + c.rate : '—'}</div></div>
      </div>
      <div style={{ borderTop: '1px solid var(--border)', paddingTop: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
        <span className="faint" style={{ fontSize: 11.5 }}>{c.projs.length} projeto{c.projs.length > 1 ? 's' : ''}</span>
        <div style={{ display: 'flex', gap: 6, flex: 1 }}>
          {c.projs.map(pid => (
            <span key={pid} className="tt-chip" style={{ background: 'var(--surface-2)' }}><ClDot id={pid} size={7} />{clProj(pid).name}</span>
          ))}
        </div>
        <ClIcon.arrow size={15} style={{ color: 'var(--text-3)' }} />
      </div>
    </div>
  );
}

function ClientsTable() {
  return (
    <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto', height: '100%' }}>
      <table className="tt-table">
        <thead><tr>
          <th>Cliente</th><th>Contato</th><th>Projetos</th><th className="num">Esta semana</th><th className="num">Total</th><th className="num">Valor/h</th><th>Status</th>
        </tr></thead>
        <tbody>
          {ClD().clients.map(c => (
            <tr key={c.id}>
              <td><span style={{ display: 'flex', alignItems: 'center', gap: 9, fontWeight: 540 }}><span style={{ width: 10, height: 10, borderRadius: 3, background: c.color }} />{c.name}</span></td>
              <td className="muted">{c.contact}</td>
              <td className="muted">{c.projs.map(p => clProj(p).name).join(', ')}</td>
              <td className="num">{c.week}h</td>
              <td className="num muted">{c.total}h</td>
              <td className="num muted">{c.rate ? 'R$' + c.rate : '—'}</td>
              <td>{statusPill(c.status)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Clients() {
  const [view, setView] = React.useState('cards');
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <ClSide active="clients" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 26, gap: 20 }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow" style={{ whiteSpace: 'nowrap' }}>{ClD().clients.length} clientes · 38,6h esta semana</div>
            <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>Clientes</h1>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 9, padding: 2, gap: 2 }}>
              {[['cards', 'Cartões', ClIcon.grid], ['table', 'Tabela', ClIcon.table]].map(([v, l, I]) => (
                <button key={v} onClick={() => setView(v)} style={{
                  border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540,
                  padding: '6px 12px', borderRadius: 7, display: 'flex', alignItems: 'center', gap: 6,
                  background: view === v ? 'var(--surface-1)' : 'transparent', color: view === v ? 'var(--text-1)' : 'var(--text-2)',
                }}><I size={14} />{l}</button>
              ))}
            </div>
            <button className="tt-btn tt-btn--ghost"><ClIcon.search size={15} />Buscar</button>
            <button className="tt-btn tt-btn--accent"><ClIcon.plus size={15} sw={2.2} />Novo cliente</button>
          </div>
        </div>
        {view === 'cards' ? (
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gridAutoRows: 'min-content', gap: 16, alignContent: 'start' }}>
            {ClD().clients.map(c => <ClientCard key={c.id} c={c} />)}
          </div>
        ) : (
          <div style={{ flex: 1, minHeight: 0 }}><ClientsTable /></div>
        )}
      </main>
    </div>
  );
}

window.TT_Clients = { Clients };
