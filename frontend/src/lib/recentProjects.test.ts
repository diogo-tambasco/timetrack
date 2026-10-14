// recentProjects.test.ts — fase 23 T-01 (CA-06/06.1): a fila de projetos
// recentes do controle remoto. Função PURA: project_id distintos das entries
// recentes, na ordem do mais recente, sem duplicar, ignorando null. Espelha o
// estilo puro de lib/recent.ts (testável sem React).
import { describe, expect, it } from 'vitest';
import type { TimeEntry } from './api';
import { recentProjectIds } from './recentProjects';

// fábrica mínima de entry: só o que a função lê (project_id + started_at);
// o resto é preenchido pra satisfazer o tipo.
function entry(project_id: string | null, started_at: string): TimeEntry {
  return {
    id: `e-${started_at}-${project_id ?? 'null'}`,
    workspace_id: 'w1',
    project_id,
    task_id: null,
    description: '',
    tags: [],
    started_at,
    stopped_at: null,
    duration_seconds: null,
    source: 'timer',
  };
}

describe('recentProjectIds', () => {
  it('lista vazia → []', () => {
    expect(recentProjectIds([])).toEqual([]);
  });

  it('project_id distintos na ordem do mais recente', () => {
    const entries = [
      entry('p3', '2026-06-22T10:00:00Z'),
      entry('p1', '2026-06-22T09:00:00Z'),
      entry('p2', '2026-06-22T08:00:00Z'),
    ];
    expect(recentProjectIds(entries)).toEqual(['p3', 'p1', 'p2']);
  });

  it('dedup mantém a 1ª ocorrência (a mais recente)', () => {
    const entries = [
      entry('p1', '2026-06-22T12:00:00Z'), // p1 aqui é o mais recente
      entry('p2', '2026-06-22T11:00:00Z'),
      entry('p1', '2026-06-22T09:00:00Z'), // ocorrência antiga de p1 — ignorada
      entry('p3', '2026-06-22T08:00:00Z'),
    ];
    expect(recentProjectIds(entries)).toEqual(['p1', 'p2', 'p3']);
  });

  it('entries com project_id null são ignoradas', () => {
    const entries = [
      entry(null, '2026-06-22T12:00:00Z'),
      entry('p1', '2026-06-22T11:00:00Z'),
      entry(null, '2026-06-22T10:00:00Z'),
    ];
    expect(recentProjectIds(entries)).toEqual(['p1']);
  });

  it('só nulls → []', () => {
    const entries = [entry(null, '2026-06-22T12:00:00Z'), entry(null, '2026-06-22T11:00:00Z')];
    expect(recentProjectIds(entries)).toEqual([]);
  });

  it('ordena por started_at desc mesmo se a lista vier fora de ordem', () => {
    const entries = [
      entry('p2', '2026-06-22T08:00:00Z'),
      entry('p1', '2026-06-22T10:00:00Z'),
      entry('p3', '2026-06-22T09:00:00Z'),
    ];
    expect(recentProjectIds(entries)).toEqual(['p1', 'p3', 'p2']);
  });

  it('undefined (query ainda carregando) → []', () => {
    expect(recentProjectIds(undefined)).toEqual([]);
  });
});
