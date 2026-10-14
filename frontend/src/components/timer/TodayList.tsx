// TodayList (T-17) — a lista "Hoje" do cockpit (CA-35), referência visual:
// Timesheet de parts.jsx. Mais recente primeiro (ordem do servidor), botão
// continuar por linha, a entry rodando em accent com stop e duração viva
// derivada de started_at + offset (RNF-02 — nunca acumula tempo local).
import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Icon } from '../icons';
import { useCurrentWorkspace } from '../../lib/auth';
import { fmtClock, fmtHourMinute } from '../../lib/format';
import { useProjects } from '../../lib/queries/projects';
import { entriesKey, useContinueEntry, useTodayEntries } from '../../lib/queries/entries';
import { useTimerStore } from '../../lib/timerStore';
import EntryModal from './EntryModal';
import type { TimeEntry } from '../../lib/api';

export default function TodayList() {
  const ws = useCurrentWorkspace();
  const qc = useQueryClient();
  const entriesQ = useTodayEntries(ws?.id ?? '');
  // archived:true — entries do dia podem apontar pra projeto arquivado e a
  // linha precisa do nome/cor mesmo assim
  const projectsQ = useProjects(ws?.id ?? '', { archived: true });
  const continuar = useContinueEntry(ws?.id ?? '');
  const stop = useTimerStore((s) => s.stop);
  const offsetMs = useTimerStore((s) => s.offsetMs);
  const tick = useTimerStore((s) => s.tick);
  const activeEntryID = useTimerStore((s) => s.state?.entry?.id ?? null);
  const [editando, setEditando] = useState<TimeEntry | null>(null);

  // start/stop/pause mudam o dia: re-busca quando a entry ativa muda
  useEffect(() => {
    void qc.invalidateQueries({ queryKey: entriesKey(ws?.id ?? '') });
  }, [activeEntryID, qc, ws?.id]);

  const entries = entriesQ.data?.entries ?? [];
  const projectOf = (e: TimeEntry) =>
    (projectsQ.data ?? []).find((p) => p.id === e.project_id) ?? null;

  // duração viva da rodando: relógio do servidor (offset) − started_at
  void tick;
  const liveSeconds = (e: TimeEntry) =>
    Math.max(0, Math.floor((Date.now() + offsetMs - Date.parse(e.started_at)) / 1000));

  const running = entries.find((e) => e.stopped_at === null) ?? null;
  const totalSeconds = (entriesQ.data?.total_seconds ?? 0) + (running ? liveSeconds(running) : 0);

  return (
    <section className="tt-panel" style={{ borderRadius: 'var(--r-lg)', overflow: 'hidden' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '14px 16px',
          borderBottom: '1px solid var(--border)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
          <span style={{ fontSize: 14, fontWeight: 600 }}>Hoje</span>
          <span className="faint" style={{ fontSize: 12 }}>
            {entries.length} {entries.length === 1 ? 'entrada' : 'entradas'}
          </span>
        </div>
        <div style={{ textAlign: 'right' }}>
          <span className="faint" style={{ fontSize: 11.5, marginRight: 8 }}>
            Total hoje
          </span>
          <span className="mono" style={{ fontSize: 15, fontWeight: 600 }}>
            {fmtClock(totalSeconds)}
          </span>
        </div>
      </div>

      <div style={{ padding: '6px 14px' }}>
        {entries.length === 0 && (
          <div className="muted" style={{ padding: '18px 4px', fontSize: 13 }}>
            Nada cronometrado hoje ainda.
          </div>
        )}
        {entries.map((e) => {
          const isRunning = e.stopped_at === null;
          const project = projectOf(e);
          return (
            <div
              key={e.id}
              data-testid={`entry-${e.id}`}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                height: 'var(--row-h)',
                padding: '0 4px',
                borderBottom: '1px solid var(--border)',
              }}
            >
              <i
                className="tt-dot"
                style={{ background: project?.color ?? 'var(--text-3)' }}
              />
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
                data-live-duration={isRunning ? '' : undefined}
                style={{
                  fontSize: 'var(--fs-row)',
                  fontWeight: 540,
                  minWidth: 78,
                  textAlign: 'right',
                  color: isRunning ? 'var(--accent)' : 'var(--text-1)',
                }}
              >
                {isRunning ? fmtClock(liveSeconds(e)) : fmtClock(e.duration_seconds ?? 0)}
              </span>
              {isRunning ? (
                <button
                  className="tt-iconbtn"
                  title="Parar"
                  aria-label="Parar"
                  style={{ width: 28, height: 28, background: 'transparent', color: 'var(--accent)' }}
                  onClick={() => void stop()}
                >
                  <Icon.stop size={13} />
                </button>
              ) : (
                <>
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
                </>
              )}
            </div>
          );
        })}
      </div>
      {editando && ws && (
        <EntryModal wsId={ws.id} entry={editando} onClose={() => setEditando(null)} />
      )}
    </section>
  );
}
