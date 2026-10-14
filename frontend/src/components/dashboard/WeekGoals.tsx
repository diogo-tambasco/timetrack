// WeekGoals (fase 13, T-04) — a seção "da semana" da home: metas de horas por
// projeto (CA-16/17/18). REUSA o GoalsSection da fase 07 (GoalBar fill na cor,
// accent ≥100%) e o /reports/week (sem backend novo — RNF-03). Resiliente: um
// erro aqui NÃO derruba o dashboard (CA-21); vazio → mensagem (CA-19).
//
// CA-17 (furo confirmado na verificação da spec): progresso POR OBJETIVO só
// existe no /board por projeto (BoardObjective); agregar por workspace = N
// requests/RNF-02 ou backend novo/RNF-03 → fica pra fase futura. A seção da
// semana é metas de horas, ponto.
import GoalsSection from '../reports/GoalsSection';
import { useCurrentWorkspace } from '../../lib/auth';
import { browserTZ } from '../../lib/format';
import { useWeekReport } from '../../lib/queries/reports';
import { localDayISO } from '../../lib/recent';

export default function WeekGoals() {
  const ws = useCurrentWorkspace();
  const today = localDayISO(new Date().toISOString(), browserTZ());
  const q = useWeekReport(ws?.id ?? '', today);

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 style={{ margin: 0, fontSize: 14, fontWeight: 600 }}>Metas da semana</h2>
      {q.isLoading ? (
        <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
          Carregando metas…
        </div>
      ) : q.isError ? (
        <div className="muted" style={{ padding: '14px 4px', fontSize: 13 }}>
          Não foi possível carregar as metas.
        </div>
      ) : (
        <GoalsSection goals={q.data?.goals ?? []} />
      )}
    </section>
  );
}
