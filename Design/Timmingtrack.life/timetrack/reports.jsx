/* Timetrack — Relatórios (expandido: foco, trackeado vs. não, tabela on-demand) */
const { Icon: RIcon, SideNav: RSide, Dot: RDot, proj: rProj } = window.TT;
const RD = () => window.TT_DATA;

function RCard({ title, sub, children, span }) {
  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: 16, display: 'flex', flexDirection: 'column', gridColumn: span ? `span ${span}` : 'auto', minHeight: 0, overflow: 'hidden' }}>
      <div style={{ marginBottom: 12 }}>
        <div style={{ fontSize: 13.5, fontWeight: 600 }}>{title}</div>
        {sub && <div className="faint" style={{ fontSize: 11.5, marginTop: 1 }}>{sub}</div>}
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>{children}</div>
    </section>
  );
}

function KPI({ label, value, unit, sub, accent }) {
  return (
    <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', padding: '14px 16px', flex: 1 }}>
      <div className="faint" style={{ fontSize: 11.5 }}>{label}</div>
      <div className="mono" style={{ fontSize: 24, fontWeight: 600, marginTop: 4, color: accent ? 'var(--accent)' : 'var(--text-1)' }}>
        {value}<span className="faint" style={{ fontSize: 13, fontWeight: 500 }}>{unit}</span>
      </div>
      {sub && <div className="faint" style={{ fontSize: 11.5, marginTop: 3 }}>{sub}</div>}
    </div>
  );
}

/* tempo por cliente/projeto */
function TimeByProject() {
  const data = RD().week, max = Math.max(...data.map(d => d.hrs));
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
      {data.map(d => (
        <div key={d.p} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ width: 74, fontSize: 12.5, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}><RDot id={d.p} size={7} />{rProj(d.p).name}</span>
          <div style={{ flex: 1, height: 18, borderRadius: 5, background: 'var(--surface-3)', overflow: 'hidden' }}>
            <div style={{ width: (d.hrs / max * 100) + '%', height: '100%', borderRadius: 5, background: rProj(d.p).color, opacity: .88 }} />
          </div>
          <span className="mono" style={{ width: 42, textAlign: 'right', fontSize: 12.5, fontWeight: 540 }}>{d.hrs}h</span>
        </div>
      ))}
    </div>
  );
}

/* tracked vs untracked — donut + day strip */
function TrackedDonut() {
  const { trackedPct, tracked, untracked } = RD().day;
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 22, height: '100%' }}>
      <div style={{ position: 'relative', width: 124, height: 124, flex: '0 0 auto' }}>
        <div style={{ width: '100%', height: '100%', borderRadius: '50%', background: `conic-gradient(var(--accent) ${trackedPct}%, var(--surface-3) 0)` }} />
        <div style={{ position: 'absolute', inset: 16, borderRadius: '50%', background: 'var(--surface-1)', display: 'grid', placeItems: 'center' }}>
          <div style={{ textAlign: 'center' }}>
            <div className="mono" style={{ fontSize: 22, fontWeight: 600, color: 'var(--accent)' }}>{trackedPct}%</div>
            <div className="faint" style={{ fontSize: 10 }}>rastreado</div>
          </div>
        </div>
      </div>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, whiteSpace: 'nowrap' }}><i style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--accent)' }} />Rastreado</div>
          <div className="mono" style={{ fontSize: 16, fontWeight: 600, marginTop: 3 }}>{tracked}</div>
        </div>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--text-2)', whiteSpace: 'nowrap' }}><i style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--surface-3)' }} />Não rastreado</div>
          <div className="mono" style={{ fontSize: 16, fontWeight: 600, marginTop: 3, color: 'var(--text-2)' }}>{untracked}</div>
        </div>
      </div>
    </div>
  );
}

