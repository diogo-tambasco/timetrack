/* Timetrack — Projetos (minimal, Toggl-style list) */
const { Icon: PrIcon, NavRail: PrNav, SideNav } = window.TT;

function ProjRow({ p }) {
  const pct = Math.min(100, Math.round(p.done / p.goal * 100));
  const paused = p.status === 'Pausado';
  return (
    <div style={{
      display: 'grid', gridTemplateColumns: '2.4fr 1fr 1.5fr 78px 92px', alignItems: 'center', gap: 16,
      padding: '0 14px', height: 60, borderBottom: '1px solid var(--border)', opacity: paused ? .62 : 1,
    }}>
      {/* name + color rule */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
        <span style={{ width: 4, height: 30, borderRadius: 3, background: p.color, flex: '0 0 auto' }} />
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ fontSize: 14, fontWeight: 540, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.name}</div>
          <div className="faint" style={{ fontSize: 11.5, marginTop: 1 }}>{p.tasks} tasks</div>
        </div>
      </div>
      {/* client */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13, color: 'var(--text-2)' }}>
        <PrIcon.cliente size={14} style={{ color: 'var(--text-3)' }} />{p.client}
      </div>
      {/* this week vs goal */}
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6, whiteSpace: 'nowrap' }}>
          <span className="mono" style={{ fontSize: 12, fontWeight: 540 }}>{p.done}h</span>
          <span className="mono faint" style={{ fontSize: 11.5 }}>meta {p.goal}h</span>
        </div>
        <div className="tt-goal"><i style={{ width: pct + '%', background: pct >= 100 ? 'var(--accent)' : p.color }} /></div>
      </div>
      {/* total */}
      <div className="mono" style={{ fontSize: 13, textAlign: 'right', color: 'var(--text-2)' }}>{p.total}h</div>
      {/* status */}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <span style={{
          display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11.5, fontWeight: 540,
          padding: '3px 10px', borderRadius: 999, border: '1px solid var(--border)',
          color: paused ? 'var(--text-3)' : 'var(--text-2)',
        }}>
          <i className="tt-dot" style={{ width: 6, height: 6, background: paused ? 'var(--text-3)' : 'var(--accent)' }} />{p.status}
        </span>
      </div>
    </div>
  );
}

function Projects() {
  const D = window.TT_DATA;
  const clients = [...new Set(D.projects.map(p => p.client))].length;
  const totalWeek = D.projects.reduce((s, p) => s + p.done, 0).toFixed(1);
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <SideNav active="proj" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 26, gap: 20 }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow" style={{ whiteSpace: 'nowrap' }}>{D.projects.length} projetos · {clients} clientes</div>
            <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>Projetos</h1>
          </div>
          <div style={{ display: 'flex', gap: 10 }}>
            <button className="tt-btn tt-btn--ghost"><PrIcon.search size={15} />Buscar</button>
            <button className="tt-btn tt-btn--accent"><PrIcon.plus size={15} sw={2.2} />Novo projeto</button>
          </div>
        </div>

        <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'hidden' }}>
          {/* column header */}
          <div style={{ display: 'grid', gridTemplateColumns: '2.4fr 1fr 1.5fr 78px 92px', gap: 16, padding: '11px 14px', borderBottom: '1px solid var(--border)', background: 'var(--surface-1)' }}>
            {['Projeto', 'Cliente', 'Esta semana', 'Total', 'Status'].map((h, i) => (
              <span key={h} className="eyebrow" style={{ textAlign: i >= 3 ? 'right' : 'left' }}>{h}</span>
            ))}
          </div>
          {D.projects.map(p => <ProjRow key={p.id} p={p} />)}
        </section>

        <div style={{ display: 'flex', gap: 22, padding: '0 4px', color: 'var(--text-2)', fontSize: 13 }}>
          <span>Total esta semana <b className="mono" style={{ color: 'var(--text-1)', fontWeight: 600, marginLeft: 4 }}>{totalWeek}h</b></span>
          <span>Projetos ativos <b className="mono" style={{ color: 'var(--text-1)', fontWeight: 600, marginLeft: 4 }}>{D.projects.filter(p => p.status === 'Ativo').length}</b></span>
        </div>
      </main>
    </div>
  );
}

window.TT_Projects = { Projects };
