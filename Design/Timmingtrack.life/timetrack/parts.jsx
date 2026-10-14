/* Timetrack — shared UI primitives (exported to window.TT) */

// ---- icons ----
const _ic = (p, vb = 24) => (props) => (
  <svg viewBox={`0 0 ${vb} ${vb}`} width={props.size || 18} height={props.size || 18}
    fill="none" stroke="currentColor" strokeWidth={props.sw || 1.7}
    strokeLinecap="round" strokeLinejoin="round" style={props.style} className={props.className}>
    {p}
  </svg>
);
const Icon = {
  today:   _ic(<><rect x="3" y="4.5" width="18" height="16" rx="2.5"/><path d="M3 9h18M8 2.5v4M16 2.5v4"/><circle cx="12" cy="14.5" r="2" fill="currentColor" stroke="none"/></>),
  cal:     _ic(<><rect x="3" y="4.5" width="18" height="16" rx="2.5"/><path d="M3 9h18M8 2.5v4M16 2.5v4M7.5 13h2M11 13h2M14.5 13h2M7.5 16.5h2M11 16.5h2"/></>),
  list:    _ic(<><path d="M8 6h12M8 12h12M8 18h12M3.5 6h.01M3.5 12h.01M3.5 18h.01"/></>),
  folder:  _ic(<><path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></>),
  chart:   _ic(<><path d="M4 20V4M4 20h16M8 16v-4M12.5 16V9M17 16v-7"/></>),
  focus:   _ic(<><path d="M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16"/><circle cx="12" cy="12" r="2.5"/></>),
  cog:     _ic(<><circle cx="12" cy="12" r="3"/><path d="M12 2.5v2.5M12 19v2.5M4.2 7l2.1 1.3M17.7 15.7l2.1 1.3M19.8 7l-2.1 1.3M6.3 15.7L4.2 17M2.5 12H5M19 12h2.5"/></>),
  play:    _ic(<><path d="M7 5.5v13l11-6.5z" fill="currentColor" stroke="none"/></>),
  stop:    _ic(<><rect x="6.5" y="6.5" width="11" height="11" rx="2" fill="currentColor" stroke="none"/></>),
  pause:   _ic(<><rect x="7" y="6" width="3.2" height="12" rx="1.2" fill="currentColor" stroke="none"/><rect x="13.8" y="6" width="3.2" height="12" rx="1.2" fill="currentColor" stroke="none"/></>),
  plus:    _ic(<><path d="M12 5v14M5 12h14"/></>),
  search:  _ic(<><circle cx="11" cy="11" r="6.5"/><path d="M20 20l-4-4"/></>),
  clock:   _ic(<><circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/></>),
  bolt:    _ic(<><path d="M13 2.5 4.5 13.5H11l-1 8 8.5-11H12z"/></>),
  arrow:   _ic(<><path d="M5 12h14M13 6l6 6-6 6"/></>),
  cliente: _ic(<><circle cx="12" cy="8" r="3.5"/><path d="M5.5 20a6.5 6.5 0 0 1 13 0"/></>),
  briefcase: _ic(<><rect x="3" y="7" width="18" height="13" rx="2.5"/><path d="M8 7V5.5A1.5 1.5 0 0 1 9.5 4h5A1.5 1.5 0 0 1 16 5.5V7M3 12h18"/></>),
  users:   _ic(<><circle cx="9" cy="8" r="3"/><path d="M3.5 19a5.5 5.5 0 0 1 11 0M16 5.5a3 3 0 0 1 0 5.8M17 19a5.5 5.5 0 0 0-2.2-4.4"/></>),
  mail:    _ic(<><rect x="3" y="5" width="18" height="14" rx="2.5"/><path d="M4 7l8 6 8-6"/></>),
  link:    _ic(<><path d="M9 15l6-6M10.5 6.5l1.2-1.2a3.7 3.7 0 0 1 5.2 5.2L15.7 11.7M8.3 12.3l-1.2 1.2a3.7 3.7 0 0 0 5.2 5.2l1.2-1.2"/></>),
  shield:  _ic(<><path d="M12 3l7 2.5v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9v-5z"/><path d="M9 12l2 2 4-4"/></>),
  check:   _ic(<><path d="M4.5 12.5l5 5 10-11"/></>),
  x:       _ic(<><path d="M6 6l12 12M18 6L6 18"/></>),
  table:   _ic(<><rect x="3.5" y="4.5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M3.5 14.5h17M9.5 4.5v15"/></>),
  chevron: _ic(<><path d="M6 9l6 6 6-6"/></>),
  dots:    _ic(<><circle cx="5" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.4" fill="currentColor" stroke="none"/></>),
  trash:   _ic(<><path d="M4 7h16M9 7V5.5A1.5 1.5 0 0 1 10.5 4h3A1.5 1.5 0 0 1 15 5.5V7M6 7l1 13h10l1-13"/></>),
  globe:   _ic(<><circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.5 2.5 2.5 14.5 0 17M12 3.5c-2.5 2.5-2.5 14.5 0 17"/></>),
  grid:    _ic(<><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></>),
};

// ---- running clock hook ----
function useClock(startSec = 0, run = true) {
  const [s, setS] = React.useState(startSec);
  React.useEffect(() => {
    if (!run) return;
    const t = setInterval(() => setS(v => v + 1), 1000);
    return () => clearInterval(t);
  }, [run]);
  return s;
}
function fmt(sec, withH = true) {
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
  const pad = n => String(n).padStart(2, '0');
  return withH ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

const D = () => window.TT_DATA;
const proj = id => D().byId[id];

// ---- project chip ----
function Chip({ id, sub }) {
  const p = proj(id);
  return (
    <span className="tt-chip">
      <i className="tt-dot" style={{ background: p.color }} />
      {p.name}{sub && <span className="faint" style={{ fontWeight: 400 }}>· {sub}</span>}
    </span>
  );
}
function Dot({ id, size = 9 }) {
  return <i className="tt-dot" style={{ background: proj(id).color, width: size, height: size }} />;
}

// ---- nav rail ----
function NavRail({ active = 'today', compact = false, brand = true }) {
  const items = [
    ['today', 'Hoje', Icon.today], ['cal', 'Calendário', Icon.cal],
    ['proj', 'Projetos', Icon.folder], ['rep', 'Relatórios', Icon.chart],
    ['foco', 'Foco', Icon.focus],
  ];
  return (
    <nav style={{ display: 'flex', flexDirection: 'column', height: '100%', padding: 'var(--pad-sm)', gap: 3 }}>
      {brand && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '6px 10px 16px' }}>
          <span style={{ width: 22, height: 22, borderRadius: 7, background: 'var(--accent)', display: 'grid', placeItems: 'center' }}>
            <Icon.clock size={14} style={{ color: 'var(--accent-ink)' }} sw={2.2} />
          </span>
          <span style={{ fontWeight: 600, fontSize: 15, letterSpacing: '-.02em' }}>Timetrack</span>
        </div>
      )}
      {items.map(([k, label, I]) => (
        <div key={k} className="tt-nav" aria-current={active === k}>
          <I className="tt-navicon" />{!compact && label}
        </div>
      ))}
      <div style={{ flex: 1 }} />
      <div className="tt-nav"><Icon.cog className="tt-navicon" />{!compact && 'Ajustes'}</div>
    </nav>
  );
}

