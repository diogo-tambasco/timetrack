// TaskTimeControl (fase 30, CA-10..15/20): o controle de tempo do popup de task
// — o ELO entre os dois mundos (cronometrar PUXANDO a task; MILESTONE-3 §2).
// Substitui o bloco Time inline mock da fase 24 (mesmo elemento
// data-testid="task-start" — decisão de coordenação B). AUTOSSUFICIENTE (D-03):
// lê o useTimerStore/useTimerClock GLOBAIS — zero fetch próprio, zero 2º
// attachTimerSync (padrão da fase 23/RNF-04). Estados:
//   • idle nesta task + canWrite → Start (countup puxando a task; CA-10/10.1)
//   • rodando NESTA task        → Parar + tempo ao vivo (tracked+elapsed; CA-11/11.1)
//   • idle + !canWrite          → só leitura (sem botão; CA-14)
//   • rodando + !canWrite (borda) → Parar NÃO some (não prende o timer; CA-14/D-09)
//   • sem project_id            → Start inerte (CA-15)
// Fonte única da verdade = state.entry.task_id === task.id (D-05); o tempo ao
// vivo NÃO persiste sozinho — converge ao tracked_seconds quando a entry fecha e
// o /board refaz a query (a invalidação mora no ProjectDetail — CA-13/30-H5).
import { Icon } from '../icons';
import { fmtDayTotal } from '../../lib/format';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';

export default function TaskTimeControl({
  task,
  workspaceId,
  canWrite,
  compact = false,
}: {
  task: { id: string; project_id: string; name: string; tracked_seconds: number };
  workspaceId: string;
  canWrite: boolean;
  // fase 31: compact = só um ícone play/stop discreto (no header do popup), sem
  // o rótulo de tempo. Mesma lógica de elo; mesmo data-testid="task-start".
  compact?: boolean;
}) {
  const start = useTimerStore((s) => s.start);
  const stop = useTimerStore((s) => s.stop);
  // primitivo estável no selector (objeto novo = loop de render) — D-05.
  const entryTaskId = useTimerStore((s) => s.state?.entry?.task_id ?? null);
  const { elapsed } = useTimerClock();

  const running = entryTaskId === task.id;
  // CA-11.1: vivo = tracked_seconds (entries FECHADAS, do /board fase 12) +
  // elapsed da entry rodando SOMENTE quando é DESTA task. Nunca soma o elapsed
  // de outra task. Sem dupla contagem: ao parar, o board refaz a query e o
  // elapsed some (timer ocioso) enquanto a entry fechada entra no tracked.
  const shownSeconds = task.tracked_seconds + (running ? elapsed : 0);

  // CA-10/10.1: start countup puxando a task (sem escolher projeto — vem da
  // task; RNF-02 captura < 2s) + description = task.name (entry legível). O
  // start é fire-and-forget e nunca rejeita (CA-17 do timerStore).
  const onStart = () => {
    if (!task.project_id) return; // CA-15: defesa — nunca dispara sem project_id
    void start({
      workspace_id: workspaceId,
      project_id: task.project_id,
      task_id: task.id,
      description: task.name,
      tags: [],
    });
  };

  // fase 31: compact = só o ícone (header do popup, discreto). Play (idle+escrita)
  // ou Stop (rodando, em accent). Idle+viewer → nada. Mesmo data-testid/elo.
  if (compact) {
    if (running) {
      return (
        <button
          type="button"
          data-testid="task-start"
          aria-label="Parar"
          title="Parar"
          onClick={() => void stop()}
          className="tt-iconbtn"
          style={{ width: 30, height: 30, background: 'transparent', color: 'var(--accent)' }}
        >
          <Icon.stop size={14} />
        </button>
      );
    }
    if (canWrite) {
      return (
        <button
          type="button"
          data-testid="task-start"
          aria-label="Iniciar"
          title="Iniciar cronômetro nesta task"
          onClick={onStart}
          className="tt-iconbtn"
          style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
        >
          <Icon.play size={14} />
        </button>
      );
    }
    return null;
  }

  // o MESMO slot da fase 24 (data-testid="task-start") renderiza Start OU Parar.
  let button: React.ReactNode = null;
  if (running) {
    // CA-11/14: rodando nesta task → Parar, visível SEMPRE que roda (inclusive
    // viewer — não prender o timer global; padrão CA-11 da fase 23).
    button = (
      <button
        type="button"
        data-testid="task-start"
        aria-label="Parar"
        title="Parar"
        onClick={() => void stop()}
        className="tt-btn tt-btn--accent"
        style={{ padding: '4px 9px', fontSize: 11.5, gap: 5 }}
      >
        <Icon.stop size={12} />
        Parar
      </button>
    );
  } else if (canWrite) {
    // CA-10: idle nesta task + escrita → Start.
    button = (
      <button
        type="button"
        data-testid="task-start"
        aria-label="Iniciar"
        title="Iniciar"
        onClick={onStart}
        className="tt-btn tt-btn--accent"
        style={{ padding: '4px 9px', fontSize: 11.5, gap: 5 }}
      >
        <Icon.play size={12} />
        Start
      </button>
    );
  }
  // CA-14: idle + !canWrite → button null (só a leitura de tempo).

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: 1, minWidth: 0 }}>
      <span className="mono" style={{ fontSize: 13, fontWeight: 600 }}>
        {fmtDayTotal(shownSeconds)}
      </span>
      {/* Start/Parar empurrado pra direita da linha (mockup: margin-left:auto) */}
      {button && <span style={{ marginLeft: 'auto' }}>{button}</span>}
    </div>
  );
}
