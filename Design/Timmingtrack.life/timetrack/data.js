/* Timetrack — sample data (real client/project names from REQUISITOS) */
window.TT_DATA = (function () {
  const projects = [
    { id: 'lh',  name: 'Acme',  client: 'Acme',   color: 'var(--p-acme)', goal: 18, done: 11.5, tasks: 8, total: 142, status: 'Ativo' },
    { id: 'la',  name: 'Lauda',     client: 'Lauda',      color: 'var(--p-lauda)',    goal: 12, done: 9.2,  tasks: 5, total: 88,  status: 'Ativo' },
    { id: 'ak',  name: 'Arkan',     client: 'Arkan',      color: 'var(--p-arkan)',    goal: 8,  done: 8.4,  tasks: 4, total: 36,  status: 'Ativo' },
    { id: 'pr',  name: 'Primor',    client: 'Primor Labs',color: 'var(--p-primor)',   goal: 14, done: 6.0,  tasks: 6, total: 64,  status: 'Ativo' },
    { id: 'fr',  name: 'Freela',    client: 'Avulso',     color: 'var(--p-freela)',   goal: 10, done: 3.5,  tasks: 3, total: 21,  status: 'Pausado' },
  ];
  const byId = Object.fromEntries(projects.map(p => [p.id, p]));

  // today's timeline blocks — planned (from GCal) vs real (tracked)
  // h = start hour (decimal), len = hours
  const blocks = [
    { p:'lh', task:'Sprint review + roadmap',     h:9.0,  len:1.0, plan:[9.0,1.0], src:'calendar' },
    { p:'la', task:'Revisão de laudas — lote 14',  h:10.25,len:1.5, plan:[10.0,1.5], src:'timer' },
    { p:'pr', task:'PrimaOS — deploy Caddy',       h:12.0, len:0.5, plan:null, src:'timer' },
    { p:'ak', task:'Arkan · API tokens',           h:13.5, len:1.25,plan:[13.0,1.0], src:'timer' },
    { p:'lh', task:'Call cliente — escopo Q3',     h:15.0, len:0.75,plan:[15.0,1.0], src:'calendar' },
    { p:'fr', task:'Freela — landing hero',        h:16.25,len:1.5, plan:null, src:'timer' },
  ];

  // timesheet rows for today
  const entries = [
    { p:'fr', task:'Freela — landing hero',         start:'16:15', dur:'1:24:30', running:true },
    { p:'lh', task:'Call cliente — escopo Q3',      start:'15:00', dur:'0:46:10', running:false },
    { p:'ak', task:'Arkan · API tokens',            start:'13:30', dur:'1:12:55', running:false },
    { p:'pr', task:'PrimaOS — deploy Caddy',        start:'12:00', dur:'0:31:40', running:false },
    { p:'la', task:'Revisão de laudas — lote 14',   start:'10:15', dur:'1:28:05', running:false },
    { p:'lh', task:'Sprint review + roadmap',       start:'09:00', dur:'0:58:20', running:false },
  ];

  // weekly totals per project (hrs) for reports
  const week = [
    { p:'lh', hrs:11.5 }, { p:'la', hrs:9.2 }, { p:'ak', hrs:8.4 },
    { p:'pr', hrs:6.0 }, { p:'fr', hrs:3.5 },
  ];

  // focus vs fragmentation — daily deep-work blocks (count of long >45min blocks)
  const focusDays = [
    { d:'Seg', deep:4.2, frag:1.1 }, { d:'Ter', deep:5.5, frag:0.6 },
    { d:'Qua', deep:2.1, frag:2.8 }, { d:'Qui', deep:6.0, frag:0.9 },
    { d:'Sex', deep:3.4, frag:1.7 }, { d:'Sáb', deep:1.0, frag:0.3 },
  ];

  // week view (Seg 9 – Dom 15). day: 0=Seg … 6=Dom. today = índice 1 (Ter 10).
  const weekDays = [
    { wd: 'Seg', date: 9 }, { wd: 'Ter', date: 10 }, { wd: 'Qua', date: 11 },
    { wd: 'Qui', date: 12 }, { wd: 'Sex', date: 13 }, { wd: 'Sáb', date: 14 }, { wd: 'Dom', date: 15 },
  ];
  const weekEvents = [
    { day:0, h:9,    len:1.5, p:'lh', task:'Planning da semana', planned:true },
    { day:0, h:11,   len:1,   p:'la', task:'Laudas — triagem' },
    { day:0, h:14,   len:2,   p:'pr', task:'PrimaOS — infra' },
    { day:1, h:9,    len:1,   p:'lh', task:'Sprint review', planned:true },
    { day:1, h:10.25,len:1.5, p:'la', task:'Revisão laudas — lote 14' },
    { day:1, h:13.5, len:1.25,p:'ak', task:'Arkan · API tokens' },
    { day:1, h:15,   len:.75, p:'lh', task:'Call escopo Q3', planned:true },
    { day:1, h:16.25,len:1.5, p:'fr', task:'Freela — landing hero' },
    { day:2, h:9.5,  len:2,   p:'pr', task:'Deploy Caddy + vhost' },
    { day:2, h:13,   len:1.5, p:'ak', task:'Skill /timetrack-task' },
    { day:2, h:16,   len:1,   p:'la', task:'Sync laudas', planned:true },
    { day:3, h:8.5,  len:1,   p:'lh', task:'Daily + revisão', planned:true },
    { day:3, h:10,   len:2.5, p:'lh', task:'Acme — roadmap Q3' },
    { day:3, h:14.5, len:1.5, p:'fr', task:'Freela — componentes' },
    { day:4, h:9,    len:1.5, p:'pr', task:'Primor — review' },
    { day:4, h:11,   len:1,   p:'ak', task:'Arkan — testes' },
    { day:4, h:15,   len:1.5, p:'la', task:'Laudas — fechamento', planned:true },
    { day:5, h:10,   len:1,   p:'fr', task:'Freela — ajustes' },
  ];

  // ---- clients (Toggl-style aggregation) ----
  const clients = [
    { id:'acme', name:'Acme',    projs:['lh'],      color:'var(--p-acme)', week:11.5, total:142, rate:180, contact:'Marina Souza', status:'Ativo' },
    { id:'lauda',    name:'Lauda',       projs:['la'],      color:'var(--p-lauda)',    week:9.2,  total:88,  rate:150, contact:'Rafael Tavares', status:'Ativo' },
    { id:'arkan',    name:'Arkan',       projs:['ak'],      color:'var(--p-arkan)',    week:8.4,  total:36,  rate:160, contact:'Equipe Arkan', status:'Ativo' },
    { id:'primor',   name:'Primor Labs', projs:['pr'],      color:'var(--p-primor)',   week:6.0,  total:64,  rate:0,   contact:'Interno', status:'Interno' },
    { id:'avulso',   name:'Avulso',      projs:['fr'],      color:'var(--p-freela)',   week:3.5,  total:21,  rate:120, contact:'Diversos', status:'Pausado' },
  ];

  // ---- workspace members + roles ----
  const members = [
    { id:'diogo',  name:'Diogo Prado', email:'diogo@timetrack.live', role:'Owner',  initials:'DP', color:'var(--accent)',      active:'agora',    running:'Freela — landing hero' },
    { id:'carlos', name:'Carlos Lima', email:'carlos@primorlabs.cloud', role:'Membro', initials:'CL', color:'var(--p-acme)', active:'há 4 min', running:'Acme — roadmap Q3' },
    { id:'kadu',   name:'Kadu Reis',   email:'kadu@primorlabs.cloud', role:'Membro', initials:'KR', color:'var(--p-arkan)',     active:'há 1 h',   running:null },
    { id:'marina', name:'Marina Souza',email:'marina@acme.com',  role:'Viewer', initials:'MS', color:'var(--p-lauda)',     active:'ontem',    running:null },
  ];
  const invites = [
    { email:'rafael@lauda.com', role:'Membro', sent:'há 2 dias' },
  ];
  const roles = [
    { id:'Owner',  desc:'Controle total: cobrança, membros, exclusão do workspace.' },
    { id:'Membro', desc:'Cria projetos e tasks, rastreia tempo, vê o que é compartilhado.' },
    { id:'Viewer', desc:'Só leitura: vê projetos e relatórios compartilhados.' },
  ];

  // ---- day stats: tracked vs untracked + focus ----
  const day = {
    tracked:'6:21:40', untracked:'1:38:20', total:'8:00:00',
    trackedH:6.36, untrackedH:1.64, trackedPct:79,
    longestFocus:'1:28:05', avgSession:'42 min', sessions:9, deepPct:71,
  };
  // hourly deep-focus intensity (0–1) for area chart, 07–19
  const focusByHour = [
    {h:7,v:.0},{h:8,v:.35},{h:9,v:.9},{h:10,v:.95},{h:11,v:.62},{h:12,v:.18},
    {h:13,v:.48},{h:14,v:.88},{h:15,v:.7},{h:16,v:.96},{h:17,v:.8},{h:18,v:.34},{h:19,v:.08},
  ];

  // ---- Google Calendar integration ----
  const gcal = {
    lastSync:'há 2 min',
    accounts: [
      { id:'diogo',  email:'diogo@primorlabs.cloud', primary:true,  synced:true, events:14, color:'var(--accent)' },
      { id:'carlos', email:'carlos@acme.com',    primary:false, synced:true, events:8,  color:'var(--p-acme)' },
    ],
    // event ↔ entry mapping examples
    mappings: [
      { event:'Sprint review + roadmap', cal:'diogo',  entry:'0:58:20', p:'lh', state:'synced' },
      { event:'Revisão de laudas — lote 14', cal:'diogo', entry:'1:28:05', p:'la', state:'synced' },
      { event:'Call cliente — escopo Q3', cal:'carlos', entry:'0:46:10', p:'lh', state:'synced' },
      { event:'Almoço', cal:'diogo', entry:null, p:null, state:'ignored' },
      { event:'Freela — landing hero', cal:null, entry:'1:24:30', p:'fr', state:'push' },
    ],
  };

  return { projects, byId, blocks, entries, week, focusDays, weekDays, weekEvents,
    clients, members, invites, roles, day, focusByHour, gcal,
    activeEntry: entries[0],
    totalToday: '6:21:40' };
})();
