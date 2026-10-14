// EntryModal (T-18) — nova entrada / edição manual (CA-14/15 na UI):
// descrição, projeto, task, tags e início/fim. Form inválido nunca chama a
// API; editar envia PATCH só com o diff (padrão do ProjectModal da fase
// 02). Referência visual: botão "Nova entrada" + cards do cockpit.jsx.
import { useState, type FormEvent, type ReactNode } from 'react';
import type { EntryInput, TimeEntry } from '../../lib/api';
import { Icon } from '../icons';
import { useProjects } from '../../lib/queries/projects';
import { useTasks } from '../../lib/queries/tasks';
import { useCreateEntry, usePatchEntry } from '../../lib/queries/entries';

type EntryModalProps = {
  wsId: string;
  // presente = modo Editar
  entry?: TimeEntry | null;
  onClose: () => void;
};

const inputStyle = {
  background: 'var(--surface-2)',
  border: '1px solid var(--border-2)',
  borderRadius: 'var(--r-md)',
  padding: '11px 13px',
  fontSize: 14,
  color: 'var(--text-1)',
  fontFamily: 'var(--font-ui)',
  outline: 'none',
  width: '100%',
  boxSizing: 'border-box',
} as const;

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
      <label htmlFor={id} style={{ fontSize: 12.5, fontWeight: 540, color: 'var(--text-2)' }}>
        {label}
      </label>
      {children}
    </div>
  );
}

// ISO (UTC) → valor do input datetime-local no fuso do navegador
function toLocalInput(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default function EntryModal({ wsId, entry = null, onClose }: EntryModalProps) {
  const isEdit = entry !== null;
  const criar = useCreateEntry(wsId);
  const editar = usePatchEntry(wsId);

  const [description, setDescription] = useState(entry?.description ?? '');
  const [projectID, setProjectID] = useState(entry?.project_id ?? '');
  const [taskID, setTaskID] = useState(entry?.task_id ?? '');
  const [tags, setTags] = useState((entry?.tags ?? []).join(', '));
  const [started, setStarted] = useState(toLocalInput(entry?.started_at ?? null));
  const [stopped, setStopped] = useState(toLocalInput(entry?.stopped_at ?? null));
  const [erro, setErro] = useState<string | null>(null);

  // archived:true + filtro na UI: o select mostra os ativos E o projeto
  // vinculado da entry mesmo arquivado (senão o vínculo aparecia como
  // "Sem projeto" na edição)
  const projectsQ = useProjects(wsId, { archived: true });
  const tasksQ = useTasks(wsId, projectID ? { project_id: projectID } : undefined);
  const projects = (projectsQ.data ?? []).filter((p) => !p.archived_at || p.id === entry?.project_id);
  const tasks = projectID ? (tasksQ.data ?? []) : [];

  const parseTags = (raw: string) =>
    raw
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setErro(null);
    // o fim/início são obrigatórios e o fim vem depois — espelho do CA-24,
    // barrado ANTES de qualquer chamada
    if (!started || !stopped) {
      setErro('Preencha início e fim.');
      return;
    }
    const startISO = new Date(started).toISOString();
    const stopISO = new Date(stopped).toISOString();
    if (Date.parse(stopISO) <= Date.parse(startISO)) {
      setErro('O fim deve ser depois do início.');
      return;
    }

    if (!isEdit) {
      const input: EntryInput = {
        description,
        project_id: projectID || null,
        task_id: taskID || null,
        tags: parseTags(tags),
        started_at: startISO,
        stopped_at: stopISO,
      };
      criar.mutate(input, {
        onSuccess: onClose,
        onError: (err) => setErro(err.message),
      });
      return;
    }

    // diff: PATCH só carrega o que mudou. Horários comparam no GRÃO DO
    // INPUT (minuto): o datetime-local descarta os segundos do servidor e
    // comparar ISO contra ISO gerava diff falso que deslocava os horários
    // de toda entry do timer a cada edição de descrição/tags.
    const patch: Partial<EntryInput> = {};
    if (description !== entry.description) patch.description = description;
    if ((projectID || null) !== entry.project_id) patch.project_id = projectID || null;
    if ((taskID || null) !== entry.task_id) patch.task_id = taskID || null;
    const novasTags = parseTags(tags);
    if (novasTags.join(',') !== entry.tags.join(',')) patch.tags = novasTags;
    if (started !== toLocalInput(entry.started_at)) patch.started_at = startISO;
    if (entry.stopped_at === null || stopped !== toLocalInput(entry.stopped_at)) {
      patch.stopped_at = stopISO;
    }
    if (Object.keys(patch).length === 0) {
      onClose();
      return;
    }
    editar.mutate(
      { id: entry.id, patch },
      { onSuccess: onClose, onError: (err) => setErro(err.message) },
    );
  };

  const titulo = isEdit ? 'Editar entrada' : 'Nova entrada';

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 60, display: 'grid', placeItems: 'center' }}>
      <div
        aria-hidden
        onClick={onClose}
        style={{
          position: 'absolute',
          inset: 0,
          background: 'rgba(0,0,0,.45)',
          backdropFilter: 'blur(2px)',
        }}
      />
      <form
        role="dialog"
        aria-modal="true"
        aria-label={titulo}
        onSubmit={submit}
        className="tt-card"
        style={{
          position: 'relative',
          width: 520,
          maxWidth: 'calc(100vw - 32px)',
          maxHeight: '92%',
          background: 'var(--surface-1)',
          boxShadow: 'var(--shadow)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '18px 22px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <div style={{ fontSize: 17, fontWeight: 600 }}>{titulo}</div>
          <button
            type="button"
            aria-label="Fechar"
            className="tt-iconbtn"
            onClick={onClose}
            style={{ width: 32, height: 32, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.x size={16} />
          </button>
        </div>

        <div style={{ padding: 22, display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
          <Field id="em-desc" label="Descrição">
            <input
              id="em-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Ex.: Call de alinhamento"
              autoFocus
              style={inputStyle}
            />
          </Field>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field id="em-project" label="Projeto">
              <select
                id="em-project"
                value={projectID}
                onChange={(e) => {
                  setProjectID(e.target.value);
                  setTaskID('');
                }}
                style={inputStyle}
              >
                <option value="">Sem projeto</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </Field>
            <Field id="em-task" label="Task">
              <select
                id="em-task"
                value={taskID}
                onChange={(e) => setTaskID(e.target.value)}
                disabled={!projectID}
                style={inputStyle}
              >
                <option value="">Sem task</option>
                {tasks.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <Field id="em-tags" label="Tags (separadas por vírgula)">
            <input
              id="em-tags"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="deep-work, lote-14"
              style={inputStyle}
            />
          </Field>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field id="em-start" label="Início">
              <input
                id="em-start"
                type="datetime-local"
                value={started}
                onChange={(e) => setStarted(e.target.value)}
                style={inputStyle}
              />
            </Field>
            <Field id="em-stop" label="Fim">
              <input
                id="em-stop"
                type="datetime-local"
                value={stopped}
                onChange={(e) => setStopped(e.target.value)}
                style={inputStyle}
              />
            </Field>
          </div>

          {erro && (
            <span role="alert" style={{ color: 'var(--danger)', fontSize: 12.5 }}>
              {erro}
            </span>
          )}
        </div>

        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 10,
            padding: '16px 22px',
            borderTop: '1px solid var(--border)',
          }}
        >
          <button type="button" className="tt-btn tt-btn--ghost" onClick={onClose}>
            Cancelar
          </button>
          <button type="submit" className="tt-btn tt-btn--accent">
            Salvar
          </button>
        </div>
      </form>
    </div>
  );
}
