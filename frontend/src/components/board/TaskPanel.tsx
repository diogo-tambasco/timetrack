// TaskPanel (fase 24 T-12, CA-20/21/22/26/28.1): o popup central da task — usa
// CenterPanel (toggle side↔full). Título grande editável (Enter → foca o textarea
// de descrição — CA-21), TaskPanelProps (Status/Priority/Due/Resp/Solic/Time+Start
// mock/Notion), descrição .md SEMPRE editável (PATCH notes no blur), SubtaskSidebar
// à direita (vira seção abaixo < md — RNF-06), e a thread de comentários da fase 19
// gateada por canCommentOn (carve-out viewer-assignee preservado — CA-28.1). Editar
// reflete sem reload (PATCH + invalida boardKey via onChanged). Nome vazio NÃO chama
// a API (CA-21).
//
// O corpo (TaskPanelBody) é KEYED por task.id: o quick-add abre o popup ANTES do
// board refetchar com a task nova, então o corpo só monta (com useState fresco do
// nome/notas) quando a task aparece no board — sem draft preso em "".
import { useEffect, useRef, useState } from 'react';
import type { Board, BoardTask, PropertyDef } from '../../lib/api';
import { useAuthStore } from '../../lib/auth';
import { usePatchTask, useDeleteTask } from '../../lib/queries/tasks';
import { useWorkspaceMembers } from '../../lib/queries/workspace';
import { Icon } from '../icons';
import CenterPanel from './CenterPanel';
import TaskPanelProps from './TaskPanelProps';
import TaskTimeControl from './TaskTimeControl';
import CustomProperties from './CustomProperties';
import SubtaskSidebar from './SubtaskSidebar';
import TaskThread from './TaskThread';
import { canCommentOn } from './canComment';
import { useMediaQuery } from '../../lib/useMediaQuery';

function findTask(board: Board, id: string): BoardTask | null {
  for (const o of board.objectives) {
    const t = o.tasks.find((x) => x.id === id);
    if (t) return t;
  }
  return board.general.tasks.find((x) => x.id === id) ?? null;
}

export default function TaskPanel({
  wsId,
  projectId,
  taskId,
  board,
  canWrite,
  onClose,
  onChanged,
  focusTitle = false,
  scheduleOnOpen = false,
}: {
  wsId: string;
  projectId: string;
  taskId: string;
  board: Board;
  canWrite: boolean;
  onClose: () => void;
  onChanged: () => void;
  // abrir já focado/selecionado no título (task criada via /task sem nome — fase 31)
  focusTitle?: boolean;
  // abrir já com o calendário de Due aberto (criada via /task nome — fase 31)
  scheduleOnOpen?: boolean;
}) {
  const task = findTask(board, taskId);
  return (
    <CenterPanel
      ariaLabel={task?.name ?? 'Task'}
      onClose={onClose}
      width={1320}
      fillHeight
      headerExtra={
        task && canWrite ? (
          <TaskMenu wsId={wsId} taskId={task.id} onDeleted={onClose} onChanged={onChanged} />
        ) : null
      }
      headerActions={
        // fase 31: play discreto no topo-direito — inicia/para o cronômetro NESTA
        // task (o elo da fase 30, agora compacto; mesmo data-testid="task-start").
        task ? <TaskTimeControl task={task} workspaceId={wsId} canWrite={canWrite} compact /> : null
      }
    >
      {task ? (
        <TaskPanelBody
          key={task.id}
          wsId={wsId}
          projectId={projectId}
          task={task}
          defs={board.project.property_defs}
          canWrite={canWrite}
          onChanged={onChanged}
          focusTitle={focusTitle}
          scheduleOnOpen={scheduleOnOpen}
        />
      ) : (
        // a task ainda não chegou no board (refetch pós quick-add): chrome aberto,
        // corpo monta keyed quando a task aparece (useState fresco do nome).
        <div style={{ padding: 24 }} />
      )}
    </CenterPanel>
  );
}

