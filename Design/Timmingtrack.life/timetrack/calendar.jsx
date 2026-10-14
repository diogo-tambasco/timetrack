/* Timetrack — Calendário (week grid + calendar/timesheet split) */
const { Icon: CIcon, NavRail: CNav, SideNav, Timesheet: CTimesheet, proj: cProj } = window.TT;

/* ---- week grid core ---- */
function WeekGrid({ dayIdxs, startH = 7, endH = 19, pph = 58, todayIdx = 1, now = 16.6 }) {
  const D = window.TT_DATA;
  const days = dayIdxs.map(i => ({ ...D.weekDays[i], idx: i }));
  const hours = []; for (let h = startH; h <= endH; h++) hours.push(h);
  const top = h => (h - startH) * pph;
  const colTmpl = `52px repeat(${days.length}, 1fr)`;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      {/* day header */}
      <div style={{ display: 'grid', gridTemplateColumns: colTmpl, borderBottom: '1px solid var(--border)', flex: '0 0 auto' }}>
        <div />
        {days.map(d => {
          const isToday = d.idx === todayIdx;
          return (
            <div key={d.idx} style={{ padding: '10px 0 11px', textAlign: 'center', borderLeft: '1px solid var(--border)' }}>
              <div className="eyebrow" style={{ color: isToday ? 'var(--accent)' : 'var(--text-3)', letterSpacing: '.1em' }}>{d.wd}</div>
              <div style={{
                marginTop: 5, fontSize: 18, fontWeight: 600, fontFamily: 'var(--font-mono)',
                width: 32, height: 32, lineHeight: '32px', borderRadius: '50%', display: 'inline-block',
                color: isToday ? 'var(--accent-ink)' : 'var(--text-1)', background: isToday ? 'var(--accent)' : 'transparent',
              }}>{d.date}</div>
            </div>
          );
        })}
      </div>
      {/* time grid */}
      <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
        <div style={{ position: 'relative', height: (endH - startH) * pph }}>
          {/* hour lines + labels */}
          {hours.map(h => (
            <div key={h} style={{ position: 'absolute', top: top(h), left: 0, right: 0, height: 1, background: 'var(--border)' }}>
              <span className="mono faint" style={{ position: 'absolute', left: 8, top: -8, fontSize: 11 }}>{String(h).padStart(2, '0')}:00</span>
            </div>
          ))}
          {/* columns */}
          <div style={{ position: 'absolute', inset: 0, display: 'grid', gridTemplateColumns: colTmpl }}>
            <div />
            {days.map(d => {
              const evs = D.weekEvents.filter(e => e.day === d.idx);
              return (
                <div key={d.idx} style={{ position: 'relative', borderLeft: '1px solid var(--border)' }}>
                  {evs.map((e, i) => {
                    const planned = e.planned;
                    return (
                      <div key={i} style={{
                        position: 'absolute', top: top(e.h) + 1, height: e.len * pph - 3, left: 3, right: 3,
                        borderRadius: 7, padding: '5px 8px', overflow: 'hidden',
                        background: planned ? 'transparent' : `color-mix(in srgb, ${cProj(e.p).color} 17%, var(--surface-2))`,
                        border: planned ? `1.4px dashed color-mix(in srgb, ${cProj(e.p).color} 50%, var(--border-2))`
                                        : `1px solid color-mix(in srgb, ${cProj(e.p).color} 30%, transparent)`,
                        borderLeft: `3px solid ${cProj(e.p).color}`,
                      }}>
                        <div style={{ fontSize: 11.5, fontWeight: 540, lineHeight: 1.25, color: 'var(--text-1)', overflow: 'hidden', textOverflow: 'ellipsis', display: '-webkit-box', WebkitLineClamp: e.len > 1 ? 2 : 1, WebkitBoxOrient: 'vertical' }}>{e.task}</div>
                        {e.len >= 1.5 && <div className="faint" style={{ fontSize: 10.5, marginTop: 3, fontStyle: planned ? 'italic' : 'normal' }}>{planned ? 'planejado' : cProj(e.p).name}</div>}
                      </div>
                    );
                  })}
                  {d.idx === todayIdx && (
                    <div style={{ position: 'absolute', top: top(now), left: -1, right: 0, height: 2, background: 'var(--accent)', zIndex: 4 }}>
                      <span style={{ position: 'absolute', left: -4, top: -4, width: 9, height: 9, borderRadius: '50%', background: 'var(--accent)' }} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

function CalHeader({ title, range, right }) {
  return (
    <header style={{ height: 62, flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 22px', borderBottom: '1px solid var(--border)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
        <h1 style={{ margin: 0, fontSize: 20, fontWeight: 600, letterSpacing: '-.02em' }}>{title}</h1>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <button className="tt-btn tt-btn--ghost" style={{ padding: 7 }}><CIcon.arrow size={15} style={{ transform: 'rotate(180deg)' }} /></button>
          <button className="tt-btn tt-btn--ghost" style={{ padding: '7px 12px', fontWeight: 540 }}>Hoje</button>
          <button className="tt-btn tt-btn--ghost" style={{ padding: 7 }}><CIcon.arrow size={15} /></button>
          <span className="mono" style={{ fontSize: 14, fontWeight: 540, marginLeft: 8, whiteSpace: 'nowrap' }}>{range}</span>
        </div>
      </div>
      {right}
    </header>
  );
}
function ViewToggle({ sel = 1 }) {
  return (
    <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 8, padding: 2, gap: 2 }}>
      {['Dia', 'Semana', 'Mês'].map((t, i) => (
        <span key={t} style={{ fontSize: 12.5, fontWeight: 540, padding: '5px 13px', borderRadius: 6, cursor: 'pointer', background: sel === i ? 'var(--surface-1)' : 'transparent', color: sel === i ? 'var(--text-1)' : 'var(--text-2)' }}>{t}</span>
      ))}
    </div>
  );
}

/* full week, Google-Calendar-ish */
function WeekCalendar() {
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <SideNav active="cal" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', background: 'var(--surface-1)' }}>
        <CalHeader title="Calendário" range="9 – 15 jun" right={
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <ViewToggle sel={1} />
            <button className="tt-btn tt-btn--accent"><CIcon.plus size={15} sw={2.2} />Entrada</button>
          </div>
        } />
        <div style={{ flex: 1, minHeight: 0, padding: '6px 14px 14px' }}><WeekGrid dayIdxs={[0, 1, 2, 3, 4, 5, 6]} /></div>
      </main>
    </div>
  );
}

/* Toggl-2.0-style split: calendar (3 dias) + timesheet do dia */
function CalSplit() {
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <SideNav active="cal" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <CalHeader title="Calendário" range="10 – 12 jun" right={
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 8, padding: 2, gap: 2 }}>
              <span style={{ fontSize: 12.5, fontWeight: 540, padding: '5px 13px', borderRadius: 6, color: 'var(--text-2)' }}>Calendário</span>
              <span style={{ fontSize: 12.5, fontWeight: 540, padding: '5px 13px', borderRadius: 6, background: 'var(--surface-1)', color: 'var(--text-1)' }}>Split</span>
            </div>
            <button className="tt-btn tt-btn--accent"><CIcon.plus size={15} sw={2.2} />Entrada</button>
          </div>
        } />
        <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <div style={{ flex: 1, minWidth: 0, padding: '6px 12px 12px', background: 'var(--surface-1)' }}>
            <WeekGrid dayIdxs={[1, 2, 3]} pph={56} />
          </div>
          <div style={{ width: 348, flex: '0 0 auto', borderLeft: '1px solid var(--border)', display: 'flex', flexDirection: 'column', background: 'var(--bg)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '15px 18px', borderBottom: '1px solid var(--border)' }}>
              <div><div style={{ fontSize: 14, fontWeight: 600 }}>Timesheet</div><div className="faint" style={{ fontSize: 12, marginTop: 1 }}>Ter, 10 jun</div></div>
              <span className="mono" style={{ fontSize: 15, fontWeight: 600 }}>{window.TT_DATA.totalToday}</span>
            </div>
            <div style={{ padding: '6px 16px', overflowY: 'auto', flex: 1 }}><CTimesheet /></div>
            <div style={{ padding: '12px 18px', borderTop: '1px solid var(--border)' }}>
              <button className="tt-btn" style={{ width: '100%', justifyContent: 'center' }}><CIcon.plus size={15} sw={2.2} />Entrada manual</button>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

window.TT_Calendar = { WeekCalendar, CalSplit };