/* foco por hora — intensity bars */
function FocusByHour() {
  const data = RD().focusByHour, max = 1;
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 5, height: 130 }}>
        {data.map(d => (
          <div key={d.h} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, height: '100%', justifyContent: 'flex-end' }}>
            <div style={{ width: '100%', height: Math.max(3, d.v / max * 108), borderRadius: 4, background: 'var(--accent)', opacity: .25 + d.v * .75 }} />
            {d.h % 2 === 1 && <span className="mono faint" style={{ fontSize: 9.5 }}>{d.h}</span>}
          </div>
        ))}
      </div>
      <div className="faint" style={{ fontSize: 11.5, marginTop: 10 }}>Pico de foco às <b style={{ color: 'var(--text-1)' }}>16h</b> · vale no almoço</div>
    </div>
  );
}

/* foco vs fragmentação */
function FocusFrag() {
  const days = RD().focusDays, max = Math.max(...days.map(d => d.deep + d.frag));
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 12, height: 130 }}>
        {days.map(d => {
          const tot = d.deep + d.frag;
          return (
            <div key={d.d} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, height: '100%', justifyContent: 'flex-end' }}>
              <div style={{ width: 26, height: (tot / max * 108), display: 'flex', flexDirection: 'column', justifyContent: 'flex-end', borderRadius: 5, overflow: 'hidden' }}>
                <div style={{ height: (d.frag / tot * 100) + '%', background: 'var(--surface-3)' }} />
                <div style={{ height: (d.deep / tot * 100) + '%', background: 'var(--accent)' }} />
              </div>
              <span className="faint" style={{ fontSize: 10.5 }}>{d.d}</span>
            </div>
          );
        })}
      </div>
      <div style={{ display: 'flex', gap: 14, marginTop: 10, fontSize: 11.5 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><i style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--accent)' }} />foco</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, color: 'var(--text-2)' }}><i style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--surface-3)' }} />fragmentado</span>
      </div>
    </div>
  );
}

/* meta vs realizado */
function GoalVsReal() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {RD().projects.map(p => {
        const pct = Math.min(112, p.done / p.goal * 100), over = p.done >= p.goal;
        return (
          <div key={p.id}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 5, fontSize: 12.5 }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontWeight: 500 }}><RDot id={p.id} size={7} />{p.name}</span>
              <span className="mono faint" style={{ fontSize: 11.5 }}>{p.done}/{p.goal}h {over && <span style={{ color: 'var(--accent)' }}>✓</span>}</span>
            </div>
            <div style={{ position: 'relative', height: 8, borderRadius: 999, background: 'var(--surface-3)' }}>
              <div style={{ width: pct + '%', height: '100%', borderRadius: 999, background: over ? 'var(--accent)' : p.color }} />
              <div style={{ position: 'absolute', left: '89%', top: -3, bottom: -3, width: 2, background: 'var(--text-2)', borderRadius: 2 }} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

/* planejado vs real */
function PlannedVsReal() {
  const rows = [{ p:'lh', plan:16, real:11.5 },{ p:'la', plan:10, real:9.2 },{ p:'ak', plan:8, real:8.4 },{ p:'pr', plan:12, real:6 },{ p:'fr', plan:6, real:3.5 }];
  const max = Math.max(...rows.flatMap(r => [r.plan, r.real]));
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
      {rows.map(r => (
        <div key={r.p} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ width: 74, fontSize: 12.5, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}><RDot id={r.p} size={7} />{rProj(r.p).name}</span>
          <div style={{ flex: 1, display: 'grid', gap: 4 }}>
            <div style={{ height: 8, borderRadius: 999, background: 'var(--surface-3)', position: 'relative' }}><i style={{ position: 'absolute', inset: 0, width: (r.plan / max * 100) + '%', borderRadius: 999, border: `1.4px dashed color-mix(in srgb, ${rProj(r.p).color} 60%, var(--border-2))` }} /></div>
            <div style={{ height: 8, borderRadius: 999, background: 'var(--surface-3)' }}><i style={{ display: 'block', height: '100%', width: (r.real / max * 100) + '%', borderRadius: 999, background: rProj(r.p).color }} /></div>
          </div>
          <span className="mono faint" style={{ width: 64, textAlign: 'right', fontSize: 11.5 }}>{r.real}/{r.plan}h</span>
        </div>
      ))}
    </div>
  );
}

