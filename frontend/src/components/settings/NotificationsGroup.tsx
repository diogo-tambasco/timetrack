// Grupo "Notificações" dos Ajustes (fase 16, CA-13/17): um botão que pede a
// permissão de Web Push num GESTO do usuário (NÃO no load — boa prática +
// Safari/iOS exige gesto). Degrada sem suporte/negado: o botão informa, mas o
// app segue (o banner in-app cobre quem está com o app aberto).
import { useState } from 'react';
import { enablePush, pushSupported, type EnablePushResult } from '../../lib/push';

const MENSAGEM: Record<EnablePushResult, string> = {
  subscribed: 'Notificações ativadas neste navegador.',
  unsupported: 'Este navegador não suporta notificações push.',
  denied: 'Permissão negada — ative as notificações nas configurações do navegador.',
  'no-key': 'Notificações indisponíveis no servidor no momento.',
  error: 'Não foi possível ativar as notificações. Tente novamente.',
};

export default function NotificationsGroup() {
  const [status, setStatus] = useState<EnablePushResult | null>(null);
  const [pending, setPending] = useState(false);
  const supported = pushSupported();

  const onClick = async () => {
    setPending(true);
    const res = await enablePush();
    setStatus(res);
    setPending(false);
  };

  return (
    <div className="tt-card" style={{ marginTop: 'var(--gap)', padding: 'var(--pad)' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 'var(--gap)',
        }}
      >
        <div>
          <div style={{ fontWeight: 540, fontSize: 14 }}>Notificações</div>
          <div className="muted" style={{ fontSize: 12.5 }}>
            Avisa no celular/desktop quando o timer é pausado automaticamente após 2h.
          </div>
        </div>
        <button
          type="button"
          className="tt-btn tt-btn--accent"
          onClick={onClick}
          disabled={!supported || pending}
        >
          {pending ? 'Ativando…' : 'Ativar notificações'}
        </button>
      </div>
      {(status || !supported) && (
        <div className="muted" role="status" style={{ marginTop: 8, fontSize: 12.5 }}>
          {status ? MENSAGEM[status] : MENSAGEM.unsupported}
        </div>
      )}
    </div>
  );
}
