// Agrupamento por dia das entradas recentes do dashboard (fase 13, P3).
// Funções PURAS e tz-corretas (dia local por PAREDE no fuso passado), pra a
// virada da meia-noite não escorregar de dia no fuso do Diogo. Sem lib de
// datas (RNF-01). Espelha o rigor da fase 11 (calendar-layout).
import type { TimeEntry } from './api';

// Janela default de "recentes": 7 dias (hoje + 6 anteriores) — cobre a virada
// do dia (P3) sem inchar o request (D-01). Constante, não configurável aqui.
export const RECENT_DAYS = 7;

export type DayGroup = { dayISO: string; label: string; items: TimeEntry[] };

// localDayISO — a data de calendário (YYYY-MM-DD) de um instante ISO NO fuso
// `tz`. Usa Intl com timeZone → correto pra qualquer fuso/DST.
export function localDayISO(iso: string, tz: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date(iso));
  const get = (t: string) => parts.find((p) => p.type === t)?.value ?? '';
  return `${get('year')}-${get('month')}-${get('day')}`;
}

// addDaysISO — aritmética de CALENDÁRIO pura (UTC, imune a DST) sobre uma data
// YYYY-MM-DD. Usada pra achar "ontem" a partir de hoje (com clamp de mês/ano) e
// a borda `from` da janela de recentes (hoje−6).
export function addDaysISO(dayISO: string, n: number): string {
  const [y, m, d] = dayISO.split('-').map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d));
  dt.setUTCDate(dt.getUTCDate() + n);
  return dt.toISOString().slice(0, 10);
}

// dayLabel — "Hoje" / "Ontem" / data pt-BR (ex.: "seg., 15 de jun.").
// Compara só strings YYYY-MM-DD (já no fuso certo); a data pt-BR é do calendário
// (meio-dia LOCAL, sem risco de shift de dia).
export function dayLabel(dayISO: string, todayISO: string): string {
  if (dayISO === todayISO) return 'Hoje';
  if (dayISO === addDaysISO(todayISO, -1)) return 'Ontem';
  const [y, m, d] = dayISO.split('-').map(Number);
  return new Intl.DateTimeFormat('pt-BR', {
    weekday: 'short',
    day: '2-digit',
    month: 'short',
  }).format(new Date(y, m - 1, d, 12, 0, 0));
}

// groupByDay — agrupa entries por dia local (no fuso `tz`), grupos do mais
// recente pro mais antigo; dentro do grupo, mais recente primeiro. todayISO é
// explícito (função pura, sem Date.now — testável).
export function groupByDay(entries: TimeEntry[], tz: string, todayISO: string): DayGroup[] {
  const byDay = new Map<string, TimeEntry[]>();
  for (const e of entries) {
    const day = localDayISO(e.started_at, tz);
    const arr = byDay.get(day);
    if (arr) arr.push(e);
    else byDay.set(day, [e]);
  }
  return [...byDay.keys()]
    .sort((a, b) => (a < b ? 1 : a > b ? -1 : 0)) // dia desc (string ISO ordena lexicograficamente)
    .map((dayISO) => ({
      dayISO,
      label: dayLabel(dayISO, todayISO),
      items: [...byDay.get(dayISO)!].sort((a, b) =>
        a.started_at < b.started_at ? 1 : a.started_at > b.started_at ? -1 : 0,
      ),
    }));
}