// ---- global timer bar (the hero) ----
function TimerBar({ variant = 'calm', startSec = 5070 }) {
  const sec = useClock(startSec, true);
  const e = D().activeEntry;
  const big = variant === 'hero';
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--gap)', width: '100%' }}>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 12 }}>
        <input
          defaultValue={e.task}
          style={{
            flex: 1, minWidth: 0, background: 'transparent', border: 'none', outline: 'none',
            color: 'var(--text-1)', fontFamily: 'var(--font-ui)', fontWeight: 500,
            fontSize: big ? 17 : 15, letterSpacing: '-.01em',
          }} />
        <Chip id={e.p} />
      </div>
      <div className="mono" style={{
        fontSize: big ? 34 : 22, fontWeight: 600, letterSpacing: '-.02em',
        color: 'var(--text-1)', fontVariantNumeric: 'tabular-nums', minWidth: big ? 160 : 116, textAlign: 'right',
      }}>{fmt(sec)}</div>
      <button className="tt-iconbtn tt-iconbtn--stop" title="Parar" style={big ? { width: 46, height: 46 } : null}>
        <Icon.stop size={big ? 18 : 15} />
      </button>
    </div>
  );
}

// ---- day calendar grid (planned vs real) ----
function CalendarGrid({ startH = 8, endH = 18, pxPerHour = 54, now = 16.6, dense = false }) {
  const hours = []; for (let h = startH; h <= endH; h++) hours.push(h);
  const top = h => (h - startH) * pxPerHour;
  const blocks = D().blocks;
  return (
    <div style={{ position: 'relative', height: (endH - startH) * pxPerHour + 8 }}>
      {/* hour lines */}
      {hours.map(h => (
        <div key={h} style={{ position: 'absolute', top: top(h), left: 0, right: 0, height: 1, background: 'var(--border)' }}>
          <span className="mono faint" style={{ position: 'absolute', left: 0, top: -7, fontSize: 11, background: 'var(--surface-1)', paddingRight: 6 }}>
            {String(h).padStart(2, '0')}:00
          </span>
        </div>
      ))}
      {/* blocks */}
      <div style={{ position: 'absolute', left: 46, right: 8, top: 0, bottom: 0 }}>
        {blocks.map((b, i) => {
          const planned = b.src === 'calendar';
          return (
            <div key={i} style={{
              position: 'absolute', top: top(b.h), height: b.len * pxPerHour - 4, left: 0, right: 0,
              borderRadius: 8, padding: dense ? '4px 9px' : '6px 11px', overflow: 'hidden',
              background: planned ? 'transparent' : `color-mix(in srgb, ${proj(b.p).color} 18%, var(--surface-2))`,
              border: planned ? `1.5px dashed color-mix(in srgb, ${proj(b.p).color} 55%, var(--border-2))`
                              : `1px solid color-mix(in srgb, ${proj(b.p).color} 35%, transparent)`,
              borderLeft: `3px solid ${proj(b.p).color}`,
            }}>
              <div style={{ fontSize: dense ? 12 : 13, fontWeight: 540, color: 'var(--text-1)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {b.task}
              </div>
              {b.len > 0.7 && (
                <div className="muted" style={{ fontSize: 11, marginTop: 2, display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Dot id={b.p} size={7} />{proj(b.p).name}
                  {planned && <span className="faint" style={{ fontStyle: 'italic' }}>· planejado</span>}
                </div>
              )}
            </div>
          );
        })}
        {/* now line */}
        <div style={{ position: 'absolute', top: top(now), left: -6, right: 0, height: 2, background: 'var(--accent)', zIndex: 3 }}>
          <span style={{ position: 'absolute', left: -6, top: -4, width: 9, height: 9, borderRadius: '50%', background: 'var(--accent)' }} />
        </div>
      </div>
    </div>
  );
}

// ---- timesheet list ----
function Timesheet({ dense = false, max = 99 }) {
  const rows = D().entries.slice(0, max);
  return (
    <div>
      {rows.map((r, i) => (
        <div key={i} style={{
          display: 'flex', alignItems: 'center', gap: 12, height: 'var(--row-h)',
          padding: '0 4px', borderBottom: '1px solid var(--border)',
        }}>
          <Dot id={r.p} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 'var(--fs-row)', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{r.task}</div>
            {!dense && <div className="faint" style={{ fontSize: 11.5 }}>{proj(r.p).name}</div>}
          </div>
          <span className="mono faint" style={{ fontSize: 12 }}>{r.start}</span>
          <span className="mono" style={{ fontSize: 'var(--fs-row)', fontWeight: 540, minWidth: 78, textAlign: 'right', color: r.running ? 'var(--accent)' : 'var(--text-1)' }}>
            {r.dur}
          </span>
          <button className="tt-iconbtn" style={{ width: 28, height: 28, background: 'transparent', color: r.running ? 'var(--accent)' : 'var(--text-3)' }}>
            {r.running ? <Icon.stop size={13} /> : <Icon.play size={13} />}
          </button>
        </div>
      ))}
    </div>
  );
}

// ---- goal bar row ----
function GoalRow({ id }) {
  const p = proj(id);
  const pct = Math.min(100, Math.round((p.done / p.goal) * 100));
  const over = p.done > p.goal;
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
      <Dot id={id} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 5 }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>{p.name}</span>
          <span className="mono faint" style={{ fontSize: 11.5 }}>{p.done}/{p.goal}h</span>
        </div>
        <div className="tt-goal">
          <i style={{ width: pct + '%', background: over ? 'var(--accent)' : p.color, opacity: over ? 1 : .85 }} />
        </div>
      </div>
    </div>
  );
}

// ---- hover-expand sidebar (canonical nav) ----
function SideNav({ active = 'today' }) {
  const items = [
    ['today', 'Hoje', Icon.today], ['cal', 'Calendário', Icon.cal],
    ['proj', 'Projetos', Icon.folder], ['clients', 'Clientes', Icon.briefcase],
    ['rep', 'Relatórios', Icon.chart], ['ws', 'Workspace', Icon.users],
    ['foco', 'Foco', Icon.focus],
  ];
  return (
    <aside className="tt-sidenav">
      <div className="tt-sidenav-inner">
        <div style={{ display: 'flex', alignItems: 'center', gap: 11, padding: '6px 8px 16px' }}>
          <span style={{ width: 24, height: 24, borderRadius: 7, background: 'var(--accent)', display: 'grid', placeItems: 'center', flex: '0 0 auto' }}>
            <Icon.clock size={15} style={{ color: 'var(--accent-ink)' }} sw={2.2} />
          </span>
          <span className="tt-brandname" style={{ fontWeight: 600, fontSize: 15, letterSpacing: '-.02em' }}>Timetrack</span>
        </div>
        {items.map(([k, label, I]) => (
          <div key={k} className="tt-nav" aria-current={active === k}>
            <I className="tt-navicon" /><span className="tt-navlabel">{label}</span>
          </div>
        ))}
        <div style={{ flex: 1 }} />
        <div className="tt-nav" aria-current={active === 'cog'}><Icon.cog className="tt-navicon" /><span className="tt-navlabel">Ajustes</span></div>
      </div>
    </aside>
  );
}

// ---- avatar ----
function Avatar({ m, size = 30 }) {
  return <span className="tt-avatar" style={{ width: size, height: size, fontSize: size * .38, background: m.color, color: 'var(--accent-ink)' }}>{m.initials}</span>;
}

// ---- toggle switch ----
function Switch({ on = false }) {
  return <div className="tt-switch" role="switch" aria-checked={on}><i /></div>;
}

window.TT = { Icon, useClock, fmt, proj, Chip, Dot, NavRail, SideNav, Avatar, Switch, TimerBar, CalendarGrid, Timesheet, GoalRow };
