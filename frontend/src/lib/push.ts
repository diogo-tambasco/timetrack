// push.ts (fase 16, CA-13/14/17): liga o Web Push do navegador. Pede a
// permissão (gesto do usuário — NÃO no load), monta a subscription com a VAPID
// public key e POSTa no backend. Degrada SEM throw onde não há suporte
// (Safari/iOS antigo, permissão negada) — o banner in-app cobre quem está com
// o app aberto. Zero dependência nova (Push/Notification são nativas — RNF-07).
import { getPushKey, postPushSubscribe } from './api';

// urlBase64ToUint8Array converte a VAPID public key (base64url) pro
// applicationServerKey que o PushManager.subscribe exige.
function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  // ArrayBuffer explícito: o applicationServerKey exige BufferSource
  // (Uint8Array<ArrayBuffer>), não o genérico Uint8Array<ArrayBufferLike>.
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

export type EnablePushResult =
  | 'subscribed'
  | 'unsupported'
  | 'denied'
  | 'no-key'
  | 'error';

// pushSupported reporta se o navegador tem o necessário (SW + PushManager +
// Notification). Os Ajustes escondem/desabilitam o botão se for false.
export function pushSupported(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    'serviceWorker' in navigator &&
    typeof window !== 'undefined' &&
    'PushManager' in window &&
    'Notification' in window
  );
}

// enablePush é disparado por um GESTO do usuário (botão nos Ajustes). Pede a
// permissão, busca a VAPID public key, assina e persiste. Nunca lança — devolve
// o resultado pra UI dar feedback.
export async function enablePush(): Promise<EnablePushResult> {
  if (!pushSupported()) return 'unsupported';

  try {
    const permission = await Notification.requestPermission();
    if (permission !== 'granted') return 'denied';

    const { key } = await getPushKey();
    if (!key) return 'no-key'; // push desligado no servidor (sem VAPID)

    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(key),
    });

    // PushSubscription.toJSON() → { endpoint, keys: { p256dh, auth } }
    const json = sub.toJSON();
    const endpoint = json.endpoint ?? sub.endpoint;
    const p256dh = json.keys?.p256dh;
    const auth = json.keys?.auth;
    if (!endpoint || !p256dh || !auth) return 'error';

    await postPushSubscribe({ endpoint, keys: { p256dh, auth } });
    return 'subscribed';
  } catch {
    return 'error';
  }
}
