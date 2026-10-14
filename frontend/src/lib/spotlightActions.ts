// spotlightActions.ts (fase 27, D-08/D-09/D-10/D-11): a casca de ORQUESTRAÇÃO
// do Spotlight. runSpot roteia o input parseado pros fluxos JÁ EXISTENTES e
// nunca rejeita pro componente (try/catch gracioso — CA-17, igual ao mutate do
// timerStore da fase 05).
//
// CONTRATOS REAIS CONFIRMADOS (T-04):
//   - createTask(ws, {project_id, name})  → api.ts (Task com id)              [fase 02/24]
//   - createNote(ws, {project_id, title}) → api.ts (Note com id)             [fase 26]
//   - createObjective(ws, projectId, {name}) → api.ts (Objective)           [fase 12]
//   - useTimerStore.getState().start({mode:'countup',...})                  [fase 03]
//   - useTimerStore.getState().editCurrent({description, project_id})       [allowance 27-H1]
//   - useTimerStore.getState().state?.entry → detecta rodando × ocioso       [fase 03]
//   * NÃO usar patchEntry (mexe em timing e rejeita entry rodando — CA-13.1).
//   * O popup de task/nota das fases 24/26 é ABERTO POR ESTADO LOCAL de
//     TasksTab/NotesTab (não há abridor GLOBAL — TasksTab.tsx:36-64,
//     NotesTab via NotePanel). Logo o Spotlight DEGRADA (D-09/CA-17): cria a
//     entidade e NAVEGA pra /projetos/{id} (a página abre a aba certa). O
//     `open` opcional (openTask/openNote) é injetado p/ o futuro abridor da 24/26;
//     ausente → fallback de navegação (nunca quebra).
import {
  createTask,
  createNote,
  createObjective,
} from './api';
import { boardKey } from './queries/objectives';
import { queryClient } from './queryClient';
import { spotParse } from './spotlight';
import { useTimerStore } from './timerStore';

export interface SpotCtx {
  ws: string;
  projectId: string | null;
  canWrite: boolean;
}

// Dependências injetadas pelo componente (testáveis; degradam se ausentes).
export interface SpotDeps {
  /** fecha o Spotlight (sempre disponível) */
  close: () => void;
  /** navega pra uma rota (fallback de abertura — sempre disponível) */
  navigate: (to: string) => void;
  /** abridor de task (fase 31): opts.focusTitle foca o título; opts.scheduleOnOpen
   *  abre o calendário de Due. Ausente → fallback navegação. */
  openTask?: (id: string, opts?: { focusTitle?: boolean; scheduleOnOpen?: boolean }) => void;
  /** abridor de nota (fase 31): opts.focusTitle foca/seleciona o título da nota
   *  recém-criada. Ausente → fallback navegação. */
  openNote?: (id: string, opts?: { focusTitle?: boolean }) => void;
  /** sinaliza ausência de contexto/permissão à-prova-de-culpa (CA-15/16) */
  notify?: (msg: string) => void;
}

// runSpot executa o comando do input no contexto dado. SEMPRE resolve (CA-17):
// nenhuma exceção/promessa rejeitada vaza pro componente.
export async function runSpot(input: string, ctx: SpotCtx, deps: SpotDeps): Promise<void> {
  const { act, title } = spotParse(input);
  const timer = useTimerStore.getState();
  const running = !!timer.state?.entry;
  const projectIdOrUndef = ctx.projectId ?? undefined;

  try {
    // ── /task /note /goal: exigem projeto de contexto + escrita (CA-15/16) ──
    if (act === 'task' || act === 'note' || act === 'goal') {
      if (!ctx.projectId) {
        deps.notify?.('Abra um projeto pra criar aqui');
        return; // mantém aberto, NÃO chama API (CA-15)
      }
      if (!ctx.canWrite) {
        deps.notify?.('Você não tem permissão de escrita neste projeto');
        return; // mantém aberto, NÃO chama API (CA-16)
      }
      const projectId = ctx.projectId;

      if (act === 'task') {
        // nasce em backlog (default do planejamento — fase 31): capturou, vai
        // pro backlog até você puxar pro To do/In progress.
        const t = await createTask(ctx.ws, { project_id: projectId, name: title || 'New task', status: 'backlog' });
        // invalida o board do projeto: o entity novo reflete mesmo navegando pra
        // a MESMA rota /projetos/:id (a query não refetcha sozinha — espelha o
        // invalidate do useCreateTask da 24).
        void queryClient.invalidateQueries({ queryKey: boardKey(ctx.ws, projectId) });
        deps.close();
        // abre o popup completo. COM nome → abre o calendário de Due ("quando
        // preciso fazer isso?"). SEM nome → foca/seleciona o título pra digitar.
        if (deps.openTask) deps.openTask(t.id, title ? { scheduleOnOpen: true } : { focusTitle: true });
        else deps.navigate(`/projetos/${projectId}`); // fallback (D-09/CA-17)
        return;
      }
      if (act === 'note') {
        // nasce sem título (mostra "Untitled") e abre FOCADA no título pra nomear
        // direto — captura < 2s (fase 31). Se veio nome no /note, já entra preenchido.
        const n = await createNote(ctx.ws, { project_id: projectId, title: title || '' });
        deps.close();
        if (deps.openNote) deps.openNote(n.id, { focusTitle: true });
        else deps.navigate(`/projetos/${projectId}`); // fallback (D-09/CA-17)
        return;
      }
      // act === 'goal'
      await createObjective(ctx.ws, projectId, { name: title || 'New goal' });
      void queryClient.invalidateQueries({ queryKey: boardKey(ctx.ws, projectId) });
      deps.close();
      deps.navigate(`/projetos/${projectId}`); // sem editor de objetivo dedicado (§6)
      return;
    }

    // ── /timer: começa a cronometrar (reusa o store — CA-11) ──
    if (act === 'timer') {
      await timer.start({ workspace_id: ctx.ws, mode: 'countup', description: title, project_id: projectIdOrUndef });
      deps.close();
      return;
    }

    // ── sem comando (RF-19) ──
    if (running) {
      // timer RODANDO + texto → edita o timer atual (allowance §3.5 — CA-13)
      if (input.trim() === '') return; // vazio = no-op (não apaga a descrição)
      await timer.editCurrent({ description: input, project_id: projectIdOrUndef });
      deps.close();
      return;
    }
    // OCIOSO + texto → começa um count-up (CA-14)
    if (input.trim() === '') return; // vazio = no-op (mantém aberto)
    await timer.start({ workspace_id: ctx.ws, mode: 'countup', description: input, project_id: projectIdOrUndef });
    deps.close();
  } catch {
    // CA-17: falha graciosa — não rejeita pro componente, não quebra a UI.
    deps.notify?.('Não consegui criar agora — tente de novo');
  }
}
