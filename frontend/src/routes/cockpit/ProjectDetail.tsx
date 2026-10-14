// ProjectDetail (fase 24 T-10 / fase 28 T-11, CA-15/19): rota /projetos/:id vira
// o WORKSPACE DE PROJETO com abas (Overview · Tasks · Notes · Goals · Calendar).
// Header = cor + nome + chip de cliente (de board.project.client_name — CA-14.1,
// sem 2º fetch). Fase 28: aba default = Overview, com OverviewTab + ProjectCalendar
// ricas (Goals segue placeholder até a 29). Trocar de projeto reseta pra Overview
// (key={projectId} remonta a página — CA-19). Overview/Calendar abrem a task pelo
// TaskPanel da fase 24 (RF-23), erguido aqui no shell.
import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useCurrentWorkspace } from '../../lib/auth';
import { useBoard, boardKey } from '../../lib/queries/objectives';
import { useOpenTaskStore } from '../../lib/openTaskStore';
import { useOpenNoteStore } from '../../lib/openNoteStore';
import { Icon } from '../../components/icons';
import ProjectTabs, { type ProjectTab } from '../../components/board/ProjectTabs';
import TasksTab from '../../components/board/TasksTab';
import GoalsTab from '../../components/board/GoalsTab';
import NotesTab from '../../components/notes/NotesTab';
import OverviewTab from '../../components/project/OverviewTab';
import ProjectCalendarTab from '../../components/project/ProjectCalendarTab';
import TaskPanel from '../../components/board/TaskPanel';
import NotePanel from '../../components/notes/NotePanel';
import { useBoardTimerSync } from '../../components/board/useBoardTimerSync';

export default function ProjectDetail() {
  const ws = useCurrentWorkspace();
  const { id } = useParams<{ id: string }>();
  if (!ws || !id) return null;
  // key={id}: trocar de projeto remonta a página → a aba volta pra Overview
  // (CA-19 — a escolha de aba não persiste entre projetos).
  return <ProjectDetailPage key={id} wsId={ws.id} projectId={id} />;
}

