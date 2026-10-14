// FocoPicker (fase 15, CA-09..13) — a tela de ENTRADA do Modo Foco pensada
// pro toque (iPad/celular): um campo grande pra digitar o que se faz +
// projetos recentes como cartões grandes (≥80px) + continuar uma entrada
// recente. Substitui o ocioso "0:00:00 + sem timer rodando" da fase 05
// (desvio sancionado — spec §7). Toda cor é token (--focus-*/--accent*); o
// color do PROJETO é DADO de domínio (hex da API), não literal de UI.
import { useMemo, useState } from 'react';
import type { Project, TimeEntry } from '../../lib/api';
import { fmtClock } from '../../lib/format';
import { useClients } from '../../lib/queries/clients';
import { useTodayEntries } from '../../lib/queries/entries';
import { useContinueEntry } from '../../lib/queries/entries';
import { useProjects } from '../../lib/queries/projects';
import { useTimerStore } from '../../lib/timerStore';

const RECENT_PROJECTS = 5;
const CONTINUE_ITEMS = 3;

const wrapStyle: React.CSSProperties = {
  width: '100%',
  maxWidth: 780,
  display: 'flex',
  flexDirection: 'column',
  gap: 28,
};

const cardBase: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 10,
  padding: 16,
  borderRadius: 16,
  cursor: 'pointer',
  textAlign: 'left',
  minHeight: 92, // alvo de toque ≥80px (CA-10)
  border: '1.5px solid var(--focus-dim)',
  background: 'transparent',
  color: 'var(--focus-ink)',
  fontFamily: 'var(--font-ui)',
};

// ordena projetos ATIVOS por uso recente nas entries de hoje (project_id mais
// recente primeiro); fallback = ordem da listagem quando não há histórico.
function recentProjects(projects: Project[], entries: TimeEntry[]): Project[] {
  const order = new Map<string, number>();
  let rank = 0;
  for (const e of entries) {
    if (e.project_id && !order.has(e.project_id)) {
      order.set(e.project_id, rank++);
    }
  }
  const ranked = [...projects].sort((a, b) => {
    const ra = order.has(a.id) ? order.get(a.id)! : Number.POSITIVE_INFINITY;
    const rb = order.has(b.id) ? order.get(b.id)! : Number.POSITIVE_INFINITY;
    return ra - rb;
  });
  return ranked.slice(0, RECENT_PROJECTS);
}

