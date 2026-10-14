// presence.spec — fase 21 T-11 (CA-22). LIMITAÇÃO DO STUB: o googlestub
// autentica UM único usuário (diogo.fixture@example.com — cmd/googlestub /
// testdata/google/userinfo.json). Logo NÃO dá pra exercitar A→B com dois
// usuários reais logados via Playwright. O cross-user (presence de A chegando a
// B AO VIVO pelo WebSocket) é coberto pelos TESTES DE INTEGRAÇÃO GO
// (internal/presence/service_test.go T-03/T-04 + presence_routes_test.go T-05).
//
// O que o E2E faz com o que tem: logado como o user único, abre /workspace,
// confirma que a PresenceStrip BUSCA o snapshot GET /workspaces/{id}/presence
// (200) sem erro e que a página não quebra. Pra ter algo na faixa (a própria
// presence de A NÃO aparece pra A — D-03), semeia por SQL um COLEGA member
// rodando um projeto shared → o snapshot lista esse colega pro usuário logado.
//
// Arquivo PARALELO (não muta o timer do usuário): o colega é OUTRO user.
// Exceção ao red estrutural documentada (precedente do T-19 da 03): o app é
// entregue por T-01..T-10; red honesto = spec rodado ANTES dos ajustes.
import { test, expect } from '@playwright/test';
import { loginViaStub, seedColleagueRunning } from './helpers';

test('CA-22: /workspace busca o snapshot de presence sem erro e a faixa lista o colega rodando (shared)', async ({
  page,
}) => {
  // captura a resposta do snapshot pra provar 200 (a cadeia REST viva)
  const snapPromise = page.waitForResponse(
    (r) => /\/api\/v1\/workspaces\/[^/]+\/presence$/.test(r.url()) && r.request().method() === 'GET',
  );

  await loginViaStub(page);

  // colega member rodando um projeto SHARED → o snapshot do usuário logado
  // lista esse colega (a própria presence de A não aparece pra A — D-03)
  seedColleagueRunning({
    googleId: 'g-presence-colega',
    name: 'Cadu Presence',
    email: 'cadu.presence@example.com',
    projectName: 'Site Presence E2E',
  });

  await page.goto('/workspace');

  // o snapshot respondeu 200 (rota nova viva pela cadeia Caddy→app)
  const snap = await snapPromise;
  expect(snap.status()).toBe(200);

  // a página não quebrou (o bloco da fase 10 segue) E a faixa lista o colega.
  // ESCOPA na presence-strip: o colega é member, então também aparece na lista
  // de membros da fase 10 (Cadu Presence renderiza 2x — strict mode).
  await expect(page.getByTestId('workspace-name')).toBeVisible();
  const strip = page.getByTestId('presence-strip');
  await expect(strip).toBeVisible();
  await expect(strip.getByText('Cadu Presence')).toBeVisible();
  // projeto shared → o nome aparece na faixa (CA-05)
  await expect(strip.getByText('Site Presence E2E')).toBeVisible();
});
