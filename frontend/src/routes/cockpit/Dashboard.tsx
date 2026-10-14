// Dashboard (fase 13, T-07) — a HOME de comando (Direção C "Hero Timer"):
// substitui o /hoje (CA-04). Saudação + card do timer (hero) + entradas de HOJE
// (TodayList reusado — CA-24.1) + Recentes dos dias anteriores (P3) + metas da
// semana + atalho pro Foco. Mantém a TimerBar global do topo (não duplica — o
// card só reflete o store). Sem backend novo (RNF-03). Cada seção é resiliente
// (CA-21): trata seu próprio isLoading/isError, nenhuma derruba as outras.
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Icon } from '../../components/icons';
import TimerCard from '../../components/dashboard/TimerCard';
import RecentEntries from '../../components/dashboard/RecentEntries';
import WeekGoals from '../../components/dashboard/WeekGoals';
import EntryModal from '../../components/timer/EntryModal';
import TodayList from '../../components/timer/TodayList';
import PresenceStrip from '../../components/PresenceStrip';
import { useAuthStore, useCurrentWorkspace } from '../../lib/auth';

export default function Dashboard() {
  const ws = useCurrentWorkspace();
  const me = useAuthStore((s) => s.me);
  const [novaEntrada, setNovaEntrada] = useState(false);

  const firstName = me?.user.name?.trim().split(/\s+/)[0] ?? '';
  const saudacao = firstName ? `Bom trabalho, ${firstName}.` : 'Bom trabalho.';
  const hoje = new Intl.DateTimeFormat('pt-BR', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(new Date());

  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 'var(--gap)' }}>
      {/* fase 22: cabeçalho quebra linha < md (o botão "Nova entrada" cai) */}
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: 12,
        }}
      >
        <div>
          <p className="eyebrow" style={{ textTransform: 'capitalize' }}>{hoje}</p>
          <h1 style={{ margin: '4px 0 0', fontSize: 26, fontWeight: 600, letterSpacing: '-.02em' }}>
            {saudacao}
          </h1>
        </div>
        <button className="tt-btn tt-btn--accent" onClick={() => setNovaEntrada(true)}>
          <Icon.plus size={15} sw={2.2} />
          Nova entrada
        </button>
      </div>

      <TimerCard onNewEntry={() => setNovaEntrada(true)} />

      {/* fase 21: presence ao vivo dos colegas do workspace (aditivo — some
          sozinho quando ninguém está ativo). */}
      <PresenceStrip wsId={ws?.id} />

      {/* fase 22: 1 coluna < md, 2 colunas (1.6fr/1fr — idêntico ao inline de
          hoje) ≥ md. Grid via classe Tailwind pra responder ao breakpoint. */}
      <div
        data-testid="dashboard-grid"
        className="grid grid-cols-1 md:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]"
        style={{ gap: 'var(--gap)', alignItems: 'start' }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--gap)', minWidth: 0 }}>
          <TodayList />
          <RecentEntries />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--gap)', minWidth: 0 }}>
          <WeekGoals />
          <Link
            to="/foco"
            className="tt-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              padding: '16px 18px',
              textDecoration: 'none',
              color: 'var(--text-1)',
            }}
          >
            <Icon.focus size={18} style={{ color: 'var(--accent-ink)' }} />
            <div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>Modo Foco</div>
              <div className="faint" style={{ fontSize: 12 }}>
                Flip-clock em tela cheia
              </div>
            </div>
          </Link>
        </div>
      </div>

      {novaEntrada && ws && <EntryModal wsId={ws.id} onClose={() => setNovaEntrada(false)} />}
    </section>
  );
}