function TaskPanelBody({
  wsId,
  projectId,
  task,
  defs,
  canWrite,
  onChanged,
  focusTitle,
  scheduleOnOpen,
}: {
  wsId: string;
  projectId: string;
  task: BoardTask;
  defs: PropertyDef[];
  canWrite: boolean;
  onChanged: () => void;
  focusTitle?: boolean;
  scheduleOnOpen?: boolean;
}) {
  const patchTask = usePatchTask(wsId);
  const currentUserId = useAuthStore((s) => s.me?.user.id ?? null);
  // fase 31: comentários só aparecem em workspace multiplayer (>1 membro) — solo
  // não tem com quem conversar; o mockup de planejamento individual não os mostra.
  const multiplayer = (useWorkspaceMembers(wsId).data ?? []).length > 1;
  const isWide = useMediaQuery('(min-width: 768px)');
  const [name, setName] = useState(task.name);
  const [notes, setNotes] = useState(task.notes);
  const descRef = useRef<HTMLTextAreaElement>(null);
  const titleRef = useRef<HTMLInputElement>(null);
  // sidebar de Task-goals retrátil (fase 31): enxuta e recolhível (raramente passa
  // de 1 frase por goal). Estado local — reabre expandida.
  const [goalsOpen, setGoalsOpen] = useState(true);
  // descrição = CANVAS INFINITO (estilo Notion): a textarea CRESCE com o conteúdo
  // (sem scroll interno), então a página inteira rola e as propriedades somem ao
  // subir. autoGrow zera a altura e usa o scrollHeight.
  const autoGrow = (el: HTMLTextAreaElement | null) => {
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  };
  // /task sem nome → abre focado e SELECIONA "New task" pro Diogo digitar por cima
  // (captura sem fricção). Roda 1× no mount do corpo (keyed por task.id).
  useEffect(() => {
    if (focusTitle && canWrite) {
      const el = titleRef.current;
      if (el) {
        el.focus();
        el.select();
      }
    }
    autoGrow(descRef.current); // ajusta a altura ao conteúdo inicial
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // último nome efetivamente SALVO (não o do board, que só atualiza no refetch).
  // Enter dispara saveName E move o foco pro corpo → o onBlur do título dispara
  // saveName DE NOVO; sem isto, como task.name ainda é o valor pré-rename até o
  // board refetchar, o blur reenviava um 2º PATCH idêntico (duplicado, e que
  // bagunçava a ordem dos PATCHes). Comparar com o último salvo torna o blur um
  // no-op quando nada mudou desde o último save.
  const savedName = useRef(task.name);

  // Um PATCH por campo (nome no blur/Enter, status/priority/due/assignee/etc. no
  // chip/select). PATCHes parciais concorrentes na MESMA task são serializados no
  // BACKEND via SELECT … FOR UPDATE (tasks/service.go::Patch) → sem lost update do
  // read-modify-write (ex.: requested_by não zera mais o assignee recém-setado). O
  // onChanged faz cancel+invalidate do board → reflete sem reload (CA-20).
  const patch = (p: Parameters<typeof patchTask.mutate>[0]['patch']) =>
    patchTask.mutate({ id: task.id, patch: p }, { onSuccess: onChanged });

  const saveName = () => {
    const v = name.trim();
    if (v === '' || v === savedName.current) return; // vazio/inalterado NÃO chama a API (CA-21)
    savedName.current = v;
    patch({ name: v });
  };
  const saveNotes = () => {
    if (notes === task.notes) return;
    patch({ notes });
  };
  const canComment = canCommentOn(canWrite, task, currentUserId);

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: isWide ? 'row' : 'column',
        gap: 0,
        alignItems: 'stretch',
      }}
    >
      <div style={{ flex: 1, minWidth: 0, padding: '26px 34px 44px', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <input
          ref={titleRef}
          aria-label="Título da task"
          value={name}
          readOnly={!canWrite}
          onChange={(e) => setName(e.target.value)}
          onBlur={saveName}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              saveName();
              const el = descRef.current;
              if (el) {
                el.focus();
                const end = el.value.length;
                el.setSelectionRange(end, end);
              }
            }
          }}
          placeholder="Untitled"
          style={{
            background: 'transparent',
            border: 'none',
            outline: 'none',
            color: 'var(--text-1)',
            fontFamily: 'var(--font-ui)',
            fontSize: 30,
            fontWeight: 700,
            letterSpacing: '-.02em',
            width: '100%',
          }}
        />

        {/* bloco de propriedades (nativas + custom + Add property), estilo Notion:
            uma coluna de linhas com borda inferior fechando o grupo (mockup .prop). */}
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            paddingBottom: 14,
            borderBottom: '1px solid var(--border)',
          }}
        >
          <TaskPanelProps wsId={wsId} task={task} canWrite={canWrite} onPatch={(p) => patch(p)} autoOpenDue={scheduleOnOpen} />
          {/* fase 25 (CA-21): propriedades CUSTOM do projeto abaixo das nativas + Add property */}
          <CustomProperties wsId={wsId} projectId={projectId} task={task} defs={defs} canWrite={canWrite} />
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span className="eyebrow">Description</span>
          <textarea
            ref={descRef}
            aria-label="Descrição da task"
            value={notes}
            readOnly={!canWrite}
            onChange={(e) => {
              setNotes(e.target.value);
              autoGrow(e.target);
            }}
            onBlur={saveNotes}
            placeholder="Escreva o conteúdo da task em markdown…  (o que precisa ser feito, contexto, links)"
            style={{
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'var(--text-1)',
              fontFamily: 'var(--font-ui)',
              fontSize: 15.5,
              lineHeight: 1.7,
              padding: 0,
              minHeight: '52vh',
              resize: 'none',
              overflow: 'hidden',
              width: '100%',
            }}
          />
        </div>

        {/* thread de comentários da fase 19, só em multiplayer (fase 31); gating
            de escrita por canCommentOn preservado (CA-28.1) */}
        {multiplayer && <TaskThread wsId={wsId} taskId={task.id} canComment={canComment} />}
      </div>

      {/* Task-goals: sidebar enxuta e RETRÁTIL (fase 31). Aberta = 232px com botão
          de retrair; recolhida = trilho fino com botão de expandir. < md vira seção. */}
      {!isWide || goalsOpen ? (
        <div
          style={{
            flex: isWide ? '0 0 232px' : '1 1 auto',
            borderLeft: isWide ? '1px solid var(--border)' : 'none',
            borderTop: isWide ? 'none' : '1px solid var(--border)',
            background: 'var(--surface-2)',
            padding: '18px 16px 26px',
            position: 'relative',
          }}
        >
          {isWide && (
            <button
              type="button"
              aria-label="Recolher Task-goals"
              title="Recolher"
              className="tt-iconbtn"
              onClick={() => setGoalsOpen(false)}
              style={{ position: 'absolute', top: 12, right: 10, width: 24, height: 24, background: 'transparent', color: 'var(--text-3)' }}
            >
              <Icon.chevron size={15} style={{ transform: 'rotate(-90deg)' }} />
            </button>
          )}
          <SubtaskSidebar wsId={wsId} taskId={task.id} projectId={projectId} canWrite={canWrite} />
        </div>
      ) : (
        // retraída = trilho fino, transparente, sem borda — só um chevron discreto
        // pra trazer de volta (minimalista; nada de coluna escura com estrela solta).
        <div style={{ flex: '0 0 30px', display: 'flex', justifyContent: 'center', paddingTop: 22 }}>
          <button
            type="button"
            aria-label="Expandir Task-goals"
            title="Task-goals"
            className="tt-iconbtn"
            onClick={() => setGoalsOpen(true)}
            style={{ width: 24, height: 24, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.chevron size={15} style={{ transform: 'rotate(90deg)' }} />
          </button>
        </div>
      )}
    </div>
  );
}

