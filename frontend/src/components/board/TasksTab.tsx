// TasksTab (fase 24 T-11/T-12/T-14): a aba Tasks rica. Toolbar (segmented
// Board/List/Table persistido em tt.view.board com mapeamento legado — CA-16/16.1
// + "New task") + as 3 visões. Board = 4 colunas DnD nativo + quick-add (CA-17/18).
// Abrir/criar uma task abre o popup central (TaskPanel — CA-20). Gating por
// board.project.can_write (CA-25). Invalida o boardKey no onSuccess local (D-12).
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { Board, BoardTask, TaskStatus } from '../../lib/api';
import { Icon } from '../icons';
import { useView } from '../ui/ViewToggle';
import { useCreateTask, usePatchTask } from '../../lib/queries/tasks';
import { boardKey } from '../../lib/queries/objectives';
import { STATUS_ORDER, STATUS_META } from './status';
import { type DropMutation } from './dnd';
import TaskBoardColumns from './TaskBoardColumns';
import TaskListView from './TaskListView';
import TaskTableView from './TaskTableView';
import TaskPanel from './TaskPanel';

type TaskView = 'board' | 'list' | 'table';
const VIEW_OPTS = ['board', 'list', 'table'] as const;
const VIEW_LEGACY = { cards: 'board', table: 'list' } as const;
// a Table NATIVA colide com o 'table' LEGADO da fase 12 (que mapeia pra List) →
// grava sob token próprio pra persistir de verdade no reload (CA-16.1/RF-03).
const VIEW_PERSIST = { table: 'table-v2' } as const;
const VIEW_SEG: { v: TaskView; label: string; Glyph: typeof Icon.board }[] = [
  { v: 'board', label: 'Board', Glyph: Icon.board },
  { v: 'list', label: 'List', Glyph: Icon.list },
  { v: 'table', label: 'Table', Glyph: Icon.table },
];

export default function TasksTab({ wsId, board }: { wsId: string; board: Board }) {
  const projectId = board.project.id;
  const canWrite = board.project.can_write;
  const [view, setView] = useView<TaskView>('tt.view.board', VIEW_OPTS, 'board', VIEW_LEGACY, VIEW_PERSIST);
  const [panel, setPanel] = useState<{ taskId: string } | null>(null);

  const qc = useQueryClient();
  // cancel-then-invalidate: edições rápidas em sequência no popup (blur do título
  // → PATCH name, depois clicar Status → PATCH status) disparam invalidações
  // sobrepostas; sem o cancel, a 2ª invalidação é DEDUPLICADA no refetch da 1ª
  // (que leu o estado ANTES do 2º commit) e o board não reflete sem reload
  // (CA-17/20). Cancelar o refetch em voo força um fetch novo pós-commit — como o
  // onSuccess só roda após o commit do PATCH, o ÚLTIMO invalidate lê o DB completo.
  // revert:false é OBRIGATÓRIO: o default (revert:true) do cancelQueries REVERTE o
  // cache pro estado ANTES do fetch cancelado — quando o invalidate do 2º PATCH
  // cancela o refetch do 1º, o board volta pro dado PRÉ-rename ("New task") e, no
  // intercalar de dois cancel+invalidate assíncronos, podia ficar preso nesse stale
  // sem refetch pendente. Com revert:false o cancel só aborta; o invalidate sempre
  // dispara um fetch novo que lê o DB mais recente.
  const invalidate = async () => {
    await qc.cancelQueries({ queryKey: boardKey(wsId, projectId) }, { revert: false });
    await qc.invalidateQueries({ queryKey: boardKey(wsId, projectId) });
  };
  const createTask = useCreateTask(wsId);
  const patchTask = usePatchTask(wsId);

  // todas as tasks do board (objetivos + Geral), pra distribuir nas colunas/grupos.
  const tasks: BoardTask[] = [
    ...board.objectives.flatMap((o) => o.tasks),
    ...board.general.tasks,
  ];

  const openTask = (t: BoardTask) => setPanel({ taskId: t.id });

  // quick-add por coluna/grupo: cria com nome default no status dado e abre o popup
  // focado (RNF-02). New task cria no todo.
  const quickAdd = (status: TaskStatus) => {
    if (!canWrite) return;
    createTask.mutate(
      { project_id: projectId, name: 'New task', status },
      {
        onSuccess: (created) => {
          invalidate();
          setPanel({ taskId: created.id });
        },
      },
    );
  };

  const onDrop = (m: DropMutation) =>
    patchTask.mutate({ id: m.id, patch: m.patch }, { onSuccess: invalidate });

  // cicla status/priority via chip (1 clique — CA-19/20).
  const cycleStatus = (t: BoardTask, next: TaskStatus) =>
    patchTask.mutate({ id: t.id, patch: { status: next } }, { onSuccess: invalidate });
  const cyclePriority = (t: BoardTask, next: number) =>
    patchTask.mutate({ id: t.id, patch: { priority: next } }, { onSuccess: invalidate });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
        <div
          role="tablist"
          aria-label="Visão das tasks"
          style={{ display: 'flex', background: 'var(--surface-3)', borderRadius: 9, padding: 2, gap: 2 }}
        >
          {VIEW_SEG.map(({ v, label, Glyph }) => (
            <button
              key={v}
              type="button"
              role="tab"
              aria-selected={view === v}
              onClick={() => setView(v)}
              style={{
                border: 'none',
                cursor: 'pointer',
                fontFamily: 'var(--font-ui)',
                fontSize: 12.5,
                fontWeight: 540,
                padding: '6px 12px',
                borderRadius: 7,
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                background: view === v ? 'var(--surface-1)' : 'transparent',
                color: view === v ? 'var(--text-1)' : 'var(--text-2)',
              }}
            >
              <Glyph size={14} />
              {label}
            </button>
          ))}
        </div>
        {canWrite && (
          <button type="button" className="tt-btn tt-btn--accent" onClick={() => quickAdd('backlog')}>
            <Icon.plus size={15} sw={2.2} />
            New task
          </button>
        )}
      </div>

      {view === 'board' && (
        <TaskBoardColumns
          tasks={tasks}
          canWrite={canWrite}
          onOpenTask={openTask}
          onDrop={onDrop}
          onQuickAdd={quickAdd}
        />
      )}
      {view === 'list' && (
        <TaskListView
          tasks={tasks}
          canWrite={canWrite}
          onOpenTask={openTask}
          onQuickAdd={quickAdd}
          onCycleStatus={cycleStatus}
        />
      )}
      {view === 'table' && (
        <TaskTableView
          tasks={tasks}
          canWrite={canWrite}
          onOpenTask={openTask}
          onQuickAdd={quickAdd}
          onCycleStatus={cycleStatus}
          onCyclePriority={cyclePriority}
          wsId={wsId}
          projectId={projectId}
          defs={board.project.property_defs}
        />
      )}

      {panel && (
        <TaskPanel
          wsId={wsId}
          projectId={projectId}
          taskId={panel.taskId}
          board={board}
          canWrite={canWrite}
          onClose={() => setPanel(null)}
          onChanged={invalidate}
        />
      )}
    </div>
  );
}

// reexport pra specs/overview reusarem a ordem/rótulos canônicos.
export { STATUS_ORDER, STATUS_META };
