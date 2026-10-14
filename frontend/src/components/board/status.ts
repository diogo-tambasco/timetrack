// status.ts (fase 24): metadados de domínio de status e prioridade da task —
// rótulos em INGLÊS alinhados ao mockup/Notion (dados de domínio), cores via
// tokens (mockup linha 197). Fonte ÚNICA pra board/list/table/overview/popup.
import type { TaskStatus } from '../../lib/api';

export const STATUS_ORDER: TaskStatus[] = ['backlog', 'todo', 'doing', 'done'];

export const STATUS_META: Record<TaskStatus, { label: string; color: string }> = {
  backlog: { label: 'Backlog', color: 'var(--text-3)' },
  todo: { label: 'To do', color: 'var(--text-2)' },
  doing: { label: 'In progress', color: 'var(--accent)' },
  done: { label: 'Done', color: 'var(--p-primor)' },
};

// próximo status no ciclo (chip 1-clique cicla — CA-19/20).
export function nextStatus(s: TaskStatus): TaskStatus {
  const i = STATUS_ORDER.indexOf(s);
  return STATUS_ORDER[(i + 1) % STATUS_ORDER.length];
}

export const PRIORITY_LABEL = ['No priority', 'Low', 'Medium', 'High'] as const;

// ordem do dropdown de prioridade (fase 31): No priority → Low → Medium → High.
export const PRIORITY_ORDER = [0, 1, 2, 3] as const;

// cor do nível de prioridade: High (3) = danger (a cor "quente" sancionada — o
// mockup usa --warn, que não é token aqui; --danger é o equivalente do design);
// demais = text-2 (mockup).
export function priorityColor(p: number): string {
  return p >= 3 ? 'var(--danger)' : 'var(--text-2)';
}

// próximo nível de prioridade no ciclo 0→1→2→3→0 (chip 1-clique cicla).
export function nextPriority(p: number): number {
  return (p + 1) % 4;
}
