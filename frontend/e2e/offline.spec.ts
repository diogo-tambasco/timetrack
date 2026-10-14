import { test, expect } from '@playwright/test';
import { loginViaStub } from './helpers';

// CA-14: após uma visita online, o shell carrega offline a partir do
// precache do service worker (rail + TimerBar renderizam sem rede).
// Plano B documentado (tasks.md T-14): se o setOffline do Playwright não
// alcançar os fetches do SW, derrubar o server / page.route com abort —
// nunca dar o critério por verde sem provar o offline real.
test('app shell carrega offline após visita online (CA-14)', async ({ page, context }) => {
  await loginViaStub(page);
  await expect(page.locator('aside.tt-sidenav')).toBeVisible();

  // espera o SW ativo (precache concluído na fase de install)
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready;
  });

  await context.setOffline(true);
  await page.reload();

  await expect(page.locator('aside.tt-sidenav')).toBeVisible();
  await expect(page.getByTestId('timerbar')).toBeVisible();
  await expect(page.getByTestId('timerbar').locator('.mono')).toHaveText('0:00:00');

  await context.setOffline(false);
});
