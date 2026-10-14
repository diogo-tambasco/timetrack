// TaskModal (fase 12 T-10, CA-21): criar/editar task (nome, status [edição],
// data, notas, SELECT de objetivo com "Geral"). Usa os hooks de task da fase 02
// (invalidam tasksKey, sem regressão); o board reflete via onSaved (o ProjectBoard
// invalida o boardKey — plan §6). objective_id "" no select = Geral (null).
import { useState, type FormEvent } from 'react';
import type { BoardTask, TaskStatus } from '../../lib/api';
import { Icon } from '../icons';
import { useCreateTask, usePatchTask } from '../../lib/queries/tasks';
import { useWorkspaceMembers } from '../../lib/queries/workspace';
import TaskThread from './TaskThread';
import ReminderButton from './ReminderButton';

const inputStyle = {
  background: 'var(--surface-2)',
  border: '1px solid var(--border-2)',
  borderRadius: 8,
  color: 'var(--text-1)',
  font: 'inherit',
  fontSize: 13,
  padding: '8px 10px',
  width: '100%',
} as const;

const statusLabel: Record<TaskStatus, string> = {
  backlog: 'Backlog',
  todo: 'A fazer',
  doing: 'Fazendo',
  done: 'Feito',
};

export default function TaskModal({
  wsId,
  projectId,
  objectives,
  task = null,
  defaultObjectiveId = null,
  canComment = false,
  onClose,
  onSaved,
}: {
  wsId: string;
  projectId: string;
  objectives: { id: string; name: string }[];
  task?: BoardTask | null;
  defaultObjectiveId?: string | null;
  // fase 19: pode comentar nesta task? can_write || (assignee === currentUser).
  canComment?: boolean;
  onClose: () => void;
  onSaved?: () => void;
}) {
  const editing = task !== null;
  const [nome, setNome] = useState(task?.name ?? '');
  const [status, setStatus] = useState<TaskStatus>(task?.status ?? 'todo');
  const [data, setData] = useState(task?.due_date ?? '');
  const [notas, setNotas] = useState(task?.notes ?? '');
  const [objId, setObjId] = useState<string>(task?.objective_id ?? defaultObjectiveId ?? '');
  // fase 18: Responsável (assignee) e Solicitante (requested_by); "" no select = null.
  const [assignee, setAssignee] = useState<string>(task?.assignee ?? '');
  const [requestedBy, setRequestedBy] = useState<string>(task?.requested_by ?? '');
  const [erro, setErro] = useState('');

  const criar = useCreateTask(wsId);
  const editar = usePatchTask(wsId);
  const members = useWorkspaceMembers(wsId).data ?? [];
  const titulo = editing ? 'Editar task' : 'Nova task';

  const done = () => {
    onSaved?.();
    onClose();
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const name = nome.trim();
    if (!name) {
      setErro('Dê um nome à task.');
      return;
    }
    setErro('');
    const objectiveId = objId === '' ? null : objId;
    const assigneeId = assignee === '' ? null : assignee;
    const requestedById = requestedBy === '' ? null : requestedBy;

    if (editing && task) {
      editar.mutate(
        {
          id: task.id,
          patch: {
            name,
            status,
            objective_id: objectiveId,
            due_date: data === '' ? null : data,
            notes: notas,
            assignee: assigneeId,
            requested_by: requestedById,
          },
        },
        { onSuccess: done },
      );
    } else {
      criar.mutate(
        {
          project_id: projectId,
          name,
          objective_id: objectiveId,
          ...(data !== '' ? { due_date: data } : {}),
          notes: notas,
          assignee: assigneeId,
          requested_by: requestedById,
        },
        { onSuccess: done },
      );
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 60, display: 'grid', placeItems: 'center' }}>
      <div
        aria-hidden
        onClick={onClose}
        style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(2px)' }}
      />
      <form
        role="dialog"
        aria-modal="true"
        aria-label={titulo}
        onSubmit={submit}
        className="tt-card"
        style={{
          position: 'relative',
          width: 480,
          maxWidth: 'calc(100vw - 32px)',
          // fase 19: com a thread o modal pode passar da viewport — rola o
          // conteúdo pra alcançar o input/botão de comentar e o rodapé.
          maxHeight: 'calc(100vh - 32px)',
          overflowY: 'auto',
          background: 'var(--surface-1)',
          boxShadow: 'var(--shadow)',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <div style={{ fontSize: 16, fontWeight: 600 }}>{titulo}</div>
          <button
            type="button"
            aria-label="Fechar"
            className="tt-iconbtn"
            onClick={onClose}
            style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.x size={16} />
          </button>
        </div>

        <div style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Nome</span>
            <input
              aria-label="Nome da task"
              value={nome}
              onChange={(e) => setNome(e.target.value)}
              placeholder="O que precisa ser feito?"
              style={inputStyle}
            />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Objetivo</span>
            <select
              aria-label="Objetivo"
              value={objId}
              onChange={(e) => setObjId(e.target.value)}
              style={inputStyle}
            >
              <option value="">Geral (sem objetivo)</option>
              {objectives.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Responsável</span>
            <select
              aria-label="Responsável"
              value={assignee}
              onChange={(e) => setAssignee(e.target.value)}
              style={inputStyle}
            >
              <option value="">— sem responsável</option>
              {members.map((m) => (
                <option key={m.user_id} value={m.user_id}>
                  {m.name}
                </option>
              ))}
            </select>
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Solicitante</span>
            <select
              aria-label="Solicitante"
              value={requestedBy}
              onChange={(e) => setRequestedBy(e.target.value)}
              style={inputStyle}
            >
              <option value="">— ninguém</option>
              {members.map((m) => (
                <option key={m.user_id} value={m.user_id}>
                  {m.name}
                </option>
              ))}
            </select>
          </label>

          {editing && (
            <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
              <span className="faint">Status</span>
              <select
                aria-label="Status da task"
                value={status}
                onChange={(e) => setStatus(e.target.value as TaskStatus)}
                style={inputStyle}
              >
                {(['todo', 'doing', 'done'] as TaskStatus[]).map((s) => (
                  <option key={s} value={s}>
                    {statusLabel[s]}
                  </option>
                ))}
              </select>
            </label>
          )}

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Data (opcional)</span>
            <input
              aria-label="Data da task"
              type="date"
              value={data}
              onChange={(e) => setData(e.target.value)}
              style={inputStyle}
            />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Notas (opcional)</span>
            <textarea
              aria-label="Notas da task"
              value={notas}
              onChange={(e) => setNotas(e.target.value)}
              rows={2}
              style={{ ...inputStyle, resize: 'vertical' }}
            />
          </label>

          {erro && (
            <p role="alert" style={{ margin: 0, fontSize: 12.5, color: 'var(--danger)' }}>
              {erro}
            </p>
          )}

          {/* fase 20: lembrete manual — SÓ no modo Editar (a task existe). O
              picker usa a due_date corrente pro preset "no prazo". */}
          {editing && task && (
            <ReminderButton wsId={wsId} taskId={task.id} dueDate={data === '' ? null : data} />
          )}
        </div>

        {/* fase 19: thread de comentários — SÓ no modo Editar (a task existe). */}
        {editing && task && (
          <TaskThread wsId={wsId} taskId={task.id} canComment={canComment} />
        )}

        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 8,
            padding: '14px 20px',
            borderTop: '1px solid var(--border)',
          }}
        >
          <button type="button" className="tt-btn tt-btn--ghost" onClick={onClose}>
            Cancelar
          </button>
          <button type="submit" className="tt-btn tt-btn--accent">
            {editing ? 'Salvar' : 'Criar task'}
          </button>
        </div>
      </form>
    </div>
  );
}
