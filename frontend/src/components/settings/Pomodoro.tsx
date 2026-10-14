// Grupo Pomodoro dos Ajustes (T-18 fase 03, estendido na fase 15 — CA-20/
// 20.1). Foco (slider 15–90) + pausa (presets/custom 1–60) da fase 03, MAIS:
// ciclos até a pausa longa (1–12), duração da pausa longa (1–120) e 3
// toggles (auto-start pausa / auto-start foco / alerta sonoro). Validação no
// client → role=alert sem chamar a API; válido → PUT com o corpo COMPLETO
// (7 campos, CA-02).
import { useState } from 'react';
import Switch from '../Switch';
import { usePomodoroSettings, useSavePomodoro } from '../../lib/queries/settings';
import type { PomodoroSettings } from '../../lib/api';

const PRESETS = [5, 15, 20];

export default function PomodoroGroup() {
  const { data } = usePomodoroSettings();
  if (!data) return null;
  return <GroupForm initial={data} />;
}

function GroupForm({ initial }: { initial: PomodoroSettings }) {
  const salvar = useSavePomodoro();
  const [focus, setFocus] = useState(initial.focus_minutes);
  const [presetBreak, setPresetBreak] = useState(initial.break_minutes);
  const [custom, setCustom] = useState(
    PRESETS.includes(initial.break_minutes) ? '' : String(initial.break_minutes),
  );
  const [cycles, setCycles] = useState(String(initial.cycles_before_long));
  const [longBreak, setLongBreak] = useState(String(initial.long_break_minutes));
  const [autoStartBreak, setAutoStartBreak] = useState(initial.auto_start_break);
  const [autoStartFocus, setAutoStartFocus] = useState(initial.auto_start_focus);
  const [soundEnabled, setSoundEnabled] = useState(initial.sound_enabled);
  const [erro, setErro] = useState<string | null>(null);
  const [salvo, setSalvo] = useState(false);

  // custom preenchido vence o preset; vazio → preset selecionado
  const breakMinutes = custom.trim() !== '' ? Number(custom) : presetBreak;
  const cyclesNum = Number(cycles);
  const longBreakNum = Number(longBreak);

  const submit = () => {
    setErro(null);
    setSalvo(false);
    if (!Number.isInteger(breakMinutes) || breakMinutes < 1 || breakMinutes > 60) {
      setErro('A pausa deve ficar entre 1 e 60 minutos.');
      return;
    }
    if (!Number.isInteger(cyclesNum) || cyclesNum < 1 || cyclesNum > 12) {
      setErro('Os ciclos até a pausa longa devem ficar entre 1 e 12.');
      return;
    }
    if (!Number.isInteger(longBreakNum) || longBreakNum < 1 || longBreakNum > 120) {
      setErro('A pausa longa deve ficar entre 1 e 120 minutos.');
      return;
    }
    // CA-02: PUT com o corpo COMPLETO (7 campos)
    salvar.mutate(
      {
        focus_minutes: focus,
        break_minutes: breakMinutes,
        cycles_before_long: cyclesNum,
        long_break_minutes: longBreakNum,
        auto_start_break: autoStartBreak,
        auto_start_focus: autoStartFocus,
        sound_enabled: soundEnabled,
      },
      { onSuccess: () => setSalvo(true), onError: (err) => setErro(err.message) },
    );
  };

  return (
    <section className="tt-card" style={{ marginTop: 'var(--gap)', padding: 'var(--pad)' }}>
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 14, fontWeight: 600 }}>Pomodoro</div>
        <div className="faint" style={{ fontSize: 12, marginTop: 2 }}>
          Ao pausar, abre uma pausa com esta duração.
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8, fontSize: 13 }}>
            <label htmlFor="pomo-focus">Foco</label>
            <span className="mono" style={{ fontWeight: 600 }}>
              {focus} min
            </span>
          </div>
          <input
            id="pomo-focus"
            type="range"
            min={15}
            max={90}
            step={5}
            value={focus}
            onChange={(e) => setFocus(Number(e.target.value))}
            style={{ width: '100%', accentColor: 'var(--accent)' }}
          />
        </div>

        <div>
          <div style={{ fontSize: 13, marginBottom: 8 }}>Pausa</div>
          <div style={{ display: 'flex', gap: 8 }}>
            {PRESETS.map((m) => {
              const on = custom.trim() === '' && presetBreak === m;
              return (
                <button
                  key={m}
                  type="button"
                  aria-pressed={on}
                  onClick={() => {
                    setPresetBreak(m);
                    setCustom('');
                  }}
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
                  {m} min
                </button>
              );
            })}
            <input
              aria-label="Pausa personalizada (minutos)"
              type="number"
              min={1}
              max={60}
              placeholder="custom"
              value={custom}
              onChange={(e) => setCustom(e.target.value)}
              style={{
                width: 86,
                padding: '8px 10px',
                borderRadius: 9,
                border: '1px solid',
                borderColor: custom.trim() !== '' ? 'var(--accent)' : 'var(--border-2)',
                background: custom.trim() !== '' ? 'var(--accent-soft)' : 'var(--surface-2)',
                color: 'var(--text-1)',
                fontFamily: 'var(--font-ui)',
                fontSize: 12.5,
                outline: 'none',
              }}
            />
          </div>
        </div>

        {/* fase 15: ciclos até a pausa longa + duração da pausa longa */}
        <div style={{ display: 'flex', gap: 16 }}>
          <div style={{ flex: 1 }}>
            <label htmlFor="pomo-cycles" style={{ display: 'block', fontSize: 13, marginBottom: 8 }}>
              Ciclos até a pausa longa
            </label>
            <input
              id="pomo-cycles"
              type="number"
              min={1}
              max={12}
              value={cycles}
              onChange={(e) => setCycles(e.target.value)}
              style={inputNumStyle}
            />
          </div>
          <div style={{ flex: 1 }}>
            <label htmlFor="pomo-long-break" style={{ display: 'block', fontSize: 13, marginBottom: 8 }}>
              Pausa longa (min)
            </label>
            <input
              id="pomo-long-break"
              type="number"
              min={1}
              max={120}
              value={longBreak}
              onChange={(e) => setLongBreak(e.target.value)}
              style={inputNumStyle}
            />
          </div>
        </div>

        {/* fase 15: toggles de auto-start + alerta sonoro */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <ToggleRow
            label="Iniciar a pausa automaticamente"
            on={autoStartBreak}
            onToggle={() => setAutoStartBreak((v) => !v)}
          />
          <ToggleRow
            label="Voltar ao foco automaticamente"
            on={autoStartFocus}
            onToggle={() => setAutoStartFocus((v) => !v)}
          />
          <ToggleRow
            label="Alerta sonoro no fim de cada fase"
            on={soundEnabled}
            onToggle={() => setSoundEnabled((v) => !v)}
          />
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
              Salvo — vale na próxima pausa.
            </span>
          )}
        </div>
      </div>
    </section>
  );
}

const inputNumStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 10px',
  borderRadius: 9,
  border: '1px solid var(--border-2)',
  background: 'var(--surface-2)',
  color: 'var(--text-1)',
  fontFamily: 'var(--font-ui)',
  fontSize: 12.5,
  outline: 'none',
};

function ToggleRow({ label, on, onToggle }: { label: string; on: boolean; onToggle: () => void }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
      <span style={{ fontSize: 13 }}>{label}</span>
      <Switch on={on} onToggle={onToggle} label={label} />
    </div>
  );
}
