// ObjectiveModal (fase 12 T-09, CA-21/02): criar/editar objetivo (nome,
// descrição, meta de horas). Valida no client ANTES da API: nome 1–200 e
// goal_hours vazio (sem meta) OU 0 < x <= 9999. Form inválido NÃO chama a API
// e sinaliza o erro. A meta vazia em modo Editar envia goal_hours:null (remove).
import { useState, type FormEvent } from 'react';
import type { BoardObjective } from '../../lib/api';
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

export default function ObjectiveModal({
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
  const [meta, setMeta] = useState(objective?.goal_hours != null ? String(objective.goal_hours) : '');
  const [erro, setErro] = useState('');

  const criar = useCreateObjective(wsId, projectId);
  const editar = usePatchObjective(wsId, projectId);
  const titulo = editing ? 'Editar objetivo' : 'Novo objetivo';

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const name = nome.trim();
    if (!name || name.length > 200) {
      setErro('Dê um nome ao objetivo (1–200 caracteres).');
      return;
    }
    const metaTrim = meta.trim();
    let goalHours: number | null = null;
    if (metaTrim !== '') {
      const n = Number(metaTrim);
      if (!Number.isFinite(n) || n <= 0 || n > 9999) {
        setErro('A meta de horas deve ser maior que 0 e no máximo 9999.');
        return;
      }
      goalHours = n;
    }
    setErro('');

    if (editing && objective) {
      editar.mutate(
        { id: objective.id, patch: { name, description: descricao, goal_hours: goalHours } },
        { onSuccess: onClose },
      );
    } else {
      criar.mutate(
        { name, description: descricao, ...(goalHours !== null ? { goal_hours: goalHours } : {}) },
        { onSuccess: onClose },
      );
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
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5 }}>
            <span className="faint">Meta de horas (opcional)</span>
            <input
              aria-label="Meta de horas"
              type="number"
              min={0}
              step="any"
              value={meta}
              onChange={(e) => setMeta(e.target.value)}
              placeholder="Ex.: 12"
              style={inputStyle}
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
            {editing ? 'Salvar' : 'Criar objetivo'}
          </button>
        </div>
      </form>
    </div>
  );
}