export default function FocoPicker({ workspaceId }: { workspaceId: string }) {
  const start = useTimerStore((s) => s.start);
  const projectsQ = useProjects(workspaceId); // ativos (archived:false default)
  const clientsQ = useClients(workspaceId); // cache barato p/ nome do cliente
  const entriesQ = useTodayEntries(workspaceId);
  const continueEntry = useContinueEntry(workspaceId);

  const [description, setDescription] = useState('');
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);

  const projects = projectsQ.data ?? [];
  const entries = useMemo(() => entriesQ.data?.entries ?? [], [entriesQ.data]);
  const recents = useMemo(() => recentProjects(projects, entries), [projects, entries]);

  // nome do cliente do cache (CA-10): Project só tem client_id; degrada pra
  // cor+nome se o cliente não estiver carregado.
  const clientName = (clientId: string | null): string | null => {
    if (!clientId) return null;
    return clientsQ.data?.find((c) => c.id === clientId)?.name ?? null;
  };

  // continuar (CA-12): entries FECHADAS de hoje, mais recentes primeiro, top N
  const continuables = useMemo(
    () =>
      entries
        .filter((e) => e.stopped_at !== null)
        .slice()
        .sort((a, b) => Date.parse(b.started_at) - Date.parse(a.started_at))
        .slice(0, CONTINUE_ITEMS),
    [entries],
  );

  // CA-11: o start mínimo permitido pelo backend é "nada", mas a UX exige ao
  // menos descrição OU projeto pra fazer sentido.
  const canStart = description.trim() !== '' || selectedProjectId !== null;

  const begin = () => {
    if (!canStart) return;
    void start({
      workspace_id: workspaceId,
      description: description.trim(),
      project_id: selectedProjectId,
    });
  };

  const labelFor = (p: Project) => {
    const cli = clientName(p.client_id);
    return cli ? `${p.name}, ${cli}` : p.name;
  };

  return (
    <div data-testid="focus-picker" style={wrapStyle}>
      <div>
        <div
          className="eyebrow"
          style={{ color: 'var(--accent)', letterSpacing: '.14em', fontSize: 12, fontWeight: 600, textTransform: 'uppercase' }}
        >
          Modo Foco · nova sessão
        </div>
        <h1 style={{ margin: '8px 0 0', fontSize: 30, fontWeight: 600, letterSpacing: '-.02em', color: 'var(--focus-ink)' }}>
          No que você tá trabalhando?
        </h1>
      </div>

      {/* campo grande de descrição (CA-09/13) — input real e focável */}
      <input
        data-testid="focus-picker-description"
        aria-label="Descrição da sessão"
        type="text"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Digite o que você vai fazer…"
        style={{
          width: '100%',
          padding: '20px 24px',
          borderRadius: 18,
          border: '1.5px solid var(--accent-line)',
          background: 'transparent',
          color: 'var(--focus-ink)',
          fontFamily: 'var(--font-ui)',
          fontSize: 24,
          fontWeight: 500,
          outline: 'none',
        }}
      />

      {/* projetos recentes — cartões grandes tocáveis (CA-10) */}
      {recents.length > 0 && (
        <div>
          <div style={{ fontSize: 14, color: 'var(--focus-dim)', marginBottom: 12 }}>Projetos recentes</div>
          <div style={{ display: 'grid', gridTemplateColumns: `repeat(${Math.min(recents.length, RECENT_PROJECTS)}, 1fr)`, gap: 12 }}>
            {recents.map((p) => {
              const on = selectedProjectId === p.id;
              const cli = clientName(p.client_id);
              return (
                <button
                  key={p.id}
                  type="button"
                  data-testid="focus-picker-project"
                  aria-pressed={on}
                  aria-label={labelFor(p)}
                  onClick={() => setSelectedProjectId(on ? null : p.id)}
                  style={{
                    ...cardBase,
                    borderColor: on ? p.color : 'var(--focus-dim)',
                    background: on ? 'var(--accent-soft)' : 'transparent',
                  }}
                >
                  <span style={{ width: 12, height: 12, borderRadius: '50%', background: p.color }} />
                  <div>
                    <div style={{ fontSize: 16, fontWeight: 600, color: 'var(--focus-ink)' }}>{p.name}</div>
                    {cli && (
                      <div data-testid="focus-picker-client" style={{ fontSize: 12.5, color: 'var(--focus-dim)', marginTop: 2 }}>
                        {cli}
                      </div>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* continuar — entradas recentes (CA-12); some se não há nenhuma */}
      {continuables.length > 0 && (
        <div>
          <div style={{ fontSize: 14, color: 'var(--focus-dim)', marginBottom: 12 }}>Continuar</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {continuables.map((e) => {
              const p = projects.find((pr) => pr.id === e.project_id) ?? null;
              const dur = e.duration_seconds ?? 0;
              return (
                <button
                  key={e.id}
                  type="button"
                  data-testid="focus-picker-continue"
                  aria-label={`Continuar ${e.description || p?.name || 'entrada'}`}
                  onClick={() => continueEntry.mutate(e.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 14,
                    padding: '14px 18px',
                    borderRadius: 14,
                    cursor: 'pointer',
                    background: 'transparent',
                    border: '1px solid var(--focus-dim)',
                    color: 'var(--focus-ink)',
                    textAlign: 'left',
                    fontFamily: 'var(--font-ui)',
                  }}
                >
                  <span style={{ width: 9, height: 9, borderRadius: '50%', background: p ? p.color : 'var(--accent)' }} />
                  <span style={{ flex: 1, fontSize: 16, fontWeight: 500 }}>{e.description || p?.name || 'Entrada'}</span>
                  <span className="mono" style={{ fontSize: 14, color: 'var(--focus-dim)' }}>
                    {fmtClock(dur)}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* começar (CA-11) */}
      <button
        type="button"
        data-testid="focus-picker-start"
        disabled={!canStart}
        onClick={begin}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 12,
          height: 64,
          borderRadius: 16,
          border: 'none',
          cursor: canStart ? 'pointer' : 'not-allowed',
          background: 'var(--accent)',
          color: 'var(--accent-ink)',
          fontFamily: 'var(--font-ui)',
          fontSize: 18,
          fontWeight: 600,
          opacity: canStart ? 1 : 0.5,
        }}
      >
        Começar a cronometrar
      </button>
    </div>
  );
}
