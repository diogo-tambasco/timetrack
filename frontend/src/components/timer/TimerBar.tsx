// Barra de timer global (herói do cockpit — TimerBar de parts.jsx).
// Fase 03 (T-15): timer real por cima do useTimerStore — ociosa inicia um
// count up; rodando exibe descrição + chip do projeto + dígitos mono e o
// stop pulsante (CA-31). O estado de pausa vira o BreakOverlay (T-16).
import { useState } from 'react';
import { Icon } from '../icons';
import { useCurrentWorkspace } from '../../lib/auth';
import { fmtClock } from '../../lib/format';
import { useProjects } from '../../lib/queries/projects';
import { useTasks } from '../../lib/queries/tasks';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';
import BreakOverlay from './BreakOverlay';

// fase 22: height/flex-wrap moram na classe .tt-timerbar (não inline) pra o
// modo mobile (< md) poder reflowar sem ser sobrescrito por inline. ≥768 a
// classe reproduz EXATAMENTE o layout de hoje (height 66, linha única).
const barStyle: React.CSSProperties = {
  gap: 'var(--gap)',
  padding: '0 var(--pad)',
  borderLeft: 'none',
  borderTop: 'none',
  borderRight: 'none',
};

const inputStyle: React.CSSProperties = {
  flex: 1,
  minWidth: 0,
  background: 'transparent',
  border: 'none',
  outline: 'none',
  color: 'var(--text-1)',
  fontFamily: 'var(--font-ui)',
  fontWeight: 500,
  fontSize: 15,
  letterSpacing: '-.01em',
};

// fase 22: maxWidth na classe .tt-timer-select (não inline) pra encolher < md.
const selectStyle: React.CSSProperties = {
  background: 'var(--surface-2)',
  color: 'var(--text-2)',
  border: '1px solid var(--border)',
  borderRadius: 8,
  padding: '6px 9px',
  fontFamily: 'var(--font-ui)',
  fontSize: 12.5,
};

// fase 22: minWidth da classe .tt-timer-digits (não inline) pra encolher < md.
const digitsStyle: React.CSSProperties = {
  fontSize: 22,
  fontWeight: 600,
  letterSpacing: '-.02em',
  color: 'var(--text-1)',
  fontVariantNumeric: 'tabular-nums',
  textAlign: 'right',
};

export default function TimerBar() {
  const ws = useCurrentWorkspace();
  const state = useTimerStore((s) => s.state);

  if (state?.mode === 'break') {
    return (
      <header data-testid="timerbar" className="tt-panel tt-timerbar" style={barStyle}>
        <BreakOverlay />
      </header>
    );
  }
  return (
    <header data-testid="timerbar" className="tt-panel tt-timerbar" style={barStyle}>
      {state ? <RunningBar /> : <IdleBar wsID={ws?.id ?? null} />}
    </header>
  );
}

