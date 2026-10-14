// MyTasks (fase 18 T-10, CA-18/19): "Minhas tasks" (assigned) e "Pedidas por
// mim" (requested) — visões CROSS-WORKSPACE (rotas por-usuário /api/v1/tasks/
// assigned|requested). Cada item mostra a task + projeto (chip cor) + workspace
// + cliente. A propagação ao vivo está FORA de escopo (spec §7): a lista
// converge no reload; a notificação chega ao vivo pelo NoticeBanner.
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Icon } from '../../components/icons';
import ProjectChip from '../../components/projects/ProjectChip';
import {
  getAssignedTasks,
  getRequestedTasks,
  type AssignedTask,
  type TaskStatus,
} from '../../lib/api';

const statusLabel: Record<TaskStatus, string> = {
  backlog: 'Backlog',
  todo: 'A fazer',
  doing: 'Fazendo',
  done: 'Feito',
};

type Tab = 'assigned' | 'requested';

export default function MyTasks() {
  const [tab, setTab] = useState<Tab>('assigned');

  const assigned = useQuery({ queryKey: ['tasks', 'assigned'], queryFn: getAssignedTasks });
  const requested = useQuery({ queryKey: ['tasks', 'requested'], queryFn: getRequestedTasks });
  const active = tab === 'assigned' ? assigned : requested;
  const tasks = active.data ?? [];

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 20, height: '100%' }}>
      <div>
        <p className="eyebrow" style={{ margin: 0 }}>Minhas tasks</p>
        <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>
          Minha fila
        </h1>
      </div>

      <div role="tablist" aria-label="Visão das tasks" style={{ display: 'flex', gap: 8 }}>
        <TabButton id="assigned" current={tab} onClick={setTab} label="Minhas tasks" />
        <TabButton id="requested" current={tab} onClick={setTab} label="Pedidas por mim" />
      </div>

      {active.isLoading ? (
        <p className="faint">Carregando…</p>
      ) : tasks.length === 0 ? (
        <div className="tt-card" style={{ padding: 24, textAlign: 'center', color: 'var(--text-3)' }}>
          <span style={{ display: 'inline-flex', marginBottom: 8, color: 'var(--text-3)' }} aria-hidden>
            <Icon.list size={22} />
          </span>
          <p style={{ margin: 0 }}>
            {tab === 'assigned'
              ? 'Nenhuma task atribuída a você por enquanto.'
              : 'Você ainda não pediu nenhuma task.'}
          </p>
        </div>
      ) : (
        <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 10 }}>
          {tasks.map((t) => (
            <TaskRow key={t.id} task={t} />
          ))}
        </ul>
      )}
    </section>
  );
}

function TabButton({
  id,
  current,
  onClick,
  label,
}: {
  id: Tab;
  current: Tab;
  onClick: (t: Tab) => void;
  label: string;
}) {
  const active = current === id;
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={() => onClick(id)}
      className={active ? 'tt-btn tt-btn--accent' : 'tt-btn tt-btn--ghost'}
    >
      {label}
    </button>
  );
}

function TaskRow({ task }: { task: AssignedTask }) {
  return (
    <li
      className="tt-card"
      data-testid="mytask-row"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 14,
        padding: '12px 16px',
      }}
    >
      <div style={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span style={{ fontSize: 14, fontWeight: 500 }}>{task.name}</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', fontSize: 12, color: 'var(--text-3)' }}>
          <ProjectChip name={task.project.name} color={task.project.color} />
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            <Icon.users size={13} /> {task.workspace.name}
          </span>
          {task.client && (
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              <Icon.briefcase size={13} /> {task.client}
            </span>
          )}
        </span>
      </div>
      <span className="tt-chip" style={{ background: 'var(--surface-2)', flex: '0 0 auto' }}>
        {statusLabel[task.status]}
      </span>
    </li>
  );
}
