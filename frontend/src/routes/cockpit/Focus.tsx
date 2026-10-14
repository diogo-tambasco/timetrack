// Modo Foco (fase 05) — o flip clock do iPad como 1ª classe: overlay
// fullscreen PRETO por cima do shell (D-01 — o AppShell continua montado
// por baixo com o runtime único do timer), estados como função direta do
// timerStore (D-07): ocioso / rodando / descanso. Dígitos 100% derivados
// localmente (useTimerClock — zero requisição por segundo, CA-01/RNF-03).
// Toda cor daqui é token --focus-*/--accent (CA-03 — nada de tema, nada de
// hex; check-cores barra hex literal).
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import FocoBreak from '../../components/focus/FocoBreak';
import FocoPicker from '../../components/focus/FocoPicker';
import { useCurrentWorkspace } from '../../lib/auth';
import { fmtClock, fmtDayTotal } from '../../lib/format';
import { entriesKey, useTodayEntries } from '../../lib/queries/entries';
import { useProjects } from '../../lib/queries/projects';
import { useTimerClock, useTimerStore } from '../../lib/timerStore';
import { useWakeLock } from '../../lib/wakeLock';

const overlayStyle: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  zIndex: 80, // acima do rail (z 30) e da TimerBar — plan §6
  background: 'var(--focus-bg)',
  color: 'var(--focus-ink)',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 22,
};

const digitsStyle: React.CSSProperties = {
  fontSize: 'clamp(72px, 16vw, 200px)',
  fontWeight: 600,
  letterSpacing: '-.03em',
  lineHeight: 1,
  fontVariantNumeric: 'tabular-nums',
};

const ghostBtn: React.CSSProperties = {
  padding: '9px 20px',
  borderRadius: 999,
  border: '1px solid var(--focus-dim)',
  background: 'transparent',
  color: 'var(--focus-dim)',
  fontFamily: 'var(--font-ui)',
  fontSize: 13,
  fontWeight: 500,
  cursor: 'pointer',
};

const exitStyle: React.CSSProperties = {
  position: 'absolute',
  top: 16,
  right: 16,
  width: 42,
  height: 42,
  borderRadius: '50%',
  border: 'none',
  background: 'transparent',
  color: 'var(--focus-dim)',
  fontSize: 19,
  cursor: 'pointer',
};

export default function Focus() {
  useWakeLock(); // CA-14/15/16 — comportamento provado no wakeLock.spec
  const navigate = useNavigate();
  const ws = useCurrentWorkspace();
  const qc = useQueryClient();
  const state = useTimerStore((s) => s.state);
  const pause = useTimerStore((s) => s.pause);
  const stop = useTimerStore((s) => s.stop);
  const offsetMs = useTimerStore((s) => s.offsetMs);
  const { elapsed, remaining } = useTimerClock();
  // archived:true — a entry rodando pode apontar pra projeto arquivado e o
  // contexto não pode sumir (mesma regra do chip da TimerBar — CA-06)
  const projectsQ = useProjects(ws?.id ?? '', { archived: true });
  const entriesQ = useTodayEntries(ws?.id ?? '');

  // D-06: a entry ativa mudou (start/stop/troca) → a recém-fechada precisa
  // entrar no total_seconds do servidor. Depende do ID (string), nunca do
  // objeto state — re-render por tick não pode invalidar query (plan §6).
  const entryId = state?.entry?.id;
  useEffect(() => {
    void qc.invalidateQueries({ queryKey: entriesKey(ws?.id ?? '') });
  }, [entryId, qc, ws?.id]);

  // total do dia (CA-10): fechado do servidor + vivo da ENTRY RODANDO vinda
  // da QUERY (aritmética do TodayList) — nunca o clock cru, que em pausa é
  // o elapsed da PAUSA e não é tempo trabalhado (D-06)
  const runningEntry = (entriesQ.data?.entries ?? []).find((e) => e.stopped_at === null) ?? null;
  const liveSeconds = runningEntry
    ? Math.max(0, Math.floor((Date.now() + offsetMs - Date.parse(runningEntry.started_at)) / 1000))
    : 0;
  const dayTotal = (entriesQ.data?.total_seconds ?? 0) + liveSeconds;

  const isBreak = state?.mode === 'break';
  const isRunning = state !== null && !isBreak;

  // contexto (CA-06): projeto > descrição > nada (sem placeholder)
  let context: string | null = null;
  if (state?.entry) {
    const project = (projectsQ.data ?? []).find((p) => p.id === state.entry?.project_id);
    context = project?.name ?? (state.entry.description || null);
  }

  // regra `shown` idêntica à RunningBar: countup → elapsed; resto → restante
  const shown = state?.mode === 'countup' ? elapsed : (remaining ?? elapsed);

  return (
    <div data-testid="focus" style={overlayStyle}>
      <button aria-label="Sair" title="Sair" style={exitStyle} onClick={() => navigate('/hoje')}>
        ✕
      </button>

      {/* CA-09 (fase 15): ocioso = FocoPicker (desvio sancionado da fase 05 —
          o ocioso "0:00:00 + sem timer" virou a tela de entrada com start) */}
      {!state && ws && <FocoPicker workspaceId={ws.id} />}

      {isRunning && (
        <>
          <div className="mono" data-testid="focus-digits" style={digitsStyle}>
            {fmtClock(shown)}
          </div>
          {context && (
            <span data-testid="focus-context" style={{ color: 'var(--focus-dim)', fontSize: 17 }}>
              {context}
            </span>
          )}
          <div style={{ display: 'flex', gap: 12, marginTop: 8 }}>
            <button style={ghostBtn} onClick={() => void pause()}>
              Pausar
            </button>
            <button style={ghostBtn} onClick={() => void stop()}>
              Parar
            </button>
          </div>
        </>
      )}

      {/* CA-14 (fase 15): modo descanso redesenhado (FocoBreak) */}
      {isBreak && <FocoBreak />}

      <span
        data-testid="focus-day-total"
        style={{ position: 'absolute', bottom: 18, color: 'var(--focus-dim)', fontSize: 13, pointerEvents: 'none' }}
      >
        hoje: {fmtDayTotal(dayTotal)}
      </span>
    </div>
  );
}