/* detailed table — only when the user selects "Tabela" */
function DetailTable() {
  const rows = RD().projects.map(p => ({
    ...p, untracked: (p.done * 0.18).toFixed(1), focus: 60 + (p.id.charCodeAt(0) % 30), plan: { lh:16,la:10,ak:8,pr:12,fr:6 }[p.id], sessions: p.tasks + 2,
  }));
  return (
    <div className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'auto', height: '100%' }}>
      <table className="tt-table">
        <thead><tr>
          <th>Projeto</th><th>Cliente</th><th className="num">Rastreado</th><th className="num">Não rastr.</th>
          <th className="num">Foco %</th><th className="num">Meta</th><th className="num">Planejado</th><th className="num">Sessões</th>
        </tr></thead>
        <tbody>
          {rows.map(r => (
            <tr key={r.id}>
              <td><span style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 540 }}><RDot id={r.id} size={8} />{r.name}</span></td>
              <td className="muted">{r.client}</td>
              <td className="num">{r.done}h</td><td className="num muted">{r.untracked}h</td>
              <td className="num" style={{ color: 'var(--accent)' }}>{r.focus}%</td>
              <td className="num muted">{r.goal}h</td><td className="num muted">{r.plan}h</td><td className="num">{r.sessions}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Reports() {
  const [view, setView] = React.useState('charts');
  const d = RD().day;
  return (
    <div className="tt" style={{ width: '100%', height: '100%', display: 'flex', overflow: 'hidden' }}>
      <RSide active="rep" />
      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 24, gap: 16 }}>
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <div className="eyebrow">Semana · 9–15 jun</div>
            <h1 style={{ margin: '4px 0 0', fontSize: 24, fontWeight: 600, letterSpacing: '-.02em' }}>Relatórios</h1>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 9, padding: 2, gap: 2 }}>
              {[['charts', 'Gráficos', RIcon.chart], ['table', 'Tabela', RIcon.table]].map(([v, l, I]) => (
                <button key={v} onClick={() => setView(v)} style={{
                  border: 'none', cursor: 'pointer', fontFamily: 'var(--font-ui)', fontSize: 12.5, fontWeight: 540,
                  padding: '6px 12px', borderRadius: 7, display: 'flex', alignItems: 'center', gap: 6,
                  background: view === v ? 'var(--surface-1)' : 'transparent', color: view === v ? 'var(--text-1)' : 'var(--text-2)',
                }}><I size={14} />{l}</button>
              ))}
            </div>
            <button className="tt-btn"><RIcon.cal size={15} />Esta semana</button>
            <button className="tt-btn tt-btn--ghost"><RIcon.arrow size={15} />Exportar</button>
          </div>
        </div>

        {/* KPI strip */}
        <div style={{ display: 'flex', gap: 14, flex: '0 0 auto' }}>
          <KPI label="Rastreado (semana)" value="38,6" unit="h" sub="+12% vs. semana passada" accent />
          <KPI label="Foco profundo" value={d.deepPct} unit="%" sub={`${d.longestFocus} maior bloco`} />
          <KPI label="Sessão média" value={d.avgSession.replace(' min', '')} unit=" min" sub={`${d.sessions} sessões hoje`} />
          <KPI label="Não rastreado (hoje)" value={d.untracked.slice(0, 4)} unit="" sub="lacunas na agenda" />
        </div>

        {view === 'charts' ? (
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gridTemplateRows: '1fr 1fr', gap: 14 }}>
            <RCard title="Tempo por cliente / projeto" sub="38,6h esta semana"><TimeByProject /></RCard>
            <RCard title="Rastreado vs. não rastreado" sub="proporção do dia"><TrackedDonut /></RCard>
            <RCard title="Foco por hora" sub="intensidade ao longo do dia"><FocusByHour /></RCard>
            <RCard title="Foco vs. fragmentação" sub="blocos longos × pula-pula"><FocusFrag /></RCard>
            <RCard title="Meta vs. realizado" sub="meta semanal por projeto"><GoalVsReal /></RCard>
            <RCard title="Planejado vs. real" sub="calendário × rastreado"><PlannedVsReal /></RCard>
          </div>
        ) : (
          <div style={{ flex: 1, minHeight: 0 }}><DetailTable /></div>
        )}
      </main>
    </div>
  );
}

window.TT_Reports = { Reports };
