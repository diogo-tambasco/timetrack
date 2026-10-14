// calendar-layout.test.ts — fase 11 T-04 (CA-09/10/11/11.1/12/13.1/14/19.1/19.2,
// RNF-04). O coração testável: datas → números, sem React/DOM. Os casos
// obrigatórios do plan §4 são a fonte (incluindo os 5 findings da verificação
// adversarial da spec). Tudo em fuso LOCAL: os ISO dos eventos são gerados a
// partir de Date local → toISOString, então fazem round-trip pro mesmo
// horário de parede em QUALQUER fuso de máquina (teste tz-independente).
import { describe, it, expect } from 'vitest';
import type { CalendarEvent } from './api';
import {
  splitByDay,
  layoutTimed,
  allDayByDay,
  monthGridDays,
  inMonth,
  addMonths,
  addDays,
  addWeeks,
  periodTitle,
  weekDays,
  weekRange,
  dayRange,
  monthRange,
  MIN_SEGMENT_MINUTES,
} from './calendar-layout';

// at — ISO de um instante de PAREDE local (round-trip tz-independente).
const at = (y: number, mo: number, d: number, h = 0, mi = 0) =>
  new Date(y, mo - 1, d, h, mi).toISOString();

const ev = (over: Partial<CalendarEvent>): CalendarEvent => ({
  id: 'e',
  summary: 'E',
  start: at(2026, 6, 15, 10, 0),
  end: at(2026, 6, 15, 11, 0),
  all_day: false,
  html_link: '',
  ...over,
});

const DAY = '2026-06-15';
const NEXT = '2026-06-16';

describe('splitByDay (CA-09/11/11.1)', () => {
  it('evento 10:00–11:00 → 1 segmento na coluna do dia', () => {
    const segs = splitByDay(ev({}), [DAY, NEXT]);
    expect(segs).toHaveLength(1);
    expect(segs[0]).toMatchObject({ dayISO: DAY, startMin: 600, endMin: 660 });
  });

  it('23:00→01:00 cruza meia-noite → 2 segmentos clipados (CA-11)', () => {
    const segs = splitByDay(
      ev({ start: at(2026, 6, 15, 23, 0), end: at(2026, 6, 16, 1, 0) }),
      [DAY, NEXT],
    );
    expect(segs).toHaveLength(2);
    expect(segs.find((s) => s.dayISO === DAY)).toMatchObject({ startMin: 1380, endMin: 1440 });
    expect(segs.find((s) => s.dayISO === NEXT)).toMatchObject({ startMin: 0, endMin: 60 });
  });

  it('10:00→00:00 (fim na meia-noite) → 1 único segmento, NENHUM fantasma no dia+1 (CA-11.1)', () => {
    const segs = splitByDay(
      ev({ start: at(2026, 6, 15, 10, 0), end: at(2026, 6, 16, 0, 0) }),
      [DAY, NEXT],
    );
    expect(segs).toHaveLength(1);
    expect(segs[0]).toMatchObject({ dayISO: DAY, startMin: 600, endMin: 1440 });
    expect(segs.some((s) => s.dayISO === NEXT)).toBe(false);
  });

  it('evento multi-dia com horário → segmento clipado em cada dia', () => {
    const segs = splitByDay(
      ev({ start: at(2026, 6, 15, 22, 0), end: at(2026, 6, 17, 2, 0) }),
      [DAY, NEXT, '2026-06-17'],
    );
    expect(segs.map((s) => [s.dayISO, s.startMin, s.endMin])).toEqual([
      [DAY, 1320, 1440],
      [NEXT, 0, 1440],
      ['2026-06-17', 0, 120],
    ]);
  });
});

describe('layoutTimed (CA-09/10/12)', () => {
  it('top/height proporcionais ao horário e à duração (CA-09)', () => {
    const [p] = layoutTimed([ev({})], [DAY]);
    expect(p.topPct).toBeCloseTo(600 / 1440, 6);
    expect(p.heightPct).toBeCloseTo(60 / 1440, 6);
    expect(p).toMatchObject({ colIndex: 0, colCount: 1 });
  });

  it('evento de 5min respeita a altura mínima (CA-10)', () => {
    const [p] = layoutTimed([ev({ start: at(2026, 6, 15, 10, 0), end: at(2026, 6, 15, 10, 5) })], [DAY]);
    expect(p.heightPct).toBeCloseTo(MIN_SEGMENT_MINUTES / 1440, 6);
  });

  it('3 eventos sobrepostos → 3 colunas, colIndex 0/1/2 (CA-12)', () => {
    const evs = [0, 1, 2].map((i) =>
      ev({ id: `o${i}`, start: at(2026, 6, 15, 10, 0), end: at(2026, 6, 15, 11, 0) }),
    );
    const ps = layoutTimed(evs, [DAY]);
    expect(ps.every((p) => p.colCount === 3)).toBe(true);
    expect(ps.map((p) => p.colIndex).sort()).toEqual([0, 1, 2]);
  });

  it('eventos borda-a-borda NÃO dividem coluna (CA-12)', () => {
    const evs = [
      ev({ id: 'a', start: at(2026, 6, 15, 10, 0), end: at(2026, 6, 15, 11, 0) }),
      ev({ id: 'b', start: at(2026, 6, 15, 11, 0), end: at(2026, 6, 15, 12, 0) }),
    ];
    const ps = layoutTimed(evs, [DAY]);
    expect(ps.every((p) => p.colCount === 1 && p.colIndex === 0)).toBe(true);
  });

  it('all-day não entra na grade de horas', () => {
    const ps = layoutTimed([ev({ all_day: true, start: at(2026, 6, 15, 0, 0), end: at(2026, 6, 16, 0, 0) })], [DAY]);
    expect(ps).toHaveLength(0);
  });
});

