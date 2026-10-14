// calendar-layout.ts — fase 11 (plan §4): o CORAÇÃO da grade. Funções PURAS
// de posicionamento por tempo — datas → números, sem React/DOM. Tudo em fuso
// LOCAL de parede (CA-14): `new Date(iso)` dá o instante; getHours()/getDate()
// dão a parede local, imune a DST e offsets. Os componentes só consomem
// `Positioned[]` (TimeGrid) e `allDayByDay` (faixa do topo).
import type { CalendarEvent } from './api';

// altura mínima legível de um segmento (em minutos de grade) — pro título
// caber e o clique funcionar mesmo num evento curtíssimo (CA-10).
export const MIN_SEGMENT_MINUTES = 20;

const MIN_PER_DAY = 1440;
const pad = (n: number) => String(n).padStart(2, '0');

// ——— helpers de data (fuso local) ———

// isoDay — Date → 'YYYY-MM-DD' pela parede local.
export function isoDay(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// parseDay — 'YYYY-MM-DD' → Date local à meia-noite (new Date(iso) trataria
// como UTC).
export function parseDay(dayISO: string): Date {
  const [y, m, d] = dayISO.split('-').map(Number);
  return new Date(y, m - 1, d);
}

// localDayISO — ISO com horário → dia local 'YYYY-MM-DD'.
function localDayISO(iso: string): string {
  return isoDay(new Date(iso));
}

export function addDays(d: Date, n: number): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate() + n);
}
export function addWeeks(d: Date, n: number): Date {
  return addDays(d, n * 7);
}

// addMonths — soma meses com CLAMP do dia em min(dia, últimoDiaDoMêsAlvo)
// (CA-19.1): a navegação ingênua new Date(y, m±1, d) pula fevereiro a partir
// de 31/jan. Ancorado no 1º do mês alvo + clamp.
export function addMonths(d: Date, delta: number): Date {
  const target = new Date(d.getFullYear(), d.getMonth() + delta, 1);
  const lastDay = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate();
  return new Date(target.getFullYear(), target.getMonth(), Math.min(d.getDate(), lastDay));
}

// mondayOf — segunda-feira (local) da semana do refDate (semana na segunda,
// CA-20).
function mondayOf(d: Date): Date {
  const dow = (d.getDay() + 6) % 7; // segunda = 0
  return addDays(d, -dow);
}

// ——— segmentação por dia (CA-09/11/11.1) ———

export type Segment = {
  event: CalendarEvent;
  dayISO: string;
  startMin: number; // 0..1440 (parede local, clipado ao dia)
  endMin: number; // 0..1440
};

export type Positioned = Segment & {
  topPct: number; // fração 0..1 do dia (grade renderiza 24h sempre — CA-15)
  heightPct: number;
  colIndex: number;
  colCount: number;
};

// minOfDay — minuto-de-parede de um instante dentro de [dayStart, dayEnd];
// clipa nas bordas (0 / 1440), DST-proof.
function minOfDay(t: Date, dayStart: Date, dayEnd: Date): number {
  if (t.getTime() <= dayStart.getTime()) return 0;
  if (t.getTime() >= dayEnd.getTime()) return MIN_PER_DAY;
  return t.getHours() * 60 + t.getMinutes() + t.getSeconds() / 60;
}

// splitByDay — corta um evento COM HORÁRIO nos limites de dia local; cada
// segmento clipado a [00:00, 24:00) na coluna do seu dia (cross-midnight e
// multi-dia — CA-11). DESCARTA segmento de duração de parede zero (fim
// exatamente em 00:00 — CA-11.1): a sobreposição de instante nula nunca vira
// segmento, então 10:00→00:00 dá 1 só segmento, sem fantasma no dia+1.
export function splitByDay(event: CalendarEvent, days: string[]): Segment[] {
  const start = new Date(event.start);
  const end = new Date(event.end);
  const out: Segment[] = [];
  for (const dayISO of days) {
    const dayStart = parseDay(dayISO);
    const dayEnd = addDays(dayStart, 1);
    const segStart = Math.max(start.getTime(), dayStart.getTime());
    const segEnd = Math.min(end.getTime(), dayEnd.getTime());
    if (segEnd <= segStart) continue; // sem sobreposição / duração zero (CA-11.1)
    out.push({
      event,
      dayISO,
      startMin: minOfDay(new Date(segStart), dayStart, dayEnd),
      endMin: minOfDay(new Date(segEnd), dayStart, dayEnd),
    });
  }
  return out;
}

// ——— empacotamento em colunas (CA-12) ———

// packColumns — varredura de intervalos num MESMO dia: clusters de
// sobreposição recebem colIndex/colCount mínimos. Borda-a-borda (end==start)
// NÃO sobrepõe (pode reusar coluna). Retorna Positioned com top/height.
export function packColumns(segments: Segment[]): Positioned[] {
  const sorted = segments
    .slice()
    .sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin);

  const out: Positioned[] = [];
  let cluster: Positioned[] = [];
  let clusterEnd = -Infinity; // maior endMin do cluster atual
  let colEnds: number[] = []; // último endMin colocado em cada coluna

  const flush = () => {
    for (const p of cluster) p.colCount = colEnds.length || 1;
    cluster = [];
    colEnds = [];
    clusterEnd = -Infinity;
  };

  for (const seg of sorted) {
    if (seg.startMin >= clusterEnd) flush(); // não toca nenhum do cluster → novo
    // primeira coluna livre (último evento terminou em <= início deste; borda
    // ok → reusa)
    let col = colEnds.findIndex((e) => e <= seg.startMin);
    if (col === -1) {
      col = colEnds.length;
      colEnds.push(seg.endMin);
    } else {
      colEnds[col] = seg.endMin;
    }
    const dur = seg.endMin - seg.startMin;
    const p: Positioned = {
      ...seg,
      topPct: seg.startMin / MIN_PER_DAY,
      heightPct: Math.max(dur, MIN_SEGMENT_MINUTES) / MIN_PER_DAY,
      colIndex: col,
      colCount: 1, // definido no flush
    };
    cluster.push(p);
    out.push(p);
    clusterEnd = Math.max(clusterEnd, seg.endMin);
  }
  flush();
  return out;
}

