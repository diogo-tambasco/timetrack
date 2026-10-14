// NoticeBanner (fase 16, CA-12): banner dispensável no shell que avisa do
// corte automático de 2h. Vive no AppShell → aparece em QUALQUER rota. Fechar
// (✕ ou Esc) só limpa o store — NÃO chama o servidor. O estado do timer já
// convergiu pra ocioso pelo evento type:"timer" que o corte também emite; o
// banner é o complemento humano.
import { Icon } from './icons';
import { useNoticeStore } from '../lib/noticeStore';

export default function NoticeBanner() {
  const notice = useNoticeStore((s) => s.notice);
  const dismiss = useNoticeStore((s) => s.dismiss);

  if (!notice) return null;

  // mensagem + ícone por kind (fase 18 CA-17 adiciona task_assigned). O default
  // cai no aviso de corte (timer_auto_cut) — o único kind anterior.
  // fase 20: lembrete (kind:"reminder") leva o título da task no payload.
  let message: string;
  let NoticeIcon: typeof Icon.clock;
  if (notice.kind === 'task_assigned') {
    message = 'Você recebeu uma nova task.';
    NoticeIcon = Icon.check;
  } else if (notice.kind === 'reminder') {
    message = `Lembrete: ${typeof notice.title === 'string' ? notice.title : 'uma task'}`;
    NoticeIcon = Icon.today;
  } else {
    message = 'Seu timer foi pausado automaticamente após 2h.';
    NoticeIcon = Icon.clock;
  }

  return (
    <div
      role="status"
      data-testid="notice-banner"
      onKeyDown={(e) => {
        if (e.key === 'Escape') dismiss();
      }}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--gap)',
        padding: '10px var(--pad)',
        background: 'var(--surface-2)',
        borderBottom: '1px solid var(--line)',
        color: 'var(--text-1)',
        fontSize: 13.5,
      }}
    >
      <span style={{ display: 'inline-flex', color: 'var(--accent-ink)' }} aria-hidden>
        <NoticeIcon />
      </span>
      <span style={{ flex: 1, minWidth: 0 }}>{message}</span>
      <button
        type="button"
        className="tt-btn tt-btn--ghost"
        aria-label="Dispensar aviso"
        onClick={dismiss}
        style={{ padding: 4, lineHeight: 0 }}
      >
        <Icon.x />
      </button>
    </div>
  );
}
