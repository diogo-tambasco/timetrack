// SubtaskSidebar (fase 24 T-13 / fase 31): "Task-goals" — os objetivos DESTA task
// (conceito tipo lembretes do iPhone: cada item é uma meta da task). useSubtasks
// (taskId); input "add a goal" (Enter → cria + limpa + mantém foco); checkbox
// done; cabeçalho feitas/total. Vazio → "No goals yet." Gating: can_write=false →
// sem add/toggles (CA-25). Fase 31: removida a estrela is_goal (TODOS já são
// goals da task — o conceito de "sub-task com estrela" some).
import { useRef, useState } from 'react';
import { Icon } from '../icons';
import {
  useSubtasks,
  useCreateSubtask,
  usePatchSubtask,
  useDeleteSubtask,
} from '../../lib/queries/subtasks';

export default function SubtaskSidebar({
  wsId,
  taskId,
  projectId,
  canWrite,
}: {
  wsId: string;
  taskId: string;
  projectId: string;
  canWrite: boolean;
}) {
  const { data: subs = [] } = useSubtasks(wsId, taskId);
  const create = useCreateSubtask(wsId, taskId, projectId);
  const patch = usePatchSubtask(wsId, taskId, projectId);
  const del = useDeleteSubtask(wsId, taskId, projectId);
  const [draft, setDraft] = useState('');
  // o "x" de remover só aparece no hover do item (declutter — fase 31).
  const [hoverId, setHoverId] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const total = subs.length;
  const done = subs.filter((s) => s.done).length;

  const add = () => {
    const v = draft.trim();
    if (v === '') return;
    create.mutate(v, {
      onSuccess: () => {
        setDraft('');
        inputRef.current?.focus();
      },
    });
  };

  return (
    <aside
      data-testid="subtask-sidebar"
      style={{ display: 'flex', flexDirection: 'column', gap: 8, minWidth: 0 }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon.star size={13} style={{ color: 'var(--accent)' }} />
        <span style={{ fontSize: 13, fontWeight: 600 }}>Task-goals</span>
        <span className="mono faint" style={{ fontSize: 11.5 }}>
          {done}/{total}
        </span>
      </div>

      {subs.length === 0 && (
        <p className="faint" style={{ fontSize: 12, margin: 0 }}>
          No goals yet.
        </p>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {subs.map((s) => (
          <div
            key={s.id}
            onMouseEnter={() => setHoverId(s.id)}
            onMouseLeave={() => setHoverId((h) => (h === s.id ? null : h))}
            style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '3px 0' }}
          >
            <button
              type="button"
              role="checkbox"
              aria-checked={s.done}
              aria-label={`Concluir ${s.title}`}
              disabled={!canWrite}
              onClick={() => canWrite && patch.mutate({ id: s.id, patch: { done: !s.done } })}
              style={{
                width: 16,
                height: 16,
                borderRadius: 4,
                border: '1.5px solid var(--border-2)',
                background: s.done ? 'var(--accent)' : 'transparent',
                color: 'var(--accent-ink)',
                cursor: canWrite ? 'pointer' : 'default',
                display: 'grid',
                placeItems: 'center',
                flex: '0 0 auto',
              }}
            >
              {s.done && <Icon.check size={11} sw={2.6} />}
            </button>
            <span
              style={{
                flex: 1,
                fontSize: 12.5,
                textDecoration: s.done ? 'line-through' : 'none',
                color: s.done ? 'var(--text-3)' : 'var(--text-1)',
                wordBreak: 'break-word',
              }}
            >
              {s.title}
            </span>
            {canWrite && (
              <button
                type="button"
                aria-label={`Remover ${s.title}`}
                onClick={() => del.mutate(s.id)}
                className="tt-iconbtn"
                style={{
                  width: 22,
                  height: 22,
                  background: 'transparent',
                  color: 'var(--text-3)',
                  flex: '0 0 auto',
                  opacity: hoverId === s.id ? 1 : 0,
                  transition: 'opacity .12s',
                }}
              >
                <Icon.x size={12} />
              </button>
            )}
          </div>
        ))}
      </div>

      {canWrite && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '3px 0', marginTop: 2 }}>
          <Icon.plus size={13} sw={2.2} style={{ color: 'var(--text-3)', flex: '0 0 auto' }} />
          <input
            ref={inputRef}
            aria-label="Adicionar task-goal"
            placeholder="New goal…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                add();
              }
            }}
            style={{
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'var(--text-1)',
              font: 'inherit',
              fontSize: 12.5,
              padding: 0,
              flex: 1,
              minWidth: 0,
            }}
          />
        </div>
      )}
    </aside>
  );
}