describe('allDayByDay (CA-13/13.1 — end EXCLUSIVO)', () => {
  it('all-day D→D+1 ocupa 1 coluna (só D)', () => {
    const m = allDayByDay(
      [ev({ all_day: true, start: at(2026, 6, 15, 0, 0), end: at(2026, 6, 16, 0, 0) })],
      [DAY, NEXT, '2026-06-17'],
    );
    expect([...m.keys()]).toEqual([DAY]);
    expect(m.get(DAY)).toHaveLength(1);
  });

  it('all-day D→D+3 ocupa 3 colunas (D, D+1, D+2), nunca 4', () => {
    const m = allDayByDay(
      [ev({ all_day: true, start: at(2026, 6, 15, 0, 0), end: at(2026, 6, 18, 0, 0) })],
      ['2026-06-14', DAY, NEXT, '2026-06-17', '2026-06-18'],
    );
    expect(new Set(m.keys())).toEqual(new Set([DAY, NEXT, '2026-06-17']));
    expect(m.has('2026-06-18')).toBe(false);
  });
});

describe('navegação (CA-19.1/19.2/20)', () => {
  it('addMonths com clamp: 2026-01-31 +1 → fevereiro/2026 (não março)', () => {
    const r = addMonths(new Date(2026, 0, 31), 1);
    expect(r.getFullYear()).toBe(2026);
    expect(r.getMonth()).toBe(1); // fevereiro
    expect(r.getDate()).toBe(28);
  });

  it('round-trip next→prev preserva o mês (CA-19.1)', () => {
    const jan31 = new Date(2026, 0, 31);
    const back = addMonths(addMonths(jan31, 1), -1);
    expect(back.getMonth()).toBe(0); // janeiro
  });

  it('monthGridDays: 42 dias começando numa segunda (fevereiro 2026)', () => {
    const days = monthGridDays(new Date(2026, 1, 10));
    expect(days).toHaveLength(42);
    expect(new Date(days[0] + 'T00:00:00').getDay()).toBe(1); // segunda
    expect(days.some((d) => d.startsWith('2026-02'))).toBe(true);
  });

  it('inMonth distingue dias do mês de fora', () => {
    const ref = new Date(2026, 6, 15);
    expect(inMonth('2026-07-01', ref)).toBe(true);
    expect(inMonth('2026-06-30', ref)).toBe(false);
  });

  it('periodTitle do MÊS deriva do refDate, não da grade de 42 dias (CA-19.2)', () => {
    // grade de Julho começa em 29/06 e termina em 09/08, mas o título é "Julho 2026"
    expect(periodTitle('month', new Date(2026, 6, 15))).toBe('Julho 2026');
  });

  it('periodTitle do DIA e da SEMANA', () => {
    expect(periodTitle('day', new Date(2026, 5, 15))).toContain('15');
    expect(periodTitle('day', new Date(2026, 5, 15))).toContain('junho');
    expect(periodTitle('week', new Date(2026, 5, 17))).toContain('2026');
  });

  it('addDays/addWeeks', () => {
    expect(addDays(new Date(2026, 5, 15), 1).getDate()).toBe(16);
    expect(addWeeks(new Date(2026, 5, 15), 1).getDate()).toBe(22);
  });
});

describe('janelas de fetch (CA-05/06/07) e weekDays (CA-20)', () => {
  it('weekDays: segunda→domingo da semana do refDate', () => {
    const days = weekDays(new Date(2026, 5, 17)); // quarta 17/06
    expect(days).toEqual([
      '2026-06-15', '2026-06-16', '2026-06-17', '2026-06-18', '2026-06-19', '2026-06-20', '2026-06-21',
    ]);
  });

  it('weekRange: from=segunda, to=domingo', () => {
    expect(weekRange(new Date(2026, 5, 17))).toEqual({ from: '2026-06-15', to: '2026-06-21' });
  });

  it('dayRange: from=to=dia exibido', () => {
    expect(dayRange(new Date(2026, 5, 17))).toEqual({ from: '2026-06-17', to: '2026-06-17' });
  });

  it('monthRange: primeira e última célula da grade mensal (1 request)', () => {
    const r = monthRange(new Date(2026, 6, 15));
    const days = monthGridDays(new Date(2026, 6, 15));
    expect(r).toEqual({ from: days[0], to: days[41] });
  });
});
