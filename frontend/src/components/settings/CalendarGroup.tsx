// Grupo "Calendário" dos Ajustes (fase 11 T-10, CA-02/03/25): edita a view
// padrão, a faixa de horas e a densidade do Calendário 2.0. Salva via PUT
// (useSaveCalendarSettings); formulário inválido (hour_end ≤ hour_start) NÃO
// chama a API e sinaliza o erro. As preferências sincronizam entre devices.
import { useState } from 'react';
import { useCalendarSettings, useSaveCalendarSettings } from '../../lib/queries/calendar';
import type { CalendarSettings } from '../../lib/api';

const VIEWS: { v: CalendarSettings['default_view']; label: string }[] = [
  { v: 'day', label: 'Dia' },
  { v: 'week', label: 'Semana' },
  { v: 'month', label: 'Mês' },
];

const fieldStyle = {
  padding: '8px 10px',
  borderRadius: 9,
  border: '1px solid var(--border-2)',
  background: 'var(--surface-2)',
  color: 'var(--text-1)',
  fontFamily: 'var(--font-ui)',
  fontSize: 12.5,
  outline: 'none',
} as const;

export default function CalendarGroup() {
  const { data } = useCalendarSettings();
  if (!data) return null;
  return <GroupForm initial={data} />;
}

function GroupForm({ initial }: { initial: CalendarSettings }) {
  const salvar = useSaveCalendarSettings();
  const [view, setView] = useState<CalendarSettings['default_view']>(initial.default_view);
  const [hourStart, setHourStart] = useState(initial.hour_start);
  const [hourEnd, setHourEnd] = useState(initial.hour_end);
  const [density, setDensity] = useState<CalendarSettings['density']>(initial.density);
  const [erro, setErro] = useState<string | null>(null);
  const [salvo, setSalvo] = useState(false);

  const submit = () => {
    setErro(null);
    setSalvo(false);
    if (hourEnd <= hourStart) {
      setErro('A hora final deve ser maior que a inicial.');
      return;
    }
    salvar.mutate(
      { default_view: view, hour_start: hourStart, hour_end: hourEnd, density },
      { onSuccess: () => setSalvo(true), onError: (err) => setErro(err.message) },
    );
  };

  return (
    <section className="tt-card" style={{ marginTop: 'var(--gap)', padding: 'var(--pad)' }}>
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 14, fontWeight: 600 }}>Calendário</div>
        <div className="faint" style={{ fontSize: 12, marginTop: 2 }}>
          View padrão, faixa de horas e densidade da agenda — vale em todos os dispositivos.
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div>
          <div style={{ fontSize: 13, marginBottom: 8 }}>View padrão</div>
          <div style={{ display: 'flex', gap: 8 }}>
            {VIEWS.map(({ v, label }) => {
              const on = v === view;
              return (
                <button
                  key={v}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setView(v)}
                  style={{
                    flex: 1,
                    padding: '8px 0',
                    borderRadius: 9,
                    cursor: 'pointer',
                    fontFamily: 'var(--font-ui)',
                    fontSize: 12.5,
                    fontWeight: 540,
                    border: '1px solid',
                    borderColor: on ? 'var(--accent)' : 'var(--border-2)',
                    background: on ? 'var(--accent-soft)' : 'transparent',
                    color: on ? 'var(--accent)' : 'var(--text-2)',
                  }}
                >
                  {label}
                </button>
              );
            })}
          </div>
        </div>

        <div style={{ display: 'flex', gap: 16 }}>
          <label style={{ flex: 1, fontSize: 13 }}>
            <span style={{ display: 'block', marginBottom: 8 }}>Hora inicial</span>
            <input
              aria-label="Hora inicial"
              type="number"
              min={0}
              max={23}
              value={hourStart}
              onChange={(e) => setHourStart(Number(e.target.value))}
              style={{ ...fieldStyle, width: '100%' }}
            />
          </label>
          <label style={{ flex: 1, fontSize: 13 }}>
            <span style={{ display: 'block', marginBottom: 8 }}>Hora final</span>
            <input
              aria-label="Hora final"
              type="number"
              min={1}
              max={24}
              value={hourEnd}
              onChange={(e) => setHourEnd(Number(e.target.value))}
              style={{ ...fieldStyle, width: '100%' }}
            />
          </label>
        </div>

        <div>
          <div style={{ fontSize: 13, marginBottom: 8 }}>Densidade</div>
          <div style={{ display: 'flex', gap: 8 }}>
            {(['comfortable', 'compact'] as const).map((d) => {
              const on = d === density;
              return (
                <button
                  key={d}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setDensity(d)}
                  style={{
                    flex: 1,
                    padding: '8px 0',
                    borderRadius: 9,
                    cursor: 'pointer',
                    fontFamily: 'var(--font-ui)',
                    fontSize: 12.5,
                    fontWeight: 540,
                    border: '1px solid',
                    borderColor: on ? 'var(--accent)' : 'var(--border-2)',
                    background: on ? 'var(--accent-soft)' : 'transparent',
                    color: on ? 'var(--accent)' : 'var(--text-2)',
                  }}
                >
                  {d === 'comfortable' ? 'Confortável' : 'Compacta'}
                </button>
              );
            })}
          </div>
        </div>

        {erro && (
          <span role="alert" style={{ color: 'var(--danger)', fontSize: 12.5 }}>
            {erro}
          </span>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <button type="button" className="tt-btn tt-btn--accent" onClick={submit}>
            Salvar
          </button>
          {salvo && (
            <span className="faint" style={{ fontSize: 12 }}>
              Salvo — vale em todos os dispositivos.
            </span>
          )}
        </div>
      </div>
    </section>
  );
}