// layoutTimed — posiciona TODOS os eventos com horário dos `days`: segmenta
// por dia e empacota colunas por dia. All-day é ignorado aqui (faixa própria).
export function layoutTimed(events: CalendarEvent[], days: string[]): Positioned[] {
  const byDay = new Map<string, Segment[]>();
  for (const e of events) {
    if (e.all_day) continue;
    for (const seg of splitByDay(e, days)) {
      const list = byDay.get(seg.dayISO) ?? [];
      list.push(seg);
      byDay.set(seg.dayISO, list);
    }
  }
  const out: Positioned[] = [];
  for (const segs of byDay.values()) out.push(...packColumns(segs));
  return out;
}

// ——— all-day (CA-13/13.1) ———

// allDayByDay — agrupa eventos all-day por dia COBERTO, intervalo
// [startDay, endDay) com end EXCLUSIVO (convenção GCal do client.go — CA-13.1):
// all-day D→D+1 cobre só D; D→D+3 cobre D, D+1, D+2 (nunca N+1). Só inclui
// dias presentes em `days`.
export function allDayByDay(events: CalendarEvent[], days: string[]): Map<string, CalendarEvent[]> {
  const inView = new Set(days);
  const m = new Map<string, CalendarEvent[]>();
  for (const e of events) {
    if (!e.all_day) continue;
    const startDay = localDayISO(e.start);
    const endDay = localDayISO(e.end); // EXCLUSIVO
    let cur = parseDay(startDay);
    // cobre [startDay, endDay): para no dia anterior a endDay
    while (isoDay(cur) < endDay) {
      const k = isoDay(cur);
      if (inView.has(k)) {
        const list = m.get(k) ?? [];
        list.push(e);
        m.set(k, list);
      }
      cur = addDays(cur, 1);
    }
  }
  return m;
}

// ——— grade do mês (CA-18) ———

// monthGridDays — 42 dias (6 semanas) segunda→domingo cobrindo o mês do
// refDate, incluindo dias adjacentes do mês anterior/seguinte.
export function monthGridDays(refDate: Date): string[] {
  const first = new Date(refDate.getFullYear(), refDate.getMonth(), 1);
  const gridStart = mondayOf(first);
  const out: string[] = [];
  for (let i = 0; i < 42; i++) out.push(isoDay(addDays(gridStart, i)));
  return out;
}

// inMonth — o dia pertence ao mês do refDate? (pra atenuar as bordas).
export function inMonth(dayISO: string, refDate: Date): boolean {
  const d = parseDay(dayISO);
  return d.getFullYear() === refDate.getFullYear() && d.getMonth() === refDate.getMonth();
}

// ——— título do período (CA-19.2) — derivado do refDate ———

const MONTHS = [
  'janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho',
  'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro',
];
const MONTHS_SHORT = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export type View = 'day' | 'week' | 'month';

// periodTitle — título legível a partir do refDate (NUNCA da janela de fetch
// — CA-19.2): Mês = mês/ano do refDate; Dia = data; Semana = faixa seg→dom.
export function periodTitle(view: View, refDate: Date): string {
  if (view === 'month') return `${cap(MONTHS[refDate.getMonth()])} ${refDate.getFullYear()}`;
  if (view === 'day') return `${refDate.getDate()} de ${MONTHS[refDate.getMonth()]} de ${refDate.getFullYear()}`;
  // semana: segunda→domingo
  const mon = mondayOf(refDate);
  const sun = addDays(mon, 6);
  if (mon.getMonth() === sun.getMonth()) {
    return `${mon.getDate()}–${sun.getDate()} de ${MONTHS[mon.getMonth()]} de ${mon.getFullYear()}`;
  }
  const left = `${mon.getDate()} ${MONTHS_SHORT[mon.getMonth()]}`;
  const right = `${sun.getDate()} ${MONTHS_SHORT[sun.getMonth()]}`;
  return `${left} – ${right} ${sun.getFullYear()}`;
}

// ——— dias e janelas de fetch (CA-05/06/07/20) ———

// weekDays — os 7 dias (seg→dom) da semana do refDate.
export function weekDays(refDate: Date): string[] {
  const mon = mondayOf(refDate);
  return Array.from({ length: 7 }, (_, i) => isoDay(addDays(mon, i)));
}

// weekRange — janela de fetch da semana: from=segunda, to=domingo (CA-05).
export function weekRange(refDate: Date): { from: string; to: string } {
  const mon = mondayOf(refDate);
  return { from: isoDay(mon), to: isoDay(addDays(mon, 6)) };
}

// dayRange — janela do dia: from=to=o dia exibido (CA-06).
export function dayRange(refDate: Date): { from: string; to: string } {
  const d = isoDay(refDate);
  return { from: d, to: d };
}

// monthRange — janela do mês: primeira e última célula da grade de 42 dias,
// um único request (CA-07).
export function monthRange(refDate: Date): { from: string; to: string } {
  const days = monthGridDays(refDate);
  return { from: days[0], to: days[days.length - 1] };
}
