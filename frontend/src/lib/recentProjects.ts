// recentProjects (fase 23, CA-06/06.1) — a fila de "projetos recentes" do
// controle remoto: os project_id DISTINTOS das entries recentes, na ordem do
// MAIS RECENTE, sem duplicar, ignorando null. Função PURA (sem React), espelha
// o estilo de lib/recent.ts — testável sem mock. A resolução de nome/cor é da
// UI (via useProjects archived:true — CA-06.1); aqui só sai a ordem de ids.
import type { TimeEntry } from './api';

export function recentProjectIds(entries: TimeEntry[] | undefined): string[] {
  if (!entries || entries.length === 0) return [];
  // ordena por started_at DESC (mais recente primeiro) sem mutar o array de
  // entrada — a lista pode vir em qualquer ordem da query.
  const sorted = [...entries].sort((a, b) =>
    a.started_at < b.started_at ? 1 : a.started_at > b.started_at ? -1 : 0,
  );
  const seen = new Set<string>();
  const out: string[] = [];
  for (const e of sorted) {
    const id = e.project_id;
    if (!id) continue; // null/'' → ignora (entry sem projeto)
    if (seen.has(id)) continue; // dedup: mantém a 1ª (mais recente)
    seen.add(id);
    out.push(id);
  }
  return out;
}
