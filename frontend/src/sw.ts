/// <reference lib="webworker" />
// Service worker customizado (fase 16, CA-17/RNF-06): estratégia
// injectManifest do vite-plugin-pwa. Substitui o generateSW MANUALMENTE —
// recria o precache do app shell + navigateFallback + denylist /api|/auth|
// /healthz (NÃO regredir offline/login das fases 00/01) E adiciona os
// handlers push/notificationclick do Web Push.
//
// PROIBIDO (herdado da fase 04 §6): cachear /api ou estado de timer aqui.
import { precacheAndRoute, createHandlerBoundToURL } from 'workbox-precaching';
import { NavigationRoute, registerRoute } from 'workbox-routing';

declare const self: ServiceWorkerGlobalScope;

// __WB_MANIFEST é injetado pelo vite-plugin-pwa (injectManifest) com a lista
// de arquivos do app shell a precachear (mesmo que o generateSW fazia).
precacheAndRoute(self.__WB_MANIFEST);

// navigateFallback: navegações (SPA) caem no index.html precacheado, EXCETO
// /api, /auth e /healthz — rotas do SERVIDOR (OAuth redirects, JSON), que o SW
// nunca pode responder com o shell (quebraria o login — fase 00/01).
const fallback = createHandlerBoundToURL('/index.html');
const navigationRoute = new NavigationRoute(fallback, {
  denylist: [/^\/api\//, /^\/auth\//, /^\/healthz/],
});
registerRoute(navigationRoute);

// take over assim que ativar (autoUpdate da config antiga)
self.addEventListener('install', () => {
  void self.skipWaiting();
});
self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

// push (CA-17): mostra a notificação do corte. O payload é o JSON que o
// backend assinou ({title, body, kind}); sem corpo, usa um texto padrão.
self.addEventListener('push', (event) => {
  let data: { title?: string; body?: string } = {};
  try {
    data = event.data ? (event.data.json() as { title?: string; body?: string }) : {};
  } catch {
    data = {};
  }
  const title = data.title ?? 'Timetrack';
  const body = data.body ?? 'Seu timer foi pausado automaticamente após 2h.';
  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon: '/icons/icon-192.png',
      badge: '/icons/icon-192.png',
      tag: 'timer-auto-cut',
    }),
  );
});

// notificationclick (CA-17): foca uma aba aberta do app ou abre uma nova em /.
self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  event.waitUntil(
    (async () => {
      const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
      for (const client of all) {
        if ('focus' in client) return client.focus();
      }
      if (self.clients.openWindow) return self.clients.openWindow('/');
    })(),
  );
});