// IdleBar — descrição + projeto/task + play (CA-04/20: projeto e task são
// opcionais; o start é sempre count up — countdown/pomodoro têm UI própria).
function IdleBar({ wsID }: { wsID: string | null }) {
  const [description, setDescription] = useState('');
  const [projectID, setProjectID] = useState('');
  const [taskID, setTaskID] = useState('');
  const start = useTimerStore((s) => s.start);

  const projectsQ = useProjects(wsID ?? '');
  const tasksQ = useTasks(wsID ?? '', projectID ? { project_id: projectID } : undefined);
  const projects = (projectsQ.data ?? []).filter((p) => !p.archived_at);
  const tasks = projectID ? (tasksQ.data ?? []) : [];

  const onStart = () => {
    if (!wsID) return;
    void start({
      workspace_id: wsID,
      description,
      project_id: projectID || null,
      task_id: taskID || null,
      tags: [],
    });
    setDescription('');
    setProjectID('');
    setTaskID('');
  };

  return (
    <>
      <input
        placeholder="No que você está trabalhando?"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && onStart()}
        style={inputStyle}
      />
      <select
        aria-label="Projeto"
        className="tt-timer-select"
        value={projectID}
        onChange={(e) => {
          setProjectID(e.target.value);
          setTaskID('');
        }}
        style={selectStyle}
      >
        <option value="">Sem projeto</option>
        {projects.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>
      {projectID && tasks.length > 0 && (
        <select
          aria-label="Task"
          className="tt-timer-select"
          value={taskID}
          onChange={(e) => setTaskID(e.target.value)}
          style={selectStyle}
        >
          <option value="">Sem task</option>
          {tasks.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      )}
      <div className="mono tt-timer-digits" style={digitsStyle}>
        0:00:00
      </div>
      <button className="tt-iconbtn tt-iconbtn--go" title="Iniciar" aria-label="Iniciar" onClick={onStart}>
        <Icon.play size={15} />
      </button>
    </>
  );
}

// RunningBar — CA-31: descrição (somente leitura — editar entry rodando é
// 409 no backend), chip do projeto, dígitos derivados localmente e stop
// pulsante. Countdown/pomodoro mostram o tempo RESTANTE do alvo.
function RunningBar() {
  const ws = useCurrentWorkspace();
  const state = useTimerStore((s) => s.state);
  const stop = useTimerStore((s) => s.stop);
  const pause = useTimerStore((s) => s.pause);
  const { elapsed, remaining } = useTimerClock();
  // archived:true — a entry rodando pode apontar pra projeto arquivado
  // (arquivar com timer rodando / continuar entry antiga) e o chip do
  // CA-31 não pode sumir
  const projectsQ = useProjects(ws?.id ?? '', { archived: true });

  if (!state?.entry) return null;
  const project = (projectsQ.data ?? []).find((p) => p.id === state.entry?.project_id);
  const shown = state.mode === 'countup' ? elapsed : (remaining ?? elapsed);

  return (
    <>
      <input value={state.entry.description} readOnly style={inputStyle} />
      {project && (
        <span className="tt-chip">
          <i className="tt-dot" style={{ background: project.color }} />
          {project.name}
        </span>
      )}
      {/* fase 30 (CA-16): chip da task quando a entry rodando aponta pra uma —
          object permanence do que se cronometra agora. Componente FILHO só
          montado com task_id → o caminho SEM task fica byte-idêntico (CA-18/
          RNF-04: nenhuma query de tasks dispara sem task). Sem ícone (30-H1:
          Icon.board não usado — espelha o chip de projeto, só texto). */}
      {state.entry.task_id && ws?.id && (
        <RunningTaskChip wsId={ws.id} projectId={state.entry.project_id} taskId={state.entry.task_id} />
      )}
      <div className="mono tt-timer-digits" style={digitsStyle}>
        {fmtClock(shown)}
      </div>
      <button
        className="tt-iconbtn"
        title="Pausar"
        aria-label="Pausar"
        style={{ background: 'var(--surface-3)', color: 'var(--text-1)' }}
        onClick={() => void pause()}
      >
        <Icon.pause size={15} />
      </button>
      <button
        className="tt-iconbtn tt-iconbtn--stop tt-pulse"
        title="Parar"
        aria-label="Parar"
        onClick={() => void stop()}
      >
        <Icon.stop size={15} />
      </button>
    </>
  );
}

// RunningTaskChip (fase 30, CA-16/D-08): resolve o nome da task da entry rodando
// por uma query de tasks JÁ cacheada (mesmo padrão do chip de projeto) e o
// exibe como `tt-chip` só com texto (sem ícone — 30-H1). Componente FILHO: só
// monta quando há task_id, então o caminho sem-task não dispara query (CA-18/
// RNF-04). Nome não resolve → não renderiza (degrada — nunca quebra a TimerBar
// compartilhada com o TRACKING).
function RunningTaskChip({
  wsId,
  projectId,
  taskId,
}: {
  wsId: string;
  projectId: string | null;
  taskId: string;
}) {
  const tasksQ = useTasks(wsId, projectId ? { project_id: projectId } : undefined);
  const task = (tasksQ.data ?? []).find((t) => t.id === taskId);
  if (!task) return null;
  return <span className="tt-chip">{task.name}</span>;
}
