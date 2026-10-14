// goalProgress (fase 29 T-08, CA-12.1): % de progresso de um Goal POR MODO de
// conclusão — a barra nunca mente sobre o critério. by_tasks/by_date → done_pct;
// by_hours → progress_pct (horas vs meta, já calculado no /board, 0 sem meta).
// À prova de culpa (RNF-03): sempre [0,100], nunca negativo, sem conceito de
// "falho" (um marco vencido aberto só fica em Open — não há flag de erro aqui).
import type { BoardObjective, CompletionMode } from '../../lib/api';

export function goalProgress(
  o: { completion_mode: CompletionMode; done_pct: number; progress_pct: number },
): number {
  const pct = o.completion_mode === 'by_hours' ? o.progress_pct : o.done_pct;
  return Math.max(0, Math.min(100, Math.round(pct)));
}

// reexport do tipo pra os consumidores não reimportarem (comodidade).
export type { BoardObjective };
