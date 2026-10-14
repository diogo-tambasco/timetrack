/* Timetrack — Cockpit: three visual directions */
const { Icon, NavRail, SideNav, TimerBar, CalendarGrid, Timesheet, GoalRow, Chip, Dot, useClock, fmt, proj } = window.TT;

function Frame({ children }) {
  return <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>{children}</div>;
}
function PanelHead({ title, right, sub }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderBottom: '1px solid var(--border)' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
        <span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span>
        {sub && <span className="faint" style={{ fontSize: 12 }}>{sub}</span>}
      </div>
      {right}
    </div>
  );
}
function SegToggle({ a, b, sel = 0 }) {
  return (
    <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 8, padding: 2, gap: 2 }}>
      {[a, b].map((t, i) => (
        <span key={t} style={{
          fontSize: 12, fontWeight: 540, padding: '4px 11px', borderRadius: 6, cursor: 'pointer',
          background: sel === i ? 'var(--surface-1)' : 'transparent',
          color: sel === i ? 'var(--text-1)' : 'var(--text-2)',
        }}>{t}</span>
      ))}
    </div>
  );
}

/* ─────────── Direction A · Calm Split ───────────
   The by-the-book baseline. Sidebar + top timer bar + calendar/timesheet
   split, goals in a quiet right column. Generous spacing does the work. */
function CockpitA() {
  return (
    <Frame>
      <SideNav active="today" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <header style={{ height: 66, flex: '0 0 auto', display: 'flex', alignItems: 'center', padding: '0 22px', background: 'var(--surface-1)', borderBottom: '1px solid var(--border)' }}>
          <TimerBar variant="calm" />
        </header>
        <div style={{ flex: 1, minHeight: 0, padding: 22, display: 'flex', flexDirection: 'column', gap: 18 }}>
          <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
            <div>
              <div className="eyebrow">Terça, 10 de junho</div>
              <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>Hoje</h1>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 18 }}>
              <div style={{ textAlign: 'right' }}>
                <div className="faint" style={{ fontSize: 11.5 }}>Total hoje</div>
                <div className="mono" style={{ fontSize: 19, fontWeight: 600 }}>{window.TT_DATA.totalToday}</div>
              </div>
              <button className="tt-btn tt-btn--accent"><Icon.plus size={15} sw={2.2} />Nova entrada</button>
            </div>
          </div>
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: '1.45fr 1fr', gap: 18 }}>
            <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <PanelHead title="Calendário" right={<SegToggle a="Calendário" b="Timesheet" sel={0} />} />
              <div style={{ padding: '16px 16px 8px', flex: 1, overflow: 'hidden' }}>
                <CalendarGrid pxPerHour={56} />
              </div>
            </section>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 18, minHeight: 0 }}>
              <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', flex: 1.3, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
                <PanelHead title="Timesheet" sub="6 entradas" right={<span className="tt-chip" style={{ borderColor: 'var(--accent-line)' }}><i className="tt-dot" style={{ background: 'var(--accent)' }} />ativo</span>} />
                <div style={{ padding: '6px 14px', overflowY: 'auto', flex: 1 }}><Timesheet /></div>
              </section>
              <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', flex: '0 0 auto', overflow: 'hidden' }}>
                <PanelHead title="Metas da semana" sub="h por projeto" />
                <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 14 }}>
                  {window.TT_DATA.projects.map(p => <GoalRow key={p.id} id={p.id} />)}
                </div>
              </section>
            </div>
          </div>
        </div>
      </main>
    </Frame>
  );
}

/* ─────────── Direction B · Mission Control (denso) ───────────
   Compact density, icon rail, a stat strip up top, split calendar +
   timesheet, and a planned-vs-real ledger across the bottom. Pro feel. */