function ProjectDetailPage({ wsId, projectId }: { wsId: string; projectId: string }) {
  const { data: board, isError } = useBoard(wsId, projectId);
  // fase 30 (CA-13/30-H5): o tempo cronometrado pela task "gruda" — ao parar uma
  // entry rodando DESTE projeto (inclusive pela TimerBar/troca/auto-corte com o
  // popup fechado), invalida o board pro tracked_seconds convergir sem reload.
  // Vive AQUI (shell sempre montado), não no TaskTimeControl (desmonta no fechar).
  useBoardTimerSync(wsId, projectId);
  const [tab, setTab] = useState<ProjectTab>('overview');
  // TaskPanel erguido no shell: Overview/Calendar abrem a task (RF-23) E o
  // Spotlight (/task) abre via store global (fase 31 — captura → popup completo).
  // O store é a fonte única do popup do shell; limpa ao trocar de projeto.
  const openTaskId = useOpenTaskStore((s) => s.taskId);
  const focusTitle = useOpenTaskStore((s) => s.focusTitle);
  const scheduleOnOpen = useOpenTaskStore((s) => s.scheduleOnOpen);
  const openTask = useOpenTaskStore((s) => s.open);
  const closeTask = useOpenTaskStore((s) => s.close);
  useEffect(() => () => closeTask(), [closeTask]);
  // NotePanel erguido no shell: o Spotlight (/note) cria a nota e abre o popup
  // completo via store global (espelha o openTask). Sem isso, /note caía no
  // fallback (só navegava pro projeto) e a nota nunca abria (fase 31 fix).
  const openNoteId = useOpenNoteStore((s) => s.noteId);
  const noteFocusTitle = useOpenNoteStore((s) => s.focusTitle);
  const openNote = useOpenNoteStore((s) => s.open);
  const closeNote = useOpenNoteStore((s) => s.close);
  useEffect(() => () => closeNote(), [closeNote]);
  const qc = useQueryClient();
  // Editar uma task pelo TaskPanel (aberto da aba Calendar/Overview) muda o board
  // E pode mudar o due_date → a grade do Calendar precisa atualizar AO VIVO (28-REG-1,
  // object permanence). Invalida o board E o calendar do projeto (prefixo sem month
  // — match parcial de QUALQUER mês carregado; ver projectCalendarKey).
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: boardKey(wsId, projectId) });
    qc.invalidateQueries({ queryKey: ['projects', wsId, 'calendar', projectId] });
  };

  // Uma vez que TEMOS dados, SEMPRE renderiza o conteúdo — mesmo que um refetch de
  // fundo (invalidate pós-PATCH do popup) FALHE: o React Query mantém o último
  // `data` no erro, mas marca `isError`. Tratar `isError || !board` desmontaria
  // toda a subárvore (abas + popup central aberto) num erro transitório de refetch
  // sob carga → o center-close some no meio da edição (CA-20). Só mostra o estado
  // de erro/carregando quando NÃO há dado nenhum (1º load).
  if (!board) {
    if (isError) {
      return (
        <section style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <BackLink />
          <p className="faint">Projeto não encontrado.</p>
        </section>
      );
    }
    return null; // boot silencioso (1º load, sem dado ainda)
  }

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 18, height: '100%' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <BackLink />
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
          <span
            aria-hidden
            style={{ width: 5, height: 30, borderRadius: 3, background: board.project.color, flex: '0 0 auto' }}
          />
          <h1
            style={{
              margin: 0,
              fontSize: 25,
              fontWeight: 600,
              letterSpacing: '-.02em',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {board.project.name}
          </h1>
          {board.project.client_name != null && (
            <span
              className="chip"
              data-testid="project-client"
              style={{ display: 'inline-flex', alignItems: 'center', gap: 5, flex: '0 0 auto' }}
            >
              <Icon.briefcase size={12} />
              {board.project.client_name}
            </span>
          )}
        </div>
      </div>

      <ProjectTabs active={tab} onChange={setTab} />

      <div style={{ flex: 1, minHeight: 0 }}>
        {tab === 'overview' && (
          <OverviewTab wsId={wsId} board={board} onOpenTask={(id) => openTask(id)} />
        )}
        {tab === 'tasks' && <TasksTab wsId={wsId} board={board} />}
        {tab === 'notes' && <NotesTab wsId={wsId} board={board} />}
        {tab === 'goals' && <GoalsTab wsId={wsId} board={board} />}
        {tab === 'calendar' && (
          <ProjectCalendarTab wsId={wsId} board={board} onOpenTask={(id) => openTask(id)} />
        )}
      </div>

      {/* TaskPanel da fase 24 erguido no shell: Overview/Calendar abrem a task
          por aqui (RF-23). O TasksTab tem o seu próprio popup pro board. */}
      {openTaskId && (
        <TaskPanel
          wsId={wsId}
          projectId={projectId}
          taskId={openTaskId}
          focusTitle={focusTitle}
          scheduleOnOpen={scheduleOnOpen}
          board={board}
          canWrite={board.project.can_write}
          onClose={closeTask}
          onChanged={invalidate}
        />
      )}

      {/* NotePanel erguido no shell: o Spotlight (/note) abre a nota por aqui
          (fase 31 fix). Links da nota → task abre o TaskPanel, → sub-nota reabre
          este painel. */}
      {openNoteId && (
        <NotePanel
          wsId={wsId}
          projectId={projectId}
          noteId={openNoteId}
          canWrite={board.project.can_write}
          focusTitle={noteFocusTitle}
          onClose={closeNote}
          onOpenLink={(link) => {
            if (link.ref_type === 'task') openTask(link.ref_id);
            else openNote(link.ref_id);
          }}
        />
      )}
    </section>
  );
}

function BackLink() {
  return (
    <Link
      to="/projetos"
      className="tt-btn tt-btn--ghost"
      style={{ alignSelf: 'flex-start', textDecoration: 'none' }}
    >
      <Icon.chevron size={15} style={{ transform: 'rotate(90deg)' }} />
      Projetos
    </Link>
  );
}