// TaskMenu (fase 31): menu "…" no header do popup com "Excluir task" — delete
// simples estilo Notion. Confirma antes (ação destrutiva, sem undo). Ao apagar:
// fecha o popup E invalida o board (onChanged) → some da lista sem reload.
function TaskMenu({
  wsId,
  taskId,
  onDeleted,
  onChanged,
}: {
  wsId: string;
  taskId: string;
  onDeleted: () => void;
  onChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const deleteTask = useDeleteTask(wsId);

  const remove = () => {
    setOpen(false);
    if (typeof window !== 'undefined' && !window.confirm('Excluir esta task? Não dá pra desfazer.')) return;
    deleteTask.mutate(taskId, {
      onSuccess: () => {
        onChanged();
        onDeleted();
      },
    });
  };

  return (
    <div style={{ position: 'relative' }}>
      <button
        type="button"
        aria-label="Mais ações"
        data-testid="task-menu"
        className="tt-iconbtn"
        onClick={() => setOpen((o) => !o)}
        style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
      >
        <Icon.dots size={16} />
      </button>
      {open && (
        <>
          <div
            aria-hidden
            onClick={() => setOpen(false)}
            style={{ position: 'fixed', inset: 0, zIndex: 70 }}
          />
          <div
            role="menu"
            style={{
              position: 'absolute',
              top: '100%',
              left: 0,
              marginTop: 4,
              zIndex: 71,
              minWidth: 160,
              background: 'var(--surface-2)',
              border: '1px solid var(--border-2)',
              borderRadius: 9,
              boxShadow: 'var(--shadow)',
              padding: 5,
            }}
          >
            <button
              type="button"
              role="menuitem"
              data-testid="task-delete"
              onClick={remove}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 9,
                width: '100%',
                padding: '8px 10px',
                borderRadius: 6,
                border: 'none',
                background: 'transparent',
                color: 'var(--danger)',
                font: 'inherit',
                fontSize: 13,
                cursor: 'pointer',
                textAlign: 'left',
              }}
            >
              <Icon.trash size={15} />
              Excluir task
            </button>
          </div>
        </>
      )}
    </div>
  );
}
