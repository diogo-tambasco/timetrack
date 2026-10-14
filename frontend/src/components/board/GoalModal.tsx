// GoalModal (fase 29 T-09, CA-13): criar/editar Goal com SELETOR DE MODO de
// conclusão (by_tasks/by_hours/by_date). by_hours mostra goal_hours; by_date
// mostra a data do marco (target_date); by_tasks esconde os dois. Valida no
// client ANTES da API (nome 1–200; goal_hours em (0,9999] se by_hours; data
// válida se by_date). Inválido NÃO chama a API e sinaliza. Edit pré-preenchido,
// PATCH só com o diff (padrão da fase 12). Trocar modo NÃO zera goal_hours/
// target_date residual (CA-04 — só são reescritos se vierem no diff).
import { useState, type FormEvent } from 'react';
import type { BoardObjective, CompletionMode, ObjectiveInput, ObjectivePatch } from '../../lib/api';
import { Icon } from '../icons';
import { useCreateObjective, usePatchObjective } from '../../lib/queries/objectives';

const inputStyle = {
  background: 'var(--surface-2)',
  border: '1px solid var(--border-2)',
  borderRadius: 8,
  color: 'var(--text-1)',
  font: 'inherit',
  fontSize: 13,
  padding: '8px 10px',
  width: '100%',
} as const;

const MODES: { mode: CompletionMode; label: string }[] = [
  { mode: 'by_tasks', label: 'Por tasks' },
  { mode: 'by_hours', label: 'Por horas' },
  { mode: 'by_date', label: 'Por data' },
];

export default function GoalModal({
  wsId,
  projectId,
  objective = null,
  onClose,
}: {
  wsId: string;
  projectId: string;
  objective?: BoardObjective | null;
  onClose: () => void;
}) {
  const editing = objective !== null;
  const [nome, setNome] = useState(objective?.name ?? '');
  const [descricao, setDescricao] = useState(objective?.description ?? '');
  const [mode, setMode] = useState<CompletionMode>(objective?.completion_mode ?? 'by_tasks');
  const [meta, setMeta] = useState(objective?.goal_hours != null ? String(objective.goal_hours) : '');
  const [data, setData] = useState(objective?.target_date ?? '');
  const [erro, setErro] = useState('');

  const criar = useCreateObjective(wsId, projectId);
  const editar = usePatchObjective(wsId, projectId);
  const titulo = editing ? 'Editar goal' : 'Novo goal';

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const name = nome.trim();
    if (!name || name.length > 200) {
      setErro('Dê um nome ao goal (1–200 caracteres).');
      return;
    }
    let goalHours: number | null = null;
    if (mode === 'by_hours') {
      const n = Number(meta.trim());
      if (meta.trim() === '' || !Number.isFinite(n) || n <= 0 || n > 9999) {
        setErro('A meta de horas deve ser maior que 0 e no máximo 9999.');
        return;
      }
      goalHours = n;
    }
    let targetDate: string | null = null;
    if (mode === 'by_date') {
      const d = data.trim();
      if (!/^\d{4}-\d{2}-\d{2}$/.test(d) || Number.isNaN(Date.parse(d))) {
        setErro('Escolha uma data válida para o marco (AAAA-MM-DD).');
        return;
      }
      targetDate = d;
    }
    setErro('');

    if (editing && objective) {
      // PATCH só com o diff (CA-04): mode/goal_hours/target_date residual não
      // são reescritos a menos que mudem sob o modo atual.
      const patch: ObjectivePatch = {};
      if (name !== objective.name) patch.name = name;
      if (descricao !== objective.description) patch.description = descricao;
      if (mode !== objective.completion_mode) patch.completion_mode = mode;
      if (mode === 'by_hours' && goalHours !== objective.goal_hours) patch.goal_hours = goalHours;
      if (mode === 'by_date' && targetDate !== (objective.target_date ?? null)) {
        patch.target_date = targetDate;
      }
      editar.mutate({ id: objective.id, patch }, { onSuccess: onClose });
    } else {
      const input: ObjectiveInput = { name, description: descricao, completion_mode: mode };
      if (mode === 'by_hours' && goalHours !== null) input.goal_hours = goalHours;
      if (mode === 'by_date' && targetDate !== null) input.target_date = targetDate;
      criar.mutate(input, { onSuccess: onClose });
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 60, display: 'grid', placeItems: 'center' }}>
      <div
        aria-hidden
        onClick={onClose}
        style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(2px)' }}
      />
      <form
        role="dialog"
        aria-modal="true"
        aria-label={titulo}
        onSubmit={submit}
        className="tt-card"
        style={{
          position: 'relative',
          width: 460,
          maxWidth: 'calc(100vw - 32px)',
          background: 'var(--surface-1)',
          boxShadow: 'var(--shadow)',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <div style={{ fontSize: 16, fontWeight: 600 }}>{titulo}</div>
          <button
            type="button"
            aria-label="Fechar"
            className="tt-iconbtn"
            onClick={onClose}
            style={{ width: 30, height: 30, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.x size={16} />
          </button>
        </div>

        <div style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Nome</span>
            <input
              aria-label="Nome do objetivo"
              value={nome}
              onChange={(e) => setNome(e.target.value)}
              placeholder="Ex.: Lançar onboarding v2"
              style={inputStyle}
            />
          </label>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Como conclui</span>
            <div role="group" aria-label="Modo de conclusão" style={{ display: 'flex', gap: 6 }}>
              {MODES.map(({ mode: m, label }) => {
                const on = mode === m;
                return (
                  <button
                    key={m}
                    type="button"
                    aria-pressed={on}
                    onClick={() => setMode(m)}
                    className="tt-btn"
                    style={{
                      flex: 1,
                      fontSize: 12.5,
                      color: on ? 'var(--accent)' : 'var(--text-2)',
                      border: on ? '1px solid var(--accent-line)' : '1px solid var(--border)',
                      background: on ? 'var(--accent-soft)' : 'var(--surface-2)',
                    }}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>

          {mode === 'by_hours' && (
            <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
              <span className="faint">Meta de horas</span>
              <input
                aria-label="Meta de horas"
                type="number"
                min={0}
                step="any"
                value={meta}
                onChange={(e) => setMeta(e.target.value)}
                placeholder="Ex.: 40"
                style={inputStyle}
              />
            </label>
          )}

          {mode === 'by_date' && (
            <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
              <span className="faint">Data do marco</span>
              <input
                aria-label="Data do marco"
                type="date"
                value={data}
                onChange={(e) => setData(e.target.value)}
                style={inputStyle}
              />
            </label>
          )}

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Descrição (opcional)</span>
            <textarea
              aria-label="Descrição do objetivo"
              value={descricao}
              onChange={(e) => setDescricao(e.target.value)}
              rows={2}
              style={{ ...inputStyle, resize: 'vertical' }}
            />
          </label>

          {erro && (
            <p role="alert" style={{ margin: 0, fontSize: 12.5, color: 'var(--danger)' }}>
              {erro}
            </p>
          )}
        </div>

        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 8,
            padding: '14px 20px',
            borderTop: '1px solid var(--border)',
          }}
        >
          <button type="button" className="tt-btn tt-btn--ghost" onClick={onClose}>
            Cancelar
          </button>
          <button type="submit" className="tt-btn tt-btn--accent">
            {editing ? 'Salvar' : 'Criar goal'}
          </button>
        </div>
      </form>
    </div>
  );
}
