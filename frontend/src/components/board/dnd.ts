// dnd.ts (fase 24, CA-17): a lógica do drop entre colunas do board é uma função
// PURA testável (Playwright HTML5 drag é flaky — CA-28; o jsdom testa esta).
// Recebe o id arrastado, o status de destino e a lista de tasks; devolve a
// mutação de PATCH (status + position no FIM da coluna destino — D-04) ou null se
// nada muda (drop na mesma coluna = no-op no MVP).
import type { BoardTask, TaskStatus } from '../../lib/api';

export type DropMutation = { id: string; patch: { status: TaskStatus; position: number } };

export function dropStatus(
  dragId: string,
  target: TaskStatus,
  tasks: BoardTask[],
): DropMutation | null {
  const dragged = tasks.find((t) => t.id === dragId);
  if (!dragged) return null;
  if (dragged.status === target) return null; // mesma coluna = no-op (D-04)
  // position no FIM da coluna destino = max(position das tasks do target) + 1.
  const inTarget = tasks.filter((t) => t.status === target);
  const maxPos = inTarget.reduce((m, t) => Math.max(m, t.position), -1);
  return { id: dragId, patch: { status: target, position: maxPos + 1 } };
}

// tasksByStatus: ordena por position asc (desempate created_at) cada coluna.
export function tasksByStatus(tasks: BoardTask[], status: TaskStatus): BoardTask[] {
  return tasks
    .filter((t) => t.status === status)
    .sort((a, b) => a.position - b.position || a.created_at.localeCompare(b.created_at));
}
