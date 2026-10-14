// RecentEntries (fase 13, T-05) — o coração do P3: entradas dos DIAS
// ANTERIORES (ontem..−6) agrupadas por dia local, pra não sumir na virada do
// dia. As entradas de HOJE são do <TodayList> reusado no dashboard (CA-24.1) —
// aqui pulamos o grupo "Hoje". Replica a FORMA da linha do TodayList (sem tocá-
// lo, CA-24); testid distinto `recent-entry-` pra não colidir com o
// `[data-testid^="entry-"]` do cronometro.spec.
import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Icon } from '../icons';
import EntryModal from '../timer/EntryModal';
import { useCurrentWorkspace } from '../../lib/auth';
import { browserTZ, fmtClock, fmtHourMinute } from '../../lib/format';
import { entriesKey, useContinueEntry, useRecentEntries } from '../../lib/queries/entries';
import { useProjects } from '../../lib/queries/projects';
import { groupByDay, localDayISO } from '../../lib/recent';
import { useTimerStore } from '../../lib/timerStore';
import type { TimeEntry } from '../../lib/api';

export default function RecentEntries() {
  const ws = useCurrentWorkspace();
  const qc = useQueryClient();
  const recentQ = useRecentEntries(ws?.id ?? '');
  const projectsQ = useProjects(ws?.id ?? '', { archived: true });
  const continuar = useContinueEntry(ws?.id ?? '');
  const activeEntryID = useTimerStore((s) => s.state?.entry?.id ?? null);
  const [editando, setEditando] = useState<TimeEntry | null>(null);

  // start/stop/continue mudam o histórico recente: re-busca ao mudar a ativa
  useEffect(() => {
    void qc.invalidateQueries({ queryKey: entriesKey(ws?.id ?? '') });
  }, [activeEntryID, qc, ws?.id]);

  const tz = browserTZ();
  const today = localDayISO(new Date().toISOString(), tz);
  const projectOf = (e: TimeEntry) => (projectsQ.data ?? []).find((p) => p.id === e.project_id) ?? null;

  // só os dias ANTERIORES (o grupo de hoje é do TodayList — CA-24.1)
  const groups = groupByDay(recentQ.data?.entries ?? [], tz, today).filter((g) => g.dayISO !== today);

  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'hidden' }}>
      <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border)' }}>
        <span style={{ fontSize: 14, fontWeight: 600 }}>Recentes</span>
        <span className="faint" style={{ fontSize: 12, marginLeft: 9 }}>
          últimos dias
        </span>
      </div>

      {groups.length === 0 ? (
        <div className="muted" style={{ padding: '18px 18px', fontSize: 13 }}>
          Nada cronometrado nos últimos dias.
        </div>
      ) : (
        groups.map((g) => (
          <div key={g.dayISO}>
            <div
              className="faint"
              style={{
                padding: '10px 16px 4px',
                fontSize: 11.5,
                fontWeight: 600,
                textTransform: 'capitalize',
              }}
            >
              {g.label}
            </div>
            <div style={{ padding: '0 14px' }}>
              {g.items.map((e) => {
                const project = projectOf(e);
                return (
                  <div
                    key={e.id}
                    data-testid={`recent-entry-${e.id}`}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 12,
                      height: 'var(--row-h)',
                      padding: '0 4px',
                      borderBottom: '1px solid var(--border)',
                    }}
                  >
                    <i className="tt-dot" style={{ background: project?.color ?? 'var(--text-3)' }} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div
                        style={{
                          fontSize: 'var(--fs-row)',
                          fontWeight: 500,
                          whiteSpace: 'nowrap',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                        }}
                      >
                        {e.description || '(sem descrição)'}
                      </div>
                      {project && (
                        <div className="faint" style={{ fontSize: 11.5 }}>
                          {project.name}
                        </div>
                      )}
                    </div>
                    <span className="mono faint" style={{ fontSize: 12 }}>
                      {fmtHourMinute(e.started_at)}
                    </span>
                    <span
                      className="mono"
                      style={{ fontSize: 'var(--fs-row)', fontWeight: 540, minWidth: 78, textAlign: 'right' }}
                    >
                      {fmtClock(e.duration_seconds ?? 0)}
                    </span>
                    <button
                      className="tt-iconbtn"
                      title="Editar"
                      aria-label="Editar"
                      style={{ width: 28, height: 28, background: 'transparent', color: 'var(--text-3)' }}
                      onClick={() => setEditando(e)}
                    >
                      <Icon.dots size={13} />
                    </button>
                    <button
                      className="tt-iconbtn"
                      title="Continuar"
                      aria-label="Continuar"
                      style={{ width: 28, height: 28, background: 'transparent', color: 'var(--text-3)' }}
                      onClick={() => continuar.mutate(e.id)}
                    >
                      <Icon.play size={13} />
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        ))
      )}
      {editando && ws && (
        <EntryModal wsId={ws.id} entry={editando} onClose={() => setEditando(null)} />
      )}
    </section>
  );
}
