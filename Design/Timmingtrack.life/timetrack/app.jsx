/* Timetrack — app shell: live-skin engine + canvas assembly */

// ---------- live skin engine (drives :root, persists, re-skins all frames) ----------
(function () {
  const KEY = 'tt-skin';
  const ACCENTS = { teal: '#2dd4bf', amber: '#e0a13c', peri: '#7c9cf5', lime: '#9ae66e', violet: '#b07cd6', rose: '#e0795e' };
  window.__TT_ACCENTS = Object.entries(ACCENTS).map(([id, hex]) => ({ id, hex }));

  let state = { accentId: 'teal', accent: ACCENTS.teal, theme: 'dark', density: 'comfortable' };
  try { Object.assign(state, JSON.parse(localStorage.getItem(KEY) || '{}')); } catch {}
  const subs = new Set();

  const hex2rgb = h => { const n = parseInt(h.replace('#', ''), 16); return [n >> 16 & 255, n >> 8 & 255, n & 255]; };
  const lum = h => { const [r, g, b] = hex2rgb(h).map(v => v / 255); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };

  function apply() {
    const r = document.documentElement, s = r.style;
    const [R, G, B] = hex2rgb(state.accent);
    s.setProperty('--accent', state.accent);
    s.setProperty('--accent-soft', `rgba(${R},${G},${B},.14)`);
    s.setProperty('--accent-line', `rgba(${R},${G},${B},.42)`);
    s.setProperty('--live', state.accent);
    s.setProperty('--accent-ink', lum(state.accent) > 0.62 ? '#10261f' : '#04221d');
    const theme = state.theme === 'system'
      ? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
      : state.theme;
    r.setAttribute('data-theme', theme);
    r.setAttribute('data-density', state.density);
    try { localStorage.setItem(KEY, JSON.stringify(state)); } catch {}
    subs.forEach(fn => fn({ ...state }));
  }

  window.TTskin = {
    get: () => ({ ...state }),
    set: p => { state = { ...state, ...p }; apply(); },
    subscribe: fn => { subs.add(fn); return () => subs.delete(fn); },
  };
  apply();
})();

// ---------- canvas ----------
const { CockpitA, CockpitB, CockpitC } = window.TT_Cockpit;
const { WeekCalendar, CalSplit } = window.TT_Calendar;
const { Projects } = window.TT_Projects;
const { Clients } = window.TT_Clients;
const { Workspace } = window.TT_Workspace;
const { NewProject } = window.TT_NewProject;
const { Gcal } = window.TT_Gcal;
const { FocoWork, FocoBreak, FocoPicker } = window.TT_Foco;
const { Reports } = window.TT_Reports;
const { Settings } = window.TT_Settings;
const { TweakBar } = window.TT_TweakBar;

const dark = { background: 'var(--bg)' };
const black = { background: '#000' };

function App() {
  return (
    <>
      <DesignCanvas>
        <DCSection id="cockpit" title="Cockpit — desktop" subtitle="Três direções da mesma estrutura (calendário + timesheet, timer no topo). Compare e escolha.">
          <DCArtboard id="a" label="A · Calm Split" width={1440} height={900} style={dark}><CockpitA /></DCArtboard>
          <DCArtboard id="b" label="B · Mission Control (denso)" width={1440} height={900} style={dark}><CockpitB /></DCArtboard>
          <DCArtboard id="c" label="C · Hero Timer" width={1440} height={900} style={dark}><CockpitC /></DCArtboard>
        </DCSection>

        <DCSection id="cal" title="Calendário" subtitle="Estrutura do Google Calendar / Toggl 2.0 — grade da semana com planejado (tracejado) vs. rastreado.">
          <DCArtboard id="week" label="Semana" width={1440} height={900} style={dark}><WeekCalendar /></DCArtboard>
          <DCArtboard id="split" label="Split · calendário + timesheet" width={1440} height={900} style={dark}><CalSplit /></DCArtboard>
          <DCArtboard id="gcal" label="Google Calendar · integração" width={1440} height={900} style={dark}><Gcal /></DCArtboard>
        </DCSection>

        <DCSection id="proj" title="Projetos" subtitle="Lista enxuta no estilo Toggl + criação de projeto com membros específicos.">
          <DCArtboard id="projects" label="Projetos · lista" width={1440} height={900} style={dark}><Projects /></DCArtboard>
          <DCArtboard id="newproject" label="Novo projeto · membros" width={1440} height={900} style={dark}><NewProject /></DCArtboard>
        </DCSection>

        <DCSection id="clients" title="Clientes" subtitle="Estilo Toggl: cartões com horas/valor por cliente · tabela detalhada só quando você seleciona.">
          <DCArtboard id="clients" label="Clientes · cartões + tabela" width={1440} height={900} style={dark}><Clients /></DCArtboard>
        </DCSection>

        <DCSection id="foco" title="Modo Foco — iPad" subtitle="Escolher task (anti-Toggl: só digitar + recentes), flip-clock fullscreen, e Pomodoro ao pausar.">
          <DCArtboard id="picker" label="Escolher task · iPad" width={1194} height={834} style={black}><FocoPicker /></DCArtboard>
          <DCArtboard id="work" label="Trabalho · count-up" width={1194} height={834} style={black}><FocoWork /></DCArtboard>
          <DCArtboard id="break" label="Pausa · Pomodoro" width={1194} height={834} style={black}><FocoBreak /></DCArtboard>
        </DCSection>

        <DCSection id="rep" title="Relatórios" subtitle="Foco por hora, rastreado vs. não rastreado, metas — e tabela detalhada só quando você seleciona.">
          <DCArtboard id="reports" label="Relatórios" width={1440} height={900} style={dark}><Reports /></DCArtboard>
        </DCSection>

        <DCSection id="ws" title="Workspace" subtitle="Multi-user: membros, papéis (Owner/Membro/Viewer), presença ao vivo e convites.">
          <DCArtboard id="workspace" label="Workspace · membros & papéis" width={1440} height={900} style={dark}><Workspace /></DCArtboard>
        </DCSection>

        <DCSection id="set" title="Ajustes" subtitle="Personalização real: accent, tema, densidade, cor por projeto, Pomodoro. (Os controles mexem nas telas de verdade.)">
          <DCArtboard id="settings" label="Settings · personalização" width={1440} height={900} style={dark}><Settings /></DCArtboard>
        </DCSection>
      </DesignCanvas>
      <TweakBar />
    </>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />);
