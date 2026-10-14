// dnd.test.ts — fase 24 T-11 (CA-17): dropStatus puro (Playwright drag é flaky →
// o DnD vira teste de unidade aqui). tasksByStatus ordena por position.
import { describe, it, expect } from 'vitest';
import { dropStatus, tasksByStatus } from './dnd';
import { mkTask } from './testFixtures';

const tasks = [
  mkTask({ id: 'a', status: 'todo', position: 1 }),
  mkTask({ id: 'b', status: 'todo', position: 0 }),
  mkTask({ id: 'c', status: 'doing', position: 0 }),
];

describe('dropStatus (CA-17)', () => {
  it('mover pra outra coluna → patch status + position no fim (max+1)', () => {
    // doing tem 1 task (position 0) → o drop vai pra position 1.
    expect(dropStatus('a', 'doing', tasks)).toEqual({
      id: 'a',
      patch: { status: 'doing', position: 1 },
    });
  });

  it('coluna vazia → position 0', () => {
    expect(dropStatus('a', 'done', tasks)).toEqual({
      id: 'a',
      patch: { status: 'done', position: 0 },
    });
  });

  it('drop na MESMA coluna → no-op (null)', () => {
    expect(dropStatus('a', 'todo', tasks)).toBeNull();
  });

  it('id inexistente → null', () => {
    expect(dropStatus('zzz', 'done', tasks)).toBeNull();
  });
});

describe('tasksByStatus ordena por position', () => {
  it('todo vem [b(0), a(1)]', () => {
    const ord = tasksByStatus(tasks, 'todo').map((t) => t.id);
    expect(ord).toEqual(['b', 'a']);
  });
});
