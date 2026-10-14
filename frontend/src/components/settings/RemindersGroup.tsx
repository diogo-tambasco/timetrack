// Grupo "Meus lembretes" dos Ajustes (fase 20, T-09, plan §5): lista os
// lembretes PENDENTES do usuário (cross-workspace) com o nome da task + quando,
// e um botão pra cancelar cada um. Espelha o NotificationsGroup da fase 16. A
// lista invalida ao cancelar (useCancelReminder) — o item some sem reload.
import { useReminders, useCancelReminder } from '../../lib/queries/reminders';

function fmtWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export default function RemindersGroup() {
  const { data: reminders = [], isLoading } = useReminders('pending');
  const cancel = useCancelReminder();

  return (
    <div
      className="tt-card"
      style={{ marginTop: 'var(--gap)', padding: 'var(--pad)' }}
      data-testid="reminders-group"
    >
      <div style={{ fontWeight: 540, fontSize: 14 }}>Meus lembretes</div>
      <div className="muted" style={{ fontSize: 12.5, marginBottom: 8 }}>
        Lembretes pendentes de prazos e tasks. Cancele os que não precisa mais.
      </div>

      {isLoading ? (
        <div className="muted" style={{ fontSize: 12.5 }}>
          Carregando…
        </div>
      ) : reminders.length === 0 ? (
        <div className="muted" style={{ fontSize: 12.5 }} data-testid="reminders-empty">
          Nenhum lembrete pendente.
        </div>
      ) : (
        <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
          {reminders.map((r) => (
            <li
              key={r.id}
              data-testid="reminder-item"
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 'var(--gap)',
                padding: '8px 10px',
                background: 'var(--surface-2)',
                borderRadius: 8,
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 13, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {r.task_name ?? 'Task'}
                </div>
                <div className="muted" style={{ fontSize: 12 }}>
                  {r.kind === 'due' ? 'Prazo' : 'Lembrete'} · {fmtWhen(r.remind_at)}
                </div>
              </div>
              <button
                type="button"
                className="tt-btn tt-btn--ghost"
                aria-label={`Cancelar lembrete de ${r.task_name ?? 'task'}`}
                disabled={cancel.isPending}
                onClick={() =>
                  cancel.mutate({ ws: r.workspace_id, task: r.task_id, reminderId: r.id })
                }
              >
                Cancelar
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
