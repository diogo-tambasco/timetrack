// ReminderButton (fase 20, T-09): ação "Lembrar" no modal de task (modo Editar).
// Abre um picker de QUANDO — presets ("em 1h", "amanhã 9h", "no prazo" se houver
// due_date) + um input de data/hora — e cria o reminder manual via
// useCreateReminder. Pós-sucesso fecha o picker e mostra uma confirmação leve.
// O carve-out viewer-assignee da API vale; pela UI do board hoje o viewer não
// abre o modal (limitação registrada na fase 19), então o botão é alcançado por
// owner/member — a API suporta o viewer-assignee (E2E/teste de API cobrem).
import { useState } from 'react';
import { useCreateReminder } from '../../lib/queries/reminders';

// toLocalInput converte um Date pro valor de um <input type="datetime-local">
// (YYYY-MM-DDTHH:mm na PAREDE local — sem segundos/tz).
function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// presets calculam um Date local; o backend recebe RFC3339 (toISOString, com tz).
function inOneHour(): Date {
  return new Date(Date.now() + 60 * 60 * 1000);
}
function tomorrowAt9(): Date {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  d.setHours(9, 0, 0, 0);
  return d;
}
// "no prazo": 09:00 LOCAL do dia da due_date (YYYY-MM-DD). Mesma regra do due.
function dueAt9(dueDate: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dueDate);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 9, 0, 0, 0);
}

export default function ReminderButton({
  wsId,
  taskId,
  dueDate = null,
}: {
  wsId: string;
  taskId: string;
  dueDate?: string | null;
}) {
  const [open, setOpen] = useState(false);
  const [when, setWhen] = useState<string>(toLocalInput(inOneHour()));
  const [done, setDone] = useState(false);
  const [erro, setErro] = useState('');
  const criar = useCreateReminder(wsId, taskId);

  const create = (at: Date | null) => {
    if (!at || Number.isNaN(at.getTime())) {
      setErro('Escolha uma data/hora válida.');
      return;
    }
    setErro('');
    criar.mutate(at.toISOString(), {
      onSuccess: () => {
        setDone(true);
        setOpen(false);
      },
      onError: () => setErro('Não foi possível criar o lembrete.'),
    });
  };

  if (!open) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          onClick={() => {
            setDone(false);
            setOpen(true);
          }}
        >
          Lembrar
        </button>
        {done && (
          <span className="muted" role="status" style={{ fontSize: 12.5 }}>
            Lembrete criado.
          </span>
        )}
      </div>
    );
  }

  return (
    <div
      style={{ display: 'flex', flexDirection: 'column', gap: 8 }}
      data-testid="reminder-picker"
    >
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
        <button type="button" className="tt-btn tt-btn--ghost" onClick={() => create(inOneHour())}>
          Em 1h
        </button>
        <button type="button" className="tt-btn tt-btn--ghost" onClick={() => create(tomorrowAt9())}>
          Amanhã 9h
        </button>
        {dueDate && (
          <button
            type="button"
            className="tt-btn tt-btn--ghost"
            onClick={() => create(dueAt9(dueDate))}
          >
            No prazo
          </button>
        )}
      </div>
      <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
        <input
          aria-label="Quando lembrar"
          type="datetime-local"
          value={when}
          onChange={(e) => setWhen(e.target.value)}
          style={{
            background: 'var(--surface-2)',
            border: '1px solid var(--border-2)',
            borderRadius: 8,
            color: 'var(--text-1)',
            font: 'inherit',
            fontSize: 13,
            padding: '6px 8px',
            flex: 1,
          }}
        />
        <button
          type="button"
          className="tt-btn tt-btn--accent"
          onClick={() => create(when ? new Date(when) : null)}
        >
          Lembrar
        </button>
        <button type="button" className="tt-btn tt-btn--ghost" onClick={() => setOpen(false)}>
          Cancelar
        </button>
      </div>
      {erro && (
        <p role="alert" style={{ margin: 0, fontSize: 12.5, color: 'var(--danger)' }}>
          {erro}
        </p>
      )}
    </div>
  );
}
