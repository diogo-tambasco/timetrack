// Formatação de tempo (fase 03): dígitos do timer e durações da lista —
// Date/Intl nativos, sem lib de datas (RNF-01).

// fmtClock — h:mm:ss (ou mm:ss), o formato dos dígitos mono do design.
export function fmtClock(sec: number, withHours = true): string {
  const s = Math.max(0, Math.floor(sec));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  const pad = (n: number) => String(n).padStart(2, '0');
  return withHours ? `${h}:${pad(m)}:${pad(r)}` : `${pad(m)}:${pad(r)}`;
}

// fmtHourMinute — hora local de início de uma entry (ex.: "14:03").
export function fmtHourMinute(iso: string): string {
  return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', hour12: false })
    .format(new Date(iso));
}

// browserTZ — fuso IANA do navegador, enviado em toda listagem (CA-35.1).
export function browserTZ(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

// fmtDateTime — data+hora local de um ISO (fase 19, thread de comentários):
// ex. "12/06, 14:03". Por fuso do navegador (padrão das outras listagens).
export function fmtDateTime(iso: string): string {
  return new Intl.DateTimeFormat(undefined, {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(iso));
}

// fmtDayTotal — total do dia do Modo Foco (fase 05, CA-10): "5h12".
// Horas sem zero à esquerda e sem teto de 24; minutos com 2 dígitos.
export function fmtDayTotal(sec: number): string {
  const s = Math.max(0, Math.floor(sec));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h${String(m).padStart(2, '0')}`;
}

// fmtHours (fase 28) — horas inteiras pro card vivo do projeto (ex.: 511200s →
// "142h"). Mata o `0h` hardcoded desde a fase 02 (CA-16). 0 = "0h".
export function fmtHours(sec: number): string {
  return `${Math.floor(Math.max(0, sec) / 3600)}h`;
}

// fmtDateBr (fase 28) — uma data ISO YYYY-MM-DD → "30/09" (dia/mês), por parede
// LOCAL (sem epoch — imune a DST/offset). Pro chip de prazo do card/Upcoming.
export function fmtDateBr(iso: string): string {
  const [, m, d] = iso.split('-');
  return `${d}/${m}`;
}