function StatCell({ label, value, accent, dotColor }) {
  return (
    <div style={{ flex: 1, padding: '0 18px', borderLeft: '1px solid var(--border)' }}>
      <div className="faint" style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 6 }}>
        {dotColor && <i className="tt-dot" style={{ background: dotColor, width: 7, height: 7 }} />}{label}
      </div>
      <div className="mono" style={{ fontSize: 18, fontWeight: 600, marginTop: 3, color: accent ? 'var(--accent)' : 'var(--text-1)' }}>{value}</div>
    </div>
  );
}
function PlanReal({ id, plan, real }) {
  const max = 4; const pw = Math.min(100, plan / max * 100), rw = Math.min(100, real / max * 100);
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12 }}>
      <span style={{ width: 64, display: 'flex', alignItems: 'center', gap: 6 }}><Dot id={id} size={7} />{proj(id).name}</span>
      <div style={{ flex: 1, display: 'grid', gap: 3 }}>
        <div style={{ height: 6, borderRadius: 3, background: 'var(--surface-3)', position: 'relative' }}>
          <i style={{ position: 'absolute', inset: 0, width: pw + '%', borderRadius: 3, border: `1px dashed ${proj(id).color}`, opacity: .7 }} />
        </div>
        <div style={{ height: 6, borderRadius: 3, background: 'var(--surface-3)' }}>
          <i style={{ display: 'block', height: '100%', width: rw + '%', borderRadius: 3, background: proj(id).color }} />
        </div>
      </div>
      <span className="mono faint" style={{ width: 64, textAlign: 'right' }}>{real}/{plan}h</span>
    </div>
  );
}
function CockpitB() {
  return (
    <Frame>
      <div data-density="compact" style={{ display: 'flex', width: '100%', height: '100%' }}>
        <SideNav active="today" />
        <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', height: 60, padding: '0 18px', background: 'var(--surface-1)', borderBottom: '1px solid var(--border)', gap: 18 }}>
            <div style={{ flex: 1.4, minWidth: 0 }}><TimerBar variant="calm" /></div>
            <StatCell label="Total hoje" value={window.TT_DATA.totalToday} accent />
            <StatCell label="Foco profundo" value="4.6h" />
            <StatCell label="Entradas" value="6" />
          </header>
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: '1.5fr 1fr', gridTemplateRows: '1fr auto', gap: 1, background: 'var(--border)' }}>
            <section style={{ background: 'var(--surface-1)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <PanelHead title="Calendário" sub="08–18 · planejado vs. real" right={<button className="tt-btn tt-btn--ghost" style={{ padding: '5px 9px' }}><Icon.cal size={15} /></button>} />
              <div style={{ padding: '14px 16px 6px', flex: 1, overflow: 'hidden' }}><CalendarGrid pxPerHour={50} dense /></div>
            </section>
            <section style={{ background: 'var(--surface-1)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <PanelHead title="Timesheet" right={<Icon.search size={15} style={{ color: 'var(--text-3)' }} />} />
              <div style={{ padding: '4px 14px', overflowY: 'auto', flex: 1 }}><Timesheet dense /></div>
            </section>
            <section style={{ gridColumn: '1 / -1', background: 'var(--surface-1)', padding: '13px 18px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 18, marginBottom: 11 }}>
                <span style={{ fontSize: 13, fontWeight: 600 }}>Planejado vs. real</span>
                <span className="faint" style={{ fontSize: 11.5, display: 'flex', gap: 14 }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><i style={{ width: 14, height: 0, borderTop: '1px dashed var(--text-3)' }} />planejado</span>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><i style={{ width: 14, height: 5, borderRadius: 2, background: 'var(--text-2)' }} />real</span>
                </span>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', columnGap: 40, rowGap: 9 }}>
                <PlanReal id="lh" plan={3} real={2.5} /><PlanReal id="la" plan={1.5} real={1.5} />
                <PlanReal id="ak" plan={1} real={1.25} /><PlanReal id="pr" plan={0} real={0.5} />
              </div>
            </section>
          </div>
        </main>
      </div>
    </Frame>
  );
}

/* ─────────── Direction C · Hero Timer ───────────
   The running timer dominates: a large card with giant mono digits and a
   soft accent glow. Calendar + timesheet recede into a calmer base. */
function HeroTimer() {
  const sec = useClock(5070, true);
  const e = window.TT_DATA.activeEntry;
  return (
    <div className="tt-card tt-pulse" style={{ padding: '26px 30px', display: 'flex', alignItems: 'center', gap: 28, background: 'var(--surface-2)', borderColor: 'var(--accent-line)', overflow: 'hidden', position: 'relative' }}>
      <div style={{ position: 'absolute', right: -40, top: -60, width: 240, height: 240, borderRadius: '50%', background: 'var(--accent-soft)', filter: 'blur(20px)', pointerEvents: 'none' }} />
      <div style={{ flex: 1, minWidth: 0, position: 'relative' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 9, marginBottom: 10 }}>
          <span className="tt-dot" style={{ background: 'var(--accent)', width: 8, height: 8 }} />
          <span className="eyebrow" style={{ color: 'var(--accent)' }}>Rodando agora</span>
        </div>
        <div style={{ fontSize: 19, fontWeight: 600, letterSpacing: '-.02em', marginBottom: 12 }}>{e.task}</div>
        <Chip id={e.p} sub="cliente Acme" />
      </div>
      <div className="mono" style={{ fontSize: 64, fontWeight: 600, letterSpacing: '-.03em', color: 'var(--text-1)', position: 'relative' }}>{fmt(sec)}</div>
      <div style={{ display: 'flex', gap: 10, position: 'relative' }}>
        <button className="tt-iconbtn" style={{ width: 52, height: 52, background: 'var(--surface-3)', color: 'var(--text-1)' }}><Icon.pause size={18} /></button>
        <button className="tt-iconbtn tt-iconbtn--go" style={{ width: 52, height: 52 }}><Icon.stop size={18} /></button>
      </div>
    </div>
  );
}
function CockpitC() {
  return (
    <Frame>
      <SideNav active="today" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 26, gap: 20, background: 'var(--bg)' }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow">Terça, 10 de junho</div>
            <h1 style={{ margin: '4px 0 0', fontSize: 28, fontWeight: 600, letterSpacing: '-.025em' }}>Bom trabalho, Diogo.</h1>
          </div>
          <button className="tt-btn"><Icon.plus size={15} sw={2.2} />Nova entrada</button>
        </div>
        <HeroTimer />
        <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: '1.3fr 1fr', gap: 20 }}>
          <section style={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>Hoje no calendário</span>
              <SegToggle a="Dia" b="Semana" sel={0} />
            </div>
            <div className="tt-card" style={{ padding: '16px 16px 8px', flex: 1, overflow: 'hidden', background: 'var(--surface-1)' }}>
              <CalendarGrid pxPerHour={48} />
            </div>
          </section>
          <section style={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>Entradas recentes</span>
              <span className="faint" style={{ fontSize: 12 }}>Total {window.TT_DATA.totalToday}</span>
            </div>
            <div className="tt-card" style={{ padding: '6px 16px', flex: 1, overflowY: 'auto', background: 'var(--surface-1)' }}>
              <Timesheet max={5} />
            </div>
            <div style={{ marginTop: 16, display: 'flex', gap: 14 }}>
              {window.TT_DATA.projects.slice(0, 3).map(p => (
                <div key={p.id} className="tt-card" style={{ flex: 1, padding: '12px 14px', background: 'var(--surface-1)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginBottom: 8 }}><Dot id={p.id} size={8} /><span style={{ fontSize: 12.5, fontWeight: 540 }}>{p.name}</span></div>
                  <div className="mono" style={{ fontSize: 16, fontWeight: 600 }}>{p.done}<span className="faint" style={{ fontSize: 12 }}> /{p.goal}h</span></div>
                  <div className="tt-goal" style={{ marginTop: 8 }}><i style={{ width: Math.min(100, p.done / p.goal * 100) + '%', background: p.color }} /></div>
                </div>
              ))}
            </div>
          </section>
        </div>
      </main>
    </Frame>
  );
}

window.TT_Cockpit = { CockpitA, CockpitB, CockpitC };
