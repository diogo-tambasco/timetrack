// Testes das funções puras do agrupamento por dia do dashboard (fase 13, T-01).
// O coração do P3 ("virou o dia, sumiram minhas tasks") testa SEM render.
// Dia local por PAREDE no fuso passado (tz-independente — os testes passam tz
// explícito), espelhando a abordagem da fase 11 (calendar-layout).
import { describe, expect, it } from 'vitest';
import { RECENT_DAYS, dayLabel, groupByDay } from './recent';
import type { TimeEntry } from './api';

// fábrica mínima de TimeEntry (só o que groupByDay lê: id + started_at)
function entry(id: string, startedAt: string): TimeEntry {
  return {
    id,
    workspace_id: 'ws',
    project_id: null,
    task_id: null,
    description: id,
    tags: [],
    started_at: startedAt,
    stopped_at: startedAt,
    duration_seconds: 60,
    source: 'manual',
  };
}

describe('RECENT_DAYS', () => {
  it('a janela de recentes é 7 dias (D-01)', () => {
    expect(RECENT_DAYS).toBe(7);
  });
});

describe('dayLabel', () => {
  const today = '2026-06-20';
  it('hoje → "Hoje"', () => {
    expect(dayLabel('2026-06-20', today)).toBe('Hoje');
  });
  it('ontem → "Ontem"', () => {
    expect(dayLabel('2026-06-19', today)).toBe('Ontem');
  });
  it('ontem atravessa virada de mês (clamp de calendário)', () => {
    expect(dayLabel('2026-05-31', '2026-06-01')).toBe('Ontem');
  });
  it('5 dias atrás → rótulo de data pt-BR (não "Hoje"/"Ontem")', () => {
    const label = dayLabel('2026-06-15', today);
    expect(label).not.toBe('Hoje');
    expect(label).not.toBe('Ontem');
    // pt-BR: contém o dia e o mês abreviado (ex.: "15 de jun." / "seg., 15 de jun.")
    expect(label.toLowerCase()).toContain('jun');
    expect(label).toContain('15');
  });
});

describe('groupByDay', () => {
  const tz = 'America/Sao_Paulo'; // UTC−3 fixo (sem DST desde 2019)
  const today = '2026-06-20';

  it('lista vazia → [] (CA-11)', () => {
    expect(groupByDay([], tz, today)).toEqual([]);
  });

  it('entry de hoje cai no grupo "Hoje"', () => {
    const g = groupByDay([entry('a', '2026-06-20T13:00:00-03:00')], tz, today);
    expect(g).toHaveLength(1);
    expect(g[0].dayISO).toBe('2026-06-20');
    expect(g[0].label).toBe('Hoje');
    expect(g[0].items.map((e) => e.id)).toEqual(['a']);
  });

  it('entry de ontem cai no grupo "Ontem"', () => {
    const g = groupByDay([entry('a', '2026-06-19T09:00:00-03:00')], tz, today);
    expect(g[0].dayISO).toBe('2026-06-19');
    expect(g[0].label).toBe('Ontem');
  });

  it('grupos em ordem decrescente: hoje antes de ontem antes de antes', () => {
    const g = groupByDay(
      [
        entry('antes', '2026-06-15T10:00:00-03:00'),
        entry('hoje', '2026-06-20T10:00:00-03:00'),
        entry('ontem', '2026-06-19T10:00:00-03:00'),
      ],
      tz,
      today,
    );
    expect(g.map((x) => x.dayISO)).toEqual(['2026-06-20', '2026-06-19', '2026-06-15']);
  });

  it('dois itens no mesmo dia ficam juntos, mais recente primeiro', () => {
    const g = groupByDay(
      [
        entry('manha', '2026-06-19T08:00:00-03:00'),
        entry('tarde', '2026-06-19T16:00:00-03:00'),
      ],
      tz,
      today,
    );
    expect(g).toHaveLength(1);
    expect(g[0].items.map((e) => e.id)).toEqual(['tarde', 'manha']);
  });

  it('FUSO: domingo 23:30 em São Paulo cai no domingo local, não na segunda UTC', () => {
    // 2026-06-15T02:30:00Z = segunda 02:30 UTC = DOMINGO 23:30 em SP (UTC−3)
    const iso = '2026-06-15T02:30:00Z';
    const spGroups = groupByDay([entry('a', iso)], 'America/Sao_Paulo', '2026-06-16');
    expect(spGroups[0].dayISO).toBe('2026-06-14'); // domingo local SP
    // o MESMO instante em UTC cai na segunda
    const utcGroups = groupByDay([entry('a', iso)], 'UTC', '2026-06-16');
    expect(utcGroups[0].dayISO).toBe('2026-06-15'); // segunda UTC
  });
});
